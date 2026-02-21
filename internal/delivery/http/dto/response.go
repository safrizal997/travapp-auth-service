package dto

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
}

type MessageResponse struct {
	Message string `json:"message"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Code    int    `json:"code"`
	ReqID   string `json:"request_id,omitempty"`
}

type RegisterResponse struct {
	Message      string `json:"message"`
	CredentialID string `json:"credential_id"`
}
