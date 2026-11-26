package oauth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"net/url"
	"strings"
	"testing"
	"time"
)

// memRepo is an in-memory Repository.
type memRepo struct {
	clients map[string]Client
	codes   map[string]Code
	tokens  map[string]*struct {
		tok     Token
		refresh string
		access  string
	}
}

func newMem() *memRepo {
	return &memRepo{clients: map[string]Client{}, codes: map[string]Code{}, tokens: map[string]*struct {
		tok     Token
		refresh string
		access  string
	}{}}
}

func (m *memRepo) CreateClient(_ context.Context, name string, uris []string) (Client, error) {
	c := Client{ID: "11111111-1111-1111-1111-11111111111" + string(rune('0'+len(m.clients))), Name: name, RedirectURIs: uris, CreatedAt: time.Now()}
	m.clients[c.ID] = c
	return c, nil
}
func (m *memRepo) GetClient(_ context.Context, id string) (Client, error) {
	c, ok := m.clients[id]
	if !ok {
		return Client{}, errNotFound
	}
	return c, nil
}
func (m *memRepo) CreateCode(_ context.Context, h string, c Code) error { m.codes[h] = c; return nil }
func (m *memRepo) ConsumeCode(_ context.Context, h string) (Code, error) {
	c, ok := m.codes[h]
	if !ok {
		return Code{}, errNotFound
	}
	delete(m.codes, h)
	return c, nil
}
func (m *memRepo) DeleteExpiredCodes(context.Context) error { return nil }
func (m *memRepo) CreateToken(_ context.Context, userID, clientID, name string, s Secrets) error {
	id := "tok" + string(rune('0'+len(m.tokens)))
	m.tokens[id] = &struct {
		tok     Token
		refresh string
		access  string
	}{Token{ID: id, ClientID: clientID, UserID: userID, Name: name}, s.RefreshHash, s.AccessHash}
	return nil
}
func (m *memRepo) GetTokenByRefresh(_ context.Context, h string) (Token, error) {
	for _, t := range m.tokens {
		if t.refresh == h {
			return t.tok, nil
		}
	}
	return Token{}, errNotFound
}
func (m *memRepo) RotateToken(_ context.Context, id, old string, s Secrets) error {
	t := m.tokens[id]
	if t == nil || t.refresh != old {
		return errNotFound
	}
	t.refresh, t.access = s.RefreshHash, s.AccessHash
	return nil
}

func challenge(v string) string {
	sum := sha256.Sum256([]byte(v))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

const verifier = "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXkdBjftJeZ4CVP"

func TestFullFlow(t *testing.T) {
	ctx := context.Background()
	repo := newMem()
	svc := NewService(repo)

	if _, e := svc.Register(ctx, RegisterRequest{ClientName: "Evil", RedirectURIs: []string{"http://evil.example/cb"}}); e == nil {
		t.Fatal("plain http non-loopback redirect must be rejected")
	}
	reg, e := svc.Register(ctx, RegisterRequest{ClientName: "Claude", RedirectURIs: []string{"https://claude.ai/api/mcp/auth_callback"}})
	if e != nil {
		t.Fatal(e)
	}
	redirect := reg.RedirectURIs[0]

	if _, err := svc.GetClient(ctx, reg.ClientID, "https://claude.ai/other"); err != ErrBadRedirect {
		t.Fatalf("unregistered redirect: %v", err)
	}
	if _, err := svc.Authorize(ctx, "user-1", AuthorizeRequest{ClientID: reg.ClientID, RedirectURI: redirect, CodeChallengeMethod: "plain", CodeChallenge: verifier}); err != ErrPKCE {
		t.Fatalf("plain PKCE must be rejected: %v", err)
	}
	auth, err := svc.Authorize(ctx, "user-1", AuthorizeRequest{ClientID: reg.ClientID, RedirectURI: redirect, CodeChallengeMethod: "S256", CodeChallenge: challenge(verifier), State: "xyz"})
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(auth.RedirectURL)
	code := u.Query().Get("code")
	if !strings.HasPrefix(auth.RedirectURL, redirect+"?") || code == "" || u.Query().Get("state") != "xyz" {
		t.Fatalf("redirect: %s", auth.RedirectURL)
	}

	form := url.Values{"grant_type": {"authorization_code"}, "code": {code}, "client_id": {reg.ClientID}, "redirect_uri": {redirect}, "code_verifier": {verifier + "x"}}
	if _, e := svc.Token(ctx, form); e == nil || e.Code != "invalid_grant" {
		t.Fatalf("wrong verifier must fail: %v", e)
	}
	// The failed attempt burned the code; get a new one.
	auth, _ = svc.Authorize(ctx, "user-1", AuthorizeRequest{ClientID: reg.ClientID, RedirectURI: redirect, CodeChallengeMethod: "S256", CodeChallenge: challenge(verifier)})
	u, _ = url.Parse(auth.RedirectURL)
	form.Set("code", u.Query().Get("code"))
	form.Set("code_verifier", verifier)
	tok, e := svc.Token(ctx, form)
	if e != nil {
		t.Fatal(e)
	}
	if !strings.HasPrefix(tok.AccessToken, "ksk_") || !strings.HasPrefix(tok.RefreshToken, "ksr_") || tok.ExpiresIn != 3600 || tok.TokenType != "Bearer" {
		t.Fatalf("token: %+v", tok)
	}
	if _, e := svc.Token(ctx, form); e == nil {
		t.Fatal("code reuse must fail")
	}

	ref := url.Values{"grant_type": {"refresh_token"}, "refresh_token": {tok.RefreshToken}, "client_id": {reg.ClientID}}
	tok2, e := svc.Token(ctx, ref)
	if e != nil || tok2.RefreshToken == tok.RefreshToken {
		t.Fatalf("refresh: %+v %v", tok2, e)
	}
	if _, e := svc.Token(ctx, ref); e == nil {
		t.Fatal("rotated refresh token must not work twice")
	}
	if _, e := svc.Token(ctx, url.Values{"grant_type": {"password"}}); e == nil || e.Code != "unsupported_grant_type" {
		t.Fatalf("unsupported grant: %v", e)
	}
}
