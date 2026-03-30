---
name: cherrypick-devflow
description: >
  Use for cherry-picking commits from the legacy agentauth repo into agentauth-core.
  This is a migration workflow — not for new feature development (use devflow for that).
  Trigger on: "cherry-pick", "migrate commits", "bring over P0", "bring over security",
  "cherry-pick next batch", "continue migration", "what's left to cherry-pick",
  "resume cherry-pick". Guides the operator through picking, conflict resolution,
  testing, and verification for each batch of commits.
---

# Cherry-Pick Migration Flow

## On Invoke

1. Read `MEMORY.md` — find the cherry-pick batch tracker to know which batch is current
2. Read `FLOW.md` — find what step within the current batch we're on
3. Read the **Cherry-Pick Guide** from the legacy repo:
   `/Users/divineartis/proj/agentauth/.plans/modularization/Cherry-Pick-Guide.md`
   — this has the exact commits, conflict expectations, and verification for the current batch
4. If conflicts are expected, also read the **Feature Inventory** Part 2 ("Modified Files" table):
   `/Users/divineartis/proj/agentauth/.plans/modularization/Cowork-Feature-Inventory.md`
   — this has the keep/drop list for each file

## Per-Batch Flow

Each batch goes through these steps. Do them in order. Do not skip.

### 1. Analyze

Before cherry-picking, delegate to an `Explore` agent to inspect every commit in the batch:

```bash
git -C /Users/divineartis/proj/agentauth show <hash> --stat
git -C /Users/divineartis/proj/agentauth diff <hash>^..<hash>
```

For each commit, answer: What does it change? Does the diff touch any add-on code (approval/, oidc/, cloud/, hitl)? What's the conflict risk?

Save to `.plans/cherry-pick/B<N>-analysis.md`.

Do not cherry-pick until analysis is written.

### 2. Pick

Run the cherry-pick commands from the Cherry-Pick Guide. The guide has them per-batch.

If conflicts occur, resolve using the Feature Inventory's "Modified Files" table:
- **Keep** everything in the "Core Additions" column
- **Drop** everything in the "DO NOT Include" column
- **Drop** any reference to: hitl, approval, oidc, federation, issuer, cloud, thumbprint, jwk, sidecar

Document every conflict resolution: which file, what was kept, what was dropped.

### 3. Verify

Run the automated test script. This is the single gate — it covers compile, unit tests,
contamination, Docker build/start, smoke test, and batch-specific checks:

```bash
./scripts/test_batch.sh B<N> --all
```

The script runs gates G1–G7, prints structured PASS/FAIL per gate, and appends evidence
to `.plans/cherry-pick/TESTING.md` automatically.

**Modes available:**
- `--go-only` — G1-G3 only (compile, unit tests, contamination). Use when Docker is not available.
- `--docker` — G4-G6 only (Docker build, start, smoke test). Use when Go gates already passed.
- `--smoke` — G6 only (broker must already be running).
- `--all` — G1-G7 (default, recommended for merge verification).

**Hard stop if any gate FAILs.** Fix the issue, re-run, and do not proceed until all gates are green.

### 4. Update Application Docs

**This is not optional.** B0-B4 deferred doc updates caused 54 findings (8 CRITICAL). Every batch that changes behavior MUST update the application docs in the same branch.

**Doc files to check:** `docs/api.md`, `docs/architecture.md`, `docs/concepts.md`, `docs/implementation-map.md`, `docs/getting-started-operator.md`, `docs/scenarios.md`, `docs/api/openapi.yaml`.

For each file, ask: "Does this batch change anything this doc describes?" If yes, update it. Specifically:

- **api.md** — new endpoints, changed response shapes, changed error messages, new headers
- **architecture.md** — new middleware, changed request lifecycle, new components in diagrams
- **concepts.md** — new security properties, changed component descriptions
- **implementation-map.md** — new source files, changed function behavior
- **getting-started-operator.md** — new config options, changed defaults, new security behavior operators should know about
- **openapi.yaml** — new endpoints, changed schemas, new headers

**Verification:** After updating, read each changed doc section and verify it matches the actual code. Do not trust sub-agent doc updates blindly — spot-check against the handler structs and middleware code using jcodemunch. The B4 doc disaster happened because no one verified the doc changes matched reality.

### 5. Acceptance Tests

Follow `tests/LIVE-TEST-TEMPLATE.md` exactly. Key rules:

- **Individual story files** — one `story-*.md` per story, not a bulk script dump
- **Executive-readable banners** — Who/What/Why/How/Expected must make sense to a non-technical reader
- **Real personas** — App (automated software), Operator (human managing), Security Reviewer (verifying controls). NOT "Developer (curl)" when the real actor is an App.
- **Ground in reality** — if using curl to emulate an app, say so. Describe the production scenario, not the testing mechanic.
- **One story at a time** — run, see output, then write verdict. Don't pre-write PASS.
- **VPS first, Container second** — compiled binary, then Docker
- **README.md** in evidence/ — summary table with all verdicts

If the batch has existing acceptance tests in `agentauth/tests/<batch>/`, adapt them for core (remove OIDC/HITL/sidecar, fix field names, fix registration flow to use challenge-response). Do NOT copy legacy tests without auditing every field name against the actual handler structs.

### 6. Review

Delegate to a `code-reviewer` sub-agent: compare what the batch was supposed to add (from the guide) against what actually landed (`git diff HEAD~<N>..HEAD`). Flag anything that doesn't match or shouldn't be there.

Save to `.plans/cherry-pick/B<N>-review.md`.

### 7. Record

- Save evidence to `.plans/cherry-pick/B<N>-evidence.md` (terminal output from build, tests, contamination check, spot-check)
- Update `MEMORY.md` batch tracker — mark batch done, add lessons learned
- Update `FLOW.md` — record what happened, set next batch
- Update `CHANGELOG.md` — what was added
- Commit: `git add -A && git commit -m "docs(cherry-pick): B<N> [name] — analysis, review, evidence"`

Announce: **"Batch B<N> done. Next: B<N+1>. Invoke cherrypick-devflow to continue."**

## Rules

- One batch at a time. B1 must be fully done before starting B2.
- Read the Cherry-Pick Guide fresh each time — it has the commits and conflicts. Do not memorize or hardcode them.
- Zero contamination tolerance. The grep check is a hard gate.
- Delegate analysis, review, and gate checks to sub-agents. Keep the main conversation focused.
- Evidence files are mandatory. Terminal output, not claims.
- CHANGELOG updated with each batch.
