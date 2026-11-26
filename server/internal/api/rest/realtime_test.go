package rest

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alexcao194/kairso/internal/config"
	"github.com/alexcao194/kairso/pkg/jwt"
)

// A refresh token must not open the realtime socket; it is rejected before any lookup.
func TestRealtimeRejectsRefreshToken(t *testing.T) {
	tokens := jwt.NewJWTService("secret")
	_, refresh, err := tokens.GenerateToken(&config.AppConfig{AccessTokenDuration: time.Minute, RefreshTokenDuration: time.Hour}, "00000000-0000-0000-0000-000000000001")
	if err != nil {
		t.Fatal(err)
	}
	h := NewRealtimeHandler(nil, tokens, nil)
	rec := httptest.NewRecorder()
	h.Serve(rec, httptest.NewRequest(http.MethodGet, "/ws?token="+refresh, nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d, want 401", rec.Code)
	}
}
