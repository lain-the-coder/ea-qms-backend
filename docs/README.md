# API documentation

Interactive reference for the EA QMS Change Control API — 23 endpoints.

| File           |                                                               |
| -------------- | ------------------------------------------------------------- |
| `openapi.yaml` | The OpenAPI 3.0.3 specification. **The definitive contract.** |
| `index.html`   | Swagger UI, loading the spec from this folder.                |

## Viewing it

The page fetches `openapi.yaml` over HTTP, so opening `index.html` directly from
the filesystem will fail on the browser's CORS policy. Serve the folder:

```bash
cd docs
python3 -m http.server 8080
```

Then open <http://localhost:8080>.

Any static server works — `npx serve`, `php -S localhost:8080`, or publishing the
folder to GitHub Pages.

## Trying requests against a running API

1. Start the backend (`go run .`) — it listens on `localhost:1304`.
2. In Swagger UI, **Authorize** (top right) and paste an access token from
   `POST /login`. The token persists across page reloads.
3. Use **Try it out** on any operation.

Note the API sets no CORS headers, so browser-originated requests from
`localhost:8080` to `localhost:1304` will be blocked. Swagger UI is therefore a
**reference**, not a test client — use Postman for calls, and this for the
contract.

## Keeping it accurate

The spec is hand-written from the handler code, not generated. **When an endpoint
changes, update `openapi.yaml` in the same commit.** A stale spec is worse than
none, because it is trusted.

Validate before committing:

```bash
python3 -c "import yaml; yaml.safe_load(open('openapi.yaml')); print('ok')"
```

Or paste it into <https://editor.swagger.io> for full schema validation.

## What the spec captures that Postman cannot

Postman records the responses that happened to be saved. This records what the API
**accepts and guarantees**:

- Which fields are nullable, and which are required
- Every enum value — including the six with **ASCII hyphens** that the BRD and the
  HTML prototypes render with en-dashes
- Every status code an endpoint can return, and what each one means
- The partial-update semantics of the two save endpoints (absent vs `null` vs
  value)
- Which endpoints are role-gated, ownership-gated or state-gated
