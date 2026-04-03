package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/safrizal997/travapp-auth-service/internal/domain/entity"
	"github.com/safrizal997/travapp-auth-service/internal/mocks"
	"github.com/safrizal997/travapp-auth-service/internal/pkg/oauth"
	"github.com/safrizal997/travapp-auth-service/internal/testutil"
	"go.uber.org/mock/gomock"
)

func newOAuthUCWithMocks(t *testing.T, ctrl *gomock.Controller) (
	*OAuthUseCase,
	*mocks.MockCredentialRepository,
	*mocks.MockTenantRepository,
	*mocks.MockRoleRepository,
	*mocks.MockTokenRepository,
	*mocks.MockLoginHistoryRepository,
) {
	credRepo := mocks.NewMockCredentialRepository(ctrl)
	tenantRepo := mocks.NewMockTenantRepository(ctrl)
	roleRepo := mocks.NewMockRoleRepository(ctrl)
	tokenRepo := mocks.NewMockTokenRepository(ctrl)
	loginHistRepo := mocks.NewMockLoginHistoryRepository(ctrl)

	googleCfg := &oauth.GoogleConfig{
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		RedirectURI:  "http://localhost:8080/callback",
	}

	uc := NewOAuthUseCase(
		credRepo, tenantRepo, roleRepo, tokenRepo, loginHistRepo,
		googleCfg,
		testutil.NewTestJWTManager(t),
		testutil.NewTestLogger(),
		15*time.Minute, 7*24*time.Hour,
	)

	return uc, credRepo, tenantRepo, roleRepo, tokenRepo, loginHistRepo
}

func TestOAuth_Initiate_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	uc, _, tenantRepo, _, tokenRepo, _ := newOAuthUCWithMocks(t, ctrl)

	tenantID := uuid.New()
	tenant := &entity.AuthTenant{ID: tenantID, IsActive: true}

	tenantRepo.EXPECT().GetActiveTenant(gomock.Any(), tenantID).Return(tenant, nil)
	tokenRepo.EXPECT().StoreOAuthState(gomock.Any(), gomock.Any(), gomock.Any(), 10*time.Minute).Return(nil)

	output, err := uc.Initiate(context.Background(), tenantID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output.RedirectURL == "" {
		t.Error("redirect URL should not be empty")
	}
}

func TestOAuth_Initiate_InvalidTenant(t *testing.T) {
	ctrl := gomock.NewController(t)
	uc, _, tenantRepo, _, _, _ := newOAuthUCWithMocks(t, ctrl)

	tenantID := uuid.New()
	tenantRepo.EXPECT().GetActiveTenant(gomock.Any(), tenantID).Return(nil, nil)

	_, err := uc.Initiate(context.Background(), tenantID)
	assertAppErrorCode(t, err, 400)
}

func TestOAuth_Initiate_TenantError(t *testing.T) {
	ctrl := gomock.NewController(t)
	uc, _, tenantRepo, _, _, _ := newOAuthUCWithMocks(t, ctrl)

	tenantID := uuid.New()
	tenantRepo.EXPECT().GetActiveTenant(gomock.Any(), tenantID).Return(nil, errors.New("db error"))

	_, err := uc.Initiate(context.Background(), tenantID)
	assertAppErrorCode(t, err, 500)
}

func TestOAuth_Callback_InvalidState(t *testing.T) {
	ctrl := gomock.NewController(t)
	uc, _, _, _, tokenRepo, _ := newOAuthUCWithMocks(t, ctrl)

	tokenRepo.EXPECT().GetOAuthState(gomock.Any(), "invalid-state").Return(nil, nil)

	_, err := uc.Callback(context.Background(), OAuthCallbackInput{
		Code:  "some-code",
		State: "invalid-state",
	})
	assertAppErrorCode(t, err, 400)
}

func TestOAuth_Callback_StateError(t *testing.T) {
	ctrl := gomock.NewController(t)
	uc, _, _, _, tokenRepo, _ := newOAuthUCWithMocks(t, ctrl)

	tokenRepo.EXPECT().GetOAuthState(gomock.Any(), "state").Return(nil, errors.New("redis error"))

	_, err := uc.Callback(context.Background(), OAuthCallbackInput{
		Code:  "some-code",
		State: "state",
	})
	assertAppErrorCode(t, err, 500)
}

func TestOAuth_Callback_ExistingUser(t *testing.T) {
	// The Callback method calls ExchangeCode which makes an HTTP call to Google.
	// We can't easily mock that without refactoring, so we skip this test.
	t.Skip("Callback requires HTTP mock for Google token exchange")
}

func TestOAuth_Initiate_StoreStateError(t *testing.T) {
	ctrl := gomock.NewController(t)
	uc, _, tenantRepo, _, tokenRepo, _ := newOAuthUCWithMocks(t, ctrl)

	tenantID := uuid.New()
	tenant := &entity.AuthTenant{ID: tenantID, IsActive: true}

	tenantRepo.EXPECT().GetActiveTenant(gomock.Any(), tenantID).Return(tenant, nil)
	tokenRepo.EXPECT().StoreOAuthState(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("redis error"))

	_, err := uc.Initiate(context.Background(), tenantID)
	assertAppErrorCode(t, err, 500)
}
