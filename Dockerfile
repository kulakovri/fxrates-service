# syntax=docker/dockerfile:1.6
# --- builder ---
FROM golang:1.23-alpine AS build
WORKDIR /app
RUN apk add --no-cache ca-certificates tzdata git

# Cache deps first
COPY go.mod go.sum ./
ENV GOMODCACHE=/go/pkg/mod
ENV GOCACHE=/root/.cache/go-build
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build go mod download

# Copy source
COPY . .

# Build static binaries
ENV CGO_ENABLED=0
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build go build -trimpath -ldflags="-s -w" -o /out/worker ./cmd/worker

# --- runtime (distroless) ---
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/api /usr/local/bin/api
COPY --from=build /out/worker /usr/local/bin/worker
# ship OpenAPI spec for Swagger in container
COPY --from=build /app/api/openapi.yaml /usr/local/share/fxrates/openapi.yaml
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/usr/local/bin/api"]


