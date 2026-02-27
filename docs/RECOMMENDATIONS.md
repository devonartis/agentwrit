# Documentation & Development Recommendations

> **Date:** February 2026 | **Context:** Post documentation upgrade review
>
> Recommendations for additional guides, documentation gaps, and future development work.

---

## Recommended New Guides

### 1. SDK Quick Start Guide (`docs/sdk-quickstart.md`)

**Priority:** High — blocks demo readiness for Python developers

The current developer guide (`getting-started-developer.md`) shows raw HTTP examples. Python developers need a guide built around the Python SDK (once built). This guide should cover: `uv add agentauth` (using [Astral uv](https://docs.astral.sh/uv/) as the package manager), connecting to a sidecar, requesting tokens, token renewal, and token release. Think of it as the 5-minute onboarding path for a developer who has never seen AgentAuth. All Python tooling should use `uv` — not pip — for dependency management, virtual environments, and script execution.

### 2. ~~Sidecar Deployment Guide~~ (`docs/sidecar-deployment.md`) — DONE

Completed. Comprehensive guide covering Docker Compose, systemd, trust boundary theory, and operational procedures.

### 3. Security Hardening Guide (`docs/security-hardening.md`)

**Priority:** High — production readiness requirement

A dedicated guide covering: enabling mTLS end-to-end (broker + sidecar), UDS socket permissions and ownership, admin secret management (rotation, vault integration patterns), network segmentation recommendations, and how to address each known issue (KI-001 through KI-004). The current SECURITY.md covers vulnerability reporting but not operational hardening.

### 4. Migration / Upgrade Guide (`docs/upgrade-guide.md`)

**Priority:** Medium — needed before any versioned release

Document what happens when upgrading between versions: SQLite schema migrations, signing key regeneration behavior, backward compatibility of tokens, configuration changes between releases.

### 5. Monitoring & Alerting Guide (`docs/monitoring.md`)

**Priority:** Medium — operators need this for production

Document all Prometheus metrics (broker and sidecar), recommended alert thresholds (token issuance rate, circuit breaker state, audit event volume, revocation count), Grafana dashboard examples. The metrics exist but aren't documented anywhere except the code.

### 6. ~~Integration Patterns Guide~~ (`docs/integration-patterns.md`) — DONE

Completed. Six integration patterns with mermaid diagrams, Python examples, security analysis, and dangerous-path comparisons.

---

## Documentation Gaps to Address

### ~~OpenAPI Spec~~ (`docs/api/openapi.yaml`) — DONE

Updated. Added: `POST /v1/token/release`, `GET /v1/admin/sidecars`, `GET/PUT /v1/admin/sidecars/{id}/ceiling`, `outcome` query parameter, structured audit fields, `token_released` and `scopes_ceiling_updated` event types, health response fields, and sidecar management schemas. Fixed license (MIT → Apache 2.0) and removed duplicate response keys.

### ~~aactl Man Pages / Help Text~~ (`docs/aactl-reference.md`) — DONE

Completed. Comprehensive CLI reference with all commands, flags, examples, common workflows, and security best practices.

### Architecture Decision Records (ADRs)

ADR-002 (sidecar architecture decision) was archived during pre-release cleanup. Consider creating a `docs/adr/` directory in the repo for accepted ADRs that should be part of the public record. Future ADRs (admin secret narrowing, direct broker access, etc.) should live here.

---

## Development Work Recommendations

### Immediate (Before Demo)

1. **Python SDK** — First demo audience is Python developers. SDK should cover: agent registration, token request via sidecar, token renewal, token release. The sidecar API is the primary interface (not the broker).

2. **KI-001 Fix (Admin Secret Narrowing)** — High-priority security fix. New `POST /v1/sidecar/launch-tokens` endpoint gated by `sidecar:manage:*` scope. Sidecars stop needing admin secret after bootstrap.

### Short-Term (Post Demo)

4. **TypeScript SDK** — Second priority after Python SDK for broader developer reach.

5. **KI-002 Fix (UDS Default)** — Make UDS the default listener mode. Update quickstart and Docker Compose to demonstrate UDS-first.

6. **SQLite WAL Mode** — The `SQLITE_BUSY` concurrent write issue (found in Session 14) needs a proper fix. WAL mode or a write mutex in SqlStore.

7. **Token Introspection Endpoint** — `POST /v1/token/introspect` (RFC 7662 compatible) for resource servers that need to validate tokens without the broker's public key.

### Medium-Term (Production Readiness)

8. **KI-003 Fix (Per-Sidecar Credentials)** — Each sidecar gets unique credentials instead of sharing admin secret. Enables sidecar-level audit attribution.

9. **Key Rotation** — Support for rolling key rotation without dropping all in-flight tokens. Dual-key verification window during rotation.

10. **HA / Replication** — Currently single-broker. Design for multi-broker with shared SQLite (via Litestream or similar) or migration to PostgreSQL.

11. **Rate Limiting per Agent** — Current rate limiting is per-IP on admin auth only. Add per-agent rate limiting for token requests to prevent credential abuse.

---

## Existing Documentation Quality Assessment

| Document | Quality | Notes |
|----------|---------|-------|
| README.md | Excellent | Professional badges, clear structure, comprehensive |
| docs/api.md | Excellent | Complete endpoint reference with examples |
| docs/architecture.md | Excellent | Accurate mermaid diagrams, pattern mapping |
| docs/concepts.md | Good | May need refresh when SDK is available |
| docs/getting-started-user.md | Good | Clear 5-step quickstart |
| docs/getting-started-developer.md | Good | Needs SDK examples when available |
| docs/getting-started-operator.md | Excellent | Comprehensive aactl guide |
| docs/common-tasks.md | Good | Thorough step-by-step workflows |
| docs/troubleshooting.md | Good | UDS and TLS sections now added |
| CHANGELOG.md | Excellent | Detailed per-file change tracking |
| CONTRIBUTING.md | Good | Clear development setup |
| SECURITY.md | Good | Needs hardening guide companion |
| KNOWN-ISSUES.md | Excellent | Transparent, actionable |
