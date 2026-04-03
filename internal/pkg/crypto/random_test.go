package crypto

import (
	"encoding/hex"
	"testing"
)

func TestGenerateRandomBytes(t *testing.T) {
	tests := []struct {
		name string
		n    int
	}{
		{"16 bytes", 16},
		{"32 bytes", 32},
		{"64 bytes", 64},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := GenerateRandomBytes(tt.n)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(b) != tt.n {
				t.Errorf("expected %d bytes, got %d", tt.n, len(b))
			}
		})
	}
}

func TestGenerateRandomHex(t *testing.T) {
	h, err := GenerateRandomHex(32)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(h) != 64 {
		t.Errorf("expected 64 hex chars, got %d", len(h))
	}

	// Verify it's valid hex
	_, err = hex.DecodeString(h)
	if err != nil {
		t.Errorf("not valid hex: %v", err)
	}

	// Uniqueness
	h2, _ := GenerateRandomHex(32)
	if h == h2 {
		t.Error("two random hex strings should not be equal")
	}
}

func TestSHA256Hash(t *testing.T) {
	hash1 := SHA256Hash("hello")
	hash2 := SHA256Hash("hello")
	if hash1 != hash2 {
		t.Error("SHA256 should be deterministic")
	}

	hash3 := SHA256Hash("world")
	if hash1 == hash3 {
		t.Error("different inputs should produce different hashes")
	}

	if len(hash1) != 64 {
		t.Errorf("expected 64 hex chars, got %d", len(hash1))
	}
}

func TestGenerateCodeVerifier(t *testing.T) {
	v, err := GenerateCodeVerifier()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(v) == 0 {
		t.Error("code verifier should not be empty")
	}

	v2, _ := GenerateCodeVerifier()
	if v == v2 {
		t.Error("two code verifiers should not be equal")
	}
}

func TestGenerateCodeChallenge(t *testing.T) {
	verifier := "test-verifier-string"
	c1 := GenerateCodeChallenge(verifier)
	c2 := GenerateCodeChallenge(verifier)
	if c1 != c2 {
		t.Error("code challenge should be deterministic for same verifier")
	}

	c3 := GenerateCodeChallenge("different-verifier")
	if c1 == c3 {
		t.Error("different verifiers should produce different challenges")
	}

	if len(c1) == 0 {
		t.Error("code challenge should not be empty")
	}
}
