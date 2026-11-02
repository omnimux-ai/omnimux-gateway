package volcengine

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestASRClientSubmitTask(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, PathAUCBigModelSubmit, r.URL.Path)
		assert.Equal(t, "my-key", r.Header.Get("X-Api-Key"))
		assert.Equal(t, ResourceSeedASRAUC, r.Header.Get("X-Api-Resource-Id"))
		assert.NotEmpty(t, r.Header.Get("X-Api-Request-Id"))
		assert.Equal(t, "-1", r.Header.Get("X-Api-Sequence"))

		w.Header().Set("X-Api-Status-Code", StatusSuccess)
		w.Header().Set("X-Api-Message", "Success")
		w.WriteHeader(http.StatusOK)

		resp := VolcengineASRSubmitResponse{
			TaskId:     "task_abc123",
			StatusCode: StatusSuccess,
		}
		data, _ := common.Marshal(resp)
		_, _ = w.Write(data)
	}))
	defer ts.Close()

	client := NewVolcengineASRClient(ts.URL, ts.Client())
	submitReq := &VolcengineASRSubmitRequest{
		Audio: VolcengineAudioInfo{
			URL:    "https://example.com/audio.mp3",
			Format: "mp3",
		},
		Request: VolcengineASRRequestConfig{
			ModelName:      "bigmodel",
			EnableITN:      true,
			EnablePunc:     true,
			EnableDDC:      true,
			ShowUtterances: true,
		},
	}

	resp, err := client.SubmitTask(context.Background(), "my-key", ResourceSeedASRAUC, submitReq)
	require.NoError(t, err)
	assert.Equal(t, "task_abc123", resp.TaskId)
	assert.Equal(t, StatusSuccess, resp.StatusCode)
}

func TestASRPollerSuccess(t *testing.T) {
	var queryCount int32

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, PathAUCBigModelQuery, r.URL.Path)
		assert.Equal(t, "task_abc123", r.Header.Get("X-Api-Request-Id"))

		count := atomic.AddInt32(&queryCount, 1)
		w.WriteHeader(http.StatusOK)

		if count < 3 {
			// 前两次返回处理中 20000001
			w.Header().Set("X-Api-Status-Code", StatusProcessing)
			resp := VolcengineASRQueryResponse{
				StatusCode: StatusProcessing,
				Message:    "Processing",
			}
			data, _ := common.Marshal(resp)
			_, _ = w.Write(data)
		} else {
			// 第三次返回成功 20000000
			w.Header().Set("X-Api-Status-Code", StatusSuccess)
			resp := VolcengineASRQueryResponse{
				StatusCode: StatusSuccess,
				Result: VolcengineASRResult{
					Text: "测试转写成功",
					Utterances: []VolcengineASRUtterance{
						{
							Text:      "测试转写成功",
							StartTime: 0,
							EndTime:   1500,
						},
					},
				},
			}
			data, _ := common.Marshal(resp)
			_, _ = w.Write(data)
		}
	}))
	defer ts.Close()

	client := NewVolcengineASRClient(ts.URL, ts.Client())
	poller := NewASRPoller(&ASRPollerConfig{
		InitialInterval: 10 * time.Millisecond,
		MaxInterval:     50 * time.Millisecond,
		BackoffFactor:   1.5,
		OverallTimeout:  2 * time.Second,
	})

	result, err := poller.Poll(context.Background(), client, "my-key", ResourceSeedASRAUC, "task_abc123")
	require.NoError(t, err)
	assert.Equal(t, "测试转写成功", result.Text)
	assert.Equal(t, int32(3), atomic.LoadInt32(&queryCount))
}

func TestASRPollerFailureTerminalCode(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Api-Status-Code", "45000001")
		w.WriteHeader(http.StatusOK)

		resp := VolcengineASRQueryResponse{
			StatusCode: "45000001",
			Message:    "Invalid audio format",
		}
		data, _ := common.Marshal(resp)
		_, _ = w.Write(data)
	}))
	defer ts.Close()

	client := NewVolcengineASRClient(ts.URL, ts.Client())
	poller := NewASRPoller(&ASRPollerConfig{
		InitialInterval: 10 * time.Millisecond,
		OverallTimeout:  1 * time.Second,
	})

	_, err := poller.Poll(context.Background(), client, "my-key", ResourceSeedASRAUC, "task_fail")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "45000001")
	assert.Contains(t, err.Error(), "Invalid audio format")
}

func TestASRPollerContextCancel(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Api-Status-Code", StatusProcessing)
		resp := VolcengineASRQueryResponse{
			StatusCode: StatusProcessing,
		}
		data, _ := common.Marshal(resp)
		_, _ = w.Write(data)
	}))
	defer ts.Close()

	client := NewVolcengineASRClient(ts.URL, ts.Client())
	poller := NewASRPoller(&ASRPollerConfig{
		InitialInterval: 50 * time.Millisecond,
		OverallTimeout:  5 * time.Second,
	})

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(30 * time.Millisecond)
		cancel()
	}()

	_, err := poller.Poll(ctx, client, "my-key", ResourceSeedASRAUC, "task_cancel")
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestASRPollerTimeout(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Api-Status-Code", StatusProcessing)
		resp := VolcengineASRQueryResponse{
			StatusCode: StatusProcessing,
		}
		data, _ := common.Marshal(resp)
		_, _ = w.Write(data)
	}))
	defer ts.Close()

	client := NewVolcengineASRClient(ts.URL, ts.Client())
	poller := NewASRPoller(&ASRPollerConfig{
		InitialInterval: 20 * time.Millisecond,
		OverallTimeout:  60 * time.Millisecond,
	})

	_, err := poller.Poll(context.Background(), client, "my-key", ResourceSeedASRAUC, "task_timeout")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "timed out")
}

func TestResolveASRResourceId(t *testing.T) {
	// 默认 2.0
	assert.Equal(t, ResourceSeedASRAUC, resolveASRResourceId(dto.AudioRequest{Model: ModelDoubaoASRBigModel}, nil))
	assert.Equal(t, ResourceSeedASRAUC, resolveASRResourceId(dto.AudioRequest{Model: ModelSeedASRAUC}, nil))

	// 1.0
	assert.Equal(t, ResourceBigASRAUC, resolveASRResourceId(dto.AudioRequest{Model: ModelBigASRAUC}, nil))
	assert.Equal(t, ResourceBigASRAUC, resolveASRResourceId(dto.AudioRequest{Model: "bigasr-1.0"}, nil))

	// 显式指定
	assert.Equal(t, "custom.resource.id", resolveASRResourceId(dto.AudioRequest{ResourceId: "custom.resource.id"}, nil))
}

func TestInferAudioFormat(t *testing.T) {
	assert.Equal(t, "wav", inferAudioFormat(dto.AudioRequest{}, "https://example.com/audio.wav?v=1"))
	assert.Equal(t, "m4a", inferAudioFormat(dto.AudioRequest{}, "https://example.com/voice.m4a"))
	assert.Equal(t, "mp3", inferAudioFormat(dto.AudioRequest{}, "https://example.com/file.mp3"))
	assert.Equal(t, "mp4", inferAudioFormat(dto.AudioRequest{}, "https://example.com/video.mp4"))
	assert.Equal(t, "ogg", inferAudioFormat(dto.AudioRequest{}, "https://example.com/music.ogg"))
	assert.Equal(t, "mp3", inferAudioFormat(dto.AudioRequest{}, "https://example.com/unknown_ext"))

	// 显式指定覆盖
	assert.Equal(t, "flac", inferAudioFormat(dto.AudioRequest{Format: "flac"}, "https://example.com/audio.wav"))
}

func TestHandleASRE2E(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case PathAUCBigModelSubmit:
			w.Header().Set("X-Api-Status-Code", StatusSuccess)
			resp := VolcengineASRSubmitResponse{
				TaskId:     "task_e2e_001",
				StatusCode: StatusSuccess,
			}
			data, _ := common.Marshal(resp)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(data)
		case PathAUCBigModelQuery:
			assert.Equal(t, "task_e2e_001", r.Header.Get("X-Api-Request-Id"))
			w.Header().Set("X-Api-Status-Code", StatusSuccess)
			resp := VolcengineASRQueryResponse{
				StatusCode: StatusSuccess,
				Result: VolcengineASRResult{
					Text: "测试端到端完整链路转写",
					Utterances: []VolcengineASRUtterance{
						{
							Text:      "测试端到端完整链路转写",
							StartTime: 0,
							EndTime:   5420, // 5.42 秒 -> ceil(5.42/60*1000) = 91 tokens
							Additions: VolcengineASRAdditions{
								Speaker:    "speaker_0",
								Emotion:    "happy",
								SpeechRate: 4.5,
							},
						},
					},
				},
			}
			data, _ := common.Marshal(resp)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(data)
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	// 1. 测试 JSON 格式
	{
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/audio/transcriptions", nil)

		info := &relaycommon.RelayInfo{
			OriginModelName: ModelDoubaoASRBigModel,
			RelayMode:       constant.RelayModeAudioTranscription,
			ChannelMeta: &relaycommon.ChannelMeta{
				ChannelBaseUrl: ts.URL,
				ApiKey:         "test-key",
			},
		}

		req := dto.AudioRequest{
			Model:          ModelDoubaoASRBigModel,
			URL:            "https://example.com/audio.mp3",
			ResponseFormat: "json",
		}

		adaptor := &Adaptor{}
		reader, err := adaptor.ConvertAudioRequest(c, info, req)
		require.NoError(t, err)
		require.NotNil(t, reader)

		reqResp, err := adaptor.DoRequest(c, info, reader)
		require.NoError(t, err)
		assert.Nil(t, reqResp)

		usageObj, apiErr := adaptor.DoResponse(c, nil, info)
		require.Nil(t, apiErr)
		require.NotNil(t, usageObj)

		usage, ok := usageObj.(*dto.Usage)
		require.True(t, ok)
		assert.Equal(t, 91, usage.PromptTokens)
		assert.Equal(t, 91, usage.TotalTokens)
		assert.Equal(t, 91, usage.PromptTokensDetails.AudioTokens)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

		var resp dto.AudioResponse
		err = common.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "测试端到端完整链路转写", resp.Text)
	}

	// 2. 测试 SRT 格式
	{
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/audio/transcriptions", nil)

		info := &relaycommon.RelayInfo{
			OriginModelName: ModelDoubaoASRBigModel,
			RelayMode:       constant.RelayModeAudioTranscription,
			ChannelMeta: &relaycommon.ChannelMeta{
				ChannelBaseUrl: ts.URL,
				ApiKey:         "test-key",
			},
		}

		req := dto.AudioRequest{
			Model:          ModelDoubaoASRBigModel,
			URL:            "https://example.com/audio.mp3",
			ResponseFormat: "srt",
		}

		adaptor := &Adaptor{}
		_, _ = adaptor.ConvertAudioRequest(c, info, req)
		usageObj, apiErr := adaptor.DoResponse(c, nil, info)
		require.Nil(t, apiErr)
		require.NotNil(t, usageObj)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "text/plain; charset=utf-8", w.Header().Get("Content-Type"))
		assert.Contains(t, w.Body.String(), "00:00:00,000 --> 00:00:05,420")
		assert.Contains(t, w.Body.String(), "测试端到端完整链路转写")
	}
}

