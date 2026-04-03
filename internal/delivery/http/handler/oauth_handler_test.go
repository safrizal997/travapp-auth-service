package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/safrizal997/travapp-auth-service/internal/domain/entity"
	"github.com/safrizal997/travapp-auth-service/internal/mocks"
	"github.com/safrizal997/travapp-auth-service/internal/pkg/oauth"
	"github.com/safrizal997/travapp-auth-service/internal/testutil"
	"github.com/safrizal997/travapp-auth-service/internal/usecase"
	"go.uber.org/mock/gomock"
)

func newOAuthHandlerWithMocks(t *testing.T, ctrl *gomock.Controller) (
	*OAuthHandler,
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

	logger := testutil.NewTestLogger()
	jwtMgr := testutil.NewTestJWTManager(t)

	googleCfg := &oauth.GoogleConfig{
		ClientID:     "test-client-id",
		ClientSecret: "test-secret",
		RedirectURI:  "http://localhost:8080/callback",
	}

	oauthUC := usecase.NewOAuthUseCase(
		credRepo, tenantRepo, roleRepo, tokenRepo, loginHistRepo,
		googleCfg, jwtMgr, logger,
		15*time.Minute, 7*24*time.Hour,
	)

	handler := NewOAuthHandler(oauthUC, logger)
	return handler, credRepo, tenantRepo, roleRepo, tokenRepo, loginHistRepo
}

func TestOAuthHandler_InitiateGoogle_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	handler, _, tenantRepo, _, tokenRepo, _ := newOAuthHandlerWithMocks(t, ctrl)

	tenantID := uuid.New()
	tenant := &entity.AuthTenant{ID: tenantID, IsActive: true}

	tenantRepo.EXPECT().GetActiveTenant(gomock.Any(), tenantID).Return(tenant, nil)
	tokenRepo.EXPECT().StoreOAuthState(gomock.Any(), gomock.Any(), gomock.Any(), 10*time.Minute).Return(nil)

	r := gin.New()
	r.GET("/oauth/google", handler.InitiateGoogle)
	req := httptest.NewRequest(http.MethodGet, "/oauth/google?tenant_id="+tenantID.String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 302 {
		t.Errorf("expected 302, got %d: %s", w.Code, w.Body.String())
	}
}

func TestOAuthHandler_InitiateGoogle_MissingTenantID(t *testing.T) {
	ctrl := gomock.NewController(t)
	handler, _, _, _, _, _ := newOAuthHandlerWithMocks(t, ctrl)

	r := gin.New()
	r.GET("/oauth/google", handler.InitiateGoogle)
	req := httptest.NewRequest(http.MethodGet, "/oauth/google", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestOAuthHandler_InitiateGoogle_InvalidTenantID(t *testing.T) {
	ctrl := gomock.NewController(t)
	handler, _, _, _, _, _ := newOAuthHandlerWithMocks(t, ctrl)

	r := gin.New()
	r.GET("/oauth/google", handler.InitiateGoogle)
	req := httptest.NewRequest(http.MethodGet, "/oauth/google?tenant_id=invalid", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestOAuthHandler_GoogleCallback_MissingParams(t *testing.T) {
	ctrl := gomock.NewController(t)
	handler, _, _, _, _, _ := newOAuthHandlerWithMocks(t, ctrl)

	tests := []struct {
		name string
		url  string
	}{
		{"missing both", "/callback"},
		{"missing code", "/callback?state=abc"},
		{"missing state", "/callback?code=abc"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := gin.New()
			r.GET("/callback", handler.GoogleCallback)
			req := httptest.NewRequest(http.MethodGet, tt.url, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != 400 {
				t.Errorf("expected 400, got %d", w.Code)
			}
		})
	}
}

func TestOAuthHandler_GoogleCallback_OAuthError(t *testing.T) {
	ctrl := gomock.NewController(t)
	handler, _, _, _, _, _ := newOAuthHandlerWithMocks(t, ctrl)

	r := gin.New()
	r.GET("/callback", handler.GoogleCallback)
	req := httptest.NewRequest(http.MethodGet, "/callback?error=access_denied", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Errorf("expected 400, got %d", w.Code)
	}
}
