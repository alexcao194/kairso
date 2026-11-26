package apikey

import (
	"context"
	"errors"
	"time"

	"github.com/alexcao194/kairso/internal/database"
	"github.com/alexcao194/kairso/pkg/pgutil"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("không tìm thấy API key")

type Repository interface {
	Create(ctx context.Context, userID, name, prefix, hash string, expiresAt *time.Time) (APIKeyResponse, error)
	ListByUser(ctx context.Context, userID string) ([]APIKeyResponse, error)
	GetActiveByHash(ctx context.Context, hash string) (APIKey, error)
	Touch(ctx context.Context, id string) error
	Revoke(ctx context.Context, id, userID string) error
}

type repository struct {
	queries *database.Queries
}

func NewRepository(pool *pgxpool.Pool) Repository {
	return &repository{queries: database.New(pool)}
}

func (r *repository) Create(ctx context.Context, userID, name, prefix, hash string, expiresAt *time.Time) (APIKeyResponse, error) {
	uid, err := pgutil.ParseUUID(userID)
	if err != nil {
		return APIKeyResponse{}, err
	}
	var exp pgtype.Timestamptz
	if expiresAt != nil {
		exp = pgtype.Timestamptz{Time: *expiresAt, Valid: true}
	}
	row, err := r.queries.CreateApiKey(ctx, database.CreateApiKeyParams{UserID: uid, Name: name, Prefix: prefix, KeyHash: hash, ExpiresAt: exp})
	if err != nil {
		return APIKeyResponse{}, err
	}
	return toResponse(row), nil
}

func (r *repository) ListByUser(ctx context.Context, userID string) ([]APIKeyResponse, error) {
	uid, err := pgutil.ParseUUID(userID)
	if err != nil {
		return nil, err
	}
	rows, err := r.queries.ListApiKeysByUser(ctx, uid)
	if err != nil {
		return nil, err
	}
	out := make([]APIKeyResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, toResponse(row))
	}
	return out, nil
}

func (r *repository) GetActiveByHash(ctx context.Context, hash string) (APIKey, error) {
	row, err := r.queries.GetActiveApiKeyByHash(ctx, hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return APIKey{}, ErrNotFound
	}
	if err != nil {
		return APIKey{}, err
	}
	return toAPIKey(row), nil
}

func (r *repository) Touch(ctx context.Context, id string) error {
	uid, err := pgutil.ParseUUID(id)
	if err != nil {
		return err
	}
	return r.queries.TouchApiKey(ctx, uid)
}

func (r *repository) Revoke(ctx context.Context, id, userID string) error {
	kid, err := pgutil.ParseUUID(id)
	if err != nil {
		return ErrNotFound
	}
	uid, err := pgutil.ParseUUID(userID)
	if err != nil {
		return err
	}
	n, err := r.queries.RevokeApiKey(ctx, database.RevokeApiKeyParams{ID: kid, UserID: uid})
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
