package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/relay/channel/task/sora"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestRelayImageRoutingDecision(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 1. DALL-E 3 on Type-1 OpenAI channel should NOT match async image
	assert.False(t, sora.IsAsyncImageModel("dall-e-3"))

	// 2. grok-imagine-image on Type-1 OpenAI channel should match async image
	assert.True(t, sora.IsAsyncImageModel("grok-imagine-image"))
	assert.True(t, sora.IsAsyncImageModel("grok-imagine-image-2-0"))
	assert.True(t, sora.IsAsyncImageModel("grok-imagine-image-lite"))
}

func TestGrok2apiSyncImageRewritesURLAndTaskCompletes(t *testing.T) {
	prevServer := system_setting.ServerAddress
	system_setting.ServerAddress = "https://omnimux.ai"
	t.Cleanup(func() { system_setting.ServerAddress = prevServer })

	syncBody := []byte(`{
		"created": 1783860000,
		"data": [
			{"url": "http://127.0.0.1:8000/v1/media/images/img_test123.jpg"}
		]
	}`)

	origURL, isSync := sora.LooksLikeSyncImageResponse(syncBody)
	assert.True(t, isSync)
	assert.Equal(t, "http://127.0.0.1:8000/v1/media/images/img_test123.jpg", origURL)

	adaptor := &sora.TaskAdaptor{}
	adaptor.Init(nil)
	assert.NotNil(t, adaptor)
}
