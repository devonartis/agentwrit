package handler

import (
	"bufio"
	"encoding/json"
	"net"
	"net/http"
	"time"

	"github.com/divineartis/agentauth/internal/obs"
	"github.com/divineartis/agentauth/internal/store"
)

// HealthHdl serves broker health details for operators and probes.
type HealthHdl struct {
	sqlStore   *store.SqlStore
	redisStore *store.RedisStore
	version    string
	startedAt  time.Time
}

// NewHealthHdl creates a health handler with current process start time.
func NewHealthHdl(sqlStore *store.SqlStore, redisStore *store.RedisStore, version string) *HealthHdl {
	return NewHealthHdlWithStart(sqlStore, redisStore, version, time.Now().UTC())
}

// NewHealthHdlWithStart creates a health handler with explicit start time (useful for tests).
func NewHealthHdlWithStart(sqlStore *store.SqlStore, redisStore *store.RedisStore, version string, startedAt time.Time) *HealthHdl {
	if version == "" {
		version = "0.1.0"
	}
	if startedAt.IsZero() {
		startedAt = time.Now().UTC()
	}
	return &HealthHdl{
		sqlStore:   sqlStore,
		redisStore: redisStore,
		version:    version,
		startedAt:  startedAt,
	}
}

type healthResp struct {
	Status        string            `json:"status"`
	Version       string            `json:"version"`
	UptimeSeconds int64             `json:"uptime_seconds"`
	Components    map[string]string `json:"components"`
}

// ServeHTTP reports broker/component health.
func (h *HealthHdl) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	components := map[string]string{
		"sqlite": h.sqliteStatus(),
		"redis":  h.redisStatus(),
	}

	status := "healthy"
	if components["sqlite"] != "healthy" {
		status = "unhealthy"
	} else if components["redis"] != "healthy" {
		status = "degraded"
	}

	obs.Ok("OBS", "HealthHdl.ServeHTTP", "health check", "status="+status)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(healthResp{
		Status:        status,
		Version:       h.version,
		UptimeSeconds: int64(time.Since(h.startedAt).Seconds()),
		Components:    components,
	})
}

func (h *HealthHdl) sqliteStatus() string {
	if h.sqlStore == nil {
		return "unhealthy"
	}
	return "healthy"
}

func (h *HealthHdl) redisStatus() string {
	// Redis is optional in current broker wiring; if not configured, report healthy.
	if h.redisStore == nil || h.redisStore.Addr == "" {
		return "healthy"
	}

	conn, err := net.DialTimeout("tcp", h.redisStore.Addr, 250*time.Millisecond)
	if err != nil {
		return "unhealthy"
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(250 * time.Millisecond))

	if _, err := conn.Write([]byte("*1\r\n$4\r\nPING\r\n")); err != nil {
		return "unhealthy"
	}
	reply, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		return "unhealthy"
	}
	if len(reply) >= 5 && reply[:5] == "+PONG" {
		return "healthy"
	}
	return "unhealthy"
}
