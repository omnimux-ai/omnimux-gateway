package typesafe

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

type trackingReadCloser struct {
	reader    io.Reader
	readErr   error
	closeCall int
}

func (body *trackingReadCloser) Read(p []byte) (int, error) {
	if body.readErr != nil {
		return 0, body.readErr
	}
	return body.reader.Read(p)
}

func (body *trackingReadCloser) Close() error {
	body.closeCall++
	return nil
}

func TestTypeSafeAdaptor(t *testing.T) {
	adaptor := &Adaptor{}

	t.Run("GetRequestURL default", func(t *testing.T) {
		info := &relaycommon.RelayInfo{}
		u, err := adaptor.GetRequestURL(info)
		require.NoError(t, err)
		assert.Equal(t, "https://api.typesafe.ai/v1/systemone", u)
	})

	t.Run("GetRequestURL nil info", func(t *testing.T) {
		u, err := adaptor.GetRequestURL(nil)
		require.NoError(t, err)
		assert.Equal(t, "https://api.typesafe.ai/v1/systemone", u)
	})

	t.Run("GetRequestURL custom base", func(t *testing.T) {
		info := &relaycommon.RelayInfo{
			ChannelMeta: &relaycommon.ChannelMeta{
				ChannelBaseUrl: "https://custom.typesafe.mock",
			},
		}
		u, err := adaptor.GetRequestURL(info)
		require.NoError(t, err)
		assert.Equal(t, "https://custom.typesafe.mock/v1/systemone", u)
	})

	t.Run("SetupRequestHeader", func(t *testing.T) {
		info := &relaycommon.RelayInfo{
			ChannelMeta: &relaycommon.ChannelMeta{
				ApiKey: "sk-typesafe-test",
			},
		}
		reqHeader := http.Header{}
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/systemone", nil)
		c.Request.Header.Set("Content-Type", "application/json")
		err := adaptor.SetupRequestHeader(c, &reqHeader, info)
		require.NoError(t, err)
		assert.Equal(t, "Bearer sk-typesafe-test", reqHeader.Get("Authorization"))
		assert.Equal(t, "application/json", reqHeader.Get("Content-Type"))
	})

	t.Run("DoResponse parses usage and pipes body", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		upstreamBody := `{"model":"jev-1.13.0","judgments":[{"id":"q1","probability":0.95}],"usage":{"input_tokens":250,"output_tokens":15}}`
		httpResp := &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(bytes.NewBufferString(upstreamBody)),
		}

		info := &relaycommon.RelayInfo{}
		usage, apiErr := adaptor.DoResponse(c, httpResp, info)
		require.Nil(t, apiErr)
		require.NotNil(t, usage)

		u, ok := usage.(*dto.Usage)
		require.True(t, ok)
		assert.Equal(t, 250, u.PromptTokens)
		assert.Equal(t, 15, u.CompletionTokens)
		assert.Equal(t, 265, u.TotalTokens)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.JSONEq(t, upstreamBody, w.Body.String())
	})

	t.Run("DoResponse closes body after read error", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		readErr := errors.New("upstream read failed")
		body := &trackingReadCloser{readErr: readErr}
		httpResp := &http.Response{Body: body}

		usage, apiErr := adaptor.DoResponse(c, httpResp, &relaycommon.RelayInfo{})
		require.Nil(t, usage)
		require.NotNil(t, apiErr)
		assert.Equal(t, readErr.Error(), apiErr.Error())
		assert.Equal(t, 1, body.closeCall)
	})

	t.Run("DoResponse closes body once after successful read", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		responseBody := `{"model":"jev","usage":{"input_tokens":1,"output_tokens":0}}`
		body := &trackingReadCloser{reader: bytes.NewBufferString(responseBody)}
		httpResp := &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       body,
		}

		usage, apiErr := adaptor.DoResponse(c, httpResp, &relaycommon.RelayInfo{})
		require.Nil(t, apiErr)
		require.NotNil(t, usage)
		assert.Equal(t, 1, body.closeCall)
	})

	t.Run("DoResponse returns error on malformed upstream response", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		invalidBody := `{"model": invalid json`
		httpResp := &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(bytes.NewBufferString(invalidBody)),
		}

		info := &relaycommon.RelayInfo{}
		usage, apiErr := adaptor.DoResponse(c, httpResp, info)
		require.NotNil(t, apiErr)
		assert.Nil(t, usage)
		assert.Equal(t, types.ErrorCodeBadResponseBody, apiErr.GetErrorCode())
	})

	t.Run("DoResponse returns error on missing or zero usage", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		zeroUsageBody := `{"model":"jev-1.13.0","judgments":[],"usage":{"input_tokens":0,"output_tokens":0}}`
		httpResp := &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(bytes.NewBufferString(zeroUsageBody)),
		}

		info := &relaycommon.RelayInfo{}
		usage, apiErr := adaptor.DoResponse(c, httpResp, info)
		require.NotNil(t, apiErr)
		assert.Nil(t, usage)
		assert.Contains(t, apiErr.Error(), "missing or invalid usage")
	})

	t.Run("Model list and channel name", func(t *testing.T) {
		assert.Equal(t, "typesafe", adaptor.GetChannelName())
		assert.Equal(t, []string{"jev"}, adaptor.GetModelList())
	})

	t.Run("ChannelType registration", func(t *testing.T) {
		assert.Equal(t, "TypeSafe", constant.GetChannelTypeName(constant.ChannelTypeTypeSafe))
	})

}
