# Go Backend Coding Guide

**EA QMS — Change Control Module**
Version 1.0 · Supersedes `BACKEND_BLUEPRINT.md`

---

## What this is

A record of how this backend is written and why. Every rule here was arrived at
by building something, hitting a problem, and deciding how to handle it — not by
adopting a style guide wholesale.

It is not about formatting. `gofmt` settles that and there is nothing to discuss.
This is about **structure and approach**: when to abstract, where to put a lock,
what belongs in an audit row, how a handler is shaped.

## How to read it

Each rule is numbered and stated in its heading. Underneath: code showing what to
do and what not to do, then a short explanation of the trade. If a rule has an
exception, the exception is stated with it — an unqualified rule that gets broken
in practice is worse than no rule.

Examples are taken from this codebase, not invented.

## The one-line summary

**Write code that can be read top to bottom without following anything.** Prefer a
longer function you can see all of over a shorter one that sends you elsewhere.
Extract when repetition is genuinely painful or when a piece needs testing on its
own — not because a block looks big.

---

## Contents

**Part I — Approach**

1. [Explicit over implicit](#1-explicit-over-implicit)
2. [Abstraction is earned, not assumed](#2-abstraction-is-earned-not-assumed)

**Part II — Data** 3. [Let the database enforce what it can](#3-let-the-database-enforce-what-it-can) 4. [Queries](#4-queries) 5. [Nullability](#5-nullability)

**Part III — Correctness under load** 6. [Transactions](#6-transactions) 7. [Locking](#7-locking) 8. [Concurrency patterns](#8-concurrency-patterns)

**Part IV — The HTTP layer** 9. [Handlers](#9-handlers) 10. [Authorisation](#10-authorisation) 11. [Validation](#11-validation) 12. [Errors and status codes](#12-errors-and-status-codes) 13. [JSON](#13-json)

**Part V — Traceability** 14. [The audit trail](#14-the-audit-trail) 15. [Logging](#15-logging)

**Part VI — Working practice** 16. [Migrations](#16-migrations) 17. [Testing](#17-testing) 18. [Documentation discipline](#18-documentation-discipline)

---

# Part I — Approach

## 1. Explicit over implicit

### 1.1 Write the branch, don't dispatch it

```go
// Avoid — a transition engine
type Transition interface {
    Validate(cc ChangeControl) []string
    Apply(tx *sql.Tx, cc ChangeControl) error
    Meaning() string
}
var transitions = map[string]Transition{"T2": submitT2{}, "T4": approveT4{}, ...}
```

```go
// Prefer — eight handlers, each written out
func (cfg *apiConfig) HandlerSubmitForImplApproval(...)  { ... }
func (cfg *apiConfig) HandlerCancelChangeControl(...)    { ... }
func (cfg *apiConfig) HandlerImplementationDecision(...) { ... }
```

The engine is fewer lines. But answer "what exactly happens at T5?" from it: you
read the interface, find the implementation, trace the dispatch, and hold three
files in your head. With eight handlers you open one and read it.

Indirection does not remove work. It defers it to whoever reads the code next.

### 1.2 `switch` over a membership abstraction

```go
// Avoid
if !slices.Contains(validChangeTypes, *v) {
    → 400
}
```

```go
// Prefer
switch *v {
case changeTypeApplication, changeTypeInfrastructure, changeTypeDatabase,
     changeTypeSecurity, changeTypeNetwork, changeTypeHardware,
     changeTypeProcess, changeTypeOther:
default:
    → 400 "Invalid change_type"
}
```

The `switch` shows every accepted value at the point of decision. The helper shows
a variable name and sends you to find it. One extra line, and nothing to look up.

### 1.3 No service layer, no repository layer

```
handler → sqlc → Postgres
```

Not:

```
handler → service → repository → sqlc → Postgres
```

Those layers exist to swap implementations and to isolate business logic from
storage. Neither applies here: there is one database, it is not changing, and the
business logic _is_ mostly about what gets stored and when. The layers would add
two files per feature and answer a question nobody is asking.

### 1.4 Standard library over framework

`net/http` and `ServeMux`, not Gin, Echo, or Fiber.

Go 1.22's mux does method matching and path parameters, which is all this project
needs. A framework would add a dependency, a routing DSL to learn, and its own
opinions about middleware — in exchange for saving perhaps forty lines.

### 1.5 State the cost

Explicit code is longer. `HandlerSaveDraft` is roughly 700 lines, most of it 24
near-identical field blocks. A descriptor table with closures would be about a
third of that.

The trade is deliberate and worth naming rather than pretending it is free:

- **What you pay:** length. The function does not fit on a screen.
- **What you get:** `grep change_title` lands on exactly the block that handles it.
  Nothing is hidden behind a loop over a table of function pointers.
- **Why it is acceptable here:** the repetition is _inside one function read top to
  bottom_, not scattered across files where two copies can drift apart unnoticed.

That last point is the qualifier. Duplication in one place is verbose. Duplication
across files is a bug waiting to happen. Treat them differently.

---

## 2. Abstraction is earned, not assumed

### 2.1 Write the first three inline

Do not extract on the second occurrence. Two things that look alike often are not.

The signature-verification block in this codebase reached **five verbatim copies**
before extraction was scheduled — T2, T3, T4/T5, T6, T7/T8. By then its shape was
obvious from five real examples rather than guessed from one.

### 2.2 Extract for testability, which is a different reason

```go
// Before — buried inline in the upload handler, testable only by uploading files
safe := filepath.Base(header.Filename)
safe = strings.Map(func(r rune) rune { ... }, safe)
// ...
```

```go
// After — pure input → output, five table-driven test cases
func sanitizeFilename(filename string) string { ... }
```

This is not "the block was repetitive." It occurred **once**. It was extracted
because testing it in place meant uploading files named `report"; drop.pdf` by
hand, and the edge cases — path traversal, a 300-character name, `filepath.Base("")`
returning `"."` — are exactly the ones you want covered by a test.

Repetition is one reason to extract. Testability is another. Size is not.

### 2.3 Extract when two call sites are provably identical

`toChangeControlResponse` was left inline at first, deliberately, on the assumption
that the list endpoint would share it. It did not — the list needed a 10-field
summary, not the 54-field record.

It was extracted only when `GET /{ccID}` and `PUT /{ccID}` were observed returning
**byte-identical bodies**. Two real call sites, not a predicted three.

### 2.4 When an abstraction cannot do everything, keep the rest at the call site

The signature block writes an audit row **and** responds **and** returns. A helper
cannot do all three cleanly — a function that writes to a `ResponseWriter` and also
returns a value is doing two jobs.

```go
// The shape to aim for
ok, err := cfg.verifySignature(r.Context(), cc, user, email, password, now)
if err != nil { → 500 }
if !ok {
    respondWithError(w, "Invalid credentials", http.StatusUnauthorized)
    return
}
```

The helper performs the checks and the audit write. `respondWithError` stays where
the reader expects it — in the handler, next to every other response.

---

# Part II — Data

## 3. Let the database enforce what it can

### 3.1 Constraints are the guarantee; Go is the error message

```sql
current_state TEXT NOT NULL DEFAULT 'Initiated'
    CONSTRAINT ck_cc_current_state CHECK (current_state IN (
        'Initiated', 'Pending Implementation Approval', ...
    ))
```

```go
// And in the handler, before the query runs
switch s {
case stateInitiated, statePendingImplApproval, ...:
default:
    → 400 "Invalid state"
}
```

Both. The CHECK constraint is what makes an invalid state _impossible_, including
from psql or a future service. The Go check exists so the client gets a clean 400
naming the field, rather than a constraint violation surfacing as a 500.

**Validate what you can name; let the constraint be the backstop.**

### 3.2 Generate identifiers in the database

```sql
CREATE SEQUENCE cc_number_seq;

cc_number BIGINT NOT NULL DEFAULT nextval('cc_number_seq'),
cc_id TEXT NOT NULL GENERATED ALWAYS AS (
    'CC-' || CASE WHEN cc_number < 1000
                  THEN LPAD(cc_number::text, 3, '0')
                  ELSE cc_number::text
             END
) STORED,
```

```go
// The insert never mentions cc_id
INSERT INTO change_controls (change_owner_id, last_updated_by_id)
VALUES ($1, $2) RETURNING *;
```

No Go code participates in ID generation, which is exactly what makes it
collision-free under concurrency. Two simultaneous creates cannot produce the same
CC-ID because neither one computes it.

Accept the consequence: `nextval` is non-transactional, so a rolled-back create
burns a number. **Gaps in CC-IDs are expected and permanent.** That is the correct
trade — a gap is cosmetic, a duplicate is a data integrity failure.

### 3.3 Never check-then-act on uniqueness

```go
// Avoid — TOCTOU race
_, err := q.GetUserByEmail(ctx, email)
if err == nil {
    respondWithError(w, "Email already exists", 409)
    return
}
q.CreateUser(ctx, params)
```

```go
// Prefer — let the unique index decide
newUser, err := qtx.CreateUser(ctx, params)
if err != nil {
    var pqErr *pq.Error
    if errors.As(err, &pqErr) && pqErr.Code == "23505" {
        respondWithError(w, "A user with that email already exists", 409)
        return
    }
    → 500
}
```

Between the SELECT and the INSERT, another request can take the email. The unique
index is the only thing that can decide this correctly, so let it and translate the
violation.

Note `errors.As`, not `errors.Is` — you need the _value_ to read `.Code`.

### 3.4 Use `ON CONFLICT` when the row may or may not exist

```sql
INSERT INTO file_attachments (change_control_id, field_name, file_name, ...)
VALUES ($1, $2, $3, ...)
ON CONFLICT (change_control_id, field_name) DO UPDATE
SET file_name = EXCLUDED.file_name,
    file_data = EXCLUDED.file_data,
    uploaded_on = NOW()
RETURNING id, change_control_id, field_name, file_name, ...;
```

"Replace on re-upload" becomes one atomic statement instead of a SELECT and a
branch. `EXCLUDED` refers to the row that was attempted.

Two details that matter:

- **`uploaded_on = NOW()` explicitly.** The column's `DEFAULT NOW()` fires only on
  INSERT, so without this line a replacement keeps the original timestamp.
- **The unique constraint is load-bearing, not protective.** Postgres detects the
  conflict via that index. Remove it and this statement does not lose a safety
  check — it fails to run.

### 3.5 Choose the right tool per situation

| Situation                                     | Approach                          |
| --------------------------------------------- | --------------------------------- |
| Must not already exist                        | plain INSERT, catch `23505` → 409 |
| Definitely exists (already loaded and locked) | plain UPDATE                      |
| Either, and you do not care which             | `ON CONFLICT DO UPDATE`           |

---

## 4. Queries

### 4.1 sqlc does not validate SQL — Postgres does

```sql
-- This generated compiling Go containing broken SQL
UPDATE users SET is_active = $2, updated_one = NOW() WHERE id = $1
--                              ^^^^^^^^^^ column does not exist
```

sqlc parses the schema well enough to generate structs, but it does **not** verify
every column reference — particularly in an `UPDATE ... SET` clause. The typo above
produced a function that compiled and would have failed at runtime as a 500.

**Run every new query through psql before wiring the handler.** Thirty seconds, and
it is the only thing that checks the SQL is real.

### 4.2 Read the generated struct; do not assume the parameter order

```go
// The SQL says LIMIT then OFFSET. The struct says:
type ListChangeControlsParams struct {
    OwnerID  *uuid.UUID
    // ...
    Offset   int32   // ← $7
    Limit    int32   // ← $8
}
```

sqlc numbers named parameters in the order it encounters them, which is not always
source order. Harmless with keyed struct literals; silently catastrophic if you
ever pass positionally.

### 4.3 `ORDER BY` is mandatory whenever there is a `LIMIT`

```sql
-- Avoid
SELECT * FROM users LIMIT $1 OFFSET $2;
```

```sql
-- Prefer
SELECT * FROM users ORDER BY full_name LIMIT $1 OFFSET $2;
```

Without a deterministic sort, Postgres may return rows in a different order between
calls — so page 2 can repeat or skip rows from page 1. This is not a style point;
pagination is incoherent without it.

### 4.4 A count query and its list query must share the same `WHERE`

```sql
-- name: ListUsers :many
SELECT * FROM users
WHERE (sqlc.narg('is_active')::boolean IS NULL OR is_active = sqlc.narg('is_active'))
ORDER BY full_name LIMIT $1 OFFSET $2;

-- name: CountUsers :one
SELECT COUNT(*) FROM users
WHERE (sqlc.narg('is_active')::boolean IS NULL OR is_active = sqlc.narg('is_active'));
```

Copy-paste the `WHERE` block rather than retyping it. If the two drift, `total`
describes a different set than the page — and every pager in the UI is silently
wrong with no error anywhere.

The same applies to the FROM and JOIN block: keep it identical even when the count
does not select the joined columns, or adding a filter that references a joined
table later breaks one query and not the other.

### 4.5 Optional filters: `sqlc.narg` and `IS NULL OR`

```sql
WHERE 1=1
  AND (sqlc.narg('owner_id')::uuid IS NULL OR cc.change_owner_id = sqlc.narg('owner_id'))
  AND (sqlc.narg('state')::text   IS NULL OR cc.current_state   = sqlc.narg('state'))
  AND (sqlc.narg('search')::text  IS NULL
       OR cc.cc_id        ILIKE '%' || sqlc.narg('search') || '%'
       OR cc.change_title ILIKE '%' || sqlc.narg('search') || '%'
       OR owner.full_name ILIKE '%' || sqlc.narg('search') || '%')
```

`sqlc.narg` generates a **pointer** parameter, so `nil` makes the condition
`NULL IS NULL` — trivially true — and the filter disappears. Six independent
filters, any combination, from one query. The alternative is a query per
permutation.

Three things to note:

- **`WHERE 1=1`** so every real condition can be uniformly prefixed with `AND` and
  the first one needs no special case.
- **The `::type` cast** is required — Postgres cannot infer a parameter's type from
  `IS NULL` alone.
- **The parentheses around the search group are load-bearing.** `AND` binds tighter
  than `OR`; without them the owner and state filters get swallowed into the OR
  chain and stop applying.

### 4.6 Put a join's own condition in `ON`, not `WHERE`

```sql
-- Avoid — silently turns the LEFT JOIN back into an inner join
LEFT JOIN file_attachments ev ON ev.change_control_id = cc.id
WHERE ev.field_name = 'implementation_evidence'
```

```sql
-- Prefer
LEFT JOIN file_attachments ev ON ev.change_control_id = cc.id
                             AND ev.field_name = 'implementation_evidence'
```

In the `WHERE`, rows where the join found nothing are filtered out — so every
change control without a file disappears from the result entirely.

### 4.7 `INNER JOIN` for NOT NULL, `LEFT JOIN` for nullable

```sql
JOIN      users owner    ON owner.id    = cc.change_owner_id       -- NOT NULL + FK
LEFT JOIN users approver ON approver.id = cc.assigned_approver_id  -- nullable
```

An inner join on `assigned_approver_id` would return **zero rows** for every record
in `Initiated`, since none has an approver yet. Choosing per column also lets sqlc
emit `string` rather than `*string` for the guaranteed ones.

### 4.8 Never select bulk columns in a query that runs often

```sql
-- The CC read, which runs on every fetch, every save and every transition
SELECT ev.file_name, ev.file_size, ev.content_type, ev.uploaded_on
--     never ev.file_data
```

`file_data` is up to 10 MB. Exactly one query in this codebase selects it — the
download endpoint — because that is the only place the bytes are wanted.

### 4.9 `sqlc.embed` when a row carries a whole table plus extras

```sql
SELECT sqlc.embed(cc),
       owner.full_name AS owner_name,
       updater.full_name AS updater_name
FROM change_controls cc
    JOIN users owner   ON owner.id   = cc.change_owner_id
    JOIN users updater ON updater.id = cc.last_updated_by_id
```

```go
type GetChangeControlByCcIDRow struct {
    ChangeControl ChangeControl  // ← the whole 50-field struct, nested
    OwnerName     string
    UpdaterName   string
}
```

Without `embed`, all 50 columns flatten into the row struct alongside the joined
names and the mapper reads them at one level. With it, `row.ChangeControl` is the
type you already have.

### 4.10 Index because a query needs it, not because a document lists it

Two timestamp indexes in this schema, added for different reasons:

- **`idx_cc_created_on`** came from the design document before the queries existed.
  It is used only by the two optional date filters.
- **`idx_cc_last_updated_on`** was added in migration 007 because **five** queries
  sort by that column and none had an index — including one with no `WHERE` clause
  at all, which would otherwise sort the whole table to return five rows.

State the cost when you add one. `last_updated_on` changes on every save, every
transition and every upload, so this index is maintained on every write. That is
acceptable here because B-tree maintenance is microseconds against argon2id's
~200 ms, and reads vastly outnumber writes — but say so rather than assuming
indexes are free.

---

## 5. Nullability

### 5.1 Pointers, not `sql.NullString`

```go
// Avoid
ChangeTitle sql.NullString
if cc.ChangeTitle.Valid { use(cc.ChangeTitle.String) }
```

```go
// Prefer
ChangeTitle *string
if cc.ChangeTitle != nil { use(*cc.ChangeTitle) }
```

Pointers marshal to `null` in JSON without a custom marshaller, compare with a
plain `!= nil`, and read the same as every other optional value in the language.

### 5.2 `emit_pointers_for_null_types` is inert under `database/sql` — use overrides

```yaml
emit_pointers_for_null_types: true # silently ignored with lib/pq
overrides:
  - db_type: "text"
    nullable: true
    go_type: { type: "string", pointer: true }
```

Every nullable type needs an explicit override, and each one only surfaces when a
query first makes that type nullable. The settled spellings, found empirically:

| Type        | `db_type`         |
| ----------- | ----------------- |
| TEXT        | `text`            |
| TIMESTAMPTZ | `timestamptz`     |
| DATE        | `date`            |
| UUID        | `uuid`            |
| TIME        | `pg_catalog.time` |
| BOOLEAN     | `pg_catalog.bool` |
| BIGINT      | `pg_catalog.int8` |

There is no pattern to which need the prefix. Try the bare name, then
`pg_catalog.`, and record the answer.

### 5.3 Nullability lives on the outer pointer, not the inner fields

```go
// Prefer
type FileRef struct {
    FileName    string    `json:"file_name"`     // plain
    FileSize    int64     `json:"file_size"`     // plain
    ContentType string    `json:"content_type"`  // plain
    UploadedOn  time.Time `json:"uploaded_on"`   // plain
}

ImplementationEvidence *FileRef `json:"implementation_evidence"`  // ← nullable here
```

If the reference exists at all, every field is populated — they come from one row
that either matched or did not. Making the inner fields pointers too would imply a
half-present file, which cannot happen.

### 5.4 Guard every dereference, even when the invariant says you need not

```go
// Prefer
if row.EvidenceFileName != nil && row.EvidenceFileSize != nil &&
   row.EvidenceContentType != nil && row.EvidenceUploadedOn != nil {
    evidence = &FileRef{ FileName: *row.EvidenceFileName, ... }
}
```

All four columns are `NOT NULL` in the table, so a partial row should be impossible.
Relying on that turns one bad assumption into a panic. The verbose version degrades
to `null` instead.

### 5.5 Two small helpers, opposite directions

```go
func strPtr(s string) *string { return &s }   // value → pointer, for writing
func strValue(s *string) string {              // pointer → value, for reading
    if s != nil { return *s }
    return ""
}
```

`strPtr` exists because Go will not take the address of a literal — `&"is_active"`
does not compile, but `&s` inside a function does, since a parameter is a real
variable.

### 5.6 Comparison helpers for delta detection

```go
func sameStrPtr(a, b *string) bool {
    if a == nil || b == nil { return a == b }
    return *a == *b
}

func sameTimePtr(a, b *time.Time) bool {
    if a == nil || b == nil { return a == b }
    return a.Equal(*b)   // ← .Equal, not ==
}
```

Both nil means unchanged. One nil means changed. Both set means compare values.

`time.Time` must use `.Equal()` — `==` compares struct fields including the
monotonic clock reading and the location pointer, so two values representing the
same instant can compare unequal.

---

# Part III — Correctness under load

## 6. Transactions

### 6.1 The shape

```go
tx, err := cfg.rawDB.BeginTx(r.Context(), nil)
if err != nil {
    log.Error("...", "reason", "could not begin transaction", "error", err)
    respondWithError(w, "Something went wrong", http.StatusInternalServerError)
    return
}
defer tx.Rollback()

qtx := cfg.db.WithTx(tx)

// ... every database call below uses qtx ...

if err := tx.Commit(); err != nil { → 500 }
```

Five lines, and every transaction in this codebase is that skeleton with a
different middle.

### 6.2 `defer tx.Rollback()` on the line after `BeginTx`

Not later, not conditionally. `defer` runs on **every** return path — an early
validation failure, a 500, a panic — so no error branch needs its own rollback.

The part that looks wrong but is not: the deferred rollback still runs after a
successful commit. A committed transaction returns `sql.ErrTxDone`, which is a
no-op. That is precisely why the idiom works, and the one place ignoring an error
return is correct.

### 6.3 `qtx` inside, always

```go
// Wrong — runs on a different connection, commits immediately,
// and survives the rollback
err = cfg.db.InsertAuditLog(...)
```

```go
// Right
err = qtx.InsertAuditLog(...)
```

A stray `cfg.db` inside a transaction produces no error and no warning. It just
silently escapes, giving you exactly the split state the transaction exists to
prevent.

Name the variable `qtx` so `cfg.db` between `BeginTx` and `Commit` looks wrong at a
glance.

### 6.4 The one deliberate exception — and comment it

```go
if !match {
    // written with cfg.db, NOT qtx — this must survive the rollback (FR-6.2.31)
    err = cfg.db.InsertAuditLog(r.Context(), database.InsertAuditLogParams{
        ActionType: actionSignatureFailed, ...
    })
    respondWithError(w, "Invalid credentials", http.StatusUnauthorized)
    return
}
```

A failed e-signature must be recorded **and** the record left untouched. Written
with `qtx`, the same rollback that reverts the transition would discard the audit
row, leaving no evidence of the attempt.

This is the only place in the codebase where `cfg.db` appears inside a transaction,
and it carries a comment saying so — otherwise a future reader will "fix" it.

### 6.5 A transaction is for multiple writes, not for reads

```go
// No transaction — two reads, nothing to make atomic
id, err := cfg.db.GetChangeControlIDByCcID(r.Context(), ccID)
row, err := cfg.db.GetFileAttachment(r.Context(), params)
```

The download and dashboard handlers open no transaction at all. Ceremony without
purpose is noise.

### 6.6 Read the response **inside** the transaction, before the commit

```go
// Avoid
tx.Commit()
row, err := cfg.db.GetChangeControlByCcID(ctx, ccID)   // ← if this fails, the
if err != nil { → 500 }                                //   write already landed
```

```go
// Prefer
row, err := qtx.GetChangeControlByCcID(ctx, ccID)      // reads its own writes
if err != nil { → 500 }
if err := tx.Commit(); err != nil { → 500 }
respondWithJSON(w, http.StatusOK, toChangeControlResponse(row))
```

Reading after the commit leaves a window where the write succeeded but the client
sees a 500. For a save that is merely confusing. For a **transition** it is worse:
the client retries and gets a **409**, because the state has already moved — so
they are told "something went wrong" and then "this is not in the right state."

Reading before the commit means any failure leaves nothing written. **The error and
the record's state agree.**

A transaction sees its own uncommitted writes, so the joined read still returns
post-update values.

### 6.7 Write and audit atomically

```go
newUser, err := qtx.CreateUser(ctx, params)
if err != nil { → 409 or 500; the deferred rollback discards it }

err = qtx.InsertAuditLog(ctx, auditParams)
if err != nil { → 500; the user creation is rolled back too }

tx.Commit()
```

If the audit insert can fail while the write succeeds, the system can hold a change
with no audit trail. In a regulated context an unaudited change effectively did not
happen — so it must not happen at all.

---

## 7. Locking

### 7.1 A transaction gives atomicity, not isolation from writers

Under Read Committed — Postgres's default — a plain `SELECT` inside a transaction
takes a snapshot and another session can update that row a millisecond later. The
transaction guarantees your writes land together. It does not stop anyone else.

### 7.2 `FOR UPDATE` whenever you read, decide, then write

```go
cc, err := qtx.GetChangeControlForUpdate(ctx, ccID)   // 1. read  🔒
if cc.CurrentState != stateInitiated { → 409 }        // 2. decide
qtx.UpdateChangeControlDraft(ctx, params)             // 3. write
```

The decision at step 2 is based on what step 1 read. Without a lock, something can
change the row in between and the decision was made on facts that are no longer
true.

The `UPDATE`'s own implicit lock arrives too late — it protects the instant of
writing, not the reasoning that led there.

| Situation                          | Lock?                               |
| ---------------------------------- | ----------------------------------- |
| Read, decide, then write           | **Yes** — `FOR UPDATE` at the read  |
| Blind write with no prior decision | No — the UPDATE's own lock suffices |
| Read-only                          | No                                  |

### 7.3 The lock lives until the transaction ends

```go
tx, _ := cfg.rawDB.BeginTx(...)
cc, _ := qtx.GetChangeControlForUpdate(...)   // 🔒 taken

qtx.UpsertFileAttachment(...)                 // still locked
qtx.TouchChangeControl(...)                   // still locked

tx.Commit()                                   // 🔓 released here, and only here
```

`COMMIT` or `ROLLBACK` releases it — nothing else. Outside a transaction every
statement auto-commits, so a lock would be taken and released in the same instant
and protect nothing.

### 7.4 What it blocks

| Another session does                | Result                                       |
| ----------------------------------- | -------------------------------------------- |
| plain `SELECT` on that row          | **not blocked** — reads the pre-update value |
| `SELECT ... FOR UPDATE` on that row | waits                                        |
| `UPDATE` / `DELETE` on that row     | waits                                        |
| anything on a different row         | not blocked                                  |

Row-level, not table-level. Plain readers never block, so a `GET` running
concurrently with a save is unaffected.

### 7.5 Keep the locking read separate from the ordinary one

```sql
-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;

-- name: GetUserForUpdate :one
SELECT * FROM users WHERE id = $1 FOR UPDATE;
```

Same SELECT, opposite intent. `GetUserByID` runs in `middlewareAuth` on **every**
authenticated request and must not lock — otherwise every request in the system
serialises through a row lock. Two queries, deliberately.

### 7.6 Never hold a lock across a slow operation

```go
// File is read and validated BEFORE the transaction opens
data, err := io.ReadAll(file)
if len(data) > maxUploadBytes { → 400 }
if http.DetectContentType(data) != contentTypePDF { → 400 }

tx, _ := cfg.rawDB.BeginTx(...)      // ← lock taken only now
cc, _ := qtx.GetChangeControlForUpdate(...)
```

A 10 MB upload over a slow connection takes seconds. Holding a database row lock
for that duration would block every other write to that record.

The accepted cost: a non-owner's upload is fully processed before the 403. Wasting
work on a doomed request is cheaper than holding a lock.

---

## 8. Concurrency patterns

### 8.1 Assume two requests, not one

The realistic trigger is not two users racing. It is **one user with two browser
tabs**, a double-clicked button, or a client retry.

```
t1  Upload   SELECT state → "In Implementation" ✓
t2                              T6: locks, verifies evidence, transitions, COMMITS
t3  Upload   upserts the file  ← evidence replaced on a record already submitted
```

Both actions are legal. Neither user did anything wrong. The approver would then
review a file the owner never signed for.

### 8.2 The catalogue

| Hazard                                         | Mechanism                                          |
| ---------------------------------------------- | -------------------------------------------------- |
| Duplicate identifiers under concurrent creates | sequence + generated column, no Go involvement     |
| Duplicate uniques between check and insert     | unique index + catch `23505`, never check-then-act |
| Stale decision between read and write          | `SELECT ... FOR UPDATE` inside a transaction       |
| Split state between a write and its audit row  | one transaction, all-or-nothing                    |
| Row may or may not exist                       | `ON CONFLICT DO UPDATE`                            |
| A record that must survive a rollback          | `cfg.db`, deliberately outside the transaction     |

### 8.3 Prefer letting the database resolve a race to detecting it

Every entry above pushes the resolution into Postgres rather than coordinating in
Go. That is not laziness — Postgres can do it atomically and application code
cannot.

---

# Part IV — The HTTP layer

## 9. Handlers

### 9.1 Two signatures, and the second is a compile-time guarantee

```go
// Public
func (cfg *apiConfig) HandlerLogin(w http.ResponseWriter, r *http.Request)

// Authenticated — the verified user arrives as an argument
type authedHandler func(http.ResponseWriter, *http.Request, database.User)
func (cfg *apiConfig) HandlerGetMe(w http.ResponseWriter, r *http.Request, user database.User)
```

```go
// This does not compile
mux.HandleFunc("GET /api/me", cfg.HandlerGetMe)
// cannot use cfg.HandlerGetMe (value of type func(..., database.User))
//   as func(http.ResponseWriter, *http.Request) value
```

A three-parameter handler is not a valid `http.HandlerFunc`, so the only way to
route it is through `middlewareAuth`, which supplies the user. **Forgetting
authentication is a build error, not a runtime hole.**

That is the whole design: the compiler enforces it, not discipline.

### 9.2 Three registration variations, and no others

```go
// 1 — public
mux.Handle("POST /api/login",
    cfg.middlewareLogging(http.HandlerFunc(cfg.HandlerLogin)))

// 2 — authenticated
mux.Handle("GET /api/me",
    cfg.middlewareLogging(cfg.middlewareAuth(cfg.HandlerGetMe)))

// 3 — authenticated + role
mux.Handle("POST /api/users",
    cfg.middlewareLogging(cfg.middlewareAuth(cfg.requireRole(roleAdmin, cfg.HandlerCreateUser))))
```

Variation 2 needs no `http.HandlerFunc` wrap — `middlewareAuth` already returns a
handler.

### 9.3 Pass the request logger by context; pass the user by argument

```go
// Logger — context. Its absence is harmless.
log := logging.LoggerFrom(r.Context())   // falls back to slog.Default()

// User — argument. Its absence must be a compile error.
func (cfg *apiConfig) HandlerX(w, r, user database.User)
```

Same program, two mechanisms, chosen by what happens when the value is missing.
A missing logger degrades. A missing user is a security hole.

**Match the mechanism to the stakes of its absence.**

### 9.4 Never marshal a database struct

```go
// Avoid — hashed_password goes out over the wire
respondWithJSON(w, 200, user)
```

```go
// Prefer
type UserResponse struct {
    ID       uuid.UUID `json:"id"`
    FullName string    `json:"full_name"`
    Email    string    `json:"email"`
    Role     string    `json:"role"`
}
respondWithJSON(w, 200, UserResponse{ ID: user.ID, ... })
```

Beyond the obvious secret: a database struct has no JSON tags, so Go emits
PascalCase field names; it exposes internal columns like `cc_number`; and it
couples the API contract to the schema, so adding a column silently changes the
response.

Where a query can avoid selecting the column at all, prefer that — the approver
list selects only `id, full_name`, so `hashed_password` never enters the result
set rather than being dropped later.

### 9.5 Name the third parameter for its role in _this_ handler

```go
func (cfg *apiConfig) HandlerCreateUser(w, r, admin database.User)         // actor
func (cfg *apiConfig) HandlerImplementationDecision(w, r, approver database.User)
```

In the `authedHandler` pattern the user is always **who is acting**, never who is
acted upon. An early draft of `HandlerCreateUser` returned the _admin's_ record as
the 201 body because both `user` and `newUser` were in scope and plausible.
Renaming the parameter makes that class of mistake visible.

### 9.6 Reject on the cheapest available information first

```
path parameter  → 400   (no I/O)
decode body     → 400   (no I/O)
field validation→ 400   (no I/O)
── transaction opens, row lock taken ──
record exists   → 404
ownership       → 403
state           → 409
```

Self-deactivation is caught before `BeginTx`, because there is no point locking a
row to refuse something already known invalid.

### 9.7 Ownership before state

```go
if cc.ChangeOwnerID != user.ID          { → 403 }
if cc.CurrentState != stateInitiated     { → 409 }
```

If someone else's record happens to be in the wrong state, they should be told
"not yours" rather than "wrong state" — the latter confirms the record exists and
reveals its stage.

Same reasoning as verifying the password before the `is_active` check at login.

---

## 10. Authorisation

### 10.1 Role in middleware, ownership and state in the handler

```go
// Role — a property of the user alone
cfg.requireRole(roleAdmin, cfg.HandlerCreateUser)

// Ownership — a property of the record
if cc.ChangeOwnerID != user.ID { → 403 }

// Assignment — also a property of the record
if cc.AssignedApproverID == nil || *cc.AssignedApproverID != approver.ID { → 403 }
```

`requireRole` cannot express "is this the _assigned_ approver" — that needs the
record, which the middleware has not loaded. So record-level checks live in the
handler.

Note the nil check first. Short-circuit evaluation means the dereference runs only
when the pointer is non-nil; a broken invariant would otherwise panic.

### 10.2 `requireRole` is `authedHandler` in and out

```go
func (cfg *apiConfig) requireRole(role string, next authedHandler) authedHandler
```

Three parameters on both sides, which is what lets it nest **inside**
`middlewareAuth`. And because the return type is already a func type, no
`http.HandlerFunc` conversion is needed — unlike the other two middlewares.

### 10.3 The params struct is a permission boundary

```go
type UpdateChangeControlDraftParams struct {
    // 24 editable fields
    LastUpdatedByID uuid.UUID
    CcID            string
}
// decision, risk_level, current_state, actual_closure_date: absent
```

A field absent from the params struct is a field that endpoint **physically cannot
write**. That is a stronger guarantee than a runtime check, because there is no
code path to get it wrong.

### 10.4 The API is the boundary; the UI is a convenience

The frontend hides the pencil for your own row. The API still rejects a
self-role-change. The frontend will not offer "Approve" on a closed record. The API
still returns 409.

Anyone with a token and curl can call any endpoint in any order. **Every rule that
matters lives in the backend**; the UI's job is making the legal paths pleasant.

### 10.5 Layer the same rule, deliberately

| Layer    | Example                                                              |
| -------- | -------------------------------------------------------------------- |
| Database | `ck_users_role` rejects an invalid role, even from psql              |
| Handler  | validates the role first, so the client gets a 400 rather than a 500 |
| Frontend | a dropdown, so it never arises                                       |

Remove the top two and the system is still correct, with worse error messages.
Remove the bottom one and it is not.

---

## 11. Validation

### 11.1 Format at save; presence at transition; business rules only when they apply

| Check                               | Save                                 | Transition                                                                        |
| ----------------------------------- | ------------------------------------ | --------------------------------------------------------------------------------- |
| Length, enum values, JSON types     | ✅                                   | ❌ — an invalid value cannot be stored, so re-checking is checking the impossible |
| Presence                            | ❌ — a draft is incomplete by nature | ✅                                                                                |
| Time-relative rules (business days) | ❌                                   | ✅ — validity changes as time passes                                              |
| Signature                           | ❌                                   | ✅                                                                                |

Neither endpoint checks everything, and that is deliberate. A draft must be
saveable while incomplete, and a date valid on Monday may be invalid by Thursday —
so it can only be judged at the moment of submission.

### 11.2 Collect all failures; never stop at the first

```go
// Avoid
if cc.ChangeTitle == nil {
    respondWithError(w, "Change Title is required", 400)
    return
}
```

```go
// Prefer
var problems []string
if cc.ChangeTitle == nil       { problems = append(problems, "Change Title") }
if cc.ChangeDescription == nil { problems = append(problems, "Change Description") }
// ... twenty checks ...

if cc.ProposedImplementationDate != nil &&
   cc.ProposedImplementationDate.Before(businessDaysFrom(today, 2)) {
    problems = append(problems, "Proposed Implementation Date must be at least 2 business days from today")
}

if len(problems) > 0 {
    respondWithJSON(w, http.StatusBadRequest, validationErrorResponse{
        Error:  "Cannot submit: some requirements are not met",
        Issues: problems,
    })
    return
}
```

Fail-fast makes the user fix one field, resubmit, and discover the next. Twenty
fields is twenty round trips.

Business rules append to the **same** list as presence checks, so a user with two
bad dates learns both at once.

### 11.3 Snake_case in logs, human labels in responses

```go
log.Warn("save draft failed", "reason", "comments_for_approver must be 2000 characters or fewer")
respondWithError(w, "Comments for Approver must be 2000 characters or fewer", 400)
```

Logs are searched by column name. Responses are read by people. Different
audiences, different vocabulary.

The exception: when the response names **JSON keys the developer sent** — as when
rejecting non-editable fields — return the keys, since that is what they must
remove.

### 11.4 Return validation errors verbatim; hide internal ones

```go
// Validation — tell them exactly what is wrong
respondWithError(w, err.Error(), http.StatusBadRequest)
//   "invalid limit: must be a positive integer"

// System — log the detail, return nothing
log.Error("...", "error", err)
respondWithError(w, "Something went wrong", http.StatusInternalServerError)
```

"Log the real error, return a generic one" protects **internal** detail. It is not
a reason to withhold input feedback the caller needs to fix their request.

### 11.5 Count runes, not bytes

```go
// Avoid — a pasted em-dash or accented character counts as 2–3
if len(*v) > 200 { → 400 }
```

```go
// Prefer
if len([]rune(*v)) > 200 { → 400 }
```

Otherwise a visibly 198-character title is rejected as "over 200" and the user
cannot see why.

### 11.6 Never trim a password

```go
reqBody.Email = strings.TrimSpace(reqBody.Email)   // yes
// reqBody.Password — never
```

Leading and trailing spaces are legitimate password characters. Trimming at
creation but not at login means an account that can never be signed into, with no
error anywhere.

Found by comparing two handlers, not by a test.

### 11.7 Reject unknown keys where they are plausibly confusable

```go
var invalidFields []string
for key := range body {
    if _, ok := draftEditableFields[key]; !ok {
        invalidFields = append(invalidFields, key)
    }
}
if len(invalidFields) > 0 {
    sort.Strings(invalidFields)   // map iteration order is random
    → 400 with the list
}
```

Ignoring unknown fields is the conventional behaviour and usually right. But Save
Draft accepts 24 keys out of 50 possible columns, so a client sending
`{"decision":"Approve"}` could reasonably believe it applied — and got a 200 while
nothing was written.

Applied to that endpoint only. A three-field login body is not confusable, and
strictness there would break on a harmless extra field.

`sort.Strings` because map iteration order is randomised; without it the same
request returns the same fields in a different order each time.

---

## 12. Errors and status codes

### 12.1 The table

| Code    | Means                              | Example                                              |
| ------- | ---------------------------------- | ---------------------------------------------------- |
| **400** | The request is malformed           | bad JSON, invalid enum, missing field                |
| **401** | I do not know who you are          | no token, expired token, bad credentials             |
| **403** | I know who you are and you may not | wrong role, not the owner, not the assigned approver |
| **404** | It does not exist                  | unknown CC-ID, unknown user                          |
| **409** | Valid request, wrong state         | double submit, editing a closed record               |
| **500** | The system broke                   | database down, commit failed                         |

### 12.2 401 and 403 answer different questions

```go
// middlewareAuth — "who are you?"
log.Warn("auth failed", "reason", "invalid jwt")
→ 401

// requireRole / ownership — "you, specifically, may not"
log.Warn("authorization failed", "reason", "insufficient role",
         "required", role, "actual", user.Role)
→ 403
```

A CC Owner calling an approver endpoint on a record in the correct state gets
**403**. They are perfectly authenticated; they are simply not the assigned
approver.

### 12.3 409 is what makes a transition non-idempotent

```go
if cc.CurrentState != stateInitiated {
    respondWithError(w, "Submit only allowed at Initiated state", 409)
    return
}
```

`PUT` and `DELETE` are idempotent by HTTP contract; `POST` is not. A transition
**must not** be idempotent — a double-click would otherwise produce a second
e-signature. The state check is the guard, and 409 is the honest code for
"the request is fine, the resource is not in a state that permits it."

### 12.4 Make an operation idempotent when repeating it is harmless

```sql
UPDATE refresh_tokens SET revoked_at = NOW(), updated_on = NOW()
WHERE token = $1 AND revoked_at IS NULL;
```

Logout returns **204** whether the token existed, was already revoked, or never
existed. Two reasons: logging out should never fail, and identical responses stop a
prober learning whether a token is real.

`AND revoked_at IS NULL` means a repeat call preserves the **original** logout
timestamp rather than overwriting it.

### 12.5 A malformed body is 400, not 500

```go
if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
    log.Warn("...", "reason", "malformed request body", "error", err)
    respondWithError(w, "Invalid request body", http.StatusBadRequest)
    return
}
```

Bad JSON is the client's fault. A 500 tells them it is yours.

### 12.6 Return identical responses for failures that must not be distinguished

```go
// Unknown email and wrong password both produce this
respondWithError(w, "Incorrect email or password", http.StatusUnauthorized)
```

Distinct messages let an attacker enumerate valid accounts. The **log** records
which it was; the client cannot tell.

The same reasoning extends to timing. An unknown-email path that skips password
hashing returns in 5 ms while a wrong-password path takes 137 ms — a measurable
signal. Hashing against a dummy hash on the unknown path equalises it:

```go
// generated once at startup, with the real params
_, _ = auth.CheckPasswordHash(password, cfg.dummyHash)
```

Measured: 5 ms / 137 ms before, 209 ms / 229 ms after.

### 12.7 Once the status is written, it cannot be changed

```go
w.Header().Set("Content-Length", strconv.FormatInt(row.FileSize, 10))
w.WriteHeader(http.StatusOK)          // ← "200 OK" has left the server

if _, err := w.Write(row.FileData); err != nil {
    // Cannot respond with 500 — the client already received 200.
    // The browser detects the truncation via Content-Length.
    log.Error("file download failed", "reason", "write to client failed", "error", err)
    return
}
```

Log it and return; there is nothing else available. All `w.Header().Set` calls must
precede `WriteHeader` — afterwards they silently do nothing.

---

## 13. JSON

### 13.1 Three intents, and a struct cannot express them

| Client sends       | Means           |
| ------------------ | --------------- |
| key absent         | leave unchanged |
| `"field": null`    | clear it        |
| `"field": "value"` | set it          |

```go
// Avoid — absent and null both decode to nil
type Request struct {
    ChangeTitle *string `json:"change_title"`
}
```

```go
// Prefer — the map's KEYS answer "was this sent?"
var body map[string]json.RawMessage
json.NewDecoder(r.Body).Decode(&body)

if raw, present := body["change_title"]; present {
    var v *string
    json.Unmarshal(raw, &v)     // JSON null → nil; "Fix" → &"Fix"
    // ...
}
```

`json.RawMessage` is `[]byte` with a marker meaning "hand me the bytes, do not
parse yet." Presence comes from the map; the value comes from a second decode.

This is RFC 7386 merge-patch semantics — the production standard for partial
updates.

**Rejected alternative:** requiring the client to always send every field. That
removes the ambiguity but rests on a _promise_; one partial body from a bug or a
second client silently wipes the omitted fields.

### 13.2 When every column is assigned, seed the params from the record

```go
// The UPDATE assigns all 24 columns unconditionally, so a field the client
// did not send must be re-written as-is or it would be nulled.
params := database.UpdateChangeControlDraftParams{
    CcID:            ccID,
    LastUpdatedByID: user.ID,
    ChangeTitle:     cc.ChangeTitle,       // ← current value
    // ... all 24 seeded from cc
}
```

Each field block then overwrites only its own entry, so **"leave unchanged" is the
default** and a block is the exception.

### 13.3 `COALESCE` is the wrong idiom here

```sql
-- Avoid
SET change_title = COALESCE(sqlc.narg('change_title'), change_title)
```

The usual partial-update trick — "use the new value if given, otherwise keep the
current one." It makes NULL mean _keep current_, so a field can never be cleared.
That is precisely the case the map decoding exists to support.

### 13.4 `make(..., 0, len)` so an empty list marshals as `[]`

```go
// Avoid — marshals as null, breaking a frontend calling .map()
var items []UserResponse
```

```go
// Prefer
items := make([]UserResponse, 0, len(rows))
```

### 13.5 No `omitempty` on a form-backed response

```go
ChangeTitle *string `json:"change_title"`   // no omitempty
```

A null field must appear as `"change_title": null`, not vanish. The form needs to
know the field exists and is empty.

### 13.6 Return `null`; do not substitute a placeholder

Normalising a missing title to `"(Untitled)"` destroys the distinction between "no
title" and a record literally named that — and presentation belongs to the client,
which may want different text on a card than in a table.

### 13.7 A fixed struct, not a map, when every key must appear

```go
// Prefer
type DashboardOverview struct {
    Initiated                     int64 `json:"initiated"`
    PendingImplementationApproval int64 `json:"pending_implementation_approval"`
    // ...five states
}

overview := DashboardOverview{}          // starts at zero
for _, row := range stateCounts {        // rows overwrite
    switch row.CurrentState { ... }
}
```

`GROUP BY` returns only states that **have** records. A state with none is simply
absent from the rows — so a map-based response would omit the key entirely and the
card would not render. Starting from a zero-valued struct is what guarantees all
five appear.

### 13.8 Wrap a top-level array

```go
// Avoid                          // Prefer
[ {...}, {...} ]                  { "signatures": [ {...} ] }
```

A bare array cannot grow. A wrapped one can gain a `total` or a `next` without a
breaking change.

### 13.9 Return both the ID and the display name

```json
"change_owner_id": "73960fc2-…",
"change_owner_name": "Default CC Owner"
```

**ID for comparisons** — `cc.change_owner_id === currentUser.id` decides whether a
button renders, and names are unstable (two people can share one, and this API can
change them). **Name for display** — the client cannot resolve a UUID on its own,
and a per-record lookup would be N+1.

### 13.10 Fold in what the page always needs; keep unbounded things separate

| Data                                                           | Where                  |
| -------------------------------------------------------------- | ---------------------- |
| Evidence metadata — 4 fields, always needed to render the form | **in** the CC response |
| Signature history — variable length, a collapsible panel       | separate endpoint      |
| File bytes — up to 10 MB, only on click                        | separate endpoint      |

Embedding the signature list would put it on every CC read, every save and every
transition, for a panel that is often closed. The client can fetch both in parallel
— one round trip's latency, not two — or defer the second until the panel opens,
which it cannot do if the data is baked in.

---

# Part V — Traceability

## 14. The audit trail

### 14.1 Verify the scope before designing the mechanism

**The most expensive near-miss in this project.** Four messages of design were built
on the assumption that every field a user edits is audited. The specification lists
**nine** fields, and states explicitly that non-critical field changes generate no
audit entry.

Building on the assumption would have put roughly forty spurious rows on a single
record and made the trail harder to read, not more complete.

It was caught by asking _"what is the audit entry even for?"_ rather than
_"how do I write it?"_

**Read the requirement. Then design.**

### 14.2 Record changes, not typing

```go
// Prefer — write and audit only on an actual delta
if !sameStrPtr(v, cc.ChangeTitle) {
    changed = true
}
```

Nine fields are tracked. The other forty-one are not, including every free-text
narrative. An audit row saying `full_name: "John Doe" → "John Doe"` is false
information in a regulatory record.

The same principle produces the **no-op branch**: an unchanged save commits without
running the UPDATE, so `last_updated_on` is not bumped for an edit that did not
happen.

### 14.3 `old_value` comes from the loaded record

```go
oldName := current.FullName          // captured BEFORE the update
current, err = qtx.UpdateUserName(...)

qtx.InsertAuditLog(..., OldValue: strPtr(oldName),
                        NewValue: strPtr(current.FullName), ...)
```

Capture old values immediately after the locking read, before anything reassigns
the record. Reading `current.FullName` after the update would record
`"Jane" → "Jane"`.

This is what makes the trail the **only** surviving record of an overwritten
decision:

```
15:28  decision  null   → Reject      ← first review
15:57  decision  Reject → Approve     ← re-review overwrites the record
```

The record now holds only `Approve`. The rejection exists nowhere else.

### 14.4 One action, one timestamp

```go
now := time.Now().UTC()          // captured once, before anything is written

qtx.InsertESignature(..., SignedOn: now)
qtx.InsertAuditLog(..., CreatedOn: now)   // StateChanged
qtx.InsertAuditLog(..., CreatedOn: now)   // FieldUpdated
qtx.InsertAuditLog(..., CreatedOn: now)   // SignatureCaptured
```

Multiple entries from one action must share one timestamp so they group when the
trail is read back. This is why `InsertAuditLog` takes `created_on` as a
**parameter** rather than defaulting to `NOW()`.

Same reason `implementation_approval_on` is a parameter: verified byte-identical
across three tables — `change_controls`, `audit_logs` and `esignatures` — at
`15:57:56.407064`.

`NOW()` inside a transaction returns the _transaction start_ time, which differs
from a Go timestamp by however long the request took.

| Situation                          | Use                               |
| ---------------------------------- | --------------------------------- |
| Must align with other rows         | Go's `now`, passed as a parameter |
| Standalone write, nothing to match | `NOW()` in the SQL                |

### 14.5 Denormalise names into audit rows

```go
PerformedByID:   admin.ID,
PerformedByName: admin.FullName,     // ← snapshot, not a join
```

The trail must be readable during an inspection without joins, and must record what
was true **at the time**. Verified accidentally: after renaming a user, their older
audit rows still showed the previous name. A live join would have silently
rewritten history.

The same applies to values: an approver change records
`null → Default Approver`, not a UUID. `4bba81a9-…` tells an auditor nothing.

### 14.6 Some records must survive the rollback

Covered in §6.4. The principle: **the transition is atomic, but the record of a
failed attempt must persist regardless.**

Verified: three failed signature attempts left three audit rows while
`current_state` never moved.

### 14.7 Do not audit what the audit cannot preserve

File uploads write **no** audit row. A row reading
`evidence-v1.pdf → evidence-v2.pdf` would advertise a gap rather than close one —
claiming a document existed while being unable to produce it, because the upsert
overwrote the bytes.

Preserving that history would need a versioned table, which is a different feature.
Silence is more honest than a half-truth.

---

## 15. Logging

### 15.1 Stable `msg`, varying `reason`

```go
// Avoid — every failure needs its own grep
log.Warn("email does not match")
log.Warn("password mismatch")
```

```go
// Prefer
log.Warn("submit failed", "reason", "email does not match")
log.Warn("submit failed", "reason", "password mismatch")
```

`msg` is the filter key: one grep returns every failure of an endpoint. `reason`
is the detail. Keep reasons terse and stable — `token revoked`, not
`refresh token revoked and not valid`.

### 15.2 Warn is the client's fault; Error is yours

```go
log.Warn("auth failed", "reason", "invalid jwt")            // expired token — normal
log.Error("auth failed", "reason", "user lookup failed")    // database is down
```

If you ever alert on `level=ERROR`, an expired session must not page anyone.

### 15.3 One concept, one key name

```go
"user_id"          // always the caller
"target_user_id"   // the user being acted upon
"cc_id"            // the business key, CC-001
"cc_uuid"          // the primary key
```

`middlewareAuth` already attaches `user_id` for the caller. Logging a different
value under the same key emits it twice in one line with two meanings.

### 15.4 Enrich the context logger once, benefit everywhere

```go
log = log.With("user_id", user.ID)
r = r.WithContext(logging.ContextWithLogger(r.Context(), log))
log.Info("authenticated", "role", user.Role, "email", user.Email)
next(w, r, user)
```

Two lines in `middlewareAuth`. Every subsequent handler line carries `user_id`
automatically, and the explicit `authenticated` line answers "who made this
request" even for handlers that log nothing of their own.

Note the limit: enrichment only reaches code that reads the logger from context
_after_ it — `middlewareLogging` holds its own variable from before, so its
"request finished" line does not carry `user_id`.

### 15.5 Log after the thing succeeded

```go
if err := tx.Commit(); err != nil { → 500 }
log.Info("cc draft saved", "cc_id", ccID)     // ← after
```

Logging before the commit reports a save that may then fail.

Where a write can fail after the response has started, name that state precisely:
`"file download failed"` with `reason: "write to client failed"` — not a success
line followed by an error.

### 15.6 Never log a secret

Passwords, refresh tokens, file bytes. A refresh token is a live 24-hour credential
that mints access tokens; writing one to `app.log` is equivalent to logging a
password.

```go
// Avoid
log.Warn("refresh failed", "reason", "token not found", "refresh_token", token)
```

`request_id` already ties the lines together, and `user_id` is the identifier you
would actually search on. If a token must be correlated, log a short prefix.

### 15.7 Log counts, not user content

```go
log.Warn("submit failed", "reason", "validation failed", "problem_count", len(problems))
```

The specifics are in the response. A search term or a validation message is
user-typed content with no reason to be in a log file.

### 15.8 Division of labour

**Middleware logs the request lifecycle and identity. Handlers log decisions and
failures.** A handler with no branches staying silent is correct, not a gap.

---

# Part VI — Working practice

## 16. Migrations

### 16.1 An applied migration is immutable

Goose records a checksum. Editing an applied file makes it fail to re-run, and any
other environment already has the old version. **Never edit; always add.**

`idx_cc_last_updated_on` became migration `007` rather than a line appended to
`002`, even though `002` created the table.

### 16.2 Always verify the down path

```bash
goose ... up      # apply
goose ... status  # confirm
goose ... down    # roll back
goose ... up      # re-apply
```

A migration whose `down` does not work is a migration you cannot roll back. Cheap
to check, expensive to discover during a failed deployment.

### 16.3 Name every constraint

```sql
-- Avoid — Postgres invents users_email_key
email TEXT NOT NULL UNIQUE

-- Prefer
CONSTRAINT uq_users_email UNIQUE (email)
```

Generated names are guessable but not guaranteed, and error handling that matches
on them is fragile. Naming also documents intent at the point of definition.

### 16.4 Constraints belong in the schema, not in application code

An enum enforced only in Go is enforced only for traffic that goes through Go.
psql, a future service, and a migration script all bypass it.

### 16.5 `ON DELETE RESTRICT` by default; `CASCADE` only for owned data

```sql
CONSTRAINT fk_cc_change_owner_id FOREIGN KEY (change_owner_id)
    REFERENCES users(id) ON DELETE RESTRICT      -- audit integrity

CONSTRAINT fk_file_attachments_change_control_id FOREIGN KEY (change_control_id)
    REFERENCES change_controls(id) ON DELETE CASCADE   -- the file belongs to the CC
```

An attachment has no meaning without its record. A user referenced by an audit row
must not be deletable at all.

### 16.6 Soft-reference audit rows

```sql
entity_id UUID NOT NULL,        -- deliberately NOT a foreign key
entity_type TEXT NOT NULL
```

`audit_logs` references both users and change controls in one column, so no single
FK can express it. And an audit row must outlive whatever it describes — that is
the point of an audit row.

---

## 17. Testing

### 17.1 Unit-test pure logic; exercise handlers by hand

```go
// Worth a test file — no dependencies, easy to get subtly wrong
func businessDaysFrom(start time.Time, n int) time.Time
func sanitizeFilename(filename string) string
func MakeJWT / ValidateJWT
func HashPassword / CheckPasswordHash
```

Handlers need a database, a transaction and a request. A full harness is worth
building for a long-lived system; it was not built here, and the endpoints were
verified through Postman against a real database with the results recorded.

Be honest about that trade rather than claiming coverage that does not exist.

### 17.2 Table-driven tests, one behaviour per name

```go
tests := []struct {
    name  string
    start time.Time
    n     int
    want  time.Time
}{
    {"friday plus 1 skips the weekend to monday", fri, 1, mon},
    {"saturday plus 1 is monday", sat, 1, mon},
    {"zero days returns the start unchanged", mon, 0, mon},
}
for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) { ... })
}
```

The name appears in the failure output, so it should describe the behaviour rather
than the inputs.

### 17.3 Add a property test where a table cannot cover the space

```go
func TestBusinessDaysFromNeverReturnsWeekend(t *testing.T) {
    for n := 1; n <= 40; n++ {
        got := businessDaysFrom(start, n)
        if got.Weekday() == time.Saturday || got.Weekday() == time.Sunday {
            t.Errorf("n=%d produced %s, a %s", n, got.Format("2006-01-02"), got.Weekday())
        }
    }
}
```

Rather than asserting specific dates, assert an **invariant** across many inputs.
Catches whole classes of error a table misses.

### 17.4 Test both sides of a boundary

```
proposed date = earliest − 1 day   → 400
proposed date = earliest exactly   → passes
```

One-sided boundary tests pass with an off-by-one bug in either direction.

### 17.5 Verify the data, not just the response

```sql
SELECT action_type, field_name, old_value, new_value, created_on
FROM audit_logs WHERE entity_id = (...) ORDER BY created_on;
```

A 200 with the right JSON says nothing about whether four audit rows landed with
one shared timestamp. Several defects in this project were visible only in psql —
including whether a `SignatureFailed` row survived a rollback.

### 17.6 Test the invisible paths deliberately

Some behaviour cannot be reached by using the system normally:

- A rejection loop, to prove `old_value` records the overwrite
- A CC with no evidence file, to prove the T6 check fires
- Three failed signatures, to prove the audit rows persist while the state does not
- A renamed PNG with a `.pdf` extension, to prove the sniff beats the extension

Set the state up by hand if necessary. Untested paths are where defects live.

---

## 18. Documentation discipline

### 18.1 Record decisions with their reasoning, including reversals

A decision log containing only successes is fiction. This project's log records
that the implementation-details save endpoint **reversed** an earlier decision, and
why the first call was wrong.

Without that, someone re-reading the original rationale would "fix" it back.

### 18.2 A flag is not a defect

Distinguish three things, and keep them apart:

|              |                                               |
| ------------ | --------------------------------------------- |
| **Decision** | settled, with reasoning                       |
| **Flag**     | known, deliberately deferred, with the reason |
| **Defect**   | wrong, needs fixing                           |

"Log rotation is not configured" is a flag with a stated reason and an estimated
cost. It is not a bug, and calling it one obscures the ones that are.

### 18.3 When the code departs from the specification, amend the specification

Eleven amendments came out of this build — narrowed file types, a corrected column
count, a removed contradiction, a rule the specification did not state.

Leaving them undone means the two authoritative documents disagree, and the next
person to read the spec will "correct" working code.

### 18.4 Comment the surprising, not the obvious

```go
// Avoid
// loop over the rows
for _, row := range rows { ... }
```

```go
// Prefer
// written with cfg.db, NOT qtx — this must survive the rollback (FR-6.2.31)

// ASCII HYPHEN. The BRD's table shows an en-dash (–, U+2013); the CHECK
// constraint requires "-". Copying from the BRD breaks every signature.

// GROUP BY returns only states that HAVE records, so a state with none is
// absent from the rows. Starting at zero is what makes all five cards render.
```

The test: would a competent reader be surprised, or reach for the "fix"? If so,
comment it. Otherwise the code says it already.

### 18.5 Cite the requirement

`(FR-6.2.31)`, `(BR-8.8.3)`, `(§6.6.2)`. It turns "why is this here?" into a
lookup, and marks the line as deliberate rather than incidental.

---

# Appendix A — Quick reference

## Non-negotiables

1. `defer tx.Rollback()` on the line after `BeginTx`
2. `qtx` inside a transaction — the one exception is commented
3. Read the response before the commit, not after
4. `FOR UPDATE` whenever you read, decide, then write
5. Never check-then-act on uniqueness — catch `23505`
6. Never marshal a database struct
7. Never log a password, a token, or file bytes
8. Never trim a password
9. `ORDER BY` with every `LIMIT`
10. Count and list queries share a `WHERE`
11. Run every new query through psql before wiring it
12. Verify the audit scope before designing the mechanism

## Judgement calls

| Question                       | Default                                                  |
| ------------------------------ | -------------------------------------------------------- |
| Extract this?                  | Not before the third copy — unless it needs a test       |
| Abstraction or repetition?     | Repetition, if it lives in one readable function         |
| Framework or standard library? | Standard library                                         |
| Validate here or there?        | Format at save, presence at transition                   |
| Fail fast or collect?          | Collect                                                  |
| Transaction?                   | Only for multiple writes                                 |
| Lock?                          | Only if you decide something between reading and writing |
| Audit it?                      | Only what the specification lists                        |
| Fold into the response?        | If it is bounded and always needed                       |

## The four failures that produced most of these rules

| What happened                                                   | The rule                                                       |
| --------------------------------------------------------------- | -------------------------------------------------------------- |
| Audit scope assumed as 24 fields; the spec says 9               | §14.1 — verify the requirement first                           |
| Password trimmed on create, not on login                        | §11.6 — never trim a password                                  |
| A `sqlc` query compiled with a non-existent column              | §4.1 — psql validates SQL, sqlc does not                       |
| A field omitted from a keyed struct literal wrote NULL silently | §9.4 — keyed literals are safe for ordering, weak for omission |

---

**End of guide.**

Each rule was arrived at by building something and finding out. Where a rule
proves wrong, change it — and record why, so the next reader inherits the
reasoning rather than the conclusion.
