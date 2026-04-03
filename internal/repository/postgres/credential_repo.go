package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/safrizal997/travapp-auth-service/internal/domain/entity"
)

type CredentialRepo struct {
	pool *pgxpool.Pool
}

func NewCredentialRepo(pool *pgxpool.Pool) *CredentialRepo {
	return &CredentialRepo{pool: pool}
}

func (r *CredentialRepo) Create(ctx context.Context, cred *entity.AuthCredential) error {
	query := `
		INSERT INTO auth_credentials (id, tenant_id, email, password_hash, provider, provider_id, is_active, is_email_verified, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`

	_, err := r.pool.Exec(ctx, query,
		cred.ID, cred.TenantID, cred.Email, cred.PasswordHash, cred.Provider,
		cred.ProviderID, cred.IsActive, cred.IsEmailVerified, cred.CreatedAt, cred.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create credential: %w", err)
	}
	return nil
}

func (r *CredentialRepo) GetByID(ctx context.Context, id uuid.UUID) (*entity.AuthCredential, error) {
	query := `
		SELECT id, tenant_id, email, password_hash, provider, provider_id, is_active,
		       is_email_verified, email_verified_at, failed_login_attempts, locked_until,
		       last_login_at, password_changed_at, created_at, updated_at
		FROM auth_credentials WHERE id = $1`

	cred := &entity.AuthCredential{}
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&cred.ID, &cred.TenantID, &cred.Email, &cred.PasswordHash, &cred.Provider,
		&cred.ProviderID, &cred.IsActive, &cred.IsEmailVerified, &cred.EmailVerifiedAt,
		&cred.FailedLoginAttempts, &cred.LockedUntil, &cred.LastLoginAt,
		&cred.PasswordChangedAt, &cred.CreatedAt, &cred.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get credential by ID: %w", err)
	}
	return cred, nil
}

func (r *CredentialRepo) GetByEmailAndTenant(ctx context.Context, email string, tenantID uuid.UUID, provider string) (*entity.AuthCredential, error) {
	query := `
		SELECT id, tenant_id, email, password_hash, provider, provider_id, is_active,
		       is_email_verified, email_verified_at, failed_login_attempts, locked_until,
		       last_login_at, password_changed_at, created_at, updated_at
		FROM auth_credentials
		WHERE email = $1 AND tenant_id = $2 AND provider = $3`

	cred := &entity.AuthCredential{}
	err := r.pool.QueryRow(ctx, query, email, tenantID, provider).Scan(
		&cred.ID, &cred.TenantID, &cred.Email, &cred.PasswordHash, &cred.Provider,
		&cred.ProviderID, &cred.IsActive, &cred.IsEmailVerified, &cred.EmailVerifiedAt,
		&cred.FailedLoginAttempts, &cred.LockedUntil, &cred.LastLoginAt,
		&cred.PasswordChangedAt, &cred.CreatedAt, &cred.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get credential by email and tenant: %w", err)
	}
	return cred, nil
}

func (r *CredentialRepo) GetByProviderID(ctx context.Context, providerID string, provider string, tenantID uuid.UUID) (*entity.AuthCredential, error) {
	query := `
		SELECT id, tenant_id, email, password_hash, provider, provider_id, is_active,
		       is_email_verified, email_verified_at, failed_login_attempts, locked_until,
		       last_login_at, password_changed_at, created_at, updated_at
		FROM auth_credentials
		WHERE provider_id = $1 AND provider = $2 AND tenant_id = $3`

	cred := &entity.AuthCredential{}
	err := r.pool.QueryRow(ctx, query, providerID, provider, tenantID).Scan(
		&cred.ID, &cred.TenantID, &cred.Email, &cred.PasswordHash, &cred.Provider,
		&cred.ProviderID, &cred.IsActive, &cred.IsEmailVerified, &cred.EmailVerifiedAt,
		&cred.FailedLoginAttempts, &cred.LockedUntil, &cred.LastLoginAt,
		&cred.PasswordChangedAt, &cred.CreatedAt, &cred.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get credential by provider ID: %w", err)
	}
	return cred, nil
}

func (r *CredentialRepo) UpdatePassword(ctx context.Context, id uuid.UUID, passwordHash string) error {
	query := `UPDATE auth_credentials SET password_hash = $1, password_changed_at = $2 WHERE id = $3`
	_, err := r.pool.Exec(ctx, query, passwordHash, time.Now(), id)
	if err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}
	return nil
}

func (r *CredentialRepo) UpdateFailedAttempts(ctx context.Context, id uuid.UUID, attempts int, lockedUntil *string) error {
	query := `UPDATE auth_credentials SET failed_login_attempts = $1, locked_until = $2 WHERE id = $3`
	_, err := r.pool.Exec(ctx, query, attempts, lockedUntil, id)
	if err != nil {
		return fmt.Errorf("failed to update failed attempts: %w", err)
	}
	return nil
}

func (r *CredentialRepo) ResetFailedAttempts(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE auth_credentials SET failed_login_attempts = 0, locked_until = NULL WHERE id = $1`
	_, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to reset failed attempts: %w", err)
	}
	return nil
}

func (r *CredentialRepo) UpdateLastLogin(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE auth_credentials SET last_login_at = $1 WHERE id = $2`
	_, err := r.pool.Exec(ctx, query, time.Now(), id)
	if err != nil {
		return fmt.Errorf("failed to update last login: %w", err)
	}
	return nil
}

func (r *CredentialRepo) VerifyEmail(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE auth_credentials SET is_email_verified = true, email_verified_at = $1 WHERE id = $2`
	_, err := r.pool.Exec(ctx, query, time.Now(), id)
	if err != nil {
		return fmt.Errorf("failed to verify email: %w", err)
	}
	return nil
}

func (r *CredentialRepo) UpdateIsActive(ctx context.Context, id uuid.UUID, isActive bool) error {
	query := `UPDATE auth_credentials SET is_active = $1 WHERE id = $2`
	_, err := r.pool.Exec(ctx, query, isActive, id)
	if err != nil {
		return fmt.Errorf("failed to update is_active: %w", err)
	}
	return nil
}
