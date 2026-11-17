# Design System

CI View is a simple self-hosted application.

The interface should use a small consistent vocabulary rather than
one-off styling for each new control.

## Component Rules

Theme and component styles come from the shared design system.

- Use Svelte 5 runes, typed props, and callback props.
- Keep desktop and phone layouts, keyboard focus, and recoverable error states.
- Fonts are the theme's system stack and `--mono`; never load web fonts or embed font
  binaries. `--mono` is for wordmarks, identifiers, and technical data.
- Use neutral surfaces, subtle one-pixel borders, rounded controls, and no decorative
  shadows on bordered panels.
- Preserve `scrollbar-gutter: stable`.
- Controls come from `@ui-sdk/svelte` (one entry per component), styled by the synced Cadence
  theme in `frontend/src/design-system/`. That folder is synced from the `design-system` repo
  and never edited by hand; app styles live in `src/app.css`, which `main.ts` imports last (after the
  synced theme and custom CSS), so app rules win ties with the theme; prefer tokens and class
  selectors over `!important`.
- Use the theme tokens (`--ink`, `--muted`, `--line`, `--space-*`, `--radius-*`, `--mono`) and
  keep the library's default control sizes. CI run-state colors are app tokens in `app.css`.
- Keep local components for app-specific behavior, not pass-through wrappers.
- Give fields one visible focus treatment and keep pressed controls stationary.

## Appearance and Accessibility

Start in dark mode and respect explicit light or system preferences. The header
ThemeToggle saves the choice under the shared `color-scheme` key. System mode follows
the operating system. Keep focus visible, controls labeled, and reduced-motion
preferences respected. Phone layouts must not widen the page.

## Application Patterns

Keep the wordmark, theme/refresh controls, and URL-backed filters above source
and repository groups. Transparent repository panels have rounded outlines and
fine row separators. Workflow rows align identity, history, metrics, and status;
they stack on phones.

Run bars stay square. Color reinforces an exact state label. Hover and keyboard
focus reveal run details; missing timing remains unavailable. Keep distinct
success, failure, running, queued, canceled, skipped, and unknown outcomes.

Keep stale or partial rows usable. Source notices belong in the collapsed
native disclosure below the workflows. Preserve verified workflow-file markers
independently of run outcomes. Provider links open their original validated URLs.
