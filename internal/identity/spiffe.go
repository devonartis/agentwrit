package identity

import (
	"errors"
	"fmt"
	"strings"
)

var (
	// ErrSpiffeInvalid indicates a SPIFFE ID format violation.
	ErrSpiffeInvalid = errors.New("invalid spiffe id")
)

// SpiffeId represents a parsed SPIFFE identifier.
type SpiffeId struct {
	TrustDomain string
	OrchId      string
	TaskId      string
	InstanceId  string
	Raw         string
}

// NewSpiffeId builds a SPIFFE ID in the canonical AgentAuth format.
func NewSpiffeId(trustDomain, orchId, taskId, instanceId string) string {
	return fmt.Sprintf("spiffe://%s/agent/%s/%s/%s", trustDomain, orchId, taskId, instanceId)
}

// ParseSpiffeId parses a SPIFFE ID string into structured fields.
func ParseSpiffeId(id string) (*SpiffeId, error) {
	if err := ValidateSpiffeId(id); err != nil {
		return nil, err
	}

	trimmed := strings.TrimPrefix(id, "spiffe://")
	parts := strings.Split(trimmed, "/")
	return &SpiffeId{
		TrustDomain: parts[0],
		OrchId:      parts[2],
		TaskId:      parts[3],
		InstanceId:  parts[4],
		Raw:         id,
	}, nil
}

// ValidateSpiffeId validates the AgentAuth SPIFFE ID format.
func ValidateSpiffeId(id string) error {
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("%w: empty", ErrSpiffeInvalid)
	}
	if !strings.HasPrefix(id, "spiffe://") {
		return fmt.Errorf("%w: missing spiffe:// prefix", ErrSpiffeInvalid)
	}

	trimmed := strings.TrimPrefix(id, "spiffe://")
	if strings.Contains(trimmed, " ") {
		return fmt.Errorf("%w: whitespace not allowed", ErrSpiffeInvalid)
	}

	parts := strings.Split(trimmed, "/")
	if len(parts) != 5 {
		return fmt.Errorf("%w: expected 5 segments", ErrSpiffeInvalid)
	}
	if parts[1] != "agent" {
		return fmt.Errorf("%w: missing agent segment", ErrSpiffeInvalid)
	}

	segments := []string{parts[0], parts[2], parts[3], parts[4]}
	for _, seg := range segments {
		if strings.TrimSpace(seg) == "" {
			return fmt.Errorf("%w: empty segment", ErrSpiffeInvalid)
		}
		if strings.Contains(seg, "/") {
			return fmt.Errorf("%w: slash in segment", ErrSpiffeInvalid)
		}
	}
	return nil
}

