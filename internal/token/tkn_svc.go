package token

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/divineartis/agentauth/internal/cfg"
	"github.com/divineartis/agentauth/internal/obs"
)

// Sentinel errors returned by [TknSvc.Verify].
var (
	ErrInvalidToken     = errors.New("invalid token format")
	ErrSignatureInvalid = errors.New("signature verification failed")
)

// IssueReq contains the parameters for issuing a new token via
// [TknSvc.Issue]. At minimum Sub and Scope must be set. If TTL is zero
// the broker's configured DefaultTTL is used.
type IssueReq struct {
	Sub        string
	Scope      []string
	TaskId     string
	OrchId     string
	Sid        string
	TTL        int // seconds; 0 means use DefaultTTL
	DelegChain []DelegRecord
	ChainHash  string
}

// IssueResp is returned by [TknSvc.Issue] and contains the compact JWT
// string, the effective TTL, the token type ("Bearer"), and a pointer to
// the embedded [TknClaims] for convenience.
type IssueResp struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	TokenType   string `json:"token_type"`
	Claims      *TknClaims
}

// jwtHeader is the fixed EdDSA JWT header.
type jwtHeader struct {
	Alg string `json:"alg"`
	Typ string `json:"typ"`
}

// TknSvc is the core token service. It holds an Ed25519 key pair and
// the broker configuration, and provides [Issue], [Verify], and [Renew]
// operations. A single TknSvc instance is shared across all services
// and handlers.
type TknSvc struct {
	signingKey ed25519.PrivateKey
	pubKey     ed25519.PublicKey
	cfg        cfg.Cfg
}

// NewTknSvc creates a new token service with the given Ed25519 key pair
// and broker configuration. The key pair is used for signing (Issue) and
// verification (Verify) of all tokens.
func NewTknSvc(signingKey ed25519.PrivateKey, pubKey ed25519.PublicKey, c cfg.Cfg) *TknSvc {
	return &TknSvc{
		signingKey: signingKey,
		pubKey:     pubKey,
		cfg:        c,
	}
}

// Issue creates a new EdDSA-signed JWT from the given [IssueReq]. It
// generates a fresh JTI, sets iat/nbf/exp based on the current time and
// TTL, and returns an [IssueResp] containing the compact token string.
func (s *TknSvc) Issue(req IssueReq) (*IssueResp, error) {
	ttl := req.TTL
	if ttl <= 0 {
		ttl = s.cfg.DefaultTTL
	}

	now := time.Now().Unix()
	jti := randomJTI()

	claims := &TknClaims{
		Iss:        "agentauth",
		Sub:        req.Sub,
		Exp:        now + int64(ttl),
		Nbf:        now,
		Iat:        now,
		Jti:        jti,
		Sid:        req.Sid,
		Scope:      req.Scope,
		TaskId:     req.TaskId,
		OrchId:     req.OrchId,
		DelegChain: req.DelegChain,
		ChainHash:  req.ChainHash,
	}

	tokenStr, err := s.sign(claims)
	if err != nil {
		return nil, fmt.Errorf("sign token: %w", err)
	}

	// Metric: count issued tokens by primary scope.
	scopeLabel := "none"
	if len(req.Scope) > 0 {
		scopeLabel = req.Scope[0]
	}
	obs.TokensIssuedTotal.WithLabelValues(scopeLabel).Inc()

	return &IssueResp{
		AccessToken: tokenStr,
		ExpiresIn:   ttl,
		TokenType:   "Bearer",
		Claims:      claims,
	}, nil
}

// Verify parses a compact JWT string, verifies the Ed25519 signature, and
// validates the claims (issuer, subject, JTI, expiry, nbf). On success it
// returns the decoded [TknClaims]. On failure it returns one of
// [ErrInvalidToken], [ErrSignatureInvalid], or a claims validation error.
func (s *TknSvc) Verify(tokenStr string) (*TknClaims, error) {
	parts := strings.SplitN(tokenStr, ".", 3)
	if len(parts) != 3 {
		return nil, ErrInvalidToken
	}

	// Decode and verify signature
	signingInput := parts[0] + "." + parts[1]
	sigBytes, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, ErrSignatureInvalid
	}

	if !ed25519.Verify(s.pubKey, []byte(signingInput), sigBytes) {
		return nil, ErrSignatureInvalid
	}

	// Decode claims
	claimsJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, ErrInvalidToken
	}

	var claims TknClaims
	if err := json.Unmarshal(claimsJSON, &claims); err != nil {
		return nil, ErrInvalidToken
	}

	if err := claims.Validate(); err != nil {
		return nil, err
	}

	return &claims, nil
}

// Renew verifies an existing token and, if valid, issues a replacement
// token with the same subject, scope, task, orchestration, and delegation
// chain but fresh timestamps and a new JTI. The original token remains
// valid until its own expiry (or revocation).
func (s *TknSvc) Renew(tokenStr string) (*IssueResp, error) {
	claims, err := s.Verify(tokenStr)
	if err != nil {
		return nil, err
	}

	return s.Issue(IssueReq{
		Sub:        claims.Sub,
		Scope:      claims.Scope,
		TaskId:     claims.TaskId,
		OrchId:     claims.OrchId,
		Sid:        claims.Sid,
		DelegChain: claims.DelegChain,
		ChainHash:  claims.ChainHash,
	})
}

// PublicKey returns the Ed25519 public key used for token signature
// verification. This can be shared with external services that need to
// verify tokens independently.
func (s *TknSvc) PublicKey() ed25519.PublicKey {
	return s.pubKey
}

func (s *TknSvc) sign(claims *TknClaims) (string, error) {
	hdr := jwtHeader{Alg: "EdDSA", Typ: "JWT"}
	hdrJSON, err := json.Marshal(hdr)
	if err != nil {
		return "", err
	}

	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}

	hdrB64 := base64.RawURLEncoding.EncodeToString(hdrJSON)
	claimsB64 := base64.RawURLEncoding.EncodeToString(claimsJSON)
	signingInput := hdrB64 + "." + claimsB64

	sig := ed25519.Sign(s.signingKey, []byte(signingInput))
	sigB64 := base64.RawURLEncoding.EncodeToString(sig)

	return signingInput + "." + sigB64, nil
}

func randomJTI() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	return hex.EncodeToString(b)
}
