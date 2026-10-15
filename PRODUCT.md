# CI View

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
- Sources preserve historical workflows by default. They can opt into showing
  only workflow files still on the selected named/default branch; all-branch
  views preserve historical workflows.
- Mark historical workflows whose files are verified absent from the selected
  branch; unavailable file checks must not imply removal.
- Keep workflow identity compact and recent history broad and horizontal.
- Hide GitHub-generated Dependabot workflows by default, with a per-source
  opt-in; preserve regular CI triggered by dependency-update pull requests.
- Show real status, timing and coverage. Missing timing stays unavailable.
- Preserve useful rows during outages and explain stale or partial data.
- Keep configuration and credentials on the Go server.
- Search and filters survive refresh and browser navigation.
- The providers own logs and run actions; CI View links to them.
