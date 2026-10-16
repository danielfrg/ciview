---
name: CI View
description: A charcoal, monospaced workspace for reading CI history.
colors:
  charcoal: "#161616"
  surface: "#242424"
  inset: "#202020"
  surface-hover: "#303030"
  foreground: "#ffffff"
  secondary: "#b8b8b8"
  muted: "#a3a3a3"
  border: "#4a4a4a"
  border-muted: "#3a3a3a"
  amber: "#e8a44b"
  success: "#8dcc9c"
  failure: "#ff9b9b"
  running: "#8bcbd5"
  light-background: "#f5f3ef"
  light-surface: "#ffffff"
  light-inset: "#eeece8"
  light-surface-hover: "#e8e5df"
  light-foreground: "#242424"
  light-secondary: "#57534e"
  light-muted: "#68635c"
  light-border: "#bab5ad"
  light-border-muted: "#d4cfc6"
  light-focus: "#94601c"
typography:
  title:
    fontFamily: "IBM Plex Mono, ui-monospace, SFMono-Regular, Menlo, monospace"
    fontSize: "0.8125rem"
    fontWeight: 500
    lineHeight: 1.6
  label:
    fontFamily: "IBM Plex Mono, ui-monospace, SFMono-Regular, Menlo, monospace"
    fontSize: "0.75rem"
    fontWeight: 400
    lineHeight: 1.4
    letterSpacing: "0.06em"
rounded:
  panel: "0px"
spacing:
  phone-gutter: "16px"
  desktop-gutter: "32px"
  row-padding: "20px"
  row-column-gap: "24px"
  section-gap: "32px"
components:
  repository-panel:
    backgroundColor: "{colors.surface}"
    textColor: "{colors.foreground}"
    rounded: "{rounded.panel}"
  run-bar:
    rounded: "{rounded.panel}"
    height: "28px"
---

# Design System: CI View

## Overview

**Creative North Star: "The Charcoal Workspace"**

CI View is an Operate surface: a quiet, monospaced workspace where workflow
identity and broad run history carry the hierarchy. The visual direction follows
the charcoal treatment shared with golinks and the restrained data presentation
of OpenCode's data page. Flat surfaces, precise rules and selective status color
keep the interface readable across a large collection of workflows.

Kit UI (`@kenn-io/kit-ui`) supplies semantic tokens and reusable controls. Its
dependency is pinned to an upstream GitHub commit in `frontend/package.json`.
`frontend/src/app.css` defines the application palette, type and workflow layout.
There is no Tailwind. Import shared controls directly from Kit UI; local
components belong to CI presentation or app-specific behavior.

**Key Characteristics:**

- Charcoal on first visit, with explicit light and system preferences respected.
- Self-hosted monospace typography and restrained weight changes.
- Square repository panels, horizontal run samples and clear separators.
- Kit UI controls at their default sizes.

## Colors

The default palette uses neutral charcoal layers with warm amber interaction
accents and distinct CI outcome colors. Frontmatter records the application
palette; CSS semantic variables remain the implementation source of truth.

### Primary

Amber supplies keyboard focus, text selection and the input caret. The Kit UI
accent token retains its upstream name, `--accent-blue`, while its value is
amber. Do not infer a blue visual accent from the token name.

### Secondary

Soft green represents success, soft red failure and cyan running. Amber also
represents queued, waiting and attention states. Canceled, skipped, neutral and
unknown use neutral ink, with their exact outcome retained in labels and
accessible descriptions. Status text uses semantic mixtures with foreground
ink; chips use Kit UI's tone treatments. The running chip locally maps its info
accent to cyan.

### Neutral

Charcoal is the page background; surface and inset tones separate panels and
controls. White carries primary text, while secondary and muted grays carry
supporting information. Default borders define panels and row separators;
muted borders support quieter control boundaries.

The light palette uses warm off-white, white surfaces and dark neutral ink,
with a darker amber focus accent. Light-mode outcome accents inherit Kit UI's
semantic palette. First visit selects dark mode; the header ThemeToggle persists
the user's preference under `ciview-theme`. An explicit light or system mode
must survive reloads, and system mode follows the operating system.

**The Outcome Rule.** Color supports the exact status label; it never replaces
it or merges distinct terminal outcomes.

## Typography

IBM Plex Mono is self-hosted in regular and medium weights with `font-display:
swap`; fallbacks are `ui-monospace`, `SFMono-Regular`, `Menlo` and `monospace`.
Both the sans and mono semantic font roles use this stack. Medium and semibold
semantic weights resolve to 500.

The uppercase CI View wordmark uses a compact medium heading (1rem, line-height
1.25). Workflow titles use the title style. Source headings use
the label style with uppercase text. Supporting copy is restrained
(.8125rem), branch scope is quieter (.6875rem on desktop), and metric labels and
values use .625rem and .75rem respectively. Phone metrics increase to .6875rem
and .875rem. Metric numerals are tabular. Long repository and workflow names
wrap instead of truncating identity.

## Layout

The centered outer content container is capped at 1344px, including 32px side
gutters. Phone gutters become 16px. The header pairs the wordmark with theme and
refresh controls; the filter grid follows the header and precedes source groups.
There is no sidebar, decorative dashboard card grid or footer.

Source headings separate configured sources. Each source contains bordered
repository panels, each with one workflow per row and no repository or column
heading. A row title combines the repository link, a hyphen and the workflow
history link; branch scope sits beneath it.

Desktop rows align identity, history, three metrics and latest status. Rows have
20px padding, a minimum height of 108px and 24px column gaps. Repository panels
are separated by 16px and source groups by 32px. At 1190px, rows become two
columns with metrics and latest status below identity and history, and search
spans the filter grid. At 640px, filters form two columns and rows stack identity,
status, history and metrics. Large configured samples scroll within the history
region; they must not widen the page.

## Elevation & Depth

Repository panels use tonal separation and one-pixel borders without shadows.
Raised, sunken and panel shadow tokens are disabled. Preserve Kit UI's transient
menu behavior and chrome; do not add decorative elevation to the overview.
Run tooltips float above content so their exact details remain readable.

## Shapes

Application panel, field and tooltip radius tokens are square (0px). Run bars
are rectangular. Preserve Kit UI's native component silhouettes, including
pill-shaped chips and circular status dots; the square panel language does not
require overriding those component styles. Small focus-outline and skeleton
corners are implementation details, not a second panel style.

## Components

### Shared controls

SearchInput, SelectDropdown, Button, ThemeToggle, Chip and EmptyState come
directly from Kit UI. Keep default control sizes, padding and font-size tokens;
do not scale them down to match surrounding metadata. Buttons and SearchInput
use their default medium size; Chip also defaults to medium. Theme and palette
overrides change their visual character without replacing their sizing contract.

Search is labeled for assistive technology, and selects remain keyboard
accessible. The search field wrapper owns its amber focus border, avoiding a
second outline on the nested input. Select triggers stay still while their
fixed-position menus are measured and opened. Filters live in URL query
parameters and survive refresh, back and forward navigation.

### Workflow rows and history

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
amber two-pixel outline with an offset; Kit UI owns shared control focus.
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
- **Don't** add shadows, rounded repository cards or a sidebar to the overview.
- **Don't** invent metrics, infer links or use color as the only status signal.
- **Don't** hide errors, pending collection or unavailable selections behind an empty state.
