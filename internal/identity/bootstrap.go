package identity

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/divineartis/agentauth/internal/obs"
	"github.com/divineartis/agentauth/internal/store"
)

// CreateLaunchToken generates a cryptographically random launch token and persists it in the store.
func CreateLaunchToken(sqlStore *store.SqlStore, orchId, taskId string, scope []string, ttl time.Duration) (string, error) {
	if sqlStore == nil {
		return "", fmt.Errorf("sql store is required")
	}
	if ttl <= 0 {
		ttl = 30 * time.Second
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		obs.Fail("IDENTITY", "CreateLaunchToken", "token generation failed", "error="+err.Error())
		return "", err
	}
	token := hex.EncodeToString(raw)
	err := sqlStore.CreateLaunchToken(store.LaunchTokenData{
		Token:     token,
		OrchId:    orchId,
		TaskId:    taskId,
		Scope:     append([]string{}, scope...),
		ExpiresAt: time.Now().UTC().Add(ttl),
	})
	if err != nil {
		obs.Fail("IDENTITY", "CreateLaunchToken", "token persist failed", "error="+err.Error())
		return "", err
	}
	obs.Ok("IDENTITY", "CreateLaunchToken", "launch token created", "orch_id="+orchId, "task_id="+taskId)
	return token, nil
}

// ValidateLaunchToken consumes and validates a launch token, returning its associated data.
func ValidateLaunchToken(sqlStore *store.SqlStore, token string) (*store.LaunchTokenData, error) {
	if sqlStore == nil {
		return nil, fmt.Errorf("sql store is required")
	}
	data, err := sqlStore.ConsumeLaunchToken(token)
	if err != nil {
		obs.Fail("IDENTITY", "ValidateLaunchToken", "launch token rejected", "error="+err.Error())
		return nil, err
	}
	obs.Ok("IDENTITY", "ValidateLaunchToken", "launch token consumed", "orch_id="+data.OrchId, "task_id="+data.TaskId)
	return data, nil
}

