# Module M00: Project Scaffold

## What exists

- `cmd/broker/main.go`: broker boot with `/v1/health`
- `internal/cfg`: env-based configuration load
- `internal/obs`: structured log format and level filtering
- `internal/store`: in-memory maps (named SqlStore for planned SQL migration)
- `scripts/gates.sh`: task/module/milestone/all levels

## Design rationale

**In-memory maps over SQLite**: The broker is single-process and stateless across restarts (agent tokens are ephemeral). In-memory maps with a mutex provide the simplest correct implementation. The `SqlStore` name preserves the migration path — when persistence is needed, the struct gains a `*sql.DB` field and methods switch from map lookups to SQL queries.

**Zero dependencies**: `go.mod` has zero `require` entries. All cryptography uses `crypto/ed25519` from the standard library. JWT-style tokens are hand-built rather than importing a JWT library. This keeps the supply chain minimal for a security-critical component.

**Structured logging over stdlib log**: The `obs` package provides level-filtered, module-tagged logging with separate stdout/stderr streams. This enables machine-parseable output while keeping the dependency count at zero.

## Extension points

**Adding a new store backend**: Implement the same method set as `SqlStore` (e.g., `CreateLaunchToken`, `ConsumeLaunchToken`, `SaveAgent`, `PutNonce`, `ConsumeNonce`). Wire the new implementation in `cmd/broker/main.go`.

**Adding new env vars**: Add the field to `cfg.Cfg` struct and read it in `cfg.Load()`. All env vars use the `AA_` prefix.

**Adding a new HTTP endpoint**: Create a handler struct in `internal/handler/` implementing `http.Handler`. Wire it in `cmd/broker/main.go` via `mux.Handle()`.

## Constraints

- Zero-dependency policy: no external Go modules
- Naming conventions: spec abbreviations (`IdSvc`, `TknSvc`, `ValMw`, `RevSvc`)
- Logging: `obs.Ok/Warn/Fail/Trace(module, component, msg, ...ctx)`
- Error format: RFC 7807 `application/problem+json`

## Verification

```bash
go test ./...
./scripts/gates.sh task
```
