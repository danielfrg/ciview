# Product

## Register

product

## Users

Developers checking GitHub Actions and Forgejo Actions across repositories,
accounts, and organizations from a desktop browser or phone.

## Product Purpose

Provide a read-only view of workflow state, recent runs, and timing in one
place. Success means spotting a failure or an incomplete collection, inspecting
recent history, and opening the exact provider run quickly.

## Brand Personality

Technical, restrained, and trustworthy. Real workflow data carries the hierarchy.

## Anti-references

Avoid decorative analytics dashboards, invented metrics, ambiguous status
colors, and controls for triggering or canceling provider runs.

## Design Principles

- Group workflows by source and repository with broad horizontal run history.
- Keep provider links exact and metrics owned by the Go server.
- Preserve distinct terminal outcomes and unavailable timing.
- Retain useful rows during outages and explain stale or partial collection.
- Keep filters in the URL and credentials on the server.

## Accessibility & Inclusion

Support keyboard access to run details, text labels alongside status colors,
readable light and dark themes, reduced motion, and phone layouts. Long names
wrap; large run samples scroll inside their own region.
