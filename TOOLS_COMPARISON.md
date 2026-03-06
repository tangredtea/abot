# abot vs Memoh-v2 工具对比

## abot 默认工具（17个核心工具）

### 文件操作（5个）✅
- `read_file` - 读取文件（最大 2MB）
- `write_file` - 写入文件（自动创建父目录）
- `edit_file` - 精确文本替换
- `append_file` - 追加内容
- `list_dir` - 列出目录（支持虚拟文件注入）

### 系统操作（2个）✅
- `exec` - 执行 shell 命令
- `message` - 发送消息到用户

### 网络操作（2个）✅
- `web_search` - 网页搜索
- `web_fetch` - 获取网页内容

### 任务管理（2个）✅
- `spawn` - 创建子任务
- `cron` - 定时任务调度

### 技能管理（4个）✅
- `find_skills` - 查找技能
- `install_skill` - 安装技能
- `create_skill` - 创建技能
- `promote_skill` - 推广技能到全局

### 记忆管理（2个）✅
- `save_memory` - 保存记忆（支持 embedding 缓存、BM25、MMR）
- `search_memory` - 搜索记忆（混合检索：40% 语义 + 30% BM25 + 20% 时间 + 10% 显著性）

### 可选工具（需要 Subagent）
- `subagent` - 子代理调用
- `list_tasks` - 列出任务

---

## Memoh-v2 工具提供者（18个模块）

### 基础功能
- `container` - 容器管理（启动/停止/状态）
- `directory` - 目录操作（读/写/列表）
- `memory` - 记忆管理（保存/搜索/删除）
- `message` - 消息发送

### 高级功能 ⭐
- `browser` - 浏览器自动化（Playwright）
- `webread` - 智能网页读取（自动降级策略）
- `web` - 网页操作
- `openviking` - 上下文数据库（分层检索 L0/L1/L2）

### 内容管理
- `inbox` - 收件箱管理
- `contacts` - 联系人管理
- `knowledge` - 知识库
- `history` - 历史记录

### 扩展功能
- `imagegen` - 图片生成
- `schedule` - 定时任务
- `skillstore` - 技能商店
- `admin` - 管理功能

---

## 功能对比矩阵

| 功能类别 | abot | Memoh-v2 | 评价 |
|---------|------|----------|------|
| **文件操作** | ✅ 5个工具 | ✅ directory | abot 更细粒度 |
| **命令执行** | ✅ exec | ✅ container | abot 更直接 |
| **网页访问** | ✅ web_search + web_fetch | ✅ web + webread | Memoh 更智能 |
| **浏览器自动化** | ❌ | ✅ browser | **Memoh 独有** |
| **记忆管理** | ✅ 优化版（缓存+BM25+MMR） | ✅ 基础版 | abot 更优 |
| **任务调度** | ✅ cron + spawn | ✅ schedule | 功能相当 |
| **技能系统** | ✅ 完整生态 | ✅ skillstore | abot 更完善 |
| **图片生成** | ❌ | ✅ imagegen | **Memoh 独有** |
| **上下文数据库** | ❌ | ✅ openviking | **Memoh 独有** |
| **消息发送** | ✅ message | ✅ message | 功能相当 |
| **子代理** | ✅ subagent | ❌ | **abot 独有** |

---

## 核心差异分析

### abot 优势 ✅

1. **记忆系统更强**
   - Embedding 缓存（减少 70% API 成本）
   - BM25 关键词索引
   - MMR 重排序
   - 混合检索策略

2. **技能生态完整**
   - 查找、安装、创建、推广全流程
   - 支持全局技能推广
   - 技能版本管理

3. **文件操作更细粒度**
   - 独立的 read/write/edit/append/list
   - 智能路由（persona 文件自动同步 MySQL）
   - 虚拟文件系统

4. **子代理支持**
   - 可以创建专门的子任务代理
   - 任务隔离和管理

### Memoh-v2 优势 ⭐

1. **浏览器自动化** 🌟
   - Playwright 集成
   - 完整的页面交互能力
   - 适合复杂的网页操作任务

2. **智能网页读取** 🌟
   - 自动降级策略（markdown → actionbook → browser）
   - 更可靠的内容提取

3. **OpenViking 上下文数据库** 🌟
   - 分层检索（L0 摘要 / L1 概览 / L2 详情）
   - 语义文件系统（viking:// URI）
   - 适合大规模知识管理

4. **图片生成**
   - 内置图片生成能力
   - 适合内容创作场景

---

## 建议

### abot 当前状态：✅ 完善

**核心工具已经非常完整：**
- 文件操作 ✅
- 命令执行 ✅
- 网络访问 ✅
- 记忆管理 ✅（且更优）
- 任务调度 ✅
- 技能系统 ✅（且更完善）

**可选增强（按优先级）：**

1. **浏览器自动化**（如果需要复杂网页交互）
   - 优点：能处理动态网页、表单提交、登录等
   - 缺点：增加依赖（Playwright/Chromium）、资源占用大
   - 建议：作为可选技能提供，不作为默认工具

2. **图片生成**（如果有内容创作需求）
   - 优点：丰富 Agent 能力
   - 缺点：需要集成第三方 API（DALL-E/Midjourney）
   - 建议：作为可选技能提供

3. **OpenViking 类似的分层检索**
   - 优点：更好的大规模知识管理
   - 缺点：复杂度高，当前的记忆系统已经足够好
   - 建议：暂不需要，当前的 BM25+MMR 已经很优秀

### 结论

**abot 的工具集已经非常完善，无需大改。**

核心优势：
- 记忆系统更优（缓存+BM25+MMR）
- 技能生态更完整
- 架构更简洁（无状态、易扩展）

如果要增强，建议：
- 浏览器自动化作为**可选技能**（不是默认工具）
- 保持核心工具的简洁性
- 通过技能系统扩展高级功能

**当前配置完全可以作为默认 Agent 使用。**
