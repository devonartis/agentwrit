# MEMORY.md

## 2026-02-18

Built P0 audit persistence — SQLite-backed so audit events survive broker restarts. Merged to `develop` (`9290e9d`). Branch `docs/coWork-EnhanceDocs` is active for doc improvements.

User feedback this session:
- "you are just doing a terrible job when it comes to testing and docs for new features" — led to adding 9 missing tests and 9 CHANGELOG entries
- "not just unit tests we need real user tests like someone using it" — must do Docker E2E, not just mocks
- "always show evidence when you run" — terminal output required
- "dont merge i am going to have another team test" — separate review team validates before merge
- Docker stack is currently running on ports 8080/8081 (admin secret: `change-me-in-production`)

## Notes

- CLAUDE.md is checked into the repo while it's private. Remove it before going public.
