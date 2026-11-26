package oauth

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

var errNotFound = errors.New("not found")

// Secrets bundles a freshly issued access/refresh pair, already hashed for storage.
type Secrets struct {
	Prefix           string
	AccessHash       string
	AccessExpiresAt  time.Time
	RefreshHash      string
	RefreshExpiresAt time.Time
}

type Repository interface {
	CreateClient(ctx context.Context, name string, redirectURIs []string) (Client, error)
	GetClient(ctx context.Context, id string) (Client, error)
	CreateCode(ctx context.Context, hash string, c Code) error
	ConsumeCode(ctx context.Context, hash string) (Code, error)
	DeleteExpiredCodes(ctx context.Context) error
	CreateToken(ctx context.Context, userID, clientID, name string, s Secrets) error
	GetTokenByRefresh(ctx context.Context, refreshHash string) (Token, error)
	RotateToken(ctx context.Context, id, oldRefreshHash string, s Secrets) error
}

type repository struct {
	queries *database.Queries
}

func NewRepository(pool *pgxpool.Pool) Repository {
	return &repository{queries: database.New(pool)}
}

func ts(t time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: t, Valid: true} }
func text(s string) pgtype.Text         { return pgtype.Text{String: s, Valid: true} }

func (r *repository) CreateClient(ctx context.Context, name string, redirectURIs []string) (Client, error) {
	c, err := r.queries.CreateOAuthClient(ctx, database.CreateOAuthClientParams{Name: name, RedirectUris: redirectURIs})
	if err != nil {
		return Client{}, err
	}
	return toClient(c), nil
}

func (r *repository) GetClient(ctx context.Context, id string) (Client, error) {
	uid, err := pgutil.ParseUUID(id)
	if err != nil {
		return Client{}, errNotFound
	}
	c, err := r.queries.GetOAuthClient(ctx, uid)
	if errors.Is(err, pgx.ErrNoRows) {
		return Client{}, errNotFound
	}
	if err != nil {
		return Client{}, err
	}
	return toClient(c), nil
}

func (r *repository) CreateCode(ctx context.Context, hash string, c Code) error {
	cid, err := pgutil.ParseUUID(c.ClientID)
	if err != nil {
		return err
	}
	uid, err := pgutil.ParseUUID(c.UserID)
	if err != nil {
		return err
	}
	return r.queries.CreateOAuthCode(ctx, database.CreateOAuthCodeParams{
		CodeHash: hash, ClientID: cid, UserID: uid, RedirectUri: c.RedirectURI,
		CodeChallenge: c.CodeChallenge, Scope: c.Scope, ExpiresAt: ts(c.ExpiresAt),
	})
}

func (r *repository) ConsumeCode(ctx context.Context, hash string) (Code, error) {
	c, err := r.queries.ConsumeOAuthCode(ctx, hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return Code{}, errNotFound
	}
	if err != nil {
		return Code{}, err
	}
	return toCode(c), nil
}

func (r *repository) DeleteExpiredCodes(ctx context.Context) error {
	return r.queries.DeleteExpiredOAuthCodes(ctx)
}

func (r *repository) CreateToken(ctx context.Context, userID, clientID, name string, s Secrets) error {
	uid, err := pgutil.ParseUUID(userID)
	if err != nil {
		return err
	}
	cid, err := pgutil.ParseUUID(clientID)
	if err != nil {
		return err
	}
	_, err = r.queries.CreateOAuthToken(ctx, database.CreateOAuthTokenParams{
		UserID: uid, Name: name, Prefix: s.Prefix, KeyHash: s.AccessHash, ExpiresAt: ts(s.AccessExpiresAt),
		ClientID: cid, RefreshHash: text(s.RefreshHash), RefreshExpiresAt: ts(s.RefreshExpiresAt),
	})
	return err
}

func (r *repository) GetTokenByRefresh(ctx context.Context, refreshHash string) (Token, error) {
	k, err := r.queries.GetOAuthTokenByRefresh(ctx, text(refreshHash))
	if errors.Is(err, pgx.ErrNoRows) {
		return Token{}, errNotFound
	}
	if err != nil {
		return Token{}, err
	}
	return toToken(k), nil
}

func (r *repository) RotateToken(ctx context.Context, id, oldRefreshHash string, s Secrets) error {
	uid, err := pgutil.ParseUUID(id)
	if err != nil {
		return err
	}
	n, err := r.queries.RotateOAuthToken(ctx, database.RotateOAuthTokenParams{
		ID: uid, KeyHash: s.AccessHash, Prefix: s.Prefix, ExpiresAt: ts(s.AccessExpiresAt),
		RefreshHash: text(s.RefreshHash), RefreshExpiresAt: ts(s.RefreshExpiresAt), OldRefreshHash: text(oldRefreshHash),
	})
	if err != nil {
		return err
	}
	if n == 0 {
		return errNotFound
	}
	return nil
}
