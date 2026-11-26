package jwt

import (
	"testing"
	"time"

	"github.com/alexcao194/kairso/internal/config"
)

func TestAuthorizesRequests(t *testing.T) {
	svc := NewJWTService("secret")
	access, refresh, err := svc.GenerateToken(&config.AppConfig{AccessTokenDuration: time.Minute, RefreshTokenDuration: time.Hour}, "u")
	if err != nil {
		t.Fatal(err)
	}
	render, err := svc.GenerateRenderToken("u", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	for tok, want := range map[string]bool{access: true, render: true, refresh: false} {
		c, err := svc.ValidateToken(tok)
		if err != nil {
			t.Fatal(err)
		}
		if got := c.AuthorizesRequests(); got != want {
			t.Errorf("%s token: AuthorizesRequests = %v, want %v", c.TokenType, got, want)
		}
	}
}
