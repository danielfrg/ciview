# CI View

GitHub Actions and Forgejo Actions viewer.

## Local development

Requires Go 1.25, Node 26, pnpm 10 and just.

```shell
pnpm --dir frontend install --frozen-lockfile
cp ciview.example.toml ciview.toml
```

Edit `ciview.toml` with your source selections and credential references.
See [configuration](docs/configuration.md) and [credentials](docs/credentials.md).
Start the API and frontend in separate terminals:

```shell
just api
just frontend
```

Open `http://127.0.0.1:5173`.

## Build

```shell
just check
just build
```

The Go executable embeds the frontend. `CIVIEW_CONFIG` selects the TOML file;
`CIVIEW_ADDR` selects the listen address, defaulting to `127.0.0.1:8080`.

## Container image

The [Docker workflow](.github/workflows/docker.yml) builds Linux amd64 and arm64
images on pull requests. The Dockerfile runs the frontend tests, type checks and
build, then Go tests and vet before compiling the embedded server.

Pushes to `main` and manual runs on `main` publish
`ghcr.io/danielfrg/ciview:latest` and `ghcr.io/danielfrg/ciview:sha-<commit>`.
Publishing uses the built-in `GITHUB_TOKEN`; no additional CI secrets are needed.

## Config

Configuration is done via TOML files.

```toml
# Public GitHub access works without credentials
[viewer]
refresh_interval = "10m"
discovery_interval = "15m"
history_runs = 30
history_pages = 5
branch = "default"

[[sources]]
name = "github"
provider = "github"
repos = ["actions/checkout"]

# Using users or orgs will read all repos (ignoring archived or forks).
users = ["danielfrg"]

# For private repositories or a higher GitHub API allowance, add one credential:
token_env = "CIVIEW_GITHUB_TOKEN"
token_file = "/run/secrets/github-token"
orgs = ["example-org"]
repos = ["example-org/private-repository"]

# Forgejo instances as separate sources. base_url is the instance root.
[[sources]]
name = "forgejo"
provider = "forgejo"
base_url = "https://forgejo.example.net"
token_env = "CIVIEW_FORGEJO_TOKEN"

users = ["danielfrg"]
orgs = ["example-org"]
repos = ["example-owner/example-repository"]
```

`branch = "default"` uses each repository's default branch (usually `main`).
A named branch such as `branch = "release"` uses that branch instead.

Sources include historical workflows by default, even when their files have
been deleted. Rows show a **Not on branch** badge when a successful check confirms
that the workflow file is absent from the selected branch. Unverified file status
and all-branch views receive no removal badge. To hide those workflows for one
source, set this inside its `[[sources]]` block:

```toml
existing_workflows_only = true
```

The default is `false`. When enabled, only workflows whose files still exist
on the selected branch are shown; removed workflows disappear after the next
successful refresh. If branch contents cannot be read, previous rows remain
visible as stale data. This option has no effect with `branch = "all"`, which
keeps historical workflows across all branches. GitHub fine-grained tokens need
repository Contents read access in addition to Actions read access to verify
workflow files. Without Contents access, history remains available when this
option is disabled, with a file-status notice.

GitHub-generated Dependabot update and dependency-graph workflows are hidden
by default. To include them for a source, add this to its `[[sources]]` block:

```toml
show_dependabot_updates = true
```

The default is `false`, including with `branch = "all"`. This matches GitHub's
`dynamic/dependabot/` workflow paths; regular CI workflows triggered by Dependabot
pull requests remain visible within the configured branch scope. The
`existing_workflows_only` filter still applies when enabled, so leave it `false`
to include generated workflows in named/default branch views.

Forgejo token requires:

- Repository → Read (read:repository)
- User → Read (read:user)
- Organization → Read (read:organization)
