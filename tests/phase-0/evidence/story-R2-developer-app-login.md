# P0-R2 — Developer Logs In as the App

Who: The developer.

What: The developer takes the client_id (ct-09ccbf99777a) and client_secret that the operator gave them from R1, and logs in to the broker using curl. The developer doesn't have aactl — they interact with the broker directly over HTTP. When they send their credentials, the broker checks the secret against the stored hash and returns a short-lived token with app-level permissions. This token is what the developer uses for all subsequent API calls.

Why: If app authentication broke during the Phase 0 cleanup, the developer would be locked out. This is a regression check.

How to run: Send a POST to /v1/app/auth with the client_id and client_secret from R1. The broker validates the secret and returns a token.

Expected: HTTP 200 with a JSON response containing access_token (a JWT), expires_in (300 seconds), and token_type "Bearer".

## Test Output

{"access_token":"eyJhbGciOiJFZERTQSIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJhZ2VudGF1dGgiLCJzdWIiOiJhcHA6YXBwLWNsZWFudXAtdGVzdC1jMGU3YjgiLCJleHAiOjE3NzI2MzUyNDUsIm5iZiI6MTc3MjYzNDk0NSwiaWF0IjoxNzcyNjM0OTQ1LCJqdGkiOiJiYjA5NjMwNGFiNTE3NDQwMmY2NmVjMTBmYjk3NDVjYSIsInNjb3BlIjpbImFwcDpsYXVuY2gtdG9rZW5zOioiLCJhcHA6YWdlbnRzOioiLCJhcHA6YXVkaXQ6cmVhZCJdfQ.REa1VCWKrtGa-VX96InwhYpSQBLh2EtOz67P87Eco2oSxEcHiy3CehHkG3y3mFm5vM3EP0AASBF9tR9boGxwDA","expires_in":300,"token_type":"Bearer","scopes":["app:launch-tokens:*","app:agents:*","app:audit:read"]}

HTTP 200

## Verdict

PASS — Developer authenticated successfully. Got a JWT with app-level scopes (app:launch-tokens:*, app:agents:*, app:audit:read), expires in 300 seconds.
