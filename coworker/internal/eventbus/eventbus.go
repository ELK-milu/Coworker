package eventbus

import (
	"log"
	"sync"
)

// 事件类型常量
const (
	EventBeforeCompact = "before_compact" // compaction 即将执行（同步：必须完成后才压缩）
	EventAfterCompact  = "after_compact"  // compaction 完成（异步）
	EventTurnCompleted = "turn_completed" // 一轮对话结束（异步）
	EventSessionEnd    = "session_end"    // WS 断开（异步）
	EventSessionStart  = "session_start"  // 新会话创建（异步）
)

// Event 事件载荷
type Event struct {
	Type      string
	UserID    string
	SessionID string
	// 通用数据载荷，不同事件类型携带不同数据
	// BeforeCompact: "turns" → []ConversationTurn (interface{} 避免循环依赖)
	// TurnCompleted: "session" → *session.Session
	Data map[string]interface{}
}

// Handler 事件处理函数
type Handler func(event Event)

// Bus 事件总线
// 支持同步和异步两种处理模式：
// - 同步：Emit 时阻塞等待所有 handler 完成（用于 BeforeCompact 等必须先完成的场景）
// - 异步：Emit 时启动 goroutine 执行（用于 TurnCompleted 等不阻塞主流程的场景）
type Bus struct {
	syncHandlers  map[string][]Handler
	asyncHandlers map[string][]Handler
	mu            sync.RWMutex
}

// New 创建事件总线
func New() *Bus {
	return &Bus{
		syncHandlers:  make(map[string][]Handler),
		asyncHandlers: make(map[string][]Handler),
	}
}

// OnSync 注册同步事件处理器
// handler 在 Emit 时同步执行，阻塞直到完成
func (b *Bus) OnSync(eventType string, handler Handler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.syncHandlers[eventType] = append(b.syncHandlers[eventType], handler)
}

// OnAsync 注册异步事件处理器
// handler 在 Emit 时通过 goroutine 异步执行
func (b *Bus) OnAsync(eventType string, handler Handler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.asyncHandlers[eventType] = append(b.asyncHandlers[eventType], handler)
}

// Emit 触发事件
// 1. 先顺序执行所有同步 handler（阻塞）
// 2. 再并发启动所有异步 handler（不阻塞）
func (b *Bus) Emit(event Event) {
	b.mu.RLock()
	syncH := b.syncHandlers[event.Type]
	asyncH := b.asyncHandlers[event.Type]
	b.mu.RUnlock()

	// 同步 handler：顺序执行，recover panic
	for _, h := range syncH {
		func(handler Handler) {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("[EventBus] Sync handler panic for %s: %v", event.Type, r)
				}
			}()
			handler(event)
		}(h)
	}

	// 异步 handler：每个一个 goroutine
	for _, h := range asyncH {
		go func(handler Handler) {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("[EventBus] Async handler panic for %s: %v", event.Type, r)
				}
			}()
			handler(event)
		}(h)
	}
}

// EmitSync 触发事件并等待所有 handler（包括异步）完成
// 用于需要确保所有处理完成后再继续的场景
func (b *Bus) EmitSync(event Event) {
	b.mu.RLock()
	syncH := b.syncHandlers[event.Type]
	asyncH := b.asyncHandlers[event.Type]
	b.mu.RUnlock()

	// 同步 handler
	for _, h := range syncH {
		func(handler Handler) {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("[EventBus] Sync handler panic for %s: %v", event.Type, r)
				}
			}()
			handler(event)
		}(h)
	}

	// 异步 handler 也同步等待
	var wg sync.WaitGroup
	for _, h := range asyncH {
		wg.Add(1)
		go func(handler Handler) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					log.Printf("[EventBus] Async handler panic for %s: %v", event.Type, r)
				}
			}()
			handler(event)
		}(h)
	}
	wg.Wait()
}
