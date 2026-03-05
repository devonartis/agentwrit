# P1B-S8 — Agent Traceability to Originating App

Who: The security reviewer.

What: The security reviewer verifies that every agent registered through an app-created
launch token carries the app_id of the originating app in the audit trail. The reviewer
also confirms that agents registered through admin-created launch tokens do NOT carry
an app_id. This is the full traceability chain: app authenticates → creates launch
token (with app_id) → agent registers (with app_id) → all visible in audit.

Why: If an app is compromised, the security team needs to identify every agent that
app created. Without app_id on the agent's audit events, there is no way to trace
the compromise path. This traceability is a core security requirement — it enables
incident response and forensic investigation.

How to run: Source the environment file. Pull the full audit trail and inspect the
traceability chain: app_authenticated → launch_token_issued (with app_id) →
agent_registered (with app_id) → token_issued (with app_id). Compare against the
admin-created agent which should have none of these app_id markers.

Expected: App-created agents show app_id on agent_registered and token_issued events.
Admin-created agents show no app_id on those same event types. The full chain from
app auth to agent registration is traceable.

## Test Output — Full Audit Trail

ID          TIMESTAMP                       EVENT TYPE              AGENT ID                                                                 OUTCOME  DETAIL
evt-000001  2026-03-04T14:34:11.469587841Z  admin_auth                                                                                       success  admin authenticated as admin
evt-000002  2026-03-04T14:35:15.451494926Z  admin_auth                                                                                       success  admin authenticated as admin
evt-000003  2026-03-04T14:35:15.721592801Z  app_registered                                                                                   success  app=cleanup-test client_id=ct-09ccbf99777a scopes=[read:d...
evt-000004  2026-03-04T14:35:45.641544759Z  app_authenticated                                                                                success  client_id=ct-09ccbf99777a app_id=app-cleanup-test-c0e7b8
evt-000005  2026-03-04T14:36:08.137592047Z  scope_violation         app:app-cleanup-test-c0e7b8                                              denied   scope_violation | required=admin:audit:* | actual=app:lau...
evt-000006  2026-03-04T14:36:26.78621875Z   admin_auth                                                                                       success  admin authenticated as admin
evt-000007  2026-03-05T02:25:57.71390488Z   admin_auth                                                                                       success  admin authenticated as admin
evt-000008  2026-03-05T02:25:57.985480838Z  app_registered                                                                                   success  app=weather-bot client_id=wb-250e2e5c0fe7 scopes=[read:we...
evt-000009  2026-03-05T02:26:31.572700507Z  app_authenticated                                                                                success  client_id=wb-250e2e5c0fe7 app_id=app-weather-bot-fffad0
evt-000010  2026-03-05T02:26:43.713494013Z  launch_token_issued                                                                              success  launch token issued for agent=fetcher scope=[read:weather...
evt-000011  2026-03-05T02:27:10.883712011Z  app_authenticated                                                                                success  client_id=wb-250e2e5c0fe7 app_id=app-weather-bot-fffad0
evt-000012  2026-03-05T02:27:10.938496178Z  scope_ceiling_exceeded  app:app-weather-bot-fffad0                                               denied   app=app-weather-bot-fffad0 requested=[write:data:all] cei...
evt-000013  2026-03-05T02:27:10.950968553Z  launch_token_issued                                                                              success  launch token issued for agent=good-fetcher scope=[read:we...
evt-000014  2026-03-05T02:27:39.595887052Z  admin_auth                                                                                       success  admin authenticated as admin
evt-000015  2026-03-05T02:27:39.609038802Z  admin_auth                                                                                       success  admin authenticated as admin
evt-000016  2026-03-05T02:28:24.987785754Z  admin_auth                                                                                       success  admin authenticated as admin
evt-000017  2026-03-05T02:28:25.00016267Z   admin_auth                                                                                       success  admin authenticated as admin
evt-000018  2026-03-05T02:34:52.468747377Z  app_authenticated                                                                                success  client_id=wb-250e2e5c0fe7 app_id=app-weather-bot-fffad0
evt-000019  2026-03-05T02:34:52.473861711Z  launch_token_issued                                                                              success  launch token issued for agent=fetcher scope=[read:weather...
evt-000020  2026-03-05T02:34:52.532675211Z  admin_auth                                                                                       success  admin authenticated as admin
evt-000021  2026-03-05T02:35:30.136996839Z  app_authenticated                                                                                success  client_id=wb-250e2e5c0fe7 app_id=app-weather-bot-fffad0
evt-000022  2026-03-05T02:35:30.143092173Z  launch_token_issued                                                                              success  launch token issued for agent=fetcher scope=[read:weather...
evt-000023  2026-03-05T02:35:30.165249381Z  agent_registered        spiffe://agentauth.local/agent/test-orch-1/test-task-1/a6154416937a6ade  success  Agent registered with scope [read:weather:current] app_id...
evt-000024  2026-03-05T02:35:30.167740464Z  token_issued            spiffe://agentauth.local/agent/test-orch-1/test-task-1/a6154416937a6ade  success  Token issued, jti=6e9062b60c4c1157255615a10ddb0955, ttl=3...
evt-000025  2026-03-05T02:35:30.188816173Z  admin_auth                                                                                       success  admin authenticated as admin
evt-000026  2026-03-05T02:35:30.210453464Z  admin_auth                                                                                       success  admin authenticated as admin
evt-000027  2026-03-05T02:35:42.081958303Z  admin_auth                                                                                       success  admin authenticated as admin
evt-000028  2026-03-05T02:35:50.964249377Z  admin_auth                                                                                       success  admin authenticated as admin
evt-000029  2026-03-05T02:36:02.424964924Z  admin_auth                                                                                       success  admin authenticated as admin
evt-000030  2026-03-05T02:36:32.717413799Z  admin_auth                                                                                       success  admin authenticated as admin
evt-000031  2026-03-05T02:36:32.971317841Z  admin_auth                                                                                       success  admin authenticated as admin
evt-000032  2026-03-05T02:36:57.323998755Z  admin_auth                                                                                       success  admin authenticated as admin
evt-000033  2026-03-05T02:36:57.446324546Z  launch_token_issued                                                                              success  launch token issued for agent=admin-agent scope=[admin:*]...
evt-000034  2026-03-05T02:36:57.562237922Z  admin_auth                                                                                       success  admin authenticated as admin
evt-000035  2026-03-05T02:37:17.325733833Z  admin_auth                                                                                       success  admin authenticated as admin
evt-000036  2026-03-05T02:37:50.850497293Z  app_authenticated                                                                                success  client_id=wb-250e2e5c0fe7 app_id=app-weather-bot-fffad0
evt-000037  2026-03-05T02:37:50.856156918Z  launch_token_issued                                                                              success  launch token issued for agent=test scope=[read:weather:cu...
evt-000038  2026-03-05T02:37:50.859745543Z  scope_ceiling_exceeded  app:app-weather-bot-fffad0                                               denied   app=app-weather-bot-fffad0 requested=[write:data:all] cei...
evt-000039  2026-03-05T02:37:50.86292171Z   scope_ceiling_exceeded  app:app-weather-bot-fffad0                                               denied   app=app-weather-bot-fffad0 requested=[read:weather:* writ...
evt-000040  2026-03-05T02:37:50.886732668Z  admin_auth                                                                                       success  admin authenticated as admin
evt-000041  2026-03-05T02:39:54.525628295Z  app_authenticated                                                                                success  client_id=wb-250e2e5c0fe7 app_id=app-weather-bot-fffad0
evt-000042  2026-03-05T02:39:54.556305212Z  launch_token_issued                                                                              success  launch token issued for agent=test scope=[read:weather:cu...
evt-000043  2026-03-05T02:39:54.575080003Z  scope_ceiling_exceeded  app:app-weather-bot-fffad0                                               denied   app=app-weather-bot-fffad0 requested=[write:data:all] cei...
evt-000044  2026-03-05T02:39:54.592886378Z  scope_ceiling_exceeded  app:app-weather-bot-fffad0                                               denied   app=app-weather-bot-fffad0 requested=[read:weather:* writ...
evt-000045  2026-03-05T02:39:54.621299087Z  admin_auth                                                                                       success  admin authenticated as admin
evt-000046  2026-03-05T02:40:59.759626131Z  app_authenticated                                                                                success  client_id=wb-250e2e5c0fe7 app_id=app-weather-bot-fffad0
evt-000047  2026-03-05T02:40:59.786796214Z  launch_token_issued                                                                              success  launch token issued for agent=test scope=[read:weather:cu...
evt-000048  2026-03-05T02:40:59.803351256Z  scope_ceiling_exceeded  app:app-weather-bot-fffad0                                               denied   app=app-weather-bot-fffad0 requested=[write:data:all] cei...
evt-000049  2026-03-05T02:40:59.817127381Z  scope_ceiling_exceeded  app:app-weather-bot-fffad0                                               denied   app=app-weather-bot-fffad0 requested=[read:weather:* writ...
evt-000050  2026-03-05T02:40:59.838999714Z  admin_auth                                                                                       success  admin authenticated as admin
evt-000051  2026-03-05T02:42:19.812318834Z  admin_auth                                                                                       success  admin authenticated as admin
evt-000052  2026-03-05T02:42:19.957828918Z  launch_token_issued                                                                              success  launch token issued for agent=admin-provisioned scope=[ad...
evt-000053  2026-03-05T02:42:19.979001293Z  launch_token_issued                                                                              success  launch token issued for agent=admin-provisioned-2 scope=[...
evt-000054  2026-03-05T02:42:20.307276251Z  agent_registered        spiffe://agentauth.local/agent/admin-orch/admin-task-1/5eec57c0e1fe9b0c  success  Agent registered with scope [admin:reports:read]
evt-000055  2026-03-05T02:42:20.311530751Z  token_issued            spiffe://agentauth.local/agent/admin-orch/admin-task-1/5eec57c0e1fe9b0c  success  Token issued, jti=12fe39cc78e4b918d0f62f8a25e3ad4f, ttl=300
evt-000056  2026-03-05T02:42:20.341891876Z  admin_auth                                                                                       success  admin authenticated as admin
evt-000057  2026-03-05T02:42:59.871020256Z  app_authenticated                                                                                success  client_id=wb-250e2e5c0fe7 app_id=app-weather-bot-fffad0
evt-000058  2026-03-05T02:42:59.896056506Z  scope_ceiling_exceeded  app:app-weather-bot-fffad0                                               denied   app=app-weather-bot-fffad0 requested=[write:weather:*] ce...
evt-000059  2026-03-05T02:42:59.913195173Z  launch_token_issued                                                                              success  launch token issued for agent=test scope=[read:weather:cu...
evt-000060  2026-03-05T02:42:59.930182381Z  launch_token_issued                                                                              success  launch token issued for agent=test scope=[read:weather:*]...
evt-000061  2026-03-05T02:42:59.953264048Z  admin_auth                                                                                       success  admin authenticated as admin
evt-000062  2026-03-05T02:43:00.256688964Z  app_registered                                                                                   success  app=narrow-bot client_id=nb-296b127a7ed1 scopes=[read:wea...
evt-000063  2026-03-05T02:43:00.267230256Z  admin_auth                                                                                       success  admin authenticated as admin
evt-000064  2026-03-05T02:43:00.533253465Z  app_registered                                                                                   success  app=narrow-bot-2 client_id=nb2-8e6c9347b85d scopes=[read:...
evt-000065  2026-03-05T02:43:01.061967132Z  app_authenticated                                                                                success  client_id=nb2-8e6c9347b85d app_id=app-narrow-bot-2-357caa
evt-000066  2026-03-05T02:43:01.081580923Z  scope_ceiling_exceeded  app:app-narrow-bot-2-357caa                                              denied   app=app-narrow-bot-2-357caa requested=[read:weather:*] ce...
evt-000067  2026-03-05T02:43:26.910529421Z  admin_auth                                                                                       success  admin authenticated as admin
Showing 67 of 67 events (offset=0, limit=100)


## Verdict

PASS — The full traceability chain is visible in the audit trail.

App-created agent chain (weather-bot):
- evt-000021: app_authenticated — client_id=wb-250e2e5c0fe7 app_id=app-weather-bot-fffad0
- evt-000022: launch_token_issued — created_by=app:app-weather-bot-fffad0 app_id=app-weather-bot-fffad0
- evt-000023: agent_registered — app_id=app-weather-bot-fffad0
- evt-000024: token_issued — app_id=app-weather-bot-fffad0

Admin-created agent chain (no app):
- evt-000053: launch_token_issued — created_by=admin (no app_id)
- evt-000054: agent_registered — no app_id
- evt-000055: token_issued — no app_id

The distinction is clear: app-created agents carry app_id on every event in the
chain. Admin-created agents have no app_id. A security team investigating a
compromised app can filter by app_id and find every agent that app ever created.
