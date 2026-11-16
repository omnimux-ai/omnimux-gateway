package service

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"omnimux-inspiration/internal/errs"
	"omnimux-inspiration/internal/media"
	"omnimux-inspiration/internal/model"
	"omnimux-inspiration/internal/store"
)

type coverSaveResult struct {
	key string
	err error
}

type coverSaveCall struct {
	kind string
	url  string
}

type fakeCoverStore struct {
	results map[string]coverSaveResult
	calls   []coverSaveCall
}

type blockingCoverStore struct {
	started chan struct{}
	release chan struct{}
}

func (s *blockingCoverStore) SaveURL(ctx context.Context, _, _ string) (*media.Stored, error) {
	close(s.started)
	select {
	case <-s.release:
		return &media.Stored{Key: "covers/2026/09/unblocked.jpg"}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (f *fakeCoverStore) SaveURL(_ context.Context, kind, rawURL string) (*media.Stored, error) {
	f.calls = append(f.calls, coverSaveCall{kind: kind, url: rawURL})
	result, ok := f.results[rawURL]
	if !ok {
		return nil, errors.New("unexpected media URL")
	}
	if result.err != nil {
		return nil, result.err
	}
	return &media.Stored{Key: result.key}, nil
}

func newTestService(t *testing.T, covers CoverStore) (*Service, *store.DB) {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "service.sqlite"))
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if _, err := db.Migrate(""); err != nil {
		t.Fatalf("migrate test db: %v", err)
	}
	sqlDB, err := db.DB.DB()
	if err != nil {
		t.Fatalf("get sql db: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return New(db, "/media", covers), db
}

func assertMediaKeys(t *testing.T, rec *model.Inspiration, want []string) {
	t.Helper()
	var got []string
	if err := json.Unmarshal(rec.MediaKeys, &got); err != nil {
		t.Fatalf("decode media keys: %v", err)
	}
	if strings.Join(got, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("media keys = %v, want %v", got, want)
	}
}

func TestNormalizeURL(t *testing.T) {
	cases := []struct{ in, want string }{
		{" https://TikTok.com/@a/video/123/ ", "https://tiktok.com/@a/video/123"},
		{"https://example.com/x/?utm_source=tt&from=web", "https://example.com/x"},
		{"https://example.com/y/", "https://example.com/y"},
		{"https://example.com/z#frag", "https://example.com/z"},
		{"not a url at all", "not a url at all"},
	}
	for _, c := range cases {
		if got := NormalizeURL(c.in); got != c.want {
			t.Errorf("NormalizeURL(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestHashURLStable(t *testing.T) {
	a := HashURL("https://TikTok.com/@a/video/123/")
	b := HashURL("https://tiktok.com/@a/video/123")
	if a != b {
		t.Errorf("hash mismatch for normalized-equivalent URLs: %s vs %s", a, b)
	}
	if len(a) != 64 {
		t.Errorf("hash length = %d, want 64", len(a))
	}
}

func TestNormalizeNames(t *testing.T) {
	got := normalizeNames([]string{" b ", "", "a", "b", "  "})
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Errorf("normalizeNames = %v", got)
	}
	if strings.Join(got, ",") != "a,b" {
		t.Errorf("not sorted: %v", got)
	}
}

func TestCreateMaterializesExternalCover(t *testing.T) {
	const external = "https://cdn.example.com/cover.jpg"
	covers := &fakeCoverStore{results: map[string]coverSaveResult{
		external: {key: "covers/2026/09/saved.jpg"},
	}}
	svc, _ := newTestService(t, covers)

	rec, existed, err := svc.Create(t.Context(), &CreateInspirationReq{
		Type:       "image",
		Title:      "external cover",
		SourceURL:  "https://example.com/items/1",
		CoverKey:   external,
		MediaKeys:  []string{"files/existing.mp4"},
		IsFavorite: true,
	}, false)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if existed {
		t.Fatal("new record reported as existing")
	}
	if rec.CoverKey != "covers/2026/09/saved.jpg" {
		t.Fatalf("cover key = %q", rec.CoverKey)
	}
	assertMediaKeys(t, rec, []string{"files/existing.mp4"})
	if len(covers.calls) != 1 || covers.calls[0] != (coverSaveCall{kind: "cover", url: external}) {
		t.Fatalf("cover calls = %+v", covers.calls)
	}
	stored, err := svc.Get(t.Context(), rec.ID)
	if err != nil {
		t.Fatalf("get stored record: %v", err)
	}
	if stored.CoverKey != rec.CoverKey {
		t.Fatalf("stored cover key = %q, want %q", stored.CoverKey, rec.CoverKey)
	}
}

func TestCreateCoverFailureLeavesDatabaseUnchanged(t *testing.T) {
	const external = "https://cdn.example.com/broken.jpg"
	fetchErr := errors.New("fetch failed")
	covers := &fakeCoverStore{results: map[string]coverSaveResult{external: {err: fetchErr}}}
	svc, db := newTestService(t, covers)

	_, _, err := svc.Create(t.Context(), &CreateInspirationReq{
		Title: "broken cover", SourceURL: "https://example.com/items/broken", CoverKey: external,
	}, false)
	if !errors.Is(err, fetchErr) {
		t.Fatalf("create error = %v, want fetch error", err)
	}
	var count int64
	if err := db.Model(&model.Inspiration{}).Count(&count).Error; err != nil {
		t.Fatalf("count records: %v", err)
	}
	if count != 0 {
		t.Fatalf("record count = %d, want 0", count)
	}
}

func TestCreatePreservesKeysAndDeduplicatesBeforeCoverSave(t *testing.T) {
	covers := &fakeCoverStore{results: map[string]coverSaveResult{}}
	svc, _ := newTestService(t, covers)

	rec, _, err := svc.Create(t.Context(), &CreateInspirationReq{
		Title: "stored cover", SourceURL: "https://example.com/items/stored", CoverKey: "covers/2026/09/existing.jpg",
	}, false)
	if err != nil {
		t.Fatalf("create stored key: %v", err)
	}
	if rec.CoverKey != "covers/2026/09/existing.jpg" {
		t.Fatalf("stored key changed to %q", rec.CoverKey)
	}
	empty, _, err := svc.Create(t.Context(), &CreateInspirationReq{
		Title: "empty cover", SourceURL: "https://example.com/items/empty", CoverKey: "",
	}, false)
	if err != nil {
		t.Fatalf("create empty key: %v", err)
	}
	if empty.CoverKey != "" {
		t.Fatalf("empty cover changed to %q", empty.CoverKey)
	}

	duplicate, existed, err := svc.Create(t.Context(), &CreateInspirationReq{
		Title: "duplicate", SourceURL: "https://example.com/items/stored", CoverKey: "https://cdn.example.com/unused.jpg",
	}, true)
	if err != nil || !existed {
		t.Fatalf("return existing: existed=%v err=%v", existed, err)
	}
	if duplicate.ID != rec.ID || duplicate.CoverKey != rec.CoverKey {
		t.Fatalf("duplicate result = %+v, want original id=%d cover=%q", duplicate, rec.ID, rec.CoverKey)
	}
	if _, _, err := svc.Create(t.Context(), &CreateInspirationReq{
		Title: "duplicate", SourceURL: "https://example.com/items/stored", CoverKey: "https://cdn.example.com/unused.jpg",
	}, false); err != errs.ErrDuplicateURL {
		t.Fatalf("duplicate error = %v, want DUPLICATE_URL", err)
	}
	if len(covers.calls) != 0 {
		t.Fatalf("persistent/empty/duplicate covers triggered saves: %+v", covers.calls)
	}
}

func TestExternalLikeNonHTTPCoverIsRejected(t *testing.T) {
	covers := &fakeCoverStore{results: map[string]coverSaveResult{}}
	svc, db := newTestService(t, covers)
	for i, cover := range []string{"ftp://cdn.example.com/cover.jpg", "//cdn.example.com/cover.jpg", "https:/missing-host.jpg"} {
		_, _, err := svc.Create(t.Context(), &CreateInspirationReq{
			Title: "invalid cover", SourceURL: "https://example.com/invalid/" + string(rune('a'+i)), CoverKey: cover,
		}, false)
		if err != errs.ErrInvalidParam {
			t.Errorf("cover %q error = %v, want INVALID_PARAM", cover, err)
		}
	}
	var count int64
	if err := db.Model(&model.Inspiration{}).Count(&count).Error; err != nil {
		t.Fatalf("count records: %v", err)
	}
	if count != 0 || len(covers.calls) != 0 {
		t.Fatalf("invalid covers wrote records/called saver: count=%d calls=%+v", count, covers.calls)
	}
}

func TestUpdateMaterializesCoverAndFailureIsAtomic(t *testing.T) {
	const (
		external = "https://cdn.example.com/updated.jpg"
		broken   = "https://cdn.example.com/broken-update.jpg"
	)
	fetchErr := errors.New("fetch failed")
	covers := &fakeCoverStore{results: map[string]coverSaveResult{
		external: {key: "covers/2026/09/updated.jpg"},
		broken:   {err: fetchErr},
	}}
	svc, _ := newTestService(t, covers)
	rec, _, err := svc.Create(t.Context(), &CreateInspirationReq{
		Title: "before", SourceURL: "https://example.com/items/update",
		CoverKey: "covers/2026/09/before.jpg", MediaKeys: []string{"files/keep.mp4"}, Tags: []string{"original"},
	}, false)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	updated, err := svc.Update(t.Context(), rec.ID, &UpdateInspirationReq{CoverKey: ptr(external)})
	if err != nil {
		t.Fatalf("update external cover: %v", err)
	}
	if updated.CoverKey != "covers/2026/09/updated.jpg" {
		t.Fatalf("updated cover = %q", updated.CoverKey)
	}
	assertMediaKeys(t, updated, []string{"files/keep.mp4"})

	_, err = svc.Update(t.Context(), rec.ID, &UpdateInspirationReq{
		Title: ptr("must not persist"), CoverKey: ptr(broken), Tags: []string{"changed"},
	})
	if !errors.Is(err, fetchErr) {
		t.Fatalf("failed update error = %v, want fetch error", err)
	}
	stored, err := svc.Get(t.Context(), rec.ID)
	if err != nil {
		t.Fatalf("get after failed update: %v", err)
	}
	if stored.Title != "before" || stored.CoverKey != "covers/2026/09/updated.jpg" {
		t.Fatalf("failed update changed record: title=%q cover=%q", stored.Title, stored.CoverKey)
	}
	assertMediaKeys(t, stored, []string{"files/keep.mp4"})
	if len(stored.Tags) != 1 || stored.Tags[0] != "original" {
		t.Fatalf("failed update changed tags: %+v", stored.Tags)
	}
}

func TestBulkUpsertTxPropagatesCoverStore(t *testing.T) {
	const (
		good = "https://cdn.example.com/bulk-good.jpg"
		bad  = "https://cdn.example.com/bulk-bad.jpg"
	)
	covers := &fakeCoverStore{results: map[string]coverSaveResult{
		good: {key: "covers/2026/09/bulk.jpg"},
		bad:  {err: errors.New("fetch failed")},
	}}
	svc, db := newTestService(t, covers)
	result, err := svc.BulkUpsertTx(t.Context(), &BulkReq{Items: []CreateInspirationReq{
		{Title: "bulk good", SourceURL: "https://example.com/bulk/good", CoverKey: good},
		{Title: "bulk bad", SourceURL: "https://example.com/bulk/bad", CoverKey: bad},
	}})
	if err != nil {
		t.Fatalf("bulk: %v", err)
	}
	if result.Created != 1 || result.Skipped != 1 || result.Updated != 0 {
		t.Fatalf("bulk result = %+v", result)
	}
	if len(result.Items) != 2 || result.Items[0].Action != "created" || result.Items[1].Action != "skipped" {
		t.Fatalf("bulk item results = %+v", result.Items)
	}
	var records []model.Inspiration
	if err := db.Order("id").Find(&records).Error; err != nil {
		t.Fatalf("list bulk records: %v", err)
	}
	if len(records) != 1 || records[0].CoverKey != "covers/2026/09/bulk.jpg" {
		t.Fatalf("bulk records = %+v", records)
	}
	if len(covers.calls) != 2 {
		t.Fatalf("bulk cover calls = %+v", covers.calls)
	}
}

func TestBulkUpsertTxReusesPreparedCoverForDuplicateSource(t *testing.T) {
	const (
		firstCover  = "https://cdn.example.com/duplicate-first.jpg"
		secondCover = "https://cdn.example.com/duplicate-second.jpg"
	)
	covers := &fakeCoverStore{results: map[string]coverSaveResult{
		firstCover: {key: "covers/2026/09/duplicate.jpg"},
	}}
	svc, db := newTestService(t, covers)
	result, err := svc.BulkUpsertTx(t.Context(), &BulkReq{Items: []CreateInspirationReq{
		{Title: "duplicate first", SourceURL: "https://example.com/bulk/duplicate", CoverKey: firstCover},
		{Title: "duplicate second", SourceURL: "https://example.com/bulk/duplicate", CoverKey: secondCover},
	}})
	if err != nil {
		t.Fatalf("bulk: %v", err)
	}
	if result.Created != 1 || result.Updated != 1 || result.Skipped != 0 {
		t.Fatalf("bulk result = %+v", result)
	}
	if len(result.Items) != 2 || result.Items[0].Action != "created" || result.Items[1].Action != "updated" {
		t.Fatalf("bulk item results = %+v", result.Items)
	}
	if len(covers.calls) != 1 || covers.calls[0].url != firstCover {
		t.Fatalf("duplicate cover calls = %+v", covers.calls)
	}
	var records []model.Inspiration
	if err := db.Find(&records).Error; err != nil {
		t.Fatalf("list records: %v", err)
	}
	if len(records) != 1 || records[0].CoverKey != "covers/2026/09/duplicate.jpg" {
		t.Fatalf("records = %+v", records)
	}
}

func TestBulkUpsertTxExistingRecordSkipsCoverSave(t *testing.T) {
	covers := &fakeCoverStore{results: map[string]coverSaveResult{}}
	svc, _ := newTestService(t, covers)
	existing, _, err := svc.Create(t.Context(), &CreateInspirationReq{
		Title: "existing", SourceURL: "https://example.com/bulk/existing", CoverKey: "covers/2026/09/existing.jpg",
	}, false)
	if err != nil {
		t.Fatalf("seed existing record: %v", err)
	}
	result, err := svc.BulkUpsertTx(t.Context(), &BulkReq{Items: []CreateInspirationReq{{
		Title: "duplicate", SourceURL: "https://example.com/bulk/existing",
		CoverKey: "https://cdn.example.com/must-not-fetch.jpg",
	}}})
	if err != nil {
		t.Fatalf("bulk: %v", err)
	}
	if result.Updated != 1 || result.Created != 0 || result.Skipped != 0 || result.Items[0].ID != existing.ID {
		t.Fatalf("bulk result = %+v", result)
	}
	if len(covers.calls) != 0 {
		t.Fatalf("existing record triggered cover save: %+v", covers.calls)
	}
}

func TestBulkUpsertTxRetriesDuplicateAfterCoverFailure(t *testing.T) {
	const (
		firstCover  = "https://cdn.example.com/retry-first.jpg"
		secondCover = "https://cdn.example.com/retry-second.jpg"
	)
	covers := &fakeCoverStore{results: map[string]coverSaveResult{
		firstCover:  {err: errors.New("fetch failed")},
		secondCover: {key: "covers/2026/09/retried.jpg"},
	}}
	svc, db := newTestService(t, covers)
	result, err := svc.BulkUpsertTx(t.Context(), &BulkReq{Items: []CreateInspirationReq{
		{Title: "retry first", SourceURL: "https://example.com/bulk/retry", CoverKey: firstCover},
		{Title: "retry second", SourceURL: "https://example.com/bulk/retry", CoverKey: secondCover},
	}})
	if err != nil {
		t.Fatalf("bulk: %v", err)
	}
	if result.Created != 1 || result.Updated != 0 || result.Skipped != 1 {
		t.Fatalf("bulk result = %+v", result)
	}
	if len(result.Items) != 2 || result.Items[0].Action != "skipped" || result.Items[1].Action != "created" {
		t.Fatalf("bulk item results = %+v", result.Items)
	}
	if len(covers.calls) != 2 || covers.calls[0].url != firstCover || covers.calls[1].url != secondCover {
		t.Fatalf("retry cover calls = %+v", covers.calls)
	}
	var records []model.Inspiration
	if err := db.Find(&records).Error; err != nil {
		t.Fatalf("list records: %v", err)
	}
	if len(records) != 1 || records[0].Title != "retry second" || records[0].CoverKey != "covers/2026/09/retried.jpg" {
		t.Fatalf("records = %+v", records)
	}
}

func TestBulkUpsertTxUsesDBResultAfterPreparedInsertFailure(t *testing.T) {
	const (
		firstCover  = "https://cdn.example.com/insert-first.jpg"
		secondCover = "https://cdn.example.com/insert-second.jpg"
	)
	covers := &fakeCoverStore{results: map[string]coverSaveResult{
		firstCover: {key: "covers/2026/09/reused-after-insert-failure.jpg"},
	}}
	svc, db := newTestService(t, covers)
	if err := db.Exec(`CREATE TRIGGER fail_first_bulk_insert
		BEFORE INSERT ON inspirations
		WHEN NEW.title = 'force insert failure'
		BEGIN SELECT RAISE(FAIL, 'forced insert failure'); END`).Error; err != nil {
		t.Fatalf("create failure trigger: %v", err)
	}
	result, err := svc.BulkUpsertTx(t.Context(), &BulkReq{Items: []CreateInspirationReq{
		{Title: "force insert failure", SourceURL: "https://example.com/bulk/insert-retry", CoverKey: firstCover},
		{Title: "insert succeeds", SourceURL: "https://example.com/bulk/insert-retry", CoverKey: secondCover},
	}})
	if err != nil {
		t.Fatalf("bulk: %v", err)
	}
	if result.Created != 1 || result.Updated != 0 || result.Skipped != 1 {
		t.Fatalf("bulk result = %+v", result)
	}
	if len(result.Items) != 2 || result.Items[0].Action != "skipped" || result.Items[1].Action != "created" {
		t.Fatalf("bulk item results = %+v", result.Items)
	}
	if len(covers.calls) != 1 || covers.calls[0].url != firstCover {
		t.Fatalf("prepared cover calls = %+v", covers.calls)
	}
	var records []model.Inspiration
	if err := db.Find(&records).Error; err != nil {
		t.Fatalf("list records: %v", err)
	}
	if len(records) != 1 || records[0].Title != "insert succeeds" || records[0].CoverKey != "covers/2026/09/reused-after-insert-failure.jpg" {
		t.Fatalf("records = %+v", records)
	}
}

func TestBulkUpsertTxDoesNotHoldDBConnectionWhileSavingCover(t *testing.T) {
	covers := &blockingCoverStore{started: make(chan struct{}), release: make(chan struct{})}
	svc, db := newTestService(t, covers)
	type bulkOutcome struct {
		result *BulkResult
		err    error
	}
	done := make(chan bulkOutcome, 1)
	go func() {
		result, err := svc.BulkUpsertTx(context.Background(), &BulkReq{Items: []CreateInspirationReq{{
			Title: "blocking cover", SourceURL: "https://example.com/bulk/blocking",
			CoverKey: "https://cdn.example.com/blocking.jpg",
		}}})
		done <- bulkOutcome{result: result, err: err}
	}()

	select {
	case <-covers.started:
	case <-time.After(time.Second):
		t.Fatal("cover save did not start")
	}
	pingCtx, cancelPing := context.WithTimeout(context.Background(), 300*time.Millisecond)
	pingErr := db.Ping(pingCtx)
	cancelPing()
	close(covers.release)

	var outcome bulkOutcome
	select {
	case outcome = <-done:
	case <-time.After(time.Second):
		t.Fatal("bulk did not finish after cover save was released")
	}
	if pingErr != nil {
		t.Fatalf("database ping blocked while cover save waited: %v", pingErr)
	}
	if outcome.err != nil || outcome.result.Created != 1 {
		t.Fatalf("bulk outcome = result=%+v err=%v", outcome.result, outcome.err)
	}
}

func ptr[T any](value T) *T { return &value }
