package googleidentity

import (
	"context"
	"strings"

	"github.com/thinhnguyenwilliam/book-store/backend/internal/auth/application"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/auth/domain"
	"google.golang.org/api/idtoken"
)

const (
	googleIssuer       = "accounts.google.com"
	googleSecureIssuer = "https://accounts.google.com"
)

type Verifier struct {
	clientID string
}

func NewVerifier(clientID string) *Verifier {
	return &Verifier{clientID: strings.TrimSpace(clientID)}
}

func (v *Verifier) Verify(ctx context.Context, credential, expectedNonce string) (application.VerifiedIdentity, error) {
	if v.clientID == "" {
		return application.VerifiedIdentity{}, domain.ErrIdentityUnavailable
	}
	expectedNonce = strings.TrimSpace(expectedNonce)
	if strings.TrimSpace(credential) == "" || expectedNonce == "" {
		return application.VerifiedIdentity{}, domain.ErrInvalidIdentity
	}

	payload, err := idtoken.Validate(ctx, credential, v.clientID)
	if err != nil {
		return application.VerifiedIdentity{}, domain.ErrInvalidIdentity
	}
	if payload.Issuer != googleIssuer && payload.Issuer != googleSecureIssuer {
		return application.VerifiedIdentity{}, domain.ErrInvalidIdentity
	}

	email, emailOK := stringClaim(payload.Claims, "email")
	displayName, _ := stringClaim(payload.Claims, "name")
	hostedDomain, _ := stringClaim(payload.Claims, "hd")
	emailVerified, verifiedOK := boolClaim(payload.Claims, "email_verified")
	if payload.Subject == "" || !emailOK || !verifiedOK || !emailVerified || !nonceMatches(payload.Claims, expectedNonce) {
		return application.VerifiedIdentity{}, domain.ErrInvalidIdentity
	}

	normalizedEmail := domain.NormalizeEmail(email)
	return application.VerifiedIdentity{
		Provider:           domain.IdentityProviderGoogle,
		Subject:            payload.Subject,
		Email:              normalizedEmail,
		DisplayName:        displayName,
		EmailVerified:      true,
		EmailAuthoritative: strings.HasSuffix(normalizedEmail, "@gmail.com") || hostedDomain != "",
	}, nil
}

func nonceMatches(claims map[string]any, expected string) bool {
	nonce, ok := stringClaim(claims, "nonce")
	return ok && expected != "" && nonce == expected
}

func stringClaim(claims map[string]any, name string) (string, bool) {
	value, ok := claims[name].(string)
	value = strings.TrimSpace(value)
	return value, ok && value != ""
}

func boolClaim(claims map[string]any, name string) (bool, bool) {
	value, ok := claims[name]
	if !ok {
		return false, false
	}
	verified, ok := value.(bool)
	if !ok {
		return false, false
	}
	return verified, true
}

var _ application.IdentityVerifier = (*Verifier)(nil)
