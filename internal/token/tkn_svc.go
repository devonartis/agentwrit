package token

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/divineartis/agentauth/internal/cfg"
)

// Token verification errors.
var (
	// ErrTokenMalformed indicates the token string is not well-formed.
	ErrTokenMalformed = errors.New("malformed token")
	// ErrTokenSignature indicates the token signature verification failed.
	ErrTokenSignature = errors.New("invalid token signature")
	// ErrTokenExpired indicates the token has passed its expiration time.
	ErrTokenExpired = errors.New("token expired")
	// ErrTokenNotYet indicates the token is not yet valid per its nbf claim.
	ErrTokenNotYet = errors.New("token not valid yet")
)

// IssueReq holds the parameters needed to issue a new agent token.
type IssueReq struct {
	AgentID   string
	OrchID    string
	TaskID    string
	Scope     []string
	TTLSecond int
}

// IssueResp contains the issued token and its timing metadata.
type IssueResp struct {
	AccessToken  string `json:"access_token"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshAfter int    `json:"refresh_after"`
}

// TknSvc provides token issuance, verification, and renewal using EdDSA signatures.
type TknSvc struct {
	signingKey ed25519.PrivateKey
	pubKey     ed25519.PublicKey
	cfg        cfg.Cfg
	clockSkew  int64
}

// NewTknSvc creates a TknSvc with the given Ed25519 key pair and configuration.
func NewTknSvc(signingKey ed25519.PrivateKey, pubKey ed25519.PublicKey, c cfg.Cfg) *TknSvc {
	return &TknSvc{
		signingKey: signingKey,
		pubKey:     pubKey,
		cfg:        c,
		clockSkew:  30,
	}
}

// Issue creates and signs a new JWT token from the given request parameters.
func (s *TknSvc) Issue(req IssueReq) (*IssueResp, error) {
	ttl := req.TTLSecond
	if ttl <= 0 {
		ttl = s.cfg.DefaultTTL
		if ttl <= 0 {
			ttl = 300
		}
	}
	now := time.Now().UTC()
	claims := TknClaims{
		Iss:        "agentauth://" + s.cfg.TrustDomain,
		Sub:        req.AgentID,
		Aud:        []string{"resource-server"},
		Exp:        now.Add(time.Duration(ttl) * time.Second).Unix(),
		Nbf:        now.Add(-1 * time.Second).Unix(),
		Iat:        now.Unix(),
		Jti:        randomJTI(),
		Scope:      append([]string{}, req.Scope...),
		TaskId:     req.TaskID,
		OrchId:     req.OrchID,
		DelegChain: []DelegRecord{},
	}
	if err := claims.Validate(now); err != nil {
		return nil, err
	}
	token, err := s.signClaims(claims)
	if err != nil {
		return nil, err
	}
	return &IssueResp{
		AccessToken:  token,
		ExpiresIn:    ttl,
		RefreshAfter: int(float64(ttl) * 0.75),
	}, nil
}

// Verify validates the token signature and claims, returning the parsed claims on success.
func (s *TknSvc) Verify(tokenStr string) (*TknClaims, error) {
	parts := strings.Split(tokenStr, ".")
	if len(parts) != 3 {
		return nil, ErrTokenMalformed
	}
	signingInput := parts[0] + "." + parts[1]
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, ErrTokenMalformed
	}
	if !ed25519.Verify(s.pubKey, []byte(signingInput), sig) {
		return nil, ErrTokenSignature
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, ErrTokenMalformed
	}
	var claims TknClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, ErrTokenMalformed
	}

	now := time.Now().UTC().Unix()
	if claims.Exp == 0 || now > claims.Exp+s.clockSkew {
		return nil, ErrTokenExpired
	}
	if claims.Nbf != 0 && now+s.clockSkew < claims.Nbf {
		return nil, ErrTokenNotYet
	}
	if err := claims.Validate(time.Now().UTC()); err != nil {
		return nil, err
	}
	return &claims, nil
}

// Renew verifies an existing token and issues a fresh token with the same claims.
func (s *TknSvc) Renew(tokenStr string) (*IssueResp, error) {
	claims, err := s.Verify(tokenStr)
	if err != nil {
		return nil, err
	}
	return s.Issue(IssueReq{
		AgentID: claims.Sub,
		OrchID:  claims.OrchId,
		TaskID:  claims.TaskId,
		Scope:   claims.Scope,
	})
}

func (s *TknSvc) signClaims(claims TknClaims) (string, error) {
	header := `{"alg":"EdDSA","typ":"JWT"}`
	headerB64 := base64.RawURLEncoding.EncodeToString([]byte(header))
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	payloadB64 := base64.RawURLEncoding.EncodeToString(payload)
	signingInput := headerB64 + "." + payloadB64
	sig := ed25519.Sign(s.signingKey, []byte(signingInput))
	if subtle.ConstantTimeEq(int32(len(sig)), int32(ed25519.SignatureSize)) != 1 {
		return "", fmt.Errorf("signature size mismatch")
	}
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}

func randomJTI() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "jti-fallback"
	}
	return hex.EncodeToString(b)
}
