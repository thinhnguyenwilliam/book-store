package vnpay

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/payment/domain"
)

const testSecret = "test-vnpay-hash-secret"

func newTestGateway(t *testing.T) *Gateway {
	t.Helper()
	gateway, err := New(Config{
		PayURL:  "https://sandbox.vnpayment.vn/paymentv2/vpcpay.html",
		APIURL:  "https://sandbox.vnpayment.vn/merchant_webapi/api/transaction",
		TMNCode: "TESTCODE", HashSecret: testSecret,
		ReturnURL: "https://store.example.test/payment/result", ServerIP: "127.0.0.1",
		TimeZone: "Asia/Ho_Chi_Minh", ExpireAfter: 15 * time.Minute, HTTPTimeout: time.Second,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return gateway
}

func TestCreateCheckoutSignsSortedParameters(t *testing.T) {
	gateway := newTestGateway(t)
	paymentID := uuid.NewString()
	checkout, err := gateway.CreateCheckout(context.Background(), domain.CheckoutRequest{
		PaymentID: paymentID, OrderID: uuid.NewString(), AmountCents: 10000, Currency: "VND",
		ClientIP: "127.0.0.1", Locale: "vn", CreatedAt: time.Date(2026, 8, 26, 8, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("CreateCheckout() error = %v", err)
	}
	parsed, err := url.Parse(checkout.URL)
	if err != nil {
		t.Fatalf("parse checkout URL: %v", err)
	}
	parameters := parsed.Query()
	signature := parameters.Get("vnp_SecureHash")
	parameters.Del("vnp_SecureHash")
	if signature != independentSignature(testSecret, parameters.Encode()) {
		t.Fatalf("checkout signature is invalid")
	}
	if parameters.Get("vnp_Amount") != "1000000" || parameters.Get("vnp_TxnRef") != paymentID {
		t.Fatalf("unexpected checkout parameters: %v", parameters)
	}
}

func TestParseWebhookVerifiesSignatureAndAmount(t *testing.T) {
	gateway := newTestGateway(t)
	values := url.Values{
		"vnp_Amount": {"1000000"}, "vnp_ResponseCode": {"00"},
		"vnp_TmnCode":           {"TESTCODE"},
		"vnp_TransactionStatus": {"00"}, "vnp_TransactionNo": {"12345"},
		"vnp_TxnRef": {uuid.NewString()}, "vnp_PayDate": {"20260826150000"},
	}
	parameters := make(map[string]string, len(values)+1)
	for key := range values {
		parameters[key] = values.Get(key)
	}
	parameters["vnp_SecureHash"] = independentSignature(testSecret, values.Encode())

	result, err := gateway.ParseWebhook(context.Background(), parameters)
	if err != nil {
		t.Fatalf("ParseWebhook() error = %v", err)
	}
	if result.Status != domain.StatusSucceeded || result.AmountCents != 10000 || result.EventID == "" {
		t.Fatalf("unexpected gateway result: %+v", result)
	}

	parameters["vnp_Amount"] = "2000000"
	if _, err := gateway.ParseWebhook(context.Background(), parameters); !errors.Is(err, domain.ErrInvalidSignature) {
		t.Fatalf("tampered webhook error = %v, want invalid signature", err)
	}
}

func TestRefundSignsRequestAndVerifiesResponse(t *testing.T) {
	var received refundRequest
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if err := json.NewDecoder(request.Body).Decode(&received); err != nil {
			return nil, err
		}
		if received.SecureHash != independentSignature(testSecret, received.signaturePayload()) {
			t.Errorf("refund request signature is invalid")
		}
		response := refundResponse{
			ResponseID: "refund-response-1", Command: "refund", ResponseCode: "00",
			Message: "success", TMNCode: "TESTCODE", TxnRef: received.TxnRef,
			Amount: received.Amount, BankCode: "NCB", PayDate: "20260826150000",
			TransactionNo: "987654", TransactionType: "02", TransactionStatus: "00",
			OrderInfo: received.OrderInfo,
		}
		response.SecureHash = independentSignature(testSecret, response.signaturePayload())
		body, err := json.Marshal(response)
		if err != nil {
			return nil, err
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(bytes.NewReader(body)),
			Request:    request,
		}, nil
	})

	gateway := newTestGateway(t)
	gateway.client = &http.Client{Transport: transport}
	payment := &domain.Payment{
		ID: uuid.NewString(), OrderID: uuid.NewString(), AmountCents: 10000,
		ProviderReference: uuid.NewString(), ProviderTransactionID: "123456",
		CreatedAt: time.Date(2026, 8, 26, 8, 0, 0, 0, time.UTC),
	}
	result, err := gateway.Refund(context.Background(), domain.RefundRequest{
		Payment: payment, IdempotencyKey: "refund-order-1", Reason: "customer cancellation",
		CreatedBy: "admin", CreatedAt: time.Date(2026, 8, 26, 9, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("Refund() error = %v", err)
	}
	if result.Status != domain.StatusRefunded || result.AmountCents != payment.AmountCents {
		t.Fatalf("unexpected refund result: %+v", result)
	}
	if received.RequestID == "" || len(received.RequestID) > 32 || received.TransactionType != "02" {
		t.Fatalf("unexpected refund request: %+v", received)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func independentSignature(secret, payload string) string {
	mac := hmac.New(sha512.New, []byte(secret))
	_, _ = mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}
