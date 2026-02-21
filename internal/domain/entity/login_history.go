package entity

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type AuthLoginHistory struct {
	ID            uuid.UUID       `db:"id" json:"id"`
	CredentialID  *uuid.UUID      `db:"credential_id" json:"credential_id,omitempty"`
	TenantID      uuid.UUID       `db:"tenant_id" json:"tenant_id"`
	LoginMethod   string          `db:"login_method" json:"login_method"`
	IPAddress     string          `db:"ip_address" json:"ip_address"`
	UserAgent     *string         `db:"user_agent" json:"user_agent,omitempty"`
	Status        string          `db:"status" json:"status"`
	FailureReason *string         `db:"failure_reason" json:"failure_reason,omitempty"`
	Metadata      json.RawMessage `db:"metadata" json:"metadata,omitempty"`
	CreatedAt     time.Time       `db:"created_at" json:"created_at"`
}
