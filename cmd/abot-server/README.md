# abot-server

API-only mode for ABot - provides REST API endpoints without Web UI.

## Features

- Pure API server (no static file serving)
- Full agent management via REST API
- JWT authentication
- MySQL-backed persistence
- Docker support
- Health check endpoint

## Quick Start

### 1. Configuration

Copy the example config:

```bash
cp config.server.example.yaml config.yaml
```

Edit `config.yaml` and set:
- `mysql_dsn`: Your MySQL connection string
- `providers[0].api_key`: Your LLM API key
- `console.jwt_secret`: A random secret (at least 32 chars)
- `console.allowed_origins`: Your frontend URLs

### 2. Run Locally

```bash
# Build
go build -o abot-server ./cmd/abot-server

# Run
./abot-server -config config.yaml
```

Server starts on `http://localhost:3000` by default.

### 3. Run with Docker

```bash
# Build image
docker build -t abot-server -f cmd/abot-server/Dockerfile .

# Run container
docker run -d \
  --name abot-server \
  -p 3000:3000 \
  -v $(pwd)/config.yaml:/app/config.yaml:ro \
  abot-server
```

### 4. Run with Docker Compose

Create `docker-compose.yml`:

```yaml
version: '3.8'

services:
  mysql:
    image: mysql:8.0
    environment:
      MYSQL_ROOT_PASSWORD: rootpass
      MYSQL_DATABASE: abot
      MYSQL_USER: abot
      MYSQL_PASSWORD: abotpass
    volumes:
      - mysql_data:/var/lib/mysql
    ports:
      - "3306:3306"

  abot-server:
    build:
      context: .
      dockerfile: cmd/abot-server/Dockerfile
    ports:
      - "3000:3000"
    volumes:
      - ./config.yaml:/app/config.yaml:ro
    depends_on:
      - mysql
    environment:
      - TZ=UTC

volumes:
  mysql_data:
```

Run:

```bash
docker-compose up -d
```

## API Endpoints

All API endpoints are under `/api/` prefix:

- `POST /api/auth/login` - User login
- `POST /api/auth/register` - User registration
- `GET /api/agents` - List agents
- `POST /api/agents` - Create agent
- `GET /api/agents/:id` - Get agent details
- `PUT /api/agents/:id` - Update agent
- `DELETE /api/agents/:id` - Delete agent
- `POST /api/chat` - Send chat message
- `GET /api/sessions` - List chat sessions
- `GET /health` - Health check

See full API documentation at `/api/docs` (if enabled).

## Configuration

Key configuration options:

```yaml
# MySQL (required)
mysql_dsn: "user:pass@tcp(host:3306)/dbname?charset=utf8mb4&parseTime=True"

# API server
console:
  addr: ":3000"                    # Listen address
  jwt_secret: "your-secret-here"   # JWT signing key
  allowed_origins:                 # CORS origins
    - "https://your-frontend.com"

# LLM provider
providers:
  - name: primary
    api_base: "https://api.openai.com/v1"
    api_key: "sk-..."
    model: "gpt-4o-mini"
```

## Environment Variables

You can override config with environment variables:

- `ABOT_MYSQL_DSN` - MySQL connection string
- `ABOT_JWT_SECRET` - JWT secret
- `ABOT_API_KEY` - LLM API key
- `ABOT_LISTEN_ADDR` - Server listen address

## Health Check

```bash
curl http://localhost:3000/health
# Response: OK
```

## Production Deployment

1. Use a strong JWT secret (32+ random characters)
2. Enable HTTPS (use reverse proxy like nginx/Caddy)
3. Set specific CORS origins (avoid `*`)
4. Use connection pooling for MySQL
5. Monitor `/health` endpoint
6. Set resource limits in Docker/K8s

## Differences from abot-web

| Feature | abot-server | abot-web |
|---------|-------------|----------|
| Web UI | ❌ No | ✅ Yes |
| API endpoints | ✅ Yes | ✅ Yes |
| Static files | ❌ No | ✅ Yes |
| Binary size | ~15MB | ~20MB |
| Use case | API backend | Full-stack |

## Troubleshooting

**Server won't start:**
- Check MySQL connection (`mysql_dsn`)
- Verify port 3000 is available
- Check logs for errors

**Authentication fails:**
- Verify `jwt_secret` is set
- Check token expiry (default 24h)
- Ensure CORS origins are correct

**Database errors:**
- Run migrations: `./abot-server -config config.yaml` (auto-migrates on start)
- Check MySQL version (8.0+ recommended)

## License

See root LICENSE file.
