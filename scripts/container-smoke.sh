#!/usr/bin/env bash
set -euo pipefail
image_ref=${1:?usage: container-smoke.sh IMAGE}
container="ciview-smoke-$$"
cleanup() { docker rm -f "$container" >/dev/null 2>&1 || true; }
trap cleanup EXIT
[[ $(docker image inspect "$image_ref" --format '{{.Config.User}}') == 65532:65532 ]]
docker run -d --name "$container" --read-only --tmpfs /tmp \
  -p 127.0.0.1::8080 "$image_ref" >/dev/null
port=$(docker port "$container" 8080/tcp | head -n 1)
base="http://$port"
ready=false
for _ in {1..60}; do
  if curl --fail --silent "$base/readyz" >/dev/null; then ready=true; break; fi
  sleep .2
done
if [[ "$ready" != true ]]; then docker logs "$container"; exit 1; fi
curl --fail --silent "$base/healthz" >/dev/null
html=$(curl --fail --silent "$base/")
asset=$(printf '%s' "$html" | sed -n 's/.*src="\([^" ]*\.js\)".*/\1/p')
[[ "$asset" == /assets/*.js ]]
curl --fail --silent "$base$asset" >/dev/null
curl --fail --silent "$base/api/docs" >/dev/null
curl --fail --silent "$base/api/openapi.json" >/dev/null
go run ./testdata/compat/mcp_smoke.go "$base/mcp"
printf 'Container verified: nonroot, read-only filesystem, embedded UI, API docs, and probes.\n'
