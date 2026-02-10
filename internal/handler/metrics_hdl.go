package handler

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// NewMetricsHdl returns an [http.Handler] that exposes all registered
// Prometheus metrics in the standard exposition format at GET /v1/metrics.
// No authentication is required.
func NewMetricsHdl() http.Handler {
	return promhttp.Handler()
}
