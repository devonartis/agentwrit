package store

import (
	"database/sql"
	"errors"
	"sync"
	"time"
)

var (
	ErrLaunchTokenNotFound = errors.New("launch token not found")
	ErrLaunchTokenExpired  = errors.New("launch token expired")
	ErrLaunchTokenConsumed = errors.New("launch token already consumed")
	ErrNonceNotFound       = errors.New("nonce not found")
	ErrNonceExpired        = errors.New("nonce expired")
	ErrAgentExists         = errors.New("agent already exists")
)

type LaunchTokenData struct {
	Token      string
	OrchId     string
	TaskId     string
	Scope      []string
	ExpiresAt  time.Time
	Consumed   bool
	ConsumedAt time.Time
}

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

type NonceRecord struct {
	Nonce     string
	ExpiresAt time.Time
}

type SqlStore struct {
	DB           *sql.DB
	mu           sync.Mutex
	launchTokens map[string]LaunchTokenData
	agents       map[string]AgentRecord
	nonces       map[string]NonceRecord
}

func NewSqlStore() *SqlStore {
	return &SqlStore{
		launchTokens: make(map[string]LaunchTokenData),
		agents:       make(map[string]AgentRecord),
		nonces:       make(map[string]NonceRecord),
	}
}

func (s *SqlStore) CreateLaunchToken(data LaunchTokenData) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.launchTokens[data.Token] = data
	return nil
}

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

func (s *SqlStore) SaveAgent(rec AgentRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.agents[rec.AgentID]; ok {
		return ErrAgentExists
	}
	s.agents[rec.AgentID] = rec
	return nil
}

func (s *SqlStore) PutNonce(nonce string, expiresAt time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nonces[nonce] = NonceRecord{Nonce: nonce, ExpiresAt: expiresAt}
}

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
