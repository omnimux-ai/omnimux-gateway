package service

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"

	"omnimux-inspiration/internal/store"
)

type repairTestReader struct{ data []byte }

func (r repairTestReader) Read(_ context.Context, _ string) (*CoverObject, error) {
	return &CoverObject{Data: append([]byte(nil), r.data...), ContentType: "image/jpeg", ETag: "fixture"}, nil
}

func TestCoverRepairPlanDigestAndVerification(t *testing.T) {
	plan := repairTestPlan(t)
	if err := ValidateCoverRepairPlan(plan); err != nil {
		t.Fatalf("validate plan: %v", err)
	}
	verification, err := VerifyCoverRepairPlan(t.Context(), plan, repairTestReader{data: []byte("fixture image")})
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if len(verification.Items) != 10 {
		t.Fatalf("verified items = %d", len(verification.Items))
	}
	tampered := *plan
	tampered.OperationID = "tampered"
	if err := ValidateCoverRepairPlan(&tampered); err == nil {
		t.Fatal("tampered plan digest was accepted")
	}
}

func TestCoverRepairIndentedPlanRoundTrip(t *testing.T) {
	plan := repairTestPlan(t)
	rows := make([]InspirationSnapshot, len(plan.Items))
	tags := make(map[uint64][]TagAssociationSnapshot)
	for i := range plan.Items {
		plan.Items[i].Before.Analysis = DBJSON{Value: json.RawMessage(`{"nested":{"count":9007199254740993,"values":[1,2]}}`)}
		plan.Items[i].Before.MediaKeys = DBJSON{Value: json.RawMessage(`["one","two"]`)}
		rows[i] = plan.Items[i].Before
		tags[rows[i].ID] = plan.Items[i].Tags
	}
	var err error
	plan.Digest, err = planDigest(plan)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	var loaded CoverRepairPlan
	if err := json.Unmarshal(raw, &loaded); err != nil {
		t.Fatal(err)
	}
	if err := CheckPlanDigest(&loaded, plan.Digest); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(plan, &loaded) {
		t.Fatal("saved plan changed JSON values or integer precision")
	}
	if status := classifyRepair(&loaded, rows, tags); status.State != CoverRepairBefore {
		t.Fatalf("unchanged rows classified as %s", status.State)
	}
	for i := range rows {
		rows[i].CoverKey = plan.Items[i].Pair.AfterCoverKey
	}
	if status := classifyRepair(&loaded, rows, tags); status.State != CoverRepairAfter {
		t.Fatalf("applied rows classified as %s", status.State)
	}
	rows[0].Analysis.Value = json.RawMessage(`{"nested":{"count":9007199254740992,"values":[1,2]}}`)
	if status := classifyRepair(&loaded, rows, tags); status.State != CoverRepairConflict {
		t.Fatal("changed integer was not detected")
	}
}

func TestCoverRepairMySQLTransaction(t *testing.T) {
	db := repairTestMySQL(t)
	resetRepairSchema(t, db)
	pairs := repairTestPairs()
	plan, err := BuildCoverRepairPlan(t.Context(), db, "test-apply-rollback", pairs)
	if err != nil {
		t.Fatalf("build plan: %v", err)
	}
	if err := ValidateCoverRepairPlan(plan); err != nil {
		t.Fatalf("validate plan: %v", err)
	}
	serializedPlan, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		t.Fatalf("serialize plan: %v", err)
	}
	if strings.Contains(string(serializedPlan), "r2_fixture_only_584") || strings.Contains(string(serializedPlan), "127.0.0.1:32771") {
		t.Fatal("plan contains DSN material")
	}
	if err := json.Unmarshal(serializedPlan, &plan); err != nil {
		t.Fatalf("reload persisted plan: %v", err)
	}
	beforeUpdatedAt := make(map[uint64]string, len(plan.Items))
	for _, item := range plan.Items {
		beforeUpdatedAt[item.Pair.ID] = item.Before.UpdatedAt
		if len(item.Tags) != 1 || item.Tags[0].TagID != 1 {
			t.Fatalf("id %d tags = %+v", item.Pair.ID, item.Tags)
		}
	}

	receipt, err := ApplyCoverRepair(t.Context(), db, plan)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if receipt.State != CoverRepairAfter {
		t.Fatalf("apply state = %s", receipt.State)
	}
	if err := ValidateCoverRepairReceipt(receipt, plan); err != nil {
		t.Fatalf("validate receipt: %v", err)
	}
	status, err := StatusCoverRepair(t.Context(), db, plan)
	if err != nil || status.State != CoverRepairAfter {
		t.Fatalf("status after apply = %+v err=%v", status, err)
	}
	rows, _, err := readRepairSnapshots(t.Context(), db.DB, plan.IDs, false)
	if err != nil {
		t.Fatalf("read after apply: %v", err)
	}
	for _, row := range rows {
		if row.UpdatedAt != beforeUpdatedAt[row.ID] {
			t.Fatalf("id %d updated_at changed: %s -> %s", row.ID, beforeUpdatedAt[row.ID], row.UpdatedAt)
		}
	}
	retryReceipt, err := ApplyCoverRepair(t.Context(), db, plan)
	if err != nil {
		t.Fatalf("idempotent apply: %v", err)
	}
	if retryReceipt.State != CoverRepairAfter {
		t.Fatalf("idempotent apply state = %s", retryReceipt.State)
	}

	serializedReceipt, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(serializedReceipt, &receipt); err != nil {
		t.Fatalf("reload persisted receipt: %v", err)
	}
	rollbackReceipt, err := RollbackCoverRepair(t.Context(), db, plan, receipt)
	if err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if rollbackReceipt.State != CoverRepairRolledBack {
		t.Fatalf("rollback state = %s", rollbackReceipt.State)
	}
	status, err = StatusCoverRepair(t.Context(), db, plan)
	if err != nil || status.State != CoverRepairBefore {
		t.Fatalf("status after rollback = %+v err=%v", status, err)
	}
}

func TestCoverRepairRejectsConflictAndTriggers(t *testing.T) {
	t.Run("row conflict", func(t *testing.T) {
		db := repairTestMySQL(t)
		resetRepairSchema(t, db)
		plan, err := BuildCoverRepairPlan(t.Context(), db, "test-conflict", repairTestPairs())
		if err != nil {
			t.Fatalf("build plan: %v", err)
		}
		if err := db.Exec("UPDATE inspirations SET title = 'changed', updated_at = updated_at WHERE id = 149").Error; err != nil {
			t.Fatalf("create conflict: %v", err)
		}
		status, err := StatusCoverRepair(t.Context(), db, plan)
		if err != nil {
			t.Fatalf("status: %v", err)
		}
		if status.State != CoverRepairConflict || !containsID(status.ConflictIDs, 149) {
			t.Fatalf("conflict status = %+v", status)
		}
		if _, err := ApplyCoverRepair(t.Context(), db, plan); err == nil {
			t.Fatal("apply accepted conflicting row")
		}
	})

	t.Run("mixed before and after", func(t *testing.T) {
		db := repairTestMySQL(t)
		resetRepairSchema(t, db)
		plan, err := BuildCoverRepairPlan(t.Context(), db, "test-mixed", repairTestPairs())
		if err != nil {
			t.Fatalf("build plan: %v", err)
		}
		if err := db.Exec("UPDATE inspirations SET cover_key = ?, updated_at = updated_at WHERE id = 149", plan.Items[4].Pair.AfterCoverKey).Error; err != nil {
			t.Fatalf("create mixed state: %v", err)
		}
		status, err := StatusCoverRepair(t.Context(), db, plan)
		if err != nil {
			t.Fatalf("status: %v", err)
		}
		if status.State != CoverRepairConflict || !containsID(status.BeforeIDs, 145) || !containsID(status.AfterIDs, 149) {
			t.Fatalf("mixed status = %+v", status)
		}
		if _, err := ApplyCoverRepair(t.Context(), db, plan); err == nil {
			t.Fatal("apply accepted mixed before/after state")
		}
	})

	t.Run("large JSON integer differs by one", func(t *testing.T) {
		db := repairTestMySQL(t)
		resetRepairSchema(t, db)
		if err := db.Exec(`UPDATE inspirations SET analysis = JSON_OBJECT('value', CAST('9007199254740992' AS UNSIGNED)), updated_at = updated_at WHERE id = 149`).Error; err != nil {
			t.Fatalf("set large JSON integer: %v", err)
		}
		plan, err := BuildCoverRepairPlan(t.Context(), db, "test-large-json-integer", repairTestPairs())
		if err != nil {
			t.Fatalf("build plan: %v", err)
		}
		if err := db.Exec(`UPDATE inspirations SET analysis = JSON_OBJECT('value', CAST('9007199254740993' AS UNSIGNED)), updated_at = updated_at WHERE id = 149`).Error; err != nil {
			t.Fatalf("change large JSON integer: %v", err)
		}
		status, err := StatusCoverRepair(t.Context(), db, plan)
		if err != nil {
			t.Fatalf("status: %v", err)
		}
		if status.State != CoverRepairConflict || !containsID(status.ConflictIDs, 149) {
			t.Fatalf("large integer conflict status = %+v", status)
		}
	})

	t.Run("rollback conflict", func(t *testing.T) {
		db := repairTestMySQL(t)
		resetRepairSchema(t, db)
		plan, err := BuildCoverRepairPlan(t.Context(), db, "test-rollback-conflict", repairTestPairs())
		if err != nil {
			t.Fatalf("build plan: %v", err)
		}
		receipt, err := ApplyCoverRepair(t.Context(), db, plan)
		if err != nil {
			t.Fatalf("apply: %v", err)
		}
		if err := db.Exec("UPDATE inspirations SET title = 'changed', updated_at = updated_at WHERE id = 149").Error; err != nil {
			t.Fatalf("create conflict: %v", err)
		}
		if _, err := RollbackCoverRepair(t.Context(), db, plan, receipt); err == nil {
			t.Fatal("rollback accepted conflicting row")
		}
	})

	t.Run("trigger preflight", func(t *testing.T) {
		db := repairTestMySQL(t)
		resetRepairSchema(t, db)
		if err := db.Exec(`CREATE TRIGGER cover_repair_test_trigger BEFORE UPDATE ON inspirations
			FOR EACH ROW SET NEW.hot_score = NEW.hot_score`).Error; err != nil {
			t.Fatalf("create trigger: %v", err)
		}
		if _, err := BuildCoverRepairPlan(t.Context(), db, "test-trigger", repairTestPairs()); err == nil || !strings.Contains(err.Error(), "triggers") {
			t.Fatalf("trigger preflight error = %v", err)
		}
	})

	t.Run("orphan tag association", func(t *testing.T) {
		db := repairTestMySQL(t)
		resetRepairSchema(t, db)
		plan, err := BuildCoverRepairPlan(t.Context(), db, "test-orphan-tag", repairTestPairs())
		if err != nil {
			t.Fatalf("build initial plan: %v", err)
		}
		if err := db.Exec("INSERT INTO inspiration_tags (inspiration_id, tag_id) VALUES (145, 999)").Error; err != nil {
			t.Fatalf("insert orphan association: %v", err)
		}
		if _, err := BuildCoverRepairPlan(t.Context(), db, "test-orphan-tag-new-plan", repairTestPairs()); err == nil || !strings.Contains(err.Error(), "orphan") {
			t.Fatalf("orphan association error = %v", err)
		}
		if _, err := StatusCoverRepair(t.Context(), db, plan); err == nil || !strings.Contains(err.Error(), "orphan") {
			t.Fatalf("orphan status error = %v", err)
		}
		if _, err := ApplyCoverRepair(t.Context(), db, plan); err == nil || !strings.Contains(err.Error(), "orphan") {
			t.Fatalf("orphan transaction error = %v", err)
		}
	})
}

func TestCoverRepairReceiptPreservesEmptyTags(t *testing.T) {
	db := repairTestMySQL(t)
	resetRepairSchema(t, db)
	if err := db.Exec("DELETE FROM inspiration_tags WHERE inspiration_id = 154").Error; err != nil {
		t.Fatalf("delete fixture tag association: %v", err)
	}
	plan, err := BuildCoverRepairPlan(t.Context(), db, "test-empty-tags", repairTestPairs())
	if err != nil {
		t.Fatalf("build plan: %v", err)
	}
	if plan.Items[9].Tags == nil || len(plan.Items[9].Tags) != 0 {
		t.Fatalf("plan empty tags = %#v", plan.Items[9].Tags)
	}
	receipt, err := ApplyCoverRepair(t.Context(), db, plan)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if receipt.Tags[154] == nil || len(receipt.Tags[154]) != 0 {
		t.Fatalf("receipt empty tags = %#v", receipt.Tags[154])
	}
	if err := ValidateCoverRepairReceipt(receipt, plan); err != nil {
		t.Fatalf("validate receipt: %v", err)
	}
}

func TestCoverRepairCommitOutcomeUnknown(t *testing.T) {
	t.Run("apply committed", func(t *testing.T) {
		db := repairTestMySQL(t)
		resetRepairSchema(t, db)
		plan, err := BuildCoverRepairPlan(t.Context(), db, "test-unknown-apply", repairTestPairs())
		if err != nil {
			t.Fatalf("build plan: %v", err)
		}
		faultDB := repairCommitErrorDB(t, db)
		receipt, err := ApplyCoverRepair(t.Context(), faultDB, plan)
		assertCommitUnknown(t, receipt, err)
		if err := ValidateCoverRepairReceipt(receipt, plan); err != nil {
			t.Fatalf("validate unknown apply receipt: %v", err)
		}
		status, statusErr := StatusCoverRepair(t.Context(), db, plan)
		if statusErr != nil || status.State != CoverRepairAfter {
			t.Fatalf("status after unknown apply = %+v err=%v", status, statusErr)
		}
		rollbackReceipt, err := RollbackCoverRepair(t.Context(), db, plan, receipt)
		if err != nil || rollbackReceipt.State != CoverRepairRolledBack {
			t.Fatalf("rollback after unknown apply = %+v err=%v", rollbackReceipt, err)
		}
	})

	t.Run("rollback committed", func(t *testing.T) {
		db := repairTestMySQL(t)
		resetRepairSchema(t, db)
		plan, err := BuildCoverRepairPlan(t.Context(), db, "test-unknown-rollback", repairTestPairs())
		if err != nil {
			t.Fatalf("build plan: %v", err)
		}
		applyReceipt, err := ApplyCoverRepair(t.Context(), db, plan)
		if err != nil {
			t.Fatalf("apply: %v", err)
		}
		faultDB := repairCommitErrorDB(t, db)
		receipt, err := RollbackCoverRepair(t.Context(), faultDB, plan, applyReceipt)
		assertCommitUnknown(t, receipt, err)
		if err := ValidateCoverRepairReceipt(receipt, plan); err != nil {
			t.Fatalf("validate unknown rollback receipt: %v", err)
		}
		status, statusErr := StatusCoverRepair(t.Context(), db, plan)
		if statusErr != nil || status.State != CoverRepairBefore {
			t.Fatalf("status after unknown rollback = %+v err=%v", status, statusErr)
		}
	})
}

func TestValidateRepairPairsRejectsInvalidRefs(t *testing.T) {
	tests := map[string]func(*CoverRepairPair){
		"external URL":   func(pair *CoverRepairPair) { pair.AfterCoverKey = "https://example.com/cover.jpg" },
		"path traversal": func(pair *CoverRepairPair) { pair.AfterCoverKey = "r2/publications/../cover.jpg" },
		"encoded path":   func(pair *CoverRepairPair) { pair.AfterCoverKey = "r2/publications/a%2fb/cover.jpg" },
		"too long":       func(pair *CoverRepairPair) { pair.AfterCoverKey = "r2/publications/" + strings.Repeat("a", 240) },
		"unchanged":      func(pair *CoverRepairPair) { pair.BeforeCoverKey = pair.AfterCoverKey },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			pairs := repairTestPairs()
			mutate(&pairs[0])
			if _, err := validateRepairPairs(pairs); err == nil {
				t.Fatal("invalid pair was accepted")
			}
		})
	}
}

func TestCoverRepairLockConflictDoesNotPartiallyApply(t *testing.T) {
	db := repairTestMySQL(t)
	resetRepairSchema(t, db)
	plan, err := BuildCoverRepairPlan(t.Context(), db, "test-lock-conflict", repairTestPairs())
	if err != nil {
		t.Fatalf("build plan: %v", err)
	}

	lockTx := db.Begin()
	if lockTx.Error != nil {
		t.Fatalf("begin lock transaction: %v", lockTx.Error)
	}
	t.Cleanup(func() { _ = lockTx.Rollback().Error })
	var lockedID uint64
	if err := lockTx.Raw("SELECT id FROM inspirations WHERE id = 145 FOR UPDATE").Scan(&lockedID).Error; err != nil {
		t.Fatalf("lock row: %v", err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 300*time.Millisecond)
	defer cancel()
	if _, err := ApplyCoverRepair(ctx, db, plan); err == nil {
		t.Fatal("apply unexpectedly acquired conflicting row lock")
	}
	if err := lockTx.Rollback().Error; err != nil {
		t.Fatalf("release row lock: %v", err)
	}
	status, err := StatusCoverRepair(t.Context(), db, plan)
	if err != nil || status.State != CoverRepairBefore {
		t.Fatalf("status after lock conflict = %+v err=%v", status, err)
	}
}

func TestCoverRepairFifthUpdateFailureRollsBackAllRows(t *testing.T) {
	db := repairTestMySQL(t)
	resetRepairSchema(t, db)
	if err := db.Exec("CREATE UNIQUE INDEX cover_repair_unique_cover_key ON inspirations (cover_key)").Error; err != nil {
		t.Fatalf("create unique fixture index: %v", err)
	}
	pairs := repairTestPairs()
	pairs[4].AfterCoverKey = pairs[0].AfterCoverKey
	plan, err := BuildCoverRepairPlan(t.Context(), db, "test-fifth-update-failure", pairs)
	if err != nil {
		t.Fatalf("build plan: %v", err)
	}
	if _, err := ApplyCoverRepair(t.Context(), db, plan); err == nil {
		t.Fatal("apply unexpectedly accepted duplicate fifth cover key")
	}
	status, err := StatusCoverRepair(t.Context(), db, plan)
	if err != nil || status.State != CoverRepairBefore {
		t.Fatalf("status after fifth update failure = %+v err=%v", status, err)
	}
}

func repairTestPlan(t *testing.T) *CoverRepairPlan {
	t.Helper()
	pairs := repairTestPairs()
	plan := &CoverRepairPlan{
		SchemaVersion: CoverRepairSchemaVersion,
		OperationID:   "pure-test",
		IDs:           repairIDs(pairs),
		Items:         make([]CoverRepairPlanItem, len(pairs)),
	}
	for i, pair := range pairs {
		plan.Items[i] = CoverRepairPlanItem{Pair: pair, ObjectKey: strings.TrimPrefix(pair.AfterCoverKey, "r2/"), Before: InspirationSnapshot{ID: pair.ID, CoverKey: pair.BeforeCoverKey}, Tags: []TagAssociationSnapshot{}}
	}
	var err error
	plan.Digest, err = planDigest(plan)
	if err != nil {
		t.Fatalf("digest plan: %v", err)
	}
	return plan
}

func repairTestPairs() []CoverRepairPair {
	image := []byte("fixture image")
	sum := sha256.Sum256(image)
	pairs := make([]CoverRepairPair, 10)
	for i := range pairs {
		id := uint64(145 + i)
		pairs[i] = CoverRepairPair{
			ID: id, SourceID: "source", BeforeCoverKey: "https://old.example/cover.jpg?id=" + string(rune('a'+i)),
			AfterCoverKey:       "r2/publications/test/" + string(rune('a'+i)) + "/cover.jpg",
			ExpectedImageSHA256: hex.EncodeToString(sum[:]), ExpectedImageBytes: int64(len(image)),
		}
	}
	return pairs
}

func repairTestMySQL(t *testing.T) *store.DB {
	t.Helper()
	dsn := os.Getenv("INSPIRATION_COVER_REPAIR_TEST_DSN")
	if dsn == "" {
		t.Skip("INSPIRATION_COVER_REPAIR_TEST_DSN is not set")
	}
	db, err := store.Open(dsn)
	if err != nil {
		t.Fatalf("open MySQL: %v", err)
	}
	if db.Dialect != store.DialectMySQL {
		t.Fatal("cover repair test requires MySQL")
	}
	return db
}

func resetRepairSchema(t *testing.T, db *store.DB) {
	t.Helper()
	statements := []string{
		"DROP TRIGGER IF EXISTS cover_repair_test_trigger",
		"DROP TABLE IF EXISTS inspiration_tags",
		"DROP TABLE IF EXISTS tags",
		"DROP TABLE IF EXISTS inspirations",
		`CREATE TABLE inspirations (
			id BIGINT UNSIGNED PRIMARY KEY, type VARCHAR(16) NOT NULL DEFAULT 'link', title VARCHAR(512) NOT NULL DEFAULT '',
			content TEXT, source_url VARCHAR(1024) NOT NULL DEFAULT '', source_url_hash CHAR(64) NOT NULL DEFAULT '',
			cover_key VARCHAR(255) NOT NULL DEFAULT '', media_keys JSON, hot_score INT NOT NULL DEFAULT 0,
			is_favorite TINYINT(1) NOT NULL DEFAULT 0, analysis JSON,
			created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
			updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3), deleted_at DATETIME(3) NULL
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE tags (id BIGINT UNSIGNED PRIMARY KEY, name VARCHAR(128) NOT NULL, created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)) ENGINE=InnoDB`,
		`CREATE TABLE inspiration_tags (inspiration_id BIGINT UNSIGNED NOT NULL, tag_id BIGINT UNSIGNED NOT NULL,
			PRIMARY KEY (inspiration_id, tag_id)) ENGINE=InnoDB`,
		"INSERT INTO tags (id, name) VALUES (1, 'fixture-tag')",
	}
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("reset schema: %v", err)
		}
	}
	for i := 0; i < 10; i++ {
		id := 145 + i
		mediaJSON, _ := json.Marshal(map[string]any{"z": id, "a": []int{1, 2}})
		analysisJSON, _ := json.Marshal(map[string]any{"score": id})
		if err := db.Exec(`INSERT INTO inspirations
			(id,type,title,content,source_url,source_url_hash,cover_key,media_keys,hot_score,is_favorite,analysis,created_at,updated_at,deleted_at)
			VALUES (?, 'image', ?, ?, ?, ?, ?, ?, ?, ?, ?, '2026-09-05 01:02:03.123', '2026-09-05 04:05:06.789', NULL)`,
			id, "title", "content", "https://source.example/item", strings.Repeat(string(rune('a'+i)), 64),
			repairTestPairs()[i].BeforeCoverKey, string(mediaJSON), id, i%2, string(analysisJSON)).Error; err != nil {
			t.Fatalf("insert id %d: %v", id, err)
		}
		if err := db.Exec("INSERT INTO inspiration_tags (inspiration_id, tag_id) VALUES (?, 1)", id).Error; err != nil {
			t.Fatalf("insert tag link id %d: %v", id, err)
		}
	}
}

func containsID(ids []uint64, want uint64) bool {
	for _, id := range ids {
		if id == want {
			return true
		}
	}
	return false
}

var errInjectedCommitResponse = errors.New("injected lost commit response")

type commitErrorPool struct{ db *sql.DB }

func (p *commitErrorPool) PrepareContext(ctx context.Context, query string) (*sql.Stmt, error) {
	return p.db.PrepareContext(ctx, query)
}

func (p *commitErrorPool) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return p.db.ExecContext(ctx, query, args...)
}

func (p *commitErrorPool) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return p.db.QueryContext(ctx, query, args...)
}

func (p *commitErrorPool) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return p.db.QueryRowContext(ctx, query, args...)
}

func (p *commitErrorPool) BeginTx(ctx context.Context, opts *sql.TxOptions) (gorm.ConnPool, error) {
	tx, err := p.db.BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}
	return &commitErrorTx{Tx: tx}, nil
}

type commitErrorTx struct{ *sql.Tx }

func (tx *commitErrorTx) Commit() error {
	if err := tx.Tx.Commit(); err != nil {
		return err
	}
	return errInjectedCommitResponse
}

func repairCommitErrorDB(t *testing.T, db *store.DB) *store.DB {
	t.Helper()
	sqlDB, err := db.DB.DB()
	if err != nil {
		t.Fatalf("get SQL DB: %v", err)
	}
	gormDB, err := gorm.Open(gormmysql.New(gormmysql.Config{
		Conn:                      &commitErrorPool{db: sqlDB},
		SkipInitializeWithVersion: true,
	}), &gorm.Config{})
	if err != nil {
		t.Fatalf("open commit-error GORM wrapper: %v", err)
	}
	return &store.DB{DB: gormDB, Dialect: store.DialectMySQL}
}

func assertCommitUnknown(t *testing.T, receipt *CoverRepairReceipt, err error) {
	t.Helper()
	var unknown *CommitUnknownError
	if !errors.As(err, &unknown) || !errors.Is(err, errInjectedCommitResponse) {
		t.Fatalf("commit error = %v", err)
	}
	if receipt == nil || receipt.State != CoverRepairUnknown {
		t.Fatalf("unknown receipt = %+v", receipt)
	}
}
