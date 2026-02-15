package main

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func TestSidecarMetricsRegistered(t *testing.T) {
	metrics := []prometheus.Collector{
		SidecarBootstrapTotal,
		SidecarRenewalsTotal,
		SidecarTokenExchangesTotal,
		SidecarScopeDenialsTotal,
		SidecarAgentsRegistered,
		SidecarRequestDuration,
	}
	for i, m := range metrics {
		if m == nil {
			t.Errorf("metric %d is nil", i)
		}
	}
}

func TestRecordSidecarBootstrap(t *testing.T) {
	RecordBootstrap("success")
	RecordBootstrap("failure")
}

func TestRecordSidecarRenewal(t *testing.T) {
	RecordRenewal("success")
	RecordRenewal("failure")
}

func TestRecordSidecarExchange(t *testing.T) {
	RecordExchange("success")
	RecordExchange("failure")
}

func TestRecordSidecarScopeDenial(t *testing.T) {
	RecordScopeDenial()
}

func TestCircuitBreakerMetrics_NotNil(t *testing.T) {
	if SidecarCircuitState == nil {
		t.Error("SidecarCircuitState is nil")
	}
	if SidecarCircuitTripsTotal == nil {
		t.Error("SidecarCircuitTripsTotal is nil")
	}
	if SidecarCachedTokensServedTotal == nil {
		t.Error("SidecarCachedTokensServedTotal is nil")
	}
}
