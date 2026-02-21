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
	"github.com/safrizal997/travapp-auth-service/internal/domain/repository"
	"github.com/safrizal997/travapp-auth-service/internal/mocks"
	"github.com/safrizal997/travapp-auth-service/internal/pkg/crypto"
	"github.com/safrizal997/travapp-auth-service/internal/testutil"
	"github.com/safrizal997/travapp-auth-service/internal/usecase"
	"go.uber.org/mock/gomock"
)

func newTokenHandlerWithMocks(t *testing.T, ctrl *gomock.Controller) (
	*TokenHandler,
	*mocks.MockCredentialRepository,
	*mocks.MockRoleRepository,
	*mocks.MockTokenRepository,
) {
	credRepo := mocks.NewMockCredentialRepository(ctrl)
	roleRepo := mocks.NewMockRoleRepository(ctrl)
	tokenRepo := mocks.NewMockTokenRepository(ctrl)

	logger := testutil.NewTestLogger()
	jwtMgr := testutil.NewTestJWTManager(t)

	tokenUC := usecase.NewTokenUseCase(
		credRepo, roleRepo, tokenRepo,
		jwtMgr, logger,
		15*time.Minute, 7*24*time.Hour,
	)

	handler := NewTokenHandler(tokenUC, jwtMgr, logger)
	return handler, credRepo, roleRepo, tokenRepo
}

func TestTokenHandler_RefreshToken_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	handler, credRepo, roleRepo, tokenRepo := newTokenHandlerWithMocks(t, ctrl)

	credID := uuid.New()
	tenantID := uuid.New()
	refreshToken := "test-refresh-token"
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

	body, _ := json.Marshal(dto.RefreshTokenRequest{
		RefreshToken: refreshToken,
		TenantID:     tenantID.String(),
	})

	r := gin.New()
	r.POST("/refresh", handler.RefreshToken)
	req := httptest.NewRequest(http.MethodPost, "/refresh", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestTokenHandler_RefreshToken_InvalidJSON(t *testing.T) {
	ctrl := gomock.NewController(t)
	handler, _, _, _ := newTokenHandlerWithMocks(t, ctrl)

	r := gin.New()
	r.POST("/refresh", handler.RefreshToken)
	req := httptest.NewRequest(http.MethodPost, "/refresh", bytes.NewReader([]byte("invalid")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestTokenHandler_RefreshToken_InvalidTenantID(t *testing.T) {
	ctrl := gomock.NewController(t)
	handler, _, _, _ := newTokenHandlerWithMocks(t, ctrl)

	body, _ := json.Marshal(map[string]string{
		"refresh_token": "some-token",
		"tenant_id":     "not-a-uuid",
	})

	r := gin.New()
	r.POST("/refresh", handler.RefreshToken)
	req := httptest.NewRequest(http.MethodPost, "/refresh", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestTokenHandler_RefreshToken_InvalidToken(t *testing.T) {
	ctrl := gomock.NewController(t)
	handler, _, _, tokenRepo := newTokenHandlerWithMocks(t, ctrl)

	tenantID := uuid.New()
	tokenRepo.EXPECT().FindRefreshTokenByHash(gomock.Any(), gomock.Any()).Return(nil, "", "", nil)

	body, _ := json.Marshal(dto.RefreshTokenRequest{
		RefreshToken: "invalid-token",
		TenantID:     tenantID.String(),
	})

	r := gin.New()
	r.POST("/refresh", handler.RefreshToken)
	req := httptest.NewRequest(http.MethodPost, "/refresh", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 401 {
		t.Errorf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
}

func TestTokenHandler_JWKS(t *testing.T) {
	ctrl := gomock.NewController(t)
	handler, _, _, _ := newTokenHandlerWithMocks(t, ctrl)

	r := gin.New()
	r.GET("/jwks", handler.JWKS)
	req := httptest.NewRequest(http.MethodGet, "/jwks", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var result map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &result)
	keys, ok := result["keys"].([]interface{})
	if !ok || len(keys) == 0 {
		t.Error("expected keys array in JWKS response")
	}
}
