package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/safrizal997/travapp-auth-service/internal/domain/entity"
	"github.com/safrizal997/travapp-auth-service/internal/domain/repository"
	"github.com/safrizal997/travapp-auth-service/internal/mocks"
	"github.com/safrizal997/travapp-auth-service/internal/pkg/crypto"
	"github.com/safrizal997/travapp-auth-service/internal/testutil"
	"go.uber.org/mock/gomock"
)

func newTokenUCWithMocks(t *testing.T, ctrl *gomock.Controller) (
	*TokenUseCase,
	*mocks.MockCredentialRepository,
	*mocks.MockRoleRepository,
	*mocks.MockTokenRepository,
) {
	credRepo := mocks.NewMockCredentialRepository(ctrl)
	roleRepo := mocks.NewMockRoleRepository(ctrl)
	tokenRepo := mocks.NewMockTokenRepository(ctrl)

	uc := NewTokenUseCase(
		credRepo, roleRepo, tokenRepo,
		testutil.NewTestJWTManager(t),
		testutil.NewTestLogger(),
		15*time.Minute, 7*24*time.Hour,
	)

	return uc, credRepo, roleRepo, tokenRepo
}

func TestRefreshTokens_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	uc, credRepo, roleRepo, tokenRepo := newTokenUCWithMocks(t, ctrl)

	credID := uuid.New()
	tenantID := uuid.New()
	refreshToken := "original-refresh-token"
	tokenHash := crypto.SHA256Hash(refreshToken)

	rtData := &repository.RefreshTokenData{
		CredentialID: credID.String(),
		TenantID:     tenantID.String(),
		TokenHash:    tokenHash,
		FamilyID:     "family-123",
		IPAddress:    "127.0.0.1",
		UserAgent:    "test",
	}

	cred := &entity.AuthCredential{
		ID:       credID,
		TenantID: tenantID,
		Provider: "email",
		IsActive: true,
	}

	role := &entity.AuthRole{Code: "traveler"}

	tokenRepo.EXPECT().FindRefreshTokenByHash(gomock.Any(), tokenHash).Return(rtData, credID.String(), "old-jti", nil)
	tokenRepo.EXPECT().DeleteRefreshToken(gomock.Any(), credID, "old-jti").Return(nil)
	credRepo.EXPECT().GetByID(gomock.Any(), credID).Return(cred, nil)
	roleRepo.EXPECT().GetRoleWithPermissions(gomock.Any(), credID, tenantID).Return(role, []string{"read"}, nil)
	tokenRepo.EXPECT().StoreRefreshToken(gomock.Any(), credID, gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

	output, err := uc.RefreshTokens(context.Background(), RefreshInput{
		RefreshToken: refreshToken,
		TenantID:     tenantID,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output.AccessToken == "" {
		t.Error("access token should not be empty")
	}
	if output.RefreshToken == "" {
		t.Error("refresh token should not be empty")
	}
	if output.TokenType != "Bearer" {
		t.Errorf("expected Bearer, got %s", output.TokenType)
	}
}

func TestRefreshTokens_InvalidToken(t *testing.T) {
	ctrl := gomock.NewController(t)
	uc, _, _, tokenRepo := newTokenUCWithMocks(t, ctrl)

	tenantID := uuid.New()
	tokenRepo.EXPECT().FindRefreshTokenByHash(gomock.Any(), gomock.Any()).Return(nil, "", "", nil)

	_, err := uc.RefreshTokens(context.Background(), RefreshInput{
		RefreshToken: "invalid-token",
		TenantID:     tenantID,
	})
	assertAppErrorCode(t, err, 401)
}

func TestRefreshTokens_TenantMismatch(t *testing.T) {
	ctrl := gomock.NewController(t)
	uc, _, _, tokenRepo := newTokenUCWithMocks(t, ctrl)

	credID := uuid.New()
	tenantID := uuid.New()
	wrongTenantID := uuid.New()

	rtData := &repository.RefreshTokenData{
		CredentialID: credID.String(),
		TenantID:     tenantID.String(),
	}

	tokenRepo.EXPECT().FindRefreshTokenByHash(gomock.Any(), gomock.Any()).Return(rtData, credID.String(), "jti", nil)

	_, err := uc.RefreshTokens(context.Background(), RefreshInput{
		RefreshToken: "some-token",
		TenantID:     wrongTenantID,
	})
	assertAppErrorCode(t, err, 401)
}

func TestRefreshTokens_SuspendedCredential(t *testing.T) {
	ctrl := gomock.NewController(t)
	uc, credRepo, _, tokenRepo := newTokenUCWithMocks(t, ctrl)

	credID := uuid.New()
	tenantID := uuid.New()

	rtData := &repository.RefreshTokenData{
		CredentialID: credID.String(),
		TenantID:     tenantID.String(),
	}

	cred := &entity.AuthCredential{
		ID:       credID,
		IsActive: false,
	}

	tokenRepo.EXPECT().FindRefreshTokenByHash(gomock.Any(), gomock.Any()).Return(rtData, credID.String(), "jti", nil)
	tokenRepo.EXPECT().DeleteRefreshToken(gomock.Any(), credID, "jti").Return(nil)
	credRepo.EXPECT().GetByID(gomock.Any(), credID).Return(cred, nil)

	_, err := uc.RefreshTokens(context.Background(), RefreshInput{
		RefreshToken: "some-token",
		TenantID:     tenantID,
	})
	assertAppErrorCode(t, err, 401)
}

func TestRefreshTokens_FindError(t *testing.T) {
	ctrl := gomock.NewController(t)
	uc, _, _, tokenRepo := newTokenUCWithMocks(t, ctrl)

	tenantID := uuid.New()
	tokenRepo.EXPECT().FindRefreshTokenByHash(gomock.Any(), gomock.Any()).Return(nil, "", "", errors.New("redis error"))

	_, err := uc.RefreshTokens(context.Background(), RefreshInput{
		RefreshToken: "some-token",
		TenantID:     tenantID,
	})
	assertAppErrorCode(t, err, 500)
}

func TestRefreshTokens_DeleteError(t *testing.T) {
	ctrl := gomock.NewController(t)
	uc, _, _, tokenRepo := newTokenUCWithMocks(t, ctrl)

	credID := uuid.New()
	tenantID := uuid.New()

	rtData := &repository.RefreshTokenData{
		CredentialID: credID.String(),
		TenantID:     tenantID.String(),
	}

	tokenRepo.EXPECT().FindRefreshTokenByHash(gomock.Any(), gomock.Any()).Return(rtData, credID.String(), "jti", nil)
	tokenRepo.EXPECT().DeleteRefreshToken(gomock.Any(), credID, "jti").Return(errors.New("redis error"))

	_, err := uc.RefreshTokens(context.Background(), RefreshInput{
		RefreshToken: "some-token",
		TenantID:     tenantID,
	})
	assertAppErrorCode(t, err, 500)
}

func TestRefreshTokens_NoRole(t *testing.T) {
	ctrl := gomock.NewController(t)
	uc, credRepo, roleRepo, tokenRepo := newTokenUCWithMocks(t, ctrl)

	credID := uuid.New()
	tenantID := uuid.New()
	refreshToken := "test-refresh-token"
	tokenHash := crypto.SHA256Hash(refreshToken)

	rtData := &repository.RefreshTokenData{
		CredentialID: credID.String(),
		TenantID:     tenantID.String(),
		TokenHash:    tokenHash,
		FamilyID:     "family-123",
	}

	cred := &entity.AuthCredential{
		ID:       credID,
		TenantID: tenantID,
		Provider: "email",
		IsActive: true,
	}

	tokenRepo.EXPECT().FindRefreshTokenByHash(gomock.Any(), tokenHash).Return(rtData, credID.String(), "old-jti", nil)
	tokenRepo.EXPECT().DeleteRefreshToken(gomock.Any(), credID, "old-jti").Return(nil)
	credRepo.EXPECT().GetByID(gomock.Any(), credID).Return(cred, nil)
	roleRepo.EXPECT().GetRoleWithPermissions(gomock.Any(), credID, tenantID).Return(nil, nil, nil)
	tokenRepo.EXPECT().StoreRefreshToken(gomock.Any(), credID, gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

	output, err := uc.RefreshTokens(context.Background(), RefreshInput{
		RefreshToken: refreshToken,
		TenantID:     tenantID,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output.AccessToken == "" {
		t.Error("access token should not be empty")
	}
}

func TestRefreshTokens_CredentialNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	uc, credRepo, _, tokenRepo := newTokenUCWithMocks(t, ctrl)

	credID := uuid.New()
	tenantID := uuid.New()

	rtData := &repository.RefreshTokenData{
		CredentialID: credID.String(),
		TenantID:     tenantID.String(),
	}

	tokenRepo.EXPECT().FindRefreshTokenByHash(gomock.Any(), gomock.Any()).Return(rtData, credID.String(), "jti", nil)
	tokenRepo.EXPECT().DeleteRefreshToken(gomock.Any(), credID, "jti").Return(nil)
	credRepo.EXPECT().GetByID(gomock.Any(), credID).Return(nil, nil)

	_, err := uc.RefreshTokens(context.Background(), RefreshInput{
		RefreshToken: "some-token",
		TenantID:     tenantID,
	})
	assertAppErrorCode(t, err, 401)
}
