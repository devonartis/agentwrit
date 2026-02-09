package authz

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/divineartis/agentauth/internal/audit"
	"github.com/divineartis/agentauth/internal/deleg"
	"github.com/divineartis/agentauth/internal/obs"
	"github.com/divineartis/agentauth/internal/revoke"
	"github.com/divineartis/agentauth/internal/token"
)

type ctxKey string

const (
	ctxRequiredScope ctxKey = "required_scope"
	ctxAgentID       ctxKey = "agent_id"
)

// ValMw is the zero-trust authorization middleware for bearer token verification.
type ValMw struct {
	tknSvc     *token.TknSvc
	revChecker revoke.RevChecker
	auditLog   *audit.AuditLog
}

// NewValMw creates a validation middleware with token verification, revocation checking, and optional audit logging.
func NewValMw(tknSvc *token.TknSvc, revChecker revoke.RevChecker, auditLog *audit.AuditLog) *ValMw {
	return &ValMw{tknSvc: tknSvc, revChecker: revChecker, auditLog: auditLog}
}

// Wrap returns an http.Handler that enforces bearer token authentication and scope validation.
func (m *ValMw) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authz := strings.TrimSpace(r.Header.Get("Authorization"))
		if !strings.HasPrefix(strings.ToLower(authz), "bearer ") {
			deny(w, r, http.StatusUnauthorized, "urn:agentauth:error:missing-token", "missing bearer token")
			obs.RecordValidation(false)
			obs.Fail("AUTHZ", "ValMw.Wrap", "authorization denied", "reason=missing_bearer")
			m.emitDenied("", r.URL.Path, "missing_bearer")
			return
		}
		tokenStr := strings.TrimSpace(authz[len("Bearer "):])
		claims, err := m.tknSvc.Verify(tokenStr)
		if err != nil {
			deny(w, r, http.StatusUnauthorized, "urn:agentauth:error:invalid-token", "invalid token")
			obs.RecordValidation(false)
			obs.Fail("AUTHZ", "ValMw.Wrap", "authorization denied", "reason=invalid_token")
			m.emitDenied("", r.URL.Path, "invalid_token")
			return
		}

		// Enforce delegated-chain integrity if token carries delegation history.
		if len(claims.DelegChain) > 0 {
			if ok, cerr := deleg.VerifyChain(claims.DelegChain, claims.Scope, m.revChecker, m.tknSvc.PublicKey()); !ok {
				deny(w, r, http.StatusUnauthorized, "urn:agentauth:error:invalid-delegation-chain", "invalid delegation chain")
				obs.RecordValidation(false)
				obs.Fail("AUTHZ", "ValMw.Wrap", "authorization denied",
					"reason=invalid_delegation_chain",
					"hop="+strconv.Itoa(cerr.Hop),
					"detail="+cerr.Reason,
				)
				m.emitDenied(claims.Sub, r.URL.Path, "invalid_delegation_chain")
				return
			}
		}

		chainHash := computeChainHash(claims.DelegChain)
		if m.revChecker != nil {
			if revoked, level := m.revChecker.IsRevoked(claims.Jti, claims.Sub, claims.TaskId, chainHash); revoked {
				deny(w, r, http.StatusUnauthorized, "urn:agentauth:error:token-revoked", "token has been revoked")
				obs.RecordValidation(false)
				obs.Fail("AUTHZ", "ValMw.Wrap", "authorization denied", "reason=revoked", "level="+level)
				m.emitDenied(claims.Sub, r.URL.Path, "revoked:"+level)
				return
			}
		}

		if required, _ := r.Context().Value(ctxRequiredScope).(string); required != "" {
			ok := false
			for _, have := range claims.Scope {
				if token.ScopeMatch(required, have) {
					ok = true
					break
				}
			}
			if !ok {
				deny(w, r, http.StatusForbidden, "urn:agentauth:error:scope-mismatch", "insufficient scope")
				obs.RecordValidation(false)
				obs.Fail("AUTHZ", "ValMw.Wrap", "authorization denied", "reason=scope_mismatch", "required="+required)
				m.emitDenied(claims.Sub, r.URL.Path, "scope_mismatch")
				return
			}
		}

		ctx := context.WithValue(r.Context(), ctxAgentID, claims.Sub)
		obs.RecordValidation(true)
		obs.Ok("AUTHZ", "ValMw.Wrap", "authorization granted", "agent_id="+claims.Sub)
		if m.auditLog != nil {
			_ = m.auditLog.LogEvent(&audit.AuditEvt{
				EventType:       audit.EvtAccessGranted,
				AgentInstanceId: claims.Sub,
				TaskId:          claims.TaskId,
				OrchId:          claims.OrchId,
				Resource:        r.URL.Path,
				Action:          r.Method,
				Outcome:         "granted",
				DelegDepth:      len(claims.DelegChain),
				DelegChainHash:  chainHash,
			})
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// WithRequiredScope wraps a handler to inject a required scope into the request context.
func WithRequiredScope(scope string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), ctxRequiredScope, scope)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// AgentIDFromContext extracts the authenticated agent's SPIFFE ID from the request context.
func AgentIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(ctxAgentID).(string)
	return id
}

func (m *ValMw) emitDenied(agentId, resource, reason string) {
	if m.auditLog != nil {
		_ = m.auditLog.LogEvent(&audit.AuditEvt{
			EventType:       audit.EvtAccessDenied,
			AgentInstanceId: agentId,
			Resource:        resource,
			Action:          "access",
			Outcome:         "denied",
			DenialReason:    reason,
		})
	}
}

func deny(w http.ResponseWriter, r *http.Request, status int, typ, title string) {
	obs.WriteProblemForRequest(w, r, status, typ, title, title)
}

// computeChainHash returns the SHA-256 hex digest of the JSON-serialized
// delegation chain, or empty string if the chain is empty.
func computeChainHash(chain []token.DelegRecord) string {
	if len(chain) == 0 {
		return ""
	}
	raw, err := json.Marshal(chain)
	if err != nil {
		return ""
	}
	h := sha256.Sum256(raw)
	return hex.EncodeToString(h[:])
}
