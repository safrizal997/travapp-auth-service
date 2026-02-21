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
	jwtpkg "github.com/safrizal997/travapp-auth-service/internal/pkg/jwt"
	"github.com/safrizal997/travapp-auth-service/internal/pkg/oauth"
	"go.uber.org/zap"
)

type OAuthUseCase struct {
	credentialRepo repository.CredentialRepository
	tenantRepo     repository.TenantRepository
	roleRepo       repository.RoleRepository
	tokenRepo      repository.TokenRepository
	loginHistRepo  repository.LoginHistoryRepository
	googleConfig   *oauth.GoogleConfig
	jwtManager     *jwtpkg.Manager
	logger         *zap.Logger
	accessTTL      time.Duration
	refreshTTL     time.Duration
}

func NewOAuthUseCase(
	credentialRepo repository.CredentialRepository,
	tenantRepo repository.TenantRepository,
	roleRepo repository.RoleRepository,
	tokenRepo repository.TokenRepository,
	loginHistRepo repository.LoginHistoryRepository,
	googleConfig *oauth.GoogleConfig,
	jwtManager *jwtpkg.Manager,
	logger *zap.Logger,
	accessTTL, refreshTTL time.Duration,
) *OAuthUseCase {
	return &OAuthUseCase{
		credentialRepo: credentialRepo,
		tenantRepo:     tenantRepo,
		roleRepo:       roleRepo,
		tokenRepo:      tokenRepo,
		loginHistRepo:  loginHistRepo,
		googleConfig:   googleConfig,
		jwtManager:     jwtManager,
		logger:         logger,
		accessTTL:      accessTTL,
		refreshTTL:     refreshTTL,
	}
}

type OAuthInitiateOutput struct {
	RedirectURL string
}

func (uc *OAuthUseCase) Initiate(ctx context.Context, tenantID uuid.UUID) (*OAuthInitiateOutput, error) {
	tenant, err := uc.tenantRepo.GetActiveTenant(ctx, tenantID)
	if err != nil {
		return nil, apperror.NewInternal(err)
	}
	if tenant == nil {
		return nil, apperror.NewBadRequest("Invalid or inactive tenant")
	}

	state, err := crypto.GenerateRandomHex(32)
	if err != nil {
		return nil, apperror.NewInternal(fmt.Errorf("failed to generate state: %w", err))
	}

	codeVerifier, err := crypto.GenerateCodeVerifier()
	if err != nil {
		return nil, apperror.NewInternal(fmt.Errorf("failed to generate code verifier: %w", err))
	}

	codeChallenge := crypto.GenerateCodeChallenge(codeVerifier)

	stateData := &repository.OAuthStateData{
		TenantID:     tenantID.String(),
		CodeVerifier: codeVerifier,
		CreatedAt:    time.Now().Format(time.RFC3339),
	}

	if err := uc.tokenRepo.StoreOAuthState(ctx, state, stateData, 10*time.Minute); err != nil {
		return nil, apperror.NewInternal(fmt.Errorf("failed to store OAuth state: %w", err))
	}

	redirectURL := uc.googleConfig.GetAuthURL(state, codeChallenge)

	return &OAuthInitiateOutput{RedirectURL: redirectURL}, nil
}

type OAuthCallbackInput struct {
	Code      string
	State     string
	IPAddress string
	UserAgent string
}

func (uc *OAuthUseCase) Callback(ctx context.Context, input OAuthCallbackInput) (*LoginOutput, error) {
	stateData, err := uc.tokenRepo.GetOAuthState(ctx, input.State)
	if err != nil {
		return nil, apperror.NewInternal(err)
	}
	if stateData == nil {
		return nil, apperror.NewBadRequest("Invalid or expired OAuth state")
	}

	_ = uc.tokenRepo.DeleteOAuthState(ctx, input.State)

	tenantID, err := uuid.Parse(stateData.TenantID)
	if err != nil {
		return nil, apperror.NewInternal(fmt.Errorf("invalid tenant_id in state: %w", err))
	}

	tokenResp, err := uc.googleConfig.ExchangeCode(ctx, input.Code, stateData.CodeVerifier)
	if err != nil {
		return nil, apperror.NewInternal(fmt.Errorf("failed to exchange code: %w", err))
	}

	userInfo, err := uc.googleConfig.VerifyIDToken(tokenResp.IDToken)
	if err != nil {
		return nil, apperror.NewInternal(fmt.Errorf("failed to verify ID token: %w", err))
	}

	cred, err := uc.credentialRepo.GetByProviderID(ctx, userInfo.Sub, "google", tenantID)
	if err != nil {
		return nil, apperror.NewInternal(err)
	}

	if cred == nil {
		now := time.Now()
		credentialID := uuid.New()
		providerID := userInfo.Sub
		cred = &entity.AuthCredential{
			ID:              credentialID,
			TenantID:        tenantID,
			Email:           userInfo.Email,
			Provider:        "google",
			ProviderID:      &providerID,
			IsActive:        true,
			IsEmailVerified: true,
			EmailVerifiedAt: &now,
			CreatedAt:       now,
			UpdatedAt:       now,
		}

		if err := uc.credentialRepo.Create(ctx, cred); err != nil {
			return nil, apperror.NewInternal(fmt.Errorf("failed to create Google credential: %w", err))
		}

		defaultRole, err := uc.roleRepo.GetDefaultRole(ctx, tenantID)
		if err != nil {
			uc.logger.Error("Failed to get default role", zap.Error(err))
		}
		if defaultRole != nil {
			assignment := &entity.AuthRoleAssignment{
				ID:           uuid.New(),
				CredentialID: credentialID,
				RoleID:       defaultRole.ID,
				TenantID:     tenantID,
				AssignedAt:   now,
			}
			if err := uc.roleRepo.AssignRole(ctx, assignment); err != nil {
				uc.logger.Error("Failed to assign default role", zap.Error(err))
			}
		}
	}

	if !cred.IsActive {
		return nil, apperror.NewUnauthorized("Account is suspended")
	}

	_ = uc.credentialRepo.UpdateLastLogin(ctx, cred.ID)

	return uc.generateOAuthTokens(ctx, cred, input.IPAddress, input.UserAgent)
}

func (uc *OAuthUseCase) generateOAuthTokens(ctx context.Context, cred *entity.AuthCredential, ipAddress, userAgent string) (*LoginOutput, error) {
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

	uc.recordLoginHistory(ctx, cred.ID, cred.TenantID, ipAddress, userAgent)

	return &LoginOutput{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    int(uc.accessTTL.Seconds()),
	}, nil
}

func (uc *OAuthUseCase) recordLoginHistory(ctx context.Context, credID uuid.UUID, tenantID uuid.UUID, ip, ua string) {
	var uaPtr *string
	if ua != "" {
		uaPtr = &ua
	}

	history := &entity.AuthLoginHistory{
		ID:           uuid.New(),
		CredentialID: &credID,
		TenantID:     tenantID,
		LoginMethod:  "google",
		IPAddress:    ip,
		UserAgent:    uaPtr,
		Status:       "success",
		Metadata:     json.RawMessage("{}"),
		CreatedAt:    time.Now(),
	}

	if err := uc.loginHistRepo.Create(ctx, history); err != nil {
		uc.logger.Error("Failed to record login history", zap.Error(err))
	}
}
