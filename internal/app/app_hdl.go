// Package app — HTTP handler for app registration and authentication endpoints.
package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/divineartis/agentauth/internal/authz"
	"github.com/divineartis/agentauth/internal/obs"
	"github.com/divineartis/agentauth/internal/problemdetails"
	"github.com/divineartis/agentauth/internal/store"
)

const (
	hdlMod      = "app"
	hdlCmp      = "handler"
	maxBodyBytes = int64(1 << 20) // 1 MB
)

// AppHdl handles HTTP requests for app registration and authentication.
// Admin endpoints require a Bearer token with admin:launch-tokens:* scope.
// POST /v1/app/auth is unauthenticated (rate-limited only).
type AppHdl struct {
	appSvc      *AppSvc
	valMw       *authz.ValMw
	rateLimiter *authz.RateLimiter
}

// NewAppHdl returns a new AppHdl wired with the given dependencies.
// The app auth endpoint is rate-limited at 10 requests/minute per client_id
// (0.167 req/sec, burst 3) to prevent credential stuffing.
func NewAppHdl(appSvc *AppSvc, valMw *authz.ValMw) *AppHdl {
	return &AppHdl{
		appSvc:      appSvc,
		valMw:       valMw,
		rateLimiter: authz.NewRateLimiter(10.0/60.0, 3),
	}
}

// RegisterRoutes wires all app endpoints onto mux.
func (h *AppHdl) RegisterRoutes(mux *http.ServeMux) {
	requireAdmin := func(next http.Handler) http.Handler {
		return h.valMw.Wrap(h.valMw.RequireScope("admin:launch-tokens:*", next))
	}

	mux.Handle("POST /v1/admin/apps", requireAdmin(http.HandlerFunc(h.handleRegisterApp)))
	mux.Handle("GET /v1/admin/apps", requireAdmin(http.HandlerFunc(h.handleListApps)))
	mux.Handle("GET /v1/admin/apps/{id}", requireAdmin(http.HandlerFunc(h.handleGetApp)))
	mux.Handle("PUT /v1/admin/apps/{id}", requireAdmin(http.HandlerFunc(h.handleUpdateApp)))
	mux.Handle("DELETE /v1/admin/apps/{id}", requireAdmin(http.HandlerFunc(h.handleDeregisterApp)))
	mux.Handle("POST /v1/app/auth", h.rateLimiter.WrapWithKeyExtractor(http.HandlerFunc(h.handleAppAuth), clientIDFromBody))
}

// ---------------------------------------------------------------------------
// Request / response types
// ---------------------------------------------------------------------------

type registerAppReq struct {
	Name   string   `json:"name"`
	Scopes []string `json:"scopes"`
}

type appResp struct {
	AppID          string   `json:"app_id"`
	Name           string   `json:"name"`
	ClientID       string   `json:"client_id"`
	ClientSecret   string   `json:"client_secret,omitempty"` // only on register
	Scopes         []string `json:"scopes"`
	Status         string   `json:"status"`
	CreatedAt      string   `json:"created_at,omitempty"`
	UpdatedAt      string   `json:"updated_at,omitempty"`
	DeregisteredAt string   `json:"deregistered_at,omitempty"`
}

type listAppsResp struct {
	Apps  []appResp `json:"apps"`
	Total int       `json:"total"`
}

type updateAppReq struct {
	Scopes []string `json:"scopes"`
}

type appAuthReq struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
}

type appAuthResp struct {
	AccessToken string   `json:"access_token"`
	ExpiresIn   int      `json:"expires_in"`
	TokenType   string   `json:"token_type"`
	Scopes      []string `json:"scopes"`
}

// ---------------------------------------------------------------------------
// Handlers
// ---------------------------------------------------------------------------

// handleRegisterApp — POST /v1/admin/apps
func (h *AppHdl) handleRegisterApp(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	var req registerAppReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		problemdetails.WriteProblem(r.Context(), w, http.StatusBadRequest, "invalid_request", "malformed JSON body", r.URL.Path)
		return
	}
	if req.Name == "" {
		problemdetails.WriteProblem(r.Context(), w, http.StatusBadRequest, "invalid_request", "name is required", r.URL.Path)
		return
	}
	if len(req.Scopes) == 0 {
		problemdetails.WriteProblem(r.Context(), w, http.StatusBadRequest, "invalid_request", "scopes must not be empty", r.URL.Path)
		return
	}

	claims := authz.ClaimsFromContext(r.Context())
	createdBy := ""
	if claims != nil {
		createdBy = claims.Sub
	}

	resp, err := h.appSvc.RegisterApp(req.Name, req.Scopes, createdBy)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidAppName):
			problemdetails.WriteProblem(r.Context(), w, http.StatusBadRequest, "invalid_request", "invalid app name: must be lowercase letters, digits, and hyphens", r.URL.Path)
		case errors.Is(err, ErrInvalidScopeFormat):
			problemdetails.WriteProblem(r.Context(), w, http.StatusBadRequest, "invalid_request", "invalid scope format: use action:resource:identifier", r.URL.Path)
		default:
			obs.Fail(hdlMod, hdlCmp, "register app failed", "err="+err.Error())
			problemdetails.WriteProblem(r.Context(), w, http.StatusInternalServerError, "internal_error", "failed to register app", r.URL.Path)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(appResp{
		AppID:        resp.AppID,
		ClientID:     resp.ClientID,
		ClientSecret: resp.ClientSecret,
		Scopes:       resp.ScopeCeiling,
	}); err != nil {
		obs.Warn(hdlMod, hdlCmp, "failed to encode register response", "err="+err.Error())
	}
}

// handleListApps — GET /v1/admin/apps
func (h *AppHdl) handleListApps(w http.ResponseWriter, r *http.Request) {
	apps, err := h.appSvc.ListApps()
	if err != nil {
		obs.Fail(hdlMod, hdlCmp, "list apps failed", "err="+err.Error())
		problemdetails.WriteProblem(r.Context(), w, http.StatusInternalServerError, "internal_error", "failed to list apps", r.URL.Path)
		return
	}

	items := make([]appResp, len(apps))
	for i, a := range apps {
		items[i] = storeAppToResp(a)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(listAppsResp{Apps: items, Total: len(items)}); err != nil {
		obs.Warn(hdlMod, hdlCmp, "failed to encode list response", "err="+err.Error())
	}
}

// handleGetApp — GET /v1/admin/apps/{id}
func (h *AppHdl) handleGetApp(w http.ResponseWriter, r *http.Request) {
	appID := r.PathValue("id")
	if appID == "" {
		problemdetails.WriteProblem(r.Context(), w, http.StatusBadRequest, "invalid_request", "app id is required", r.URL.Path)
		return
	}

	rec, err := h.appSvc.GetApp(appID)
	if err != nil {
		if errors.Is(err, store.ErrAppNotFound) {
			problemdetails.WriteProblem(r.Context(), w, http.StatusNotFound, "not_found", "app not found", r.URL.Path)
			return
		}
		obs.Fail(hdlMod, hdlCmp, "get app failed", "app_id="+appID, "err="+err.Error())
		problemdetails.WriteProblem(r.Context(), w, http.StatusInternalServerError, "internal_error", "failed to get app", r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(storeAppToResp(*rec)); err != nil {
		obs.Warn(hdlMod, hdlCmp, "failed to encode get response", "err="+err.Error())
	}
}

// handleUpdateApp — PUT /v1/admin/apps/{id}
func (h *AppHdl) handleUpdateApp(w http.ResponseWriter, r *http.Request) {
	appID := r.PathValue("id")
	if appID == "" {
		problemdetails.WriteProblem(r.Context(), w, http.StatusBadRequest, "invalid_request", "app id is required", r.URL.Path)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	var req updateAppReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		problemdetails.WriteProblem(r.Context(), w, http.StatusBadRequest, "invalid_request", "malformed JSON body", r.URL.Path)
		return
	}
	if len(req.Scopes) == 0 {
		problemdetails.WriteProblem(r.Context(), w, http.StatusBadRequest, "invalid_request", "scopes must not be empty", r.URL.Path)
		return
	}

	claims := authz.ClaimsFromContext(r.Context())
	updatedBy := ""
	if claims != nil {
		updatedBy = claims.Sub
	}

	if err := h.appSvc.UpdateApp(appID, req.Scopes, updatedBy); err != nil {
		switch {
		case errors.Is(err, store.ErrAppNotFound):
			problemdetails.WriteProblem(r.Context(), w, http.StatusNotFound, "not_found", "app not found", r.URL.Path)
		case errors.Is(err, ErrInvalidScopeFormat):
			problemdetails.WriteProblem(r.Context(), w, http.StatusBadRequest, "invalid_request", "invalid scope format: use action:resource:identifier", r.URL.Path)
		default:
			obs.Fail(hdlMod, hdlCmp, "update app failed", "app_id="+appID, "err="+err.Error())
			problemdetails.WriteProblem(r.Context(), w, http.StatusInternalServerError, "internal_error", "failed to update app", r.URL.Path)
		}
		return
	}

	rec, err := h.appSvc.GetApp(appID)
	if err != nil {
		obs.Fail(hdlMod, hdlCmp, "get app after update failed", "app_id="+appID, "err="+err.Error())
		problemdetails.WriteProblem(r.Context(), w, http.StatusInternalServerError, "internal_error", "failed to retrieve updated app", r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(appResp{
		AppID:     rec.AppID,
		Scopes:    rec.ScopeCeiling,
		UpdatedAt: rec.UpdatedAt.UTC().Format(time.RFC3339),
	}); err != nil {
		obs.Warn(hdlMod, hdlCmp, "failed to encode update response", "err="+err.Error())
	}
}

// handleDeregisterApp — DELETE /v1/admin/apps/{id}
func (h *AppHdl) handleDeregisterApp(w http.ResponseWriter, r *http.Request) {
	appID := r.PathValue("id")
	if appID == "" {
		problemdetails.WriteProblem(r.Context(), w, http.StatusBadRequest, "invalid_request", "app id is required", r.URL.Path)
		return
	}

	claims := authz.ClaimsFromContext(r.Context())
	deregisteredBy := ""
	if claims != nil {
		deregisteredBy = claims.Sub
	}

	if err := h.appSvc.DeregisterApp(appID, deregisteredBy); err != nil {
		if errors.Is(err, store.ErrAppNotFound) {
			problemdetails.WriteProblem(r.Context(), w, http.StatusNotFound, "not_found", "app not found", r.URL.Path)
			return
		}
		obs.Fail(hdlMod, hdlCmp, "deregister app failed", "app_id="+appID, "err="+err.Error())
		problemdetails.WriteProblem(r.Context(), w, http.StatusInternalServerError, "internal_error", "failed to deregister app", r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(appResp{
		AppID:          appID,
		Status:         "inactive",
		DeregisteredAt: time.Now().UTC().Format(time.RFC3339),
	}); err != nil {
		obs.Warn(hdlMod, hdlCmp, "failed to encode deregister response", "err="+err.Error())
	}
}

// handleAppAuth — POST /v1/app/auth (no Bearer required)
func (h *AppHdl) handleAppAuth(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	var req appAuthReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		problemdetails.WriteProblem(r.Context(), w, http.StatusBadRequest, "invalid_request", "malformed JSON body", r.URL.Path)
		return
	}
	if req.ClientID == "" || req.ClientSecret == "" {
		problemdetails.WriteProblem(r.Context(), w, http.StatusBadRequest, "invalid_request", "client_id and client_secret are required", r.URL.Path)
		return
	}

	resp, err := h.appSvc.AuthenticateApp(req.ClientID, req.ClientSecret)
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			problemdetails.WriteProblem(r.Context(), w, http.StatusUnauthorized, "unauthorized", "Authentication failed", r.URL.Path)
			return
		}
		obs.Fail(hdlMod, hdlCmp, "app auth failed", "client_id="+req.ClientID, "err="+err.Error())
		problemdetails.WriteProblem(r.Context(), w, http.StatusInternalServerError, "internal_error", "authentication error", r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	scopes := []string{}
	if resp.Claims != nil {
		scopes = resp.Claims.Scope
	}
	if err := json.NewEncoder(w).Encode(appAuthResp{
		AccessToken: resp.AccessToken,
		ExpiresIn:   resp.ExpiresIn,
		TokenType:   "Bearer",
		Scopes:      scopes,
	}); err != nil {
		obs.Warn(hdlMod, hdlCmp, "failed to encode auth response", "err="+err.Error())
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// clientIDFromBody peeks at the JSON body to extract client_id for per-app
// rate limiting. It buffers the entire body and resets r.Body so the handler
// still receives the full request. Returns empty string on any error so the
// caller can fall back to IP-based limiting.
func clientIDFromBody(r *http.Request) string {
	if r.Body == nil {
		return ""
	}
	data, err := io.ReadAll(io.LimitReader(r.Body, maxBodyBytes))
	if err != nil {
		return ""
	}
	r.Body = io.NopCloser(bytes.NewReader(data))

	var req struct {
		ClientID string `json:"client_id"`
	}
	if err := json.Unmarshal(data, &req); err != nil {
		return ""
	}
	return req.ClientID
}

// storeAppToResp converts a store.AppRecord to a JSON-safe appResp.
// client_secret_hash is never included.
func storeAppToResp(a store.AppRecord) appResp {
	return appResp{
		AppID:     a.AppID,
		Name:      a.Name,
		ClientID:  a.ClientID,
		Scopes:    a.ScopeCeiling,
		Status:    a.Status,
		CreatedAt: a.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt: a.UpdatedAt.UTC().Format(time.RFC3339),
	}
}
