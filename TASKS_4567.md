# 功能4-7实施任务清单

## 功能4：浪潮式记忆系统 (TagMemo)

### 后端任务
- [ ] 4.1 创建 `claudecli/internal/memory/memory.go` - 记忆数据模型和管理器
- [ ] 4.2 创建 `claudecli/internal/memory/retrieval.go` - 三阶段检索算法
- [ ] 4.3 创建 `claudecli/internal/memory/extractor.go` - 记忆提取器
- [ ] 4.4 修改 `claudecli/init.go` - 初始化 Memory 管理器
- [ ] 4.5 修改 `claudecli/internal/api/websocket.go` - Memory WebSocket 消息处理
- [ ] 4.6 修改 `claudecli/internal/prompt/builder.go` - 添加 RelevantMemories 字段

### 前端任务
- [ ] 4.7 创建 `web/src/pages/Coworker/components/MemoryPanel.jsx` - 记忆面板组件
- [ ] 4.8 创建 `web/src/pages/Coworker/components/MemoryPanel.css` - 样式
- [ ] 4.9 修改 `web/src/pages/Coworker/index.jsx` - 集成记忆状态
- [ ] 4.10 修改 `web/src/pages/Coworker/components/SessionSidebar.jsx` - 添加记忆标签页

---

## 功能5：用户画像系统 (UserProfile)

### 后端任务
- [ ] 5.1 创建 `claudecli/internal/profile/profile.go` - 用户画像模型
- [ ] 5.2 创建 `claudecli/internal/profile/learner.go` - 自动学习器
- [ ] 5.3 修改 `claudecli/init.go` - 初始化 Profile 管理器
- [ ] 5.4 修改 `claudecli/internal/api/websocket.go` - Profile WebSocket 消息处理
- [ ] 5.5 修改 `claudecli/internal/prompt/builder.go` - 添加 UserProfile 字段

### 前端任务
- [ ] 5.6 创建 `web/src/pages/Coworker/components/ProfileSettings.jsx` - 画像设置组件
- [ ] 5.7 创建 `web/src/pages/Coworker/components/ProfileSettings.css` - 样式
- [ ] 5.8 修改 `web/src/pages/Coworker/index.jsx` - 集成画像状态

---

## 功能6：动态变量替换系统

### 后端任务
- [ ] 6.1 创建 `claudecli/internal/variable/variable.go` - 变量模型和解析器
- [ ] 6.2 创建 `claudecli/internal/variable/builtin.go` - 内置变量定义
- [ ] 6.3 修改 `claudecli/init.go` - 初始化 Variable 管理器
- [ ] 6.4 修改 `claudecli/internal/prompt/builder.go` - 集成变量解析

---

## 功能7：会话记忆模板增强

### 后端任务
- [ ] 7.1 修改 `claudecli/internal/context/session_memory.go` - 增强现有实现
- [ ] 7.2 创建 `claudecli/internal/context/memory_generator.go` - AI 生成器
- [ ] 7.3 修改 `claudecli/internal/loop/conversation.go` - 对话结束时生成
- [ ] 7.4 修改 `claudecli/internal/prompt/builder.go` - 注入相关 Session Memory

---

## 实施顺序

1. **功能6** (动态变量) - 基础设施，其他功能依赖
2. **功能7** (Session Memory增强) - 已有基础，增强即可
3. **功能4** (记忆系统) - 核心功能
4. **功能5** (用户画像) - 依赖记忆系统

---

## 当前进度

- 开始时间: 2026-02-07
- 当前阶段: 功能6 - 动态变量替换系统
