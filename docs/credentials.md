# Credentials

Provider tokens are read by the Go server. Use either `token_env` or
`token_file` for a source, never both. Omit both for anonymous public access.
A configured missing, unreadable or empty credential fails startup.

For local development, put the token in an ignored `.env` or export the named
environment variable before `just api`. Store token files outside the repository;
container deployments can mount them under `/run/secrets`. Do not put credentials
in TOML values, browser settings, URLs or committed fixtures.

GitHub fine-grained tokens require **Metadata: Read** for discovery and
**Actions: Read** for workflow history on selected private repositories. Classic
tokens need the `repo` scope for private repositories. User discovery merges
public account repositories with token-visible repositories owned by selected
users, including private repositories shared with the authenticated account.
Token repository selection and organization policy still constrain visibility.
See [GitHub repository discovery](https://docs.github.com/en/rest/repos/repos#list-repositories-for-the-authenticated-user).

Forgejo tokens require **Repository → Read** (`read:repository`) for repository
metadata and Actions history. Add **User → Read** (`read:user`) when using
`users`, and **Organization → Read** (`read:organization`) when using `orgs`.
For private account or organization discovery, choose **All (public, private,
and limited)** when creating the token. **Specific repositories** tokens work
with explicit `repos` entries and `read:repository`, since they cannot include
user or organization scopes. The token's account still needs access to each
repository and its Actions unit. See [Forgejo token scopes](https://forgejo.org/docs/v15.0/user/authentication/token-scope/).

A user or organization lists only repositories visible to its identity. Unreadable
explicit selections appear as notices; permission failures are different from
repositories with no observed runs. Upstream errors are sanitized and tokens
never appear in the overview response. Serve private CI metadata behind an
existing trusted-network or access-control boundary.
