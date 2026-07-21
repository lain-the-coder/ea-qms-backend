# PROGRESS — EA QMS Change Control Backend (Go)

**Scope of this file:** what is built, what is next, decisions made in working sessions
that are not recorded in any guardrail document, and open flags. Nothing else — the six
guardrail docs carry the substance and are always attached.

- **Repo:** `github.com/lain-the-coder/ea-qms-backend`
- **Last checkpoint:** 8b — structured logging (slog + context) · **all plumbing complete**
- **Next task:** checkpoint 9 — `POST /api/login` (first real endpoint)
- **Schema version:** 6 · all six tables built and verified
- **Review loop:** paste code in chat for review _before_ committing — review precedes
  commit, never follows it. (The repo is public and can be cloned if ever useful to look
  at already-committed code, but that is not the default workflow.)

---

## Phase status

| Phase                             | State                                             |
| --------------------------------- | ------------------------------------------------- |
| Migrations (001–006)              | ✅ Complete — all six tables applied and verified |
| sqlc setup                        | ✅ Complete — pointer types working under lib/pq  |
| `internal/auth` (argon2id)        | ✅ Complete — hashing + tests + app wiring        |
| `cmd/seed`                        | ✅ Complete — 4 users seeded and verified         |
| Structured logging (slog+context) | ✅ Complete — request IDs proven end to end       |
| API implementation (22 endpoints) | ⬜ **Next** — starting `POST /api/login`          |

---

## Completed

### ✅ Checkpoint 1 — Scaffold + users migration

**Repo**

- `go mod init github.com/lain-the-coder/ea-qms-backend`
- `sql/schema/` and `sql/queries/` created
- `.gitignore` contains `.env`; `.env` filled; `.env.example` committed with empty values
- Keys in both: `DB_URL`, `PLATFORM`, `JWT_SECRET`
- Local database `ea_qms` created via psql
- Placeholder `main.go` — `ServeMux`, `WelcomeHome` handler, port `:1304`

**`sql/schema/001_users.sql`** — applied, up → `\d` → down → `\dt` → up clean.

| Check (DB §3.1 / §5.1 / §6.1)                                  | Result |
| -------------------------------------------------------------- | ------ |
| 8 columns, §3.1 order and names                                | ✅     |
| `TIMESTAMPTZ` on `created_on`, `updated_on`                    | ✅     |
| 4 defaults: `gen_random_uuid()`, `true`, `now()`, `now()`      | ✅     |
| `ck_users_role` — four values, ASCII                           | ✅     |
| `uq_users_email` **functional** unique index on `lower(email)` | ✅     |
| `idx_users_role_active` composite `(role, is_active)`          | ✅     |

### ✅ Checkpoint 2 — change_controls migration

**`sql/schema/002_change_controls.sql`** — applied, full up → down → up cycle verified.
The largest file in the schema.

| Check (DB §3.2 / §4.1 / §5.1 / §6.1 / §6.2 / §8.1)                                                                       | Result |
| ------------------------------------------------------------------------------------------------------------------------ | ------ |
| **50 columns** — confirmed by `information_schema.columns` count, not by eye                                             | ✅     |
| Field-group order per §3.2; BRD fields 24 and 34 correctly absent                                                        | ✅     |
| Types incl. `DATE` ×3, `TIME` ×2, `TIMESTAMPTZ` ×5                                                                       | ✅     |
| 10 NOT NULL, 40 NULL (§1.6 — required for Save Draft)                                                                    | ✅     |
| 7 defaults; `cc_id` has none                                                                                             | ✅     |
| `cc_number_seq` + `cc_id GENERATED ALWAYS AS (...) STORED` with the `CASE` LPAD guard (§8.1)                             | ✅     |
| 13 CHECKs, `ck_cc_*` names, values verbatim                                                                              | ✅     |
| Three value traps held: ASCII hyphens in `'Yes - Full testing'`; `'Approve'`/`'Reject'` not past tense; no `'Emergency'` | ✅     |
| 5 FKs → `users(id)`, all `ON DELETE RESTRICT` (§4.1 rows 1–5)                                                            | ✅     |
| `uq_change_controls_cc_id` as a **UNIQUE CONSTRAINT**, not a `CREATE INDEX` (§5.2 #3)                                    | ✅     |
| 6 `CREATE INDEX` (§5.1 #4–#9), `DESC` on `idx_cc_created_on`                                                             | ✅     |
| Down drops **table then sequence**; `\ds` confirms; re-`up` succeeds                                                     | ✅     |

**Lesson:** a separately-created sequence is not owned by the table. `DROP TABLE` alone
orphans it and the next `up` fails. Order matters — dropping the sequence first fails,
because the column default depends on it.

### ✅ Checkpoint 3 — file_attachments migration

**`sql/schema/003_file_attachments.sql`** — applied, cycle clean.

| Check (DB §3.3 / §4.1 / §5.1 #10 / §5.3 / §6.1 / §6.2)                 | Result |
| ---------------------------------------------------------------------- | ------ |
| 9 columns, all NOT NULL; `BYTEA` / `BIGINT` correct                    | ✅     |
| 2 defaults; `ck_file_attachments_field_name`                           | ✅     |
| `change_control_id` → **ON DELETE CASCADE** (§4.1 #6)                  | ✅     |
| `uploaded_by_id` → **ON DELETE RESTRICT** (§4.1 #7)                    | ✅     |
| `uq_file_attachments_cc_field` as a **UNIQUE CONSTRAINT** (§5.2 #3)    | ✅     |
| **Zero `CREATE INDEX` statements** (§5.3)                              | ✅     |
| No `file_size` CHECK — 10 MB and MIME rules stay in the handler (§3.3) | ✅     |

**Lesson:** no separate index on `change_control_id` — leftmost prefix of the composite
already serves "all files for this CC".

### ✅ Checkpoint 4 — audit_logs migration

**`sql/schema/004_audit_logs.sql`** — applied, cycle clean.

| Check (DB §3.4 / §2.3 / §4.1 #8 / §5.1 #11–13 / §6.1 / §6.2)         | Result |
| -------------------------------------------------------------------- | ------ |
| 10 columns; 7 NOT NULL, 3 nullable                                   | ✅     |
| **No `action_description` column**                                   | ✅     |
| `ck_audit_logs_entity_type` (2), `ck_audit_logs_action_type` (**9**) | ✅     |
| **`entity_id` is a bare `UUID NOT NULL` with no FK** (§2.3)          | ✅     |
| `fk_audit_logs_performed_by_id` RESTRICT — the only FK               | ✅     |
| 3 indexes; no UNIQUE; no immutability triggers (§8.3)                | ✅     |

**Lesson:** `entity_id` looks exactly like a foreign key and must not be one — it points
at either table depending on `entity_type`, and audit rows must outlive what they describe.

### ✅ Checkpoint 5 — esignatures migration

**`sql/schema/005_esignatures.sql`** — applied, cycle clean.

| Check (DB §3.5 / §4.1 #9–10 / §4.3 / §5.1 #14 / §6.1 / §6.2)                   | Result |
| ------------------------------------------------------------------------------ | ------ |
| 7 columns, all NOT NULL; no `updated_on`, no soft-delete (§3.5)                | ✅     |
| `ck_esignatures_transition` — T2–T8, **T1 never signs**                        | ✅     |
| `ck_esignatures_meaning` — 7 values, **ASCII hyphens verified in the catalog** | ✅     |
| Both FKs **RESTRICT**, incl. `change_control_id` (§4.3)                        | ✅     |
| `idx_esignatures_cc`; **no UNIQUE** (rejection loops repeat a gate)            | ✅     |

**Lesson:** mirror image of checkpoint 3 — `change_control_id` CASCADEs in
`file_attachments` and RESTRICTs here. Same column, same target, opposite rule.

### ✅ Checkpoint 6 — refresh_tokens migration · schema complete

**`sql/schema/006_refresh_tokens.sql`** — applied, cycle clean.

| Check (DB §3.6 / §4.1 #11 / §4.3 / §5.1 #15 / §6.2 / §6.4)           | Result |
| -------------------------------------------------------------------- | ------ |
| 6 columns; **PK is `token TEXT`, no surrogate `id UUID`**            | ✅     |
| `revoked_at` the only nullable column                                | ✅     |
| **`updated_on`, not `updated_at`** — flag #3 resolved for the DB doc | ✅     |
| 2 defaults; **zero CHECK constraints** — the only such table         | ✅     |
| `fk_refresh_tokens_user_id` **ON DELETE CASCADE** (§4.3)             | ✅     |
| `idx_refresh_tokens_user`                                            | ✅     |

**Lesson:** three timestamps, three jobs. `updated_on` = 30-min **sliding** inactivity
window; `expires_at` = absolute cap; `revoked_at` = logout. And CASCADE here vs RESTRICT
on `audit_logs`/`esignatures` — a session is disposable, a signature is not.

### ✅ Checkpoint 7 — sqlc setup

**`sqlc.yaml`** (v2) — engine `postgresql`, queries `sql/queries`, schema `sql/schema`,
out `internal/database`, package `database`. sqlc understands goose annotations and
ignores the `-- +goose Down` statements.

**`sql/queries/users.sql`** — two queries:

- `CreateUser :one` — inserts only `full_name`, `email`, `hashed_password`, `role`;
  the other four columns come from schema defaults. `RETURNING *`
- `GetUserByEmail :one` — `WHERE LOWER(email) = LOWER(sqlc.arg(email))`. The `LOWER()`
  wrapper is required twice over: case-insensitive login, **and** the planner only uses
  `uq_users_email` when the query contains the same expression the index was built on.
  `sqlc.arg(email)` forces the generated parameter name (sqlc otherwise inferred `lower`)

**Generated:** `internal/database/{db.go, models.go, users.sql.go}` — never hand-edited.
`WithTx(tx *sql.Tx)` is present, which Blueprint §9's `qtx := cfg.db.WithTx(tx)` needs.

**Deps:** `lib/pq`, `google/uuid`. sqlc and goose are CLI tools, not imports — correctly
absent from `go.mod`.

**Verified:** `models.go` no longer imports `database/sql` at all — proof that zero
`sql.NullXxx` types remain. `ChangeControl` has all 50 fields, 40 as pointers and the 10
NOT NULL ones plain. `User`, `Esignature` and `FileAttachment` are entirely plain, so
`nullable: true` scoped correctly.

**Lesson:** see decision #10 — the Blueprint §2 / §4 contradiction and how it was resolved.

### ✅ Checkpoint 8 (Part A) — `internal/auth` argon2id + app wiring

Auth foundation plus the application plumbing the API sits on. `go build ./...` and
`go test ./...` both pass.

**`internal/auth/password.go`** — `HashPassword(password, *argon2id.Params)` and
`CheckPasswordHash(password, hash)`, wrapping `alexedwards/argon2id`. Blueprint §2 names
the _algorithm_ (argon2id), not a package, so the library choice was open — using a
reviewed implementation rather than hand-rolling PHC encoding, `crypto/rand` salting and
constant-time comparison. `CheckPasswordHash` returns `(false, nil)` for a wrong password
and an error only for a malformed hash, so the 401-vs-500 distinction survives to the
caller.

**`internal/auth/password_test.go`** — external test package (`auth_test`), so it exercises
only the exported API as a real caller would. Asserts: correct password matches; wrong
password → `false` with `nil` error; the same password hashed twice → two different strings
(per-call random salt). Uses `argon2id.DefaultParams` so the test needs no `.env`.

**`apiConfig`** — all four Blueprint §8 fields: `db *database.Queries`, `rawDB *sql.DB`,
`secret string`, `params *argon2id.Params`. `rawDB` is required for `BeginTx`, which
`*database.Queries` cannot do on its own.

**`main.go`** — `sql.Open` **followed by `Ping`**: `Open` is lazy and will not surface bad
credentials or a stopped server, so `Ping` is what actually fails loudly at startup.
Distinct fatal messages for the two failures (driver/URL vs network/credentials/server).
`log.Fatal(server.ListenAndServe())`.

**`helpers.go`** — `respondWithJSON` / `respondWithError` and the `errorResponse` type.
**`config.go`** — `loadArgon2idParams` / `parseUintConfig`: params from env with explicit
code defaults. A **missing** variable falls back to the default; a **malformed** one is
**fatal** and names the offending variable and value — no silent weakening of a security
parameter. `.env.example` carries the five `ARGON2ID_*` keys.

**Structure note (Blueprint §5):** flat `package main` at the repo root — `main.go`,
`helpers.go`, `config.go`, and later `middleware.go` / `handlers_*.go`. Only `internal/auth`
and `internal/database` are separate packages. Handlers get their own files, all still
`package main`, created when written — not stubbed ahead. Run with **`go run .`** (compiles
the whole package), not `go run main.go` (one file — can't see the helpers).

**Lesson:** `sql.Open` does not connect. Without `Ping`, a bad DSN or a down server yields
a clean startup and a failure on the first query, far from the cause.

---

### ✅ Checkpoint 8 (Part B) — `cmd/seed`

**`cmd/seed/main.go`** — standalone dev-only command (DB §7). Its own `package main` under
`cmd/`, run with `go run ./cmd/seed`. Sequence: `godotenv.Load` → **`PLATFORM == "dev"` gate
before any DB connection** (§7.5) → `sql.Open` + `Ping` → `database.New` → hash
`DevPassw0rd!` once with `auth.HashPassword` (argon2id) → loop the four §7.2 users through
the generated `CreateUser`.

Four users seeded and verified in the table: correct roles (`'CC Owner'` with its space
satisfied `ck_users_role`), `is_active = t` and `created_on` from schema defaults never
touched by the insert, `created_on` carrying a `+04` offset — `TIMESTAMPTZ` storing zone,
not naive local time (§1.4, now visible in real data). UUIDs generated by the database.

**Second-run behaviour: fail loudly.** Errors are fatal per user; a re-run stops on the
first `uq_users_email` duplicate with Postgres code **`23505`** surfaced through `pq` —
proving the case-insensitive unique index is live and the seed won't double-insert. That
`23505` inspection is the same one Blueprint §11 uses to turn a unique violation into a
clean 409.

**Lesson:** this was the first full `Go → sqlc → lib/pq → Postgres` path — every layer
(config load, driver, argon2id hashing, sqlc query, schema defaults) proving itself at
once, with nothing hand-wired.

### ✅ Checkpoint 8b — structured logging (slog + context)

Consciously fills the Blueprint §15 gap (was `log.Printf`, no correlation IDs). **All
plumbing is now complete.**

**`internal/logging/logging.go`** —

- `NewLogger(logDir)` builds a `*slog.Logger` whose handler is a custom **`multiHandler`**
  fanning out to two destinations: `slog.NewTextHandler` → console (human-readable) and
  `slog.NewJSONHandler` → `logs/app.log` (append). slog ships no multi-destination handler,
  so `multiHandler` implements the four `slog.Handler` methods and forwards each to both
  children. The log file is never closed — it lives for the process lifetime (deliberate).
- **`contextKey` / `loggerKey`** — unexported key type + const, so the context key can't
  collide and only this package can touch it.
- **`ContextWithLogger(ctx, logger)`** (setter) and **`LoggerFrom(ctx)`** (getter) — the
  guarded door in and out of context. `LoggerFrom` uses the `comma-ok` assertion and falls
  back to `slog.Default()` if absent — never fails, never panics.

**`middlewareLogging`** (in `middleware.go`, outermost middleware) — mints a
`uuid.NewString()` request ID, derives a per-request logger with
`cfg.logger.With("request_id", id)`, stashes it in context via `ContextWithLogger`, and
logs `request started` / `request finished` with method, path and duration.

**Handlers** pull the request logger with `log := logging.LoggerFrom(r.Context())` as the
first body line — every line they emit is auto-stamped with that request's `request_id`.
This is the one reusable line dropped into all 22 handlers.

**`main.go`** — logger built **first**, before config, so startup failures are logged;
`slog.SetDefault`; the startup `log.Fatalf` calls converted to `logger.Error` + `os.Exit`.
`WelcomeHome` wired through `middlewareLogging` (variation 1) as the end-to-end proof.

**Verified:** three lines (`request started` → `welcome home hit` → `request finished`)
share one `request_id` across console (text) and `logs/app.log` (JSON); concurrent requests
get distinct IDs; the `Warn` path (blank message) fires from inside the handler carrying its
own request's ID. `logs/` gitignored.

**Lesson — the delivery split (decision #12).** The request logger travels via **context**
because its absence is harmless (`LoggerFrom` degrades to default). The authenticated user
(coming next) will travel as an **explicit handler argument** because its absence must be a
_compile error_, not a runtime surprise. Same principle — match the delivery mechanism to
what happens when the thing is missing. Also learned: `http.Handler` is the interface
(accept it, call `next.ServeHTTP`); `http.HandlerFunc(...)` is the converter that turns a
bare `func(w,r)` into a handler. Middleware output registers with `mux.Handle`; bare
handlers with `mux.HandleFunc`.

---

## Next

### ⬜ Checkpoint 9 — `POST /api/login` (first real endpoint)

The first genuine feature, logging in as one of the four seeded accounts. Per the API
Endpoint Plan (endpoint 1) and Blueprint §6–§7: decode `{email, password}` → `GetUserByEmail`
(case-insensitive, already built) → reject if `!is_active` → `auth.CheckPasswordHash` →
issue a JWT access token (30 min) + a refresh token row. Needs the JWT half of
`internal/auth` (`MakeJWT`) written for the first time. First handler to use the
`log := logging.LoggerFrom(r.Context())` thread in anger.

**Then, in API Endpoint Plan order:** `middlewareAuth` (variation 2/3 registration; the
`authedHandler` explicit-user pattern) → `GET /api/me` → refresh/revoke (sliding window) →
user management (incl. the `FOR UPDATE` transaction) → CC create/get/list/save-draft →
**T2 submit, the first full transition, written inline** → T3, T4/5 (extract only then) →
files → T6 → T7/8 → dashboard → signatures.

---

## Session decisions not in any guardrail doc

Settled in working sessions and binding. They exist nowhere else.

| #   | Decision                                                                                                                                                                                    | Rationale                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                |
| --- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1   | **Module path `github.com/lain-the-coder/ea-qms-backend`** — not `-cc-backend`                                                                                                              | Future QMS modules (Deviation, CAPA) live under the same module rather than forcing a second repo                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                        |
| 2   | **Constraint/index naming follows §5.1 and §6.1 verbatim, including their abbreviations** — `ck_cc_*`, `idx_cc_*`, `idx_audit_*` short; CHECKs full (`ck_audit_logs_*`, `ck_esignatures_*`) | §5.1/§6.1 are definitions, cross-referenced by name elsewhere (§8.2 cites `idx_cc_owner_state`); §1.3 is a convention statement with one stale example. Also keeps names clear of Postgres's 63-byte identifier truncation                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               |
| 3   | **Two naming exceptions kept verbatim:** `uq_change_controls_cc_id` and `ck_cc_post_impl_issues`                                                                                            | Spelled that way in §3.2/§5.1 and §6.1 — do not "regularize" them                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                        |
| 4   | **FK constraints use the long form** — `fk_<table>_<column>`                                                                                                                                | §4 lists all eleven FKs but never names the constraints, so §1.3 stands unopposed here. The name is what appears in the Postgres error you map to a 409 (Blueprint §11)                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                  |
| 5   | **PostgreSQL 14.23 accepted** (doc §1.2 specifies 15+)                                                                                                                                      | Every needed feature traced and predates 14: `gen_random_uuid()` core (13), `GENERATED ALWAYS AS ... STORED` (12), `ON CONFLICT DO UPDATE` (9.5), `SELECT ... FOR UPDATE`, functional/composite indexes                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                  |
| 6   | **`log.Fatal(server.ListenAndServe())`**, not a bare call                                                                                                                                   | A discarded error means a bind failure exits silently with status 0 and no message                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                       |
| 7   | **Goose run as a global CLI from `sql/schema`**                                                                                                                                             | Keeps migration files free of Go wiring                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                  |
| 8   | **Uniqueness form rule:** plain columns → table `CONSTRAINT ... UNIQUE`; expressions or partials → `CREATE UNIQUE INDEX`                                                                    | A `UNIQUE` table constraint accepts only a column list, so `uq_users_email` on `LOWER(email)` _must_ be an index. Constraints are preferred otherwise: `ON CONFLICT ON CONSTRAINT <name>`, visibility in `information_schema.table_constraints`, and Postgres's own recommendation                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                       |
| 9   | **DBeaver is connected for browsing only.** All schema changes go through goose                                                                                                             | Applied migrations are the schema's only description (Blueprint §13). DBeaver also splits `\d` across tabs and blurs the constraint-vs-index distinction from #8 — **DBeaver to navigate, psql to verify**                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               |
| 10  | **Nullable columns are forced to Go pointers via explicit sqlc `db_type` overrides, keeping `lib/pq`**                                                                                      | **Resolves a real contradiction between Blueprint §2 and §4.** sqlc's `emit_pointers_for_null_types` is _silently ignored_ unless `sql_package` is `pgx/v4` or `pgx/v5` — so §2 (lib/pq, deliberate) and §4 (pointers) cannot both hold as written. Rejected: switching to pgx (abandons §2's reasoning and changes the `BeginTx`/`WithTx` shape) and accepting `sql.NullXxx` (pays every cost §4 argued against — garbage JSON, a ×40 mapping loop, hand-rolled three-state draft logic). Five overrides give both. **The `db_type` spellings are not uniform and were found empirically: `text`, `timestamptz`, `date`, `uuid` bare; `time` requires `pg_catalog.time`.** Also: omit the `package` key when the import path already ends in the package name, or sqlc emits duplicate imports and the build fails                      |
| 11  | **Password hashing uses `github.com/alexedwards/argon2id`, not raw `golang.org/x/crypto/argon2`**                                                                                           | Blueprint §2 names the algorithm (argon2id), not a package, so the choice was open. The library already does PHC-string encoding, `crypto/rand` salting, parameter round-tripping and constant-time comparison — a reviewed implementation rather than hand-rolled crypto plumbing. Params are set **explicitly** (not `DefaultParams` in app code) so a library-default change can't silently alter hashing strength, and so the values are auditable                                                                                                                                                                                                                                                                                                                                                                                   |
| 12  | **Data delivered to handlers by two different mechanisms, chosen by failure mode: request logger via `context`, authenticated user via explicit argument**                                  | Fills the Blueprint §15 logging gap. The rule: _match the delivery mechanism to what happens when the thing is missing._ A missing logger is harmless → `context` value with a `slog.Default()` fallback (`LoggerFrom`). A missing authenticated user is a security hole → explicit third argument on an `authedHandler` type, so forgetting auth is a **compile error**, not a runtime surprise — the compiler becomes an auth control, which matters for a regulated system. Not inconsistency: same principle, opposite stakes. (Considered and rejected: context for both, for surface consistency — it would trade a compile-time guarantee for a per-route discipline across 22 routes.) Logging is minimal per §0/§15: request ID + start/finish + errors; runtime level-filtering deferred (slog provides the levels regardless) |

---

## Open flags

| #   | Flag                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                            | Status                                                                                                             |
| --- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------ |
| 1   | **`change_controls` column count contradiction.** §3.2 and the §3 Summary state 48; §3.2's own parenthetical sums to 50                                                                                                                                                                                                                                                                                                                                                                                                                         | **Resolved: built 50, confirmed in the database.** Doc correction pending                                          |
| 2   | **`change_controls` DEFAULT count.** §6.4 says 8; §6.2 enumerates 7 (`cc_id` uses a generation expression, not a DEFAULT)                                                                                                                                                                                                                                                                                                                                                                                                                       | **Resolved: 7, confirmed in the database.** Doc correction pending                                                 |
| 3   | **`updated_at` vs `updated_on` on `refresh_tokens`.** Blueprint §7's code sample uses `updated_at`; DB Design §3.6 says `updated_on`                                                                                                                                                                                                                                                                                                                                                                                                            | **Resolved in the schema: `updated_on`.** The Blueprint snippet is stale — adjust when writing the refresh handler |
| 4   | **En-dash in HTML prototype `<option value="...">`.** A frontend built from the prototypes verbatim fails `ck_cc_requires_testing` on every submit. Frontend must normalize at the API boundary, or the prototypes get fixed                                                                                                                                                                                                                                                                                                                    | Open for `change_controls`; closed for `esignatures` (DB §6.5)                                                     |
| 5   | **BRD §13.1 deferral note** for the three descoped password flows                                                                                                                                                                                                                                                                                                                                                                                                                                                                               | Lain to add on next BRD touch                                                                                      |
| 6   | **Production version parity.** Dev is on PostgreSQL 14.23; if production runs 15/16 there's a major-version gap. No feature dependency — belongs in deployment notes                                                                                                                                                                                                                                                                                                                                                                            | Noted                                                                                                              |
| 7   | **The two `.docx` guardrail files are stored as plain text** despite the extension. Read them directly; do not unzip                                                                                                                                                                                                                                                                                                                                                                                                                            | Environmental note                                                                                                 |
| 8   | **CC-ID gaps are expected and permanent.** `nextval()` is non-transactional, so a rolled-back or failed insert burns a number forever. Not a defect — the cost of collision-free IDs under concurrency — but QA will ask                                                                                                                                                                                                                                                                                                                        | Behaviour note; may warrant a line in user documentation                                                           |
| 9   | **`TIME` columns scanning into `*time.Time` is unverified at runtime.** `database/sql`'s `convertAssign` handles pointer-to-pointer natively, so `*string` and `*time.Time` are safe for `text`/`timestamptz`/`date` with lib/pq. But bare `TIME` (`implementation_window_start` / `_end`) may arrive from lib/pq as `[]byte` rather than `time.Time`, which would fail conversion. **First exposed when reading a CC with window times (≈ endpoint 12).** If it fails, the fix is a `column:` override to `string` plus parsing in the handler | Unverified — test at first read                                                                                    |

---

## Environment & workflow

WSL Ubuntu 22.04 · VS Code Remote-WSL · Go 1.25.3 · PostgreSQL 14.23 · sqlc v1.31.1 ·
DBeaver for browsing (decision #9).

```bash
# migrations — run from sql/schema
goose postgres "postgres://postgres:PASS@localhost:5432/ea_qms?sslmode=disable" up
goose postgres "postgres://postgres:PASS@localhost:5432/ea_qms?sslmode=disable" down
goose postgres "postgres://postgres:PASS@localhost:5432/ea_qms?sslmode=disable" status

# dry-run a migration before handing it to goose — psql gives a line number and a caret
psql "postgres://postgres:PASS@localhost:5432/ea_qms?sslmode=disable" -f <file>.sql

# sqlc — run from the repo root
rm -rf internal/database && sqlc generate && go build ./... && go mod tidy

# go — run the whole main package, not a single file
go run .                    # NOT `go run main.go` (that can't see helpers.go / config.go)
go build ./...              # compile-check every package
go test ./...               # run all tests (build alone does not execute them)
go run ./cmd/seed           # run the seed command (checkpoint 8 Part B)

# psql
psql "postgres://postgres:PASS@localhost:5432/ea_qms?sslmode=disable"
#   \l  databases   \dt  tables   \ds  sequences   \d  everything   \d <table>  detail
#   \pset pager off        before \d on wide tables, or the output gets mangled
```

**Every migration gets up → `\d` → down → `\dt` (+ `\ds` if it creates a sequence) → up
before it counts as done.** The final `up` is easy to forget — `goose status` confirms
where the database actually stands.

### Things learned at the prompt

- Postgres **rewrites `IN (...)` as `= ANY (ARRAY[...])`** in the catalog, so `\d` never
  reads back character-for-character as written. Normalization, not drift.
- A **UNIQUE constraint** displays as `UNIQUE CONSTRAINT, btree (col)`; a bare
  `CREATE UNIQUE INDEX` displays as `UNIQUE, btree (col)`.
- `TIMESTAMPTZ` → `timestamp with time zone`; `TIME` → `time without time zone`.
- **Verify counts with SQL, not by counting a terminal paste:**
  `SELECT count(*) FROM information_schema.columns WHERE table_name = 'x';`
- **Postgres column-definition rule** (cost several rounds on 002 and 003): everything
  between `CREATE TABLE x (` and `)` is one comma-separated list. Column-level constraints
  sit inside a column's definition and take no column list; table-level constraints (FKs,
  multi-column UNIQUE) are their own list items and require one. Comma between items, none
  after the last, semicolon only after the closing paren.
- **Sequences are non-transactional** — `nextval()` does not roll back (flag #8).
- Once a statement errors inside a transaction, psql aborts the block until `ROLLBACK`.
- **Invisible characters can't be eyeballed** — scan a file for any codepoint above 127
  before running it. Every value in this schema is ASCII.
- **Copy-paste between migrations is the most common error source.** 006's index was
  briefly created `ON esignatures`; it failed only because the column name didn't exist
  there. Always re-read the table name in a copied `CREATE INDEX`.
- **`go build ./...`** — `...` is Go's recursive package wildcard, so this compiles every
  package in the module rather than just the current directory. Build first, `go mod tidy`
  after it's clean; tidy parses imports and is unreliable on broken files.
- **`go run .` runs the package; `go run main.go` runs one file.** Once `package main` is
  split across files (`helpers.go`, `config.go`), the single-file form fails with
  "undefined" errors for everything in the other files. Use `go run .`.
- **`sql.Open` does not connect** — it's lazy. Always `Ping` right after, so a bad DSN or a
  down server fails at startup instead of on the first query in a handler.
- **Review loop:** the repo is public, so committed code can be cloned and reviewed
  directly — push, say "pushed", no need to paste files. Only committed code is visible;
  uncommitted working-tree changes are not. `.env` is correctly gitignored and never seen.
