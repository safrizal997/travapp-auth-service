package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type RefreshTokenData struct {
	CredentialID string `json:"credential_id"`
	TenantID     string `json:"tenant_id"`
	TokenHash    string `json:"token_hash"`
	IPAddress    string `json:"ip_address"`
	UserAgent    string `json:"user_agent"`
	CreatedAt    string `json:"created_at"`
	FamilyID     string `json:"family_id"`
}

type OAuthStateData struct {
	TenantID     string `json:"tenant_id"`
	CodeVerifier string `json:"code_verifier"`
	CreatedAt    string `json:"created_at"`
}

type TokenRepository interface {
	StoreRefreshToken(ctx context.Context, credentialID uuid.UUID, jti string, data *RefreshTokenData, ttl time.Duration) error
	GetRefreshToken(ctx context.Context, credentialID uuid.UUID, jti string) (*RefreshTokenData, error)
	DeleteRefreshToken(ctx context.Context, credentialID uuid.UUID, jti string) error
	DeleteAllRefreshTokens(ctx context.Context, credentialID uuid.UUID) error
	FindRefreshTokenByHash(ctx context.Context, tokenHash string) (*RefreshTokenData, string, string, error)
	DeleteRefreshTokenFamily(ctx context.Context, familyID string) error

	BlacklistAccessToken(ctx context.Context, jti string, ttl time.Duration) error
	IsAccessTokenBlacklisted(ctx context.Context, jti string) (bool, error)

	StoreOAuthState(ctx context.Context, state string, data *OAuthStateData, ttl time.Duration) error
	GetOAuthState(ctx context.Context, state string) (*OAuthStateData, error)
	DeleteOAuthState(ctx context.Context, state string) error

	IncrementRateLimit(ctx context.Context, key string, ttl time.Duration) (int64, error)
	GetRateLimit(ctx context.Context, key string) (int64, error)

	StoreRBACCache(ctx context.Context, credentialID uuid.UUID, role string, permissions []string, ttl time.Duration) error
	GetRBACCache(ctx context.Context, credentialID uuid.UUID) (string, []string, error)
}
