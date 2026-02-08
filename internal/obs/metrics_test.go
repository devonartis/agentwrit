package obs

import (
	"math"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	dto "github.com/prometheus/client_model/go"
)

func histogramCount(t *testing.T, collector interface{ Write(*dto.Metric) error }) uint64 {
	t.Helper()
	m := &dto.Metric{}
	if err := collector.Write(m); err != nil {
		t.Fatalf("read histogram metric: %v", err)
	}
	return m.GetHistogram().GetSampleCount()
}

func almostEqual(a, b float64) bool {
	const eps = 1e-9
	return math.Abs(a-b) < eps
}

func TestRecordIssuance(t *testing.T) {
	ensureMetricsRegistered()
	before := histogramCount(t, tokenIssuanceDurationMs)
	RecordIssuance(12.5)
	after := histogramCount(t, tokenIssuanceDurationMs)
	if after != before+1 {
		t.Fatalf("expected issuance histogram count %d, got %d", before+1, after)
	}
}

func TestRecordValidation(t *testing.T) {
	ensureMetricsRegistered()
	allowBefore := testutil.ToFloat64(validationDecision.WithLabelValues("allow"))
	denyBefore := testutil.ToFloat64(validationDecision.WithLabelValues("deny"))

	RecordValidation(true)
	RecordValidation(false)

	allowAfter := testutil.ToFloat64(validationDecision.WithLabelValues("allow"))
	denyAfter := testutil.ToFloat64(validationDecision.WithLabelValues("deny"))

	if !almostEqual(allowAfter, allowBefore+1) {
		t.Fatalf("expected allow counter %f, got %f", allowBefore+1, allowAfter)
	}
	if !almostEqual(denyAfter, denyBefore+1) {
		t.Fatalf("expected deny counter %f, got %f", denyBefore+1, denyAfter)
	}
}

func TestSetRevocationCacheHitRatio(t *testing.T) {
	ensureMetricsRegistered()
	SetRevocationCacheHitRatio(0.42)
	if got := testutil.ToFloat64(revocationCacheHitRatio); !almostEqual(got, 0.42) {
		t.Fatalf("expected 0.42, got %f", got)
	}
	SetRevocationCacheHitRatio(2)
	if got := testutil.ToFloat64(revocationCacheHitRatio); !almostEqual(got, 1) {
		t.Fatalf("expected clamped 1.0, got %f", got)
	}
}

func TestRecordClockSkew(t *testing.T) {
	ensureMetricsRegistered()
	before := testutil.ToFloat64(clockSkewDetected)
	RecordClockSkew()
	after := testutil.ToFloat64(clockSkewDetected)
	if !almostEqual(after, before+1) {
		t.Fatalf("expected counter %f, got %f", before+1, after)
	}
}

func TestRecordDelegationDepth(t *testing.T) {
	ensureMetricsRegistered()
	before := histogramCount(t, delegationChainDepth)
	RecordDelegationDepth(3)
	after := histogramCount(t, delegationChainDepth)
	if after != before+1 {
		t.Fatalf("expected delegation depth histogram count %d, got %d", before+1, after)
	}
}

func TestRecordAnomalyRevocation(t *testing.T) {
	ensureMetricsRegistered()
	before := testutil.ToFloat64(anomalyRevocationTriggered)
	RecordAnomalyRevocation()
	after := testutil.ToFloat64(anomalyRevocationTriggered)
	if !almostEqual(after, before+1) {
		t.Fatalf("expected anomaly counter %f, got %f", before+1, after)
	}
}

func TestSetHeartbeatMissRate(t *testing.T) {
	ensureMetricsRegistered()
	SetHeartbeatMissRate(0.125)
	if got := testutil.ToFloat64(heartbeatMissRate); !almostEqual(got, 0.125) {
		t.Fatalf("expected 0.125, got %f", got)
	}
	SetHeartbeatMissRate(-1)
	if got := testutil.ToFloat64(heartbeatMissRate); !almostEqual(got, 0) {
		t.Fatalf("expected clamped 0, got %f", got)
	}
}
