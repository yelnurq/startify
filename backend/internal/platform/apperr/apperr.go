// Package apperr defines domain errors that carry a stable, localisable code.
// Services return *Error values; the HTTP layer renders them as RFC 9457 problems.
package apperr

import (
	"errors"
	"fmt"
	"time"
)

// Kind classifies an error and determines its HTTP status.
type Kind uint8

// Error kinds.
const (
	KindInternal Kind = iota
	KindBadRequest
	KindValidation
	KindUnauthorized
	KindForbidden
	KindNotFound
	KindConflict
	KindPreconditionFailed
	KindPreconditionRequired
	KindTooLarge
	KindRateLimited
	KindUnavailable
)

// FieldError describes a single invalid input field.
type FieldError struct {
	Field   string         `json:"field"`
	Code    string         `json:"code"`
	Message string         `json:"message,omitempty"`
	Params  map[string]any `json:"params,omitempty"`
}

// Error is a classified error with a machine-readable code such as "project.not_found".
type Error struct {
	Kind    Kind
	Code    string
	Message string
	Params  map[string]any
	Fields  []FieldError
	// RetryAfter is reported to clients for rate-limited and unavailable errors.
	RetryAfter time.Duration
	cause      error
}

func (e *Error) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.cause)
	}
	return e.Code + ": " + e.Message
}

func (e *Error) Unwrap() error { return e.cause }

// Wrap attaches the underlying cause, kept for logs but never shown to clients.
func (e *Error) Wrap(cause error) *Error {
	c := *e
	c.cause = cause
	return &c
}

// WithParams attaches interpolation values for the localised message.
func (e *Error) WithParams(params map[string]any) *Error {
	c := *e
	c.Params = params
	return &c
}

// WithRetryAfter attaches a retry hint.
func (e *Error) WithRetryAfter(d time.Duration) *Error {
	c := *e
	c.RetryAfter = d
	return &c
}

// New builds an error of the given kind.
func New(kind Kind, code, message string) *Error {
	return &Error{Kind: kind, Code: code, Message: message}
}

// BadRequest reports a malformed request.
func BadRequest(code, message string) *Error { return New(KindBadRequest, code, message) }

// Validation reports semantically invalid input field by field.
func Validation(fields ...FieldError) *Error {
	return &Error{Kind: KindValidation, Code: "validation.failed", Message: "request validation failed", Fields: fields}
}

// Unauthorized reports missing or invalid credentials.
func Unauthorized(code, message string) *Error { return New(KindUnauthorized, code, message) }

// Forbidden reports insufficient permissions.
func Forbidden(code, message string) *Error { return New(KindForbidden, code, message) }

// NotFound reports a missing resource.
func NotFound(code, message string) *Error { return New(KindNotFound, code, message) }

// Conflict reports a state conflict, e.g. a duplicate.
func Conflict(code, message string) *Error { return New(KindConflict, code, message) }

// PreconditionFailed reports a failed If-Match check.
func PreconditionFailed(code, message string) *Error {
	return New(KindPreconditionFailed, code, message)
}

// PreconditionRequired reports a missing If-Match on a resource that is only
// updated with one.
func PreconditionRequired(code, message string) *Error {
	return New(KindPreconditionRequired, code, message)
}

// RateLimited reports an exhausted quota.
func RateLimited(code, message string, retryAfter time.Duration) *Error {
	return New(KindRateLimited, code, message).WithRetryAfter(retryAfter)
}

// Unavailable reports a temporarily unavailable dependency.
func Unavailable(code, message string) *Error { return New(KindUnavailable, code, message) }

// Internal wraps an unexpected failure.
func Internal(cause error) *Error {
	return New(KindInternal, "internal", "internal server error").Wrap(cause)
}

// As extracts an *Error from err's chain.
func As(err error) (*Error, bool) {
	var e *Error
	if errors.As(err, &e) {
		return e, true
	}
	return nil, false
}

// Is reports whether err carries the given code.
func Is(err error, code string) bool {
	e, ok := As(err)
	return ok && e.Code == code
}
