// Package obs provides structured logging and Prometheus metrics for the
// AgentAuth broker.
//
// # Logging
//
// Four severity helpers — [Ok], [Warn], [Fail], and [Trace] — emit lines in
// the format:
//
//	[AA:MODULE:LEVEL] TIMESTAMP | COMPONENT | MESSAGE | ctx...
//
// Ok and Warn write to stdout; Fail writes to stderr. Output is controlled
// by the global log level set via [Configure]:
//
//	quiet    – only Fail messages
//	standard – Ok, Warn, Fail
//	verbose  – Ok, Warn, Fail (same as standard; default)
//	trace    – all of the above plus Trace
//
// # Metrics
//
// The package registers Prometheus counters, gauges, and histograms via
// promauto so they are automatically collected by the default registry.
// Metric names are prefixed with "agentauth_" to namespace them.
package obs

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Log levels ordered by increasing verbosity. Higher values include all
// messages from lower levels.
const (
	LevelQuiet    = 0 // Fail only
	LevelStandard = 1 // Ok, Warn, Fail
	LevelVerbose  = 2 // same as Standard (default)
	LevelTrace    = 3 // all messages including Trace
)

var (
	mu       sync.RWMutex
	logLevel = LevelVerbose
)

// Configure sets the global log level from a human-readable string.
// Accepted values are "quiet", "standard", "verbose", and "trace"
// (case-insensitive). Unrecognized values default to verbose.
func Configure(level string) {
	mu.Lock()
	defer mu.Unlock()
	switch strings.ToLower(level) {
	case "quiet":
		logLevel = LevelQuiet
	case "standard":
		logLevel = LevelStandard
	case "verbose":
		logLevel = LevelVerbose
	case "trace":
		logLevel = LevelTrace
	default:
		logLevel = LevelVerbose
	}
}

func currentLevel() int {
	mu.RLock()
	defer mu.RUnlock()
	return logLevel
}

// Ok logs a success message to stdout. It is emitted at standard level
// and above. The variadic ctx strings are joined with ", " and appended
// after a pipe separator.
func Ok(module, component, msg string, ctx ...string) {
	if currentLevel() >= LevelStandard {
		emit(os.Stdout, module, "OK", component, msg, ctx)
	}
}

// Warn logs a warning message to stdout. It is emitted at standard level
// and above.
func Warn(module, component, msg string, ctx ...string) {
	if currentLevel() >= LevelStandard {
		emit(os.Stdout, module, "WARN", component, msg, ctx)
	}
}

// Fail logs an error message to stderr. It is emitted at every level
// including quiet, making it suitable for unrecoverable errors.
func Fail(module, component, msg string, ctx ...string) {
	if currentLevel() >= LevelQuiet {
		emit(os.Stderr, module, "FAIL", component, msg, ctx)
	}
}

// Trace logs a debug-level message to stdout. It is only emitted when
// the log level is set to trace.
func Trace(module, component, msg string, ctx ...string) {
	if currentLevel() >= LevelTrace {
		emit(os.Stdout, module, "TRACE", component, msg, ctx)
	}
}

func emit(w *os.File, module, level, component, msg string, ctx []string) {
	ts := time.Now().UTC().Format(time.RFC3339)
	ctxStr := ""
	if len(ctx) > 0 {
		ctxStr = " | " + strings.Join(ctx, ", ")
	}
	fmt.Fprintf(w, "[AA:%s:%s] %s | %s | %s%s\n", module, level, ts, component, msg, ctxStr)
}

// ---------------------------------------------------------------------------
// Prometheus Metrics
// ---------------------------------------------------------------------------

// TokensIssuedTotal counts the number of tokens issued, partitioned by the
// primary scope string granted in each token.
var TokensIssuedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "agentauth_tokens_issued_total",
	Help: "Total number of tokens issued",
}, []string{"scope"})

// TokensRevokedTotal counts revocation operations, partitioned by the
// revocation level ("token", "agent", "task", or "chain").
var TokensRevokedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "agentauth_tokens_revoked_total",
	Help: "Total number of tokens revoked",
}, []string{"level"})

// RegistrationsTotal counts agent registration attempts, partitioned by
// outcome ("success" or "failure").
var RegistrationsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "agentauth_registrations_total",
	Help: "Total number of agent registrations",
}, []string{"status"})

// AdminAuthTotal counts admin authentication attempts, partitioned by
// outcome ("success" or "failure").
var AdminAuthTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "agentauth_admin_auth_total",
	Help: "Total number of admin auth attempts",
}, []string{"status"})

// LaunchTokensCreatedTotal counts the total number of launch tokens created
// through the admin API.
var LaunchTokensCreatedTotal = promauto.NewCounter(prometheus.CounterOpts{
	Name: "agentauth_launch_tokens_created_total",
	Help: "Total number of launch tokens created",
})

// ActiveAgents tracks the current number of registered agents. It is
// incremented on successful registration; no decrement is implemented yet.
var ActiveAgents = promauto.NewGauge(prometheus.GaugeOpts{
	Name: "agentauth_active_agents",
	Help: "Number of currently active agents",
})

// RequestDuration observes HTTP request durations in seconds, partitioned
// by the logical endpoint name (e.g. "token_issue").
var RequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
	Name:    "agentauth_request_duration_seconds",
	Help:    "Request duration in seconds",
	Buckets: prometheus.DefBuckets,
}, []string{"endpoint"})

// ClockSkewTotal counts the number of times a clock-skew condition was
// detected during token validation (nbf in the future).
var ClockSkewTotal = promauto.NewCounter(prometheus.CounterOpts{
	Name: "agentauth_clock_skew_total",
	Help: "Number of clock skew events detected",
})

// RecordIssuance records a token issuance duration. The caller provides
// the elapsed time in milliseconds; it is converted to seconds before
// being observed in the [RequestDuration] histogram.
func RecordIssuance(ms float64) {
	RequestDuration.WithLabelValues("token_issue").Observe(ms / 1000.0)
}

// RecordClockSkew increments the [ClockSkewTotal] counter by one.
func RecordClockSkew() {
	ClockSkewTotal.Inc()
}
