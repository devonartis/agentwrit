# Phase 0 — Legacy Cleanup: User Stories

**Branch:** `fix/phase-0-legacy-cleanup`
**Purpose:** Remove dead sidecar routes, fix the admin login format, and confirm existing features still work.

---

## Personas

| Persona | Tool | Description |
|---------|------|-------------|
| **Operator** | `aactl` | Runs the broker, manages apps and agents |
| **Developer** | `curl` | Builds against the API, has no CLI |
| **Security Reviewer** | `curl` | Checks that removed routes are gone and boundaries hold |

---

## Cleanup Stories

### P0-S1: Sidecar list endpoint is gone

The security reviewer calls the old sidecar list endpoint to confirm it no longer exists on the broker.

**Route:** `GET /v1/admin/sidecars`
**Tool:** `curl`
**Expected:** 404

---

### P0-S2: Sidecar activation creation endpoint is gone

The security reviewer calls the old endpoint that created sidecar activation tokens. It should no longer exist.

**Route:** `POST /v1/admin/sidecar-activations`
**Tool:** `curl`
**Expected:** 404

---

### P0-S3: Sidecar activate endpoint is gone

The security reviewer calls the old endpoint where a sidecar exchanged its activation token for a bearer token. It should no longer exist.

**Route:** `POST /v1/sidecar/activate`
**Tool:** `curl`
**Expected:** 404

---

### P0-S4: Token exchange endpoint is gone

The security reviewer calls the old endpoint where a sidecar exchanged its bearer token for a short-lived agent token. It should no longer exist.

**Route:** `POST /v1/token/exchange`
**Tool:** `curl`
**Expected:** 404

---

### P0-S5: Sidecar ceiling read endpoint is gone

The security reviewer calls the old endpoint that returned a sidecar's scope ceiling. It should no longer exist.

**Route:** `GET /v1/admin/sidecars/{id}/ceiling`
**Tool:** `curl`
**Expected:** 404

---

### P0-S6: Sidecar ceiling update endpoint is gone

The security reviewer calls the old endpoint that updated a sidecar's scope ceiling. It should no longer exist.

**Route:** `PUT /v1/admin/sidecars/{id}/ceiling`
**Tool:** `curl`
**Expected:** 404

---

### P0-S7: Operator logs in with the new admin format

The operator uses `aactl` to log in and list apps. Under the hood, `aactl` sends the admin secret in the new format (`{"secret": "..."}`) instead of the old format. If the app list comes back, the login worked.

**Tool:** `aactl`
**Expected:** App list returned (may be empty on fresh stack)

---

### P0-S8: Old admin login format is rejected with a helpful error

A developer sends the old admin login format (`{"client_id": "admin", "client_secret": "..."}`) directly to the broker. The broker should reject it with a clear message explaining the new format.

**Tool:** `curl`
**Expected:** 400 with message telling the caller to use `{"secret": "..."}`

---

## Regression Stories

### P0-R1: Operator registers a new app

The operator registers a new app called `cleanup-test` using `aactl`. This is the core Phase 1a feature — if it still works after the sidecar removal, nothing is broken.

**Tool:** `aactl`
**Expected:** App created with app_id, client_id, client_secret returned

---

### P0-R2: Developer logs in as the app

The developer takes the client_id and client_secret from P0-R1 and authenticates with the broker using `curl`. The broker returns a scoped JWT that only has app-level permissions.

**Tool:** `curl`
**Expected:** 200 with JWT containing `app:` scopes

---

### P0-R3: App JWT cannot access admin endpoints

The security reviewer takes the app JWT from P0-R2 and tries to access the admin audit trail. The broker should reject it because app tokens don't have admin scopes.

**Tool:** `curl`
**Expected:** 403 Forbidden

---

### P0-R4: Audit trail records all the activity

The operator uses `aactl` to pull the audit trail and checks that the app registration from R1 and the app login from R2 both appear. No client_secret values should appear anywhere in the trail.

**Tool:** `aactl`
**Expected:** `app_registered` and `app_authenticated` events present, no secrets leaked

---

## Test Sequence

Run in this order — later stories use results from earlier ones:

1. P0-S1 through P0-S6 (sidecar routes — independent, run in any order)
2. P0-S7 (operator login — independent)
3. P0-S8 (old format rejected — independent)
4. P0-R1 (register app — needs working admin auth from S7)
5. P0-R2 (developer login — needs credentials from R1)
6. P0-R3 (scope isolation — needs JWT from R2)
7. P0-R4 (audit trail — needs events from R1 + R2)
