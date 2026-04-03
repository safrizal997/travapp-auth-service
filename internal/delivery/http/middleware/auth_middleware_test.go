package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/safrizal997/travapp-auth-service/internal/mocks"
	"github.com/safrizal997/travapp-auth-service/internal/testutil"
	"go.uber.org/mock/gomock"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func setupAuthRouter(t *testing.T, ctrl *gomock.Controller) (*gin.Engine, *mocks.MockTokenRepository) {
	jwtMgr := testutil.NewTestJWTManager(t)
	tokenRepo := mocks.NewMockTokenRepository(ctrl)

	r := gin.New()
	r.Use(AuthMiddleware(jwtMgr, tokenRepo))
	r.GET("/protected", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"credential_id": GetCredentialID(c),
			"tenant_id":     GetTenantID(c),
			"jti":           GetJTI(c),
		})
	})

	return r, tokenRepo
}

func TestAuthMiddleware_ValidToken(t *testing.T) {
	ctrl := gomock.NewController(t)
	r, tokenRepo := setupAuthRouter(t, ctrl)

	jwtMgr := testutil.NewTestJWTManager(t)
	credID := uuid.New()
	tenantID := uuid.New()

	// Generate a token with the same manager the middleware uses
	// Need to rebuild with same manager
	tokenRepo2 := mocks.NewMockTokenRepository(ctrl)
	r2 := gin.New()
	r2.Use(AuthMiddleware(jwtMgr, tokenRepo2))
	r2.GET("/protected", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"credential_id": GetCredentialID(c),
			"tenant_id":     GetTenantID(c),
		})
	})

	token, jti, _ := jwtMgr.GenerateAccessToken(credID, tenantID, "admin", []string{"read"}, "email", 15*time.Minute)
	tokenRepo2.EXPECT().IsAccessTokenBlacklisted(gomock.Any(), jti).Return(false, nil)

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	r2.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}

	_ = r
	_ = tokenRepo
}

func TestAuthMiddleware_MissingHeader(t *testing.T) {
	ctrl := gomock.NewController(t)
	r, _ := setupAuthRouter(t, ctrl)

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != 401 {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestAuthMiddleware_InvalidToken(t *testing.T) {
	ctrl := gomock.NewController(t)
	r, _ := setupAuthRouter(t, ctrl)

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != 401 {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestAuthMiddleware_BlacklistedToken(t *testing.T) {
	ctrl := gomock.NewController(t)

	jwtMgr := testutil.NewTestJWTManager(t)
	tokenRepo := mocks.NewMockTokenRepository(ctrl)

	r := gin.New()
	r.Use(AuthMiddleware(jwtMgr, tokenRepo))
	r.GET("/protected", func(c *gin.Context) {
		c.JSON(200, nil)
	})

	credID := uuid.New()
	tenantID := uuid.New()
	token, jti, _ := jwtMgr.GenerateAccessToken(credID, tenantID, "admin", nil, "email", 15*time.Minute)

	tokenRepo.EXPECT().IsAccessTokenBlacklisted(gomock.Any(), jti).Return(true, nil)

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != 401 {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestAuthMiddleware_RawToken(t *testing.T) {
	ctrl := gomock.NewController(t)

	jwtMgr := testutil.NewTestJWTManager(t)
	tokenRepo := mocks.NewMockTokenRepository(ctrl)

	r := gin.New()
	r.Use(AuthMiddleware(jwtMgr, tokenRepo))
	r.GET("/protected", func(c *gin.Context) {
		c.JSON(200, nil)
	})

	credID := uuid.New()
	tenantID := uuid.New()
	token, jti, _ := jwtMgr.GenerateAccessToken(credID, tenantID, "admin", nil, "email", 15*time.Minute)

	tokenRepo.EXPECT().IsAccessTokenBlacklisted(gomock.Any(), jti).Return(false, nil)

	// Send without "Bearer " prefix
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", token)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestGetTokenExpiry_NoValue(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	expiry := GetTokenExpiry(c)
	if time.Until(expiry) > 2*time.Second {
		t.Error("expected expiry close to now when not set")
	}
}

func TestGetTokenExpiry_WithValue(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	expected := time.Now().Add(15 * time.Minute)
	c.Set("token_exp", expected)

	expiry := GetTokenExpiry(c)
	if expiry.Sub(expected) > time.Second {
		t.Error("expected expiry to match set value")
	}
}

func TestGetCredentialID(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("credential_id", "test-cred-id")

	if got := GetCredentialID(c); got != "test-cred-id" {
		t.Errorf("expected test-cred-id, got %s", got)
	}
}

func TestGetTenantID(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("tenant_id", "test-tenant-id")

	if got := GetTenantID(c); got != "test-tenant-id" {
		t.Errorf("expected test-tenant-id, got %s", got)
	}
}

func TestGetJTI(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("jti", "test-jti")

	if got := GetJTI(c); got != "test-jti" {
		t.Errorf("expected test-jti, got %s", got)
	}
}
