package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/safrizal997/travapp-auth-service/internal/domain/entity"
)

type LoginHistoryRepo struct {
	pool *pgxpool.Pool
}

func NewLoginHistoryRepo(pool *pgxpool.Pool) *LoginHistoryRepo {
	return &LoginHistoryRepo{pool: pool}
}

func (r *LoginHistoryRepo) Create(ctx context.Context, history *entity.AuthLoginHistory) error {
	query := `
		INSERT INTO auth_login_history (id, credential_id, tenant_id, login_method, ip_address, user_agent, status, failure_reason, metadata, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`

	_, err := r.pool.Exec(ctx, query,
		history.ID, history.CredentialID, history.TenantID, history.LoginMethod,
		history.IPAddress, history.UserAgent, history.Status, history.FailureReason,
		history.Metadata, history.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create login history: %w", err)
	}
	return nil
}
