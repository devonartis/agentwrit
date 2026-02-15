package main

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// ---------------------------------------------------------------------------
// Sidecar Prometheus Metrics
//
// Each binary owns its own metrics. Broker metrics live in internal/obs.
// Sidecar metrics live here, prefixed "agentauth_sidecar_".
// ---------------------------------------------------------------------------

// SidecarBootstrapTotal counts bootstrap attempts (success/failure).
var SidecarBootstrapTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "agentauth_sidecar_bootstrap_total",
	Help: "Total sidecar bootstrap attempts",
}, []string{"status"})

// SidecarRenewalsTotal counts token renewal outcomes (success/failure).
var SidecarRenewalsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "agentauth_sidecar_renewals_total",
	Help: "Total sidecar token renewal attempts",
}, []string{"status"})

// SidecarTokenExchangesTotal counts agent token exchanges (success/failure).
var SidecarTokenExchangesTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "agentauth_sidecar_token_exchanges_total",
	Help: "Total agent token exchanges via sidecar",
}, []string{"status"})

// SidecarScopeDenialsTotal counts scope ceiling enforcement denials.
var SidecarScopeDenialsTotal = promauto.NewCounter(prometheus.CounterOpts{
	Name: "agentauth_sidecar_scope_denials_total",
	Help: "Total scope ceiling enforcement denials",
})

// SidecarAgentsRegistered tracks the current number of registered agents.
var SidecarAgentsRegistered = promauto.NewGauge(prometheus.GaugeOpts{
	Name: "agentauth_sidecar_agents_registered",
	Help: "Number of currently registered agents in sidecar memory",
})

// SidecarRequestDuration observes per-endpoint HTTP latency in seconds.
var SidecarRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
	Name:    "agentauth_sidecar_request_duration_seconds",
	Help:    "Sidecar HTTP request duration in seconds",
	Buckets: prometheus.DefBuckets,
}, []string{"endpoint"})

// ---------------------------------------------------------------------------
// Convenience helpers — thin wrappers so call sites stay clean.
// ---------------------------------------------------------------------------

// RecordBootstrap increments the bootstrap counter with the given status.
func RecordBootstrap(status string) {
	SidecarBootstrapTotal.WithLabelValues(status).Inc()
}

// RecordRenewal increments the renewal counter with the given status.
func RecordRenewal(status string) {
	SidecarRenewalsTotal.WithLabelValues(status).Inc()
}

// RecordExchange increments the token exchange counter with the given status.
func RecordExchange(status string) {
	SidecarTokenExchangesTotal.WithLabelValues(status).Inc()
}

// RecordScopeDenial increments the scope denial counter.
func RecordScopeDenial() {
	SidecarScopeDenialsTotal.Inc()
}
