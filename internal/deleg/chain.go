package deleg

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/divineartis/agentauth/internal/revoke"
	"github.com/divineartis/agentauth/internal/token"
)

// ChainError describes a verification failure at a specific hop in a delegation chain.
type ChainError struct {
	Hop    int    `json:"hop"`
	Reason string `json:"reason"`
	Detail string `json:"detail"`
}

// Error implements the error interface for ChainError.
func (e *ChainError) Error() string {
	return fmt.Sprintf("chain verification failed at hop %d: %s — %s", e.Hop, e.Reason, e.Detail)
}

// VerifyChain validates a delegation chain's integrity, scope attenuation, and revocation status.
// It checks that every DelegRecord has a valid Ed25519 signature, that scope narrows or stays
// equal at each hop, and that no agent in the chain is revoked. The pubKey is the broker's
// signing key used when creating delegation records.
func VerifyChain(chain []token.DelegRecord, finalScope []string, revChecker revoke.RevChecker, pubKey ed25519.PublicKey) (bool, *ChainError) {
	if len(chain) == 0 {
		return true, nil
	}

	// Verify each hop in the chain.
	for i, rec := range chain {
		// Verify Ed25519 signature on the record.
		if err := verifyRecordSig(rec, pubKey); err != nil {
			return false, &ChainError{
				Hop:    i,
				Reason: "invalid_signature",
				Detail: fmt.Sprintf("agent=%s: %v", rec.Agent, err),
			}
		}

		// Check revocation status via RevSvc.
		if revChecker != nil && revChecker.IsAgentRevoked(rec.Agent) {
			return false, &ChainError{
				Hop:    i,
				Reason: "agent_revoked",
				Detail: fmt.Sprintf("agent=%s is revoked", rec.Agent),
			}
		}

		// Verify scope attenuation between consecutive hops.
		if i > 0 {
			prevScope := chain[i-1].Scope
			if !token.ScopeIsSubset(rec.Scope, prevScope) {
				return false, &ChainError{
					Hop:    i,
					Reason: "scope_escalation",
					Detail: fmt.Sprintf("scope %v exceeds previous hop scope %v", rec.Scope, prevScope),
				}
			}
		}
	}

	// Verify final scope is covered by the last hop's scope.
	lastScope := chain[len(chain)-1].Scope
	if !token.ScopeIsSubset(finalScope, lastScope) {
		return false, &ChainError{
			Hop:    len(chain) - 1,
			Reason: "final_scope_mismatch",
			Detail: fmt.Sprintf("final scope %v not covered by last delegation scope %v", finalScope, lastScope),
		}
	}

	return true, nil
}

// VerifyChainHash checks that the provided hash matches the SHA-256 of the serialized chain.
func VerifyChainHash(chain []token.DelegRecord, expectedHash string) (bool, *ChainError) {
	computed, err := computeChainHash(chain)
	if err != nil {
		return false, &ChainError{
			Hop:    -1,
			Reason: "hash_computation_failed",
			Detail: err.Error(),
		}
	}
	if computed != expectedHash {
		return false, &ChainError{
			Hop:    -1,
			Reason: "chain_hash_mismatch",
			Detail: fmt.Sprintf("computed=%s expected=%s", computed, expectedHash),
		}
	}
	return true, nil
}

// verifyRecordSig verifies the Ed25519 signature on a DelegRecord.
// The signature covers the record with the Signature field cleared.
func verifyRecordSig(rec token.DelegRecord, pubKey ed25519.PublicKey) error {
	// Reconstruct the unsigned record for verification.
	unsigned := token.DelegRecord{
		Agent:       rec.Agent,
		Scope:       rec.Scope,
		DelegatedAt: rec.DelegatedAt,
	}
	data, err := json.Marshal(unsigned)
	if err != nil {
		return fmt.Errorf("marshal record: %w", err)
	}
	sig, err := hex.DecodeString(rec.Signature)
	if err != nil {
		return fmt.Errorf("decode signature: %w", err)
	}
	if !ed25519.Verify(pubKey, data, sig) {
		return fmt.Errorf("Ed25519 signature verification failed")
	}
	return nil
}
