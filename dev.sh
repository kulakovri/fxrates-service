#!/usr/bin/env bash
set -euo pipefail

# --- config -------------------------------------------------------------------
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
APP_NAME="fxrates-service"
COMPOSE="${ROOT_DIR}/ops/docker/docker-compose.yml"
MIGRATIONS_DIR="${ROOT_DIR}/migrations"
API_SPEC="${ROOT_DIR}/api/openapi.yaml"
GEN_API="${ROOT_DIR}/ops/docker/gen_api.sh"

# Default DATABASE_URL (override in env or .env)
: "${DATABASE_URL:=postgres://postgres:postgres@localhost:5432/fxrates?sslmode=disable}"

# --- helpers ------------------------------------------------------------------
log() { printf "▶ %s\n" "$*"; }
die() { printf "✗ %s\n" "$*" >&2; exit 1; }

usage() {
  cat <<EOF
Usage: $0 <cmd>

# Docker / Infra
  up            Start Postgres (+ adminer) via docker-compose
  down          Stop and remove containers/volumes
  psql          psql shell to DATABASE_URL
  ready         Wait until DB is ready (simple TCP check)

# Dev
  genapi        Regenerate OpenAPI server/types (oapi-codegen)
  migrate       Apply SQL migrations with psql (all *.sql in migrations/)
  run           Run API locally (go run ./cmd/api)
  test          Unit tests (fast)
  test-int      Integration tests (Testcontainers or -tags=integration)
  lint          Go vet / fmt check (optional: golangci-lint if installed)
  swag          Alias for genapi

Examples:
  $0 up && $0 migrate && $0 run
  $0 genapi && $0 test
EOF
}

# --- commands -----------------------------------------------------------------
cmd_up() {
  log "Starting docker-compose..."
  docker compose -f "$COMPOSE" up -d
}

cmd_down() {
  log "Stopping docker-compose..."
  docker compose -f "$COMPOSE" down -v
}

cmd_ready() {
  # naive wait for PG
  log "Waiting for Postgres on localhost:5432..."
  for i in {1..30}; do
    (echo > /dev/tcp/127.0.0.1/5432) >/dev/null 2>&1 && { log "Postgres is up"; return 0; }
    sleep 1
  done
  die "Postgres not reachable"
}

cmd_psql() {
  log "Opening psql..."
  psql "$DATABASE_URL"
}

cmd_genapi() {
  log "Regenerating OpenAPI server/types..."
  bash "$GEN_API"
}

cmd_swag() { cmd_genapi; }

cmd_migrate() {
  log "Applying migrations from ${MIGRATIONS_DIR} to ${DATABASE_URL}"
  [ -d "$MIGRATIONS_DIR" ] || die "No migrations dir at ${MIGRATIONS_DIR}"
  # Apply *.sql in alphabetical order (V1__init.sql, etc.)
  shopt -s nullglob
  for f in "${MIGRATIONS_DIR}"/*.sql; do
    log "Applying $(basename "$f")"
    psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f "$f"
  done
  shopt -u nullglob
  log "Migrations applied."
  # NOTE: If you prefer golang-migrate, replace with its CLI or docker image.
}

cmd_run() {
  log "Running ${APP_NAME}..."
  go run ./cmd/api
}

cmd_test() {
  log "Running unit tests..."
  go test ./... -race -count=1 -run '^(?:(?!Integration).)*$'
}

cmd_test_int() {
  log "Running integration tests..."
  # Option A: build tags (if you use //go:build integration)
  # go test -tags=integration ./internal/test/integration -v -race
  # Option B: run all and rely on package selection:
  go test ./internal/test/integration -v -race || die "integration tests failed"
}

cmd_lint() {
  log "go vet..."
  go vet ./...
  if command -v golangci-lint >/dev/null 2>&1; then
    log "golangci-lint..."
    golangci-lint run
  else
    log "golangci-lint not installed (skipping)."
  fi
  log "fmt check..."
  test -z "$(gofmt -l .)" || { gofmt -l .; die "gofmt needed"; }
  log "lint OK."
}

# --- entry --------------------------------------------------------------------
case "${1:-}" in
  up) cmd_up ;;
  down) cmd_down ;;
  ready) cmd_ready ;;
  psql) cmd_psql ;;
  genapi) cmd_genapi ;;
  swag) cmd_swag ;;
  migrate) cmd_migrate ;;
  run) cmd_run ;;
  test) cmd_test ;;
  test-int) cmd_test_int ;;
  lint) cmd_lint ;;
  ""|-h|--help|help) usage ;;
  *) usage; die "unknown command: $1" ;;
esac
