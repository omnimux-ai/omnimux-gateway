package controller

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	pluginruntime "github.com/QuantumNous/new-api/pkg/jsplugin"
	"github.com/QuantumNous/new-api/relay"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// The account setup uses the same native funding path as queued generation.
// A member's two provider groups share its discount, independently of the
// ordinary user's effective charge.
func configureNativeFailoverBilling(t *testing.T, f *nativeFailoverFixture, userGroup, funding string) *model.UserSubscription {
	t.Helper()
	previous := ratio_setting.GroupGroupRatio2JSONString()
	require.NoError(t, ratio_setting.UpdateGroupGroupRatioByJSONString(`{"member":{"default":0.6,"backup":0.6}}`))
	t.Cleanup(func() { require.NoError(t, ratio_setting.UpdateGroupGroupRatioByJSONString(previous)) })
	f.user.Group = userGroup
	f.user.Setting = fmt.Sprintf(`{"billing_preference":%q}`, funding+"_only")
	require.NoError(t, f.db.Model(&f.user).Select("group", "setting").Updates(&f.user).Error)
	f.user.ToBaseUser().WriteContext(f.c)
	f.info.UserGroup = userGroup
	f.info.UserSetting = f.user.GetSetting()
	f.info.RequestId = "native-failover-billing-request"
	if funding == "wallet" {
		return nil
	}
	require.NoError(t, f.db.AutoMigrate(&model.SubscriptionPreConsumeRecord{}))
	t.Cleanup(func() { require.NoError(t, f.db.Migrator().DropTable(&model.SubscriptionPreConsumeRecord{})) })
	plan := model.SubscriptionPlan{Title: "native failover subscription", TotalAmount: 100000, QuotaResetPeriod: model.SubscriptionResetNever}
	require.NoError(t, f.db.Create(&plan).Error)
	sub := &model.UserSubscription{UserId: f.user.Id, PlanId: plan.Id, AmountTotal: 100000,
		Status: "active", StartTime: common.GetTimestamp() - 1, EndTime: common.GetTimestamp() + 3600, AllowanceEpoch: 7}
	require.NoError(t, f.db.Create(sub).Error)
	return sub
}

type nativeFundingWrites struct {
	walletDebits, walletCredits                    atomic.Int32
	subscriptionReservations, subscriptionReleases atomic.Int32
}

func observeNativeFailoverFunding(t *testing.T, f *nativeFailoverFixture) *nativeFundingWrites {
	t.Helper()
	writes := &nativeFundingWrites{}
	require.NoError(t, f.db.Callback().Update().After("gorm:update").Register("observe_native_failover_funding", func(tx *gorm.DB) {
		if tx.Error != nil || tx.RowsAffected == 0 {
			return
		}
		if strings.HasSuffix(tx.Statement.Table, "users") {
			if values, ok := tx.Statement.Dest.(map[string]interface{}); ok {
				if expr, ok := values["quota"].(clause.Expr); ok {
					if strings.Contains(expr.SQL, "quota -") {
						writes.walletDebits.Add(1)
					} else if strings.Contains(expr.SQL, "quota +") {
						writes.walletCredits.Add(1)
					}
				}
			}
		}
		if strings.HasSuffix(tx.Statement.Table, "user_subscriptions") {
			switch value := tx.Statement.Dest.(type) {
			case *model.UserSubscription:
				if value.AmountUsed > 0 {
					writes.subscriptionReservations.Add(1)
				}
			case map[string]interface{}:
				if value["amount_used"] == int64(0) {
					writes.subscriptionReleases.Add(1)
				}
			}
		}
	}))
	return writes
}

func TestNativeFailoverPreservesOrdinaryAndMemberChargeForBothFundingSources(t *testing.T) {
	for _, group := range []string{"default", "member"} {
		for _, funding := range []string{"wallet", "subscription"} {
			for _, route := range []string{"primary", "backup"} {
				t.Run(group+"/"+funding+"/"+route, func(t *testing.T) {
					var calls atomic.Int32
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						calls.Add(1)
						w.Header().Set("Content-Type", "application/json")
						if strings.HasPrefix(r.URL.Path, "/api/image-tasks/") {
							_, _ = io.WriteString(w, `{"id":"primary-image","status":"queued"}`)
						} else {
							_, _ = io.WriteString(w, `{"data":[{"url":"https://example.test/complete.png"}]}`)
						}
					}))
					defer server.Close()
					primaryURL := server.URL
					if route == "backup" {
						primaryURL = ":invalid-before-transport"
					}
					f := newNativeFailoverFixture(t, primaryURL, server.URL, false)
					sub := configureNativeFailoverBilling(t, f, group, funding)
					writes := observeNativeFailoverFunding(t, f)
					outcome, taskErr := executeTaskSubmissionWith(f.c, f.info, relay.RelayTaskSubmit)
					require.Nil(t, taskErr)
					require.NotNil(t, outcome)
					wantQuota := 5000
					if group == "member" {
						wantQuota = 3000
					}
					assert.Equal(t, wantQuota, outcome.Task.Quota)
					assert.Equal(t, funding, outcome.Task.PrivateData.BillingSource)
					assert.Equal(t, int32(1), calls.Load())
					wantChannel := f.primary.Id
					if route == "backup" {
						wantChannel = f.backup.Id
					}
					assert.Equal(t, wantChannel, outcome.Task.ChannelId)
					var account model.User
					require.NoError(t, f.db.First(&account, f.user.Id).Error)
					assert.Equal(t, wantQuota, account.UsedQuota)
					assert.Equal(t, 1, account.RequestCount)
					if funding == "wallet" {
						assert.Equal(t, 100000-wantQuota, account.Quota)
						assert.Equal(t, int32(1), writes.walletDebits.Load())
						assert.Zero(t, writes.walletCredits.Load())
					} else {
						require.NoError(t, f.db.First(sub, sub.Id).Error)
						assert.Equal(t, int64(wantQuota), sub.AmountUsed)
						assert.Equal(t, int64(wantQuota), outcome.Task.PrivateData.SubscriptionAllowanceAllocated)
						assert.Equal(t, 100000, account.Quota)
						assert.Equal(t, int32(1), writes.subscriptionReservations.Load())
						assert.Zero(t, writes.subscriptionReleases.Load())
						var records []model.SubscriptionPreConsumeRecord
						require.NoError(t, f.db.Find(&records).Error)
						require.Len(t, records, 1)
						assert.Equal(t, int64(wantQuota), records[0].PreConsumed)
					}
				})
			}
		}
	}
}

func TestNativeFailoverAllPreSendFailuresReleaseFundingExactlyOnce(t *testing.T) {
	for _, group := range []string{"default", "member"} {
		for _, funding := range []string{"wallet", "subscription"} {
			t.Run(group+"/"+funding, func(t *testing.T) {
				f := newNativeFailoverFixture(t, ":invalid-primary", ":invalid-backup", false)
				sub := configureNativeFailoverBilling(t, f, group, funding)
				writes := observeNativeFailoverFunding(t, f)
				_, taskErr := executeTaskSubmissionWith(f.c, f.info, relay.RelayTaskSubmit)
				require.NotNil(t, taskErr)
				assert.True(t, taskErr.NoTaskCreated)
				var stored model.Task
				require.NoError(t, f.db.First(&stored, f.task.ID).Error)
				assert.Equal(t, model.TaskStatus(model.TaskStatusFailure), stored.Status)
				assert.Zero(t, stored.Quota)
				assert.Zero(t, stored.PrivateData.SubscriptionAllowanceAllocated)
				// A repeated settlement and task-release reconciliation must remain
				// harmless after the original native session returned its funds.
				require.NoError(t, service.SettleBilling(f.c, f.info, 0))
				require.True(t, service.ReleaseUnloggedTaskReservation(f.c.Request.Context(), &stored, "reloaded terminal task"))
				var account model.User
				require.NoError(t, f.db.First(&account, f.user.Id).Error)
				assert.Equal(t, 100000, account.Quota)
				assert.Zero(t, account.UsedQuota)
				assert.Zero(t, account.RequestCount)
				if funding == "wallet" {
					assert.Equal(t, int32(1), writes.walletDebits.Load())
					assert.Equal(t, int32(1), writes.walletCredits.Load())
				} else {
					require.NoError(t, f.db.First(sub, sub.Id).Error)
					assert.Zero(t, sub.AmountUsed)
					assert.Equal(t, int32(1), writes.subscriptionReservations.Load())
					assert.Equal(t, int32(1), writes.subscriptionReleases.Load())
				}
			})
		}
	}
}

func TestNativeFailoverBillingBasisChangeStopsBackupBeforeSending(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1) }))
	defer server.Close()
	f := newNativeFailoverFixture(t, ":invalid-primary", server.URL, false)
	configureNativeFailoverBilling(t, f, "default", "wallet")
	_, taskErr := executeTaskSubmissionWith(f.c, f.info, func(c *gin.Context, info *relaycommon.RelayInfo) (*relay.TaskSubmitResult, *dto.TaskError) {
		result, err := relay.RelayTaskSubmit(c, info)
		if err != nil && err.NoTaskCreated {
			// Preserve the rounded 5000 charge while changing the billing
			// basis from one request to its validated image quantity.
			withTieredBillingConfig(t, map[string]string{"gpt-image-2.5": "tiered_expr"}, map[string]string{"gpt-image-2.5": `tier("image", fixed(0.01)) * image_count`})
		}
		return result, err
	})
	require.NotNil(t, taskErr)
	assert.Equal(t, "generation_quote_changed", taskErr.Code)
	assert.Equal(t, 5000, f.info.PriceData.Quota)
	assert.Zero(t, calls.Load())
	var account model.User
	require.NoError(t, f.db.First(&account, f.user.Id).Error)
	assert.Equal(t, 100000, account.Quota)
}

func TestNativeFailoverLegacyTaskWithoutRoutingDoesNotResubmit(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1) }))
	defer server.Close()
	f := newNativeFailoverFixture(t, ":invalid-primary", server.URL, false)
	f.task.PrivateData.GenerationQueue.Routing = nil
	require.NoError(t, f.db.Model(f.task).Select("private_data").Updates(f.task).Error)
	_, taskErr := executeTaskSubmissionWith(f.c, f.info, relay.RelayTaskSubmit)
	require.NotNil(t, taskErr)
	assert.Zero(t, calls.Load())
	assert.Equal(t, []string{fmt.Sprint(f.primary.Id)}, f.c.GetStringSlice("use_channel"))
	var stored model.Task
	require.NoError(t, f.db.First(&stored, f.task.ID).Error)
	assert.Nil(t, stored.PrivateData.GenerationQueue.Routing)
	var account model.User
	require.NoError(t, f.db.First(&account, f.user.Id).Error)
	assert.Equal(t, 95000, account.Quota, "legacy tasks retain their original reconciliation and funding semantics")
	assert.Equal(t, model.TaskStatusDispatching, stored.Status)
}

func TestNativeFailoverKeepsOneReservationBeforeMixedCompletionSettlement(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"metered-backup-video"}`)
	}))
	defer server.Close()
	f := newNativeFailoverFixture(t, "https://not-sent.invalid", server.URL, true)
	sub := configureNativeFailoverBilling(t, f, "default", "subscription")
	require.NoError(t, f.db.Model(sub).Update("amount_total", 6000).Error)
	// Mixed funding occurs natively only when completed usage exceeds the
	// original subscription reservation. Submission itself admits one source.
	const source = `export const meta={apiVersion:1,key:"billing-failover-video",name:"Billing failover video",version:"1.0.0",author:{name:"Test"},models:["failover-video"],fetchMode:"per_task",usageSchema:{seconds:{type:"number",unit:"second",description:{en:"Video generation unit price",zh:"视频生成单价"}}}};
export function buildSubmitRequest(ctx){return {url:ctx.baseUrl+"/submit",body:ctx.baseUrl.includes("not-sent.invalid")?{file:{__fileRef:"missing",encoding:"base64"}}:ctx.requestBody};}
export function parseSubmitResponse(ctx,resp){return {taskId:resp.body.id,taskData:resp.body};}
export function extractUsage(ctx){return {seconds:ctx.requestBody.duration};}
export function buildQueryRequest(){return {};}
export function parseTaskResult(){return {status:"SUCCESS"};}`
	_, err := pluginruntime.DefaultRegistry.Register(source, pluginruntime.Options{})
	require.NoError(t, err)
	t.Cleanup(func() { pluginruntime.DefaultRegistry.Unregister("billing-failover-video") })
	providerSetting := `{"task_plugin_key":"billing-failover-video"}`
	f.primary.Setting, f.backup.Setting = &providerSetting, &providerSetting
	require.NoError(t, f.db.Model(&model.Channel{}).Where("id IN ?", []int{f.primary.Id, f.backup.Id}).Update("setting", providerSetting).Error)
	require.Nil(t, middleware.SetupContextForSelectedChannel(f.c, &f.primary, "failover-video"))
	withTieredBillingConfig(t, map[string]string{"failover-video": "tiered_expr"}, map[string]string{"failover-video": `u("seconds") * 0.002`})
	writes := observeNativeFailoverFunding(t, f)
	outcome, taskErr := executeTaskSubmissionWith(f.c, f.info, relay.RelayTaskSubmit)
	require.Nil(t, taskErr)
	require.NotNil(t, outcome)
	assert.Equal(t, int32(1), calls.Load())
	assert.Equal(t, f.backup.Id, outcome.Task.ChannelId)
	assert.Equal(t, 5000, outcome.Task.Quota)
	assert.Equal(t, int32(1), writes.subscriptionReservations.Load())
	assert.Zero(t, writes.walletDebits.Load(), "switching providers cannot reserve a second funding source")

	stale := *outcome.Task
	actual, _, err := service.EvaluateTaskCompletionUsage(outcome.Task.PrivateData.BillingContext.TieredSnapshot, map[string]any{"seconds": float64(8)})
	require.NoError(t, err)
	assert.Equal(t, 8000, actual.ActualQuotaAfterGroup)
	service.RecalculateTaskQuota(f.c.Request.Context(), outcome.Task, actual.ActualQuotaAfterGroup, common.NewMessage("native metered completion"))
	require.NoError(t, f.db.First(sub, sub.Id).Error)
	var account model.User
	require.NoError(t, f.db.First(&account, f.user.Id).Error)
	assert.EqualValues(t, 6000, sub.AmountUsed)
	assert.Equal(t, 98000, account.Quota)
	assert.EqualValues(t, 2000, outcome.Task.PrivateData.SubscriptionWalletConsumed)
	assert.EqualValues(t, 6000, outcome.Task.PrivateData.SubscriptionAllowanceAllocated)
	service.RecalculateTaskQuota(f.c.Request.Context(), &stale, actual.ActualQuotaAfterGroup, common.NewMessage("replayed native completion"))
	require.NoError(t, f.db.First(&account, f.user.Id).Error)
	assert.Equal(t, 98000, account.Quota)
	assert.Equal(t, int32(1), writes.walletDebits.Load(), "stale completion does not debit the mixed wallet allocation twice")

	require.True(t, service.RefundTaskQuota(f.c.Request.Context(), outcome.Task, "native mixed funding recovery"))
	require.True(t, service.RefundTaskQuota(f.c.Request.Context(), &stale, "reloaded native mixed funding recovery"))
	require.NoError(t, f.db.First(sub, sub.Id).Error)
	require.NoError(t, f.db.First(&account, f.user.Id).Error)
	assert.Zero(t, sub.AmountUsed)
	assert.Equal(t, 100000, account.Quota)
	assert.Zero(t, account.UsedQuota)
	assert.Equal(t, 1, account.RequestCount)
	assert.Equal(t, int32(1), writes.subscriptionReleases.Load())
	assert.Equal(t, int32(1), calls.Load(), "funding recovery never submits another provider request")
}
