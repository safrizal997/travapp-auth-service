package usecase

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/safrizal997/travapp-auth-service/internal/domain/entity"
	"github.com/safrizal997/travapp-auth-service/internal/domain/repository"
	"github.com/safrizal997/travapp-auth-service/internal/pkg/crypto"
	"go.uber.org/zap"
)

type LogoutUseCase struct {
	tokenRepo     repository.TokenRepository
	loginHistRepo repository.LoginHistoryRepository
	logger        *zap.Logger
}

func NewLogoutUseCase(
	tokenRepo repository.TokenRepository,
	loginHistRepo repository.LoginHistoryRepository,
	logger *zap.Logger,
) *LogoutUseCase {
	return &LogoutUseCase{
		tokenRepo:     tokenRepo,
		loginHistRepo: loginHistRepo,
		logger:        logger,
	}
}

type LogoutInput struct {
	CredentialID uuid.UUID
	TenantID     uuid.UUID
	RefreshToken string
	AccessJTI    string
	AccessExpiry time.Time
	IPAddress    string
	UserAgent    string
}

func (uc *LogoutUseCase) Logout(ctx context.Context, input LogoutInput) error {
	if input.RefreshToken != "" {
		tokenHash := crypto.SHA256Hash(input.RefreshToken)
		rtData, credIDStr, jti, err := uc.tokenRepo.FindRefreshTokenByHash(ctx, tokenHash)
		if err != nil {
			uc.logger.Error("Failed to find refresh token", zap.Error(err))
		}
		if rtData != nil {
			credID, _ := uuid.Parse(credIDStr)
			_ = uc.tokenRepo.DeleteRefreshToken(ctx, credID, jti)
		}
	}

	remaining := time.Until(input.AccessExpiry)
	if remaining > 0 {
		if err := uc.tokenRepo.BlacklistAccessToken(ctx, input.AccessJTI, remaining); err != nil {
			uc.logger.Error("Failed to blacklist access token", zap.Error(err))
		}
	}

	uc.recordLogoutHistory(ctx, input.CredentialID, input.TenantID, input.IPAddress, input.UserAgent)
	return nil
}

func (uc *LogoutUseCase) LogoutAll(ctx context.Context, input LogoutInput) error {
	if err := uc.tokenRepo.DeleteAllRefreshTokens(ctx, input.CredentialID); err != nil {
		uc.logger.Error("Failed to delete all refresh tokens", zap.Error(err))
	}

	remaining := time.Until(input.AccessExpiry)
	if remaining > 0 {
		if err := uc.tokenRepo.BlacklistAccessToken(ctx, input.AccessJTI, remaining); err != nil {
			uc.logger.Error("Failed to blacklist access token", zap.Error(err))
		}
	}

	uc.recordLogoutHistory(ctx, input.CredentialID, input.TenantID, input.IPAddress, input.UserAgent)
	return nil
}

func (uc *LogoutUseCase) recordLogoutHistory(ctx context.Context, credID, tenantID uuid.UUID, ip, ua string) {
	var uaPtr *string
	if ua != "" {
		uaPtr = &ua
	}

	history := &entity.AuthLoginHistory{
		ID:           uuid.New(),
		CredentialID: &credID,
		TenantID:     tenantID,
		LoginMethod:  "logout",
		IPAddress:    ip,
		UserAgent:    uaPtr,
		Status:       "logout",
		Metadata:     json.RawMessage("{}"),
		CreatedAt:    time.Now(),
	}

	if err := uc.loginHistRepo.Create(ctx, history); err != nil {
		uc.logger.Error("Failed to record logout history", zap.Error(err))
	}
}
