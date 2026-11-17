# Agent Instructions

CI View is a self-hosted, read-only viewer for GitHub Actions and Forgejo
Actions. It builds one Go binary that serves a Huma HTTP API, an MCP endpoint,
and an embedded Svelte 5 frontend over cached provider data.

Read `PRODUCT.md` for user intent and `DESIGN.md` before changing the frontend.

The `README.md` and `justfile` own setup and usage.
They are written for humans: avoid big restructures, but update them when changes are user facing.

## Architecture

- Go for the API in /cmd and /internal (module `github.com/danielfrg/ciview`).
- TypeScript and Svelte 5 for the frontend in /frontend.
- Should build into a single binary with an embedded frontend.

## Behavior Invariants

- Keep the viewer read-only: no run triggering, cancellation, or logs.
- Preserve exact terminal outcomes, missing timing, stale rows, and URL filters.
- Never expose credentials or raw provider errors; credential references resolve on the server and tokens never reach the browser.
- Validate original provider links; keep metrics and collection on the Go server.
- MCP tools read cached state without triggering upstream collection and follow the same data contracts as the HTTP API.

## Development

Use the `just` recipes (`just --list`):

- `just install`: install the frontend dependencies.
- `just api`: run the Go API (serves the frontend embedded by the last `just build`).
- `just frontend`: Vite dev server with HMR, proxies the API.
- `just check`: gofmt, vet, race tests, oxfmt, oxlint, frontend tests and svelte-check.
- `just build`: build the frontend, embed it, and build the single binary.
- `just run`: build, then run the binary.
- `just image-check`: build the container image and run `scripts/container-smoke.sh`.

## Frontend Conventions

- `DESIGN.md` owns the theme, typography, layout, and interaction rules; read it before changing the frontend.
- `frontend/src/design-system/` is synced from the `design-system` repo with `pnpm sync ciview`
  there. Never edit it by hand; change it in `design-system` and re-sync.
- `main.ts` imports the synced design-system CSS first, then `palette.css` if any (none here),
  then `app.css` last, so app rules win ties with the theme; prefer tokens and class selectors
  over `!important`. `app.css` keeps `@layer ui-sdk;` as its first statement.
