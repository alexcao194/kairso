package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alexcao194/kairso/internal/config"
	"github.com/alexcao194/kairso/pkg/jwt"
)

func TestOnlyAccessAndRenderTokensAuthenticate(t *testing.T) {
	tokens := jwt.NewJWTService("secret")
	access, refresh, err := tokens.GenerateToken(&config.AppConfig{AccessTokenDuration: time.Minute, RefreshTokenDuration: time.Hour}, "user-1")
	if err != nil {
		t.Fatal(err)
	}
	h := NewAuthMiddleware(tokens).Handle(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) })
	for name, tc := range map[string]struct {
		token string
		want  int
	}{
		"access":  {access, http.StatusNoContent},
		"refresh": {refresh, http.StatusUnauthorized},
	} {
		req := httptest.NewRequest(http.MethodGet, "/v1/x", nil)
		req.Header.Set("Authorization", "Bearer "+tc.token)
		rec := httptest.NewRecorder()
		h(rec, req)
		if rec.Code != tc.want {
			t.Errorf("%s token: status %d, want %d", name, rec.Code, tc.want)
		}
	}
}
