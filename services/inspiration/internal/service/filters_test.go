package service

import (
	"encoding/json"
	"math"
	"testing"
	"time"

	"omnimux-inspiration/internal/errs"
)

func TestFilterServiceValidation(t *testing.T) {
	svc, _ := newTestService(t, nil)
	for _, req := range []CreateInspirationReq{
		{Duration: -1}, {Duration: int(math.MaxUint32) + 1}, {Views: -1}, {TrafficType: "paid"}, {CountryCode: "123456789"},
	} {
		req.Title = "invalid"
		req.SourceURL = "https://example.com/invalid"
		if _, _, err := svc.Create(t.Context(), &req, false); err != errs.ErrInvalidParam {
			t.Errorf("Create(%+v) error=%v", req, err)
		}
	}
	min, max := 2, 1
	viewsMin, viewsMax := int64(2), int64(1)
	for _, q := range []ListQuery{{DurationMin: &min, DurationMax: &max}, {ViewsMin: &viewsMin, ViewsMax: &viewsMax}, {TrafficType: "paid"}} {
		if _, err := svc.List(t.Context(), &q); err != errs.ErrInvalidParam {
			t.Errorf("List(%+v) error=%v", q, err)
		}
	}
	rec, _, err := svc.Create(t.Context(), &CreateInspirationReq{Title: "valid", SourceURL: "https://example.com/valid"}, false)
	if err != nil {
		t.Fatal(err)
	}
	empty := ""
	if _, err := svc.Update(t.Context(), rec.ID, &UpdateInspirationReq{TrafficType: &empty}); err != errs.ErrInvalidParam {
		t.Errorf("empty traffic update error=%v", err)
	}
}

func TestPostedBoundParsing(t *testing.T) {
	for _, tc := range []struct {
		raw   string
		upper bool
		want  string
	}{
		{"2026-09-09", false, "2026-09-09T00:00:00Z"},
		{"2026-09-09", true, "2026-09-09T23:59:59.999Z"},
		{"2026-09-09T08:30:00+08:00", false, "2026-09-09T00:30:00Z"},
		{"2026-09-09T00:00:00.123456Z", true, "2026-09-09T00:00:00.123456Z"},
	} {
		got, err := ParsePostedBound(tc.raw, tc.upper)
		if err != nil {
			t.Fatal(err)
		}
		if got.Format(time.RFC3339Nano) != tc.want {
			t.Errorf("%s => %s want %s", tc.raw, got.Format(time.RFC3339Nano), tc.want)
		}
	}
	for _, raw := range []string{"", "2026-02-30", "2026-09-09T12:00:00", "0001-01-01", "9999-12-31T23:00:00-08:00"} {
		if _, err := ParsePostedBound(raw, false); err != errs.ErrInvalidParam {
			t.Errorf("%q error=%v", raw, err)
		}
	}
}

func TestOptionalPostedAt(t *testing.T) {
	for _, tc := range []struct {
		raw           string
		set, hasValue bool
	}{
		{`{}`, false, false}, {`{"posted_at":null}`, true, false}, {`{"posted_at":"2026-09-09T00:00:00Z"}`, true, true},
	} {
		var req UpdateInspirationReq
		if err := json.Unmarshal([]byte(tc.raw), &req); err != nil {
			t.Fatal(err)
		}
		if req.PostedAt.Set != tc.set || (req.PostedAt.Value != nil) != tc.hasValue {
			t.Fatalf("%s => %+v", tc.raw, req.PostedAt)
		}
	}
}
