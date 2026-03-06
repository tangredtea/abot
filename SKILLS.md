# abot 技能列表

## 内置技能（9个）

### 1. browser - 浏览器自动化 🆕
使用 Playwright 进行浏览器自动化，支持页面导航、元素交互、数据提取。

### 2. clawhub - ClawHub 集成
与 ClawHub 技能市场集成，搜索和安装社区技能。

### 3. cron - 定时任务
创建和管理定时任务，支持 cron 表达式。

### 4. github - GitHub 集成
使用 gh CLI 与 GitHub 交互，管理 PR、Issue、CI 等。

### 5. memory - 记忆管理
高级记忆管理，支持 embedding 缓存、BM25、MMR 重排序。

### 6. skill-creator - 技能创建器
帮助创建新技能的模板和指导。

### 7. summarize - 文本摘要
总结长文本、文档或对话内容。

### 8. tmux - Tmux 管理
管理 tmux 会话、窗口和面板。

### 9. weather - 天气查询
查询天气信息。

---

## 技能使用

### 安装技能
```bash
# 从 ClawHub 安装
abot skill install <skill-name>

# 从本地安装
abot skill install ./path/to/skill
```

### 列出技能
```bash
abot skill list
```

### 使用技能
技能会自动加载到 Agent 的上下文中，Agent 可以根据技能文档使用相应的工具和命令。

---

## 技能开发

### 创建新技能

1. 创建技能目录：
```bash
mkdir -p skills/my-skill
```

2. 创建 SKILL.md：
```markdown
---
name: my-skill
description: "Brief description"
---

# My Skill

Documentation here...
```

3. 安装技能：
```bash
abot skill install ./skills/my-skill
```

### 技能结构

```
skills/
└── my-skill/
    └── SKILL.md  # 技能文档（必需）
```

### 最佳实践

- 提供清晰的使用示例
- 包含常见模式和最佳实践
- 说明依赖和安装步骤
- 使用代码块展示命令
