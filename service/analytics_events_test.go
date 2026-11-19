package service

import (
	"context"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/analytics"
	"github.com/QuantumNous/new-api/types"
	"github.com/glebarez/sqlite"
	"github.com/go-redis/redis/v8"
	mysqldriver "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestFirstVideoMilestoneHistory(t *testing.T) {
	db := openAnalyticsFixture(t, sqlite.Open(filepath.Join(t.TempDir(), "history.db")))
	firstStart, firstSuccess, laterSuccess := seedVideoMilestones(t, db)

	start, err := earliestVideoMilestoneTask(t.Context(), db, firstStart.UserId, false)
	require.NoError(t, err)
	require.NotNil(t, start)
	assert.Equal(t, firstStart.ID, start.ID)

	success, err := earliestVideoMilestoneTask(t.Context(), db, firstStart.UserId, true)
	require.NoError(t, err)
	require.NotNil(t, success)
	assert.Equal(t, firstSuccess.ID, success.ID)
	assert.NotEqual(t, laterSuccess.ID, success.ID)

	// The older placeholder and the discarded video do not count towards the
	// baseline. The same read gives zero confirmed users without the two facts.
	require.NoError(t, db.Where("id IN ?", []int64{firstSuccess.ID, laterSuccess.ID}).Delete(&model.Task{}).Error)
	success, err = earliestVideoMilestoneTask(t.Context(), db, firstStart.UserId, true)
	require.NoError(t, err)
	assert.Nil(t, success)
}

type analyticsArtifactFixture struct{ TaskPollingAdaptor }

func (analyticsArtifactFixture) ListArtifactsWithContext(_ context.Context, task *model.Task) ([]types.TaskArtifact, error) {
	if string(task.Data) == `{"ready":true}` {
		return []types.TaskArtifact{{Key: "video", Type: "video", MimeType: "video/mp4"}}, nil
	}
	return []types.TaskArtifact{{Key: "preview", Type: "image", MimeType: "image/jpeg"}}, nil
}

func TestFirstVideoMilestoneUsesTaskArtifactProjection(t *testing.T) {
	db := openAnalyticsFixture(t, sqlite.Open(filepath.Join(t.TempDir(), "artifacts.db")))
	previous := GetTaskAdaptorFunc
	GetTaskAdaptorFunc = func(constant.TaskPlatform) TaskPollingAdaptor { return analyticsArtifactFixture{} }
	t.Cleanup(func() { GetTaskAdaptorFunc = previous })
	for _, ready := range []bool{false, true} {
		task := model.Task{
			TaskID: "task_artifact_" + strconv.FormatBool(ready), UserId: 18,
			Action: constant.TaskActionGenerate, Status: model.TaskStatusSuccess, FinishTime: 1,
			PrivateData: model.TaskPrivateData{Execution: &model.TaskExecutionSnapshot{TaskPlugin: &model.TaskPluginSnapshot{Key: "fixture"}}},
		}
		task.SetData(map[string]bool{"ready": ready})
		require.NoError(t, db.Create(&task).Error)
	}
	result, err := earliestVideoMilestoneTask(t.Context(), db, 18, true)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "task_artifact_true", result.TaskID)
}

// External verification uses task-owned databases plus two independent Redis
// clients. Ordinary go test still exercises metadata and history on SQLite.
func TestFirstVideoMilestoneDatabaseMatrix(t *testing.T) {
	redisURL := os.Getenv("E03_TEST_REDIS_URL")
	if redisURL == "" {
		t.Skip("set E03_TEST_REDIS_URL and task-owned database DSNs to run the integration matrix")
	}
	redisOpts, err := redis.ParseURL(redisURL)
	require.NoError(t, err)
	requireAnalyticsFixtureAddress(t, redisOpts.Addr)

	mysqlDSN := os.Getenv("E03_TEST_MYSQL_DSN")
	mysqlConfig, err := mysqldriver.ParseDSN(mysqlDSN)
	require.NoError(t, err)
	require.Equal(t, "tcp", mysqlConfig.Net)
	require.True(t, strings.HasPrefix(mysqlConfig.DBName, "__test_e03_"))
	requireAnalyticsFixtureAddress(t, mysqlConfig.Addr)

	postgresDSN := os.Getenv("E03_TEST_POSTGRES_DSN")
	postgresURL, err := url.Parse(postgresDSN)
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(postgresURL.Path, "/__test_e03_"))
	requireAnalyticsFixtureAddress(t, postgresURL.Host)

	for _, fixture := range []struct {
		name    string
		dialect gorm.Dialector
	}{
		{"sqlite", sqlite.Open(filepath.Join(t.TempDir(), "matrix.db"))},
		{"mysql", mysql.Open(mysqlDSN)},
		{"postgres", postgres.Open(postgresDSN)},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			db := openAnalyticsFixture(t, fixture.dialect)
			firstStart, firstSuccess, laterSuccess := seedVideoMilestones(t, db)
			workerA, workerB := redis.NewClient(redisOpts), redis.NewClient(redisOpts)
			t.Cleanup(func() { _ = workerA.Close() })
			t.Cleanup(func() { _ = workerB.Close() })
			site := "fixture:" + filepath.Base(t.TempDir()) + ":" + fixture.name
			t.Cleanup(func() {
				user := strconv.Itoa(firstStart.UserId)
				for _, name := range []string{analytics.EventFirstVideoStart, analytics.EventFirstVideoSuccess} {
					_ = workerA.Del(context.Background(), "analytics:e03:"+site+":"+name+":"+user).Err()
				}
			})

			start, reason := firstVideoMilestone(t.Context(), db, workerA, site, firstStart.UserId, firstStart.ID, analytics.EventFirstVideoStart, "api")
			assert.Empty(t, reason)
			assert.Equal(t, analytics.EventFirstVideoStart, start.Name)
			assert.Equal(t, firstStart.TaskID, start.Data["task_id"])

			later, reason := firstVideoMilestone(t.Context(), db, workerB, site, laterSuccess.UserId, laterSuccess.ID, analytics.EventFirstVideoSuccess, "")
			assert.Empty(t, reason)
			assert.Empty(t, later.Name, "a later task must respect the existing first success")

			type outcome struct {
				event  analytics.Event
				reason string
			}
			results := make(chan outcome, 2)
			ready := make(chan struct{})
			var concurrent sync.WaitGroup
			for _, worker := range []*redis.Client{workerA, workerB} {
				concurrent.Go(func() {
					<-ready
					event, reason := firstVideoMilestone(t.Context(), db, worker, site, firstSuccess.UserId, firstSuccess.ID, analytics.EventFirstVideoSuccess, "")
					results <- outcome{event, reason}
				})
			}
			close(ready)
			concurrent.Wait()
			close(results)
			count := 0
			for result := range results {
				assert.Empty(t, result.reason)
				if result.event.Name != "" {
					count++
					assert.Equal(t, firstSuccess.TaskID, result.event.Data["task_id"])
					assert.Equal(t, int64(5250), result.event.Data["duration_ms"])
					assert.Equal(t, int64(123_456), result.event.Data["file_size"])
				}
			}
			assert.Equal(t, 1, count)

			restarted := redis.NewClient(redisOpts)
			t.Cleanup(func() { _ = restarted.Close() })
			replay, reason := firstVideoMilestone(t.Context(), db, restarted, site, firstSuccess.UserId, firstSuccess.ID, analytics.EventFirstVideoSuccess, "")
			assert.Empty(t, reason)
			assert.Empty(t, replay.Name)

			disabled, reason := firstVideoMilestone(t.Context(), db, nil, site, firstSuccess.UserId, firstSuccess.ID, analytics.EventFirstVideoSuccess, "")
			assert.NotEmpty(t, reason)
			assert.Empty(t, disabled.Name)
		})
	}
}

func openAnalyticsFixture(t *testing.T, dialect gorm.Dialector) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(dialect, &gorm.Config{})
	require.NoError(t, err)
	connection, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = connection.Close() })
	require.NoError(t, db.AutoMigrate(&model.Task{}))
	return db
}

func requireAnalyticsFixtureAddress(t *testing.T, addr string) {
	t.Helper()
	host, textPort, err := net.SplitHostPort(addr)
	require.NoError(t, err)
	require.Equal(t, "127.0.0.1", host)
	port, err := strconv.Atoi(textPort)
	require.NoError(t, err)
	require.GreaterOrEqual(t, port, 49152)
}

func seedVideoMilestones(t *testing.T, db *gorm.DB) (firstStart, firstSuccess, laterSuccess *model.Task) {
	t.Helper()
	const userID = 92_101
	t.Cleanup(func() { require.NoError(t, db.Where("user_id = ?", userID).Delete(&model.Task{}).Error) })

	// More than one page of ambiguous legacy actions shares a finish time.
	images := make([]model.Task, 33)
	for i := range images {
		images[i] = model.Task{UserId: userID, TaskID: "task_legacy_image_" + strconv.Itoa(i),
			Action: constant.TaskActionGenerate, Status: model.TaskStatusSuccess, FinishTime: 10,
			Properties:  model.Properties{OriginModelName: "gpt-image-2.5"},
			PrivateData: model.TaskPrivateData{ResultURL: "https://example.com/thumbnail.jpg"}}
	}
	require.NoError(t, db.Create(&images).Error)
	video := model.Properties{OriginModelName: "seedance-2-5"}
	firstStart = &model.Task{UserId: userID, TaskID: "task_failed_video", Action: constant.TaskActionTextToVideo,
		Status: model.TaskStatusFailure, FinishTime: 5, Properties: video}
	placeholder := &model.Task{UserId: userID, TaskID: "task_placeholder", Action: constant.TaskActionImageToVideo,
		Status: model.TaskStatusSuccess, FinishTime: 10, Properties: video,
		PrivateData: model.TaskPrivateData{ResultURL: "https://example.com/v1/videos/task_placeholder/content"}}
	discarded := &model.Task{UserId: userID, TaskID: "task_discarded", Action: constant.TaskActionTextToVideo,
		Status: model.TaskStatusSuccess, FinishTime: 10, Properties: video,
		PrivateData: model.TaskPrivateData{ResultURL: "https://example.com/output.mp4", ResultDiscarded: true}}
	metadata, err := common.Marshal(dto.InfluencerVideoMetadata{VideoDuration: 5.25, Width: 480, Height: 854, SizeBytes: 123_456})
	require.NoError(t, err)
	firstSuccess = &model.Task{UserId: userID, TaskID: "task_workbench_video", Action: constant.TaskActionInfluencerMotion,
		Status: model.TaskStatusSuccess, FinishTime: 10, Properties: video, OutputVideoMetadata: metadata,
		PrivateData: model.TaskPrivateData{ResultURL: "https://example.com/output.mp4"}}
	laterSuccess = &model.Task{UserId: userID, TaskID: "task_later_video", Action: constant.TaskActionReferenceToVideo,
		Status: model.TaskStatusSuccess, FinishTime: 10, Properties: video,
		PrivateData: model.TaskPrivateData{ResultURL: "https://example.com/output.webm"}}
	for _, task := range []*model.Task{firstStart, placeholder, discarded, firstSuccess, laterSuccess} {
		require.NoError(t, db.Create(task).Error)
	}
	return firstStart, firstSuccess, laterSuccess
}
