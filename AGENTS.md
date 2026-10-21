# Agent Instructions

CI View is a self-hosted, read-only viewer for GitHub Actions and Forgejo
Actions. It builds one Go binary that serves a Huma HTTP API, an MCP endpoint,
and an embedded Svelte 5 frontend over cached provider data.

Read `PRODUCT.md` for user intent and `DESIGN.md` before changing the frontend.

The `README.md` and `justfile` own setup and usage.
They are written for humans: avoid big restructures, but update them when changes are user facing.

## Architecture

- Go for the API in /cmd and /internal.
- TypeScript and Svelte 5 for the frontend in /frontend.
- Should build into a single binary with an embedded frontend.

## Behavior Invariants

- Keep the viewer read-only: no run triggering, cancellation, or logs.
- Preserve exact terminal outcomes, missing timing, stale rows, and URL filters.
- Never expose credentials or raw provider errors; credential references resolve on the server and tokens never reach the browser.
- Validate original provider links; keep metrics and collection on the Go server.
- MCP tools read cached state without triggering upstream collection and follow the same data contracts as the HTTP API.

## Frontend Conventions

`DESIGN.md` owns the theme, typography, layout, and interaction rules; read it before changing the frontend.

## Deployment

Daniel (danielfrg/danrodriguez) deploys this service in his homelab; the deployment source is in `~/code/danielfrg/homelab`.
