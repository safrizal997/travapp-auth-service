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
	"github.com/safrizal997/travapp-auth-service/internal/pkg/apperror"
	"github.com/safrizal997/travapp-auth-service/internal/testutil"
	"github.com/safrizal997/travapp-auth-service/internal/usecase"
	"go.uber.org/mock/gomock"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func newAuthHandlerWithMocks(t *testing.T, ctrl *gomock.Controller) (
	*AuthHandler,
	*mocks.MockCredentialRepository,
	*mocks.MockTenantRepository,
	*mocks.MockRoleRepository,
	*mocks.MockTokenRepository,
	*mocks.MockLoginHistoryRepository,
	*mocks.MockEmailVerificationStore,
) {
	credRepo := mocks.NewMockCredentialRepository(ctrl)
	tenantRepo := mocks.NewMockTenantRepository(ctrl)
	roleRepo := mocks.NewMockRoleRepository(ctrl)
	tokenRepo := mocks.NewMockTokenRepository(ctrl)
	loginHistRepo := mocks.NewMockLoginHistoryRepository(ctrl)
	evStore := mocks.NewMockEmailVerificationStore(ctrl)

	logger := testutil.NewTestLogger()
	hasher := testutil.NewTestHasher()
	jwtMgr := testutil.NewTestJWTManager(t)

	loginUC := usecase.NewLoginUseCase(
		credRepo, tenantRepo, roleRepo, tokenRepo, loginHistRepo,
		hasher, jwtMgr, logger,
		15*time.Minute, 7*24*time.Hour,
		5, 15*time.Minute,
	)

	registerUC := usecase.NewRegisterUseCase(
		credRepo, tenantRepo, roleRepo,
		hasher, logger, evStore,
		"http://localhost:8080",
	)

	handler := NewAuthHandler(loginUC, registerUC, logger)
	return handler, credRepo, tenantRepo, roleRepo, tokenRepo, loginHistRepo, evStore
}

func TestAuthHandler_Login_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	handler, credRepo, tenantRepo, roleRepo, tokenRepo, loginHistRepo, _ := newAuthHandlerWithMocks(t, ctrl)

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

	body, _ := json.Marshal(dto.LoginRequest{
		Email:    "test@example.com",
		Password: "password123",
		TenantID: tenantID.String(),
	})

	r := gin.New()
	r.POST("/login", handler.Login)
	req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp dto.TokenResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.AccessToken == "" {
		t.Error("access_token should not be empty")
	}
}

func TestAuthHandler_Login_InvalidJSON(t *testing.T) {
	ctrl := gomock.NewController(t)
	handler, _, _, _, _, _, _ := newAuthHandlerWithMocks(t, ctrl)

	r := gin.New()
	r.POST("/login", handler.Login)
	req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader([]byte("invalid")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestAuthHandler_Login_MissingFields(t *testing.T) {
	ctrl := gomock.NewController(t)
	handler, _, _, _, _, _, _ := newAuthHandlerWithMocks(t, ctrl)

	body, _ := json.Marshal(map[string]string{"email": "test@example.com"})

	r := gin.New()
	r.POST("/login", handler.Login)
	req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestAuthHandler_Login_InvalidTenantID(t *testing.T) {
	ctrl := gomock.NewController(t)
	handler, _, _, _, _, _, _ := newAuthHandlerWithMocks(t, ctrl)

	body, _ := json.Marshal(map[string]string{
		"email":     "test@example.com",
		"password":  "password123",
		"tenant_id": "not-a-uuid",
	})

	r := gin.New()
	r.POST("/login", handler.Login)
	req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestAuthHandler_Login_WrongPassword(t *testing.T) {
	ctrl := gomock.NewController(t)
	handler, credRepo, tenantRepo, _, tokenRepo, loginHistRepo, _ := newAuthHandlerWithMocks(t, ctrl)

	tenantID := uuid.New()
	credID := uuid.New()

	hasher := testutil.NewTestHasher()
	pwHash, _ := hasher.Hash("correctPassword")

	cred := &entity.AuthCredential{
		ID:           credID,
		TenantID:     tenantID,
		Email:        "test@example.com",
		PasswordHash: &pwHash,
		Provider:     "email",
	}
	tenant := &entity.AuthTenant{ID: tenantID, IsActive: true}

	tenantRepo.EXPECT().GetActiveTenant(gomock.Any(), tenantID).Return(tenant, nil)
	tokenRepo.EXPECT().IncrementRateLimit(gomock.Any(), gomock.Any(), gomock.Any()).Return(int64(1), nil)
	credRepo.EXPECT().GetByEmailAndTenant(gomock.Any(), "test@example.com", tenantID, "email").Return(cred, nil)
	credRepo.EXPECT().UpdateFailedAttempts(gomock.Any(), credID, gomock.Any(), gomock.Any()).Return(nil)
	loginHistRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)

	body, _ := json.Marshal(dto.LoginRequest{
		Email:    "test@example.com",
		Password: "wrongPassword",
		TenantID: tenantID.String(),
	})

	r := gin.New()
	r.POST("/login", handler.Login)
	req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 401 {
		t.Errorf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAuthHandler_Register_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	handler, credRepo, tenantRepo, roleRepo, _, _, evStore := newAuthHandlerWithMocks(t, ctrl)

	tenantID := uuid.New()
	tenant := &entity.AuthTenant{ID: tenantID, IsActive: true}

	tenantRepo.EXPECT().GetActiveTenant(gomock.Any(), tenantID).Return(tenant, nil)
	credRepo.EXPECT().GetByEmailAndTenant(gomock.Any(), "new@example.com", tenantID, "email").Return(nil, nil)
	credRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
	roleRepo.EXPECT().GetDefaultRole(gomock.Any(), tenantID).Return(nil, nil)
	evStore.EXPECT().CreateEmailVerification(gomock.Any(), gomock.Any()).Return(nil)

	body, _ := json.Marshal(dto.RegisterRequest{
		Email:                "new@example.com",
		Password:             "password123",
		PasswordConfirmation: "password123",
		TenantID:             tenantID.String(),
	})

	r := gin.New()
	r.POST("/register", handler.Register)
	req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 201 {
		t.Errorf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAuthHandler_Register_Conflict(t *testing.T) {
	ctrl := gomock.NewController(t)
	handler, credRepo, tenantRepo, _, _, _, _ := newAuthHandlerWithMocks(t, ctrl)

	tenantID := uuid.New()
	tenant := &entity.AuthTenant{ID: tenantID, IsActive: true}
	existing := &entity.AuthCredential{ID: uuid.New()}

	tenantRepo.EXPECT().GetActiveTenant(gomock.Any(), tenantID).Return(tenant, nil)
	credRepo.EXPECT().GetByEmailAndTenant(gomock.Any(), "existing@example.com", tenantID, "email").Return(existing, nil)

	body, _ := json.Marshal(dto.RegisterRequest{
		Email:                "existing@example.com",
		Password:             "password123",
		PasswordConfirmation: "password123",
		TenantID:             tenantID.String(),
	})

	r := gin.New()
	r.POST("/register", handler.Register)
	req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 409 {
		t.Errorf("expected 409, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAuthHandler_Register_InvalidJSON(t *testing.T) {
	ctrl := gomock.NewController(t)
	handler, _, _, _, _, _, _ := newAuthHandlerWithMocks(t, ctrl)

	r := gin.New()
	r.POST("/register", handler.Register)
	req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewReader([]byte("invalid")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandleError_AppError(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("request_id", "req-123")

	err := apperror.NewConflict("conflict")
	handleError(c, err)

	if w.Code != 409 {
		t.Errorf("expected 409, got %d", w.Code)
	}
}

func TestHandleError_GenericError(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	handleError(c, apperror.NewInternal(nil))

	if w.Code != 500 {
		t.Errorf("expected 500, got %d", w.Code)
	}
}
