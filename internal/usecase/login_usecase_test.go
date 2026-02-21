package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/safrizal997/travapp-auth-service/internal/domain/entity"
	"github.com/safrizal997/travapp-auth-service/internal/mocks"
	"github.com/safrizal997/travapp-auth-service/internal/pkg/apperror"
	"github.com/safrizal997/travapp-auth-service/internal/testutil"
	"go.uber.org/mock/gomock"
)

func newLoginUCWithMocks(t *testing.T, ctrl *gomock.Controller) (
	*LoginUseCase,
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

	uc := NewLoginUseCase(
		credRepo, tenantRepo, roleRepo, tokenRepo, loginHistRepo,
		testutil.NewTestHasher(),
		testutil.NewTestJWTManager(t),
		testutil.NewTestLogger(),
		15*time.Minute, 7*24*time.Hour,
		5, 15*time.Minute,
	)

	return uc, credRepo, tenantRepo, roleRepo, tokenRepo, loginHistRepo
}

func makeLoginInput(tenantID uuid.UUID) LoginInput {
	return LoginInput{
		Email:     "test@example.com",
		Password:  "password123",
		TenantID:  tenantID,
		IPAddress: "127.0.0.1",
		UserAgent: "test-agent",
	}
}

func assertAppErrorCode(t *testing.T, err error, expectedCode int) {
	t.Helper()
	var appErr *apperror.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected *apperror.AppError, got %T: %v", err, err)
	}
	if appErr.Code != expectedCode {
		t.Errorf("expected code %d, got %d", expectedCode, appErr.Code)
	}
}

func TestLoginUseCase_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	uc, credRepo, tenantRepo, roleRepo, tokenRepo, loginHistRepo := newLoginUCWithMocks(t, ctrl)

	tenantID := uuid.New()
	credID := uuid.New()

	hasher := testutil.NewTestHasher()
	pwHash, _ := hasher.Hash("password123")

	cred := &entity.AuthCredential{
		ID:              credID,
		TenantID:        tenantID,
		Email:           "test@example.com",
		PasswordHash:    &pwHash,
		Provider:        "email",
		IsActive:        true,
		IsEmailVerified: true,
	}

	tenant := &entity.AuthTenant{ID: tenantID, IsActive: true}
	role := &entity.AuthRole{Code: "traveler"}

	tenantRepo.EXPECT().GetActiveTenant(gomock.Any(), tenantID).Return(tenant, nil)
	tokenRepo.EXPECT().IncrementRateLimit(gomock.Any(), gomock.Any(), gomock.Any()).Return(int64(1), nil)
	credRepo.EXPECT().GetByEmailAndTenant(gomock.Any(), "test@example.com", tenantID, "email").Return(cred, nil)
	credRepo.EXPECT().ResetFailedAttempts(gomock.Any(), credID).Return(nil)
	credRepo.EXPECT().UpdateLastLogin(gomock.Any(), credID).Return(nil)
	roleRepo.EXPECT().GetRoleWithPermissions(gomock.Any(), credID, tenantID).Return(role, []string{"read"}, nil)
	tokenRepo.EXPECT().StoreRefreshToken(gomock.Any(), credID, gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	loginHistRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)

	input := makeLoginInput(tenantID)
	output, err := uc.Execute(context.Background(), input)
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
	if output.ExpiresIn != 900 {
		t.Errorf("expected 900, got %d", output.ExpiresIn)
	}
}

func TestLoginUseCase_InvalidTenant(t *testing.T) {
	ctrl := gomock.NewController(t)
	uc, _, tenantRepo, _, _, _ := newLoginUCWithMocks(t, ctrl)

	tenantID := uuid.New()
	tenantRepo.EXPECT().GetActiveTenant(gomock.Any(), tenantID).Return(nil, nil)

	_, err := uc.Execute(context.Background(), makeLoginInput(tenantID))
	assertAppErrorCode(t, err, 400)
}

func TestLoginUseCase_RateLimitExceeded(t *testing.T) {
	ctrl := gomock.NewController(t)
	uc, _, tenantRepo, _, tokenRepo, _ := newLoginUCWithMocks(t, ctrl)

	tenantID := uuid.New()
	tenant := &entity.AuthTenant{ID: tenantID, IsActive: true}

	tenantRepo.EXPECT().GetActiveTenant(gomock.Any(), tenantID).Return(tenant, nil)
	tokenRepo.EXPECT().IncrementRateLimit(gomock.Any(), gomock.Any(), gomock.Any()).Return(int64(6), nil)

	_, err := uc.Execute(context.Background(), makeLoginInput(tenantID))
	assertAppErrorCode(t, err, 429)
}

func TestLoginUseCase_RateLimitError(t *testing.T) {
	ctrl := gomock.NewController(t)
	uc, _, tenantRepo, _, tokenRepo, _ := newLoginUCWithMocks(t, ctrl)

	tenantID := uuid.New()
	tenant := &entity.AuthTenant{ID: tenantID, IsActive: true}

	tenantRepo.EXPECT().GetActiveTenant(gomock.Any(), tenantID).Return(tenant, nil)
	tokenRepo.EXPECT().IncrementRateLimit(gomock.Any(), gomock.Any(), gomock.Any()).Return(int64(0), errors.New("redis error"))

	_, err := uc.Execute(context.Background(), makeLoginInput(tenantID))
	assertAppErrorCode(t, err, 429)
}

func TestLoginUseCase_CredentialNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	uc, credRepo, tenantRepo, _, tokenRepo, loginHistRepo := newLoginUCWithMocks(t, ctrl)

	tenantID := uuid.New()
	tenant := &entity.AuthTenant{ID: tenantID, IsActive: true}

	tenantRepo.EXPECT().GetActiveTenant(gomock.Any(), tenantID).Return(tenant, nil)
	tokenRepo.EXPECT().IncrementRateLimit(gomock.Any(), gomock.Any(), gomock.Any()).Return(int64(1), nil)
	credRepo.EXPECT().GetByEmailAndTenant(gomock.Any(), "test@example.com", tenantID, "email").Return(nil, nil)
	loginHistRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)

	_, err := uc.Execute(context.Background(), makeLoginInput(tenantID))
	assertAppErrorCode(t, err, 401)
}

func TestLoginUseCase_AccountLocked(t *testing.T) {
	ctrl := gomock.NewController(t)
	uc, credRepo, tenantRepo, _, tokenRepo, _ := newLoginUCWithMocks(t, ctrl)

	tenantID := uuid.New()
	tenant := &entity.AuthTenant{ID: tenantID, IsActive: true}
	lockTime := time.Now().Add(30 * time.Minute)
	pwHash := "some-hash"
	cred := &entity.AuthCredential{
		ID:           uuid.New(),
		TenantID:     tenantID,
		Email:        "test@example.com",
		PasswordHash: &pwHash,
		Provider:     "email",
		LockedUntil:  &lockTime,
	}

	tenantRepo.EXPECT().GetActiveTenant(gomock.Any(), tenantID).Return(tenant, nil)
	tokenRepo.EXPECT().IncrementRateLimit(gomock.Any(), gomock.Any(), gomock.Any()).Return(int64(1), nil)
	credRepo.EXPECT().GetByEmailAndTenant(gomock.Any(), "test@example.com", tenantID, "email").Return(cred, nil)

	_, err := uc.Execute(context.Background(), makeLoginInput(tenantID))
	assertAppErrorCode(t, err, 423)
}

func TestLoginUseCase_WrongPassword(t *testing.T) {
	ctrl := gomock.NewController(t)
	uc, credRepo, tenantRepo, _, tokenRepo, loginHistRepo := newLoginUCWithMocks(t, ctrl)

	tenantID := uuid.New()
	credID := uuid.New()
	tenant := &entity.AuthTenant{ID: tenantID, IsActive: true}

	hasher := testutil.NewTestHasher()
	pwHash, _ := hasher.Hash("correctPassword")
	cred := &entity.AuthCredential{
		ID:           credID,
		TenantID:     tenantID,
		Email:        "test@example.com",
		PasswordHash: &pwHash,
		Provider:     "email",
	}

	tenantRepo.EXPECT().GetActiveTenant(gomock.Any(), tenantID).Return(tenant, nil)
	tokenRepo.EXPECT().IncrementRateLimit(gomock.Any(), gomock.Any(), gomock.Any()).Return(int64(1), nil)
	credRepo.EXPECT().GetByEmailAndTenant(gomock.Any(), "test@example.com", tenantID, "email").Return(cred, nil)
	credRepo.EXPECT().UpdateFailedAttempts(gomock.Any(), credID, 1, gomock.Any()).Return(nil)
	loginHistRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)

	_, err := uc.Execute(context.Background(), makeLoginInput(tenantID))
	assertAppErrorCode(t, err, 401)
}

func TestLoginUseCase_WrongPasswordLockout(t *testing.T) {
	ctrl := gomock.NewController(t)
	uc, credRepo, tenantRepo, _, tokenRepo, loginHistRepo := newLoginUCWithMocks(t, ctrl)

	tenantID := uuid.New()
	credID := uuid.New()
	tenant := &entity.AuthTenant{ID: tenantID, IsActive: true}

	hasher := testutil.NewTestHasher()
	pwHash, _ := hasher.Hash("correctPassword")
	cred := &entity.AuthCredential{
		ID:                  credID,
		TenantID:            tenantID,
		Email:               "test@example.com",
		PasswordHash:        &pwHash,
		Provider:            "email",
		FailedLoginAttempts: 4,
	}

	tenantRepo.EXPECT().GetActiveTenant(gomock.Any(), tenantID).Return(tenant, nil)
	tokenRepo.EXPECT().IncrementRateLimit(gomock.Any(), gomock.Any(), gomock.Any()).Return(int64(1), nil)
	credRepo.EXPECT().GetByEmailAndTenant(gomock.Any(), "test@example.com", tenantID, "email").Return(cred, nil)
	credRepo.EXPECT().UpdateFailedAttempts(gomock.Any(), credID, 5, gomock.Not(gomock.Nil())).Return(nil)
	loginHistRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)

	_, err := uc.Execute(context.Background(), makeLoginInput(tenantID))
	assertAppErrorCode(t, err, 401)
}

func TestLoginUseCase_Inactive(t *testing.T) {
	ctrl := gomock.NewController(t)
	uc, credRepo, tenantRepo, _, tokenRepo, _ := newLoginUCWithMocks(t, ctrl)

	tenantID := uuid.New()
	tenant := &entity.AuthTenant{ID: tenantID, IsActive: true}

	hasher := testutil.NewTestHasher()
	pwHash, _ := hasher.Hash("password123")
	cred := &entity.AuthCredential{
		ID:           uuid.New(),
		TenantID:     tenantID,
		Email:        "test@example.com",
		PasswordHash: &pwHash,
		Provider:     "email",
		IsActive:     false,
	}

	tenantRepo.EXPECT().GetActiveTenant(gomock.Any(), tenantID).Return(tenant, nil)
	tokenRepo.EXPECT().IncrementRateLimit(gomock.Any(), gomock.Any(), gomock.Any()).Return(int64(1), nil)
	credRepo.EXPECT().GetByEmailAndTenant(gomock.Any(), "test@example.com", tenantID, "email").Return(cred, nil)

	_, err := uc.Execute(context.Background(), makeLoginInput(tenantID))
	assertAppErrorCode(t, err, 401)
}

func TestLoginUseCase_EmailNotVerified(t *testing.T) {
	ctrl := gomock.NewController(t)
	uc, credRepo, tenantRepo, _, tokenRepo, _ := newLoginUCWithMocks(t, ctrl)

	tenantID := uuid.New()
	tenant := &entity.AuthTenant{ID: tenantID, IsActive: true}

	hasher := testutil.NewTestHasher()
	pwHash, _ := hasher.Hash("password123")
	cred := &entity.AuthCredential{
		ID:              uuid.New(),
		TenantID:        tenantID,
		Email:           "test@example.com",
		PasswordHash:    &pwHash,
		Provider:        "email",
		IsActive:        true,
		IsEmailVerified: false,
	}

	tenantRepo.EXPECT().GetActiveTenant(gomock.Any(), tenantID).Return(tenant, nil)
	tokenRepo.EXPECT().IncrementRateLimit(gomock.Any(), gomock.Any(), gomock.Any()).Return(int64(1), nil)
	credRepo.EXPECT().GetByEmailAndTenant(gomock.Any(), "test@example.com", tenantID, "email").Return(cred, nil)

	_, err := uc.Execute(context.Background(), makeLoginInput(tenantID))
	assertAppErrorCode(t, err, 401)
}

func TestLoginUseCase_NilPasswordHash(t *testing.T) {
	ctrl := gomock.NewController(t)
	uc, credRepo, tenantRepo, _, tokenRepo, _ := newLoginUCWithMocks(t, ctrl)

	tenantID := uuid.New()
	tenant := &entity.AuthTenant{ID: tenantID, IsActive: true}
	cred := &entity.AuthCredential{
		ID:           uuid.New(),
		TenantID:     tenantID,
		Email:        "test@example.com",
		PasswordHash: nil,
		Provider:     "email",
	}

	tenantRepo.EXPECT().GetActiveTenant(gomock.Any(), tenantID).Return(tenant, nil)
	tokenRepo.EXPECT().IncrementRateLimit(gomock.Any(), gomock.Any(), gomock.Any()).Return(int64(1), nil)
	credRepo.EXPECT().GetByEmailAndTenant(gomock.Any(), "test@example.com", tenantID, "email").Return(cred, nil)

	_, err := uc.Execute(context.Background(), makeLoginInput(tenantID))
	assertAppErrorCode(t, err, 401)
}

func TestLoginUseCase_TenantRepoError(t *testing.T) {
	ctrl := gomock.NewController(t)
	uc, _, tenantRepo, _, _, _ := newLoginUCWithMocks(t, ctrl)

	tenantID := uuid.New()
	tenantRepo.EXPECT().GetActiveTenant(gomock.Any(), tenantID).Return(nil, errors.New("db error"))

	_, err := uc.Execute(context.Background(), makeLoginInput(tenantID))
	assertAppErrorCode(t, err, 500)
}

func TestLoginUseCase_RoleRepoError(t *testing.T) {
	ctrl := gomock.NewController(t)
	uc, credRepo, tenantRepo, roleRepo, tokenRepo, _ := newLoginUCWithMocks(t, ctrl)

	tenantID := uuid.New()
	credID := uuid.New()
	tenant := &entity.AuthTenant{ID: tenantID, IsActive: true}

	hasher := testutil.NewTestHasher()
	pwHash, _ := hasher.Hash("password123")
	cred := &entity.AuthCredential{
		ID:              credID,
		TenantID:        tenantID,
		Email:           "test@example.com",
		PasswordHash:    &pwHash,
		Provider:        "email",
		IsActive:        true,
		IsEmailVerified: true,
	}

	tenantRepo.EXPECT().GetActiveTenant(gomock.Any(), tenantID).Return(tenant, nil)
	tokenRepo.EXPECT().IncrementRateLimit(gomock.Any(), gomock.Any(), gomock.Any()).Return(int64(1), nil)
	credRepo.EXPECT().GetByEmailAndTenant(gomock.Any(), "test@example.com", tenantID, "email").Return(cred, nil)
	credRepo.EXPECT().ResetFailedAttempts(gomock.Any(), credID).Return(nil)
	credRepo.EXPECT().UpdateLastLogin(gomock.Any(), credID).Return(nil)
	roleRepo.EXPECT().GetRoleWithPermissions(gomock.Any(), credID, tenantID).Return(nil, nil, errors.New("role error"))

	_, err := uc.Execute(context.Background(), makeLoginInput(tenantID))
	assertAppErrorCode(t, err, 500)
}

func TestLoginUseCase_StoreTokenError(t *testing.T) {
	ctrl := gomock.NewController(t)
	uc, credRepo, tenantRepo, roleRepo, tokenRepo, _ := newLoginUCWithMocks(t, ctrl)

	tenantID := uuid.New()
	credID := uuid.New()
	tenant := &entity.AuthTenant{ID: tenantID, IsActive: true}

	hasher := testutil.NewTestHasher()
	pwHash, _ := hasher.Hash("password123")
	cred := &entity.AuthCredential{
		ID:              credID,
		TenantID:        tenantID,
		Email:           "test@example.com",
		PasswordHash:    &pwHash,
		Provider:        "email",
		IsActive:        true,
		IsEmailVerified: true,
	}

	role := &entity.AuthRole{Code: "traveler"}

	tenantRepo.EXPECT().GetActiveTenant(gomock.Any(), tenantID).Return(tenant, nil)
	tokenRepo.EXPECT().IncrementRateLimit(gomock.Any(), gomock.Any(), gomock.Any()).Return(int64(1), nil)
	credRepo.EXPECT().GetByEmailAndTenant(gomock.Any(), "test@example.com", tenantID, "email").Return(cred, nil)
	credRepo.EXPECT().ResetFailedAttempts(gomock.Any(), credID).Return(nil)
	credRepo.EXPECT().UpdateLastLogin(gomock.Any(), credID).Return(nil)
	roleRepo.EXPECT().GetRoleWithPermissions(gomock.Any(), credID, tenantID).Return(role, []string{"read"}, nil)
	tokenRepo.EXPECT().StoreRefreshToken(gomock.Any(), credID, gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("redis error"))

	_, err := uc.Execute(context.Background(), makeLoginInput(tenantID))
	assertAppErrorCode(t, err, 500)
}

func TestLoginUseCase_NoRole(t *testing.T) {
	ctrl := gomock.NewController(t)
	uc, credRepo, tenantRepo, roleRepo, tokenRepo, loginHistRepo := newLoginUCWithMocks(t, ctrl)

	tenantID := uuid.New()
	credID := uuid.New()

	hasher := testutil.NewTestHasher()
	pwHash, _ := hasher.Hash("password123")

	cred := &entity.AuthCredential{
		ID:              credID,
		TenantID:        tenantID,
		Email:           "test@example.com",
		PasswordHash:    &pwHash,
		Provider:        "email",
		IsActive:        true,
		IsEmailVerified: true,
	}

	tenant := &entity.AuthTenant{ID: tenantID, IsActive: true}

	tenantRepo.EXPECT().GetActiveTenant(gomock.Any(), tenantID).Return(tenant, nil)
	tokenRepo.EXPECT().IncrementRateLimit(gomock.Any(), gomock.Any(), gomock.Any()).Return(int64(1), nil)
	credRepo.EXPECT().GetByEmailAndTenant(gomock.Any(), "test@example.com", tenantID, "email").Return(cred, nil)
	credRepo.EXPECT().ResetFailedAttempts(gomock.Any(), credID).Return(nil)
	credRepo.EXPECT().UpdateLastLogin(gomock.Any(), credID).Return(nil)
	roleRepo.EXPECT().GetRoleWithPermissions(gomock.Any(), credID, tenantID).Return(nil, nil, nil)
	tokenRepo.EXPECT().StoreRefreshToken(gomock.Any(), credID, gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	loginHistRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)

	output, err := uc.Execute(context.Background(), makeLoginInput(tenantID))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output.AccessToken == "" {
		t.Error("access token should not be empty")
	}
}

func TestLoginUseCase_CredRepoError(t *testing.T) {
	ctrl := gomock.NewController(t)
	uc, credRepo, tenantRepo, _, tokenRepo, _ := newLoginUCWithMocks(t, ctrl)

	tenantID := uuid.New()
	tenant := &entity.AuthTenant{ID: tenantID, IsActive: true}

	tenantRepo.EXPECT().GetActiveTenant(gomock.Any(), tenantID).Return(tenant, nil)
	tokenRepo.EXPECT().IncrementRateLimit(gomock.Any(), gomock.Any(), gomock.Any()).Return(int64(1), nil)
	credRepo.EXPECT().GetByEmailAndTenant(gomock.Any(), "test@example.com", tenantID, "email").Return(nil, errors.New("db error"))

	_, err := uc.Execute(context.Background(), makeLoginInput(tenantID))
	assertAppErrorCode(t, err, 500)
}
