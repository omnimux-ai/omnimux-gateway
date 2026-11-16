// Package middleware 请求 ID、访问日志、鉴权、限流、recovery。
package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// RequestID 生成短请求 ID 并注入 context + 响应头。
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		b := make([]byte, 8)
		_, _ = rand.Read(b)
		id := hex.EncodeToString(b)
		c.Set("request_id", id)
		c.Header("X-Request-Id", id)
		c.Next()
	}
}

// Access slog JSON 访问日志。
func Access(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		log.Info("access",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"latency_ms", float64(time.Since(start).Microseconds())/1000.0,
			"request_id", c.GetString("request_id"),
			"client_ip", c.ClientIP(),
		)
	}
}

// Recovery 兜底 panic。
func Recovery(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				log.Error("panic", "recover", r, "path", c.Request.URL.Path)
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"success": false, "code": "INTERNAL", "message": "internal error",
					"request_id": c.GetString("request_id"),
				})
			}
		}()
		c.Next()
	}
}

// Auth 静态 Bearer token 校验；token 为空表示本地模式不鉴权。
func Auth(token string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if token == "" {
			c.Next()
			return
		}
		h := c.GetHeader("Authorization")
		if len(h) > 7 && (h[:7] == "Bearer " || h[:7] == "bearer ") {
			if h[7:] == token {
				c.Next()
				return
			}
		}
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"success": false, "code": "UNAUTHORIZED", "message": "unauthorized",
			"request_id": c.GetString("request_id"),
		})
	}
}

// RateLimit 简单固定窗口限流（rps<=0 不启用）。
type window struct {
	start time.Time
	count int
}

func RateLimit(rps int) gin.HandlerFunc {
	if rps <= 0 {
		return func(c *gin.Context) { c.Next() }
	}
	var mu sync.Mutex
	buckets := map[string]*window{}
	return func(c *gin.Context) {
		key := c.ClientIP()
		now := time.Now()
		mu.Lock()
		w, ok := buckets[key]
		if !ok || now.Sub(w.start) >= time.Second {
			w = &window{start: now}
			buckets[key] = w
		}
		w.count++
		over := w.count > rps
		mu.Unlock()
		if over {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"success": false, "code": "RATE_LIMITED", "message": "rate limited",
				"request_id": c.GetString("request_id"),
			})
			return
		}
		c.Next()
	}
}
