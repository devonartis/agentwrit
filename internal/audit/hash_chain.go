package audit

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// HashEvent computes the SHA-256 hash of the event fields concatenated with prevHash.
// The hash covers all domain fields to ensure tamper evidence — changing any field
// invalidates the hash and breaks the chain.
func HashEvent(evt *AuditEvt, prevHash string) string {
	data := fmt.Sprintf("%s|%s|%s|%s|%s|%s|%s|%s|%s|%s|%d|%s|%s",
		evt.EventId,
		evt.EventType,
		evt.Timestamp,
		evt.AgentInstanceId,
		evt.TaskId,
		evt.OrchId,
		evt.Resource,
		evt.Action,
		evt.Outcome,
		evt.DenialReason,
		evt.DelegDepth,
		evt.DelegChainHash,
		prevHash,
	)
	h := sha256.Sum256([]byte(data))
	return hex.EncodeToString(h[:])
}

// VerifyChain validates the integrity of a sequence of audit events.
// Returns (valid, firstInvalidIndex). For empty chains, returns (true, -1).
func VerifyChain(events []AuditEvt) (bool, int) {
	if len(events) == 0 {
		return true, -1
	}
	for i, evt := range events {
		expected := HashEvent(&evt, evt.PrevHash)
		if evt.EventHash != expected {
			return false, i
		}
		if i > 0 && evt.PrevHash != events[i-1].EventHash {
			return false, i
		}
	}
	return true, -1
}
