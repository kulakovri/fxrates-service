# --- builder ---
FROM golang:1.23-alpine AS build
WORKDIR /app
RUN apk add --no-cache ca-certificates tzdata git

# Cache deps first
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build static binaries
ENV CGO_ENABLED=0
RUN go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api
RUN go build -trimpath -ldflags="-s -w" -o /out/worker ./cmd/worker

# --- runtime (distroless) ---
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/api /usr/local/bin/api
COPY --from=build /out/worker /usr/local/bin/worker
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/usr/local/bin/api"]


