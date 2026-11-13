package relay

import (
	"bytes"
	"encoding/json"
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

type recordingBillingReservation struct {
	held      int
	settled   int
	refunded  bool
	limit     int
}

func (r *recordingBillingReservation) Reserve(quota int) error {
	if quota > r.limit {
		return errorsNew("insufficient quota")
	}
	r.held = quota
	return nil
}
func (r *recordingBillingReservation) GetPreConsumedQuota() int { return r.held }
func (r *recordingBillingReservation) Settle(quota int) error {
	r.settled = quota
	return nil
}
func (r *recordingBillingReservation) Refund(*gin.Context) {
	r.refunded = true
}
func (r *recordingBillingReservation) NeedsRefund() bool {
	return !r.refunded && r.settled == 0 && r.held > 0
}

func errorsNew(text string) error {
	return &testCustomError{text: text}
}

type testCustomError struct {
	text string
}

func (e *testCustomError) Error() string { return e.text }

func setupBillingTestDB(t *testing.T) {
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
	require.NoError(t, db.Create(&model.User{Id: 1, Username: "billing-user", Quota: 1000000}).Error)
	require.NoError(t, db.Create(&model.Token{Id: 1, UserId: 1, Key: "billing-token", RemainQuota: 1000000, Status: common.TokenStatusEnabled}).Error)
}

func TestVolcImageUpscale_Billing_ExactDeductionAtGroupRatio1(t *testing.T) {
	gin.SetMode(gin.TestMode)
	setupBillingTestDB(t)

	upstreamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"code": 10000,
			"message": "Success",
			"data": {
				"image_urls": ["https://visual.volcengine.com/upscaled.png"],
				"request_id": "req-bill-001"
			}
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

	// Model price: 0.006 CNY / 6.8 FX = 0.0008823529411764706 USD.
	// 0.0008823529411764706 * 500000 = 441.17647... -> 441 quota.
	modelPriceUSD := 0.006 / 6.8
	reservation := &recordingBillingReservation{limit: 1000000}
	info := &relaycommon.RelayInfo{
		UserId:          1,
		TokenId:         1,
		RelayMode:       relayconstant.RelayModeImagesEdits,
		OriginModelName: "volc-image-upscale",
		Request:         &imgReq,
		StartTime:       time.Now(),
		Billing:         reservation,
		PriceData: hosttypes.PriceData{
			UsePrice:       true,
			ModelPrice:     modelPriceUSD,
			GroupRatioInfo: hosttypes.GroupRatioInfo{GroupRatio: 1.0},
		},
	}

	apiErr := ImageHelper(c, info)
	require.Nil(t, apiErr)
	assert.Equal(t, http.StatusOK, w.Code)

	// Pre-consumed quota must be exactly 441
	assert.Equal(t, 441, info.PriceData.QuotaToPreConsume)
	assert.Equal(t, 441, reservation.held)
}

func TestVolcImageUpscale_Billing_InsufficientQuotaBlocked403(t *testing.T) {
	gin.SetMode(gin.TestMode)

	upstreamCalled := false
	upstreamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalled = true
		w.WriteHeader(http.StatusOK)
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

	// Limit is less than required 441 quota (e.g. user only has 100 quota)
	reservation := &recordingBillingReservation{limit: 100}
	info := &relaycommon.RelayInfo{
		UserId:          1,
		TokenId:         1,
		RelayMode:       relayconstant.RelayModeImagesEdits,
		OriginModelName: "volc-image-upscale",
		Request:         &imgReq,
		StartTime:       time.Now(),
		Billing:         reservation,
		PriceData: hosttypes.PriceData{
			UsePrice:       true,
			ModelPrice:     0.006 / 6.8,
			GroupRatioInfo: hosttypes.GroupRatioInfo{GroupRatio: 1.0},
		},
	}

	apiErr := ImageHelper(c, info)
	require.NotNil(t, apiErr)
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
	assert.False(t, upstreamCalled, "upstream must never be called when user quota is insufficient")
}

func TestVolcImageUpscale_Billing_UpstreamFailureRefundGuaranteed(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name         string
		upstreamCode int
		upstreamBody string
	}{
		{
			name:         "upstream HTTP 500 error",
			upstreamCode: http.StatusInternalServerError,
			upstreamBody: `{"error":"internal upstream error"}`,
		},
		{
			name:         "upstream business code 50005 error",
			upstreamCode: http.StatusOK,
			upstreamBody: `{"code":50005,"message":"image algorithm internal failure","data":{}}`,
		},
		{
			name:         "upstream empty image data",
			upstreamCode: http.StatusOK,
			upstreamBody: `{"code":10000,"message":"Success","data":{"image_urls":[]}}`,
		},
		{
			name:         "upstream malformed non-JSON body",
			upstreamCode: http.StatusOK,
			upstreamBody: `<html>Bad Gateway</html>`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			upstreamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.upstreamCode)
				_, _ = w.Write([]byte(tc.upstreamBody))
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

			reservation := &recordingBillingReservation{limit: 1000000}
			info := &relaycommon.RelayInfo{
				UserId:          1,
				TokenId:         1,
				RelayMode:       relayconstant.RelayModeImagesEdits,
				OriginModelName: "volc-image-upscale",
				Request:         &imgReq,
				StartTime:       time.Now(),
				Billing:         reservation,
				PriceData: hosttypes.PriceData{
					UsePrice:       true,
					ModelPrice:     0.006 / 6.8,
					GroupRatioInfo: hosttypes.GroupRatioInfo{GroupRatio: 1.0},
				},
			}

			apiErr := ImageHelper(c, info)
			require.NotNil(t, apiErr)
			assert.NotEqual(t, http.StatusOK, apiErr.StatusCode)

			// Quota reservation was held before upstream call
			assert.Equal(t, 441, reservation.held)
			// Settlement was NEVER executed because request failed
			assert.Equal(t, 0, reservation.settled)
			// Refund is required
			assert.True(t, reservation.NeedsRefund(), "reservation must require refund when upstream fails")
		})
	}
}
