package dto

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
	TenantID string `json:"tenant_id" binding:"required,uuid"`
}

type RegisterRequest struct {
	Email                string `json:"email" binding:"required,email"`
	Password             string `json:"password" binding:"required,min=8"`
	PasswordConfirmation string `json:"password_confirmation" binding:"required,eqfield=Password"`
	TenantID             string `json:"tenant_id" binding:"required,uuid"`
}

type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
	TenantID     string `json:"tenant_id" binding:"required,uuid"`
}

type LogoutRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type ChangePasswordRequest struct {
	CurrentPassword         string `json:"current_password" binding:"required"`
	NewPassword             string `json:"new_password" binding:"required,min=8"`
	NewPasswordConfirmation string `json:"new_password_confirmation" binding:"required,eqfield=NewPassword"`
}

type ForgotPasswordRequest struct {
	Email    string `json:"email" binding:"required,email"`
	TenantID string `json:"tenant_id" binding:"required,uuid"`
}

type ResetPasswordRequest struct {
	Token                   string `json:"token" binding:"required"`
	Email                   string `json:"email" binding:"required,email"`
	NewPassword             string `json:"new_password" binding:"required,min=8"`
	NewPasswordConfirmation string `json:"new_password_confirmation" binding:"required,eqfield=NewPassword"`
}
