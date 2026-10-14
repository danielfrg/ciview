# Civiewer

Read `PRODUCT.md` and `DESIGN.md` before changing the frontend.

## Architecture

- `cmd/civiewer` owns configuration, dependency wiring and server lifecycle.
- `internal/config` loads TOML and resolves server-side credential references.
- `internal/providers` normalizes GitHub and Forgejo REST responses.
- `internal/refresh` owns discovery, scheduling and snapshot publication.
- `internal/snapshots` provides concurrent, independently owned snapshots.
- `internal/ci` defines contracts and calculates workflow overviews.
- `internal/httpapi` exposes cached state through Huma; `internal/web` embeds UI.
- `frontend` uses plain Vite, Svelte 5, TypeScript and Tailwind 4 with pnpm.

## Conventions

Use locally owned UI primitives under `frontend/src/lib/components/ui` and CI
presentation components under `components/ci`. Preserve semantic tokens,
Svelte runes, typed props, callback props, snippets and barrel exports. No
SvelteKit or provider transport in the browser. Go owns timing and sample metrics.

Keep this viewer read-only. Preserve terminal outcomes, unavailable duration,
partial coverage and stale rows. Never expose tokens or raw provider errors.
Every run link uses its original absolute HTTP(S) URL after validation. Filters
live in URL query parameters and survive refresh and browser navigation.

Local development uses `just api` and `just frontend`; `CIVIEWER_API_TARGET`
overrides the frontend proxy target. Configuration loads at startup from
`CIVIEWER_CONFIG`, defaulting to `civiewer.toml`. Keep local config, `.env`,
credentials and private payloads out of source control.
Keep specs, plans and agent reports local and uncommitted. Use synthetic examples
in public documentation and tests; never include machine-specific personal paths.

## Check

```shell
go test ./...
go test -race ./...
go vet ./...
pnpm --dir frontend test
pnpm --dir frontend check
pnpm --dir frontend build
just build
```
