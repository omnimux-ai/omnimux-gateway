package tests

import (
	"io"
	"log/slog"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"omnimux-inspiration/internal/config"
	"omnimux-inspiration/internal/router"
	"omnimux-inspiration/internal/store"
)

func TestR2MediaRouteLocalFirstAndMissing(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "media-routes.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB.DB()
	t.Cleanup(func() { sqlDB.Close() })
	if _, err = db.Migrate("../../migrations"); err != nil {
		t.Fatal(err)
	}
	mediaDir := t.TempDir()
	key := "r2/publications/genviral/slideshows/example/slide-1.jpg"
	local := filepath.Join(mediaDir, filepath.FromSlash(key))
	if err := os.MkdirAll(filepath.Dir(local), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(local, []byte("local-media"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{Token: "fixture", MediaDir: mediaDir, MediaURLPrefix: "/media", MaxBodyMB: 8}
	r := router.New(cfg, db, slog.New(slog.NewTextHandler(io.Discard, nil)), "test")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/media/"+key, nil))
	if w.Code != 200 || w.Body.String() != "local-media" {
		t.Fatalf("local first: %d %q", w.Code, w.Body.String())
	}

	missing := "r2/publications/genviral/slideshows/example/missing.jpg"
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/media/"+missing, nil))
	if w.Code != 404 {
		t.Fatalf("unconfigured R2 missing: %d", w.Code)
	}
}
