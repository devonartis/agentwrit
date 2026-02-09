package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/divineartis/agentauth/internal/identity"
	"github.com/divineartis/agentauth/internal/problemdetails"
	"github.com/divineartis/agentauth/internal/store"
)

// RegHdl handles POST /v1/register. It decodes the registration request,
// delegates to [identity.IdSvc.Register], and maps service-level errors to
// appropriate RFC 7807 HTTP responses.
type RegHdl struct {
	idSvc *identity.IdSvc
}

// NewRegHdl creates a new registration handler backed by the given
// identity service.
func NewRegHdl(idSvc *identity.IdSvc) *RegHdl {
	return &RegHdl{idSvc: idSvc}
}

func (h *RegHdl) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var req identity.RegisterReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		problemdetails.WriteProblem(r.Context(), w, http.StatusBadRequest, "invalid_request", "malformed JSON body", r.URL.Path)
		return
	}

	resp, err := h.idSvc.Register(req)
	if err != nil {
		switch {
		case errors.Is(err, identity.ErrMissingField):
			problemdetails.WriteProblem(r.Context(), w, http.StatusBadRequest, "invalid_request", err.Error(), r.URL.Path)
		case errors.Is(err, identity.ErrScopeViolation):
			problemdetails.WriteProblem(r.Context(), w, http.StatusForbidden, "scope_violation", err.Error(), r.URL.Path)
		case errors.Is(err, store.ErrTokenNotFound), errors.Is(err, store.ErrTokenExpired), errors.Is(err, store.ErrTokenConsumed):
			problemdetails.WriteProblem(r.Context(), w, http.StatusUnauthorized, "unauthorized", err.Error(), r.URL.Path)
		case errors.Is(err, store.ErrNonceNotFound), errors.Is(err, store.ErrNonceConsumed):
			problemdetails.WriteProblem(r.Context(), w, http.StatusUnauthorized, "unauthorized", err.Error(), r.URL.Path)
		case errors.Is(err, identity.ErrInvalidSignature), errors.Is(err, identity.ErrInvalidPublicKey):
			problemdetails.WriteProblem(r.Context(), w, http.StatusUnauthorized, "unauthorized", err.Error(), r.URL.Path)
		default:
			problemdetails.WriteProblem(r.Context(), w, http.StatusInternalServerError, "internal_error", "registration failed", r.URL.Path)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}
