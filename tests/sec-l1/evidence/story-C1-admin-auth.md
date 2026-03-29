# C1 — Admin Authentication Works [ACCEPTANCE]
Who: The operator. What: Admin auth with correct secret. Expected: HTTP 200 + token.
## Test Output
{"access_token":"eyJhbGciOiJFZERTQSIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJhZ2VudGF1dGgiLCJzdWIiOiJhZG1pbiIsImF1ZCI6WyJhZ2VudGF1dGgiXSwiZXhwIjoxNzc0ODExNDcwLCJuYmYiOjE3NzQ4MTExNzAsImlhdCI6MTc3NDgxMTE3MCwianRpIjoiNGE1ZWQ5NzkzYjMyMjZkNjRhZDdmYmRjYjc4MTUxOTEiLCJzY29wZSI6WyJhZG1pbjpsYXVuY2gtdG9rZW5zOioiLCJhZG1pbjpyZXZva2U6KiIsImFkbWluOmF1ZGl0OioiXX0.S7-3i0NFsZunMCGm7wg3b0_rpNEjdzED-yoW7HMuzwxAUf2wSRBJO_UfopRJFtXg2R5CC19_KfKI0tQQ-t20Dg","expires_in":300,"token_type":"Bearer"}

HTTP 200
## Verdict
PASS — Admin auth returned HTTP 200 with access_token.
