# Build stage
FROM golang:1.23-alpine AS builder

WORKDIR /build

# Install build dependencies
RUN apk add --no-cache git make

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build abot-server binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags="-w -s" -o abot-server ./cmd/abot-server

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata docker-cli

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/abot-server .

# Copy config example
COPY --from=builder /build/cmd/abot-server/config.server.example.yaml ./config.example.yaml

# Expose API port
EXPOSE 3000

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:3000/health || exit 1

# Run as non-root user
RUN addgroup -g 1000 abot && \
    adduser -D -u 1000 -G abot abot && \
    chown -R abot:abot /app

USER abot

ENTRYPOINT ["./abot-server"]
CMD ["-config", "config.yaml"]
