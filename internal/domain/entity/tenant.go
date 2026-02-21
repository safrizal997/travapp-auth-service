package entity

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type AuthTenant struct {
	ID        uuid.UUID       `db:"id" json:"id"`
	Code      string          `db:"code" json:"code"`
	Name      string          `db:"name" json:"name"`
	IsActive  bool            `db:"is_active" json:"is_active"`
	Settings  json.RawMessage `db:"settings" json:"settings"`
	CreatedAt time.Time       `db:"created_at" json:"created_at"`
	UpdatedAt time.Time       `db:"updated_at" json:"updated_at"`
}

type TenantSettings struct {
	PasswordPolicy       PasswordPolicy `json:"password_policy"`
	OAuthProviders       []string       `json:"oauth_providers"`
	SessionConfig        SessionConfig  `json:"session_config"`
	MaxFailedAttempts    int            `json:"max_failed_attempts"`
	LockoutDurationMins  int            `json:"lockout_duration_minutes"`
}

type PasswordPolicy struct {
	MinLength        int  `json:"min_length"`
	RequireUppercase bool `json:"require_uppercase"`
}

type SessionConfig struct {
	AccessTokenTTL  int `json:"access_token_ttl"`
	RefreshTokenTTL int `json:"refresh_token_ttl"`
}
