# NanoClaw vs abot 深度对比分析（基于最新代码）

## 📊 核心数据对比

| 维度 | NanoClaw | abot (最新) |
|------|----------|-------------|
| 代码行数 | ~6,700 行 | ~24,000 行 |
| 文件数 | 26 个 TS 文件 | 204 个 Go 文件 |
| 架构 | 单进程 Node.js | 多二进制 Go |
| 容器隔离 | ✅ Docker (已实现) | ⚠️ 应用层 (计划中) |
| 群组隔离 | ✅ 每群组独立容器 | ❌ 租户级隔离 |
| SubAgent | ❌ 无 | ✅ 有 (SubagentManager) |
| 并发模型 | 事件循环 | Go goroutines |

## 🎯 abot 已有但可改进的功能

### 1. SubAgent 系统 ✅ (已有)

**abot 当前实现**:
```go
// pkg/agent/subagent.go
type SubagentManager struct {
    registry       *AgentRegistry
    tasks          map[string]*SubagentTask
    // 支持后台任务
}
```

**NanoClaw 无此功能**，但 abot 可以改进：
- 添加群组级隔离
- 每个 SubAgent 独立容器

### 2. 多渠道支持 ✅ (已有)

**abot**: Discord, Telegram, WeCom, Feishu
**NanoClaw**: WhatsApp, Telegram, Discord, Slack, Gmail

**abot 优势**: 企业级渠道（WeCom, Feishu）

## 🔥 abot 急需借鉴的功能

### 1. Docker 容器隔离 ⭐⭐⭐ (最高优先级)

**当前问题**: abot 只有应用层防护，可被绕过

**NanoClaw 方案**:
```typescript
// 每个群组独立容器
docker run --rm \
  --name nanoclaw-group-123 \
  --user node \
  -v /groups/123:/workspace/group \
  nanoclaw/agent
```

**abot 实施建议**:
```go
// pkg/agent/container_runner.go (新建)
type ContainerRunner struct {
    runtime string // "docker" or "podman"
}

func (r *ContainerRunner) RunInContainer(
    ctx context.Context,
    groupID string,
    workspace string,
    input AgentInput,
) (*AgentOutput, error) {
    args := []string{
        "run", "--rm",
        "--name", fmt.Sprintf("abot-%s", groupID),
        "--user", "abot",
        "-v", fmt.Sprintf("%s:/workspace:rw", workspace),
        "abot/agent-runner:latest",
    }
    
    cmd := exec.CommandContext(ctx, r.runtime, args...)
    // IPC 通信...
}
```


### 2. 群组级隔离 ⭐⭐⭐

**当前问题**: abot 只有租户级隔离，同租户下的群组共享资源

**NanoClaw 方案**:
```
groups/
├── whatsapp-group-123/
│   ├── CLAUDE.md (独立记忆)
│   └── workspace/ (独立文件系统)
└── telegram-chat-456/
    ├── CLAUDE.md
    └── workspace/
```

**abot 实施建议**:
```go
// pkg/session/group_session.go (新建)
type GroupSession struct {
    TenantID     string
    GroupID      string  // channel:chatID
    MemoryFile   string  // groups/{tenant}/{group}/MEMORY.md
    WorkspaceDir string  // groups/{tenant}/{group}/workspace/
}

func GetGroupWorkspace(tenantID, channel, chatID string) string {
    groupID := fmt.Sprintf("%s-%s", channel, chatID)
    return filepath.Join("groups", tenantID, groupID, "workspace")
}
```

### 3. 挂载白名单 ⭐⭐

**NanoClaw 实现**:
```typescript
// ~/.config/nanoclaw/mount-allowlist.json (容器外)
{
  "allowedRoots": [
    {"path": "~/projects", "allowReadWrite": true},
    {"path": "/usr/local/bin", "allowReadWrite": false}
  ],
  "blockedPatterns": [".ssh", ".aws", "credentials"]
}
```

**abot 实施建议**:
```go
// pkg/tools/mount_allowlist.go (新建)
var DefaultBlockedPatterns = []string{
    ".ssh", ".gnupg", ".aws", ".kube",
    "credentials", "secrets", ".env",
}

func ValidateMountPath(path string) error {
    for _, pattern := range DefaultBlockedPatterns {
        if strings.Contains(path, pattern) {
            return fmt.Errorf("blocked pattern: %s", pattern)
        }
    }
    return nil
}
```


## 🚀 实施优先级和路线图

### P0 - 立即实施（Week 1-2）

**1. Docker 容器隔离**
```bash
# 创建文件
pkg/agent/container_runner.go
cmd/abot-agent-runner/main.go
Dockerfile.agent-runner

# 配置
config.yaml:
  container:
    enabled: true
    runtime: docker
    max_concurrent: 3
```

**2. 群组级隔离**
```bash
# 创建文件
pkg/session/group_session.go

# 目录结构
groups/{tenant}/{channel}-{chatID}/
  ├── MEMORY.md
  └── workspace/
```

### P1 - 短期实施（Week 3-4）

**3. 挂载白名单**
```bash
# 配置文件
~/.config/abot/mount-allowlist.json

# 实现
pkg/tools/mount_allowlist.go
```

**4. 容器队列管理**
```bash
pkg/agent/group_queue.go
```


## 📝 核心结论

### NanoClaw 的 3 大优势

1. **完整的容器隔离** - Docker 提供真正的 OS 级隔离
2. **群组级隔离** - 每个群组独立容器和文件系统
3. **极简架构** - 6,700 行代码，易于理解和定制

### abot 的 3 大优势

1. **企业级功能** - Web 控制台、API 服务器、多租户
2. **SubAgent 系统** - 后台任务管理（NanoClaw 无）
3. **Go 并发** - 高性能并发处理

### 最关键的改进

**安全性提升 10x**: 从应用层防护升级到容器隔离

```
当前: 命令拦截 (可绕过)
  ↓
改进: Docker 容器 (OS 级隔离)
```

### 实施建议

**最小可行方案** (2 周):
1. 实现 `ContainerRunner`
2. 构建 `abot-agent-runner` 镜像
3. 添加群组级目录结构

**完整方案** (4 周):
+ 挂载白名单验证
+ 容器队列管理
+ 配置化容器选项

---

**报告生成时间**: 2026-03-06
**基于代码**: abot (24,000 行), NanoClaw (6,700 行)

