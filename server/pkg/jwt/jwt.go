package jwt

import (
	"errors"
	"fmt"
	"time"

	"github.com/alexcao194/kairso/internal/config"

	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrInvalidToken = errors.New("Token không hợp lệ")
	ErrExpiredToken = errors.New("Token đã hết hạn")
)

const (
	TokenTypeAccess  = "access"
	TokenTypeRefresh = "refresh"
	// TokenTypeRender is a short-lived, read-only token the server hands to its own headless
	// browser so it can open a print page as the user (server-side PDF export).
	TokenTypeRender = "render"
)

type TokenService interface {
	GenerateToken(cfg *config.AppConfig, userID string) (string, string, error)
	ValidateToken(tokenStr string) (*Claims, error)
	GenerateRenderToken(userID string, ttl time.Duration) (string, error)
}
type jwtService struct {
	secret []byte
}

func NewJWTService(secret string) TokenService {
	return &jwtService{
		secret: []byte(secret),
	}
}

type Claims struct {
	UserID    string `json:"userId"`
	TokenType string `json:"tokenType"`
	jwt.RegisteredClaims
}

// AuthorizesRequests reports whether the token may authenticate API and WebSocket requests:
// access tokens and render tokens. Refresh tokens are only good for /auth/refresh.
func (c *Claims) AuthorizesRequests() bool {
	return c.TokenType == TokenTypeAccess || c.TokenType == TokenTypeRender
}

func (s *jwtService) GenerateToken(cfg *config.AppConfig, userID string) (string, string, error) {
	if userID == "" {
		return "", "", errors.New("userId không được để trống")
	}

	secret := s.secret
	if cfg.JWTSecret != "" {
		secret = []byte(cfg.JWTSecret)
	}

	accessClaims := &Claims{
		UserID:    userID,
		TokenType: TokenTypeAccess,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(cfg.AccessTokenDuration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	accessTokenObj := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	accessToken, err := accessTokenObj.SignedString(secret)
	if err != nil {
		return "", "", fmt.Errorf("failed to sign access token: %w", err)
	}

	refreshClaims := &Claims{
		UserID:    userID,
		TokenType: TokenTypeRefresh,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(cfg.RefreshTokenDuration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	refreshTokenObj := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	refreshToken, err := refreshTokenObj.SignedString(secret)
	if err != nil {
		return "", "", fmt.Errorf("failed to sign refresh token: %w", err)
	}

	return accessToken, refreshToken, nil
}

// GenerateRenderToken signs a read-only token for userID that expires after ttl.
func (s *jwtService) GenerateRenderToken(userID string, ttl time.Duration) (string, error) {
	if userID == "" {
		return "", errors.New("userId không được để trống")
	}
	now := time.Now()
	claims := &Claims{
		UserID:    userID,
		TokenType: TokenTypeRender,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.secret)
}

func (s *jwtService) ValidateToken(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return s.secret, nil
	})
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
		}
		return nil, ErrInvalidToken
	}
	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}
	return nil, ErrInvalidToken
}
