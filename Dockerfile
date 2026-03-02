# Multi-stage build for abot binaries
FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build argument to specify which binary to build
ARG BINARY=abot-agent
ARG VERSION=dev

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags "-X main.Version=${VERSION} -s -w" \
    -o /app/bin/${BINARY} \
    ./cmd/${BINARY}

# Final stage
FROM alpine:latest

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1000 abot && \
    adduser -D -u 1000 -G abot abot

WORKDIR /app

# Copy binary from builder
ARG BINARY=abot-agent
COPY --from=builder /app/bin/${BINARY} /app/abot

# Create data directory
RUN mkdir -p /app/data && chown -R abot:abot /app

USER abot

EXPOSE 8080

ENTRYPOINT ["/app/abot"]
CMD ["-config", "/app/config.yaml"]
