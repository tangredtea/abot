# abot Examples

快速上手示例，展示如何使用 abot 核心 API。

## 📁 示例列表

### 1. minimal - 最小化示例
最简单的 Agent 实现，仅需 50 行代码。

```bash
cd examples/minimal
go run main.go
```

**特点**：
- 内存会话（无持久化）
- 无数据库依赖
- 适合快速测试

---

### 2. with-tools - 带工具的 Agent
展示如何启用内置工具（文件操作、网页搜索、Shell 执行）。

```bash
cd examples/with-tools
go run main.go
```

**特点**：
- JSONL 会话持久化
- 沙箱安全限制
- 完整工具集

---

### 3. multi-agent - 多 Agent 系统
展示多 Agent 协作和 LLM 故障转移。

```bash
cd examples/multi-agent
go run main.go
```

**特点**：
- 多个 Agent（不同角色）
- 多个 LLM 提供商（自动故障转移）
- 适合复杂场景

---

## 🚀 使用说明

1. **替换 API Key**
   ```go
   APIKey: "sk-your-api-key-here", // 替换为你的 API Key
   ```

2. **运行示例**
   ```bash
   cd examples/minimal
   go run main.go
   ```

3. **嵌入到你的项目**
   ```bash
   cp examples/minimal/main.go your-project/
   # 修改配置后运行
   ```

---

## 📚 进阶使用

完整功能请使用官方二进制：

```bash
# CLI 模式（推荐新手）
./abot-agent init          # 交互式生成配置
./abot-agent -config config.yaml

# Web 模式（团队协作）
./abot-web -config config.yaml

# API 模式（集成到应用）
./abot-server -config config.yaml
```

---

## 🔗 相关文档

- [项目架构](../docs/PROJECT_ARCHITECTURE.md)
- [部署指南](../docs/DEPLOYMENT.md)
- [配置说明](../config.example.yaml)
