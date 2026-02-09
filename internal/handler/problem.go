// Package handler provides HTTP handlers for the AgentAuth broker's public
// and authenticated API endpoints.
//
// Each handler type implements [http.Handler] and is wired to a route in
// cmd/broker/main.go. Public endpoints (challenge, health, metrics, token
// validation) require no authentication. Authenticated endpoints (renew,
// delegate) require a valid Bearer token via [authz.ValMw]. Admin endpoints
// (revoke, audit) additionally require admin-level scopes.
//
// All error responses use RFC 7807 application/problem+json via
// [WriteProblem].
package handler

import (
	"encoding/json"
	"net/http"
)

// ProblemDetail represents an RFC 7807 "application/problem+json" error
// response. The Type field uses the AgentAuth URN namespace
// "urn:agentauth:error:{errType}".
type ProblemDetail struct {
	Type     string `json:"type"`
	Title    string `json:"title"`
	Status   int    `json:"status"`
	Detail   string `json:"detail"`
	Instance string `json:"instance"`
}

// WriteProblem writes an RFC 7807 application/problem+json response with
// the given HTTP status code. The errType is appended to the
// "urn:agentauth:error:" URN prefix. The instance field should be the
// request path.
func WriteProblem(w http.ResponseWriter, status int, errType, detail, instance string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(ProblemDetail{
		Type:     "urn:agentauth:error:" + errType,
		Title:    http.StatusText(status),
		Status:   status,
		Detail:   detail,
		Instance: instance,
	})
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
