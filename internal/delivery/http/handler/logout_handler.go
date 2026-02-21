package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/safrizal997/travapp-auth-service/internal/delivery/http/dto"
	"github.com/safrizal997/travapp-auth-service/internal/delivery/http/middleware"
	"github.com/safrizal997/travapp-auth-service/internal/usecase"
	"go.uber.org/zap"
)

type LogoutHandler struct {
	logoutUC *usecase.LogoutUseCase
	logger   *zap.Logger
}

func NewLogoutHandler(logoutUC *usecase.LogoutUseCase, logger *zap.Logger) *LogoutHandler {
	return &LogoutHandler{
		logoutUC: logoutUC,
		logger:   logger,
	}
}

// Logout invalidates the current session's tokens.
// @Summary Logout current session
// @Description Invalidates the current access token and optionally the refresh token
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body dto.LogoutRequest false "Logout request with optional refresh token"
// @Success 200 {object} dto.MessageResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Security BearerAuth
// @Router /api/v1/auth/logout [post]
func (h *LogoutHandler) Logout(c *gin.Context) {
	var req dto.LogoutRequest
	_ = c.ShouldBindJSON(&req)

	credID, err := uuid.Parse(middleware.GetCredentialID(c))
	if err != nil {
		c.JSON(http.StatusUnauthorized, dto.ErrorResponse{Error: "Invalid credential", Code: http.StatusUnauthorized})
		return
	}

	tenantID, err := uuid.Parse(middleware.GetTenantID(c))
	if err != nil {
		c.JSON(http.StatusUnauthorized, dto.ErrorResponse{Error: "Invalid tenant", Code: http.StatusUnauthorized})
		return
	}

	input := usecase.LogoutInput{
		CredentialID: credID,
		TenantID:     tenantID,
		RefreshToken: req.RefreshToken,
		AccessJTI:    middleware.GetJTI(c),
		AccessExpiry: middleware.GetTokenExpiry(c),
		IPAddress:    c.ClientIP(),
		UserAgent:    c.GetHeader("User-Agent"),
	}

	if err := h.logoutUC.Logout(c.Request.Context(), input); err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.MessageResponse{Message: "Logged out successfully"})
}

// LogoutAll invalidates all sessions for the current user.
// @Summary Logout all sessions
// @Description Invalidates all active sessions and tokens for the authenticated user
// @Tags Auth
// @Produce json
// @Success 200 {object} dto.MessageResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Security BearerAuth
// @Router /api/v1/auth/logout/all [post]
func (h *LogoutHandler) LogoutAll(c *gin.Context) {
	credID, err := uuid.Parse(middleware.GetCredentialID(c))
	if err != nil {
		c.JSON(http.StatusUnauthorized, dto.ErrorResponse{Error: "Invalid credential", Code: http.StatusUnauthorized})
		return
	}

	tenantID, err := uuid.Parse(middleware.GetTenantID(c))
	if err != nil {
		c.JSON(http.StatusUnauthorized, dto.ErrorResponse{Error: "Invalid tenant", Code: http.StatusUnauthorized})
		return
	}

	input := usecase.LogoutInput{
		CredentialID: credID,
		TenantID:     tenantID,
		AccessJTI:    middleware.GetJTI(c),
		AccessExpiry: middleware.GetTokenExpiry(c),
		IPAddress:    c.ClientIP(),
		UserAgent:    c.GetHeader("User-Agent"),
	}

	if err := h.logoutUC.LogoutAll(c.Request.Context(), input); err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.MessageResponse{Message: "All sessions logged out successfully"})
}
