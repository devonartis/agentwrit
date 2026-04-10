# Public release readiness (internal)

**Audience:** Maintainers and agents preparing `main` for a public GitHub release.  
**Not on `main`:** This file lives under `.plans/` and is removed by `scripts/strip_for_main.sh` on develop→main merge.

---

## Where internal notes live

| Content | Location (develop only) |
|--------|-------------------------|
| Checklists, license options, merge procedure | This file |
| Decisions (short) | `FLOW.md` |
| Session context | `MEMORY.md` |
| Deferred issues | `TECH-DEBT.md` (index) |
| Handoff | `AGENTS.md` |

Do **not** put internal-only release notes in `README.md`, `docs/*.md`, or `CHANGELOG.md` unless the text is intentionally public.

---

## Pre-merge checklist (develop → main)

1. **Branch:** All work committed on `develop`; CI/gates green.
2. **Dry-run strip:** On a throwaway branch or after `git checkout main && git merge develop --no-commit`, run `./scripts/strip_for_main.sh --dry-run` and confirm only expected paths are removed.
3. **Merge:** `git merge develop --no-commit` → `./scripts/strip_for_main.sh` → `git add -A` → commit with message like `merge: develop → main (stripped dev files)`.
4. **Verify:** `go build ./...`, `go test ./...`, `./scripts/gates.sh task` on the **stripped** tree (what `main` will contain).
5. **Push:** `main` to remote.

---

## License model (product / legal)

**Current state on `main`:** [LICENSE](../LICENSE) is **Apache License 2.0** — OSI-approved, permissive: commercial use, modification, distribution, and **private** or **commercial** hosting of derivatives are allowed subject to license terms (notice, patent grant, etc.).

**Stated product goal (from maintainers):** Allow commercial use and modification for personal/business use, but **restrict** resale of the software and/or use to operate a **hosted service for third parties** (exact wording TBD).

**Implication:** Those restrictions are **not** expressible under Apache 2.0 alone. Apache 2.0 cannot be “tweaked” with informal README rules; replacing the license requires a **vetted source-available** or **custom** license (or **dual licensing**: OSS under one terms, commercial under another).

**Options (summary — not legal advice):**

| Approach | OSI “open source”? | Typical use |
|----------|---------------------|-------------|
| **Keep Apache 2.0** | Yes | Full OSS norms; cannot block SaaS/hosting or resale of derivative products. |
| **Business Source License (BSL)** | No | Time-delayed conversion to OSS; may restrict commercial hosting during change period. |
| **Server Side Public License (SSPL)** | No | Strong restrictions on offering as a service; controversial compatibility. |
| **Elastic License 2.0 (ELv2)** | No | Restricts certain commercial offerings around the software. |
| **PolyForm / custom** | No | Tailored terms; requires lawyer review. |

**Decision record:** See `FLOW.md` (append-only) for what was **decided** and shipped. Until a replacement license is approved in writing, **`main` keeps Apache 2.0** and README/LICENSE stay aligned.

---

## Documentation hygiene (public)

- Clone URLs must use **`https://github.com/devonartis/agentauth.git`** (module + canonical repo).
- `CONTRIBUTING.md` must match the **two-binary** layout (`broker`, `aactl`); no references to removed commands.
- `SECURITY.md` must match **actual** behavior (signing key persistence, rate limits, etc.).
- `CODE_OF_CONDUCT.md` should exist if `CONTRIBUTING.md` references it.

---

## Grep audits (repeat before public flip)

```bash
# Stale public names (should be empty in docs/ + README + CHANGELOG)
rg -n "agentauth-core|github.com/agentauth/agentauth|v1\.2[^0-9]" docs README.md CHANGELOG.md CONTRIBUTING.md SECURITY.md

# Secrets (should only be in tests/docs examples)
rg -n "AA_ADMIN_SECRET|password|BEGIN.*PRIVATE" --glob '!*_test.go' cmd internal
```

---

## Post-public

- [ ] Enable GitHub Security Advisories / Dependabot as applicable.
- [ ] Confirm `pkg.go.dev` resolves for `github.com/devonartis/agentauth`.
- [ ] Tag releases (`v2.0.x`) from `main` after strip.
