# Independent Review: Hardcoded Identity Audit (2026-04-10)

**Reviewer:** Pi session (2026-04-10)
**Documents reviewed:**
- `.plans/reviews/2026-04-10-hardcoded-identity-audit.md`
- `TECH-DEBT.md` (Hardcoded Identity section)
- Source code in `internal/token/`, `internal/cfg/`, `cmd/aactl/`, and all `*_test.go` files

---

## Executive Summary

**The findings are real, confirmed, and the proposed solutions are sound.** I independently verified every claim in the audit against the source code and found zero fabricated or exaggerated findings. Every hardcoded `"agentauth"` string cited exists exactly where the audit says it does.

**However, I disagree with the severity framing in one important way:** the audit was written by an agent that treated this as a catastrophic break-the-world emergency. The language ("wire-protocol break", "CRITICAL", "smoking gun", "regression") makes it sound like the system is fundamentally broken. It is not. The system works correctly today — tokens issue, validate, renew, and revoke. The "bug" is that the issuer value can't be changed, which only matters when you want to rebrand.

**The right framing:** This is a **configurability debt** that blocks the rebrand to AgentWrit. It's not a correctness bug (tokens do validate correctly against the hardcoded value). But it IS something that needs fixing before the rebrand, and the fix should be done carefully.

---

## Finding-by-Finding Verdict

| Finding | Audit Claims | Verified? | My Severity | Notes |
|---------|-------------|-----------|-------------|-------|
| C1-1: Issuer validation hardcode | `tkn_claims.go:62` rejects any non-"agentauth" issuer | ✅ Exact match | **HIGH** (blocks rebrand) | Not CRITICAL — the system works. But you can't change the name without fixing this. |
| C1-2: Issuer issuance hardcode | `tkn_svc.go:141` always emits "agentauth" | ✅ Exact match | **HIGH** (blocks rebrand) | Paired with C1-1 — must fix both. |
| C1-3: Doc comments enshrine hardcode | Comments say "always agentauth" | ✅ Exact match | **MEDIUM** | Comments mislead, but don't break anything. Fix with the code. |
| C1-4: FHS path hardcode | `/etc/agentauth/config` in `configfile.go:21` | ✅ Exact match | **HIGH** | Also found `~/.agentauth/config` at line 24, and "AgentAuth Configuration" in generated file header at line 108 — audit missed the last two. |
| C1-5: CLI command name | `Use: "aactl"` in `root.go:16` | ✅ Exact match | **MEDIUM** | Mechanical rename. |
| C2-1: TrustDomain default | `"agentauth.local"` in `cfg.go:77` | ✅ Exact match | **HIGH** | Leaks into every SPIFFE ID. |
| C2-2: DBPath default | `"./agentauth.db"` in `cfg.go` | ✅ Exact match | **LOW** | Cosmetic. File on disk. |
| C2-3: Audience default | `"agentauth"` in `cfg.go` audience block | ✅ Exact match | **HIGH** | Violates own "empty = skip" contract. |
| C3: Tests lock hardcodes | 7 assertions + 1 tombstone | ✅ All confirmed | **HIGH** | Tombstone at line 521 confirms deliberate deletion. |
| C4: Test SPIFFE fixtures | ~45 occurrences in 6 files | ✅ Count confirmed | **MEDIUM** | Inconsistency cleanup. |
| C5: Binary name `aactl` | 24 occurrences in `cmd/` alone | ✅ Confirmed | **MEDIUM** | Decision-driven, not a bug. |

---

## Where I Agree With the Audit

1. **Root cause analysis is correct.** The OIDC-coupling explanation is confirmed by the tombstone at `tkn_svc_test.go:521-522`. Someone deleted `IssuerURL` alongside OIDC code, not realizing the plain JWT `iss` claim is independent.

2. **Decision 2 (defaults strategy) is sound.** Making `Issuer` and `TrustDomain` required (empty = startup failure) is the right call. Making `Audience` honor its own "empty = skip" contract fixes an actual bug. Neutral infrastructure defaults (`./data.db`) are sensible.

3. **Decision 3 (keep `AA_*` prefix) is correct.** Changing env var names is the highest-friction change in any rebrand. `AA_` is short, non-obvious as "AgentAuth" to a newcomer. Leave it.

4. **Decision 4 (`awrit`) is a good name.** Direct brand carry. The user's instinct was right — `awrit` carries the brand, `awctl` hides it.

5. **Decision 5 (one TD at a time) is correct.** Small focused changes are safer and easier to review.

6. **The execution order (C1+C3 first) is correct.** You must make the issuer configurable and update the tests in the same PR, or the tests go red.

---

## Where I Disagree With the Audit

### 1. Severity inflation — partially wrong, partially right

The audit uses "CRITICAL" and "wire-protocol break" language that implies the system is broken. It is not — the tokens work perfectly.

But the previous agent's instinct was pointing at something real: **this is bad production practice.** Production software should not have identity defaults baked in. If a value matters — issuer, trust domain, audience — the broker should **refuse to start** if the operator didn't set it. Silent defaults that carry a brand name are a code smell, whether they're hardcoded literals or `envOr()` fallbacks.

My revised severity:
- **HIGH** for the issuer (zero config path — worst offense)
- **HIGH** for TrustDomain (has a config path but a branded default that silently starts)
- **MEDIUM** for DBPath and Audience (lower-stakes defaults)
- **MEDIUM** for test fixtures and binary rename

### 2. C1-4 is undercounted

The audit lists only `/etc/agentauth/config` (line 21) but misses:
- `~/.agentauth/config` (line 24) — same pattern, also hardcoded
- `# AgentAuth Configuration` in the generated config file header (line 108) — the `WriteConfigFile` function writes "AgentAuth" into every config file it creates

### 3. The PR 1 scope is too large

The audit bundles 6 TD entries into "PR 1" (TD-CFG-002, TD-TOKEN-001, TD-TOKEN-002, TD-CFG-001, TD-TOKEN-003, TD-TEST-001). That's touching `cfg.go`, `cfg_test.go`, `tkn_claims.go`, `tkn_svc.go`, `tkn_svc_test.go`, `val_mw_test.go`, `configfile.go`, `admin_hdl_test.go`, `id_svc_test.go`, `mutauth/*_test.go` — at least 12 files in one PR.

Decision 5 says "one TD at a time," but then PR 1 bundles 6 of them. This contradicts the principle. I'd recommend:

- **PR 1a:** TD-TOKEN-001 + TD-TOKEN-003 (issuer field + test rewrites in token package only) — the atomic correctness fix
- **PR 1b:** TD-TOKEN-002 + TD-CFG-001 (audience + TrustDomain + DBPath defaults in cfg package) — the defaults cleanup
- **PR 1c:** TD-CFG-002 (configfile.go FHS paths) — filesystem layout
- **PR 1d:** TD-TEST-001 (SPIFFE fixture cleanup in all other test files) — mechanical sed

Each PR is small, focused, and independently revertable. PR 1a unblocks the rebrand. The rest is cleanup that can be parallelized.

### 4. The "startup failure" approach needs a migration path

Making `Issuer` required (empty = fatal) means existing deployments that don't set `AA_ISSUER` will break on upgrade. The fix should include:
- A clear error message: `"AA_ISSUER is required. Set it to the issuer name for this broker (e.g. 'agentwrit'). Previously this defaulted to 'agentauth'."`
- A mention in CHANGELOG as a BREAKING CHANGE
- Documentation in the getting-started guide

The audit mentions this in the "Review checklist" but doesn't spell it out as a requirement of the fix.

---

## The "Breaking" Concern

You mentioned one agent "acted like there are breaking" issues. Here's my honest assessment:

**The agent was right about the principle but overblown about the impact.** The real issue is simpler than the audit makes it sound: **production code shouldn't have hardcoded identity values — period.** No defaults, no branded fallbacks, no `envOr("AA_ISSUER", "agentauth")`. If the operator didn't explicitly configure it, the broker should error out at startup with a message telling them what to set.

This isn't "breaking" in the sense of data loss or security breach. It's a **code quality standard** that the codebase doesn't meet. The fix is straightforward:
- Add the config fields
- Remove the defaults (or make them `""`)  
- Fail fast at startup if they're unset
- Done

The previous agent was right to flag it. The dramatic language was unnecessary, but the underlying point — **no literals in production** — is correct.

---

## Recommended Path Forward

**The principle: No identity defaults in production code. If it matters, require it. If it's not set, fail fast.**

1. **Approve the audit** with the adjustments noted above
2. **Apply the "fail fast" principle consistently** — Issuer, TrustDomain, and Audience should all be required. DBPath should get a neutral generic default.
3. **Split PR 1 into smaller focused PRs** (1a through 1d)
4. **Execute PR 1a first** — add `Issuer` config field, make it required, fix the two code sites and their tests
5. **PR 2 (`aactl` → `awrit` rename)** can proceed in parallel with PR 1b-1d
6. **After all PRs merge**, do a final grep for any remaining `"agentauth"` strings in production code

Individual finding documents with solutions follow in this directory.
