# PROGRESS — EA QMS Change Control Backend (Go)

**Scope of this file:** what is built, what is next, decisions made in working sessions
that are not recorded in any guardrail document, and open flags. Nothing else — the six
guardrail docs carry the substance and are always attached.

- **Repo:** `github.com/lain-the-coder/ea-qms-backend`
- **Last checkpoint:** 4 — `004_audit_logs.sql`
- **Next task:** checkpoint 5 — `005_esignatures.sql` (not started)
- **Schema version:** 4 · tables: `users`, `change_controls`, `file_attachments`, `audit_logs`

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
| Field-group order per §3.2; fields 24 and 34 correctly absent (they are `file_attachments` rows)                         | ✅     |
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
First table with an FK into another migration's table; dropping it leaves
`change_controls` untouched.

| Check (DB §3.3 / §4.1 / §5.1 #10 / §5.3 / §6.1 / §6.2)                                 | Result |
| -------------------------------------------------------------------------------------- | ------ |
| 9 columns, §3.3 order and names                                                        | ✅     |
| `BYTEA` for `file_data`, `BIGINT` for `file_size`                                      | ✅     |
| **All 9 NOT NULL** — inverse of `change_controls`; a file row exists only after upload | ✅     |
| 2 defaults — `gen_random_uuid()`, `NOW()`                                              | ✅     |
| `ck_file_attachments_field_name` — `{supporting_documents, implementation_evidence}`   | ✅     |
| `change_control_id` → `change_controls(id)` **ON DELETE CASCADE** (§4.1 #6)            | ✅     |
| `uploaded_by_id` → `users(id)` **ON DELETE RESTRICT** (§4.1 #7)                        | ✅     |
| `uq_file_attachments_cc_field` as a **UNIQUE CONSTRAINT** (§5.2 #3)                    | ✅     |
| **Zero `CREATE INDEX` statements** (§5.3)                                              | ✅     |
| No `file_size` CHECK — the 10 MB limit and MIME rules stay in the Go handler (§3.3)    | ✅     |

**Lesson:** why no separate index on `change_control_id` — the composite
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
| No UNIQUE constraint                                                                     | ✅     |
| **No triggers or rules attempting immutability** (§8.3 — the application's job)          | ✅     |

**Lesson:** `entity_id` looks exactly like a foreign key and must not be one. It points at
either `change_controls` or `users` depending on `entity_type` — a single column can't FK
two tables — and audit rows must outlive whatever they describe. CASCADE would erase
history; RESTRICT would block legitimate cleanup.

---

## Next

### ⬜ Checkpoint 5 — `005_esignatures.sql`

Sources: DB Design **§3.5** (7 columns) · **§3.5 note "the two-table separation"** ·
**§4.1** · **§5.1** · **§6.1** · **§6.2**.

- 7 columns, **all NOT NULL**: `id`, `change_control_id`, `signer_id`, `signer_name`,
  `transition`, `meaning`, `signed_on`
- `signer_name` is a snapshot, same rationale as `audit_logs.performed_by_name` (BR-8.8.5)
- Two CHECKs: `ck_esignatures_transition` — `{'T2'..'T8'}`, **T1 never signs**; and
  `ck_esignatures_meaning` — the seven-value closed set, **ASCII hyphens** in the four
  gate meanings
- **No `updated_on`, no soft-delete column** — immutability is the point (§3.5)
- Rows accumulate and are never updated or deleted (BR-8.8.7). A CC through a rejection
  loop holds multiple rows for the same gate. `change_controls` holds only the _latest_
  decision; `esignatures` holds _all_. **Do not deduplicate**
- A successful signature writes to **both** `audit_logs` (`SignatureCaptured`) and
  `esignatures`, in one transaction. Failed attempts go to `audit_logs` only (BR-8.8.8)

### ⬜ Remaining

`006_refresh_tokens` (see flag #3 — `updated_on`, not `updated_at`) · seed (4 users, one
per role, hashed with the app's own argon2id — never pasted hashes, gated on
`PLATFORM=dev`, §7; **no CC, file, audit, esignature or token rows**, §7.3)

### ⬜ Then

sqlc setup (`emit_pointers_for_null_types: true`) → build in API Endpoint Plan order,
starting `POST /api/login`.

---

## Session decisions not in any guardrail doc

Settled in working sessions and binding. They exist nowhere else.

| #   | Decision                                                                                                                                                                                                             | Rationale                                                                                                                                                                                                                                                                                            |
| --- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1   | **Module path `github.com/lain-the-coder/ea-qms-backend`** — not `-cc-backend`                                                                                                                                       | Future QMS modules (Deviation, CAPA) live under the same module rather than forcing a second repo                                                                                                                                                                                                    |
| 2   | **Constraint/index naming follows §5.1 and §6.1 verbatim, including their abbreviations** — `ck_cc_*`, `idx_cc_*`, `idx_audit_*` (short) while CHECKs stay full (`ck_audit_logs_*`) — _not_ §1.3's long-form example | §5.1/§6.1 are definitions and are cross-referenced by name elsewhere (§8.2 cites `idx_cc_owner_state`); §1.3 is a convention statement with one stale example. Also keeps names clear of Postgres's 63-byte identifier truncation                                                                    |
| 3   | **Two naming exceptions kept verbatim:** `uq_change_controls_cc_id` and `ck_cc_post_impl_issues`                                                                                                                     | Spelled that way in §3.2/§5.1 and §6.1 respectively — do not "regularize" them                                                                                                                                                                                                                       |
| 4   | **FK constraints use the long form** — `fk_<table>_<column>`, e.g. `fk_audit_logs_performed_by_id`                                                                                                                   | §4 lists all eleven FKs but never names the constraints, so §1.3 stands unopposed for this object type. Decision #2 does not extend to FKs. The name is what appears in the Postgres error text you map to a 409 (Blueprint §11)                                                                     |
| 5   | **PostgreSQL 14.23 accepted** (doc §1.2 specifies 15+)                                                                                                                                                               | Every needed feature traced and predates 14: `gen_random_uuid()` core (13), `GENERATED ALWAYS AS ... STORED` (12), `ON CONFLICT DO UPDATE` (9.5), `SELECT ... FOR UPDATE`, functional/composite indexes                                                                                              |
| 6   | **`log.Fatal(server.ListenAndServe())`**, not a bare call                                                                                                                                                            | A discarded error means a bind failure exits silently with status 0 and no message                                                                                                                                                                                                                   |
| 7   | **Goose run as a global CLI from `sql/schema`**                                                                                                                                                                      | Matches prior boot.dev workflow; keeps migration files free of Go wiring                                                                                                                                                                                                                             |
| 8   | **Uniqueness form rule:** plain columns → table `CONSTRAINT ... UNIQUE`; expressions or partials → `CREATE UNIQUE INDEX`                                                                                             | A `UNIQUE` table constraint accepts only a column list, so `uq_users_email` on `LOWER(email)` _must_ be an index. Constraints are preferred otherwise: they support `ON CONFLICT ON CONSTRAINT <name>`, appear in `information_schema.table_constraints`, and are what Postgres's own docs recommend |
| 9   | **DBeaver is connected for browsing only.** All schema changes go through goose; no UI edits, no UI-created rows                                                                                                     | Applied migrations are the schema's only description (Blueprint §13). DBeaver also splits `\d` across tabs and blurs the constraint-vs-index distinction from decision #8 — so **DBeaver to navigate, psql to verify**                                                                               |

---

## Open flags

| #   | Flag                                                                                                                                                                                                                                                                                                                                                                 | Status                                                                    |
| --- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------- |
| 1   | **`change_controls` column count contradiction.** §3.2 and the §3 Summary state **48 columns**, but §3.2's own parenthetical reads "id + cc_number + 48 of the 50 BRD fields" = **50**                                                                                                                                                                               | **Resolved: built 50, confirmed in the database.** Doc correction pending |
| 2   | **`change_controls` DEFAULT count.** §6.4 says 8; §6.2 enumerates 7. The likely 8th is `cc_id`, which §6.2 explicitly states has _no_ DEFAULT                                                                                                                                                                                                                        | **Resolved: 7, confirmed in the database.** Doc correction pending        |
| 3   | **`updated_at` vs `updated_on` on `refresh_tokens`.** Blueprint §7's code sample uses `updated_at`; DB Design §3.6 and CONTEXT_HANDOFF both say `updated_on`. Per precedence the DB doc wins — the Blueprint snippet is stale. Affects migration 006 and the refresh handler                                                                                         | Logged, resolution clear                                                  |
| 4   | **En-dash in HTML prototype `<option value="...">`.** A frontend built from the prototypes verbatim fails `ck_cc_requires_testing` on every submit — that constraint is now live. Frontend must normalize at the API boundary, or the prototypes get fixed. The same ASCII-hyphen risk applies to the four gate meanings in `ck_esignatures_meaning` (migration 005) | Open risk, pre-existing (DB §6.5)                                         |
| 5   | **BRD §13.1 deferral note** for the three descoped password flows                                                                                                                                                                                                                                                                                                    | Lain to add on next BRD touch                                             |
| 6   | **Production version parity.** Dev is on PostgreSQL 14.23; if production runs 15/16 there's a major-version gap. No feature dependency — belongs in deployment notes                                                                                                                                                                                                 | Noted                                                                     |
| 7   | **The two `.docx` guardrail files are stored as plain text** despite the extension. Read them directly; do not attempt to unzip                                                                                                                                                                                                                                      | Environmental note                                                        |
| 8   | **CC-ID gaps are expected and permanent.** `nextval()` is non-transactional, so a rolled-back or failed insert burns a number forever. Not a defect — the cost of collision-free IDs under concurrency — but QA will ask                                                                                                                                             | Behaviour note; may warrant a line in user documentation                  |

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
psql "postgres://postgres:PASS@localhost:5432/ea_qms?sslmode=disable" -f 005_esignatures.sql

# psql
psql "postgres://postgres:PASS@localhost:5432/ea_qms?sslmode=disable"
#   \l  databases   \dt  tables   \ds  sequences   \d <table>  detail   \q  quit
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
