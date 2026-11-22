package controller

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Calcium-Ion/go-epay/epay"
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stripe/stripe-go/v81"
	"github.com/stripe/stripe-go/v81/webhook"
	waffo "github.com/waffo-com/waffo-go"
	"github.com/waffo-com/waffo-go/core"
	"github.com/waffo-com/waffo-go/types/order"
	"github.com/waffo-com/waffo-go/utils"
	pancake "github.com/waffo-com/waffo-pancake-sdk-go"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func confirmPaymentComplianceForTest(t *testing.T) {
	t.Helper()
	paymentSetting := operation_setting.GetPaymentSetting()
	originalConfirmed := paymentSetting.ComplianceConfirmed
	originalTermsVersion := paymentSetting.ComplianceTermsVersion
	t.Cleanup(func() {
		paymentSetting.ComplianceConfirmed = originalConfirmed
		paymentSetting.ComplianceTermsVersion = originalTermsVersion
	})
	paymentSetting.ComplianceConfirmed = true
	paymentSetting.ComplianceTermsVersion = operation_setting.CurrentComplianceTermsVersion
}

func TestStripeWebhookEnabledRequiresTopUpAndWebhookConfig(t *testing.T) {
	confirmPaymentComplianceForTest(t)
	originalAPISecret := setting.StripeApiSecret
	originalWebhookSecret := setting.StripeWebhookSecret
	originalPriceID := setting.StripePriceId
	t.Cleanup(func() {
		setting.StripeApiSecret = originalAPISecret
		setting.StripeWebhookSecret = originalWebhookSecret
		setting.StripePriceId = originalPriceID
	})

	setting.StripeWebhookSecret = ""
	setting.StripeApiSecret = "sk_test_123"
	setting.StripePriceId = "price_123"
	require.False(t, isStripeWebhookEnabled())

	setting.StripeWebhookSecret = "whsec_test"
	require.True(t, isStripeWebhookEnabled())

	setting.StripePriceId = ""
	require.False(t, isStripeWebhookEnabled())
}

func TestCreemWebhookEnabledRequiresTopUpAndWebhookConfig(t *testing.T) {
	confirmPaymentComplianceForTest(t)
	originalAPIKey := setting.CreemApiKey
	originalProducts := setting.CreemProducts
	originalWebhookSecret := setting.CreemWebhookSecret
	t.Cleanup(func() {
		setting.CreemApiKey = originalAPIKey
		setting.CreemProducts = originalProducts
		setting.CreemWebhookSecret = originalWebhookSecret
	})

	setting.CreemWebhookSecret = ""
	setting.CreemApiKey = "creem_api_key"
	setting.CreemProducts = `[{"productId":"prod_123"}]`
	require.False(t, isCreemWebhookEnabled())

	setting.CreemWebhookSecret = "creem_secret"
	require.True(t, isCreemWebhookEnabled())

	setting.CreemProducts = "[]"
	require.False(t, isCreemWebhookEnabled())
}

func TestWaffoWebhookEnabledRequiresTopUpAndWebhookConfig(t *testing.T) {
	confirmPaymentComplianceForTest(t)
	originalEnabled := setting.WaffoEnabled
	originalSandbox := setting.WaffoSandbox
	originalAPIKey := setting.WaffoApiKey
	originalPrivateKey := setting.WaffoPrivateKey
	originalPublicCert := setting.WaffoPublicCert
	originalSandboxAPIKey := setting.WaffoSandboxApiKey
	originalSandboxPrivateKey := setting.WaffoSandboxPrivateKey
	originalSandboxPublicCert := setting.WaffoSandboxPublicCert
	t.Cleanup(func() {
		setting.WaffoEnabled = originalEnabled
		setting.WaffoSandbox = originalSandbox
		setting.WaffoApiKey = originalAPIKey
		setting.WaffoPrivateKey = originalPrivateKey
		setting.WaffoPublicCert = originalPublicCert
		setting.WaffoSandboxApiKey = originalSandboxAPIKey
		setting.WaffoSandboxPrivateKey = originalSandboxPrivateKey
		setting.WaffoSandboxPublicCert = originalSandboxPublicCert
	})

	setting.WaffoEnabled = true
	setting.WaffoSandbox = false
	setting.WaffoApiKey = ""
	setting.WaffoPrivateKey = "private"
	setting.WaffoPublicCert = "public"
	require.False(t, isWaffoWebhookEnabled())

	setting.WaffoApiKey = "api"
	require.True(t, isWaffoWebhookEnabled())

	setting.WaffoEnabled = false
	require.False(t, isWaffoWebhookEnabled())

	setting.WaffoEnabled = true
	setting.WaffoSandbox = true
	setting.WaffoSandboxApiKey = ""
	setting.WaffoSandboxPrivateKey = "sandbox_private"
	setting.WaffoSandboxPublicCert = "sandbox_public"
	require.False(t, isWaffoWebhookEnabled())

	setting.WaffoSandboxApiKey = "sandbox_api"
	require.True(t, isWaffoWebhookEnabled())
}

func TestWaffoPancakeWebhookEnabledRequiresTopUpAndWebhookConfig(t *testing.T) {
	confirmPaymentComplianceForTest(t)
	originalMerchantID := setting.WaffoPancakeMerchantID
	originalPrivateKey := setting.WaffoPancakePrivateKey
	originalProductID := setting.WaffoPancakeProductID
	t.Cleanup(func() {
		setting.WaffoPancakeMerchantID = originalMerchantID
		setting.WaffoPancakePrivateKey = originalPrivateKey
		setting.WaffoPancakeProductID = originalProductID
	})

	// Presence of all three credentials enables the gateway. Webhook public
	// keys are bundled in the SDK and there is no separate Enabled toggle —
	// clear any of the three fields to disable.
	setting.WaffoPancakeMerchantID = ""
	setting.WaffoPancakePrivateKey = "private"
	setting.WaffoPancakeProductID = "product"
	require.False(t, isWaffoPancakeWebhookEnabled())

	setting.WaffoPancakeMerchantID = "merchant"
	require.True(t, isWaffoPancakeWebhookEnabled())

	setting.WaffoPancakeProductID = ""
	require.False(t, isWaffoPancakeWebhookEnabled())

	setting.WaffoPancakeProductID = "product"
	setting.WaffoPancakePrivateKey = ""
	require.False(t, isWaffoPancakeWebhookEnabled())
}

func TestEpayWebhookEnabledRequiresTopUpAndWebhookConfig(t *testing.T) {
	confirmPaymentComplianceForTest(t)
	originalPayAddress := operation_setting.PayAddress
	originalEpayID := operation_setting.EpayId
	originalEpayKey := operation_setting.EpayKey
	originalPayMethods := operation_setting.PayMethods
	t.Cleanup(func() {
		operation_setting.PayAddress = originalPayAddress
		operation_setting.EpayId = originalEpayID
		operation_setting.EpayKey = originalEpayKey
		operation_setting.PayMethods = originalPayMethods
	})

	operation_setting.PayAddress = "https://pay.example.com"
	operation_setting.EpayId = "epay_id"
	operation_setting.EpayKey = ""
	operation_setting.PayMethods = []map[string]string{{"type": "alipay"}}
	require.False(t, isEpayWebhookEnabled())

	operation_setting.EpayKey = "epay_key"
	require.True(t, isEpayWebhookEnabled())

	operation_setting.PayMethods = nil
	require.False(t, isEpayWebhookEnabled())
}

func TestEpayCallbacksVerifyBeforeSettlement(t *testing.T) {
	confirmPaymentComplianceForTest(t)
	originalPayAddress := operation_setting.PayAddress
	originalEpayID := operation_setting.EpayId
	originalEpayKey := operation_setting.EpayKey
	originalPayMethods := operation_setting.PayMethods
	originalServerAddress := system_setting.ServerAddress
	originalDB := model.DB
	t.Cleanup(func() {
		operation_setting.PayAddress = originalPayAddress
		operation_setting.EpayId = originalEpayID
		operation_setting.EpayKey = originalEpayKey
		operation_setting.PayMethods = originalPayMethods
		system_setting.ServerAddress = originalServerAddress
		model.DB = originalDB
	})
	operation_setting.PayAddress = "https://pay.example.com"
	operation_setting.EpayId = "epay_id"
	operation_setting.EpayKey = "test-key"
	operation_setting.PayMethods = []map[string]string{{"type": "alipay"}}
	system_setting.ServerAddress = "https://dashboard.example.com"
	// Rejected callbacks and valid non-success events must never reach settlement.
	// A nil database makes an accidental attempt to settle fail the test.
	model.DB = nil
	require.True(t, isEpayWebhookEnabled())

	// Both ambiguous maps below had this same valid signing string in v0.0.4.
	// This must fail because of parameter ambiguity, not a mismatched signature.
	ambiguousSign := epay.MD5String("money=1.00&name=item&out_trade_no=other&out_trade_no=order&pid=epay_id&trade_status=TRADE_SUCCESS&type=alipay", "test-key")
	pendingSign := epay.MD5String("money=1.00&name=item%26other&out_trade_no=order&pid=epay_id&trade_status=WAIT_BUYER_PAY&type=alipay", "test-key")
	cases := []struct {
		name         string
		productName  string
		tradeNo      string
		status       string
		sign         string
		validPending bool
	}{
		{"separator in product name", "item&out_trade_no=other", "order", epay.StatusTradeSuccess, ambiguousSign, false},
		{"separator in order number", "item", "other&out_trade_no=order", epay.StatusTradeSuccess, ambiguousSign, false},
		{"invalid signature", "item", "order", epay.StatusTradeSuccess, "invalid", false},
		{"missing signature", "item", "order", epay.StatusTradeSuccess, "", false},
		{"valid pending with literal percent escape", "item%26other", "order", "WAIT_BUYER_PAY", pendingSign, true},
	}
	endpoints := []struct {
		name             string
		path             string
		handler          gin.HandlerFunc
		status           int
		rejectedBody     string
		pendingBody      string
		rejectedLocation string
		pendingLocation  string
	}{
		{
			name: "topup notify", path: "/api/user/epay/notify", handler: EpayNotify,
			status: http.StatusOK, rejectedBody: "fail", pendingBody: "success",
		},
		{
			name: "subscription notify", path: "/api/subscription/epay/notify", handler: SubscriptionEpayNotify,
			status: http.StatusOK, rejectedBody: "fail", pendingBody: "fail",
		},
		{
			name: "subscription return", path: "/api/subscription/epay/return", handler: SubscriptionEpayReturn,
			status:           http.StatusFound,
			rejectedLocation: "https://dashboard.example.com/wallet?pay=fail",
			pendingLocation:  "https://dashboard.example.com/wallet?pay=pending",
		},
	}
	router := gin.New()
	for _, endpoint := range endpoints {
		router.Any(endpoint.path, endpoint.handler)
	}
	for _, endpoint := range endpoints {
		for _, method := range []string{http.MethodGet, http.MethodPost} {
			for _, tc := range cases {
				t.Run(endpoint.name+"/"+method+"/"+tc.name, func(t *testing.T) {
					params := url.Values{
						"pid": {"epay_id"}, "type": {"alipay"}, "money": {"1.00"},
						"name": {tc.productName}, "out_trade_no": {tc.tradeNo},
						"trade_status": {tc.status}, "sign_type": {"MD5"},
					}
					if tc.sign != "" {
						params.Set("sign", tc.sign)
					}
					request := httptest.NewRequest(method, endpoint.path+"?"+params.Encode(), nil)
					if method == http.MethodPost {
						request = httptest.NewRequest(method, endpoint.path, strings.NewReader(params.Encode()))
						request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
					}
					recorder := httptest.NewRecorder()
					require.NotPanics(t, func() { router.ServeHTTP(recorder, request) })
					assert.Equal(t, endpoint.status, recorder.Code)
					wantBody, wantLocation := endpoint.rejectedBody, endpoint.rejectedLocation
					if tc.validPending {
						wantBody, wantLocation = endpoint.pendingBody, endpoint.pendingLocation
					}
					if wantBody != "" {
						assert.Equal(t, wantBody, recorder.Body.String())
					}
					assert.Equal(t, wantLocation, recorder.Header().Get("Location"))
				})
			}
		}
	}
}

// SystemTask's existing finish path uses the real table names. These tests
// require an empty dedicated database for either external dialect.
func paymentOrderTestDatabase(t *testing.T) *gorm.DB {
	t.Helper()
	dialect := common.DatabaseType(os.Getenv("TEST_PAYMENT_ORDER_DB_DIALECT"))
	var driver gorm.Dialector
	switch dialect {
	case "", common.DatabaseTypeSQLite:
		dialect = common.DatabaseTypeSQLite
		driver = sqlite.Open(filepath.Join(t.TempDir(), "payments.sqlite") + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)")
	case common.DatabaseTypeMySQL:
		dsn := os.Getenv("TEST_PAYMENT_ORDER_MYSQL_DSN")
		require.NotEmpty(t, dsn)
		driver = mysql.Open(dsn)
	case common.DatabaseTypePostgreSQL:
		dsn := os.Getenv("TEST_PAYMENT_ORDER_POSTGRES_DSN")
		require.NotEmpty(t, dsn)
		driver = postgres.New(postgres.Config{DSN: dsn, PreferSimpleProtocol: true})
	default:
		t.Fatalf("invalid payment order test dialect %q", dialect)
	}
	db, err := gorm.Open(driver, &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(4)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	var version string
	query := "SELECT version()"
	if dialect == common.DatabaseTypeSQLite {
		query = "SELECT sqlite_version()"
	}
	require.NoError(t, db.Raw(query).Scan(&version).Error)
	t.Logf("payment test database: %s %s", dialect, version)
	tables := []any{&model.User{}, &model.TopUp{}, &model.SubscriptionPlan{}, &model.SubscriptionOrder{}, &model.UserSubscription{}, &model.Log{}, &model.SystemTask{}, &model.SystemTaskLock{}}
	for _, table := range tables {
		require.False(t, db.Migrator().HasTable(table), "payment tests require a dedicated empty database")
	}
	t.Cleanup(func() { require.NoError(t, db.Migrator().DropTable(tables...)) })
	require.NoError(t, db.AutoMigrate(tables...))

	oldDB, oldLogDB := model.DB, model.LOG_DB
	oldMain, oldLog := common.MainDatabaseType(), common.LogDatabaseType()
	oldRedis, oldCache, oldMaster := common.RedisEnabled, common.MemoryCacheEnabled, common.IsMasterNode
	oldTransport := http.DefaultTransport
	t.Cleanup(func() {
		model.DB, model.LOG_DB = oldDB, oldLogDB
		common.SetDatabaseTypes(oldMain, oldLog)
		common.RedisEnabled, common.MemoryCacheEnabled, common.IsMasterNode = oldRedis, oldCache, oldMaster
		http.DefaultTransport = oldTransport
	})
	model.DB = db
	common.SetDatabaseTypes(dialect, dialect)
	common.RedisEnabled, common.MemoryCacheEnabled, common.IsMasterNode = false, false, false
	t.Setenv("LOG_SQL_DSN", "")
	require.NoError(t, model.InitLogDB())
	confirmPaymentComplianceForTest(t)
	http.DefaultTransport = paymentTestRoundTripper(func(req *http.Request) (*http.Response, error) {
		t.Errorf("unexpected outbound request: %s %s", req.Method, req.URL.Path)
		return nil, fmt.Errorf("no fixture for this request")
	})
	return db
}

type paymentTestRoundTripper func(*http.Request) (*http.Response, error)

func (f paymentTestRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func paymentTestResponse(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
}

func postPaymentTestEvent(handler gin.HandlerFunc, body, signatureHeader, signature string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set(signatureHeader, signature)
	handler(c)
	return recorder
}

func paymentTestBuyer(t *testing.T, db *gorm.DB) (model.User, model.SubscriptionPlan) {
	t.Helper()
	user := model.User{Username: "payment-fixture", Email: "buyer@example.test", AffCode: "payment-aff"}
	plan := model.SubscriptionPlan{Title: "Payment fixture", PriceAmount: 1, Enabled: true, DurationUnit: model.SubscriptionDurationMonth, DurationValue: 1, CreemProductId: "prod_fixture"}
	require.NoError(t, db.Create(&user).Error)
	require.NoError(t, db.Create(&plan).Error)
	model.InvalidateSubscriptionPlanCache(plan.Id)
	t.Cleanup(func() { model.InvalidateSubscriptionPlanCache(plan.Id) })
	return user, plan
}

func paymentTestState(t *testing.T, subscription bool, tradeNo string) (string, int64) {
	t.Helper()
	if subscription {
		order := model.GetSubscriptionOrderByTradeNo(tradeNo)
		require.NotNil(t, order)
		return order.Status, order.CompleteTime
	}
	order := model.GetTopUpByTradeNo(tradeNo)
	require.NotNil(t, order)
	return order.Status, order.CompleteTime
}

func TestPaymentOrderCreemCheckoutRejection(t *testing.T) {
	db := paymentOrderTestDatabase(t)
	user, plan := paymentTestBuyer(t, db)
	apiKey, products, secret, testMode := setting.CreemApiKey, setting.CreemProducts, setting.CreemWebhookSecret, setting.CreemTestMode
	t.Cleanup(func() {
		setting.CreemApiKey, setting.CreemProducts, setting.CreemWebhookSecret, setting.CreemTestMode = apiKey, products, secret, testMode
	})
	setting.CreemApiKey, setting.CreemWebhookSecret, setting.CreemTestMode = "fixture-api-key", "fixture-signing-key", false
	setting.CreemProducts = `[{"productId":"prod_fixture","name":"Fixture","price":1,"currency":"USD","quota":500000}]`

	for _, purchase := range []struct {
		name         string
		subscription bool
		handler      gin.HandlerFunc
		body         string
	}{
		{"top-up", false, RequestCreemPay, `{"product_id":"prod_fixture","payment_method":"creem"}`},
		{"subscription", true, SubscriptionRequestCreemPay, fmt.Sprintf(`{"plan_id":%d}`, plan.Id)},
	} {
		for _, tc := range []struct {
			name     string
			code     int
			body     string
			paid     bool
			expected string
		}{
			{"invalid product", http.StatusBadRequest, `{"message":"invalid product"}`, false, common.TopUpStatusFailed},
			{"invalid credentials", http.StatusUnauthorized, `{"message":"unauthorized"}`, false, common.TopUpStatusFailed},
			{"request timeout", http.StatusRequestTimeout, "", false, common.TopUpStatusPending},
			{"conflict", http.StatusConflict, "", false, common.TopUpStatusPending},
			{"network timeout", 0, "", false, common.TopUpStatusPending},
			{"upstream outage", http.StatusBadGateway, "", false, common.TopUpStatusPending},
			{"unparseable success", http.StatusOK, `{"id":`, false, common.TopUpStatusPending},
			{"missing checkout link", http.StatusOK, `{"id":"checkout_fixture"}`, false, common.TopUpStatusPending},
			{"already paid during checkout", http.StatusBadRequest, `{}`, true, common.TopUpStatusSuccess},
		} {
			t.Run(purchase.name+"/"+tc.name, func(t *testing.T) {
				var reference string
				var paidAt int64
				var outbound int
				http.DefaultTransport = paymentTestRoundTripper(func(req *http.Request) (*http.Response, error) {
					outbound++
					require.Equal(t, http.MethodPost, req.Method)
					require.Equal(t, "/v1/checkouts", req.URL.Path)
					var checkout CreemCheckoutRequest
					require.NoError(t, common.DecodeJson(req.Body, &checkout))
					reference = checkout.RequestId
					require.NotEmpty(t, reference)
					status, _ := paymentTestState(t, purchase.subscription, reference)
					require.Equal(t, common.TopUpStatusPending, status)
					if tc.paid {
						body := fmt.Sprintf(`{"id":"paid_fixture","eventType":"checkout.completed","object":{"request_id":%q,"order":{"id":"o_fixture","status":"paid","type":"onetime","currency":"USD","amount_paid":100}}}`, reference)
						recorder := postPaymentTestEvent(CreemWebhook, body, CreemSignatureHeader, generateCreemSignature(body, setting.CreemWebhookSecret))
						require.Equal(t, http.StatusOK, recorder.Code)
						status, paidAt = paymentTestState(t, purchase.subscription, reference)
						require.Equal(t, common.TopUpStatusSuccess, status)
					}
					if tc.code == 0 {
						return nil, context.DeadlineExceeded
					}
					return paymentTestResponse(tc.code, tc.body), nil
				})
				recorder := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(recorder)
				c.Request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(purchase.body))
				c.Request.Header.Set("Content-Type", "application/json")
				c.Set("id", user.Id)
				purchase.handler(c)
				assert.Equal(t, 1, outbound)
				assert.Equal(t, http.StatusOK, recorder.Code)
				assert.Contains(t, recorder.Body.String(), "Failed to start payment")
				status, completed := paymentTestState(t, purchase.subscription, reference)
				assert.Equal(t, tc.expected, status)
				if tc.paid {
					assert.Positive(t, paidAt)
					assert.Equal(t, paidAt, completed)
				}
			})
		}
	}
	var updated model.User
	require.NoError(t, db.First(&updated, user.Id).Error)
	assert.Equal(t, 500000, updated.Quota)
	var subscriptions int64
	require.NoError(t, db.Model(&model.UserSubscription{}).Where("user_id = ?", user.Id).Count(&subscriptions).Error)
	assert.EqualValues(t, 1, subscriptions)
}

func configureWaffoPaymentTest(t *testing.T) {
	t.Helper()
	key, err := waffo.GenerateKeyPair()
	require.NoError(t, err)
	enabled, sandbox, apiKey := setting.WaffoEnabled, setting.WaffoSandbox, setting.WaffoApiKey
	privateKey, publicKey, merchant := setting.WaffoPrivateKey, setting.WaffoPublicCert, setting.WaffoMerchantId
	t.Cleanup(func() {
		setting.WaffoEnabled, setting.WaffoSandbox, setting.WaffoApiKey = enabled, sandbox, apiKey
		setting.WaffoPrivateKey, setting.WaffoPublicCert, setting.WaffoMerchantId = privateKey, publicKey, merchant
	})
	setting.WaffoEnabled, setting.WaffoSandbox, setting.WaffoApiKey = true, false, "fixture-api"
	setting.WaffoPrivateKey, setting.WaffoPublicCert, setting.WaffoMerchantId = key.PrivateKey, key.PublicKey, "fixture-merchant"
}

func TestPaymentOrderWaffoWebhookTerminalStates(t *testing.T) {
	db := paymentOrderTestDatabase(t)
	configureWaffoPaymentTest(t)
	user, _ := paymentTestBuyer(t, db)
	for _, tc := range []struct {
		name, notification, provider, initial, want string
	}{
		{"processing", core.OrderStatusPayInProgress, model.PaymentProviderWaffo, common.TopUpStatusPending, common.TopUpStatusPending},
		{"auth required", core.OrderStatusAuthorizationRequired, model.PaymentProviderWaffo, common.TopUpStatusPending, common.TopUpStatusPending},
		{"wait capture", core.OrderStatusAuthedWaitingCapture, model.PaymentProviderWaffo, common.TopUpStatusPending, common.TopUpStatusPending},
		{"unknown", "fixture-unknown-state", model.PaymentProviderWaffo, common.TopUpStatusPending, common.TopUpStatusPending},
		{"closed", core.OrderStatusOrderClose, model.PaymentProviderWaffo, common.TopUpStatusPending, common.TopUpStatusFailed},
		{"closed after success", core.OrderStatusOrderClose, model.PaymentProviderWaffo, common.TopUpStatusSuccess, common.TopUpStatusSuccess},
		{"closed wrong provider", core.OrderStatusOrderClose, model.PaymentProviderCreem, common.TopUpStatusPending, common.TopUpStatusPending},
		{"paid", core.OrderStatusPaySuccess, model.PaymentProviderWaffo, common.TopUpStatusPending, common.TopUpStatusSuccess},
	} {
		t.Run(tc.name, func(t *testing.T) {
			row := model.TopUp{UserId: user.Id, Amount: 1, Money: 1, TradeNo: tc.name, Status: tc.initial, PaymentMethod: model.PaymentMethodWaffo, PaymentProvider: tc.provider}
			require.NoError(t, row.Insert())
			body, err := common.Marshal(core.PaymentNotification{EventType: core.EventPayment, Result: &core.PaymentNotificationResult{MerchantOrderID: row.TradeNo, OrderStatus: tc.notification, OrderCurrency: "USD", OrderAmount: "1.00"}})
			require.NoError(t, err)
			sig, err := utils.Sign(string(body), setting.WaffoPrivateKey)
			require.NoError(t, err)
			recorder := postPaymentTestEvent(WaffoWebhook, string(body), core.HeaderSignature, sig)
			assert.Equal(t, http.StatusOK, recorder.Code)
			assert.JSONEq(t, `{"message":"success"}`, recorder.Body.String())
			status, _ := paymentTestState(t, false, row.TradeNo)
			assert.Equal(t, tc.want, status)
		})
	}
	var updated model.User
	require.NoError(t, db.First(&updated, user.Id).Error)
	assert.EqualValues(t, common.QuotaPerUnit, updated.Quota)
}

func configurePancakePaymentTest(t *testing.T) {
	t.Helper()
	key, err := waffo.GenerateKeyPair()
	require.NoError(t, err)
	merchant, store, privateKey := setting.WaffoPancakeMerchantID, setting.WaffoPancakeStoreID, setting.WaffoPancakePrivateKey
	t.Cleanup(func() {
		setting.WaffoPancakeMerchantID, setting.WaffoPancakeStoreID, setting.WaffoPancakePrivateKey = merchant, store, privateKey
	})
	setting.WaffoPancakeMerchantID = "MER_AbCdEfGhIjKlMnOpQrStUv"
	setting.WaffoPancakeStoreID = "STO_fixture"
	setting.WaffoPancakePrivateKey = key.PrivateKey
}

func pancakeOrderInquiryFixture(reference string) (string, string) {
	payments := fmt.Sprintf(`{"data":{"paymentsCount":2,"payments":[{"id":"pay_a","orderId":"ORD_fixture","orderMerchantExternalId":%q,"status":"failed"},{"id":"pay_b","orderId":"ORD_fixture","orderMerchantExternalId":%q,"status":"canceled"}]}}`, reference, reference)
	order := fmt.Sprintf(`{"data":{"onetimeOrder":{"id":"ORD_fixture","orderMerchantExternalId":%q,"status":"canceled","testMode":false,"store":{"id":"STO_fixture"},"payments":[{"id":"pay_a","status":"failed","refundStatus":null},{"id":"pay_b","status":"canceled","refundStatus":null}]}}}`, reference)
	return payments, order
}

func decodePancakeInquiry(t *testing.T, req *http.Request) pancake.GraphQLParams {
	t.Helper()
	require.Equal(t, http.MethodPost, req.Method)
	require.Equal(t, "/v1/graphql", req.URL.Path)
	assert.Equal(t, "MER_AbCdEfGhIjKlMnOpQrStUv", req.Header.Get("X-Merchant-Id"))
	assert.NotEmpty(t, req.Header.Get("X-Signature"))
	assert.Empty(t, req.Header.Get("X-Idempotency-Key"))
	var query pancake.GraphQLParams
	require.NoError(t, common.DecodeJson(req.Body, &query))
	assert.True(t, strings.HasPrefix(strings.TrimSpace(query.Query), "query"))
	return query
}

func TestPaymentOrderPancakeInquiry(t *testing.T) {
	configurePancakePaymentTest(t)
	priorTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = priorTransport })
	const reference = "WAFFO_PANCAKE-7-fixture"
	payments, detail := pancakeOrderInquiryFixture(reference)
	for _, tc := range []struct {
		name, payments, detail string
		status, calls          int
		confirmed, failed      bool
	}{
		{"closed unpaid", payments, detail, 200, 2, true, false},
		{"open order", payments, strings.Replace(detail, `"status":"canceled"`, `"status":"pending"`, 1), 200, 2, false, false},
		{"paid attempt", strings.Replace(payments, `"status":"failed"`, `"status":"succeeded"`, 1), detail, 200, 1, false, false},
		{"in-flight attempt", strings.Replace(payments, `"status":"failed"`, `"status":"pending"`, 1), detail, 200, 1, false, false},
		{"unknown attempt", strings.Replace(payments, `"status":"failed"`, `"status":"fixture-unknown"`, 1), detail, 200, 1, false, false},
		{"reference shared by distinct orders", strings.Replace(payments, `"orderId":"ORD_fixture"`, `"orderId":"ORD_other"`, 1), detail, 200, 1, false, false},
		{"different reference", strings.Replace(payments, reference, "other-reference", 1), detail, 200, 1, false, false},
		{"truncated history", strings.Replace(payments, `"paymentsCount":2`, `"paymentsCount":3`, 1), detail, 200, 1, false, false},
		{"not found", `{"data":{"payments":[],"paymentsCount":0}}`, detail, 200, 1, false, false},
		{"wrong store", payments, strings.ReplaceAll(detail, "STO_fixture", "STO_other"), 200, 2, false, false},
		{"test order", payments, strings.ReplaceAll(detail, `"testMode":false`, `"testMode":true`), 200, 2, false, false},
		{"missing mode", payments, strings.ReplaceAll(detail, `"testMode":false,`, ""), 200, 2, false, false},
		{"attempt changed between queries", payments, strings.Replace(detail, `"status":"failed"`, `"status":"succeeded"`, 1), 200, 2, false, false},
		{"refund evidence", payments, strings.Replace(detail, `"refundStatus":null`, `"refundStatus":"succeeded"`, 1), 200, 2, false, false},
		{"partial GraphQL error", payments, `{"errors":[{"message":"partial result"}],` + strings.TrimPrefix(detail, "{"), 200, 2, false, true},
		{"upstream unavailable", payments, detail, 502, 1, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls int
			http.DefaultTransport = paymentTestRoundTripper(func(req *http.Request) (*http.Response, error) {
				calls++
				query := decodePancakeInquiry(t, req)
				if calls == 1 {
					require.Equal(t, reference, query.Variables["reference"])
					assert.Contains(t, query.Query, "orderMerchantExternalId: { eq: $reference }")
					return paymentTestResponse(tc.status, tc.payments), nil
				}
				require.Equal(t, "ORD_fixture", query.Variables["id"])
				return paymentTestResponse(http.StatusOK, tc.detail), nil
			})
			closed, err := service.QueryCanceledWaffoPancakeOrder(t.Context(), reference, 7, false)
			assert.Equal(t, tc.confirmed, closed)
			assert.Equal(t, tc.failed, err != nil)
			assert.Equal(t, tc.calls, calls)
		})
	}
	http.DefaultTransport = paymentTestRoundTripper(func(*http.Request) (*http.Response, error) {
		t.Fatal("buyer or order type mismatch reached the provider")
		return nil, nil
	})
	closed, err := service.QueryCanceledWaffoPancakeOrder(t.Context(), reference, 70, false)
	require.NoError(t, err)
	assert.False(t, closed)
	closed, err = service.QueryCanceledWaffoPancakeOrder(t.Context(), reference, 7, true)
	require.NoError(t, err)
	assert.False(t, closed)
}

func TestPaymentOrderExpiryConfiguration(t *testing.T) {
	h := paymentOrderExpiryHandler{}
	t.Setenv("PAYMENT_ORDER_EXPIRY_TASK_ENABLED", "")
	t.Setenv("PAYMENT_ORDER_EXPIRY_TASK_INTERVAL_MINUTES", "")
	t.Setenv("PAYMENT_ORDER_EXPIRY_MIN_AGE_HOURS", "")
	now := common.GetTimestamp()
	assert.False(t, h.Enabled())
	assert.Equal(t, time.Hour, h.Interval())
	assert.Equal(t, now-72*3600, paymentOrderExpiryCutoff(now))
	t.Setenv("PAYMENT_ORDER_EXPIRY_TASK_ENABLED", "true")
	t.Setenv("PAYMENT_ORDER_EXPIRY_TASK_INTERVAL_MINUTES", "90")
	t.Setenv("PAYMENT_ORDER_EXPIRY_MIN_AGE_HOURS", "5")
	assert.True(t, h.Enabled())
	assert.Equal(t, 90*time.Minute, h.Interval())
	assert.Equal(t, now-25*3600, paymentOrderExpiryCutoff(now))
}

func TestPaymentOrderExpiryReconcilesBothTables(t *testing.T) {
	db := paymentOrderTestDatabase(t)
	configureWaffoPaymentTest(t)
	configurePancakePaymentTest(t)
	t.Setenv("PAYMENT_ORDER_EXPIRY_TASK_ENABLED", "true")
	t.Setenv("PAYMENT_ORDER_EXPIRY_MIN_AGE_HOURS", "72")
	user, plan := paymentTestBuyer(t, db)
	now := common.GetTimestamp()
	old := now - 73*3600
	var expectedQueries = map[string]int{}
	type orderFixture struct {
		subscription bool
		tradeNo      string
		want         string
		completed    int64
	}
	var fixtures []orderFixture
	for _, subscription := range []bool{false, true} {
		kind := "topup"
		pancakePrefix := "WAFFO_PANCAKE-"
		if subscription {
			kind, pancakePrefix = "subscription", "WAFFO_PANCAKE_SUB-"
		}
		for _, tc := range []struct {
			name, provider, status, want string
			created, completed           int64
			queries                      int
		}{
			{"waffo-closed", model.PaymentProviderWaffo, common.TopUpStatusPending, common.TopUpStatusExpired, old, 0, 1},
			{"waffo-processing", model.PaymentProviderWaffo, common.TopUpStatusPending, common.TopUpStatusPending, old, 0, 1},
			{"waffo-paid", model.PaymentProviderWaffo, common.TopUpStatusPending, common.TopUpStatusPending, old, 0, 1},
			{"waffo-captured", model.PaymentProviderWaffo, common.TopUpStatusPending, common.TopUpStatusPending, old, 0, 1},
			{"waffo-mismatch", model.PaymentProviderWaffo, common.TopUpStatusPending, common.TopUpStatusPending, old, 0, 1},
			{"waffo-bad-signature", model.PaymentProviderWaffo, common.TopUpStatusPending, common.TopUpStatusPending, old, 0, 1},
			{"waffo-unavailable", model.PaymentProviderWaffo, common.TopUpStatusPending, common.TopUpStatusPending, old, 0, 1},
			{"pancake-closed", model.PaymentProviderWaffoPancake, common.TopUpStatusPending, common.TopUpStatusExpired, old, 0, 2},
			{"too-young", model.PaymentProviderWaffo, common.TopUpStatusPending, common.TopUpStatusPending, now - 71*3600, 0, 0},
			{"already-paid", model.PaymentProviderWaffo, common.TopUpStatusSuccess, common.TopUpStatusSuccess, old, old + 5, 0},
			{"already-failed", model.PaymentProviderWaffo, common.TopUpStatusFailed, common.TopUpStatusFailed, old, old + 6, 0},
			{"already-expired", model.PaymentProviderWaffoPancake, common.TopUpStatusExpired, common.TopUpStatusExpired, old, old + 7, 0},
			{"stripe-delayed", model.PaymentProviderStripe, common.TopUpStatusPending, common.TopUpStatusPending, old, 0, 0},
			{"creem-unknown", model.PaymentProviderCreem, common.TopUpStatusPending, common.TopUpStatusPending, old, 0, 0},
			{"epay-unknown", model.PaymentProviderEpay, common.TopUpStatusPending, common.TopUpStatusPending, old, 0, 0},
			{"legacy-unbound", "", common.TopUpStatusPending, common.TopUpStatusPending, old, 0, 0},
		} {
			ref := fmt.Sprintf("%s-%s", tc.name, kind)
			if tc.provider == model.PaymentProviderWaffoPancake {
				ref = fmt.Sprintf("%s%d-%s", pancakePrefix, user.Id, ref)
			}
			if subscription {
				require.NoError(t, (&model.SubscriptionOrder{UserId: user.Id, PlanId: plan.Id, TradeNo: ref, PaymentProvider: tc.provider, PaymentMethod: tc.provider, Money: 1, Status: tc.status, CreateTime: tc.created, CompleteTime: tc.completed}).Insert())
			} else {
				require.NoError(t, (&model.TopUp{UserId: user.Id, TradeNo: ref, PaymentProvider: tc.provider, PaymentMethod: tc.provider, Money: 1, Amount: 1, Status: tc.status, CreateTime: tc.created, CompleteTime: tc.completed}).Insert())
			}
			fixtures = append(fixtures, orderFixture{subscription, ref, tc.want, tc.completed})
			if tc.queries > 0 {
				expectedQueries[ref] = tc.queries
			}
		}
	}
	actualQueries := map[string]int{}
	var pancakeRef string
	http.DefaultTransport = paymentTestRoundTripper(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path == "/v1/graphql" {
			query := decodePancakeInquiry(t, req)
			if ref, ok := query.Variables["reference"].(string); ok {
				pancakeRef = ref
				actualQueries[ref]++
				payments, _ := pancakeOrderInquiryFixture(ref)
				return paymentTestResponse(http.StatusOK, payments), nil
			}
			require.Equal(t, "ORD_fixture", query.Variables["id"])
			actualQueries[pancakeRef]++
			_, detail := pancakeOrderInquiryFixture(pancakeRef)
			return paymentTestResponse(http.StatusOK, detail), nil
		}
		require.True(t, strings.HasSuffix(req.URL.Path, "/order/inquiry"), "unexpected payment endpoint: %s", req.URL.Path)
		assert.Equal(t, http.MethodPost, req.Method)
		var inquiry order.InquiryOrderParams
		require.NoError(t, common.DecodeJson(req.Body, &inquiry))
		ref := inquiry.PaymentRequestID
		actualQueries[ref]++
		data := order.InquiryOrderData{PaymentRequestID: ref, MerchantOrderID: ref, AcquiringOrderID: "upstream_order", OrderStatus: core.OrderStatusOrderClose}
		switch {
		case strings.HasPrefix(ref, "waffo-processing"):
			data.OrderStatus = core.OrderStatusPayInProgress
		case strings.HasPrefix(ref, "waffo-paid"):
			data.OrderStatus = core.OrderStatusPaySuccess
		case strings.HasPrefix(ref, "waffo-captured"):
			data.FinalDealAmount = "1.00"
		case strings.HasPrefix(ref, "waffo-mismatch"):
			data.MerchantOrderID = "different-order"
		}
		body, err := common.Marshal(core.ApiResponse[order.InquiryOrderData]{Code: "0", Data: &data})
		require.NoError(t, err)
		sig, err := utils.Sign(string(body), setting.WaffoPrivateKey)
		require.NoError(t, err)
		if strings.HasPrefix(ref, "waffo-bad-signature") {
			sig = "invalid"
		}
		status := http.StatusOK
		if strings.HasPrefix(ref, "waffo-unavailable") {
			status = http.StatusBadGateway
		}
		resp := paymentTestResponse(status, string(body))
		resp.Header.Set(core.HeaderSignature, sig)
		return resp, nil
	})

	for pass := range 2 {
		clear(actualQueries)
		task, err := model.CreateSystemTask(model.SystemTaskTypePaymentOrderExpiry, nil, nil)
		require.NoError(t, err)
		claimedTask, claimed, err := model.ClaimSystemTask(task.ID, task.Type, "payment-worker", now+120)
		require.NoError(t, err)
		require.True(t, claimed)
		paymentOrderExpiryHandler{}.Run(t.Context(), claimedTask, "payment-worker")
		storedTask, err := model.GetSystemTaskByTaskID(task.TaskID)
		require.NoError(t, err)
		require.Equal(t, model.SystemTaskStatusSucceeded, storedTask.Status, storedTask.Error)
		var result paymentOrderExpiryResult
		require.NoError(t, common.UnmarshalJsonStr(storedTask.Result, &result))
		assert.Equal(t, result.TopUps, result.Subscriptions)
		assert.Equal(t, 2, result.TopUps.QueryErrors)
		if pass == 0 {
			assert.Equal(t, expectedQueries, actualQueries)
			assert.Equal(t, 2, result.TopUps.Expired)
		} else {
			assert.Equal(t, 0, result.TopUps.Expired)
			for ref := range actualQueries {
				assert.NotContains(t, ref, "closed", "expired orders must not be queried again")
			}
		}
		for i, fixture := range fixtures {
			got, ts := paymentTestState(t, fixture.subscription, fixture.tradeNo)
			assert.Equal(t, fixture.want, got, fixture.tradeNo)
			if pass == 0 && fixture.completed == 0 && fixture.want == common.TopUpStatusExpired {
				assert.GreaterOrEqual(t, ts, now, fixture.tradeNo)
				fixtures[i].completed = ts
			} else {
				assert.Equal(t, fixture.completed, ts, fixture.tradeNo)
			}
		}
		assert.Nil(t, storedTask.ActiveKey)
	}

	configureStripePaymentTest(t)
	assert.Equal(t, http.StatusOK, postStripePaymentTestEvent(t, stripe.EventTypeCheckoutSessionAsyncPaymentSucceeded, "stripe-delayed-topup").Code)
	assert.Equal(t, http.StatusOK, postStripePaymentTestEvent(t, stripe.EventTypeCheckoutSessionAsyncPaymentSucceeded, "stripe-delayed-subscription").Code)
	for _, fixture := range fixtures {
		if strings.HasPrefix(fixture.tradeNo, "stripe-delayed-") {
			status, completed := paymentTestState(t, fixture.subscription, fixture.tradeNo)
			assert.Equal(t, common.TopUpStatusSuccess, status)
			assert.GreaterOrEqual(t, completed, now)
		}
	}
	var updated model.User
	require.NoError(t, db.First(&updated, user.Id).Error)
	assert.EqualValues(t, common.QuotaPerUnit, updated.Quota)
	var subscriptions int64
	require.NoError(t, db.Model(&model.UserSubscription{}).Count(&subscriptions).Error)
	assert.EqualValues(t, 1, subscriptions)
}

func configureStripePaymentTest(t *testing.T) {
	t.Helper()
	apiKey, webhookSecret, priceID := setting.StripeApiSecret, setting.StripeWebhookSecret, setting.StripePriceId
	t.Cleanup(func() {
		setting.StripeApiSecret, setting.StripeWebhookSecret, setting.StripePriceId = apiKey, webhookSecret, priceID
	})
	setting.StripeApiSecret, setting.StripeWebhookSecret, setting.StripePriceId = "fixture-stripe-api", "fixture-stripe-signing", "price_fixture"
}

func postStripePaymentTestEvent(t *testing.T, eventType stripe.EventType, reference string) *httptest.ResponseRecorder {
	t.Helper()
	body, err := common.Marshal(gin.H{
		"id": "evt_fixture", "object": "event", "type": eventType,
		"data": gin.H{"object": gin.H{"client_reference_id": reference, "customer": "cus_fixture", "amount_total": 100, "currency": "usd"}},
	})
	require.NoError(t, err)
	signature := webhook.GenerateTestSignedPayload(&webhook.UnsignedPayload{Payload: body, Secret: setting.StripeWebhookSecret}).Header
	return postPaymentTestEvent(StripeWebhook, string(body), "Stripe-Signature", signature)
}

func TestPaymentOrderStripeAsyncFailure(t *testing.T) {
	paymentOrderTestDatabase(t)
	configureStripePaymentTest(t)
	for _, tc := range []struct {
		name, provider, initial, want string
	}{
		{"pending", model.PaymentProviderStripe, common.TopUpStatusPending, common.TopUpStatusFailed},
		{"successful", model.PaymentProviderStripe, common.TopUpStatusSuccess, common.TopUpStatusSuccess},
		{"expired", model.PaymentProviderStripe, common.TopUpStatusExpired, common.TopUpStatusExpired},
		{"other provider", model.PaymentProviderCreem, common.TopUpStatusPending, common.TopUpStatusPending},
	} {
		t.Run(tc.name, func(t *testing.T) {
			topup := model.TopUp{TradeNo: "top-up " + tc.name, PaymentProvider: tc.provider, Status: tc.initial}
			subscription := model.SubscriptionOrder{TradeNo: "subscription " + tc.name, PaymentProvider: tc.provider, Status: tc.initial}
			require.NoError(t, topup.Insert())
			require.NoError(t, subscription.Insert())
			for _, ref := range []string{topup.TradeNo, subscription.TradeNo} {
				assert.Equal(t, http.StatusOK, postStripePaymentTestEvent(t, stripe.EventTypeCheckoutSessionAsyncPaymentFailed, ref).Code)
			}
			topupStatus, _ := paymentTestState(t, false, topup.TradeNo)
			subStatus, completed := paymentTestState(t, true, subscription.TradeNo)
			assert.Equal(t, tc.want, topupStatus)
			assert.Equal(t, tc.want, subStatus)
			assert.Equal(t, http.StatusOK, postStripePaymentTestEvent(t, stripe.EventTypeCheckoutSessionAsyncPaymentFailed, subscription.TradeNo).Code)
			_, afterReplay := paymentTestState(t, true, subscription.TradeNo)
			assert.Equal(t, completed, afterReplay)
		})
	}
}

func TestPaymentOrderExpiryRejectsStaleLeaseAndOrder(t *testing.T) {
	db := paymentOrderTestDatabase(t)
	ctx := t.Context()
	now := common.GetTimestamp()
	cutoff := now - 72*3600
	task, err := model.CreateSystemTask(model.SystemTaskTypePaymentOrderExpiry, nil, nil)
	require.NoError(t, err)
	_, err = model.CreateSystemTask(model.SystemTaskTypePaymentOrderExpiry, nil, nil)
	require.Error(t, err, "the active-key guard must prevent another scheduled task")

	type claimResult struct {
		runner  string
		claimed bool
		err     error
	}
	start := make(chan struct{})
	claims := make(chan claimResult, 2)
	for _, runner := range []string{"worker-one", "worker-two"} {
		go func() {
			<-start
			_, claimed, err := model.ClaimSystemTask(task.ID, task.Type, runner, now+120)
			claims <- claimResult{runner, claimed, err}
		}()
	}
	close(start)
	var owner, other string
	for range 2 {
		result := <-claims
		require.NoError(t, result.err)
		if result.claimed {
			require.Empty(t, owner)
			owner = result.runner
		} else {
			other = result.runner
		}
	}
	require.NotEmpty(t, owner)
	require.NotEmpty(t, other)

	for _, subscription := range []bool{false, true} {
		for _, tc := range []struct {
			name    string
			changes map[string]any
			want    string
		}{
			{"unchanged", nil, common.TopUpStatusExpired},
			{"settled during inquiry", map[string]any{"status": common.TopUpStatusSuccess, "complete_time": now - 5}, common.TopUpStatusSuccess},
			{"reference changed", map[string]any{"trade_no": "new-reference"}, common.TopUpStatusPending},
			{"channel changed", map[string]any{"payment_provider": model.PaymentProviderWaffoPancake}, common.TopUpStatusPending},
			{"buyer changed", map[string]any{"user_id": 2}, common.TopUpStatusPending},
			{"creation time changed", map[string]any{"create_time": now}, common.TopUpStatusPending},
		} {
			t.Run(fmt.Sprintf("subscription=%v/%s", subscription, tc.name), func(t *testing.T) {
				ref := "snapshot-" + tc.name
				var row any = &model.TopUp{UserId: 1, TradeNo: ref, PaymentProvider: model.PaymentProviderWaffo, Status: common.TopUpStatusPending, CreateTime: cutoff}
				if subscription {
					row = &model.SubscriptionOrder{UserId: 1, TradeNo: ref, PaymentProvider: model.PaymentProviderWaffo, Status: common.TopUpStatusPending, CreateTime: cutoff}
				}
				require.NoError(t, db.Create(row).Error)
				candidates, err := model.ListPaymentOrderExpiryCandidates(ctx, subscription, 0, cutoff, 100)
				require.NoError(t, err)
				require.NotEmpty(t, candidates)
				candidate := candidates[len(candidates)-1]
				require.Equal(t, ref, candidate.TradeNo, "the configured age boundary is inclusive")
				if tc.changes != nil {
					require.NoError(t, db.Model(row).Updates(tc.changes).Error)
					if changedRef, ok := tc.changes["trade_no"].(string); ok {
						ref = changedRef
					}
				}
				expired, err := model.ExpireConfirmedUnpaidOrder(ctx, task.TaskID, other, subscription, candidate, cutoff)
				require.ErrorIs(t, err, model.ErrSystemTaskLockLost)
				assert.False(t, expired)
				expired, err = model.ExpireConfirmedUnpaidOrder(ctx, task.TaskID, owner, subscription, candidate, cutoff)
				require.NoError(t, err)
				assert.Equal(t, tc.want == common.TopUpStatusExpired, expired)
				status, completed := paymentTestState(t, subscription, ref)
				assert.Equal(t, tc.want, status)
				if tc.want == common.TopUpStatusSuccess {
					assert.EqualValues(t, now-5, completed)
				}
			})
		}
	}

	row := &model.TopUp{TradeNo: "lease-expired-during-inquiry", PaymentProvider: model.PaymentProviderWaffo, Status: common.TopUpStatusPending, CreateTime: cutoff}
	require.NoError(t, row.Insert())
	candidates, err := model.ListPaymentOrderExpiryCandidates(ctx, false, row.Id-1, cutoff, 1)
	require.NoError(t, err)
	require.Len(t, candidates, 1)
	require.NoError(t, db.Model(&model.SystemTaskLock{}).Where("type = ?", task.Type).Update("locked_until", now-1).Error)
	expired, err := model.ExpireConfirmedUnpaidOrder(ctx, task.TaskID, owner, false, candidates[0], cutoff)
	require.ErrorIs(t, err, model.ErrSystemTaskLockLost)
	assert.False(t, expired)
	status, completed := paymentTestState(t, false, row.TradeNo)
	assert.Equal(t, common.TopUpStatusPending, status)
	assert.Zero(t, completed)
}

func TestPaymentOrderExpiryReadsBeyondFirstPage(t *testing.T) {
	db := paymentOrderTestDatabase(t)
	t.Setenv("PAYMENT_ORDER_EXPIRY_TASK_ENABLED", "true")
	t.Setenv("PAYMENT_ORDER_EXPIRY_MIN_AGE_HOURS", "72")
	old := common.GetTimestamp() - 73*3600
	rows := make([]model.TopUp, paymentOrderExpiryBatchSize+1)
	for i := range rows {
		rows[i] = model.TopUp{TradeNo: fmt.Sprintf("unconfirmed-%d", i), PaymentProvider: model.PaymentProviderWaffo, CreateTime: old, Status: common.TopUpStatusPending}
	}
	require.NoError(t, db.Create(&rows).Error)
	var counts paymentOrderExpiryCounts
	// An unavailable inquiry client leaves each row pending, including rows
	// from the first batch. They must not prevent scanning the next batch.
	require.NoError(t, reconcileAgedPaymentOrders(t.Context(), &model.SystemTask{}, "", false, old, nil, &counts))
	assert.Equal(t, len(rows), counts.Checked)
	assert.Equal(t, len(rows), counts.Unconfirmed)
	assert.Zero(t, counts.Expired)
}
