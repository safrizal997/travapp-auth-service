package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/safrizal997/travapp-auth-service/internal/domain/entity"
)

type CredentialRepository interface {
	Create(ctx context.Context, cred *entity.AuthCredential) error
	GetByID(ctx context.Context, id uuid.UUID) (*entity.AuthCredential, error)
	GetByEmailAndTenant(ctx context.Context, email string, tenantID uuid.UUID, provider string) (*entity.AuthCredential, error)
	GetByProviderID(ctx context.Context, providerID string, provider string, tenantID uuid.UUID) (*entity.AuthCredential, error)
	UpdatePassword(ctx context.Context, id uuid.UUID, passwordHash string) error
	UpdateFailedAttempts(ctx context.Context, id uuid.UUID, attempts int, lockedUntil *string) error
	ResetFailedAttempts(ctx context.Context, id uuid.UUID) error
	UpdateLastLogin(ctx context.Context, id uuid.UUID) error
	VerifyEmail(ctx context.Context, id uuid.UUID) error
	UpdateIsActive(ctx context.Context, id uuid.UUID, isActive bool) error
}
