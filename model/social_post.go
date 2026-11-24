package model

import (
	"errors"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

// fork: tiktok-webhooks — canonical monotonic status values for SocialPost.
const (
	SocialPostStatusSubmitted   = "submitted"
	SocialPostStatusProcessing  = "processing"
	SocialPostStatusSentToInbox = "sent_to_inbox"
	SocialPostStatusPublished   = "published"
	SocialPostStatusFailed      = "failed"
	SocialPostStatusPartial     = "partial"
	SocialPostStatusDeleted     = "deleted"
)

// SocialPostStateUpdate describes one monotonic status and result_snapshot
// transition from upstream polling or a verified webhook event.
type SocialPostStateUpdate struct {
	TargetStatus                string
	EventName                   string
	EventTime                   int64
	SnapshotFields              map[string]any
	AddPostID                   string
	RemovePostID                string
	Takedown                    bool
	AllowPublishedSnapshotMerge bool
}

// SocialPost is a local ledger of a publishing request (Zernio post mirrored for query + isolation).
type SocialPost struct {
	ID               int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	UserId           int       `json:"user_id" gorm:"not null;index"`
	Provider         string    `json:"provider" gorm:"type:varchar(32);not null;default:'zernio';index"`
	ProviderPostID   string    `json:"provider_post_id" gorm:"type:varchar(64);index"`
	IdempotencyKey   string    `json:"idempotency_key" gorm:"type:varchar(128);index:idx_social_post_idem,priority:2"`
	Status           string    `json:"status" gorm:"type:varchar(32);not null;index;default:'submitted'"`
	ContentPreview   string    `json:"content_preview" gorm:"type:varchar(512)"`
	RequestSnapshot  string    `json:"request_snapshot" gorm:"type:text"`
	ResultSnapshot   string    `json:"result_snapshot" gorm:"type:text"`
	QuotaCharged     int       `json:"quota_charged" gorm:"default:0"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

func (SocialPost) TableName() string {
	return "social_posts"
}

func CreateSocialPost(p *SocialPost) error {
	if p == nil || p.UserId <= 0 {
		return errors.New("invalid social post")
	}
	if p.Provider == "" {
		p.Provider = "zernio"
	}
	return DB.Create(p).Error
}

func GetSocialPostByIDForUser(id int64, userId int) (*SocialPost, error) {
	var p SocialPost
	err := DB.Where("id = ? AND user_id = ?", id, userId).First(&p).Error
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func GetSocialPostByIdempotency(userId int, key string) (*SocialPost, error) {
	if key == "" {
		return nil, gorm.ErrRecordNotFound
	}
	var p SocialPost
	err := DB.Where("user_id = ? AND idempotency_key = ?", userId, key).First(&p).Error
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func GetSocialPostByProviderPostID(provider, providerPostID string) (*SocialPost, error) {
	var p SocialPost
	err := DB.Where("provider = ? AND provider_post_id = ?", provider, providerPostID).First(&p).Error
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func UpdateSocialPostResult(id int64, status, providerPostID, resultSnapshot string) error {
	updates := map[string]any{
		"status":     status,
		"updated_at": time.Now(),
	}
	if providerPostID != "" {
		updates["provider_post_id"] = providerPostID
	}
	if resultSnapshot != "" {
		updates["result_snapshot"] = resultSnapshot
	}
	return DB.Model(&SocialPost{}).Where("id = ?", id).Updates(updates).Error
}

// ApplySocialPostStateUpdateByID locks the SocialPost row in a transaction,
// computes the monotonic next status and merged result_snapshot, and writes
// back only when either field actually changes.
// fork: tiktok-webhooks
func ApplySocialPostStateUpdateByID(id int64, upd SocialPostStateUpdate) (bool, *SocialPost, error) {
	if id <= 0 {
		return false, nil, gorm.ErrRecordNotFound
	}
	return applySocialPostStateUpdate(func(tx *gorm.DB) *gorm.DB {
		return lockForUpdate(tx).Where("id = ?", id)
	}, upd)
}

// ApplySocialPostStateUpdateByProviderPostID looks up and locks a SocialPost row
// by (provider, provider_post_id) in a transaction and applies upd monotonically.
// fork: tiktok-webhooks
func ApplySocialPostStateUpdateByProviderPostID(provider, providerPostID string, upd SocialPostStateUpdate) (bool, *SocialPost, error) {
	provider = strings.TrimSpace(provider)
	providerPostID = strings.TrimSpace(providerPostID)
	if provider == "" || providerPostID == "" {
		return false, nil, gorm.ErrRecordNotFound
	}
	return applySocialPostStateUpdate(func(tx *gorm.DB) *gorm.DB {
		return lockForUpdate(tx).Where("provider = ? AND provider_post_id = ?", provider, providerPostID)
	}, upd)
}

func applySocialPostStateUpdate(query func(tx *gorm.DB) *gorm.DB, upd SocialPostStateUpdate) (bool, *SocialPost, error) {
	if DB == nil {
		return false, nil, errors.New("database is not initialized")
	}
	var (
		post    SocialPost
		updated bool
	)
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := query(tx).First(&post).Error; err != nil {
			return err
		}
		nextStatus, nextSnapshot, changed, err := computeSocialPostStateUpdate(&post, upd)
		if err != nil {
			return err
		}
		if !changed {
			return nil
		}
		now := time.Now()
		res := tx.Model(&SocialPost{}).
			Where("id = ?", post.ID).
			Updates(map[string]any{
				"status":          nextStatus,
				"result_snapshot": nextSnapshot,
				"updated_at":      now,
			})
		// Never treat MySQL RowsAffected == 0 (identical column values) as an error;
		// only genuine DB failures should abort the transaction.
		if res.Error != nil {
			return res.Error
		}
		post.Status = nextStatus
		post.ResultSnapshot = nextSnapshot
		post.UpdatedAt = now
		updated = true
		return nil
	})
	if err != nil {
		return false, nil, err
	}
	return updated, &post, nil
}

func computeSocialPostStateUpdate(current *SocialPost, upd SocialPostStateUpdate) (string, string, bool, error) {
	cur := strings.ToLower(strings.TrimSpace(current.Status))
	if cur == SocialPostStatusFailed || cur == SocialPostStatusPartial || cur == SocialPostStatusDeleted {
		return current.Status, current.ResultSnapshot, false, nil
	}

	snap := parseSocialPostSnapshot(current.ResultSnapshot)
	postIDs := snapshotStringList(snap, "post_ids")
	unavailableIDs := snapshotStringList(snap, "unavailable_post_ids")

	var nextStatus string
	if upd.Takedown {
		removeID := strings.TrimSpace(upd.RemovePostID)
		if removeID != "" {
			unavailableIDs = appendUniqueString(unavailableIDs, removeID)
		}
		surviving := survivingPostIDs(postIDs, unavailableIDs)
		keepPublished := cur == SocialPostStatusPublished && len(surviving) > 0 && (removeID != "" || len(postIDs) > 1)
		mergeSnapshotFields(snap, upd.SnapshotFields)
		if len(postIDs) > 0 {
			snap["post_ids"] = postIDs
		}
		if len(unavailableIDs) > 0 {
			snap["unavailable_post_ids"] = unavailableIDs
		}
		applySnapshotEventMeta(snap, upd.EventName, upd.EventTime)
		if keepPublished {
			nextStatus = SocialPostStatusPublished
		} else {
			nextStatus = SocialPostStatusDeleted
			snap["status"] = "DELETED"
		}
	} else {
		target := strings.ToLower(strings.TrimSpace(upd.TargetStatus))
		if target == "" {
			return current.Status, current.ResultSnapshot, false, nil
		}
		if cur == SocialPostStatusPublished {
			if target != SocialPostStatusPublished || !upd.AllowPublishedSnapshotMerge {
				return current.Status, current.ResultSnapshot, false, nil
			}
			nextStatus = SocialPostStatusPublished
		} else if isSocialPostTerminalTarget(target) {
			nextStatus = target
		} else {
			curRank, curOK := socialPostNonTerminalRank(cur)
			targetRank, targetOK := socialPostNonTerminalRank(target)
			if !curOK || !targetOK || targetRank < curRank {
				return current.Status, current.ResultSnapshot, false, nil
			}
			nextStatus = target
		}

		if addID := strings.TrimSpace(upd.AddPostID); addID != "" {
			postIDs = appendUniqueString(postIDs, addID)
		}
		mergeSnapshotFields(snap, upd.SnapshotFields)
		if len(postIDs) > 0 {
			snap["post_ids"] = postIDs
		}
		if len(unavailableIDs) > 0 {
			snap["unavailable_post_ids"] = unavailableIDs
		}
		applySnapshotEventMeta(snap, upd.EventName, upd.EventTime)
	}

	nextSnapshot := current.ResultSnapshot
	if len(snap) > 0 {
		raw, err := common.Marshal(snap)
		if err != nil {
			return current.Status, current.ResultSnapshot, false, err
		}
		nextSnapshot = string(raw)
	}
	if nextStatus == current.Status && nextSnapshot == current.ResultSnapshot {
		return current.Status, current.ResultSnapshot, false, nil
	}
	return nextStatus, nextSnapshot, true, nil
}

func isSocialPostTerminalTarget(status string) bool {
	switch status {
	case SocialPostStatusPublished, SocialPostStatusFailed, SocialPostStatusPartial, SocialPostStatusDeleted:
		return true
	default:
		return false
	}
}

func socialPostNonTerminalRank(status string) (int, bool) {
	switch status {
	case "":
		return 0, true
	case SocialPostStatusSubmitted:
		return 1, true
	case SocialPostStatusProcessing:
		return 2, true
	case SocialPostStatusSentToInbox:
		return 3, true
	default:
		return 0, false
	}
}

func parseSocialPostSnapshot(raw string) map[string]any {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return map[string]any{}
	}
	var m map[string]any
	if err := common.Unmarshal([]byte(trimmed), &m); err != nil || m == nil {
		return map[string]any{}
	}
	return m
}

func mergeSnapshotFields(dst, src map[string]any) {
	for k, v := range src {
		if k == "" || k == "post_ids" || k == "unavailable_post_ids" {
			continue
		}
		dst[k] = v
	}
}

func snapshotStringList(snap map[string]any, key string) []string {
	raw, ok := snap[key]
	if !ok || raw == nil {
		return nil
	}
	var out []string
	switch v := raw.(type) {
	case []string:
		for _, item := range v {
			out = appendUniqueString(out, item)
		}
	case []any:
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = appendUniqueString(out, s)
			}
		}
	}
	return out
}

func appendUniqueString(list []string, item string) []string {
	item = strings.TrimSpace(item)
	if item == "" {
		return list
	}
	for _, existing := range list {
		if existing == item {
			return list
		}
	}
	return append(list, item)
}

func survivingPostIDs(postIDs, unavailableIDs []string) []string {
	if len(postIDs) == 0 {
		return nil
	}
	removed := make(map[string]struct{}, len(unavailableIDs))
	for _, id := range unavailableIDs {
		removed[id] = struct{}{}
	}
	var surviving []string
	for _, id := range postIDs {
		if _, gone := removed[id]; !gone {
			surviving = append(surviving, id)
		}
	}
	return surviving
}

func snapshotInt64(snap map[string]any, key string) int64 {
	raw, ok := snap[key]
	if !ok || raw == nil {
		return 0
	}
	switch v := raw.(type) {
	case int64:
		return v
	case int:
		return int64(v)
	case float64:
		return int64(v)
	default:
		return 0
	}
}

func applySnapshotEventMeta(snap map[string]any, eventName string, eventTime int64) {
	eventName = strings.TrimSpace(eventName)
	if eventName == "" && eventTime <= 0 {
		return
	}
	existingTime := snapshotInt64(snap, "last_event_time")
	if eventTime > 0 {
		if existingTime > 0 && eventTime < existingTime {
			return
		}
		snap["last_event_time"] = eventTime
		if eventName != "" {
			snap["last_event"] = eventName
		}
		return
	}
	if existingTime == 0 && eventName != "" {
		snap["last_event"] = eventName
	}
}
