package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/safrizal997/travapp-auth-service/internal/delivery/http/dto"
	"github.com/safrizal997/travapp-auth-service/internal/domain/entity"
	"github.com/safrizal997/travapp-auth-service/internal/mocks"
	"github.com/safrizal997/travapp-auth-service/internal/testutil"
	"github.com/safrizal997/travapp-auth-service/internal/usecase"
	"go.uber.org/mock/gomock"
)

func newPasswordHandlerWithMocks(t *testing.T, ctrl *gomock.Controller) (
	*PasswordHandler,
	*mocks.MockCredentialRepository,
	*mocks.MockTenantRepository,
	*mocks.MockTokenRepository,
	*mocks.MockPasswordResetStore,
) {
	credRepo := mocks.NewMockCredentialRepository(ctrl)
	tenantRepo := mocks.NewMockTenantRepository(ctrl)
	tokenRepo := mocks.NewMockTokenRepository(ctrl)
	prStore := mocks.NewMockPasswordResetStore(ctrl)

	logger := testutil.NewTestLogger()
	hasher := testutil.NewTestHasher()

	passwordUC := usecase.NewPasswordUseCase(
		credRepo, tenantRepo, tokenRepo,
		hasher, logger, prStore,
		"http://localhost:8080",
	)

	handler := NewPasswordHandler(passwordUC, logger)
	return handler, credRepo, tenantRepo, tokenRepo, prStore
}

func TestPasswordHandler_ChangePassword_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	handler, credRepo, _, tokenRepo, _ := newPasswordHandlerWithMocks(t, ctrl)

	credID := uuid.New()
	hasher := testutil.NewTestHasher()
	pwHash, _ := hasher.Hash("currentPassword")

	cred := &entity.AuthCredential{
		ID:           credID,
		PasswordHash: &pwHash,
	}

	credRepo.EXPECT().GetByID(gomock.Any(), credID).Return(cred, nil)
	credRepo.EXPECT().UpdatePassword(gomock.Any(), credID, gomock.Any()).Return(nil)
	tokenRepo.EXPECT().DeleteAllRefreshTokens(gomock.Any(), credID).Return(nil)

	body, _ := json.Marshal(dto.ChangePasswordRequest{
		CurrentPassword:         "currentPassword",
		NewPassword:             "newPassword123",
		NewPasswordConfirmation: "newPassword123",
	})

	r := gin.New()
	r.POST("/change", func(c *gin.Context) {
		c.Set("credential_id", credID.String())
		handler.ChangePassword(c)
	})

	req := httptest.NewRequest(http.MethodPost, "/change", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPasswordHandler_ChangePassword_InvalidJSON(t *testing.T) {
	ctrl := gomock.NewController(t)
	handler, _, _, _, _ := newPasswordHandlerWithMocks(t, ctrl)

	r := gin.New()
	r.POST("/change", func(c *gin.Context) {
		c.Set("credential_id", uuid.New().String())
		handler.ChangePassword(c)
	})

	req := httptest.NewRequest(http.MethodPost, "/change", bytes.NewReader([]byte("invalid")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestPasswordHandler_ChangePassword_InvalidCredential(t *testing.T) {
	ctrl := gomock.NewController(t)
	handler, _, _, _, _ := newPasswordHandlerWithMocks(t, ctrl)

	body, _ := json.Marshal(dto.ChangePasswordRequest{
		CurrentPassword:         "currentPassword",
		NewPassword:             "newPassword123",
		NewPasswordConfirmation: "newPassword123",
	})

	r := gin.New()
	r.POST("/change", func(c *gin.Context) {
		c.Set("credential_id", "invalid-uuid")
		handler.ChangePassword(c)
	})

	req := httptest.NewRequest(http.MethodPost, "/change", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 401 {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestPasswordHandler_ForgotPassword_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	handler, credRepo, _, tokenRepo, prStore := newPasswordHandlerWithMocks(t, ctrl)

	tenantID := uuid.New()
	credID := uuid.New()
	cred := &entity.AuthCredential{ID: credID, Email: "user@example.com"}

	tokenRepo.EXPECT().GetRateLimit(gomock.Any(), gomock.Any()).Return(int64(0), nil)
	tokenRepo.EXPECT().IncrementRateLimit(gomock.Any(), gomock.Any(), gomock.Any()).Return(int64(1), nil)
	credRepo.EXPECT().GetByEmailAndTenant(gomock.Any(), "user@example.com", tenantID, "email").Return(cred, nil)
	prStore.EXPECT().CreatePasswordReset(gomock.Any(), gomock.Any()).Return(nil)

	body, _ := json.Marshal(dto.ForgotPasswordRequest{
		Email:    "user@example.com",
		TenantID: tenantID.String(),
	})

	r := gin.New()
	r.POST("/forgot", handler.ForgotPassword)
	req := httptest.NewRequest(http.MethodPost, "/forgot", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPasswordHandler_ForgotPassword_InvalidJSON(t *testing.T) {
	ctrl := gomock.NewController(t)
	handler, _, _, _, _ := newPasswordHandlerWithMocks(t, ctrl)

	r := gin.New()
	r.POST("/forgot", handler.ForgotPassword)
	req := httptest.NewRequest(http.MethodPost, "/forgot", bytes.NewReader([]byte("invalid")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestPasswordHandler_ForgotPassword_InvalidTenantID(t *testing.T) {
	ctrl := gomock.NewController(t)
	handler, _, _, _, _ := newPasswordHandlerWithMocks(t, ctrl)

	body, _ := json.Marshal(map[string]string{
		"email":     "user@example.com",
		"tenant_id": "not-a-uuid",
	})

	r := gin.New()
	r.POST("/forgot", handler.ForgotPassword)
	req := httptest.NewRequest(http.MethodPost, "/forgot", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestPasswordHandler_ResetPassword_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	handler, credRepo, _, tokenRepo, prStore := newPasswordHandlerWithMocks(t, ctrl)

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

	body, _ := json.Marshal(dto.ResetPasswordRequest{
		Token:                   "valid-token",
		Email:                   "user@example.com",
		NewPassword:             "newPassword123",
		NewPasswordConfirmation: "newPassword123",
	})

	r := gin.New()
	r.POST("/reset", handler.ResetPassword)
	req := httptest.NewRequest(http.MethodPost, "/reset", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPasswordHandler_ResetPassword_InvalidJSON(t *testing.T) {
	ctrl := gomock.NewController(t)
	handler, _, _, _, _ := newPasswordHandlerWithMocks(t, ctrl)

	r := gin.New()
	r.POST("/reset", handler.ResetPassword)
	req := httptest.NewRequest(http.MethodPost, "/reset", bytes.NewReader([]byte("invalid")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestPasswordHandler_VerifyEmail_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	handler, credRepo, _, _, prStore := newPasswordHandlerWithMocks(t, ctrl)

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

	r := gin.New()
	r.GET("/verify-email", handler.VerifyEmail)
	req := httptest.NewRequest(http.MethodGet, "/verify-email?token=valid-token&email=user@example.com", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPasswordHandler_ResetPassword_InvalidToken(t *testing.T) {
	ctrl := gomock.NewController(t)
	handler, _, _, _, prStore := newPasswordHandlerWithMocks(t, ctrl)

	prStore.EXPECT().GetPasswordResetByToken(gomock.Any(), gomock.Any()).Return(nil, nil)

	body, _ := json.Marshal(dto.ResetPasswordRequest{
		Token:                   "invalid-token",
		Email:                   "user@example.com",
		NewPassword:             "newPassword123",
		NewPasswordConfirmation: "newPassword123",
	})

	r := gin.New()
	r.POST("/reset", handler.ResetPassword)
	req := httptest.NewRequest(http.MethodPost, "/reset", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPasswordHandler_ForgotPassword_RateLimited(t *testing.T) {
	ctrl := gomock.NewController(t)
	handler, _, _, tokenRepo, _ := newPasswordHandlerWithMocks(t, ctrl)

	tenantID := uuid.New()
	tokenRepo.EXPECT().GetRateLimit(gomock.Any(), gomock.Any()).Return(int64(5), nil)

	body, _ := json.Marshal(dto.ForgotPasswordRequest{
		Email:    "user@example.com",
		TenantID: tenantID.String(),
	})

	r := gin.New()
	r.POST("/forgot", handler.ForgotPassword)
	req := httptest.NewRequest(http.MethodPost, "/forgot", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 429 {
		t.Errorf("expected 429, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPasswordHandler_VerifyEmail_MissingParams(t *testing.T) {
	ctrl := gomock.NewController(t)
	handler, _, _, _, _ := newPasswordHandlerWithMocks(t, ctrl)

	tests := []struct {
		name string
		url  string
	}{
		{"missing both", "/verify-email"},
		{"missing token", "/verify-email?email=test@example.com"},
		{"missing email", "/verify-email?token=some-token"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := gin.New()
			r.GET("/verify-email", handler.VerifyEmail)
			req := httptest.NewRequest(http.MethodGet, tt.url, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != 400 {
				t.Errorf("expected 400, got %d", w.Code)
			}
		})
	}
}
