package handler

import (
	"context"
	"encoding/json"
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
	WriteProblemExtended(ctx, w, status, errType, detail, instance, "", "")
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
	json.NewEncoder(w).Encode(p)
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
