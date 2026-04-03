package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/safrizal997/travapp-auth-service/internal/delivery/http/dto"
	"github.com/safrizal997/travapp-auth-service/internal/pkg/apperror"
	"github.com/safrizal997/travapp-auth-service/internal/usecase"
	"go.uber.org/zap"
)

type AuthHandler struct {
	loginUC    *usecase.LoginUseCase
	registerUC *usecase.RegisterUseCase
	logger     *zap.Logger
}

func NewAuthHandler(loginUC *usecase.LoginUseCase, registerUC *usecase.RegisterUseCase, logger *zap.Logger) *AuthHandler {
	return &AuthHandler{
		loginUC:    loginUC,
		registerUC: registerUC,
		logger:     logger,
	}
}

// Login authenticates a user and returns JWT tokens.
// @Summary User login
// @Description Authenticate user with email and password, returns access and refresh tokens
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body dto.LoginRequest true "Login credentials"
// @Success 200 {object} dto.TokenResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/v1/auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var req dto.LoginRequest
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

	input := usecase.LoginInput{
		Email:     req.Email,
		Password:  req.Password,
		TenantID:  tenantID,
		IPAddress: c.ClientIP(),
		UserAgent: c.GetHeader("User-Agent"),
	}

	output, err := h.loginUC.Execute(c.Request.Context(), input)
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

// Register creates a new user account.
// @Summary User registration
// @Description Register a new user account with email and password
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body dto.RegisterRequest true "Registration details"
// @Success 201 {object} dto.RegisterResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 409 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/v1/auth/register [post]
func (h *AuthHandler) Register(c *gin.Context) {
	var req dto.RegisterRequest
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

	input := usecase.RegisterInput{
		Email:                req.Email,
		Password:             req.Password,
		PasswordConfirmation: req.PasswordConfirmation,
		TenantID:             tenantID,
	}

	output, err := h.registerUC.Execute(c.Request.Context(), input)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusCreated, dto.RegisterResponse{
		Message:      "Registration successful. Please verify your email.",
		CredentialID: output.CredentialID.String(),
	})
}

func handleError(c *gin.Context, err error) {
	if appErr, ok := err.(*apperror.AppError); ok {
		reqID, _ := c.Get("request_id")
		reqIDStr, _ := reqID.(string)
		c.JSON(appErr.Code, dto.ErrorResponse{
			Error: appErr.Message,
			Code:  appErr.Code,
			ReqID: reqIDStr,
		})
		return
	}
	c.JSON(http.StatusInternalServerError, dto.ErrorResponse{
		Error: "Internal server error",
		Code:  http.StatusInternalServerError,
	})
}
