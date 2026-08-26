package vnpay

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/payment/domain"
)

const version = "2.1.0"

type Config struct {
	PayURL      string
	APIURL      string
	TMNCode     string
	HashSecret  string
	ReturnURL   string
	ServerIP    string
	TimeZone    string
	ExpireAfter time.Duration
	HTTPTimeout time.Duration
}

type Gateway struct {
	config   Config
	location *time.Location
	client   *http.Client
}

func New(config Config) (*Gateway, error) {
	if strings.TrimSpace(config.PayURL) == "" || strings.TrimSpace(config.APIURL) == "" ||
		strings.TrimSpace(config.TMNCode) == "" || strings.TrimSpace(config.HashSecret) == "" ||
		strings.TrimSpace(config.ReturnURL) == "" {
		return nil, domain.ErrProviderDisabled
	}
	if _, err := url.ParseRequestURI(config.PayURL); err != nil {
		return nil, fmt.Errorf("invalid VNPAY pay URL: %w", err)
	}
	if _, err := url.ParseRequestURI(config.APIURL); err != nil {
		return nil, fmt.Errorf("invalid VNPAY API URL: %w", err)
	}
	if _, err := url.ParseRequestURI(config.ReturnURL); err != nil {
		return nil, fmt.Errorf("invalid VNPAY return URL: %w", err)
	}
	location, err := time.LoadLocation(config.TimeZone)
	if err != nil {
		return nil, fmt.Errorf("load VNPAY timezone: %w", err)
	}
	if config.ExpireAfter <= 0 {
		config.ExpireAfter = 15 * time.Minute
	}
	if config.HTTPTimeout <= 0 {
		config.HTTPTimeout = 5 * time.Second
	}
	if config.ServerIP == "" {
		config.ServerIP = "127.0.0.1"
	}
	return &Gateway{config: config, location: location, client: &http.Client{Timeout: config.HTTPTimeout}}, nil
}

func (g *Gateway) Name() string { return domain.ProviderVNPay }

func (g *Gateway) CreateCheckout(_ context.Context, request domain.CheckoutRequest) (domain.Checkout, error) {
	if request.Currency != "VND" || request.AmountCents <= 0 || request.AmountCents > math.MaxInt64/100 {
		return domain.Checkout{}, domain.ErrInvalidInput
	}
	createdAt := request.CreatedAt.In(g.location)
	expiresAt := createdAt.Add(g.config.ExpireAfter)
	locale := strings.ToLower(strings.TrimSpace(request.Locale))
	if locale != "en" {
		locale = "vn"
	}
	clientIP := strings.TrimSpace(request.ClientIP)
	if clientIP == "" {
		clientIP = "127.0.0.1"
	}
	parameters := url.Values{
		"vnp_Version":    {version},
		"vnp_Command":    {"pay"},
		"vnp_TmnCode":    {g.config.TMNCode},
		"vnp_Amount":     {strconv.FormatInt(request.AmountCents*100, 10)},
		"vnp_CreateDate": {createdAt.Format("20060102150405")},
		"vnp_CurrCode":   {"VND"},
		"vnp_ExpireDate": {expiresAt.Format("20060102150405")},
		"vnp_IpAddr":     {clientIP},
		"vnp_Locale":     {locale},
		"vnp_OrderInfo":  {"Thanh toan don hang " + request.OrderID},
		"vnp_OrderType":  {"other"},
		"vnp_ReturnUrl":  {g.config.ReturnURL},
		"vnp_TxnRef":     {request.PaymentID},
	}
	if bankCode := strings.TrimSpace(request.BankCode); bankCode != "" {
		parameters.Set("vnp_BankCode", bankCode)
	}
	signature := g.sign(parameters.Encode())
	parameters.Set("vnp_SecureHash", signature)
	return domain.Checkout{
		ProviderReference: request.PaymentID,
		URL:               g.config.PayURL + "?" + parameters.Encode(),
		ExpiresAt:         expiresAt.UTC(),
	}, nil
}

func (g *Gateway) ParseWebhook(_ context.Context, parameters map[string]string) (domain.GatewayResult, error) {
	values := make(url.Values)
	for key, value := range parameters {
		if strings.HasPrefix(key, "vnp_") && key != "vnp_SecureHash" && key != "vnp_SecureHashType" {
			values.Set(key, value)
		}
	}
	if !g.validSignature(values.Encode(), parameters["vnp_SecureHash"]) {
		return domain.GatewayResult{}, domain.ErrInvalidSignature
	}
	if parameters["vnp_TmnCode"] != g.config.TMNCode || strings.TrimSpace(parameters["vnp_TxnRef"]) == "" {
		return domain.GatewayResult{}, domain.ErrInvalidInput
	}
	amount, err := parseAmount(parameters["vnp_Amount"])
	if err != nil {
		return domain.GatewayResult{}, err
	}
	status := domain.StatusFailed
	if parameters["vnp_ResponseCode"] == "00" && parameters["vnp_TransactionStatus"] == "00" {
		status = domain.StatusSucceeded
	}
	payload := []byte(values.Encode())
	eventHash := sha256.Sum256(payload)
	return domain.GatewayResult{
		Provider:              domain.ProviderVNPay,
		EventID:               hex.EncodeToString(eventHash[:]),
		ProviderReference:     parameters["vnp_TxnRef"],
		ProviderTransactionID: parameters["vnp_TransactionNo"],
		Status:                status,
		AmountCents:           amount,
		OccurredAt:            g.parseTime(parameters["vnp_PayDate"]),
		RawPayload:            payload,
	}, nil
}

func (g *Gateway) Query(ctx context.Context, payment *domain.Payment) (domain.GatewayResult, error) {
	now := time.Now().In(g.location)
	requestID := strings.ReplaceAll(uuid.NewString(), "-", "")
	request := queryRequest{
		RequestID: requestID, Version: version, Command: "querydr", TMNCode: g.config.TMNCode,
		TxnRef: payment.ProviderReference, OrderInfo: "Truy van don hang " + payment.OrderID,
		TransactionDate: payment.CreatedAt.In(g.location).Format("20060102150405"),
		CreateDate:      now.Format("20060102150405"), IPAddress: g.config.ServerIP,
	}
	request.SecureHash = g.sign(strings.Join([]string{
		request.RequestID, request.Version, request.Command, request.TMNCode, request.TxnRef,
		request.TransactionDate, request.CreateDate, request.IPAddress, request.OrderInfo,
	}, "|"))
	payload, err := json.Marshal(request)
	if err != nil {
		return domain.GatewayResult{}, fmt.Errorf("encode VNPAY query: %w", err)
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, g.config.APIURL, bytes.NewReader(payload))
	if err != nil {
		return domain.GatewayResult{}, fmt.Errorf("create VNPAY query: %w", err)
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	response, err := g.client.Do(httpRequest)
	if err != nil {
		return domain.GatewayResult{}, fmt.Errorf("query VNPAY transaction: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return domain.GatewayResult{}, fmt.Errorf("read VNPAY query response: %w", err)
	}
	if response.StatusCode != http.StatusOK {
		return domain.GatewayResult{}, fmt.Errorf("VNPAY query returned HTTP %d", response.StatusCode)
	}
	var result queryResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return domain.GatewayResult{}, fmt.Errorf("decode VNPAY query response: %w", err)
	}
	if !g.validSignature(result.signaturePayload(), result.SecureHash) {
		return domain.GatewayResult{}, domain.ErrInvalidSignature
	}
	if result.TMNCode != g.config.TMNCode {
		return domain.GatewayResult{}, domain.ErrInvalidInput
	}
	if result.ResponseCode != "00" {
		return domain.GatewayResult{}, fmt.Errorf("VNPAY query failed: %s", result.ResponseCode)
	}
	amount, err := parseAmount(result.Amount)
	if err != nil {
		return domain.GatewayResult{}, err
	}
	status := transactionStatus(result.TransactionType, result.TransactionStatus)
	return domain.GatewayResult{
		Provider: domain.ProviderVNPay, EventID: "query:" + result.ResponseID,
		ProviderReference: result.TxnRef, ProviderTransactionID: result.TransactionNo,
		Status: status, AmountCents: amount, OccurredAt: g.parseTime(result.PayDate), RawPayload: body,
	}, nil
}

func (g *Gateway) Refund(ctx context.Context, input domain.RefundRequest) (domain.GatewayResult, error) {
	if input.Payment == nil || input.Payment.AmountCents <= 0 ||
		input.Payment.AmountCents > math.MaxInt64/100 || input.Payment.ProviderReference == "" {
		return domain.GatewayResult{}, domain.ErrInvalidInput
	}
	now := input.CreatedAt.In(g.location)
	requestHash := sha256.Sum256([]byte(input.Payment.ID + "|" + input.IdempotencyKey))
	request := refundRequest{
		RequestID: hex.EncodeToString(requestHash[:16]), Version: version, Command: "refund",
		TMNCode: g.config.TMNCode, TransactionType: "02", TxnRef: input.Payment.ProviderReference,
		Amount:          strconv.FormatInt(input.Payment.AmountCents*100, 10),
		TransactionNo:   input.Payment.ProviderTransactionID,
		TransactionDate: input.Payment.CreatedAt.In(g.location).Format("20060102150405"),
		CreateBy:        strings.TrimSpace(input.CreatedBy), CreateDate: now.Format("20060102150405"),
		IPAddress: g.config.ServerIP, OrderInfo: strings.TrimSpace(input.Reason),
	}
	if request.CreateBy == "" {
		request.CreateBy = "payment-service"
	}
	if request.OrderInfo == "" {
		request.OrderInfo = "Hoan tien don hang " + input.Payment.OrderID
	}
	request.SecureHash = g.sign(request.signaturePayload())
	payload, err := json.Marshal(request)
	if err != nil {
		return domain.GatewayResult{}, fmt.Errorf("encode VNPAY refund: %w", err)
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, g.config.APIURL, bytes.NewReader(payload))
	if err != nil {
		return domain.GatewayResult{}, fmt.Errorf("create VNPAY refund: %w", err)
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	response, err := g.client.Do(httpRequest)
	if err != nil {
		return domain.GatewayResult{}, fmt.Errorf("refund VNPAY transaction: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return domain.GatewayResult{}, fmt.Errorf("read VNPAY refund response: %w", err)
	}
	if response.StatusCode != http.StatusOK {
		return domain.GatewayResult{}, fmt.Errorf("VNPAY refund returned HTTP %d", response.StatusCode)
	}
	var result refundResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return domain.GatewayResult{}, fmt.Errorf("decode VNPAY refund response: %w", err)
	}
	if !g.validSignature(result.signaturePayload(), result.SecureHash) {
		return domain.GatewayResult{}, domain.ErrInvalidSignature
	}
	if result.TMNCode != g.config.TMNCode || result.TxnRef != input.Payment.ProviderReference {
		return domain.GatewayResult{}, domain.ErrInvalidInput
	}
	amount := input.Payment.AmountCents
	if result.Amount != "" {
		amount, err = parseAmount(result.Amount)
		if err != nil {
			return domain.GatewayResult{}, err
		}
	}
	status := transactionStatus(result.TransactionType, result.TransactionStatus)
	if result.ResponseCode == "94" {
		status = domain.StatusRefundPending
	} else if result.ResponseCode != "00" {
		return domain.GatewayResult{}, fmt.Errorf("VNPAY refund failed: %s", result.ResponseCode)
	}
	return domain.GatewayResult{
		Provider: domain.ProviderVNPay, EventID: "refund:" + result.ResponseID,
		ProviderReference: result.TxnRef, ProviderTransactionID: result.TransactionNo,
		Status: status, AmountCents: amount, OccurredAt: g.parseTime(result.PayDate), RawPayload: body,
	}, nil
}

func transactionStatus(transactionType, status string) string {
	if transactionType == "02" || transactionType == "03" {
		switch status {
		case "00":
			return domain.StatusRefunded
		case "05", "06":
			return domain.StatusRefundPending
		default:
			return domain.StatusFailed
		}
	}
	switch status {
	case "00":
		return domain.StatusSucceeded
	case "01":
		return domain.StatusPending
	default:
		return domain.StatusFailed
	}
}

func (g *Gateway) sign(payload string) string {
	mac := hmac.New(sha512.New, []byte(g.config.HashSecret))
	_, _ = mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

func (g *Gateway) validSignature(payload, signature string) bool {
	expected, err := hex.DecodeString(g.sign(payload))
	if err != nil {
		return false
	}
	actual, err := hex.DecodeString(strings.TrimSpace(signature))
	return err == nil && hmac.Equal(expected, actual)
}

func (g *Gateway) parseTime(value string) time.Time {
	parsed, err := time.ParseInLocation("20060102150405", value, g.location)
	if err != nil {
		return time.Time{}
	}
	return parsed.UTC()
}

func parseAmount(value string) (int64, error) {
	raw, err := strconv.ParseInt(value, 10, 64)
	if err != nil || raw <= 0 || raw%100 != 0 {
		return 0, domain.ErrInvalidInput
	}
	return raw / 100, nil
}

type queryRequest struct {
	RequestID       string `json:"vnp_RequestId"`
	Version         string `json:"vnp_Version"`
	Command         string `json:"vnp_Command"`
	TMNCode         string `json:"vnp_TmnCode"`
	TxnRef          string `json:"vnp_TxnRef"`
	OrderInfo       string `json:"vnp_OrderInfo"`
	TransactionDate string `json:"vnp_TransactionDate"`
	CreateDate      string `json:"vnp_CreateDate"`
	IPAddress       string `json:"vnp_IpAddr"`
	SecureHash      string `json:"vnp_SecureHash"`
}

type queryResponse struct {
	ResponseID        string `json:"vnp_ResponseId"`
	Command           string `json:"vnp_Command"`
	ResponseCode      string `json:"vnp_ResponseCode"`
	Message           string `json:"vnp_Message"`
	TMNCode           string `json:"vnp_TmnCode"`
	TxnRef            string `json:"vnp_TxnRef"`
	Amount            string `json:"vnp_Amount"`
	BankCode          string `json:"vnp_BankCode"`
	PayDate           string `json:"vnp_PayDate"`
	TransactionNo     string `json:"vnp_TransactionNo"`
	TransactionType   string `json:"vnp_TransactionType"`
	TransactionStatus string `json:"vnp_TransactionStatus"`
	OrderInfo         string `json:"vnp_OrderInfo"`
	PromotionCode     string `json:"vnp_PromotionCode"`
	PromotionAmount   string `json:"vnp_PromotionAmount"`
	SecureHash        string `json:"vnp_SecureHash"`
}

func (r queryResponse) signaturePayload() string {
	return strings.Join([]string{
		r.ResponseID, r.Command, r.ResponseCode, r.Message, r.TMNCode, r.TxnRef, r.Amount,
		r.BankCode, r.PayDate, r.TransactionNo, r.TransactionType, r.TransactionStatus,
		r.OrderInfo, r.PromotionCode, r.PromotionAmount,
	}, "|")
}

type refundRequest struct {
	RequestID       string `json:"vnp_RequestId"`
	Version         string `json:"vnp_Version"`
	Command         string `json:"vnp_Command"`
	TMNCode         string `json:"vnp_TmnCode"`
	TransactionType string `json:"vnp_TransactionType"`
	TxnRef          string `json:"vnp_TxnRef"`
	Amount          string `json:"vnp_Amount"`
	TransactionNo   string `json:"vnp_TransactionNo"`
	TransactionDate string `json:"vnp_TransactionDate"`
	CreateBy        string `json:"vnp_CreateBy"`
	CreateDate      string `json:"vnp_CreateDate"`
	IPAddress       string `json:"vnp_IpAddr"`
	OrderInfo       string `json:"vnp_OrderInfo"`
	SecureHash      string `json:"vnp_SecureHash"`
}

func (r refundRequest) signaturePayload() string {
	return strings.Join([]string{
		r.RequestID, r.Version, r.Command, r.TMNCode, r.TransactionType, r.TxnRef,
		r.Amount, r.TransactionNo, r.TransactionDate, r.CreateBy, r.CreateDate,
		r.IPAddress, r.OrderInfo,
	}, "|")
}

type refundResponse struct {
	ResponseID        string `json:"vnp_ResponseId"`
	Command           string `json:"vnp_Command"`
	ResponseCode      string `json:"vnp_ResponseCode"`
	Message           string `json:"vnp_Message"`
	TMNCode           string `json:"vnp_TmnCode"`
	TxnRef            string `json:"vnp_TxnRef"`
	Amount            string `json:"vnp_Amount"`
	BankCode          string `json:"vnp_BankCode"`
	PayDate           string `json:"vnp_PayDate"`
	TransactionNo     string `json:"vnp_TransactionNo"`
	TransactionType   string `json:"vnp_TransactionType"`
	TransactionStatus string `json:"vnp_TransactionStatus"`
	OrderInfo         string `json:"vnp_OrderInfo"`
	SecureHash        string `json:"vnp_SecureHash"`
}

func (r refundResponse) signaturePayload() string {
	return strings.Join([]string{
		r.ResponseID, r.Command, r.ResponseCode, r.Message, r.TMNCode, r.TxnRef,
		r.Amount, r.BankCode, r.PayDate, r.TransactionNo, r.TransactionType,
		r.TransactionStatus, r.OrderInfo,
	}, "|")
}
