package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alexcao194/kairso/internal/config"
	"github.com/alexcao194/kairso/pkg/jwt"
)

func TestRenderTokenIsReadOnly(t *testing.T) {
	tokens := jwt.NewJWTService("secret")
	token, err := tokens.GenerateRenderToken("user-1", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	h := NewAuthMiddleware(tokens).Handle(func(w http.ResponseWriter, r *http.Request) {
		if id, _ := GetUserIDFromContext(r.Context()); id != "user-1" {
			t.Errorf("user = %q", id)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	for method, want := range map[string]int{
		http.MethodGet: http.StatusNoContent, http.MethodHead: http.StatusNoContent,
		http.MethodPost: http.StatusForbidden, http.MethodPatch: http.StatusForbidden, http.MethodDelete: http.StatusForbidden,
	} {
		req := httptest.NewRequest(method, "/v1/x", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		h(rec, req)
		if rec.Code != want {
			t.Errorf("%s: status %d, want %d", method, rec.Code, want)
		}
	}
}

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
