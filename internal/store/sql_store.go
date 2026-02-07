package store

import (
	"database/sql"
	"errors"
	"sync"
	"time"
)

// ErrLaunchTokenNotFound indicates the requested launch token does not exist in the store.
// ErrLaunchTokenExpired indicates the launch token has passed its expiration time.
// ErrLaunchTokenConsumed indicates the launch token has already been used.
// ErrNonceNotFound indicates the requested nonce does not exist in the store.
// ErrNonceExpired indicates the nonce has passed its expiration time.
// ErrAgentExists indicates an agent with the same ID is already registered.
var (
	ErrLaunchTokenNotFound = errors.New("launch token not found")
	ErrLaunchTokenExpired  = errors.New("launch token expired")
	ErrLaunchTokenConsumed = errors.New("launch token already consumed")
	ErrNonceNotFound       = errors.New("nonce not found")
	ErrNonceExpired        = errors.New("nonce expired")
	ErrAgentExists         = errors.New("agent already exists")
)

// LaunchTokenData holds the metadata and state of an issued launch token.
type LaunchTokenData struct {
	Token      string
	OrchId     string
	TaskId     string
	Scope      []string
	ExpiresAt  time.Time
	Consumed   bool
	ConsumedAt time.Time
}

// AgentRecord represents a registered agent's identity and cryptographic material.
type AgentRecord struct {
	AgentID    string
	OrchId     string
	TaskId     string
	Scope      []string
	CreatedAt  time.Time
	PublicKey  []byte
	LastNonce  string
	LastSigRaw []byte
}

// NonceRecord represents a challenge nonce and its expiration time.
type NonceRecord struct {
	Nonce     string
	ExpiresAt time.Time
}

// SqlStore provides in-memory storage for launch tokens, agents, and nonces behind a mutex.
type SqlStore struct {
	DB           *sql.DB
	mu           sync.Mutex
	launchTokens map[string]LaunchTokenData
	agents       map[string]AgentRecord
	nonces       map[string]NonceRecord
}

// NewSqlStore creates a new SqlStore with initialized in-memory maps.
func NewSqlStore() *SqlStore {
	return &SqlStore{
		launchTokens: make(map[string]LaunchTokenData),
		agents:       make(map[string]AgentRecord),
		nonces:       make(map[string]NonceRecord),
	}
}

// CreateLaunchToken stores a new launch token in the store.
func (s *SqlStore) CreateLaunchToken(data LaunchTokenData) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.launchTokens[data.Token] = data
	return nil
}

// ConsumeLaunchToken marks a launch token as consumed and returns its data, or an error if the token is missing, expired, or already consumed.
func (s *SqlStore) ConsumeLaunchToken(token string) (*LaunchTokenData, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, ok := s.launchTokens[token]
	if !ok {
		return nil, ErrLaunchTokenNotFound
	}
	now := time.Now().UTC()
	if now.After(data.ExpiresAt) {
		return nil, ErrLaunchTokenExpired
	}
	if data.Consumed {
		return nil, ErrLaunchTokenConsumed
	}

	data.Consumed = true
	data.ConsumedAt = now
	s.launchTokens[token] = data
	return &data, nil
}

// SaveAgent persists a new agent record, returning ErrAgentExists if the agent ID is already registered.
func (s *SqlStore) SaveAgent(rec AgentRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.agents[rec.AgentID]; ok {
		return ErrAgentExists
	}
	s.agents[rec.AgentID] = rec
	return nil
}

// PutNonce stores a challenge nonce with the given expiration time.
func (s *SqlStore) PutNonce(nonce string, expiresAt time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nonces[nonce] = NonceRecord{Nonce: nonce, ExpiresAt: expiresAt}
}

// ConsumeNonce validates and removes a nonce from the store, returning an error if it is missing or expired.
func (s *SqlStore) ConsumeNonce(nonce string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.nonces[nonce]
	if !ok {
		return ErrNonceNotFound
	}
	if time.Now().UTC().After(rec.ExpiresAt) {
		return ErrNonceExpired
	}
	delete(s.nonces, nonce)
	return nil
}
