package controller

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/storage"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Only object storage and provider HTTP are substitutes; dispatch, routing and billing stay native.
type generationMaterialStorage struct {
	storage.StorageDriver
	getError error
	uploads  atomic.Int32
	reads    atomic.Int64
}

func (s *generationMaterialStorage) Upload(ctx context.Context, key string, data []byte, mime string) (string, error) {
	s.uploads.Add(1)
	return s.StorageDriver.Upload(ctx, key, data, mime)
}

func (s *generationMaterialStorage) Get(ctx context.Context, key string) (io.ReadCloser, string, int64, error) {
	if s.getError != nil {
		return nil, "", 0, s.getError
	}
	reader, mime, size, err := s.StorageDriver.Get(ctx, key)
	if err != nil {
		return nil, "", 0, err
	}
	return generationMaterialReader{ReadCloser: reader, reads: &s.reads}, mime, size, nil
}

type generationMaterialReader struct {
	io.ReadCloser
	reads *atomic.Int64
}

func (r generationMaterialReader) Read(p []byte) (int, error) {
	n, err := r.ReadCloser.Read(p)
	r.reads.Add(int64(n))
	return n, err
}

func generationMaterialStore(t *testing.T) *generationMaterialStorage {
	t.Helper()
	t.Setenv("FILE_PUBLIC_BASE_URL", "https://8.8.8.8")
	previous := storage.GetStorageDriver()
	driver := &generationMaterialStorage{StorageDriver: storage.NewLocalStorageDriver(t.TempDir())}
	storage.SetGlobalDriver(driver)
	t.Cleanup(func() { storage.SetGlobalDriver(previous) })
	return driver
}

func generationMaterialObservePrecharge(t *testing.T, db *gorm.DB) *atomic.Bool {
	t.Helper()
	var precharged atomic.Bool
	require.NoError(t, db.Callback().Update().After("gorm:update").Register("material_observe_native_precharge", func(tx *gorm.DB) {
		if tx.Statement.Table != "users" {
			return
		}
		if values, ok := tx.Statement.Dest.(map[string]interface{}); ok {
			if expr, ok := values["quota"].(clause.Expr); ok && strings.Contains(strings.ReplaceAll(expr.SQL, " ", ""), "quota-?") {
				precharged.Store(true)
			}
		}
	}))
	return &precharged
}

func generationMaterialQueue(t *testing.T, user model.User, out, meta map[string]any, video bool) *model.Task {
	t.Helper()
	task := insertCorrectionTask(t, user, 1, video)
	body, err := common.Marshal(out)
	require.NoError(t, err)
	input, err := common.Marshal(meta)
	require.NoError(t, err)
	task.Properties.Input = string(input)
	task.PrivateData.GenerationQueue.Body = body
	if !video {
		task.PrivateData.GenerationQueue.Path = "/pg/images/edits"
	}
	require.NoError(t, task.Update())
	return task
}

func generationMaterialClaimWithSixth(t *testing.T, db *gorm.DB, user model.User, scanner *model.SystemTask) (*model.Task, *model.Task) {
	t.Helper()
	for i := 2; i <= 5; i++ {
		active := insertCorrectionTask(t, user, i, i%2 == 0)
		require.NoError(t, db.Model(active).Update("status", model.TaskStatusDispatching).Error)
	}
	sixth := insertCorrectionTask(t, user, 6, false)
	claimed, err := model.ClaimGenerationTasks(context.Background(), user.Id, scanner.TaskID)
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	return claimed[0], sixth
}

func generationMaterialAssertSafeFailure(t *testing.T, user model.User, task, sixth *model.Task, scanner *model.SystemTask, calls *atomic.Int32, precharged *atomic.Bool) {
	t.Helper()
	stored, found, err := model.GetByTaskId(user.Id, task.TaskID)
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, model.TaskStatus(model.TaskStatusFailure), stored.Status)
	assert.Zero(t, stored.Quota)
	assert.Zero(t, calls.Load(), "invalid material must never reach generation POST")
	assert.False(t, precharged.Load(), "not a refunded precharge: native funding must never start")
	assert.Nil(t, stored.PrivateData.BillingContext)
	account, err := model.GetUserById(user.Id, false)
	require.NoError(t, err)
	assert.Equal(t, 100000, account.Quota)
	assert.Zero(t, account.UsedQuota)
	next, err := model.ClaimGenerationTasks(context.Background(), user.Id, scanner.TaskID)
	require.NoError(t, err)
	require.Len(t, next, 1)
	assert.Equal(t, sixth.ID, next[0].ID)
}

func TestGenerationQueueMaterialFrozenFilesFailBeforeNativeChargeAndReleaseSixth(t *testing.T) {
	for _, field := range []string{"videos", "audios"} {
		for _, mode := range []string{"expired", "deleted", "missing_row", "foreign_owner", "pending", "duplicate_url", "missing_object", "storage_error", "wrong_namespace", "url_key_mismatch", "empty_object", "database_error"} {
			t.Run(field+"/"+mode, func(t *testing.T) {
				var calls atomic.Int32
				db, user, scanner := generationCorrectionFixture(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls.Add(1)
					_, _ = w.Write([]byte(`{"id":"accepted"}`))
				}))
				require.NoError(t, db.AutoMigrate(&model.File{}))
				precharged := generationMaterialObservePrecharge(t, db)
				driver := generationMaterialStore(t)
				out := map[string]any{"model": "queue-correction", "prompt": "frozen", "images": []string{"https://8.8.4.4/external-subject.png"}}
				var target *model.File
				for _, media := range []string{"videos", "audios"} {
					key := "influencer-reference/frozen/" + media + ".bin"
					url, err := driver.Upload(context.Background(), key, []byte("accepted-bytes"), "application/octet-stream")
					require.NoError(t, err)
					file := &model.File{FileId: "file_" + media, UserId: user.Id, StorageKey: key, FileUrl: url, Status: model.FileStatusActive, ExpiresAt: time.Now().Add(time.Hour)}
					require.NoError(t, model.CreateFile(file))
					out[media] = []string{url}
					if media == field {
						target = file
					}
				}
				switch mode {
				case "expired":
					target.ExpiresAt = time.Now().Add(-time.Second)
				case "deleted":
					target.Status = model.FileStatusDeleted
				case "foreign_owner":
					target.UserId = user.Id + 1
				case "pending":
					target.Status = model.FileStatusUploadPending
				case "wrong_namespace":
					target.StorageKey = "user-uploads/not-frozen.bin"
				case "url_key_mismatch":
					target.StorageKey = "influencer-reference/frozen/other.bin"
				case "missing_row":
					require.NoError(t, model.DeleteFilePermanently(target.Id))
				case "duplicate_url":
					duplicate := *target
					duplicate.Id = 0
					duplicate.FileId = "file_duplicate"
					require.NoError(t, model.CreateFile(&duplicate))
				case "missing_object":
					require.NoError(t, driver.Delete(context.Background(), target.StorageKey))
				case "empty_object":
					_, err := driver.Upload(context.Background(), target.StorageKey, nil, "application/octet-stream")
					require.NoError(t, err)
				case "storage_error":
					driver.getError = errors.New("temporary storage failure")
				case "database_error":
					require.NoError(t, db.Callback().Query().Before("gorm:query").Register("material_files_unavailable", func(tx *gorm.DB) {
						if tx.Statement.Table == "files" {
							tx.AddError(errors.New("temporary file query failure"))
						}
					}))
				}
				if mode != "missing_row" {
					require.NoError(t, db.Save(target).Error)
				}
				task := generationMaterialQueue(t, user, out, map[string]any{"kind": "motion", "prepared_video_sha256": "original-digest"}, true)
				claimed, sixth := generationMaterialClaimWithSixth(t, db, user, scanner)
				driver.uploads.Store(0)
				dispatchInfluencerGeneration(context.Background(), claimed, scanner, "correction")
				generationMaterialAssertSafeFailure(t, user, task, sixth, scanner, &calls, precharged)
				assert.Zero(t, driver.uploads.Load(), "dispatch must not recreate or upload expired material")
			})
		}
	}
}

func TestGenerationQueueMaterialFrozenFilesPreserveMetadataAndUseNativeBilling(t *testing.T) {
	var calls atomic.Int32
	var submitted map[string]any
	db, user, scanner := generationCorrectionFixture(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		require.NoError(t, common.DecodeJson(r.Body, &submitted))
		_, _ = w.Write([]byte(`{"id":"accepted"}`))
	}))
	require.NoError(t, db.AutoMigrate(&model.File{}))
	precharged := generationMaterialObservePrecharge(t, db)
	driver := generationMaterialStore(t)
	out := map[string]any{"model": "queue-correction", "prompt": "frozen", "images": []string{"https://8.8.4.4/external-subject.png"}}
	for _, field := range []string{"videos", "audios"} {
		key := "influencer-reference/frozen/" + field + ".bin"
		url, err := driver.Upload(context.Background(), key, []byte("original-frozen-bytes"), "application/octet-stream")
		require.NoError(t, err)
		require.NoError(t, model.CreateFile(&model.File{FileId: "file_" + field, UserId: user.Id, StorageKey: key, FileUrl: url, Status: model.FileStatusActive, ExpiresAt: time.Now().Add(time.Hour)}))
		out[field] = []string{url}
	}
	task := generationMaterialQueue(t, user, out, map[string]any{"kind": "motion", "prepared_video_sha256": "original-digest"}, true)
	originalMeta, originalBody := task.Properties.Input, string(task.PrivateData.GenerationQueue.Body)
	claimed, err := model.ClaimGenerationTasks(context.Background(), user.Id, scanner.TaskID)
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	driver.uploads.Store(0)
	dispatchInfluencerGeneration(context.Background(), claimed[0], scanner, "correction")
	stored, found, err := model.GetByTaskId(user.Id, task.TaskID)
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, int32(1), calls.Load())
	assert.True(t, precharged.Load())
	assert.Equal(t, 5000, stored.Quota)
	assert.Equal(t, originalMeta, stored.Properties.Input)
	wire, err := common.Marshal(submitted)
	require.NoError(t, err)
	assert.JSONEq(t, originalBody, string(wire))
	assert.Zero(t, driver.uploads.Load())
	assert.Equal(t, int64(2), driver.reads.Load(), "only bounded readability probes, not a full video download or decode")
}

func TestGenerationQueueMaterialMultiViewParentFailBeforeNativeChargeAndReleaseSixth(t *testing.T) {
	for _, mode := range []string{"missing", "foreign_owner", "not_success", "discarded", "wrong_action", "multiview_parent", "wire_mismatch", "wrong_origin", "wrong_artifact", "bad_signature", "substring_only", "unreadable", "database_error", "valid_old_artifact"} {
		t.Run(mode, func(t *testing.T) {
			var calls atomic.Int32
			db, user, scanner := generationCorrectionFixture(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				_, _ = w.Write([]byte(`{"id":"accepted"}`))
			}))
			precharged := generationMaterialObservePrecharge(t, db)
			driver := generationMaterialStore(t)
			oldAddress, oldSecret, oldClient := system_setting.TaskPublicAddress, common.CryptoSecret, influencerReferenceHTTPClient
			system_setting.TaskPublicAddress, common.CryptoSecret = "https://8.8.8.8", "material-test-only"
			t.Cleanup(func() {
				system_setting.TaskPublicAddress, common.CryptoSecret, influencerReferenceHTTPClient = oldAddress, oldSecret, oldClient
			})
			parent := &model.Task{TaskID: "material-parent", UserId: user.Id, Action: ActionInfluencerSheet, Status: model.TaskStatusSuccess, SubmitTime: time.Now().Add(-96 * time.Hour).Unix(), FinishTime: time.Now().Add(-96 * time.Hour).Unix(), Properties: model.Properties{Input: `{"brief":"ordinary character"}`, OriginModelName: "gpt-image-2.5"}, PrivateData: model.TaskPrivateData{ResultURL: "https://8.8.4.4/old-upstream.png"}}
			require.NoError(t, parent.Insert())
			imageURL, err := service.BuildTaskArtifactContentURL(parent.TaskID, "image")
			require.NoError(t, err)
			var pngBytes bytes.Buffer
			require.NoError(t, png.Encode(&pngBytes, image.NewRGBA(image.Rect(0, 0, 2, 2))))
			_, err = driver.Upload(context.Background(), "task-artifacts/"+parent.TaskID+"/image", pngBytes.Bytes(), "image/png")
			require.NoError(t, err)
			artifactEngine := gin.New()
			artifactEngine.GET("/v1/tasks/:key/artifacts/:artifact_key/content", func(c *gin.Context) { c.Set("id", user.Id) }, TaskArtifactContent)
			influencerReferenceHTTPClient = &http.Client{Transport: referenceControllerFetchFunc(func(req *http.Request) (*http.Response, error) {
				recorder := httptest.NewRecorder()
				artifactEngine.ServeHTTP(recorder, req)
				return recorder.Result(), nil
			})}
			wireURL := imageURL
			switch mode {
			case "missing":
				require.NoError(t, db.Delete(parent).Error)
			case "foreign_owner":
				parent.UserId = user.Id + 1
			case "not_success":
				parent.Status = model.TaskStatusFailure
			case "discarded":
				parent.PrivateData.ResultDiscarded = true
			case "wrong_action":
				parent.Action = ActionInfluencerMotion
			case "multiview_parent":
				parent.Properties.Input = `{"kind":"multiview"}`
			case "wire_mismatch":
				wireURL = "https://8.8.4.4/unrelated.png"
			case "wrong_origin":
				imageURL = strings.Replace(imageURL, "8.8.8.8", "8.8.4.4", 1)
				wireURL = imageURL
			case "wrong_artifact":
				imageURL = strings.Replace(imageURL, "/image/content", "/video/content", 1)
				wireURL = imageURL
			case "bad_signature":
				imageURL = strings.Split(imageURL, "?")[0] + "?access=bad"
				wireURL = imageURL
			case "substring_only":
				imageURL = "https://8.8.8.8/x?next=" + parentArtifactPath(parent.TaskID)
				wireURL = imageURL
			case "unreadable":
				influencerReferenceHTTPClient = &http.Client{Transport: referenceControllerFetchFunc(func(*http.Request) (*http.Response, error) {
					return nil, errors.New("temporary artifact fetch failure")
				})}
			case "database_error":
				require.NoError(t, db.Callback().Query().After("gorm:query").Register("parent_query_failure", func(tx *gorm.DB) {
					if row, ok := tx.Statement.Dest.(**model.Task); ok && *row != nil && (*row).TaskID == parent.TaskID {
						tx.AddError(errors.New("temporary parent query failure"))
					}
				}))
			}
			if mode != "missing" {
				require.NoError(t, parent.Update())
			}
			task := generationMaterialQueue(t, user, map[string]any{"model": "queue-correction", "prompt": "frozen", "image": wireURL}, map[string]any{"kind": "multiview", "parent_task_id": parent.TaskID, "image_url": imageURL}, false)
			claimed, sixth := generationMaterialClaimWithSixth(t, db, user, scanner)
			driver.uploads.Store(0)
			dispatchInfluencerGeneration(context.Background(), claimed, scanner, "correction")
			if mode == "database_error" {
				require.NoError(t, db.Callback().Query().Remove("parent_query_failure"))
			}
			if mode == "valid_old_artifact" {
				stored, found, err := model.GetByTaskId(user.Id, task.TaskID)
				require.NoError(t, err)
				require.True(t, found)
				assert.Equal(t, int32(1), calls.Load())
				assert.True(t, precharged.Load())
				assert.Equal(t, 5000, stored.Quota)
				assert.Equal(t, task.Properties.Input, stored.Properties.Input)
				assert.Greater(t, driver.reads.Load(), int64(0), "real artifact endpoint serves the stored old image")
			} else {
				generationMaterialAssertSafeFailure(t, user, task, sixth, scanner, &calls, precharged)
			}
			assert.Zero(t, driver.uploads.Load())
		})
	}
}

func TestGenerationQueueMaterialAlreadySentUnknownDoesNotReleaseOnExpiry(t *testing.T) {
	var calls atomic.Int32
	db, user, scanner := generationCorrectionFixture(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_, _ = w.Write([]byte(`{"unexpected":"unknown acknowledgement"}`))
	}))
	require.NoError(t, db.AutoMigrate(&model.File{}))
	driver := generationMaterialStore(t)
	key := "influencer-reference/frozen/unknown.mp4"
	url, err := driver.Upload(context.Background(), key, []byte("accepted-video"), "video/mp4")
	require.NoError(t, err)
	file := &model.File{FileId: "file_unknown", UserId: user.Id, StorageKey: key, FileUrl: url, Status: model.FileStatusActive, ExpiresAt: time.Now().Add(time.Hour)}
	require.NoError(t, model.CreateFile(file))
	task := generationMaterialQueue(t, user, map[string]any{"model": "queue-correction", "videos": []string{url}}, map[string]any{"kind": "motion"}, true)
	claimed, sixth := generationMaterialClaimWithSixth(t, db, user, scanner)
	stale := *claimed
	staleRequest := *claimed.PrivateData.GenerationQueue
	stale.PrivateData.GenerationQueue = &staleRequest
	dispatchInfluencerGeneration(context.Background(), claimed, scanner, "correction")
	unknown, found, err := model.GetByTaskId(user.Id, task.TaskID)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, model.TaskStatusDispatching, unknown.Status)
	require.NotZero(t, unknown.PrivateData.GenerationQueue.SendStartedAt)
	require.Equal(t, 5000, unknown.Quota)
	require.NoError(t, db.Model(file).Updates(map[string]any{"status": model.FileStatusExpired, "expires_at": time.Now().Add(-time.Second)}).Error)
	require.NoError(t, driver.Delete(context.Background(), key))
	driver.uploads.Store(0)
	dispatchInfluencerGeneration(context.Background(), unknown, scanner, "correction")
	dispatchInfluencerGeneration(context.Background(), &stale, scanner, "correction")
	require.NoError(t, model.RecoverGenerationClaims(context.Background(), scanner, "correction"))
	stored, found, err := model.GetByTaskId(user.Id, task.TaskID)
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, model.TaskStatusDispatching, stored.Status)
	assert.Equal(t, 5000, stored.Quota)
	assert.Equal(t, int32(1), calls.Load(), "expiry after possible send never causes resubmission")
	assert.Zero(t, driver.uploads.Load())
	account, err := model.GetUserById(user.Id, false)
	require.NoError(t, err)
	assert.Equal(t, 95000, account.Quota)
	next, err := model.ClaimGenerationTasks(context.Background(), user.Id, scanner.TaskID)
	require.NoError(t, err)
	assert.Empty(t, next)
	waiting, found, err := model.GetByTaskId(user.Id, sixth.TaskID)
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, model.TaskStatusWaiting, waiting.Status)
}

func TestGenerationQueueMaterialOldOwnerFailureCannotOverwriteNewClaim(t *testing.T) {
	var calls atomic.Int32
	db, user, scanner := generationCorrectionFixture(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1) }))
	task := generationMaterialQueue(t, user, map[string]any{"model": "queue-correction", "videos": []string{"https://8.8.8.8/influencer-reference/frozen/missing.mp4"}}, map[string]any{"kind": "motion"}, true)
	require.NoError(t, db.AutoMigrate(&model.File{}))
	claimed, err := model.ClaimGenerationTasks(context.Background(), user.Id, scanner.TaskID)
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	newer := *claimed[0]
	request := *newer.PrivateData.GenerationQueue
	request.ClaimOwner = "new-owner"
	newer.PrivateData.GenerationQueue = &request
	require.NoError(t, newer.Update())
	dispatchInfluencerGeneration(context.Background(), claimed[0], scanner, "correction")
	stored, found, err := model.GetByTaskId(user.Id, task.TaskID)
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, model.TaskStatusClaimed, stored.Status)
	assert.Equal(t, "new-owner", stored.PrivateData.GenerationQueue.ClaimOwner)
	assert.Empty(t, stored.FailReason)
	assert.Zero(t, calls.Load())
}
