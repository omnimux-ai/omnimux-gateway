package openai

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type realtimeDebtResult struct {
	err   *types.NewAPIError
	usage *dto.RealtimeUsage
}

type realtimeDebtRelay struct {
	db       *gorm.DB
	client   *websocket.Conn
	upstream *websocket.Conn
	result   <-chan realtimeDebtResult
}

func realtimeDebtConnectionPair(t *testing.T, wrap ...func(net.Conn) net.Conn) (*websocket.Conn, *websocket.Conn) {
	t.Helper()
	type upgradeResult struct {
		conn *websocket.Conn
		err  error
	}
	upgraded := make(chan upgradeResult, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{}
		conn, err := upgrader.Upgrade(w, r, nil)
		upgraded <- upgradeResult{conn: conn, err: err}
	}))
	t.Cleanup(server.Close)
	dialer := *websocket.DefaultDialer
	if len(wrap) > 0 {
		dialer.NetDialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
			conn, err := (&net.Dialer{}).DialContext(ctx, network, address)
			if err != nil {
				return nil, err
			}
			return wrap[0](conn), nil
		}
	}
	peer, _, err := dialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = peer.Close() })
	select {
	case result := <-upgraded:
		require.NoError(t, result.err)
		t.Cleanup(func() { _ = result.conn.Close() })
		return peer, result.conn
	case <-time.After(5 * time.Second):
		t.Fatal("websocket upgrade did not finish")
		return nil, nil
	}
}

func startRealtimeDebtRelay(t *testing.T, info *relaycommon.RelayInfo, setup ...func(*gin.Context, *gorm.DB)) *realtimeDebtRelay {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&model.User{}))
	require.NoError(t, db.Create(&model.User{Id: 1, Username: "realtime-debt", Quota: 10}).Error)
	previousDB := model.DB
	previousRedis, previousBatch := common.RedisEnabled, common.BatchUpdateEnabled
	model.DB = db
	common.RedisEnabled, common.BatchUpdateEnabled = false, false
	t.Cleanup(func() {
		model.DB = previousDB
		common.RedisEnabled, common.BatchUpdateEnabled = previousRedis, previousBatch
		assert.NoError(t, sqlDB.Close())
	})

	client, relayClient := realtimeDebtConnectionPair(t)
	upstream, relayTarget := realtimeDebtConnectionPair(t)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodGet, "/v1/realtime", nil)
	info.UserId = 1
	info.UserQuota = 10 // This stale connection snapshot must not forgive SQL debt.
	info.ChannelMeta = &relaycommon.ChannelMeta{UpstreamModelName: "gpt-realtime"}
	info.InputAudioFormat = "pcm16"
	info.ClientWs, info.TargetWs = relayClient, relayTarget
	if info.TokenId == 0 && !info.IsChannelTest && !info.IsPlayground {
		info.TokenId, info.TokenKey = 1, "realtime-debt-token"
		require.NoError(t, db.AutoMigrate(&model.Token{}))
		require.NoError(t, db.Create(&model.Token{Id: 1, UserId: 1, Key: info.TokenKey, RemainQuota: 1000, UnlimitedQuota: info.TokenUnlimited, Status: common.TokenStatusEnabled}).Error)
		ctx.Set("token_quota", 1000)
	}
	for _, prepare := range setup {
		prepare(ctx, db)
	}
	result := make(chan realtimeDebtResult, 1)
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		apiErr, usage := OpenaiRealtimeHandler(ctx, info)
		if apiErr != nil {
			// Match the existing controller's native realtime error envelope.
			helper.WssError(ctx, relayClient, apiErr.ToOpenAIError())
		}
		_ = relayTarget.Close()
		_ = relayClient.Close()
		result <- realtimeDebtResult{err: apiErr, usage: usage}
	}()
	t.Cleanup(func() {
		_ = client.Close()
		_ = upstream.Close()
		select {
		case <-finished:
		case <-time.After(5 * time.Second):
			t.Error("realtime relay did not stop")
		}
	})
	return &realtimeDebtRelay{db: db, client: client, upstream: upstream, result: result}
}

func TestOpenaiRealtimeFiniteTokenExhaustion(t *testing.T) {
	oldModels, oldGroups := ratio_setting.ModelRatio2JSONString(), ratio_setting.GroupRatio2JSONString()
	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(`{"token-realtime":1}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"token-test":1}`))
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(oldModels))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(oldGroups))
	})
	for _, tc := range []struct {
		name                        string
		allowance, reserved         int
		chunks                      []int
		unlimited, batch, allowNext bool
	}{
		{name: "exactly exhausted", allowance: 120, reserved: 100, chunks: []int{80, 40}},
		{name: "completed overrun still fully charged", allowance: 120, reserved: 100, chunks: []int{80, 60}},
		{name: "entire allowance initially reserved", allowance: 100, reserved: 100, chunks: []int{40, 60}},
		{name: "pending native batch debit", allowance: 120, reserved: 100, chunks: []int{80, 40}, batch: true},
		{name: "unused initial reservation remains available", allowance: 100, reserved: 100, chunks: []int{40, 40}, allowNext: true},
		{name: "unlimited token", allowance: 120, reserved: 100, chunks: []int{80, 60}, unlimited: true, allowNext: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			info := &relaycommon.RelayInfo{TokenId: 1, TokenKey: "finite-realtime-token", TokenUnlimited: tc.unlimited, RequestId: t.Name(), OriginModelName: "token-realtime", UsingGroup: "token-test", UserGroup: "token-test", UserSetting: dto.UserSetting{BillingPreference: "subscription_only"}}
			var billingContext *gin.Context
			relay := startRealtimeDebtRelay(t, info, func(ctx *gin.Context, db *gorm.DB) {
				billingContext = ctx
				require.NoError(t, db.AutoMigrate(&model.Token{}, &model.SubscriptionPlan{}, &model.UserSubscription{}, &model.SubscriptionPreConsumeRecord{}))
				require.NoError(t, db.Model(&model.User{}).Where("id = 1").Update("quota", 0).Error)
				require.NoError(t, db.Create(&model.Token{Id: 1, UserId: 1, Key: info.TokenKey, RemainQuota: tc.allowance, UnlimitedQuota: tc.unlimited, Status: common.TokenStatusEnabled}).Error)
				require.NoError(t, db.Create(&model.SubscriptionPlan{Id: 1, Title: "Realtime token fixture", QuotaResetPeriod: model.SubscriptionResetNever}).Error)
				require.NoError(t, db.Create(&model.UserSubscription{Id: 1, UserId: 1, PlanId: 1, AmountTotal: 10000, Status: "active", StartTime: time.Now().Unix() - 60, EndTime: time.Now().Unix() + 3600}).Error)
				ctx.Set("token_quota", tc.allowance)
				require.Nil(t, service.PreConsumeBilling(ctx, tc.reserved, info))
				common.BatchUpdateEnabled = tc.batch
			})
			if tc.batch {
				t.Cleanup(func() {
					// Remove only this fixture's pending token delta without a
					// background batch flusher or touching another test's queue.
					delta := info.Billing.GetPreConsumedQuota() - tc.reserved
					if delta > 0 {
						require.NoError(t, model.IncreaseTokenQuota(info.TokenId, info.TokenKey, delta))
					}
				})
			}
			const generate = `{"type":"response.create"}`
			actual := 0
			for _, tokens := range tc.chunks {
				require.NoError(t, relay.client.WriteMessage(websocket.TextMessage, []byte(generate)))
				assert.JSONEq(t, generate, string(readRealtimeDebtMessage(t, relay.upstream)))
				usage := &dto.RealtimeUsage{TotalTokens: tokens, InputTokens: tokens}
				usage.InputTokenDetails.TextTokens = tokens
				message, err := common.Marshal(dto.RealtimeEvent{Type: dto.RealtimeEventTypeResponseDone, Response: &dto.RealtimeResponse{Usage: usage}})
				require.NoError(t, err)
				require.NoError(t, relay.upstream.WriteMessage(websocket.TextMessage, message))
				assert.JSONEq(t, string(message), string(readRealtimeDebtMessage(t, relay.client)))
				actual += tokens
			}
			var token model.Token
			var user model.User
			var sub model.UserSubscription
			require.NoError(t, relay.db.First(&token, 1).Error)
			require.NoError(t, relay.db.First(&user, 1).Error)
			require.NoError(t, relay.db.First(&sub, 1).Error)
			assert.Zero(t, user.Quota)
			assert.Equal(t, int64(max(actual, tc.reserved)), sub.AmountUsed)
			if tc.batch {
				assert.Equal(t, tc.allowance-tc.reserved, token.RemainQuota, "native batch debit is still pending")
			}
			const cancel = `{"type":"response.cancel"}`
			require.NoError(t, relay.client.WriteMessage(websocket.TextMessage, []byte(cancel)))
			assert.JSONEq(t, cancel, string(readRealtimeDebtMessage(t, relay.upstream)))
			require.NoError(t, relay.client.WriteMessage(websocket.TextMessage, []byte(generate)))
			if tc.allowNext {
				assert.JSONEq(t, generate, string(readRealtimeDebtMessage(t, relay.upstream)))
				require.NoError(t, relay.client.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")))
			} else {
				var event dto.RealtimeEvent
				require.NoError(t, common.Unmarshal(readRealtimeDebtMessage(t, relay.client), &event))
				require.NotNil(t, event.Error)
				assert.Equal(t, string(types.ErrorCodePreConsumeTokenQuotaFailed), event.Error.Code)
			}
			select {
			case result := <-relay.result:
				if tc.allowNext {
					assert.Nil(t, result.err)
				} else {
					require.NotNil(t, result.err)
					assert.Equal(t, types.ErrorCodePreConsumeTokenQuotaFailed, result.err.GetErrorCode())
					assert.True(t, types.IsSkipRetryError(result.err))
				}
				require.NotNil(t, result.usage)
				assert.Equal(t, actual, result.usage.TotalTokens)
			case <-time.After(5 * time.Second):
				t.Fatal("realtime token test did not close")
			}
			require.NoError(t, relay.upstream.SetReadDeadline(time.Now().Add(5*time.Second)))
			_, message, err := relay.upstream.ReadMessage()
			require.Error(t, err)
			assert.Empty(t, message, "denied generation must not reach upstream")
			require.NoError(t, info.Billing.Settle(info.RealtimeConsumedQuota))
			info.Billing.Refund(billingContext)
			require.NoError(t, relay.db.First(&sub, 1).Error)
			require.NoError(t, relay.db.First(&token, 1).Error)
			require.NoError(t, relay.db.First(&user, 1).Error)
			assert.Equal(t, int64(actual), sub.AmountUsed)
			assert.Zero(t, user.Quota)
			if !tc.batch {
				assert.Equal(t, actual, token.UsedQuota)
				assert.Equal(t, tc.allowance-actual, token.RemainQuota)
			}
		})
	}
}

func TestOpenaiRealtimeSettlementErrorReturnsObservedUsageOnce(t *testing.T) {
	oldModels, oldGroups := ratio_setting.ModelRatio2JSONString(), ratio_setting.GroupRatio2JSONString()
	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(`{"token-realtime":1}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"token-test":1}`))
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(oldModels))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(oldGroups))
	})
	info := &relaycommon.RelayInfo{BillingSource: service.BillingSourceSubscription, SubscriptionId: 1, OriginModelName: "token-realtime", UsingGroup: "token-test", UserGroup: "token-test"}
	var billingContext *gin.Context
	relay := startRealtimeDebtRelay(t, info, func(ctx *gin.Context, db *gorm.DB) {
		billingContext = ctx
		require.NoError(t, db.AutoMigrate(&model.UserSubscription{}))
		require.NoError(t, db.Model(&model.User{}).Where("id = 1").Update("quota", 0).Error)
		require.NoError(t, db.Create(&model.UserSubscription{Id: 1, UserId: 1, AmountTotal: 10000, Status: "active"}).Error)
		var failOnce sync.Once
		require.NoError(t, db.Callback().Update().Before("gorm:update").Register("realtime_settlement_failure", func(tx *gorm.DB) {
			if tx.Statement.Table == "user_subscriptions" {
				failOnce.Do(func() { _ = tx.AddError(errors.New("fixture subscription write failure")) })
			}
		}))
	})
	const generate = `{"type":"response.create"}`
	require.NoError(t, relay.client.WriteMessage(websocket.TextMessage, []byte(generate)))
	assert.JSONEq(t, generate, string(readRealtimeDebtMessage(t, relay.upstream)))
	const completed = `{"type":"response.done","response":{"usage":{"total_tokens":140,"input_tokens":140,"input_token_details":{"text_tokens":140}}}}`
	require.NoError(t, relay.upstream.WriteMessage(websocket.TextMessage, []byte(completed)))
	var event dto.RealtimeEvent
	require.NoError(t, common.Unmarshal(readRealtimeDebtMessage(t, relay.client), &event))
	require.NotNil(t, event.Error)
	assert.Equal(t, string(types.ErrorCodeUpdateDataError), event.Error.Code)
	select {
	case result := <-relay.result:
		require.NotNil(t, result.err)
		assert.Equal(t, types.ErrorCodeUpdateDataError, result.err.GetErrorCode())
		require.NotNil(t, result.usage)
		assert.Equal(t, 140, result.usage.TotalTokens, "tail must not accumulate a failed completed response twice")
		// Keep the notification-only snapshot above its warning threshold:
		// this settlement fixture does not own asynchronous email delivery.
		info.UserQuota = common.QuotaRemindThreshold + result.usage.TotalTokens + 1
		require.NoError(t, service.SettleBilling(billingContext, info, result.usage.TotalTokens))
	case <-time.After(5 * time.Second):
		t.Fatal("failed realtime settlement did not return")
	}
	var token model.Token
	var sub model.UserSubscription
	require.NoError(t, relay.db.First(&token, 1).Error)
	require.NoError(t, relay.db.First(&sub, 1).Error)
	assert.Equal(t, 140, token.UsedQuota)
	assert.Equal(t, int64(140), sub.AmountUsed)
}

func readRealtimeDebtMessage(t *testing.T, conn *websocket.Conn) []byte {
	t.Helper()
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(5*time.Second)))
	_, message, err := conn.ReadMessage()
	require.NoError(t, err)
	return message
}

func TestOpenaiRealtimeRejectsGenerationAfterWalletDebt(t *testing.T) {
	cases := []struct {
		name      string
		message   string
		free      bool
		perCall   bool
		unlimited bool
	}{
		{name: "explicit response", message: `{"type":"response.create"}`},
		{name: "voice detection audio append", message: `{"type":"input_audio_buffer.append","audio":"AAA="}`},
		{name: "audio commit", message: `{"type":"input_audio_buffer.commit"}`},
		{name: "session can enable buffered generation", message: `{"type":"session.update","session":{"turn_detection":{"type":"server_vad","create_response":true}}}`},
		{name: "conversation input", message: `{"type":"conversation.item.create","item":{"type":"message","role":"user"}}`},
		{name: "unknown event is not a control bypass", message: `{"type":"future.generation.create"}`},
		{name: "free model", message: `{"type":"response.create"}`, free: true},
		{name: "per call model", message: `{"type":"response.create"}`, perCall: true},
		{name: "unlimited token", message: `{"type":"response.create"}`, unlimited: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			info := &relaycommon.RelayInfo{TokenUnlimited: tc.unlimited, UsePrice: tc.perCall}
			info.PriceData.FreeModel = tc.free
			info.PriceData.UsePrice = tc.perCall
			relay := startRealtimeDebtRelay(t, info)

			// A real websocket message passes while the durable wallet is solvent.
			require.NoError(t, relay.client.WriteMessage(websocket.TextMessage, []byte(tc.message)))
			assert.JSONEq(t, tc.message, string(readRealtimeDebtMessage(t, relay.upstream)))
			output := `{"type":"response.created"}`
			expectedTokens := 0
			if tc.perCall {
				output = `{"type":"response.done","response":{"usage":{"total_tokens":2,"input_tokens":2,"input_token_details":{"text_tokens":2}}}}`
				expectedTokens = 2
			}
			require.NoError(t, relay.upstream.WriteMessage(websocket.TextMessage, []byte(output)))
			assert.JSONEq(t, output, string(readRealtimeDebtMessage(t, relay.client)))

			require.NoError(t, relay.db.Model(&model.User{}).Where("id = ?", 1).Update("quota", -1).Error)
			const cancel = `{"type":"response.cancel"}`
			require.NoError(t, relay.client.WriteMessage(websocket.TextMessage, []byte(cancel)))
			assert.JSONEq(t, cancel, string(readRealtimeDebtMessage(t, relay.upstream)))

			require.NoError(t, relay.client.WriteMessage(websocket.TextMessage, []byte(tc.message)))
			var event dto.RealtimeEvent
			require.NoError(t, common.Unmarshal(readRealtimeDebtMessage(t, relay.client), &event))
			assert.Equal(t, dto.RealtimeEventTypeError, event.Type)
			require.NotNil(t, event.Error)
			assert.Equal(t, string(types.ErrorCodeInsufficientUserQuota), event.Error.Code)
			assert.NotEmpty(t, event.Error.Message)

			select {
			case result := <-relay.result:
				require.NotNil(t, result.err)
				assert.Equal(t, types.ErrorCodeInsufficientUserQuota, result.err.GetErrorCode())
				assert.Equal(t, http.StatusForbidden, result.err.StatusCode)
				assert.True(t, types.IsSkipRetryError(result.err))
				require.NotNil(t, result.usage)
				assert.Equal(t, expectedTokens, result.usage.TotalTokens)
			case <-time.After(5 * time.Second):
				t.Fatal("generation rejection did not return")
			}
			// Closing the relay endpoint is the deterministic barrier: upstream
			// sees EOF, not an additional generation frame, without a timing wait.
			require.NoError(t, relay.upstream.SetReadDeadline(time.Now().Add(5*time.Second)))
			_, message, err := relay.upstream.ReadMessage()
			require.Error(t, err)
			assert.Empty(t, message)
		})
	}
}

func TestOpenaiRealtimeDebtPreservesControlAndRepaidSubscription(t *testing.T) {
	relay := startRealtimeDebtRelay(t, &relaycommon.RelayInfo{BillingSource: "subscription"})
	require.NoError(t, relay.db.Model(&model.User{}).Where("id = ?", 1).Update("quota", -1).Error)
	for _, eventType := range []string{
		"response.cancel", "input_audio_buffer.clear", "output_audio_buffer.clear",
		"conversation.item.delete", "conversation.item.truncate", "conversation.item.retrieve",
	} {
		message, err := common.Marshal(map[string]string{"type": eventType})
		require.NoError(t, err)
		require.NoError(t, relay.client.WriteMessage(websocket.TextMessage, message))
		assert.JSONEq(t, string(message), string(readRealtimeDebtMessage(t, relay.upstream)))
	}
	// A debt-free zero wallet can resume a prepaid subscription on this socket.
	require.NoError(t, relay.db.Model(&model.User{}).Where("id = ?", 1).Update("quota", 0).Error)
	const generate = `{"type":"response.create"}`
	require.NoError(t, relay.client.WriteMessage(websocket.TextMessage, []byte(generate)))
	assert.JSONEq(t, generate, string(readRealtimeDebtMessage(t, relay.upstream)))
	require.NoError(t, relay.client.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")))
	select {
	case result := <-relay.result:
		assert.Nil(t, result.err)
		require.NotNil(t, result.usage)
	case <-time.After(5 * time.Second):
		t.Fatal("repaid realtime session did not close")
	}
}
