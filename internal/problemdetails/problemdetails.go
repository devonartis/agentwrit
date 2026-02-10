package problemdetails

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
)

// ProblemDetail represents an RFC 7807 "application/problem+json" error
// response. The Type field uses the AgentAuth URN namespace
// "urn:agentauth:error:{errType}". It includes extensions for diagnostics
// and sidecar support: ErrorCode, RequestID, and an optional Hint.
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
		Type:      "urn:agentauth:error:" + errType,
		Title:    http.StatusText(status),
		Status:   status,
		Detail:   detail,
		Instance: instance,
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
func MaxBytesBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
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
