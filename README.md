# EA QMS — Change Control API

A Go backend for the Change Control module of a Quality Management System. Change records move through a six-state workflow with two approval gates, every decision is captured by an electronic signature, and every state change is written to an immutable audit trail.

- **API documentation:** <https://lain-the-coder.github.io/ea-qms-backend/>
- **Specification and UI prototypes:** [Change-Control-HTML-Design](https://github.com/lain-the-coder/Change-Control-HTML-Design) · [live prototypes](https://lain-the-coder.github.io/Change-Control-HTML-Design/)
- **Docker Hub:** [20dumpling/ea-qms-backend](https://hub.docker.com/r/20dumpling/ea-qms-backend)

---

## What it does

A change control is raised, filled in, submitted for implementation approval,
implemented with evidence attached, submitted for final approval, and closed —
or rejected at either gate and sent back for rework, or cancelled while still a
draft.

```

                    ┌──────────────── T5 reject ───────────────┐
                    │                                          │
                    ▼                                          │
  (T1) ──▶ Initiated ──── T2 ────▶ Pending Impl Approval ─────┘
              │                              │
              │ T3                           │ T4 approve
              ▼                              │
          Cancelled                          │       ┌─ T8 reject ─┐
                                             ▼       ▼             │
                                        In Implementation          │
                                             │                     │
                                             │ T6                  │
                                             ▼                     │
                                   Pending Final Approval ─────────┘
                                             │
                                             │ T7 approve
                                             ▼
                                           Closed

```

| #   | From                            | To                              | Action                    | Actor    | E-signature |
| --- | ------------------------------- | ------------------------------- | ------------------------- | -------- | ----------- |
| T1  | —                               | Initiated                       | Create                    | CC Owner | No          |
| T2  | Initiated                       | Pending Implementation Approval | Submit for Approval       | CC Owner | Yes         |
| T3  | Initiated                       | Cancelled                       | Cancel                    | CC Owner | Yes         |
| T4  | Pending Implementation Approval | In Implementation               | Approve                   | Approver | Yes         |
| T5  | Pending Implementation Approval | Initiated                       | Reject                    | Approver | Yes         |
| T6  | In Implementation               | Pending Final Approval          | Submit for Final Approval | CC Owner | Yes         |
| T7  | Pending Final Approval          | Closed                          | Approve                   | Approver | Yes         |
| T8  | Pending Final Approval          | In Implementation               | Reject                    | Approver | Yes         |

Four roles — Admin, CC Owner, Approver, Viewer — with permissions that depend on
both the role and the record's current state. A CC Owner can edit twenty-four
fields while a record is `Initiated` and none of them afterwards; an Approver can
act only on records assigned to them, and only at the gate they are assigned to.

Transitions T2 through T8 require a native electronic signature: the acting user
re-enters their email and password, and the system records what they attested to.
T1 — creating the record — does not.

## Stack

|                |                                                                               |
| -------------- | ----------------------------------------------------------------------------- |
| **Language**   | Go 1.22+                                                                      |
| **HTTP**       | `net/http` + `ServeMux` — no framework                                        |
| **Database**   | PostgreSQL 14, `lib/pq`                                                       |
| **Queries**    | [sqlc](https://sqlc.dev) — SQL is written by hand and Go is generated from it |
| **Migrations** | [goose](https://github.com/pressly/goose)                                     |
| **Auth**       | argon2id password hashing, JWT access tokens, opaque refresh tokens           |
| **Logging**    | `log/slog`, structured JSON, one request ID per request                       |
| **Deployment** | Docker multi-stage Alpine image (~24MB), Docker Compose                       |

No ORM, no service layer, no repository layer. A handler talks to sqlc, which
talks to Postgres.

## Running it

**Prerequisites:** Go 1.22+, PostgreSQL 14+, [goose](https://github.com/pressly/goose#installation),
[sqlc](https://docs.sqlc.dev/en/latest/overview/install.html) (only if you change a query).

```bash
git clone https://github.com/lain-the-coder/ea-qms-backend.git
cd ea-qms-backend
go mod download
```

**Create the database:**

```bash
createdb ea_qms
```

**Configure** — copy `.env.example` to `.env` and fill it in:

```
DB_URL=postgres://postgres:password@localhost:5432/ea_qms?sslmode=disable
JWT_SECRET=<a long random string>
PLATFORM=dev
ALLOWED_ORIGINS=http://localhost:5173
DB_MAX_OPEN_CONNS=25
```

Generate a secret with `openssl rand -base64 64`.

**Do not quote values.** `godotenv` strips surrounding quotes, but `docker run --env-file`
and Compose's `env_file` do not — a quoted `JWT_SECRET` would carry literal quote
characters into the container, and a token signed on bare metal would then fail to
validate there. Unquoted is the only form that works in all three.

`DB_MAX_OPEN_CONNS` is the connection-pool ceiling **per process**, defaulting to 10 if
unset. Running N instances against one database means N × this value, which must stay
under Postgres's `max_connections`.

`INSTANCE_ID` is optional and unnecessary here — it identifies which replica served a
request, and falls back to the hostname when unset. It matters only when the API runs as
more than one instance, and is set per container rather than in this file, since
`docker run --name` does not set a container's hostname.

**Run the migrations:**

```bash
cd sql/schema
goose postgres "$DB_URL" up
cd ../..
```

**Seed four test users and some change controls** (development only):

```bash
go run ./cmd/seed
```

**Start it:**

```bash
go run .          # listens on :1304
```

Then open <http://localhost:1304/docs> for the interactive API reference, where
**Try it out** works because the documentation is served from the same origin as
the API.

### Test users

All four share the password printed by the seed command.

| Email                  | Role     |
| ---------------------- | -------- |
| `admin@eaqms.local`    | Admin    |
| `owner@eaqms.local`    | CC Owner |
| `approver@eaqms.local` | Approver |
| `viewer@eaqms.local`   | Viewer   |

### Building a binary

```bash
go build -o ea-qms-backend .
./ea-qms-backend
```

The API documentation is compiled into the binary with `go:embed`, so it deploys
with the code it describes. The binary still needs a reachable database and a
`.env` in the working directory.

### Docker

Pull and run the pre-built image directly from Docker Hub:

```bash
docker pull 20dumpling/ea-qms-backend:latest
```

Build locally:

```bash
docker build -t 20dumpling/ea-qms-backend:latest .
```

The multi-stage build compiles a static Linux binary (CGO_ENABLED=0) stripped of DWARF symbols (-ldflags="-w -s"), packing the runtime onto alpine:latest for a minimal 24 MB image footprint.

A dedicated health check (GET /api/healthz) performs an active rawDB.PingContext() check for orchestrator readiness and liveness verification without polluting route logs.

### Docker Compose (Full Stack)

If you want to spin up the entire backend stack (API, PostgreSQL, schema migrations, and seeded test accounts) using Docker without configuring a local Go or PostgreSQL toolchain:

1. Navigate to the Docker deployment directory:

```bash
   cd deploy/docker
```

2. Follow the setup instructions in [`deploy/docker/README.md`](deploy/docker/README.md) to start the containers via `docker compose up -d`.

## The API

23 endpoints across seven groups:

| Group               |                                                                          |
| ------------------- | ------------------------------------------------------------------------ |
| **Auth**            | login, refresh, revoke                                                   |
| **Users**           | current user, create, list, update, activate/deactivate, approver lookup |
| **Change Controls** | create, read, list, save draft, save implementation details              |
| **Workflow**        | the five transition endpoints covering T2–T8                             |
| **Files**           | evidence upload and download                                             |
| **Dashboard**       | state counts, action items, recent activity — in one call                |
| **Signatures**      | the e-signature history for a record                                     |

Every shape, enum value and status code is in
[`docs/openapi.yaml`](docs/openapi.yaml), which is hand-written from the handler
code rather than generated from example traffic. A Postman collection is in
[`postman/`](postman/).

## Design notes

A few decisions that shaped the codebase. The reasoning for all of them, and for
forty-odd others, is in [`GO_Coding_Guide.md`](GO_Coding_Guide.md).

**The database enforces what it can.** Enum values are CHECK constraints, record
IDs come from a sequence and a generated column, and uniqueness is decided by a
unique index rather than a `SELECT` before an `INSERT`. Go validates the same
things — but for the error message, not for the guarantee.

**Authentication is a compile-time property.** Authenticated handlers take a
third parameter:

```go
type authedHandler func(http.ResponseWriter, *http.Request, database.User)
```

A three-parameter function is not a valid `http.HandlerFunc`, so the only way to
route one is through the auth middleware that supplies the user. Forgetting
authentication is a build error rather than a runtime hole.

**Writes and their audit rows are atomic.** A change with no audit trail is, in a
regulated context, a change that did not happen — so it must not be possible. The
one deliberate exception is a _failed_ e-signature, which is written outside the
transaction so that the record of the attempt survives the rollback.

**Validation is split by what can be known when.** Saving a draft validates
format — lengths, enum membership, JSON types. Submitting validates presence and
the business-day date rules, because a draft is incomplete by nature and a date
valid on Monday may be invalid by Thursday. Failures are collected and returned
together rather than one at a time.

**Explicit over clever.** Eight transitions are eight handlers, not a transition
engine. Twenty-four editable fields are twenty-four blocks, not a table of
closures. This costs length — the save-draft handler is around 700 lines — and
buys the ability to `grep` for a field name and land on the code that handles it.
The trade is deliberate, and the guide argues it properly.

**Bounds are per-process; the resources they consume are not.** `sql.Open` defaults
to an unlimited pool, which is survivable for one process and wrong for several —
each instance holds its own pool and cannot see its peers, while Postgres enforces
one global `max_connections`. The pool is bounded and its ceiling read from the
environment, so contention produces queueing inside the process rather than
`too many clients` at the database: bounded resources degrade, unbounded ones
collapse. The same reasoning sets the server's timeouts, and explains why
`ReadTimeout` is deliberately left unset — it is a property of the server, which
has not yet matched a route, so a value strict enough to be useful against a slow
client would also truncate a legitimate evidence upload. Body size is bounded in
the handler, which does know which route it is.

## Project layout

```
.
├── main.go, config.go, middleware.go       # wiring, shared config, cross-cutting
├── handlers_*.go                           # one file per endpoint group
├── helpers.go                              # small pure functions, with tests
├── internal/
│   ├── auth/                               # argon2id, JWT, refresh tokens
│   ├── database/                           # generated by sqlc — never edited
│   └── logging/                            # slog setup, request-scoped logger
├── sql/
│   ├── schema/                             # goose migrations
│   └── queries/                            # hand-written SQL, input to sqlc
├── docs/                                   # OpenAPI spec + Swagger UI
├── postman/                                # request collection
└── cmd/seed/                               # development data
```

## Documentation

|                                                  |                                                                                                               |
| ------------------------------------------------ | ------------------------------------------------------------------------------------------------------------- |
| [`GO_Coding_Guide.md`](GO_Coding_Guide.md)       | 19 sections, 124 rules, each with code and the trade it makes                                                 |
| [`docs/openapi.yaml`](docs/openapi.yaml)         | The API contract                                                                                              |
| [`FRONTEND_BLUEPRINT.md`](FRONTEND_BLUEPRINT.md) | The API contract from a client's perspective, plus the frontend plan                                          |
| `PROGRESS.md`                                    | Every decision made during the build, with its reasoning — including the ones that reversed earlier decisions |

**In [Change-Control-HTML-Design](https://github.com/lain-the-coder/Change-Control-HTML-Design)** — the specification this was built against:

|                                         |                                                                        |
| --------------------------------------- | ---------------------------------------------------------------------- |
| Business Requirements Document          | 50 fields, 25 user stories, the workflow, the e-signature ruleset      |
| Security Matrix                         | 50 fields × states × 4 roles — which are editable, read-only or hidden |
| Database Design                         | The PostgreSQL schema, constraints, indexes and concurrency handling   |
| CC Field Reference                      | Per-field validation, and the canonical string values used here        |
| State machine, sequence and ER diagrams |                                                                        |
| 17 HTML prototypes + `global.css`       | Every screen, every state, every role                                  |

Those documents were written before any code and amended at completion so that
they describe what was built rather than what was intended — eleven amendments
across four documents, each recorded with its reason.

## Testing

Pure logic — business-day arithmetic, filename sanitising, password hashing, JWT
handling — is covered by unit tests:

```bash
go test ./...
```

The endpoints themselves were verified by hand against a real database, with the
results recorded per endpoint. There is no HTTP-level test harness; building one
is the most obvious next piece of work, and the absence is worth stating plainly
rather than implying coverage that does not exist.

## Status and scope

All 23 endpoints are built and verified. The frontend has not started.

Deliberately out of scope for this release, and recorded as such in the
requirements: password reset and change, self-service profile editing, email
notifications, saved searches and reporting, and reassigning a change control to
a different approver.

Known deferred work: log rotation, a configurable business timezone (date rules
currently compute in UTC), a cleanup job for expired refresh tokens, and
extracting the signature-verification block that is currently repeated across
five transition handlers.

Also deferred, from the resource-bounds work: mapping `context.DeadlineExceeded`
to 504 and reclassifying `context.Canceled` as a client disconnect rather than a
server error — one edit repeated at every database call site, so a timed-out
request currently returns 500 and is identifiable only by its duration; a
`Flusher` / `Hijacker` passthrough on the response recorder, which nothing needs
until the first streaming or WebSocket endpoint; and writing logs to stdout rather
than a file, which is the correct destination once the process runs in a container
and its filesystem is discarded with it.

## Context

Built as a solo project against a full pre-development specification — business
requirements, a security matrix defining field permissions per role per state, a
database design, and HTML prototypes for all seventeen screens. That
specification lives in
[Change-Control-HTML-Design](https://github.com/lain-the-coder/Change-Control-HTML-Design)
and came first; where the implementation departed from it, the specification was
amended rather than the deviation left undocumented.
