# EA QMS — Change Control Frontend Blueprint

**Version 1.0** · Written at backend completion, against the built API
Supersedes DRAFT V0.9

---

## How to read this

**Part A — The API contract.** Framework-agnostic. Everything the backend expects
of any client: request sequencing, traps, per-screen behaviour. This survives a
change of framework.

**Part B — The Svelte 5 build.** Stack, conventions, structure, build order.

**Neither part documents request and response shapes.** Those live in the OpenAPI
specification, which is generated from nothing and hand-written from the handler
code — it is the definitive source for field names, nullability, enum values and
status codes.

| Resource | Use it for |
|---|---|
| **<https://lain-the-coder.github.io/ea-qms-backend/>** | Reading the contract. Every endpoint, schema and example |
| **`localhost:1304/docs`** | The same, with **Try it out** working (same-origin with the API) |
| **`docs/openapi.yaml`** | The spec itself — searchable, diffable |
| **Postman collection** | Actually calling the API. The only client not subject to CORS |

When this document and the spec disagree, **the spec is right** — it is verified
against the code.

## Guardrail documents

| Document | Authority on |
|---|---|
| **BRD V1.2** | Business rules, workflow, roles, Phase 1 limitations |
| **`CC_Field_Reference.md` V1.2** | Field-level validation, max lengths, **canonical enum strings** |
| **Security Matrix V2.1** | Which fields are editable/read-only/hidden per role per state |
| **`openapi.yaml`** | Request and response shapes |
| **The HTML prototypes + `global.css`** | Visual source of truth — this build is a port, not a redesign |

---

# Part A — The API contract

*Framework-agnostic. Applies to any client.*

## A1. Authentication

### A1.1 Two tokens, two lifetimes

| | Lifetime | Store where |
|---|---|---|
| **Access token** (JWT) | 30 minutes | Memory only |
| **Refresh token** (opaque) | 24 h absolute, **2 h sliding inactivity** | `localStorage` |

The refresh token is **not rotated** — `POST /refresh` returns a new access token
and the same refresh token. Keep using it.

The sliding window advances on every successful refresh. The 24-hour absolute
expiry does not move, so a session cannot outlive a day regardless of activity.

**Why `localStorage` for the refresh token:** an httpOnly cookie is impossible
without a server on the frontend's origin, and this is a static SPA. Accepted for
Phase 1 — document it rather than hiding it. Access tokens stay in memory so a
closed tab does not leave one behind.

### A1.2 Refresh proactively, and retry once on 401

Refresh at **~24 minutes** — 80% of the access token's life — rather than waiting
for a 401.

⚠️ **Gate it on activity.** A bare timer means an idle tab refreshes forever —
`updated_on` advances every 24 minutes, the server's 2-hour sliding window never
expires, and since the inactivity popup is optional (A1.5), **nothing enforces
inactivity at all.** The server cannot tell a working user from an open tab; a
refresh *is* activity as far as it knows.

Skip the scheduled refresh if there has been no user interaction since the last
one. The reactive path below covers waking from idle.

Also implement that reactive path, since a laptop that slept will wake with a dead
token:

```
request → 401
  → POST /refresh (once)
      → success: retry the original request (once)
      → failure: clear the store, redirect to login
```

**Never loop.** One refresh, one retry, then give up.

### A1.3 The refresh token goes in the JSON body

```
POST /refresh   { "refresh_token": "..." }
```

Not an `Authorization` header — it is not a bearer credential.

### A1.4 Logout is idempotent

`POST /revoke` returns **204** whether the token was valid, already revoked, or
never existed. Logging out never fails. Clear local state regardless of the
response.

### A1.5 The inactivity popup is courtesy, not enforcement

The server's 2-hour sliding window is the real rule. A client-side "Still there?"
prompt at ~30 minutes idle is a UX nicety: **Yes** → `POST /refresh`, **No** or
timeout → `POST /revoke` and log out.

Build it last. The system is correct without it.

## A2. The save-then-submit contract

**The single most important sequencing rule in the API.**

Transitions carry **no field values**. `POST /{ccID}/submit` and
`POST /{ccID}/submit-final` send only `{email, password}` and validate **what is
already stored**.

```
User edits the form
  → PUT /{ccID}              save (as often as you like)
  → POST /{ccID}/submit      validate what was saved, sign, transition
```

**Consequence:** the Submit button must be **disabled while the form is dirty**,
or must save first. Otherwise the user submits with unsaved edits and the API
rejects fields they can see filled in on screen — because that text never left
the browser.

This applies to **both** save endpoints:

| State | Save endpoint | Then |
|---|---|---|
| `Initiated` | `PUT /{ccID}` — 24 fields | `POST /{ccID}/submit` |
| `In Implementation` | `PUT /{ccID}/implementation` — 5 fields | `POST /{ccID}/submit-final` |

### A2.1 Where validation happens

| Check | Save | Transition |
|---|---|---|
| Length, enum membership, JSON type | ✅ | ❌ |
| Presence of mandatory fields | ❌ | ✅ |
| Business-day date rules | ❌ | ✅ |
| `actual_implementation_date` not in the future | ❌ | ✅ |
| Evidence file exists | ❌ | ✅ (T6) |
| E-signature | ❌ | ✅ |

A draft can be saved empty, and a date valid on Monday may be invalid by
Thursday — so those rules can only apply at submission.

**Mirror the presence checks client-side** so the signature modal never opens on a
form that will be rejected. The API enforces them regardless; the client-side copy
is purely to avoid asking for a password before a certain failure.

## A3. Partial updates — absent, null, value

Both save endpoints accept a **partial** body (RFC 7386 merge-patch):

| You send | Result |
|---|---|
| key **absent** | unchanged |
| `"field": null` | **cleared** |
| `"field": "value"` | set |
| `"field": ""` | **cleared** — text fields only |

⚠️ **`""` is a parse error on date and time fields.** Only `null` clears those.
A cleared date picker must send `null`.

**Only the fields listed for that state are accepted.** Any other key returns
**400** listing every offending key, and **nothing is written** — the rejection is
atomic, so a valid field sent alongside an invalid key is not saved either.

Sending the whole form on every save is fine; sending only what changed is fine.

## A4. The en-dash trap

⚠️ **The single most likely silent failure in this port.**

The BRD and the HTML prototypes render six enum values with an **en-dash**
(`–`, U+2013). The database CHECK constraints require an **ASCII hyphen**
(`-`, U+002D). Copying an `<option value>` from a prototype produces a 400 on
every submission, with an error message that does not mention the character.

| Affected | Correct value |
|---|---|
| `requires_testing` | `Yes - Full testing`, `Yes - Partial testing` |
| `post_implementation_issues` | `Issues requiring follow-up` |
| Signature meanings (display) | `Approved - Implementation Approval`, `Rejected - Implementation Approval`, `Approved - Final Approval`, `Rejected - Final Approval` |

**Take every option value from `openapi.yaml` or `CC_Field_Reference.md`, never by
copy-paste from the HTML.** The display text may keep an en-dash if preferred —
only the submitted value matters.

## A5. Dates and times

### A5.1 RFC 3339 only

```
"2026-10-15T00:00:00Z"      ✅
"2026-10-15"                ❌ 400
""                          ❌ 400 — use null
```

`DATE` columns arrive and depart as **midnight UTC**.

### A5.2 Time-of-day fields carry a placeholder date

`implementation_window_start` and `_end` are `TIME` columns and return as:

```
"0000-01-01T09:00:00Z"
```

The date portion is an artifact — Go's `time.Time` always carries one. Strip it
for display; send the same shape back.

### A5.3 Two date rules, enforced at T2

- `proposed_implementation_date` ≥ **2 business days** from today
- `target_closure_date` ≥ **10 business days** from today

Weekdays only; public holidays are not modelled. **Computed in UTC** — in a UTC+
deployment, a submission between midnight and the offset is evaluated against the
previous calendar day.

### A5.4 `actual_implementation_date` must not be in the future

Retrospective by nature. **Accepted at save, rejected at T6** — so a user can
draft on Monday for work scheduled Wednesday.

**Disable future dates in the picker** so the rule is never hit.

## A6. Files

### A6.0 One upload field, not two

The BRD describes two file fields — **Supporting Documents** (field 24, in
`Initiated`) and **Implementation Evidence** (field 34, in `In Implementation`).

**Only Implementation Evidence is implemented** (BRD V1.2 §13.1 L12). The database
schema and its CHECK constraint still permit `supporting_documents`; the API's
whitelist rejects it with a 400.

**Consequences for the port:**

- The `Initiated` form has **no upload control**. The block was removed from
  `cc-form-initated-state.html`
- File upload appears **only** on the `In Implementation` view
- The CC response carries `implementation_evidence` and nothing else file-related

### A6.1 Upload is multipart with a part named `file`

```
POST /changecontrols/{ccID}/files/implementation_evidence
Content-Type: multipart/form-data          ← let the browser set this
```

**Do not set `Content-Type` by hand** — the browser generates it with a boundary
string, and overriding it breaks parsing.

**PDF only, 10 MB maximum.** The type is verified by inspecting the file's
contents, so renaming a `.png` does not work. One file per field: re-uploading
**replaces**, and there is no delete endpoint.

Owner only, `In Implementation` only.

### A6.2 Download cannot be a hyperlink

⚠️ The endpoint requires the bearer token, and `<a href>` cannot send headers.
The obvious approach fails with a 401.

```js
const res = await fetch(url, { headers: { Authorization: `Bearer ${token}` } });
const blob = await res.blob();          // throws if the transfer was truncated
const href = URL.createObjectURL(blob);
// synthesise a click, then URL.revokeObjectURL(href)
```

The `try/catch` also covers truncation: the browser compares bytes received
against `Content-Length` and rejects the promise itself. No manual byte counting.

`Content-Disposition` carries the filename; `Content-Length` the size. Both are
exposed to JavaScript by the CORS configuration.

Download is open to **any authenticated role, in any state** — an approver must
review evidence, and a closed record's evidence must stay reachable.

### A6.3 Do not probe for a file's existence

The CC response carries `implementation_evidence` — `null`, or an object with
`file_name`, `file_size`, `content_type`, `uploaded_on`. Use that to decide
whether to render a download link.

Calling the download endpoint to find out would transfer up to 10 MB to learn a
filename.

## A7. E-signatures

### A7.1 The modal collects EMAIL and password

⚠️ The prototypes label the first field "Username". **There is no username** — the
API compares against the user's email, case-insensitively.

### A7.2 Sign as yourself only

The email must match the **logged-in user** (BR-8.8.3). Valid credentials
belonging to somebody else are rejected with 401.

Pre-fill the field with the current user's email.

### A7.3 A failed signature changes nothing

401, an audit row recording the attempt, and the record **untouched**. Retrying is
safe. Never store the password; clear it when the modal closes.

### A7.4 The signature comes last

The API validates presence, then business rules, then the signature. A validation
failure never reaches the signature check — so the modal should only open once the
client-side checks pass.

### A7.5 Show the meaning being signed

Each transition has a fixed meaning string. Display it in the modal so the user
knows what they are attesting to. **ASCII hyphens** — see A4.

## A8. Errors

### A8.1 Two shapes

```json
{ "error": "Change Control not found" }
```

```json
{
  "error": "Cannot submit: some requirements are not met",
  "issues": ["Change Category", "Implementation Evidence",
             "Target Closure Date must be at least 10 business days from today"]
}
```

The second comes from the transitions (missing fields, failed date rules, missing
evidence — **all collected**) and from the save endpoints (non-editable keys).
**Render every item**, not just the first.

### A8.2 What each status means for the UI

| Code | Do |
|---|---|
| **400** | Show the message, or the `issues` list, next to the fields |
| **401** | The wrapper handles it — refresh once, retry once, else log out |
| **403** | "You do not have permission" — the record loaded, the action is not yours |
| **404** | The record is gone; return to the list |
| **409** | **Refetch the record.** Someone changed its state, so the UI is stale |
| **500** | Generic apology. Nothing actionable client-side |

**409 is the interesting one.** It means the request was valid but the record
moved — the honest response is to reload it and re-render.

### A8.3 A 409 with a body

`PUT /users/{userID}` and `.../active` can return blocked CC-IDs:

```json
{ "error": "Cannot change role while the user has active change controls",
  "blocked_cc_ids": ["CC-001", "CC-003"] }
```

**The request is all-or-nothing** — a name change submitted alongside a blocked
role change is *also* rejected. Do not tell the user the name was saved.

## A9. Lists and pagination

- **`limit` defaults to 50.** Send it explicitly if the UI shows fewer.
- **`offset`, not page numbers.** `offset = (page - 1) * limit`.
- **Reset `offset` to 0 whenever a filter changes**, or the user lands on an empty
  page.
- **`total` is the count matching the filter**, ignoring pagination — use it for
  the page count, not `items.length`.
- **Sort is fixed** at `last_updated_on DESC`. There is no sort parameter, so
  column headers are not sortable.
- **`?owner=me` and `?assigned=me`** are flags resolved server-side from the
  token. No user ID ever appears in a URL.
- **`?state=` accepts one value.** For "either pending state", use the dashboard's
  `pending_approvals` block, which is purpose-built for it.
- **`?search=`** matches CC-ID, change title and owner name only — not
  descriptions, not affected systems.

## A10. Per-screen notes

### Dashboard
One call returns everything. **The lists are capped (2, 2, 5); the totals are
not** — three drafts returns `my_drafts_total: 3` with two items, so the card
reads "3" over two rows.

All five `overview` keys are always present, reporting `0` where no records exist.
The five cards link to `?state=<value>`.

`Cancelled` is absent from the counts but can appear in recent activity.

### Change control form
**One form, every state and role.** The Security Matrix decides what is editable,
read-only or hidden — mirroring how the prototypes are one form in different
states. Do not build a page per state.

`change_title` can be `null` on a draft; render a placeholder.

### User management
The pencil and the status toggle are **separate calls** — `PUT /users/{userID}`
and `PUT /users/{userID}/active`.

Disable the status toggle on your own row (the API returns 400) and hide the role
selector for yourself (403).

### Profile
**Read-only in Phase 1.** No self-update endpoint and no change-password endpoint.
Name, email and role display only.

### Approvals
`?assigned=me` returns your records but takes **one** state. The dashboard's
`pending_approvals` block spans both gates — each item carries its own
`current_state` for the badge.

## A11. IDs and names

Every reference comes as a pair:

```json
"change_owner_id": "73960fc2-…",
"change_owner_name": "Default CC Owner"
```

**Compare on the id** — `cc.change_owner_id === currentUser.id` decides whether a
button renders. Names are not unique and can change.

**Display the name.** The client cannot resolve a UUID, and a lookup per record
would be N+1.

Note `last_updated_by_name` is **not** always the owner — after a rejection it is
the approver.

## A12. CORS

The API allows origins listed in its `ALLOWED_ORIGINS` environment variable.
`http://localhost:5173` (the Svelte dev server) must be among them.

A misconfigured origin fails in a confusing way: **the request reaches the server
and executes**, and only then does the browser refuse to hand the response to
JavaScript. A database write can succeed while the client sees a network error.

If requests fail with no useful message, check the browser console for a CORS
error before suspecting the API.

## A13. Deviations from the original BRD

Each of these is deliberate, recorded in **BRD V1.2**, and reflected in the API.
They are collected here because they are easy to miss individually — if an older
copy of a guardrail document says otherwise, **the API is the contract**.

| Deviation | Original | Now | Why |
|---|---|---|---|
| **Session inactivity** | 30 minutes | **2 hours** | The access token's 30-minute life and the inactivity window were identical, which made the sliding window meaningless — a session could never idle out before the token expired anyway |
| **File types** | PDF, DOCX, XLSX, PNG, JPG | **PDF only** | Evidence should be a fixed artefact. It also makes the type check unforgeable: DOCX and XLSX are both ZIP archives and cannot be told apart by inspecting contents |
| **Supporting Documents** | field 24, uploadable in `Initiated` | **not implemented** | Deferred to a later release (L12). Only Implementation Evidence exists |
| **Blocked role change** | *"the name change can still be saved"* | **all-or-nothing** | A 409 whose transaction commits is incoherent, and the response would have to explain what was and was not saved. The prototype's role-block message was corrected to match |
| **Search** | excluded from scope | **`?search=` implemented** | Text matching on CC-ID, title and owner name. Saved searches and reporting remain out of scope |
| **`actual_implementation_date`** | no rule stated | **must not be in the future**, at T6 | The field is retrospective; the BRD only ruled out a *minimum* lead time, saying nothing about the other direction |

---

# Part B — The Svelte 5 build

*Decisions below are settled. Refine details during the build; do not re-litigate
the stack or the forbidden list.*

## B1. Philosophy — the same identity as the backend

**Flat first.** Abstractions are earned by felt pain, not predicted from the spec.
Do not build a generic `<FormField>` until the same markup has been written inline
**three times**.

**This build is a port, not a redesign.** The prototypes define every screen. When
in doubt, their markup and `global.css` classes are the answer — invent nothing
visual.

**The frontend is deliberately thin.** All correctness lives in the API and the
database: presence validation, enum validity, role and ownership and state checks,
e-signature verification. The frontend renders state, collects input, and displays
the API's verdicts.

It **mirrors** business rules for UX — disabling a button the API would reject —
but never treats its copy as the source of truth. Where the two disagree, the API
wins and the UI is wrong.

## B2. Stack

| Concern | Choice |
|---|---|
| Framework | **Svelte 5** (runes) |
| App shell | **SvelteKit in SPA mode** — `ssr = false`, `prerender = false`, `adapter-static`. Build output is a static folder; **no Node server exists at runtime** |
| Language | **TypeScript** |
| Styling | **`global.css` as-is**, imported once in the root layout. No Tailwind, no UI library, no new design tokens |
| HTTP | Native `fetch` behind one wrapper (`lib/api.ts`). No axios, no query library |
| State | Svelte 5 runes only. **No `svelte/store`** — `writable`/`readable` are the legacy API |
| Backend | The Go API. SvelteKit's server features are unused (see B4) |

## B3. The allowed feature set

**Runes:** `$state`, `$derived`, `$props`, and `$effect` **only** for fetch-on-mount.
`$state` in `.svelte.ts` modules for cross-component state — the auth store.

**Template:** `{#if}` / `{:else if}` / `{:else}`, `{#each}`, `{#await}` where it
reads better, text interpolation.

**Bindings and events:** `bind:value`, event attributes (`onclick={...}` — Svelte 5
style, not `on:click`), callback props for child→parent communication.

**TypeScript:** `interface` for API shapes, string-literal unions for the six
states and four roles, typed `$props`, `| null` on nullable fields to match the
spec.

**SvelteKit subset:** filesystem routing including dynamic params (`[ccId]`),
`+layout.svelte` (root and authenticated shell), `goto` for programmatic
navigation, `$page.url.searchParams` for list filters and pagination.

## B4. Forbidden

- Snippets (`{#snippet}`), transitions and animations, the context API
  (`setContext`/`getContext`), actions (`use:`), `$bindable`, class-based state,
  special elements (`<svelte:window>` and friends)
- Svelte 4 syntax: `export let`, `on:click`, `$:` reactive statements,
  `svelte/store`
- **Every SvelteKit server feature** — `load` functions, form actions,
  `+page.server.ts`, `+server.ts`, hooks, cookie and session handling. The Go API
  is the backend; anything labelled "server" in the SvelteKit docs does not apply
  here
- `$effect` for anything beyond load-on-mount. Deriving state in an effect is the
  classic Svelte 5 anti-pattern — use `$derived`
- New CSS tokens, or visual components not present in the prototypes

## B5. Project structure

```
src/
├── routes/
│   ├── +layout.svelte                    # imports global.css once
│   ├── +layout.ts                        # ssr = false, prerender = false
│   ├── login/+page.svelte
│   └── (app)/                            # authenticated group
│       ├── +layout.svelte                # sidebar shell + route guard
│       ├── dashboard/+page.svelte
│       ├── change-controls/
│       │   ├── +page.svelte              # All CCs — filters via URL params
│       │   └── [ccId]/+page.svelte        # THE form — all states, all roles
│       ├── my-change-controls/+page.svelte
│       ├── approvals/+page.svelte
│       └── settings/
│           ├── +page.svelte               # profile (read-only)
│           └── users/+page.svelte          # admin only
└── lib/
    ├── api.ts                            # fetch wrapper: base URL, bearer, refresh-and-retry
    ├── auth.svelte.ts                    # $state store: user + accessToken
    ├── types.ts                          # mirrors openapi.yaml
    └── components/                       # extracted ONLY after 3× repetition
```

**One page renders the CC form in every state and for every role** — the Security
Matrix expressed as `{#if}` and `disabled`, mirroring how the prototypes are one
form in different states. **Do not create a page per state.**

`EsigModal` and `SignatureTable` will likely earn extraction. Let them earn it.

## B6. `types.ts` — derive it from the spec

Write the interfaces from `openapi.yaml`, not from observed responses. The spec
records which fields are nullable; a sample response does not.

```ts
export type State =
  | 'Initiated'
  | 'Pending Implementation Approval'
  | 'In Implementation'
  | 'Pending Final Approval'
  | 'Closed'
  | 'Cancelled';

export type Role = 'Admin' | 'CC Owner' | 'Approver' | 'Viewer';

export type Decision = 'Approve' | 'Reject';        // imperative, not past tense
export type RiskLevel = 'Low' | 'Medium' | 'High';
```

String-literal unions rather than `enum`: they match the wire format exactly,
narrow correctly in `{#if}` blocks, and need no conversion.

**Copy every enum member from the spec**, including the ASCII hyphens (A4).

## B7. `api.ts` — one wrapper, no raw fetch in components

Responsibilities:

- Prefix the base URL from `PUBLIC_API_URL`
- Attach `Authorization: Bearer` from the auth store
- `Content-Type: application/json` for JSON bodies — but **not** for `FormData`,
  where the browser must set it with a boundary
- On **401**: refresh once, retry once, else clear the store and `goto('/login')`.
  Never loop
- Parse `{error}` and `{error, issues}` into a typed error the caller can branch on
- Expose a separate path for the file download, which returns a blob rather than
  JSON

Everything else calls this. A raw `fetch` in a component means a request that skips
the token, the refresh, and the error parsing.

## B8. `auth.svelte.ts` — the store

`$state` holding `user` (id, full_name, email, role) and `accessToken`.

The **refresh token** lives in `localStorage`; the **access token** in memory only.
On app start, attempt one silent refresh to restore a session — then `GET /me` to
populate the user.

The `(app)/+layout.svelte` guard checks the store and redirects to `/login` when
empty.

## B9. Build order

Each step is independently verifiable against the running API. Do not start a step
until the previous one works end to end.

| # | Step | Proves |
|---|---|---|
| 1 | **Scaffold** — `npx sv create` (TS), adapter-static, `ssr=false`, import `global.css`, commit | The shell builds and the prototype styling carries over |
| 2 | **`types.ts`** from `openapi.yaml` | The contract is transcribed before any code depends on it |
| 3 | **Login page + auth store + `api.ts`** against the real `POST /login` | Auth works, CORS is configured, the token is stored |
| 4 | **Authenticated layout** — sidebar, route guard, silent refresh on load | Navigation and session restoration |
| 5 | **Dashboard** | First data fetch, first `{#each}`, and every list shape in one screen |
| 6 | **All Change Controls** — filters and pagination via URL params | Query-parameter handling, `total` vs `limit`, offset reset |
| 7 | **The CC form, `Initiated`, owner view** — create, Save Draft, dirty tracking | The largest single piece. Partial updates, the enum values, the null-vs-empty rule |
| 8 | **T2 submit + the e-signature modal** | The first transition end to end, and the save-then-submit gate |
| 9 | **Approver flow** — queue and the implementation decision | The second role, and the first approval gate |
| 10 | **The `In Implementation` view** — save implementation details, then **file upload** | The second save endpoint, then `FormData`, the part named `file`, and the PDF/size limits |
| 11 | **T6 + the final decision** + the signature history panel | The remaining gates, and the full state machine exercised |
| 12 | **File download** | Blob handling, `Content-Disposition` |
| 13 | **Admin settings** — user management, including the 409 with `blocked_cc_ids` | The third role, and a non-trivial error shape |
| 14 | **Inactivity popup** | Courtesy only — the system is correct without it |

**Why this order.** Steps 1–4 are infrastructure: nothing renders until they work.
Step 5 exercises every list shape in one screen, which is a cheap way to validate
`types.ts`. Step 7 is deliberately early — it is the biggest and most detail-dense
piece, and everything after it is a variation. Step 8 introduces the signature
once, before it appears in five more places.

**Upload cannot come earlier than step 10**, because the only upload field is
`implementation_evidence` and it is writable only in `In Implementation` — a state
a record can only reach by passing through steps 8 and 9. Download (12) follows it
naturally, and is late because the CC response already carries the metadata needed
to render the *link*.

Steps 8–11 walk a single record through the whole state machine in order, which
means each transition is tested on a record that arrived there legitimately rather
than one nudged into place by hand.

## B10. Settled during the build

These were open in V0.9 and are now answered:

| Question | Answer |
|---|---|
| Exact JSON field names and shapes | `openapi.yaml` — 46 schemas |
| The 400 validation payload | `{error, issues[]}` — see A8.1 |
| Dashboard response shape | Four blocks — see A10 and the spec |
| API base URL handling | `PUBLIC_API_URL` via Vite, read in `api.ts` |
| CORS origin for dev | `http://localhost:5173`, already in the API's `ALLOWED_ORIGINS` |

## B11. Known deferred

- Inline styles in the dashboard and settings prototypes resolve naturally during
  componentisation
- The list views still carry a "Created By" column, sort chevrons, and a date-range
  dropdown that the API does not support — see A9 and drop them during the port.
  There is no `created_by` field: the creator **is** the owner, immutably, so the
  two columns could never differ
- Offline Swagger UI (the docs page loads its assets from a CDN)

---

**End of blueprint.**

Part A is a contract and should change only when the API changes. Part B is a
plan and will change as it meets reality — when it does, amend it rather than
letting the code and the document drift apart.
