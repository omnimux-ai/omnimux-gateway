package relay

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	hosttypes "github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type mockImageReservation struct {
	held, limit int
}

func (s *mockImageReservation) Reserve(quota int) error {
	if quota > s.limit {
		return errors.New("insufficient quota")
	}
	s.held = max(s.held, quota)
	return nil
}
func (s *mockImageReservation) GetPreConsumedQuota() int { return s.held }
func (*mockImageReservation) Settle(int) error           { return nil }
func (*mockImageReservation) Refund(*gin.Context)        {}
func (*mockImageReservation) NeedsRefund() bool          { return false }

func setupTestDB(t *testing.T) {
	t.Helper()
	oldDB, oldLogDB := model.DB, model.LOG_DB
	oldRedis, oldBatch := common.RedisEnabled, common.BatchUpdateEnabled
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	model.DB, model.LOG_DB = db, db
	common.RedisEnabled, common.BatchUpdateEnabled = false, false
	t.Cleanup(func() {
		model.DB, model.LOG_DB = oldDB, oldLogDB
		common.RedisEnabled, common.BatchUpdateEnabled = oldRedis, oldBatch
		_ = sqlDB.Close()
	})
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Token{}, &model.Log{}, &model.Channel{}))
	require.NoError(t, db.Create(&model.User{Id: 1, Username: "test-user", Quota: 1000000}).Error)
	require.NoError(t, db.Create(&model.Token{Id: 1, UserId: 1, Key: "test-token", RemainQuota: 1000000, Status: common.TokenStatusEnabled}).Error)
}

func TestImageHelper_VolcImageUpscale_E2E_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	setupTestDB(t)

	// Mock upstream Volcengine service
	upstreamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "CVProcess", r.URL.Query().Get("Action"))
		assert.Equal(t, "2022-08-31", r.URL.Query().Get("Version"))
		assert.NotEmpty(t, r.Header.Get("Authorization"))

		bodyBytes, err := io.ReadAll(r.Body)
		require.NoError(t, err)

		var payload map[string]any
		require.NoError(t, json.Unmarshal(bodyBytes, &payload))
		assert.Equal(t, "lens_vida_nnsr", payload["req_key"])
		assert.Equal(t, true, payload["return_url"])

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"code": 10000,
			"message": "Success",
			"data": {
				"image_urls": ["https://visual.volcengine.com/upscaled_result_2x.png"],
				"request_id": "test-req-001"
			}
		}`))
	}))
	defer upstreamServer.Close()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	reqPayload := `{"model":"volc-image-upscale","image":"https://example.com/test_500x500.png"}`
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/edits", bytes.NewBufferString(reqPayload))
	c.Request.Header.Set("Content-Type", "application/json")

	common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeJimeng)
	common.SetContextKey(c, constant.ContextKeyChannelKey, "test_ak|test_sk")
	common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, upstreamServer.URL)
	common.SetContextKey(c, constant.ContextKeyOriginalModel, "volc-image-upscale")

	var imgReq dto.ImageRequest
	require.NoError(t, json.Unmarshal([]byte(reqPayload), &imgReq))
	imgReq.N = common.GetPointer(uint(1))

	reservation := &mockImageReservation{limit: 1000000, held: 441}
	info := &relaycommon.RelayInfo{
		UserId:          1,
		TokenId:         1,
		RelayMode:       relayconstant.RelayModeImagesEdits,
		OriginModelName: "volc-image-upscale",
		Request:         &imgReq,
		StartTime:       time.Now(),
		Billing:         reservation,
		PriceData:       hosttypes.PriceData{UsePrice: true, ModelPrice: 0.0008823529, GroupRatioInfo: hosttypes.GroupRatioInfo{GroupRatio: 1}},
	}

	apiErr := ImageHelper(c, info)
	require.Nil(t, apiErr)

	assert.Equal(t, http.StatusOK, w.Code)
	var oaiResp dto.ImageResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &oaiResp))
	require.Len(t, oaiResp.Data, 1)
	assert.Equal(t, "https://visual.volcengine.com/upscaled_result_2x.png", oaiResp.Data[0].Url)
}

func TestImageHelper_VolcImageUpscale_E2E_UpstreamError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	setupTestDB(t)

	// Mock upstream Volcengine service returning business error
	upstreamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"code": 50005,
			"message": "input image resolution too small",
			"data": {}
		}`))
	}))
	defer upstreamServer.Close()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	reqPayload := `{"model":"volc-image-upscale","image":"https://example.com/test.png"}`
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/edits", bytes.NewBufferString(reqPayload))
	c.Request.Header.Set("Content-Type", "application/json")

	common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeJimeng)
	common.SetContextKey(c, constant.ContextKeyChannelKey, "test_ak|test_sk")
	common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, upstreamServer.URL)
	common.SetContextKey(c, constant.ContextKeyOriginalModel, "volc-image-upscale")

	var imgReq dto.ImageRequest
	require.NoError(t, json.Unmarshal([]byte(reqPayload), &imgReq))
	imgReq.N = common.GetPointer(uint(1))

	reservation := &mockImageReservation{limit: 1000000, held: 441}
	info := &relaycommon.RelayInfo{
		UserId:          1,
		TokenId:         1,
		RelayMode:       relayconstant.RelayModeImagesEdits,
		OriginModelName: "volc-image-upscale",
		Request:         &imgReq,
		StartTime:       time.Now(),
		Billing:         reservation,
		PriceData:       hosttypes.PriceData{UsePrice: true, ModelPrice: 0.0008823529, GroupRatioInfo: hosttypes.GroupRatioInfo{GroupRatio: 1}},
	}

	apiErr := ImageHelper(c, info)
	require.NotNil(t, apiErr)
	assert.NotEqual(t, http.StatusOK, apiErr.StatusCode)
	assert.Contains(t, apiErr.Error(), "input image resolution too small")
}
