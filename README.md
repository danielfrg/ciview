# CI viewer

GitHub Actions and Forgejo Actions viewer.

## Local development

Requires Go 1.25, Node 26, pnpm 10 and just.

```shell
pnpm --dir frontend install --frozen-lockfile
cp civiewer.example.toml civiewer.toml
```

Edit `civiewer.toml` with your source selections and credential references.
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

The Go executable embeds the frontend. `CIVIEWER_CONFIG` selects the TOML file;
`CIVIEWER_ADDR` selects the listen address, defaulting to `127.0.0.1:8080`.

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
token_env = "CIVIEWER_GITHUB_TOKEN"
token_file = "/run/secrets/github-token"
orgs = ["example-org"]
repos = ["example-org/private-repository"]

# Forgejo instances as separate sources. base_url is the instance root.
[[sources]]
name = "forgejo"
provider = "forgejo"
base_url = "https://forgejo.example.net"
token_env = "CIVIEWER_FORGEJO_TOKEN"

users = ["danielfrg"]
orgs = ["example-org"]
repos = ["example-owner/example-repository"]
```

Forgejo token requires:

- Repository → Read (read:repository)
- User → Read (read:user)
- Organization → Read (read:organization)
