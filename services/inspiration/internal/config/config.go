// Package config 从环境变量加载配置（OS env 优先，可选 .env）。
package config

import (
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	DSN            string
	Addr           string
	MediaDir       string
	MediaURLPrefix string
	Token          string
	AutoMigrate    bool
	LogLevel       slog.Level
	RateLimit      int
	MaxBodyMB      int64
	MaxUploadMB    int64
	Env            string
	ShutdownWait   time.Duration

	CoverR2Endpoint  string
	CoverR2Bucket    string
	CoverR2AccessKey string
	CoverR2SecretKey string

	// Optional Cloudflare R2 backup / sink
	R2Endpoint  string
	R2AccessKey string
	R2SecretKey string
	R2Bucket    string
	R2Region    string
	R2Prefix    string
}

func Load(envFile string, flagEnvFile string) *Config {
	// flag 指定 .env 优先，其次默认 .env
	load := envFile
	if flagEnvFile != "" {
		load = flagEnvFile
	}
	if load != "" {
		_ = godotenv.Load(load) // 静默失败：无文件则用 OS env
	}
	return &Config{
		DSN:            getenv("INSPIRATION_DSN", ""),
		Addr:           getenv("INSPIRATION_ADDR", ":8210"),
		MediaDir:       getenv("INSPIRATION_MEDIA_DIR", "./data/media"),
		MediaURLPrefix: getenv("INSPIRATION_MEDIA_URL_PREFIX", "/media"),
		Token:          getenv("INSPIRATION_TOKEN", ""),
		AutoMigrate:    getenvBool("INSPIRATION_AUTO_MIGRATE", true),
		LogLevel:       parseInt(slog.LevelInfo, "INSPIRATION_LOG_LEVEL"),
		RateLimit:      getenvInt("INSPIRATION_RATE_LIMIT", 0),
		MaxBodyMB:      int64(getenvInt("INSPIRATION_MAX_BODY_MB", 8)),
		MaxUploadMB:    int64(getenvInt("INSPIRATION_MAX_UPLOAD_MB", 5)),
		Env:            getenv("INSPIRATION_ENV", "dev"),
		ShutdownWait:   time.Duration(getenvInt("INSPIRATION_SHUTDOWN_WAIT_SEC", 10)) * time.Second,

		CoverR2Endpoint:  strings.TrimSpace(getenv("INSPIRATION_COVER_R2_ENDPOINT", "")),
		CoverR2Bucket:    strings.TrimSpace(getenv("INSPIRATION_COVER_R2_BUCKET", "")),
		CoverR2AccessKey: strings.TrimSpace(getenv("INSPIRATION_COVER_R2_ACCESS_KEY", "")),
		CoverR2SecretKey: strings.TrimSpace(getenv("INSPIRATION_COVER_R2_SECRET_KEY", "")),
		R2Endpoint:       getenv("INSPIRATION_R2_ENDPOINT", ""),
		R2AccessKey:      getenv("INSPIRATION_R2_ACCESS_KEY", ""),
		R2SecretKey:      getenv("INSPIRATION_R2_SECRET_KEY", ""),
		R2Bucket:         getenv("INSPIRATION_R2_BUCKET", ""),
		R2Region:         getenv("INSPIRATION_R2_REGION", "auto"),
		R2Prefix:         getenv("INSPIRATION_R2_PREFIX", "inspirations"),
	}
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func getenvBool(k string, def bool) bool {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	switch strings.ToLower(v) {
	case "true", "1", "yes", "on":
		return true
	default:
		return false
	}
}

func getenvInt(k string, def int) int {
	if v := os.Getenv(k); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func parseInt(def slog.Level, k string) slog.Level {
	switch strings.ToLower(os.Getenv(k)) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return def
	}
}

// ValidateCoverR2 rejects partial configuration and an unauthenticated private reader.
func (c *Config) ValidateCoverR2() error {
	if c == nil {
		return fmt.Errorf("cover reader configuration required")
	}
	values := []string{c.CoverR2Endpoint, c.CoverR2Bucket, c.CoverR2AccessKey, c.CoverR2SecretKey}
	count := 0
	for _, v := range values {
		if v != "" {
			count++
		}
	}
	if count == 0 {
		return nil
	}
	if count != 4 || strings.TrimSpace(c.Token) == "" {
		return fmt.Errorf("all INSPIRATION_COVER_R2 fields and INSPIRATION_TOKEN are required")
	}
	u, err := url.Parse(c.CoverR2Endpoint)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || !strings.HasSuffix(u.Hostname(), ".r2.cloudflarestorage.com") || strings.ContainsAny(c.CoverR2Bucket, "/\\?#%:") {
		return fmt.Errorf("invalid cover R2 endpoint or bucket")
	}
	return nil
}
