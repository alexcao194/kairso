package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/alexcao194/kairso/pkg/httpx"
)

// APIKeyPrefix starts every personal API key, so a bearer token can be told apart from a JWT.
const APIKeyPrefix = "ksk_"

const apiKeyContextKey contextKey = "api_key"

// APIKeyResolver maps a plain API key to the ID of the user who owns it.
type APIKeyResolver interface {
	ResolveAPIKey(ctx context.Context, key string) (userID string, err error)
}

// BearerAPIKey returns the API key in the Authorization header, or "" when the header holds
// something else (a JWT, nothing).
func BearerAPIKey(r *http.Request) string {
	h := r.Header.Get("Authorization")
	token, ok := strings.CutPrefix(h, "Bearer ")
	if !ok || !strings.HasPrefix(token, APIKeyPrefix) {
		return ""
	}
	return token
}

// IsAPIKeyRequest reports whether the caller authenticated with a personal API key instead of
// a signed-in session.
func IsAPIKeyRequest(ctx context.Context) bool {
	v, _ := ctx.Value(apiKeyContextKey).(bool)
	return v
}

// WithAPIKeyUser marks the context as authenticated by an API key belonging to userID.
func WithAPIKeyUser(ctx context.Context, userID string) context.Context {
	ctx = context.WithValue(ctx, UserContextKey, userID)
	return context.WithValue(ctx, apiKeyContextKey, true)
}

// authenticateAPIKey resolves an API key request. It returns false (after writing the error)
// when the key is not valid.
func authenticateAPIKey(w http.ResponseWriter, r *http.Request, resolver APIKeyResolver, key string) (*http.Request, bool) {
	if IsAPIKeyRequest(r.Context()) {
		return r, true
	}
	if resolver == nil {
		httpx.Error(w, http.StatusUnauthorized, "API key không được hỗ trợ")
		return r, false
	}
	userID, err := resolver.ResolveAPIKey(r.Context(), key)
	if err != nil || userID == "" {
		httpx.Error(w, http.StatusUnauthorized, "API key không hợp lệ hoặc đã bị thu hồi")
		return r, false
	}
	return r.WithContext(WithAPIKeyUser(r.Context(), userID)), true
}

// RequireSession rejects API key callers, for endpoints a key must not reach (managing keys).
func RequireSession(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if IsAPIKeyRequest(r.Context()) {
			httpx.Error(w, http.StatusForbidden, "Thao tác này cần đăng nhập, không dùng được API key")
			return
		}
		next(w, r)
	}
}
