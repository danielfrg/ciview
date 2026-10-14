# Credentials

Provider tokens are read by the Go server. Use either `token_env` or
`token_file` for a source, never both. Omit both for anonymous public access.
A configured missing, unreadable or empty credential fails startup.

For local development, put the token in an ignored `.env` or export the named
environment variable before `just api`. Store token files outside the repository;
container deployments can mount them under `/run/secrets`. Do not put credentials
in TOML values, browser settings, URLs or committed fixtures.

GitHub fine-grained tokens require Actions read permission and metadata access
for selected private repositories. Repository selection and organization policy
still constrain visibility. Forgejo tokens need repository and Actions read
access; organization discovery also requires organization read access. A
repository-specific token can be used with explicit repository selections.

An organization lists only repositories visible to its identity. Unreadable
explicit selections appear as notices; permission failures are different from
repositories with no observed runs. Upstream errors are sanitized and tokens
never appear in the overview response. Serve private CI metadata behind an
existing trusted-network or access-control boundary.
