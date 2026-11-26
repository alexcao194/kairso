package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/alexcao194/kairso/pkg/httpx"
	"github.com/alexcao194/kairso/pkg/jwt"
)

type contextKey string

const (
	UserContextKey contextKey = "user_id"
	ConnContextKey contextKey = "conn_id"
)

const ConnIDHeader = "X-Conn-Id"

type AuthMiddleware struct {
	jwtSvc jwt.TokenService
	keys   APIKeyResolver
}

func NewAuthMiddleware(jwtSvc jwt.TokenService) *AuthMiddleware {
	return &AuthMiddleware{jwtSvc: jwtSvc}
}

// SetAPIKeyResolver accepts "Authorization: Bearer ksk_..." personal API keys besides JWTs.
func (m *AuthMiddleware) SetAPIKeyResolver(r APIKeyResolver) { m.keys = r }

func (m *AuthMiddleware) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Already authenticated by an API key upstream (app token middleware, MCP tool calls).
		if IsAPIKeyRequest(r.Context()) {
			next.ServeHTTP(w, r)
			return
		}
		if key := BearerAPIKey(r); key != "" {
			r, ok := authenticateAPIKey(w, r, m.keys, key)
			if ok {
				next.ServeHTTP(w, r)
			}
			return
		}
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			httpx.Error(w, http.StatusUnauthorized, "Thiếu header Authorization")
			return
		}
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			httpx.Error(w, http.StatusUnauthorized, "Định dạng Authorization không hợp lệ")
			return
		}
		tokenStr := parts[1]
		claims, err := m.jwtSvc.ValidateToken(tokenStr)
		if err != nil {
			httpx.Error(w, http.StatusUnauthorized, err.Error())
			return
		}
		if !claims.AuthorizesRequests() {
			httpx.Error(w, http.StatusUnauthorized, jwt.ErrInvalidToken.Error())
			return
		}
		// Render tokens only let the PDF renderer read the page it prints.
		if claims.TokenType == jwt.TokenTypeRender && r.Method != http.MethodGet && r.Method != http.MethodHead {
			httpx.Error(w, http.StatusForbidden, "Token chỉ được phép đọc dữ liệu")
			return
		}
		ctx := context.WithValue(r.Context(), UserContextKey, claims.UserID)
		if connID := r.Header.Get(ConnIDHeader); connID != "" {
			ctx = context.WithValue(ctx, ConnContextKey, connID)
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
func (m *AuthMiddleware) Handle(next http.HandlerFunc) http.HandlerFunc {
	return m.Authenticate(next).ServeHTTP
}
func GetUserIDFromContext(ctx context.Context) (string, bool) {
	userID, ok := ctx.Value(UserContextKey).(string)
	return userID, ok
}

func GetConnIDFromContext(ctx context.Context) string {
	connID, _ := ctx.Value(ConnContextKey).(string)
	return connID
}
