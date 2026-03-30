# Deployment

面向本仓库**当前实现**的部署说明。架构见 [PROJECT_ARCHITECTURE.md](./PROJECT_ARCHITECTURE.md)。

## 前置条件

- **Go**：与 `go.mod` 一致（例如 1.24.x）
- **MySQL 8**：`abot-server` / `abot-web` / 统一 `cmd/abot` 控制台模式需要
- **LLM API Key**：在配置或环境中提供

## 最快路径：仓库内 Docker Compose

根目录 [`docker-compose.yml`](../docker-compose.yml) 可构建并启动 MySQL、`abot-agent`、`abot-server`、`abot-web`（端口以 compose 为准，如 server **8080**、web **3000**）。

```bash
export OPENAI_API_KEY=sk-...
export JWT_SECRET=$(openssl rand -hex 32)
docker compose up -d --build
```

```bash
curl -s http://localhost:8080/health   # abot-server
curl -s http://localhost:3000/health   # abot-web
```

本仓库**不提供**官方 Helm Chart。

## 单容器（自建镜像）

需已有 MySQL、有效 `config.yaml`，且 `console.static_dir` 指向构建好的前端（如 `web/out`）。

```bash
docker run -d --name abot-web -p 3000:3000 \
  -v "$(pwd)/config.yaml:/app/config.yaml:ro" \
  -e MYSQL_DSN="user:pass@tcp(host.docker.internal:3306)/abot?charset=utf8mb4&parseTime=True" \
  -e OPENAI_API_KEY="sk-..." \
  your-registry/abot-web:latest \
  -config /app/config.yaml
```

## 裸机 / systemd

1. `make web` / `go build -o bin/abot-web ./cmd/abot-web`
2. 配置 `mysql_dsn`、`providers`、`console`（`jwt_secret`、`static_dir` 等）
3. systemd 示例：`ExecStart=/usr/local/bin/abot-web -config /etc/abot/config.yaml`

## 数据库

```sql
CREATE DATABASE abot CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE USER 'abot'@'%' IDENTIFIED BY 'strong-password';
GRANT ALL ON abot.* TO 'abot'@'%';
FLUSH PRIVILEGES;
```

**迁移**：连上 MySQL 后启动时 GORM AutoMigrate，无单独的 `-migrate-only` flag。

```bash
mysqldump -u abot -p abot | gzip > abot-$(date +%Y%m%d).sql.gz
```

## 反向代理与 TLS

Nginx/Caddy：`proxy_pass` 到监听地址；`/api/` WebSocket 需透传 `Upgrade`。TLS 用 certbot 或云 LB。

## 可观测性（现状）

- **`GET /health`** → 200，正文 **`OK`**（`abot-web`、`abot-server`）。
- **无**内置 `GET /metrics`；可在网关统计或自行扩展。

## 扩容与多实例

水平扩展：多副本 + 共享 MySQL；注意 WebSocket 粘性。

### 分布式心跳（多实例）

问题：每实例各跑心跳会重复执行。实现：MySQL 分布式锁 + Leader 选举（`pkg/scheduler/distributed_lock.go`、`distributed_heartbeat.go`）。仅 Leader 执行心跳；锁过期后由其他实例接管。详见代码与配置中的 `DistributedHeartbeatConfig`。

## 从 v1（单 `abot` 子命令）迁移

| 旧 | 新 |
|----|-----|
| `abot agent` | `abot-agent` |
| `abot console` | `abot-web` |
| （若曾有 server 子命令） | `abot-server` |

要点：`abot-agent` 可不配 `mysql_dsn`；`abot-server`/`abot-web` 需要；`abot-web` 需要 `console.static_dir`。`cmd/abot` 统一入口可与三二进制**并存**。共享逻辑在 **`pkg/bootstrap`**。迁移前 **备份** 数据库与配置。systemd 将 `abot console` 改为 `abot-web` 等。

## 排错

| 现象 | 排查 |
|------|------|
| 起不来 | 日志、`mysql_dsn`、`static_dir`、MySQL |
| 端口 | `lsof -i :3000` 或改 `console.addr` |
| WebSocket | 反代 Upgrade |

## 清单

强 JWT、HTTPS、密钥走环境变量、定期备份。
