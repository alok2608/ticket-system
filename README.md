# Ticket System API

A small ticket-system backend in Go. Users register, log in, create tickets, and can view
and update **only their own** tickets.

- **Language:** Go 1.25 + [Gin](https://github.com/gin-gonic/gin)
- **Storage:** SQLite (`modernc.org/sqlite`, pure Go — no cgo)
- **Auth:** JWT (`Authorization: Bearer <token>`), passwords hashed with bcrypt
- **Frontend:** plain HTML/CSS/JS served at `/` by the same binary
- **Port:** 8080

**Deployed URL:** https://ticket-system-phyz.onrender.com
**Health check:** https://ticket-system-phyz.onrender.com/health

```bash
curl https://ticket-system-phyz.onrender.com/health
# {"status":"ok"}
```

> Hosted on Render's free tier, which sleeps after inactivity — the first request after an
> idle period can take up to a minute to wake.

---

## Run locally

### With Docker

```bash
docker build -t ticket-system .
docker run -p 8080:8080 ticket-system
curl http://localhost:8080/health
# {"status":"ok"}
```

No database server and no setup step: the SQLite file is created automatically on first start.

### With Go

```bash
cp .env.example .env     # optional
go run .
```

---

## Environment variables

All optional — the service runs on sensible defaults. A real environment variable wins over
`.env`, which wins over the default. `.env` is git-ignored and kept out of the Docker image,
so on a hosting platform set the variables in its dashboard instead.

| Variable | Default | Purpose |
|---|---|---|
| `JWT_SECRET` | *(random per run)* | Key used to sign JWTs. If unset, a random key is generated at start-up and tokens stop working after a restart — set a real value in production. |
| `DB_PATH` | `tickets.db` (`/app/tickets.db` in Docker) | SQLite file location. |
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

### Register and log in

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

`POST /auth/login` takes the same body and returns the same token fields.
`409` if the email is already registered, `400` if a field is missing, `401` on bad credentials.

### Create a ticket

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

New tickets always start as `open`; `title` is required.

### Read and update

```bash
curl http://localhost:8080/tickets -H "Authorization: Bearer $TOKEN"
curl http://localhost:8080/tickets/1 -H "Authorization: Bearer $TOKEN"

curl -X PATCH http://localhost:8080/tickets/1/status \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"status":"in_progress"}'
```

`GET /tickets` returns a JSON array of the caller's tickets — `[]` when there are none.
A ticket belonging to someone else returns `404`, exactly as a non-existent one does.

---

## Frontend

A small demo UI is served at `/` by the same Go binary — open http://localhost:8080 after
starting the service. It supports register, login, logout, ticket creation, listing your own
tickets, viewing one, and moving its status forward.

```
static/index.html   # structure and semantic markup
static/style.css    # presentation
static/app.js       # behaviour: auth state, API calls, rendering, events
```

No framework, no build step and no npm: three files of plain HTML, CSS and vanilla JavaScript.
They are compiled into the binary with `embed` (see `static.go`), so the deployed service stays
a single self-contained artifact with no runtime file dependency.

Inside `app.js`, `apiRequest()` is the only place `fetch` is configured — it attaches the
`Authorization: Bearer <token>` header and turns an error response into a thrown `Error`.
Functions that fetch never touch the DOM, and functions that render never fetch.

The UI calls the existing API only; it adds no endpoints and changes no request or response
field. Serving it needed three extra routes (`/`, `/style.css`, `/app.js`) and nothing else.

**The JWT is kept in `localStorage`**, which is a deliberate simplification for a demo UI: it
is readable by any script on the page, so a production app would prefer an httpOnly cookie.

---

## Status flow

```
open -> in_progress -> closed
open -> closed
closed -> (final, cannot go back to open or in_progress)
```

Valid statuses are `open`, `in_progress` and `closed`; anything else, a backward move, or any
change out of `closed` returns `400`.

## Status codes

| Code | When |
|---|---|
| 200 | Successful read or update |
| 201 | User or ticket created |
| 400 | Invalid body, missing field, invalid status or transition |
| 401 | Missing, malformed or expired token; bad login credentials |
| 404 | Ticket not found, or not owned by the caller |
| 409 | Email already registered |

Every error response has the shape `{"error":"..."}`.

---

## Project structure

```
main.go         # server struct, router, routes, start-up
static.go       # embeds and serves the frontend
env.go          # .env file loader
db.go           # SQLite connection + schema
models.go       # User / Ticket structs and request bodies
auth.go         # register + login handlers, bcrypt, JWT signing
middleware.go   # Bearer-token auth middleware
tickets.go      # ticket handlers, ownership checks, status flow
static/         # frontend: index.html, style.css, app.js
api_test.go     # handler tests over the real router
tickets_test.go # status-transition tests
Dockerfile      # multi-stage build, static binary
```

One `main` package, one file per concern — no repository or service layers, since the brief
asks for a simple implementation.

Handlers are methods on a small `server` struct holding the database handle and the JWT
secret. Passing dependencies explicitly keeps package-level mutable state out of the program
and lets the tests build a server against a temporary database.

Two pieces carry the rules the brief cares about:

- `findOwnedTicket(id, userID)` is the only way a handler can load a ticket, and it filters on
  `user_id` in the SQL itself — so an ownership check cannot be forgotten at a call site.
- `canTransition(from, to)` is a pure function holding the status rules, independent of Gin
  and the database.

## Tests

```bash
go test ./...
```

Covers the health response, registration and login, bcrypt storage, bearer-token enforcement
on every protected route, ownership isolation between two users, the status flow, and input
validation. The handler tests run against the real router and a throwaway SQLite file.

---

## Deployment

Deployed on Render as a free web service built from this repository's `Dockerfile`. No
database needs provisioning — SQLite lives inside the container.

New → Web Service → connect the repo → runtime **Docker** → instance type **Free**, then set
`JWT_SECRET` under *Environment* (`openssl rand -hex 32`) and set the health check path to
`/health`.

---

## Assumptions

The brief left a few details open. These are the choices made, and why:

1. **Login identifier.** `email` is the primary field; `username` is accepted as an alias, so
   either field name works.
2. **Token field.** Register and login return the JWT under both `token` and `access_token`.
3. **List response.** `GET /tickets` returns a bare JSON array, not a wrapped object.
4. **Another user's ticket returns `404`, not `403`,** so the API does not reveal that a
   ticket with that id exists.
5. **`open -> closed` is allowed.** The only rule the brief states is that a closed ticket can
   never be reopened; moving backwards is rejected, moving forwards is not.
6. **Password rules.** Passwords must be non-empty; no length or complexity rule is enforced,
   as the brief specifies none. Tokens last 24 hours and are signed with HS256.
7. **No default signing key.** If `JWT_SECRET` is unset the service generates a random key and
   logs a warning rather than falling back to a fixed string, since a known key committed to
   the repository would let anyone forge tokens.
8. **Persistence.** SQLite writes to a file inside the container, and the schema is created at
   start-up with `CREATE TABLE IF NOT EXISTS`. On free hosts the container filesystem is
   ephemeral, so data resets on redeploy or restart; pointing `DB_PATH` at a mounted disk
   would fix that without a code change.
