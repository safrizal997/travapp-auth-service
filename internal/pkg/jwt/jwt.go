package jwt

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type Manager struct {
	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey
	keyID      string
}

type AccessTokenClaims struct {
	Sub         string   `json:"sub"`
	TenantID    string   `json:"tenant_id"`
	Role        string   `json:"role"`
	Permissions []string `json:"permissions"`
	Provider    string   `json:"provider"`
	TokenType   string   `json:"token_type"`
	JTI         string   `json:"jti"`
	jwt.RegisteredClaims
}

func NewManager(privateKeyPath, publicKeyPath string) (*Manager, error) {
	privBytes, err := os.ReadFile(privateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key: %w", err)
	}

	block, _ := pem.Decode(privBytes)
	if block == nil {
		return nil, fmt.Errorf("failed to decode private key PEM")
	}

	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		pkcs8Key, err2 := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err2 != nil {
			return nil, fmt.Errorf("failed to parse private key: %w", err)
		}
		var ok bool
		privateKey, ok = pkcs8Key.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("private key is not RSA")
		}
	}

	pubBytes, err := os.ReadFile(publicKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read public key: %w", err)
	}

	pubBlock, _ := pem.Decode(pubBytes)
	if pubBlock == nil {
		return nil, fmt.Errorf("failed to decode public key PEM")
	}

	pubInterface, err := x509.ParsePKIXPublicKey(pubBlock.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key: %w", err)
	}

	publicKey, ok := pubInterface.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("public key is not RSA")
	}

	return &Manager{
		privateKey: privateKey,
		publicKey:  publicKey,
		keyID:      "auth-key-1",
	}, nil
}

func (m *Manager) GenerateAccessToken(credentialID uuid.UUID, tenantID uuid.UUID, role string, permissions []string, provider string, ttl time.Duration) (string, string, error) {
	jti := uuid.New().String()
	now := time.Now()

	claims := AccessTokenClaims{
		Sub:         credentialID.String(),
		TenantID:    tenantID.String(),
		Role:        role,
		Permissions: permissions,
		Provider:    provider,
		TokenType:   "access",
		JTI:         jti,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
			ID:        jti,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = m.keyID

	signedToken, err := token.SignedString(m.privateKey)
	if err != nil {
		return "", "", fmt.Errorf("failed to sign token: %w", err)
	}

	return signedToken, jti, nil
}

func (m *Manager) ValidateAccessToken(tokenString string) (*AccessTokenClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &AccessTokenClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return m.publicKey, nil
	})
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	claims, ok := token.Claims.(*AccessTokenClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	return claims, nil
}

type JWK struct {
	Kty string `json:"kty"`
	Use string `json:"use"`
	Kid string `json:"kid"`
	Alg string `json:"alg"`
	N   string `json:"n"`
	E   string `json:"e"`
}

type JWKS struct {
	Keys []JWK `json:"keys"`
}

func (m *Manager) GetJWKS() *JWKS {
	return &JWKS{
		Keys: []JWK{
			{
				Kty: "RSA",
				Use: "sig",
				Kid: m.keyID,
				Alg: "RS256",
				N:   base64.RawURLEncoding.EncodeToString(m.publicKey.N.Bytes()),
				E:   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(m.publicKey.E)).Bytes()),
			},
		},
	}
}

func (m *Manager) GetPublicKey() *rsa.PublicKey {
	return m.publicKey
}
