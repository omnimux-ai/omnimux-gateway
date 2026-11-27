// Package router 装配全部路由。
package router

import (
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"omnimux-inspiration/internal/api"
	"omnimux-inspiration/internal/config"
	"omnimux-inspiration/internal/media"
	"omnimux-inspiration/internal/middleware"
	"omnimux-inspiration/internal/service"
	"omnimux-inspiration/internal/store"
)

// New 构建 gin engine。version 形如 0.1.0（ldflags 注入）。
func New(cfg *config.Config, db *store.DB, log *slog.Logger, version string) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()

	r.Use(middleware.RequestID(), middleware.Recovery(log), middleware.Access(log))
	r.MaxMultipartMemory = 8 << 20

	mediaStore := media.New(cfg)
	svc := service.New(db, cfg.MediaURLPrefix, mediaStore)
	h := api.New(svc, mediaStore, cfg.MediaURLPrefix)
	reader, err := media.NewCoverReader(cfg)
	if err != nil {
		panic(err)
	}
	h.Covers = reader
	mediaReader, err := media.NewMediaReader(cfg)
	if err != nil {
		panic(err)
	}
	if mediaReader != nil {
		h.MediaStreamer = mediaReader
	}

	// 运维面（免鉴权）
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	r.GET("/readyz", func(c *gin.Context) {
		if err := db.Ping(c.Request.Context()); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"ok": false, "error": "db unavailable"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	r.GET("/version", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"version": version, "env": cfg.Env})
	})
	r.GET("/metrics", metricsHandler(db, log))

	// 媒体（免鉴权，内网；路径穿越防护）
	localMedia := mediaHandler(cfg.MediaDir)
	auth := middleware.AuthWithEnv(cfg.Token, cfg.Env)
	r.GET("/media/*key", func(c *gin.Context) {
		key := strings.TrimPrefix(c.Param("key"), "/")
		if strings.HasPrefix(key, "inspiration-covers/") {
			if cfg.Token == "" {
				c.AbortWithStatusJSON(401, gin.H{"success": false, "code": "UNAUTHORIZED", "request_id": c.GetString("request_id")})
				return
			}
			auth(c)
			if c.IsAborted() {
				return
			}
			h.Cover(c, strings.TrimPrefix(key, "inspiration-covers/"))
			return
		}
		if strings.HasPrefix(key, "r2/") {
			if full, err := mediaStore.SafeLocalPath(key); err == nil {
				if info, err := os.Stat(full); err == nil && info.Mode().IsRegular() {
					localMedia(c)
					return
				}
			}
			h.StreamMedia(c, key)
			return
		}
		localMedia(c)
	})

	// 业务面（鉴权 + 限流 + body 上限）
	v1 := r.Group("/v1", middleware.RateLimit(cfg.RateLimit), auth)
	v1.Use(func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, cfg.MaxBodyMB<<20)
		c.Next()
	})

	v1.GET("/inspirations", h.List)
	v1.GET("/inspirations/:id", h.Get)
	v1.POST("/inspirations", h.Create)
	v1.POST("/inspirations:bulk", h.Bulk)
	v1.PATCH("/inspirations/:id", h.Update)
	v1.DELETE("/inspirations/:id", h.Delete)
	v1.GET("/tags", h.ListTags)
	v1.POST("/tags", h.CreateTag)
	v1.DELETE("/tags/:id", h.DeleteTag)
	v1.POST("/media", h.UploadMedia)

	return r
}

func isSafeLocalMediaKey(key string) bool {
	if key == "" || len(key) > 512 || strings.Contains(key, "..") || strings.HasPrefix(key, "/") || strings.HasSuffix(key, "/") || strings.Contains(key, "//") || strings.Contains(key, "\\") {
		return false
	}
	for i := range len(key) {
		ch := key[i]
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') ||
			ch == '.' || ch == '_' || ch == '-' || ch == '/' {
			continue
		}
		return false
	}
	return true
}

// mediaHandler 出本地媒体文件，支持 Range（视频拖动），禁止目录枚举。
func mediaHandler(root string) gin.HandlerFunc {
	cleanRoot := filepath.Clean(root)
	return func(c *gin.Context) {
		key := strings.TrimPrefix(c.Param("key"), "/")
		if !isSafeLocalMediaKey(key) {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "code": "INVALID_PARAM", "message": "bad media key"})
			return
		}
		full := filepath.Join(cleanRoot, filepath.FromSlash(key))
		if !strings.HasPrefix(full, cleanRoot+string(os.PathSeparator)) {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "code": "INVALID_PARAM", "message": "bad media key"})
			return
		}
		info, err := os.Stat(full)
		if err != nil || !info.Mode().IsRegular() {
			c.JSON(http.StatusNotFound, gin.H{"success": false, "code": "NOT_FOUND", "message": "media not found"})
			return
		}
		c.Header("Content-Security-Policy", "sandbox; default-src 'none'")
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("Referrer-Policy", "no-referrer")
		http.ServeFile(c.Writer, c.Request, full) // 自带 Content-Type + Range
	}
}

func i64(n int64) string { return strconv.FormatInt(n, 10) }

// metricsHandler 极简 Prometheus 文本指标（无第三方依赖）。
func metricsHandler(db *store.DB, log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		var total int64
		var fav int64
		var tags int64
		db.Table("inspirations").Where("deleted_at IS NULL").Count(&total)
		db.Table("inspirations").Where("deleted_at IS NULL AND is_favorite = 1").Count(&fav)
		db.Table("tags").Count(&tags)
		body := strings.Builder{}
		body.WriteString("# TYPE inspiration_total gauge\n")
		body.WriteString("inspiration_total " + i64(total) + "\n")
		body.WriteString("# TYPE inspiration_favorite_total gauge\n")
		body.WriteString("inspiration_favorite_total " + i64(fav) + "\n")
		body.WriteString("# TYPE inspiration_tag_total gauge\n")
		body.WriteString("inspiration_tag_total " + i64(tags) + "\n")
		c.String(http.StatusOK, body.String())
	}
}
