# API documentation

Interactive reference for the EA QMS Change Control API — 23 endpoints.

| File           |                                                               |
| -------------- | ------------------------------------------------------------- |
| `openapi.yaml` | The OpenAPI 3.0.3 specification. **The definitive contract.** |
| `index.html`   | Swagger UI, loading the spec from this folder.                |

## Viewing it

**Published:** <https://lain-the-coder.github.io/ea-qms-backend/>

Served by GitHub Pages from this folder on `main`. Read-only — there is no API
behind a static page, so **Try it out** will not work there.

**Locally, with the API running:**

```bash
go run .
```

Then open <http://localhost:1304/docs>. Both files are compiled into the binary
with `go:embed`, so no separate static server is needed.

The specification is served at two paths — `/docs/openapi.yaml` and
`/openapi.yaml` — so that `index.html` can reference it **relatively** and work
unchanged both here and on GitHub Pages, where this folder is the site root.

## Trying requests against a running API

Only from `localhost:1304/docs`, not from the published site.

1. Start the API (`go run .`) with Postgres running.
2. `POST /login` → **Try it out** → **Execute**. Copy the `token` from the
   response — the JWT, not the refresh token.
3. Click **Authorize** (top right) and paste it.
4. Every operation now carries the token. It survives a page reload.

This works because the docs are served from the same origin as the API. Requests
from any other origin need that origin listed in `ALLOWED_ORIGINS` — see the CORS
middleware.

## Keeping it accurate

The spec is **hand-written from the handler code**, not generated. When an
endpoint changes, update `openapi.yaml` **in the same commit**. A stale spec is
worse than none, because it is trusted.

`go:embed` compiles the spec into the binary, so a change requires a rebuild to
take effect — which is the right coupling: the spec and the code it describes
move together.

Validate before committing:

```bash
python3 -c "import yaml; yaml.safe_load(open('docs/openapi.yaml')); print('ok')"
```

Or paste it into <https://editor.swagger.io> for full schema validation.

## What the spec captures that a Postman export cannot

Postman records the responses that happened to be saved. This records what the
API **accepts and guarantees**:

- Which fields are nullable, and which are required
- Every enum value — including the six with **ASCII hyphens** that the BRD and
  the HTML prototypes render with en-dashes. Taking option values from here
  rather than from the prototypes avoids the most likely silent failure in a
  frontend port
- Every status code an endpoint can return, and what each one means
- The partial-update semantics of the two save endpoints — absent means
  unchanged, `null` means clear, and an unknown key returns 400 with nothing
  written
- Which endpoints are gated by role, by ownership, or by workflow state

## Note on offline use

`index.html` loads Swagger UI's CSS and JavaScript from a CDN (unpkg), so the
page needs an internet connection to render. Only the 40-line page and the spec
are embedded in the binary. To make it fully self-contained, download
`swagger-ui.css`, `swagger-ui-bundle.js` and `swagger-ui-standalone-preset.js`
into this folder, embed them, and point the HTML at the local paths.
