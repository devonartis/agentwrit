package audit

// Event type constants for audit trail categorization.
const (
	// EvtCredentialIssued records a new agent credential being issued.
	EvtCredentialIssued = "credential_issued" // #nosec G101 -- event type constant, not a credential
	// EvtAccessGranted records a successful authorization check.
	EvtAccessGranted = "access_granted"
	// EvtAccessDenied records a failed authorization check.
	EvtAccessDenied = "access_denied"
	// EvtTokenRevoked records a token revocation event.
	EvtTokenRevoked = "token_revoked"
	// EvtDelegationCreated records a delegation chain creation.
	EvtDelegationCreated = "delegation_created"
	// EvtDelegationRevoked records a delegation chain revocation.
	EvtDelegationRevoked = "delegation_revoked"
	// EvtAnomalyDetected records an anomaly detection event (M09 — not yet emitted).
	EvtAnomalyDetected = "anomaly_detected"
)

// AuditEvt represents a single immutable audit event in the hash chain.
type AuditEvt struct {
	EventId         string `json:"event_id"`
	EventType       string `json:"event_type"`
	Timestamp       string `json:"timestamp"`
	AgentInstanceId string `json:"agent_instance_id"`
	TaskId          string `json:"task_id"`
	OrchId          string `json:"orchestration_id"`
	Resource        string `json:"resource"`
	Action          string `json:"action"`
	Outcome         string `json:"outcome"`
	DenialReason    string `json:"denial_reason,omitempty"`
	DelegDepth      int    `json:"delegation_depth"`
	DelegChainHash  string `json:"delegation_chain_hash,omitempty"`
	PrevHash        string `json:"prev_hash"`
	EventHash       string `json:"event_hash"`
}

// AuditFilter holds optional query parameters for filtering audit events.
type AuditFilter struct {
	AgentId   string
	TaskId    string
	OrchId    string
	EventType string
	From      string // ISO 8601
	To        string // ISO 8601
	Limit     int    // default 100, max 1000
	Offset    int
}
