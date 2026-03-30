# abot-server

仅 HTTP API（无静态前端）。需 MySQL + 有效配置。

## 运行

```bash
cp cmd/abot-server/config.server.example.yaml config.yaml
# 编辑 mysql_dsn、providers、console.jwt_secret、console.allowed_origins
go run ./cmd/abot-server -config config.yaml
```

`console.addr` 监听地址须与 Docker 端口映射一致（例如 compose 中 `8080:8080` 则配置 `:8080`）。

## API（均在 `/api/` 下）

含认证、agents、chat、sessions 等；具体路由以 `pkg/api/console` 注册为准。

- `GET /health` → `200`，正文 `OK`

## 配置要点

```yaml
mysql_dsn: "user:pass@tcp(host:3306)/abot?charset=utf8mb4&parseTime=True"
providers:
  - name: primary
    api_key: "sk-..."
    model: "gpt-4o-mini"
console:
  addr: ":3000"
  jwt_secret: "32+ random bytes"
  allowed_origins: ["https://your-frontend"]
```

启动时自动执行 GORM 迁移。更多部署见 [docs/DEPLOYMENT.md](../../docs/DEPLOYMENT.md)。
