package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"testing"
)

// TestMultiSidecarUDS proves two sidecars can run on the same host using
// different Unix sockets, and a client can independently connect to each.
// This is the core value proposition of Fix 5: no port conflicts, no
// network exposure, N sidecars per host.
func TestMultiSidecarUDS(t *testing.T) {
	// Use /tmp to stay within Unix socket path length limit (104 chars on macOS).
	sock1 := "/tmp/aa-multi-test-1.sock"
	sock2 := "/tmp/aa-multi-test-2.sock"

	// Start two listeners on different socket paths.
	ln1, cleanup1, err := startListener(sock1, "0")
	if err != nil {
		t.Fatalf("sidecar-1 listen: %v", err)
	}
	defer cleanup1()
	defer ln1.Close()

	ln2, cleanup2, err := startListener(sock2, "0")
	if err != nil {
		t.Fatalf("sidecar-2 listen: %v", err)
	}
	defer cleanup2()
	defer ln2.Close()

	// Each sidecar serves a unique identity response (simulating /v1/health).
	serve := func(ln net.Listener, id string) {
		mux := http.NewServeMux()
		mux.HandleFunc("/v1/health", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintf(w, `{"sidecar_id":"%s","status":"ok"}`, id)
		})
		http.Serve(ln, mux)
	}
	go serve(ln1, "sidecar-app1")
	go serve(ln2, "sidecar-app2")

	// Build a UDS-aware HTTP client for a given socket path.
	udsClient := func(sockPath string) *http.Client {
		return &http.Client{
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
					return net.Dial("unix", sockPath)
				},
			},
		}
	}

	client1 := udsClient(sock1)
	client2 := udsClient(sock2)

	// Hit both sidecars concurrently — proves no interference.
	var wg sync.WaitGroup
	results := make([]string, 2)
	errors := make([]error, 2)

	wg.Add(2)
	go func() {
		defer wg.Done()
		results[0], errors[0] = fetchHealth(client1)
	}()
	go func() {
		defer wg.Done()
		results[1], errors[1] = fetchHealth(client2)
	}()
	wg.Wait()

	for i, err := range errors {
		if err != nil {
			t.Fatalf("sidecar-%d: %v", i+1, err)
		}
	}

	// Each sidecar returns its own identity.
	if results[0] != `{"sidecar_id":"sidecar-app1","status":"ok"}` {
		t.Errorf("sidecar-1 response: %s", results[0])
	}
	if results[1] != `{"sidecar_id":"sidecar-app2","status":"ok"}` {
		t.Errorf("sidecar-2 response: %s", results[1])
	}
}

func fetchHealth(c *http.Client) (string, error) {
	resp, err := c.Get("http://unix/v1/health")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}
