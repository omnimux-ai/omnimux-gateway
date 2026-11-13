package controller

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

// Backfill only inspects by default. Apply enqueues existing originals; it never
// submits generation requests or changes task status or quota.
type imageThumbnailBatchRequest struct {
	Scope  string `json:"scope"`
	Before int64  `json:"before"`
	Limit  int    `json:"limit"`
	DryRun *bool  `json:"dry_run"`
	Since  int64  `json:"-"`
}

type imageThumbnailBatchItem struct {
	ID     int64  `json:"id"`
	TaskID string `json:"task_id,omitempty"`
	Key    string `json:"artifact_key,omitempty"`
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
}

type imageThumbnailBatchResult struct {
	Scope      string                    `json:"scope"`
	DryRun     bool                      `json:"dry_run"`
	Scanned    int                       `json:"scanned"`
	NextBefore int64                     `json:"next_before"`
	Items      []imageThumbnailBatchItem `json:"items"`
}

func ImageThumbnailBackfill(c *gin.Context) {
	var input imageThumbnailBatchRequest
	if err := common.DecodeJson(c.Request.Body, &input); err != nil {
		common.ApiError(c, errors.New("invalid thumbnail backfill request"))
		return
	}
	result, err := inspectImageThumbnailBatch(c.Request.Context(), input)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}

func inspectImageThumbnailBatch(ctx context.Context, input imageThumbnailBatchRequest) (*imageThumbnailBatchResult, error) {
	if input.Scope != "presets" && input.Scope != "tasks" {
		return nil, errors.New("scope must be presets or tasks")
	}
	if input.Before < 0 || input.Limit < 0 || input.Limit > 100 {
		return nil, errors.New("invalid backfill pagination")
	}
	if input.Limit == 0 {
		input.Limit = 100
	}
	dry := input.DryRun == nil || *input.DryRun
	result := &imageThumbnailBatchResult{Scope: input.Scope, DryRun: dry, Items: []imageThumbnailBatchItem{}}
	if input.Scope == "presets" {
		var rows []model.InfluencerPreset
		query := model.DB.WithContext(ctx).Order("id DESC").Limit(input.Limit)
		if input.Before > 0 {
			query = query.Where("id < ?", input.Before)
		}
		if err := query.Find(&rows).Error; err != nil {
			return nil, err
		}
		result.Scanned = len(rows)
		if len(rows) == input.Limit {
			result.NextBefore = rows[len(rows)-1].Id
		}
		for i := range rows {
			p := &rows[i]
			item := imageThumbnailBatchItem{ID: p.Id, Status: "missing"}
			if influencerPresetThumbnailReady(p) {
				item.Status = "ready"
			} else if influencerPresetThumbnailProcessing(p) {
				item.Status = "pending"
			} else if p.ThumbnailAttempts >= 3 {
				item.Status = "failed"
				item.Reason = "thumbnail retries exhausted; inspect storage and encoder logs"
			} else if !dry {
				item.Status = "pending"
				if enqueueInfluencerPresetThumbnail(p.Id) {
					item.Status = "queued"
				}
			}
			result.Items = append(result.Items, item)
		}
		return result, nil
	}
	var rows []*model.Task
	query := model.DB.WithContext(ctx).Where("status = ?", model.TaskStatusSuccess).Order("id DESC").Limit(input.Limit)
	if input.Before > 0 {
		query = query.Where("id < ?", input.Before)
	}
	if input.Since > 0 {
		query = query.Where("finish_time >= ?", input.Since)
	}
	if err := query.Find(&rows).Error; err != nil {
		return nil, err
	}
	result.Scanned = len(rows)
	if len(rows) == input.Limit {
		result.NextBefore = rows[len(rows)-1].ID
	}
	type candidate struct {
		task *model.Task
		key  string
	}
	candidates := []candidate{}
	for _, task := range rows {
		if !task.ResultRetrievable() {
			continue
		}
		keys, err := taskImageArtifactKeys(task)
		if err != nil {
			result.Items = append(result.Items, imageThumbnailBatchItem{ID: task.ID, TaskID: task.TaskID, Status: "failed", Reason: "image artifact projection unavailable"})
			continue
		}
		for _, key := range keys {
			candidates = append(candidates, candidate{task, key})
		}
	}
	// Storage reads share an overall deadline and fixed concurrency. Results are
	// indexed so reports and resume checkpoints remain deterministic.
	items := make([]imageThumbnailBatchItem, len(candidates))
	probeCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	jobs := make(chan int)
	var workers sync.WaitGroup
	for n := 0; n < 8; n++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for i := range jobs {
				candidate := candidates[i]
				task, key := candidate.task, candidate.key
				item := imageThumbnailBatchItem{ID: task.ID, TaskID: task.TaskID, Key: key, Status: "missing"}
				if probeCtx.Err() != nil {
					item.Status = "unknown"
					item.Reason = "storage probe deadline reached"
				} else if metadata, _ := service.ResolveTaskImageThumbnail(probeCtx, task, key); metadata != nil {
					item.Status = "ready"
				} else if reason := taskImageThumbnailFailure(task, key); reason != "" {
					item.Status = "failed"
					item.Reason = reason
				} else if !dry {
					item.Status = "pending"
					if taskImageThumbnailSchedule(task, key) {
						item.Status = "queued"
					}
				}
				items[i] = item
			}
		}()
	}
	for i := range candidates {
		jobs <- i
	}
	close(jobs)
	workers.Wait()
	result.Items = append(result.Items, items...)
	return result, nil
}

const imageThumbnailRecoveryType = "image_thumbnail_recovery"

// Reuse the native leased scheduler for recovery, including after restarts.
// Old task history is explicit backfill; automatic recovery covers the last day.
type imageThumbnailRecoveryHandler struct{}

func (imageThumbnailRecoveryHandler) Type() string            { return imageThumbnailRecoveryType }
func (imageThumbnailRecoveryHandler) Enabled() bool           { return true }
func (imageThumbnailRecoveryHandler) Interval() time.Duration { return time.Minute }

type imageThumbnailRecoveryCursor struct {
	Tasks   int64 `json:"tasks"`
	Presets int64 `json:"presets"`
}

func (imageThumbnailRecoveryHandler) NewPayload() any {
	cursor := imageThumbnailRecoveryCursor{}
	if last, err := model.GetLatestSystemTask(imageThumbnailRecoveryType); err == nil && last != nil && last.Result != "" {
		_ = common.UnmarshalJsonStr(last.Result, &cursor)
	}
	return cursor
}
func (imageThumbnailRecoveryHandler) Run(ctx context.Context, task *model.SystemTask, runnerID string) {
	cursor := imageThumbnailRecoveryCursor{}
	if err := task.DecodePayload(&cursor); err != nil {
		finishSystemTaskHandler(task, runnerID, model.SystemTaskStatusFailed, nil, err)
		return
	}
	apply := false
	for _, scope := range []string{"presets", "tasks"} {
		before := cursor.Presets
		if scope == "tasks" {
			before = cursor.Tasks
		}
		result, err := inspectImageThumbnailBatch(ctx, imageThumbnailBatchRequest{Scope: scope, Before: before, Limit: 100, DryRun: &apply, Since: common.GetTimestamp() - 86400})
		if err != nil {
			finishSystemTaskHandler(task, runnerID, model.SystemTaskStatusFailed, cursor, err)
			return
		}
		if scope == "tasks" {
			cursor.Tasks = result.NextBefore
		} else {
			cursor.Presets = result.NextBefore
		}
	}
	finishSystemTaskHandler(task, runnerID, model.SystemTaskStatusSucceeded, cursor, nil)
}
