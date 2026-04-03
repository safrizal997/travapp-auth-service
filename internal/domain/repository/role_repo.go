package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/safrizal997/travapp-auth-service/internal/domain/entity"
)

type RoleRepository interface {
	GetDefaultRole(ctx context.Context, tenantID uuid.UUID) (*entity.AuthRole, error)
	GetByCodeAndTenant(ctx context.Context, code string, tenantID uuid.UUID) (*entity.AuthRole, error)
	AssignRole(ctx context.Context, assignment *entity.AuthRoleAssignment) error
	GetRoleAssignment(ctx context.Context, credentialID uuid.UUID, tenantID uuid.UUID) (*entity.AuthRoleAssignment, error)
	GetRoleWithPermissions(ctx context.Context, credentialID uuid.UUID, tenantID uuid.UUID) (*entity.AuthRole, []string, error)
}
