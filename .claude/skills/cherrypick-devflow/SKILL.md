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

Three checks, all mandatory:

**Build + test:**
```bash
go build ./...
go test ./...
```

**Contamination check:**
```bash
grep -ri "hitl\|approval\|oidc\|federation\|issuer\|cloud\|thumbprint\|jwk\|sidecar" internal/ cmd/ --include="*.go"
```
Must return nothing. Hard stop if it doesn't.

**Spot-check:** Run the batch-specific test from the Cherry-Pick Guide. Each batch has a different thing to verify (keystore loads, aactl init works, etc).

### 4. Review

Delegate to a `code-reviewer` sub-agent: compare what the batch was supposed to add (from the guide) against what actually landed (`git diff HEAD~<N>..HEAD`). Flag anything that doesn't match or shouldn't be there.

Save to `.plans/cherry-pick/B<N>-review.md`.

### 5. Record

- Save evidence to `.plans/cherry-pick/B<N>-evidence.md` (terminal output from build, tests, contamination check, spot-check)
- Update `MEMORY.md` batch tracker — mark batch done
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
