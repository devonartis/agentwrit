package store

import (
	"errors"
	"testing"
	"time"
)

// --- Nonce lifecycle ---

func TestCreateNonce_Returns64HexChars(t *testing.T) {
	st := NewSqlStore()
	nonce := st.CreateNonce()
	if len(nonce) != 64 {
		t.Errorf("expected 64-char hex nonce, got %d chars", len(nonce))
	}
}

func TestConsumeNonce_Success(t *testing.T) {
	st := NewSqlStore()
	nonce := st.CreateNonce()

	if err := st.ConsumeNonce(nonce); err != nil {
		t.Fatalf("unexpected error consuming nonce: %v", err)
	}
}

func TestConsumeNonce_DoubleConsume(t *testing.T) {
	st := NewSqlStore()
	nonce := st.CreateNonce()

	if err := st.ConsumeNonce(nonce); err != nil {
		t.Fatalf("first consume: %v", err)
	}

	err := st.ConsumeNonce(nonce)
	if err != ErrNonceConsumed {
		t.Errorf("expected ErrNonceConsumed on double consume, got: %v", err)
	}
}

func TestConsumeNonce_NotFound(t *testing.T) {
	st := NewSqlStore()

	err := st.ConsumeNonce("nonexistent-nonce")
	if err != ErrNonceNotFound {
		t.Errorf("expected ErrNonceNotFound, got: %v", err)
	}
}

func TestConsumeNonce_Expired(t *testing.T) {
	st := NewSqlStore()
	nonce := st.CreateNonce()

	// Manually backdate the expiry.
	st.mu.Lock()
	st.nonces[nonce].expiresAt = time.Now().Add(-1 * time.Second)
	st.mu.Unlock()

	err := st.ConsumeNonce(nonce)
	if err != ErrNonceNotFound {
		t.Errorf("expected ErrNonceNotFound for expired nonce, got: %v", err)
	}
}

func TestCreateNonce_Uniqueness(t *testing.T) {
	st := NewSqlStore()
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		nonce := st.CreateNonce()
		if seen[nonce] {
			t.Fatalf("duplicate nonce at iteration %d", i)
		}
		seen[nonce] = true
	}
}

// --- Launch Token lifecycle ---

func TestSaveLaunchToken_AndGet(t *testing.T) {
	st := NewSqlStore()

	rec := LaunchTokenRecord{
		Token:        "test-token-abc",
		AgentName:    "data-reader",
		AllowedScope: []string{"read:Customers:*"},
		MaxTTL:       300,
		SingleUse:    true,
		CreatedAt:    time.Now().UTC(),
		ExpiresAt:    time.Now().UTC().Add(60 * time.Second),
		CreatedBy:    "admin",
	}

	if err := st.SaveLaunchToken(rec); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := st.GetLaunchToken("test-token-abc")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.AgentName != "data-reader" {
		t.Errorf("expected agent_name=data-reader, got %s", got.AgentName)
	}
	if len(got.AllowedScope) != 1 || got.AllowedScope[0] != "read:Customers:*" {
		t.Errorf("unexpected scope: %v", got.AllowedScope)
	}
}

func TestGetLaunchToken_NotFound(t *testing.T) {
	st := NewSqlStore()

	_, err := st.GetLaunchToken("nonexistent")
	if err != ErrTokenNotFound {
		t.Errorf("expected ErrTokenNotFound, got: %v", err)
	}
}

func TestGetLaunchToken_Expired(t *testing.T) {
	st := NewSqlStore()

	rec := LaunchTokenRecord{
		Token:     "expired-token",
		ExpiresAt: time.Now().UTC().Add(-1 * time.Second),
	}
	_ = st.SaveLaunchToken(rec) //nolint:errcheck // test setup

	_, err := st.GetLaunchToken("expired-token")
	if err != ErrTokenExpired {
		t.Errorf("expected ErrTokenExpired, got: %v", err)
	}
}

func TestConsumeLaunchToken_Success(t *testing.T) {
	st := NewSqlStore()

	rec := LaunchTokenRecord{
		Token:     "consume-me",
		ExpiresAt: time.Now().UTC().Add(60 * time.Second),
	}
	_ = st.SaveLaunchToken(rec) //nolint:errcheck // test setup

	if err := st.ConsumeLaunchToken("consume-me"); err != nil {
		t.Fatalf("consume: %v", err)
	}

	// After consumption, GetLaunchToken should return ErrTokenConsumed.
	_, err := st.GetLaunchToken("consume-me")
	if err != ErrTokenConsumed {
		t.Errorf("expected ErrTokenConsumed after consumption, got: %v", err)
	}
}

func TestConsumeLaunchToken_DoubleConsume(t *testing.T) {
	st := NewSqlStore()

	rec := LaunchTokenRecord{
		Token:     "double-consume",
		ExpiresAt: time.Now().UTC().Add(60 * time.Second),
	}
	_ = st.SaveLaunchToken(rec) //nolint:errcheck // test setup

	_ = st.ConsumeLaunchToken("double-consume") //nolint:errcheck // test setup
	err := st.ConsumeLaunchToken("double-consume")
	if err != ErrTokenConsumed {
		t.Errorf("expected ErrTokenConsumed on double consume, got: %v", err)
	}
}

func TestConsumeLaunchToken_NotFound(t *testing.T) {
	st := NewSqlStore()

	err := st.ConsumeLaunchToken("nonexistent")
	if err != ErrTokenNotFound {
		t.Errorf("expected ErrTokenNotFound, got: %v", err)
	}
}

// --- Agent CRUD ---

func TestSaveAgent_AndGet(t *testing.T) {
	st := NewSqlStore()

	rec := AgentRecord{
		AgentID:      "spiffe://test/agent/o/t/i",
		PublicKey:    []byte("fake-key-bytes"),
		OrchID:       "orch-1",
		TaskID:       "task-1",
		Scope:        []string{"read:data:*"},
		RegisteredAt: time.Now(),
		LastSeen:     time.Now(),
	}

	if err := st.SaveAgent(rec); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := st.GetAgent("spiffe://test/agent/o/t/i")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.OrchID != "orch-1" {
		t.Errorf("expected orch_id=orch-1, got %s", got.OrchID)
	}
	if got.TaskID != "task-1" {
		t.Errorf("expected task_id=task-1, got %s", got.TaskID)
	}
}

func TestGetAgent_NotFound(t *testing.T) {
	st := NewSqlStore()

	_, err := st.GetAgent("nonexistent")
	if err != ErrAgentNotFound {
		t.Errorf("expected ErrAgentNotFound, got: %v", err)
	}
}

func TestUpdateAgentLastSeen(t *testing.T) {
	st := NewSqlStore()

	past := time.Now().Add(-1 * time.Hour)
	rec := AgentRecord{
		AgentID:  "spiffe://test/agent/o/t/ls",
		LastSeen: past,
	}
	_ = st.SaveAgent(rec) //nolint:errcheck // test setup

	if err := st.UpdateAgentLastSeen("spiffe://test/agent/o/t/ls"); err != nil {
		t.Fatalf("update: %v", err)
	}

	got, _ := st.GetAgent("spiffe://test/agent/o/t/ls")
	if !got.LastSeen.After(past) {
		t.Error("expected LastSeen to be updated to a more recent time")
	}
}

func TestUpdateAgentLastSeen_NotFound(t *testing.T) {
	st := NewSqlStore()

	err := st.UpdateAgentLastSeen("nonexistent")
	if err != ErrAgentNotFound {
		t.Errorf("expected ErrAgentNotFound, got: %v", err)
	}
}

func TestSaveAgent_Overwrite(t *testing.T) {
	st := NewSqlStore()

	rec1 := AgentRecord{
		AgentID: "spiffe://test/agent/o/t/ow",
		Scope:   []string{"read:data:*"},
	}
	_ = st.SaveAgent(rec1) //nolint:errcheck // test setup

	rec2 := AgentRecord{
		AgentID: "spiffe://test/agent/o/t/ow",
		Scope:   []string{"write:data:*"},
	}
	_ = st.SaveAgent(rec2) //nolint:errcheck // test setup

	got, _ := st.GetAgent("spiffe://test/agent/o/t/ow")
	if len(got.Scope) != 1 || got.Scope[0] != "write:data:*" {
		t.Errorf("expected overwritten scope [write:data:*], got %v", got.Scope)
	}
}

// --- Sidecar ceiling ---

func TestSaveCeiling_AndGet(t *testing.T) {
	st := NewSqlStore()

	err := st.SaveCeiling("sc-001", []string{"read:tickets:*", "write:tickets:metadata"})
	if err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := st.GetCeiling("sc-001")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if len(got) != 2 || got[0] != "read:tickets:*" {
		t.Errorf("expected [read:tickets:* write:tickets:metadata], got %v", got)
	}
}

func TestGetCeiling_NotFound(t *testing.T) {
	st := NewSqlStore()

	_, err := st.GetCeiling("nonexistent")
	if !errors.Is(err, ErrCeilingNotFound) {
		t.Errorf("expected ErrCeilingNotFound, got %v", err)
	}
}

func TestSaveCeiling_Overwrite(t *testing.T) {
	st := NewSqlStore()

	_ = st.SaveCeiling("sc-001", []string{"read:tickets:*"})
	_ = st.SaveCeiling("sc-001", []string{"read:tickets:*", "write:tickets:metadata"})

	got, err := st.GetCeiling("sc-001")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("expected overwrite to 2 scopes, got %d", len(got))
	}
}
