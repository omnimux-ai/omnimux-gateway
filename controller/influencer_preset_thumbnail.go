package controller

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/storage"
	"github.com/QuantumNous/new-api/service"
	"gorm.io/gorm"
)

// fork: image-thumbnails; queue identifiers rather than retaining original image bytes.
var influencerPresetThumbnailQueue = struct {
	sync.Mutex
	once    sync.Once
	pending map[int64]bool
	jobs    chan int64
}{pending: make(map[int64]bool), jobs: make(chan int64, 32)}

// Shared by deletion and thumbnail writes so a concurrent withdrawal cannot leave a derived object behind.
var influencerPresetThumbnailLocks [64]sync.Mutex

// The seam lets handler tests disable background work while exercising publish semantics.
var enqueueInfluencerPresetThumbnail = queueInfluencerPresetThumbnail
var generateInfluencerPresetThumbnailImage = service.GenerateImageThumbnail

func influencerPresetThumbnailReady(preset *model.InfluencerPreset) bool {
	return preset != nil && preset.ThumbnailKey != "" && preset.ThumbnailMime == "image/webp" && preset.ThumbnailWidth > 0 && preset.ThumbnailHeight > 0
}

// An attempt is live for at most its 30-second processing budget. Persisting
// the lease keeps the final running attempt distinct from exhausted failures.
func influencerPresetThumbnailProcessing(preset *model.InfluencerPreset) bool {
	return preset != nil && !influencerPresetThumbnailReady(preset) && preset.ThumbnailAttemptedAt > time.Now().Add(-30*time.Second).UnixMilli()
}

func queueInfluencerPresetThumbnail(id int64) bool {
	if id <= 0 {
		return false
	}
	q := &influencerPresetThumbnailQueue
	q.once.Do(func() {
		for i := 0; i < 2; i++ {
			go runInfluencerPresetThumbnailWorker()
		}
	})
	q.Lock()
	defer q.Unlock()
	if q.pending[id] {
		return false
	}
	select {
	case q.jobs <- id:
		q.pending[id] = true
		return true
	default:
		return false
	}
}

func runInfluencerPresetThumbnailWorker() {
	for id := range influencerPresetThumbnailQueue.jobs {
		for attempt := 0; attempt < 3; attempt++ {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			err := generateInfluencerPresetThumbnail(ctx, id)
			cancel()
			if err == nil {
				break
			}
			common.SysError(fmt.Sprintf("influencer preset thumbnail %d attempt %d failed: %v", id, attempt+1, err))
		}
		influencerPresetThumbnailQueue.Lock()
		delete(influencerPresetThumbnailQueue.pending, id)
		influencerPresetThumbnailQueue.Unlock()
	}
}

func generateInfluencerPresetThumbnail(ctx context.Context, id int64) error {
	lock := &influencerPresetThumbnailLocks[uint64(id)%uint64(len(influencerPresetThumbnailLocks))]
	lock.Lock()
	defer lock.Unlock()
	preset, err := model.GetInfluencerPreset(id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if influencerPresetThumbnailReady(preset) || influencerPresetThumbnailProcessing(preset) || preset.ThumbnailAttempts >= 3 {
		return nil
	}
	attemptedAt := time.Now().UnixMilli()
	attempt := model.DB.WithContext(ctx).Model(&model.InfluencerPreset{}).Where("id = ? AND preview_key = ? AND (thumbnail_attempts < ? OR thumbnail_attempts IS NULL) AND (thumbnail_attempted_at IS NULL OR thumbnail_attempted_at <= ?)", id, preset.PreviewKey, 3, attemptedAt-30000).Updates(map[string]interface{}{"thumbnail_attempts": gorm.Expr("COALESCE(thumbnail_attempts, 0) + 1"), "thumbnail_attempted_at": attemptedAt})
	if attempt.Error != nil {
		return attempt.Error
	}
	if attempt.RowsAffected != 1 {
		return nil
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := model.DB.WithContext(cleanup).Model(&model.InfluencerPreset{}).Where("id = ? AND preview_key = ? AND thumbnail_attempted_at = ?", id, preset.PreviewKey, attemptedAt).Update("thumbnail_attempted_at", 0).Error; err != nil {
			common.SysError("influencer preset thumbnail lease cleanup failed: " + err.Error())
		}
	}()
	driver := storage.GetStorageDriver()
	if driver == nil {
		return errors.New("no storage backend")
	}
	reader, _, size, err := driver.Get(ctx, preset.PreviewKey)
	if err != nil {
		return err
	}
	defer reader.Close()
	if size > influencerPresetMaxAssetBytes {
		return errors.New("preset source exceeds image limit")
	}
	data, err := io.ReadAll(io.LimitReader(reader, influencerPresetMaxAssetBytes+1))
	if err != nil {
		return err
	}
	if len(data) > influencerPresetMaxAssetBytes {
		return errors.New("preset source exceeds image limit")
	}
	thumbnail, err := generateInfluencerPresetThumbnailImage(ctx, data)
	if err != nil {
		return err
	}
	key := influencerPresetKeyPrefix + influencerPresetDigest([]byte(preset.SourceTaskID)) + fmt.Sprintf("/thumbnail-v1-%d-", preset.Id) + influencerPresetDigest(thumbnail.Data) + ".webp"
	if _, err = driver.Upload(ctx, key, thumbnail.Data, "image/webp"); err != nil {
		return err
	}
	result := model.DB.WithContext(ctx).Model(&model.InfluencerPreset{}).Where("id = ? AND preview_key = ? AND (thumbnail_key = ? OR thumbnail_key IS NULL) AND thumbnail_attempted_at = ?", id, preset.PreviewKey, preset.ThumbnailKey, attemptedAt).Updates(map[string]interface{}{"thumbnail_key": key, "thumbnail_mime": "image/webp", "thumbnail_width": thumbnail.Width, "thumbnail_height": thumbnail.Height})
	if result.Error != nil || result.RowsAffected != 1 {
		// Another instance may already have committed these content-addressed bytes.
		// Never remove an object while any row references it.
		var references int64
		if err := model.DB.Model(&model.InfluencerPreset{}).Where("thumbnail_key = ?", key).Count(&references).Error; err != nil || references != 0 {
			return result.Error
		}
		// Cleanup has its own budget: request expiration must not pin an orphan object.
		cleanup, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		influencerPresetDeleteObjects(cleanup, driver, []string{key})
		return result.Error
	}
	return nil
}
