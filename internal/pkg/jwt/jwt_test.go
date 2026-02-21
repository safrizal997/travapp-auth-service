package jwt

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
)

func generateTestKeys(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate RSA key: %v", err)
	}

	privPath := filepath.Join(dir, "private.pem")
	privBytes := x509.MarshalPKCS1PrivateKey(key)
	privPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: privBytes})
	os.WriteFile(privPath, privPEM, 0600)

	pubPath := filepath.Join(dir, "public.pem")
	pubBytes, _ := x509.MarshalPKIXPublicKey(&key.PublicKey)
	pubPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubBytes})
	os.WriteFile(pubPath, pubPEM, 0600)

	return privPath, pubPath
}

func TestNewManager(t *testing.T) {
	privPath, pubPath := generateTestKeys(t)
	mgr, err := NewManager(privPath, pubPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mgr == nil {
		t.Fatal("manager should not be nil")
	}
}

func TestNewManager_InvalidPaths(t *testing.T) {
	_, err := NewManager("/nonexistent/private.pem", "/nonexistent/public.pem")
	if err == nil {
		t.Error("expected error for nonexistent keys")
	}
}

func TestGenerateAndValidateAccessToken(t *testing.T) {
	privPath, pubPath := generateTestKeys(t)
	mgr, _ := NewManager(privPath, pubPath)

	credID := uuid.New()
	tenantID := uuid.New()
	role := "admin"
	perms := []string{"read", "write"}
	provider := "email"
	ttl := 15 * time.Minute

	token, jti, err := mgr.GenerateAccessToken(credID, tenantID, role, perms, provider, ttl)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}
	if token == "" {
		t.Error("token should not be empty")
	}
	if jti == "" {
		t.Error("jti should not be empty")
	}

	claims, err := mgr.ValidateAccessToken(token)
	if err != nil {
		t.Fatalf("failed to validate token: %v", err)
	}

	if claims.Sub != credID.String() {
		t.Errorf("expected sub %s, got %s", credID.String(), claims.Sub)
	}
	if claims.TenantID != tenantID.String() {
		t.Errorf("expected tenant_id %s, got %s", tenantID.String(), claims.TenantID)
	}
	if claims.Role != role {
		t.Errorf("expected role %s, got %s", role, claims.Role)
	}
	if claims.Provider != provider {
		t.Errorf("expected provider %s, got %s", provider, claims.Provider)
	}
	if claims.TokenType != "access" {
		t.Errorf("expected token_type 'access', got %s", claims.TokenType)
	}
	if claims.JTI != jti {
		t.Errorf("expected jti %s, got %s", jti, claims.JTI)
	}
	if len(claims.Permissions) != 2 {
		t.Errorf("expected 2 permissions, got %d", len(claims.Permissions))
	}
}

func TestValidateAccessToken_Expired(t *testing.T) {
	privPath, pubPath := generateTestKeys(t)
	mgr, _ := NewManager(privPath, pubPath)

	token, _, err := mgr.GenerateAccessToken(uuid.New(), uuid.New(), "user", nil, "email", -1*time.Hour)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	_, err = mgr.ValidateAccessToken(token)
	if err == nil {
		t.Error("expected error for expired token")
	}
}

func TestValidateAccessToken_WrongKey(t *testing.T) {
	privPath1, pubPath1 := generateTestKeys(t)
	mgr1, _ := NewManager(privPath1, pubPath1)

	_, pubPath2 := generateTestKeys(t)
	privPath3, _ := generateTestKeys(t)
	mgr2, _ := NewManager(privPath3, pubPath2)

	token, _, _ := mgr1.GenerateAccessToken(uuid.New(), uuid.New(), "user", nil, "email", 15*time.Minute)

	_, err := mgr2.ValidateAccessToken(token)
	if err == nil {
		t.Error("expected error when validating with wrong key")
	}
}

func TestValidateAccessToken_Invalid(t *testing.T) {
	privPath, pubPath := generateTestKeys(t)
	mgr, _ := NewManager(privPath, pubPath)

	_, err := mgr.ValidateAccessToken("not.a.valid.jwt")
	if err == nil {
		t.Error("expected error for invalid token")
	}
}

func TestGetJWKS(t *testing.T) {
	privPath, pubPath := generateTestKeys(t)
	mgr, _ := NewManager(privPath, pubPath)

	jwks := mgr.GetJWKS()
	if jwks == nil {
		t.Fatal("JWKS should not be nil")
	}
	if len(jwks.Keys) != 1 {
		t.Fatalf("expected 1 key, got %d", len(jwks.Keys))
	}

	key := jwks.Keys[0]
	if key.Kty != "RSA" {
		t.Errorf("expected kty RSA, got %s", key.Kty)
	}
	if key.Use != "sig" {
		t.Errorf("expected use sig, got %s", key.Use)
	}
	if key.Alg != "RS256" {
		t.Errorf("expected alg RS256, got %s", key.Alg)
	}
	if key.Kid != "auth-key-1" {
		t.Errorf("expected kid auth-key-1, got %s", key.Kid)
	}
	if key.N == "" {
		t.Error("N should not be empty")
	}
	if key.E == "" {
		t.Error("E should not be empty")
	}
}

func TestGetPublicKey(t *testing.T) {
	privPath, pubPath := generateTestKeys(t)
	mgr, _ := NewManager(privPath, pubPath)

	pub := mgr.GetPublicKey()
	if pub == nil {
		t.Error("public key should not be nil")
	}
}

func TestNewManager_InvalidPrivateKeyPEM(t *testing.T) {
	dir := t.TempDir()
	privPath := filepath.Join(dir, "private.pem")
	pubPath := filepath.Join(dir, "public.pem")
	os.WriteFile(privPath, []byte("not a pem"), 0600)
	os.WriteFile(pubPath, []byte("not a pem"), 0600)

	_, err := NewManager(privPath, pubPath)
	if err == nil {
		t.Error("expected error for invalid PEM")
	}
}

func TestNewManager_InvalidPublicKeyPEM(t *testing.T) {
	dir := t.TempDir()

	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	privPath := filepath.Join(dir, "private.pem")
	privBytes := x509.MarshalPKCS1PrivateKey(key)
	privPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: privBytes})
	os.WriteFile(privPath, privPEM, 0600)

	pubPath := filepath.Join(dir, "public.pem")
	os.WriteFile(pubPath, []byte("not a pem"), 0600)

	_, err := NewManager(privPath, pubPath)
	if err == nil {
		t.Error("expected error for invalid public key PEM")
	}
}

func TestNewManager_PKCS8PrivateKey(t *testing.T) {
	dir := t.TempDir()

	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	privPath := filepath.Join(dir, "private.pem")
	pkcs8Bytes, _ := x509.MarshalPKCS8PrivateKey(key)
	privPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: pkcs8Bytes})
	os.WriteFile(privPath, privPEM, 0600)

	pubPath := filepath.Join(dir, "public.pem")
	pubBytes, _ := x509.MarshalPKIXPublicKey(&key.PublicKey)
	pubPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubBytes})
	os.WriteFile(pubPath, pubPEM, 0600)

	mgr, err := NewManager(privPath, pubPath)
	if err != nil {
		t.Fatalf("expected PKCS8 key to work: %v", err)
	}
	if mgr == nil {
		t.Fatal("manager should not be nil")
	}
}

func TestNewManager_InvalidPrivateKeyBytes(t *testing.T) {
	dir := t.TempDir()

	privPath := filepath.Join(dir, "private.pem")
	privPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: []byte("garbage")})
	os.WriteFile(privPath, privPEM, 0600)

	pubPath := filepath.Join(dir, "public.pem")
	os.WriteFile(pubPath, []byte("dummy"), 0600)

	_, err := NewManager(privPath, pubPath)
	if err == nil {
		t.Error("expected error for invalid private key bytes")
	}
}

func TestNewManager_InvalidPublicKeyBytes(t *testing.T) {
	dir := t.TempDir()

	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	privPath := filepath.Join(dir, "private.pem")
	privBytes := x509.MarshalPKCS1PrivateKey(key)
	privPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: privBytes})
	os.WriteFile(privPath, privPEM, 0600)

	pubPath := filepath.Join(dir, "public.pem")
	pubPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: []byte("garbage")})
	os.WriteFile(pubPath, pubPEM, 0600)

	_, err := NewManager(privPath, pubPath)
	if err == nil {
		t.Error("expected error for invalid public key bytes")
	}
}

func TestGenerateAccessToken_NilPermissions(t *testing.T) {
	privPath, pubPath := generateTestKeys(t)
	mgr, _ := NewManager(privPath, pubPath)

	token, jti, err := mgr.GenerateAccessToken(uuid.New(), uuid.New(), "user", nil, "email", 15*time.Minute)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token == "" || jti == "" {
		t.Error("token and jti should not be empty")
	}

	claims, err := mgr.ValidateAccessToken(token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if claims.Permissions != nil {
		// nil is acceptable, empty slice also
	}
}
