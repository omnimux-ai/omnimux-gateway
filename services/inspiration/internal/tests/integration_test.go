// Package tests 端到端 HTTP 集成测试（SQLite 内存库）。
package tests

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"omnimux-inspiration/internal/config"
	"omnimux-inspiration/internal/media"
	"omnimux-inspiration/internal/model"
	"omnimux-inspiration/internal/router"
	"omnimux-inspiration/internal/service"
	"omnimux-inspiration/internal/store"
)

func newTestServer(t *testing.T) *gin.Engine {
	t.Helper()
	db, err := store.Open("file::memory:?cache=shared")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	db.Exec("PRAGMA journal_mode=WAL")
	if _, err := db.Migrate("../../migrations"); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	cfg := &config.Config{
		Token: "test-token", MediaDir: "/tmp/omnimux-inspiration-test-media",
		MediaURLPrefix: "/media", MaxBodyMB: 8,
	}
	s := service.New(db, "/media", media.New(cfg))
	if _, _, err := s.Create(t.Context(), &service.CreateInspirationReq{
		Type: "video", Title: "电影感人像教程", Content: "灯光运镜调色",
		SourceURL: "https://tiktok.com/@demo/video/1", Tags: []string{"爆款拆解"},
	}, true); err != nil {
		t.Fatalf("seed: %v", err)
	}
	return router.New(cfg, db, slog.New(slog.NewJSONHandler(io.Discard, &slog.HandlerOptions{})), "test")
}

func doReq(t *testing.T, r *gin.Engine, method, path, token string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		payload, _ := json.Marshal(body)
		buf.Write(payload)
	}
	req := httptest.NewRequest(method, path, &buf)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestHealthz(t *testing.T) {
	r := newTestServer(t)
	if w := doReq(t, r, "GET", "/healthz", "", nil); w.Code != http.StatusOK {
		t.Fatalf("healthz = %d", w.Code)
	}
}

func TestUnauthorized(t *testing.T) {
	r := newTestServer(t)
	if w := doReq(t, r, "GET", "/v1/inspirations", "", nil); w.Code != http.StatusUnauthorized {
		t.Fatalf("unauth = %d, want 401", w.Code)
	}
}

func TestListAndSearch(t *testing.T) {
	r := newTestServer(t)
	w := doReq(t, r, "GET", "/v1/inspirations?q=电影", "test-token", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list = %d, body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Success bool `json:"success"`
		Data    struct {
			Total int64               `json:"total"`
			Items []model.Inspiration `json:"items"`
		} `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Data.Total < 1 {
		t.Fatalf("total=%d", resp.Data.Total)
	}
	if len(resp.Data.Items) > 0 && resp.Data.Items[0].Title != "电影感人像教程" {
		t.Fatalf("item=%+v", resp.Data.Items[0])
	}
}

func TestCreateDedup(t *testing.T) {
	r := newTestServer(t)
	body := map[string]any{"type": "video", "title": "第二个", "source_url": "https://tiktok.com/@demo/video/2"}
	w := doReq(t, r, "POST", "/v1/inspirations", "test-token", body)
	if w.Code != http.StatusCreated {
		t.Fatalf("create = %d, body=%s", w.Code, w.Body.String())
	}
	// 默认去重应返回 409 DUPLICATE_URL
	w2 := doReq(t, r, "POST", "/v1/inspirations", "test-token", body)
	if w2.Code != http.StatusConflict {
		t.Fatalf("dedup default = %d, want 409", w2.Code)
	}
	// 带 return_existing=true 应返回 200 + existed
	w3 := doReq(t, r, "POST", "/v1/inspirations?return_existing=true", "test-token", body)
	if w3.Code != http.StatusOK {
		t.Fatalf("dedup return_existing = %d, want 200", w3.Code)
	}
	var resp struct {
		Existed bool `json:"existed"`
	}
	_ = json.Unmarshal(w3.Body.Bytes(), &resp)
	if !resp.Existed {
		t.Fatalf("expected existed=true")
	}
}

func TestTagCRUD(t *testing.T) {
	r := newTestServer(t)
	w := doReq(t, r, "POST", "/v1/tags", "test-token", map[string]string{"name": "新标签"})
	if w.Code != http.StatusCreated {
		t.Fatalf("tag create = %d", w.Code)
	}
	if w := doReq(t, r, "GET", "/v1/tags", "test-token", nil); w.Code != http.StatusOK {
		t.Fatalf("tag list = %d", w.Code)
	}
}

func TestCreateValidate(t *testing.T) {
	r := newTestServer(t)
	w := doReq(t, r, "POST", "/v1/inspirations", "test-token", map[string]any{"title": ""})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("validate = %d, want 400", w.Code)
	}
}

func TestUploadMedia(t *testing.T) {
	r := newTestServer(t)
	// 1. 私网 URL 在发起请求前拒绝
	bad := doReq(t, r, "POST", "/v1/media", "test-token", map[string]any{"url": "http://127.0.0.1:1/not-image.txt"})
	if bad.Code != http.StatusBadRequest {
		t.Fatalf("private URL upload status = %d, want 400", bad.Code)
	}

	// 2. 本地合法 JPEG 写入
	// 最小 12+ 字节 JPEG header
	fakeJPEG := []byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00, 0x01, 0x01, 0x01, 0x00, 0x48}
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("file", "cover.jpg")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	_, _ = fw.Write(fakeJPEG)
	_ = mw.Close()

	req := httptest.NewRequest("POST", "/v1/media", &buf)
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("upload multipart = %d, body=%s", w.Code, w.Body.String())
	}
	var res struct {
		Success bool `json:"success"`
		Data    struct {
			Key  string `json:"key"`
			URL  string `json:"url"`
			Size int64  `json:"size"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("parse res: %v", err)
	}
	if !res.Success || !strings.HasPrefix(res.Data.Key, "covers/") || !strings.HasSuffix(res.Data.Key, ".jpg") {
		t.Fatalf("unexpected res data: %+v", res.Data)
	}

	// 3. GET /media/:key 可以读出来并带安全隔离响应头
	getReq := httptest.NewRequest("GET", res.Data.URL, nil)
	getW := httptest.NewRecorder()
	r.ServeHTTP(getW, getReq)
	if getW.Code != http.StatusOK {
		t.Fatalf("get uploaded media = %d", getW.Code)
	}
	if getW.Body.Len() != len(fakeJPEG) {
		t.Fatalf("uploaded bytes mismatch: %d != %d", getW.Body.Len(), len(fakeJPEG))
	}
	if got := getW.Header().Get("Content-Security-Policy"); got != "sandbox; default-src 'none'" {
		t.Fatalf("missing sandbox CSP on media response: %q", got)
	}
	if got := getW.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("missing nosniff header on media response: %q", got)
	}

	// 4. 目录访问必须拒绝，不允许列目录
	dirW := httptest.NewRecorder()
	r.ServeHTTP(dirW, httptest.NewRequest("GET", "/media/covers", nil))
	if dirW.Code != http.StatusNotFound {
		t.Fatalf("directory access status = %d, want 404", dirW.Code)
	}

	// 5. 非 dev 环境空 token 必须 Fail-Closed (401)
	db, err := store.Open("file::memory:?cache=shared")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	prodRouter := router.New(&config.Config{
		Env:            "prod",
		Token:          "",
		MediaDir:       t.TempDir(),
		MediaURLPrefix: "/media",
		MaxBodyMB:      8,
	}, db, slog.New(slog.NewJSONHandler(io.Discard, &slog.HandlerOptions{})), "test")
	unauthW := doReq(t, prodRouter, "GET", "/v1/inspirations", "", nil)
	if unauthW.Code != http.StatusUnauthorized {
		t.Fatalf("prod empty token status = %d, want 401", unauthW.Code)
	}
}
