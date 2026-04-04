package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/devonartis/agentauth/internal/audit"
	"github.com/devonartis/agentauth/internal/obs"
)

// DBChecker reports whether a database connection is active. Implemented by
// [store.SqlStore].
type DBChecker interface {
	HasDB() bool
}

// HealthHdl handles GET /v1/health. It returns the broker's status,
// version, uptime, database connectivity, and audit event count.
// No authentication is required.
type HealthHdl struct {
	startTime time.Time
	version   string
	auditLog  *audit.AuditLog
	dbChecker DBChecker
}

// NewHealthHdl creates a new health handler that reports the given version
// string and starts tracking uptime from the moment of creation. Pass a
// non-nil auditLog to expose audit_events_count, and a non-nil dbChecker
// to expose db_connected. Either may be nil for memory-only or test setups.
func NewHealthHdl(version string, auditLog *audit.AuditLog, dbChecker DBChecker) *HealthHdl {
	return &HealthHdl{
		startTime: time.Now(),
		version:   version,
		auditLog:  auditLog,
		dbChecker: dbChecker,
	}
}

type healthResp struct {
	Status           string `json:"status"`
	Version          string `json:"version"`
	Uptime           int64  `json:"uptime"`
	DBConnected      bool   `json:"db_connected"`
	AuditEventsCount int    `json:"audit_events_count"`
}

func (h *HealthHdl) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	uptime := int64(time.Since(h.startTime).Seconds())

	dbConnected := false
	if h.dbChecker != nil {
		dbConnected = h.dbChecker.HasDB()
	}

	auditCount := 0
	if h.auditLog != nil {
		auditCount = len(h.auditLog.Events())
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(healthResp{
		Status:           "ok",
		Version:          h.version,
		Uptime:           uptime,
		DBConnected:      dbConnected,
		AuditEventsCount: auditCount,
	}); err != nil {
		obs.Warn("HEALTH", "hdl", "failed to encode response", "err="+err.Error())
	}
}
