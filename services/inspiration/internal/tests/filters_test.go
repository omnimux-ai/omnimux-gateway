package tests

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	mysql "github.com/go-sql-driver/mysql"
	"omnimux-inspiration/internal/config"
	"omnimux-inspiration/internal/router"
	"omnimux-inspiration/internal/testutil"
)

func newFilterServer(t *testing.T) *gin.Engine {
	t.Helper()
	db := testutil.OpenDB(t)
	return router.New(&config.Config{Token: "test-token", MediaDir: t.TempDir(), MediaURLPrefix: "/media", MaxBodyMB: 8}, db, slog.New(slog.NewTextHandler(io.Discard, nil)), "test")
}

func responseData(t *testing.T, w *httptest.ResponseRecorder, status int) map[string]any {
	t.Helper()
	if w.Code != status {
		t.Fatalf("status=%d want=%d body=%s", w.Code, status, w.Body.String())
	}
	var envelope struct {
		Success bool           `json:"success"`
		Data    map[string]any `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if !envelope.Success {
		t.Fatalf("unsuccessful response: %s", w.Body.String())
	}
	return envelope.Data
}

func TestFiltersMySQLNonUTC(t *testing.T) {
	dsn := os.Getenv("INSPIRATION_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("requires dedicated MySQL test DSN")
	}
	cfg, err := mysql.ParseDSN(strings.TrimPrefix(dsn, "mysql://"))
	if err != nil {
		t.Fatal(err)
	}
	cfg.Loc, err = time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("INSPIRATION_TEST_MYSQL_DSN", cfg.FormatDSN())
	t.Run("CRUD", TestFilterFieldsCRUD)
	t.Run("filters", TestMultidimensionalFilters)
}

func TestFilterFieldsCRUD(t *testing.T) {
	r := newFilterServer(t)
	body := map[string]any{"title": "fields", "source_url": "https://example.com/fields", "country_code": "US", "category": "厨房用品", "duration": 15, "views": int64(5000000000), "traffic_type": "ad", "posted_at": "2026-09-09T20:30:45.123987+08:00"}
	data := responseData(t, doReq(t, r, "POST", "/v1/inspirations", "test-token", body), 201)
	path := fmt.Sprintf("/v1/inspirations/%.0f", data["id"])
	want := map[string]any{"country_code": "US", "category": "厨房用品", "duration": float64(15), "views": float64(5000000000), "traffic_type": "ad", "posted_at": "2026-09-09T12:30:45.123Z"}
	assertFields := func(data map[string]any) {
		t.Helper()
		for key, value := range want {
			if !reflect.DeepEqual(data[key], value) {
				t.Errorf("%s=%v want=%v", key, data[key], value)
			}
		}
	}
	assertFields(data)
	assertFields(responseData(t, doReq(t, r, "GET", path, "test-token", nil), 200))
	assertFields(responseData(t, doReq(t, r, "PATCH", path, "test-token", map[string]any{"title": "renamed"}), 200))
	want = map[string]any{"country_code": "GB", "category": "beauty_skincare", "duration": float64(30), "views": float64(100), "traffic_type": "organic", "posted_at": "2026-09-10T00:00:00Z"}
	assertFields(responseData(t, doReq(t, r, "PATCH", path, "test-token", want), 200))
	want = map[string]any{"country_code": "", "category": "", "duration": float64(0), "views": float64(0), "traffic_type": "organic", "posted_at": nil}
	assertFields(responseData(t, doReq(t, r, "PATCH", path, "test-token", want), 200))
	assertFields(responseData(t, doReq(t, r, "GET", path, "test-token", nil), 200))
	legacy := responseData(t, doReq(t, r, "POST", "/v1/inspirations", "test-token", map[string]any{"title": "legacy", "source_url": "https://example.com/legacy"}), 201)
	assertFields(legacy)
}

func TestMultidimensionalFilters(t *testing.T) {
	r := newFilterServer(t)
	fixtures := []map[string]any{
		{"country_code": "US", "category": "厨房用品", "duration": 15, "views": 100000, "traffic_type": "ad", "posted_at": "2026-09-09T00:00:00Z", "hot_score": 9, "tags": []string{"demo"}, "is_favorite": true},
		{"country_code": "US", "category": "beauty_skincare", "duration": 30, "views": 1000000, "traffic_type": "organic", "posted_at": "2026-09-09T23:59:59.999Z", "hot_score": 7},
		{"country_code": "GB", "category": "厨房用品", "duration": 61, "views": 1000000, "traffic_type": "ad", "posted_at": "2026-09-10T00:00:00Z", "hot_score": 5},
		{"hot_score": 3},
		{"country_code": "US", "category": "厨房用品", "duration": 15, "views": 100000, "traffic_type": "ad", "posted_at": "2026-09-09T12:00:00Z", "hot_score": 1},
	}
	for i, body := range fixtures {
		body["title"] = fmt.Sprintf("item%d", i+1)
		body["source_url"] = fmt.Sprintf("https://example.com/filter/%d", i)
		body["type"] = "video"
		data := responseData(t, doReq(t, r, "POST", "/v1/inspirations", "test-token", body), 201)
		if i == 4 {
			responseData(t, doReq(t, r, "DELETE", fmt.Sprintf("/v1/inspirations/%.0f", data["id"]), "test-token", nil), 200)
		}
	}
	cases := []struct {
		name, query string
		want        []string
		total       int
	}{
		{"default", "", []string{"item1", "item2", "item3", "item4"}, 4},
		{"country", "country=US", []string{"item1", "item2"}, 2},
		{"category", "category=" + url.QueryEscape("厨房用品"), []string{"item1", "item3"}, 2},
		{"duration upper", "duration_max=15", []string{"item1", "item4"}, 2},
		{"duration lower", "duration_min=30", []string{"item2", "item3"}, 2},
		{"duration exact", "duration_min=15&duration_max=15", []string{"item1"}, 1},
		{"zero duration", "duration_max=0", []string{"item4"}, 1},
		{"views lower", "views_min=1000000", []string{"item2", "item3"}, 2},
		{"views upper", "views_max=100000", []string{"item1", "item4"}, 2},
		{"views exact", "views_min=100000&views_max=100000", []string{"item1"}, 1},
		{"zero views", "views_max=0", []string{"item4"}, 1},
		{"traffic", "traffic_type=organic", []string{"item2", "item4"}, 2},
		{"posted lower", "posted_after=2026-09-10", []string{"item3"}, 1},
		{"posted upper includes day", "posted_before=2026-09-09", []string{"item1", "item2"}, 2},
		{"posted same day", "posted_after=2026-09-09&posted_before=2026-09-09", []string{"item1", "item2"}, 2},
		{"posted timestamp inclusive", "posted_after=2026-09-09T00:00:00Z&posted_before=2026-09-09T00:00:00Z", []string{"item1"}, 1},
		{"timezone equivalent", "posted_after=" + url.QueryEscape("2026-09-09T08:00:00+08:00") + "&posted_before=" + url.QueryEscape("2026-09-09T08:00:00+08:00"), []string{"item1"}, 1},
		{"issue example", "country=US&duration_max=15&views_min=100000&traffic_type=ad", []string{"item1"}, 1},
		{"all filters and existing", "country=US&category=" + url.QueryEscape("厨房用品") + "&duration_min=15&duration_max=15&views_min=100000&views_max=100000&traffic_type=ad&posted_after=2026-09-09&posted_before=2026-09-09&type=video&tag=demo&is_favorite=true", []string{"item1"}, 1},
		{"views sort tie breaker", "sort=views", []string{"item3", "item2", "item1", "item4"}, 4},
		{"filtered pagination", "country=US&sort=views&page=2&page_size=1", []string{"item1"}, 2},
		{"no matches", "country=CA", []string{}, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data := responseData(t, doReq(t, r, "GET", "/v1/inspirations?"+tc.query, "test-token", nil), 200)
			got := []string{}
			for _, item := range data["items"].([]any) {
				got = append(got, item.(map[string]any)["title"].(string))
			}
			if !reflect.DeepEqual(got, tc.want) || data["total"] != float64(tc.total) {
				t.Fatalf("items=%v total=%v want=%v total=%d", got, data["total"], tc.want, tc.total)
			}
		})
	}
}

func TestFilterValidation(t *testing.T) {
	r := newFilterServer(t)
	for _, query := range []string{"duration_min=-1", "duration_max=abc", "duration_max=", "duration_min=1.5", "duration_max=4294967296", "duration_min=16&duration_max=15", "views_min=-1", "views_max=no", "views_min=2&views_max=1", "views_max=9223372036854775808", "traffic_type=paid", "posted_after=no", "posted_before=2026-02-30", "posted_after=2026-09-11&posted_before=2026-09-09", "posted_after=2026-09-10T00:00:00Z&posted_before=2026-09-09", "country=" + strings.Repeat("x", 9), "category=" + strings.Repeat("x", 65)} {
		t.Run(query, func(t *testing.T) {
			w := doReq(t, r, "GET", "/v1/inspirations?"+query, "test-token", nil)
			if w.Code != 400 || !strings.Contains(w.Body.String(), "INVALID_PARAM") {
				t.Fatalf("%d %s", w.Code, w.Body.String())
			}
		})
	}
	data := responseData(t, doReq(t, r, "POST", "/v1/inspirations", "test-token", map[string]any{"title": "keep", "source_url": "https://example.com/keep"}), 201)
	path := fmt.Sprintf("/v1/inspirations/%.0f", data["id"])
	for i, invalid := range []map[string]any{{"duration": -1}, {"duration": 4294967296}, {"views": -1}, {"views": 1.5}, {"traffic_type": "paid"}, {"country_code": strings.Repeat("x", 9)}, {"category": strings.Repeat("界", 65)}, {"posted_at": "not-date"}, {"posted_at": "0001-01-01T00:00:00Z"}} {
		for _, method := range []string{"POST", "PATCH"} {
			t.Run(fmt.Sprintf("%s%d", method, i), func(t *testing.T) {
				body := map[string]any{"title": "must not persist", "source_url": fmt.Sprintf("https://example.com/invalid/%d", i)}
				for k, v := range invalid {
					body[k] = v
				}
				target := "/v1/inspirations"
				if method == "PATCH" {
					target = path
				}
				w := doReq(t, r, method, target, "test-token", body)
				if w.Code != 400 || !strings.Contains(w.Body.String(), "INVALID_PARAM") {
					t.Fatalf("%d %s", w.Code, w.Body.String())
				}
			})
		}
	}
	if got := responseData(t, doReq(t, r, "GET", path, "test-token", nil), 200); got["title"] != "keep" {
		t.Fatalf("invalid PATCH mutated record: %v", got)
	}
}

func TestBulkPreservesFilterFields(t *testing.T) {
	r := newFilterServer(t)
	body := map[string]any{"items": []map[string]any{{"title": "bulk", "source_url": "https://example.com/bulk-filters", "country_code": "US", "category": "厨房用品", "duration": 15, "views": 100000, "traffic_type": "ad", "posted_at": "2026-09-09T00:00:00Z"}}}
	data := responseData(t, doReq(t, r, "POST", "/v1/inspirations:bulk", "test-token", body), 200)
	if data["created"] != float64(1) {
		t.Fatalf("bulk=%v", data)
	}
	listed := responseData(t, doReq(t, r, "GET", "/v1/inspirations?country=US&duration_max=15&views_min=100000&traffic_type=ad&posted_before=2026-09-09&category="+url.QueryEscape("厨房用品"), "test-token", nil), 200)
	if listed["total"] != float64(1) {
		t.Fatalf("bulk lost filter fields: %v", listed)
	}
}
