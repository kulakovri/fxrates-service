#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(pwd)"
API_SPEC="$ROOT_DIR/api/openapi.yaml"
OUT_PKG_DIR="$ROOT_DIR/internal/infrastructure/http/openapi"

# Ensure oapi-codegen is present
if ! command -v oapi-codegen >/dev/null 2>&1; then
  echo "oapi-codegen not found. Installing..."
  go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@2.3.0
fi

rm -rf "$OUT_PKG_DIR"
mkdir -p "$OUT_PKG_DIR"

# Generate types and server stubs for chi
oapi-codegen \
  -generate "types,chi-server" \
  -package openapi \
  -o "$OUT_PKG_DIR/gen.go" \
  "$API_SPEC"

echo "✅ OpenAPI server/types generated at $OUT_PKG_DIR"
