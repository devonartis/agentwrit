package obs

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

var registerMetricsOnce sync.Once

var (
	tokenIssuanceDurationMs = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "aa_token_issuance_duration_ms",
		Help:    "Duration of token issuance operations in milliseconds.",
		Buckets: []float64{1, 5, 10, 25, 50, 100, 250, 500, 1000, 2500, 5000},
	})

	validationDecision = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "aa_validation_decision_total",
		Help: "Count of allow/deny validation decisions.",
	}, []string{"decision"})

	revocationCacheHitRatio = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "aa_revocation_cache_hit_ratio",
		Help: "Current revocation cache hit ratio in range [0,1].",
	})

	clockSkewDetected = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "aa_clock_skew_detected_total",
		Help: "Number of detected clock-skew events.",
	})

	delegationChainDepth = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "aa_delegation_chain_depth",
		Help:    "Observed delegation chain depths.",
		Buckets: []float64{0, 1, 2, 3, 4, 5, 8, 13},
	})

	anomalyRevocationTriggered = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "aa_anomaly_revocation_triggered_total",
		Help: "Number of anomaly-triggered revocations.",
	})

	heartbeatMissRate = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "aa_heartbeat_miss_rate",
		Help: "Current heartbeat miss rate in range [0,1].",
	})
)

func ensureMetricsRegistered() {
	registerMetricsOnce.Do(func() {
		prometheus.MustRegister(
			tokenIssuanceDurationMs,
			validationDecision,
			revocationCacheHitRatio,
			clockSkewDetected,
			delegationChainDepth,
			anomalyRevocationTriggered,
			heartbeatMissRate,
		)
	})
}

// RegisterMetrics registers all AgentAuth Prometheus collectors exactly once.
func RegisterMetrics() {
	ensureMetricsRegistered()
}

// RecordIssuance records token issuance duration in milliseconds.
func RecordIssuance(durationMs float64) {
	ensureMetricsRegistered()
	if durationMs < 0 {
		durationMs = 0
	}
	tokenIssuanceDurationMs.Observe(durationMs)
}

// RecordValidation increments validation decision counters with allow/deny label.
func RecordValidation(allowed bool) {
	ensureMetricsRegistered()
	decision := "deny"
	if allowed {
		decision = "allow"
	}
	validationDecision.WithLabelValues(decision).Inc()
}

// SetRevocationCacheHitRatio sets the revocation cache hit ratio gauge.
func SetRevocationCacheHitRatio(ratio float64) {
	ensureMetricsRegistered()
	if ratio < 0 {
		ratio = 0
	}
	if ratio > 1 {
		ratio = 1
	}
	revocationCacheHitRatio.Set(ratio)
}

// RecordClockSkew increments the clock-skew detection counter.
func RecordClockSkew() {
	ensureMetricsRegistered()
	clockSkewDetected.Inc()
}

// RecordDelegationDepth records a delegation chain depth observation.
func RecordDelegationDepth(depth float64) {
	ensureMetricsRegistered()
	if depth < 0 {
		depth = 0
	}
	delegationChainDepth.Observe(depth)
}

// RecordAnomalyRevocation increments anomaly-triggered revocation count.
func RecordAnomalyRevocation() {
	ensureMetricsRegistered()
	anomalyRevocationTriggered.Inc()
}

// SetHeartbeatMissRate sets the current heartbeat miss-rate gauge.
func SetHeartbeatMissRate(rate float64) {
	ensureMetricsRegistered()
	if rate < 0 {
		rate = 0
	}
	if rate > 1 {
		rate = 1
	}
	heartbeatMissRate.Set(rate)
}
