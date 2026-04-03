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

type TenantRepo struct {
	pool *pgxpool.Pool
}

func NewTenantRepo(pool *pgxpool.Pool) *TenantRepo {
	return &TenantRepo{pool: pool}
}

func (r *TenantRepo) GetByID(ctx context.Context, id uuid.UUID) (*entity.AuthTenant, error) {
	query := `SELECT id, code, name, is_active, settings, created_at, updated_at FROM auth_tenants WHERE id = $1`

	tenant := &entity.AuthTenant{}
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&tenant.ID, &tenant.Code, &tenant.Name, &tenant.IsActive,
		&tenant.Settings, &tenant.CreatedAt, &tenant.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get tenant by ID: %w", err)
	}
	return tenant, nil
}

func (r *TenantRepo) GetByCode(ctx context.Context, code string) (*entity.AuthTenant, error) {
	query := `SELECT id, code, name, is_active, settings, created_at, updated_at FROM auth_tenants WHERE code = $1`

	tenant := &entity.AuthTenant{}
	err := r.pool.QueryRow(ctx, query, code).Scan(
		&tenant.ID, &tenant.Code, &tenant.Name, &tenant.IsActive,
		&tenant.Settings, &tenant.CreatedAt, &tenant.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get tenant by code: %w", err)
	}
	return tenant, nil
}

func (r *TenantRepo) GetActiveTenant(ctx context.Context, id uuid.UUID) (*entity.AuthTenant, error) {
	query := `SELECT id, code, name, is_active, settings, created_at, updated_at FROM auth_tenants WHERE id = $1 AND is_active = true`

	tenant := &entity.AuthTenant{}
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&tenant.ID, &tenant.Code, &tenant.Name, &tenant.IsActive,
		&tenant.Settings, &tenant.CreatedAt, &tenant.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get active tenant: %w", err)
	}
	return tenant, nil
}
