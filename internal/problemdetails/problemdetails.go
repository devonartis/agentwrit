// Package problemdetails implements RFC 7807 "application/problem+json"
// error responses and HTTP request infrastructure for the AgentWrit broker.
//
// All broker error responses use the AgentWrit URN namespace
// "urn:agentwrit:error:{errType}" for machine-readable error categorization.
// Responses include a unique request ID for log correlation, an optional
// error code for programmatic handling, and an optional diagnostic hint.
//
// This package also provides request-scoped middleware:
//   - RequestIDMiddleware generates or propagates a unique X-Request-ID for
//     every incoming request, stored in context for downstream handlers.
//   - MaxBytesBody limits request body size to prevent resource exhaustion
//     (default 1 MB).
package problemdetails

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
)

// ProblemDetail represents an RFC 7807 "application/problem+json" error
// response. The Type field uses the AgentWrit URN namespace
// "urn:agentwrit:error:{errType}". It includes extensions for diagnostics
// and diagnostics: ErrorCode, RequestID, and an optional Hint.
type ProblemDetail struct {
	Type      string `json:"type"`
	Title     string `json:"title"`
	Status    int    `json:"status"`
	Detail    string `json:"detail"`
	Instance  string `json:"instance"`
	ErrorCode string `json:"error_code,omitempty"`
	RequestID string `json:"request_id,omitempty"`
	Hint      string `json:"hint,omitempty"`
}

// WriteProblem writes an RFC 7807 application/problem+json response with
// the given HTTP status code.
func WriteProblem(ctx context.Context, w http.ResponseWriter, status int, errType, detail, instance string) {
	WriteProblemExtended(ctx, w, status, errType, detail, instance, errType, "")
}

// WriteProblemExtended is like WriteProblem but allows providing a specific
// errorCode and hint.
func WriteProblemExtended(ctx context.Context, w http.ResponseWriter, status int, errType, detail, instance, errorCode, hint string) {
	requestID := GetRequestID(ctx)

	p := ProblemDetail{
		Type:      "urn:agentwrit:error:" + errType,
		Title:     http.StatusText(status),
		Status:    status,
		Detail:    detail,
		Instance:  instance,
		ErrorCode: errorCode,
		RequestID: requestID,
		Hint:      hint,
	}

	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(p); err != nil {
		log.Printf("[AA:PROBLEM:WARN] failed to encode problem+json response: %v", err)
	}
}

// maxBodyBytes is the maximum allowed request body size (1 MB).
const maxBodyBytes int64 = 1 << 20

// MaxBytesBody wraps an [http.Handler] so that the request body is
// limited to 1 MB via [http.MaxBytesReader]. Requests that exceed the
// limit receive a 413 Request Entity Too Large RFC 7807 response.
//
// The body is eagerly buffered up to the limit so that handlers using
// streaming decoders (e.g. json.NewDecoder) also get the 413 when the
// payload exceeds the limit, regardless of how much they actually read.
func MaxBytesBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		limited := http.MaxBytesReader(w, r.Body, maxBodyBytes)
		buf, err := io.ReadAll(limited)
		if err != nil {
			var mbe *http.MaxBytesError
			if errors.As(err, &mbe) {
				WriteProblem(r.Context(), w, http.StatusRequestEntityTooLarge,
					"payload_too_large", "request body must not exceed 1 MB", r.URL.Path)
				return
			}
			WriteProblem(r.Context(), w, http.StatusBadRequest,
				"invalid_request", "failed to read request body", r.URL.Path)
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(buf))
		next.ServeHTTP(w, r)
	})
}

type contextKey string

const requestIDKey contextKey = "request_id"

// RequestIDMiddleware injects a unique X-Request-ID into the request context
// and response headers. It uses a random 16-character hex string.
func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			id = generateID()
		}

		ctx := context.WithValue(r.Context(), requestIDKey, id)
		w.Header().Set("X-Request-ID", id)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetRequestID retrieves the request ID from the context.
// It returns an empty string if no ID is found.
func GetRequestID(ctx context.Context) string {
	if id, ok := ctx.Value(requestIDKey).(string); ok {
		return id
	}
	return ""
}

func generateID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "unknown"
	}
	return hex.EncodeToString(b)
}
