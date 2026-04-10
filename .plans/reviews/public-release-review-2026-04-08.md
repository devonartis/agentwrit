# Public release review — 2026-04-08

**Scope:** First-pass review of security-sensitive areas, public docs, code quality, and `main` surface area for a future public release.  
**Not exhaustive:** Replace with deeper audits and external review before high-assurance production claims.

---

## 1. Security

| Area | Finding | Severity | Notes |
|------|---------|----------|--------|
| `internal/token/` | EdDSA JWT, `kid`, MaxTTL, revocation checks present in code paths reviewed | Info | Unit tests in `tkn_svc_test.go` |
| `internal/authz/` | Scope middleware + rate limiter on admin auth (`admin_hdl.go`) | Info | TD-S06 partially addressed by rate limiter |
| `internal/admin/` | bcrypt for admin secret; constant-time compare patterns | Info | Ensure `AA_ADMIN_SECRET` never logged |
| `internal/store/` | SQLite; JTI/revocation tables | Medium (ops) | TD-009 JTI pruning still relevant |
| **SECURITY.md accuracy** | Previously claimed ephemeral-only signing keys and no built-in rate limiting | **Fixed** | Aligned with persistent keystore and admin rate limiting |
| Secrets in repo | No hardcoded production secrets in `cmd/`/`internal/` from spot scan | Info | Test fixtures use obvious test values |

---

## 2. Documentation

| Item | Status |
|------|--------|
| `README.md` clone URL | Uses `devonartis/agentauth` |
| `docs/` v1.3 / agentauth-core | No `agentauth-core` or stray `v1.2` in `docs/` + `README` (grep) |
| `CONTRIBUTING.md` | **Fixed:** wrong clone URL (`agentauth/agentauth`), wrong module import example, obsolete `smoketest` tree |
| `SECURITY.md` | **Fixed:** wrong limitations; broken `KNOWN-ISSUES.md` link |
| `CODE_OF_CONDUCT.md` | **Added** (referenced by CONTRIBUTING) |

---

## 3. Code quality

| Check | Result |
|-------|--------|
| `go test ./...` | Run as part of `./scripts/gates.sh task` |
| Comments / TD-014 | Ongoing; large audit previously logged in FLOW |
| TODO/FIXME | Not fully enumerated; low priority for release gate |

---

## 4. Public surface (`main`)

| Check | Result |
|-------|--------|
| `git ls-tree -r main` | ~199 paths; dev files stripped (MEMORY, FLOW, `.plans/`, etc.) |
| `strip_for_main.sh` | Removes listed paths; refuses to run on `develop` for real runs |
| Tests on `main` | Live test evidence and `tests/` remain (intentional for transparency) |

---

## 5. License / business model

| Topic | Finding |
|-------|---------|
| Apache 2.0 on `main` | Allows commercial use and hosting; **does not** match “no sale / no host” without a different license |
| Next step | Product/legal chooses license; document decision in `FLOW.md`; update `LICENSE` + README in same release |

---

## 6. Recommended follow-ups (pre-public)

1. **Security:** External or second-agent pass on `internal/token`, `internal/handler`, `internal/store` SQL injection surfaces (parameterized queries verified in spot checks).
2. **Docs:** Resolve TD-012 / TD-014 items still open in `TECH-DEBT.md`.
3. **CI:** Ensure GitHub Actions (or equivalent) runs `gates.sh` on PRs to `main`.
4. **Email:** Confirm `security@agentauth.dev` is monitored before public SECURITY.md.
