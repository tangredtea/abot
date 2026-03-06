# abot 存储架构分析

## 📊 当前存储分层

### 1. 中心化存储（有状态，共享）

**MySQL 数据库**:
- ✅ 租户信息 (tenants)
- ✅ 用户账号 (accounts)
- ✅ Agent 配置 (agents)
- ✅ Skills 注册表 (skill_registry)
- ✅ 租户-Skills 关联 (tenant_skills)
- ✅ 工作空间文档 (workspace_docs)

**对象存储 (S3/本地)**:
- ✅ Skills 文件 (skills/{skillID}/)
- ✅ 大文件存储

**向量数据库 (Qdrant)**:
- ✅ 记忆向量 (embeddings)
- ✅ 语义搜索

### 2. 本地文件存储（有状态，隔离）

**JSONL 会话文件**:
```
sessions/{tenantID}/{userID}/{sessionID}.jsonl
```
- 对话历史
- 事件记录
- 会话状态

**工作空间文件**:
```
workspace/{tenantID}/{userID}/
```
- 用户上传的文件
- Agent 生成的文件
- 临时文件


## 🎯 针对 abot 的群组隔离方案

### 问题分析

**当前路径结构**:
```
workspace/{tenantID}/{userID}/
sessions/{tenantID}/{userID}/{sessionID}.jsonl
```

**问题**: 同一用户在不同群组/频道共享工作空间

**场景举例**:
```
用户张三在：
- Discord #dev 频道
- Telegram HR 群
- Slack #sales 频道

当前都共享: workspace/company-a/zhang/
```

### 推荐方案：轻量级群组隔离

**方案 1: 扩展路径结构（推荐）⭐⭐⭐**

```
workspace/{tenantID}/{userID}/{groupID}/
sessions/{tenantID}/{userID}/{groupID}/{sessionID}.jsonl
```

**groupID 生成规则**:
```go
groupID := fmt.Sprintf("%s-%s", channel, chatID)
// 例如: "discord-123456", "telegram-789012"
```

**优势**:
- ✅ 实现简单（修改路径生成逻辑）
- ✅ 零性能开销
- ✅ 向后兼容（默认 groupID = "default"）
- ✅ 不需要修改数据库


### 具体实施代码

**步骤 1: 修改路径生成函数**

```go
// pkg/tools/security.go
func GroupWorkspaceDir(baseDir, tenantID, userID, channel, chatID string) string {
    groupID := "default"
    if channel != "" && chatID != "" {
        groupID = fmt.Sprintf("%s-%s", channel, chatID)
    }
    
    return filepath.Join(baseDir, tenantID, userID, groupID)
}
```

**步骤 2: 修改 session 路径**

```go
// pkg/session/jsonl_store.go
func sessionFilePath(dir, tenantID, userID, groupID, sessionID string) string {
    if groupID == "" {
        groupID = "default"
    }
    return filepath.Join(dir, tenantID, userID, groupID, sessionID+".jsonl")
}
```


**步骤 3: 修改消息处理**

```go
// pkg/agent/agent_loop.go
// 在处理消息时传入 groupID
groupID := fmt.Sprintf("%s-%s", msg.Channel, msg.ChatID)
workspace := tools.GroupWorkspaceDir(baseDir, msg.TenantID, msg.UserID, msg.Channel, msg.ChatID)
```

### 效果对比

**改进前**:
```
workspace/company-a/zhang/
  ├── dev_code.zip      (Discord 上传)
  ├── hr_salary.xlsx    (Telegram 上传)
  └── sales_data.csv    (Slack 上传)

Discord: @abot 列出文件
Agent: 看到所有文件 ❌
```

**改进后**:
```
workspace/company-a/zhang/
  ├── discord-123456/
  │   └── dev_code.zip
  ├── telegram-789012/
  │   └── hr_salary.xlsx
  └── slack-456789/
      └── sales_data.csv

Discord: @abot 列出文件
Agent: 只看到 dev_code.zip ✅
```

### 实施成本

- **代码修改**: 3-5 个文件
- **工作量**: 1-2 天
- **测试**: 1 天
- **风险**: 低（向后兼容）

