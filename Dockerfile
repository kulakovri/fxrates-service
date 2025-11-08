# --- builder ---
FROM golang:1.23-alpine AS build
WORKDIR /app
RUN apk add --no-cache ca-certificates tzdata git

# Cache deps first
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build static binary
ENV CGO_ENABLED=0
RUN go build -trimpath -ldflags="-s -w" -o /out/fxrates ./cmd/api

# --- runtime (distroless) ---
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/fxrates /app/fxrates
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/app/fxrates"]


