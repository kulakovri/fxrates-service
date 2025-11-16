#!/usr/bin/env bash
set -euo pipefail

PROFILE=""
BUILD=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    -p|--profile)
      PROFILE="${2:-}"
      shift 2
      ;;
    -b|--build)
      BUILD=1
      shift
      ;;
    *)
      echo "Usage: $0 [-b|--build] [-p|--profile chan|db|grpc]"
      exit 1
      ;;
  esac
done

if [[ ! -f .env ]]; then
  echo "   .env not found in repo root."
  echo "   Create .env with PROVIDER( fake | exchangeratesapi ), EXCHANGE_API_BASE, EXCHANGE_API_KEY, etc."
  exit 1
fi

if (( BUILD )); then
  echo ">>> Building fxrates:dev"
  docker build -t fxrates:dev .
fi

compose="docker compose --env-file .env -f ops/docker/docker-compose.yml"

start_profile() {
  local p="$1"
  local project
  case "$p" in
    chan) project="fxrates-chan" ;;
    db)   project="fxrates-db" ;;
    grpc) project="fxrates-grpc" ;;
    *)
      echo "Unknown profile: $p (expected chan|db|grpc)"
      exit 1
      ;;
  esac

  echo ">>> Starting profile: $p (project: $project)"
  $compose -p "$project" --profile "$p" up -d
}

if [[ -n "$PROFILE" ]]; then
  start_profile "$PROFILE"
else
  start_profile chan
  start_profile db
  start_profile grpc
fi