package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/safrizal997/travapp-auth-service/internal/domain/entity"
	"github.com/safrizal997/travapp-auth-service/internal/domain/repository"
	"github.com/safrizal997/travapp-auth-service/internal/pkg/apperror"
	"github.com/safrizal997/travapp-auth-service/internal/pkg/crypto"
	"github.com/safrizal997/travapp-auth-service/internal/pkg/hash"
	jwtpkg "github.com/safrizal997/travapp-auth-service/internal/pkg/jwt"
	"go.uber.org/zap"
)

type LoginUseCase struct {
	credentialRepo  repository.CredentialRepository
	tenantRepo      repository.TenantRepository
	roleRepo        repository.RoleRepository
	tokenRepo       repository.TokenRepository
	loginHistRepo   repository.LoginHistoryRepository
	hasher          *hash.BCryptHasher
	jwtManager      *jwtpkg.Manager
	logger          *zap.Logger
	accessTTL       time.Duration
	refreshTTL      time.Duration
	rateLimitMax    int
	rateLimitWindow time.Duration
}

func NewLoginUseCase(
	credentialRepo repository.CredentialRepository,
	tenantRepo repository.TenantRepository,
	roleRepo repository.RoleRepository,
	tokenRepo repository.TokenRepository,
	loginHistRepo repository.LoginHistoryRepository,
	hasher *hash.BCryptHasher,
	jwtManager *jwtpkg.Manager,
	logger *zap.Logger,
	accessTTL, refreshTTL time.Duration,
	rateLimitMax int,
	rateLimitWindow time.Duration,
) *LoginUseCase {
	return &LoginUseCase{
		credentialRepo:  credentialRepo,
		tenantRepo:      tenantRepo,
		roleRepo:        roleRepo,
		tokenRepo:       tokenRepo,
		loginHistRepo:   loginHistRepo,
		hasher:          hasher,
		jwtManager:      jwtManager,
		logger:          logger,
		accessTTL:       accessTTL,
		refreshTTL:      refreshTTL,
		rateLimitMax:    rateLimitMax,
		rateLimitWindow: rateLimitWindow,
	}
}

type LoginInput struct {
	Email     string
	Password  string
	TenantID  uuid.UUID
	IPAddress string
	UserAgent string
}

type LoginOutput struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
}

func (uc *LoginUseCase) Execute(ctx context.Context, input LoginInput) (*LoginOutput, error) {
	tenant, err := uc.tenantRepo.GetActiveTenant(ctx, input.TenantID)
	if err != nil {
		return nil, apperror.NewInternal(err)
	}
	if tenant == nil {
		return nil, apperror.NewBadRequest("Invalid or inactive tenant")
	}

	rateLimitKey := fmt.Sprintf("rate:login:%s:%s", input.Email, input.TenantID.String())

	rateLimitCount, err := uc.tokenRepo.IncrementRateLimit(ctx, rateLimitKey, uc.rateLimitWindow)
	if err != nil {
		uc.logger.Error("Failed to increment rate limit", zap.Error(err))
		return nil, apperror.NewTooManyRequests("Too many login attempts. Please try again later.")
	}
	if rateLimitCount > int64(uc.rateLimitMax) {
		return nil, apperror.NewTooManyRequests("Too many login attempts. Please try again later.")
	}

	cred, err := uc.credentialRepo.GetByEmailAndTenant(ctx, input.Email, input.TenantID, "email")
	if err != nil {
		return nil, apperror.NewInternal(err)
	}
	if cred == nil {
		uc.recordLoginHistory(ctx, nil, input.TenantID, "email", input.IPAddress, input.UserAgent, "failed", "invalid_credentials")
		return nil, apperror.NewUnauthorized("Invalid email or password")
	}

	if cred.LockedUntil != nil && cred.LockedUntil.After(time.Now()) {
		return nil, apperror.NewLocked("Account is temporarily locked. Please try again later.")
	}

	if cred.PasswordHash == nil {
		return nil, apperror.NewUnauthorized("Invalid email or password")
	}

	if err := uc.hasher.Compare(*cred.PasswordHash, input.Password); err != nil {
		newAttempts := cred.FailedLoginAttempts + 1
		var lockedUntil *string
		if newAttempts >= 5 {
			lockTime := time.Now().Add(30 * time.Minute).Format(time.RFC3339)
			lockedUntil = &lockTime
		}
		_ = uc.credentialRepo.UpdateFailedAttempts(ctx, cred.ID, newAttempts, lockedUntil)
		uc.recordLoginHistory(ctx, &cred.ID, input.TenantID, "email", input.IPAddress, input.UserAgent, "failed", "invalid_password")
		return nil, apperror.NewUnauthorized("Invalid email or password")
	}

	if !cred.IsActive {
		return nil, apperror.NewUnauthorized("Account is suspended")
	}

	if !cred.IsEmailVerified {
		return nil, apperror.NewUnauthorized("Please verify your email before logging in")
	}

	_ = uc.credentialRepo.ResetFailedAttempts(ctx, cred.ID)
	_ = uc.credentialRepo.UpdateLastLogin(ctx, cred.ID)

	return uc.generateTokens(ctx, cred, input.IPAddress, input.UserAgent)
}

func (uc *LoginUseCase) generateTokens(ctx context.Context, cred *entity.AuthCredential, ipAddress, userAgent string) (*LoginOutput, error) {
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

	accessToken, jti, err := uc.jwtManager.GenerateAccessToken(cred.ID, cred.TenantID, roleCode, permCodes, cred.Provider, uc.accessTTL)
	if err != nil {
		return nil, apperror.NewInternal(fmt.Errorf("failed to generate access token: %w", err))
	}

	refreshToken, err := crypto.GenerateRandomHex(32)
	if err != nil {
		return nil, apperror.NewInternal(fmt.Errorf("failed to generate refresh token: %w", err))
	}

	familyID := uuid.New().String()
	tokenHash := crypto.SHA256Hash(refreshToken)

	rtData := &repository.RefreshTokenData{
		CredentialID: cred.ID.String(),
		TenantID:     cred.TenantID.String(),
		TokenHash:    tokenHash,
		IPAddress:    ipAddress,
		UserAgent:    userAgent,
		CreatedAt:    time.Now().Format(time.RFC3339),
		FamilyID:     familyID,
	}

	if err := uc.tokenRepo.StoreRefreshToken(ctx, cred.ID, jti, rtData, uc.refreshTTL); err != nil {
		return nil, apperror.NewInternal(fmt.Errorf("failed to store refresh token: %w", err))
	}

	uc.recordLoginHistory(ctx, &cred.ID, cred.TenantID, cred.Provider, ipAddress, userAgent, "success", "")

	return &LoginOutput{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    int(uc.accessTTL.Seconds()),
	}, nil
}

func (uc *LoginUseCase) recordLoginHistory(ctx context.Context, credID *uuid.UUID, tenantID uuid.UUID, method, ip, ua, status, failureReason string) {
	var fr *string
	if failureReason != "" {
		fr = &failureReason
	}
	var uaPtr *string
	if ua != "" {
		uaPtr = &ua
	}

	history := &entity.AuthLoginHistory{
		ID:            uuid.New(),
		CredentialID:  credID,
		TenantID:      tenantID,
		LoginMethod:   method,
		IPAddress:     ip,
		UserAgent:     uaPtr,
		Status:        status,
		FailureReason: fr,
		Metadata:      json.RawMessage("{}"),
		CreatedAt:     time.Now(),
	}

	if err := uc.loginHistRepo.Create(ctx, history); err != nil {
		uc.logger.Error("Failed to record login history", zap.Error(err))
	}
}
