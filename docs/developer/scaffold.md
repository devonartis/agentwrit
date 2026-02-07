# Module M00: Project Scaffold

## What exists

- `cmd/broker/main.go`: broker boot with `/v1/health`
- `internal/cfg`: env-based configuration load
- `internal/obs`: structured log format and level filtering
- `internal/store`: SQLite and Redis placeholders
- `scripts/gates.sh`: task/module/milestone/all levels

## Verification

```bash
go test ./...
./scripts/gates.sh task
```

