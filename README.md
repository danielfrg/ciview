# CI View

A read-only GitHub Actions and Forgejo Actions viewer. Ready for self-hosting.

Browse workflows, recent runs, timing, and status across repositories,
accounts, and organizations in one place.

![CI View](docs/screenshots/homepage.png)

## Installation

### Docker

Copy [ciview.example.toml](ciview.example.toml) to `ciview.toml`, edit your
sources, then run:

```zsh
docker run -d --name ciview \
  -p 127.0.0.1:8080:8080 \
  -v "${PWD}/ciview.toml:/app/ciview.toml:ro" \
  ghcr.io/danielfrg/ciview:latest
```

- Open [http://localhost:8080](http://localhost:8080)
- Pass referenced token environment variables with `-e`, or mount token files
  read-only at the paths in the configuration
- The container runs as UID/GID 65532

Published images support Linux AMD64 and ARM64. `latest` tracks `main`.

### Standalone

Build a single executable with `just build` (see [Development](#development)),
then run:

```zsh
cp ciview.example.toml ciview.toml
./ciview
```

Open [http://127.0.0.1:8080](http://127.0.0.1:8080).

## Usage

Search and filter by provider, source, repository, or latest status. Filters
survive reloads and browser navigation. Workflow titles open provider history;
run bars and status links open the exact run. Hover or focus a bar for details.
Refresh reloads the cached overview; the server schedules upstream collection.

Provider failures retain useful rows and identify stale or incomplete data.
Source notices explain collection issues. CI View does not trigger or cancel
runs. API documentation is at `/api/docs`.

### MCP

Agents can read cached CI state through the MCP endpoint at `/mcp`
(Streamable HTTP). Tools: `list_workflows`, `get_workflow`, `list_runs`, and
`get_source_status`. All tools read cached state without triggering upstream
collection; no run triggering, cancellation, or logs are exposed. List tools
accept `limit` (default 20, maximum 100) and nonnegative `offset`.

MCP only accepts requests addressed to localhost. Behind a proxy or container
network, set `CIVIEW_MCP_ALLOWED_HOSTS` to a comma-separated list of exact
hostnames or IP addresses (no schemes, ports, or wildcards).

### Security

There is no authentication: anyone who can reach the service can see the
configured CI data, so run it on a trusted network or behind an authenticated
proxy. Credential references are resolved on the server; tokens never reach
the browser.

## Configuration

Sources are configured in a TOML file:

```toml
# Public GitHub access works without credentials. Copy to ciview.toml to run.
[viewer]
refresh_interval = "10m"
discovery_interval = "10m"
history_runs = 30
history_pages = 5
branch = "default"

[[sources]]
name = "github"
provider = "github"
# Set true to hide workflows whose files no longer exist on the selected branch.
existing_workflows_only = false
# Generated Dependabot update and dependency-graph workflows are hidden by default.
show_dependabot_updates = false
repos = ["actions/checkout"]

# For private repositories or a higher GitHub API allowance, add one credential:
# token_env = "CIVIEW_GITHUB_TOKEN"
# token_file = "/run/secrets/github-token"
# orgs = ["example-org"]
# users = ["example-user"]
# repos = ["example-owner/example-private-repository"]

# Add named Forgejo instances as separate sources. base_url is the instance root.
# [[sources]]
# name = "forgejo"
# provider = "forgejo"
# base_url = "https://forgejo.example.net"
# token_env = "CIVIEW_FORGEJO_TOKEN"
# orgs = ["example-org"]
```

`branch = "default"` uses each repository's default branch; a named branch
such as `branch = "release"` uses that branch instead, and `branch = "all"`
keeps workflows across all branches.

Sources include historical workflows by default, even when their files have
been deleted; rows show a **Not on branch** badge when a successful check
confirms the file is absent from the selected branch. Set
`existing_workflows_only = true` on a source to hide them after the next
successful refresh (no effect with `branch = "all"`; stale rows remain if
branch contents cannot be read). GitHub-generated Dependabot update and
dependency-graph workflows (`dynamic/dependabot/` paths) are hidden by
default; set `show_dependabot_updates = true` on a source to include them.
Regular CI workflows triggered by Dependabot pull requests remain visible.

GitHub fine-grained tokens need Actions read access, plus Contents read
access to verify workflow files. Forgejo tokens need Repository, User, and
Organization read access. Forgejo rows use the selected branch's workflow
`name:` when available and fall back to the workflow filename.

| Variable                   | Default                                      |
| -------------------------- | -------------------------------------------- |
| `CIVIEW_CONFIG`            | `ciview.toml`                                |
| `CIVIEW_ADDR`              | `127.0.0.1:8080`                             |
| `CIVIEW_MCP_ALLOWED_HOSTS` | unset (MCP allows localhost only)            |
| `CIVIEW_API_TARGET`        | `http://127.0.0.1:8080` (frontend dev proxy) |

Liveness is `/healthz`; readiness is `/readyz`. Both check the running HTTP
service without depending on provider availability, so upstream outages do
not remove access to cached data.

## Development

Requires Go 1.25, Node 26, pnpm 10, and just.

Start the API and Frontend in two terminals:

```shell
# Terminal 1:
just install
cp ciview.example.toml ciview.toml
just api

# Terminal 2:
just frontend
```

Open [http://127.0.0.1:5173](http://127.0.0.1:5173). Vite proxies API requests
to the Go server. Run `just check` before sending changes. See `AGENTS.md` for
project structure and conventions.

## Build

```zsh
just build
```

```zsh
just image-check
```
