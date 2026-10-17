# ciview

Read `PRODUCT.md` for user intent and `DESIGN.md` before changing the frontend.

## Architecture

- `cmd/ciview` owns configuration, dependency wiring, and server lifecycle.
- `internal/httpapi` serves Huma endpoints; `internal/web` embeds the frontend.
- `internal/config` loads TOML and credential references; `internal/providers` normalizes provider responses; `internal/refresh`, `internal/snapshots`, and `internal/ci` own scheduling, cached state, and metrics.
- `frontend` uses Vite, Svelte 5, TypeScript, and Kit UI with pnpm. No SvelteKit or Tailwind.

- `internal/mcp` owns typed MCP tools and the host/origin policy; keep its transport conventions consistent across the three apps.

## Conventions

Import shared controls directly from `@kenn-io/kit-ui`; local components own
app-specific behavior. Use Svelte runes, typed props, and semantic CSS tokens.
Keep `frontend/src/theme.css` identical across Golinks, CI View, and Homelab Launcher.
Load Inter and IBM Plex Mono from Google Fonts; never bundle font binaries.
Inter is for interface text; IBM Plex Mono is for identity and data. Preserve
`scrollbar-gutter: stable`, dark default, and explicit light/system preferences.
Keep desktop and phone layouts, keyboard focus, and recoverable error states.

Keep the viewer read-only. Preserve exact terminal outcomes, missing timing, stale rows, and URL filters. Never expose credentials or raw provider errors. Validate original provider links; keep metrics and collection on the Go server.

## Documentation

README sections are overview, installation, usage, configuration, development,
and build. Use synthetic examples without private paths, hostnames, or deployment
processes. Keep docs limited to screenshots and essential user references.
Keep `PRODUCT.md` and `DESIGN.md` as concise, maintained project guides.
Do not commit generated plans, specs, agent reports, or design-tool state. The root
`LICENSE` contains the standard Apache 2.0 text only.

## Development

Use `just install`, `just api`, and `just frontend`. `just check` runs Go race
tests, vet, frontend tests, and type checks. `just build` verifies and embeds the
frontend in one executable; `just image-check` builds and smoke-tests a container.
For UI changes, verify both themes and desktop/phone widths in a browser.
