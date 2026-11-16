package api

import (
	"bytes"
	"context"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"omnimux-inspiration/internal/media"
	"omnimux-inspiration/internal/service"
	"omnimux-inspiration/internal/store"
)

type fakeCoverReader struct{ calls int }

func (f *fakeCoverReader) Read(context.Context, string) (*media.CoverImage, error) {
	f.calls++
	return &media.CoverImage{Data: []byte("0123456789"), ContentType: "image/png", ETag: "\"same\"", LastModified: time.Unix(1700000000, 0)}, nil
}
func TestCoverResponses(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "covers.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB.DB()
	t.Cleanup(func() { sqlDB.Close() })
	if _, err := db.Migrate("../../migrations"); err != nil {
		t.Fatal(err)
	}
	svc := service.New(db, "/media", nil)
	row, _, err := svc.Create(t.Context(), &service.CreateInspirationReq{Type: "video", Title: "fixture", SourceURL: "https://tiktok.com/@fixture/video/145", CoverKey: "r2/publications/a/cover.png"}, false)
	if err != nil {
		t.Fatal(err)
	}
	reader := &fakeCoverReader{}
	h := New(svc, nil, "/media")
	h.Covers = reader
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.GET("/media/*key", func(c *gin.Context) { h.Cover(c, c.Param("key")[1:]) })
	for _, tt := range []struct {
		name, rangeHeader, ifRange string
		status                     int
		body                       string
	}{{"full", "", "", 200, "0123456789"}, {"partial", "bytes=2-4", "", 206, "234"}, {"match", "bytes=2-4", "\"same\"", 206, "234"}, {"mismatch", "bytes=2-4", "\"different\"", 200, "0123456789"}, {"invalid", "bytes=99-", "", 416, ""}} {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/media/1", nil)
			req.Header.Set("Range", tt.rangeHeader)
			req.Header.Set("If-Range", tt.ifRange)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != tt.status {
				t.Fatalf("%d: %s", w.Code, w.Body.String())
			}
			if tt.body != "" && !bytes.Equal(w.Body.Bytes(), []byte(tt.body)) {
				t.Fatal(w.Body.String())
			}
		})
	}
	before := reader.calls
	for _, id := range []string{"0", "-1", "01", "x", "999"} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", "/media/"+id, nil))
		if w.Code != 400 && w.Code != 404 {
			t.Fatal(w.Code)
		}
	}
	if reader.calls != before {
		t.Fatal("invalid record reached storage")
	}
	if err := db.Exec("UPDATE inspirations SET deleted_at=CURRENT_TIMESTAMP WHERE id=?", row.ID).Error; err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/media/1", nil))
	if w.Code != 404 || reader.calls != before {
		t.Fatalf("deleted record: %d", w.Code)
	}
}
