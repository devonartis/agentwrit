# Design: Python SDK Repo (`divineartis/agentauth-python`)

**Created:** 2026-04-01
**Status:** APPROVED
**Scope:** Extract Python SDK from monorepo, remove HITL contamination, align with core broker API, set up as independent open-source repo.

---

## Decision Summary

- **Model:** Separate per-language repo (Stripe/Twilio pattern)
- **Extraction:** `git filter-repo --subdirectory-filter agentauth-python/` from `devonartis/agentauth-clients`
- **Package:** `agentauth` on PyPI, version `v0.2.0` (continues from monorepo's `v0.1.0`)
- **Package manager:** `uv` — no pip, no poetry
- **Type safety:** Strict — every variable, parameter, and return annotated. `mypy --strict` enforced.
- **Focus:** Python first. TypeScript follows the same pattern later.

---

## 1. Repo Extraction

1. Clone `devonartis/agentauth-clients` fresh (don't touch working copies)
2. Run `git filter-repo --subdirectory-filter agentauth-python/` — makes Python subdirectory the new root
3. Result: standalone repo with `src/agentauth/`, `tests/`, `pyproject.toml` at root level
4. All commits that touched `agentauth-python/` are preserved with paths rewritten to root
5. All TypeScript code, monorepo scaffolding dropped
6. Create `divineartis/agentauth-python` on GitHub
7. Set remote, push

---

## 2. HITL Contamination Removal

Remove all HITL code from core SDK. Same pattern as B0 sidecar removal from the broker.

| File | Action |
|------|--------|
| `src/agentauth/__init__.py` | Remove `HITLApprovalRequired` export |
| `src/agentauth/errors.py` | Remove `HITLApprovalRequired` exception class |
| `src/agentauth/client.py` | Remove HITL retry/polling logic from `get_token` |
| `tests/integration/test_hitl.py` | Delete entire file |
| `tests/sdk-core/s6_hitl.py` | Delete entire file |
| `docs/hitl-implementation-guide.md` | Delete entire file |
| `README.md` | Remove HITL references |

**Extension point:** Error hierarchy stays extensible (enterprise can subclass `AgentAuthError`) but no HITL hooks built into core. YAGNI.

**`get_token` flow simplifies to:** challenge -> sign -> exchange -> done. No approval polling loop.

---

## 3. API Contract Audit & Live Verification

### 3A. Inspect

Read `agentauth-core/docs/api.md` (source of truth) and diff every SDK HTTP call against it:

| SDK Call | Core Endpoint | Status |
|----------|---------------|--------|
| App auth (challenge-response) | `POST /v1/app/auth` | Exists |
| Create launch token | `POST /v1/app/launch-tokens` | Exists |
| Challenge | `POST /v1/challenge` | Exists |
| Register agent | `POST /v1/register` | Exists |
| Delegate | `POST /v1/delegate` | Exists |
| Release token | `POST /v1/token/release` | Exists |
| Validate token | `POST /v1/token/validate` | Exists |
| HITL approval polling | N/A | **Remove** |

**Known mismatches (from migration lessons):** `token` vs `access_token`, `allowed_scopes` vs `allowed_scope`, `agent_name` required, nonce encoding (base64 vs hex).

### 3B. Live Verification

Stand up the core broker (Docker via `broker-up` or `scripts/stack_up.sh`), run each SDK call against it, record what actually happens. The broker is the ultimate source of truth.

---

## 4. Testing Strategy

| Layer | What | How | Broker needed? |
|-------|------|-----|----------------|
| **Unit tests** | SDK logic (crypto, cache, retry, errors) | `uv run pytest tests/unit/` | No |
| **Integration tests** | SDK -> live broker round-trips | `uv run pytest -m integration` | Yes (Docker) |
| **Acceptance tests** | End-to-end stories per devflow | Stand up broker, run stories, record evidence with banners | Yes (Docker) |

---

## 5. Repo Structure

```
agentauth-python/
├── CLAUDE.md
├── README.md
├── pyproject.toml
├── uv.lock
├── src/agentauth/
│   ├── __init__.py
│   ├── client.py
│   ├── crypto.py
│   ├── errors.py
│   ├── retry.py
│   ├── token.py
│   └── py.typed
├── tests/
│   ├── unit/
│   ├── integration/
│   └── sdk-core/
└── docs/
    ├── getting-started.md
    ├── concepts.md
    └── developer-guide.md
```

### CLAUDE.md Rules

- **Strict type safety** — every variable, parameter, and return type annotated. `mypy --strict` enforced. No `Any` unless absolutely unavoidable.
- **`uv` is the package manager** — no pip, no poetry.
- **No HITL/OIDC/enterprise code** — zero tolerance.
- **Code comments** — who/why/boundaries, not what.

### CI (GitHub Actions)

- `uv run ruff check .` — lint
- `uv run mypy --strict src/` — type check
- `uv run pytest tests/unit/` — unit tests on every PR
- Integration tests: manual or scheduled (need broker)

---

## 6. Versioning

- **Start at `v0.2.0`** — continues from monorepo's `v0.1.0`
- **SemVer.** Pre-1.0, breaking changes expected.
- **Package name:** `agentauth` on PyPI

---

## What Comes After Python

Once `agentauth-python` is extracted, cleaned, and verified against the live broker:

1. **TypeScript SDK** — same process: `git filter-repo` extraction from `agentauth-clients`, HITL removal, API alignment, `divineartis/agentauth-ts` repo. Same design, different language.
2. **Archive `devonartis/agentauth-clients`** — mark archived on GitHub, README points to new per-language repos.
3. **Phase 1 (Repo Cleanup)** — archive old `divineartis/agentauth`, rename `agentauth-core` -> `divineartis/agentauth`.

---

## Open Questions (resolve during implementation)

1. Go SDK — should `divineartis/agentauth-go` exist?
2. OpenAPI codegen — generate stubs instead of hand-writing? (Requires fixing TD-S14 first.)
3. HITL extension point design — deferred to Phase 4.
