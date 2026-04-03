package entity

import (
	"time"

	"github.com/google/uuid"
)

type AuthRoleAssignment struct {
	ID           uuid.UUID  `db:"id" json:"id"`
	CredentialID uuid.UUID  `db:"credential_id" json:"credential_id"`
	RoleID       uuid.UUID  `db:"role_id" json:"role_id"`
	TenantID     uuid.UUID  `db:"tenant_id" json:"tenant_id"`
	AssignedBy   *uuid.UUID `db:"assigned_by" json:"assigned_by,omitempty"`
	AssignedAt   time.Time  `db:"assigned_at" json:"assigned_at"`
	ExpiresAt    *time.Time `db:"expires_at" json:"expires_at,omitempty"`
}
