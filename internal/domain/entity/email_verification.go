package entity

import (
	"time"

	"github.com/google/uuid"
)

type AuthEmailVerification struct {
	ID           uuid.UUID  `db:"id" json:"id"`
	CredentialID uuid.UUID  `db:"credential_id" json:"credential_id"`
	TokenHash    string     `db:"token_hash" json:"-"`
	ExpiresAt    time.Time  `db:"expires_at" json:"expires_at"`
	VerifiedAt   *time.Time `db:"verified_at" json:"verified_at,omitempty"`
	CreatedAt    time.Time  `db:"created_at" json:"created_at"`
}
