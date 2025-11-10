#!/usr/bin/env bash
set -euo pipefail

# Ensure protoc plugins are installed and on PATH:
#   go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.33.0
#   go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.4.0
#   export PATH=\"$(go env GOPATH)/bin:$PATH\"
#
# Because the module path is 'fxrates-service' (not a fully-qualified path),
# we must pass module mapping to avoid generating nested 'fxrates-service/' folders.
protoc -I api \
  --go_out=. --go_opt=module=fxrates-service \
  --go-grpc_out=. --go-grpc_opt=module=fxrates-service \
  api/rate.proto


