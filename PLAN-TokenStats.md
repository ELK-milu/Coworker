# 修复计划：Token 用量 + Cost 计数为 0

## 问题诊断

前端显示 `Cost:$0.0000 Tokens:0`，但 `Time:3.7s` 正常 → status 事件确实收到了，但 token 数据全是 0。

### 根因链路

```
sendStatusEvent()
  → l.session.GetContextStats()
    → s.Context.GetStats()
      → 遍历 m.turns 累加 t.APIUsage
        → m.turns 为空！→ 返回 0
```

**三个断裂点：**

1. **`handler.go`** — `message_start` 事件未处理，`input_tokens` 从未被捕获
   - 只在 `message_delta` 中捕获了 `output_tokens`
   - Anthropic API 的 `input_tokens` 在 `message_start` 事件中返回

2. **`processStream`** — 没有 `case client.EventUsage` 分支
   - handler.go 发出的 `EventUsage` 事件被完全忽略
   - 没有任何地方存储 usage 数据

3. **`context.AddTurn()`** — 从未被调用
   - 对话循环用 `session.AddMessage()` 直接追加消息
   - `context.Manager.turns` 始终为空
   - `GetStats()` 遍历空数组，返回全 0

---

## 修复方案

### 方案选择：在 ConversationLoop 中直接追踪 usage（最简单）

不改动 context.Manager 的 AddTurn 架构，而是在 ConversationLoop 层面直接捕获和使用 API 返回的 usage 数据。

---

### 文件 1：`claudecli/internal/client/handler.go`

**新增 `message_start` 事件处理，捕获 `input_tokens`：**

```go
case "message_start":
    if event.Message.Usage.InputTokens > 0 {
        eventCh <- StreamEvent{
            Type: EventUsage,
            Usage: &UsageInfo{
                InputTokens: int(event.Message.Usage.InputTokens),
            },
        }
    }
```

现有 `message_delta` 中已有 `output_tokens` 捕获，保持不变。

---

### 文件 2：`claudecli/internal/loop/conversation.go`

**2.1 新增字段：**

```go
type ConversationLoop struct {
    // ... 现有字段 ...
    lastInputTokens  int   // 本轮 API 返回的 input_tokens
    lastOutputTokens int   // 本轮 API 返回的 output_tokens
}
```

**2.2 在 `processStream` 中处理 `EventUsage`：**

在 switch 中新增：
```go
case client.EventUsage:
    if event.Usage != nil {
        if event.Usage.InputTokens > 0 {
            l.lastInputTokens = event.Usage.InputTokens
        }
        if event.Usage.OutputTokens > 0 {
            l.lastOutputTokens = event.Usage.OutputTokens
        }
    }
```

**2.3 在每轮循环开始时重置：**

在 `for` 循环开头（调用 API 前）：
```go
l.lastInputTokens = 0
l.lastOutputTokens = 0
```

**2.4 修改 `sendStatusEvent` 使用直接数据：**

```go
func (l *ConversationLoop) sendStatusEvent() {
    stats := l.session.GetContextStats()
    elapsed := time.Now().UnixMilli() - l.startTime

    // 使用 API 直接返回的 usage 数据（而非 context.GetStats 的累计值）
    inputTokens := l.lastInputTokens
    outputTokens := l.lastOutputTokens
    totalTokens := inputTokens + outputTokens

    contextPercent := 0.0
    if stats.ContextMax > 0 {
        contextPercent = float64(stats.ContextUsed) / float64(stats.ContextMax) * 100
    }

    l.eventCh <- LoopEvent{
        Type: EventTypeStatus,
        Status: &StatusInfo{
            Model:          l.client.GetModel(),
            InputTokens:    inputTokens,
            OutputTokens:   outputTokens,
            TotalTokens:    totalTokens,
            ContextUsed:    stats.ContextUsed,
            ContextMax:     stats.ContextMax,
            ContextPercent: contextPercent,
            ElapsedMs:      elapsed,
            Mode:           l.mode,
        },
    }
}
```

---

### 不需要修改的文件

- **前端 `index.jsx`** — status 事件处理、turnStats/sessionStats 累加、calculateCost 逻辑已正确实现
- **前端 `styles.css`** — 样式已添加
- **`context/context.go`** — GetStats 逻辑本身没问题，只是 turns 没数据
- **`controller/claudecli.go`** — `/coworker/ratio_config` 已添加
- **`router/claudecli-router.go`** — 路由已注册

---

## 数据流（修复后）

```
Anthropic API 流式响应
  ├─ message_start → input_tokens → handler.go → EventUsage → processStream → l.lastInputTokens
  └─ message_delta → output_tokens → handler.go → EventUsage → processStream → l.lastOutputTokens
                                                                      ↓
                                                            sendStatusEvent()
                                                            使用 l.lastInputTokens + l.lastOutputTokens
                                                                      ↓
                                                            WebSocket status 消息
                                                            { input_tokens: N, output_tokens: M, total_tokens: N+M }
                                                                      ↓
                                                            前端 case 'status' → turnStats
                                                                      ↓
                                                            前端 case 'done' → sessionStats 累加 → localStorage
                                                                      ↓
                                                            UI: Model | Cost:$X.XX | Tokens:N | Time:Xs
```

---

## 涉及文件清单

| 文件 | 操作 | 说明 |
|------|------|------|
| `claudecli/internal/client/handler.go` | 修改 | 新增 message_start 处理，捕获 input_tokens |
| `claudecli/internal/loop/conversation.go` | 修改 | 新增 lastUsage 字段 + EventUsage 处理 + sendStatusEvent 改用直接数据 |
