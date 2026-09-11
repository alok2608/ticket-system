# Ticket System API

A small ticket-system backend in Go. Users register, log in, create tickets, and can view
and update **only their own** tickets.

- **Language:** Go 1.25 + [Gin](https://github.com/gin-gonic/gin)
- **Storage:** SQLite (`modernc.org/sqlite`, pure Go — no cgo needed)
- **Auth:** JWT (`Authorization: Bearer <token>`), passwords hashed with bcrypt
- **Port:** 8080

**Deployed URL:** `<PASTE DEPLOYED URL HERE>`
**Health check:** `<PASTE DEPLOYED URL HERE>/health`

---

## Run locally

### With Docker (matches the assignment contract)

```bash
docker build -t ticket-system .
docker run -p 8080:8080 ticket-system
curl http://localhost:8080/health
# {"status":"ok"}
```

No database server, no compose file, no setup step: the SQLite file is created automatically
on first start.

### With Go

```bash
cp .env.example .env     # optional - edit the values you want
go mod download
go run .
```

The service reads `./.env` on start-up if the file is present.

The server listens on `:8080` and creates the SQLite file (`tickets.db`) on first start.

---

## Environment variables

All of them are optional — the service runs with sensible defaults so it works with a plain
`docker run -p 8080:8080 ticket-system`. Copy `.env.example` to `.env` to override them.

Values are resolved in this order, first match wins:

1. A real environment variable (`export JWT_SECRET=...`, or `docker run -e JWT_SECRET=...`).
2. The `.env` file.
3. The built-in default.

`.env` is git-ignored and deliberately kept out of the Docker image, so secrets are never
baked into a build. To use it with Docker, mount it or hand it to Docker directly:

```bash
docker run -p 8080:8080 -v "$PWD/.env:/app/.env" ticket-system
docker run -p 8080:8080 --env-file .env ticket-system
```

On a hosting platform such as Render, set the variables in the dashboard instead of shipping
a `.env`.

| Variable | Default | Purpose |
|---|---|---|
| `JWT_SECRET` | *(random per run)* | Key used to sign JWTs. If unset, a random key is generated at start-up and tokens stop working after a restart, so set a real value in production. |
| `DB_PATH` | `tickets.db` (resolves to `/app/tickets.db` in Docker) | SQLite file location. |
| `PORT` | `8080` | HTTP listen port. |

---

## API

All `/tickets` routes require `Authorization: Bearer <token>`.

| Method | Endpoint | Auth | Purpose |
|---|---|---|---|
| GET | `/health` | no | Health check |
| POST | `/auth/register` | no | Register a user |
| POST | `/auth/login` | no | Log in, returns a JWT |
| POST | `/tickets` | yes | Create a ticket |
| GET | `/tickets` | yes | List the logged-in user's tickets |
| GET | `/tickets/{id}` | yes | Get one of your own tickets |
| PATCH | `/tickets/{id}/status` | yes | Update the status of your own ticket |

### POST /auth/register → `201 Created`

```bash
curl -X POST http://localhost:8080/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"email":"alice@example.com","password":"secret123"}'
```

```json
{
  "id": 1,
  "user_id": 1,
  "email": "alice@example.com",
  "created_at": "2026-09-11T15:24:26Z",
  "token": "<jwt>",
  "access_token": "<jwt>"
}
```

`409 Conflict` if the email is already registered, `400` if email or password is missing.

### POST /auth/login → `200 OK`

```bash
curl -X POST http://localhost:8080/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"alice@example.com","password":"secret123"}'
```

```json
{
  "token": "<jwt>",
  "access_token": "<jwt>",
  "token_type": "Bearer",
  "user_id": 1,
  "email": "alice@example.com"
}
```

`401 Unauthorized` on a wrong password or unknown user.

### POST /tickets → `201 Created`

```bash
curl -X POST http://localhost:8080/tickets \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"title":"Printer broken","description":"3rd floor"}'
```

```json
{
  "id": 1,
  "user_id": 1,
  "title": "Printer broken",
  "description": "3rd floor",
  "status": "open",
  "created_at": "2026-09-11T15:24:26Z",
  "updated_at": "2026-09-11T15:24:26Z"
}
```

New tickets always start as `open`. `title` is required (`400` otherwise).

### GET /tickets → `200 OK`

Returns a JSON array of the caller's tickets only — `[]` when there are none.

### GET /tickets/{id} → `200 OK`

Returns the ticket. `404` if it does not exist **or** belongs to someone else.

### PATCH /tickets/{id}/status → `200 OK`

```bash
curl -X PATCH http://localhost:8080/tickets/1/status \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"status":"in_progress"}'
```

Returns the updated ticket.

---

## Status flow

```
open -> in_progress -> closed
open -> closed
closed -> (final, cannot go back to open or in_progress)
```

- Valid statuses: `open`, `in_progress`, `closed`. Anything else → `400`.
- Backward moves (`in_progress -> open`) → `400`.
- Any change out of `closed` → `400` with `{"error":"a closed ticket cannot be reopened"}`.

## Status codes

| Code | When |
|---|---|
| 200 | Successful read or update |
| 201 | User or ticket created |
| 400 | Invalid body, missing field, invalid status or invalid transition |
| 401 | Missing, malformed or expired token; bad login credentials |
| 404 | Ticket not found, or not owned by the caller |
| 409 | Email already registered |

Every error response has the shape `{"error":"..."}`.

---

## Project structure

```
.
├── main.go         # router, routes, server start-up
├── env.go          # .env file loader
├── db.go           # SQLite connection + schema
├── models.go       # User / Ticket structs and request bodies
├── auth.go         # register + login handlers, bcrypt, JWT signing
├── middleware.go   # Bearer-token auth middleware
├── tickets.go      # ticket handlers, ownership checks, status flow
├── Dockerfile      # multi-stage build, static binary
├── .env.example
└── README.md
```

One `main` package, one file per concern — no repository/service layers, since the brief asks
for a simple implementation.

---

## Deployment

The service is deployed from this repository's `Dockerfile` on a free-tier host. No database
service needs to be provisioned — SQLite lives inside the container.

Steps used (Render free web service):

1. Push this repo to GitHub.
2. On [Render](https://render.com) → **New → Web Service** → connect the repo.
3. Runtime **Docker** (the `Dockerfile` is detected automatically); instance type **Free**.
4. Under *Environment*, set `JWT_SECRET` to a long random value (`openssl rand -hex 32`).
5. Deploy, then verify `https://<your-app>.onrender.com/health` returns `{"status":"ok"}`.

Fly.io, Koyeb or Railway work the same way — they all build the same `Dockerfile`.

Free web services sleep after inactivity, so the first request after a sleep can take ~50
seconds to wake.

---

## Assumptions

The brief left a few details open. These are the choices made, and why:

1. **Login identifier.** `email` is the primary field; `username` is accepted as an alias in
   the same body, so either field name works.
2. **Token field.** The login/register response returns the JWT under both `token` and
   `access_token`.
3. **List response.** `GET /tickets` returns a bare JSON array (`[]` when empty), not a
   wrapped object.
4. **Another user's ticket returns `404`, not `403`,** so the API does not reveal that a
   ticket with that id exists.
5. **`open -> closed` is allowed.** The only rule the brief states explicitly is that a closed
   ticket can never be reopened; moving backwards is rejected, moving forwards is not.
6. **Invalid transitions return `400`** (validation error) rather than `409`.
7. **Password rules.** Passwords must be non-empty; no length or complexity rule is enforced,
   as the brief specifies none.
8. **Token lifetime** is 24 hours, signed with HS256.
9. **No default signing key.** If `JWT_SECRET` is unset the service generates a random key
   and logs a warning, rather than falling back to a fixed string. A known key committed to
   the repository would let anyone forge tokens against a deployment that forgot to set it.
10. **Timestamps** are RFC3339 UTC strings.
11. **`.env` loading** is a ~30-line loader in `env.go` rather than a third-party dependency,
   since the format needed here is only `KEY=VALUE`.
12. **Persistence.** SQLite writes to a file inside the container. On free hosts the container
    filesystem is ephemeral, so data resets on redeploy or restart. That is acceptable for this
    assignment, which does not ask for durable storage; pointing `DB_PATH` at a mounted disk
    would fix it without any code change.
13. **Schema creation** happens automatically at start-up with `CREATE TABLE IF NOT EXISTS`,
    so there is no migration step.
14. **Timestamps are stored as TEXT** in RFC3339, so the JSON the API returns is exactly what
    was written.
