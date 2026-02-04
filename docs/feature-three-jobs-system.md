# 功能3：事项系统 (Jobs) - Cron 定时任务调度

## 概述

实现了完整的 Cron 定时任务系统，支持在侧边栏管理定时事项，通过 Cron 表达式调度执行，并通过 WebSocket 实时通知前端。

---

## 后端实现

### 新建文件

| 文件 | 说明 |
|------|------|
| `claudecli/internal/job/job.go` | Job 模型、Manager（CRUD、持久化、到期检查） |
| `claudecli/internal/job/scheduler.go` | Cron 调度器（1 分钟 Ticker、goroutine 执行） |

### 修改文件

| 文件 | 变更 |
|------|------|
| `claudecli/init.go` | 初始化 JobManager、启动调度器、优雅关闭 |
| `claudecli/internal/api/handler.go` | ListJobs/CreateJob/UpdateJob/DeleteJob/RunJob/ReorderJobs REST 处理器 |
| `claudecli/internal/api/websocket.go` | Job WebSocket 消息处理、SetJobManager |
| `router/claudecli-router.go` | GET/POST/PUT/DELETE /coworker/jobs 路由 |
| `controller/claudecli.go` | Job 控制器代理方法 |
| `go.mod` | 添加 `github.com/gorhill/cronexpr` 依赖 |

### 数据模型

```go
type Job struct {
    ID        string                 // UUID
    UserID    string                 // 所属用户
    Name      string                 // 事项名称
    CronExpr  string                 // Cron 表达式
    Command   string                 // 执行命令/AI 指令
    Enabled   bool                   // 启用状态
    LastRun   int64                  // 上次执行时间戳
    NextRun   int64                  // 下次执行时间戳
    Status    Status                 // idle / running / failed
    LastError string                 // 上次错误信息
    Order     int                    // 排序序号
    Metadata  map[string]interface{} // 扩展元数据
    CreatedAt int64
    UpdatedAt int64
}
```

### REST API

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/coworker/jobs?user_id=` | 获取事项列表 |
| POST | `/coworker/jobs` | 创建事项 |
| PUT | `/coworker/jobs/:id` | 更新事项 |
| DELETE | `/coworker/jobs/:id?user_id=` | 删除事项 |
| POST | `/coworker/jobs/:id/run` | 手动触发 |
| PUT | `/coworker/jobs/reorder` | 批量排序 |

### WebSocket 消息

**Server → Client：**

| 类型 | 说明 | Payload |
|------|------|---------|
| `job_execution` | 事项开始执行 | `job_id, name, command, status` |
| `job_status` | 事项状态更新 | `job_id, status, last_run, next_run, last_error` |

### 调度器设计

- 使用 `time.Ticker` 每 1 分钟检查一次到期任务
- 通过 `github.com/gorhill/cronexpr` 解析 Cron 表达式
- 到期任务在独立 goroutine 中执行
- 支持 `SetExecutor` 注入执行回调
- 持久化路径：`userdata/{user_id}/.claude/jobs/{job_id}.json`

---

## 前端实现

### 新建文件

| 文件 | 说明 |
|------|------|
| `web/src/pages/Coworker/components/JobList.jsx` | 事项列表组件 |
| `web/src/pages/Coworker/components/JobList.css` | 事项列表样式 |

### 修改文件

| 文件 | 变更 |
|------|------|
| `web/src/pages/Coworker/components/SessionSidebar.jsx` | 添加"事项"标签页、导入 JobList、传递 props |
| `web/src/pages/Coworker/services/api.js` | listJobs/createJob/updateJob/deleteJob/runJob/reorderJobs API |
| `web/src/pages/Coworker/index.jsx` | jobs 状态、CRUD 函数、WebSocket 消息处理 |

### JobList 组件功能

- 创建/编辑/删除 Job
- 启用/禁用开关（Switch 组件）
- 手动执行按钮
- 拖拽排序
- 展开查看详情（命令、Cron 表达式、上次/下次执行时间、错误信息）
- Cron 表达式人类可读转换（内置简易 `cronToReadable`）
- 状态指示器（空闲/运行中/失败）

### Cron 表达式可读转换示例

| Cron 表达式 | 可读文本 |
|-------------|----------|
| `* * * * *` | 每分钟 |
| `*/5 * * * *` | 每 5 分钟 |
| `0 * * * *` | 每小时第 0 分钟 |
| `30 8 * * *` | 每天 08:30 |
| `0 9 * * 1` | 每周一 09:00 |
| `0 0 1 * *` | 每月 1 日 00:00 |

---

## 同期完成的其他功能

### 功能1：会话侧边栏实时更新 + 自动标题

- Session 模型添加 Title 字段
- `handleChat` 检测新会话并发送 `session_created` 事件
- AI 自动生成会话标题（`generateSessionTitle`）
- 前端实时处理 `session_created` 和 `title_updated` 事件

### 功能2：任务清单嵌入系统提示词

- `task.Manager.RenderCompact()` 紧凑渲染任务列表
- `PromptContext.TasksRender` 注入系统提示词
- AI 每次对话都能看到当前任务状态

---

## 验证方法

1. **创建事项**：在侧边栏"事项"标签页点击 + 创建，填写名称、Cron 表达式和命令
2. **手动执行**：点击播放按钮触发执行
3. **启用/禁用**：Switch 开关控制调度器是否调度该事项
4. **自动调度**：创建 `* * * * *`（每分钟）的测试事项，等待 1 分钟后检查日志
5. **拖拽排序**：拖动事项改变顺序
6. **展开详情**：点击事项查看命令、时间等详细信息
