# CI viewer

Read-only overview of GitHub Actions and Forgejo Actions.

## Register

product

## Users and purpose

A homelab owner checks CI across personal accounts, organizations and
repositories from a desktop or phone. Success means recognizing the latest state,
inspecting a recent run sample, and opening the exact provider run quickly.

## Principles

- Collect active source repositories; exclude archived repositories and forks.
- Group workflows by source, then repository, with one workflow per row.
- Keep workflow identity compact and recent history broad and horizontal.
- Show real status, timing and coverage. Missing timing stays unavailable.
- Preserve useful rows during outages and explain stale or partial data.
- Keep configuration and credentials on the Go server.
- Search and filters survive refresh and browser navigation.
- The providers own logs and run actions; CI viewer links to them.
