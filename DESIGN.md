# Civiewer design

## Foundation

Operate mode: broad horizontal workflow rows inspired by Buildkite, compact
system typography and restrained boundaries. The page has a header, filters
and a single shared workflow surface. No sidebar or decorative dashboard cards.

Semantic tokens live in `frontend/src/app.css` and follow system light/dark
preference. Light uses a near-white background, white rows, charcoal text and
cool neutral boundaries; dark uses charcoal surfaces and near-white text.

| Role | Light | Dark |
| --- | --- | --- |
| Background | `#f7f8fa` | `#16191e` |
| Card | `#ffffff` | `#1d2229` |
| Foreground | `#20252c` | `#e6eaf0` |
| Muted foreground | `#5f6875` | `#a2adbb` |
| Border | `#dce1e7` | `#343d49` |
| Focus ring | `#477aaf` | `#91b8e6` |
| Success | `#258c66` | `#55bf94` |
| Failure | `#d44759` | `#e66b7c` |
| Running | `#3d80ca` | `#6ca9eb` |
| Waiting / queued | `#b1841c` | `#dabb57` |
| Attention | `#c16d32` | `#e29a69` |
| Other outcomes | `#7d8997` | `#758292` |

Status text uses dedicated darker light-theme and lighter dark-theme tokens.
Pair colors with visible latest-status labels and accessible run descriptions.

## Layout and type

Content is capped at 1500px with 32px desktop and 16px phone gutters. Desktop
rows align identity, history, three metrics and latest status. At 1190px rows
use two columns; at 640px identity, status, history and metrics stack. Long
names wrap. Large configured samples scroll inside the history region.

Use the system sans-serif stack, a 1.5rem semibold page heading, .9375rem workflow
titles and compact supporting labels. Metric numerals are tabular. A shared
bordered surface and row separators establish hierarchy without shadows.

## Interaction and states

Native labeled search/select controls and local Button/Input/Select primitives
share semantic fields and visible focus. Filters persist in the URL. Refresh
reloads cached server state. Background reads retain the previous overview on
failure and expose Retry; upstream health remains separately visible.

History shows the server's complete displayed sample, normally 30 runs, oldest
to newest. Each colored bar has constant 28px visual height inside a 44px link
area. Hover and focus reveal exact title, branch, status, local time and known
execution duration. Links open provider pages in a new tab. No inferred run
links or invented metrics. Missing metrics use an em dash; zero duration is 0s.

Keep initial loading, collection in progress, no observed workflows, no filter
matches, request failure, stale repositories and unavailable selections distinct.
Limited history is labeled beside the branch scope. Collection timestamps come
from provider health, never browser request time. Reduced motion is respected.
