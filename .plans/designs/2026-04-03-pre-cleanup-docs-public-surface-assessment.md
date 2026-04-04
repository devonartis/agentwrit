# Design: Pre-Cleanup Docs & Public Surface Assessment

**Created:** 2026-04-03
**Status:** DRAFT — review only, no cleanup actions taken
**Scope:** Pre-cleanup assessment of documentation alignment, code-to-doc drift, and files that likely should not ship publicly. This document is meant to guide a later cleanup plan without deleting or moving anything yet.

---

## Decision Summary

- This is a **review document only**. No cleanup work has been executed.
- The repo already has enough signal to plan cleanup safely:
  - the code gives a stable baseline for the real public API and runtime behavior
  - several public docs drift from that baseline
  - several files in the repo are clearly internal or pre-release only
- The most important doc-level cleanup themes are:
  1. **AgentAuth vs `agentauth-core` naming drift**
  2. **7-component / 8-component drift**
  3. **v1.2 / v1.3 pattern drift**
  4. **core audit vs downstream resource audit clarity gap**
  5. **public-facing docs referencing internal artifacts**
  6. **public repo contains internal-only materials**

---

## 1. Code Baseline Used for Assessment

The code baseline for this review came primarily from:
- `cmd/broker/main.go`
- `internal/handler/health_hdl.go`
- route comments and handler registrations in code
- package comments and tests around app/admin/auth routes

### Verified code facts

#### 1.1 Product/runtime API baseline
`cmd/broker/main.go` confirms the current route table includes:
- `GET /v1/challenge`
- `POST /v1/register`
- `POST /v1/token/validate`
- `POST /v1/token/renew`
- `POST /v1/token/release`
- `POST /v1/delegate`
- `POST /v1/revoke`
- `GET /v1/audit/events`
- `POST /v1/admin/auth`
- `POST /v1/admin/launch-tokens`
- `POST /v1/admin/apps`
- `GET /v1/admin/apps`
- `GET /v1/admin/apps/{id}`
- `PUT /v1/admin/apps/{id}`
- `DELETE /v1/admin/apps/{id}`
- `POST /v1/app/auth`
- `GET /v1/health`
- `GET /v1/metrics`

#### 1.2 Current broker version in code
- `cmd/broker/main.go` sets `const version = "2.0.0"`

#### 1.3 Health payload baseline
`internal/handler/health_hdl.go` confirms `GET /v1/health` returns:
- `status`
- `version`
- `uptime`
- `db_connected`
- `audit_events_count`

#### 1.4 Audit reality in core
From code + clarified product boundary:
- the core audit trail is about **credential lifecycle and related security events**
- it is not yet the same thing as a complete downstream resource-action log
- a resource server or downstream logging integration is needed for full end-to-end action telemetry

That distinction should drive public documentation.

---

## 2. Major Documentation Alignment Findings

## 2.1 Pattern version and component-count drift
This is the most visible public inconsistency.

### Found drift
- `README.md` still says **v1.2** and **7-component architecture**
- `docs/concepts.md` still says “how all the **7 components** work together” even though later in the same document it references **8 components**
- `docs/architecture.md` says “The **7-component** Ephemeral Agent Credentialing pattern maps directly to Go packages”
- `docs/getting-started-operator.md` says “the **7-component** security pattern”
- `README.md` references “7-component breakdown” in the docs index

### Why this matters
This is not cosmetic. It creates uncertainty about:
- what the actual pattern version is
- whether the docs are current
- whether the code and architecture story are in sync

### Pre-cleanup conclusion
A future cleanup must normalize all public docs to one story:
- **Ephemeral Agent Credentialing v1.3**
- **8 components**

---

## 2.2 Public naming drift: AgentAuth vs `agentauth-core`
### Found drift
- `README.md` tells users to clone `https://github.com/divineartis/agentauth-core.git`
- `docs/getting-started-user.md` also uses `agentauth-core`
- internal/history docs still naturally refer to `agentauth-core`, which is expected in internal files but not ideal in public-facing docs

### Why this matters
The user clarified the intended product name:
- product = **AgentAuth**
- repo under review = **OSS core**

Public-facing docs should not force users to mentally translate between:
- AgentAuth
- agentauth-core
- future rename/archive states

### Pre-cleanup conclusion
A cleanup phase should clearly separate:
- internal migration/history references
- public product/repo naming

---

## 2.3 Core audit vs end-to-end audit is not yet explained clearly enough
### Current reality
The code and clarified product direction indicate:
- core audit covers issuance, scope, TTL, credential lifecycle, security events
- end-to-end action logging belongs in a **resource server** or user-owned downstream logging system

### Documentation gap
Current docs heavily emphasize audit, but public docs do not yet sharply explain:
- what the broker audit proves
- what it does not prove by itself
- how a resource server extends the story
- how users can propagate `agent_id`, `task_id`, and delegation lineage into their own logs

### Why this matters
Without this distinction, users may assume the broker already records every downstream action. That would oversell the current OSS core and muddy the broader platform story.

### Pre-cleanup conclusion
This should be one of the central cleanup themes:
- **credential-lifecycle audit in core**
- **action-level audit in resource server / downstream logging integrations**

---

## 2.4 Config-file documentation drift
### Confirmed stale examples
`docs/troubleshooting.md` still references:
- `/etc/agentauth/config.yaml`
- `/path/to/config.yaml`
- `AA_CONFIG_PATH=/etc/agentauth/config.yaml`

But current repo docs and earlier project decisions indicate the config path format is now typically:
- `/etc/agentauth/config`
- `~/.agentauth/config`
- no `.yaml` suffix required by current design

### Additional issue
The same troubleshooting section uses `go run ./cmd/broker` in examples. That is not appropriate for live-test guidance and is inconsistent with the project rule that compiled binaries should be used for live/system testing.

### Pre-cleanup conclusion
`docs/troubleshooting.md` needs a focused pass for:
- config path examples
- startup examples
- consistency with `aactl init` behavior
- consistency with compiled-binary guidance

---

## 2.5 Public docs still expose internal decision references
### Examples found
- `docs/cc-design-decisions.md` references `TD-013 in TECH-DEBT.md`
- internal/public boundaries are blurred when public docs require internal artifacts to explain design choices

### Why this matters
Public docs should not require readers to consult:
- `TECH-DEBT.md`
- `MEMORY.md`
- `FLOW.md`
- other internal project-management files

### Pre-cleanup conclusion
If a design rationale belongs in public docs, it should be self-contained there. Public docs should not depend on internal files for essential meaning.

---

## 2.6 Public docs are broad, but some appear overlapping or transitional
The docs tree currently includes multiple concept-heavy files:
- `docs/agentauth-explained.md`
- `docs/cc-design-decisions.md`
- `docs/cc-foundations.md`
- `docs/cc-scope-model.md`
- `docs/cc-token-concept.md`
- `docs/token-roles.md`
- `docs/concepts.md`
- `docs/architecture.md`

### Assessment
These may all be useful, but pre-cleanup they look like a mix of:
- public conceptual docs
- authoring drafts
- internal educational material
- overlapping explanatory content

### Risk
Even when content is technically correct, overlap creates:
- discoverability issues
- maintenance burden
- drift risk
- uncertainty about which doc is canonical

### Pre-cleanup conclusion
A later cleanup should classify docs by audience and intent before deleting or merging anything.

---

## 3. Files and Content That Likely Should Not Be Public

This section is intentionally conservative. It identifies material that is very likely internal-only or pre-release-only.

## 3.1 Clearly internal project-memory / coordination artifacts
These should be treated as internal unless explicitly sanitized and intentionally published:
- `MEMORY.md`
- `MEMORY_ARCHIVE.md`
- `FLOW.md`
- `TECH-DEBT.md` (likely sanitize/remove for public release)
- `COWORK_SESSION.md`
- `COWORK_DOCS_AUDIT.md`
- `CLAUDE.md`
- `.claude/` settings and local tooling files

### Why
They expose:
- internal workflow
- decision process
- planning state
- coordination details
- repo-internal strategy and tradeoffs

---

## 3.2 Clearly internal audit/review artifacts
These look like internal working material, not public product docs:
- `audit/CONSOLIDATED-DOC-AUDIT.md`
- `audit/DOCS-AUDIT-REPORT.md`
- `audit/EVIDENCE-DOC-AUDIT-REPORT.md`
- `audit/UNIFIED-AUDIT-REPORT.md`
- `AgentAuth_Code_Review.docx`
- `AgentAuth_Doc_Audit.docx`
- `audit/AgentAuth_Doc_Audit.docx`

### Why
These are process artifacts, not user-facing docs.

---

## 3.3 Clearly internal / sensitive strategy artifacts
The strongest example is:
- `docs/patent/`

### Why
The patent folder includes:
- patent filing analysis
- legal/strategic commentary
- provisional patent specification material
- statements like “NEVER ship” are already echoed elsewhere in internal tracking

### Pre-cleanup conclusion
`docs/patent/` should be treated as internal-only.

---

## 3.4 Obviously unshippable or low-signal public-surface artifacts
These should be reviewed for removal or relocation before any public release:
- `tests/FUCKING QUETIONS.MD `
- `Archive.zip`
- `.DS_Store` files
- stray `.docx` artifacts in repo root or audit folders

### Why
These weaken professionalism and make the repo look uncurated.

---

## 3.5 Planning artifacts not intended as public product docs
Likely internal unless explicitly curated:
- `.plans/` contents broadly
- tracker files
- release-strategy drafts
- templates and review notes

### Why
These are useful internally but are not public documentation.

---

## 4. Code-to-Doc Alignment Snapshot

## 4.1 What appears aligned enough
Based on spot checks against code:
- route set in `docs/api.md` generally matches `cmd/broker/main.go`
- health payload fields appear aligned with `internal/handler/health_hdl.go`
- app/admin route split appears aligned with code comments and route registration
- app auth / admin auth / launch-token split is broadly documented correctly

## 4.2 What is visibly misaligned or risky
- 7 vs 8 component story
- v1.2 vs v1.3 story
- `agentauth-core` naming in public docs
- troubleshooting config path examples using `.yaml`
- public docs referencing internal artifacts like `TECH-DEBT.md`
- audit story not yet clearly split between broker lifecycle audit and downstream resource audit

---

## 5. Public-Surface Risks Beyond Docs

## 5.1 Internal repo history leaking into public narrative
Files like:
- `MEMORY.md`
- `FLOW.md`
- `TECH-DEBT.md`
- coordination docs

may be extremely useful internally, but they also reveal:
- migration history
- uncertainty
- internal debates
- roadmap and cleanup gaps

That may be acceptable privately, but it should be intentional if public.

## 5.2 Inconsistent professionalism signals
Examples:
- stray audit artifacts
- `.docx` files
- `Archive.zip`
- profanity-named test file
- hidden macOS junk files

These do not change the quality of the software, but they do affect perceived project maturity.

---

## 6. Recommended Pre-Cleanup Workstreams (Planning Only)

This section is not execution. It is the review-based structure for a later plan.

## Workstream A — Canonical public story
Define the canonical public answers for:
- product name
- repo name
- OSS core boundary
- broader platform boundary
- 8-component story
- audit story split

## Workstream B — Public doc alignment pass
Bring public docs into alignment with code on:
- pattern version
- component count
- route set
- config file paths
- startup examples
- naming

## Workstream C — Public/private classification
Classify every file into one of these buckets:
- public product doc
- public engineering doc
- internal strategy / planning
- internal audit artifact
- unshippable junk / archive material

## Workstream D — Release-surface hygiene
Handle obviously unshippable files and low-signal artifacts only after the classification pass is approved.

---

## 7. Highest-Priority Findings for the Cleanup Plan

If a later cleanup plan needs a first-pass priority order, these are the highest-signal items from this assessment:

1. **Normalize all public docs to v1.3 / 8 components**
2. **Clarify AgentAuth product vs OSS core vs broader platform**
3. **Clarify core audit vs resource-server / downstream action audit**
4. **Fix public naming drift (`agentauth-core`)**
5. **Fix stale config/troubleshooting examples**
6. **Remove or relocate clearly internal files from the intended public surface**
7. **Classify overlapping docs before deleting or merging anything**

---

## 8. Final Conclusion

The repo is not in “random cleanup” territory. It is in **classify first, then clean carefully** territory.

That is important because there is valuable material here:
- good public docs
- strong internal reasoning
- useful conceptual notes
- strategic artifacts that should not be lost

The right approach is:
1. establish the canonical public product story
2. compare docs to code
3. classify public vs internal
4. only then decide what to merge, move, sanitize, archive, or remove

This document should be the baseline for that later cleanup plan.
