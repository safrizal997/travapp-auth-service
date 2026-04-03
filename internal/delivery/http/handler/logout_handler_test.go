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
	"github.com/safrizal997/travapp-auth-service/internal/mocks"
	"github.com/safrizal997/travapp-auth-service/internal/testutil"
	"github.com/safrizal997/travapp-auth-service/internal/usecase"
	"go.uber.org/mock/gomock"
)

func newLogoutHandlerWithMocks(ctrl *gomock.Controller) (
	*LogoutHandler,
	*mocks.MockTokenRepository,
	*mocks.MockLoginHistoryRepository,
) {
	tokenRepo := mocks.NewMockTokenRepository(ctrl)
	loginHistRepo := mocks.NewMockLoginHistoryRepository(ctrl)

	logger := testutil.NewTestLogger()
	logoutUC := usecase.NewLogoutUseCase(tokenRepo, loginHistRepo, logger)

	handler := NewLogoutHandler(logoutUC, logger)
	return handler, tokenRepo, loginHistRepo
}

func setupLogoutContext(c *gin.Context, credID, tenantID uuid.UUID) {
	c.Set("credential_id", credID.String())
	c.Set("tenant_id", tenantID.String())
	c.Set("jti", "test-jti")
	c.Set("token_exp", time.Now().Add(15*time.Minute))
}

func TestLogoutHandler_Logout_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	handler, tokenRepo, loginHistRepo := newLogoutHandlerWithMocks(ctrl)

	credID := uuid.New()
	tenantID := uuid.New()

	tokenRepo.EXPECT().BlacklistAccessToken(gomock.Any(), "test-jti", gomock.Any()).Return(nil)
	loginHistRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)

	body, _ := json.Marshal(dto.LogoutRequest{})

	r := gin.New()
	r.POST("/logout", func(c *gin.Context) {
		setupLogoutContext(c, credID, tenantID)
		handler.Logout(c)
	})

	req := httptest.NewRequest(http.MethodPost, "/logout", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestLogoutHandler_Logout_InvalidCredential(t *testing.T) {
	ctrl := gomock.NewController(t)
	handler, _, _ := newLogoutHandlerWithMocks(ctrl)

	r := gin.New()
	r.POST("/logout", func(c *gin.Context) {
		c.Set("credential_id", "invalid-uuid")
		c.Set("tenant_id", uuid.New().String())
		handler.Logout(c)
	})

	req := httptest.NewRequest(http.MethodPost, "/logout", bytes.NewReader([]byte("{}")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 401 {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestLogoutHandler_Logout_InvalidTenant(t *testing.T) {
	ctrl := gomock.NewController(t)
	handler, _, _ := newLogoutHandlerWithMocks(ctrl)

	r := gin.New()
	r.POST("/logout", func(c *gin.Context) {
		c.Set("credential_id", uuid.New().String())
		c.Set("tenant_id", "invalid-uuid")
		handler.Logout(c)
	})

	req := httptest.NewRequest(http.MethodPost, "/logout", bytes.NewReader([]byte("{}")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 401 {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestLogoutHandler_LogoutAll_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	handler, tokenRepo, loginHistRepo := newLogoutHandlerWithMocks(ctrl)

	credID := uuid.New()
	tenantID := uuid.New()

	tokenRepo.EXPECT().DeleteAllRefreshTokens(gomock.Any(), credID).Return(nil)
	tokenRepo.EXPECT().BlacklistAccessToken(gomock.Any(), "test-jti", gomock.Any()).Return(nil)
	loginHistRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)

	r := gin.New()
	r.POST("/logout/all", func(c *gin.Context) {
		setupLogoutContext(c, credID, tenantID)
		handler.LogoutAll(c)
	})

	req := httptest.NewRequest(http.MethodPost, "/logout/all", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestLogoutHandler_LogoutAll_InvalidCredential(t *testing.T) {
	ctrl := gomock.NewController(t)
	handler, _, _ := newLogoutHandlerWithMocks(ctrl)

	r := gin.New()
	r.POST("/logout/all", func(c *gin.Context) {
		c.Set("credential_id", "invalid")
		handler.LogoutAll(c)
	})

	req := httptest.NewRequest(http.MethodPost, "/logout/all", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 401 {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestLogoutHandler_LogoutAll_InvalidTenant(t *testing.T) {
	ctrl := gomock.NewController(t)
	handler, _, _ := newLogoutHandlerWithMocks(ctrl)

	r := gin.New()
	r.POST("/logout/all", func(c *gin.Context) {
		c.Set("credential_id", uuid.New().String())
		c.Set("tenant_id", "invalid")
		handler.LogoutAll(c)
	})

	req := httptest.NewRequest(http.MethodPost, "/logout/all", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 401 {
		t.Errorf("expected 401, got %d", w.Code)
	}
}
