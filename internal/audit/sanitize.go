package audit

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
)

var (
	emailRe = regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`)
	phoneRe = regexp.MustCompile(`\+?[0-9][\d\-\s()]{7,}[0-9]`)
	// Customer ID pattern: standalone alphanumeric ID of 6+ characters.
	custIdRe = regexp.MustCompile(`\b[A-Z]{0,4}[0-9]{6,}\b`)
)

// Sanitize returns a copy of evt with PII redacted:
//   - Customer IDs hashed with consistent SHA-256 (preserves forensic linking)
//   - Email addresses redacted to [REDACTED_EMAIL]
//   - Phone numbers redacted to [REDACTED_PHONE]
func Sanitize(evt *AuditEvt) *AuditEvt {
	out := *evt
	out.Resource = sanitizeField(out.Resource)
	out.Action = sanitizeField(out.Action)
	return &out
}

func sanitizeField(s string) string {
	s = emailRe.ReplaceAllString(s, "[REDACTED_EMAIL]")
	s = phoneRe.ReplaceAllString(s, "[REDACTED_PHONE]")
	s = custIdRe.ReplaceAllStringFunc(s, hashCustId)
	return s
}

func hashCustId(id string) string {
	h := sha256.Sum256([]byte(id))
	return fmt.Sprintf("[CID:%s]", hex.EncodeToString(h[:8]))
}

// AggregateReads combines consecutive read events for the same agent+resource
// into a single summary event. Failures and writes are never aggregated.
func AggregateReads(events []AuditEvt) []AuditEvt {
	if len(events) == 0 {
		return nil
	}
	var result []AuditEvt
	i := 0
	for i < len(events) {
		evt := events[i]
		if !isAggregatable(evt) {
			result = append(result, evt)
			i++
			continue
		}
		count := 1
		last := evt
		for i+count < len(events) {
			next := events[i+count]
			if !isAggregatable(next) ||
				next.AgentInstanceId != evt.AgentInstanceId ||
				next.Resource != evt.Resource {
				break
			}
			last = next
			count++
		}
		if count == 1 {
			result = append(result, evt)
		} else {
			summary := evt
			summary.Action = fmt.Sprintf("read (x%d)", count)
			summary.Timestamp = last.Timestamp
			result = append(result, summary)
		}
		i += count
	}
	return result
}

func isAggregatable(evt AuditEvt) bool {
	return evt.Outcome == "granted" && evt.EventType == EvtAccessGranted
}
