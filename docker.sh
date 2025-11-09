#!/usr/bin/env bash
set -euo pipefail

COMPOSE_FILE="ops/docker/docker-compose.yml"

compose() {
  docker compose -f "$COMPOSE_FILE" "$@"
}

build_image() {
  local no_cache_flag=""
  if [[ "${1:-}" == "--no-cache" ]]; then
    no_cache_flag="--no-cache"
  fi
  docker build $no_cache_flag -t fxrates:dev .
}

start_all() {
  if ! docker image inspect fxrates:dev >/dev/null 2>&1; then
    build_image
  fi
  compose up -d
}

restart_stack() {
  compose stop api worker
  compose rm -f api worker >/dev/null 2>&1 || true
  build_image --no-cache
  compose up -d api worker
}

case "${1:-start}" in
  start)
    start_all
    ;;
  restart)
    restart_stack
    ;;
  *)
    echo "Usage: $0 [start|restart]"
    exit 1
    ;;
esac

