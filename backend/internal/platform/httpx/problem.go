// Package httpx contains HTTP plumbing shared by every module: RFC 9457 problem
// responses, middleware and request-scoped values.
package httpx

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"math"
	"net/http"
	"strconv"

	"github.com/yelnurq/startify/backend/internal/platform/apperr"
)

// ContentTypeProblem is the RFC 9457 media type.
const ContentTypeProblem = "application/problem+json"

const problemTypeBase = "https://qadam.kz/problems/"

// Problem is an RFC 9457 Problem Details document extended with a localisable code.
// It mirrors components.schemas.Problem in api/openapi.yaml.
type Problem struct {
	Type      string              `json:"type"`
	Title     string              `json:"title"`
	Status    int                 `json:"status"`
	Detail    string              `json:"detail,omitempty"`
	Instance  string              `json:"instance,omitempty"`
	Code      string              `json:"code"`
	RequestID string              `json:"request_id,omitempty"`
	Params    map[string]any      `json:"params,omitempty"`
	Errors    []apperr.FieldError `json:"errors,omitempty"`
}

// StatusFor maps an error kind to its HTTP status code.
func StatusFor(kind apperr.Kind) int {
	switch kind {
	case apperr.KindBadRequest:
		return http.StatusBadRequest
	case apperr.KindValidation:
		return http.StatusUnprocessableEntity
	case apperr.KindUnauthorized:
		return http.StatusUnauthorized
	case apperr.KindForbidden:
		return http.StatusForbidden
	case apperr.KindNotFound:
		return http.StatusNotFound
	case apperr.KindConflict:
		return http.StatusConflict
	case apperr.KindPreconditionFailed:
		return http.StatusPreconditionFailed
	case apperr.KindTooLarge:
		return http.StatusRequestEntityTooLarge
	case apperr.KindRateLimited:
		return http.StatusTooManyRequests
	case apperr.KindUnavailable:
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}

// ProblemFrom converts any error into a problem document. Internal details are
// never exposed: unknown errors become a generic 500.
func ProblemFrom(r *http.Request, err error) Problem {
	var maxBytes *http.MaxBytesError
	switch {
	case errors.As(err, &maxBytes):
		err = apperr.New(apperr.KindTooLarge, "request.too_large", "request body is too large").
			WithParams(map[string]any{"limit_bytes": maxBytes.Limit})
	case errors.Is(err, context.DeadlineExceeded):
		err = apperr.Unavailable("request.timeout", "the request took too long")
	}

	e, ok := apperr.As(err)
	if !ok {
		e = apperr.Internal(err)
	}
	status := StatusFor(e.Kind)
	p := Problem{
		Type:      problemTypeBase + e.Code,
		Title:     http.StatusText(status),
		Status:    status,
		Code:      e.Code,
		Instance:  r.URL.Path,
		RequestID: RequestIDFrom(r.Context()),
		Params:    e.Params,
		Errors:    e.Fields,
	}
	if e.Kind != apperr.KindInternal {
		p.Detail = e.Message
	}
	return p
}

// WriteError renders err as a problem response and logs server-side failures.
func WriteError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, context.Canceled) && r.Context().Err() != nil {
		// The client went away; nobody is left to read a response.
		return
	}
	p := ProblemFrom(r, err)
	if p.Status >= http.StatusInternalServerError {
		slog.ErrorContext(r.Context(), "request failed", slog.Int("status", p.Status), slog.Any("error", err))
	}
	if e, ok := apperr.As(err); ok && e.RetryAfter > 0 {
		w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(e.RetryAfter.Seconds()))))
	}
	WriteProblem(w, p)
}

// WriteProblem writes a problem document.
func WriteProblem(w http.ResponseWriter, p Problem) {
	w.Header().Set("Content-Type", ContentTypeProblem)
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(p.Status)
	_ = json.NewEncoder(w).Encode(p)
}

// WriteJSON writes v as a JSON response.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// NotFoundHandler answers unknown routes with a problem document.
func NotFoundHandler(w http.ResponseWriter, r *http.Request) {
	WriteError(w, r, apperr.NotFound("route.not_found", "no such endpoint"))
}

// MethodNotAllowedHandler answers known routes requested with a wrong method.
func MethodNotAllowedHandler(w http.ResponseWriter, r *http.Request) {
	WriteProblem(w, Problem{
		Type:      problemTypeBase + "route.method_not_allowed",
		Title:     http.StatusText(http.StatusMethodNotAllowed),
		Status:    http.StatusMethodNotAllowed,
		Code:      "route.method_not_allowed",
		Instance:  r.URL.Path,
		RequestID: RequestIDFrom(r.Context()),
	})
}
