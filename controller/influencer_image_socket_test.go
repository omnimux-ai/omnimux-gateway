package controller

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"net/http"
	"testing"
)

func TestInfluencerSunburstResponsesSocketRejectsGenerationBeforeFunding(t *testing.T) {
	fixture := newResponsesWSBillingTest(t, `tier("request",fixed(0.002))`, func(ws *websocket.Conn, _ *http.Request) {
		_, _, err := ws.ReadMessage()
		if err != nil {
			return
		}
		_ = ws.WriteMessage(websocket.TextMessage, []byte(`{"type":"response.completed","response":{"id":"local-only-response","status":"completed","output":[],"usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}}`))
	})
	require.NoError(t, model.DB.Model(fixture.channel).Update("models", influencerImageModel).Error)
	require.NoError(t, model.DB.Model(&model.Ability{}).Where("channel_id = ?", fixture.channel.Id).Update("model", influencerImageModel).Error)
	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{"billing_setting.billing_mode": `{"ws-billing":"tiered_expr","gpt-image-2.5-sunburst":"tiered_expr"}`, "billing_setting.billing_expr": `{"ws-billing":"tier(\"request\",fixed(0.002))","gpt-image-2.5-sunburst":"tier(\"request\",fixed(0.002))"}`}))
	for _, payload := range []string{
		`{"type":"response.create","event_id":"closed","model":"gpt-image-2.5-sunburst","input":"local-only request","task_type":"image_task_v1","influencer_image_inputs":{"version":1,"mode":"text"}}`,
		`{"type":"response.create","event_id":"closed-again","response":{"model":"gpt-image-2.5-sunburst","input":"local-only request","task_type":"image_task_v1","generation_queue_reserved_task":{"id":1}}}`,
	} {
		require.NoError(t, fixture.client.WriteMessage(websocket.TextMessage, []byte(payload)))
		event := readResponsesWSTestEvent(t, fixture.client)
		t.Logf("actual local websocket event=%v local_upstream_connections=%d", event, fixture.connections.Load())
		assert.Equal(t, "error", event["type"])
		assert.Equal(t, float64(http.StatusServiceUnavailable), event["status"])
		data, err := common.Marshal(event)
		require.NoError(t, err)
		assert.Contains(t, string(data), helper.ErrImageTaskDurableEntryRequired.Error())
		assert.Zero(t, fixture.connections.Load())
	}
	fixture.closeAndWait(t)
	assertResponsesWSAccounting(t, fixture, nil)
}
