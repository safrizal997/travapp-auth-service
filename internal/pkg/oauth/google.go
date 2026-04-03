package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

type GoogleConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
}

type GoogleTokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	IDToken      string `json:"id_token"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
}

type GoogleUserInfo struct {
	Sub           string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
}

func (g *GoogleConfig) GetAuthURL(state, codeChallenge string) string {
	params := url.Values{
		"client_id":             {g.ClientID},
		"redirect_uri":         {g.RedirectURI},
		"response_type":        {"code"},
		"scope":                {"openid email profile"},
		"state":                {state},
		"code_challenge":       {codeChallenge},
		"code_challenge_method": {"S256"},
		"access_type":          {"offline"},
		"prompt":               {"consent"},
	}
	return "https://accounts.google.com/o/oauth2/v2/auth?" + params.Encode()
}

func (g *GoogleConfig) ExchangeCode(ctx context.Context, code, codeVerifier string) (*GoogleTokenResponse, error) {
	data := url.Values{
		"code":          {code},
		"client_id":     {g.ClientID},
		"client_secret": {g.ClientSecret},
		"redirect_uri":  {g.RedirectURI},
		"grant_type":    {"authorization_code"},
		"code_verifier": {codeVerifier},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://oauth2.googleapis.com/token", strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read token response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token exchange failed with status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp GoogleTokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("failed to parse token response: %w", err)
	}

	return &tokenResp, nil
}

func (g *GoogleConfig) VerifyIDToken(idToken string) (*GoogleUserInfo, error) {
	parser := jwt.NewParser(jwt.WithoutClaimsValidation())
	token, _, err := parser.ParseUnverified(idToken, jwt.MapClaims{})
	if err != nil {
		return nil, fmt.Errorf("failed to parse id_token: %w", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("invalid id_token claims")
	}

	aud, _ := claims["aud"].(string)
	if aud != g.ClientID {
		return nil, fmt.Errorf("invalid audience in id_token")
	}

	iss, _ := claims["iss"].(string)
	if iss != "https://accounts.google.com" && iss != "accounts.google.com" {
		return nil, fmt.Errorf("invalid issuer in id_token")
	}

	emailVerified, _ := claims["email_verified"].(bool)

	return &GoogleUserInfo{
		Sub:           fmt.Sprintf("%v", claims["sub"]),
		Email:         fmt.Sprintf("%v", claims["email"]),
		EmailVerified: emailVerified,
		Name:          fmt.Sprintf("%v", claims["name"]),
		Picture:       fmt.Sprintf("%v", claims["picture"]),
	}, nil
}
