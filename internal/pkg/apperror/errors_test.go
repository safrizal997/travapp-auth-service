package apperror

import (
	"errors"
	"net/http"
	"testing"
)

func TestNewBadRequest(t *testing.T) {
	err := NewBadRequest("bad input")
	if err.Code != http.StatusBadRequest {
		t.Errorf("expected code %d, got %d", http.StatusBadRequest, err.Code)
	}
	if err.Message != "bad input" {
		t.Errorf("expected message 'bad input', got %q", err.Message)
	}
	if err.Err != nil {
		t.Error("expected nil wrapped error")
	}
}

func TestNewUnauthorized(t *testing.T) {
	err := NewUnauthorized("not allowed")
	if err.Code != http.StatusUnauthorized {
		t.Errorf("expected code %d, got %d", http.StatusUnauthorized, err.Code)
	}
	if err.Message != "not allowed" {
		t.Errorf("expected message 'not allowed', got %q", err.Message)
	}
}

func TestNewForbidden(t *testing.T) {
	err := NewForbidden("forbidden")
	if err.Code != http.StatusForbidden {
		t.Errorf("expected code %d, got %d", http.StatusForbidden, err.Code)
	}
}

func TestNewNotFound(t *testing.T) {
	err := NewNotFound("not found")
	if err.Code != http.StatusNotFound {
		t.Errorf("expected code %d, got %d", http.StatusNotFound, err.Code)
	}
}

func TestNewConflict(t *testing.T) {
	err := NewConflict("conflict")
	if err.Code != http.StatusConflict {
		t.Errorf("expected code %d, got %d", http.StatusConflict, err.Code)
	}
}

func TestNewLocked(t *testing.T) {
	err := NewLocked("locked")
	if err.Code != http.StatusLocked {
		t.Errorf("expected code %d, got %d", http.StatusLocked, err.Code)
	}
}

func TestNewTooManyRequests(t *testing.T) {
	err := NewTooManyRequests("slow down")
	if err.Code != http.StatusTooManyRequests {
		t.Errorf("expected code %d, got %d", http.StatusTooManyRequests, err.Code)
	}
}

func TestNewInternal(t *testing.T) {
	inner := errors.New("db error")
	err := NewInternal(inner)
	if err.Code != http.StatusInternalServerError {
		t.Errorf("expected code %d, got %d", http.StatusInternalServerError, err.Code)
	}
	if err.Message != "Internal server error" {
		t.Errorf("unexpected message: %q", err.Message)
	}
	if err.Err != inner {
		t.Error("expected wrapped error to be inner")
	}
}

func TestNewInternalWithMessage(t *testing.T) {
	inner := errors.New("db error")
	err := NewInternalWithMessage("custom msg", inner)
	if err.Code != http.StatusInternalServerError {
		t.Errorf("expected code %d, got %d", http.StatusInternalServerError, err.Code)
	}
	if err.Message != "custom msg" {
		t.Errorf("unexpected message: %q", err.Message)
	}
	if err.Err != inner {
		t.Error("expected wrapped error to be inner")
	}
}

func TestAppError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *AppError
		expected string
	}{
		{
			name:     "without wrapped error",
			err:      NewBadRequest("bad input"),
			expected: "bad input",
		},
		{
			name:     "with wrapped error",
			err:      NewInternal(errors.New("db down")),
			expected: "Internal server error: db down",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestAppError_Unwrap(t *testing.T) {
	inner := errors.New("root cause")
	err := NewInternal(inner)
	if err.Unwrap() != inner {
		t.Error("Unwrap should return inner error")
	}

	err2 := NewBadRequest("no inner")
	if err2.Unwrap() != nil {
		t.Error("Unwrap should return nil when no inner error")
	}
}
