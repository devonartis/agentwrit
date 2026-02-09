// Package store provides the persistence layer for nonces, launch tokens,
// and agent records.
//
// The current implementation ([SqlStore]) keeps everything in memory behind
// a [sync.RWMutex]. The type is named SqlStore to ease a future migration to
// a SQL-backed store without changing call sites.
//
// All public methods are safe for concurrent use.
package store

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"time"
)

// Sentinel errors returned by store operations. Callers can use
// [errors.Is] to match specific failure modes.
var (
	ErrNonceNotFound = errors.New("nonce not found or expired")
	ErrNonceConsumed = errors.New("nonce already consumed")
	ErrTokenNotFound = errors.New("launch token not found")
	ErrTokenExpired  = errors.New("launch token expired")
	ErrTokenConsumed = errors.New("launch token already consumed")
	ErrAgentNotFound = errors.New("agent not found")
)

// LaunchTokenRecord represents a pre-authorized launch token created by an
// admin. It binds an agent name to an allowed scope ceiling and optional
// TTL cap. A single-use token is consumed on first registration.
type LaunchTokenRecord struct {
	Token        string
	AgentName    string
	AllowedScope []string
	MaxTTL       int
	SingleUse    bool
	CreatedAt    time.Time
	ExpiresAt    time.Time
	ConsumedAt   *time.Time
	CreatedBy    string
}

// AgentRecord stores the persistent state of a registered agent,
// including its SPIFFE-format AgentID, Ed25519 public key, and the
// scope granted at registration time.
type AgentRecord struct {
	AgentID      string
	PublicKey    []byte
	OrchID       string
	TaskID       string
	Scope        []string
	RegisteredAt time.Time
	LastSeen     time.Time
}

type nonceRecord struct {
	value     string
	expiresAt time.Time
	consumed  bool
}

// SqlStore provides in-memory storage with read/write mutex protection.
// Create one with [NewSqlStore] and share it across all services.
type SqlStore struct {
	mu           sync.RWMutex
	nonces       map[string]*nonceRecord
	launchTokens map[string]*LaunchTokenRecord
	agents       map[string]*AgentRecord
}

// NewSqlStore returns an initialized, empty in-memory store ready for use.
func NewSqlStore() *SqlStore {
	return &SqlStore{
		nonces:       make(map[string]*nonceRecord),
		launchTokens: make(map[string]*LaunchTokenRecord),
		agents:       make(map[string]*AgentRecord),
	}
}

// CreateNonce generates a cryptographically random 64-character hex nonce,
// stores it with a 30-second TTL, and returns the nonce string. The nonce
// must be consumed via [SqlStore.ConsumeNonce] before it expires.
func (s *SqlStore) CreateNonce() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	nonce := hex.EncodeToString(b)

	s.mu.Lock()
	defer s.mu.Unlock()
	s.nonces[nonce] = &nonceRecord{
		value:     nonce,
		expiresAt: time.Now().Add(30 * time.Second),
		consumed:  false,
	}
	return nonce
}

// ConsumeNonce atomically marks a nonce as consumed. It returns
// [ErrNonceNotFound] if the nonce does not exist or has expired, or
// [ErrNonceConsumed] if it was already consumed.
func (s *SqlStore) ConsumeNonce(nonce string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	rec, ok := s.nonces[nonce]
	if !ok {
		return ErrNonceNotFound
	}
	if rec.consumed {
		return ErrNonceConsumed
	}
	if time.Now().After(rec.expiresAt) {
		delete(s.nonces, nonce)
		return ErrNonceNotFound
	}
	rec.consumed = true
	return nil
}

// SaveLaunchToken persists a [LaunchTokenRecord], keyed by its Token field.
// If a record with the same token already exists it is silently overwritten.
func (s *SqlStore) SaveLaunchToken(rec LaunchTokenRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.launchTokens[rec.Token] = &rec
	return nil
}

// GetLaunchToken retrieves a launch token record by its opaque token string.
// It returns [ErrTokenNotFound] if the token does not exist,
// [ErrTokenExpired] if it has passed its ExpiresAt time, or
// [ErrTokenConsumed] if it was already consumed.
func (s *SqlStore) GetLaunchToken(token string) (*LaunchTokenRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rec, ok := s.launchTokens[token]
	if !ok {
		return nil, ErrTokenNotFound
	}
	if time.Now().After(rec.ExpiresAt) {
		return nil, ErrTokenExpired
	}
	if rec.ConsumedAt != nil {
		return nil, ErrTokenConsumed
	}
	return rec, nil
}

// ConsumeLaunchToken sets the ConsumedAt timestamp on a launch token,
// preventing it from being used again. It returns [ErrTokenNotFound] if
// the token does not exist or [ErrTokenConsumed] if already consumed.
func (s *SqlStore) ConsumeLaunchToken(token string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	rec, ok := s.launchTokens[token]
	if !ok {
		return ErrTokenNotFound
	}
	if rec.ConsumedAt != nil {
		return ErrTokenConsumed
	}
	now := time.Now()
	rec.ConsumedAt = &now
	return nil
}

// SaveAgent persists an [AgentRecord], keyed by its AgentID (SPIFFE ID).
func (s *SqlStore) SaveAgent(rec AgentRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.agents[rec.AgentID] = &rec
	return nil
}

// GetAgent retrieves an [AgentRecord] by its SPIFFE ID. It returns
// [ErrAgentNotFound] if no record exists for agentID.
func (s *SqlStore) GetAgent(agentID string) (*AgentRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rec, ok := s.agents[agentID]
	if !ok {
		return nil, ErrAgentNotFound
	}
	return rec, nil
}

// UpdateAgentLastSeen sets the LastSeen field of the agent identified by
// agentID to the current time. It returns [ErrAgentNotFound] if no matching
// record exists.
func (s *SqlStore) UpdateAgentLastSeen(agentID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	rec, ok := s.agents[agentID]
	if !ok {
		return ErrAgentNotFound
	}
	rec.LastSeen = time.Now()
	return nil
}
