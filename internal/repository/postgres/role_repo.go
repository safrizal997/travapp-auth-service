package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/safrizal997/travapp-auth-service/internal/domain/entity"
)

type RoleRepo struct {
	pool *pgxpool.Pool
}

func NewRoleRepo(pool *pgxpool.Pool) *RoleRepo {
	return &RoleRepo{pool: pool}
}

func (r *RoleRepo) GetDefaultRole(ctx context.Context, tenantID uuid.UUID) (*entity.AuthRole, error) {
	query := `SELECT id, tenant_id, code, name, description, is_default, is_system, created_at, updated_at
	          FROM auth_roles WHERE tenant_id = $1 AND is_default = true LIMIT 1`

	role := &entity.AuthRole{}
	err := r.pool.QueryRow(ctx, query, tenantID).Scan(
		&role.ID, &role.TenantID, &role.Code, &role.Name, &role.Description,
		&role.IsDefault, &role.IsSystem, &role.CreatedAt, &role.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get default role: %w", err)
	}
	return role, nil
}

func (r *RoleRepo) GetByCodeAndTenant(ctx context.Context, code string, tenantID uuid.UUID) (*entity.AuthRole, error) {
	query := `SELECT id, tenant_id, code, name, description, is_default, is_system, created_at, updated_at
	          FROM auth_roles WHERE code = $1 AND tenant_id = $2`

	role := &entity.AuthRole{}
	err := r.pool.QueryRow(ctx, query, code, tenantID).Scan(
		&role.ID, &role.TenantID, &role.Code, &role.Name, &role.Description,
		&role.IsDefault, &role.IsSystem, &role.CreatedAt, &role.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get role by code: %w", err)
	}
	return role, nil
}

func (r *RoleRepo) AssignRole(ctx context.Context, assignment *entity.AuthRoleAssignment) error {
	query := `
		INSERT INTO auth_role_assignments (id, credential_id, role_id, tenant_id, assigned_by, assigned_at, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (credential_id, tenant_id) DO UPDATE SET role_id = $3, assigned_by = $5, assigned_at = $6, expires_at = $7`

	_, err := r.pool.Exec(ctx, query,
		assignment.ID, assignment.CredentialID, assignment.RoleID, assignment.TenantID,
		assignment.AssignedBy, assignment.AssignedAt, assignment.ExpiresAt,
	)
	if err != nil {
		return fmt.Errorf("failed to assign role: %w", err)
	}
	return nil
}

func (r *RoleRepo) GetRoleAssignment(ctx context.Context, credentialID uuid.UUID, tenantID uuid.UUID) (*entity.AuthRoleAssignment, error) {
	query := `SELECT id, credential_id, role_id, tenant_id, assigned_by, assigned_at, expires_at
	          FROM auth_role_assignments WHERE credential_id = $1 AND tenant_id = $2`

	ra := &entity.AuthRoleAssignment{}
	err := r.pool.QueryRow(ctx, query, credentialID, tenantID).Scan(
		&ra.ID, &ra.CredentialID, &ra.RoleID, &ra.TenantID,
		&ra.AssignedBy, &ra.AssignedAt, &ra.ExpiresAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get role assignment: %w", err)
	}
	return ra, nil
}

func (r *RoleRepo) GetRoleWithPermissions(ctx context.Context, credentialID uuid.UUID, tenantID uuid.UUID) (*entity.AuthRole, []string, error) {
	query := `
		SELECT ar.id, ar.tenant_id, ar.code, ar.name, ar.description, ar.is_default, ar.is_system, ar.created_at, ar.updated_at
		FROM auth_roles ar
		JOIN auth_role_assignments ara ON ar.id = ara.role_id
		WHERE ara.credential_id = $1 AND ara.tenant_id = $2
		AND (ara.expires_at IS NULL OR ara.expires_at > NOW())`

	role := &entity.AuthRole{}
	err := r.pool.QueryRow(ctx, query, credentialID, tenantID).Scan(
		&role.ID, &role.TenantID, &role.Code, &role.Name, &role.Description,
		&role.IsDefault, &role.IsSystem, &role.CreatedAt, &role.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("failed to get role with permissions: %w", err)
	}

	permQuery := `
		SELECT ap.code FROM auth_permissions ap
		JOIN auth_role_permissions arp ON ap.id = arp.permission_id
		WHERE arp.role_id = $1`

	rows, err := r.pool.Query(ctx, permQuery, role.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get permissions: %w", err)
	}
	defer rows.Close()

	var permissions []string
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			return nil, nil, fmt.Errorf("failed to scan permission: %w", err)
		}
		permissions = append(permissions, code)
	}

	return role, permissions, nil
}
