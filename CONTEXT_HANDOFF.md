# EA QMS — Change Control: Frontend Phase Handoff

**Read this first.** It says where the project stands, which document answers
which question, what has already been decided, and how we work.

---

## 1. Where the project stands

**The backend is complete.** 23 endpoints, 7 tables, 8 workflow transitions, all
built and verified against a real database. Nothing about it is expected to
change during the frontend build.

**The frontend has not started.** No repository, no scaffold. The HTML prototypes
exist and define every screen; this build is a **port**, not a redesign.

### What exists

| | |
|---|---|
| **Go API** | `net/http` + PostgreSQL + sqlc. Runs on `localhost:1304`, mounted at `/api` |
| **Live API docs** | <https://lain-the-coder.github.io/ea-qms-backend/> — 23 operations, 46 schemas |
| **17 HTML prototypes** | Three role flows (Admin, CC Owner, Approver) + `global.css` |
| **Seeded data** | Four test users, ten change controls across five states |
| **Postman collection** | The only client that can call the API outside a browser |

### The workflow, in one picture

```
                    ┌──────────── T5 reject ───────────┐
                    ▼                                  │
  (T1) ──▶ Initiated ──T2──▶ Pending Impl Approval ────┤
              │                                   T4 approve
              │ T3                                     ▼
              ▼                            In Implementation ◀── T8 reject ──┐
          Cancelled                                    │                     │
                                                       │ T6                  │
                                                       ▼                     │
                                            Pending Final Approval ──────────┤
                                                       │                     │
                                                  T7 approve                 │
                                                       ▼                     │
                                                    Closed ──────────────────┘
```

**T1 is record creation and needs no signature.** T2–T8 each require an
e-signature. `Closed` and `Cancelled` are terminal.

---

## 2. Which document answers which question

Read the one that owns the question. When two disagree, the order below is the
precedence.

| Question | Document |
|---|---|
| *What does this endpoint accept and return?* | **`openapi.yaml`** — field names, nullability, enums, status codes. Verified against the handler code |
| *What must the client do, and in what order?* | **`FRONTEND_BLUEPRINT.md` Part A** — sequencing, traps, per-screen behaviour |
| *How do I build it in Svelte?* | **`FRONTEND_BLUEPRINT.md` Part B** — stack, conventions, structure, build order |
| *Which fields can this role edit in this state?* | **Security Matrix V2.1** |
| *What is the exact valid value for this dropdown?* | **`CC_Field_Reference.md`** — Canonical String Values. **Overrides the BRD** on the six hyphenated enums |
| *What is the business rule, and why?* | **BRD V1.2** — §2 workflow, §6 functional requirements, §8 business rules, §9 UI, §13 limitations |
| *What does this screen look like?* | **The HTML prototypes + `global.css`** |

**The API is the contract.** Where a document describes behaviour the API does
not implement, the API is right and the document is stale — but that should not
happen: all four guardrail documents were amended to V1.2 at backend completion
precisely so they agree with the code.

---

## 3. Decisions already made — do not re-litigate

### Stack

Svelte 5 (runes) · SvelteKit in **SPA mode** (`ssr = false`, `adapter-static`) ·
TypeScript · `global.css` unchanged · native `fetch` behind one wrapper · runes
only, **no `svelte/store`**.

**Every SvelteKit server feature is out** — `load`, form actions,
`+page.server.ts`, `+server.ts`, hooks, cookies. The Go API is the backend.
Anything labelled "server" in the SvelteKit documentation does not apply.

`FRONTEND_BLUEPRINT.md` §B3 lists the allowed feature set and §B4 the forbidden
one. Both are settled.

### Architecture

- **One page renders the CC form in every state, for every role.** The Security
  Matrix becomes `{#if}` and `disabled`. Do not create a page per state
- **No component extraction until the same markup is written inline three times**
- **The frontend is thin.** All correctness lives in the API. The client mirrors
  rules for UX — disabling a button the API would reject — but never treats its
  copy as authoritative

### Deferred to a later release

Password change · self-service profile editing · Supporting Documents (field 24)
· forgot-password · notifications by email · saved searches and reporting.

The prototypes contain screens for some of these. **They are not built.**

---

## 4. The five things most likely to cost a day

1. **The en-dash trap.** Six enum values render with `–` (U+2013) in the BRD and
   the prototypes; the database requires `-` (U+002D). Copying an `<option value>`
   from a prototype produces a 400 on every submission, with an error that does
   not mention the character. **Take enum values from `openapi.yaml`, never from
   the HTML.** Blueprint A4.

2. **Save-then-submit.** Transitions carry **no field values** — they validate
   what is already stored. Submit must be disabled while the form is dirty, or the
   API rejects fields the user can see filled in on screen. Blueprint A2.

3. **The file download cannot be an `<a href>`.** It needs the bearer token, and a
   link cannot send headers. `fetch` → `.blob()` → object URL. Blueprint A6.2.

4. **`null` clears a field; `""` is a parse error on dates.** Text fields accept
   both. Date and time fields accept only `null`. Blueprint A3 and A5.

5. **CORS fails confusingly.** A blocked request **reaches the server and
   executes** — the browser only refuses to hand the response to JavaScript
   afterwards. A write can succeed while the client sees a network error. Check
   the browser console before suspecting the API. Blueprint A12.

---

## 5. Running the backend

```bash
# Postgres must be running with the ea_qms database
cd ea-qms-backend
go run .                    # listens on :1304
```

`localhost:1304/docs` serves Swagger UI with **Try it out** working, because it is
same-origin with the API.

**Test users** — all with the same password, in the seed:

| Email | Role |
|---|---|
| `admin@eaqms.local` | Admin |
| `owner@eaqms.local` | CC Owner |
| `approver@eaqms.local` | Approver |
| `viewer@eaqms.local` | Viewer |

`http://localhost:5173` is already in the API's `ALLOWED_ORIGINS`, so the Svelte
dev server can call it.

---

## 6. How we work

**I write as much of the code as possible myself.** Teach concepts at the moment
of need, review my code like a senior engineer, and verify against the guardrail
documents. **Do not dump finished implementations unless I explicitly ask.**

**Work in chunks.** One screen, one concern at a time. Finish and verify before
moving on. Nothing gets written that I cannot explain.

**Check the documents rather than assuming.** If something is ambiguous, or two
documents disagree, **stop and ask** — do not guess and do not go beyond the
documented scope. Several real defects in the backend phase were caught exactly
this way, and at least three came from an assumption that was never verified
against the specification.

**Flag rather than invent.** If a rule seems missing, say so and let me decide.
Do not add behaviour the documents do not describe.

**Record decisions as they are made** — including reversals, with the reasoning.
A decision log containing only successes is fiction.

---

## 7. Build order

The blueprint's §B9 has fourteen steps, each independently verifiable against the
running API and each stating what it proves. The first four are infrastructure —
nothing renders until they work.

**Start at step 1: scaffold.**

The one ordering constraint worth knowing: **file upload cannot come before the
`In Implementation` view**, because the only upload field is writable only in
that state, which a record reaches only after two transitions.
