# NanoClaw 值得借鉴的功能（非隔离场景）

## 🎯 你的需求
- ✅ 一个 Agent 跨多个群组
- ✅ 共享人设和记忆
- ✅ 统一的上下文
- ❌ 不需要群组隔离

## 📋 NanoClaw 值得借鉴的 TOP 5

### 1. 定时任务系统 ⭐⭐⭐

**NanoClaw 实现**:
```typescript
// 定时任务表
CREATE TABLE scheduled_tasks (
  id TEXT PRIMARY KEY,
  chat_jid TEXT NOT NULL,
  prompt TEXT NOT NULL,
  schedule_type TEXT,      // 'cron', 'interval', 'once'
  schedule_value TEXT,     // '0 9 * * *' 或 '3600'
  next_run TEXT,
  status TEXT              // 'active', 'paused'
);
```

**实际用例**:
```
用户: @abot 每天早上9点给我发送今日待办
Agent: 已创建定时任务

用户: @abot 每周一早上8点总结上周进展
Agent: 已创建定时任务
```

**abot 当前状态**: ✅ 已有 (pkg/scheduler/cron.go)
**改进建议**: 
- 添加任务管理界面
- 支持更灵活的调度表达式


### 2. 轻量级 SQLite 存储 ⭐⭐

**NanoClaw 优势**:
- 使用 SQLite (better-sqlite3)
- 单文件数据库
- 零配置
- 适合个人/小团队

**abot 当前**: MySQL (企业级)

**借鉴点**:
- 添加 SQLite 作为轻量级选项
- CLI 模式使用 SQLite
- Web 模式使用 MySQL

```go
// config.yaml
storage:
  type: sqlite  # or mysql
  sqlite:
    path: ./data/abot.db
  mysql:
    dsn: "user:pass@tcp(localhost:3306)/abot"
```


### 3. 任务执行日志 ⭐⭐

**NanoClaw 实现**:
```typescript
// task_run_logs 表
CREATE TABLE task_run_logs (
  id INTEGER PRIMARY KEY,
  task_id TEXT,
  run_at TEXT,
  duration_ms INTEGER,
  status TEXT,        // 'success', 'error'
  result TEXT,
  error TEXT
);
```

**价值**:
- 追踪任务执行历史
- 性能监控 (duration_ms)
- 错误诊断
- 审计日志

**abot 改进**:
- 在 `pkg/scheduler/cron.go` 添加执行日志
- 记录每次任务运行的结果和耗时


### 4. 消息队列和并发控制 ⭐⭐⭐

**NanoClaw 实现**:
```typescript
// group-queue.ts
class GroupQueue {
  - 限制最大并发容器数 (MAX_CONCURRENT_CONTAINERS)
  - 按群组排队处理消息
  - 任务优先级: tasks > messages
  - 指数退避重试机制
}
```

**核心特性**:
- 并发限制: 防止资源耗尽
- 队列管理: 公平调度多个群组
- 优雅降级: 失败重试 + 指数退避

**abot 借鉴**:
```go
// pkg/agent/queue.go (新增)
type AgentQueue struct {
    maxConcurrent int
    activeCount   int
    pending       []Task
    retryBackoff  time.Duration
}
```


### 5. 跨渠道统一消息格式 ⭐

**NanoClaw 实现**:
```typescript
// router.ts
function formatMessages(messages: NewMessage[]): string {
  return `<messages>
    <message sender="Alice" time="2024-01-01">Hello</message>
    <message sender="Bob" time="2024-01-02">Hi</message>
  </messages>`;
}
```

**价值**:
- 统一的 XML 格式
- 跨渠道一致性 (WhatsApp, Discord, Telegram)
- 易于解析和处理

**abot 当前**: 已有类似实现 (pkg/channels/)


### 6. 发送者白名单 ⭐⭐⭐

**NanoClaw 实现**:
```json
{
  "default": { "allow": "*", "mode": "trigger" },
  "chats": {
    "group1@g.us": {
      "allow": ["user1", "user2"],
      "mode": "drop"
    }
  }
}
```

**核心价值**:
- 防止群组被滥用 (只允许特定用户触发)
- 两种模式: `trigger` (保留上下文) / `drop` (直接丢弃)
- 按群组粒度配置

**abot 当前**: ❌ 缺失


### 7. 崩溃恢复机制 ⭐⭐

**NanoClaw 实现**:
```typescript
function recoverPendingMessages(): void {
  for (const [chatJid, group] of Object.entries(registeredGroups)) {
    const pending = getMessagesSince(chatJid, sinceTimestamp);
    if (pending.length > 0) {
      queue.enqueueMessageCheck(chatJid);
    }
  }
}
```

**价值**: 启动时自动恢复未处理的消息，防止崩溃丢失

**abot 当前**: ❌ 缺失


### 8. 消息游标回滚 ⭐⭐⭐

**NanoClaw 实现**:
```typescript
const previousCursor = lastAgentTimestamp[chatJid];
lastAgentTimestamp[chatJid] = newTimestamp;

if (error && !outputSentToUser) {
  lastAgentTimestamp[chatJid] = previousCursor; // 回滚重试
}
```

**价值**: 错误时回滚游标支持重试，但已发送输出则不回滚避免重复

**abot 当前**: ⚠️ 缺少回滚机制


### 9. 内部标签过滤 ⭐

**NanoClaw 实现**:
```typescript
// Agent: "思考中<internal>调用工具</internal>答案是42"
// 用户看到: "思考中答案是42"
text.replace(/<internal>[\s\S]*?<\/internal>/g, '')
```

**价值**: Agent 可以有内部思考，用户只看最终输出

**abot 当前**: ❌ 缺失


### 10. 空闲超时管理 ⭐⭐

**NanoClaw 实现**:
```typescript
const IDLE_TIMEOUT = 30 * 60 * 1000;
setTimeout(() => queue.closeStdin(chatJid), IDLE_TIMEOUT);
```

**价值**: 无活动时自动释放资源

**abot 当前**: ⚠️ 基础实现


## 🎬 实施建议

### 优先级 1: 任务执行日志
```go
// pkg/scheduler/execution_log.go
type TaskExecutionLog struct {
    TaskID     string
    RunAt      time.Time
    DurationMs int64
    Status     string
    Result     string
    Error      string
}
```

### 优先级 2: SQLite 支持
```go
// pkg/storage/sqlite/store.go
type SQLiteStore struct {
    db *sql.DB
}
```

### 优先级 3: 消息队列优化
```go
// pkg/agent/queue.go
- 添加并发限制
- 实现指数退避重试
- 任务优先级调度
```

## 📊 对比总结

| 特性 | NanoClaw | abot | 优先级 |
|------|----------|------|--------|
| 定时任务 | ✅ 完善 | ✅ 已有 | P1 添加日志 |
| SQLite | ✅ 默认 | ❌ 缺失 | P1 添加支持 |
| 任务日志 | ✅ 完善 | ❌ 缺失 | P1 实现 |
| 消息队列 | ✅ 完善 | ⚠️ 基础 | P2 优化 |
| 并发控制 | ✅ 完善 | ⚠️ 基础 | P2 优化 |
| 发送者白名单 | ✅ 完善 | ❌ 缺失 | P2 实现 |
| 崩溃恢复 | ✅ 完善 | ❌ 缺失 | P2 实现 |
| 游标回滚 | ✅ 完善 | ❌ 缺失 | P3 实现 |
| 内部标签 | ✅ 完善 | ❌ 缺失 | P3 实现 |
| 空闲超时 | ✅ 完善 | ⚠️ 基础 | P3 优化 |

## ✅ 结论

**TOP 10 值得借鉴的功能**:

**P1 (必须实现)**:
1. 任务执行日志 - 可观测性
2. SQLite 支持 - 降低门槛
3. 任务日志表 - 审计追踪

**P2 (重要)**:
4. 消息队列优化 - 并发性能
5. 发送者白名单 - 安全控制
6. 崩溃恢复 - 可靠性

**P3 (可选)**:
7. 游标回滚 - 错误重试
8. 内部标签过滤 - 用户体验
9. 空闲超时 - 资源管理
10. 指数退避重试 - 稳定性

**不需要借鉴**:
- ❌ Docker 容器隔离 (太重)
- ❌ 群组级别隔离 (不符合需求)
- ❌ 独立文件系统 (不需要)

