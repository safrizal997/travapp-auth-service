package hash

import (
	"testing"
)

func TestBCryptHasher_Hash(t *testing.T) {
	hasher := NewBCryptHasher(4)
	password := "testPassword123"

	hashed, err := hasher.Hash(password)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hashed == "" {
		t.Error("hashed password should not be empty")
	}
	if hashed == password {
		t.Error("hashed password should differ from plaintext")
	}

	// Same password produces different hashes (bcrypt salt)
	hashed2, err := hasher.Hash(password)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hashed == hashed2 {
		t.Error("same password should produce different hashes due to salt")
	}
}

func TestBCryptHasher_Compare(t *testing.T) {
	hasher := NewBCryptHasher(4)
	password := "testPassword123"

	hashed, err := hasher.Hash(password)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Correct password
	if err := hasher.Compare(hashed, password); err != nil {
		t.Errorf("expected match, got error: %v", err)
	}

	// Wrong password
	if err := hasher.Compare(hashed, "wrongPassword"); err == nil {
		t.Error("expected error for wrong password")
	}
}
