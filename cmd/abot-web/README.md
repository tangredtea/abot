# abot-web

Complete Web Console for abot (API + Web UI).

## Features

- Full Web UI for agent management
- RESTful API
- WebSocket support for real-time chat
- JWT authentication
- Multi-tenant support
- User management
- Session history
- Agent configuration

## Installation

### Binary

```bash
go install github.com/yourusername/abot/cmd/abot-web@latest
```

### Docker

```bash
docker pull abot/web:latest
```

## Quick Start

### 1. Prepare Configuration

```bash
cp config.web.example.yaml config.yaml
vim config.yaml  # Fill in your credentials
```

### 2. Build Web UI (if not using Docker)

```bash
cd web
npm install
npm run build
cd ..
```

### 3. Start MySQL

```bash
docker run -d \
  --name mysql \
  -e MYSQL_ROOT_PASSWORD=rootpass \
  -e MYSQL_DATABASE=abot \
  -p 3306:3306 \
  mysql:8
```

### 4. Run Web Console

```bash
abot-web -config config.yaml
```

### 5. Open Browser

```
http://localhost:3000
```

Default credentials:
- Email: `admin@abot.run`
- Password: `changeme`

## Docker Deployment

### Docker Run

```bash
docker run -d \
  --name abot-web \
  -p 3000:3000 \
  -e MYSQL_DSN="user:pass@tcp(mysql:3306)/abot" \
  -e OPENAI_API_KEY="sk-xxx" \
  -e JWT_SECRET="your-secret" \
  abot/web:latest
```

### Docker Compose

```yaml
version: '3.8'
services:
  abot-web:
    image: abot/web:latest
    ports:
      - "3000:3000"
    environment:
      - MYSQL_DSN=root:rootpass@tcp(mysql:3306)/abot
      - OPENAI_API_KEY=${OPENAI_API_KEY}
      - JWT_SECRET=${JWT_SECRET}
    volumes:
      - ./web/out:/app/web/out  # Optional: mount custom UI
    depends_on:
      - mysql

  mysql:
    image: mysql:8
    environment:
      - MYSQL_ROOT_PASSWORD=rootpass
      - MYSQL_DATABASE=abot
    volumes:
      - mysql-data:/var/lib/mysql

volumes:
  mysql-data:
```

### One-Click Deploy

```bash
curl -fsSL https://get.abot.run | sh
```

## Web UI Features

### Dashboard
- Agent status overview
- Recent conversations
- System metrics

### Agent Management
- Create/edit/delete agents
- Configure system prompts
- Enable/disable tools
- Set model parameters

### Chat Interface
- Real-time streaming
- Multi-agent switching
- Session history
- Export conversations

### User Management
- Create users
- Assign roles
- Manage permissions

### Settings
- Provider configuration
- Channel setup
- MCP servers
- Vector database

## Configuration

See `config.web.example.yaml` for all available options.

## Environment Variables

- `MYSQL_DSN` - MySQL connection string
- `OPENAI_API_KEY` - OpenAI API key
- `JWT_SECRET` - JWT signing secret

## API Documentation

The Web Console includes the same API as `abot-server`.

See [API.md](../../docs/API.md) for complete API reference.

## Custom Frontend

You can replace the default Web UI with your own:

1. Build your frontend
2. Place output in `web/out/`
3. Set `console.static_dir` in config
4. Restart abot-web

## Monitoring

### Health Check

```bash
curl http://localhost:3000/health
```

### Metrics (if enabled)

```bash
curl http://localhost:3000/metrics
```

## Troubleshooting

### Web UI Not Loading

```bash
# Check static files exist
ls web/out/

# Check console.static_dir in config
grep static_dir config.yaml
```

### Database Connection Failed

```bash
# Check MySQL is running
docker ps | grep mysql

# Test connection
mysql -h localhost -u root -p
```

### Cannot Login

```bash
# Reset admin password
abot-web -config config.yaml -reset-admin
```

## Development

### Run in Development Mode

```bash
# Terminal 1: Start backend
abot-web -config config.yaml

# Terminal 2: Start frontend dev server
cd web
npm run dev
```

Frontend will proxy API requests to backend.

## License

MIT
