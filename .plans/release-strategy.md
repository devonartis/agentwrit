# Release Strategy — High-Level Plan

**Created:** 2026-03-31
**Status:** DRAFT — needs user review before breaking into specs
**Scope:** Repo restructuring, archival, SDK extraction, SDK update, and open-source release. This is the release strategy for AgentAuth as an open-source project — not just SDK placement.
**Context:** Migration B0-B6 complete. SDKs exist in `devonartis/agentauth-clients` but were built against the old broker (`authAgent2`) with HITL/OIDC. Core repo (`agentauth-core`) has no HITL/OIDC. SDKs need updating and repos need restructuring. Each phase below will break into its own brainstorm → spec → plan cycle via devflow.

---

## Decision: Model 1 — Separate Per-Language Repos

Following the Stripe/Twilio/HashiCorp pattern. Each SDK gets its own repo with independent release cycle, CI, changelog, and semver.

**Why this model:**
- Open-core split requires it — core SDK ships open, HITL/OIDC SDK extensions are enterprise
- Package identity matters — `pip install agentauth` from `agentauth-python`, not a monorepo subdirectory
- SDKs evolve at different speeds than the broker
- Language-specific contributors find and fork just their SDK
- Issue triage stays clean — Python bugs in the Python repo

**Trade-off we accept:** API contract drift across N repos. Mitigated by `docs/api.md` as single source of truth (and eventually OpenAPI codegen).

**Industry examples:** Stripe (`stripe-python`, `stripe-node`, `stripe-go`), Twilio, Cloudflare, HashiCorp Vault.

---

## Phase 1: Repo Cleanup & Archive

**Goal:** Clean up `agentauth-core`, archive the old `divineartis/agentauth` repo.

### 1A: Archive `divineartis/agentauth`
- Rename to `divineartis/agentauth-legacy` or archive via GitHub settings
- This is the second archive (first archive is `devonartis/agentauth-internal`)
- Three archives total: `agentauth-internal` (412 incremental commits), `agentauth` (enterprise/HITL/OIDC), `agentauth-clients` (current SDK monorepo)

### 1B: Rename `agentauth-core` → `divineartis/agentauth`
- Update Go module path (`go.mod`)
- Update all internal import paths
- Update CI, docs, README references
- This becomes the canonical open-source repo

### 1C: Clean up repo artifacts (TD-017)
- Remove internal files before public release (MEMORY.md, FLOW.md, COWORK_SESSION.md, etc.)
- Sanitize or remove TECH-DEBT.md
- Move CLAUDE.md into `.claude/`
- Remove `tests/FUCKING QUETIONS.MD`
- Remove `docs/patent/` (NEVER ships)
- Classify all docs/, scripts/, tests/ per TD-017 inventory

---

## Phase 2: SDK Repo Setup

**Goal:** Create per-language SDK repos from the current monorepo.

### 2A: Create `divineartis/agentauth-python`
- Extract `agentauth-python/` from `agentauth-clients` into its own repo
- Preserve git history if possible (git filter-repo)
- Set up CI (pytest, mypy, ruff)
- Package identity: `agentauth` on PyPI

### 2B: Create `divineartis/agentauth-ts`
- Extract `agentauth-ts/` from `agentauth-clients` into its own repo
- Preserve git history if possible
- Set up CI (vitest, tsc, eslint)
- Package identity: `agentauth` on npm

### 2C: Archive `devonartis/agentauth-clients`
- Mark as archived on GitHub
- README points to the new per-language repos

---

## Phase 3: SDK Core Update

**Goal:** Update both SDKs to work against `agentauth-core`'s actual API surface. Remove enterprise features, verify against live broker.

### 3A: API Contract Audit
- Read `agentauth-core/docs/api.md` — the source of truth
- Compare every SDK endpoint call against what core actually exposes
- Document differences (field names, request shapes, response shapes)
- Endpoints that exist in core: `/v1/app/auth`, `/v1/app/launch-tokens`, `/v1/challenge`, `/v1/register`, `/v1/delegate`, `/v1/token/release`, `/v1/token/validate`

### 3B: Remove Enterprise Features from Core SDKs
- Remove `HITLApprovalRequired` exception (or make it an extension point)
- Remove HITL retry logic from `get_token`/`getToken`
- Remove HITL demo app and docs
- Remove `hitl_scopes` references
- Remove HITL integration tests
- Keep the interfaces pluggable — enterprise SDK extensions can add HITL back later

### 3C: Integration Testing Against Core Broker
- Stand up `agentauth-core` broker in Docker
- Run SDK unit tests (should mostly pass — no broker dependency)
- Run SDK integration tests against core broker
- Fix field name mismatches, registration flow differences, response shape differences
- Write new acceptance tests per devflow process

### 3D: Documentation Update
- Update SDK docs to reference core broker, not legacy broker
- Update getting-started guides with core broker setup
- Remove HITL implementation guide from core SDK docs (moves to enterprise)
- Update API reference to match core endpoints

---

## Phase 4: Future — Enterprise SDK Extensions

**Not in scope now.** Captured for completeness.

- Enterprise Python package: `agentauth-enterprise` or `agentauth[hitl]`
- Adds HITL approval flow, OIDC token exchange, cloud federation
- Plugs into core SDK via extension points
- Lives in private enterprise repo(s)

---

## Sequencing & Dependencies

```
Phase 1A (archive old agentauth)
  → Phase 1B (rename agentauth-core)
    → Phase 1C (clean up artifacts)
      → Phase 2A/2B (create SDK repos — can be parallel)
        → Phase 2C (archive clients monorepo)
          → Phase 3A (API audit)
            → Phase 3B/3C/3D (SDK update — can be partially parallel)
```

**Phases 1 and 2 are repo operations** — mostly git/GitHub work, low risk.
**Phase 3 is development work** — needs devflow (brainstorm → spec → tests → code → review).

---

## Open Questions

1. **Go module path:** `github.com/divineartis/agentauth` or a vanity import path?
2. **SDK versioning:** Start at v0.2.0 (continuing from v0.1.0) or v1.0.0 (fresh start for core)?
3. **HITL extension point design:** How should core SDKs expose hooks for enterprise to plug in HITL? Callback? Subclass? Middleware?
4. **OpenAPI codegen:** Should we generate SDK stubs from `docs/api/openapi.yaml` instead of hand-writing? (Requires fixing TD-S14 first — OpenAPI has 51 stale sidecar refs.)
5. **Go SDK:** Should there be a `divineartis/agentauth-go` SDK? Or is the Go broker itself the "SDK" for Go consumers?

---

## Archives Summary (after all phases)

| Repo | What | Status |
|------|------|--------|
| `devonartis/agentauth-internal` | 412 incremental commits, real history | Archive #1 |
| `divineartis/agentauth` (old) | Enterprise modules, HITL, OIDC, migration plans | Archive #2 (renamed to `agentauth-legacy`) |
| `devonartis/agentauth-clients` | SDK monorepo (Python + TS) | Archive #3 |
| `divineartis/agentauth` (new) | Go broker, open-source core | **Active — canonical** |
| `divineartis/agentauth-python` | Python SDK, open-source | **Active** |
| `divineartis/agentauth-ts` | TypeScript SDK, open-source | **Active** |
