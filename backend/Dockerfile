# ==============================================================================
# Build stage
# ==============================================================================
FROM golang:1.24-alpine AS builder

# Install git (needed for go mod download) and ca-certificates (for HTTPS)
RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /app

# Copy go mod files first (better layer caching)
COPY go.mod go.sum* ./
RUN go mod download

# Copy source code
COPY . .

# Build with optimizations
# CGO_ENABLED=0 for static binary, -ldflags for smaller binary
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-w -s" \
    -o /app/server \
    ./cmd/server

# ==============================================================================
# Runtime stage (minimal image)
# ==============================================================================
FROM alpine:3.19

# Security: run as non-root user
RUN addgroup -g 1000 appgroup && \
    adduser -u 1000 -G appgroup -D appuser

# Install ca-certificates for HTTPS calls and timezone data
RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/server .

# Copy migrations (will be needed later)
#COPY --from=builder /app/migrations ./migrations 2>/dev/null || true
COPY --from=builder /app/migrations* ./migrations/

# Use non-root user
USER appuser

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/healthz || exit 1

# Run the binary
ENTRYPOINT ["/app/server"]
