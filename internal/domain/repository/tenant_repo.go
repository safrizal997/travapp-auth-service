package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/safrizal997/travapp-auth-service/internal/domain/entity"
)

type TenantRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*entity.AuthTenant, error)
	GetByCode(ctx context.Context, code string) (*entity.AuthTenant, error)
	GetActiveTenant(ctx context.Context, id uuid.UUID) (*entity.AuthTenant, error)
}
