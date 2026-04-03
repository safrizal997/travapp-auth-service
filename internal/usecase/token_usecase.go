package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/safrizal997/travapp-auth-service/internal/domain/repository"
	"github.com/safrizal997/travapp-auth-service/internal/pkg/apperror"
	"github.com/safrizal997/travapp-auth-service/internal/pkg/crypto"
	jwtpkg "github.com/safrizal997/travapp-auth-service/internal/pkg/jwt"
	"go.uber.org/zap"
)

type TokenUseCase struct {
	credentialRepo repository.CredentialRepository
	roleRepo       repository.RoleRepository
	tokenRepo      repository.TokenRepository
	jwtManager     *jwtpkg.Manager
	logger         *zap.Logger
	accessTTL      time.Duration
	refreshTTL     time.Duration
}

func NewTokenUseCase(
	credentialRepo repository.CredentialRepository,
	roleRepo repository.RoleRepository,
	tokenRepo repository.TokenRepository,
	jwtManager *jwtpkg.Manager,
	logger *zap.Logger,
	accessTTL, refreshTTL time.Duration,
) *TokenUseCase {
	return &TokenUseCase{
		credentialRepo: credentialRepo,
		roleRepo:       roleRepo,
		tokenRepo:      tokenRepo,
		jwtManager:     jwtManager,
		logger:         logger,
		accessTTL:      accessTTL,
		refreshTTL:     refreshTTL,
	}
}

type RefreshInput struct {
	RefreshToken string
	TenantID     uuid.UUID
}

func (uc *TokenUseCase) RefreshTokens(ctx context.Context, input RefreshInput) (*LoginOutput, error) {
	tokenHash := crypto.SHA256Hash(input.RefreshToken)

	rtData, credIDStr, jti, err := uc.tokenRepo.FindRefreshTokenByHash(ctx, tokenHash)
	if err != nil {
		return nil, apperror.NewInternal(err)
	}

	if rtData == nil {
		uc.logger.Warn("Refresh token not found - possible reuse detected", zap.String("token_hash_prefix", tokenHash[:8]))
		return nil, apperror.NewUnauthorized("Invalid refresh token")
	}

	if rtData.TenantID != input.TenantID.String() {
		return nil, apperror.NewUnauthorized("Invalid refresh token")
	}

	credID, err := uuid.Parse(credIDStr)
	if err != nil {
		return nil, apperror.NewInternal(fmt.Errorf("invalid credential ID: %w", err))
	}

	if err := uc.tokenRepo.DeleteRefreshToken(ctx, credID, jti); err != nil {
		return nil, apperror.NewInternal(err)
	}

	cred, err := uc.credentialRepo.GetByID(ctx, credID)
	if err != nil {
		return nil, apperror.NewInternal(err)
	}
	if cred == nil || !cred.IsActive {
		return nil, apperror.NewUnauthorized("Account not found or suspended")
	}

	role, permissions, err := uc.roleRepo.GetRoleWithPermissions(ctx, cred.ID, cred.TenantID)
	if err != nil {
		return nil, apperror.NewInternal(err)
	}

	roleCode := "traveler"
	var permCodes []string
	if role != nil {
		roleCode = role.Code
		permCodes = permissions
	}

	newAccessToken, newJTI, err := uc.jwtManager.GenerateAccessToken(cred.ID, cred.TenantID, roleCode, permCodes, cred.Provider, uc.accessTTL)
	if err != nil {
		return nil, apperror.NewInternal(fmt.Errorf("failed to generate access token: %w", err))
	}

	newRefreshToken, err := crypto.GenerateRandomHex(32)
	if err != nil {
		return nil, apperror.NewInternal(fmt.Errorf("failed to generate refresh token: %w", err))
	}

	newTokenHash := crypto.SHA256Hash(newRefreshToken)
	newRTData := &repository.RefreshTokenData{
		CredentialID: cred.ID.String(),
		TenantID:     cred.TenantID.String(),
		TokenHash:    newTokenHash,
		IPAddress:    rtData.IPAddress,
		UserAgent:    rtData.UserAgent,
		CreatedAt:    time.Now().Format(time.RFC3339),
		FamilyID:     rtData.FamilyID,
	}

	if err := uc.tokenRepo.StoreRefreshToken(ctx, cred.ID, newJTI, newRTData, uc.refreshTTL); err != nil {
		return nil, apperror.NewInternal(fmt.Errorf("failed to store refresh token: %w", err))
	}

	return &LoginOutput{
		AccessToken:  newAccessToken,
		RefreshToken: newRefreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    int(uc.accessTTL.Seconds()),
	}, nil
}
