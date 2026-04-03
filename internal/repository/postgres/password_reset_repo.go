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

type PasswordResetRepo struct {
	pool *pgxpool.Pool
}

func NewPasswordResetRepo(pool *pgxpool.Pool) *PasswordResetRepo {
	return &PasswordResetRepo{pool: pool}
}

func (r *PasswordResetRepo) CreatePasswordReset(ctx context.Context, reset *entity.AuthPasswordReset) error {
	query := `INSERT INTO auth_password_resets (id, credential_id, token_hash, expires_at, created_at)
	          VALUES ($1, $2, $3, $4, $5)`
	_, err := r.pool.Exec(ctx, query, reset.ID, reset.CredentialID, reset.TokenHash, reset.ExpiresAt, reset.CreatedAt)
	if err != nil {
		return fmt.Errorf("failed to create password reset: %w", err)
	}
	return nil
}

func (r *PasswordResetRepo) GetPasswordResetByToken(ctx context.Context, tokenHash string) (*entity.AuthPasswordReset, error) {
	query := `SELECT id, credential_id, token_hash, expires_at, used_at, created_at
	          FROM auth_password_resets WHERE token_hash = $1`

	reset := &entity.AuthPasswordReset{}
	err := r.pool.QueryRow(ctx, query, tokenHash).Scan(
		&reset.ID, &reset.CredentialID, &reset.TokenHash, &reset.ExpiresAt, &reset.UsedAt, &reset.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get password reset: %w", err)
	}
	return reset, nil
}

func (r *PasswordResetRepo) MarkPasswordResetUsed(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE auth_password_resets SET used_at = $1 WHERE id = $2`
	_, err := r.pool.Exec(ctx, query, time.Now(), id)
	if err != nil {
		return fmt.Errorf("failed to mark password reset used: %w", err)
	}
	return nil
}

func (r *PasswordResetRepo) CreateEmailVerification(ctx context.Context, ev *entity.AuthEmailVerification) error {
	query := `INSERT INTO auth_email_verifications (id, credential_id, token_hash, expires_at, created_at)
	          VALUES ($1, $2, $3, $4, $5)`
	_, err := r.pool.Exec(ctx, query, ev.ID, ev.CredentialID, ev.TokenHash, ev.ExpiresAt, ev.CreatedAt)
	if err != nil {
		return fmt.Errorf("failed to create email verification: %w", err)
	}
	return nil
}

func (r *PasswordResetRepo) GetEmailVerificationByToken(ctx context.Context, tokenHash string) (*entity.AuthEmailVerification, error) {
	query := `SELECT id, credential_id, token_hash, expires_at, verified_at, created_at
	          FROM auth_email_verifications WHERE token_hash = $1`

	ev := &entity.AuthEmailVerification{}
	err := r.pool.QueryRow(ctx, query, tokenHash).Scan(
		&ev.ID, &ev.CredentialID, &ev.TokenHash, &ev.ExpiresAt, &ev.VerifiedAt, &ev.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get email verification: %w", err)
	}
	return ev, nil
}

func (r *PasswordResetRepo) MarkEmailVerified(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE auth_email_verifications SET verified_at = $1 WHERE id = $2`
	_, err := r.pool.Exec(ctx, query, time.Now(), id)
	if err != nil {
		return fmt.Errorf("failed to mark email verified: %w", err)
	}
	return nil
}
