# Civiewer

Read-only GitHub Actions and Forgejo Actions overview. Scan recent workflow
runs, execution timing and sample success rates, then open the original run.

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

## Build

```shell
just check
just build
```

The Go executable embeds the frontend. `CIVIEWER_CONFIG` selects the TOML file;
`CIVIEWER_ADDR` selects the listen address, defaulting to `127.0.0.1:8080`.

## Deployment

Build with the included Dockerfile. Mount a TOML configuration and optional
credential files, set `CIVIEWER_ADDR=0.0.0.0:8080`, and serve it behind your
existing trusted-network or access-control boundary. Credentials stay in Go;
the browser receives CI metadata. Health is available at `/healthz` and API
documentation at `/api/docs`.
