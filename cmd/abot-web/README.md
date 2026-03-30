# abot-web

Web 控制台：静态资源 + 与 `abot-server` 同源的 `/api/`。需 MySQL。

## 运行

```bash
cd web && npm install && npm run build && cd ..
cp config.web.example.yaml config.yaml
# 填写 mysql_dsn、providers、console（jwt_secret、static_dir: web/out、allowed_origins）
go run ./cmd/abot-web -config config.yaml
```

默认监听 `console.addr`（常为 `:3000`）。`GET /health` → `200` `OK`。

## Docker

见根目录 `docker-compose.yml` 中 `abot-web` 服务。镜像构建：`docker build -f cmd/abot-web/Dockerfile`。

## 与 abot-server

| | abot-server | abot-web |
|---|-------------|----------|
| 静态 UI | 否 | 是 |

部署与迁移：[docs/DEPLOYMENT.md](../../docs/DEPLOYMENT.md)。
