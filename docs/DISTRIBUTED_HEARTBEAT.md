# 分布式心跳解决方案

**问题**: 多实例部署时，每个实例都会独立运行心跳定时器，导致重复执行。

**解决方案**: 使用分布式锁实现 Leader Election，确保同一时刻只有一个实例执行心跳。

---

## 🎯 核心设计

### 1. 分布式锁机制

```go
// pkg/scheduler/distributed_lock.go

// 使用 MySQL 表实现分布式锁
CREATE TABLE heartbeat_locks (
    lock_name VARCHAR(64) PRIMARY KEY,
    instance_id VARCHAR(64) NOT NULL,
    acquired_at TIMESTAMP NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    INDEX idx_expires (expires_at)
);
```

### 2. Leader Election

```
实例 A: 尝试获取锁 → 成功 → 成为 Leader → 执行心跳
实例 B: 尝试获取锁 → 失败 → 等待 → 30s 后重试
实例 C: 尝试获取锁 → 失败 → 等待 → 30s 后重试

如果实例 A 崩溃:
- 锁在 5 分钟后自动过期
- 实例 B 或 C 获取锁 → 成为新 Leader
```

---

## 📋 使用方式

### 配置示例

```go
// 创建分布式心跳服务
cfg := scheduler.DistributedHeartbeatConfig{
    HeartbeatConfig: scheduler.HeartbeatConfig{
        Bus:            msgBus,
        WorkspaceStore: workspaceStore,
        Tenants:        tenantStore,
        Interval:       30 * time.Minute,
        Channel:        "telegram",
    },
    DB:         db,
    InstanceID: generateInstanceID(), // 唯一实例 ID
}

heartbeat := scheduler.NewDistributedHeartbeat(cfg)
heartbeat.Start(ctx)
```

### 生成实例 ID

```go
import (
    "fmt"
    "os"
)

func generateInstanceID() string {
    hostname, _ := os.Hostname()
    pid := os.Getpid()
    return fmt.Sprintf("%s-%d", hostname, pid)
}
```

---

## 🔧 工作原理

### 1. 启动流程

```
实例启动
  ↓
每 30 秒尝试获取锁
  ↓
获取成功？
  ├─ 是 → 成为 Leader → 执行心跳循环
  └─ 否 → 继续等待
```

### 2. Leader 运行

```
Leader 实例
  ↓
每 30 分钟执行心跳
  ↓
每 2 分钟续约锁
  ↓
失去锁？→ 停止心跳
```

### 3. 故障恢复

```
Leader 崩溃
  ↓
锁 5 分钟后过期
  ↓
其他实例获取锁
  ↓
新 Leader 接管
```

---

## 📊 对比

| 方案 | 优点 | 缺点 |
|------|------|------|
| **无锁（当前）** | 简单 | 多实例重复执行 |
| **分布式锁** | 只有一个实例执行 | 需要数据库支持 |

---

## ⚠️ 注意事项

1. **数据库依赖**: 需要 MySQL 支持
2. **锁超时**: 默认 5 分钟，可根据心跳间隔调整
3. **实例 ID**: 必须唯一，建议使用 hostname + PID
4. **故障恢复**: 最长延迟 = 锁超时时间（5 分钟）

---

## 🎉 收益

- ✅ 避免重复执行（节省 LLM 调用）
- ✅ 自动故障转移（高可用）
- ✅ 无需外部协调服务（使用现有 MySQL）

---

详细实现见:
- `pkg/scheduler/distributed_lock.go`
- `pkg/scheduler/distributed_heartbeat.go`
