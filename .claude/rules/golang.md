# Go — Rules of Engagement

- `gofmt` is the formatter. No exceptions.
- Error wrapping: `fmt.Errorf("context: %w", err)` — always wrap with context.
- Return errors, don't panic. Reserve `panic` for truly unrecoverable programmer errors.
- Context: pass `ctx context.Context` as first parameter.
- No `init()` functions. Explicit initialization only.
- No global mutable state. Pass dependencies via constructors.
- Interfaces at the consumption site, not the definition site.
- Minimize dependencies. Justify every new `go get`.
- All crypto uses Go stdlib. No third-party crypto.
- Table-driven tests with `t.Run` subtests.
- Test files alongside source: `foo_test.go` next to `foo.go`.
- Build to `./bin/` for anything that runs live. Never `go run` for live tests.

## Code Comments — Non-Negotiable

**Principle: A person or agent can read the code itself to know what it does. Comments exist to tell you what reading the code alone would NOT tell you.**

Never restate what the code does. Instead, comments must explain:
- **Who** calls this and why — which role (Admin, App, Agent), which endpoint, which scope
- **Why** this exists — the business reason, the security property, the design decision
- **Boundaries** — what this code is NOT responsible for, what the caller must ensure
- **History** — if a design choice looks wrong, explain why it's intentional (e.g. "see TD-013")

Bad: `// handleCreateLaunchToken handles launch token creation.`
(Useless — anyone can see that from the function name and body.)

Good:
`// Called by: Apps (POST /v1/app/launch-tokens, scope app:launch-tokens:*) and`
`// Admin (POST /v1/admin/launch-tokens, scope admin:launch-tokens:*).`
`// App callers are constrained by their scope ceiling (ScopeIsSubset check).`
`// Admin callers bypass the ceiling — this is a bootstrapping/dev convenience (see TD-013).`
(This tells you things you cannot learn from reading the function body alone.)

If you have to read three other files to understand who can call a function and why, the comments are insufficient.
