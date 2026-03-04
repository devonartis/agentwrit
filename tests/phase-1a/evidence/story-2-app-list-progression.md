# Story 2 — App list grows as operator registers more apps

## What we did

Registered three apps one at a time and checked the list after each one to verify the count grows and each app has distinct credentials.

## Commands and output

### After registering weather-bot (1 app)

```
$ ./bin/aactl app list --json

{
  "apps": [
    {
      "app_id": "app-weather-bot-b4065c",
      "name": "weather-bot",
      "client_id": "wb-0753894ae326",
      "scopes": ["read:weather:*", "write:logs:*"],
      "status": "active",
      "created_at": "2026-03-04T03:06:07Z",
      "updated_at": "2026-03-04T03:06:07Z"
    }
  ],
  "total": 1
}
```

### Register log-agent (app 2)

```
$ ./bin/aactl app register --name log-agent --scopes "write:logs:*"

FIELD          VALUE
APP_ID         app-log-agent-21dd57
CLIENT_ID      la-b728a7a04770
CLIENT_SECRET  8aac046f286d1388fedd6a878683234bbd071389cd47cca8be0282b77e04e4c6
SCOPES         write:logs:*

WARNING: Save the client_secret — it cannot be retrieved again.
```

### List after 2 apps — total: 2

```
$ ./bin/aactl app list --json

"total": 2
```

Both apps present with distinct client_id and client_secret values.

### Register alert-service (app 3)

```
$ ./bin/aactl app register --name alert-service --scopes "read:alerts:*,write:logs:*"

FIELD          VALUE
APP_ID         app-alert-service-1c27fc
CLIENT_ID      as-b188a0881d44
CLIENT_SECRET  5766bc3c9be8c5a7cf3c585b4d423a84626ea0b55591deaaebc7ddc6e8bf1c94
SCOPES         read:alerts:*, write:logs:*

WARNING: Save the client_secret — it cannot be retrieved again.
```

### List after 3 apps — total: 3

```
$ ./bin/aactl app list

NAME           APP_ID                    CLIENT_ID        STATUS  SCOPES                       CREATED
alert-service  app-alert-service-1c27fc  as-b188a0881d44  active  read:alerts:*,write:logs:*   2026-03-04T03:07:16Z
log-agent      app-log-agent-21dd57      la-b728a7a04770  active  write:logs:*                 2026-03-04T03:06:33Z
weather-bot    app-weather-bot-b4065c    wb-0753894ae326  active  read:weather:*,write:logs:*  2026-03-04T03:06:07Z
Total: 3
```

## All three apps have distinct credentials

| App | client_id | client_secret (first 16 chars) |
|-----|-----------|-------------------------------|
| weather-bot | wb-0753894ae326 | fb73ece075af892f... |
| log-agent | la-b728a7a04770 | 8aac046f286d1388... |
| alert-service | as-b188a0881d44 | 5766bc3c9be8c5a7... |

## Verdict

PASS — List grows from 0 to 1 to 2 to 3. All apps have distinct client_id and client_secret. All show status `active`. No client_secret_hash in output.
