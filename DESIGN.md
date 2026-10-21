# Design System

CI View is a simple self-hosted application.

The interface should use a small consistent vocabulary rather than
one-off styling for each new control.

## Component Rules

Use the shared desktop workspace theme in `frontend/src/theme.css`. We keep that file identical across the other self-hosted apps, so changes there should be generic and minimal.
Application layout and interaction rules belong in `frontend/src/app.css`.

- Use Svelte 5 runes, typed props, and callback props.
- Keep desktop and phone layouts, keyboard focus, and recoverable error states.
- Load Inter and IBM Plex Mono from Google Fonts with `display=swap`. Do not
embed font binaries in the repository or production assets.
- Inter is for interface text; IBM Plex Mono is for wordmarks, identifiers, and
technical data. Use neutral surfaces, subtle one-pixel borders, rounded controls,
and no decorative shadows.
- Focus is blue in light mode and amber in dark mode.
- Preserve `scrollbar-gutter: stable`.
- Import shared controls directly from `@kenn-io/kit-ui`.
- Use semantic theme tokens and keep the library's default control sizes.
- Keep local components for app-specific behavior, not pass-through wrappers.
- Give fields one visible focus treatment and keep pressed controls stationary.

## Appearance and Accessibility

Start in dark mode and respect explicit light or system preferences. The header
ThemeToggle saves the choice under the app's own storage key. System mode follows
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
