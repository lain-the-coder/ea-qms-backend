# PROGRESS — EA QMS Change Control Backend (Go)

**Scope of this file:** what is built, what is next, decisions made in working sessions
that are not recorded in any guardrail document, and open flags. Nothing else — the six
guardrail docs carry the substance and are always attached.

- **Repo:** `github.com/lain-the-coder/ea-qms-backend`
- **Last checkpoint:** 22 — **T3 cancel** · **15 of 22**
- **Next task:** checkpoint 23 — **T4/T5 decision** (endpoint 17) — then extract the shared helpers
- **Schema version:** 6 · all six tables built and verified
- **Review loop:** paste code in chat for review _before_ committing — review precedes
  commit, never follows it. (The repo is public and can be cloned if ever useful to look
  at already-committed code, but that is not the default workflow.)

---

## Phase status

| Phase                               | State                                             |
| ----------------------------------- | ------------------------------------------------- |
| Migrations (001–006)                | ✅ Complete — all six tables applied and verified |
| sqlc setup                          | ✅ Complete — pointer types working under lib/pq  |
| `internal/auth` (argon2id)          | ✅ Complete — hashing + tests + app wiring        |
| `cmd/seed`                          | ✅ Complete — 4 users seeded and verified         |
| Structured logging (slog+context)   | ✅ Complete — request IDs proven end to end       |
| API — Group 1 Auth (1–3)            | ✅ Complete                                       |
| API — Group 2 Users & Profile (4–9) | ✅ Complete                                       |
| API — Group 3 CCs (10–13)           | ✅ Complete                                       |
| API — Group 5 Workflow (15–19)      | 🔵 **2 / 5** — T2 ✅, T3 ✅ (both fully inline)   |
| API — Groups 4, 6, 7 (14, 20–22)    | ⬜ Dashboard, files, signatures                   |

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

### ✅ Checkpoint 9 — `POST /api/login` (endpoint 1 of 22)

First real endpoint. Registration is **variation 1** (public — logging only, bare handler
wrapped in `http.HandlerFunc`). `WelcomeHome` retired.

**`internal/auth/jwt.go`**

- `MakeJWT(userID, secret, expiresIn)` — HS256, `jwt.RegisteredClaims` only, issuer
  `ea-qms` via a shared `const issuer`, one `time.Now().UTC()` anchor for both `iat`/`exp`.
- **No role in the token, deliberately.** `middlewareAuth` loads the user from the DB on
  every request anyway (it must, for `is_active`), so the role is always fresh. Baking it
  into a 30-minute token would mean a BR-8.4.11 role change silently doesn't take effect
  for up to half an hour.
- `ValidateJWT` — `ParseWithClaims` with a keyfunc that **validates the signing method is
  HMAC** (rejects `alg` confusion / `none` attacks) and `jwt.WithIssuer(issuer)`. Expiry is
  checked by the library during parse. Returns `uuid.Nil` + wrapped error on failure.

**`internal/auth/tokens.go`** — `MakeRefreshToken()`: 32 bytes from `crypto/rand`, hex.
**Returns an error rather than `log.Fatal`** — a library function must never kill the
process; that decision belongs to the caller.

**`internal/auth/jwt_test.go`** — round trip, wrong secret rejected, expired rejected,
malformed rejected. All pass.

**`sql/queries/refresh_tokens.sql`** — `CreateRefreshToken :one` (token, user_id,
expires_at; the rest from schema defaults).

**`handlers_auth.go`** — decode → blank checks → `GetUserByEmail` → `CheckPasswordHash` →
`is_active` → `MakeJWT` → `MakeRefreshToken` → `CreateRefreshToken` → respond.

| Check                                                                                | Verified |
| ------------------------------------------------------------------------------------ | -------- |
| Six-field response (id, full_name, email, role, token, refresh_token)                | ✅       |
| `ADMIN@EAQMS.LOCAL` logs in — `LOWER(email)` query + functional index                | ✅       |
| Wrong password and unknown user return **byte-identical** 401s                       | ✅       |
| JWT payload: `exp − iat = 1800` (30 min exactly)                                     | ✅       |
| `refresh_tokens` rows with `expires_at` 24 h out, `revoked_at` NULL                  | ✅       |
| Logs: shared `request_id`, WARN + real `reason`, generic client message, no password | ✅       |
| All four seeded accounts log in; `owner@` returns role `CC Owner` (space intact)     | ✅       |
| Timing equalised: unknown email 209 ms vs wrong password 229 ms (was 5 ms vs 137 ms) | ✅       |

**Key ordering decision — password checked _before_ `is_active`.** The reverse would tell an
attacker "Account is deactivated" for a _wrong_ password, revealing the account exists.
Checking the password first means bad credentials always get the same generic 401, and
"deactivated" is only disclosed to someone who already proved they hold valid credentials.

**Timing side-channel closed.** The not-found path originally skipped argon2id entirely, so
valid emails were enumerable by response time alone (5 ms vs 137 ms — a 27× gap, measured).
Fix: `apiConfig.dummyHash` is generated **once at startup** with the real argon2id params,
and the not-found branch runs `CheckPasswordHash` against it (`_, _ =` — the result is
discarded, only the elapsed cost matters). Both branches now cost ~210–230 ms.

**Other decisions:** token lifetime is server-controlled (`accessTokenTTL = 30m` const) —
the Chirpy pattern of a client-supplied `expires_in_seconds` is an attacker-controlled
lifetime and was dropped. `refreshTokenTTL = 24h` is the **absolute** cap (see decision #13).
**No audit row** — `ck_audit_logs_action_type` has no login action; login isn't audited in
Phase 1.

**Logging convention established** (reused by every handler from here): short **stable
`msg`** (`"login failed"`) + a **`reason`** field for the variant + identifying fields
(`email`, `user_id`) + `error` only on system faults. `Warn` = user error, `Error` = system
fault. This makes `msg="login failed"` find every failure while `reason=` narrows it.

---

### ✅ Checkpoint 10 — `middlewareAuth` + `GET /api/me` (endpoint 4 of 22)

The auth spine. Heavy checkpoint, light endpoint: most of the work is infrastructure that
**19 of the 22 endpoints** then reuse for free.

**`internal/auth/tokens.go`** — `GetBearerToken(http.Header)`. Uses `strings.CutPrefix`
**with its `found` bool**, not `TrimPrefix`: `TrimPrefix` returns the string _unchanged_ when
the prefix doesn't match, so `Authorization: Basic abc123` would silently yield
`"Basic abc123"` as the token.

**`internal/auth/tokens_test.go`** — table-driven with `t.Run` subtests, 9 cases: valid,
absent header, empty header, wrong scheme, `Bearer` with no token, whitespace-only,
surrounding whitespace trimmed, dotted JWT intact, and **lowercase `bearer` rejected**
(RFC 7235 says the scheme is case-insensitive; being strict is a deliberate choice, since
every real client sends `Bearer`).

**`sql/queries/users.sql`** — `GetUserByID :one`.

**`middleware.go`**

- `type authedHandler func(http.ResponseWriter, *http.Request, database.User)` — the
  three-parameter contract.
- `middlewareAuth(next authedHandler) http.Handler` — four gates: bearer token → validate
  JWT → load user → `is_active`. Then `next(w, r, user)` — a **direct call**, because
  `authedHandler` is a plain func type (contrast `next.ServeHTTP` for the `http.Handler`
  interface in `middlewareLogging`).
- **The user is loaded from the database on every request**, never trusted from the token.
  That is what makes deactivation and role changes take effect immediately rather than
  after up to 30 minutes — and why the role is not in the JWT (checkpoint 9).
- `ErrNoRows` on the lookup → **401** (a valid JWT for a deleted user is an auth failure,
  and distinguishing it would leak information); any other DB error → **500**.
- Returns `http.Handler` rather than Blueprint §7's `http.HandlerFunc` — interchangeable
  (`http.HandlerFunc` satisfies `http.Handler`), chosen so both middlewares share one shape.

**`handlers_users.go`** — `HandlerGetMe(w, r, user)`. **No DB call**: the middleware already
fetched the user, so the handler just maps it to a response struct (rule 3 — never marshal
`database.User`, it carries `hashed_password`).

**Registered as variation 2:** `middlewareLogging(middlewareAuth(handler))` — no
`http.HandlerFunc(...)` wrap, since `middlewareAuth` already returns a handler.

| Check                                                                            | Verified |
| -------------------------------------------------------------------------------- | -------- |
| Admin → `/api/me` returns Admin's profile                                        | ✅       |
| CC Owner → `/api/me` returns CC Owner's profile (token identifies the user)      | ✅       |
| Garbage token → 401, `reason: invalid jwt`                                       | ✅       |
| No `Authorization` header → 401, `reason: token extraction failed`               | ✅       |
| `/api/me` runs in 2–4 ms vs login's 158–417 ms — argon2id is paid once, at login | ✅       |

**Lesson — decision #12 made concrete.** `mux.HandleFunc("GET /api/me", cfg.HandlerGetMe)`
**does not compile**: a three-parameter handler is not a valid `http.HandlerFunc`, so the
only way to route it is through `middlewareAuth`, which supplies the user. Forgetting
authentication is a build error, not a runtime hole.

**Logging convention reinforced:** auth failures are `Warn` (client error — an expired token
is normal user behaviour and must not page anyone), DB failure is `Error` (system fault).
**One concept, one key name, everywhere** — `user_id`, not `id`, matching login, so a single
grep returns everything about a user.

**Identity logging.** Before calling `next`, `middlewareAuth` enriches the request-scoped
logger (`log = log.With("user_id", user.ID)`, written back into context) **and** emits one
`"authenticated"` line carrying `user_id`, `role`, `email`. Two distinct benefits: the
enrichment means every later handler line inherits `user_id` for free, and the explicit line
answers "who made this request" even for handlers that log nothing of their own — one line
of code covering all 19 protected endpoints instead of one per handler.

Note the enrichment does _not_ reach `middlewareLogging`'s "request finished" line: that
middleware holds its own `reqLogger` variable created before auth ran. Context enrichment
only reaches code that reads the logger _from context after_ the enrichment — i.e. handlers.

**Division of labour:** middleware logs the request lifecycle and identity; handlers log
decisions and failures. A handler with no branches (like `HandlerGetMe`) staying silent is
correct, not a gap.

---

### ✅ Checkpoint 11 — `POST /api/refresh` + `POST /api/revoke` (endpoints 2–3)

**Auth group complete (4 of 22).** Both public — variation 1, outside `middlewareAuth`,
since they carry a refresh token rather than an access JWT.

**`sql/queries/refresh_tokens.sql`** — `GetRefreshToken :one`, `TouchRefreshToken :exec`
(`SET updated_on = NOW()`), `RevokeRefreshToken :exec`.

**`HandlerRefresh`** — six rejection gates, then touch, then mint:

| Gate                        | Client sees                 | Logged `reason`                    |
| --------------------------- | --------------------------- | ---------------------------------- |
| token blank                 | 400                         | `refresh token blank`              |
| row not found               | 401 `Invalid refresh token` | `refresh token not found`          |
| `RevokedAt != nil`          | 401 `Invalid refresh token` | `refresh token revoked`            |
| `now > expires_at`          | 401 `Session expired`       | `refresh token time limit expired` |
| `now − updated_on > window` | 401 `Session expired`       | `inactivity timeout`               |
| `!user.IsActive`            | 401                         | `account deactivated`              |

On success: `TouchRefreshToken` (slides the window) → new 30-minute JWT → `{"token": ...}`.

`RevokedAt` is `*time.Time`, so the check is a plain `!= nil` — the sqlc pointer overrides
from checkpoint 7 paying off (no `sql.NullTime{}.Valid`). `is_active` is re-checked here
because otherwise a deactivated user could keep minting JWTs for up to 24 hours.

**`HandlerRevoke`** — decode → blank check → revoke → **204, no body**.

| Check                                                        | Verified |
| ------------------------------------------------------------ | -------- |
| Login → Refresh → 200, new token                             | ✅       |
| Refresh with blank token → 400                               | ✅       |
| Refresh with garbage token → 401 `refresh token not found`   | ✅       |
| Revoke → 204                                                 | ✅       |
| Refresh with the revoked token → 401 `refresh token revoked` | ✅       |
| Revoke again → 204 (idempotent)                              | ✅       |

The logs are self-verifying on two points: `refresh successful` implies `TouchRefreshToken`
returned nil, and `refresh token revoked` on the next attempt proves `revoked_at` was
actually written.

**Lesson — idempotency.** _Doing it once and doing it many times leave the same state._
`PUT` and `DELETE` are idempotent by HTTP contract; `POST` is not. Revoke is written to be
idempotent because logout should never fail and because identical responses for
"token existed" and "token didn't" avoid an information leak — the same reasoning as login's
byte-identical 401s. Contrast the workflow transitions (T2–T8), which must **not** be
idempotent: the state check (`current_state = 'Initiated'`) is what makes a double-submit a
409, protecting against a duplicate e-signature.

**The refresh token is never logged** — it is a live 24-hour credential that mints access
tokens, so writing it to `logs/app.log` would be equivalent to logging a password.

---

### ✅ Checkpoint 12 — `POST /api/users` (endpoint 5 of 22)

First **Admin-only** endpoint and the **first database transaction**.

**`middleware.go` — `requireRole(role string, next authedHandler) authedHandler`.**
`authedHandler` in _and_ out (three params both sides), which is what lets it nest **inside**
`middlewareAuth`; and because the return type is already a func type, **no
`http.HandlerFunc` conversion** — unlike the other two middlewares. It loads nothing, just
reads `user.Role` from the user `middlewareAuth` already verified. **Registration
variation 3** appears here:
`middlewareLogging(middlewareAuth(requireRole(roleAdmin, handler)))`.

**`constants.go`** — role, audit entity-type and audit action-type constants, each group
commented with the CHECK constraint it mirrors. Plain string constants, not a defined type:
sqlc emits `Role string`, so a `type Role string` would need a conversion at every call site
for a guarantee `ck_users_role` already provides (Blueprint rule 13). All nine action types
defined up front — unused constants compile fine, and transcribing from the constraint once
beats doing it nine times.

**`sql/queries/audit_logs.sql` — `InsertAuditLog :exec`** (DB §8.3 verbatim). `created_on`
is an **explicit parameter**, not the column default, so multi-row actions can share one
timestamp (BR-8.7.5). Called with `cfg.db` when standalone, `qtx` inside a transaction.

**`handlers_users.go` — `HandlerCreateUser`:**

- Role validated against the constants **before** the DB sees it — an invalid role is then a
  clean 400 rather than a CHECK violation surfacing as a 500. Validate what you can name;
  let the constraint be the backstop.
- **Password policy** (decision #17) with **collect-all** reporting — every unmet rule in one
  message, the same shape BR-8.2.6 requires for the CC transitions.
- **Passwords are never trimmed.** `HandlerLogin` doesn't trim, so trimming at creation would
  hash a different string than login sends — a password with a leading space would create an
  account that can never sign in. Found by comparing the two handlers.
- First use of `cfg.params` (argon2id hashing).
- **The transaction** (decision #18): `BeginTx` → `defer tx.Rollback()` → `qtx` → `Commit`,
  wrapping the user insert _and_ the `UserAdded` audit row.
- **Duplicate email → `*pq.Error` code `23505` → 409**, caught _inside_ the transaction so
  the return rolls back cleanly. **No pre-check `SELECT`** — that would be a TOCTOU race of
  the same shape as BR-8.4.11; the unique index is the source of truth (Blueprint §11).
- The handler's user argument is named **`admin`** — in the `authedHandler` pattern the user
  is always _who is acting_, never _who is acted upon_. An early draft returned the Admin's
  own record as the 201 body; renaming the variable makes that class of bug visible.

| Check                                                              | Verified |
| ------------------------------------------------------------------ | -------- |
| 201 with the created user, `is_active: true`                       | ✅       |
| Duplicate email → 409                                              | ✅       |
| `JANE@EAQMS.LOCAL` → 409 — `LOWER(email)` uniqueness holds         | ✅       |
| `"weak"` → 400 listing **4** unmet rules; `"alllowercase"` → **3** | ✅       |
| Invalid role → 400; blank name → 400                               | ✅       |
| CC Owner → **403** `insufficient role`; no token → **401**         | ✅       |
| **`audit_logs` holds exactly one row after nine requests**         | ✅       |

**Lesson — the transaction proved itself.** Two 409s and five 400s produced _zero_ audit rows
and zero partial users: the failures returned inside the transaction, `defer tx.Rollback()`
fired, nothing survived. That is atomicity observed rather than assumed. The two rules to
carry into T2: **`defer tx.Rollback()` on the line after `BeginTx`** (it is a no-op after a
successful commit, returning `sql.ErrTxDone`), and **`qtx` for every call inside** — a stray
`cfg.db` would run on a different connection, commit immediately, and silently survive the
rollback.

**Also learned:** 401 and 403 answer different questions — _"who are you?"_ vs _"I know who
you are, and you may not do this."_ Both appear in the logs with distinct messages
(`auth failed` vs `authorization failed`), and the 403 line carries `user_id` for free from
`middlewareAuth`'s enrichment.

---

### ✅ Checkpoint 13 — `GET /api/users` (endpoint 6 of 22)

Admin-only, variation 3. Paginated and filterable.

**`sql/queries/users.sql` — `ListUsers :many` / `CountUsers :one`**, both carrying the same
optional filter:

```sql
WHERE (sqlc.narg('is_active')::boolean IS NULL OR is_active = sqlc.narg('is_active'))
ORDER BY full_name
LIMIT $1 OFFSET $2
```

- **`sqlc.narg`** ("nullable named argument") generates a **pointer** parameter rather than a
  value, so `nil` / `&true` / `&false` make **one query serve three cases**. The
  `IS NULL OR` construction is the standard optional-filter idiom — when the parameter is
  null the condition short-circuits to true and the filter disappears. The `::boolean` cast
  is needed because Postgres can't infer the type from `IS NULL` alone. **This is the pattern
  that saves endpoint 11** (`GET /api/changecontrols`, four optional filters — otherwise
  sixteen hand-written query permutations).
- **Both queries must carry the identical `WHERE`**, or `total` won't match the page.
- **`ORDER BY` is mandatory, not cosmetic.** Without a deterministic sort Postgres may return
  rows in a different order between calls, so page 2 could repeat or skip rows from page 1.
- sqlc happily mixed positional `$1/$2` with the named `narg`; parameter order in the
  generated struct is `Limit, Offset, IsActive` — not the order the SQL suggests, so read
  the generated struct rather than guessing.

**`sqlc.yaml`** — sixth override added: **`pg_catalog.bool` → `*bool`** (bare `bool` did not
take). No nullable boolean _column_ exists in the schema, so this only surfaced once a
nullable boolean _parameter_ appeared.

**`helpers.go` — `parsePagination(url.Values) (limit, offset int32, err error)`**

- defaults 50 / 0, maximum 200
- an oversized limit is **clamped, not rejected** — asking for too much should return the
  maximum, not fail
- `strconv.ParseInt(s, 10, 32)` rather than `Atoi`, so an out-of-range value is a clean 400
  instead of silently wrapping negative during the `int32` conversion
- the cap matters: without it `?limit=999999` is a free way to make the server serialize
  every row into a single response

**`handlers_users.go` — `HandlerListUsers`**

- `?active=true|false` → `*bool` via `strconv.ParseBool`; absent leaves it nil
- **`make([]UserResponse, 0, len(users))`** — a nil slice marshals to `null`, not `[]`, which
  breaks a frontend calling `.map()`. Pre-allocating with length 0 guarantees `[]`
- rule 3 applied over a **slice** this time, so `hashed_password` never reaches the JSON
- validation errors are returned **verbatim** to the client, so they learn _which_ parameter
  is wrong — a deliberate exception to rule 4 ("log the real error, return a generic one"),
  which exists to protect internal details, not to withhold input-validation feedback

| Check                                                              | Verified |
| ------------------------------------------------------------------ | -------- |
| Default page: 5 users, `total: 5`, `limit: 50`                     | ✅       |
| `?limit=2` then `?limit=2&offset=2` — next page, no overlap        | ✅       |
| `?limit=2&offset=99` → `"users": []` (**not `null`**), `total: 5`  | ✅       |
| `?limit=abc` / `?limit=0` / `?limit=-1` → 400 naming the parameter | ✅       |
| `?limit=999999` → clamped, response reports `limit: 200`           | ✅       |
| `?active=true` → 5 / `?active=false` → `[]` with **`total: 0`**    | ✅       |
| `?active=maybe` → 400 · CC Owner → 403                             | ✅       |
| No `hashed_password` in any response                               | ✅       |

**Lesson.** `?active=false` returning `total: 0` is the check that matters — it proves
`CountUsers` applies the _same_ filter as `ListUsers`. Had `total` come back `5`, every pager
in the UI would silently show the wrong page count.

**Noted, not acted on:** three handlers now declare a user-shaped response struct (login,
`/me`, list). That is §0's threshold for extracting a shared `userResponse` + mapper — but
the three differ (login carries tokens, list carries `is_active`/`created_on`), so they stay
separate for now. Revisit if a fourth appears.

---

### ✅ Checkpoint 14 — `GET /api/approvers` (endpoint 9 of 22)

Smallest endpoint so far. **Authenticated, not Admin-only** — registration **variation 2**.

**`sql/queries/users.sql` — `ListApprovers :many`**

```sql
SELECT id, full_name FROM users
WHERE role = 'Approver' AND is_active = true
ORDER BY full_name;
```

- **Two columns, not `SELECT *`** — `hashed_password` never enters the result set at all,
  rather than relying on the mapping step to drop it. sqlc generates a narrow
  `ListApproversRow`
- No parameters (the role is hardcoded — this is a dedicated endpoint, not a generic filter)
  and **no pagination**: the approver list is inherently small
- This is the query **`idx_users_role_active` was built for** (checkpoint 1): leading column
  present, and "active approvers" is genuinely selective — unlike `WHERE is_active = ...`
  alone, which can't use that index (no leading column) and which Postgres wouldn't index
  anyway, since a mostly-true boolean is near the worst case for selectivity

**`handlers_users.go` — `HandlerListApprovers`.** Response wrapped as
`{"approvers": [...]}` rather than a bare top-level array, so fields can be added later
without a breaking change.

**Critically: no `requireRole`.** The primary consumer is a **CC Owner** filling in the
Assign Approver field (BRD field 35). Admin-gating this would break the core workflow.

| Check                                                    | Verified |
| -------------------------------------------------------- | -------- |
| **CC Owner** → 200 with the list (the case that matters) | ✅       |
| Admin → 200, same list · Viewer → 200, same list         | ✅       |
| No token → 401                                           | ✅       |

**Note:** BR-8.3.1's segregation of duties is **structural** — one role per user — so a CC
Owner can never also be an Approver, meaning a user physically cannot appear in their own
approver dropdown. No self-filter needed; the schema enforces it.

---

### ✅ Checkpoint 15 — `PUT /api/users/{userID}/active` (endpoint 8 of 22)

Admin-only, variation 3. **First path parameter and first row-level lock.**

**Queries** — `GetUserForUpdate` (`SELECT ... FOR UPDATE`), `SetUserActiveStatus`
(`UPDATE ... RETURNING *`), `ListActiveCCIDsForUser`.

- **`GetUserForUpdate` is deliberately separate from `GetUserByID`.** Same SELECT, opposite
  intent: `GetUserByID` runs in `middlewareAuth` on _every_ request and must **not** lock, or
  every authenticated request would serialize through a row lock. The locking variant is for
  write paths only.
- **`ListActiveCCIDsForUser` returns IDs, not a count.** `len()` serves the guard and the
  same rows populate the 409 body — a separate `COUNT(*)` would be a redundant round trip.
  `sqlc.arg(user_id)` used twice binds **one** parameter to both predicates.

**Handler** — `r.PathValue("userID")` + `uuid.Parse` → 400. `is_active` is a **`*bool`** so an
absent field is distinguishable from an explicit `false`; `{}` is rejected rather than
silently meaning "deactivate". (Same absent-vs-value problem Save Draft solves for real at
endpoint 13 — rehearsed here on one field.)

Order inside the transaction: `GetUserForUpdate` (locks) → **no-op check** → **CC guard** →
update → audit → commit. Self-deactivation is rejected _before_ the transaction opens — no
point locking a row to refuse something already known invalid.

**The no-op path commits, returns 200 with the user, and writes nothing** — no update, no
audit row. Commit rather than rollback: nothing was written, and committing releases the lock
immediately. `audit_logs` records _changes_, and "false → false" isn't one.

| Check                                                                                                         | Verified |
| ------------------------------------------------------------------------------------------------------------- | -------- |
| Deactivate → 200, `is_active: false`                                                                          | ✅       |
| Repeat the same request → 200, **no audit row** (no-op)                                                       | ✅       |
| Reactivate → 200, `is_active: true`                                                                           | ✅       |
| `{}` → 400 · `{"is_active":"yes"}` → 400 · bad UUID → 400 · unknown user → 404                                | ✅       |
| Self-deactivation → 400; self-*re*activation → 200 no-op                                                      | ✅       |
| CC Owner → 403 · no token → 401                                                                               | ✅       |
| **Two audit rows for three state-changing requests** (`UserDeactivated` true→false, `UserUpdated` false→true) | ✅       |
| A deactivated user vanishes from `GET /api/approvers` and is refused at login                                 | ✅       |

**Lesson — `SELECT ... FOR UPDATE`.** A transaction gives **atomicity**, not isolation from
concurrent writers: under Read Committed a plain `SELECT` takes a snapshot and another
session can update that row a millisecond later. `FOR UPDATE` is a _declaration of intent_ —
"I'm reading this because I intend to modify it" — and Postgres holds a **row-level** lock
until the transaction ends. Other `UPDATE`s and `FOR UPDATE` reads on that row **wait**;
plain readers are never blocked, and other rows are unaffected. Without it, two concurrent
deactivations would both read `true`, both decide "this is a change", and both write an audit
row.

**Lesson — sqlc does _not_ validate every column reference.** A typo (`updated_one` in an
`UPDATE ... SET`) generated compiling Go containing broken SQL; it would have failed at
runtime as a 500. **sqlc catches type and struct mismatches; only Postgres catches SQL
correctness.** Run a new query through psql once before wiring the handler.

**Also swept this checkpoint:** every handler's malformed-body branch now returns **400**, not
500 — bad JSON is the client's fault (Blueprint §10).

---

### ✅ Checkpoint 16 — `PUT /api/users/{userID}` (endpoint 7 of 22) — **Group 2 complete**

Admin-only, variation 3. The BR-8.4.11 endpoint, and the last of the Users & Profile group.

**Queries** — `UpdateUserName`, `UpdateUserRole`. **Two separate queries**, not one combined
`UPDATE`, because either may run alone.

**Request** — `full_name` and `role` both `*string`: nil means _"not changing this field"_, so
**absent is distinguishable from empty**. Both nil → 400. Email and password are not accepted
(BR-8.4.9).

**The central lesson — "was it sent?" vs "did it change?"** Computed once after the locking
SELECT:

```go
nameChanged := reqBody.FullName != nil && fullName != current.FullName
roleChanged := reqBody.Role     != nil && role     != current.Role
```

Every subsequent branch keys off these, **not** off `!= nil`. This matters because the UI's
edit row submits **both fields on every save** — so gating the CC guard on mere presence made
it impossible to rename anyone with an open change control (the guard fired on an _unchanged_
role and returned 409). The `!= nil` half is still needed for **validation** ("was it sent? is
it blank?"); the delta is for **action** ("write it? audit it? run the guard?").

**Transaction:** `GetUserForUpdate` (locks) → no-op check → CC guard → updates → audit rows →
commit. Old values are captured before `current` is patched; the response and success log are
built from what the queries **RETURNED**, never from the request locals — otherwise a
role-only update would report `full_name: ""`.

**Two audit rows share one captured timestamp** (BR-8.7.5), with action types split per DB
§8.2: `UserUpdated` for the name, `UserRoleChanged` for the role.

| Check                                                                              | Verified |
| ---------------------------------------------------------------------------------- | -------- |
| Name-only, role-only, both — all 200 with **every** field correct                  | ✅       |
| Exact repeat → 200 no-op, **no audit rows**                                        | ✅       |
| Changed name + **unchanged** role in body → **one** audit row, CC guard skipped    | ✅       |
| Empty body / blank name / invalid role / bad UUID → 400 · unknown user → 404       | ✅       |
| Self-role-change → 400 · self-name-change → 200 · self name + unchanged role → 200 | ✅       |
| CC Owner → 403 · no token → 401                                                    | ✅       |
| **Dual change wrote two audit rows with an identical `created_on`**                | ✅       |

**This is the rehearsal for T2**, where one transition writes a state change, an e-signature
and several audit rows that must all cohere under one timestamp in one transaction.

---

### ✅ Checkpoint 17 — `POST /api/changecontrols` (endpoint 10 of 22)

First business-domain endpoint. **CC Owner only** — FR-6.1.2 rejects Approver, Viewer **and
Admin**; creating change controls is not an administrative function, which is the first time
Admin has been the _wrong_ role.

**`constants.go`** — the six `ck_cc_current_state` values and the four approval statuses.

**`CreateChangeControl` inserts exactly two columns** — `change_owner_id` and
`last_updated_by_id`, both the creating user. The other six fields FR-6.1.4 requires all come
from schema defaults set back in checkpoint 2:

| FR-6.1.4 field                 | Source                                         |
| ------------------------------ | ---------------------------------------------- |
| CC-ID                          | `cc_number_seq` → `GENERATED ALWAYS AS` column |
| Current State                  | `DEFAULT 'Initiated'`                          |
| Change Owner / Last Updated By | the two parameters                             |
| Created On / Last Updated On   | `DEFAULT NOW()`                                |
| Both approval statuses         | `DEFAULT 'Not Submitted'`                      |

**`RETURNING *` is how `cc_id` gets back** — the insert never mentions it. **No Go code
participates in ID generation**, which is exactly what makes it collision-free under
concurrency (DB §8.1).

**The handler takes no request body at all** — the entire input is the authenticated
identity. BRD field 3: `change_owner` is system-generated from the creator and immutable;
US-CC-01 confirms creation opens an _empty_ form, which Save Draft then fills.

**Response is the eight FR-6.1.4 fields plus `id` and the owner's name**, not the full 50-field
record (decision #23).

| Check                                                                  | Verified |
| ---------------------------------------------------------------------- | -------- |
| CC-001 then CC-002 — sequential, `LPAD` guard working                  | ✅       |
| State `Initiated`, both statuses `Not Submitted`, 42 fields NULL       | ✅       |
| Admin → 403, Approver → 403, Viewer → 403                              | ✅       |
| Audit `ChangeControl` / `Created`, `entity_id` = the record's **UUID** | ✅       |

**Flag #15 closed.** With real CCs existing, the two guards written in checkpoints 15–16
finally fired: deactivating and role-changing the CC Owner both returned **409 listing CC-001
and CC-002**, while a name-only change **and a name change with an unchanged role in the
body** both succeeded — the latter being the exact scenario the "was it sent vs did it
change" fix addressed, previously untestable.

**Unplanned proof:** after renaming the CC Owner, the audit rows still read
`performed_by_name: "Default CC Owner"`. That's the denormalized snapshot from DB §2.3 doing
its job — the trail records who acted _at the time_, not who they have since become. A live
join would have silently rewritten history.

---

### ✅ Checkpoint 18 — `GET /api/changecontrols/{ccID}` (endpoint 12 of 22)

Authenticated, **any role** (BR-8.4.7 — the server guards _writes_, not reads; field-level
visibility is the frontend's job per the Security Matrix). First endpoint deliberately open
to all four roles. No transaction, no audit row.

**The query — five joins, two kinds:**

- **`sqlc.embed(cc)`** makes this bearable: the 50 columns arrive as a single nested
  `ChangeControl` field rather than being flattened alongside the five joined names. sqlc
  expands it to explicit column names in the SQL and reassembles on scan
- **`JOIN` for `change_owner_id` and `last_updated_by_id`** — NOT NULL with FK constraints,
  so an inner join provably cannot drop the row, and sqlc emits plain `string` for those names
- **`LEFT JOIN` for the three nullable references.** An inner join there would silently return
  **zero rows** for every CC without an assigned approver — which is every record in
  `Initiated` state. The classic join bug, and it would have broken exactly the records used
  for testing
- Table aliases are mandatory when joining the same table five times

**Lookup is by `cc_id`, the business key** (decision #24).

**`ChangeControlResponse`** — package level in `handlers_cc.go`, **54 fields** (49 columns +
5 joined names; `cc_number` omitted as internal sequence plumbing). Nullable columns are
pointers so they marshal as `null`; **no `omitempty` anywhere** — the form needs every field
present even when empty. Verified mechanically against the schema: no missing or extra
fields, zero type or nullability mismatches. Kept **flat** rather than nesting the user
references, matching the create response (decision #25).

| Check                                                 | Verified |
| ----------------------------------------------------- | -------- |
| CC Owner, Viewer, Admin, Approver — **all 200**       | ✅       |
| `CC-999` and `garbage` → 404 · no token → 401         | ✅       |
| All 40 nullable fields explicitly `null`, not omitted | ✅       |
| The three `LEFT JOIN` name fields `null`, not `""`    | ✅       |

**Flag #9 CLOSED** — open since checkpoint 7. **lib/pq scans a `TIME` column into
`*time.Time` without error.** Verified by setting real values in psql and reading them back
through the API. No sqlc override needed; the schema stays as the DB doc specifies.

---

### ✅ Checkpoint 19 — `GET /api/changecontrols` (endpoint 11 of 22)

Authenticated, any role. **One endpoint serves four screens** — All Change Controls, My
Change Controls, the Approvals queue, dashboard click-throughs — via opt-in client filters.
**No role scoping:** it would be theatre while `GET /{ccID}` is open to everyone (you could
just enumerate CC-001, CC-002…).

**`ListChangeControls` + `CountChangeControls`** with **byte-identical FROM / JOIN / WHERE**,
so `total` describes the same set the page was sliced from. If they drift, every pager in the
UI is silently wrong.

**`WHERE 1=1` + six independent `AND (narg IS NULL OR …)` blocks.** `1=1` is always true, so
every real condition can be uniformly prefixed with `AND` and the first one needs no special
case. Each filter is independently nullable, so **any combination works from one query** —
otherwise it would be a permutation per combination.

| Filter         | URL                          | SQL                                                         |
| -------------- | ---------------------------- | ----------------------------------------------------------- |
| owner          | `?owner=me`                  | `change_owner_id` (uuid)                                    |
| assigned       | `?assigned=me`               | `assigned_approver_id` (uuid)                               |
| state          | `?state=Initiated`           | `current_state` (text)                                      |
| created after  | `?created_after=YYYY-MM-DD`  | `created_on >= $`                                           |
| created before | `?created_before=YYYY-MM-DD` | `created_on < $ + INTERVAL '1 day'`                         |
| search         | `?search=kiosk`              | `ILIKE '%…%'` on `cc_id`, `change_title`, `owner.full_name` |

- **`me` is a flag, not a value.** The UUID comes from the verified token, never the URL, so
  nobody can filter by someone else's identity — there is no `?owner=<uuid>` at all
- **`+ INTERVAL '1 day'` is not cosmetic.** `created_on` is `TIMESTAMPTZ`; a date parses as
  midnight, so without it a CC created at 09:00 on the chosen day is excluded from a range
  the user believes includes it
- **`search` uses `ILIKE`** (case-insensitive) with wildcards **in the SQL**, so the parameter
  stays raw user input. Searching `owner.full_name` works only because the owner join is
  already there for the response
- **Two joins, not five** — INNER for the owner, LEFT for the approver. The summary carries no
  approval-name fields
- `state` is validated against the six constants → an unknown state is a clean 400, not a
  silently empty table

**`ChangeControlSummary` is 10 fields**, matching the table columns — not the 54-field record.
Fewer joins _and_ ~8× less payload (decision #26).

| Check (18 cases)                                                   | Verified |
| ------------------------------------------------------------------ | -------- |
| No params → 2/2, `filtered:false`; `?owner=me` as Admin → 0/0      | ✅       |
| `?state=Draft` → 400; `?created_after=notadate` → 400              | ✅       |
| `?search=cc-001` matches stored `CC-001` — **`ILIKE`, not `LIKE`** | ✅       |
| `?search=Default` → 2 — **matched the joined owner name**          | ✅       |
| **`?created_before=2026-07-30` → 2** — the off-by-one fix          | ✅       |
| Three filters combined → 1 record                                  | ✅       |
| `?limit=1` → **count 1, total 2**; `&offset=1` → the other record  | ✅       |

**Lesson — read the generated params struct.** sqlc assigned `$7` to `offset` and `$8` to
`limit`, so the struct fields are ordered `Offset, Limit` — _not_ the order they appear in the
SQL. Harmless here because the call site uses a keyed struct literal, but passing positionally
would have silently swapped page size and offset.

---

### ✅ Checkpoint 20 — `PUT /api/changecontrols/{ccID}` — Save Draft (endpoint 13 of 22) · **Group 3 complete**

The largest handler in the project. Owner-only, **`Initiated`-only**, partial update across
the **24 fields editable in that state** (not 40 — Group 6's implementation details are
`In Implementation`-only, see flag #18).

**The core problem — three client intents per field:**

| Client sends       | Means           |
| ------------------ | --------------- |
| key absent         | leave unchanged |
| `"field": null`    | clear it        |
| `"field": "value"` | set it          |

A struct of `*string` **cannot distinguish the first two** — a missing key and an explicit
`null` both decode to `nil`. The `*string` pattern from checkpoints 15–16 solved
absent-vs-value, but those columns were `NOT NULL`, so "clear" did not exist as an intent.

**Solution (decision #27): decode into `map[string]json.RawMessage`.** The map's **keys**
answer _"was this sent?"_; the raw bytes are unmarshalled per field afterwards. Two questions,
two sources.

**The params struct is pre-seeded with the record's current values**, because the UPDATE
assigns all 24 columns unconditionally — a field the client did not send must be re-written
as-is or it would be nulled. Each block overwrites only its own entry, so **"leave unchanged"
is the default** and a block is the exception. (This is also why `COALESCE(narg, column)` —
the usual partial-update idiom — is wrong here: it makes NULL mean "keep current", so a field
could never be cleared.)

**Validation by field type:**

| Type        | Count | Checks                                                                                        |
| ----------- | ----- | --------------------------------------------------------------------------------------------- |
| plain text  | 15    | trim · `""` → NULL · max length **in runes**                                                  |
| dropdown    | 6     | as above, plus value checked against the constants                                            |
| date / time | 4     | `*time.Time`, so RFC 3339 validation is free — **`""` is rejected, only `null` clears** (F11) |
| UUID        | 1     | DB lookup: exists, active, holds the **Approver** role                                        |

- **`""` → NULL for text** so "cleared" and "never filled" are the same state; otherwise every
  T2 presence check would need `IS NULL OR = ''` forever
- **Runes, not bytes** — `len(s)` counts bytes, so a pasted em-dash or accented character
  would make a visibly-under-limit field fail
- **`requires_testing` uses ASCII hyphens** — the BRD and the prototypes show en-dashes
- **Approver failure is 400, not 404** — the CC exists and the URL is right; the _body_ names
  a user who doesn't qualify

**Audit scope (decision #28) — only 3 of the 24 fields.** See the _Audited events_ section:
FR-6.6.6 states explicitly that non-critical field edits generate **no** audit entry, naming
Change Description and Business Impact. Approver changes record **names, not UUIDs**.

**A `changed` bool** across all 24 comparisons drives a no-op branch: an unchanged save
commits without running the UPDATE, so `last_updated_on` isn't bumped for an edit that didn't
happen.

**Both paths re-fetch after commit** (decision #29) so the response carries the five joined
user names and matches `GET /{ccID}` exactly.

| Check                                                                                   | Verified |
| --------------------------------------------------------------------------------------- | -------- |
| Absent key survives untouched — the whole mechanism                                     | ✅       |
| `null` and `""` both clear a text field                                                 | ✅       |
| En-dash `requires_testing` → 400; ASCII hyphen → 200                                    | ✅       |
| `"2026-09-15"` (no time) → 400; RFC 3339 → 200                                          | ✅       |
| Self-assignment as approver → 400 on the role check                                     | ✅       |
| Repeat save → no-op, no UPDATE, no audit row                                            | ✅       |
| **`audit_logs` holds `FieldUpdated` rows for the three audited fields only**            | ✅       |
| Approver rows read `null → Default Approver` then `Default Approver → null` — **names** | ✅       |

**Lesson — verify the requirement before designing the mechanism.** Four messages of design
were built on an unchecked assumption that all 24 fields were audited. §6.6.2 is a section
titled _Auditable Events_ and says otherwise. Caught by asking _"what is the audit entry even
for?"_ — the habit that has now caught three specification errors.

**`toChangeControlResponse` extracted** — two real call sites (GET and PUT return identical
bodies), observed rather than predicted. Exactly the §0 threshold, and the reason it was left
inline at checkpoint 18.

**Amended at checkpoint 22 — non-editable keys are now rejected, not ignored (decision #33).**
Because the handler looks up only the 24 known keys, anything else (`decision`,
`current_state`, `actual_implementation_date`) sat unread in the map and the request returned
**200 while writing nothing**. Structurally safe — the params struct cannot carry those
columns — but a poor contract, since a client sending `{"decision":"Approve"}` was told it
succeeded. Now every offending key is **collected** into one 400 (map iteration order is
random, so rejecting one at a time would have meant a round trip per bad key, each naming a
different field), sorted for determinism, and returned as **JSON key names** rather than T2's
human labels: T2 lists form fields a _user_ recognises, this lists keys a _developer_ sent.
`validationErrorResponse` moved to package level — second call site.

---

### ✅ Checkpoint 21 — **T2 submit** · `POST /api/changecontrols/{ccID}/submit` (endpoint 15 of 22)

`Initiated` → `Pending Implementation Approval`. Owner-only, `Initiated`-only. **The hardest
endpoint in the project** — Save Draft was _long_ (one mechanism ×24); T2 is seven distinct
mechanisms in one handler. Written **fully inline** per §0; no helpers extracted yet.

**The two-step contract (decision #30).** BRD §2.1 separates _"Fills all 25 editable fields"_
from _"Clicks Submit for Approval **after all mandatory fields pass validation**"_. So **T2
carries no field values** — the body is `{email, password}` only, and validation runs against
what Save Draft already stored. The update touches exactly four system columns.

**The validation split that makes this coherent:**

|                               | Save Draft                           | T2                                      |
| ----------------------------- | ------------------------------------ | --------------------------------------- |
| Format — length, enums, types | ✅                                   | ❌ (impossible values can't be stored)  |
| Presence                      | ❌ (a draft is incomplete by nature) | ✅ 20 fields                            |
| Business rules (dates)        | ❌                                   | ✅ time-relative, so only checkable now |
| Signature                     | ❌                                   | ✅                                      |

**Order is specified, not preferred** — FR-6.2.34: _"the signature prompt shall appear only
after all mandatory field and date validations have passed."_ So presence → dates →
signature. A validation failure never touches the audit trail; only a _credential_ failure
does, because that is a security event.

**Presence: 20 fields, collect-all** (BR-8.2.6) — 7–14, 17–23, 25–28, 35. Not required: 15,
16 (windows), 24 (file), 36, 49. Returned as human labels in one `issues` array;
`respondWithError` can't carry a list, so this needs its own response struct.

**Dates: `businessDaysFrom(start, n)`** — weekdays only, **no public holidays in Phase 1**.
Both rules append to the _same_ issues list, so a user with two bad dates learns both at once.
`.Before(earliest)` covers "at least N business days" **and** "must be in the future" in one
comparison. Covered by `helpers_test.go` — a table of 12 cases plus a property test asserting
the result is never a weekend across 40 iterations.

**Signature (BR-8.8.2/3):** both credentials required; the email must match the **logged-in
user** via `EqualFold` — rejected even when the credentials are perfectly valid for someone
else; then argon2id. Password never logged; both failure modes return one generic message so
an attacker can't tell which half failed.

**The failure path (decision #31)** — `SignatureFailed` is written with **`cfg.db`, outside
the transaction**, so it survives the rollback (FR-6.2.31). The single deliberate exception to
rule 7, and worth its comment: written with `qtx` the audit row would vanish with everything
else, leaving no record of the attempt.

**The transaction (BR-8.8.6)** — one `now` shared by all of it: update → `esignatures` row →
`StateChanged` audit → `SignatureCaptured` audit → **re-fetch** → commit.

| Check                                                                                          | Verified |
| ---------------------------------------------------------------------------------------------- | -------- |
| Empty CC → 400 listing **all 20** in field order; a partly-filled CC lists only the 15 missing | ✅       |
| Zero `SignatureFailed` rows after a validation failure — the signature is never reached        | ✅       |
| Proposed date one day early → 400; **exactly on the boundary → passes** (off-by-one)           | ✅       |
| Both dates stale → **400 with both messages** — collect-all on business rules                  | ✅       |
| Wrong password → 401 · **valid credentials for another user → 401** (BR-8.8.3)                 | ✅       |
| Blank credential → 400, and **no** `SignatureFailed` row (fails before the check)              | ✅       |
| After 3 failed signatures: **3 audit rows survived, `current_state` still `Initiated`**        | ✅       |
| `OWNER@EAQMS.LOCAL` accepted — `EqualFold`                                                     | ✅       |
| **`esignatures.signed_on` and both audit `created_on` byte-identical** (FR-6.6.5)              | ✅       |
| `StateChanged` carries `current_state: Initiated → Pending Implementation Approval`            | ✅       |
| Re-submit → 409 · Admin → 403 · unknown CC → 404 · no token → 401                              | ✅       |
| **`PUT /CC-002` → 409** — closes Save Draft's state gate, untestable since checkpoint 20       | ✅       |

**Lesson — the re-fetch moved inside the transaction (decision #32).** Save Draft originally
re-fetched _after_ the commit. For T2 that would be worse: a failed read after a committed
transition means the user sees a 500, retries, and hits a **409** because the state has
already moved. Reading before the commit means any failure leaves nothing written — **the
error and the record's state agree.** Applied to Save Draft as well.

---

### ✅ Checkpoint 22 — **T3 cancel** · `POST /api/changecontrols/{ccID}/cancel` (endpoint 16 of 22)

`Initiated` → **`Cancelled`**. Owner-only, `Initiated`-only. **Terminal** — BRD §2.2: no
editable fields, no actions, permanent.

Reuses T2's shape almost entirely. What differs:

|                     | T2                                      | T3                                                        |
| ------------------- | --------------------------------------- | --------------------------------------------------------- |
| Body                | `{email, password}`                     | `{cancellation_reason, email, password}`                  |
| Presence validation | 20 fields                               | **none** — a draft may be abandoned at any completeness   |
| Writes              | state + impl status                     | state + **both** statuses → `N/A` + `cancellation_reason` |
| Audit rows          | 2                                       | **3** — plus `FieldUpdated` for the reason                |
| Signature meaning   | `Submitted for Implementation Approval` | `Cancelled`                                               |
| Notification        | unconditional                           | **conditional** on an approver having been assigned       |

**`cancellation_reason` is written here and nowhere else.** Save Draft deliberately excludes
it — the field reference specifies _"captured via cancellation modal only — never an inline
form field"_, and it is _"permanently read-only once saved"_. Mandatory, max 500 runes,
whitespace-only rejected. It **is** audited (BRD SC-5 lists it among critical field changes),
with `old_value` always null since nothing could have written it before.

**The notification is conditional** (`cc.AssignedApproverID != nil`) because §2.2 says the
approver is notified _"if previously assigned"_ — and T3 requires no approver, so a
half-filled draft may have none. At T2 the check is unnecessary: presence validation
guarantees one exists.

| Check                                                                                                           | Verified |
| --------------------------------------------------------------------------------------------------------------- | -------- |
| 404 / 403 / 409 / 401 gates                                                                                     | ✅       |
| Blank, whitespace-only and 501-rune reasons → 400; **500 runes passes**                                         | ✅       |
| Wrong password → 401 · valid credentials for another user → 401                                                 | ✅       |
| Two `SignatureFailed` rows written, record still `Initiated`                                                    | ✅       |
| Success: both statuses `N/A`, reason stored                                                                     | ✅       |
| **Four rows across two tables sharing one timestamp**                                                           | ✅       |
| Audit reads standalone: `StateChanged` (Initiated→Cancelled), `FieldUpdated` (null→reason), `SignatureCaptured` | ✅       |
| **`Cancelled` is terminal** — cancel, save draft and submit all 409 afterwards (SC-7)                           | ✅       |

**Incidental confirmation:** a Save Draft against the cancelled record hit the
**non-editable-key check before the state check** — the key validation runs before the
transaction opens, so the cheapest rejection happens first.

**The signature block is now duplicated verbatim twice.** After T4/T5 it will be three, which
is §0's threshold — `verifySignature` gets extracted at checkpoint 23, with three real
examples rather than one guess.

---

## Next

### ⬜ Checkpoint 23 — **T4 / T5 decision** (endpoint 17)

`POST /api/changecontrols/{ccID}/decision` — **one endpoint, two outcomes**:

|                | From                  | To                    | `implementation_approval_status` |
| -------------- | --------------------- | --------------------- | -------------------------------- |
| **T4 approve** | Pending Impl Approval | **In Implementation** | `Approved`                       |
| **T5 reject**  | Pending Impl Approval | **Initiated**         | ? — needs deciding               |

**First endpoint restricted to the Approver**, and to the _assigned_ one — the check is
`cc.AssignedApproverID == user.ID`, not a role check, so ownership logic mirrors T2/T3's
owner check rather than using `requireRole`.

**Three audited fields** (§6.6.2): `decision`, `risk_level`, `decision_comments`. So a single
T4 writes **five** audit rows — three `FieldUpdated`, plus `StateChanged` and
`SignatureCaptured` — all on one timestamp. The most rows any action produces.

**Rejection does NOT populate `implementation_approval_by_id` / `_on`** (per the API plan) —
those record who _approved_, and a rejection is not an approval.

**Decisions needed before writing:**

1. What does `implementation_approval_status` become on **rejection**? Back to `Not Submitted`,
   or stay `Pending`? Nothing states it. `Not Submitted` seems right — the record is a draft
   again and must be resubmitted.
2. Is `decision_comments` mandatory on rejection? A rejection without a reason is unhelpful,
   but the field reference marks it optional.
3. `risk_level` — mandatory at T4/T5 or only on approval?

**Then extract** (§0's threshold, three transitions written inline): `verifySignature` is the
obvious first candidate — the block is already verbatim-identical across T2 and T3. The audit
row construction is the second.

---

## Audited events — the authoritative list (BRD §6.6.2)

Checked against the BRD, not assumed. **FR-6.6.6 is explicit that non-critical field changes
— it names editing Change Description and Business Impact — generate NO audit entries.**
The trail records compliance-relevant decisions, not typing.

**Change control, record-level**

| Event                  | Endpoint | Action type    |
| ---------------------- | -------- | -------------- |
| CC creation            | 10 ✅    | `Created`      |
| Every state transition | T2–T8    | `StateChanged` |

**Change control, critical field updates — these nine fields only**

| #   | Field                          | Set at     | Audited in |
| --- | ------------------------------ | ---------- | ---------- |
| 13  | `proposed_implementation_date` | Save Draft | **13**     |
| 14  | `target_closure_date`          | Save Draft | **13**     |
| 35  | `assigned_approver_id`         | Save Draft | **13**     |
| 37  | `decision`                     | T4/T5      | 17         |
| 38  | `risk_level`                   | T4/T5      | 17         |
| 39  | `decision_comments`            | T4/T5      | 17         |
| 42  | `final_decision`               | T7/T8      | 19         |
| 43  | `final_comments`               | T7/T8      | 19         |
| 50  | `cancellation_reason`          | T3         | 16         |

**So of Save Draft's 24 editable fields, only 3 are audited.** The other 21 write no audit row
at all.

**User management** (all built) — `UserAdded` · `UserRoleChanged` · `UserDeactivated` ·
full-name change as `UserUpdated`.

**E-signatures** (T2–T8) — success writes `SignatureCaptured` **and** an `esignatures` row in
one transaction; failure writes `SignatureFailed` to the audit only, outside any transaction.

**FR-6.6.5:** each critical field change is a **separate** audit entry, and **multiple entries
from one action share one timestamp**.

---

## Guardrail docs pending amendment

Session decisions that now contradict a guardrail doc. Until amended, a future session may
"correct" a deliberate choice back.

| Doc                           | Change needed                                                                                                                                                                                                                                                                                                                                                                                                       |
| ----------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `API_ENDPOINT_PLAN.md`        | Endpoint 2: sliding window **30 min → 2 hours** (decision #15)                                                                                                                                                                                                                                                                                                                                                      |
| `API_ENDPOINT_PLAN.md`        | Endpoint 11: pagination is **`?limit=`/`?offset=`**, not `?page=`/`?page_size=` — one convention across the API, matching `GET /api/users`. Also add the two filters built beyond the plan's list: **`?search=`** (forced by pagination — client-side search can only see the current page) and **`?created_after=`/`?created_before=`**. Search is table filtering, not the reporting/analytics excluded by §1.3.2 |
| `CC_Field_Reference.md`       | Add 🔒 **Audit-tracked** to fields **13** and **14** — BRD §6.6.2 lists both, the field reference omits them (flag #19)                                                                                                                                                                                                                                                                                             |
| BRD **BR-8.4.11** scope note  | Remove _"the name change can still be saved"_ — a blocked role change now saves **nothing** (decision #22)                                                                                                                                                                                                                                                                                                          |
| DB Design **§8.2** scope note | Remove _"a name change on the same request must still succeed... the handler applies the name update regardless"_ — same override (decision #22)                                                                                                                                                                                                                                                                    |
| `CONTEXT_HANDOFF.md`          | §3 mentions the _30-minute_ sliding inactivity window → 2 hours                                                                                                                                                                                                                                                                                                                                                     |
| BRD **SC-6**                  | Remove or reword _"Target Closure Date is locked after initial submission"_ — it contradicts field reference #14, and the implementation follows the field reference (flag #23)                                                                                                                                                                                                                                     |
| BRD                           | Add that **deactivation is blocked while a user has active CC records** (decision #19), mirroring the existing role-change restriction in §2.2 / US-AD-03; add the **password policy** (decision #17 — 8 chars, 1 upper/lower/digit/special); add the frontend refresh-timer requirement (flag #12); check for any session-timeout statement; §13.1 deferral note for the three descoped password flows (flag #5)   |
| DB Design doc                 | `change_controls` column count 48 → **50** (flag #1); DEFAULT count 8 → **7** (flag #2)                                                                                                                                                                                                                                                                                                                             |

---

## Session decisions not in any guardrail doc

Settled in working sessions and binding. They exist nowhere else.

| #   | Decision                                                                                                                                                                                                                                                                                                   | Rationale                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               |
| --- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1   | **Module path `github.com/lain-the-coder/ea-qms-backend`** — not `-cc-backend`                                                                                                                                                                                                                             | Future QMS modules (Deviation, CAPA) live under the same module rather than forcing a second repo                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                       |
| 2   | **Constraint/index naming follows §5.1 and §6.1 verbatim, including their abbreviations** — `ck_cc_*`, `idx_cc_*`, `idx_audit_*` short; CHECKs full (`ck_audit_logs_*`, `ck_esignatures_*`)                                                                                                                | §5.1/§6.1 are definitions, cross-referenced by name elsewhere (§8.2 cites `idx_cc_owner_state`); §1.3 is a convention statement with one stale example. Also keeps names clear of Postgres's 63-byte identifier truncation                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                              |
| 3   | **Two naming exceptions kept verbatim:** `uq_change_controls_cc_id` and `ck_cc_post_impl_issues`                                                                                                                                                                                                           | Spelled that way in §3.2/§5.1 and §6.1 — do not "regularize" them                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                       |
| 4   | **FK constraints use the long form** — `fk_<table>_<column>`                                                                                                                                                                                                                                               | §4 lists all eleven FKs but never names the constraints, so §1.3 stands unopposed here. The name is what appears in the Postgres error you map to a 409 (Blueprint §11)                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                 |
| 5   | **PostgreSQL 14.23 accepted** (doc §1.2 specifies 15+)                                                                                                                                                                                                                                                     | Every needed feature traced and predates 14: `gen_random_uuid()` core (13), `GENERATED ALWAYS AS ... STORED` (12), `ON CONFLICT DO UPDATE` (9.5), `SELECT ... FOR UPDATE`, functional/composite indexes                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                 |
| 6   | **`log.Fatal(server.ListenAndServe())`**, not a bare call                                                                                                                                                                                                                                                  | A discarded error means a bind failure exits silently with status 0 and no message                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                      |
| 7   | **Goose run as a global CLI from `sql/schema`**                                                                                                                                                                                                                                                            | Keeps migration files free of Go wiring                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                 |
| 8   | **Uniqueness form rule:** plain columns → table `CONSTRAINT ... UNIQUE`; expressions or partials → `CREATE UNIQUE INDEX`                                                                                                                                                                                   | A `UNIQUE` table constraint accepts only a column list, so `uq_users_email` on `LOWER(email)` _must_ be an index. Constraints are preferred otherwise: `ON CONFLICT ON CONSTRAINT <name>`, visibility in `information_schema.table_constraints`, and Postgres's own recommendation                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                      |
| 9   | **DBeaver is connected for browsing only.** All schema changes go through goose                                                                                                                                                                                                                            | Applied migrations are the schema's only description (Blueprint §13). DBeaver also splits `\d` across tabs and blurs the constraint-vs-index distinction from #8 — **DBeaver to navigate, psql to verify**                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                              |
| 10  | **Nullable columns are forced to Go pointers via explicit sqlc `db_type` overrides, keeping `lib/pq`**                                                                                                                                                                                                     | **Resolves a real contradiction between Blueprint §2 and §4.** sqlc's `emit_pointers_for_null_types` is _silently ignored_ unless `sql_package` is `pgx/v4` or `pgx/v5` — so §2 (lib/pq, deliberate) and §4 (pointers) cannot both hold as written. Rejected: switching to pgx (abandons §2's reasoning and changes the `BeginTx`/`WithTx` shape) and accepting `sql.NullXxx` (pays every cost §4 argued against — garbage JSON, a ×40 mapping loop, hand-rolled three-state draft logic). Five overrides give both. **The `db_type` spellings are not uniform and were found empirically: `text`, `timestamptz`, `date`, `uuid` bare; `time` and `bool` require the `pg_catalog.` prefix.** (`bool` was added at checkpoint 13, when the first nullable boolean _parameter_ appeared — no nullable boolean column exists in the schema.) Also: omit the `package` key when the import path already ends in the package name, or sqlc emits duplicate imports and the build fails                                                                                       |
| 11  | **Password hashing uses `github.com/alexedwards/argon2id`, not raw `golang.org/x/crypto/argon2`**                                                                                                                                                                                                          | Blueprint §2 names the algorithm (argon2id), not a package, so the choice was open. The library already does PHC-string encoding, `crypto/rand` salting, parameter round-tripping and constant-time comparison — a reviewed implementation rather than hand-rolled crypto plumbing. Params are set **explicitly** (not `DefaultParams` in app code) so a library-default change can't silently alter hashing strength, and so the values are auditable                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                  |
| 12  | **Data delivered to handlers by two different mechanisms, chosen by failure mode: request logger via `context`, authenticated user via explicit argument**                                                                                                                                                 | Fills the Blueprint §15 logging gap. The rule: _match the delivery mechanism to what happens when the thing is missing._ A missing logger is harmless → `context` value with a `slog.Default()` fallback (`LoggerFrom`). A missing authenticated user is a security hole → explicit third argument on an `authedHandler` type, so forgetting auth is a **compile error**, not a runtime surprise — the compiler becomes an auth control, which matters for a regulated system. Not inconsistency: same principle, opposite stakes. (Considered and rejected: context for both, for surface consistency — it would trade a compile-time guarantee for a per-route discipline across 22 routes.) Logging is minimal per §0/§15: request ID + start/finish + errors; runtime level-filtering deferred (slog provides the levels regardless)                                                                                                                                                                                                                                |
| 13  | **Refresh token absolute expiry = 24 hours** (`refreshTokenTTL`); access token = 30 minutes (`accessTokenTTL`)                                                                                                                                                                                             | The 30-min _sliding_ window on `updated_on` is specified by the guardrails; the **absolute** cap is not, so it was chosen here. Two clocks cover two threats: the sliding window logs out a walked-away session, but it cannot stop an _active_ attacker — a stolen token refreshed every 29 minutes would live forever, because the window measures inactivity. `expires_at` is stamped once at login and never moves, so a leaked token dies within 24 h regardless. 24 h covers any shift pattern while bounding exposure to a single day. Rejected: Chirpy's 60 days (far too long for a regulated system)                                                                                                                                                                                                                                                                                                                                                                                                                                                          |
| 14  | **API testing via a committed Postman collection; no swagger annotations in code**                                                                                                                                                                                                                         | `swaggo/swag` parses the AST and **cannot resolve types declared inside function bodies** — adopting it would have forced every request/response struct out of its handler across all 22 endpoints, plus ~10 annotation lines each and a `swag init` step that silently serves stale docs when forgotten. Neither swagger nor OpenAPI appears in any guardrail doc, so this would have been a second consciously-added scope item after logging. Postman gives the practical benefit (repeatable requests, auto-captured token, saved examples) at a fraction of the cost, and **exports to OpenAPI 3.0 later** if a formal spec is ever required — good request names and saved examples are what make that export usable, so both are done as we go. Swagger UI can render such a spec externally without any code change                                                                                                                                                                                                                                             |
| 15  | **`refreshInactivityWindow = 2 hours`, deliberately decoupled from the 30-minute access token.** This **overrides** `API_ENDPOINT_PLAN.md` endpoint 2, which specifies 30 minutes — the doc must be amended                                                                                                | `updated_on` is set at login and at every refresh, and a JWT is minted at exactly those same moments, so **JWT expiry ≡ `updated_on` + 30 min**. With both windows at 30 minutes the idle check fires at precisely the instant the JWT dies, meaning refresh can only ever succeed while the caller still holds a _valid_ JWT — the sliding window adds nothing beyond the JWT's own expiry, and session continuity rests entirely on the frontend timer never missing a beat. At 2 hours the two clocks do different jobs: the 30-min JWT bounds **credential exposure**, the 2-hour window bounds **unattended session length**, and a client that misses a refresh has real room to recover (making the 401-interceptor pattern a genuine fallback rather than theatre). The security cost is small because the idle window is the weakest of four controls — the short JWT limits exposure, `expires_at` caps the session at 24 h absolutely, and `middlewareAuth` re-checks `is_active` on every request. **Changes one constant; no schema change, no migration** |
| 16  | **Refresh/revoke contract:** refresh token travels in the **JSON body**, not the `Authorization` header · revoke is **idempotent** (204 regardless) · `RevokeRefreshToken` carries `AND revoked_at IS NULL` · `revoked_at` is **not** set when a token merely expires · refresh tokens are **not rotated** | _Body not header:_ `Authorization` conventionally carries the _access_ token; one header meaning two different things per endpoint is ambiguous. _Idempotent:_ logout must never fail, and identical responses for "token existed" and "didn't" avoid an information leak (same reasoning as login's byte-identical 401s). _`revoked_at IS NULL` clause:_ a repeat revoke then preserves the **original** logout timestamp instead of overwriting it — better audit fidelity. _Not set on expiry:_ `revoked_at` records a deliberate **act** (logout); expiry is the passage of time, and conflating them would lose that distinction while adding a write on a rejection path for no behavioural change. _No rotation:_ real practice, but unspecified, complicates the client, and outside documented scope                                                                                                                                                                                                                                                           |
| 17  | **Password policy: minimum 8 characters, with at least 1 lowercase, 1 uppercase, 1 digit and 1 special character.** Enforced in `validatePassword` (`helpers.go`) with **collect-all** reporting                                                                                                           | **No guardrail doc specifies a password policy** — this is a genuine spec gap being filled, and needs adding to the BRD (see pending-amendments). Without it `"a"` would have been accepted. Collect-all (every unmet rule in one message) rather than fail-first, matching BR-8.2.6's pattern for transition validation, so a user fixes everything in one pass. The log records only the _count_ of unmet rules, never which ones — knowing "had lowercase, no digits" is a weak hint about a password that may be retried with a small variation. An earlier draft used 4-of-each, which implies a 16-character minimum; relaxed to 1-of-each as the conventional baseline. **Passwords are never trimmed** anywhere — leading/trailing spaces are legitimate characters, and trimming at creation but not at login would silently create unusable accounts                                                                                                                                                                                                          |
| 18  | **Writes and their audit rows are atomic — one transaction, all or nothing**                                                                                                                                                                                                                               | BR-8.4.9 requires all user-management actions to be logged. If the audit insert can fail while the write succeeds, the system can hold a change with no audit trail — unacceptable in a GxP context, where an unaudited change effectively did not happen. So `HandlerCreateUser` wraps the user insert and the `UserAdded` row in one transaction, introducing the `BeginTx`/`qtx` pattern earlier than the build order planned. Same pattern the transitions need for BR-8.8.6 (state change + signature + audit as one unit), so the rehearsal is on a small handler rather than T2                                                                                                                                                                                                                                                                                                                                                                                                                                                                                  |
| 19  | **Deactivation is blocked while the user owns or is assigned to any CC not in `Closed`/`Cancelled`** → 409 listing the blocking CC-IDs. Reactivation skips the check entirely                                                                                                                              | **Fills a gap in the BRD, which restricts only _role_ changes** (§2.2, BR-8.4.11) and says nothing about deactivation — yet the harm is identical: a deactivated approver cannot log in, so their assigned CCs sit in `Pending Implementation Approval` with nobody able to action them, and **no reassignment endpoint exists** to recover. Not scope creep but closing an inconsistency in the requirements; the BRD needs amending (see pending-amendments). Reactivation can only _unblock_ stranded records, so the guard is deactivation-only                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                     |
| 20  | **`PUT /api/users/{userID}/active` details:** audit action type by direction (`UserDeactivated` true→false, `UserUpdated` with `field_name`/`old`/`new` false→true) · self-**de**activation blocked · a no-op writes **no** audit row · 200 with the user on both paths                                    | _Action types:_ `ck_audit_logs_action_type` has no `UserReactivated`, so the purpose-built value is used where it exists and the generic one where it doesn't; the field/old/new columns carry the meaning precisely. _Self-guard:_ the last Admin locking themselves out is unrecoverable through the API; self-*re*activation needs no guard (it's impossible to reach). _No-op:_ `audit_logs` records changes, and "false → false" isn't one — the branch commits (releasing the lock) rather than rolling back. _200 with the user:_ the `SELECT` is required anyway for the 404 check, the audit's old value and the no-op detection, so returning the object costs nothing and saves the frontend a re-fetch                                                                                                                                                                                                                                                                                                                                                      |
| 21  | **An Admin may change their own _name_ but not their own _role_** (`PUT /api/users/{userID}` — checkpoint 16)                                                                                                                                                                                              | Same lockout footgun as self-deactivation (decision #20): an Admin demoting themselves to Viewer loses user-management access, and if they are the last Admin nobody can restore it — unrecoverable through the API. Blocking only the _role_ half is the minimum guard: the name change is harmless and the prototype's edit row offers both fields together. Note the prototype goes further and omits the pencil entirely for `(you)`; the API is deliberately less restrictive, since a self-name-change is legitimate and the API must not depend on UI discipline for the part that matters                                                                                                                                                                                                                                                                                                                                                                                                                                                                       |
| 22  | **A blocked role change saves _nothing_ — the request is all-or-nothing.** The CC guard runs **before** any write                                                                                                                                                                                          | **Overrides two guardrail documents**, unlike every other decision so far: BR-8.4.11's scope note and **DB Design §8.2** both specify that _"a name change on the same request must still succeed... the handler applies the name update regardless and gates only the role update behind the active-record check."_ That design means a 409 response whose transaction **commits**, which then requires the 409 body to report what was saved so the UI can update the row, and a banner reading "the name change has been saved". All-or-nothing removes that whole branch of complexity. **The practical loss is small:** a name-only edit is unaffected (`roleChanged` is false, so the guard never runs), and the only case that changes is "changed both, role blocked" — where the Admin simply retries with the name alone. **Both documents need amending** (see pending-amendments), and this also retires frontend note F3                                                                                                                                   |
| 23  | **`POST /api/changecontrols` returns only the eight FR-6.1.4 system fields (plus `id` and the owner's name), not the full 50-field record.** User names accompany every user ID in CC responses; both are returned                                                                                         | _Minimal response:_ at creation the other 42 fields are null and the client just created the record, so it can render the empty form with **no second round trip**. The full mapper is then built at endpoint 12 where it is genuinely required, rather than speculatively here (§0). _Names alongside IDs:_ the frontend **cannot** resolve a UUID to a name on its own — a per-record `GET /api/users/{id}` would be N+1 on a list, and caching all users is impossible for a Viewer (that endpoint is Admin-only). _Both, not either:_ **ID for comparisons** (`cc.change_owner_id === currentUser.id` decides whether Save Draft renders — names are unstable, since two people can share one and this API can change them) and **name for display**                                                                                                                                                                                                                                                                                                                |
| 24  | **CC endpoints are addressed by the business key `cc_id` (`/api/changecontrols/CC-001`), not the UUID**                                                                                                                                                                                                    | The API plan writes `{ccID}` and the field reference names field 1 `cc_id`, both pointing at the business key. It is what users say out loud, it makes logs and Postman requests readable, and `uq_change_controls_cc_id` exists to serve exactly that lookup. The usual objection to business keys — that they change — does not apply: field 1 is **immutable after creation**. Bonus: **no format validation is needed**, since a malformed id simply matches nothing and returns 404. Applies to endpoints 12, 13, all seven transitions, files and signatures                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                      |
| 25  | **`ChangeControlResponse` is flat, not nested** — `change_owner_id` + `change_owner_name` rather than `change_owner: {id, full_name}`                                                                                                                                                                      | Nesting reads better with five user references on one record, and was specced that way first — then reversed deliberately: release 1 optimises for **finishing correctly**, not for readability polish that can be applied later. Flat also matches the create response, so both CC endpoints return the same shape for user references; nesting one and not the other would have been the worse inconsistency. **Both id and name are returned** for each reference (decision #23's reasoning): id for comparisons, name for display                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   |
| 26  | **The list returns a 10-field `ChangeControlSummary`, not the 54-field record**                                                                                                                                                                                                                            | The tables display seven columns; returning 54 fields × 20 rows to render seven is ~8× the payload for nothing. Counter-intuitively the summary is also **less work**: it needs **two** joins instead of five, since it carries no approval-name fields. This is why the full mapper was deliberately left inline at checkpoint 18 rather than extracted — the assumption that three endpoints would share one shape turned out to be wrong                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                             |
| 27  | **Save Draft decodes the body into `map[string]json.RawMessage`, not a struct**                                                                                                                                                                                                                            | Three client intents must be distinguished per field — **absent** (leave unchanged), **null** (clear), **value** (set) — and a struct of `*string` collapses the first two into `nil`. The map's **keys** answer "was this sent?" while the raw bytes carry the value, decoded per field afterwards. This is RFC 7386 JSON-merge-patch semantics, the production standard for partial updates. **Rejected: requiring the client to always send all 24 fields**, which would remove the ambiguity but rests on a _promise_ — one partial body from a bug, a retry or a second client would silently wipe the omitted fields. **Bonus:** any key not among the 24 known ones is ignored entirely, so the endpoint physically cannot write a field it wasn't built for. Corollary: the params struct is **pre-seeded with current values**, since the UPDATE assigns every column unconditionally — and `COALESCE(narg, column)`, the usual idiom, is wrong here because it makes NULL mean "keep current"                                                                 |
| 28  | **Only 3 of Save Draft's 24 fields are audited; approver changes record names, not UUIDs**                                                                                                                                                                                                                 | **BRD §6.6.2** lists the auditable fields and **FR-6.6.6** states explicitly that non-critical field edits generate no audit entry — naming _Change Description_ and _Business Impact_, both Save Draft fields. So `proposed_implementation_date`, `target_closure_date` and `assign_approver` are audited; the other 21 write nothing. **Names over UUIDs** because the trail is read by humans during inspection — `4bba81a9-…` tells an auditor nothing, `Default Approver` tells them everything — and the system already denormalises `performed_by_name` for exactly this reason (DB §2.3). Cost: one extra lookup for the _previous_ approver's name, only on an actual reassignment                                                                                                                                                                                                                                                                                                                                                                             |
| 29  | **Save Draft re-fetches the record rather than building the response from `RETURNING *`** — _(the after-commit placement is superseded by #32; the re-fetch itself stands)_                                                                                                                                | The response must carry the **five joined user names**, which neither `GetChangeControlForUpdate` nor the UPDATE's `RETURNING *` provides. Building them without a re-fetch would mean reasoning about which invariants hold in `Initiated` — and one does not: **T5 (reject) returns a CC to `Initiated` with the _approver_ as `last_updated_by`**, so assuming it is the current user would be wrong. The re-fetch also makes Save Draft and `GET /{ccID}` return **identical** bodies. Accepted cost: if the re-fetch fails the save has already committed, so the client sees a 500 for work that was saved — logged distinctly as _"save draft succeeded but re-fetch failed"_, and self-healing, since a retry takes the no-op branch                                                                                                                                                                                                                                                                                                                            |
| 30  | **T2 carries no field values — the contract is save-then-submit**                                                                                                                                                                                                                                          | BRD §2.1 separates _"Fills all 25 editable fields"_ (Save Draft) from _"Clicks Submit for Approval **after all mandatory fields pass validation**"_ (T2). So the body is `{email, password}` only and validation runs against what is **stored**. This also produces a clean division of labour: Save Draft validates **format** (a malformed value can never reach the database, so T2 need not re-check it); T2 validates **presence** (a draft is incomplete by nature, so Save Draft cannot demand it) and **business rules** (time-relative — a date valid on Monday may be invalid on Thursday, so it can only be checked at the moment of submission). Consequence: the frontend **must** save before submitting, or T2 validates stale data and rejects fields the user can see filled in on screen (F12)                                                                                                                                                                                                                                                       |
| 31  | **A failed signature writes its audit row with `cfg.db`, outside the transaction**                                                                                                                                                                                                                         | FR-6.2.31 requires the failed attempt recorded _and_ the record left untouched. Written with `qtx` the row would be discarded by the same rollback that reverts the transition, leaving no evidence — a compliance failure with no visible symptom. This is the **single deliberate exception to rule 7** ("inside a transaction use `qtx`, never `cfg.db`") and carries a comment saying so, or a future reader will "fix" it. Verified: three failed signature attempts left three audit rows while `current_state` stayed `Initiated`                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                |
| 32  | **The response re-fetch happens INSIDE the transaction, before the commit** — supersedes the second half of #29                                                                                                                                                                                            | Reading after the commit leaves a window where the write succeeded but the client sees a 500. For Save Draft that was tolerable (a retry is a harmless no-op); **for a transition it is not** — the retry hits a **409**, because the state has already moved, so the user is told "something went wrong" and then "this isn't in Initiated state." Reading before the commit means any failure leaves nothing written: **the error and the record's state agree.** A transaction reads its own uncommitted writes, so the joined read still returns post-update values. Applied to Save Draft as well                                                                                                                                                                                                                                                                                                                                                                                                                                                                  |
| 33  | **Save Draft rejects request keys that are not editable in the current state, rather than silently ignoring them**                                                                                                                                                                                         | Decoding into a map means unknown keys are simply never looked up — safe, and the conventional behaviour for JSON APIs, but it produced a **200 for a request that wrote nothing**. In a regulated system "the API said OK and did nothing" is worse than a 400. Applied **only to Save Draft**: it has 24 known keys out of 50 possible CC columns, so a client can plausibly believe `decision` or `actual_implementation_date` applied. The 2–4 field bodies elsewhere (login, transitions) are not confusable, so `DisallowUnknownFields` was deliberately **not** added to them — strictness there would buy nothing and break on a harmless extra field. Cost: `draftEditableFields` must stay in sync with the 24 blocks; drift fails loudly (a valid field gets rejected), which is the safe direction                                                                                                                                                                                                                                                          |

---

## Frontend contract notes

Obligations that fall on the **frontend**, not the backend. Collected here so they survive
into the UI build / handover rather than being buried in the flags table.

| #   | Note                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                    | Source                                             |
| --- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------- |
| F1  | **Refresh proactively on a timer** at ~24 min (≈80 % of the 30-min access token), plus a 401-interceptor that refreshes and retries once as a safety net. The backend contract assumes this; without it users hit surprise logouts                                                                                                                                                                                                                                                                                                                                                                                                                      | flag #12, decision #15                             |
| F2  | **Normalize en-dashes to ASCII hyphens** before submitting. The HTML prototypes' `<option value="...">` entries contain en-dashes; `ck_cc_requires_testing` and `ck_esignatures_meaning` are live and will reject them on every submit. Either normalize at the API boundary or fix the prototypes                                                                                                                                                                                                                                                                                                                                                      | flag #4, DB §6.5                                   |
| F3  | **Rewrite the blocked-role-change banner.** The prototype reads _"…These records must be Closed or Cancelled before the role can be changed. **The name change can still be saved.**"_ — that last sentence must be **deleted**. Under decision #22 a blocked role change saves **nothing**, so the banner should state only that the role change was blocked and list the offending CC-IDs from `blocked_cc_ids`                                                                                                                                                                                                                                       | `settings-admin.html`, decision #22                |
| F4  | **Disable the active/inactive toggle for the current user's own row.** The API returns 400 for self-deactivation, so leaving it enabled produces an error the UI could have prevented. The prototype already omits the pencil icon for `(you)` but leaves the toggle live                                                                                                                                                                                                                                                                                                                                                                               | `settings-admin.html`, decision #20                |
| F5  | **Send `?limit=` explicitly.** The prototype's pager shows 20 per page; the API's default is 50 (max 200). The response echoes `limit`/`offset`/`total` back for the pager                                                                                                                                                                                                                                                                                                                                                                                                                                                                              | checkpoint 13                                      |
| F6  | **Two calls, not one, on the user-management screen.** The pencil → edit name/role → tick flow is `PUT /api/users/{userID}`; the status toggle is `PUT /api/users/{userID}/active`. Both Admin-only                                                                                                                                                                                                                                                                                                                                                                                                                                                     | checkpoints 15–16                                  |
| F7  | **The Profile screen is read-only in Phase 1.** `settings-profile-enduser.html` offers a **Full Name edit + Save Changes** and a **Change Password** form; neither has a backing endpoint. _Change password_ is a **documented descope** (API plan "Forgot / reset / change password"). _Self name-change_ is an **undocumented gap** — `GET /api/me` is described in the plan as read-only, and `PUT /api/users/{userID}` is Admin-gated, so a regular user calling it gets 403. **Phase 1 UI:** display name / email / role from `GET /api/me`, plus sign-out via `POST /api/revoke`. Both sections deferred to Phase 2                               | `settings-profile-enduser.html`, API plan §Group 2 |
| F8  | **Time-of-day fields carry a fake date — strip it on read, re-add it on write.** `implementation_window_start` / `_end` are Postgres `TIME` columns (no date), but Go's `time.Time` always carries one, so the API returns **`"0000-01-01T09:00:00Z"`**. The time part is correct; year 0 is an artifact. An `<input type="time">` needs `"09:00"`, so slice characters 11–16 on read. **Critically, Save Draft accepts only RFC 3339** — sending `"09:00:00"` will be _rejected_ with a parse error, so the frontend must send the full `"0000-01-01T09:00:00Z"` form back. The contract is symmetric: the same shape in both directions               | checkpoint 18, flag #9                             |
| F9  | **Reset `offset` to 0 whenever a filter changes.** Otherwise: the user is on page 3 (`offset=40`), types a search term matching 3 records, and offset 40 skips past all of them — an empty screen for a search that did match. Adding or changing any filter means going back to page 1                                                                                                                                                                                                                                                                                                                                                                 | checkpoint 19                                      |
| F10 | **Four prototype controls have no API support in Phase 1** — remove or hide them: (a) the **Created By** column duplicates Change Owner; there is no `created_by` column and creator _is_ owner, immutably (field 3). `my-change-controls.html` row 3 shows them differing, which is not possible; (b) **sortable column chevrons** — sort is fixed at `last_updated_on DESC` (§9.5.3), there is no sort parameter; (c) the **7/30/90-day Date Range buckets** — the frontend computes the date and sends `?created_after=`; (d) the **Ownership dropdown's three options** collapse to two, since "Created by me" and "Owned by me" are the same thing | checkpoint 19                                      |
| F11 | **To clear a date or time field, send `null` — never `""`.** Text fields accept either (the handler normalises `""` to NULL), but the four date/time fields (`proposed_implementation_date`, `target_closure_date`, `implementation_window_start`, `implementation_window_end`) unmarshal into `*time.Time`, and `""` is **not** valid RFC 3339 — it returns **400**. Only `null` clears them                                                                                                                                                                                                                                                           | checkpoint 20                                      |
| F12 | **Save before submit — the two are separate calls.** `POST /{ccID}/submit` carries **no field values**; it validates whatever `PUT /{ccID}` last stored. So the Submit button must be **disabled while the form is dirty** (or must auto-save first). Otherwise the user submits with unsaved edits, and T2 rejects fields they can see filled in on screen — because the text never left the browser                                                                                                                                                                                                                                                   | checkpoint 21, decision #30                        |
| F13 | **Validate required fields client-side _before_ opening the signature modal.** FR-6.2.34 puts the signature last for a reason: nobody should type their password only to be told a field is empty. The backend enforces the same order, so this is UX, not security                                                                                                                                                                                                                                                                                                                                                                                     | checkpoint 21                                      |
| F14 | **The signature modal's "Username" field is the user's EMAIL.** `users` has no username column; the API compares against `user.Email` (case-insensitively). Relabel the field                                                                                                                                                                                                                                                                                                                                                                                                                                                                           | checkpoint 21                                      |
| F15 | **The retry loop is: fix → save → submit.** A failed submit leaves the record completely untouched — no state change, no signature — so retrying is safe. The only trace is a `SignatureFailed` audit row, and only when the failure was a _credential_ one; a validation failure writes nothing                                                                                                                                                                                                                                                                                                                                                        | checkpoint 21                                      |

---

## Open flags

| #   | Flag                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                              | Status                                                                                                             |
| --- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------ |
| 1   | **`change_controls` column count contradiction.** §3.2 and the §3 Summary state 48; §3.2's own parenthetical sums to 50                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                           | **Resolved: built 50, confirmed in the database.** Doc correction pending                                          |
| 2   | **`change_controls` DEFAULT count.** §6.4 says 8; §6.2 enumerates 7 (`cc_id` uses a generation expression, not a DEFAULT)                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                         | **Resolved: 7, confirmed in the database.** Doc correction pending                                                 |
| 3   | **`updated_at` vs `updated_on` on `refresh_tokens`.** Blueprint §7's code sample uses `updated_at`; DB Design §3.6 says `updated_on`                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                              | **Resolved in the schema: `updated_on`.** The Blueprint snippet is stale — adjust when writing the refresh handler |
| 4   | **En-dash in HTML prototype `<option value="...">`.** A frontend built from the prototypes verbatim fails `ck_cc_requires_testing` on every submit. Frontend must normalize at the API boundary, or the prototypes get fixed                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                      | Open for `change_controls`; closed for `esignatures` (DB §6.5)                                                     |
| 5   | **BRD §13.1 deferral note** for the three descoped password flows                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                 | Lain to add on next BRD touch                                                                                      |
| 6   | **Production version parity.** Dev is on PostgreSQL 14.23; if production runs 15/16 there's a major-version gap. No feature dependency — belongs in deployment notes                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                              | Noted                                                                                                              |
| 7   | **The two `.docx` guardrail files are stored as plain text** despite the extension. Read them directly; do not unzip                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                              | Environmental note                                                                                                 |
| 8   | **CC-ID gaps are expected and permanent.** `nextval()` is non-transactional, so a rolled-back or failed insert burns a number forever. Not a defect — the cost of collision-free IDs under concurrency — but QA will ask                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                          | Behaviour note; may warrant a line in user documentation                                                           |
| 9   | ~~**`TIME` columns scanning into `*time.Time` is unverified.**~~ **CLOSED at checkpoint 18.** lib/pq scans a `TIME` column into `*time.Time` without error — verified by setting `09:00:00` / `17:30:00` in psql and reading them back through the API. No override needed; the schema stays as DB §3.2 specifies. Values return as `"0000-01-01T09:00:00Z"` (see F8)                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                             | **Closed**                                                                                                         |
| 10  | **Log rotation not implemented.** `logs/app.log` is append-only and grows unbounded. Fine for dev; production needs size/date-based rotation (e.g. lumberjack or logrotate) to avoid filling disk. Operational hardening, deliberately deferred (§0/§15 spirit — build the debugging value now, defer the ops hardening)                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                          | Deferred; belongs in deployment notes                                                                              |
| 11  | **Login timing side-channel (measured, not theoretical).** Observed response times: unknown email **5 ms**, wrong password **137 ms**, success **267 ms**. The not-found path skips the argon2id comparison entirely, so valid emails are enumerable by timing alone — no message difference needed. Mitigation is ~5 lines: run `CheckPasswordHash` against a throwaway dummy hash on the not-found path so both branches cost the same. Fixed in checkpoint 9: a dummy hash generated once at startup is verified on the not-found path, equalising the cost. Measured after: 209 ms vs 229 ms                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                  | **Closed**                                                                                                         |
| 12  | **Frontend must refresh proactively — backend contract assumes it.** The client needs a timer firing at roughly **24 minutes** (~80 % of the 30-min access token's life), plus a 401-interceptor that refreshes and retries once as a safety net. Without the timer, users hit surprise logouts. This is **frontend responsibility** and needs stating in the BRD / frontend handover                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                             | Open — needs documenting for the frontend                                                                          |
| 13  | **Two refresh gates are unverified.** Absolute expiry (`now > expires_at`, 24 h) and inactivity timeout (`now − updated_on > 2 h`) cannot be exercised without waiting. Both are single comparisons against visible columns, so risk is low — but they have not been observed firing. Could be tested by hand-updating a row's `expires_at` / `updated_on` in psql                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                | Unverified                                                                                                         |
| 14  | **Dead `refresh_tokens` rows accumulate forever.** Expired and revoked rows are never removed. Eventual hygiene is a periodic `DELETE ... WHERE expires_at < NOW() - INTERVAL '30 days' OR revoked_at < ...`, as a cron or a `cmd/cleanup` command. Irrelevant at current volume (a few users, a handful of logins a day); deferred alongside log rotation (flag #10)                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                             | Deferred                                                                                                           |
| 15  | ~~**The deactivation 409 path is untested.**~~ **CLOSED at checkpoint 17.** With CC-001 and CC-002 created, both guards fired correctly: deactivating and role-changing their owner returned 409 listing both CC-IDs, while a name-only change and a name change with an unchanged role both succeeded                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                            | **Closed**                                                                                                         |
| 16  | **No CC reassignment endpoint exists.** If a CC's assigned approver becomes unavailable — deactivated by a route that bypasses the guard (a direct DB edit), or simply unreachable — the record has **no recovery path**: it sits in `Pending Implementation Approval` with nobody able to action it. Decision #19's guard prevents the API from _creating_ this state, but cannot repair one. Out of documented scope for Phase 1                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                | Noted; Phase 2 candidate                                                                                           |
| 17  | **`ILIKE '%term%'` cannot use an index.** The leading `%` makes a B-tree useless, so `?search=` forces a sequential scan of `change_controls` joined to `users`. Irrelevant at hundreds or a few thousand records (milliseconds); at ~100k rows it would need a **trigram index** (`CREATE EXTENSION pg_trgm` + a GIN index per searched column). Not worth building now                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                          | Noted; performance only                                                                                            |
| 18  | **No partial saving during `In Implementation`.** `CC_Field_Reference.md` Group 6 states fields 29–34 are editable by the CC Owner **"In Implementation only"**, but Save Draft (endpoint 13) is **`Initiated`-only** per the API plan, and no other endpoint lets the owner edit them. The only path is **T6 (`/submit-final`, endpoint 18) carrying fields 29–33 in its request body** — so the owner must complete all five in one sitting with no save-progress. Losing the browser loses the text (the evidence _file_ is safe — endpoint 20 stores it separately). **Decided for release 1: T6 carries fields 29–33 in its request body** — the two states genuinely differ. `Initiated` is a drafting process built up over days and needs saving; implementation details are a _completion report_ written afterwards from notes, and the signature attests to them, so writing and signing them in one atomic act is arguably more correct. **Rejected: making Save Draft state-aware** — that turns a clear endpoint into one with a mode. **If save-progress is later wanted, the additive fix is a separate endpoint** (`PUT /api/changecontrols/{ccID}/implementation` or similar), not a rewrite. **Revisit at checkpoint 25 (T6)** | Decided; revisit at T6                                                                                             |
| 19  | **`CC_Field_Reference.md`'s 🔒 Audit-tracked markers are incomplete.** It marks fields 35, 37, 38, 39, 42, 43, 50 — but **omits 13 `proposed_implementation_date` and 14 `target_closure_date`**, which BRD **§6.6.2** lists explicitly (_"Target Closure Date (initial value set and any subsequent changes)"_, _"Proposed Implementation Date (if changed)"_). Per the precedence rule the BRD wins on business meaning, so **all nine are audited**. The field reference should gain 🔒 on 13 and 14                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                           | Doc correction pending                                                                                             |
| 20  | **No rule requires `implementation_window_end` to be after `implementation_window_start`.** Neither `CC_Field_Reference.md` (fields 15–16) nor the BRD states one, so nothing prevents saving a window of 17:00–09:00. Not invented at Save Draft, which validates neither presence nor business rules by design. **If the rule is wanted it belongs at T2**, alongside the business-day date checks — and it would need adding to the guardrail docs first                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                       | Open — spec gap, decide at T2                                                                                      |
| 21  | **Date rules are computed in UTC, not a business timezone.** `businessDaysFrom` is fed `time.Now().UTC()` truncated to midnight. Abu Dhabi is UTC+4, so **between 00:00 and 04:00 local, "today" in UTC is still yesterday** and both date rules are silently one day more lenient. Found empirically: a submission at 01:42 local on Aug 6 computed "today" as Aug 5 and accepted a proposed date that should have been rejected. In a regulated system the same record validating differently depending on the hour is a real defect. **Deliberately not fixed in release 1** — the intended fix is a `TIMEZONE` env var (e.g. `Asia/Dubai`) read once at startup, so the business day is defined by configuration rather than by where the server happens to run. Note `today` must stay a **UTC midnight** for the comparison, since `DATE` columns arrive as UTC midnights; only the _choice of calendar date_ is local                                                                                                                                                                                                                                                                                                                      | Deferred to a later release                                                                                        |
| 22  | **`businessDaysFrom` treats Saturday and Sunday as the weekend.** The UAE working week is Monday–Friday for most private-sector employers, but this has not been confirmed with EAMI, and the field reference says only _"weekdays only (no public holidays in Phase 1)"_. If their weekend is Friday–Saturday, every date rule is wrong by a day or two. **Worth asking before go-live**; it is a one-line change in the helper, and its test file already covers weekend boundaries                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                             | Open — needs confirming with EAMI                                                                                  |
| 23  | **Two guardrail docs contradict each other on `target_closure_date` after a rejection.** BRD **SC-6** says _"Target Closure Date is **locked after initial submission**"_; `CC_Field_Reference.md` field 14 says _"**Editable whenever state = Initiated** (incl. after rejection)"_. These cannot both hold — T5 (reject) returns a CC to `Initiated`. **Implemented per the field reference** (editable), and deliberately: after a rejection the owner may have to rework the implementation plan, and locking the closure date could leave them unable to submit a coherent record. **The BRD needs correcting**; relevant when building T5 (checkpoint 23)                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   | Open — doc conflict, implementation decided                                                                        |

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

# Postman — collection lives at postman/ea-qms.postman_collection.json
#   Environment "EA QMS Local" is NOT committed (holds live tokens).
#   Vars: baseUrl=http://localhost:1304/api, token, refresh_token, ccId
#   Login requests auto-capture token into the environment; collection-level
#   Bearer auth uses {{token}}, so protected requests need no manual header.
#   Habits that make a later OpenAPI export usable: descriptive request names,
#   and "Save as Example" on every response (truncate credentials first).

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
