package security

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/auth/application"
	"github.com/thinhnguyenwilliam/book-store/backend/internal/auth/domain"
)

type tokenClaims struct {
	Email string   `json:"email"`
	Roles []string `json:"roles"`
	jwt.RegisteredClaims
}

type TokenManager struct {
	secret []byte
	issuer string
	ttl    time.Duration
	now    func() time.Time
}

func NewTokenManager(secret, issuer string, ttl time.Duration) *TokenManager {
	return &TokenManager{secret: []byte(secret), issuer: issuer, ttl: ttl, now: time.Now}
}

func (m *TokenManager) Issue(claims application.Claims) (string, time.Time, error) {
	now := m.now().UTC()
	expiresAt := now.Add(m.ttl)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, tokenClaims{
		Email: claims.Email,
		Roles: claims.Roles,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   claims.UserID,
			Issuer:    m.issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	})

	signed, err := token.SignedString(m.secret)
	return signed, expiresAt, err
}

func (m *TokenManager) Verify(rawToken string) (application.Claims, error) {
	claims := &tokenClaims{}
	token, err := jwt.ParseWithClaims(
		rawToken,
		claims,
		func(token *jwt.Token) (any, error) {
			if token.Method != jwt.SigningMethodHS256 {
				return nil, domain.ErrInvalidToken
			}
			return m.secret, nil
		},
		jwt.WithIssuer(m.issuer),
		jwt.WithExpirationRequired(),
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
	)
	if err != nil || !token.Valid || claims.Subject == "" {
		return application.Claims{}, domain.ErrInvalidToken
	}
	if claims.Email == "" {
		return application.Claims{}, domain.ErrInvalidToken
	}

	return application.Claims{
		UserID: claims.Subject,
		Email:  claims.Email,
		Roles:  claims.Roles,
	}, nil
}
