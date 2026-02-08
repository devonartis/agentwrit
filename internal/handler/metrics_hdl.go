package handler

import (
	"net/http"

	"github.com/divineartis/agentauth/internal/obs"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// MetricsHdl serves Prometheus metrics in text exposition format.
type MetricsHdl struct {
	delegate http.Handler
}

// NewMetricsHdl creates a metrics handler backed by Prometheus collectors.
func NewMetricsHdl() *MetricsHdl {
	obs.RegisterMetrics()
	return &MetricsHdl{delegate: promhttp.Handler()}
}

// ServeHTTP exposes metrics for GET requests.
func (h *MetricsHdl) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	h.delegate.ServeHTTP(w, r)
}
