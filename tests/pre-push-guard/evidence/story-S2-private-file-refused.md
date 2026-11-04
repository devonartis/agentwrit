# S2 — A Private File Is Refused [ACCEPTANCE]

Who: A developer who makes the exact slip the guard exists for, observed by the security reviewer.

What: `MEMORY.md` is one of the project's private files. It holds internal notes and must never
appear in the public repository. The developer force-adds it (`git add -f` overrides the ignore
list) and commits it with `--no-verify`, which skips the check that normally stops this at commit
time. We emulate that slip on purpose. They then try to push the branch `s2-private` to the
practice remote.

Why: If this push goes through, the private file is public. A public repository is copied and
archived by others within minutes, so removing it afterwards does not take it back. This hook is
the last thing standing between that slip and a permanent leak.

How to run: In the practice clone, branch from `develop` as `s2-private`. Create `MEMORY.md` with a
line of text, then `git add -f MEMORY.md`, then `git commit --no-verify`. Run
`git push origin s2-private`. Afterwards list the practice remote's branches.

Expected: The push is refused with the message "push rejected — private (gitignored) paths would
be pushed" and the line `MEMORY.md` is named. The remote still has only `s1-clean`. There is no
`s2-private` branch, so nothing arrived.

## Test Output

To $GUARD_TEST_DIR/remote.git
 * [new branch]      s2-private -> s2-private
  s1-clean
  s2-private
.githooks/pre-commit
.githooks/pre-push
.githooks/prepare-commit-msg
.github/CODEOWNERS
.github/MAINTAINERS
.github/dependabot.yml
.github/workflows/ci.yml
.github/workflows/codeql.yml
.github/workflows/contribution-policy.yml
.github/workflows/nightly.yml
.github/workflows/release.yml
.github/workflows/scorecard.yml
.gitignore
.golangci.yml
.gosec.yml
CHANGELOG.md
CODE_OF_CONDUCT.md
Dockerfile
LICENSE
MEMORY.md
README.md
SECURITY.md
cmd/awrit/apps.go
cmd/awrit/audit.go
cmd/awrit/client.go
cmd/awrit/init_cmd.go
cmd/awrit/init_cmd_test.go
cmd/awrit/main.go
cmd/awrit/output.go
cmd/awrit/revoke.go
cmd/awrit/root.go
cmd/awrit/token.go
cmd/broker/main.go
cmd/broker/serve.go
cmd/broker/serve_test.go
docker-compose.mtls.yml
docker-compose.tls.yml
docker-compose.yml
docs/README.md
docs/agentwrit-explained.md
docs/api.md
docs/api/openapi.yaml
docs/architecture.md
docs/awrit-reference.md
docs/common-tasks.md
docs/concepts.md
docs/credential-model.md
docs/demos.md
docs/design-decisions.md
docs/diagrams/agentwrit-logo.png
docs/diagrams/architecture-overview.svg
docs/diagrams/getting-started-auth-flow-sketch.png
docs/diagrams/getting-started-auth-flow-sketch.svg
docs/diagrams/security-topology.svg
docs/diagrams/token-lifecycle.svg
docs/faq.md
docs/foundations.md
docs/getting-started-developer.md
docs/getting-started-operator.md
docs/getting-started-user.md
docs/implementation-map.md
docs/integration-patterns.md
docs/python-sdk.md
docs/roles.md
docs/scenarios.md
docs/scope-model.md
docs/security-topology.md
docs/token-lifecycle.md
docs/troubleshooting.md
go.mod
go.sum
internal/admin/admin_hdl.go
internal/admin/admin_hdl_test.go
internal/admin/admin_svc.go
internal/admin/admin_svc_test.go
internal/app/app_hdl.go
internal/app/app_hdl_test.go
internal/app/app_svc.go
internal/app/app_svc_test.go
internal/audit/audit_log.go
internal/audit/audit_log_test.go
internal/authz/rate_mw.go
internal/authz/rate_mw_test.go
internal/authz/scope.go
internal/authz/scope_test.go
internal/authz/val_mw.go
internal/authz/val_mw_test.go
internal/cfg/cfg.go
internal/cfg/cfg_test.go
internal/cfg/configfile.go
internal/cfg/configfile_test.go
internal/deleg/deleg_svc.go
internal/deleg/deleg_svc_test.go
internal/handler/audit_hdl.go
internal/handler/challenge_hdl.go
internal/handler/deleg_hdl.go
internal/handler/doc.go
internal/handler/handler_test.go
internal/handler/health_hdl.go
internal/handler/logging.go
internal/handler/logging_test.go
internal/handler/metrics_hdl.go
internal/handler/reg_hdl.go
internal/handler/release_hdl.go
internal/handler/release_hdl_test.go
internal/handler/renew_hdl.go
internal/handler/request_id_test.go
internal/handler/revoke_hdl.go
internal/handler/security_hdl.go
internal/handler/security_hdl_test.go
internal/handler/val_hdl.go
internal/identity/id_svc.go
internal/identity/id_svc_test.go
internal/identity/spiffe.go
internal/keystore/keystore.go
internal/keystore/keystore_test.go
internal/mutauth/discovery.go
internal/mutauth/discovery_test.go
internal/mutauth/heartbeat.go
internal/mutauth/heartbeat_test.go
internal/mutauth/mut_auth_hdl.go
internal/mutauth/mut_auth_hdl_test.go
internal/obs/obs.go
internal/problemdetails/problemdetails.go
internal/revoke/rev_svc.go
internal/revoke/rev_svc_test.go
internal/store/jti_test.go
internal/store/sql_store.go
internal/store/sql_store_app_test.go
internal/store/sql_store_revoke_test.go
internal/store/sql_store_test.go
internal/token/revoker.go
internal/token/tkn_claims.go
internal/token/tkn_svc.go
internal/token/tkn_svc_test.go
scripts/gates.sh
scripts/gen_test_certs.sh
scripts/smoke/core-contract.sh
scripts/stack_down.sh
scripts/stack_up.sh
scripts/test-gate-parity.sh
scripts/verify_compose.sh
scripts/verify_dockerfile.sh
tests/LIVE-TEST-TEMPLATE.md
tests/ossf-scorecard-tier1/user-stories.md
tests/sec-l2b/edsign/main.go
tests/sec-l2b/env.sh
tests/sec-l2b/evidence/README.md
tests/sec-l2b/evidence/story-S1-generic-validate-error.md
tests/sec-l2b/evidence/story-S2-generic-revoked-error.md
tests/sec-l2b/evidence/story-S3-renew-tampered-generic.md
tests/sec-l2b/evidence/story-S4-security-headers.md
tests/sec-l2b/evidence/story-S6-oversized-body-413.md
tests/sec-l2b/integration.sh
tests/sec-l2b/user-stories.md
tests/security-bumps/evidence/README.md
tests/security-bumps/evidence/full-testing.log

## Verdict

FAIL — The push was NOT refused. The remote now has the branch s2-private, and MEMORY.md is in it (see the file list above). On a real push the guard let a private file through.

## Diagnosis (run after the verdict, outside the push)

Git hands the hook one line per branch with the four fields separated by SPACES (see the githooks manual, pre-push section). The hook reads that line with IFS set to TAB, so the whole line lands in the first field and the commit fields are empty. The comparison step then fails, its error is swallowed (`|| true`), the list of changed files is empty, and the hook exits 0, which tells git the push is allowed.

To confirm, the reviewer ran the same hook by hand on this commit twice:
- Input separated by spaces (what git really sends): exit 0, push allowed.
- Same input separated by tabs: exit 1, "push rejected" naming MEMORY.md.

The file-checking logic works. Only the line that reads git's input is wrong. Result: the merged hook (PR #92) blocks nothing.
