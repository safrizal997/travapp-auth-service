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

type PasswordHandler struct {
	passwordUC *usecase.PasswordUseCase
	logger     *zap.Logger
}

func NewPasswordHandler(passwordUC *usecase.PasswordUseCase, logger *zap.Logger) *PasswordHandler {
	return &PasswordHandler{
		passwordUC: passwordUC,
		logger:     logger,
	}
}

// ChangePassword changes the authenticated user's password.
// @Summary Change password
// @Description Change the password for the currently authenticated user
// @Tags Password
// @Accept json
// @Produce json
// @Param request body dto.ChangePasswordRequest true "Change password request"
// @Success 200 {object} dto.MessageResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Security BearerAuth
// @Router /api/v1/auth/password/change [post]
func (h *PasswordHandler) ChangePassword(c *gin.Context) {
	var req dto.ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Error: "Invalid request: " + err.Error(),
			Code:  http.StatusBadRequest,
		})
		return
	}

	credID, err := uuid.Parse(middleware.GetCredentialID(c))
	if err != nil {
		c.JSON(http.StatusUnauthorized, dto.ErrorResponse{Error: "Invalid credential", Code: http.StatusUnauthorized})
		return
	}

	input := usecase.ChangePasswordInput{
		CredentialID:       credID,
		CurrentPassword:    req.CurrentPassword,
		NewPassword:        req.NewPassword,
		NewPasswordConfirm: req.NewPasswordConfirmation,
	}

	if err := h.passwordUC.ChangePassword(c.Request.Context(), input); err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.MessageResponse{Message: "Password changed successfully"})
}

// ForgotPassword sends a password reset email.
// @Summary Request password reset
// @Description Sends a password reset link to the user's email address
// @Tags Password
// @Accept json
// @Produce json
// @Param request body dto.ForgotPasswordRequest true "Forgot password request"
// @Success 200 {object} dto.MessageResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/v1/auth/password/forgot [post]
func (h *PasswordHandler) ForgotPassword(c *gin.Context) {
	var req dto.ForgotPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Error: "Invalid request: " + err.Error(),
			Code:  http.StatusBadRequest,
		})
		return
	}

	tenantID, err := uuid.Parse(req.TenantID)
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "Invalid tenant_id", Code: http.StatusBadRequest})
		return
	}

	input := usecase.ForgotPasswordInput{
		Email:    req.Email,
		TenantID: tenantID,
	}

	_, err = h.passwordUC.ForgotPassword(c.Request.Context(), input)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.MessageResponse{
		Message: "If an account with that email exists, a password reset link has been sent.",
	})
}

// ResetPassword resets the user's password using a reset token.
// @Summary Reset password
// @Description Reset user's password using a valid password reset token
// @Tags Password
// @Accept json
// @Produce json
// @Param request body dto.ResetPasswordRequest true "Reset password request"
// @Success 200 {object} dto.MessageResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/v1/auth/password/reset [post]
func (h *PasswordHandler) ResetPassword(c *gin.Context) {
	var req dto.ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Error: "Invalid request: " + err.Error(),
			Code:  http.StatusBadRequest,
		})
		return
	}

	input := usecase.ResetPasswordInput{
		Token:              req.Token,
		Email:              req.Email,
		NewPassword:        req.NewPassword,
		NewPasswordConfirm: req.NewPasswordConfirmation,
	}

	if err := h.passwordUC.ResetPassword(c.Request.Context(), input); err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.MessageResponse{Message: "Password has been reset successfully"})
}

// VerifyEmail verifies a user's email address.
// @Summary Verify email address
// @Description Verify user's email address using the verification token sent via email
// @Tags Password
// @Produce json
// @Param token query string true "Email verification token"
// @Param email query string true "User's email address"
// @Success 200 {object} dto.MessageResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/v1/auth/verify-email [get]
func (h *PasswordHandler) VerifyEmail(c *gin.Context) {
	token := c.Query("token")
	email := c.Query("email")

	if token == "" || email == "" {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Error: "Missing token or email parameter",
			Code:  http.StatusBadRequest,
		})
		return
	}

	input := usecase.VerifyEmailInput{
		Token: token,
		Email: email,
	}

	if err := h.passwordUC.VerifyEmail(c.Request.Context(), input); err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.MessageResponse{Message: "Email verified successfully"})
}
