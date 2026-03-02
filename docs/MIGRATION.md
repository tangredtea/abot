# Migration Guide

This guide helps you migrate from the single-binary architecture (v1.x) to the new multi-binary architecture (v2.0).

## Overview

Version 2.0 introduces a significant architectural change: the single `abot` binary with subcommands has been replaced by three specialized binaries:

- `abot-agent` (CLI mode) - replaces `abot agent`
- `abot-server` (API mode) - replaces `abot server`
- `abot-web` (Web Console mode) - replaces `abot console`

## Breaking Changes

### 1. Binary Names Changed

**Old (v1.x):**
```bash
abot agent
abot server
abot console
```

**New (v2.0):**
```bash
abot-agent
abot-server
abot-web
```

### 2. Configuration Validation

Each binary now validates configuration specific to its mode:

- **abot-agent**: Does NOT require `mysql_dsn` (can use in-memory or JSONL sessions)
- **abot-server**: Requires `mysql_dsn`
- **abot-web**: Requires both `mysql_dsn` and `console.static_dir`

### 3. Import Paths (If Using as Library)

**Old:**
```go
import "abot/cmd/abot"
```

**New:**
```go
import "abot/pkg/bootstrap"
```

### 4. Dependency Construction

The dependency construction logic has been moved from `cmd/abot/main.go` to `pkg/bootstrap/`:

**Old:**
```go
// Each binary had duplicate initialization code
func buildDeps(cfg *agent.Config) (*agent.Dependencies, error) {
    // ... lots of duplicate code
}
```

**New:**
```go
// Shared initialization in pkg/bootstrap
deps, err := bootstrap.BuildCoreDeps(cfg)      // For CLI mode
deps, err := bootstrap.BuildFullDeps(cfg)      // For Server/Web mode
```

## Migration Steps

### Step 1: Backup Everything

```bash
# Backup database
mysqldump -u abot -p abot > backup-$(date +%Y%m%d).sql

# Backup configuration
cp config.yaml config.yaml.backup

# Backup data directory (if using JSONL sessions)
tar -czf data-backup-$(date +%Y%m%d).tar.gz data/
```

### Step 2: Update Binaries

#### Option A: Using Package Manager

```bash
# Remove old binary
sudo apt remove abot  # or: brew uninstall abot

# Install new binaries
sudo apt install abot-agent abot-server abot-web
# or: brew install abot-agent abot-server abot-web
```

#### Option B: Manual Installation

```bash
# Remove old binary
sudo rm /usr/local/bin/abot

# Download new binaries
wget https://github.com/yourusername/abot/releases/v2.0.0/abot-agent-linux-amd64.tar.gz
wget https://github.com/yourusername/abot/releases/v2.0.0/abot-server-linux-amd64.tar.gz
wget https://github.com/yourusername/abot/releases/v2.0.0/abot-web-linux-amd64.tar.gz

# Extract and install
tar -xzf abot-agent-linux-amd64.tar.gz
tar -xzf abot-server-linux-amd64.tar.gz
tar -xzf abot-web-linux-amd64.tar.gz

sudo mv abot-agent /usr/local/bin/
sudo mv abot-server /usr/local/bin/
sudo mv abot-web /usr/local/bin/

# Verify installation
abot-agent -version
abot-server -version
abot-web -version
```

### Step 3: Update Configuration

#### For CLI Mode (abot-agent)

**Old config.yaml:**
```yaml
app_name: abot
mysql_dsn: "user:pass@tcp(localhost:3306)/abot"
providers:
  - api_key: sk-xxx
    model: gpt-4o-mini
agents:
  - id: default-bot
    name: assistant
```

**New config.yaml (minimal for CLI):**
```yaml
app_name: abot
# mysql_dsn is optional for CLI mode
providers:
  - api_key: sk-xxx
    model: gpt-4o-mini
agents:
  - id: default-bot
    name: assistant
```

#### For Server Mode (abot-server)

**Old config.yaml:**
```yaml
app_name: abot
mysql_dsn: "user:pass@tcp(localhost:3306)/abot"
providers:
  - api_key: sk-xxx
    model: gpt-4o-mini
```

**New config.yaml (no changes needed):**
```yaml
app_name: abot
mysql_dsn: "user:pass@tcp(localhost:3306)/abot"
providers:
  - api_key: sk-xxx
    model: gpt-4o-mini
```

#### For Web Mode (abot-web)

**Old config.yaml:**
```yaml
app_name: abot
mysql_dsn: "user:pass@tcp(localhost:3306)/abot"
providers:
  - api_key: sk-xxx
    model: gpt-4o-mini
console:
  addr: ":3000"
  jwt_secret: "your-secret"
```

**New config.yaml (add static_dir):**
```yaml
app_name: abot
mysql_dsn: "user:pass@tcp(localhost:3306)/abot"
providers:
  - api_key: sk-xxx
    model: gpt-4o-mini
console:
  addr: ":3000"
  jwt_secret: "your-secret"
  static_dir: "web/out"  # NEW: Required for web mode
  allowed_origins:
    - "http://localhost:3000"
```

### Step 4: Update Systemd Services

#### Old Service File

```ini
# /etc/systemd/system/abot.service
[Unit]
Description=abot AI Agent
After=network.target mysql.service

[Service]
Type=simple
User=abot
ExecStart=/usr/local/bin/abot console -config /etc/abot/config.yaml
Restart=on-failure

[Install]
WantedBy=multi-user.target
```

#### New Service File

```ini
# /etc/systemd/system/abot-web.service
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
```

#### Apply Changes

```bash
# Stop old service
sudo systemctl stop abot

# Disable old service
sudo systemctl disable abot

# Install new service
sudo cp abot-web.service /etc/systemd/system/

# Reload systemd
sudo systemctl daemon-reload

# Enable and start new service
sudo systemctl enable abot-web
sudo systemctl start abot-web

# Check status
sudo systemctl status abot-web
```

### Step 5: Update Docker Deployments

#### Old docker-compose.yml

```yaml
version: '3.8'
services:
  abot:
    image: abot/abot:latest
    command: ["console"]
    environment:
      - MYSQL_DSN=...
      - OPENAI_API_KEY=...
```

#### New docker-compose.yml

```yaml
version: '3.8'
services:
  abot-web:
    image: abot/web:latest
    # No command needed - binary is specialized
    environment:
      - MYSQL_DSN=...
      - OPENAI_API_KEY=...
```

#### Apply Changes

```bash
# Stop old containers
docker-compose down

# Update docker-compose.yml
vim docker-compose.yml

# Start new containers
docker-compose up -d

# Check logs
docker-compose logs -f abot-web
```

### Step 6: Update Kubernetes Deployments

#### Old Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: abot
spec:
  template:
    spec:
      containers:
      - name: abot
        image: abot/abot:latest
        args: ["console"]
```

#### New Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: abot-web
spec:
  template:
    spec:
      containers:
      - name: abot-web
        image: abot/web:latest
        # No args needed
```

#### Apply Changes

```bash
# Update deployment
kubectl apply -f deployment.yaml

# Check rollout status
kubectl rollout status deployment/abot-web

# Check pods
kubectl get pods -l app=abot-web
```

### Step 7: Verify Migration

```bash
# Test CLI mode
abot-agent -config config.yaml
# Type a message and verify response

# Test Server mode (if using)
abot-server -config config.yaml &
curl http://localhost:8080/api/v1/agents

# Test Web mode
abot-web -config config.yaml &
curl http://localhost:3000/health
open http://localhost:3000
```

### Step 8: Update Scripts and Automation

Update any scripts that reference the old binary:

**Old:**
```bash
#!/bin/bash
/usr/local/bin/abot console -config /etc/abot/config.yaml
```

**New:**
```bash
#!/bin/bash
/usr/local/bin/abot-web -config /etc/abot/config.yaml
```

## Database Migration

### Schema Changes

Version 2.0 maintains backward compatibility with v1.x database schema. No manual schema changes are required.

Migrations run automatically on startup. To run manually:

```bash
abot-web -config config.yaml -migrate-only
```

### Data Migration

No data migration is required. All existing data (sessions, messages, agents) will work with v2.0.

## Rollback Plan

If you encounter issues and need to rollback:

### Step 1: Stop New Services

```bash
# Systemd
sudo systemctl stop abot-web

# Docker
docker-compose down

# Kubernetes
kubectl delete deployment abot-web
```

### Step 2: Restore Old Binary

```bash
# Download old version
wget https://github.com/yourusername/abot/releases/v1.x/abot-linux-amd64.tar.gz
tar -xzf abot-linux-amd64.tar.gz
sudo mv abot /usr/local/bin/
```

### Step 3: Restore Configuration

```bash
cp config.yaml.backup config.yaml
```

### Step 4: Restore Database (if needed)

```bash
mysql -u abot -p abot < backup-20260302.sql
```

### Step 5: Start Old Service

```bash
# Systemd
sudo systemctl start abot

# Docker
docker-compose up -d

# Kubernetes
kubectl apply -f old-deployment.yaml
```

## Common Issues and Solutions

### Issue 1: "mysql_dsn required" Error in CLI Mode

**Problem:**
```
Error: mysql_dsn is required for this mode
```

**Solution:**
CLI mode (abot-agent) doesn't require MySQL. Either:
- Remove `mysql_dsn` from config
- Use in-memory sessions (default)
- Use JSONL sessions:
  ```yaml
  session:
    type: jsonl
    dir: data/sessions
  ```

### Issue 2: "static_dir required" Error in Web Mode

**Problem:**
```
Error: console.static_dir is required for web mode
```

**Solution:**
Add `static_dir` to console config:
```yaml
console:
  addr: ":3000"
  jwt_secret: "your-secret"
  static_dir: "web/out"
```

### Issue 3: Binary Not Found

**Problem:**
```
bash: abot-web: command not found
```

**Solution:**
```bash
# Check if binary is installed
which abot-web

# If not, install it
sudo cp abot-web /usr/local/bin/
sudo chmod +x /usr/local/bin/abot-web
```

### Issue 4: Permission Denied

**Problem:**
```
Error: permission denied: /etc/abot/config.yaml
```

**Solution:**
```bash
# Fix file permissions
sudo chown abot:abot /etc/abot/config.yaml
sudo chmod 600 /etc/abot/config.yaml

# Or run as correct user
sudo -u abot abot-web -config /etc/abot/config.yaml
```

### Issue 5: Port Already in Use

**Problem:**
```
Error: bind: address already in use
```

**Solution:**
```bash
# Find process using the port
sudo lsof -i :3000

# Kill old process
sudo kill <PID>

# Or change port in config
console:
  addr: ":3001"
```

## Feature Comparison

| Feature | v1.x (Single Binary) | v2.0 (Multi-Binary) |
|---------|---------------------|---------------------|
| CLI Mode | `abot agent` | `abot-agent` |
| API Server | `abot server` | `abot-server` |
| Web Console | `abot console` | `abot-web` |
| Binary Size | ~50MB (all modes) | ~15MB each (specialized) |
| MySQL Required | Always | Only for server/web |
| Configuration | Single config | Mode-specific validation |
| Deployment | One binary | Three binaries |
| Code Duplication | High | Zero (pkg/bootstrap) |
| Testing | Complex | Easier (isolated) |
| Security | Larger attack surface | Minimal per binary |

## Benefits of Migration

### 1. Smaller Binary Sizes
- CLI binary: ~15MB (vs 50MB)
- No web assets in CLI binary
- Faster downloads and deployments

### 2. Better Security
- Minimal attack surface per binary
- CLI mode doesn't expose web endpoints
- Easier to audit and secure

### 3. Clearer Separation
- Each binary has a single purpose
- Easier to understand and maintain
- Better error messages

### 4. Zero Code Duplication
- Shared logic in `pkg/bootstrap`
- Single source of truth
- Easier to test and maintain

### 5. Flexible Deployment
- Deploy only what you need
- CLI doesn't need database
- Independent scaling

## FAQ

### Q: Can I run both v1.x and v2.0 simultaneously?

A: Yes, but they should use different databases to avoid conflicts. Use different `mysql_dsn` values in their configs.

### Q: Will my data be lost during migration?

A: No, the database schema is compatible. Just backup before migrating as a precaution.

### Q: Do I need to rebuild my frontend?

A: No, the API is backward compatible. Existing frontends will work with v2.0.

### Q: Can I use the old config file?

A: Mostly yes, but you may need to add `console.static_dir` for web mode.

### Q: How long does migration take?

A: Typically 10-30 minutes depending on your deployment method.

### Q: Is there a migration tool?

A: No tool is needed. The binaries are drop-in replacements with minor config changes.

### Q: What if I only use CLI mode?

A: You only need to install `abot-agent`. No database or other binaries required.

### Q: Can I migrate gradually?

A: Yes, you can migrate one mode at a time. For example, migrate CLI first, then server, then web.

### Q: Are there any performance differences?

A: v2.0 has slightly better startup time due to smaller binaries. Runtime performance is the same.

### Q: What about custom plugins?

A: Custom plugins should work without changes. The plugin interface is unchanged.

## Support

If you encounter issues during migration:

1. Check the [Troubleshooting](#common-issues-and-solutions) section
2. Review the [Deployment Guide](DEPLOYMENT.md)
3. Open an issue on GitHub: https://github.com/yourusername/abot/issues
4. Join our Discord: https://discord.gg/abot

## Timeline

- **v1.x**: Maintenance mode (security fixes only)
- **v2.0**: Current stable release
- **v1.x EOL**: 2026-12-31 (end of support)

We recommend migrating to v2.0 before the EOL date.

---

**Last Updated**: 2026-03-02
**Version**: 2.0
