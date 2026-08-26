package facebookidentity

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/thinhnguyenwilliam/book-store/backend/internal/auth/application"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/auth/domain"
)

const (
	defaultGraphBaseURL = "https://graph.facebook.com"
	maxResponseBytes    = 1 << 20
)

type Verifier struct {
	appID        string
	appSecret    string
	graphVersion string
	baseURL      string
	client       *http.Client
	now          func() time.Time
}

type debugTokenResponse struct {
	Data debugTokenData `json:"data"`
}

type debugTokenData struct {
	AppID               string `json:"app_id"`
	Type                string `json:"type"`
	IsValid             bool   `json:"is_valid"`
	UserID              string `json:"user_id"`
	ExpiresAt           int64  `json:"expires_at"`
	DataAccessExpiresAt int64  `json:"data_access_expires_at"`
}

type profileResponse struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

func NewVerifier(appID, appSecret, graphVersion string) *Verifier {
	return &Verifier{
		appID:        strings.TrimSpace(appID),
		appSecret:    strings.TrimSpace(appSecret),
		graphVersion: normalizeVersion(graphVersion),
		baseURL:      defaultGraphBaseURL,
		client:       &http.Client{Timeout: 3 * time.Second},
		now:          time.Now,
	}
}

func (v *Verifier) Verify(ctx context.Context, accessToken, _ string) (application.VerifiedIdentity, error) {
	if v.appID == "" || v.appSecret == "" || v.graphVersion == "" {
		return application.VerifiedIdentity{}, domain.ErrIdentityUnavailable
	}
	accessToken = strings.TrimSpace(accessToken)
	if accessToken == "" {
		return application.VerifiedIdentity{}, domain.ErrInvalidIdentity
	}

	debug := debugTokenResponse{}
	if err := v.get(ctx, "debug_token", v.appID+"|"+v.appSecret, url.Values{
		"input_token": []string{accessToken},
	}, &debug); err != nil {
		return application.VerifiedIdentity{}, err
	}
	if !v.validDebugToken(debug.Data) {
		return application.VerifiedIdentity{}, domain.ErrInvalidIdentity
	}

	profile := profileResponse{}
	if err := v.get(ctx, "me", accessToken, url.Values{
		"fields":          []string{"id,name,email"},
		"appsecret_proof": []string{v.appSecretProof(accessToken)},
	}, &profile); err != nil {
		return application.VerifiedIdentity{}, err
	}
	profile.Email = domain.NormalizeEmail(profile.Email)
	if profile.ID == "" || profile.ID != debug.Data.UserID || profile.Email == "" {
		return application.VerifiedIdentity{}, domain.ErrInvalidIdentity
	}

	return application.VerifiedIdentity{
		Provider:           domain.IdentityProviderFacebook,
		Subject:            profile.ID,
		Email:              profile.Email,
		DisplayName:        strings.TrimSpace(profile.Name),
		EmailVerified:      true,
		EmailAuthoritative: true,
	}, nil
}

func (v *Verifier) validDebugToken(data debugTokenData) bool {
	now := v.now().Unix()
	return data.IsValid &&
		data.AppID == v.appID &&
		strings.EqualFold(data.Type, "USER") &&
		data.UserID != "" &&
		(data.ExpiresAt == 0 || data.ExpiresAt > now) &&
		(data.DataAccessExpiresAt == 0 || data.DataAccessExpiresAt > now)
}

func (v *Verifier) get(
	ctx context.Context,
	path string,
	accessToken string,
	query url.Values,
	destination any,
) error {
	endpoint := strings.TrimRight(v.baseURL, "/") + "/" + v.graphVersion + "/" + path
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("create Facebook Graph request: %w: %w", err, domain.ErrIdentityProvider)
	}
	request.URL.RawQuery = query.Encode()
	request.Header.Set("Authorization", "Bearer "+accessToken)
	request.Header.Set("Accept", "application/json")

	response, err := v.client.Do(request)
	if err != nil {
		return fmt.Errorf("call Facebook Graph API: %w: %w", err, domain.ErrIdentityProvider)
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, maxResponseBytes))
		if response.StatusCode >= http.StatusInternalServerError || response.StatusCode == http.StatusTooManyRequests {
			return domain.ErrIdentityProvider
		}
		return domain.ErrInvalidIdentity
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, maxResponseBytes))
	if err := decoder.Decode(destination); err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("decode Facebook Graph response: %w: %w", err, domain.ErrIdentityProvider)
	}
	return nil
}

func (v *Verifier) appSecretProof(accessToken string) string {
	digest := hmac.New(sha256.New, []byte(v.appSecret))
	_, _ = digest.Write([]byte(accessToken))
	return hex.EncodeToString(digest.Sum(nil))
}

func normalizeVersion(version string) string {
	version = strings.TrimSpace(version)
	if version == "" {
		return ""
	}
	if strings.HasPrefix(version, "v") {
		return version
	}
	return "v" + version
}

var _ application.IdentityVerifier = (*Verifier)(nil)
