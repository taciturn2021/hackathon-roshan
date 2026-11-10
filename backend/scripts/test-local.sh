#!/usr/bin/env bash
# Runs go vet and the Go tests locally in Docker, against throwaway Postgres 16 + Redis containers,
# so the database tests run instead of skipping. Needs only Docker (no Go, no server access).
#
#   backend/scripts/test-local.sh                          # go vet + go test -race ./...
#   backend/scripts/test-local.sh -run Accept ./internal/jobs/
set -euo pipefail

here="$(cd "$(dirname "$0")/.." && pwd)"
args=("$@")
[ ${#args[@]} -eq 0 ] && args=(./...)

net=rr-local; pg=rr-local-pg; rd=rr-local-redis
docker network inspect "$net" >/dev/null 2>&1 || docker network create "$net" >/dev/null
docker rm -f "$pg" "$rd" >/dev/null 2>&1 || true
trap 'docker rm -f "$pg" "$rd" >/dev/null 2>&1 || true' EXIT
docker run -d --name "$pg" --network "$net" -e POSTGRES_PASSWORD=test --tmpfs /var/lib/postgresql/data postgres:16-alpine >/dev/null
docker run -d --name "$rd" --network "$net" redis:7-alpine >/dev/null
for _ in $(seq 60); do docker exec "$pg" pg_isready -U postgres >/dev/null 2>&1 && break; sleep 1; done

docker run --rm --network "$net" \
  -v "$here:/src" -v rr-local-gomod:/go/pkg/mod -v rr-local-gocache:/root/.cache/go-build -w /src \
  -e TEST_DATABASE_URL="postgres://postgres:test@$pg:5432/postgres?sslmode=disable" \
  -e TEST_REDIS_ADDR="$rd:6379" \
  golang:1.27 sh -c 'go vet ./... && go test -race -count=1 "$@"' sh "${args[@]}"
