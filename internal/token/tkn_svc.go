package token

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/devonartis/agentauth/internal/cfg"
	"github.com/devonartis/agentauth/internal/obs"
)

// Sentinel errors for token verification. A single generic message is returned
// to callers — the specific error is only visible internally to prevent
// information leakage about token structure or revocation state.
var (
	ErrInvalidToken     = errors.New("invalid token format")
	ErrSignatureInvalid = errors.New("signature verification failed")
	ErrTokenRevoked     = errors.New("token has been revoked")
)

// IssueReq contains the parameters for issuing a new token.
//
// Sub identifies the principal: "admin" for operators, "app:<appID>" for
// registered applications, or a SPIFFE ID for agents. Scope defines what
// the token holder can do — admin scopes (admin:revoke:*, admin:audit:*),
// app scopes (app:launch-tokens:*, app:agents:*), or task-specific agent
// scopes (read:data:*, write:logs:*). TTL is seconds; 0 falls back to
// DefaultTTL; always clamped by MaxTTL in Issue().
type IssueReq struct {
	Sub        string
	Aud        []string
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
	Kid string `json:"kid,omitempty"`
}

// computeKid returns the RFC 7638 JWK Thumbprint of an Ed25519 public key.
func computeKid(pub ed25519.PublicKey) string {
	x := base64.RawURLEncoding.EncodeToString(pub)
	canonical := `{"crv":"Ed25519","kty":"OKP","x":"` + x + `"}`
	h := sha256.Sum256([]byte(canonical))
	return base64.RawURLEncoding.EncodeToString(h[:])
}

// TknSvc is the core token service — the single authority for all JWT
// operations in the broker. It signs, verifies, renews, and enforces TTL
// constraints on every token in the system.
//
// Callers:
//   - AdminSvc calls Issue to create admin JWTs (scope: admin:*)
//   - AppSvc calls Issue to create app JWTs (scope: app:*)
//   - IdentitySvc calls Issue to create agent JWTs (scope: task-specific, e.g. read:data:*)
//   - RenewHdl calls Renew for agents extending their session
//   - ValMw and ValHdl call Verify to authenticate any incoming Bearer token
//
// TknSvc does NOT enforce who can call it — that's the handler/middleware
// layer's job (scope checks via ValMw.RequireScope). TknSvc trusts its
// callers to have already passed authorization checks.
type TknSvc struct {
	signingKey ed25519.PrivateKey
	pubKey     ed25519.PublicKey
	kid        string // RFC 7638 JWK Thumbprint
	cfg        cfg.Cfg
	revoker    Revoker
}

// Kid returns the computed RFC 7638 JWK Thumbprint used as the JWT kid header.
func (s *TknSvc) Kid() string { return s.kid }

// SetRevoker injects a Revoker implementation (typically RevSvc) after
// construction. Called at broker bootstrap to break the circular dependency
// between TknSvc and RevSvc.
func (s *TknSvc) SetRevoker(r Revoker) {
	s.revoker = r
}

// NewTknSvc creates a new token service with the given Ed25519 key pair
// and broker configuration. The key pair is used for signing (Issue) and
// verification (Verify) of all tokens. The kid (Key ID) is computed as
// the RFC 7638 JWK Thumbprint of the public key.
func NewTknSvc(signingKey ed25519.PrivateKey, pubKey ed25519.PublicKey, c cfg.Cfg) *TknSvc {
	kid := computeKid(pubKey)
	return &TknSvc{
		signingKey: signingKey,
		pubKey:     pubKey,
		kid:        kid,
		cfg:        c,
	}
}

// Issue creates a new EdDSA-signed JWT.
//
// Called by three roles, each with different scope semantics:
//   - Admin (via AdminSvc): sub="admin", scope=admin:*, TTL=300 (hardcoded, see TD-010)
//   - App (via AppSvc.Authenticate): sub="app:<appID>", scope=app:*, TTL=app.TokenTTL
//   - Agent (via IdentitySvc.Register): sub=SPIFFE ID, scope=task-specific, TTL=launch token's max_ttl
//
// Issue does NOT check authorization — callers must have already passed
// scope checks. MaxTTL is the global safety ceiling; no caller can bypass it.
func (s *TknSvc) Issue(req IssueReq) (*IssueResp, error) {
	ttl := req.TTL
	if ttl <= 0 {
		ttl = s.cfg.DefaultTTL
	}
	if s.cfg.MaxTTL > 0 && ttl > s.cfg.MaxTTL {
		ttl = s.cfg.MaxTTL
	}

	now := time.Now().Unix()
	jti := randomJTI()

	claims := &TknClaims{
		Iss:        "agentauth",
		Sub:        req.Sub,
		Aud:        req.Aud,
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

// Verify validates a compact JWT string — signature, claims, and revocation.
//
// Role-agnostic: validates admin, app, and agent tokens identically.
// The caller (ValMw, ValHdl, Renew) decides what to do with the claims —
// Verify does not check scope or role. Apps use ValHdl (POST /v1/token/validate)
// to check agent tokens before granting access to resources.
func (s *TknSvc) Verify(tokenStr string) (*TknClaims, error) {
	parts := strings.SplitN(tokenStr, ".", 3)
	if len(parts) != 3 {
		return nil, ErrInvalidToken
	}

	// Validate JWT header: alg must be EdDSA, kid (if present) must match.
	hdrJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, ErrInvalidToken
	}
	var hdr jwtHeader
	if err := json.Unmarshal(hdrJSON, &hdr); err != nil {
		return nil, ErrInvalidToken
	}
	if hdr.Alg != "EdDSA" {
		return nil, ErrInvalidToken
	}
	if hdr.Kid != "" && hdr.Kid != s.kid {
		obs.Warn("TOKEN", "verify", "kid mismatch — possible key rotation or wrong key", "got="+hdr.Kid, "want="+s.kid)
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

	if s.revoker != nil && s.revoker.IsRevoked(&claims) {
		return nil, ErrTokenRevoked
	}

	return &claims, nil
}

// Renew replaces an agent's token with fresh timestamps and a new JTI.
//
// Called by: RenewHdl (POST /v1/token/renew) behind ValMw. Only agents
// with valid, non-revoked tokens can renew. Admin and app tokens are
// technically renewable but have no production reason to do so.
//
// SEC-A1 — TTL carry-forward: The renewed token preserves the original
// TTL (Exp - Iat), NOT DefaultTTL. Without this, an agent issued with
// TTL=120 could renew and get TTL=300 — a privilege escalation. MaxTTL
// clamp still applies via Issue().
//
// Predecessor revocation is mandatory and happens BEFORE issuance.
// If revocation fails, renewal fails — no orphaned valid predecessors.
func (s *TknSvc) Renew(tokenStr string) (*IssueResp, error) {
	claims, err := s.Verify(tokenStr)
	if err != nil {
		return nil, err
	}

	// Mandatory predecessor revocation — renewal fails if revocation fails (M5).
	// Predecessor is revoked BEFORE issuing the new token so the old JTI is
	// invalidated even if issuance subsequently fails. This ensures revoked
	// tokens cannot be reused. The caller can safely retry on issuance failure.
	if s.revoker != nil {
		if err := s.revoker.RevokeByJTI(claims.Jti); err != nil {
			return nil, fmt.Errorf("revoke predecessor: %w", err)
		}
	}

	// Carry forward the original TTL from the token being renewed.
	// This prevents renewal from escalating to s.cfg.DefaultTTL.
	originalTTL := int(claims.Exp - claims.Iat)
	if originalTTL <= 0 {
		originalTTL = s.cfg.DefaultTTL
	}

	return s.Issue(IssueReq{
		Sub:        claims.Sub,
		Aud:        claims.Aud,
		Scope:      claims.Scope,
		TaskId:     claims.TaskId,
		OrchId:     claims.OrchId,
		Sid:        claims.Sid,
		TTL:        originalTTL,
		DelegChain: claims.DelegChain,
		ChainHash:  claims.ChainHash,
	})
}

// PublicKey returns the Ed25519 public key so external services (resource
// servers, federated brokers) can verify tokens without calling back to
// the broker. Exposed via the OIDC discovery endpoint (/.well-known/jwks.json).
func (s *TknSvc) PublicKey() ed25519.PublicKey {
	return s.pubKey
}

func (s *TknSvc) sign(claims *TknClaims) (string, error) {
	hdr := jwtHeader{Alg: "EdDSA", Typ: "JWT", Kid: s.kid}
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
