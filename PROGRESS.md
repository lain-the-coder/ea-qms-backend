# PROGRESS — EA QMS Change Control Backend (Go)

**Scope of this file:** what is built, what is next, decisions made in working sessions
that are not recorded in any guardrail document, and open flags. Nothing else — the six
guardrail docs carry the substance and are always attached.

- **Repo:** `github.com/lain-the-coder/ea-qms-backend`
- **Last checkpoint:** 6 — `006_refresh_tokens.sql` · **the schema is complete**
- **Next task:** checkpoint 7 — sqlc setup (not started)
- **Schema version:** 6 · all six tables built and verified

---

## Phase status

| Phase                             | State                                                 |
| --------------------------------- | ----------------------------------------------------- |
| **Migrations (001–006)**          | ✅ **Complete** — all six tables applied and verified |
| Seed                              | ⬜ Not started (depends on sqlc + auth package)       |
| sqlc setup                        | ⬜ Not started — **next**                             |
| API implementation (22 endpoints) | ⬜ Not started                                        |

Database objects: `users`, `change_controls`, `file_attachments`, `audit_logs`,
`esignatures`, `refresh_tokens`, plus `cc_number_seq` and goose's own bookkeeping.

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
| Field-group order per §3.2; BRD fields 24 and 34 correctly absent (they are `file_attachments` rows)                     | ✅     |
| Types incl. `DATE` ×3, `TIME` ×2, `TIMESTAMPTZ` ×5                                                                       | ✅     |
| 10 NOT NULL, 40 NULL (§1.6 — required for Save Draft)                                                                    | ✅     |
| 7 defaults; `cc_id` has none                                                                                             | ✅     |
| `cc_number_seq` + `cc_id GENERATED ALWAYS AS (...) STORED` with the `CASE` LPAD guard (§8.1)                             | ✅     |
| 13 CHECKs, `ck_cc_*` names, values verbatim                                                                              | ✅     |
| Three value traps held: ASCII hyphens in `'Yes - Full testing'`; `'Approve'`/`'Reject'` not past tense; no `'Emergency'` | ✅     |
| 5 FKs → `users(id)`, all `ON DELETE RESTRICT`, long-form names (§4.1 rows 1–5)                                           | ✅     |
| `uq_change_controls_cc_id` as a **UNIQUE CONSTRAINT**, not a `CREATE INDEX` (§5.2 #3)                                    | ✅     |
| 6 `CREATE INDEX` (§5.1 #4–#9), `DESC` on `idx_cc_created_on`; 8 index entries incl. PK                                   | ✅     |
| Down drops **table then sequence**; `\ds` confirms `cc_number_seq` gone; re-`up` succeeds                                | ✅     |

**Lesson:** a separately-created sequence is not owned by the table. `DROP TABLE` alone
orphans it and the next `up` fails on `CREATE SEQUENCE ... already exists`. Order matters
too — dropping the sequence first fails, because the column default depends on it.

### ✅ Checkpoint 3 — file_attachments migration

**`sql/schema/003_file_attachments.sql`** — applied, up → `\d` → down → `\dt` → up clean.

| Check (DB §3.3 / §4.1 / §5.1 #10 / §5.3 / §6.1 / §6.2)                               | Result |
| ------------------------------------------------------------------------------------ | ------ |
| 9 columns, §3.3 order and names                                                      | ✅     |
| `BYTEA` for `file_data`, `BIGINT` for `file_size`                                    | ✅     |
| **All 9 NOT NULL** — a file row exists only after upload                             | ✅     |
| 2 defaults — `gen_random_uuid()`, `NOW()`                                            | ✅     |
| `ck_file_attachments_field_name` — `{supporting_documents, implementation_evidence}` | ✅     |
| `change_control_id` → `change_controls(id)` **ON DELETE CASCADE** (§4.1 #6)          | ✅     |
| `uploaded_by_id` → `users(id)` **ON DELETE RESTRICT** (§4.1 #7)                      | ✅     |
| `uq_file_attachments_cc_field` as a **UNIQUE CONSTRAINT** (§5.2 #3)                  | ✅     |
| **Zero `CREATE INDEX` statements** (§5.3)                                            | ✅     |
| No `file_size` CHECK — the 10 MB limit and MIME rules stay in the Go handler (§3.3)  | ✅     |

**Lesson:** no separate index on `change_control_id` — the composite
`(change_control_id, field_name)` sorts by the first column, so leftmost-prefix already
serves "all files for this CC". A second index would cost writes for nothing.

### ✅ Checkpoint 4 — audit_logs migration

**`sql/schema/004_audit_logs.sql`** — applied, up → `\d` → down → `\dt` → up clean.

| Check (DB §3.4 / §2.3 / §4.1 #8 / §5.1 #11–13 / §6.1 / §6.2)                             | Result |
| ---------------------------------------------------------------------------------------- | ------ |
| 10 columns, §3.4 order and names                                                         | ✅     |
| 7 NOT NULL; `field_name`, `old_value`, `new_value` nullable (non-field events)           | ✅     |
| **No `action_description` column** — descriptions derive at read time                    | ✅     |
| 2 defaults — `gen_random_uuid()`, `NOW()`                                                | ✅     |
| `ck_audit_logs_entity_type` — 2 values                                                   | ✅     |
| `ck_audit_logs_action_type` — **9 values** incl. `SignatureCaptured` / `SignatureFailed` | ✅     |
| **`entity_id` is a bare `UUID NOT NULL` with no FK** (§2.3)                              | ✅     |
| `fk_audit_logs_performed_by_id` → `users(id)` RESTRICT — the only FK                     | ✅     |
| `performed_by_name` a plain NOT NULL snapshot column, not a join                         | ✅     |
| 3 indexes: `idx_audit_entity`, `idx_audit_created_on` (DESC), `idx_audit_performed_by`   | ✅     |
| No UNIQUE constraint; no triggers attempting immutability (§8.3)                         | ✅     |

**Lesson:** `entity_id` looks exactly like a foreign key and must not be one. It points at
either `change_controls` or `users` depending on `entity_type` — a single column can't FK
two tables — and audit rows must outlive whatever they describe.

### ✅ Checkpoint 5 — esignatures migration

**`sql/schema/005_esignatures.sql`** — applied, up → `\d` → down → `\dt` → up clean.

| Check (DB §3.5 / §4.1 #9–10 / §4.3 / §5.1 #14 / §6.1 / §6.2)                                                     | Result |
| ---------------------------------------------------------------------------------------------------------------- | ------ |
| 7 columns, all NOT NULL                                                                                          | ✅     |
| **No `updated_on`, no soft-delete column** — immutability is the design (§3.5)                                   | ✅     |
| 2 defaults — `gen_random_uuid()`, `NOW()`                                                                        | ✅     |
| `signer_name` a snapshot column, not a join (BR-8.8.5)                                                           | ✅     |
| `ck_esignatures_transition` — T2–T8; **T1 is never signed**                                                      | ✅     |
| `ck_esignatures_meaning` — 7 values; **ASCII hyphens on the four gate meanings, verified in the catalog** (§6.5) | ✅     |
| Both FKs **ON DELETE RESTRICT**, incl. `change_control_id` (§4.3)                                                | ✅     |
| `idx_esignatures_cc` on `(change_control_id)`                                                                    | ✅     |
| **No UNIQUE constraint** — rejection loops legitimately produce multiple rows per gate                           | ✅     |

**Lesson:** the mirror image of checkpoint 3. `change_control_id` **CASCADEs** in
`file_attachments` and **RESTRICTs** here — same column name, same target table, opposite
rule. A file has no meaning without its CC; a signature is a permanent regulatory artifact
and blocking the delete is the correct outcome.

### ✅ Checkpoint 6 — refresh_tokens migration · schema complete

**`sql/schema/006_refresh_tokens.sql`** — applied, up → `\d` → down → `\dt` → up clean.

| Check (DB §3.6 / §4.1 #11 / §4.3 / §5.1 #15 / §6.2 / §6.4)                         | Result |
| ---------------------------------------------------------------------------------- | ------ |
| 6 columns, §3.6 order and names                                                    | ✅     |
| **PK is `token TEXT` — no surrogate `id UUID`**, the only such table in the schema | ✅     |
| `revoked_at` the only nullable column                                              | ✅     |
| **`updated_on`, not `updated_at`** — flag #3 resolved in favour of the DB doc      | ✅     |
| 2 defaults, both `NOW()`; **no `gen_random_uuid()` anywhere in this migration**    | ✅     |
| **Zero CHECK constraints** — the only table without one                            | ✅     |
| `fk_refresh_tokens_user_id` → `users(id)` **ON DELETE CASCADE** (§4.3)             | ✅     |
| `idx_refresh_tokens_user` on `(user_id)`                                           | ✅     |

**Lesson:** three timestamps, three jobs. `updated_on` drives the 30-minute **sliding
inactivity window** (touched on every successful refresh; a refresh where
`NOW() - updated_on > 30 min` is rejected). `expires_at` is the **absolute cap** regardless
of activity. `revoked_at` is set on logout. Also: CASCADE here vs RESTRICT on `audit_logs`
and `esignatures` — a session is disposable, a signature and an audit record are not.

---

## Next

### ⬜ Checkpoint 7 — sqlc setup

Sources: **Blueprint §4** and **§5**.

- `sqlc.yaml` at the repo root — engine `postgresql`, queries `sql/queries`, schema
  `sql/schema`, out `internal/database`, package `database`
- **`emit_pointers_for_null_types: true`** — nullable columns become `*string` / `*time.Time`
  rather than `sql.NullString`. Needed for JSON cleanliness across ~40 nullable CC columns
  and for the three-state Save Draft logic
- First query files in `sql/queries/`, then `sqlc generate`
- `internal/database` is **generated code — never hand-edited** (Blueprint §5)

Note the ordering: §7.3's recommended seed is a **Go command** that calls
`auth.HashPassword` and the generated `queries.CreateUser`, so sqlc and the auth package
both have to exist before the seed can be written.

### ⬜ Then — argon2id auth package + seed

`internal/auth` — `HashPassword` / `CheckPasswordHash` (argon2id, also used for
e-signature re-auth).

Seed (§7.2–§7.5): **four users, one per role**, all sharing a known dev password
(e.g. `DevPassw0rd!`) hashed **at seed time with the app's own argon2id** — never a pasted
hash, and never BCrypt from the old .NET seed (it would never validate).

| full_name            | email                | role     |
| -------------------- | -------------------- | -------- |
| System Administrator | admin@eaqms.local    | Admin    |
| Default CC Owner     | owner@eaqms.local    | CC Owner |
| Default Approver     | approver@eaqms.local | Approver |
| Default Viewer       | viewer@eaqms.local   | Viewer   |

`.local` domain so seed accounts can never be confused with real addresses. Must be
reversible and **gated on `PLATFORM=dev`** — never applied to production (§7.5); the first
real production admin is provisioned out-of-band.

**Nothing else is seeded** (§7.4) — no CCs, files, audit rows, signatures or tokens. Those
come from real workflow actions through the API.

### ⬜ Then — the API

Build in API Endpoint Plan order: `POST /api/login` → `middlewareAuth` → `GET /api/me` →
refresh/revoke → user management → CC create/get/list/save-draft → **T2 submit, the first
full transition, written inline** → T3, T4/5 (extract only then) → files → T6 → T7/8 →
dashboard → signatures.

---

## Session decisions not in any guardrail doc

Settled in working sessions and binding. They exist nowhere else.

| #   | Decision                                                                                                                                                                                                                                 | Rationale                                                                                                                                                                                                                                                                                            |
| --- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1   | **Module path `github.com/lain-the-coder/ea-qms-backend`** — not `-cc-backend`                                                                                                                                                           | Future QMS modules (Deviation, CAPA) live under the same module rather than forcing a second repo                                                                                                                                                                                                    |
| 2   | **Constraint/index naming follows §5.1 and §6.1 verbatim, including their abbreviations** — `ck_cc_*`, `idx_cc_*`, `idx_audit_*` (short) while CHECKs stay full (`ck_audit_logs_*`, `ck_esignatures_*`) — _not_ §1.3's long-form example | §5.1/§6.1 are definitions and are cross-referenced by name elsewhere (§8.2 cites `idx_cc_owner_state`); §1.3 is a convention statement with one stale example. Also keeps names clear of Postgres's 63-byte identifier truncation                                                                    |
| 3   | **Two naming exceptions kept verbatim:** `uq_change_controls_cc_id` and `ck_cc_post_impl_issues`                                                                                                                                         | Spelled that way in §3.2/§5.1 and §6.1 respectively — do not "regularize" them                                                                                                                                                                                                                       |
| 4   | **FK constraints use the long form** — `fk_<table>_<column>`                                                                                                                                                                             | §4 lists all eleven FKs but never names the constraints, so §1.3 stands unopposed for this object type. Decision #2 does not extend to FKs. The name is what appears in the Postgres error text you map to a 409 (Blueprint §11)                                                                     |
| 5   | **PostgreSQL 14.23 accepted** (doc §1.2 specifies 15+)                                                                                                                                                                                   | Every needed feature traced and predates 14: `gen_random_uuid()` core (13), `GENERATED ALWAYS AS ... STORED` (12), `ON CONFLICT DO UPDATE` (9.5), `SELECT ... FOR UPDATE`, functional/composite indexes                                                                                              |
| 6   | **`log.Fatal(server.ListenAndServe())`**, not a bare call                                                                                                                                                                                | A discarded error means a bind failure exits silently with status 0 and no message                                                                                                                                                                                                                   |
| 7   | **Goose run as a global CLI from `sql/schema`**                                                                                                                                                                                          | Matches prior boot.dev workflow; keeps migration files free of Go wiring                                                                                                                                                                                                                             |
| 8   | **Uniqueness form rule:** plain columns → table `CONSTRAINT ... UNIQUE`; expressions or partials → `CREATE UNIQUE INDEX`                                                                                                                 | A `UNIQUE` table constraint accepts only a column list, so `uq_users_email` on `LOWER(email)` _must_ be an index. Constraints are preferred otherwise: they support `ON CONFLICT ON CONSTRAINT <name>`, appear in `information_schema.table_constraints`, and are what Postgres's own docs recommend |
| 9   | **DBeaver is connected for browsing only.** All schema changes go through goose; no UI edits, no UI-created rows                                                                                                                         | Applied migrations are the schema's only description (Blueprint §13). DBeaver also splits `\d` across tabs and blurs the constraint-vs-index distinction from decision #8 — so **DBeaver to navigate, psql to verify**                                                                               |

---

## Open flags

| #   | Flag                                                                                                                                                                                                                         | Status                                                                                                                                      |
| --- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------- |
| 1   | **`change_controls` column count contradiction.** §3.2 and the §3 Summary state **48 columns**, but §3.2's own parenthetical reads "id + cc_number + 48 of the 50 BRD fields" = **50**                                       | **Resolved: built 50, confirmed in the database.** Doc correction pending                                                                   |
| 2   | **`change_controls` DEFAULT count.** §6.4 says 8; §6.2 enumerates 7. The likely 8th is `cc_id`, which §6.2 explicitly states has _no_ DEFAULT                                                                                | **Resolved: 7, confirmed in the database.** Doc correction pending                                                                          |
| 3   | **`updated_at` vs `updated_on` on `refresh_tokens`.** Blueprint §7's code sample uses `updated_at`; DB Design §3.6 says `updated_on`                                                                                         | **Resolved in the schema: the column is `updated_on`.** The Blueprint snippet is stale — adjust when writing the refresh handler            |
| 4   | **En-dash in HTML prototype `<option value="...">`.** A frontend built from the prototypes verbatim fails `ck_cc_requires_testing` on every submit. Frontend must normalize at the API boundary, or the prototypes get fixed | Open for `change_controls`; the parallel risk in `ck_esignatures_meaning` is closed — those four values are ASCII in the database (DB §6.5) |
| 5   | **BRD §13.1 deferral note** for the three descoped password flows                                                                                                                                                            | Lain to add on next BRD touch                                                                                                               |
| 6   | **Production version parity.** Dev is on PostgreSQL 14.23; if production runs 15/16 there's a major-version gap. No feature dependency — belongs in deployment notes                                                         | Noted                                                                                                                                       |
| 7   | **The two `.docx` guardrail files are stored as plain text** despite the extension. Read them directly; do not attempt to unzip                                                                                              | Environmental note                                                                                                                          |
| 8   | **CC-ID gaps are expected and permanent.** `nextval()` is non-transactional, so a rolled-back or failed insert burns a number forever. Not a defect — the cost of collision-free IDs under concurrency — but QA will ask     | Behaviour note; may warrant a line in user documentation                                                                                    |

---

## Environment & workflow

WSL Ubuntu 22.04 · VS Code Remote-WSL · Go 1.25.x · PostgreSQL 14.23 · DBeaver connected
for browsing (see decision #9).

```bash
# migrations — run from sql/schema
goose postgres "postgres://postgres:PASS@localhost:5432/ea_qms?sslmode=disable" up
goose postgres "postgres://postgres:PASS@localhost:5432/ea_qms?sslmode=disable" down
goose postgres "postgres://postgres:PASS@localhost:5432/ea_qms?sslmode=disable" status

# dry-run a migration before handing it to goose — psql reports the exact line
# and a caret; goose only reports that something failed
psql "postgres://postgres:PASS@localhost:5432/ea_qms?sslmode=disable" -f <file>.sql

# psql
psql "postgres://postgres:PASS@localhost:5432/ea_qms?sslmode=disable"
#   \l  databases   \dt  tables   \ds  sequences   \d  everything   \d <table>  detail
#   \pset pager off        before \d on wide tables, or the output gets mangled
```

**Every migration gets up → `\d` → down → `\dt` (+ `\ds` if it creates a sequence) → up
before it counts as done.** The final `up` is easy to forget — `goose status` confirms
where the database actually stands. If down doesn't cleanly reverse up, the migration has
a bug: find it now, locally (Blueprint §13).

### Things learned at the psql prompt

- Postgres **rewrites `IN (...)` as `= ANY (ARRAY[...])`** in the catalog, so `\d` never
  reads back character-for-character as written. Normalization, not drift.
- A **UNIQUE constraint** displays as `UNIQUE CONSTRAINT, btree (col)`; a bare
  `CREATE UNIQUE INDEX` displays as `UNIQUE, btree (col)`. This is how to tell which form
  actually got built.
- `TIMESTAMPTZ` displays as `timestamp with time zone`; `TIME` as
  `time without time zone`. Canonical spellings, nothing wrong.
- **Verify counts with SQL, not by counting a terminal paste:**
  `SELECT count(*) FROM information_schema.columns WHERE table_name = 'x';`
- **Postgres column-definition rule** (this cost several rounds on 002 and 003):
  everything between `CREATE TABLE x (` and `)` is one comma-separated list. Column-level
  constraints live inside a column's own definition and take no column list; table-level
  constraints (FKs, multi-column UNIQUE) are their own list items and require one. Comma
  between items, none after the last, semicolon only after the closing paren.
- **Sequences are non-transactional.** `nextval()` does not roll back — see flag #8.
- Once a statement errors inside a transaction, psql aborts the block: everything after
  returns _"current transaction is aborted"_ until `ROLLBACK`. Normal, not a stuck session.
- **Checking for invisible characters:** en-dash vs hyphen can't be eyeballed. Scan the
  file for any codepoint above 127 before running it — every value in this schema is ASCII.
- **Copy-paste between migrations is the most common error source.** 006's index was
  briefly created `ON esignatures`; it failed loudly only because the column name didn't
  exist there. Always re-read the table name in a copied `CREATE INDEX`.
