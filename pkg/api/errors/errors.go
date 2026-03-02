// Package errors provides centralized error handling for the API.
package errors

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

// Error codes for different error types.
const (
	CodeBadRequest          = "BAD_REQUEST"
	CodeUnauthorized        = "UNAUTHORIZED"
	CodeForbidden           = "FORBIDDEN"
	CodeNotFound            = "NOT_FOUND"
	CodeConflict            = "CONFLICT"
	CodeInternalError       = "INTERNAL_ERROR"
	CodeValidationFailed    = "VALIDATION_FAILED"
	CodeRateLimitExceeded   = "RATE_LIMIT_EXCEEDED"
	CodeInvalidCredentials  = "INVALID_CREDENTIALS"
	CodeAccountSuspended    = "ACCOUNT_SUSPENDED"
	CodeTokenExpired        = "TOKEN_EXPIRED"
	CodeEmailAlreadyExists  = "EMAIL_ALREADY_EXISTS"
	CodeSessionNotFound     = "SESSION_NOT_FOUND"
	CodeAccessDenied        = "ACCESS_DENIED"
	CodeEncryptionFailed    = "ENCRYPTION_FAILED"
)

// AppError represents an application error with context.
type AppError struct {
	Code       string    `json:"code"`
	Message    string    `json:"message"`
	StatusCode int       `json:"-"`
	Err        error     `json:"-"`
	RequestID  string    `json:"request_id,omitempty"`
	Timestamp  time.Time `json:"timestamp"`
}

// Error implements the error interface.
func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap returns the underlying error.
func (e *AppError) Unwrap() error {
	return e.Err
}

// WithRequestID adds a request ID to the error.
func (e *AppError) WithRequestID(requestID string) *AppError {
	e.RequestID = requestID
	return e
}

// New creates a new AppError.
func New(code, message string, statusCode int, err error) *AppError {
	return &AppError{
		Code:       code,
		Message:    message,
		StatusCode: statusCode,
		Err:        err,
		Timestamp:  time.Now(),
	}
}

// BadRequest creates a 400 error.
func BadRequest(message string) *AppError {
	return New(CodeBadRequest, message, http.StatusBadRequest, nil)
}

// Unauthorized creates a 401 error.
func Unauthorized(message string) *AppError {
	return New(CodeUnauthorized, message, http.StatusUnauthorized, nil)
}

// Forbidden creates a 403 error.
func Forbidden(message string) *AppError {
	return New(CodeForbidden, message, http.StatusForbidden, nil)
}

// NotFound creates a 404 error.
func NotFound(message string) *AppError {
	return New(CodeNotFound, message, http.StatusNotFound, nil)
}

// Conflict creates a 409 error.
func Conflict(message string) *AppError {
	return New(CodeConflict, message, http.StatusConflict, nil)
}

// InternalError creates a 500 error.
func InternalError(err error) *AppError {
	return New(CodeInternalError, "Internal server error", http.StatusInternalServerError, err)
}

// ValidationError creates a validation error.
func ValidationError(message string) *AppError {
	return New(CodeValidationFailed, message, http.StatusBadRequest, nil)
}

// RateLimitError creates a rate limit error.
func RateLimitError() *AppError {
	return New(CodeRateLimitExceeded, "Rate limit exceeded", http.StatusTooManyRequests, nil)
}

// ErrorResponse represents the JSON error response structure.
type ErrorResponse struct {
	Error *AppError `json:"error"`
}

// HandleError writes an error response to the client.
// It logs the error and returns a JSON response with appropriate status code.
func HandleError(w http.ResponseWriter, err error) {
	var appErr *AppError
	
	// Convert to AppError if not already
	switch e := err.(type) {
	case *AppError:
		appErr = e
	default:
		appErr = InternalError(err)
	}

	// Log the error with context
	logError(appErr)

	// Write JSON response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(appErr.StatusCode)
	json.NewEncoder(w).Encode(ErrorResponse{Error: appErr})
}

// logError logs the error with appropriate level and context.
func logError(err *AppError) {
	attrs := []any{
		"code", err.Code,
		"status", err.StatusCode,
	}
	
	if err.RequestID != "" {
		attrs = append(attrs, "request_id", err.RequestID)
	}
	
	if err.Err != nil {
		attrs = append(attrs, "underlying_error", err.Err.Error())
	}

	// Log as error for 5xx, warn for 4xx
	if err.StatusCode >= 500 {
		slog.Error(err.Message, attrs...)
	} else {
		slog.Warn(err.Message, attrs...)
	}
}
