package tests

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"

	"omnimux-inspiration/internal/config"
	"omnimux-inspiration/internal/model"
	"omnimux-inspiration/internal/router"
	"omnimux-inspiration/internal/store"
)

func TestR2CoverRouteAndDecoration(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "routes.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB.DB()
	t.Cleanup(func() { sqlDB.Close() })
	if _, err = db.Migrate("../../migrations"); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{Token: "fixture-token", MediaDir: t.TempDir(), MediaURLPrefix: "/media", MaxBodyMB: 8}
	r := router.New(cfg, db, slog.New(slog.NewTextHandler(io.Discard, nil)), "test")
	key := "r2/publications/genviral/videos/a/cover.jpg"
	created := doReq(t, r, "POST", "/v1/inspirations", cfg.Token, map[string]any{"type": "video", "title": "fixture", "source_url": "https://tiktok.com/@test/video/123", "cover_key": key})
	if created.Code != 201 {
		t.Fatalf("create: %d %s", created.Code, created.Body.String())
	}
	var create struct {
		Data model.Inspiration `json:"data"`
	}
	if err = json.Unmarshal(created.Body.Bytes(), &create); err != nil {
		t.Fatal(err)
	}
	id := strconv.FormatUint(create.Data.ID, 10)
	want := "/media/inspiration-covers/" + id
	if create.Data.CoverKey != want {
		t.Fatalf("create URL = %q", create.Data.CoverKey)
	}
	for _, method := range []string{"GET", "PATCH"} {
		var body any
		if method == "PATCH" {
			body = map[string]any{"cover_key": key}
		}
		w := doReq(t, r, method, "/v1/inspirations/"+id, cfg.Token, body)
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
		var got struct {
			Data model.Inspiration `json:"data"`
		}
		json.Unmarshal(w.Body.Bytes(), &got)
		if got.Data.CoverKey != want {
			t.Fatalf("%s URL = %q", method, got.Data.CoverKey)
		}
	}
	var saved model.Inspiration
	db.First(&saved, create.Data.ID)
	if saved.CoverKey != key {
		t.Fatal("decoration mutated database")
	}
	for _, token := range []string{"", "incorrect"} {
		w := doReq(t, r, "GET", want, token, nil)
		if w.Code != 401 {
			t.Fatalf("auth %q: %d", token, w.Code)
		}
	}
	if w := doReq(t, r, "GET", want, cfg.Token, nil); w.Code != 503 {
		t.Fatalf("unconfigured reader: %d %s", w.Code, w.Body.String())
	}
	if w := doReq(t, r, "GET", "/media/inspiration-covers/999", cfg.Token, nil); w.Code != 404 {
		t.Fatal(w.Code)
	}
	cfg.Token = ""
	r = router.New(cfg, db, slog.New(slog.NewTextHandler(io.Discard, nil)), "test")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", want, nil))
	if w.Code != 401 {
		t.Fatal("empty token permitted private route")
	}
}
