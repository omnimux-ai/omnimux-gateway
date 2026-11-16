package service

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"

	"omnimux-inspiration/internal/media"
	"omnimux-inspiration/internal/store"
)

const CoverRepairSchemaVersion = 1

type CoverRepairPair struct {
	ID                  uint64 `json:"id"`
	SourceID            string `json:"sourceId"`
	TikTokID            string `json:"tiktokId,omitempty"`
	BeforeCoverKey      string `json:"beforeCoverKey"`
	AfterCoverKey       string `json:"afterCoverKey"`
	UnchangedSourceURL  string `json:"unchangedSourceUrl,omitempty"`
	ExpectedImageSHA256 string `json:"expectedImageSha256"`
	ExpectedImageBytes  int64  `json:"expectedImageBytes"`
}

type CoverRepairManifest struct {
	Items []CoverRepairPair `json:"items"`
}

type DBJSON struct {
	Null  bool            `json:"null"`
	Value json.RawMessage `json:"value,omitempty"`
}

// Saved plans and receipts must compare with canonical database JSON after decoding.
func (value *DBJSON) UnmarshalJSON(raw []byte) error {
	type wireDBJSON DBJSON
	var decoded wireDBJSON
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return err
	}
	if len(decoded.Value) > 0 {
		canonical, err := canonicalDBJSON(sql.NullString{String: string(decoded.Value), Valid: true})
		if err != nil {
			return err
		}
		decoded.Value = canonical.Value
	}
	*value = DBJSON(decoded)
	return nil
}

type InspirationSnapshot struct {
	ID            uint64  `json:"id"`
	Type          string  `json:"type"`
	Title         string  `json:"title"`
	Content       *string `json:"content"`
	SourceURL     string  `json:"sourceUrl"`
	SourceURLHash string  `json:"sourceUrlHash"`
	CoverKey      string  `json:"coverKey"`
	MediaKeys     DBJSON  `json:"mediaKeys"`
	HotScore      int     `json:"hotScore"`
	IsFavorite    bool    `json:"isFavorite"`
	Analysis      DBJSON  `json:"analysis"`
	CreatedAt     string  `json:"createdAt"`
	UpdatedAt     string  `json:"updatedAt"`
	DeletedAt     *string `json:"deletedAt"`
}

type TagAssociationSnapshot struct {
	TagID uint64 `json:"tagId"`
	Name  string `json:"name"`
}

type DatabaseIdentity struct {
	ServerUUID  string `json:"serverUuid"`
	Hostname    string `json:"hostname"`
	Port        uint16 `json:"port"`
	Database    string `json:"database" gorm:"column:database_name"`
	Version     string `json:"version"`
	Fingerprint string `json:"fingerprint"`
}

type CoverRepairPlanItem struct {
	Pair      CoverRepairPair          `json:"pair"`
	ObjectKey string                   `json:"objectKey"`
	Before    InspirationSnapshot      `json:"before"`
	Tags      []TagAssociationSnapshot `json:"tags"`
}

type CoverRepairPlan struct {
	SchemaVersion     int                   `json:"schemaVersion"`
	OperationID       string                `json:"operationId"`
	CreatedAt         time.Time             `json:"createdAt"`
	Database          DatabaseIdentity      `json:"database"`
	SchemaFingerprint string                `json:"schemaFingerprint"`
	IDs               []uint64              `json:"ids"`
	Items             []CoverRepairPlanItem `json:"items"`
	Digest            string                `json:"digest"`
}

type CoverObject struct {
	Data         []byte
	ContentType  string
	ETag         string
	LastModified time.Time
}

type CoverObjectReader interface {
	Read(ctx context.Context, ref string) (*CoverObject, error)
}

type CoverVerificationItem struct {
	ID           uint64    `json:"id"`
	Ref          string    `json:"ref"`
	SHA256       string    `json:"sha256"`
	Bytes        int64     `json:"bytes"`
	ContentType  string    `json:"contentType"`
	ETag         string    `json:"etag,omitempty"`
	LastModified time.Time `json:"lastModified,omitempty"`
}

type CoverVerification struct {
	OperationID string                  `json:"operationId"`
	VerifiedAt  time.Time               `json:"verifiedAt"`
	Items       []CoverVerificationItem `json:"items"`
}

type CoverRepairState string

const (
	CoverRepairBefore     CoverRepairState = "BEFORE"
	CoverRepairAfter      CoverRepairState = "AFTER"
	CoverRepairConflict   CoverRepairState = "CONFLICT"
	CoverRepairUnknown    CoverRepairState = "UNKNOWN"
	CoverRepairRolledBack CoverRepairState = "ROLLED_BACK"
)

type CoverRepairStatus struct {
	OperationID string           `json:"operationId"`
	CheckedAt   time.Time        `json:"checkedAt"`
	State       CoverRepairState `json:"state"`
	BeforeIDs   []uint64         `json:"beforeIds,omitempty"`
	AfterIDs    []uint64         `json:"afterIds,omitempty"`
	ConflictIDs []uint64         `json:"conflictIds,omitempty"`
}

type CoverRepairReceipt struct {
	SchemaVersion int                                 `json:"schemaVersion"`
	OperationID   string                              `json:"operationId"`
	PlanDigest    string                              `json:"planDigest"`
	Database      DatabaseIdentity                    `json:"database"`
	State         CoverRepairState                    `json:"state"`
	RecordedAt    time.Time                           `json:"recordedAt"`
	Before        []InspirationSnapshot               `json:"before"`
	After         []InspirationSnapshot               `json:"after"`
	Tags          map[uint64][]TagAssociationSnapshot `json:"tags"`
	Digest        string                              `json:"digest"`
}

type CommitUnknownError struct{ Err error }

func (e *CommitUnknownError) Error() string { return "commit outcome unknown: " + e.Err.Error() }
func (e *CommitUnknownError) Unwrap() error { return e.Err }

func BuildCoverRepairPlan(ctx context.Context, db *store.DB, operationID string, pairs []CoverRepairPair) (*CoverRepairPlan, error) {
	if err := requireMySQL(db); err != nil {
		return nil, err
	}
	pairs, err := validateRepairPairs(pairs)
	if err != nil {
		return nil, err
	}
	operationID = strings.TrimSpace(operationID)
	if operationID == "" {
		return nil, errors.New("operation id is required")
	}
	identity, schemaFingerprint, err := inspectRepairTarget(ctx, db.DB)
	if err != nil {
		return nil, err
	}
	ids := repairIDs(pairs)
	rows, tags, err := readRepairSnapshots(ctx, db.DB, ids, false)
	if err != nil {
		return nil, err
	}
	if len(rows) != len(ids) {
		return nil, fmt.Errorf("expected %d rows, found %d", len(ids), len(rows))
	}
	plan := &CoverRepairPlan{
		SchemaVersion: CoverRepairSchemaVersion, OperationID: operationID,
		CreatedAt: time.Now().UTC(), Database: identity, SchemaFingerprint: schemaFingerprint, IDs: ids,
		Items: make([]CoverRepairPlanItem, len(pairs)),
	}
	for i, pair := range pairs {
		row := rows[i]
		if row.ID != pair.ID {
			return nil, fmt.Errorf("row order mismatch at id %d", pair.ID)
		}
		if row.CoverKey != pair.BeforeCoverKey {
			return nil, fmt.Errorf("id %d cover_key differs from manifest", pair.ID)
		}
		if pair.UnchangedSourceURL != "" && row.SourceURL != pair.UnchangedSourceURL {
			return nil, fmt.Errorf("id %d source_url differs from manifest", pair.ID)
		}
		objectKey, err := media.ParseCoverRef(pair.AfterCoverKey)
		if err != nil {
			return nil, fmt.Errorf("id %d cover ref: %w", pair.ID, err)
		}
		plan.Items[i] = CoverRepairPlanItem{Pair: pair, ObjectKey: objectKey, Before: row, Tags: tags[pair.ID]}
	}
	plan.Digest, err = planDigest(plan)
	if err != nil {
		return nil, err
	}
	return plan, nil
}

func VerifyCoverRepairPlan(ctx context.Context, plan *CoverRepairPlan, reader CoverObjectReader) (*CoverVerification, error) {
	if err := ValidateCoverRepairPlan(plan); err != nil {
		return nil, err
	}
	if reader == nil {
		return nil, errors.New("cover reader is required")
	}
	out := &CoverVerification{OperationID: plan.OperationID, VerifiedAt: time.Now().UTC(), Items: make([]CoverVerificationItem, 0, len(plan.Items))}
	for _, item := range plan.Items {
		image, err := reader.Read(ctx, item.Pair.AfterCoverKey)
		if err != nil {
			return nil, fmt.Errorf("verify id %d: %w", item.Pair.ID, err)
		}
		if image == nil {
			return nil, fmt.Errorf("verify id %d: empty reader result", item.Pair.ID)
		}
		sum := sha256.Sum256(image.Data)
		gotHash := hex.EncodeToString(sum[:])
		if int64(len(image.Data)) != item.Pair.ExpectedImageBytes || !strings.EqualFold(gotHash, item.Pair.ExpectedImageSHA256) {
			return nil, fmt.Errorf("verify id %d: object bytes/hash mismatch", item.Pair.ID)
		}
		out.Items = append(out.Items, CoverVerificationItem{
			ID: item.Pair.ID, Ref: item.Pair.AfterCoverKey, SHA256: gotHash, Bytes: int64(len(image.Data)),
			ContentType: image.ContentType, ETag: image.ETag, LastModified: image.LastModified.UTC(),
		})
	}
	return out, nil
}

func ValidateCoverRepairPlan(plan *CoverRepairPlan) error {
	if plan == nil || plan.SchemaVersion != CoverRepairSchemaVersion {
		return errors.New("unsupported cover repair plan")
	}
	want, err := planDigest(plan)
	if err != nil {
		return err
	}
	if plan.Digest == "" || !strings.EqualFold(plan.Digest, want) {
		return errors.New("cover repair plan digest mismatch")
	}
	pairs := make([]CoverRepairPair, len(plan.Items))
	for i := range plan.Items {
		pairs[i] = plan.Items[i].Pair
		if plan.Items[i].Before.ID != pairs[i].ID || plan.Items[i].Before.CoverKey != pairs[i].BeforeCoverKey {
			return fmt.Errorf("plan item %d snapshot mismatch", pairs[i].ID)
		}
		objectKey, err := media.ParseCoverRef(pairs[i].AfterCoverKey)
		if err != nil || plan.Items[i].ObjectKey != objectKey {
			return fmt.Errorf("plan item %d object key mismatch", pairs[i].ID)
		}
	}
	validated, err := validateRepairPairs(pairs)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(plan.IDs, repairIDs(validated)) {
		return errors.New("plan id set mismatch")
	}
	return nil
}

func CheckPlanDigest(plan *CoverRepairPlan, supplied string) error {
	if err := ValidateCoverRepairPlan(plan); err != nil {
		return err
	}
	if supplied == "" || !strings.EqualFold(strings.TrimSpace(supplied), plan.Digest) {
		return errors.New("supplied plan digest mismatch")
	}
	return nil
}

func StatusCoverRepair(ctx context.Context, db *store.DB, plan *CoverRepairPlan) (*CoverRepairStatus, error) {
	if err := ValidateCoverRepairPlan(plan); err != nil {
		return nil, err
	}
	if err := validateRepairTarget(ctx, db, plan); err != nil {
		return nil, err
	}
	rows, tags, err := readRepairSnapshots(ctx, db.DB, plan.IDs, false)
	if err != nil {
		return nil, err
	}
	return classifyRepair(plan, rows, tags), nil
}

func ApplyCoverRepair(ctx context.Context, db *store.DB, plan *CoverRepairPlan) (*CoverRepairReceipt, error) {
	if err := ValidateCoverRepairPlan(plan); err != nil {
		return nil, err
	}
	if err := validateRepairTarget(ctx, db, plan); err != nil {
		return nil, err
	}
	tx := db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return nil, tx.Error
	}
	rollback := true
	defer func() {
		if rollback {
			_ = tx.Rollback().Error
		}
	}()
	rows, tags, err := readRepairSnapshots(ctx, tx, plan.IDs, true)
	if err != nil {
		return nil, err
	}
	status := classifyRepair(plan, rows, tags)
	if status.State == CoverRepairConflict {
		return nil, fmt.Errorf("repair state conflict at ids %v", status.ConflictIDs)
	}
	if status.State == CoverRepairBefore {
		var affected int64
		for _, item := range plan.Items {
			res := tx.Exec("UPDATE inspirations SET cover_key = ?, updated_at = updated_at WHERE id = ? AND cover_key = ?", item.Pair.AfterCoverKey, item.Pair.ID, item.Pair.BeforeCoverKey)
			if res.Error != nil {
				return nil, res.Error
			}
			affected += res.RowsAffected
		}
		if affected != int64(len(plan.Items)) {
			return nil, fmt.Errorf("updated %d rows, expected %d", affected, len(plan.Items))
		}
	}
	after, afterTags, err := readRepairSnapshots(ctx, tx, plan.IDs, true)
	if err != nil {
		return nil, err
	}
	afterStatus := classifyRepair(plan, after, afterTags)
	if afterStatus.State != CoverRepairAfter {
		return nil, fmt.Errorf("post-update state is %s", afterStatus.State)
	}
	receipt := newRepairReceipt(plan, CoverRepairAfter)
	if err := tx.Commit().Error; err != nil {
		rollback = false
		receipt.State = CoverRepairUnknown
		receipt.Digest, _ = receiptDigest(receipt)
		return receipt, &CommitUnknownError{Err: err}
	}
	rollback = false
	return receipt, nil
}

func RollbackCoverRepair(ctx context.Context, db *store.DB, plan *CoverRepairPlan, receipt *CoverRepairReceipt) (*CoverRepairReceipt, error) {
	if err := ValidateCoverRepairPlan(plan); err != nil {
		return nil, err
	}
	if err := ValidateCoverRepairReceipt(receipt, plan); err != nil {
		return nil, err
	}
	if receipt.State != CoverRepairAfter && receipt.State != CoverRepairUnknown {
		return nil, errors.New("rollback requires an AFTER or UNKNOWN apply receipt")
	}
	if err := validateRepairTarget(ctx, db, plan); err != nil {
		return nil, err
	}
	tx := db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return nil, tx.Error
	}
	rollback := true
	defer func() {
		if rollback {
			_ = tx.Rollback().Error
		}
	}()
	rows, tags, err := readRepairSnapshots(ctx, tx, plan.IDs, true)
	if err != nil {
		return nil, err
	}
	if classifyRepair(plan, rows, tags).State != CoverRepairAfter {
		return nil, errors.New("rollback requires complete AFTER state")
	}
	var affected int64
	for _, item := range plan.Items {
		res := tx.Exec("UPDATE inspirations SET cover_key = ?, updated_at = updated_at WHERE id = ? AND cover_key = ?", item.Pair.BeforeCoverKey, item.Pair.ID, item.Pair.AfterCoverKey)
		if res.Error != nil {
			return nil, res.Error
		}
		affected += res.RowsAffected
	}
	if affected != int64(len(plan.Items)) {
		return nil, fmt.Errorf("rolled back %d rows, expected %d", affected, len(plan.Items))
	}
	after, afterTags, err := readRepairSnapshots(ctx, tx, plan.IDs, true)
	if err != nil {
		return nil, err
	}
	if classifyRepair(plan, after, afterTags).State != CoverRepairBefore {
		return nil, errors.New("post-rollback state is not BEFORE")
	}
	out := newRepairReceipt(plan, CoverRepairRolledBack)
	if err := tx.Commit().Error; err != nil {
		rollback = false
		out.State = CoverRepairUnknown
		out.Digest, _ = receiptDigest(out)
		return out, &CommitUnknownError{Err: err}
	}
	rollback = false
	return out, nil
}

func ValidateCoverRepairReceipt(receipt *CoverRepairReceipt, plan *CoverRepairPlan) error {
	if receipt == nil || receipt.SchemaVersion != CoverRepairSchemaVersion || receipt.OperationID != plan.OperationID || receipt.PlanDigest != plan.Digest {
		return errors.New("receipt does not match plan")
	}
	if receipt.Database.Fingerprint != plan.Database.Fingerprint || len(receipt.Before) != len(plan.Items) || len(receipt.After) != len(plan.Items) {
		return errors.New("receipt snapshot does not match plan")
	}
	for i, item := range plan.Items {
		wantAfter := item.Before
		wantAfter.CoverKey = item.Pair.AfterCoverKey
		if !reflect.DeepEqual(receipt.Before[i], item.Before) || !reflect.DeepEqual(receipt.After[i], wantAfter) || !reflect.DeepEqual(receipt.Tags[item.Pair.ID], item.Tags) {
			return fmt.Errorf("receipt snapshot differs at id %d", item.Pair.ID)
		}
	}
	want, err := receiptDigest(receipt)
	if err != nil {
		return err
	}
	if receipt.Digest == "" || !strings.EqualFold(receipt.Digest, want) {
		return errors.New("receipt digest mismatch")
	}
	return nil
}

func CheckReceiptDigest(receipt *CoverRepairReceipt, plan *CoverRepairPlan, supplied string) error {
	if err := ValidateCoverRepairReceipt(receipt, plan); err != nil {
		return err
	}
	if supplied == "" || !strings.EqualFold(strings.TrimSpace(supplied), receipt.Digest) {
		return errors.New("supplied receipt digest mismatch")
	}
	return nil
}

func newRepairReceipt(plan *CoverRepairPlan, state CoverRepairState) *CoverRepairReceipt {
	before := make([]InspirationSnapshot, len(plan.Items))
	after := make([]InspirationSnapshot, len(plan.Items))
	tags := make(map[uint64][]TagAssociationSnapshot, len(plan.Items))
	for i, item := range plan.Items {
		before[i] = item.Before
		after[i] = item.Before
		after[i].CoverKey = item.Pair.AfterCoverKey
		tags[item.Pair.ID] = make([]TagAssociationSnapshot, len(item.Tags))
		copy(tags[item.Pair.ID], item.Tags)
	}
	r := &CoverRepairReceipt{
		SchemaVersion: CoverRepairSchemaVersion, OperationID: plan.OperationID, PlanDigest: plan.Digest,
		Database: plan.Database, State: state, RecordedAt: time.Now().UTC(), Before: before, After: after, Tags: tags,
	}
	r.Digest, _ = receiptDigest(r)
	return r
}

func validateRepairTarget(ctx context.Context, db *store.DB, plan *CoverRepairPlan) error {
	if err := requireMySQL(db); err != nil {
		return err
	}
	identity, schemaFingerprint, err := inspectRepairTarget(ctx, db.DB)
	if err != nil {
		return err
	}
	if identity.Fingerprint != plan.Database.Fingerprint || schemaFingerprint != plan.SchemaFingerprint {
		return errors.New("database identity or schema differs from plan")
	}
	return nil
}

func requireMySQL(db *store.DB) error {
	if db == nil || db.DB == nil || db.Dialect != store.DialectMySQL {
		return errors.New("cover repair supports MySQL only")
	}
	return nil
}

func validateRepairPairs(in []CoverRepairPair) ([]CoverRepairPair, error) {
	if len(in) != 10 {
		return nil, fmt.Errorf("repair manifest must contain exactly 10 items, got %d", len(in))
	}
	out := append([]CoverRepairPair(nil), in...)
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	for i := range out {
		wantID := uint64(145 + i)
		p := &out[i]
		if p.ID != wantID {
			return nil, fmt.Errorf("repair ids must be exactly 145-154; missing %d", wantID)
		}
		if p.BeforeCoverKey == "" || p.BeforeCoverKey == p.AfterCoverKey {
			return nil, fmt.Errorf("invalid cover pairing for id %d", p.ID)
		}
		if _, err := media.ParseCoverRef(p.AfterCoverKey); err != nil {
			return nil, fmt.Errorf("invalid cover pairing for id %d", p.ID)
		}
		if p.ExpectedImageBytes <= 0 || len(p.ExpectedImageSHA256) != 64 {
			return nil, fmt.Errorf("invalid expected image evidence for id %d", p.ID)
		}
		if _, err := hex.DecodeString(p.ExpectedImageSHA256); err != nil {
			return nil, fmt.Errorf("invalid image digest for id %d", p.ID)
		}
	}
	return out, nil
}

func repairIDs(pairs []CoverRepairPair) []uint64 {
	ids := make([]uint64, len(pairs))
	for i := range pairs {
		ids[i] = pairs[i].ID
	}
	return ids
}

type rawInspirationSnapshot struct {
	ID            uint64
	Type          string
	Title         string
	Content       sql.NullString
	SourceURL     string         `gorm:"column:source_url"`
	SourceURLHash string         `gorm:"column:source_url_hash"`
	CoverKey      string         `gorm:"column:cover_key"`
	MediaKeys     sql.NullString `gorm:"column:media_keys"`
	HotScore      int            `gorm:"column:hot_score"`
	IsFavorite    bool           `gorm:"column:is_favorite"`
	Analysis      sql.NullString
	CreatedAt     string         `gorm:"column:created_at"`
	UpdatedAt     string         `gorm:"column:updated_at"`
	DeletedAt     sql.NullString `gorm:"column:deleted_at"`
}

type repairColumnInfo struct {
	TableName, ColumnName, ColumnType, IsNullable, ColumnDefault, Extra, CollationName string
	OrdinalPosition                                                                    uint64
}

func readRepairSnapshots(ctx context.Context, db *gorm.DB, ids []uint64, lock bool) ([]InspirationSnapshot, map[uint64][]TagAssociationSnapshot, error) {
	query := `SELECT id, type, title, content, source_url, source_url_hash, cover_key,
		CAST(media_keys AS CHAR) AS media_keys, hot_score, is_favorite, CAST(analysis AS CHAR) AS analysis,
		DATE_FORMAT(created_at, '%Y-%m-%d %H:%i:%s.%f') AS created_at,
		DATE_FORMAT(updated_at, '%Y-%m-%d %H:%i:%s.%f') AS updated_at,
		DATE_FORMAT(deleted_at, '%Y-%m-%d %H:%i:%s.%f') AS deleted_at
		FROM inspirations WHERE id IN ? ORDER BY id`
	if lock {
		query += " FOR UPDATE"
	}
	var raw []rawInspirationSnapshot
	if err := db.WithContext(ctx).Raw(query, ids).Scan(&raw).Error; err != nil {
		return nil, nil, err
	}
	rows := make([]InspirationSnapshot, len(raw))
	for i := range raw {
		content := nullableString(raw[i].Content)
		deleted := nullableString(raw[i].DeletedAt)
		mediaKeys, err := canonicalDBJSON(raw[i].MediaKeys)
		if err != nil {
			return nil, nil, fmt.Errorf("id %d media_keys: %w", raw[i].ID, err)
		}
		analysis, err := canonicalDBJSON(raw[i].Analysis)
		if err != nil {
			return nil, nil, fmt.Errorf("id %d analysis: %w", raw[i].ID, err)
		}
		rows[i] = InspirationSnapshot{
			ID: raw[i].ID, Type: raw[i].Type, Title: raw[i].Title, Content: content,
			SourceURL: raw[i].SourceURL, SourceURLHash: raw[i].SourceURLHash, CoverKey: raw[i].CoverKey,
			MediaKeys: mediaKeys, HotScore: raw[i].HotScore, IsFavorite: raw[i].IsFavorite, Analysis: analysis,
			CreatedAt: raw[i].CreatedAt, UpdatedAt: raw[i].UpdatedAt, DeletedAt: deleted,
		}
	}
	tags, err := readRepairTags(ctx, db, ids, lock)
	return rows, tags, err
}

func readRepairTags(ctx context.Context, db *gorm.DB, ids []uint64, lock bool) (map[uint64][]TagAssociationSnapshot, error) {
	type rawTag struct {
		InspirationID uint64        `gorm:"column:inspiration_id"`
		TagID         uint64        `gorm:"column:tag_id"`
		ResolvedTagID sql.NullInt64 `gorm:"column:resolved_tag_id"`
		Name          sql.NullString
	}
	query := `SELECT it.inspiration_id, it.tag_id, t.id AS resolved_tag_id, t.name FROM inspiration_tags it
		LEFT JOIN tags t ON t.id = it.tag_id WHERE it.inspiration_id IN ? ORDER BY it.inspiration_id, it.tag_id`
	if lock {
		query += " FOR UPDATE"
	}
	var raw []rawTag
	if err := db.WithContext(ctx).Raw(query, ids).Scan(&raw).Error; err != nil {
		return nil, err
	}
	out := make(map[uint64][]TagAssociationSnapshot, len(ids))
	for _, id := range ids {
		out[id] = []TagAssociationSnapshot{}
	}
	for _, tag := range raw {
		if !tag.ResolvedTagID.Valid || !tag.Name.Valid {
			return nil, fmt.Errorf("orphan tag association: inspiration %d tag %d", tag.InspirationID, tag.TagID)
		}
		out[tag.InspirationID] = append(out[tag.InspirationID], TagAssociationSnapshot{TagID: tag.TagID, Name: tag.Name.String})
	}
	return out, nil
}

func canonicalDBJSON(value sql.NullString) (DBJSON, error) {
	if !value.Valid {
		return DBJSON{Null: true}, nil
	}
	var decoded any
	decoder := json.NewDecoder(strings.NewReader(value.String))
	decoder.UseNumber()
	if err := decoder.Decode(&decoded); err != nil {
		return DBJSON{}, err
	}
	raw, err := json.Marshal(decoded)
	return DBJSON{Value: raw}, err
}

func nullableString(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	v := value.String
	return &v
}

func classifyRepair(plan *CoverRepairPlan, rows []InspirationSnapshot, tags map[uint64][]TagAssociationSnapshot) *CoverRepairStatus {
	status := &CoverRepairStatus{OperationID: plan.OperationID, CheckedAt: time.Now().UTC()}
	byID := make(map[uint64]InspirationSnapshot, len(rows))
	for _, row := range rows {
		byID[row.ID] = row
	}
	for _, item := range plan.Items {
		row, ok := byID[item.Pair.ID]
		if !ok {
			status.ConflictIDs = append(status.ConflictIDs, item.Pair.ID)
			continue
		}
		if reflect.DeepEqual(row, item.Before) && reflect.DeepEqual(tags[item.Pair.ID], item.Tags) {
			status.BeforeIDs = append(status.BeforeIDs, item.Pair.ID)
			continue
		}
		after := item.Before
		after.CoverKey = item.Pair.AfterCoverKey
		if reflect.DeepEqual(row, after) && reflect.DeepEqual(tags[item.Pair.ID], item.Tags) {
			status.AfterIDs = append(status.AfterIDs, item.Pair.ID)
			continue
		}
		status.ConflictIDs = append(status.ConflictIDs, item.Pair.ID)
	}
	switch {
	case len(status.BeforeIDs) == len(plan.Items):
		status.State = CoverRepairBefore
	case len(status.AfterIDs) == len(plan.Items):
		status.State = CoverRepairAfter
	default:
		status.State = CoverRepairConflict
	}
	return status
}

func inspectRepairTarget(ctx context.Context, db *gorm.DB) (DatabaseIdentity, string, error) {
	var identity DatabaseIdentity
	if err := db.WithContext(ctx).Raw(`SELECT @@server_uuid AS server_uuid, @@hostname AS hostname,
		@@port AS port, DATABASE() AS database_name, @@version AS version`).Scan(&identity).Error; err != nil {
		return identity, "", err
	}
	fingerprint, err := digestJSON(struct {
		ServerUUID string `json:"serverUuid"`
		Hostname   string `json:"hostname"`
		Port       uint16 `json:"port"`
		Database   string `json:"database"`
	}{identity.ServerUUID, identity.Hostname, identity.Port, identity.Database})
	if err != nil {
		return identity, "", err
	}
	identity.Fingerprint = fingerprint

	type tableInfo struct{ Name, Engine, TableCollation string }
	var tables []tableInfo
	if err := db.WithContext(ctx).Raw(`SELECT table_name AS name, engine, COALESCE(table_collation, '') AS table_collation FROM information_schema.tables
		WHERE table_schema = DATABASE() AND table_name IN ('inspirations','tags','inspiration_tags') ORDER BY table_name`).Scan(&tables).Error; err != nil {
		return identity, "", err
	}
	if len(tables) != 3 {
		return identity, "", errors.New("repair tables are missing")
	}
	for _, table := range tables {
		if !strings.EqualFold(table.Engine, "InnoDB") {
			return identity, "", fmt.Errorf("table %s is not InnoDB", table.Name)
		}
	}
	var triggerCount int64
	if err := db.WithContext(ctx).Raw(`SELECT COUNT(*) FROM information_schema.triggers
		WHERE trigger_schema = DATABASE() AND event_object_table IN ('inspirations','tags','inspiration_tags')`).Scan(&triggerCount).Error; err != nil {
		return identity, "", err
	}
	if triggerCount != 0 {
		return identity, "", errors.New("repair tables have triggers")
	}
	var columns []repairColumnInfo
	if err := db.WithContext(ctx).Raw(`SELECT table_name, ordinal_position, column_name, column_type, is_nullable,
		COALESCE(column_default, '<NULL>') AS column_default, extra, COALESCE(collation_name, '') AS collation_name
		FROM information_schema.columns WHERE table_schema = DATABASE()
		AND table_name IN ('inspirations','tags','inspiration_tags') ORDER BY table_name, ordinal_position`).Scan(&columns).Error; err != nil {
		return identity, "", err
	}
	if err := validateRepairColumns(columns); err != nil {
		return identity, "", err
	}
	type indexInfo struct {
		TableName, IndexName, ColumnName, Collation, IndexType, Expression string
		NonUnique, SeqInIndex, SubPart                                     uint64
	}
	var indexes []indexInfo
	if err := db.WithContext(ctx).Raw(`SELECT table_name, index_name, non_unique, seq_in_index,
		COALESCE(column_name, '') AS column_name, COALESCE(collation, '') AS collation,
		COALESCE(sub_part, 0) AS sub_part, index_type, COALESCE(expression, '') AS expression
		FROM information_schema.statistics WHERE table_schema = DATABASE()
		AND table_name IN ('inspirations','tags','inspiration_tags') ORDER BY table_name, index_name, seq_in_index`).Scan(&indexes).Error; err != nil {
		return identity, "", err
	}
	type constraintInfo struct{ TableName, ConstraintName, ConstraintType string }
	var constraints []constraintInfo
	if err := db.WithContext(ctx).Raw(`SELECT table_name, constraint_name, constraint_type
		FROM information_schema.table_constraints WHERE table_schema = DATABASE()
		AND table_name IN ('inspirations','tags','inspiration_tags') ORDER BY table_name, constraint_name`).Scan(&constraints).Error; err != nil {
		return identity, "", err
	}
	schemaFingerprint, err := digestJSON(struct {
		Tables      []tableInfo        `json:"tables"`
		Columns     []repairColumnInfo `json:"columns"`
		Indexes     []indexInfo        `json:"indexes"`
		Constraints []constraintInfo   `json:"constraints"`
	}{tables, columns, indexes, constraints})
	return identity, schemaFingerprint, err
}

func validateRepairColumns(columns []repairColumnInfo) error {
	want := map[string][]string{
		"inspirations":     {"id", "type", "title", "content", "source_url", "source_url_hash", "cover_key", "media_keys", "hot_score", "is_favorite", "analysis", "created_at", "updated_at", "deleted_at"},
		"tags":             {"id", "name", "created_at"},
		"inspiration_tags": {"inspiration_id", "tag_id"},
	}
	got := make(map[string][]string, len(want))
	for _, column := range columns {
		got[column.TableName] = append(got[column.TableName], column.ColumnName)
	}
	for table, names := range want {
		if !reflect.DeepEqual(got[table], names) {
			return fmt.Errorf("table %s columns differ from repair contract", table)
		}
	}
	return nil
}

func planDigest(plan *CoverRepairPlan) (string, error) {
	copy := *plan
	copy.Digest = ""
	return digestJSON(copy)
}

func receiptDigest(receipt *CoverRepairReceipt) (string, error) {
	copy := *receipt
	copy.Digest = ""
	return digestJSON(copy)
}

func digestJSON(value any) (string, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}
