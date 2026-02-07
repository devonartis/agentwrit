package authz

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/divineartis/agentauth/internal/obs"
	"github.com/divineartis/agentauth/internal/token"
)

type ctxKey string

const (
	ctxRequiredScope ctxKey = "required_scope"
	ctxAgentID       ctxKey = "agent_id"
)

type ValMw struct {
	tknSvc *token.TknSvc
}

func NewValMw(tknSvc *token.TknSvc) *ValMw {
	return &ValMw{tknSvc: tknSvc}
}

func (m *ValMw) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authz := strings.TrimSpace(r.Header.Get("Authorization"))
		if !strings.HasPrefix(strings.ToLower(authz), "bearer ") {
			deny(w, http.StatusUnauthorized, "urn:agentauth:error:missing-token", "missing bearer token")
			obs.Fail("AUTHZ", "ValMw.Wrap", "authorization denied", "reason=missing_bearer")
			return
		}
		tokenStr := strings.TrimSpace(authz[len("Bearer "):])
		claims, err := m.tknSvc.Verify(tokenStr)
		if err != nil {
			deny(w, http.StatusUnauthorized, "urn:agentauth:error:invalid-token", "invalid token")
			obs.Fail("AUTHZ", "ValMw.Wrap", "authorization denied", "reason=invalid_token")
			return
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
				deny(w, http.StatusForbidden, "urn:agentauth:error:scope-mismatch", "insufficient scope")
				obs.Fail("AUTHZ", "ValMw.Wrap", "authorization denied", "reason=scope_mismatch", "required="+required)
				return
			}
		}

		ctx := context.WithValue(r.Context(), ctxAgentID, claims.Sub)
		obs.Ok("AUTHZ", "ValMw.Wrap", "authorization granted", "agent_id="+claims.Sub)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func WithRequiredScope(scope string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), ctxRequiredScope, scope)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func AgentIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(ctxAgentID).(string)
	return id
}

func deny(w http.ResponseWriter, status int, typ, title string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"type":   typ,
		"title":  title,
		"status": status,
	})
}

