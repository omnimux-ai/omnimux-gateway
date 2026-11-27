// Package middleware 请求 ID、访问日志、鉴权、限流、recovery。
package middleware

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"log/slog"
	"net/http"
	"os"
	"strings"
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

// Auth 静态 Bearer token 校验；仅 INSPIRATION_ENV=dev 时允许空 token。
func Auth(token string) gin.HandlerFunc {
	return AuthWithEnv(token, os.Getenv("INSPIRATION_ENV"))
}

// AuthWithEnv 静态 Bearer token 校验（常量时间比较）；非 dev 环境空 token 一律拒绝。
func AuthWithEnv(token, env string) gin.HandlerFunc {
	allowEmpty := strings.EqualFold(strings.TrimSpace(env), "dev")
	return func(c *gin.Context) {
		if token == "" {
			if allowEmpty {
				c.Next()
				return
			}
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"success": false, "code": "UNAUTHORIZED", "message": "unauthorized",
				"request_id": c.GetString("request_id"),
			})
			return
		}
		h := c.GetHeader("Authorization")
		if len(h) > 7 && (h[:7] == "Bearer " || h[:7] == "bearer ") {
			if subtle.ConstantTimeCompare([]byte(h[7:]), []byte(token)) == 1 {
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

// RateLimit 简单固定窗口限流（rps<=0 不启用），带定期过期回收与上限保护防止内存增长。
type window struct {
	start time.Time
	count int
}

const (
	rateLimitMaxBuckets    = 10000
	rateLimitSweepInterval = 30 * time.Second
)

func RateLimit(rps int) gin.HandlerFunc {
	if rps <= 0 {
		return func(c *gin.Context) { c.Next() }
	}
	var mu sync.Mutex
	buckets := map[string]*window{}
	lastSweep := time.Now()
	return func(c *gin.Context) {
		key := c.ClientIP()
		now := time.Now()
		mu.Lock()
		if len(buckets) >= rateLimitMaxBuckets || now.Sub(lastSweep) >= rateLimitSweepInterval {
			for k, w := range buckets {
				if now.Sub(w.start) >= time.Second {
					delete(buckets, k)
				}
			}
			lastSweep = now
			if len(buckets) >= rateLimitMaxBuckets {
				clear(buckets)
			}
		}
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
