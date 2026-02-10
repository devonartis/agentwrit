package obs

import (
	"bytes"
	"strings"
	"testing"
)

func TestFailToStderrOnlyInQuiet(t *testing.T) {
	var out bytes.Buffer
	var err bytes.Buffer
	SetWriters(&out, &err)
	Configure("quiet")

	Ok("OBS", "HealthHdl.ServeHTTP", "health check", "status=healthy")
	Fail("TOKEN", "ValHdl.ServeHTTP", "validation failed", "reason=invalid")

	if out.Len() != 0 {
		t.Fatalf("expected no stdout in quiet, got: %q", out.String())
	}
	if !strings.Contains(err.String(), "[AA:TOKEN:FAIL]") {
		t.Fatalf("expected FAIL message in stderr, got: %q", err.String())
	}
}

func TestOkFormat(t *testing.T) {
	var out bytes.Buffer
	var err bytes.Buffer
	SetWriters(&out, &err)
	Configure("verbose")

	Ok("OBS", "HealthHdl.ServeHTTP", "health check", "status=healthy")
	got := out.String()
	if !strings.Contains(got, "[AA:OBS:OK]") {
		t.Fatalf("missing prefix: %q", got)
	}
	if !strings.Contains(got, "HealthHdl.ServeHTTP") {
		t.Fatalf("missing component: %q", got)
	}
	if !strings.Contains(got, "status=healthy") {
		t.Fatalf("missing context: %q", got)
	}
	if err.Len() != 0 {
		t.Fatalf("expected no stderr, got: %q", err.String())
	}
}

