package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/safrizal997/travapp-auth-service/internal/domain/repository"
	"github.com/safrizal997/travapp-auth-service/internal/mocks"
	"github.com/safrizal997/travapp-auth-service/internal/testutil"
	"go.uber.org/mock/gomock"
)

func newLogoutUCWithMocks(ctrl *gomock.Controller) (
	*LogoutUseCase,
	*mocks.MockTokenRepository,
	*mocks.MockLoginHistoryRepository,
) {
	tokenRepo := mocks.NewMockTokenRepository(ctrl)
	loginHistRepo := mocks.NewMockLoginHistoryRepository(ctrl)

	uc := NewLogoutUseCase(tokenRepo, loginHistRepo, testutil.NewTestLogger())
	return uc, tokenRepo, loginHistRepo
}

func TestLogout_WithRefreshToken(t *testing.T) {
	ctrl := gomock.NewController(t)
	uc, tokenRepo, loginHistRepo := newLogoutUCWithMocks(ctrl)

	credID := uuid.New()
	tenantID := uuid.New()

	rtData := &repository.RefreshTokenData{
		CredentialID: credID.String(),
		TenantID:     tenantID.String(),
	}

	tokenRepo.EXPECT().FindRefreshTokenByHash(gomock.Any(), gomock.Any()).Return(rtData, credID.String(), "jti-123", nil)
	tokenRepo.EXPECT().DeleteRefreshToken(gomock.Any(), credID, "jti-123").Return(nil)
	tokenRepo.EXPECT().BlacklistAccessToken(gomock.Any(), "access-jti", gomock.Any()).Return(nil)
	loginHistRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)

	err := uc.Logout(context.Background(), LogoutInput{
		CredentialID: credID,
		TenantID:     tenantID,
		RefreshToken: "some-refresh-token",
		AccessJTI:    "access-jti",
		AccessExpiry: time.Now().Add(15 * time.Minute),
		IPAddress:    "127.0.0.1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLogout_WithoutRefreshToken(t *testing.T) {
	ctrl := gomock.NewController(t)
	uc, tokenRepo, loginHistRepo := newLogoutUCWithMocks(ctrl)

	credID := uuid.New()
	tenantID := uuid.New()

	tokenRepo.EXPECT().BlacklistAccessToken(gomock.Any(), "access-jti", gomock.Any()).Return(nil)
	loginHistRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)

	err := uc.Logout(context.Background(), LogoutInput{
		CredentialID: credID,
		TenantID:     tenantID,
		RefreshToken: "",
		AccessJTI:    "access-jti",
		AccessExpiry: time.Now().Add(15 * time.Minute),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLogout_ExpiredAccessToken(t *testing.T) {
	ctrl := gomock.NewController(t)
	uc, _, loginHistRepo := newLogoutUCWithMocks(ctrl)

	credID := uuid.New()
	tenantID := uuid.New()

	// No blacklist call expected since token is already expired
	loginHistRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)

	err := uc.Logout(context.Background(), LogoutInput{
		CredentialID: credID,
		TenantID:     tenantID,
		RefreshToken: "",
		AccessJTI:    "access-jti",
		AccessExpiry: time.Now().Add(-5 * time.Minute), // already expired
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLogoutAll(t *testing.T) {
	ctrl := gomock.NewController(t)
	uc, tokenRepo, loginHistRepo := newLogoutUCWithMocks(ctrl)

	credID := uuid.New()
	tenantID := uuid.New()

	tokenRepo.EXPECT().DeleteAllRefreshTokens(gomock.Any(), credID).Return(nil)
	tokenRepo.EXPECT().BlacklistAccessToken(gomock.Any(), "access-jti", gomock.Any()).Return(nil)
	loginHistRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)

	err := uc.LogoutAll(context.Background(), LogoutInput{
		CredentialID: credID,
		TenantID:     tenantID,
		AccessJTI:    "access-jti",
		AccessExpiry: time.Now().Add(15 * time.Minute),
		IPAddress:    "127.0.0.1",
		UserAgent:    "test-agent",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLogoutAll_DeleteError(t *testing.T) {
	ctrl := gomock.NewController(t)
	uc, tokenRepo, loginHistRepo := newLogoutUCWithMocks(ctrl)

	credID := uuid.New()
	tenantID := uuid.New()

	tokenRepo.EXPECT().DeleteAllRefreshTokens(gomock.Any(), credID).Return(errors.New("redis error"))
	tokenRepo.EXPECT().BlacklistAccessToken(gomock.Any(), "access-jti", gomock.Any()).Return(nil)
	loginHistRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)

	err := uc.LogoutAll(context.Background(), LogoutInput{
		CredentialID: credID,
		TenantID:     tenantID,
		AccessJTI:    "access-jti",
		AccessExpiry: time.Now().Add(15 * time.Minute),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLogoutAll_BlacklistError(t *testing.T) {
	ctrl := gomock.NewController(t)
	uc, tokenRepo, loginHistRepo := newLogoutUCWithMocks(ctrl)

	credID := uuid.New()
	tenantID := uuid.New()

	tokenRepo.EXPECT().DeleteAllRefreshTokens(gomock.Any(), credID).Return(nil)
	tokenRepo.EXPECT().BlacklistAccessToken(gomock.Any(), "access-jti", gomock.Any()).Return(errors.New("redis error"))
	loginHistRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)

	err := uc.LogoutAll(context.Background(), LogoutInput{
		CredentialID: credID,
		TenantID:     tenantID,
		AccessJTI:    "access-jti",
		AccessExpiry: time.Now().Add(15 * time.Minute),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLogout_FindRefreshTokenError(t *testing.T) {
	ctrl := gomock.NewController(t)
	uc, tokenRepo, loginHistRepo := newLogoutUCWithMocks(ctrl)

	credID := uuid.New()
	tenantID := uuid.New()

	tokenRepo.EXPECT().FindRefreshTokenByHash(gomock.Any(), gomock.Any()).Return(nil, "", "", errors.New("redis error"))
	tokenRepo.EXPECT().BlacklistAccessToken(gomock.Any(), "access-jti", gomock.Any()).Return(nil)
	loginHistRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)

	err := uc.Logout(context.Background(), LogoutInput{
		CredentialID: credID,
		TenantID:     tenantID,
		RefreshToken: "some-token",
		AccessJTI:    "access-jti",
		AccessExpiry: time.Now().Add(15 * time.Minute),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLogout_WithUserAgent(t *testing.T) {
	ctrl := gomock.NewController(t)
	uc, tokenRepo, loginHistRepo := newLogoutUCWithMocks(ctrl)

	credID := uuid.New()
	tenantID := uuid.New()

	tokenRepo.EXPECT().BlacklistAccessToken(gomock.Any(), "access-jti", gomock.Any()).Return(nil)
	loginHistRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)

	err := uc.Logout(context.Background(), LogoutInput{
		CredentialID: credID,
		TenantID:     tenantID,
		AccessJTI:    "access-jti",
		AccessExpiry: time.Now().Add(15 * time.Minute),
		UserAgent:    "Mozilla/5.0",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLogoutAll_ExpiredAccessToken(t *testing.T) {
	ctrl := gomock.NewController(t)
	uc, tokenRepo, loginHistRepo := newLogoutUCWithMocks(ctrl)

	credID := uuid.New()
	tenantID := uuid.New()

	tokenRepo.EXPECT().DeleteAllRefreshTokens(gomock.Any(), credID).Return(nil)
	loginHistRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)

	err := uc.LogoutAll(context.Background(), LogoutInput{
		CredentialID: credID,
		TenantID:     tenantID,
		AccessJTI:    "access-jti",
		AccessExpiry: time.Now().Add(-5 * time.Minute),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
