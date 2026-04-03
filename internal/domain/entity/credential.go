package entity

import (
	"time"

	"github.com/google/uuid"
)

type AuthCredential struct {
	ID                  uuid.UUID  `db:"id" json:"id"`
	TenantID            uuid.UUID  `db:"tenant_id" json:"tenant_id"`
	Email               string     `db:"email" json:"email"`
	PasswordHash        *string    `db:"password_hash" json:"-"`
	Provider            string     `db:"provider" json:"provider"`
	ProviderID          *string    `db:"provider_id" json:"-"`
	IsActive            bool       `db:"is_active" json:"is_active"`
	IsEmailVerified     bool       `db:"is_email_verified" json:"is_email_verified"`
	EmailVerifiedAt     *time.Time `db:"email_verified_at" json:"email_verified_at,omitempty"`
	FailedLoginAttempts int        `db:"failed_login_attempts" json:"-"`
	LockedUntil         *time.Time `db:"locked_until" json:"-"`
	LastLoginAt         *time.Time `db:"last_login_at" json:"last_login_at,omitempty"`
	PasswordChangedAt   *time.Time `db:"password_changed_at" json:"-"`
	CreatedAt           time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt           time.Time  `db:"updated_at" json:"updated_at"`
}
