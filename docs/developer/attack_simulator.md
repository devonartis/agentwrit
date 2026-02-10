# Attack Simulator (M13)

## Purpose

The attack simulator implements 5 adversarial scenarios from MVP Requirements Section 2.2 that demonstrate the "gap vs. fix" security story. Each attack runs in both insecure mode (showing the vulnerability) and secure mode (showing how AgentAuth blocks it). The simulator provides the evidence needed for the live demo dashboard.

## Design decisions

**Standalone attack functions over agent subclasses**: Attackers are NOT legitimate agents, so they use raw `httpx.AsyncClient` rather than `AgentBase`. This makes the attack code independent of the agent framework and avoids the assumption that an attacker would follow the registration protocol.

**AttackResult as universal return type**: Every attack returns the same `AttackResult` dataclass with `attempts/successes/blocked/details`. This uniform shape simplifies aggregation in `SimulatorResult` and makes the dashboard rendering straightforward.

**Mode parameter does not change attack behavior**: The attack function always TRIES the same exploit. The difference is in the resource server / broker response. The attack just reports what happened. This ensures the demo honestly shows the system response, not a rigged attack.

**Mock-only unit tests**: All 41 tests use `httpx.MockTransport` via `monkeypatch.setattr`. No live broker or resource server is required. Integration with live services is deferred to M15 (Final Assembly).

## Architecture

```
SimulatorResult
  |
  +-- Attack 1: Credential Theft
  |     stolen API-key/token -> GET /customers/{id} x5
  |     insecure: 5/5 pass | secure: 0/5 (expired/scoped)
  |
  +-- Attack 2: Lateral Movement
  |     read:Customers agent -> GET /orders, PUT /tickets, POST /notifications
  |     insecure: 3/3 pass | secure: 0/3 (scope mismatch)
  |
  +-- Attack 3: Agent Impersonation
  |     rogue with fake token -> GET /customers, GET /orders
  |     insecure: 2/2 pass | secure: 0/2 (token rejected)
  |
  +-- Attack 4: Privilege Escalation
  |     POST /v1/delegate (scope escalation) + direct access
  |     insecure: 1/1 pass | secure: 0/2 (attenuation + scope)
  |
  +-- Attack 5: Accountability
        query GET /v1/audit/events for agent attribution
        insecure: no trail | secure: full SPIFFE ID attribution
```

## File layout

```
demo/
  attacks/
    __init__.py
    __main__.py              -- CLI entrypoint (python -m attacks)
    models.py                -- AttackResult dataclass
    credential_theft.py      -- Attack 1
    lateral_movement.py      -- Attack 2
    impersonation.py         -- Attack 3
    privilege_escalation.py  -- Attack 4
    accountability.py        -- Attack 5
    simulator.py             -- run_all_attacks() + SimulatorResult
    tests/
      __init__.py
      conftest.py            -- sys.path setup
      test_credential_theft.py   -- 8 tests
      test_lateral_movement.py   -- 7 tests
      test_impersonation.py      -- 7 tests
      test_privilege_escalation.py -- 5 tests
      test_accountability.py     -- 6 tests
      test_simulator.py          -- 8 tests (integration + model)
```

## Running

```bash
cd demo

# Run all attack tests (no live services needed)
pytest attacks/tests/ -v

# Run the simulator CLI in insecure mode (resource server must be running)
python -m attacks --mode insecure --resource-url http://localhost:8090

# Run in secure mode (broker on :8080, resource server on :8090)
python -m attacks --mode secure \
  --broker-url http://localhost:8080 \
  --resource-url http://localhost:8090 \
  --stolen-credential "$STOLEN_TOKEN" \
  --admin-token "$SEED_ADMIN_TOKEN"
```

## Attack summary

| # | Attack | Insecure (gap) | Secure (fix) |
|---|--------|---------------|-------------|
| 1 | Credential Theft | Shared key accesses all 5 customers | Token expired or scoped to 1 |
| 2 | Lateral Movement | API key grants full access | 403 scope mismatch on all |
| 3 | Agent Impersonation | Same shared key accepted | Fake token rejected (401) |
| 4 | Privilege Escalation | Full access, no delegation | Delegation denied + direct access denied |
| 5 | No Accountability | Cannot identify agent | Full audit trail with SPIFFE IDs |
