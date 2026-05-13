package auth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type Role string

const (
	RolePlatformAdmin Role = "platform_admin"
	RoleTenantAdmin   Role = "tenant_admin"
	RoleTenantAgent   Role = "tenant_agent"
)

type Claims struct {
	Sub      string `json:"sub"`
	TenantID string `json:"tenant,omitempty"`
	Role     string `json:"role"`
	Email    string `json:"email"`
	Iat      int64  `json:"iat"`
	Exp      int64  `json:"exp"`
}

var (
	ErrInvalidToken = errors.New("invalid_token")
	ErrExpiredToken = errors.New("expired_token")
)

func HashPassword(plain string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func CheckPassword(hash, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}

// Sign produces a HS256 JWT. We hand-roll it to avoid the dependency.
func Sign(secret string, c Claims) (string, error) {
	header := map[string]string{"alg": "HS256", "typ": "JWT"}
	hb, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	pb, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	enc := base64.RawURLEncoding
	unsigned := enc.EncodeToString(hb) + "." + enc.EncodeToString(pb)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(unsigned))
	sig := enc.EncodeToString(mac.Sum(nil))
	return unsigned + "." + sig, nil
}

func Verify(secret, token string) (Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return Claims{}, ErrInvalidToken
	}
	enc := base64.RawURLEncoding
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(parts[0] + "." + parts[1]))
	expected := enc.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expected), []byte(parts[2])) {
		return Claims{}, ErrInvalidToken
	}
	payload, err := enc.DecodeString(parts[1])
	if err != nil {
		return Claims{}, ErrInvalidToken
	}
	var c Claims
	if err := json.Unmarshal(payload, &c); err != nil {
		return Claims{}, ErrInvalidToken
	}
	if time.Now().Unix() > c.Exp {
		return c, ErrExpiredToken
	}
	return c, nil
}

func IssueToken(secret, userID, email, role, tenantID string, ttl time.Duration) (string, time.Time, error) {
	now := time.Now()
	exp := now.Add(ttl)
	c := Claims{
		Sub:      userID,
		TenantID: tenantID,
		Role:     role,
		Email:    email,
		Iat:      now.Unix(),
		Exp:      exp.Unix(),
	}
	tok, err := Sign(secret, c)
	return tok, exp, err
}

// Context plumbing.

type ctxKey int

const claimsKey ctxKey = 1

func WithClaims(ctx context.Context, c Claims) context.Context {
	return context.WithValue(ctx, claimsKey, c)
}

func FromContext(ctx context.Context) (Claims, bool) {
	c, ok := ctx.Value(claimsKey).(Claims)
	return c, ok
}

// TenantFromContext returns the tenant the caller is scoped to, or an error if the caller is unscoped.
func TenantFromContext(ctx context.Context) (string, error) {
	c, ok := FromContext(ctx)
	if !ok {
		return "", errors.New("no_claims")
	}
	if c.TenantID == "" {
		return "", errors.New("platform_caller_requires_tenant_query")
	}
	return c.TenantID, nil
}

// TenantOrOverride returns the tenant scope to use: platform admins may pass ?tenant=xxx to scope a request.
func TenantOrOverride(ctx context.Context, override string) (string, error) {
	c, ok := FromContext(ctx)
	if !ok {
		return "", errors.New("no_claims")
	}
	if c.Role == string(RolePlatformAdmin) {
		if override != "" {
			return override, nil
		}
		if c.TenantID != "" {
			return c.TenantID, nil
		}
		return "", errors.New("tenant_required")
	}
	if c.TenantID == "" {
		return "", errors.New("user_has_no_tenant")
	}
	return c.TenantID, nil
}

func IsPlatformAdmin(ctx context.Context) bool {
	c, ok := FromContext(ctx)
	return ok && c.Role == string(RolePlatformAdmin)
}

func ParseBearer(header string) (string, error) {
	if header == "" {
		return "", ErrInvalidToken
	}
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", ErrInvalidToken
	}
	return strings.TrimSpace(parts[1]), nil
}

// String helpers (kept here to keep import surface small in handlers).

func RoleString(r Role) string { return string(r) }

var _ = fmt.Sprintf
