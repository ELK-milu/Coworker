# R4: 高级工具规范

## 概述

实现 WebFetch、WebSearch、LSP、AskUserQuestion 等高级工具。

---

## WebFetchTool

```go
type WebFetchInput struct {
    URL    string `json:"url"`
    Prompt string `json:"prompt"`
}

type WebFetchTool struct {
    httpClient *http.Client
    timeout    time.Duration
}

func (t *WebFetchTool) Name() string { return "WebFetch" }
func (t *WebFetchTool) Execute(ctx context.Context, input json.RawMessage) (*ToolResult, error)
```

**功能:**
- 获取网页内容
- HTML 转 Markdown
- 支持重定向跟随

---

## AskUserQuestionTool

```go
type QuestionInput struct {
    Questions []Question `json:"questions"`
}

type Question struct {
    Question    string   `json:"question"`
    Header      string   `json:"header"`
    Options     []Option `json:"options"`
    MultiSelect bool     `json:"multiSelect"`
}

type Option struct {
    Label       string `json:"label"`
    Description string `json:"description"`
}
```

**功能:**
- 向用户提问
- 支持单选/多选
- 通过 WebSocket 交互

---

## LSPTool

```go
type LSPInput struct {
    Operation string `json:"operation"`
    FilePath  string `json:"filePath"`
    Line      int    `json:"line"`
    Character int    `json:"character"`
}
```

**支持操作:**
- goToDefinition
- findReferences
- hover
- documentSymbol

---

## 验收标准

- [ ] WebFetchTool 实现
- [ ] AskUserQuestionTool 实现
- [ ] LSPTool 基础实现
- [ ] 工具注册到 Registry
