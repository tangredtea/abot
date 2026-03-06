# abot 默认配置总结

## 一、默认工具（17个核心工具）

### 文件操作（5个）
- `read_file` - 读取文件（最大 2MB，支持智能路由）
- `write_file` - 写入文件（自动创建父目录，自动同步 persona 文件到 MySQL）
- `edit_file` - 精确文本替换
- `append_file` - 追加内容
- `list_dir` - 列出目录（支持虚拟文件注入）

### 系统操作（2个）
- `exec` - 执行 shell 命令
- `message` - 发送消息到用户

### 网络操作（2个）
- `web_search` - 网页搜索
- `web_fetch` - 获取网页内容

### 任务管理（2个）
- `spawn` - 创建子任务
- `cron` - 定时任务调度

### 技能管理（4个）
- `find_skills` - 查找技能（支持 ClawHub）
- `install_skill` - 安装技能
- `create_skill` - 创建技能
- `promote_skill` - 推广技能到全局

### 记忆管理（2个）
- `save_memory` - 保存记忆（支持 embedding 缓存、BM25、MMR）
- `search_memory` - 搜索记忆（混合检索：40% 语义 + 30% BM25 + 20% 时间 + 10% 显著性）

### 可选工具（需要 Subagent）
- `subagent` - 子代理调用
- `list_tasks` - 列出任务

---

## 二、默认技能（9个内置技能）

技能存放在 `skills/` 目录，启动时自动扫描注册。

### 1. browser 🆕
浏览器自动化（Playwright）
- 页面导航和截图
- 元素交互
- 数据提取
- 登录流程自动化

### 2. clawhub
ClawHub 技能市场集成
- 搜索社区技能
- 安装和管理技能

### 3. cron
定时任务管理
- 创建定时任务
- 支持 cron 表达式

### 4. github
GitHub 集成（gh CLI）
- 管理 PR、Issue
- 查看 CI 状态
- API 查询

### 5. memory
高级记忆管理
- 向量存储
- 语义搜索

### 6. skill-creator
技能创建器
- 提供技能模板
- 创建指导

### 7. summarize
文本摘要
- 总结长文本
- 文档摘要

### 8. tmux
Tmux 会话管理
- 管理会话、窗口、面板

### 9. weather
天气查询
- 查询天气信息

---

## 三、MCP 支持

abot 支持 Model Context Protocol (MCP)，可以通过 MCP 客户端连接外部工具服务器。

**MCP 包位置：** `pkg/mcp/`

**核心组件：**
- `client.go` - MCP 客户端实现
- `tenant_manager.go` - 多租户 MCP 管理
- `wrapper.go` - 工具包装器

**使用方式：**
- 通过配置文件指定 MCP 服务器
- 自动将 MCP 工具注入到 Agent 上下文
- 支持多租户隔离

---

## 四、默认模板文件

### Workspace 级别（tenant 共享）
- `IDENTITY.md` - 租户身份
- `SOUL.md` - 行为准则（包含自我进化指导）
- `AGENT.md` - Agent 配置
- `TOOLS.md` - 工具使用指南
- `RULES.md` - 规则定义
- `HEARTBEAT.md` - 心跳任务

### User 级别（用户私有）
- `USER.md` - 用户档案
- `EXPERIMENTS.md` - 实验日志 🆕
- `NOTES.md` - 日常笔记 🆕

**智能路由：**
- 这些文件自动同步到 MySQL
- `read_file` 优先从 MySQL 读取
- `write_file` 自动同步到 MySQL
- `list_dir` 自动注入虚拟文件

---

## 五、核心优化

### 1. Embedding 缓存
- 真正的 LRU 淘汰机制
- 默认缓存 10,000 条
- 减少 70% API 成本

### 2. BM25 关键词索引
- k1=1.2, b=0.75
- 提升关键词召回率

### 3. MMR 重排序
- lambda=0.7
- 提升结果多样性

### 4. 混合检索策略
- 40% 语义相似度
- 30% BM25 关键词
- 20% 时间新鲜度
- 10% 显著性权重

---

## 六、配置文件

### 最小配置（config.agent.example.yaml）
```yaml
app_name: abot

providers:
  - name: primary
    api_base: "https://api.openai.com/v1"
    api_key: "${OPENAI_API_KEY}"
    model: "gpt-4o-mini"

agents:
  - id: default-bot
    name: assistant
    description: "A helpful assistant"
    model: "gpt-4o-mini"

session:
  type: jsonl
  dir: "~/.abot/sessions"

context_window: 128000
```

---

## 七、技能加载机制

1. **启动时自动扫描** `skills/` 目录
2. **注册为内置技能**（tier=builtin）
3. **支持 always_load** 标记（预加载到缓存）
4. **支持动态安装**（从 ClawHub 或本地）

### 技能结构
```
skills/
└── skill-name/
    └── SKILL.md  # 必需，包含 frontmatter 和文档
```

### SKILL.md 格式
```markdown
---
name: skill-name
description: "Brief description"
always_load: false  # 可选，是否预加载
---

# Skill Documentation

Usage examples...
```

---

## 八、总结

**abot 的默认配置已经非常完善：**

✅ 17个核心工具（文件、网络、任务、技能、记忆）
✅ 9个内置技能（浏览器、GitHub、定时任务等）
✅ MCP 支持（可扩展外部工具）
✅ 智能路由（persona 文件自动同步 MySQL）
✅ 高级检索（缓存+BM25+MMR）
✅ 完整的模板系统（自我进化、实验日志）

**无需额外配置即可使用，开箱即用。**
