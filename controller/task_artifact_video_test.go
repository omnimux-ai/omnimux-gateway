package controller

import (
	"bytes"
	"context"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/storage"
	relaychannel "github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/system_setting"
)

// fork: video artifacts are persisted on first use so a replay is answered from
// our own object store instead of the provider, and the first frame is kept as
// the task's cover.

// videoArtifactTestTask builds a plugin video task whose provider serves a
// clean 200, which is the only response the persistence path buffers.
func videoArtifactTestTask(t *testing.T, upstreamURL string) *model.Task {
	t.Helper()
	task := setupGenericTaskTest(t)
	require.NoError(t, model.DB.Model(&model.Channel{}).Where("id = ?", task.ChannelId).Updates(map[string]any{
		"type":     constant.ChannelTypeGemini,
		"key":      "provider-key",
		"base_url": upstreamURL,
	}).Error)
	task.Platform = constant.TaskPlatform("google")
	task.PrivateData.Execution = &model.TaskExecutionSnapshot{TaskPlugin: &model.TaskPluginSnapshot{
		Key: "google", Name: "Google Veo (Gemini API)", Version: "1.0.0", APIVersion: 1,
	}}
	task.SetData(map[string]any{"response": map[string]any{
		"generateVideoResponse": map[string]any{
			"generatedVideos": []any{map[string]any{"video": map[string]any{"uri": upstreamURL}}},
		},
	}})
	require.NoError(t, model.DB.Save(task).Error)
	return task
}

func videoArtifactTestRequest(task *model.Task, headers map[string]string) (*gin.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set(middleware.TaskArtifactAccessContextKey, true)
	c.Params = gin.Params{
		{Key: "key", Value: task.TaskID},
		{Key: "artifact_key", Value: "video"},
	}
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/tasks/"+task.TaskID+"/artifacts/video/content", nil)
	for name, value := range headers {
		c.Request.Header.Set(name, value)
	}
	return c, recorder
}

func TestSoraVideoDeliveryPersistsWithoutProviderCredentials(t *testing.T) {
	videoBytes := []byte("existing-video-bytes")
	var upstreamHits int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&upstreamHits, 1)
		assert.Equal(t, "/delivered.mp4", r.URL.Path)
		assert.Empty(t, r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "video/mp4")
		_, _ = w.Write(videoBytes)
	}))
	defer upstream.Close()
	task := setupGenericTaskTest(t)
	require.NoError(t, model.DB.Model(&model.Channel{}).Where("id = ?", task.ChannelId).Updates(map[string]any{"type": 1, "key": "provider-key", "base_url": upstream.URL}).Error)
	task.Platform = constant.TaskPlatform("sora")
	task.Action = ActionInfluencerMotion
	task.PrivateData.Execution = &model.TaskExecutionSnapshot{TaskPlugin: &model.TaskPluginSnapshot{Key: "sora", Version: "1.1.0", APIVersion: 1}}
	task.SetData(map[string]any{"object": "video", "status": "completed", "url": upstream.URL + "/delivered.mp4"})
	require.NoError(t, model.DB.Save(task).Error)
	allowPrivateTaskMediaTest(t)
	previousMemoryCache := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = false
	t.Cleanup(func() { common.MemoryCacheEnabled = previousMemoryCache })
	c, recorder := videoArtifactTestRequest(task, nil)
	TaskArtifactContent(c)
	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, videoBytes, recorder.Body.Bytes())
	assert.True(t, service.IsTaskArtifactStored(task, "video"))
	upstream.Close()
	c, recorder = videoArtifactTestRequest(task, nil)
	TaskArtifactContent(c)
	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, videoBytes, recorder.Body.Bytes())
	assert.Equal(t, int32(1), atomic.LoadInt32(&upstreamHits))
}

func TestVideoArtifactPersistsAndReplaysFromStore(t *testing.T) {
	videoBytes := bytes.Repeat([]byte("v"), 4096)
	var upstreamHits int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&upstreamHits, 1)
		w.Header().Set("Content-Type", "video/mp4")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(videoBytes)
	}))
	defer upstream.Close()

	task := videoArtifactTestTask(t, upstream.URL)
	driver := newInfluencerPresetTestDriver()
	previousDriver := storage.GetStorageDriver()
	storage.SetGlobalDriver(driver)
	t.Cleanup(func() { storage.SetGlobalDriver(previousDriver) })
	allowPrivateTaskMediaTest(t)
	previousMemoryCache := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = false
	t.Cleanup(func() { common.MemoryCacheEnabled = previousMemoryCache })

	c, recorder := videoArtifactTestRequest(task, nil)
	TaskArtifactContent(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, videoBytes, recorder.Body.Bytes())
	assert.Equal(t, "video/mp4", recorder.Header().Get("Content-Type"))
	stored, ok := driver.objects["task-artifacts/"+task.TaskID+"/video"]
	require.True(t, ok, "the video should be kept in our own storage")
	assert.Equal(t, videoBytes, stored.body)

	// The provider is gone: a replay must still answer from the stored copy.
	upstream.Close()
	c, recorder = videoArtifactTestRequest(task, nil)
	TaskArtifactContent(c)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, videoBytes, recorder.Body.Bytes())
	assert.Equal(t, int32(1), atomic.LoadInt32(&upstreamHits), "a stored video must not hit the provider again")
}

func TestVideoArtifactPersistsFirstFrameCover(t *testing.T) {
	videoBytes := bytes.Repeat([]byte("v"), 2048)
	coverBytes := []byte("cover-jpeg")
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "video/mp4")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(videoBytes)
	}))
	defer upstream.Close()

	task := videoArtifactTestTask(t, upstream.URL)
	driver := newInfluencerPresetTestDriver()
	previousDriver := storage.GetStorageDriver()
	storage.SetGlobalDriver(driver)
	t.Cleanup(func() { storage.SetGlobalDriver(previousDriver) })
	previousExtractor := service.VideoPosterExtractor
	service.VideoPosterExtractor = func(context.Context, []byte) ([]byte, error) {
		return coverBytes, nil
	}
	t.Cleanup(func() { service.VideoPosterExtractor = previousExtractor })
	allowPrivateTaskMediaTest(t)
	previousMemoryCache := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = false
	t.Cleanup(func() { common.MemoryCacheEnabled = previousMemoryCache })

	c, recorder := videoArtifactTestRequest(task, nil)
	TaskArtifactContent(c)
	require.Equal(t, http.StatusOK, recorder.Code)

	// The cover is written off the request path, so the assertion waits for it.
	deadline := time.Now().Add(5 * time.Second)
	for !service.IsTaskPosterStored(task) && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	require.True(t, service.IsTaskPosterStored(task), "the first frame should become the stored cover")
	stored, ok := driver.objects["task-artifacts/"+task.TaskID+"/poster"]
	require.True(t, ok)
	assert.Equal(t, coverBytes, stored.body)
	assert.Equal(t, "image/jpeg", stored.contentType)

	// The list advertises the cover so the grid paints it before playback.
	items := tasksToDto([]*model.Task{task}, false, common.RoleCommonUser, true)
	require.Len(t, items, 1)
	assert.Contains(t, items[0].PosterURL, "/artifacts/poster/content")
}

func TestStoredVideoArtifactServesByteRanges(t *testing.T) {
	driver := newInfluencerPresetTestDriver()
	previousDriver := storage.GetStorageDriver()
	storage.SetGlobalDriver(driver)
	t.Cleanup(func() { storage.SetGlobalDriver(previousDriver) })

	body := []byte("0123456789")
	driver.objects["task-artifacts/task_range/video"] = influencerPresetTestObject{body: body, contentType: "video/mp4"}
	task := &model.Task{TaskID: "task_range", Status: model.TaskStatusSuccess}
	ref := &service.StoredArtifactRef{ObjectKey: "task-artifacts/task_range/video", MimeType: "video/mp4"}

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/tasks/task_range/artifacts/video/content", nil)
	c.Request.Header.Set("Range", "bytes=2-5")
	require.NoError(t, service.GetTaskArtifactStore().Serve(c, task, ref))

	assert.Equal(t, http.StatusPartialContent, recorder.Code)
	assert.Equal(t, "bytes 2-5/10", recorder.Header().Get("Content-Range"))
	assert.Equal(t, "bytes", recorder.Header().Get("Accept-Ranges"))
	assert.Equal(t, []byte("2345"), recorder.Body.Bytes())

	// A suffix range asks for the tail of the object.
	recorder = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/tasks/task_range/artifacts/video/content", nil)
	c.Request.Header.Set("Range", "bytes=-3")
	require.NoError(t, service.GetTaskArtifactStore().Serve(c, task, ref))

	assert.Equal(t, http.StatusPartialContent, recorder.Code)
	assert.Equal(t, "bytes 7-9/10", recorder.Header().Get("Content-Range"))
	assert.Equal(t, []byte("789"), recorder.Body.Bytes())

	// A start past the end is unsatisfiable rather than silently clamped.
	recorder = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/tasks/task_range/artifacts/video/content", nil)
	c.Request.Header.Set("Range", "bytes=99-")
	require.NoError(t, service.GetTaskArtifactStore().Serve(c, task, ref))

	assert.Equal(t, http.StatusRequestedRangeNotSatisfiable, recorder.Code)
	assert.Equal(t, "bytes */10", recorder.Header().Get("Content-Range"))
}

func TestVideoArtifactPersistenceSkipsNonSuccessUpstream(t *testing.T) {
	// A 206 is the provider's own ranged answer, not a whole artifact: the
	// buffered path must refuse it and keep streaming instead.
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "video/mp4")
		w.Header().Set("Content-Range", "bytes 0-13/14")
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write([]byte("artifact-bytes"))
	}))
	defer upstream.Close()

	task := videoArtifactTestTask(t, upstream.URL)
	driver := newInfluencerPresetTestDriver()
	previousDriver := storage.GetStorageDriver()
	storage.SetGlobalDriver(driver)
	t.Cleanup(func() { storage.SetGlobalDriver(previousDriver) })
	allowPrivateTaskMediaTest(t)
	previousMemoryCache := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = false
	t.Cleanup(func() { common.MemoryCacheEnabled = previousMemoryCache })

	c, recorder := videoArtifactTestRequest(task, map[string]string{"Range": "bytes=0-13"})
	TaskArtifactContent(c)

	assert.Equal(t, http.StatusPartialContent, recorder.Code)
	assert.Equal(t, []byte("artifact-bytes"), recorder.Body.Bytes())
	_, stored := driver.objects["task-artifacts/"+task.TaskID+"/video"]
	assert.False(t, stored, "a partial provider response must not be stored as the whole video")
}

func TestTaskMediaChannelTLSSettingRemainsIsolated(t *testing.T) {
	for _, route := range []string{"direct", "connect proxy"} {
		t.Run(route, func(t *testing.T) {
			videoBytes := []byte("0123456789-channel-video")
			var upstreamHits, connectHits atomic.Int32
			upstream := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				upstreamHits.Add(1)
				assert.Equal(t, http.MethodGet, r.Method)
				assert.Equal(t, "/video.mp4", r.URL.Path)
				w.Header().Set("Content-Type", "video/mp4")
				http.ServeContent(w, r, "video.mp4", time.Time{}, bytes.NewReader(videoBytes))
			}))
			t.Cleanup(upstream.Close)

			proxyURL := ""
			if route == "connect proxy" {
				upstreamHost := strings.TrimPrefix(upstream.URL, "https://")
				connectProxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Method != http.MethodConnect || r.Host != upstreamHost {
						w.WriteHeader(http.StatusForbidden)
						return
					}
					connectHits.Add(1)
					target, err := net.DialTimeout("tcp", upstreamHost, 5*time.Second)
					if err != nil {
						w.WriteHeader(http.StatusBadGateway)
						return
					}
					defer target.Close()
					conn, _, err := http.NewResponseController(w).Hijack()
					if err != nil {
						return
					}
					defer conn.Close()
					_, _ = conn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))
					go func() { _, _ = io.Copy(target, conn) }()
					_, _ = io.Copy(conn, target)
				}))
				t.Cleanup(connectProxy.Close)
				proxyURL = connectProxy.URL
			}

			task := setupGenericTaskTest(t)
			allowPrivateTaskMediaTest(t)
			require.False(t, common.TLSInsecureSkipVerify, "this test must exercise channel-only TLS exceptions")
			require.True(t, system_setting.GetFetchSetting().EnableSSRFProtection)
			previousMemoryCache := common.MemoryCacheEnabled
			common.MemoryCacheEnabled = false
			t.Cleanup(func() { common.MemoryCacheEnabled = previousMemoryCache })
			t.Cleanup(service.ResetProxyClientCache)

			channel, err := model.GetChannelById(task.ChannelId, true)
			require.NoError(t, err)
			channel.SetSetting(dto.ChannelSettings{Proxy: proxyURL})
			require.NoError(t, model.DB.Save(channel).Error)
			exceptionChannel := &model.Channel{
				Id: 2, Name: "channel-tls-exception", Key: "key", Status: common.ChannelStatusEnabled,
			}
			exceptionChannel.SetSetting(dto.ChannelSettings{Proxy: proxyURL, TLSInsecureSkipVerify: true})
			require.NoError(t, model.DB.Create(exceptionChannel).Error)
			descriptor := &relaychannel.TaskContentRequest{
				URL: upstream.URL + "/video.mp4", Method: http.MethodGet, Credentialless: true,
			}

			phases := []struct {
				name        string
				channelID   int
				wantSuccess bool
				denyPrivate bool
			}{
				{name: "default rejects untrusted certificate", channelID: channel.Id},
				{name: "channel exception accepts certificate", channelID: exceptionChannel.Id, wantSuccess: true},
				{name: "default still rejects after exception", channelID: channel.Id},
				{name: "exception still rejects forbidden private address", channelID: exceptionChannel.Id, denyPrivate: true},
			}
			for _, phase := range phases {
				t.Run(phase.name, func(t *testing.T) {
					currentTask := *task
					currentTask.ChannelId = phase.channelID
					if phase.denyPrivate {
						system_setting.GetFetchSetting().AllowPrivateIp = false
					}
					for _, operation := range []string{"playback", "range playback", "whole video persistence"} {
						t.Run(operation, func(t *testing.T) {
							beforeHits := upstreamHits.Load()
							beforeConnectHits := connectHits.Load()
							var headers map[string]string
							if operation == "range playback" {
								headers = map[string]string{"Range": "bytes=2-5"}
							}
							c, recorder := videoArtifactTestRequest(&currentTask, headers)
							var requestErr error
							if operation == "whole video persistence" {
								var media *bufferedTaskMedia
								media, requestErr = fetchTaskMediaForPersistContext(c.Request.Context(), c, &currentTask, descriptor, 1<<20, nil)
								if phase.wantSuccess {
									require.NoError(t, requestErr)
									require.NotNil(t, media)
									assert.Equal(t, videoBytes, media.data)
									assert.Equal(t, "video/mp4", media.mimeType)
								} else {
									assert.Nil(t, media)
								}
							} else {
								requestErr = proxyTaskMedia(c, &currentTask, descriptor)
								if phase.wantSuccess {
									require.NoError(t, requestErr)
									assert.Equal(t, "video/mp4", recorder.Header().Get("Content-Type"))
									if operation == "range playback" {
										assert.Equal(t, http.StatusPartialContent, recorder.Code)
										assert.Equal(t, "bytes 2-5/24", recorder.Header().Get("Content-Range"))
										assert.Equal(t, []byte("2345"), recorder.Body.Bytes())
									} else {
										assert.Equal(t, http.StatusOK, recorder.Code)
										assert.Equal(t, videoBytes, recorder.Body.Bytes())
									}
								}
							}
							if phase.wantSuccess {
								assert.Equal(t, beforeHits+1, upstreamHits.Load(), "each consumer must fetch actual upstream bytes")
								return
							}
							require.Error(t, requestErr)
							assert.Equal(t, beforeHits, upstreamHits.Load(), "rejected requests must not consume provider content")
							if phase.denyPrivate {
								assert.Contains(t, requestErr.Error(), "private IP address not allowed")
								assert.Equal(t, beforeConnectHits, connectHits.Load(), "forbidden targets must be blocked before CONNECT")
							} else {
								var certificateErr *tls.CertificateVerificationError
								assert.ErrorAs(t, requestErr, &certificateErr)
							}
						})
					}
				})
			}
			assert.Equal(t, int32(3), upstreamHits.Load())
			if route == "connect proxy" {
				assert.Positive(t, connectHits.Load(), "the HTTPS requests must traverse the CONNECT proxy")
			}
			require.True(t, system_setting.GetFetchSetting().EnableSSRFProtection)
			require.False(t, common.TLSInsecureSkipVerify)
		})
	}
}
