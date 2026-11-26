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

// APIKeyResponse describes a key without its secret.
type APIKeyResponse struct {
	ID         string     `json:"id"`
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

func toResponse(k database.ApiKey) APIKeyResponse {
	return APIKeyResponse{
		ID:         pgutil.UUIDToString(k.ID),
		Name:       k.Name,
		Prefix:     k.Prefix,
		LastUsedAt: optTime(k.LastUsedAt),
		ExpiresAt:  optTime(k.ExpiresAt),
		CreatedAt:  k.CreatedAt.Time,
	}
}

func toAPIKey(k database.ApiKey) APIKey {
	return APIKey{ID: pgutil.UUIDToString(k.ID), UserID: pgutil.UUIDToString(k.UserID), LastUsedAt: optTime(k.LastUsedAt)}
}
