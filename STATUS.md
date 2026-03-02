# ABot 项目状态

## 最新更新 (2026-03-02)

### ✅ Workspace 文档集成

**问题**: Agent 人设配置存储在 `agent_definitions.config` JSON 字段中，但系统已有完整的 Workspace 文档系统（IDENTITY, SOUL, RULES, AGENT）。

**解决方案**:
1. **后端重构**:
   - `AgentManager` 现在使用 `ContextBuilder.InstructionProvider()` 而不是从 config 构建系统提示词
   - `ContextBuilder` 自动从 `workspace_docs` 和 `user_workspace_docs` 表加载 IDENTITY, SOUL, RULES, AGENT 文档
   - 新增 3 个 API 端点:
     - `GET /api/v1/workspace/docs` - 列出所有文档
     - `GET /api/v1/workspace/docs/:doc_type` - 获取单个文档
     - `PUT /api/v1/workspace/docs/:doc_type` - 更新文档

2. **前端更新**:
   - 创建新的 `PersonalityTab` 组件，直接编辑 Workspace 文档
   - 3 个文档类型: IDENTITY (身份设定), SOUL (灵魂设定), AGENT (Agent 配置)
   - 修改后立即保存到数据库，Agent 下次对话时自动加载

3. **数据流**:
   ```
   用户编辑 Workspace 文档 → 保存到 user_workspace_docs 表
   → Agent 对话时 ContextBuilder 自动加载 → 注入到系统提示词
   ```

**优势**:
- 统一使用 Workspace 文档系统，与核心架构一致
- 支持用户级和租户级文档，灵活性更高
- 文档版本控制和历史记录
- 与 `update_doc` 工具集成，Agent 可以自我修改人设

---

## 系统架构

### 核心组件
- **Agent 管理**: 动态创建、更新、删除 Agent，无需重启
- **Workspace 文档**: IDENTITY, SOUL, RULES, AGENT 文档系统
- **多租户**: 租户隔离，每个租户独立的 Agent 和配置
- **通道系统**: Web, 企业微信, Telegram, Discord, 飞书

### 数据库表
- `tenants` - 租户
- `accounts` - 账号
- `account_tenants` - 账号-租户关联
- `chat_sessions` - 聊天会话
- `agent_definitions` - Agent 定义（config 字段存储模型、通道等配置）
- `workspace_docs` - 租户级 Workspace 文档
- `user_workspace_docs` - 用户级 Workspace 文档
- `skill_records`, `tenant_skills` - Skills 管理
- `cron_jobs`, `memory_events` - 调度和记忆

### API 端点
- `/api/v1/auth/*` - 认证（注册、登录、/me）
- `/api/v1/sessions/*` - 会话管理
- `/api/v1/agents/*` - Agent CRUD
- `/api/v1/workspace/docs/*` - Workspace 文档管理
- `/api/v1/chat/ws` - WebSocket 聊天

---

## 开发环境

### 后端
- Go 1.21+
- MySQL 8.0
- 端口: 3001

### 前端
- Next.js 15
- React 19
- TailwindCSS
- 端口: 3000

### 启动命令
```bash
# 后端
./abot console --config config.test.yaml

# 前端
cd web && npm run dev
```

### 测试账号
- Email: `test@example.com`
- Password: `Test123456`

---

## 待办事项

- [ ] Agent 删除时从 Registry 移除（目前需要重启）
- [ ] 前端 Workspace 文档编辑器优化（语法高亮、预览）
- [ ] 多模型支持（OpenAI, Anthropic）
- [ ] Skills 安装和管理界面
- [ ] 更多通道配置（企业微信、Telegram 等）
