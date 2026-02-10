package store

import (
	"errors"
	"testing"
	"time"
)

func TestConsumeActivationToken(t *testing.T) {
	st := NewSqlStore()
	jti := "test-jti-123"
	exp := time.Now().Add(1 * time.Hour).Unix()

	// Red Phase: This should fail to compile because ConsumeActivationToken is missing
	err := st.ConsumeActivationToken(jti, exp)
	if err != nil {
		t.Fatalf("first consumption should succeed, got: %v", err)
	}

	// Replay attempt should fail
	err = st.ConsumeActivationToken(jti, exp)
	if !errors.Is(err, ErrTokenConsumed) {
		t.Errorf("second consumption should return ErrTokenConsumed, got: %v", err)
	}
}
