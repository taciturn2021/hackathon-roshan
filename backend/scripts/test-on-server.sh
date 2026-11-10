#!/usr/bin/env bash
# Runs the Go test suite on the deploy server (no Docker needed locally), against a throwaway
# Postgres 16 container, so database tests (testdb.New) run instead of skipping.
#
#   backend/scripts/test-on-server.sh                 # go test -race ./...
#   backend/scripts/test-on-server.sh -run Session ./internal/identity/
#
# Needs: ssh host alias $TEST_HOST (default "oracle") with Docker. Uses only rr-test-* names; no ports published.
set -euo pipefail

HOST="${TEST_HOST:-oracle}"
DIR="hackathon-test"
here="$(cd "$(dirname "$0")/.." && pwd)"
args=("$@")
[ ${#args[@]} -eq 0 ] && args=(./...)

ssh "$HOST" mkdir -p "$DIR/backend"
rsync -az --delete --exclude media "$here/" "$HOST:$DIR/backend/"

# shellcheck disable=SC2029
ssh "$HOST" bash -s -- "$DIR" "${args[@]}" <<'REMOTE'
set -euo pipefail
dir="$HOME/$1"; shift
net=rr-test; pg=rr-test-pg
docker network inspect "$net" >/dev/null 2>&1 || docker network create "$net" >/dev/null
docker rm -f "$pg" >/dev/null 2>&1 || true
trap 'docker rm -f "$pg" >/dev/null 2>&1 || true' EXIT
docker run -d --name "$pg" --network "$net" -e POSTGRES_PASSWORD=test --tmpfs /var/lib/postgresql/data postgres:16-alpine >/dev/null
for _ in $(seq 60); do docker exec "$pg" pg_isready -U postgres >/dev/null 2>&1 && break; sleep 1; done
docker run --rm --network "$net" \
  -v "$dir/backend:/src" -v rr-test-gomod:/go/pkg/mod -v rr-test-gocache:/root/.cache/go-build \
  -w /src -e TEST_DATABASE_URL="postgres://postgres:test@$pg:5432/postgres?sslmode=disable" \
  -e GOTOOLCHAIN=auto golang:1.27 go test -race -count=1 "$@"
REMOTE
