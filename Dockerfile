# =============================================================================
# Build Stage
# =============================================================================
FROM golang:tip-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /build

# Copy go.mod and go.sum for dependency caching
COPY apps/server/go.mod apps/server/go.sum ./
RUN go mod download

# Copy source code
COPY apps/server/ ./

# Build the binary
# - CGO_ENABLED=0: Build a statically linked binary
# - -ldflags="-w -s": Strip debug info to reduce binary size
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
  -ldflags="-w -s" \
  -o /build/server \
  ./cmd/api

# =============================================================================
# Final Stage - Distroless for minimal attack surface
# =============================================================================
FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app

# Copy CA certificates for HTTPS
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy timezone data
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Copy binary
COPY --from=builder /build/server /app/server

# Copy migrations (if needed later)
COPY --from=builder /build/migrations /app/migrations

# Use nonroot user (uid 65532)
USER nonroot:nonroot

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD ["/app/server", "--health"]

# Run the server
ENTRYPOINT ["/app/server"]
