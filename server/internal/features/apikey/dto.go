package apikey

import (
	"time"

	"github.com/alexcao194/kairso/internal/database"
	"github.com/alexcao194/kairso/pkg/pgutil"
	"github.com/jackc/pgx/v5/pgtype"
)

// CreateAPIKeyRequest creates a personal API key for the caller.
type CreateAPIKeyRequest struct {
	Name string `json:"name"`
	// ExpiresInDays limits the key's lifetime; omit or 0 for a key that never expires.
	ExpiresInDays int `json:"expiresInDays,omitempty"`
}

// APIKeyResponse describes a key without its secret. Kind "oauth" is an app connected through
// OAuth (Claude connectors); its token rotates by itself, so it has no fixed expiry.
type APIKeyResponse struct {
	ID         string     `json:"id"`
	Kind       string     `json:"kind" enums:"personal,oauth"`
	Name       string     `json:"name"`
	Prefix     string     `json:"prefix"`
	LastUsedAt *time.Time `json:"lastUsedAt,omitempty" swaggertype:"string" format:"date-time"`
	ExpiresAt  *time.Time `json:"expiresAt,omitempty" swaggertype:"string" format:"date-time"`
	CreatedAt  time.Time  `json:"createdAt" swaggertype:"string" format:"date-time"`
}

// CreatedAPIKeyResponse carries the plain key, returned only once at creation.
type CreatedAPIKeyResponse struct {
	APIKeyResponse
	Key string `json:"key"`
}

// APIKey is a stored key as the service sees it.
type APIKey struct {
	ID         string
	UserID     string
	LastUsedAt *time.Time
}

func optTime(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	v := t.Time
	return &v
}

const (
	KindPersonal = "personal"
	KindOAuth    = "oauth"
)

func toResponse(k database.ApiKey) APIKeyResponse {
	r := APIKeyResponse{
		ID:         pgutil.UUIDToString(k.ID),
		Kind:       KindPersonal,
		Name:       k.Name,
		Prefix:     k.Prefix,
		LastUsedAt: optTime(k.LastUsedAt),
		ExpiresAt:  optTime(k.ExpiresAt),
		CreatedAt:  k.CreatedAt.Time,
	}
	if k.ClientID.Valid {
		r.Kind = KindOAuth
		r.Prefix = ""
		r.ExpiresAt = optTime(k.RefreshExpiresAt)
	}
	return r
}

func toAPIKey(k database.ApiKey) APIKey {
	return APIKey{ID: pgutil.UUIDToString(k.ID), UserID: pgutil.UUIDToString(k.UserID), LastUsedAt: optTime(k.LastUsedAt)}
}
