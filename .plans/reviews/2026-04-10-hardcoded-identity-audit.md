# Hardcoded Identity Audit — agentauth-core

**Date:** 2026-04-10
**Trigger:** Discovery of hardcoded `iss: "agentauth"` in `internal/token/tkn_claims.go` during rebrand inventory
**Scope:** `internal/`, `cmd/` — production source + tests. Docs and config files covered separately in the rebrand inventory.
**Rule violated:** `~/.claude/CLAUDE.md` → "No Hardcoded Identity Values — Universal, Non-Negotiable"

---

## Summary

Five categories of findings, ranked worst to least-bad:

| Category | What | Severity | Count |
|----------|------|----------|-------|
| **C1** | Brand hardcoded as literal, zero config path | **CRITICAL** | 5 locations |
| **C2** | Config field exists, default literal bakes brand into binary | **HIGH** | 3 fields |
| **C2b** | Env var prefix `AA_` bakes brand into operator interface | **HIGH** | ~22 vars |
| **C3** | Tests assert the hardcode, actively preventing fix | **HIGH** | 7 assertions |
| **C4** | Test fixtures leak brand into SPIFFE URIs (inconsistent with other tests that already use `test.local`) | **MEDIUM** | ~45 occurrences |
| **C5** | Binary name `aactl` is its own separate identity, also hardcoded | **MEDIUM (decision needed)** | 227 occurrences |

**Blast radius for rebrand:** fixing C1 + C2 + C3 unblocks the rebrand entirely. C2b and C4 are cleanup. C5 is a separate naming decision.

---

## C1 — Brand hardcoded as literal, zero config path

**These are the worst. No config field exists. The brand is baked into compiled code.**

### C1-1 — JWT issuer validation
**File:** `internal/token/tkn_claims.go:62`
```go
if c.Iss != "agentauth" {
    return ErrInvalidIssuer
}
```
**Impact:** Every token minted by a rebranded broker would be rejected. Every federated broker must use the string `"agentauth"` or be rejected. **Wire-protocol break.**

### C1-2 — JWT issuer issuance
**File:** `internal/token/tkn_svc.go:141`
```go
Iss: "agentauth",
```
**Impact:** Every token claims this issuer. Multi-tenant deployments cannot distinguish which broker minted a token.

### C1-3 — Doc comments enshrining the hardcode
**File:** `internal/token/tkn_claims.go:8, 59`
```go
// The issuer is always "agentauth". Subjects are SPIFFE-format agent IDs
// ...
// It returns an error if the issuer is not "agentauth", the subject or JTI
```
**Impact:** Documentation teaches future readers the bug is a feature. Every reviewer who read this treated it as intentional design.

### C1-4 — System config path hardcodes brand in filesystem layout
**File:** `internal/cfg/configfile.go:21`
```go
locs = append(locs, "/etc/agentauth/config")
```
**Impact:** Operators rebranding the product still have to put config under `/etc/agentauth/`. FHS layout names the product in the filesystem hierarchy.

### C1-5 — CLI command name
**File:** `cmd/aactl/root.go:16`
```go
Use: "aactl",
```
**Impact:** Cobra uses this for help text, usage strings, and shell completion. Changing the binary name without updating this leaves inconsistent output.

---

## C2 — Config field exists, default literal bakes brand into binary

**Fields are configurable — operators CAN override — but the default is the brand. Running the binary without env vars ships an AgentAuth-branded system.**

### C2-1 — SPIFFE trust domain default
**File:** `internal/cfg/cfg.go:48, 77`
```go
TrustDomain: envOr("AA_TRUST_DOMAIN", "agentauth.local"),
```
**Impact:** Agents get `spiffe://agentauth.local/...` identities unless operator overrides. Brand leaks into every agent ID at runtime.

### C2-2 — SQLite database filename default
**File:** `internal/cfg/cfg.go:53, 82`
```go
DBPath: envOr("AA_DB_PATH", "./agentauth.db"),
```
**Impact:** Database file on disk is named after the brand. Operator backups, log mentions, `ls` output all show `agentauth.db`.

### C2-3 — JWT audience default
**File:** `internal/cfg/cfg.go:59, 96`
```go
c.Audience = "agentauth"  // default when AA_AUDIENCE unset
```
**Impact:** Tokens claim `aud: "agentauth"` by default. Verifiers in client libraries check this.

**Fix pattern for C2:** Either (a) make defaults empty and require operator to set, (b) derive defaults from a build-time `-ldflags` variable, or (c) keep the defaults but make them the neutral new brand from day one. Each has trade-offs — this is a decision to surface, not a mechanical fix.

---

## C2b — Env var prefix `AA_` bakes brand into operator interface

**File:** `internal/cfg/cfg.go:10-23, 45-63, 74-87, 132` and every doc + script + compose file that mentions env vars
**Count:** ~22 env var names starting with `AA_`

Every time an operator sets an env var, deploys a systemd unit, or reads a Docker compose file, they type the brand. The rebrand either:
- **Keeps `AA_*` forever** (ugly but safe — no operator disruption)
- **Flips to new prefix** (e.g. `AW_*`) with a transition period accepting both
- **Removes prefix entirely** (use viper-style nested config, env var mapping is derived)

This is a **decision point**, not a finding to fix blindly. Flag to user.

---

## C3 — Tests lock the hardcode in place

**These are the reason nobody could "just fix" the issuer. Any attempt to make it configurable turns these red and the fixer would assume the test was right.**

| File | Line | What it asserts |
|------|------|-----------------|
| `internal/cfg/cfg_test.go` | 65 | `c.Audience == "agentauth"` must be true after defaults load |
| `internal/token/tkn_svc_test.go` | 89 | Issued token has `claims.Iss == "agentauth"` |
| `internal/token/tkn_svc_test.go` | 208 | Builds claims with `Iss: "agentauth"` |
| `internal/token/tkn_svc_test.go` | 824 | Builds claims with `Iss: "agentauth"` |
| `internal/authz/val_mw_test.go` | 227 | `Iss: "agentauth"` in fixture |
| `internal/authz/val_mw_test.go` | 249 | `Iss: "agentauth"` in fixture |
| `internal/authz/val_mw_test.go` | 341 | `Iss: "agentauth"` in fixture |

**Smoking gun for the C1-1/C1-2 regression:** `internal/token/tkn_svc_test.go:521`
```go
// TestIssClaimMatchesConfig and TestVerifyRejectsWrongIssuer removed —
// IssuerURL is not present in this build.
```
Someone deleted the config-driven tests when they deleted the config field. The tombstone comment is right there in the file.

**Fix pattern:** These tests should drive `Iss`/`Audience` from the test's `cfg.Cfg` fixture, same way they already drive `TrustDomain` from a fixture in most tests. The pattern exists — it's just not applied consistently.

---

## C4 — Test fixtures leak brand into SPIFFE URIs

**The codebase already has two conventions for test SPIFFE URIs, and the newer/better one wins. This is inconsistency cleanup.**

### Already correct (uses neutral test domain)
- `internal/store/sql_store_test.go` — `spiffe://test/agent/...`
- `internal/store/sql_store_revoke_test.go` — `spiffe://test/agent/...`
- `internal/deleg/deleg_svc_test.go` — `spiffe://test/agent/...` (all 40+ fixtures)
- `internal/revoke/rev_svc_test.go` — `spiffe://example/agent/...`
- `internal/authz/val_mw_test.go` — mostly `spiffe://test/agent/...`
- `internal/handler/handler_test.go` — `spiffe://test/agent/...` and `spiffe://test.local/...`
- `internal/token/tkn_svc_test.go:825` — `spiffe://test.local/agent/...` (one place done right, rest wrong)

### Needs cleanup (leaks brand into fixtures)
| File | Count | Example |
|------|-------|---------|
| `internal/token/tkn_svc_test.go` | ~20 | `spiffe://agentauth.local/agent/orch-1/task-1/abc123` |
| `internal/mutauth/mut_auth_hdl_test.go` | ~8 | `spiffe://agentauth.local/agent/orch-1/task-1/inst-a` |
| `internal/mutauth/heartbeat_test.go` | 6 | same shape |
| `internal/mutauth/discovery_test.go` | 6 | same shape |
| `internal/identity/id_svc_test.go` | 2 | `TrustDomain: "agentauth.local"` passed to constructor |
| `internal/admin/admin_hdl_test.go` | 1 | `spiffe://agentauth.local/agent/orch/task/inst` |

**Fix:** Replace all `agentauth.local` in tests with `test.local`. Purely mechanical. Safe. Should be done as part of the rebrand because the `agentauth.local` string in tests is a search-and-replace hazard — easy to accidentally leave them if you only grep for production code.

---

## C5 — Binary name `aactl` is its own identity

**Not a finding per se, but a decision the rebrand forces.**

The CLI binary `aactl` (path `cmd/aactl/`, Cobra command name `aactl`, 227 occurrences across code, docs, scripts, tests) is named after "AgentAuth Control". It's a separate brand surface from the product name:

- **Product rebrands to X** → does the CLI become `xctl`? `x-cli`? keep `aactl`?
- **Binary name appears in:** docs, install instructions, shell completions, user muscle memory, third-party tutorials, container images, systemd units, pre-existing user scripts

**Recommendation:** Pick the new name as part of the rebrand spec. Treat `aactl` → `<new>` as a mechanical rename (dir, Cobra command, docs, scripts) separate from the semantic config fixes in C1–C3.

---

## Recommended execution order

1. **C1 + C3 together (atomic)** — restore `Issuer` / `Audience` config fields, drive tests from fixtures, remove the hardcoded literals. Single PR. Reverts the regression that `tkn_svc_test.go:521` admitted to. **This unblocks the rebrand.**
2. **C2 defaults decision** — user picks: empty-required / build-time / new-brand-literal. Mechanical once decided.
3. **C4 cleanup** — mechanical sed over `internal/*_test.go`. Safe.
4. **C2b env var prefix decision** — surface to user, execute per decision.
5. **C5 binary name decision** — surface to user, execute as part of rebrand PR.

C1 + C3 is tech debt that **should be fixed regardless of rebrand**. If you decided never to rebrand, this still deserves to get done — it's a correctness and multi-tenancy bug, not just a naming issue.

---

## Proposed tech-debt entries

- **TD-TOKEN-001 (CRITICAL)** — Restore configurable JWT issuer; remove hardcoded `"agentauth"` literal in `tkn_claims.go` + `tkn_svc.go`. Restore deleted tests (see `tkn_svc_test.go:521` tombstone).
- **TD-TOKEN-002 (HIGH)** — Restore configurable JWT audience; remove hardcoded default `"agentauth"` in `cfg.go:96`.
- **TD-CFG-001 (HIGH)** — Audit all defaults in `cfg.go` for brand leakage (TrustDomain, DBPath, Audience). Decide default strategy.
- **TD-CFG-002 (HIGH, decision)** — Env var prefix strategy (`AA_*` vs new vs derived).
- **TD-CFG-003 (CRITICAL)** — `/etc/agentauth/config` hardcoded search path in `configfile.go:21`; operators cannot follow FHS under a different brand name without forking.
- **TD-TEST-001 (MEDIUM)** — Test fixtures leak `agentauth.local` into SPIFFE URIs; unify on `test.local` across all test files.
- **TD-CLI-001 (MEDIUM, decision)** — Binary name `aactl` rebrand strategy.

All entries to be added to `TECH-DEBT.md` at repo root after user reviews this audit.

**Status 2026-04-10:** 7 entries added to `TECH-DEBT.md` under section "Hardcoded Identity Audit (2026-04-10)". Final IDs: TD-TOKEN-001, TD-TOKEN-002, TD-TOKEN-003, TD-CFG-001, TD-CFG-002, TD-TEST-001, TD-CLI-001.

---

## Recommendations & Decisions (2026-04-10)

This section captures decisions made in session after the raw findings were analyzed. Each decision is paired with the reasoning so future sessions can evaluate whether the call still holds.

### Decision 1 — Root cause is OIDC coupling, not a one-off mistake

**What:** The `IssuerURL` field was stripped from `internal/cfg/cfg.go` and the validation was replaced with the hardcoded literal `"agentauth"` at some point during the open-core split. The tombstone at `internal/token/tkn_svc_test.go:521` (`"TestIssClaimMatchesConfig and TestVerifyRejectsWrongIssuer removed — IssuerURL is not present in this build."`) admits the config-driven tests were deleted alongside the config field.

**Why this happened:** `IssuerURL` was probably named that way because it was coupled to the OIDC provider (which needs a URL for JWKS discovery at `/.well-known/openid-configuration`). When OIDC got ripped out for the open-source split, someone treated `IssuerURL` as "OIDC stuff" and deleted it. They didn't notice that the plain JWT `iss` claim is a separate, simpler concern that core still needs — every JWT issuer needs a string identifier, with or without OIDC.

**Implication for the fix:** Do NOT restore `IssuerURL` by that name. The name drags OIDC baggage back in. Instead, add a minimal `cfg.Issuer string` field — just a string, no URL semantics, no discovery, no JWKS. When the enterprise OIDC module plugs back in via the module-based architecture, IT owns the OIDC-shaped issuer URL and JWKS endpoints; it reads `cfg.Issuer` as input or layers its own semantics on top. Clean boundary, no coupling.

**Decision is NOT to git-blame the regression.** The user explicitly declined the archaeology: "I dont need you to do that that should not be there we remove the OIDC so we can do an open source it does not matter when it was done." The fix is forward-looking.

### Decision 2 — Defaults strategy: respect what exists, fix the bug that's actually there

**Corrected 2026-04-10** after the user pointed out a distinction Claude had been flattening across multiple flips. The earlier "all identity values required, no default" rule was over-broad. The actual rule is finer-grained: the right fix depends on **what already existed in the code**, not on a sweeping classification of fields as "identity" vs "infrastructure."

**The principle:** Don't bake the brand into source. Configurable, brand-free, fail-loud-on-missing-when-no-default-makes-sense.

**The three failure modes the audit found, and the right fix for each:**

| Failure mode | Example | Right fix |
|--------------|---------|-----------|
| **No config path at all** — pure hardcoded literal in source, no env var, no config field | `Iss: "agentauth"` in `tkn_svc.go:141` | **Add a config field, make it required, no default.** There's no existing behavior to preserve. The operator must declare the value once. Fail-loud at startup if missing. |
| **Configurable, but the default literal leaks the brand** | `TrustDomain: envOr("AA_TRUST_DOMAIN", "agentauth.local")` | **Keep the default pattern, swap the literal for a neutral one.** The bug is the brand in the literal, not the existence of a default. Operators who already rely on the default keep working; the brand stops appearing in source. |
| **Code violates its own documented contract** | `cfg.go:22` comment says `"empty = skip"`, but `cfg.go:96` overrides unset → `"agentauth"` | **Fix the override, honor the contract.** Don't escalate to required, don't add a new default — just delete the offending line. The documented behavior was already correct; the implementation drifted from it. |

**Per-field resolution:**

| Field | Failure mode | Fix | TD |
|-------|-------------|-----|------|
| `Issuer` (new) | No config path | **Required, no default** — add field, fail at startup if empty | TD-TOKEN-001 |
| `TrustDomain` | Brand-coupled default `"agentauth.local"` | **Keep default**, swap literal to `"broker.local"` (or similar neutral value) | TD-CFG-001 |
| `DBPath` | Brand-coupled default `"./agentauth.db"` | **Keep default**, swap literal to `"./data.db"` | TD-CFG-001 |
| `Audience` | `cfg.go:96` overrides unset → `"agentauth"`, violating `cfg.go:22` contract that says empty=skip | **Delete the override line**, honor the documented contract — stays optional, empty stays skip | TD-TOKEN-002 |
| `SigningKeyPath` | Already neutral (`./signing.key`) | **No change** | — |
| `Port`, `BindAddress`, `LogLevel`, `TLSMode`, `DefaultTTL`, `MaxTTL`, `AppTokenTTL` | Already neutral, no brand coupling | **No change** | — |
| Hardcoded `/etc/agentauth/config` search path | Brand in filesystem layout, no override mechanism | **Replace with `--config <path>` CLI flag + cwd-relative default**. No brand in source. | TD-CFG-002 |

**Why this is more honest than the previous "all identity required" framing:**
- It matches the actual shape of the bugs in the code. Three different bugs, three different fixes.
- It avoids treating fields that already work (`TrustDomain` is configurable, just badly defaulted) the same as fields that don't work at all (`Issuer` has no config path).
- It minimizes churn on the test suite, compose files, and env.sh files. Only `Issuer` requires the cascading "set it everywhere" change because only `Issuer` is the new-required-field case.
- It avoids the trap Claude kept falling into: applying a sweeping rule when the situation called for surgical fixes.

**Pattern to remember:** When the user says "no hardcoded identity values," that means the BRAND must not appear as a literal in source code. It does NOT mean every identity field must be required-no-default. A neutral default that operators override is still configurable; only a brand-coupled default violates the rule.

### Decision 3 — Keep `AA_*` env var prefix indefinitely

**Decided by user, 2026-04-10:** `AA_*` stays.

**Reasoning:** Two letters, neutral enough to not obviously read as "AgentAuth" to a new operator. Operator-facing env vars are the highest-friction change in any rebrand (deployment scripts, systemd units, Docker compose files, CI/CD secrets, tribal knowledge). The cost of flipping is far higher than the brand hygiene benefit at this stage.

**Re-evaluate at 1.0 if ever.** Until then, do NOT touch this.

**Not creating a TD entry for it** — intentional inaction is not debt.

### Decision 4 — Binary rename: `aactl` → `awrit`

**Decided by user, 2026-04-10:** `awrit`, not my initial recommendation of `awctl`.

**Reasoning the user's call is better than mine:** I initially recommended `awctl` using syntactic arguments (parallel shape to `aactl`, `ctl` suffix convention matching `kubectl`/`systemctl`, 5-char muscle memory). The user overrode with `awrit` in seconds. The semantic point I missed: the binary name IS the brand — AgentWrit → `awrit` is direct. `awctl` hides the brand behind a convention suffix, which dilutes the product identity. The `ctl` suffix is a convention for services where the product name is a noun (Kubernetes → `kubectl`); when the product name is itself action-flavored, the suffix is redundant.

**Lesson saved** as session feedback: when making naming decisions, consider whether the name carries the brand or hides it. Syntactic reasoning (shape, length, parallel construction) is secondary to semantic fit.

### Decision 5 — One tech-debt entry at a time, not bundled PRs

**Decided by user, 2026-04-10:** Handle the 7 tech-debt entries one (or at most two) at a time. Per-TD discussion → decision → focused fix → merge → next. Do NOT bundle into a large "PR 1 correctness" that touches everything.

**Why this is better than my initial bundling recommendation:**
- Each fix gets focused review. A 7-item bundled diff buries the judgment calls — small mistakes hide in noise.
- Each merge is reversible independently. If TD-TOKEN-001's approach turns out wrong, it rolls back without taking TD-CFG-001 with it.
- Discussion is scoped. "What should the `Issuer` field default to" is a different conversation from "what should the `DBPath` default be" — bundling them forces the user to context-switch mid-decision.
- Green CI is cheaper. A 50-line focused diff runs gates in ~6 minutes and passes or fails cleanly. A 500-line bundled diff has correlated failures that take longer to triage.
- Rollout is safer. Each fix that ships is a finished thing; the codebase is never half-migrated across an in-progress bundle.

**Trade-off accepted:** The `internal/cfg/cfg.go` and test files will be touched repeatedly — 5 of the 7 TDs overlap those files. Each subsequent PR has minor merge-conflict risk with the last. Worth it for the focused review benefit.

**Module path rename** (`github.com/devonartis/agentauth` → future name) remains out-of-scope for this audit. Only needed when the GitHub repo itself is renamed. Until then, the Go module path staying `agentauth` is fine.

### Priority ranking of the 7 TD entries

Severity from TECH-DEBT.md rolled up, with execution grouping:

**PR 1 — Correctness (must ship together, atomic):**
1. **TD-CFG-002** (CRITICAL) — `/etc/agentauth/config` hardcoded FHS path. Blocks any operator who wants to rebrand the filesystem layout.
2. **TD-TOKEN-001** (CRITICAL) — `iss` claim hardcoded. Wire-protocol break for any federation/multi-instance scenario.
3. **TD-TOKEN-002** (HIGH) — `aud` claim default violates its own contract.
4. **TD-CFG-001** (HIGH) — `TrustDomain` / `DBPath` defaults bake brand into binary.
5. **TD-TOKEN-003** (HIGH) — Tests lock the hardcode. Must be rewritten *in the same PR* as TD-TOKEN-001/002 or the tests go red on merge.
6. **TD-TEST-001** (MEDIUM) — Test SPIFFE fixtures leak brand. Mechanical sed. Include in PR 1 because the tests are being rewritten anyway — no reason to touch the same files twice.

**PR 2 — Mechanical rename (can parallel PR 1):**
7. **TD-CLI-001** (MEDIUM) — `aactl` → `awrit`. Trivial in isolation, noisy diff, zero logic change.

**Rationale for bundling everything in PR 1:** every field in PR 1 touches `internal/cfg/cfg.go`, `internal/cfg/cfg_test.go`, `internal/token/tkn_svc.go`, `internal/token/tkn_claims.go`, `internal/token/tkn_svc_test.go`, or the tests that depend on those packages. Splitting the CRITICAL entries from the HIGH entries would force reviewers to review overlapping file diffs twice. Bundling lets the review happen once.

**Rationale for keeping PR 2 separate:** `cmd/aactl/` rename has a blast radius (docs, scripts, CHANGELOG, tests, shell completion, container images) that doesn't overlap with the config/token fixes. A single failed step in the rename doesn't risk the correctness fix.

### Execution plan

Once this audit is approved:

1. **Spec** — write `.plans/specs/2026-04-10-remove-hardcoded-identity-spec.md` covering PR 1. Must include: every file changed, every test rewritten, the fail-fast startup story, the CHANGELOG BREAKING entry, and the 13-gate CI validation path. One spec for PR 1 only. PR 2 gets its own mechanical-rename spec later.
2. **User review of spec** — no code until the spec is approved.
3. **Execute PR 1** via `superpowers:executing-plans` against a feature branch off `develop`.
4. **CI passes all 13 gates + acceptance story** (broker refuses to start without `AA_ISSUER`) → merge to develop.
5. **Resolve TD entries** in the same merge commit (TD-TOKEN-001, TD-TOKEN-002, TD-TOKEN-003, TD-CFG-001, TD-CFG-002, TD-TEST-001 → RESOLVED).
6. **PR 2** — mechanical `aactl` → `awrit` rename, separate branch, separate review, separate CI run.

### Out of scope for this audit

- **Docs rebrand sweep** (README brand name, `docs/agentauth-explained.md` filename + content, Decision 017 intro copy application). Lower priority, separate cycle, no code correctness impact.
- **Go module path rename** (`github.com/devonartis/agentauth` → future name). Only needed when the GitHub repo itself is renamed.
- **Domain / email placeholder updates** (TD-019). Already tracked separately, blocks public release but not the rebrand mechanics.
- **Decision 017 intro copy application** to website and core README. Copy exists in Obsidian KB, needs separate PR focused on marketing/positioning content.

---

## Review checklist for user

Before approving this audit and moving to the spec phase, confirm:

- [ ] The root cause framing (OIDC coupling regression) is accurate or correctable
- [ ] The per-field defaults resolution in Decision 2 matches your intent
- [ ] The PR 1 / PR 2 / PR 3 split is the right scope boundary
- [ ] The priority ranking is right (critical-first, mechanical-last)
- [ ] Nothing has been overlooked that should be in this audit (especially: are there other magic strings in `internal/` or `cmd/` that I missed?)
- [ ] The out-of-scope list is correct — nothing has been excluded that should be included

If all six check out, next step is the spec for PR 1.

---

## Independent review — Codex (2026-04-10)

After multiple reversals in-session, Claude requested an independent review from Codex (GPT-5) to verify the TD-TOKEN-001 fix proposal was sound. Codex read the actual codebase, not just this audit doc. **Verdict: SOUND WITH CAVEATS** — core design (default `"broker"`, move issuer check from `Validate()` to `Verify()`, no `cfg.Cfg` into claims) is correct, but five specific items in Claude's proposal were wrong or incomplete.

### What the review corrected

**1. `cfg_test.go:65` is AUDIENCE, not issuer.** Claude bundled it under TD-TOKEN-003 (issuer test assertions). It belongs to TD-TOKEN-002 (audience hardcode). The pure set of issuer-specific test assertions is **6**, not 7: `tkn_svc_test.go:89`, `:208`, `:824`, `val_mw_test.go:227`, `:249`, `:341`. `cfg_test.go:65` will get fixed separately when TD-TOKEN-002 is worked.

**2. More `Verify()` callers than Claude listed.** Claude said "`val_mw.go:90`, `mut_auth_hdl.go` (4 sites), tests." Actual production callers: `val_mw.go:90`, `handler/val_hdl.go:55`, `mut_auth_hdl.go:92,127,145,193`, and the Renew path itself at `tkn_svc.go:252` (Renew calls Verify). **Claude missed `handler/val_hdl.go:55` entirely.** This doesn't change the fix — everything still flows through `TknSvc.Verify()`, so the fix lands in one place — but the audit trail needs to list all call sites so nothing is missed during code review.

**3. `cfg.Cfg{}` literal constructions in tests are a landmine.** Multiple test files construct `cfg.Cfg{}` directly instead of calling `cfg.Load()`: `tkn_svc_test.go:50`, `deleg_svc_test.go:25`, `admin_svc_test.go:35`. If the `"broker"` default is applied only inside `cfg.Load()`, these test sites will have `Issuer == ""` at runtime and the new issuer check in `Verify()` will fail every test. **Fix options:**
- **(a)** Add the default in a helper like `cfg.WithDefaults(c *Cfg)` that both `Load()` and test constructors call
- **(b)** Update the three test sites to set `Issuer: "broker"` explicitly in their `cfg.Cfg{...}` literals
- **(c)** Have `Verify()` treat `s.cfg.Issuer == ""` as "check skipped" — rejected, re-introduces magic

Claude's pick: **(b)** for TD-TOKEN-001, because the three test sites are already aware of which fields they set (they set `TrustDomain` and `DefaultTTL` literally), and adding `Issuer` to the list is a one-line change per file. Option (a) is a cleaner abstraction but introduces a new helper for one field — premature.

**4. Config file is NOT a YAML file.** Claude said operators could set `issuer: broker.company.com` in the YAML config file. Wrong — `internal/cfg/configfile.go` currently supports only two keys in a simple `KEY=value` format: `MODE` and `ADMIN_SECRET`. There is no YAML. For TD-TOKEN-001, `AA_ISSUER` is **env var only**. If operators want config-file-based override of `Issuer` later, that's a separate tech-debt item (extend `configfile.go` to support more keys, or move to real YAML). **Do not promise YAML support in the docs.**

**5. Docs list was incomplete.** Claude listed `README.md`, `docs/getting-started-operator.md`, `docs/api.md` and the inline `cfg.go` comment. Codex found four more files that teach `iss="agentauth"` in example tokens and JWT payloads:
- `docs/api.md:219, 1063`
- `docs/common-tasks.md:235, 325`
- `docs/foundations.md:152`
- `docs/implementation-map.md:81`

All of these show example JWT payloads with `"iss": "agentauth"`. When TD-TOKEN-001 ships, the examples should update to `"iss": "broker"` (matching the new default) with a note that this is configurable via `AA_ISSUER`.

### What Codex confirmed

- **Default `"broker"` is the right call**, not required-no-default. Consistency argument: the repo is already default-heavy for local operation, with `DefaultTTL=300`, `MaxTTL=86400`, and `TrustDomain="agentauth.local"` all defaulting. Making only `Issuer` mandatory would be inconsistent.
- **Moving the issuer check from `Validate()` to `Verify()` is the best fit.** Do not pass `cfg.Cfg` into claims. The only viable alternative is `Validate(expectedIssuer string)` — but this repo has no production need for that extra API.
- **No SPIFFE or delegation coupling to `iss` exists.** SPIFFE subject generation is driven by `TrustDomain`, not `Issuer` (`identity/id_svc.go:200`). Delegation has no issuer-specific logic (`deleg_svc.go:126,152`). The issuer change is safe.
- **The enterprise module in the archive** (`~/proj/agentauth-ENT/pkg/broker/config.go:8`, `internal/token/tkn_svc.go:120,207`, `internal/oidc/oidc_hdl.go:99`) uses an OIDC-shaped issuer URL. When/if it plugs back in, it should stay enterprise-side and URL-shaped, NOT re-couple core's `cfg.Issuer` to OIDC semantics. Core's `cfg.Issuer` stays a simple string.
- **`TrustDomain` consistency note:** Codex flagged that `TrustDomain` still defaults to `"agentauth.local"`. When TD-CFG-001 is worked, the trust-domain default should follow the same pattern as `Issuer` — neutral default (e.g., `"broker.local"`), not brand-coupled. Not a finding against TD-TOKEN-001, but a forward-pointer for TD-CFG-001.

### Blind spots Codex noted

- Codex did not run tests. The "tests will fail because of `cfg.Cfg{}` literal construction" finding is a code-read prediction, not a test-run confirmation. When the fix lands, run `go test ./internal/token/... ./internal/deleg/... ./internal/admin/...` first and confirm the prediction before declaring the fix complete.
- Codex found no federation / multi-instance semantics that would make the simple-string `Issuer` wrong. This should hold for the current architecture. If federation returns via an enterprise module later, the module owns the URL-shaped issuer.

### Updated fix proposal for TD-TOKEN-001 (after Codex review + 2026-04-10 final correction)

**Final position: required, no default.** A second reversal happened after Codex's review when the user reasserted the principle. Codex's argument for `"broker"` default was **consistency with existing defaults** (TrustDomain, DefaultTTL, Port, BindAddress) — but TrustDomain's default is wrong for the same reason and is already tracked as TD-CFG-001. Codex was using one bug to justify another. The consistent position goes the other way: identity fields are required (Issuer, TrustDomain, Audience), infrastructure fields default (Port, BindAddress, LogLevel, TTLs).

The pattern Claude needs to recognize: every time there's tension between the **principle** (no hardcoded identity, ever) and **convenience** (tests and compose are easier with defaults), Claude defaulted to convenience. The user corrected this three times in-session. The principle wins. `"broker"` as a default is still a hardcoded literal that identifies what the system calls itself — it just renames the bug from `"agentauth"` to `"broker"`.

**The fix:**

1. Add `Issuer string` field to `cfg.Cfg`, env `AA_ISSUER`, **REQUIRED — no default**
2. `cfg.Load()` returns an error if `AA_ISSUER` is unset or empty. **Error message points operators at `aactl init`**, matching the existing pattern at `cfg.go:110` for the admin secret error. Format: `"AA_ISSUER is not set. Run 'aactl init' to set up your broker config interactively, or set AA_ISSUER=<value> in your environment."` One sentence, one tool, one fix path. The launcher handles all the explaining and prompting; the error message just routes the operator to it.
3. Update **three test sites** that construct `cfg.Cfg{}` literals to include `Issuer: "test-issuer"` (or similar test value): `tkn_svc_test.go:50`, `deleg_svc_test.go:25`, `admin_svc_test.go:35`
4. Update `internal/cfg/cfg_test.go` happy-path tests to set `AA_ISSUER` in their env setup. Add a new startup-failure test: `cfg.Load()` returns the expected error when `AA_ISSUER` is empty.
5. `tkn_svc.go:141`: `Iss: s.cfg.Issuer`
6. `tkn_claims.go:62`: remove issuer check from `Validate()`
7. `tkn_svc.go:227` (inside `Verify()`): add issuer check after `claims.Validate()` call, comparing `claims.Iss` to `s.cfg.Issuer`
8. Delete doc comments at `tkn_claims.go:8,59` that enshrine the hardcode
9. Update **6 test assertions** (not 7 — `cfg_test.go:65` is TD-TOKEN-002 scope): `tkn_svc_test.go:89,208,824`, `val_mw_test.go:227,249,341` — driven from the fixture's `Issuer` value, not literals
10. Update **7 doc files**: `cfg.go` inline comment, `README.md`, `docs/getting-started-operator.md`, `docs/api.md:219,1063`, `docs/common-tasks.md:235,325`, `docs/foundations.md:152`, `docs/implementation-map.md:81`. JWT example payloads change from `"iss": "agentauth"` to `"iss": "<your-issuer>"` with a note that this is set via `AA_ISSUER`.
11. **Update Docker compose files** — `docker-compose.yml`, `docker-compose.tls.yml`, `docker-compose.mtls.yml` — each adds `AA_ISSUER: broker.local` (or similar dev-only placeholder) to the broker service's environment block. The dev placeholder is fine in compose files because compose files are infrastructure code, not source code; they exist to make local dev work and the value is operator-overridable per environment. Production deployments substitute their own value.
12. **Update `scripts/stack_up.sh`** to set `AA_ISSUER` before launching the stack. Same pattern as the existing `AA_ADMIN_SECRET` handling.
13. **Update 7 acceptance test env.sh files**: `tests/p0-production-foundations/env.sh`, `tests/p1-admin-secret/env.sh`, `tests/sec-l1/env.sh`, `tests/sec-l2a/env.sh`, `tests/sec-l2b/env.sh`, `tests/sec-a1/env.sh`, `tests/app-launch-tokens/env.sh`. Each adds `export AA_ISSUER=test-broker` (or similar test-only value) so the broker boots during acceptance runs.
14. **Extend `cmd/aactl/init_cmd.go` to prompt for `AA_ISSUER`** during the interactive setup flow. Same pattern as the existing admin-secret prompt. The prompt should explain briefly what the value is (one line: "Unique identifier for this broker — typically your domain or service name") and offer a sensible suggestion the operator can accept or override. Generated config file includes the `AA_ISSUER` line. After `aactl init` runs, the broker boots without further configuration.
15. CHANGELOG entry: `Added cfg.Issuer / AA_ISSUER — broker identifier for JWT iss claim. Now required at startup; broker refuses to boot without it. Previously hardcoded as "agentauth". Operators upgrading must set AA_ISSUER before launching.` Not "BREAKING" in a public-API sense (the repo is private/pre-public), but worth a clear note in the changelog so any future contributor sees the upgrade requirement.
16. **NO YAML config file update** — `configfile.go` doesn't support arbitrary keys (only `MODE` and `ADMIN_SECRET`). Env var is the only source for `AA_ISSUER`. If config-file support is wanted later, that's a separate tech-debt item.
17. Verify all seven `Verify()` call sites continue to work without changes (`val_mw.go:90`, `handler/val_hdl.go:55`, `mut_auth_hdl.go:92,127,145,193`, `tkn_svc.go:252`). They all flow through `TknSvc.Verify()`, which is the only place that needs the new check.

**File count:** ~17 files touched (production code, tests, compose files, env.sh files, docs, `aactl init`). Bigger than the "default broker" version because every place that boots a broker needs `AA_ISSUER` set explicitly. Worth it: the identity literal never lives in source code, the rule is honored without exception, and the broker fails loudly if anyone forgets — no silent shared-default footgun.

**Pre-work: document `aactl init` properly first.** Before TD-TOKEN-001 lands, `aactl init` needs proper standalone documentation. Today it's referenced indirectly (e.g. in the admin-secret error message at `cfg.go:110` and in test stories), but there's no canonical "this is the setup tool, here's how to use it, here's what it generates" page. Add or expand:
- A dedicated section in `docs/getting-started-operator.md` titled "First-time setup with `aactl init`" — what it does, what it prompts for, what file it writes, where it writes it
- A row in `docs/aactl-reference.md` covering the `init` subcommand with all flags and the interactive flow
- A README mention in the Quick Start section pointing operators at `aactl init` as the first step

Once that documentation exists, the broker error messages can confidently point at `aactl init` knowing the operator will find clear instructions when they arrive.

**Order of operations for the fix:**
1. Document `aactl init` (the docs above)
2. Extend `aactl init` to prompt for `Issuer` and write it to the generated config
3. Add `cfg.Issuer` field + required-field validation in `cfg.Load()`
4. Replace the hardcoded literals in `tkn_svc.go` and `tkn_claims.go` with config-driven values
5. Update tests, compose files, env.sh files, docs (the rest of the 17-file list)
6. Run `gates.sh full` locally, then push and let CI validate

No new tech-debt entries created — the launcher pattern keeps the scope inside TD-TOKEN-001.

**Why this is the right call:**
- The whole point of this audit was that hardcoded identity literals are unacceptable. `"broker"` is still a hardcoded identity literal. Replacing `"agentauth"` with `"broker"` doesn't fix the bug; it renames it.
- The "consistency with other defaults" argument falls apart when you realize TrustDomain and Audience defaults are also bugs (TD-CFG-001, TD-TOKEN-002). The consistent fix is to make all three identity fields required, not to preserve the wrong pattern.
- The convenience cost is bounded and one-time. Once the env.sh and compose files are updated, no future test or dev workflow needs to think about it again.
- The fail-loud behavior catches deployment mistakes early. Today, if a test forgets to set the issuer, it silently runs with `"agentauth"`. After the fix, it crashes at startup with a clear error message — easier to debug, impossible to miss.

This is the version to implement. No more flipping. The principle wins.
