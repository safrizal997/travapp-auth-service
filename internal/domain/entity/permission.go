package entity

import (
	"time"

	"github.com/google/uuid"
)

type AuthPermission struct {
	ID          uuid.UUID `db:"id" json:"id"`
	Code        string    `db:"code" json:"code"`
	Name        string    `db:"name" json:"name"`
	Module      string    `db:"module" json:"module"`
	Description *string   `db:"description" json:"description,omitempty"`
	CreatedAt   time.Time `db:"created_at" json:"created_at"`
}
