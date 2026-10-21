# Product

## Users

Developers checking GitHub Actions and Forgejo Actions across repositories,
accounts, and organizations from a desktop browser or phone.

Assumes deployment on a trusted network; there is no auth.

## Product Purpose

Provide a read-only view of workflow state, recent runs, and timing in one
place.

Success means spotting a failure or an incomplete collection, inspecting
recent history, and opening the exact provider run quickly.

## Brand Personality

Technical, restrained, and trustworthy. Real workflow data carries the hierarchy.

### Tone

Let labels and data speak; do not explain the obvious parts of the UI with
text. Add explanatory messages only when collection is stale, partial, or
failing and the user needs to know why.

## Anti-references

Avoid decorative analytics dashboards, invented metrics, ambiguous status
colors, and controls for triggering or canceling provider runs.

## Design Principles

- Group workflows by source and repository with broad horizontal run history.
- Keep provider links exact and metrics owned by the Go server.
- Preserve distinct terminal outcomes and unavailable timing.
- Retain useful rows during outages and explain stale or partial collection.
- Keep filters in the URL and credentials on the server.
- Keep run details keyboard-accessible with text labels alongside status
  colors; long names wrap and large run samples scroll inside their own region.

## Agent Access

Agents use the same product through typed MCP tools that read cached state
without triggering upstream collection. History may be partial; results
preserve exact outcomes, nullable timing, source issues, and freshness. No run
triggering, cancellation, or logs are exposed.
