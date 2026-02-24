package client

import (
	"context"
	"log"
	"math"
	"math/rand"
	"strconv"
	"strings"
	"time"
)

// 重试常量
const (
	MaxRetries       = 3
	InitialBackoffMs = 1000  // 初始退避 1 秒
	MaxBackoffMs     = 30000 // 最大退避 30 秒
	JitterFactor     = 0.25  // 抖动因子
)

// RetryableError 可重试的错误
type RetryableError struct {
	StatusCode   int
	Message      string
	RetryAfterMs int64 // 从 header 解析的等待时间
}

func (e *RetryableError) Error() string {
	return e.Message
}

// isRetryableError 判断错误是否可重试，并提取重试信息
func isRetryableError(err error) *RetryableError {
	if err == nil {
		return nil
	}

	msg := err.Error()

	// 429 Too Many Requests
	if strings.Contains(msg, "429") || strings.Contains(msg, "rate_limit") {
		retryAfter := parseRetryAfterFromError(msg)
		return &RetryableError{
			StatusCode:   429,
			Message:      msg,
			RetryAfterMs: retryAfter,
		}
	}

	// 503 Service Unavailable
	if strings.Contains(msg, "503") || strings.Contains(msg, "service_unavailable") {
		return &RetryableError{
			StatusCode:   503,
			Message:      msg,
			RetryAfterMs: 0,
		}
	}

	// 529 Overloaded
	if strings.Contains(msg, "529") || strings.Contains(msg, "overloaded") {
		return &RetryableError{
			StatusCode:   529,
			Message:      msg,
			RetryAfterMs: 0,
		}
	}

	// 500 Internal Server Error
	if strings.Contains(msg, "500") && strings.Contains(msg, "internal") {
		return &RetryableError{
			StatusCode:   500,
			Message:      msg,
			RetryAfterMs: 0,
		}
	}

	return nil
}

// parseRetryAfterFromError 从错误消息中提取 retry-after 时间
func parseRetryAfterFromError(msg string) int64 {
	// 尝试解析 "retry-after-ms: 1000" 格式
	if idx := strings.Index(msg, "retry-after-ms"); idx != -1 {
		sub := msg[idx:]
		parts := strings.SplitN(sub, ":", 2)
		if len(parts) == 2 {
			val := strings.TrimSpace(parts[1])
			// 取到第一个非数字字符
			numStr := extractNumber(val)
			if ms, err := strconv.ParseInt(numStr, 10, 64); err == nil {
				return ms
			}
		}
	}

	// 尝试解析 "retry-after: 5" 格式（秒）
	if idx := strings.Index(msg, "retry-after"); idx != -1 {
		sub := msg[idx:]
		parts := strings.SplitN(sub, ":", 2)
		if len(parts) == 2 {
			val := strings.TrimSpace(parts[1])
			numStr := extractNumber(val)
			if secs, err := strconv.ParseFloat(numStr, 64); err == nil {
				return int64(secs * 1000)
			}
		}
	}

	return 0
}

// extractNumber 从字符串开头提取数字
func extractNumber(s string) string {
	var result strings.Builder
	for _, c := range s {
		if (c >= '0' && c <= '9') || c == '.' {
			result.WriteRune(c)
		} else if result.Len() > 0 {
			break
		}
	}
	return result.String()
}

// calculateBackoff 计算退避时间（指数退避 + 抖动）
func calculateBackoff(attempt int, retryAfterMs int64) time.Duration {
	// 如果有 retry-after，优先使用
	if retryAfterMs > 0 {
		return time.Duration(retryAfterMs) * time.Millisecond
	}

	// 指数退避: initialBackoff * 2^attempt
	backoffMs := float64(InitialBackoffMs) * math.Pow(2, float64(attempt))
	if backoffMs > float64(MaxBackoffMs) {
		backoffMs = float64(MaxBackoffMs)
	}

	// 添加随机抖动
	jitter := backoffMs * JitterFactor * (rand.Float64()*2 - 1) // ±25%
	backoffMs += jitter

	return time.Duration(int64(backoffMs)) * time.Millisecond
}

// retryWithBackoff 带退避的重试执行器
// fn 返回 error，如果可重试则自动重试
func retryWithBackoff(ctx context.Context, name string, fn func() error) error {
	var lastErr error

	for attempt := 0; attempt <= MaxRetries; attempt++ {
		if attempt > 0 {
			log.Printf("[Retry] %s: attempt %d/%d", name, attempt, MaxRetries)
		}

		lastErr = fn()
		if lastErr == nil {
			return nil
		}

		// 检查是否可重试
		retryErr := isRetryableError(lastErr)
		if retryErr == nil {
			// 不可重试的错误，直接返回
			return lastErr
		}

		// 最后一次尝试失败，不再重试
		if attempt >= MaxRetries {
			log.Printf("[Retry] %s: max retries (%d) exhausted, giving up", name, MaxRetries)
			return lastErr
		}

		// 计算退避时间
		backoff := calculateBackoff(attempt, retryErr.RetryAfterMs)
		log.Printf("[Retry] %s: retryable error (status=%d), waiting %v before retry",
			name, retryErr.StatusCode, backoff)

		// 等待退避时间或上下文取消
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
			// 继续重试
		}
	}

	return lastErr
}
