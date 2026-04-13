// SPDX-License-Identifier: LicenseRef-PolyForm-Internal-Use-1.0.0

package identity

import (
	"fmt"
	"strings"

	"github.com/spiffe/go-spiffe/v2/spiffeid"
)

// NewSpiffeId constructs a SPIFFE ID in the AgentAuth canonical format:
//
//	spiffe://{trustDomain}/agent/{orchID}/{taskID}/{instanceID}
//
// It uses the go-spiffe library for trust domain and path segment
// validation. The returned string is suitable for use as an agent's
// unique identifier (token subject, store key, etc.).
func NewSpiffeId(trustDomain, orchID, taskID, instanceID string) (string, error) {
	td, err := spiffeid.TrustDomainFromString(trustDomain)
	if err != nil {
		return "", fmt.Errorf("invalid trust domain %q: %w", trustDomain, err)
	}

	id, err := spiffeid.FromSegments(td, "agent", orchID, taskID, instanceID)
	if err != nil {
		return "", fmt.Errorf("create SPIFFE ID: %w", err)
	}

	return id.String(), nil
}

// ParseSpiffeId validates a SPIFFE ID string and extracts its path
// components. The path must follow the AgentAuth format
// /agent/{orchID}/{taskID}/{instanceID}. It returns an error if the ID
// is malformed or does not match the expected structure.
func ParseSpiffeId(id string) (orchID, taskID, instanceID string, err error) {
	parsed, err := spiffeid.FromString(id)
	if err != nil {
		return "", "", "", fmt.Errorf("invalid SPIFFE ID %q: %w", id, err)
	}

	path := parsed.Path()
	// Expected path: /agent/{orchID}/{taskID}/{instanceID}
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(parts) != 4 || parts[0] != "agent" {
		return "", "", "", fmt.Errorf("invalid SPIFFE path format %q: expected /agent/{orchID}/{taskID}/{instanceID}", path)
	}

	return parts[1], parts[2], parts[3], nil
}
