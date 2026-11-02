package relay

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// TestRealtimeReadPreservesPollingCAS uses only an in-memory ledger and local fal.
func TestRealtimeReadPreservesPollingCAS(t *testing.T) {
	oldAdaptor := service.GetTaskAdaptorFunc
	service.GetTaskAdaptorFunc = func(platform constant.TaskPlatform) service.TaskPollingAdaptor { return GetTaskAdaptor(platform) }
	defer func() { service.GetTaskAdaptorFunc = oldAdaptor }()
	oldDB, oldLog := model.DB, model.LOG_DB
	oldRedis, oldBatch := common.RedisEnabled, common.BatchUpdateEnabled
	defer func() {
		model.DB = oldDB
		model.LOG_DB = oldLog
		common.RedisEnabled = oldRedis
		common.BatchUpdateEnabled = oldBatch
	}()
	common.RedisEnabled = false
	common.BatchUpdateEnabled = false
	for _, terminal := range []string{"FAILED", "COMPLETED"} {
		for _, getFirst := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s/get-first-%v", terminal, getFirst), func(t *testing.T) {
				db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
				require.NoError(t, err)
				sqlDB, err := db.DB()
				require.NoError(t, err)
				sqlDB.SetMaxOpenConns(1)
				defer sqlDB.Close()
				model.DB = db
				model.LOG_DB = db
				common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
				require.NoError(t, db.AutoMigrate(&model.Task{}, &model.User{}, &model.Channel{}, &model.Token{}, &model.Log{}))
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if terminal == "FAILED" {
						fmt.Fprint(w, `{"status":"FAILED","detail":"fixture failure"}`)
						return
					}
					if r.URL.Path == "/minimax/h3-max/requests/task/status" {
						fmt.Fprint(w, `{"status":"COMPLETED"}`)
						return
					}
					fmt.Fprint(w, `{"video":{"url":"https://example.invalid/output.mp4"}}`)
				}))
				defer server.Close()
				base := server.URL
				ch := model.Channel{Id: 1, Type: constant.ChannelTypeFalAI, Key: "fixture", BaseURL: &base}
				require.NoError(t, db.Create(&ch).Error)
				require.NoError(t, db.Create(&model.User{Id: 1, Username: "fixture", Quota: 900, UsedQuota: 100}).Error)
				platform := constant.TaskPlatform(strconv.Itoa(constant.ChannelTypeFalAI))
				task := model.Task{TaskID: "local-task", UserId: 1, ChannelId: 1, Platform: platform, Status: model.TaskStatusInProgress, Quota: 100}
				task.PrivateData.UpstreamTaskID = "minimax/h3-max/text-to-video:task"
				require.NoError(t, db.Create(&task).Error)
				var getSnapshot, pollSnapshot model.Task
				require.NoError(t, db.First(&getSnapshot, task.ID).Error)
				require.NoError(t, db.First(&pollSnapshot, task.ID).Error)
				poll := func() {
					require.NoError(t, service.UpdateVideoTasks(context.Background(), platform, map[int][]string{1: {task.GetUpstreamTaskID()}}, map[string]*model.Task{task.GetUpstreamTaskID(): &pollSnapshot}))
				}
				if getFirst {
					tryRealtimeFetch(&getSnapshot, true)
					var stored model.Task
					require.NoError(t, db.First(&stored, task.ID).Error)
					assert.Equal(t, model.TaskStatusInProgress, string(stored.Status))
					assert.Equal(t, 100, stored.Quota)
					poll()
				} else {
					poll()
					tryRealtimeFetch(&getSnapshot, true)
				}
				var stored model.Task
				require.NoError(t, db.First(&stored, task.ID).Error)
				var user model.User
				require.NoError(t, db.First(&user, 1).Error)
				assert.NotZero(t, stored.FinishTime)
				if terminal == "FAILED" {
					assert.Equal(t, model.TaskStatusFailure, string(stored.Status))
					assert.Equal(t, 0, stored.Quota)
					assert.Equal(t, 1000, user.Quota)
					assert.Equal(t, "fixture failure", stored.FailReason)
				} else {
					assert.Equal(t, model.TaskStatusSuccess, string(stored.Status))
					assert.Equal(t, 900, user.Quota)
					assert.Equal(t, "https://example.invalid/output.mp4", stored.PrivateData.ResultURL)
				}
				// A second worker holding the same old snapshot must lose CAS, not refund twice.
				pollSnapshot = task
				poll()
				require.NoError(t, db.First(&user, 1).Error)
				if terminal == "FAILED" {
					assert.Equal(t, 1000, user.Quota)
				} else {
					assert.Equal(t, 900, user.Quota)
				}
			})
		}
	}
}
