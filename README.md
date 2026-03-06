# abot - AI Agent Bot Framework

A flexible, production-ready AI agent framework with multiple deployment modes.

## ✨ Features

- 🤖 **Multi-Agent Support** - Run multiple specialized agents with independent configurations
- 🔌 **Pluggable Architecture** - Easy to extend with tools, channels, and providers
- 🌐 **Multiple Deployment Modes** - CLI (REPL), API Server, or Web Console
- 🔒 **Enterprise Ready** - Multi-tenant support, JWT auth, role-based access control
- 💬 **Real-time Communication** - WebSocket streaming for smooth conversations
- 📦 **Easy to Deploy** - Docker, Kubernetes, or standalone binary
- 🛠️ **Rich Tool Ecosystem** - Built-in tools for web search, file operations, and more
- 🔄 **Multiple LLM Providers** - OpenAI, Anthropic, Azure OpenAI, and custom providers

## 🚀 Quick Start

### Option 1: CLI Mode (Fastest)

Perfect for personal use and quick testing.

```bash
# Download binary
wget https://github.com/yourusername/abot/releases/latest/download/abot-agent-linux-amd64.tar.gz
tar -xzf abot-agent-linux-amd64.tar.gz
sudo mv abot-agent /usr/local/bin/

# Create minimal config
cat > config.yaml <<EOF
providers:
  - api_key: sk-xxx
    model: gpt-4o-mini
EOF

# Run
abot-agent -config config.yaml

# Start chatting
You: Hello
Bot: Hello! How can I help you today?
```

### Option 2: Web Console (Full Features)

Best for teams and production use.

```bash
# 1. Start backend (port 3001)
./abot console --config config.test.yaml

# 2. Start frontend (port 3000)
cd web && npm run dev

# 3. Open browser
open http://localhost:3000

# 4. Login with test account
# Email: test@example.com
# Password: Test123456
```

### Option 3: API Server (For Integration)

Ideal for embedding in your applications.

```bash
# Using Docker
docker run -d \
  --name abot-server \
  -p 8080:8080 \
  -e OPENAI_API_KEY=sk-xxx \
  -e MYSQL_DSN="user:pass@tcp(mysql:3306)/abot" \
  abot/server:latest

# Test API
curl http://localhost:8080/api/v1/agents
```

## 📦 Installation

### Binary Installation

**macOS:**
```bash
brew tap yourusername/abot
brew install abot-agent
brew install abot-server
brew install abot-web
```

**Linux:**
```bash
# Download from releases
wget https://github.com/yourusername/abot/releases/latest/download/abot-agent-linux-amd64.tar.gz
tar -xzf abot-agent-linux-amd64.tar.gz
sudo mv abot-agent /usr/local/bin/
```

### Docker Images

```bash
docker pull abot/agent:latest
docker pull abot/server:latest
docker pull abot/web:latest
```

### Build from Source

```bash
git clone https://github.com/yourusername/abot.git
cd abot
make all

# Binaries will be in ./bin/
./bin/abot-agent -config config.yaml
```

## 🏗️ Architecture

abot uses a multi-binary architecture for clear separation of concerns:

```
┌─────────────┐  ┌──────────────┐  ┌─────────────┐
│ abot-agent  │  │ abot-server  │  │  abot-web   │
│   (CLI)     │  │    (API)     │  │ (Web UI)    │
└─────────────┘  └──────────────┘  └─────────────┘
       │                │                  │
       └────────────────┴──────────────────┘
                        │
                ┌───────▼────────┐
                │  pkg/bootstrap │  ← Shared dependency construction
                │  pkg/agent     │  ← Core agent engine
                │  pkg/providers │  ← LLM provider abstraction
                │  pkg/storage   │  ← Pluggable storage backends
                └────────────────┘
```

**Three Independent Binaries:**

- **abot-agent**: CLI REPL for personal use (no database required)
- **abot-server**: HTTP API server for integration (requires MySQL)
- **abot-web**: Web console with UI (requires MySQL)

**Shared Core:**

- `pkg/bootstrap`: Zero-duplication dependency construction
- `pkg/agent`: Core agent engine with tool execution
- `pkg/providers`: Multi-provider LLM abstraction
- `pkg/storage`: Pluggable storage (MySQL, JSONL, in-memory)

## 📖 Documentation

- [Architecture](docs/ARCHITECTURE.md) - System design and component details
- [Deployment Guide](docs/DEPLOYMENT.md) - Production deployment instructions
- [Migration Guide](docs/MIGRATION.md) - Upgrade from single-binary architecture
- [API Reference](docs/API.md) - HTTP API documentation
- [Configuration](docs/CONFIGURATION.md) - All configuration options

## 🎯 Use Cases

### Personal Assistant (CLI)

```bash
abot-agent -config config.yaml

You: Summarize this article: https://example.com/article
Bot: [AI provides summary]

You: Search for "Go best practices"
Bot: [AI searches and summarizes results]
```

### Team Collaboration (Web)

- Multiple users with role-based access
- Shared agents across teams
- Conversation history and search
- Real-time WebSocket streaming
- Multi-tenant isolation

### API Integration (Server)

- Embed AI capabilities in your app
- Custom frontend with your branding
- Webhook support for events
- Multi-tenant API with JWT auth

## 🔧 Configuration

### Minimal Config (CLI Mode)

```yaml
# config.agent.yaml
providers:
  - api_key: sk-xxx
    model: gpt-4o-mini
```

### Full Config (Web Mode)

```yaml
# config.web.yaml
app_name: abot
mysql_dsn: "user:pass@tcp(localhost:3306)/abot?charset=utf8mb4&parseTime=True"

providers:
  - api_key: sk-xxx
    model: gpt-4o-mini
    prompt_caching: true

agents:
  - id: assistant
    name: "AI Assistant"
    model: gpt-4o-mini
    system_prompt: "You are a helpful assistant."

console:
  addr: ":3000"
  jwt_secret: "your-secret-key"
  static_dir: "web/out"
  allowed_origins:
    - "http://localhost:3000"
```

See [example configs](config/) for more options.

## 🛠️ Development

### Backend Development

```bash
# Install dependencies
go mod download

# Build all binaries
make all

# Run tests
go test ./...

# Run with hot reload
air
```

### Frontend Development

```bash
cd web

# Install dependencies
npm install

# Development mode
npm run dev

# Build for production
npm run build
```

### Project Structure

```
abot/
├── cmd/
│   ├── abot-agent/      # CLI binary
│   ├── abot-server/     # API server binary
│   └── abot-web/        # Web console binary
├── pkg/
│   ├── bootstrap/       # Shared dependency construction
│   ├── agent/           # Core agent engine
│   ├── api/             # HTTP API handlers
│   ├── storage/         # Storage layer (MySQL, JSONL)
│   ├── channels/        # Channel adapters (Telegram, Discord, etc.)
│   ├── providers/       # LLM provider implementations
│   ├── tools/           # Built-in tools
│   └── types/           # Shared types
├── web/                 # Next.js frontend
└── docs/                # Documentation
```

## 🚢 Deployment

### Docker Compose (Recommended)

```bash
# Download docker-compose.yml
curl -O https://raw.githubusercontent.com/yourusername/abot/main/docker-compose.yml

# Configure .env
cat > .env <<EOF
OPENAI_API_KEY=sk-xxx
JWT_SECRET=$(openssl rand -hex 32)
MYSQL_ROOT_PASSWORD=secure-password
EOF

# Start
docker-compose up -d

# Check logs
docker-compose logs -f abot-web
```

### Kubernetes

```bash
# Using Helm
helm repo add abot https://charts.abot.run
helm install abot abot/abot \
  --set openai.apiKey=sk-xxx \
  --set mysql.enabled=true \
  --set ingress.enabled=true \
  --set ingress.host=abot.example.com
```

### Systemd Service

```bash
# Install binary
sudo mv abot-web /usr/local/bin/

# Create service
sudo cat > /etc/systemd/system/abot-web.service <<EOF
[Unit]
Description=abot Web Console
After=network.target mysql.service

[Service]
Type=simple
User=abot
ExecStart=/usr/local/bin/abot-web -config /etc/abot/config.yaml
Restart=on-failure

[Install]
WantedBy=multi-user.target
EOF

# Start service
sudo systemctl enable abot-web
sudo systemctl start abot-web
```

See [Deployment Guide](docs/DEPLOYMENT.md) for detailed instructions.

## 🤝 Contributing

We welcome contributions! Here's how you can help:

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'feat: add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

Please read [CONTRIBUTING.md](CONTRIBUTING.md) for details on our code of conduct and development process.

## 📄 License

MIT License - see [LICENSE](LICENSE) for details.

## 🙏 Acknowledgments

- Built with [Google ADK](https://github.com/google/adk) for agent orchestration
- Inspired by [LangChain](https://github.com/langchain-ai/langchain) and [AutoGPT](https://github.com/Significant-Gravitas/AutoGPT)
- UI components from [shadcn/ui](https://ui.shadcn.com/)

## 📞 Support

- 📧 Email: support@abot.run
- 💬 Discord: https://discord.gg/abot
- 🐛 Issues: https://github.com/yourusername/abot/issues
- 📖 Documentation: https://docs.abot.run

## 🗺️ Roadmap

- [ ] GraphQL API support
- [ ] Plugin marketplace
- [ ] Voice input/output
- [ ] Mobile apps (iOS/Android)
- [ ] Distributed tracing (OpenTelemetry)
- [ ] Metrics export (Prometheus)
- [ ] Redis caching layer
- [ ] Message queue integration (RabbitMQ)

---

**Made with ❤️ by the abot team**
