package authz

import (
	"context"
	"net/http"
	"strings"

	"github.com/divineartis/agentauth/internal/audit"
	"github.com/divineartis/agentauth/internal/problemdetails"
	"github.com/divineartis/agentauth/internal/token"
)

// contextKey is an unexported type for context keys in this package.
type contextKey int

const (
	claimsKey contextKey = iota
)

// TokenVerifier is the interface required by [ValMw] to verify bearer
// tokens. It is satisfied by [token.TknSvc].
type TokenVerifier interface {
	Verify(tokenStr string) (*token.TknClaims, error)
}

// RevocationChecker tests whether token claims match any active revocation
// entry. It is satisfied by [revoke.RevSvc].
type RevocationChecker interface {
	IsRevoked(claims *token.TknClaims) bool
}

// AuditRecorder records structured audit events. It is satisfied by
// [audit.AuditLog].
type AuditRecorder interface {
	Record(eventType, agentID, taskID, orchID, detail string)
}

// ValMw is the validation middleware. It extracts the Bearer token from the
// Authorization header, verifies it via [TokenVerifier], optionally checks
// revocation via [RevocationChecker], and stores the resulting
// [token.TknClaims] in the request context for downstream handlers.
//
// A nil RevocationChecker disables revocation checking.
type ValMw struct {
	tknSvc   TokenVerifier
	revSvc   RevocationChecker
	auditLog AuditRecorder
	audience string
}

// NewValMw creates a new validation middleware. The revSvc and auditLog
// parameters may be nil to disable revocation checking or audit recording
// respectively. When audience is non-empty, every token's aud claim is
// checked for a matching value; empty skips the check.
func NewValMw(tknSvc TokenVerifier, revSvc RevocationChecker, auditLog AuditRecorder, audience string) *ValMw {
	return &ValMw{
		tknSvc:   tknSvc,
		revSvc:   revSvc,
		auditLog: auditLog,
		audience: audience,
	}
}

// Wrap returns an [http.Handler] that validates the Bearer token before
// passing the request to next. On authentication failure it responds with
// a 401 or 403 RFC 7807 problem response and does not call next.
func (m *ValMw) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			if m.auditLog != nil {
				m.auditLog.Record(audit.EventTokenAuthFailed, "", "", "", "missing authorization header | path="+r.URL.Path)
			}
			problemdetails.WriteProblem(r.Context(), w, 401, "unauthorized", "missing authorization header", r.URL.Path)
			return
		}

		if !strings.HasPrefix(authHeader, "Bearer ") {
			if m.auditLog != nil {
				m.auditLog.Record(audit.EventTokenAuthFailed, "", "", "", "invalid authorization scheme | path="+r.URL.Path)
			}
			problemdetails.WriteProblem(r.Context(), w, 401, "unauthorized", "invalid authorization scheme", r.URL.Path)
			return
		}

		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
		claims, err := m.tknSvc.Verify(tokenStr)
		if err != nil {
			if m.auditLog != nil {
				m.auditLog.Record(audit.EventTokenAuthFailed, "", "", "", "token verification failed: "+err.Error()+" | path="+r.URL.Path)
			}
			problemdetails.WriteProblem(r.Context(), w, 401, "unauthorized", "token verification failed: "+err.Error(), r.URL.Path)
			return
		}

		if m.revSvc != nil && m.revSvc.IsRevoked(claims) {
			if m.auditLog != nil {
				m.auditLog.Record(audit.EventTokenRevokedAccess, claims.Sub, claims.TaskId, claims.OrchId, "revoked token used | path="+r.URL.Path)
			}
			problemdetails.WriteProblem(r.Context(), w, 403, "insufficient_scope", "token has been revoked", r.URL.Path)
			return
		}

		// Audience validation (skip when not configured)
		if m.audience != "" && !containsAudience(claims.Aud, m.audience) {
			if m.auditLog != nil {
				m.auditLog.Record(audit.EventTokenAuthFailed, claims.Sub, claims.TaskId, claims.OrchId,
					"audience mismatch | expected="+m.audience+" | path="+r.URL.Path)
			}
			problemdetails.WriteProblem(r.Context(), w, 401, "unauthorized", "token audience mismatch", r.URL.Path)
			return
		}

		ctx := context.WithValue(r.Context(), claimsKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireScope returns a handler that checks that the authenticated
// token's scopes cover the given scope string. It must be used after
// [ValMw.Wrap] so that claims are present in the context. If the scope
// check fails it responds with a 403 RFC 7807 problem response and
// records a scope_violation audit event.
func (m *ValMw) RequireScope(scope string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims := ClaimsFromContext(r.Context())
		if claims == nil {
			problemdetails.WriteProblem(r.Context(), w, 401, "unauthorized", "no claims in context", r.URL.Path)
			return
		}

		if !ScopeIsSubset([]string{scope}, claims.Scope) {
			if m.auditLog != nil {
				m.auditLog.Record(audit.EventScopeViolation, claims.Sub, claims.TaskId, claims.OrchId,
					"scope_violation | required="+scope+" | actual="+strings.Join(claims.Scope, ",")+" | path="+r.URL.Path)
			}
			problemdetails.WriteProblem(r.Context(), w, 403, "insufficient_scope", "token lacks required scope: "+scope, r.URL.Path)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// ContextWithClaims stores claims in context. Exported for testing.
func ContextWithClaims(ctx context.Context, claims *token.TknClaims) context.Context {
	return context.WithValue(ctx, claimsKey, claims)
}

// ClaimsFromContext extracts the [token.TknClaims] stored by [ValMw.Wrap]
// from the request context. It returns nil if no claims are present.
func ClaimsFromContext(ctx context.Context) *token.TknClaims {
	v := ctx.Value(claimsKey)
	if v == nil {
		return nil
	}
	claims, ok := v.(*token.TknClaims)
	if !ok {
		return nil
	}
	return claims
}

// containsAudience checks whether aud contains the expected audience string.
func containsAudience(aud []string, expected string) bool {
	for _, a := range aud {
		if a == expected {
			return true
		}
	}
	return false
}

// TokenFromRequest extracts the raw bearer token string from the
// Authorization header. It returns an empty string if the header is
// missing or does not use the "Bearer " scheme.
func TokenFromRequest(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return ""
	}
	return strings.TrimPrefix(authHeader, "Bearer ")
}
