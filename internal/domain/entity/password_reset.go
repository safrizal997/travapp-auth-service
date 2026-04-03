package entity

import (
	"time"

	"github.com/google/uuid"
)

type AuthPasswordReset struct {
	ID           uuid.UUID  `db:"id" json:"id"`
	CredentialID uuid.UUID  `db:"credential_id" json:"credential_id"`
	TokenHash    string     `db:"token_hash" json:"-"`
	ExpiresAt    time.Time  `db:"expires_at" json:"expires_at"`
	UsedAt       *time.Time `db:"used_at" json:"used_at,omitempty"`
	CreatedAt    time.Time  `db:"created_at" json:"created_at"`
}
