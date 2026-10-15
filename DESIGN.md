# CI View design

## Foundation

Operate mode: broad horizontal workflow rows inspired by Buildkite, compact
system typography and restrained boundaries. The page has a header, filters
and source groups containing repository panels and workflow rows. No sidebar or
decorative dashboard cards.

Kit UI (`@kenn-io/kit-ui`) supplies plain CSS tokens and reusable components.
The dependency is pinned to an upstream GitHub commit in `frontend/package.json`.
`frontend/src/app.css` maps its semantic palette to CI run outcomes and owns
the workflow layout.
Tailwind is not used. Light/dark mode follows the system initially; the header
ThemeToggle persists an explicit choice under `ciview-theme`.

Import Kit UI controls directly from `@kenn-io/kit-ui`. Keep local components
for CI presentation or app-specific behavior rather than pass-through aliases.
Keep visible terminal outcomes and accessible run descriptions; never collapse
distinct outcomes.

## Layout and type

Content is capped at 1500px with 32px desktop and 16px phone gutters. Desktop
rows align identity, history, three metrics and latest status. At 1190px rows
use two columns; at 640px identity, status, history and metrics stack. Long
names wrap. Large configured samples scroll inside the history region.

Use the system sans-serif stack, a 1.5rem semibold page heading, .9375rem workflow
titles and compact supporting labels. Metric numerals are tabular. Source
headings separate configured sources. Bordered repository panels contain rows
without repository or column headings. Each row title combines the repository
link, a hyphen, and the workflow history link; branch scope stays on the next line.
Spacing and row separators establish grouping without shadows.

## Interaction and states

Labeled Kit UI search and keyboard-accessible select controls share semantic
fields and visible focus. Filters persist in the URL. Refresh reloads cached
server state. Background reads retain the previous overview on
failure and expose Retry; upstream health remains separately visible.

History shows the server's complete displayed sample, normally 30 runs, oldest
to newest. Each colored bar has constant 28px visual height inside a 44px link
area. Hover and focus reveal exact title, branch, status, local time and known
execution duration. Workflow names open provider workflow history; history bars
and latest status open individual runs. Links open provider pages in a new tab. No inferred run
links or invented metrics. Missing metrics use an em dash; zero duration is 0s.

Keep initial loading, collection in progress, no observed workflows, no filter
matches, request failure, stale repositories and unavailable selections distinct.
Rows show branch scope without observed-run counts or coverage labels. A small
warning Chip labeled "Not on branch" sits beside the branch when the server has
verified that the workflow file is absent. Its tooltip explains that historical
runs remain available. This marker is independent of run outcome and stale-data
notices; unknown file status and all-branch views have no removal marker. The
header has Refresh without collection timestamps. Source diagnostics appear
in a native disclosure below workflows, collapsed by default; successful empty
scans produce no notices. Errors, pending sources and incomplete scans retain
their context, as do stale-data notices on rows. There is no footer. Reduced
motion is respected.
