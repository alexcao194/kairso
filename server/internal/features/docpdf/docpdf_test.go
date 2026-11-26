package docpdf

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alexcao194/kairso/pkg/jwt"
)

func TestPrintQueryKeepsOnlyPrintOptions(t *testing.T) {
	r := httptest.NewRequest("GET", "/x?template=admin&sign=wet&place=+H%C3%A0+N%E1%BB%99i+&signer=GI%C3%81M+%C4%90%E1%BB%90C&merge=1&fit=0&ws=evil&token=x&format=admin", nil)
	got := PrintQuery(r).Encode()
	want := "fit=0&format=admin&merge=1&place=H%C3%A0+N%E1%BB%99i&sign=wet&signer=GI%C3%81M+%C4%90%E1%BB%90C"
	if got != want {
		t.Fatalf("got %s\nwant %s", got, want)
	}
	if q := PrintQuery(httptest.NewRequest("GET", "/x?template=modern&sign=digital&merge=2&fit=yes", nil)); len(q) != 0 {
		t.Fatalf("defaults and bad values must be dropped, got %v", q)
	}
	long := strings.Repeat("a", 121)
	if q := PrintQuery(httptest.NewRequest("GET", "/x?place="+long, nil)); q.Has("place") {
		t.Fatal("over-long place must be dropped")
	}
}

type fakeRenderer struct{ path, token string }

func (f *fakeRenderer) Render(_ context.Context, path, token string) ([]byte, error) {
	f.path, f.token = path, token
	return []byte("%PDF-1.4"), nil
}

func TestExporterSignsInWithRenderToken(t *testing.T) {
	tokens := jwt.NewJWTService("secret")
	fr := &fakeRenderer{}
	if _, err := New(fr, tokens).Render(context.Background(), "user-1", "/print/quotes/q1?ws=w1"); err != nil {
		t.Fatal(err)
	}
	claims, err := tokens.ValidateToken(fr.token)
	if err != nil {
		t.Fatal(err)
	}
	if claims.UserID != "user-1" || claims.TokenType != jwt.TokenTypeRender {
		t.Fatalf("claims = %+v", claims)
	}
	if ttl := time.Until(claims.ExpiresAt.Time); ttl <= 0 || ttl > tokenTTL {
		t.Fatalf("token lives %s", ttl)
	}
	if fr.path != "/print/quotes/q1?ws=w1" {
		t.Fatalf("path = %s", fr.path)
	}
}

func TestNilExporterIsUnavailable(t *testing.T) {
	var e *Exporter
	_, err := e.Render(context.Background(), "u", "/print/x")
	if !errors.Is(err, ErrUnavailable) || Status(err) != http.StatusServiceUnavailable {
		t.Fatalf("err = %v", err)
	}
}
