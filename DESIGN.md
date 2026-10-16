---
name: CI View
description: A clean desktop workspace for reading CI history in light or charcoal themes.
colors:
  charcoal: "#161616"
  surface: "#242424"
  inset: "#202020"
  surface-hover: "#303030"
  foreground: "#ffffff"
  secondary: "#b8b8b8"
  muted: "#a3a3a3"
  border: "#333333"
  border-muted: "#3a3a3a"
  amber: "#e8a44b"
  success: "#8dcc9c"
  failure: "#ff9b9b"
  running: "#8bcbd5"
  light-background: "#ffffff"
  light-surface: "#ffffff"
  light-inset: "#fafafa"
  light-surface-hover: "#f2f2f2"
  light-foreground: "#161616"
  light-secondary: "#5c5c5c"
  light-muted: "#737373"
  light-border: "#e5e5e5"
  light-border-muted: "#e5e5e5"
  light-focus: "#3b5cf6"
  light-success: "#2eaf5a"
  light-failure: "#f17471"
  light-running: "#3b5cf6"
  light-waiting: "#a67c22"
typography:
  title:
    fontFamily: "Inter, -apple-system, BlinkMacSystemFont, Segoe UI, sans-serif"
    fontSize: "0.8125rem"
    fontWeight: 500
    lineHeight: 1.6
  label:
    fontFamily: "Inter, -apple-system, BlinkMacSystemFont, Segoe UI, sans-serif"
    fontSize: "0.75rem"
    fontWeight: 400
    lineHeight: 1.4
    letterSpacing: "0.06em"
rounded:
  control: "6px"
  state: "8px"
  run: "0px"
spacing:
  phone-gutter: "16px"
  desktop-gutter: "32px"
  row-padding: "20px"
  row-column-gap: "24px"
  section-gap: "32px"
components:
  workflow-state:
    backgroundColor: "{colors.surface}"
    textColor: "{colors.foreground}"
    rounded: "{rounded.state}"
  run-bar:
    rounded: "{rounded.run}"
    height: "28px"
---

# Design System: CI View

## Overview

**Creative North Star: "The Clean Desktop Workspace"**

CI View is an Operate surface: a clean, restrained workspace where workflow
identity and broad run history carry the hierarchy. The selected OpenCode
Desktop Light direction extends to charcoal dark mode with the same transparent
rows, rounded repository boundaries and gently rounded controls. Inter keeps the interface easy
to scan; IBM Plex Mono distinguishes branch names, metrics and exact run details.

Kit UI (`@kenn-io/kit-ui`) supplies semantic tokens and reusable controls. Its
dependency is pinned to an upstream GitHub commit in `frontend/package.json`.
`frontend/src/app.css` defines the application palette, type and workflow layout.
There is no Tailwind. Import shared controls directly from Kit UI; local
components belong to CI presentation or app-specific behavior.

**Key Characteristics:**

- Charcoal on first visit, with explicit light and system preferences respected.
- Self-hosted Inter for interface text and IBM Plex Mono for data.
- Subtly outlined repository groups, horizontal run samples and fine row separators.
- Kit UI controls at their default sizes.

## Colors

Both themes use neutral surfaces, quiet boundaries and distinct CI outcome
colors. Frontmatter records the application palette; CSS semantic variables
remain the implementation source of truth.

### Primary

Blue supplies keyboard focus and the input caret in light mode.
Warm amber fills the same role in dark mode through the semantic
`--accent-blue` token. Text selection uses the browser default highlight.

### Secondary

Green represents success, red failure, and blue (light) or cyan (dark) running.
Queued, waiting and attention states use amber. Canceled, skipped, neutral and
unknown use neutral ink, retaining exact labels and accessible descriptions.
Status text uses semantic mixtures with foreground ink; chips use Kit UI tone
treatments. The running chip locally maps its info accent to the running color.

### Neutral

Light mode uses a white page, pale neutral insets, dark ink and light gray rules.
Dark mode uses charcoal, slightly lighter control surfaces, white text and muted
gray rules. Repository groups remain transparent within subtle rounded outlines in both themes. Row hover and
focus-within add a subtle inset tone without introducing a card silhouette.

First visit selects dark mode; the header ThemeToggle persists the user's
preference under `ciview-theme`. An explicit light or system mode must survive
reloads, and system mode follows the operating system.

**The Outcome Rule.** Color supports the exact status label; it never replaces
it or merges distinct terminal outcomes.

## Typography

Inter is self-hosted as a variable font (100–900) for interface text, with
system sans-serif fallbacks. IBM Plex Mono is self-hosted in regular and medium
weights for the wordmark, branch scope, metric values and run tooltips, with
`ui-monospace`, `SFMono-Regular`, `Menlo` and `monospace` fallbacks. Both use
`font-display: swap`. Medium and semibold semantic weights resolve to 500.

The uppercase CI View wordmark is compact (1rem, weight 500, line-height 1.25).
Workflow titles use the frontmatter title style; uppercase source headings use
the label style. Supporting text is .8125rem, branch scope .6875rem on desktop,
and metric labels and values .625rem and .75rem. Phone metrics increase to
.6875rem and .875rem, and branch scope to .75rem. Metric numerals are tabular.
Long repository and workflow names wrap instead of truncating identity.

## Layout

The centered outer content container is capped at 1344px, including 32px side
gutters. Phone gutters become 16px. The header pairs the wordmark with theme and
refresh controls; the filter grid follows the header and precedes source groups.
There is no sidebar, decorative dashboard card grid or footer.

Source headings separate configured sources. Each source contains transparent
repository groups with a subtle rounded outline and separators between workflow rows,
without repository or column headings. A row title combines the repository link, a hyphen and the workflow
history link; branch scope sits beneath it.

Desktop rows align identity, history, three metrics and latest status. Rows have
20px padding, a minimum height of 108px and 24px column gaps. Repository groups
have 16px of space between them; source groups are separated by 32px. At 1190px, rows become two
columns with metrics and latest status below identity and history, and search
spans the filter grid. At 640px, filters form two columns and rows stack identity,
status, history and metrics. Large configured samples scroll within the history
region; they must not widen the page.

## Elevation & Depth

Repository groups use a one-pixel outline, rounded corners and subtle row hover
without shadows. Fine horizontal rules divide workflows within each group.
Raised, sunken and panel shadow tokens are disabled. Preserve Kit UI's transient
menu behavior and chrome; do not add decorative elevation to the overview.
Run tooltips float above content so their exact details remain readable.

## Shapes

Controls and run tooltips use gently rounded corners (6px); loading and empty
state containers use a slightly larger radius (8px). Notices use the control
radius. Repository group outlines use the larger radius (8px). Run bars stay square.
Preserve Kit UI's native component silhouettes, including pill-shaped chips
and circular status dots, without changing their default sizes.

## Components

### Shared controls

SearchInput, SelectDropdown, Button, ThemeToggle, Chip and EmptyState come
directly from Kit UI. Keep default control sizes, padding and font-size tokens;
do not scale them down to match surrounding metadata. Buttons and SearchInput
use their default medium size; Chip also defaults to medium. Theme and palette
overrides change their visual character without replacing their sizing contract.

Search is labeled for assistive technology, and selects remain keyboard
accessible. The search field wrapper owns its themed focus border, avoiding a
second outline on the nested input. Select triggers stay still while their
fixed-position menus are measured and opened. Filters live in URL query
parameters and survive refresh, back and forward navigation.

### Workflow rows and history

Empty history slots use a dedicated neutral fill, mixed from muted text and the
page background. They remain visible against resting, hovered and keyboard-focused
rows in both themes, and stay distinct from observed neutral outcomes.

History shows the complete server-displayed sample, normally 30 runs, oldest
to newest. Each run bar has constant visual height (28px) inside a 44px link
area. Hover and keyboard focus reveal exact title, branch, status, local time
and known execution duration. Hover brightens the bar without changing its size.
Workflow names open provider workflow history; history bars and latest status
open individual runs. Provider links open in a new tab and use original,
validated absolute HTTP(S) URLs. Never infer a run link.

Metrics remain server-owned. Missing values use an em dash and zero duration
uses `0s`; unavailable timing must never become zero. Preserve partial coverage
and distinct terminal outcomes in the data and accessible descriptions. Rows
show branch scope without observed-run counts or coverage labels.

### Status chips and notices

Latest status uses a default-size Kit UI Chip with a dot and plain-case label.
A warning Chip labeled "Not on branch" sits beside branch scope only when the
server has verified that the workflow file is absent. Its tooltip explains
that historical runs remain available. It is independent of run outcome and
stale-data notices; unknown file status and all-branch views have no marker.

Initial loading, collection in progress, no observed workflows, no filter
matches, request failure, stale repositories and unavailable selections remain
distinct. Background reads retain the previous overview on failure and expose
Retry. Refresh reloads cached server state; upstream health remains separately
visible. The header has Refresh without collection timestamps.

Source diagnostics appear below workflows in a native disclosure, collapsed by
default. Successful empty scans produce no notices. Errors, pending sources and
incomplete scans retain context, as do stale-data notices on rows. Messages must
never expose credentials or raw provider errors.

### Motion and focus

Keep visible focus on links, controls and disclosure summaries. App links use an
themed two-pixel outline with an offset (blue in light, amber in dark); Kit UI
owns shared control focus.
Press transforms are disabled. Respect reduced motion for animation and
transitions, including library indicators.

## Do's and Don'ts

### Do:

- **Do** preserve the charcoal default and explicit saved theme preferences.
- **Do** keep broad history, wrapping identity and usable phone layouts.
- **Do** use semantic tokens and Kit UI controls at their default sizes.
- **Do** preserve stale rows, missing timing, partial coverage and exact outcomes.
- **Do** keep CI View read-only and link to provider-owned run actions and logs.

### Don't:

- **Don't** add control size overrides to create artificial density.
- **Don't** turn ruled workflow groups into enclosing cards or add decorative shadows or a sidebar.
- **Don't** invent metrics, infer links or use color as the only status signal.
- **Don't** hide errors, pending collection or unavailable selections behind an empty state.
