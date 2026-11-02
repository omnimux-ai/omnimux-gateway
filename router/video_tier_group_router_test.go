package router

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// This file guards the router wiring of the resolution tier selection, not the
// selection logic itself (middleware/video_tier_group_test.go covers that).
// Both public video submit routes are driven through their real registration, so
// deleting middleware.SelectVideoTierGroup() from either chain turns these
// assertions red:
//
//   - /v1/video/generations  — public API key surface (TokenAuth)
//   - /pg/video/generations  — browser playground surface (UserAuth session),
//     the route the model playground actually submits to
//
// The two tier channels carry distinct channel types on purpose. Channel
// distribution selects one before the request reaches an adaptor, and the
// resulting platform failure names the type, which is how the assertions below
// observe *which* channel the tier group selected without any upstream call.

const (
	videoTierGroup720p        = "wan-3.0-720p"
	videoTierGroup1080p       = "wan-3.0-1080p"
	videoTierChannel720pType  = 2
	videoTierChannel1080pType = 3
	videoTierNoChannelMsg     = "No available channel for model"
)

type videoTierGroupRouteHarness struct {
	engine       *gin.Engine
	apiKey       string
	browserToken string
}

// newVideoTierGroupRouteHarness registers the real video and relay routers on a
// temporary database that serves only the 720p tier channel: a request that
// resolves to the 720p group selects it, while a request that resolves to any
// other group has no channel at all and reports the group it looked in.
func newVideoTierGroupRouteHarness(t *testing.T) *videoTierGroupRouteHarness {
	t.Helper()
	gin.SetMode(gin.TestMode)
	_ = i18n.Init()

	previousDB := model.DB
	previousDatabaseType := common.MainDatabaseType()
	previousLogDatabaseType := common.LogDatabaseType()
	previousSQLitePath := common.SQLitePath
	previousMasterNode := common.IsMasterNode
	previousRedisEnabled := common.RedisEnabled
	previousMemoryCache := common.MemoryCacheEnabled
	previousSessionSecret := common.SessionSecret
	previousUsableGroups := setting.UserUsableGroups2JSONString()
	previousGroupRatios := ratio_setting.GroupRatio2JSONString()
	previousPrices := ratio_setting.ModelPrice2JSONString()

	common.SQLitePath = t.TempDir() + "/video-tier-group-router.db"
	common.IsMasterNode = false
	common.RedisEnabled = false
	common.MemoryCacheEnabled = false
	common.SessionSecret = "video-tier-group-router-secret"
	t.Setenv("SQL_DSN", "")
	require.NoError(t, model.InitDB())
	database := model.DB
	require.NoError(t, database.AutoMigrate(
		&model.User{}, &model.Token{}, &model.UserSession{}, &model.Channel{},
		&model.Ability{}, &model.Option{}, &model.AuditLog{}, &model.UserSubscription{},
	))
	// The tier groups are provisioned configuration; the middleware pins a tier
	// only when the group exists, is visible to the account and has a ratio.
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(
		`{"default":"默认","wan-3.0-720p":"Wan 3.0 720p","wan-3.0-1080p":"Wan 3.0 1080p"}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(
		`{"default":1,"wan-3.0-720p":1.2,"wan-3.0-1080p":1.8}`))
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"wan-3.0":0.0543}`))

	t.Cleanup(func() {
		sqlDB, closeErr := database.DB()
		require.NoError(t, closeErr)
		require.NoError(t, sqlDB.Close())
		model.DB = previousDB
		common.SetDatabaseTypes(previousDatabaseType, previousLogDatabaseType)
		common.SQLitePath = previousSQLitePath
		common.IsMasterNode = previousMasterNode
		common.RedisEnabled = previousRedisEnabled
		common.MemoryCacheEnabled = previousMemoryCache
		common.SessionSecret = previousSessionSecret
		_ = setting.UpdateUserUsableGroupsByJSONString(previousUsableGroups)
		_ = ratio_setting.UpdateGroupRatioByJSONString(previousGroupRatios)
		_ = ratio_setting.UpdateModelPriceByJSONString(previousPrices)
	})

	require.NoError(t, database.Create(&model.User{
		Id: 91, Username: "video-tier-group", Role: common.RoleCommonUser, Status: common.UserStatusEnabled,
		Quota: 100000000, Group: "default", AuthVersion: 1,
	}).Error)
	// The stored key carries no "sk-" prefix; the client sends the prefixed form.
	apiKey := "videotiergroup"
	require.NoError(t, database.Create(&model.Token{
		Id: 1, UserId: 91, Key: apiKey, Status: common.TokenStatusEnabled,
		Name: "video tier group", ExpiredTime: -1, UnlimitedQuota: true,
	}).Error)
	session := &model.UserSession{
		SID: "video-tier-group-session", UserID: 91, Version: 1, UserAuthVersion: 1,
		Status: model.UserSessionStatusActive, RefreshHash: "video-tier-group-refresh",
		LoginMethod: "password", LastActiveAt: time.Now().Unix(), ExpiresAt: time.Now().Unix() + 3600,
	}
	require.NoError(t, model.CreateUserSession(session))
	browserToken, _, err := service.IssueAccessToken(service.AuthIdentity{
		UserID: 91, SessionID: session.SID, UserAuthVersion: 1, SessionVersion: 1,
	})
	require.NoError(t, err)

	// Only the 720p tier is served. Its channel type is the observable: the
	// request reaches adaptor resolution with the selected channel's type.
	require.NoError(t, database.Create(&model.Channel{
		Id: 71, Type: videoTierChannel720pType, Key: "tier-720p", Status: common.ChannelStatusEnabled,
		Name: videoTierGroup720p, Models: "wan-3.0", Group: videoTierGroup720p,
	}).Error)
	require.NoError(t, database.Create(&model.Ability{
		Group: videoTierGroup720p, Model: "wan-3.0", ChannelId: 71, Enabled: true, Weight: 100,
	}).Error)

	engine := gin.New()
	require.NotPanics(t, func() {
		SetVideoRouter(engine)
		SetRelayRouter(engine)
	})
	return &videoTierGroupRouteHarness{engine: engine, apiKey: apiKey, browserToken: browserToken}
}

func (h *videoTierGroupRouteHarness) post(t *testing.T, path, body string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	credential := h.apiKey
	if path == "/pg/video/generations" {
		credential = h.browserToken
	} else {
		credential = "sk-" + credential
	}
	request := httptest.NewRequest(http.MethodPost, path, bytes.NewReader([]byte(body)))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+credential)
	for name, value := range headers {
		request.Header.Set(name, value)
	}
	recorder := httptest.NewRecorder()
	h.engine.ServeHTTP(recorder, request)
	require.NotEqual(t, http.StatusUnauthorized, recorder.Code, "test credential must authenticate: %s", recorder.Body.String())
	require.NotEqual(t, http.StatusForbidden, recorder.Code, "test credential must authenticate: %s", recorder.Body.String())
	return recorder
}

// The 720p request selects the 720p tier channel; the 1080p request resolves to
// the 1080p tier group, which no channel serves, so distribution names that
// group. Together they pin the tier table, the mount point ahead of Distribute()
// and the channel each tier selects, on both real routes.
func TestVideoTierGroupRoutesSelectTierChannelAndGroup(t *testing.T) {
	harness := newVideoTierGroupRouteHarness(t)
	for _, path := range []string{"/v1/video/generations", "/pg/video/generations"} {
		t.Run(path, func(t *testing.T) {
			t.Run("720p selects the 720p tier channel", func(t *testing.T) {
				recorder := harness.post(t, path, `{"model":"wan-3.0","resolution":"720p","prompt":"cat"}`, nil)
				body := recorder.Body.String()
				assert.NotContains(t, body, videoTierNoChannelMsg, "the 720p tier must select its channel")
				assert.Contains(t, body, fmt.Sprintf("invalid api platform: %d", videoTierChannel720pType),
					"the selected channel's type identifies the 720p tier channel")
			})

			t.Run("1080p resolves to the 1080p tier group", func(t *testing.T) {
				recorder := harness.post(t, path, `{"model":"wan-3.0","resolution":"1080p","prompt":"cat"}`, nil)
				body := recorder.Body.String()
				assert.Contains(t, body, videoTierNoChannelMsg)
				assert.Contains(t, body, "under group "+videoTierGroup1080p,
					"the 1080p tier group must reach distribution")
			})

			t.Run("size declares the same tier", func(t *testing.T) {
				recorder := harness.post(t, path, `{"model":"wan-3.0","size":"1920x1080","prompt":"cat"}`, nil)
				assert.Contains(t, recorder.Body.String(), "under group "+videoTierGroup1080p)
			})
		})
	}
}

// An explicit caller choice is never overridden by the automatic tier.
func TestVideoTierGroupRoutesKeepExplicitGroup(t *testing.T) {
	harness := newVideoTierGroupRouteHarness(t)
	for _, path := range []string{"/v1/video/generations", "/pg/video/generations"} {
		t.Run(path, func(t *testing.T) {
			for _, testCase := range []struct {
				name    string
				body    string
				headers map[string]string
			}{
				{
					name:    "X-Omnimux-Group header",
					body:    `{"model":"wan-3.0","resolution":"720p","prompt":"cat"}`,
					headers: map[string]string{"X-Omnimux-Group": "default"},
				},
				{
					name:    "legacy New-Api-Group header",
					body:    `{"model":"wan-3.0","resolution":"720p","prompt":"cat"}`,
					headers: map[string]string{"New-Api-Group": "default"},
				},
				{
					name: "body group field",
					body: `{"model":"wan-3.0","group":"default","resolution":"720p","prompt":"cat"}`,
				},
				{
					name: "model@group suffix",
					body: `{"model":"wan-3.0@default","resolution":"720p","prompt":"cat"}`,
				},
			} {
				t.Run(testCase.name, func(t *testing.T) {
					recorder := harness.post(t, path, testCase.body, testCase.headers)
					body := recorder.Body.String()
					assert.Contains(t, body, "under group default", "the explicit group must survive distribution")
					assert.NotContains(t, body, "under group "+videoTierGroup720p)
				})
			}

			// An explicit group is the escape hatch from the resolution
			// requirement: it must never be answered with a 400.
			for _, testCase := range []struct {
				name    string
				body    string
				headers map[string]string
			}{
				{
					name:    "X-Omnimux-Group header",
					body:    `{"model":"wan-3.0","prompt":"cat"}`,
					headers: map[string]string{"X-Omnimux-Group": "default"},
				},
				{
					name:    "legacy New-Api-Group header",
					body:    `{"model":"wan-3.0","prompt":"cat"}`,
					headers: map[string]string{"New-Api-Group": "default"},
				},
				{name: "body group field", body: `{"model":"wan-3.0","group":"default","prompt":"cat"}`},
				{name: "model@group suffix", body: `{"model":"wan-3.0@default","prompt":"cat"}`},
			} {
				t.Run("without resolution: "+testCase.name, func(t *testing.T) {
					recorder := harness.post(t, path, testCase.body, testCase.headers)
					body := recorder.Body.String()
					assert.NotContains(t, body, "resolution is required",
						"an explicit group must never be rejected for a missing resolution")
					assert.Contains(t, body, "under group default", "the explicit group must survive distribution")
				})
			}

			t.Run("explicit tier group wins over the body resolution", func(t *testing.T) {
				recorder := harness.post(t, path, `{"model":"wan-3.0","resolution":"720p","prompt":"cat"}`,
					map[string]string{"X-Omnimux-Group": videoTierGroup1080p})
				body := recorder.Body.String()
				assert.Contains(t, body, "under group "+videoTierGroup1080p)
				assert.NotContains(t, body, fmt.Sprintf("invalid api platform: %d", videoTierChannel720pType),
					"an explicit 1080p choice must not fall back to the 720p tier channel")
			})
		})
	}
}

// Models with no declared tier keep their existing behavior, including when the
// resolution is absent or unreadable: the requirement belongs to the tier table.
func TestVideoTierGroupRoutesLeaveOtherRequestsAlone(t *testing.T) {
	harness := newVideoTierGroupRouteHarness(t)
	for _, path := range []string{"/v1/video/generations", "/pg/video/generations"} {
		t.Run(path, func(t *testing.T) {
			for _, body := range []string{
				`{"model":"seedance-2-5","resolution":"720p","prompt":"cat"}`,
				`{"model":"seedance-2-5","prompt":"cat"}`,
				`{"model":"sora-2","prompt":"cat"}`,
				`{"model":"wan-3.0-prime","resolution":"1080p","prompt":"cat"}`,
				`{"model":"wan-3.0-prime","resolution":"2K","prompt":"cat"}`,
			} {
				t.Run(body, func(t *testing.T) {
					recorder := harness.post(t, path, body, nil)
					bodyText := recorder.Body.String()
					assert.NotContains(t, bodyText, "resolution is required",
						"a model without declared tiers is never rejected for a missing resolution")
					assert.Contains(t, bodyText, "under group default",
						"a request without a declared tier must keep its existing group")
					assert.NotContains(t, bodyText, videoTierGroup720p)
					assert.NotContains(t, bodyText, videoTierGroup1080p)
				})
			}
		})
	}
}

// A declared tier model without an explicit group and without a readable
// resolution is a caller mistake, not a capacity problem: it must be answered
// with an actionable 400 that names the accepted qualities, on both routes.
func TestVideoTierGroupRoutesRequireResolutionForTierModels(t *testing.T) {
	harness := newVideoTierGroupRouteHarness(t)
	for _, path := range []string{"/v1/video/generations", "/pg/video/generations"} {
		t.Run(path, func(t *testing.T) {
			for _, body := range []string{
				`{"model":"wan-3.0","prompt":"cat"}`,
				`{"model":"wan-3.0","resolution":"","prompt":"cat"}`,
				`{"model":"wan-3.0","resolution":"480p","prompt":"cat"}`,
				`{"model":"wan-3.0","resolution":"2K","prompt":"cat"}`,
				`{"model":"wan-3.0","size":"16:9","prompt":"cat"}`,
			} {
				t.Run(body, func(t *testing.T) {
					recorder := harness.post(t, path, body, nil)
					assert.Equal(t, http.StatusBadRequest, recorder.Code)
					bodyText := recorder.Body.String()
					assert.Contains(t, bodyText, "invalid_request", "the documented error code")
					assert.Contains(t, bodyText, "resolution is required for wan-3.0")
					assert.Contains(t, bodyText, "720p")
					assert.Contains(t, bodyText, "1080p")
					assert.Contains(t, bodyText, "select a group explicitly")
					// The request must never reach distribution on a fallback group.
					assert.NotContains(t, bodyText, videoTierNoChannelMsg)
				})
			}

			// A body that cannot be parsed is not a missing resolution: the
			// middleware must leave that diagnosis to the step that owns the body.
			for _, body := range []string{`not json`, `{"model":"wan-3.0",`} {
				t.Run("unparseable "+body, func(t *testing.T) {
					recorder := harness.post(t, path, body, nil)
					assert.NotContains(t, recorder.Body.String(), "resolution is required")
				})
			}
		})
	}
}
