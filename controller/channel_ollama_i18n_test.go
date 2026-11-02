package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestOllamaConsoleDynamicMessages(t *testing.T) {
	setupTaskPluginControllerTest(t)
	const modelName = "test:{{unchanged}}"
	fixture := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/pull":
			require.Equal(t, http.MethodPost, r.Method)
			w.Header().Set("Content-Type", "application/x-ndjson")
			_, _ = w.Write([]byte("{\"status\":\"success\"}\n"))
		case "/api/delete":
			require.Equal(t, http.MethodDelete, r.Method)
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte("supplier {{detail}}"))
		default:
			t.Errorf("unexpected local request: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer fixture.Close()
	baseURL := fixture.URL
	channel := model.Channel{Type: constant.ChannelTypeOllama, Name: "local fixture", Key: "fixture-only", BaseURL: &baseURL}
	require.NoError(t, model.DB.Create(&channel).Error)

	t.Run("stream success", func(t *testing.T) {
		response := httptest.NewRecorder()
		context, _ := gin.CreateTestContext(response)
		body, err := common.Marshal(map[string]any{"channel_id": channel.Id, "model_name": modelName})
		require.NoError(t, err)
		context.Request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(string(body)))
		context.Request.Header.Set("Content-Type", "application/json")
		OllamaPullModelStream(context)
		require.Equal(t, http.StatusOK, response.Code)
		require.Equal(t, "text/event-stream", response.Header().Get("Content-Type"))
		events := strings.Split(response.Body.String(), "\n\n")
		require.Len(t, events, 4)
		require.Equal(t, `data: {"status":"success"}`, events[0])
		var message map[string]any
		require.NoError(t, common.Unmarshal([]byte(strings.TrimPrefix(events[1], "data: ")), &message))
		require.Equal(t, "Model "+modelName+" pulled successfully", message["message"])
		require.Equal(t, "Model {{model}} pulled successfully", message["message_key"])
		require.Equal(t, map[string]any{"model": modelName}, message["message_params"])
		require.NotContains(t, message, "success")
		require.Equal(t, "data: [DONE]", events[2])
	})
	t.Run("delete failure", func(t *testing.T) {
		response := httptest.NewRecorder()
		context, _ := gin.CreateTestContext(response)
		body, err := common.Marshal(map[string]any{"channel_id": channel.Id, "model_name": modelName})
		require.NoError(t, err)
		context.Request = httptest.NewRequest(http.MethodDelete, "/", strings.NewReader(string(body)))
		context.Request.Header.Set("Content-Type", "application/json")
		OllamaDeleteModel(context)
		require.Equal(t, http.StatusInternalServerError, response.Code)
		var message map[string]any
		require.NoError(t, common.Unmarshal(response.Body.Bytes(), &message))
		detail := "failed to delete model, status 400: supplier {{detail}}"
		require.Equal(t, false, message["success"])
		require.Equal(t, "Failed to delete model: "+detail, message["message"])
		require.Equal(t, "Failed to delete model: {{error}}", message["message_key"])
		require.Equal(t, map[string]any{"error": detail}, message["message_params"])
	})
}
