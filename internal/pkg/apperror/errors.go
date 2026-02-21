package apperror

import (
	"fmt"
	"net/http"
)

type AppError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Err     error  `json:"-"`
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

func (e *AppError) Unwrap() error {
	return e.Err
}

func NewBadRequest(message string) *AppError {
	return &AppError{Code: http.StatusBadRequest, Message: message}
}

func NewUnauthorized(message string) *AppError {
	return &AppError{Code: http.StatusUnauthorized, Message: message}
}

func NewForbidden(message string) *AppError {
	return &AppError{Code: http.StatusForbidden, Message: message}
}

func NewNotFound(message string) *AppError {
	return &AppError{Code: http.StatusNotFound, Message: message}
}

func NewConflict(message string) *AppError {
	return &AppError{Code: http.StatusConflict, Message: message}
}

func NewLocked(message string) *AppError {
	return &AppError{Code: http.StatusLocked, Message: message}
}

func NewTooManyRequests(message string) *AppError {
	return &AppError{Code: http.StatusTooManyRequests, Message: message}
}

func NewInternal(err error) *AppError {
	return &AppError{Code: http.StatusInternalServerError, Message: "Internal server error", Err: err}
}

func NewInternalWithMessage(message string, err error) *AppError {
	return &AppError{Code: http.StatusInternalServerError, Message: message, Err: err}
}
