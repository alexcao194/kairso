package apikey

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/alexcao194/kairso/internal/api/rest/middleware"
)

const (
	maxKeysPerUser = 20
	// touchEvery limits how often a busy key writes its last-used time.
	touchEvery = time.Minute
)

type Service interface {
	Create(ctx context.Context, userID string, req CreateAPIKeyRequest) (*CreatedAPIKeyResponse, error)
	List(ctx context.Context, userID string) ([]APIKeyResponse, error)
	Revoke(ctx context.Context, userID, id string) error
	// ResolveAPIKey implements middleware.APIKeyResolver.
	ResolveAPIKey(ctx context.Context, key string) (string, error)
}

type service struct {
	repo Repository
	now  func() time.Time
}

func NewService(repo Repository) Service {
	return &service{repo: repo, now: time.Now}
}

// HashKey is how keys and other bearer secrets are stored: hex SHA-256.
func HashKey(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}

// NewSecret returns prefix followed by 32 random bytes in URL-safe base64.
func NewSecret(prefix string) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return prefix + base64.RawURLEncoding.EncodeToString(b), nil
}

// DisplayPrefix is the part of a key shown in lists.
func DisplayPrefix(key string) string { return key[:12] }

func hashKey(key string) string { return HashKey(key) }

func newKey() (string, error) { return NewSecret(middleware.APIKeyPrefix) }

func (s *service) Create(ctx context.Context, userID string, req CreateAPIKeyRequest) (*CreatedAPIKeyResponse, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" || utf8.RuneCountInString(name) > 100 {
		return nil, errors.New("tên API key phải có từ 1 đến 100 ký tự")
	}
	if req.ExpiresInDays < 0 || req.ExpiresInDays > 3650 {
		return nil, errors.New("thời hạn API key không hợp lệ")
	}
	existing, err := s.repo.ListByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	personal := 0
	for _, k := range existing {
		if k.Kind == KindPersonal {
			personal++
		}
	}
	if personal >= maxKeysPerUser {
		return nil, errors.New("đã đạt số API key tối đa, hãy thu hồi bớt key cũ")
	}
	key, err := newKey()
	if err != nil {
		return nil, err
	}
	var expiresAt *time.Time
	if req.ExpiresInDays > 0 {
		t := s.now().Add(time.Duration(req.ExpiresInDays) * 24 * time.Hour)
		expiresAt = &t
	}
	resp, err := s.repo.Create(ctx, userID, name, DisplayPrefix(key), hashKey(key), expiresAt)
	if err != nil {
		return nil, err
	}
	return &CreatedAPIKeyResponse{APIKeyResponse: resp, Key: key}, nil
}

func (s *service) List(ctx context.Context, userID string) ([]APIKeyResponse, error) {
	return s.repo.ListByUser(ctx, userID)
}

func (s *service) Revoke(ctx context.Context, userID, id string) error {
	return s.repo.Revoke(ctx, id, userID)
}

func (s *service) ResolveAPIKey(ctx context.Context, key string) (string, error) {
	if !strings.HasPrefix(key, middleware.APIKeyPrefix) || len(key) > 128 {
		return "", ErrNotFound
	}
	k, err := s.repo.GetActiveByHash(ctx, hashKey(key))
	if err != nil {
		return "", err
	}
	if k.LastUsedAt == nil || s.now().Sub(*k.LastUsedAt) > touchEvery {
		_ = s.repo.Touch(ctx, k.ID)
	}
	return k.UserID, nil
}
