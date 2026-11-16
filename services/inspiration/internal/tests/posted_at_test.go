package tests

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	mysql "github.com/go-sql-driver/mysql"
)

func TestPostedAtMySQLDST(t *testing.T) {
	dsn := os.Getenv("INSPIRATION_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("requires dedicated MySQL test DSN")
	}
	cfg, err := mysql.ParseDSN(strings.TrimPrefix(dsn, "mysql://"))
	if err != nil {
		t.Fatal(err)
	}
	cfg.Loc, err = time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("INSPIRATION_TEST_MYSQL_DSN", cfg.FormatDSN())
	r := newFilterServer(t)
	for i, instant := range []string{"2026-11-01T05:30:00Z", "2026-11-01T06:30:00Z", "2026-03-08T02:30:00Z", "1000-01-01T00:00:00Z", "9999-12-31T23:59:59.999Z"} {
		body := map[string]any{"title": fmt.Sprint(i), "source_url": fmt.Sprintf("https://example.com/dst/%d", i), "posted_at": instant}
		created := responseData(t, doReq(t, r, "POST", "/v1/inspirations", "test-token", body), 201)
		path := fmt.Sprintf("/v1/inspirations/%.0f", created["id"])
		for _, method := range []string{"GET", "PATCH"} {
			var patch any
			if method == "PATCH" {
				patch = map[string]any{"posted_at": instant}
			}
			got := responseData(t, doReq(t, r, method, path, "test-token", patch), 200)
			if got["posted_at"] != instant {
				t.Errorf("%s posted_at=%v want=%s", method, got["posted_at"], instant)
			}
		}
		query := "/v1/inspirations?posted_after=" + url.QueryEscape(instant) + "&posted_before=" + url.QueryEscape(instant)
		got := responseData(t, doReq(t, r, "GET", query, "test-token", nil), 200)
		if got["total"] != float64(1) {
			t.Errorf("instant %s matched %v rows, want 1", instant, got["total"])
		}
		// Dedup reads must use the same UTC decoding as Get/List/Update.
		duplicate := responseData(t, doReq(t, r, "POST", "/v1/inspirations?return_existing=true", "test-token", body), 200)
		if duplicate["posted_at"] != instant {
			t.Errorf("duplicate posted_at=%v want=%s", duplicate["posted_at"], instant)
		}
	}
}

func TestPostedAtSubmillisecondBounds(t *testing.T) {
	r := newFilterServer(t)
	responseData(t, doReq(t, r, "POST", "/v1/inspirations", "test-token", map[string]any{"title": "precision", "source_url": "https://example.com/precision", "posted_at": "2026-09-09T00:00:00.123Z"}), 201)
	for _, tc := range []struct {
		query string
		total float64
	}{
		{"posted_after=2026-09-09T00:00:00.123000001Z", 0},
		{"posted_before=2026-09-09T00:00:00.123000001Z", 1},
		{"posted_after=2026-09-09T00:00:00.122999999Z", 1},
		{"posted_before=2026-09-09T00:00:00.122999999Z", 0},
		{"posted_after=9999-12-31T23:59:59.999000001Z", 0},
	} {
		got := responseData(t, doReq(t, r, "GET", "/v1/inspirations?"+tc.query, "test-token", nil), 200)
		if got["total"] != tc.total {
			t.Errorf("%s total=%v want=%v", tc.query, got["total"], tc.total)
		}
	}
}
