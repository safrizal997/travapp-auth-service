package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/safrizal997/travapp-auth-service/internal/domain/entity"
	"github.com/safrizal997/travapp-auth-service/internal/mocks"
	"github.com/safrizal997/travapp-auth-service/internal/testutil"
	"go.uber.org/mock/gomock"
)

func newPasswordUCWithMocks(t *testing.T, ctrl *gomock.Controller) (
	*PasswordUseCase,
	*mocks.MockCredentialRepository,
	*mocks.MockTenantRepository,
	*mocks.MockTokenRepository,
	*mocks.MockPasswordResetStore,
) {
	credRepo := mocks.NewMockCredentialRepository(ctrl)
	tenantRepo := mocks.NewMockTenantRepository(ctrl)
	tokenRepo := mocks.NewMockTokenRepository(ctrl)
	prStore := mocks.NewMockPasswordResetStore(ctrl)

	uc := NewPasswordUseCase(
		credRepo, tenantRepo, tokenRepo,
		testutil.NewTestHasher(),
		testutil.NewTestLogger(),
		prStore,
		"http://localhost:8080",
	)

	return uc, credRepo, tenantRepo, tokenRepo, prStore
}

// --- ChangePassword tests ---

func TestChangePassword_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	uc, credRepo, _, tokenRepo, _ := newPasswordUCWithMocks(t, ctrl)

	credID := uuid.New()
	hasher := testutil.NewTestHasher()
	pwHash, _ := hasher.Hash("currentPassword")
	cred := &entity.AuthCredential{
		ID:           credID,
		PasswordHash: &pwHash,
		Provider:     "email",
	}

	credRepo.EXPECT().GetByID(gomock.Any(), credID).Return(cred, nil)
	credRepo.EXPECT().UpdatePassword(gomock.Any(), credID, gomock.Any()).Return(nil)
	tokenRepo.EXPECT().DeleteAllRefreshTokens(gomock.Any(), credID).Return(nil)

	err := uc.ChangePassword(context.Background(), ChangePasswordInput{
		CredentialID:    credID,
		CurrentPassword: "currentPassword",
		NewPassword:     "newPassword123",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestChangePassword_WrongCurrent(t *testing.T) {
	ctrl := gomock.NewController(t)
	uc, credRepo, _, _, _ := newPasswordUCWithMocks(t, ctrl)

	credID := uuid.New()
	hasher := testutil.NewTestHasher()
	pwHash, _ := hasher.Hash("currentPassword")
	cred := &entity.AuthCredential{
		ID:           credID,
		PasswordHash: &pwHash,
	}

	credRepo.EXPECT().GetByID(gomock.Any(), credID).Return(cred, nil)

	err := uc.ChangePassword(context.Background(), ChangePasswordInput{
		CredentialID:    credID,
		CurrentPassword: "wrongPassword",
		NewPassword:     "newPassword123",
	})
	assertAppErrorCode(t, err, 401)
}

func TestChangePassword_OAuthAccount(t *testing.T) {
	ctrl := gomock.NewController(t)
	uc, credRepo, _, _, _ := newPasswordUCWithMocks(t, ctrl)

	credID := uuid.New()
	cred := &entity.AuthCredential{
		ID:           credID,
		PasswordHash: nil,
		Provider:     "google",
	}

	credRepo.EXPECT().GetByID(gomock.Any(), credID).Return(cred, nil)

	err := uc.ChangePassword(context.Background(), ChangePasswordInput{
		CredentialID:    credID,
		CurrentPassword: "anything",
		NewPassword:     "newPassword123",
	})
	assertAppErrorCode(t, err, 400)
}

func TestChangePassword_CredNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	uc, credRepo, _, _, _ := newPasswordUCWithMocks(t, ctrl)

	credID := uuid.New()
	credRepo.EXPECT().GetByID(gomock.Any(), credID).Return(nil, nil)

	err := uc.ChangePassword(context.Background(), ChangePasswordInput{
		CredentialID:    credID,
		CurrentPassword: "anything",
	})
	assertAppErrorCode(t, err, 404)
}

func TestChangePassword_RepoError(t *testing.T) {
	ctrl := gomock.NewController(t)
	uc, credRepo, _, _, _ := newPasswordUCWithMocks(t, ctrl)

	credID := uuid.New()
	credRepo.EXPECT().GetByID(gomock.Any(), credID).Return(nil, errors.New("db error"))

	err := uc.ChangePassword(context.Background(), ChangePasswordInput{
		CredentialID: credID,
	})
	assertAppErrorCode(t, err, 500)
}

// --- ForgotPassword tests ---

func TestForgotPassword_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	uc, credRepo, _, tokenRepo, prStore := newPasswordUCWithMocks(t, ctrl)

	tenantID := uuid.New()
	credID := uuid.New()
	cred := &entity.AuthCredential{ID: credID, Email: "user@example.com"}

	tokenRepo.EXPECT().GetRateLimit(gomock.Any(), gomock.Any()).Return(int64(0), nil)
	tokenRepo.EXPECT().IncrementRateLimit(gomock.Any(), gomock.Any(), gomock.Any()).Return(int64(1), nil)
	credRepo.EXPECT().GetByEmailAndTenant(gomock.Any(), "user@example.com", tenantID, "email").Return(cred, nil)
	prStore.EXPECT().CreatePasswordReset(gomock.Any(), gomock.Any()).Return(nil)

	output, err := uc.ForgotPassword(context.Background(), ForgotPasswordInput{
		Email:    "user@example.com",
		TenantID: tenantID,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output.ResetToken == "" {
		t.Error("reset token should not be empty")
	}
}

func TestForgotPassword_RateLimited(t *testing.T) {
	ctrl := gomock.NewController(t)
	uc, _, _, tokenRepo, _ := newPasswordUCWithMocks(t, ctrl)

	tenantID := uuid.New()
	tokenRepo.EXPECT().GetRateLimit(gomock.Any(), gomock.Any()).Return(int64(3), nil)

	_, err := uc.ForgotPassword(context.Background(), ForgotPasswordInput{
		Email:    "user@example.com",
		TenantID: tenantID,
	})
	assertAppErrorCode(t, err, 429)
}

func TestForgotPassword_UserNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	uc, credRepo, _, tokenRepo, _ := newPasswordUCWithMocks(t, ctrl)

	tenantID := uuid.New()
	tokenRepo.EXPECT().GetRateLimit(gomock.Any(), gomock.Any()).Return(int64(0), nil)
	tokenRepo.EXPECT().IncrementRateLimit(gomock.Any(), gomock.Any(), gomock.Any()).Return(int64(1), nil)
	credRepo.EXPECT().GetByEmailAndTenant(gomock.Any(), "unknown@example.com", tenantID, "email").Return(nil, nil)

	output, err := uc.ForgotPassword(context.Background(), ForgotPasswordInput{
		Email:    "unknown@example.com",
		TenantID: tenantID,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output.ResetToken != "" {
		t.Error("should return empty token for unknown user")
	}
}

// --- ResetPassword tests ---

func TestResetPassword_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	uc, credRepo, _, tokenRepo, prStore := newPasswordUCWithMocks(t, ctrl)

	credID := uuid.New()
	resetID := uuid.New()
	reset := &entity.AuthPasswordReset{
		ID:           resetID,
		CredentialID: credID,
		ExpiresAt:    time.Now().Add(1 * time.Hour),
		UsedAt:       nil,
	}

	prStore.EXPECT().GetPasswordResetByToken(gomock.Any(), gomock.Any()).Return(reset, nil)
	credRepo.EXPECT().UpdatePassword(gomock.Any(), credID, gomock.Any()).Return(nil)
	prStore.EXPECT().MarkPasswordResetUsed(gomock.Any(), resetID).Return(nil)
	tokenRepo.EXPECT().DeleteAllRefreshTokens(gomock.Any(), credID).Return(nil)

	err := uc.ResetPassword(context.Background(), ResetPasswordInput{
		Token:       "valid-token",
		Email:       "user@example.com",
		NewPassword: "newPassword123",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResetPassword_InvalidToken(t *testing.T) {
	ctrl := gomock.NewController(t)
	uc, _, _, _, prStore := newPasswordUCWithMocks(t, ctrl)

	prStore.EXPECT().GetPasswordResetByToken(gomock.Any(), gomock.Any()).Return(nil, nil)

	err := uc.ResetPassword(context.Background(), ResetPasswordInput{
		Token:       "invalid-token",
		Email:       "user@example.com",
		NewPassword: "newPassword123",
	})
	assertAppErrorCode(t, err, 400)
}

func TestResetPassword_ExpiredToken(t *testing.T) {
	ctrl := gomock.NewController(t)
	uc, _, _, _, prStore := newPasswordUCWithMocks(t, ctrl)

	reset := &entity.AuthPasswordReset{
		ID:        uuid.New(),
		ExpiresAt: time.Now().Add(-1 * time.Hour),
		UsedAt:    nil,
	}

	prStore.EXPECT().GetPasswordResetByToken(gomock.Any(), gomock.Any()).Return(reset, nil)

	err := uc.ResetPassword(context.Background(), ResetPasswordInput{
		Token:       "expired-token",
		NewPassword: "newPassword123",
	})
	assertAppErrorCode(t, err, 400)
}

func TestResetPassword_UsedToken(t *testing.T) {
	ctrl := gomock.NewController(t)
	uc, _, _, _, prStore := newPasswordUCWithMocks(t, ctrl)

	usedAt := time.Now().Add(-30 * time.Minute)
	reset := &entity.AuthPasswordReset{
		ID:        uuid.New(),
		ExpiresAt: time.Now().Add(1 * time.Hour),
		UsedAt:    &usedAt,
	}

	prStore.EXPECT().GetPasswordResetByToken(gomock.Any(), gomock.Any()).Return(reset, nil)

	err := uc.ResetPassword(context.Background(), ResetPasswordInput{
		Token:       "used-token",
		NewPassword: "newPassword123",
	})
	assertAppErrorCode(t, err, 400)
}

// --- VerifyEmail tests ---

func TestVerifyEmail_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	uc, credRepo, _, _, prStore := newPasswordUCWithMocks(t, ctrl)

	credID := uuid.New()
	evID := uuid.New()
	ev := &entity.AuthEmailVerification{
		ID:           evID,
		CredentialID: credID,
		ExpiresAt:    time.Now().Add(24 * time.Hour),
		VerifiedAt:   nil,
	}

	prStore.EXPECT().GetEmailVerificationByToken(gomock.Any(), gomock.Any()).Return(ev, nil)
	credRepo.EXPECT().VerifyEmail(gomock.Any(), credID).Return(nil)
	prStore.EXPECT().MarkEmailVerified(gomock.Any(), evID).Return(nil)

	err := uc.VerifyEmail(context.Background(), VerifyEmailInput{
		Token: "valid-token",
		Email: "user@example.com",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestVerifyEmail_InvalidToken(t *testing.T) {
	ctrl := gomock.NewController(t)
	uc, _, _, _, prStore := newPasswordUCWithMocks(t, ctrl)

	prStore.EXPECT().GetEmailVerificationByToken(gomock.Any(), gomock.Any()).Return(nil, nil)

	err := uc.VerifyEmail(context.Background(), VerifyEmailInput{
		Token: "invalid-token",
		Email: "user@example.com",
	})
	assertAppErrorCode(t, err, 400)
}

func TestVerifyEmail_Expired(t *testing.T) {
	ctrl := gomock.NewController(t)
	uc, _, _, _, prStore := newPasswordUCWithMocks(t, ctrl)

	ev := &entity.AuthEmailVerification{
		ExpiresAt:  time.Now().Add(-1 * time.Hour),
		VerifiedAt: nil,
	}

	prStore.EXPECT().GetEmailVerificationByToken(gomock.Any(), gomock.Any()).Return(ev, nil)

	err := uc.VerifyEmail(context.Background(), VerifyEmailInput{
		Token: "expired-token",
		Email: "user@example.com",
	})
	assertAppErrorCode(t, err, 400)
}

func TestVerifyEmail_VerifyCredError(t *testing.T) {
	ctrl := gomock.NewController(t)
	uc, credRepo, _, _, prStore := newPasswordUCWithMocks(t, ctrl)

	credID := uuid.New()
	evID := uuid.New()
	ev := &entity.AuthEmailVerification{
		ID:           evID,
		CredentialID: credID,
		ExpiresAt:    time.Now().Add(24 * time.Hour),
		VerifiedAt:   nil,
	}

	prStore.EXPECT().GetEmailVerificationByToken(gomock.Any(), gomock.Any()).Return(ev, nil)
	credRepo.EXPECT().VerifyEmail(gomock.Any(), credID).Return(errors.New("db error"))

	err := uc.VerifyEmail(context.Background(), VerifyEmailInput{
		Token: "valid-token",
		Email: "user@example.com",
	})
	assertAppErrorCode(t, err, 500)
}

func TestResetPassword_UpdatePasswordError(t *testing.T) {
	ctrl := gomock.NewController(t)
	uc, credRepo, _, _, prStore := newPasswordUCWithMocks(t, ctrl)

	credID := uuid.New()
	reset := &entity.AuthPasswordReset{
		ID:           uuid.New(),
		CredentialID: credID,
		ExpiresAt:    time.Now().Add(1 * time.Hour),
		UsedAt:       nil,
	}

	prStore.EXPECT().GetPasswordResetByToken(gomock.Any(), gomock.Any()).Return(reset, nil)
	credRepo.EXPECT().UpdatePassword(gomock.Any(), credID, gomock.Any()).Return(errors.New("db error"))

	err := uc.ResetPassword(context.Background(), ResetPasswordInput{
		Token:       "valid-token",
		Email:       "user@example.com",
		NewPassword: "newPassword123",
	})
	assertAppErrorCode(t, err, 500)
}

func TestResetPassword_StoreError(t *testing.T) {
	ctrl := gomock.NewController(t)
	uc, _, _, _, prStore := newPasswordUCWithMocks(t, ctrl)

	prStore.EXPECT().GetPasswordResetByToken(gomock.Any(), gomock.Any()).Return(nil, errors.New("db error"))

	err := uc.ResetPassword(context.Background(), ResetPasswordInput{
		Token:       "any-token",
		NewPassword: "newPassword123",
	})
	assertAppErrorCode(t, err, 500)
}

func TestForgotPassword_CredRepoError(t *testing.T) {
	ctrl := gomock.NewController(t)
	uc, credRepo, _, tokenRepo, _ := newPasswordUCWithMocks(t, ctrl)

	tenantID := uuid.New()
	tokenRepo.EXPECT().GetRateLimit(gomock.Any(), gomock.Any()).Return(int64(0), nil)
	tokenRepo.EXPECT().IncrementRateLimit(gomock.Any(), gomock.Any(), gomock.Any()).Return(int64(1), nil)
	credRepo.EXPECT().GetByEmailAndTenant(gomock.Any(), "user@example.com", tenantID, "email").Return(nil, errors.New("db error"))

	_, err := uc.ForgotPassword(context.Background(), ForgotPasswordInput{
		Email:    "user@example.com",
		TenantID: tenantID,
	})
	assertAppErrorCode(t, err, 500)
}

func TestForgotPassword_CreateResetError(t *testing.T) {
	ctrl := gomock.NewController(t)
	uc, credRepo, _, tokenRepo, prStore := newPasswordUCWithMocks(t, ctrl)

	tenantID := uuid.New()
	credID := uuid.New()
	cred := &entity.AuthCredential{ID: credID, Email: "user@example.com"}

	tokenRepo.EXPECT().GetRateLimit(gomock.Any(), gomock.Any()).Return(int64(0), nil)
	tokenRepo.EXPECT().IncrementRateLimit(gomock.Any(), gomock.Any(), gomock.Any()).Return(int64(1), nil)
	credRepo.EXPECT().GetByEmailAndTenant(gomock.Any(), "user@example.com", tenantID, "email").Return(cred, nil)
	prStore.EXPECT().CreatePasswordReset(gomock.Any(), gomock.Any()).Return(errors.New("db error"))

	_, err := uc.ForgotPassword(context.Background(), ForgotPasswordInput{
		Email:    "user@example.com",
		TenantID: tenantID,
	})
	assertAppErrorCode(t, err, 500)
}

func TestChangePassword_UpdatePasswordError(t *testing.T) {
	ctrl := gomock.NewController(t)
	uc, credRepo, _, _, _ := newPasswordUCWithMocks(t, ctrl)

	credID := uuid.New()
	hasher := testutil.NewTestHasher()
	pwHash, _ := hasher.Hash("currentPassword")
	cred := &entity.AuthCredential{
		ID:           credID,
		PasswordHash: &pwHash,
	}

	credRepo.EXPECT().GetByID(gomock.Any(), credID).Return(cred, nil)
	credRepo.EXPECT().UpdatePassword(gomock.Any(), credID, gomock.Any()).Return(errors.New("db error"))

	err := uc.ChangePassword(context.Background(), ChangePasswordInput{
		CredentialID:    credID,
		CurrentPassword: "currentPassword",
		NewPassword:     "newPassword123",
	})
	assertAppErrorCode(t, err, 500)
}

func TestVerifyEmail_RepoError(t *testing.T) {
	ctrl := gomock.NewController(t)
	uc, _, _, _, prStore := newPasswordUCWithMocks(t, ctrl)

	prStore.EXPECT().GetEmailVerificationByToken(gomock.Any(), gomock.Any()).Return(nil, errors.New("db error"))

	err := uc.VerifyEmail(context.Background(), VerifyEmailInput{
		Token: "any-token",
		Email: "user@example.com",
	})
	assertAppErrorCode(t, err, 500)
}

func TestVerifyEmail_AlreadyVerified(t *testing.T) {
	ctrl := gomock.NewController(t)
	uc, _, _, _, prStore := newPasswordUCWithMocks(t, ctrl)

	verifiedAt := time.Now().Add(-1 * time.Hour)
	ev := &entity.AuthEmailVerification{
		ExpiresAt:  time.Now().Add(24 * time.Hour),
		VerifiedAt: &verifiedAt,
	}

	prStore.EXPECT().GetEmailVerificationByToken(gomock.Any(), gomock.Any()).Return(ev, nil)

	err := uc.VerifyEmail(context.Background(), VerifyEmailInput{
		Token: "already-verified-token",
		Email: "user@example.com",
	})
	assertAppErrorCode(t, err, 400)
}
