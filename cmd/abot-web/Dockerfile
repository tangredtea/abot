# Build stage - Backend
FROM golang:1.21-alpine AS backend-builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build binary
RUN CGO_ENABLED=0 GOOS=linux go build -o abot-web ./cmd/abot-web

# Build stage - Frontend
FROM node:18-alpine AS frontend-builder

WORKDIR /app

# Copy frontend files
COPY web/package*.json ./
RUN npm ci

COPY web/ ./
RUN npm run build

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates docker-cli

WORKDIR /app

# Copy backend binary
COPY --from=backend-builder /app/abot-web .

# Copy frontend build
COPY --from=frontend-builder /app/out ./web/out

# Expose Web Console port
EXPOSE 3000

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:3000/health || exit 1

ENTRYPOINT ["./abot-web"]
