package handler

import (
	"net/http"

	"github.com/divineartis/agentauth/internal/audit"
	"github.com/divineartis/agentauth/internal/authz"
	"github.com/divineartis/agentauth/internal/obs"
	"github.com/divineartis/agentauth/internal/problemdetails"
	"github.com/divineartis/agentauth/internal/revoke"
)

// ReleaseHdl handles POST /v1/token/release. An authenticated agent
// presents its Bearer token and the handler revokes the token's JTI,
// effectively surrendering the credential. This is a self-revocation —
// no admin scope required.
type ReleaseHdl struct {
	revSvc   *revoke.RevSvc
	auditLog *audit.AuditLog
}

// NewReleaseHdl creates a new token release handler.
func NewReleaseHdl(revSvc *revoke.RevSvc, auditLog *audit.AuditLog) *ReleaseHdl {
	return &ReleaseHdl{revSvc: revSvc, auditLog: auditLog}
}

func (h *ReleaseHdl) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	claims := authz.ClaimsFromContext(r.Context())
	if claims == nil {
		problemdetails.WriteProblem(r.Context(), w, http.StatusUnauthorized, "unauthorized", "missing authentication", r.URL.Path)
		return
	}

	// Revoke own token by JTI. Idempotent — re-revoking is a no-op.
	if _, err := h.revSvc.Revoke("token", claims.Jti); err != nil {
		obs.Warn("RELEASE", "hdl", "revoke failed", "jti="+claims.Jti, "err="+err.Error())
	}

	if h.auditLog != nil {
		h.auditLog.Record(audit.EventTokenReleased, claims.Sub, claims.TaskId, claims.OrchId,
			"token released | jti="+claims.Jti,
			audit.WithOutcome("success"))
	}
	obs.TokensRevokedTotal.WithLabelValues("release").Inc()

	w.WriteHeader(http.StatusNoContent)
}
