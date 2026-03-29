package token

// Revoker allows TknSvc to revoke and check revocation without importing the revoke package.
// This breaks the circular dependency: TknSvc -> Revoker (interface) <- RevSvc (implementation).
type Revoker interface {
	RevokeByJTI(jti string) error
	IsRevoked(claims *TknClaims) bool
}
