package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/safrizal997/travapp-auth-service/internal/domain/entity"
	"github.com/safrizal997/travapp-auth-service/internal/domain/repository"
	"github.com/safrizal997/travapp-auth-service/internal/pkg/apperror"
	"github.com/safrizal997/travapp-auth-service/internal/pkg/crypto"
	"github.com/safrizal997/travapp-auth-service/internal/pkg/hash"
	"go.uber.org/zap"
)

type EmailVerificationStore interface {
	CreateEmailVerification(ctx context.Context, ev *entity.AuthEmailVerification) error
}

type RegisterUseCase struct {
	credentialRepo         repository.CredentialRepository
	tenantRepo             repository.TenantRepository
	roleRepo               repository.RoleRepository
	hasher                 *hash.BCryptHasher
	logger                 *zap.Logger
	emailVerificationStore EmailVerificationStore
	baseURL                string
}

func NewRegisterUseCase(
	credentialRepo repository.CredentialRepository,
	tenantRepo repository.TenantRepository,
	roleRepo repository.RoleRepository,
	hasher *hash.BCryptHasher,
	logger *zap.Logger,
	emailVerificationStore EmailVerificationStore,
	baseURL string,
) *RegisterUseCase {
	return &RegisterUseCase{
		credentialRepo:         credentialRepo,
		tenantRepo:             tenantRepo,
		roleRepo:               roleRepo,
		hasher:                 hasher,
		logger:                 logger,
		emailVerificationStore: emailVerificationStore,
		baseURL:                baseURL,
	}
}

type RegisterInput struct {
	Email                string
	Password             string
	PasswordConfirmation string
	TenantID             uuid.UUID
}

type RegisterOutput struct {
	CredentialID      uuid.UUID
	VerificationToken string
}

func (uc *RegisterUseCase) Execute(ctx context.Context, input RegisterInput) (*RegisterOutput, error) {
	tenant, err := uc.tenantRepo.GetActiveTenant(ctx, input.TenantID)
	if err != nil {
		return nil, apperror.NewInternal(err)
	}
	if tenant == nil {
		return nil, apperror.NewBadRequest("Invalid or inactive tenant")
	}

	existing, err := uc.credentialRepo.GetByEmailAndTenant(ctx, input.Email, input.TenantID, "email")
	if err != nil {
		return nil, apperror.NewInternal(err)
	}
	if existing != nil {
		return nil, apperror.NewConflict("An account with this email already exists")
	}

	passwordHash, err := uc.hasher.Hash(input.Password)
	if err != nil {
		return nil, apperror.NewInternal(fmt.Errorf("failed to hash password: %w", err))
	}

	now := time.Now()
	credentialID := uuid.New()
	cred := &entity.AuthCredential{
		ID:              credentialID,
		TenantID:        input.TenantID,
		Email:           input.Email,
		PasswordHash:    &passwordHash,
		Provider:        "email",
		IsActive:        true,
		IsEmailVerified: false,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	if err := uc.credentialRepo.Create(ctx, cred); err != nil {
		return nil, apperror.NewInternal(fmt.Errorf("failed to create credential: %w", err))
	}

	defaultRole, err := uc.roleRepo.GetDefaultRole(ctx, input.TenantID)
	if err != nil {
		return nil, apperror.NewInternal(err)
	}
	if defaultRole != nil {
		assignment := &entity.AuthRoleAssignment{
			ID:           uuid.New(),
			CredentialID: credentialID,
			RoleID:       defaultRole.ID,
			TenantID:     input.TenantID,
			AssignedAt:   now,
		}
		if err := uc.roleRepo.AssignRole(ctx, assignment); err != nil {
			uc.logger.Error("Failed to assign default role", zap.Error(err))
		}
	}

	verificationToken, err := crypto.GenerateRandomHex(32)
	if err != nil {
		return nil, apperror.NewInternal(fmt.Errorf("failed to generate verification token: %w", err))
	}

	tokenHash := crypto.SHA256Hash(verificationToken)
	ev := &entity.AuthEmailVerification{
		ID:           uuid.New(),
		CredentialID: credentialID,
		TokenHash:    tokenHash,
		ExpiresAt:    now.Add(24 * time.Hour),
		CreatedAt:    now,
	}

	if err := uc.emailVerificationStore.CreateEmailVerification(ctx, ev); err != nil {
		return nil, apperror.NewInternal(fmt.Errorf("failed to save email verification: %w", err))
	}

	verifyURL := fmt.Sprintf("%s/api/v1/auth/verify-email?token=%s&email=%s", uc.baseURL, verificationToken, input.Email)
	uc.logger.Info("User registered successfully",
		zap.String("credential_id", credentialID.String()),
		zap.String("email", input.Email),
		zap.String("verification_url", verifyURL),
	)

	return &RegisterOutput{
		CredentialID:      credentialID,
		VerificationToken: verificationToken,
	}, nil
}
