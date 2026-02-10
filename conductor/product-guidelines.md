# Product Guidelines - AgentAuth

## Documentation & Voice
- **Tone:** Professional, precise, and security-focused. Documentation should prioritize clarity and technical accuracy to build trust.
- **Clarity:** Use unambiguous language when describing security protocols, cryptographic operations, and authorization scopes.
- **Examples:** Provide clear, copy-pasteable integration examples for common agent environments (e.g., Python, Node.js).

## Visual & UX Principles (for APIs & CLI)
- **Developer Experience (DX):** API responses must be consistent, using RFC 7807 for error reporting.
- **Predictability:** Environment variables and configuration flags should follow standard patterns (e.g., `AA_` prefix).
- **Auditability:** Every action should result in a clear, human-readable log or audit event that explains *who* did *what* and *why*.

## Security Standards
- **Zero-Trust:** Assume every request is unauthorized until proven otherwise by a valid, non-expired cryptographic challenge or token.
- **Ephemerality by Default:** Discourage long-lived tokens; default configurations should enforce short TTLs.
- **Transparency:** All cryptographic logic and security-critical paths should be well-documented and easily auditable in the source code.
