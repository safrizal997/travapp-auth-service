package testutil

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
	"github.com/safrizal997/travapp-auth-service/internal/domain/entity"
	"github.com/safrizal997/travapp-auth-service/internal/pkg/hash"
	jwtpkg "github.com/safrizal997/travapp-auth-service/internal/pkg/jwt"
	"go.uber.org/zap"
)

func GenerateTestRSAKeys(t *testing.T) (privateKeyPath, publicKeyPath string) {
	t.Helper()
	dir := t.TempDir()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate RSA key: %v", err)
	}

	privPath := filepath.Join(dir, "private.pem")
	privBytes := x509.MarshalPKCS1PrivateKey(key)
	privPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: privBytes})
	if err := os.WriteFile(privPath, privPEM, 0600); err != nil {
		t.Fatalf("failed to write private key: %v", err)
	}

	pubPath := filepath.Join(dir, "public.pem")
	pubBytes, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatalf("failed to marshal public key: %v", err)
	}
	pubPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubBytes})
	if err := os.WriteFile(pubPath, pubPEM, 0600); err != nil {
		t.Fatalf("failed to write public key: %v", err)
	}

	return privPath, pubPath
}

func NewTestJWTManager(t *testing.T) *jwtpkg.Manager {
	t.Helper()
	privPath, pubPath := GenerateTestRSAKeys(t)
	mgr, err := jwtpkg.NewManager(privPath, pubPath)
	if err != nil {
		t.Fatalf("failed to create JWT manager: %v", err)
	}
	return mgr
}

func NewTestLogger() *zap.Logger {
	return zap.NewNop()
}

func NewTestHasher() *hash.BCryptHasher {
	return hash.NewBCryptHasher(4)
}

func MakeTestCredential() *entity.AuthCredential {
	pw := "$2a$04$test_hashed_password_placeholder"
	now := time.Now()
	return &entity.AuthCredential{
		ID:              uuid.New(),
		TenantID:        uuid.New(),
		Email:           "test@example.com",
		PasswordHash:    &pw,
		Provider:        "email",
		IsActive:        true,
		IsEmailVerified: true,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
}

func MakeTestTenant() *entity.AuthTenant {
	return &entity.AuthTenant{
		ID:       uuid.New(),
		Code:     "test-tenant",
		Name:     "Test Tenant",
		IsActive: true,
	}
}


func MakeTestRole() *entity.AuthRole {
	return &entity.AuthRole{
		ID:        uuid.New(),
		TenantID:  uuid.New(),
		Code:      "traveler",
		Name:      "Traveler",
		IsDefault: true,
	}
}
