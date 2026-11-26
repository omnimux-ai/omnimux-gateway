// Package service 灵感条目与标签的业务逻辑层。
package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"

	"omnimux-inspiration/internal/errs"
	"omnimux-inspiration/internal/media"
	"omnimux-inspiration/internal/model"
	"omnimux-inspiration/internal/store"
)

type CoverStore interface {
	SaveURL(ctx context.Context, kind, rawURL string) (*media.Stored, error)
}

type Service struct {
	DB             *store.DB
	MediaURLPrefix string
	coverStore     CoverStore
}

func New(db *store.DB, mediaURLPrefix string, coverStore CoverStore) *Service {
	return &Service{DB: db, MediaURLPrefix: mediaURLPrefix, coverStore: coverStore}
}

// ---------- 去重 hash ----------

var trailingSlashRe = regexp.MustCompile(`/+$`)

// NormalizeURL 归一化 URL：去空白、去尾斜杠、去常见跟踪参数、小写 host。
func NormalizeURL(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" {
		return strings.TrimSpace(raw)
	}
	u.Host = strings.ToLower(u.Host)
	q := u.Query()
	for k := range q {
		if strings.HasPrefix(k, "utm_") || k == "share_id" || k == "from" || k == "is_from_webapp" {
			q.Del(k)
		}
	}
	u.RawQuery = q.Encode()
	u.Fragment = ""
	p := u.Path
	u.Path = trailingSlashRe.ReplaceAllString(p, "")
	return u.String()
}

func HashURL(raw string) string {
	h := sha256.Sum256([]byte(NormalizeURL(raw)))
	return hex.EncodeToString(h[:])
}

// ---------- 请求/响应 DTO ----------

type CreateInspirationReq struct {
	Type        string     `json:"type"`
	Title       string     `json:"title"`
	Content     string     `json:"content"`
	SourceURL   string     `json:"source_url"`
	CoverKey    string     `json:"cover_key"`
	MediaKeys   []string   `json:"media_keys"`
	HotScore    *int       `json:"hot_score"`
	IsFavorite  bool       `json:"is_favorite"`
	Analysis    any        `json:"analysis"`
	Tags        []string   `json:"tags"`
	CountryCode string     `json:"country_code"`
	Category    string     `json:"category"`
	Duration    int        `json:"duration"`
	Views       int64      `json:"views"`
	TrafficType string     `json:"traffic_type"`
	PostedAt    *time.Time `json:"posted_at"`
}

type UpdateInspirationReq struct {
	Title       *string      `json:"title"`
	Content     *string      `json:"content"`
	CoverKey    *string      `json:"cover_key"`
	MediaKeys   []string     `json:"media_keys"`
	HotScore    *int         `json:"hot_score"`
	IsFavorite  *bool        `json:"is_favorite"`
	Analysis    any          `json:"analysis"`
	Tags        []string     `json:"tags"`
	CountryCode *string      `json:"country_code"`
	Category    *string      `json:"category"`
	Duration    *int         `json:"duration"`
	Views       *int64       `json:"views"`
	TrafficType *string      `json:"traffic_type"`
	PostedAt    OptionalTime `json:"posted_at"`
}

type BulkReq struct {
	Items []CreateInspirationReq `json:"items"`
}

type BulkResult struct {
	Created int              `json:"created"`
	Updated int              `json:"updated"`
	Skipped int              `json:"skipped"`
	Items   []BulkItemResult `json:"items"`
}

type BulkItemResult struct {
	Index   int    `json:"index"`
	Action  string `json:"action"` // created|updated|skipped
	ID      uint64 `json:"id,omitempty"`
	Existed bool   `json:"existed,omitempty"`
}

type ListQuery struct {
	Type                      string
	Tag                       string
	Tags                      []string
	Q                         string
	IsFavorite                *bool
	Sort                      string
	Page, Size                int
	Country                   string
	Category                  string
	DurationMin, DurationMax  *int
	ViewsMin, ViewsMax        *int64
	TrafficType               string
	PostedAfter, PostedBefore *time.Time
}

type ListResult struct {
	Total int64               `json:"total"`
	Page  int                 `json:"page"`
	Size  int                 `json:"size"`
	Items []model.Inspiration `json:"items"`
}

// ---------- CRUD ----------

var validTypes = map[string]bool{"video": true, "image": true, "link": true}

func (s *Service) Create(ctx context.Context, req *CreateInspirationReq, returnExisting bool) (*model.Inspiration, bool, error) {
	t, title, hash, err := validateCreate(req)
	if err != nil {
		return nil, false, err
	}
	exist, err := s.findBySourceHash(ctx, hash)
	if err != nil {
		return nil, false, err
	}
	if exist != nil {
		if returnExisting {
			s.attachTags(ctx, []*model.Inspiration{exist})
			return exist, true, nil
		}
		return nil, true, errs.ErrDuplicateURL
	}
	coverKey, err := s.materializeCover(ctx, req.CoverKey)
	if err != nil {
		return nil, false, err
	}

	rec := &model.Inspiration{
		Type: t, Title: title, Content: req.Content, SourceURL: req.SourceURL,
		SourceURLHash: hash, CoverKey: coverKey, HotScore: deref(req.HotScore),
		IsFavorite:  req.IsFavorite,
		CountryCode: req.CountryCode, Category: req.Category, Duration: req.Duration,
		Views: req.Views, TrafficType: req.TrafficType, PostedAt: normalizedPostedAt(req.PostedAt),
	}
	rec.MediaKeys = mustJSON(req.MediaKeys)
	if req.Analysis != nil {
		rec.Analysis = mustJSON(req.Analysis)
	}
	if err := s.DB.WithContext(ctx).Create(rec).Error; err != nil {
		if isDupKeyErr(err) {
			if returnExisting {
				exist, lookupErr := s.findBySourceHash(ctx, hash)
				if lookupErr != nil {
					return nil, false, lookupErr
				}
				if exist != nil {
					return exist, true, nil
				}
			}
			return nil, true, errs.ErrDuplicateURL
		}
		return nil, false, errs.Wrap("INTERNAL", 500, "create inspiration", err)
	}
	if len(req.Tags) > 0 {
		if err := s.replaceTags(ctx, rec.ID, req.Tags); err != nil {
			return nil, false, err
		}
	}
	s.attachTags(ctx, []*model.Inspiration{rec})
	return rec, false, nil
}

func validateCreate(req *CreateInspirationReq) (inspirationType, title, hash string, err error) {
	t := strings.ToLower(strings.TrimSpace(req.Type))
	if t == "" {
		t = "link"
	}
	if !validTypes[t] {
		return "", "", "", errs.ErrInvalidParam
	}
	title = strings.TrimSpace(req.Title)
	if title == "" {
		return "", "", "", errs.ErrInvalidParam
	}
	if !validFilterFields(req.CountryCode, req.Category, req.TrafficType, req.Duration, req.Views, req.PostedAt) {
		return "", "", "", errs.ErrInvalidParam
	}
	return t, title, HashURL(req.SourceURL), nil
}

func (s *Service) findBySourceHash(ctx context.Context, hash string) (*model.Inspiration, error) {
	// 查重（软删记录也占坑，防复活脏数据）
	var exist model.Inspiration
	err := s.DB.WithContext(ctx).Where("source_url_hash = ?", hash).First(&exist).Error
	if err == nil {
		return &exist, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return nil, errs.Wrap("INTERNAL", 500, "query duplicate", err)
}

func (s *Service) Get(ctx context.Context, id uint64) (*model.Inspiration, error) {
	var rec model.Inspiration
	if err := s.DB.WithContext(ctx).First(&rec, id).Error; err != nil {
		return nil, mapDBErr(err)
	}
	s.attachTags(ctx, []*model.Inspiration{&rec})
	return &rec, nil
}

func (s *Service) Update(ctx context.Context, id uint64, req *UpdateInspirationReq) (*model.Inspiration, error) {
	var rec model.Inspiration
	if err := s.DB.WithContext(ctx).First(&rec, id).Error; err != nil {
		return nil, mapDBErr(err)
	}
	updates, err := req.filterUpdates()
	if err != nil {
		return nil, err
	}
	if req.Title != nil {
		v := strings.TrimSpace(*req.Title)
		if v == "" {
			return nil, errs.ErrInvalidParam
		}
		updates["title"] = v
	}
	if req.Content != nil {
		updates["content"] = *req.Content
	}
	if req.CoverKey != nil {
		coverKey, err := s.materializeCover(ctx, *req.CoverKey)
		if err != nil {
			return nil, err
		}
		updates["cover_key"] = coverKey
	}
	if req.HotScore != nil {
		updates["hot_score"] = *req.HotScore
	}
	if req.IsFavorite != nil {
		updates["is_favorite"] = *req.IsFavorite
	}
	if req.MediaKeys != nil {
		updates["media_keys"] = mustJSON(req.MediaKeys)
	}
	if req.Analysis != nil {
		updates["analysis"] = mustJSON(req.Analysis)
	}
	if len(updates) > 0 {
		if err := s.DB.WithContext(ctx).Model(&rec).Updates(updates).Error; err != nil {
			return nil, errs.Wrap("INTERNAL", 500, "update inspiration", err)
		}
	}
	if req.Tags != nil {
		if err := s.replaceTags(ctx, id, req.Tags); err != nil {
			return nil, err
		}
	}
	return s.Get(ctx, id)
}

func (s *Service) Delete(ctx context.Context, id uint64) error {
	res := s.DB.WithContext(ctx).Delete(&model.Inspiration{}, id)
	if res.Error != nil {
		return errs.Wrap("INTERNAL", 500, "delete inspiration", res.Error)
	}
	if res.RowsAffected == 0 {
		return errs.ErrNotFound
	}
	return nil
}

// ---------- 列表/搜索 ----------

func (s *Service) List(ctx context.Context, q *ListQuery) (*ListResult, error) {
	if err := q.validateFilters(); err != nil {
		return nil, err
	}
	page, size := q.Page, q.Size
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}

	tx := s.DB.WithContext(ctx).Model(&model.Inspiration{})
	if q.Type != "" {
		tx = tx.Where("type = ?", q.Type)
	}
	if q.IsFavorite != nil {
		tx = tx.Where("is_favorite = ?", *q.IsFavorite)
	}
	if q.Country != "" {
		tx = tx.Where("country_code = ?", q.Country)
	}
	if q.Category != "" {
		tx = tx.Where("category = ?", q.Category)
	}
	if q.TrafficType != "" {
		tx = tx.Where("traffic_type = ?", q.TrafficType)
	}
	if q.DurationMin != nil {
		tx = tx.Where("duration >= ?", *q.DurationMin)
	}
	if q.DurationMax != nil {
		tx = tx.Where("duration <= ?", *q.DurationMax)
	}
	if q.ViewsMin != nil {
		tx = tx.Where("views >= ?", *q.ViewsMin)
	}
	if q.ViewsMax != nil {
		tx = tx.Where("views <= ?", *q.ViewsMax)
	}
	if q.PostedAfter != nil {
		lower := q.PostedAfter.UTC().Truncate(time.Millisecond)
		if lower.Before(*q.PostedAfter) {
			lower = lower.Add(time.Millisecond)
		}
		if lower.Year() > 9999 {
			tx = tx.Where("1 = 0")
		} else {
			tx = tx.Where("posted_at >= ?", model.PostedAtValue(&lower))
		}
	}
	if q.PostedBefore != nil {
		tx = tx.Where("posted_at <= ?", model.PostedAtValue(q.PostedBefore))
	}
	tagFilter := q.Tags
	if q.Tag != "" && len(tagFilter) == 0 {
		tagFilter = []string{q.Tag}
	}
	if len(tagFilter) > 0 {
		sub := s.DB.Session(&gorm.Session{NewDB: true}).
			Table("inspiration_tags it").
			Select("it.inspiration_id").
			Joins("JOIN tags t ON t.id = it.tag_id").
			Where("t.name IN ?", tagFilter).
			Having("COUNT(DISTINCT t.name) = ?", len(tagFilter)).
			Group("it.inspiration_id")
		tx = tx.Where("id IN (?)", sub)
	}
	if kw := strings.TrimSpace(q.Q); kw != "" {
		if s.DB.Dialect == store.DialectMySQL {
			tx = tx.Where("MATCH(title, content) AGAINST(? IN NATURAL LANGUAGE MODE)", kw)
		} else {
			p := "%" + kw + "%"
			tx = tx.Where("(title LIKE ? OR content LIKE ?)", p, p)
		}
	}

	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, errs.Wrap("INTERNAL", 500, "count", err)
	}
	switch q.Sort {
	case "new", "":
		tx = tx.Order("id DESC")
	case "views":
		tx = tx.Order("views DESC, id DESC")
	case "fav":
		tx = tx.Order("is_favorite DESC, updated_at DESC")
	default: // hot
		tx = tx.Order("hot_score DESC, id DESC")
	}
	var items []model.Inspiration
	if err := tx.Offset((page - 1) * size).Limit(size).Find(&items).Error; err != nil {
		return nil, errs.Wrap("INTERNAL", 500, "list", err)
	}
	s.attachTags(ctx, itemsPtr(items))
	return &ListResult{Total: total, Page: page, Size: size, Items: items}, nil
}

// itemsPtr 把 []model.Inspiration 转成指针切片，供 attachTags 原地填充。
func itemsPtr(items []model.Inspiration) []*model.Inspiration {
	ptr := make([]*model.Inspiration, len(items))
	for i := range items {
		ptr[i] = &items[i]
	}
	return ptr
}

// ---------- Bulk ----------

const bulkMax = 200

type preparedBulkItem struct {
	req CreateInspirationReq
	err error
}

// BulkUpsertTx 事务版（脚本导入主通道）。
func (s *Service) BulkUpsertTx(ctx context.Context, req *BulkReq) (*BulkResult, error) {
	if len(req.Items) > bulkMax {
		return nil, errs.ErrBulkLimit
	}
	prepared := s.prepareBulk(ctx, req.Items)
	var out *BulkResult
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txSvc := &Service{
			DB:             &store.DB{DB: tx, Dialect: s.DB.Dialect},
			MediaURLPrefix: s.MediaURLPrefix,
		}
		out = txSvc.writePreparedBulk(ctx, prepared)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Service) prepareBulk(ctx context.Context, items []CreateInspirationReq) []preparedBulkItem {
	prepared := make([]preparedBulkItem, len(items))
	coverByHash := make(map[string]string, len(items))
	for i := range items {
		prepared[i].req = items[i]
		_, _, hash, err := validateCreate(&prepared[i].req)
		if err != nil {
			prepared[i].err = err
			continue
		}
		if coverKey, ok := coverByHash[hash]; ok {
			prepared[i].req.CoverKey = coverKey
			continue
		}
		exist, err := s.findBySourceHash(ctx, hash)
		if err != nil {
			prepared[i].err = err
			continue
		}
		if exist != nil {
			prepared[i].req.CoverKey = exist.CoverKey
			coverByHash[hash] = exist.CoverKey
			continue
		}
		coverKey, err := s.materializeCover(ctx, prepared[i].req.CoverKey)
		if err != nil {
			prepared[i].err = err
			continue
		}
		prepared[i].req.CoverKey = coverKey
		coverByHash[hash] = coverKey
	}
	return prepared
}

func (s *Service) writePreparedBulk(ctx context.Context, items []preparedBulkItem) *BulkResult {
	res := &BulkResult{Items: make([]BulkItemResult, 0, len(items))}
	for i := range items {
		item := &items[i]
		r := BulkItemResult{Index: i}
		if item.err != nil {
			r.Action = "skipped"
			res.Skipped++
			res.Items = append(res.Items, r)
			continue
		}
		rec, existed, err := s.Create(ctx, &item.req, true)
		switch {
		case err != nil:
			r.Action = "skipped"
			res.Skipped++
		case existed:
			r.Action, r.Existed = "updated", true
			r.ID = rec.ID
			res.Updated++
		default:
			r.Action, r.ID = "created", rec.ID
			res.Created++
		}
		res.Items = append(res.Items, r)
	}
	return res
}

// ---------- Tags ----------

func (s *Service) ListTags(ctx context.Context) ([]model.Tag, error) {
	var tags []model.Tag
	if err := s.DB.WithContext(ctx).
		Table("tags t").
		Select("t.id, t.name, t.created_at, COUNT(it.inspiration_id) AS count").
		Joins("LEFT JOIN inspiration_tags it ON it.tag_id = t.id").
		Group("t.id, t.name, t.created_at").
		Order("count DESC, t.name ASC").
		Find(&tags).Error; err != nil {
		return nil, errs.Wrap("INTERNAL", 500, "list tags", err)
	}
	return tags, nil
}

func (s *Service) CreateTag(ctx context.Context, name string) (*model.Tag, error) {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 128 {
		return nil, errs.ErrInvalidParam
	}
	rec := &model.Tag{Name: name}
	err := s.DB.WithContext(ctx).Create(rec).Error
	if err != nil {
		if isDupKeyErr(err) {
			return nil, errs.ErrDuplicateTag
		}
		return nil, errs.Wrap("INTERNAL", 500, "create tag", err)
	}
	return rec, nil
}

func (s *Service) DeleteTag(ctx context.Context, id uint64) error {
	return s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Delete(&model.Tag{}, id)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return errs.ErrNotFound
		}
		return tx.Exec("DELETE FROM inspiration_tags WHERE tag_id = ?", id).Error
	})
}

// ---------- helpers ----------

func (s *Service) materializeCover(ctx context.Context, coverKey string) (string, error) {
	if strings.HasPrefix(coverKey, media.CoverRefPrefix) {
		if _, err := media.ParseCoverRef(coverKey); err != nil {
			return "", err
		}
		return coverKey, nil
	}
	raw := strings.TrimSpace(coverKey)
	if raw == "" {
		return coverKey, nil
	}
	u, err := url.Parse(raw)
	if err != nil {
		if strings.Contains(raw, "://") || strings.HasPrefix(raw, "//") {
			return "", errs.ErrInvalidParam
		}
		return coverKey, nil
	}
	if u.Scheme == "" {
		if u.Host != "" || strings.HasPrefix(raw, "//") {
			return "", errs.ErrInvalidParam
		}
		return coverKey, nil
	}
	if (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", errs.ErrInvalidParam
	}
	if s.coverStore == nil {
		return "", errs.Wrap("MEDIA_DISABLED", 503, "media storage disabled", nil)
	}
	stored, err := s.coverStore.SaveURL(ctx, "cover", raw)
	if err != nil {
		return "", err
	}
	if stored == nil || stored.Key == "" {
		return "", errs.Wrap("INTERNAL", 500, "media storage returned empty key", nil)
	}
	return stored.Key, nil
}

func (s *Service) replaceTags(ctx context.Context, inspirationID uint64, names []string) error {
	names = normalizeNames(names)
	if len(names) == 0 {
		return s.DB.WithContext(ctx).Exec("DELETE FROM inspiration_tags WHERE inspiration_id = ?", inspirationID).Error
	}
	return s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("DELETE FROM inspiration_tags WHERE inspiration_id = ?", inspirationID).Error; err != nil {
			return err
		}
		var tagIDs []uint64
		for _, n := range names {
			var tag model.Tag
			err := tx.Where("name = ?", n).First(&tag).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				tag = model.Tag{Name: n}
				if err := tx.Create(&tag).Error; err != nil && !isDupKeyErr(err) {
					return err
				}
				// 并发下可能已被创建，回读
				if err := tx.Where("name = ?", n).First(&tag).Error; err != nil {
					return err
				}
			} else if err != nil {
				return err
			}
			tagIDs = append(tagIDs, tag.ID)
		}
		for _, tid := range tagIDs {
			if s.DB.Dialect == store.DialectMySQL {
				if err := tx.Exec("INSERT IGNORE INTO inspiration_tags (inspiration_id, tag_id) VALUES (?, ?)", inspirationID, tid).Error; err != nil {
					return err
				}
			} else {
				if err := tx.Exec("INSERT INTO inspiration_tags (inspiration_id, tag_id) VALUES (?, ?) ON CONFLICT(inspiration_id, tag_id) DO NOTHING", inspirationID, tid).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func normalizeNames(in []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s != "" && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}

// attachTags 批量填充 tags 字段。
func (s *Service) attachTags(ctx context.Context, recs []*model.Inspiration) {
	if len(recs) == 0 {
		return
	}
	ids := make([]uint64, len(recs))
	for i, r := range recs {
		ids[i] = r.ID
	}
	type row struct {
		InspirationID uint64
		Name          string
	}
	var rows []row
	s.DB.WithContext(ctx).
		Table("inspiration_tags it").
		Select("it.inspiration_id, t.name").
		Joins("JOIN tags t ON t.id = it.tag_id").
		Where("it.inspiration_id IN ?", ids).
		Find(&rows)
	m := map[uint64][]string{}
	for _, r := range rows {
		m[r.InspirationID] = append(m[r.InspirationID], r.Name)
	}
	for _, r := range recs {
		r.Tags = m[r.ID]
		if r.Tags == nil {
			r.Tags = []string{}
		}
	}
}

func mustJSON(v any) datatypes.JSON {
	b, _ := json.Marshal(v)
	return datatypes.JSON(b)
}

func deref(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}

func isDupKeyErr(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "duplicate entry") ||
		strings.Contains(msg, "unique constraint") ||
		strings.Contains(msg, "constraint failed")
}

func mapDBErr(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return errs.ErrNotFound
	}
	return errs.Wrap("INTERNAL", 500, "db error", err)
}

// CoverReference resolves only an active record by primary key.
func (s *Service) CoverReference(ctx context.Context, id uint64) (string, error) {
	var row struct{ CoverKey string }
	result := s.DB.WithContext(ctx).Table("inspirations").Select("cover_key").Where("id = ? AND deleted_at IS NULL", id).Take(&row)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return "", media.ErrCoverNotFound
	}
	if result.Error != nil {
		return "", errs.ErrDBUnavail
	}
	if !strings.HasPrefix(row.CoverKey, media.CoverRefPrefix) {
		return "", media.ErrCoverNotFound
	}
	if _, err := media.ParseCoverRef(row.CoverKey); err != nil {
		return "", err
	}
	return row.CoverKey, nil
}
