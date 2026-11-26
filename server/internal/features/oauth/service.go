// Package oauth is a small OAuth 2.1 authorization server for MCP clients (Claude Desktop and
// claude.ai connectors): dynamic client registration, authorization code with mandatory PKCE
// (S256) and rotating refresh tokens. Access tokens are API keys bound to the client, so the
// rest of the API and /mcp accept them unchanged.
package oauth

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/alexcao194/kairso/internal/api/rest/middleware"
	"github.com/alexcao194/kairso/internal/features/apikey"
)

const (
	codeTTL         = 10 * time.Minute
	accessTTL       = time.Hour
	refreshTTL      = 90 * 24 * time.Hour
	refreshPrefix   = "ksr_"
	codePrefix      = "kac_"
	maxRedirectURIs = 10
)

var (
	ErrClientNotFound = errors.New("ứng dụng không tồn tại")
	ErrBadRedirect    = errors.New("địa chỉ chuyển hướng không khớp với ứng dụng")
	ErrPKCE           = errors.New("yêu cầu cấp quyền thiếu PKCE (S256)")
)

type Service interface {
	Register(ctx context.Context, req RegisterRequest) (*RegisterResponse, *Error)
	GetClient(ctx context.Context, clientID, redirectURI string) (*ClientResponse, error)
	Authorize(ctx context.Context, userID string, req AuthorizeRequest) (*AuthorizeResponse, error)
	Token(ctx context.Context, form url.Values) (*TokenResponse, *Error)
}

type service struct {
	repo Repository
	now  func() time.Time
}

func NewService(repo Repository) Service {
	return &service{repo: repo, now: time.Now}
}

func oauthErr(status int, code, desc string) *Error {
	return &Error{Code: code, Description: desc, status: status}
}

// validRedirect accepts https URLs and http on loopback (native apps), without fragments.
func validRedirect(raw string) bool {
	if len(raw) > 2000 {
		return false
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.Fragment != "" || u.User != nil {
		return false
	}
	switch u.Scheme {
	case "https":
		return true
	case "http":
		host := u.Hostname()
		if host == "localhost" {
			return true
		}
		ip := net.ParseIP(host)
		return ip != nil && ip.IsLoopback()
	}
	return false
}

func (s *service) Register(ctx context.Context, req RegisterRequest) (*RegisterResponse, *Error) {
	if len(req.RedirectURIs) == 0 || len(req.RedirectURIs) > maxRedirectURIs {
		return nil, oauthErr(http.StatusBadRequest, "invalid_redirect_uri", "redirect_uris must list 1 to 10 URIs")
	}
	for _, u := range req.RedirectURIs {
		if !validRedirect(u) {
			return nil, oauthErr(http.StatusBadRequest, "invalid_redirect_uri", "redirect URIs must be https, or http on localhost")
		}
	}
	name := strings.TrimSpace(req.ClientName)
	if name == "" {
		name = "MCP client"
	}
	if utf8.RuneCountInString(name) > 100 {
		name = string([]rune(name)[:100])
	}
	c, err := s.repo.CreateClient(ctx, name, req.RedirectURIs)
	if err != nil {
		return nil, oauthErr(http.StatusInternalServerError, "server_error", "could not register client")
	}
	return &RegisterResponse{
		ClientID:                c.ID,
		ClientName:              c.Name,
		RedirectURIs:            c.RedirectURIs,
		GrantTypes:              []string{"authorization_code", "refresh_token"},
		ResponseTypes:           []string{"code"},
		TokenEndpointAuthMethod: "none",
		ClientIDIssuedAt:        c.CreatedAt.Unix(),
	}, nil
}

func (s *service) clientFor(ctx context.Context, clientID, redirectURI string) (Client, error) {
	c, err := s.repo.GetClient(ctx, clientID)
	if errors.Is(err, errNotFound) {
		return Client{}, ErrClientNotFound
	}
	if err != nil {
		return Client{}, err
	}
	for _, u := range c.RedirectURIs {
		if u == redirectURI {
			return c, nil
		}
	}
	return Client{}, ErrBadRedirect
}

func (s *service) GetClient(ctx context.Context, clientID, redirectURI string) (*ClientResponse, error) {
	c, err := s.clientFor(ctx, clientID, redirectURI)
	if err != nil {
		return nil, err
	}
	u, _ := url.Parse(redirectURI)
	return &ClientResponse{ID: c.ID, Name: c.Name, RedirectHost: u.Host}, nil
}

func (s *service) Authorize(ctx context.Context, userID string, req AuthorizeRequest) (*AuthorizeResponse, error) {
	c, err := s.clientFor(ctx, req.ClientID, req.RedirectURI)
	if err != nil {
		return nil, err
	}
	if req.CodeChallengeMethod != "S256" || len(req.CodeChallenge) < 43 || len(req.CodeChallenge) > 128 {
		return nil, ErrPKCE
	}
	_ = s.repo.DeleteExpiredCodes(ctx)
	code, err := apikey.NewSecret(codePrefix)
	if err != nil {
		return nil, err
	}
	err = s.repo.CreateCode(ctx, apikey.HashKey(code), Code{
		ClientID: c.ID, UserID: userID, RedirectURI: req.RedirectURI, CodeChallenge: req.CodeChallenge,
		Scope: req.Scope, ExpiresAt: s.now().Add(codeTTL),
	})
	if err != nil {
		return nil, err
	}
	u, _ := url.Parse(req.RedirectURI)
	q := u.Query()
	q.Set("code", code)
	if req.State != "" {
		q.Set("state", req.State)
	}
	u.RawQuery = q.Encode()
	return &AuthorizeResponse{RedirectURL: u.String()}, nil
}

func pkceMatches(verifier, challenge string) bool {
	if len(verifier) < 43 || len(verifier) > 128 {
		return false
	}
	sum := sha256.Sum256([]byte(verifier))
	got := base64.RawURLEncoding.EncodeToString(sum[:])
	return subtle.ConstantTimeCompare([]byte(got), []byte(challenge)) == 1
}

// issue creates a new access/refresh pair. It returns the response and the hashed secrets.
func (s *service) issue(scope string) (*TokenResponse, Secrets, error) {
	access, err := apikey.NewSecret(middleware.APIKeyPrefix)
	if err != nil {
		return nil, Secrets{}, err
	}
	refresh, err := apikey.NewSecret(refreshPrefix)
	if err != nil {
		return nil, Secrets{}, err
	}
	now := s.now()
	sec := Secrets{
		Prefix:           apikey.DisplayPrefix(access),
		AccessHash:       apikey.HashKey(access),
		AccessExpiresAt:  now.Add(accessTTL),
		RefreshHash:      apikey.HashKey(refresh),
		RefreshExpiresAt: now.Add(refreshTTL),
	}
	return &TokenResponse{
		AccessToken:  access,
		TokenType:    "Bearer",
		ExpiresIn:    int64(accessTTL.Seconds()),
		RefreshToken: refresh,
		Scope:        scope,
	}, sec, nil
}

func (s *service) Token(ctx context.Context, form url.Values) (*TokenResponse, *Error) {
	switch form.Get("grant_type") {
	case "authorization_code":
		return s.exchangeCode(ctx, form)
	case "refresh_token":
		return s.refresh(ctx, form)
	case "":
		return nil, oauthErr(http.StatusBadRequest, "invalid_request", "grant_type is required")
	}
	return nil, oauthErr(http.StatusBadRequest, "unsupported_grant_type", "")
}

func (s *service) exchangeCode(ctx context.Context, form url.Values) (*TokenResponse, *Error) {
	raw := form.Get("code")
	if raw == "" || form.Get("code_verifier") == "" {
		return nil, oauthErr(http.StatusBadRequest, "invalid_request", "code and code_verifier are required")
	}
	invalid := oauthErr(http.StatusBadRequest, "invalid_grant", "authorization code is invalid or expired")
	// Codes are single use: consuming deletes it whether or not the checks below pass.
	code, err := s.repo.ConsumeCode(ctx, apikey.HashKey(raw))
	if err != nil {
		return nil, invalid
	}
	if s.now().After(code.ExpiresAt) ||
		code.ClientID != form.Get("client_id") ||
		code.RedirectURI != form.Get("redirect_uri") ||
		!pkceMatches(form.Get("code_verifier"), code.CodeChallenge) {
		return nil, invalid
	}
	client, err := s.repo.GetClient(ctx, code.ClientID)
	if err != nil {
		return nil, invalid
	}
	resp, sec, err := s.issue(code.Scope)
	if err != nil {
		return nil, oauthErr(http.StatusInternalServerError, "server_error", "")
	}
	if err := s.repo.CreateToken(ctx, code.UserID, client.ID, client.Name, sec); err != nil {
		return nil, oauthErr(http.StatusInternalServerError, "server_error", "")
	}
	return resp, nil
}

func (s *service) refresh(ctx context.Context, form url.Values) (*TokenResponse, *Error) {
	raw := form.Get("refresh_token")
	if raw == "" {
		return nil, oauthErr(http.StatusBadRequest, "invalid_request", "refresh_token is required")
	}
	invalid := oauthErr(http.StatusBadRequest, "invalid_grant", "refresh token is invalid, expired or revoked")
	oldHash := apikey.HashKey(raw)
	tok, err := s.repo.GetTokenByRefresh(ctx, oldHash)
	if err != nil {
		return nil, invalid
	}
	if cid := form.Get("client_id"); cid != "" && cid != tok.ClientID {
		return nil, invalid
	}
	resp, sec, err := s.issue(form.Get("scope"))
	if err != nil {
		return nil, oauthErr(http.StatusInternalServerError, "server_error", "")
	}
	// The rotation only applies while the presented refresh token is still current, so a
	// replayed (already rotated) token fails here.
	if err := s.repo.RotateToken(ctx, tok.ID, oldHash, sec); err != nil {
		return nil, invalid
	}
	return resp, nil
}
