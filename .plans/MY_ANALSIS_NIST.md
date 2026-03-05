                                                                                                                                                                  
⏺ Both agents are back. Now let me compile the comparison. Here's the gap analysis:                                                                                
                                                                                                                                                                   
  ---                                                                                                                                                              
  NIST Claims vs AgentAuth Reality                                                                                                                                 
                                                                                                                                                                   
  IMPLEMENTED — What you claimed and actually have                                                                                                                 
                                                                                                                                                                   
  ┌─────┬─────────────────────────────────────────────────────┬──────────────────────────────────────────────────────┬─────────────────────────────────────────┐   
  │  #  │                       Feature                       │                      NIST Claim                      │            Codebase Evidence            │
  ├─────┼─────────────────────────────────────────────────────┼──────────────────────────────────────────────────────┼─────────────────────────────────────────┤
  │ 1   │ SPIFFE identity format                              │ spiffe://trust-domain/agent/{orch}/{task}/{instance} │ internal/identity/spiffe.go             │
  ├─────┼─────────────────────────────────────────────────────┼──────────────────────────────────────────────────────┼─────────────────────────────────────────┤
  │ 2   │ Ed25519 challenge-response auth                     │ Cryptographic signatures, EdDSA                      │ internal/identity/id_svc.go,            │
  │     │                                                     │                                                      │ internal/token/tkn_svc.go               │
  ├─────┼─────────────────────────────────────────────────────┼──────────────────────────────────────────────────────┼─────────────────────────────────────────┤
  │ 3   │ Task-scoped JWT tokens                              │ Short-lived, task-bound credentials                  │ internal/token/tkn_svc.go               │
  ├─────┼─────────────────────────────────────────────────────┼──────────────────────────────────────────────────────┼─────────────────────────────────────────┤
  │ 4   │ Scope taxonomy action:resource:identifier           │ Granular least-privilege enforcement                 │ internal/authz/scope.go                 │
  ├─────┼─────────────────────────────────────────────────────┼──────────────────────────────────────────────────────┼─────────────────────────────────────────┤
  │ 5   │ Per-request token validation                        │ Zero-trust every-request model                       │ internal/authz/val_mw.go                │
  ├─────┼─────────────────────────────────────────────────────┼──────────────────────────────────────────────────────┼─────────────────────────────────────────┤
  │ 6   │ Scope attenuation in delegation                     │ Permissions narrow, never expand                     │ internal/deleg/deleg_svc.go             │
  ├─────┼─────────────────────────────────────────────────────┼──────────────────────────────────────────────────────┼─────────────────────────────────────────┤
  │ 7   │ Max delegation depth (5 hops)                       │ Practical depth limit                                │ internal/deleg/deleg_svc.go:32          │
  ├─────┼─────────────────────────────────────────────────────┼──────────────────────────────────────────────────────┼─────────────────────────────────────────┤
  │ 8   │ delegation_chain_hash (SHA-256)                     │ Cryptographic chain linking                          │ internal/deleg/deleg_svc.go             │
  ├─────┼─────────────────────────────────────────────────────┼──────────────────────────────────────────────────────┼─────────────────────────────────────────┤
  │ 9   │ Cryptographic signatures per delegation hop         │ Ed25519 signature of agent|scope|timestamp           │ internal/token/tkn_claims.go            │
  ├─────┼─────────────────────────────────────────────────────┼──────────────────────────────────────────────────────┼─────────────────────────────────────────┤
  │ 10  │ Hash-chained audit log                              │ Append-only, tamper-evident                          │ internal/audit/audit_log.go             │
  ├─────┼─────────────────────────────────────────────────────┼──────────────────────────────────────────────────────┼─────────────────────────────────────────┤
  │ 11  │ Audit fields: timestamp, agentID, taskID, orchID,   │ Comprehensive event capture                          │ internal/audit/audit_log.go             │
  │     │ action, resource, outcome, delegDepth, chainHash    │                                                      │                                         │
  ├─────┼─────────────────────────────────────────────────────┼──────────────────────────────────────────────────────┼─────────────────────────────────────────┤
  │ 12  │ SQLite audit persistence                            │ Survives restarts                                    │ internal/store/sql_store.go             │
  ├─────┼─────────────────────────────────────────────────────┼──────────────────────────────────────────────────────┼─────────────────────────────────────────┤
  │ 13  │ 4-level revocation (token/agent/task/chain)         │ Multi-granularity revocation                         │ internal/revoke/rev_svc.go              │
  ├─────┼─────────────────────────────────────────────────────┼──────────────────────────────────────────────────────┼─────────────────────────────────────────┤
  │ 14  │ Token self-revocation (release)                     │ Agent surrenders own credential                      │ internal/handler/release_hdl.go         │
  ├─────┼─────────────────────────────────────────────────────┼──────────────────────────────────────────────────────┼─────────────────────────────────────────┤
  │ 15  │ Short TTLs (5 min default, configurable)            │ Credential lifetime matches task                     │ AA_DEFAULT_TTL, launch token MaxTTL     │
  ├─────┼─────────────────────────────────────────────────────┼──────────────────────────────────────────────────────┼─────────────────────────────────────────┤
  │ 16  │ mTLS support                                        │ Transport-layer identity                             │ cmd/broker/serve.go (optional mode)     │
  ├─────┼─────────────────────────────────────────────────────┼──────────────────────────────────────────────────────┼─────────────────────────────────────────┤
  │ 17  │ Launch token bootstrap                              │ "Secret zero" mechanism                              │ internal/admin/admin_svc.go             │
  ├─────┼─────────────────────────────────────────────────────┼──────────────────────────────────────────────────────┼─────────────────────────────────────────┤
  │ 18  │ PII sanitization in audit logs                      │ Redaction of secrets/passwords                       │ internal/audit/audit_log.go             │
  ├─────┼─────────────────────────────────────────────────────┼──────────────────────────────────────────────────────┼─────────────────────────────────────────┤
  │ 19  │ Rate limiting on auth endpoints                     │ Per-IP token bucket                                  │ internal/authz/rate_mw.go               │
  ├─────┼─────────────────────────────────────────────────────┼──────────────────────────────────────────────────────┼─────────────────────────────────────────┤
  │ 20  │ App registration with scope ceilings                │ App → LaunchToken → Agent traceability               │ internal/app/app_svc.go                 │
  └─────┴─────────────────────────────────────────────────────┴──────────────────────────────────────────────────────┴─────────────────────────────────────────┘

  ---
  GAPS — What you claimed but don't fully have

  ┌─────┬─────────────────────────────────────────────┬────────────┬───────────────────────────────────────────────────────────────────────────────────────────┐
  │  #  │                 NIST Claim                  │   Status   │                                            Gap                                            │
  ├─────┼─────────────────────────────────────────────┼────────────┼───────────────────────────────────────────────────────────────────────────────────────────┤
  │ G1  │ original_principal field in JWT             │ MISSING    │ Inferable from DelegChain[0].Agent but no explicit claim. Human identity binding is       │
  │     │                                             │            │ implicit, not first-class                                                                 │
  ├─────┼─────────────────────────────────────────────┼────────────┼───────────────────────────────────────────────────────────────────────────────────────────┤
  │ G2  │ Ephemeral identity lifecycle                │ MISSING    │ Agent records persist indefinitely — no auto-expiration when task completes               │
  ├─────┼─────────────────────────────────────────────┼────────────┼───────────────────────────────────────────────────────────────────────────────────────────┤
  │ G3  │ Audit chain integrity verification          │ MISSING    │ No VerifyChain() method. Tamper detection only on write, not on read/query                │
  ├─────┼─────────────────────────────────────────────┼────────────┼───────────────────────────────────────────────────────────────────────────────────────────┤
  │ G4  │ Push-based revocation (OCSP/webhook)        │ MISSING    │ Revocation is pull-only (checked per-request against local list). No push notification to │
  │     │                                             │            │  validators                                                                               │
  ├─────┼─────────────────────────────────────────────┼────────────┼───────────────────────────────────────────────────────────────────────────────────────────┤
  │ G5  │ Bidirectional agent-to-agent credential     │ PARTIAL    │ internal/mutauth/ exists as Go API but not HTTP-routed or tested end-to-end               │
  │     │ validation                                  │            │                                                                                           │
  ├─────┼─────────────────────────────────────────────┼────────────┼───────────────────────────────────────────────────────────────────────────────────────────┤
  │ G6  │ Token rotation (old token invalidated on    │ MISSING    │ Old token stays valid until natural expiry after renewal                                  │
  │     │ renewal)                                    │            │                                                                                           │
  ├─────┼─────────────────────────────────────────────┼────────────┼───────────────────────────────────────────────────────────────────────────────────────────┤
  │ G7  │ mTLS as default (not optional)              │ PARTIAL    │ Supported but opt-in via AA_TLS_MODE=mtls, not the default                                │
  ├─────┼─────────────────────────────────────────────┼────────────┼───────────────────────────────────────────────────────────────────────────────────────────┤
  │ G8  │ Revocation propagation within 30 seconds    │ UNVERIFIED │ Local list is checked per-request but no propagation mechanism to external validators     │
  └─────┴─────────────────────────────────────────────┴────────────┴───────────────────────────────────────────────────────────────────────────────────────────┘

  ---
  NOT IMPLEMENTED — Features you recommended but don't have at all

  ┌─────┬─────────────────────────────────────────────────────────────────────────────────────────────────────────┬──────────────────────┬─────────────────┐
  │  #  │                                                 Feature                                                 │     NIST Section     │ Phase Candidate │
  ├─────┼─────────────────────────────────────────────────────────────────────────────────────────────────────────┼──────────────────────┼─────────────────┤
  │ F1  │ Platform attestation (TPM, OIDC workload identity, K8s SA, container image hash)                        │ Identification       │ Future (v2+)    │
  ├─────┼─────────────────────────────────────────────────────────────────────────────────────────────────────────┼──────────────────────┼─────────────────┤
  │ F2  │ MCP CIMD integration (client registration, domain-based trust)                                          │ Identification, Auth │ Future (v2+)    │
  ├─────┼─────────────────────────────────────────────────────────────────────────────────────────────────────────┼──────────────────────┼─────────────────┤
  │ F3  │ Intent classification (comparing declared purpose vs observed tool calls)                               │ Authorization        │ Future (v2+)    │
  ├─────┼─────────────────────────────────────────────────────────────────────────────────────────────────────────┼──────────────────────┼─────────────────┤
  │ F4  │ Behavioral monitoring / anomaly detection (expected tool call patterns as baseline)                     │ Authorization        │ Future (v2+)    │
  ├─────┼─────────────────────────────────────────────────────────────────────────────────────────────────────────┼──────────────────────┼─────────────────┤
  │ F5  │ NGAC / policy engine integration (aggregate access pattern evaluation)                                  │ Authorization        │ Future (v2+)    │
  ├─────┼─────────────────────────────────────────────────────────────────────────────────────────────────────────┼──────────────────────┼─────────────────┤
  │ F6  │ Data aggregation risk detection (sensitivity classification across multiple accesses)                   │ Authorization        │ Future (v2+)    │
  ├─────┼─────────────────────────────────────────────────────────────────────────────────────────────────────────┼──────────────────────┼─────────────────┤
  │ F7  │ Human-in-the-loop as cryptographic chain step (approval embedded in delegation chain, not side-channel) │ Delegation           │ Phase 2+        │
  ├─────┼─────────────────────────────────────────────────────────────────────────────────────────────────────────┼──────────────────────┼─────────────────┤
  │ F8  │ Context grounding (downstream agent actions validated against original task intent)                     │ Delegation           │ Phase 2+        │
  ├─────┼─────────────────────────────────────────────────────────────────────────────────────────────────────────┼──────────────────────┼─────────────────┤
  │ F9  │ JWKS endpoint (public key discovery for external verifiers)                                             │ Authentication       │ Phase 1C or 2   │
  ├─────┼─────────────────────────────────────────────────────────────────────────────────────────────────────────┼──────────────────────┼─────────────────┤
  │ F10 │ HSM-backed signing key (Ed25519 key currently ephemeral in memory)                                      │ Authentication       │ Phase 2+        │
  ├─────┼─────────────────────────────────────────────────────────────────────────────────────────────────────────┼──────────────────────┼─────────────────┤
  │ F11 │ Prompt injection blast radius demo (controlled injection test, ephemeral vs traditional)                │ Prompt Injection     │ Demo/PoC        │
  ├─────┼─────────────────────────────────────────────────────────────────────────────────────────────────────────┼──────────────────────┼─────────────────┤
  │ F12 │ OAuth 2.0 Token Exchange (RFC 8693) compatibility                                                       │ Delegation           │ Future (v2+)    │
  ├─────┼─────────────────────────────────────────────────────────────────────────────────────────────────────────┼──────────────────────┼─────────────────┤
  │ F13 │ Transaction Tokens (draft-ietf-oauth-transaction-tokens) compatibility                                  │ Delegation           │ Future (v2+)    │
  ├─────┼─────────────────────────────────────────────────────────────────────────────────────────────────────────┼──────────────────────┼─────────────────┤
  │ F14 │ WIMSE workload identity integration                                                                     │ Identification       │ Future (v2+)    │
  └─────┴─────────────────────────────────────────────────────────────────────────────────────────────────────────┴──────────────────────┴─────────────────┘

  ---
  Recommended Phase Assignments

  Phase 1C (next up — already scoped for revocation + audit + secret rotation):
  - G1: Add explicit original_principal JWT claim
  - G2: Ephemeral agent identity lifecycle (auto-expire records)
  - G3: VerifyChain() audit integrity method
  - G6: Token rotation (invalidate predecessor on renewal)
  - F9: JWKS endpoint

  Phase 2 (delegation + multi-agent hardening):
  - G5: Wire up mutauth as HTTP endpoints with end-to-end tests
  - G7: mTLS as default mode
  - F7: Human-in-the-loop as delegation chain step
  - F8: Context grounding for delegation validation
  - F10: HSM-backed signing key option

  Future / v2+ (ecosystem integration):
  - F1–F6, F12–F14: Platform attestation, MCP/CIMD, intent classification, behavioral monitoring, NGAC, OAuth Token Exchange, Transaction Tokens, WIMSE