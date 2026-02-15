package main

import (
	"context"
	"fmt"
	"time"
)

// renewFunc is the function signature for renewing a token.
// Matches brokerClient.tokenRenew.
type renewFunc func(token string) (newToken string, expiresIn int, err error)

const maxBackoff = 30 * time.Second

// startRenewal runs a blocking loop that renews the sidecar's bearer token
// before it expires. It blocks until ctx is cancelled.
//
// renewalBuffer is the fraction of TTL at which to renew (e.g. 0.8 = renew
// at 80% of TTL). On failure, retries with exponential backoff capped at
// maxBackoff. If the token expires (all retries failed), state is marked
// unhealthy. Auto-recovers when renewal eventually succeeds.
func startRenewal(ctx context.Context, state *sidecarState, renew renewFunc, renewalBuffer float64) {
	ttl := state.getExpiresIn()
	sleepDur := time.Duration(float64(ttl)*renewalBuffer) * time.Second
	if sleepDur < 1*time.Second {
		sleepDur = 1 * time.Second
	}

	tokenDeadline := time.Now().Add(time.Duration(ttl) * time.Second)
	backoff := 1 * time.Second

	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(sleepDur):
		}

		currentToken := state.getToken()
		newToken, newTTL, err := renew(currentToken)
		if err != nil {
			fmt.Printf("[sidecar] renewal failed: %v (retry in %v)\n", err, backoff)

			if time.Now().After(tokenDeadline) {
				state.setHealthy(false)
				fmt.Println("[sidecar] token expired, marking unhealthy")
			}

			sleepDur = backoff
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			continue
		}

		state.setToken(newToken, newTTL)
		tokenDeadline = time.Now().Add(time.Duration(newTTL) * time.Second)
		sleepDur = time.Duration(float64(newTTL)*renewalBuffer) * time.Second
		if sleepDur < 1*time.Second {
			sleepDur = 1 * time.Second
		}
		backoff = 1 * time.Second

		fmt.Printf("[sidecar] token renewed, next in %v\n", sleepDur)
	}
}
