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

type PasswordUseCase struct {
	credentialRepo repository.CredentialRepository
	tenantRepo     repository.TenantRepository
	tokenRepo      repository.TokenRepository
	hasher         *hash.BCryptHasher
	logger         *zap.Logger
	pool           PasswordResetStore
	baseURL        string
}

type PasswordResetStore interface {
	CreatePasswordReset(ctx context.Context, reset *entity.AuthPasswordReset) error
	GetPasswordResetByToken(ctx context.Context, tokenHash string) (*entity.AuthPasswordReset, error)
	MarkPasswordResetUsed(ctx context.Context, id uuid.UUID) error
	CreateEmailVerification(ctx context.Context, ev *entity.AuthEmailVerification) error
	GetEmailVerificationByToken(ctx context.Context, tokenHash string) (*entity.AuthEmailVerification, error)
	MarkEmailVerified(ctx context.Context, id uuid.UUID) error
}

func NewPasswordUseCase(
	credentialRepo repository.CredentialRepository,
	tenantRepo repository.TenantRepository,
	tokenRepo repository.TokenRepository,
	hasher *hash.BCryptHasher,
	logger *zap.Logger,
	pool PasswordResetStore,
	baseURL string,
) *PasswordUseCase {
	return &PasswordUseCase{
		credentialRepo: credentialRepo,
		tenantRepo:     tenantRepo,
		tokenRepo:      tokenRepo,
		hasher:         hasher,
		logger:         logger,
		pool:           pool,
		baseURL:        baseURL,
	}
}

type ChangePasswordInput struct {
	CredentialID       uuid.UUID
	CurrentPassword    string
	NewPassword        string
	NewPasswordConfirm string
}

func (uc *PasswordUseCase) ChangePassword(ctx context.Context, input ChangePasswordInput) error {
	cred, err := uc.credentialRepo.GetByID(ctx, input.CredentialID)
	if err != nil {
		return apperror.NewInternal(err)
	}
	if cred == nil {
		return apperror.NewNotFound("Credential not found")
	}

	if cred.PasswordHash == nil {
		return apperror.NewBadRequest("Cannot change password for OAuth accounts")
	}

	if err := uc.hasher.Compare(*cred.PasswordHash, input.CurrentPassword); err != nil {
		return apperror.NewUnauthorized("Current password is incorrect")
	}

	newHash, err := uc.hasher.Hash(input.NewPassword)
	if err != nil {
		return apperror.NewInternal(fmt.Errorf("failed to hash password: %w", err))
	}

	if err := uc.credentialRepo.UpdatePassword(ctx, cred.ID, newHash); err != nil {
		return apperror.NewInternal(fmt.Errorf("failed to update password: %w", err))
	}

	_ = uc.tokenRepo.DeleteAllRefreshTokens(ctx, cred.ID)

	uc.logger.Info("Password changed", zap.String("credential_id", cred.ID.String()))
	return nil
}

type ForgotPasswordInput struct {
	Email    string
	TenantID uuid.UUID
}

type ForgotPasswordOutput struct {
	ResetToken string
}

func (uc *PasswordUseCase) ForgotPassword(ctx context.Context, input ForgotPasswordInput) (*ForgotPasswordOutput, error) {
	rateLimitKey := fmt.Sprintf("rate:reset:%s:%s", input.Email, input.TenantID.String())
	count, err := uc.tokenRepo.GetRateLimit(ctx, rateLimitKey)
	if err != nil {
		uc.logger.Error("Failed to check rate limit", zap.Error(err))
	}
	if count >= 3 {
		return nil, apperror.NewTooManyRequests("Too many password reset requests. Please try again later.")
	}

	_, _ = uc.tokenRepo.IncrementRateLimit(ctx, rateLimitKey, 1*time.Hour)

	cred, err := uc.credentialRepo.GetByEmailAndTenant(ctx, input.Email, input.TenantID, "email")
	if err != nil {
		return nil, apperror.NewInternal(err)
	}

	if cred == nil {
		return &ForgotPasswordOutput{}, nil
	}

	resetToken, err := crypto.GenerateRandomHex(32)
	if err != nil {
		return nil, apperror.NewInternal(err)
	}

	tokenHash := crypto.SHA256Hash(resetToken)
	reset := &entity.AuthPasswordReset{
		ID:           uuid.New(),
		CredentialID: cred.ID,
		TokenHash:    tokenHash,
		ExpiresAt:    time.Now().Add(1 * time.Hour),
		CreatedAt:    time.Now(),
	}

	if err := uc.pool.CreatePasswordReset(ctx, reset); err != nil {
		return nil, apperror.NewInternal(err)
	}

	resetURL := fmt.Sprintf("%s/api/v1/auth/password/reset?token=%s&email=%s", uc.baseURL, resetToken, input.Email)
	uc.logger.Info("Password reset requested",
		zap.String("email", input.Email),
		zap.String("reset_url", resetURL),
	)

	return &ForgotPasswordOutput{ResetToken: resetToken}, nil
}

type ResetPasswordInput struct {
	Token              string
	Email              string
	NewPassword        string
	NewPasswordConfirm string
}

func (uc *PasswordUseCase) ResetPassword(ctx context.Context, input ResetPasswordInput) error {
	tokenHash := crypto.SHA256Hash(input.Token)

	reset, err := uc.pool.GetPasswordResetByToken(ctx, tokenHash)
	if err != nil {
		return apperror.NewInternal(err)
	}
	if reset == nil {
		return apperror.NewBadRequest("Invalid or expired reset token")
	}

	if time.Now().After(reset.ExpiresAt) {
		return apperror.NewBadRequest("Reset token has expired")
	}

	if reset.UsedAt != nil {
		return apperror.NewBadRequest("Reset token has already been used")
	}

	newHash, err := uc.hasher.Hash(input.NewPassword)
	if err != nil {
		return apperror.NewInternal(fmt.Errorf("failed to hash password: %w", err))
	}

	if err := uc.credentialRepo.UpdatePassword(ctx, reset.CredentialID, newHash); err != nil {
		return apperror.NewInternal(fmt.Errorf("failed to update password: %w", err))
	}

	if err := uc.pool.MarkPasswordResetUsed(ctx, reset.ID); err != nil {
		uc.logger.Error("Failed to mark reset token as used", zap.Error(err))
	}

	_ = uc.tokenRepo.DeleteAllRefreshTokens(ctx, reset.CredentialID)

	return nil
}

type VerifyEmailInput struct {
	Token string
	Email string
}

func (uc *PasswordUseCase) VerifyEmail(ctx context.Context, input VerifyEmailInput) error {
	tokenHash := crypto.SHA256Hash(input.Token)

	ev, err := uc.pool.GetEmailVerificationByToken(ctx, tokenHash)
	if err != nil {
		return apperror.NewInternal(err)
	}
	if ev == nil {
		return apperror.NewBadRequest("Invalid or expired verification token")
	}

	if time.Now().After(ev.ExpiresAt) {
		return apperror.NewBadRequest("Verification token has expired")
	}

	if ev.VerifiedAt != nil {
		return apperror.NewBadRequest("Email has already been verified")
	}

	if err := uc.credentialRepo.VerifyEmail(ctx, ev.CredentialID); err != nil {
		return apperror.NewInternal(fmt.Errorf("failed to verify email: %w", err))
	}

	if err := uc.pool.MarkEmailVerified(ctx, ev.ID); err != nil {
		uc.logger.Error("Failed to mark email as verified", zap.Error(err))
	}

	return nil
}
