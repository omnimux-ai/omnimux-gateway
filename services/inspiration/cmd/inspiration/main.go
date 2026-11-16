// inspiration 微服务入口：serve + CLI 双模（Cobra）。
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"omnimux-inspiration/internal/config"
	"omnimux-inspiration/internal/media"
	"omnimux-inspiration/internal/router"
	"omnimux-inspiration/internal/service"
	"omnimux-inspiration/internal/store"
)

var version = "dev"

func newLogger(cfg *config.Config) *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))
}

func main() {
	root := &cobra.Command{
		Use:     "inspiration",
		Short:   "OmniMux 灵感库微服务",
		Version: version,
	}
	root.AddCommand(serveCmd(), migrateCmd(), importCmd(), seedCmd(), healthcheckCmd(), versionCmd(), coversCmd())
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func serveCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "serve",
		Short: "启动 HTTP 服务",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.Load(".env", cmd.Flag("env-file").Value.String())
			if cfg.DSN == "" {
				return errors.New("INSPIRATION_DSN is required (env or .env)")
			}
			if err := cfg.ValidateCoverR2(); err != nil {
				return err
			}
			log := newLogger(cfg)
			log.Info("starting", "version", version, "env", cfg.Env, "addr", cfg.Addr)

			db, err := store.Open(cfg.DSN)
			if err != nil {
				return fmt.Errorf("open db: %w", err)
			}
			if cfg.AutoMigrate {
				applied, err := db.Migrate(migrationsDir())
				if err != nil {
					return fmt.Errorf("migrate: %w", err)
				}
				if len(applied) > 0 {
					log.Info("migrations applied", "files", applied)
				}
			}
			if err := os.MkdirAll(cfg.MediaDir, 0o755); err != nil {
				return err
			}

			srv := &http.Server{
				Addr:              cfg.Addr,
				Handler:           router.New(cfg, db, log, version),
				ReadHeaderTimeout: 5 * time.Second,
			}
			go func() {
				if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
					log.Error("listen error", "err", err)
					os.Exit(1)
				}
			}()
			log.Info("listening", "addr", cfg.Addr)

			quit := make(chan os.Signal, 1)
			signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
			<-quit
			log.Info("shutting down")
			ctx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownWait)
			defer cancel()
			return srv.Shutdown(ctx)
		},
	}
	c.Flags().String("env-file", "", "可选 .env 路径（默认 .env）")
	return c
}

func healthcheckCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "healthcheck",
		Short: "探测 DB 就绪（Docker HEALTHCHECK）",
		RunE: func(cmd *cobra.Command, args []string) error {
			dsn := os.Getenv("INSPIRATION_DSN")
			if dsn == "" {
				return errors.New("INSPIRATION_DSN required")
			}
			db, err := store.Open(dsn)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			return db.Ping(ctx)
		},
	}
}

func migrateCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "migrate",
		Short: "数据库迁移（status/up）",
	}
	status := &cobra.Command{Use: "status", RunE: func(cmd *cobra.Command, args []string) error {
		return migrateRun(cmd, false)
	}}
	up := &cobra.Command{Use: "up", RunE: func(cmd *cobra.Command, args []string) error {
		return migrateRun(cmd, true)
	}}
	for _, sub := range []*cobra.Command{status, up} {
		sub.Flags().String("dsn", "", "数据库 DSN（默认取 INSPIRATION_DSN）")
		c.AddCommand(sub)
	}
	return c
}

func migrateRun(cmd *cobra.Command, apply bool) error {
	dsn := cmd.Flag("dsn").Value.String()
	if dsn == "" {
		dsn = os.Getenv("INSPIRATION_DSN")
	}
	if dsn == "" {
		return errors.New("DSN required (--dsn or INSPIRATION_DSN)")
	}
	db, err := store.Open(dsn)
	if err != nil {
		return err
	}
	if apply {
		applied, err := db.Migrate(migrationsDir())
		if err != nil {
			return err
		}
		fmt.Println("applied:", applied)
		return nil
	}
	var versions []string
	if err := db.Raw("SELECT version FROM schema_migrations ORDER BY version").Scan(&versions).Error; err != nil {
		return err
	}
	fmt.Println("applied versions:", versions)
	return nil
}

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use: "version",
		Run: func(cmd *cobra.Command, args []string) { fmt.Println(version) },
	}
}

// importCmd 从 JSON 文件批量导入（脚本主通道）。等价于 POST /v1/inspirations:bulk，
// 但走 DSN 直连（绕过 HTTP），适合离线灌库。
// 输入格式：{"items":[{type,title,content,source_url,tags,...}, ...]}
func importCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "import",
		Short: "从 JSON 文件批量导入灵感条目",
		RunE: func(cmd *cobra.Command, args []string) error {
			dsn := cmd.Flag("dsn").Value.String()
			if dsn == "" {
				dsn = os.Getenv("INSPIRATION_DSN")
			}
			if dsn == "" {
				return errors.New("--dsn or INSPIRATION_DSN required")
			}
			path := cmd.Flag("file").Value.String()
			if path == "" {
				return errors.New("--file required")
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			var req service.BulkReq
			if err := json.Unmarshal(raw, &req); err != nil {
				return fmt.Errorf("parse %s: %w", path, err)
			}
			db, err := store.Open(dsn)
			if err != nil {
				return err
			}
			if _, err := db.Migrate(migrationsDir()); err != nil {
				return fmt.Errorf("migrate: %w", err)
			}
			cfg := config.Load("", "")
			svc := service.New(db, cfg.MediaURLPrefix, media.New(cfg))
			res, err := svc.BulkUpsertTx(context.Background(), &req)
			if err != nil {
				return err
			}
			fmt.Printf("created=%d updated=%d skipped=%d\n", res.Created, res.Updated, res.Skipped)
			return nil
		},
	}
	c.Flags().String("file", "", "JSON 文件路径")
	c.Flags().String("dsn", "", "数据库 DSN（默认 INSPIRATION_DSN）")
	return c
}

func seedCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "seed",
		Short: "导入 migrations/seed.sql（仅本地/CI，生产禁用）",
		RunE: func(cmd *cobra.Command, args []string) error {
			dsn := cmd.Flag("dsn").Value.String()
			if dsn == "" {
				dsn = os.Getenv("INSPIRATION_DSN")
			}
			if dsn == "" {
				return errors.New("--dsn or INSPIRATION_DSN required")
			}
			db, err := store.Open(dsn)
			if err != nil {
				return err
			}
			if db.Dialect != store.DialectMySQL {
				return errors.New("seed.sql is MySQL-only")
			}
			if _, err := db.Migrate(migrationsDir()); err != nil {
				return fmt.Errorf("migrate: %w", err)
			}
			p := filepath.Join(migrationsDir(), "seed.sql")
			if err := db.SeedMySQL(p); err != nil {
				return fmt.Errorf("seed %s: %w", p, err)
			}
			fmt.Println("seed applied:", p)
			return nil
		},
	}
	c.Flags().String("dsn", "", "数据库 DSN（默认 INSPIRATION_DSN）")
	return c
}

// migrationsDir 优先可执行文件同级 migrations/，其次 cwd。
func migrationsDir() string {
	if exe, err := os.Executable(); err == nil {
		p := filepath.Join(filepath.Dir(exe), "migrations")
		if st, err := os.Stat(p); err == nil && st.IsDir() {
			return p
		}
	}
	return "migrations"
}
