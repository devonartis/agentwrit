package identity

import (
	"testing"
	"time"

	"github.com/divineartis/agentauth/internal/store"
)

func TestLaunchTokenValidAndSingleUse(t *testing.T) {
	sqlStore := store.NewSqlStore()
	token, err := CreateLaunchToken(sqlStore, "orch-1", "task-1", []string{"read:Customers:1"}, 2*time.Second)
	if err != nil {
		t.Fatalf("create launch token: %v", err)
	}
	data, err := ValidateLaunchToken(sqlStore, token)
	if err != nil {
		t.Fatalf("validate launch token: %v", err)
	}
	if data.OrchId != "orch-1" || data.TaskId != "task-1" {
		t.Fatalf("unexpected data: %+v", data)
	}
	if _, err := ValidateLaunchToken(sqlStore, token); err == nil {
		t.Fatalf("expected second use to fail")
	}
}

func TestLaunchTokenExpired(t *testing.T) {
	sqlStore := store.NewSqlStore()
	token, err := CreateLaunchToken(sqlStore, "orch-1", "task-1", []string{"read:Customers:1"}, 1*time.Second)
	if err != nil {
		t.Fatalf("create launch token: %v", err)
	}
	time.Sleep(1100 * time.Millisecond)
	if _, err := ValidateLaunchToken(sqlStore, token); err == nil {
		t.Fatalf("expected expired token to fail")
	}
}

func TestLaunchTokenMalformed(t *testing.T) {
	sqlStore := store.NewSqlStore()
	if _, err := ValidateLaunchToken(sqlStore, "not-a-real-token"); err == nil {
		t.Fatalf("expected malformed token to fail")
	}
}

