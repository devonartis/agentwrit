package main

import (
	"context"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func TestStartListener_UDS(t *testing.T) {
	sockPath := filepath.Join(t.TempDir(), "test.sock")

	ln, cleanup, err := startListener(sockPath, "8081")
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	defer ln.Close()

	if ln.Addr().Network() != "unix" {
		t.Fatalf("expected unix, got %s", ln.Addr().Network())
	}

	// Serve a ping handler on the socket.
	mux := http.NewServeMux()
	mux.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("pong"))
	})
	go http.Serve(ln, mux)

	// Connect via UDS.
	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", sockPath)
			},
		},
	}
	resp, err := client.Get("http://unix/ping")
	if err != nil {
		t.Fatalf("GET via UDS: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "pong" {
		t.Fatalf("expected pong, got %q", string(body))
	}
}

func TestStartListener_TCP(t *testing.T) {
	ln, cleanup, err := startListener("", "0") // port 0 = random
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	defer ln.Close()

	if ln.Addr().Network() != "tcp" {
		t.Fatalf("expected tcp, got %s", ln.Addr().Network())
	}
}

func TestStartListener_UDS_CleansUpStaleSocket(t *testing.T) {
	// Use /tmp directly — t.TempDir() paths exceed Unix socket max (104 chars on macOS).
	sockPath := filepath.Join("/tmp", "aa-test-stale.sock")
	defer os.Remove(sockPath)

	// Create a first listener to make a socket file.
	ln1, _, err := startListener(sockPath, "8081")
	if err != nil {
		t.Fatal(err)
	}
	ln1.Close()
	// Don't call cleanup — leave the stale socket file on disk.

	// Second listener should succeed even though file existed.
	ln2, cleanup2, err := startListener(sockPath, "8081")
	if err != nil {
		t.Fatalf("expected success replacing stale socket: %v", err)
	}
	defer cleanup2()
	defer ln2.Close()
}
