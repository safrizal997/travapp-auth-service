package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/safrizal997/travapp-auth-service/internal/delivery/http/dto"
	"github.com/safrizal997/travapp-auth-service/internal/usecase"
	"go.uber.org/zap"
)

type OAuthHandler struct {
	oauthUC *usecase.OAuthUseCase
	logger  *zap.Logger
}

func NewOAuthHandler(oauthUC *usecase.OAuthUseCase, logger *zap.Logger) *OAuthHandler {
	return &OAuthHandler{
		oauthUC: oauthUC,
		logger:  logger,
	}
}

// InitiateGoogle redirects the user to Google OAuth2 consent screen.
// @Summary Initiate Google OAuth2 login
// @Description Redirects the user to Google's OAuth2 consent page for authentication
// @Tags OAuth
// @Param tenant_id query string true "Tenant ID (UUID format)"
// @Success 302 {string} string "Redirect to Google OAuth2"
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/v1/auth/oauth/google [get]
func (h *OAuthHandler) InitiateGoogle(c *gin.Context) {
	tenantIDStr := c.Query("tenant_id")
	if tenantIDStr == "" {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Error: "tenant_id query parameter is required",
			Code:  http.StatusBadRequest,
		})
		return
	}

	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Error: "Invalid tenant_id",
			Code:  http.StatusBadRequest,
		})
		return
	}

	output, err := h.oauthUC.Initiate(c.Request.Context(), tenantID)
	if err != nil {
		handleError(c, err)
		return
	}

	c.Redirect(http.StatusFound, output.RedirectURL)
}

// GoogleCallback handles the OAuth2 callback from Google.
// @Summary Google OAuth2 callback
// @Description Handles the callback from Google OAuth2, exchanges code for tokens
// @Tags OAuth
// @Produce json
// @Param code query string true "Authorization code from Google"
// @Param state query string true "OAuth2 state parameter"
// @Success 200 {object} dto.TokenResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/v1/auth/oauth/google/callback [get]
func (h *OAuthHandler) GoogleCallback(c *gin.Context) {
	code := c.Query("code")
	state := c.Query("state")

	if code == "" || state == "" {
		errMsg := c.Query("error")
		if errMsg != "" {
			c.JSON(http.StatusBadRequest, dto.ErrorResponse{
				Error: "OAuth error: " + errMsg,
				Code:  http.StatusBadRequest,
			})
			return
		}
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Error: "Missing code or state parameter",
			Code:  http.StatusBadRequest,
		})
		return
	}

	input := usecase.OAuthCallbackInput{
		Code:      code,
		State:     state,
		IPAddress: c.ClientIP(),
		UserAgent: c.GetHeader("User-Agent"),
	}

	output, err := h.oauthUC.Callback(c.Request.Context(), input)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.TokenResponse{
		AccessToken:  output.AccessToken,
		RefreshToken: output.RefreshToken,
		TokenType:    output.TokenType,
		ExpiresIn:    output.ExpiresIn,
	})
}
