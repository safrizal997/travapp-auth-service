package entity

import (
	"time"

	"github.com/google/uuid"
)

type AuthRole struct {
	ID          uuid.UUID `db:"id" json:"id"`
	TenantID    uuid.UUID `db:"tenant_id" json:"tenant_id"`
	Code        string    `db:"code" json:"code"`
	Name        string    `db:"name" json:"name"`
	Description *string   `db:"description" json:"description,omitempty"`
	IsDefault   bool      `db:"is_default" json:"is_default"`
	IsSystem    bool      `db:"is_system" json:"is_system"`
	CreatedAt   time.Time `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time `db:"updated_at" json:"updated_at"`
}
