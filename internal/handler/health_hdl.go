package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/divineartis/agentauth/internal/obs"
)

// HealthHdl handles GET /v1/health. It returns the broker's status,
// version, and uptime in seconds. No authentication is required.
type HealthHdl struct {
	startTime time.Time
	version   string
}

// NewHealthHdl creates a new health handler that reports the given version
// string and starts tracking uptime from the moment of creation.
func NewHealthHdl(version string) *HealthHdl {
	return &HealthHdl{
		startTime: time.Now(),
		version:   version,
	}
}

type healthResp struct {
	Status  string `json:"status"`
	Version string `json:"version"`
	Uptime  int64  `json:"uptime"`
}

func (h *HealthHdl) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	uptime := int64(time.Since(h.startTime).Seconds())
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(healthResp{
		Status:  "ok",
		Version: h.version,
		Uptime:  uptime,
	}); err != nil {
		obs.Warn("HEALTH", "hdl", "failed to encode response", "err="+err.Error())
	}
}
