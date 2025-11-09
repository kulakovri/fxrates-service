#!/usr/bin/env bash
set -euo pipefail

COMPOSE_FILE="ops/docker/docker-compose.yml"
COMPOSE_CMD=(docker compose -f "$COMPOSE_FILE")

function start_all() {
  "${COMPOSE_CMD[@]}" up -d
}

function restart_api() {
  "${COMPOSE_CMD[@]}" stop api worker
  "${COMPOSE_CMD[@]}" rm --force api worker
  "${COMPOSE_CMD[@]}" build --no-cache api worker
  "${COMPOSE_CMD[@]}" up -d api worker
}

case "${1:-start}" in
  start)
    start_all
    ;;
  restart)
    restart_api
    ;;
  *)
    echo "Usage: $0 [start|restart]"
    exit 1
    ;;
esac

