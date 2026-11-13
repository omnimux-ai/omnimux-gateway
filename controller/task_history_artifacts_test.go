package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTaskHistoryProjectsPersistedImageWithoutExposingSnapshot(t *testing.T) {
	task := setupGenericTaskTest(t)
	task.Action = ActionInfluencerSheet
	task.Platform = constant.TaskPlatform("sora")
	task.PrivateData.Execution = &model.TaskExecutionSnapshot{TaskPlugin: &model.TaskPluginSnapshot{Key: "sora", Version: "1.1.0", APIVersion: 1}}
	task.SetData(map[string]any{"status": "completed", "type": "image", "results": []string{"https://private.example/image.png?signature=private"}})
	require.NoError(t, model.DB.Save(task).Error)
	previousSecret, previousAddress := common.CryptoSecret, system_setting.TaskPublicAddress
	common.CryptoSecret = "history-artifact-test-secret"
	system_setting.TaskPublicAddress = "https://gateway.example"
	t.Cleanup(func() { common.CryptoSecret = previousSecret; system_setting.TaskPublicAddress = previousAddress })
	for _, admin := range []bool{false, true} {
		name := "user"
		if admin {
			name = "admin"
		}
		t.Run(name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Set("id", task.UserId)
			c.Set("role", common.RoleRootUser)
			c.Request = httptest.NewRequest(http.MethodGet, "/api/task/self?p=1&page_size=10&action=influencer_sheet&with_artifacts=true", nil)
			if admin {
				GetAllTask(c)
			} else {
				GetUserTask(c)
			}
			require.Equal(t, http.StatusOK, recorder.Code)
			var response struct {
				Success bool `json:"success"`
				Data    struct {
					Items []*dto.TaskDto `json:"items"`
				}
			}
			require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
			require.True(t, response.Success)
			require.Len(t, response.Data.Items, 1)
			item := response.Data.Items[0]
			require.Len(t, item.Artifacts, 1)
			assert.Equal(t, "image", item.Artifacts[0].Key)
			assert.Contains(t, item.Artifacts[0].ContentURL, "/artifacts/image/content")
			assert.JSONEq(t, "null", string(item.Data))
			if !admin {
				assert.Zero(t, item.ChannelId, "the internal channel remains hidden from user history")
			}
			assert.NotContains(t, recorder.Body.String(), "private.example")
			assert.NotContains(t, recorder.Body.String(), "signature=private")
		})
	}
}
