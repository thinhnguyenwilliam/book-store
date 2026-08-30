package fcm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/thinhnguyenwilliam/book-store/backend/internal/notification/domain"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const messagingScope = "https://www.googleapis.com/auth/firebase.messaging"

type Config struct {
	ProjectID       string
	CredentialsFile string
	StorefrontURL   string
	AdminURL        string
	Timeout         time.Duration
}

type Sender struct {
	client        *http.Client
	endpoint      string
	storefrontURL string
	adminURL      string
}

type sendRequest struct {
	Message message `json:"message"`
}

type message struct {
	Token        string            `json:"token"`
	Notification notification      `json:"notification"`
	Data         map[string]string `json:"data,omitempty"`
	WebPush      *webPush          `json:"webpush,omitempty"`
}

type notification struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

type webPush struct {
	FCMOptions webPushOptions `json:"fcm_options"`
}

type webPushOptions struct {
	Link string `json:"link"`
}

type sendResponse struct {
	Name  string `json:"name"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
		Details []struct {
			ErrorCode string `json:"errorCode"`
		} `json:"details"`
	} `json:"error"`
}

func NewSender(ctx context.Context, config Config) (*Sender, error) {
	projectID := strings.TrimSpace(config.ProjectID)
	if projectID == "" || config.Timeout <= 0 {
		return nil, errors.New("invalid FCM sender config")
	}
	credentials, err := credentials(ctx, strings.TrimSpace(config.CredentialsFile))
	if err != nil {
		return nil, err
	}
	client := oauth2.NewClient(ctx, credentials.TokenSource)
	client.Timeout = config.Timeout
	return &Sender{
		client:        client,
		endpoint:      "https://fcm.googleapis.com/v1/projects/" + url.PathEscape(projectID) + "/messages:send",
		storefrontURL: strings.TrimRight(config.StorefrontURL, "/"),
		adminURL:      strings.TrimRight(config.AdminURL, "/"),
	}, nil
}

func credentials(ctx context.Context, path string) (*google.Credentials, error) {
	if path == "" {
		item, err := google.FindDefaultCredentials(ctx, messagingScope)
		if err != nil {
			return nil, fmt.Errorf("find FCM application default credentials: %w", err)
		}
		return item, nil
	}
	payload, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read FCM credentials file: %w", err)
	}
	item, err := google.CredentialsFromJSONWithType(ctx, payload, google.ServiceAccount, messagingScope)
	if err != nil {
		return nil, fmt.Errorf("parse FCM credentials file: %w", err)
	}
	return item, nil
}

func (s *Sender) Send(ctx context.Context, delivery domain.PushDelivery) (string, error) {
	data := pushData(delivery)
	item := message{
		Token:        delivery.RegistrationToken,
		Notification: notification{Title: delivery.Title, Body: delivery.Body},
		Data:         data,
	}
	if delivery.Platform == "web" {
		if link := s.notificationLink(delivery); link != "" {
			item.WebPush = &webPush{FCMOptions: webPushOptions{Link: link}}
		}
	}
	payload, err := json.Marshal(sendRequest{Message: item})
	if err != nil {
		return "", fmt.Errorf("encode FCM request: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, s.endpoint, bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("create FCM request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := s.client.Do(request)
	if err != nil {
		return "", fmt.Errorf("send FCM request: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(response.Body, 64<<10))
	if err != nil {
		return "", fmt.Errorf("read FCM response: %w", err)
	}
	var result sendResponse
	if len(body) > 0 {
		_ = json.Unmarshal(body, &result)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		if invalidRegistration(result) {
			return "", fmt.Errorf("%w: FCM rejected an inactive installation", domain.ErrPushRegistrationInvalid)
		}
		status := http.StatusText(response.StatusCode)
		if result.Error != nil && strings.TrimSpace(result.Error.Status) != "" {
			status = result.Error.Status
		}
		return "", fmt.Errorf("FCM request failed with status %d (%s)", response.StatusCode, status)
	}
	if strings.TrimSpace(result.Name) == "" {
		return "", errors.New("FCM response did not contain a message name")
	}
	return result.Name, nil
}

func pushData(delivery domain.PushDelivery) map[string]string {
	result := map[string]string{
		"notification_id": delivery.NotificationID,
		"type":            delivery.NotificationType,
	}
	var data map[string]any
	if json.Unmarshal(delivery.Data, &data) != nil {
		return result
	}
	for key, value := range data {
		switch typed := value.(type) {
		case string:
			result[key] = typed
		default:
			encoded, err := json.Marshal(value)
			if err == nil {
				result[key] = string(encoded)
			}
		}
	}
	return result
}

func (s *Sender) notificationLink(delivery domain.PushDelivery) string {
	base := s.storefrontURL
	path := "/"
	if delivery.Application == "admin" {
		base = s.adminURL
		if delivery.NotificationType == "chat.message.created" {
			path = "/tro-chuyen"
		}
	} else if strings.HasPrefix(delivery.NotificationType, "payment.") {
		path = "/tai-khoan"
	}
	if base == "" {
		return ""
	}
	parsed, err := url.Parse(base)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		// FCM requires an HTTPS click action. Local HTTP development falls
		// back to the service worker's same-origin root URL.
		return ""
	}
	return base + path
}

func invalidRegistration(response sendResponse) bool {
	if response.Error == nil {
		return false
	}
	if response.Error.Status == "UNREGISTERED" || strings.Contains(response.Error.Message, "UNREGISTERED") {
		return true
	}
	for _, detail := range response.Error.Details {
		if detail.ErrorCode == "UNREGISTERED" {
			return true
		}
	}
	return false
}
