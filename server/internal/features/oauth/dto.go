package oauth

import (
	"time"

	"github.com/alexcao194/kairso/internal/database"
	"github.com/alexcao194/kairso/pkg/pgutil"
)

// RegisterRequest is RFC 7591 dynamic client registration metadata (the fields we use).
type RegisterRequest struct {
	ClientName              string   `json:"client_name"`
	RedirectURIs            []string `json:"redirect_uris"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method,omitempty"`
}

// RegisterResponse is the registered client. Every client is public: it proves possession
// with PKCE instead of a secret.
type RegisterResponse struct {
	ClientID                string   `json:"client_id"`
	ClientName              string   `json:"client_name"`
	RedirectURIs            []string `json:"redirect_uris"`
	GrantTypes              []string `json:"grant_types"`
	ResponseTypes           []string `json:"response_types"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"`
	ClientIDIssuedAt        int64    `json:"client_id_issued_at"`
}

// ClientResponse is what the consent screen shows about an app asking for access.
type ClientResponse struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	RedirectHost string `json:"redirectHost"`
}

// AuthorizeRequest is the signed-in user's approval of an authorization request.
type AuthorizeRequest struct {
	ClientID            string `json:"clientId"`
	RedirectURI         string `json:"redirectUri"`
	CodeChallenge       string `json:"codeChallenge"`
	CodeChallengeMethod string `json:"codeChallengeMethod"`
	State               string `json:"state,omitempty"`
	Scope               string `json:"scope,omitempty"`
}

// AuthorizeResponse is where the browser goes next (the client's redirect URI with the code).
type AuthorizeResponse struct {
	RedirectURL string `json:"redirectUrl"`
}

// TokenResponse is the RFC 6749 token response.
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope,omitempty"`
}

// Error is an RFC 6749 error response.
type Error struct {
	Code        string `json:"error"`
	Description string `json:"error_description,omitempty"`
	status      int
}

func (e *Error) Error() string { return e.Code + ": " + e.Description }

// Client is a stored client.
type Client struct {
	ID           string
	Name         string
	RedirectURIs []string
	CreatedAt    time.Time
}

// Code is a stored authorization code.
type Code struct {
	ClientID      string
	UserID        string
	RedirectURI   string
	CodeChallenge string
	Scope         string
	ExpiresAt     time.Time
}

// Token is a stored OAuth grant (an api_keys row).
type Token struct {
	ID       string
	ClientID string
	UserID   string
	Name     string
}

func toClient(c database.OauthClient) Client {
	return Client{ID: pgutil.UUIDToString(c.ID), Name: c.Name, RedirectURIs: c.RedirectUris, CreatedAt: c.CreatedAt.Time}
}

func toCode(c database.OauthCode) Code {
	return Code{
		ClientID:      pgutil.UUIDToString(c.ClientID),
		UserID:        pgutil.UUIDToString(c.UserID),
		RedirectURI:   c.RedirectUri,
		CodeChallenge: c.CodeChallenge,
		Scope:         c.Scope,
		ExpiresAt:     c.ExpiresAt.Time,
	}
}

func toToken(k database.ApiKey) Token {
	return Token{ID: pgutil.UUIDToString(k.ID), ClientID: pgutil.UUIDToString(k.ClientID), UserID: pgutil.UUIDToString(k.UserID), Name: k.Name}
}
