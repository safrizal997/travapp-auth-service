package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/safrizal997/travapp-auth-service/internal/delivery/http/dto"
	jwtpkg "github.com/safrizal997/travapp-auth-service/internal/pkg/jwt"
	"github.com/safrizal997/travapp-auth-service/internal/usecase"
	"go.uber.org/zap"
)

type TokenHandler struct {
	tokenUC    *usecase.TokenUseCase
	jwtManager *jwtpkg.Manager
	logger     *zap.Logger
}

func NewTokenHandler(tokenUC *usecase.TokenUseCase, jwtManager *jwtpkg.Manager, logger *zap.Logger) *TokenHandler {
	return &TokenHandler{
		tokenUC:    tokenUC,
		jwtManager: jwtManager,
		logger:     logger,
	}
}

// RefreshToken exchanges a refresh token for new access and refresh tokens.
// @Summary Refresh access token
// @Description Exchange a valid refresh token for a new pair of access and refresh tokens
// @Tags Token
// @Accept json
// @Produce json
// @Param request body dto.RefreshTokenRequest true "Refresh token request"
// @Success 200 {object} dto.TokenResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/v1/auth/token/refresh [post]
func (h *TokenHandler) RefreshToken(c *gin.Context) {
	var req dto.RefreshTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Error: "Invalid request: " + err.Error(),
			Code:  http.StatusBadRequest,
		})
		return
	}

	tenantID, err := uuid.Parse(req.TenantID)
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Error: "Invalid tenant_id",
			Code:  http.StatusBadRequest,
		})
		return
	}

	input := usecase.RefreshInput{
		RefreshToken: req.RefreshToken,
		TenantID:     tenantID,
	}

	output, err := h.tokenUC.RefreshTokens(c.Request.Context(), input)
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

// JWKS returns the JSON Web Key Set for token verification.
// @Summary Get JSON Web Key Set
// @Description Returns the public keys used to verify JWT tokens in JWKS format
// @Tags Token
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/auth/.well-known/jwks.json [get]
func (h *TokenHandler) JWKS(c *gin.Context) {
	jwks := h.jwtManager.GetJWKS()
	c.JSON(http.StatusOK, jwks)
}
