package main

import (
	"fmt"
	"net"
	"os"

	"github.com/divineartis/agentauth/internal/obs"
)

// startListener creates either a Unix domain socket or TCP listener based
// on configuration. Returns the listener, a cleanup function, and any error.
func startListener(socketPath, port string) (net.Listener, func(), error) {
	if socketPath != "" {
		// Remove stale socket file if it exists.
		os.Remove(socketPath)

		ln, err := net.Listen("unix", socketPath)
		if err != nil {
			return nil, nil, fmt.Errorf("listen unix %s: %w", socketPath, err)
		}

		// Set socket permissions to 0660 (owner + group read/write).
		if err := os.Chmod(socketPath, 0660); err != nil {
			ln.Close()
			return nil, nil, fmt.Errorf("chmod socket %s: %w", socketPath, err)
		}

		cleanup := func() {
			os.Remove(socketPath)
		}

		obs.Ok("SIDECAR", "MAIN", "listening on unix socket", "path="+socketPath)
		return ln, cleanup, nil
	}

	// TCP fallback.
	addr := ":" + port
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, nil, fmt.Errorf("listen tcp %s: %w", addr, err)
	}

	obs.Warn("SIDECAR", "MAIN", "listening on TCP — consider AA_SOCKET_PATH for production deployments", "addr="+addr)
	cleanup := func() {} // nothing to clean up for TCP

	return ln, cleanup, nil
}
