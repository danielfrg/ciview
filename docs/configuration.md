# Configuration

Copy `civiewer.example.toml` to ignored `civiewer.toml`. Configuration is read
at startup; restart after editing it. `CIVIEWER_CONFIG` chooses another file.

```toml
[viewer]
refresh_interval = "60s"
discovery_interval = "15m"
history_runs = 30
history_pages = 5
branch = "default"

[[sources]]
name = "github"
provider = "github"
token_env = "CIVIEWER_GITHUB_TOKEN"
orgs = ["example-org"]
users = ["example-user"]
repos = ["example-owner/example-repository"]

[[sources]]
name = "forgejo"
provider = "forgejo"
base_url = "https://forgejo.example.net"
token_file = "/run/secrets/forgejo-token"
users = ["example-user"]
repos = ["example-owner/example-repository"]
```

Source names must be unique. Each source selects at least one `orgs` organization,
`users` personal account, or explicit `repos` entry using `owner/repository`.
Their union is deduplicated. Use `users` for a personal account even when its
profile page looks like an organization. User selections discover repositories
owned by those accounts, including private repositories the configured token
can access. Organization selections discover all visible repositories owned by
those organizations. Archived repositories and forks are excluded from all
selections, including explicit `repos`, for both providers.

Forgejo `base_url` is the instance root and may include a path prefix. Use
separate names for instances.

`branch` accepts `default`, `all`, or an explicit branch. `all` combines branches.
Intervals accept 5 seconds through 24 hours. History allows 1–100 runs per
workflow and 1–20 pages per repository; collection remains bounded, so unseen
workflows are outside the observed inventory. Unknown fields fail startup.

The browser filters the cached sample. Refresh reloads it without bypassing the
provider scheduler. Execution describes the newest completed run; missing
timing remains unavailable. Success rate includes successful and failed
terminal runs only.

`CIVIEWER_ADDR` defaults to `127.0.0.1:8080`. For local frontend development,
`CIVIEWER_API_TARGET` defaults to `http://127.0.0.1:8080`.
