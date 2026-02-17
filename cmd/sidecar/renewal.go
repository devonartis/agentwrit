package main

import (
	"context"
	"strings"
	"time"

	"github.com/divineartis/agentauth/internal/obs"
)

// renewFunc is the function signature for renewing a token.
// Matches brokerClient.tokenRenew.
type renewFunc func(token string) (*renewResp, error)

const maxBackoff = 30 * time.Second

// startRenewal runs a blocking loop that renews the sidecar's bearer token
// before it expires. It blocks until ctx is cancelled.
//
// If ceiling is non-nil and the broker returns a scope_ceiling in the renewal
// response, the ceiling cache is updated atomically.
//
// renewalBuffer is the fraction of TTL at which to renew (e.g. 0.8 = renew
// at 80% of TTL). On failure, retries with exponential backoff capped at
// maxBackoff. If the token expires (all retries failed), state is marked
// unhealthy. Auto-recovers when renewal eventually succeeds.
func startRenewal(ctx context.Context, state *sidecarState, renew renewFunc, renewalBuffer float64, ceiling *ceilingCache) {
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
		resp, err := renew(currentToken)
		if err != nil {
			obs.Warn("SIDECAR", "RENEWAL", "renewal failed", err.Error(), "retry_in="+backoff.String())
			RecordRenewal("failure")

			if time.Now().After(tokenDeadline) {
				state.setHealthy(false)
				obs.Warn("SIDECAR", "RENEWAL", "token expired, marking unhealthy")
			}

			sleepDur = backoff
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			continue
		}

		state.setToken(resp.AccessToken, resp.ExpiresIn)
		tokenDeadline = time.Now().Add(time.Duration(resp.ExpiresIn) * time.Second)
		sleepDur = time.Duration(float64(resp.ExpiresIn)*renewalBuffer) * time.Second
		if sleepDur < 1*time.Second {
			sleepDur = 1 * time.Second
		}
		backoff = 1 * time.Second

		// Update scope ceiling if broker sent one.
		if ceiling != nil && len(resp.ScopeCeiling) > 0 {
			ceiling.set(resp.ScopeCeiling)
			obs.Ok("SIDECAR", "RENEWAL", "scope ceiling updated",
				"ceiling="+strings.Join(resp.ScopeCeiling, ","))
		}

		obs.Trace("SIDECAR", "RENEWAL", "token renewed", "next_in="+sleepDur.String())
		RecordRenewal("success")
	}
}
