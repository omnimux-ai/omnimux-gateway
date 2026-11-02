package middleware

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// The tier groups are provisioned configuration, so every test states them
// explicitly instead of relying on a default catalog.
func setupVideoTierGroupSettings(t *testing.T, usableGroups, groupRatios string) {
	t.Helper()
	originalUsableGroups := setting.UserUsableGroups2JSONString()
	originalGroupRatios := ratio_setting.GroupRatio2JSONString()
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(usableGroups))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(groupRatios))
	t.Cleanup(func() {
		_ = setting.UpdateUserUsableGroupsByJSONString(originalUsableGroups)
		_ = ratio_setting.UpdateGroupRatioByJSONString(originalGroupRatios)
	})
}

func setupVideoTierGroupDefaults(t *testing.T) {
	t.Helper()
	setupVideoTierGroupSettings(t,
		`{"default":"默认","wan-3.0-720p":"Wan 3.0 720p","wan-3.0-1080p":"Wan 3.0 1080p"}`,
		`{"default":1,"wan-3.0-720p":1.2,"wan-3.0-1080p":1.8}`)
}

type videoTierGroupProbe struct {
	status        int
	body          string
	usingGroup    string
	tokenGroup    string
	header        string
	legacyHeader  string
	downstreamOK  bool
	downstreamRaw string
	// middlewareReads is the body-read count observed immediately after this
	// middleware's turn, before any downstream handler touches the body.
	middlewareReads int
}

// runSelectVideoTierGroup drives the middleware alone and reports the group
// state the downstream chain would see, plus a second read of the body to prove
// the middleware left it intact for the plugin prepare and relay steps.
func runSelectVideoTierGroup(t *testing.T, body string, headers map[string]string, cached any) videoTierGroupProbe {
	t.Helper()
	return runSelectVideoTierGroupWithReader(t, bytes.NewReader([]byte(body)), headers, cached)
}

// runSelectVideoTierGroupWithReader is the same harness with a caller-supplied
// body reader, so a test can observe whether the middleware touched the body.
func runSelectVideoTierGroupWithReader(t *testing.T, body io.Reader, headers map[string]string, cached any) videoTierGroupProbe {
	t.Helper()
	return runSelectVideoTierGroupWithCounter(t, body, headers, cached, nil)
}

// runSelectVideoTierGroupWithCounter adds a snapshot step right after the
// middleware under test, so a counting body can prove whether that middleware
// read the request body. The downstream handler deliberately reads the body
// too, which keeps the assertion non-vacuous.
func runSelectVideoTierGroupWithCounter(t *testing.T, body io.Reader, headers map[string]string, cached any, counter *videoTierCountingBody) videoTierGroupProbe {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		common.SetContextKey(c, constant.ContextKeyUserId, 1)
		common.SetContextKey(c, constant.ContextKeyUserGroup, "default")
		common.SetContextKey(c, constant.ContextKeyUsingGroup, "default")
		common.SetContextKey(c, constant.ContextKeyTokenGroup, "default")
		if cached != nil {
			c.Set(contextKeyTaskPluginEndpointModel, cached)
		}
		c.Next()
	})
	router.Use(SelectVideoTierGroup())

	var probe videoTierGroupProbe
	if counter != nil {
		router.Use(func(c *gin.Context) {
			probe.middlewareReads = counter.reads
			c.Next()
		})
	}
	router.POST("/v1/video/generations", func(c *gin.Context) {
		probe.usingGroup = common.GetContextKeyString(c, constant.ContextKeyUsingGroup)
		probe.tokenGroup = common.GetContextKeyString(c, constant.ContextKeyTokenGroup)
		probe.header = c.GetHeader("X-Omnimux-Group")
		probe.legacyHeader = c.GetHeader("New-Api-Group")

		var downstream struct {
			Model      string `json:"model"`
			Resolution any    `json:"resolution"`
			Prompt     string `json:"prompt"`
		}
		if err := common.UnmarshalBodyReusable(c, &downstream); err != nil {
			probe.downstreamOK = false
			c.Status(http.StatusOK)
			return
		}
		raw, err := common.Marshal(downstream)
		require.NoError(t, err)
		probe.downstreamOK = true
		probe.downstreamRaw = string(raw)
		c.Status(http.StatusOK)
	})

	request := httptest.NewRequest(http.MethodPost, "/v1/video/generations", body)
	request.Header.Set("Content-Type", "application/json")
	for name, value := range headers {
		request.Header.Set(name, value)
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	probe.status = recorder.Code
	probe.body = recorder.Body.String()
	return probe
}

func TestSelectVideoTierGroupFromResolution(t *testing.T) {
	setupVideoTierGroupDefaults(t)
	cases := []struct {
		name string
		body string
		want string
	}{
		{"720p resolution", `{"model":"wan-3.0","resolution":"720p","prompt":"cat"}`, "wan-3.0-720p"},
		{"720P uppercase", `{"model":"wan-3.0","resolution":"720P","prompt":"cat"}`, "wan-3.0-720p"},
		{"720p padded", `{"model":"wan-3.0","resolution":" 720P ","prompt":"cat"}`, "wan-3.0-720p"},
		{"1280x720 size", `{"model":"wan-3.0","size":"1280x720","prompt":"cat"}`, "wan-3.0-720p"},
		{"portrait 720x1280 size", `{"model":"wan-3.0","size":"720x1280","prompt":"cat"}`, "wan-3.0-720p"},
		{"1080p resolution", `{"model":"wan-3.0","resolution":"1080p","prompt":"cat"}`, "wan-3.0-1080p"},
		{"1080P uppercase", `{"model":"wan-3.0","resolution":"1080P","prompt":"cat"}`, "wan-3.0-1080p"},
		{"1920x1080 size", `{"model":"wan-3.0","size":"1920x1080","prompt":"cat"}`, "wan-3.0-1080p"},
		{"1920X1080 uppercase separator", `{"model":"wan-3.0","size":"1920X1080","prompt":"cat"}`, "wan-3.0-1080p"},
		{"1080x1920 portrait size", `{"model":"wan-3.0","size":"1080x1920","prompt":"cat"}`, "wan-3.0-1080p"},
		{"resolution wins over unrelated size", `{"model":"wan-3.0","resolution":"1080p","size":"16:9","prompt":"cat"}`, "wan-3.0-1080p"},
		{"numeric resolution", `{"model":"wan-3.0","resolution":720,"prompt":"cat"}`, "wan-3.0-720p"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			probe := runSelectVideoTierGroup(t, tc.body, nil, nil)
			assert.Equal(t, tc.want, probe.usingGroup)
			assert.Equal(t, tc.want, probe.tokenGroup, "billing reads the token group and must agree with routing")
			assert.Equal(t, tc.want, probe.header, "distribution reads the header and must agree with routing")
			assert.True(t, probe.downstreamOK, "the body must stay readable for the downstream chain")
			assert.Contains(t, probe.downstreamRaw, `"cat"`, "the original body must survive the read")
		})
	}
}

func TestSelectVideoTierGroupExplicitChoiceWins(t *testing.T) {
	setupVideoTierGroupDefaults(t)
	cases := []struct {
		name    string
		body    string
		headers map[string]string
	}{
		{"X-Omnimux-Group header", `{"model":"wan-3.0","resolution":"1080p","prompt":"cat"}`, map[string]string{"X-Omnimux-Group": "default"}},
		{"legacy New-Api-Group header", `{"model":"wan-3.0","resolution":"1080p","prompt":"cat"}`, map[string]string{"New-Api-Group": "default"}},
		{"body group field", `{"model":"wan-3.0","group":"default","resolution":"1080p","prompt":"cat"}`, nil},
		{"model@group suffix", `{"model":"wan-3.0@wan-3.0-720p","resolution":"1080p","prompt":"cat"}`, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			probe := runSelectVideoTierGroup(t, tc.body, tc.headers, nil)
			assert.Equal(t, "default", probe.usingGroup, "an explicit selection must never be overridden")
			assert.Equal(t, "default", probe.tokenGroup)
			assert.Equal(t, tc.headers["X-Omnimux-Group"], probe.header)
			assert.Equal(t, tc.headers["New-Api-Group"], probe.legacyHeader)
			assert.True(t, probe.downstreamOK)
		})
	}
}

func TestSelectVideoTierGroupResolvedEndpointRequestWins(t *testing.T) {
	setupVideoTierGroupDefaults(t)

	t.Run("resolved suffix group", func(t *testing.T) {
		// PinTaskPluginEndpoint already folded the suffix into the endpoint
		// request and may have rewritten the body model; the resolved group must
		// still block auto-selection.
		probe := runSelectVideoTierGroup(t,
			`{"model":"wan-3.0","resolution":"1080p","prompt":"cat"}`,
			nil,
			ModelRequest{Model: "wan-3.0", Group: "wan-3.0-720p"})
		assert.Equal(t, "default", probe.usingGroup)
	})

	t.Run("resolved model only", func(t *testing.T) {
		probe := runSelectVideoTierGroup(t,
			`{"model":"wan-3.0","resolution":"1080p","prompt":"cat"}`,
			nil,
			ModelRequest{Model: "wan-3.0"})
		assert.Equal(t, "wan-3.0-1080p", probe.usingGroup)
	})

	t.Run("resolved task model wins over body", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.Use(func(c *gin.Context) {
			common.SetContextKey(c, constant.ContextKeyUserGroup, "default")
			common.SetContextKey(c, constant.ContextKeyUsingGroup, "default")
			common.SetContextKey(c, constant.ContextKeyTokenGroup, "default")
			c.Set("resolved_task_model", "wan-3.0")
			c.Next()
		})
		router.Use(SelectVideoTierGroup())
		var group string
		router.POST("/v1/video/generations", func(c *gin.Context) {
			group = common.GetContextKeyString(c, constant.ContextKeyUsingGroup)
			c.Status(http.StatusOK)
		})
		request := httptest.NewRequest(http.MethodPost, "/v1/video/generations",
			bytes.NewReader([]byte(`{"model":"wan-3.0","resolution":"720p"}`)))
		request.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, "wan-3.0-720p", group)
	})
}

func TestSelectVideoTierGroupLeavesUndeclaredModelsAlone(t *testing.T) {
	setupVideoTierGroupDefaults(t)
	bodies := []string{
		`{"model":"seedance-2-5","resolution":"720p","prompt":"cat"}`,
		`{"model":"wan-3.0-prime","resolution":"720p","prompt":"cat"}`,
		`{"model":"wan-3.0-ref","resolution":"1080p","prompt":"cat"}`,
		`{"model":"kling-v3","resolution":"1080p","prompt":"cat"}`,
		`{"resolution":"720p","prompt":"cat"}`,
		// A model with no declared tiers is never rejected for a missing
		// resolution: the tier requirement belongs to the tier table alone.
		`{"model":"seedance-2-5","prompt":"cat"}`,
		`{"model":"sora-2","prompt":"cat"}`,
		`{"model":"wan-3.0-prime","resolution":"2K","prompt":"cat"}`,
	}
	for _, body := range bodies {
		t.Run(body, func(t *testing.T) {
			probe := runSelectVideoTierGroup(t, body, nil, nil)
			assert.Equal(t, http.StatusOK, probe.status, "an undeclared model must keep its existing behavior")
			assert.Equal(t, "default", probe.usingGroup)
			assert.Equal(t, "default", probe.tokenGroup)
			assert.Empty(t, probe.header)
			assert.True(t, probe.downstreamOK)
		})
	}
}

// The resolution picks the tier, so for a declared model it is required: a
// request that names no group and declares no readable quality must be told
// that, instead of falling through to a group that cannot serve the model.
func TestSelectVideoTierGroupRequiresResolutionForTierModels(t *testing.T) {
	setupVideoTierGroupDefaults(t)
	bodies := []string{
		`{"model":"wan-3.0","prompt":"cat"}`,
		`{"model":"wan-3.0","resolution":"","prompt":"cat"}`,
		`{"model":"wan-3.0","resolution":"480p","prompt":"cat"}`,
		`{"model":"wan-3.0","resolution":"768p","prompt":"cat"}`,
		`{"model":"wan-3.0","resolution":"2K","prompt":"cat"}`,
		`{"model":"wan-3.0","resolution":"4k","prompt":"cat"}`,
		`{"model":"wan-3.0","size":"16:9","prompt":"cat"}`,
		`{"model":"wan-3.0","size":"1280","prompt":"cat"}`,
		`{"model":"wan-3.0","resolution":"1080p24","prompt":"cat"}`,
		`{"model":"wan-3.0","resolution":"auto","prompt":"cat"}`,
		`{"model":"wan-3.0","resolution":null,"prompt":"cat"}`,
	}
	for _, body := range bodies {
		t.Run(body, func(t *testing.T) {
			probe := runSelectVideoTierGroup(t, body, nil, nil)
			assert.Equal(t, http.StatusBadRequest, probe.status, "a declared tier model without a readable quality must be rejected")
			assert.Contains(t, probe.body, "resolution is required for wan-3.0")
			// The body is JSON, so the message quotes arrive escaped; assert on
			// the actionable content rather than on the escaping.
			assert.Contains(t, probe.body, "720p")
			assert.Contains(t, probe.body, "1080p")
			assert.Contains(t, probe.body, "1280x720")
			assert.Contains(t, probe.body, "1920x1080")
			assert.Contains(t, probe.body, "select a group explicitly")
			// The rejection happens before the route handler and before any group
			// is pinned, so the request is never routed on a fallback group.
			assert.Empty(t, probe.usingGroup)
			assert.Empty(t, probe.tokenGroup)
			assert.Empty(t, probe.header)
			assert.False(t, probe.downstreamOK)
		})
	}
}

// An explicit group is the documented escape hatch: it is never rejected for a
// missing resolution, and it still wins over the automatic tier.
func TestSelectVideoTierGroupExplicitGroupWithoutResolution(t *testing.T) {
	setupVideoTierGroupDefaults(t)
	body := `{"model":"wan-3.0","prompt":"cat"}`
	for _, testCase := range []struct {
		name    string
		body    string
		headers map[string]string
	}{
		{name: "X-Omnimux-Group header", body: body, headers: map[string]string{"X-Omnimux-Group": "default"}},
		{name: "legacy New-Api-Group header", body: body, headers: map[string]string{"New-Api-Group": "default"}},
		{name: "body group field", body: `{"model":"wan-3.0","group":"default","prompt":"cat"}`},
		{name: "model@group suffix", body: `{"model":"wan-3.0@default","prompt":"cat"}`},
		{name: "explicit tier group", body: body, headers: map[string]string{"X-Omnimux-Group": "wan-3.0-1080p"}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			probe := runSelectVideoTierGroup(t, testCase.body, testCase.headers, nil)
			assert.Equal(t, http.StatusOK, probe.status, "an explicit group must never be rejected for a missing resolution")
			assert.NotContains(t, probe.body, "resolution is required")
			assert.True(t, probe.downstreamOK)
			// The middleware leaves the caller's choice untouched; Distribute()
			// applies it. It never substitutes the automatic tier for it.
			assert.Equal(t, testCase.headers["X-Omnimux-Group"], probe.header)
			assert.Equal(t, "default", probe.usingGroup)
			assert.Equal(t, "default", probe.tokenGroup)
		})
	}
}

func TestSelectVideoTierGroupMalformedBodyFallsBack(t *testing.T) {
	setupVideoTierGroupDefaults(t)
	for _, body := range []string{`{"model":"wan-3.0",`, ``, `not json`} {
		t.Run(body, func(t *testing.T) {
			probe := runSelectVideoTierGroup(t, body, nil, nil)
			// A broken body is not a missing resolution: downstream keeps its
			// own invalid-request error.
			assert.Equal(t, http.StatusOK, probe.status)
			assert.NotContains(t, probe.body, "resolution is required")
			assert.Equal(t, "default", probe.usingGroup)
			assert.Empty(t, probe.header)
		})
	}
}

func TestSelectVideoTierGroupFallsBackWhenGroupIsNotProvisioned(t *testing.T) {
	// The tier groups are not configured yet: the request must keep its existing
	// group instead of failing authorization downstream.
	setupVideoTierGroupSettings(t, `{"default":"默认"}`, `{"default":1}`)
	probe := runSelectVideoTierGroup(t, `{"model":"wan-3.0","resolution":"720p","prompt":"cat"}`, nil, nil)
	assert.Equal(t, "default", probe.usingGroup)
	assert.Empty(t, probe.header)

	// A group ratio without the account-visible entry is still not selectable.
	setupVideoTierGroupSettings(t, `{"default":"默认"}`, `{"default":1,"wan-3.0-720p":1.2}`)
	probe = runSelectVideoTierGroup(t, `{"model":"wan-3.0","resolution":"720p","prompt":"cat"}`, nil, nil)
	assert.Equal(t, "default", probe.usingGroup)
}

// videoTierCountingBody fails every read and counts the attempts. It is how
// these tests observe that the middleware did or did not touch the request body:
// common.UnmarshalBodyReusable decodes the whole payload on each call, so a
// request that can never need the body must not pay for it.
type videoTierCountingBody struct{ reads int }

func (b *videoTierCountingBody) Read([]byte) (int, error) {
	b.reads++
	return 0, errors.New("the body must not be read")
}

func (b *videoTierCountingBody) Close() error { return nil }

// A model the context already resolved and that declares no tier is never
// pinned, so the body is not read at all. This is the production case: almost
// every video request names a model without tiers.
func TestSelectVideoTierGroupSkipsBodyForUndeclaredModel(t *testing.T) {
	setupVideoTierGroupDefaults(t)
	for _, modelName := range []string{"seedance-2-5", "sora-2", "kling-v1-6", "gmx-video"} {
		t.Run(modelName, func(t *testing.T) {
			body := &videoTierCountingBody{}
			probe := runSelectVideoTierGroupWithCounter(t, body, nil, ModelRequest{Model: modelName}, body)
			assert.Equal(t, http.StatusOK, probe.status)
			assert.Equal(t, "default", probe.usingGroup, "an undeclared model is never pinned")
			assert.Empty(t, probe.header)
			assert.Equal(t, 0, probe.middlewareReads, "an undeclared model must not read the body")
			assert.Greater(t, body.reads, 0, "the downstream handler does read it, so the assertion above is not vacuous")
		})
	}
}

// The same holds when the context already carries an explicit choice: a header,
// an endpoint group or a model@group suffix is decided without the body.
func TestSelectVideoTierGroupSkipsBodyForContextualExplicitChoice(t *testing.T) {
	setupVideoTierGroupDefaults(t)
	for _, testCase := range []struct {
		name    string
		headers map[string]string
		cached  any
	}{
		{name: "X-Omnimux-Group header", headers: map[string]string{"X-Omnimux-Group": "default"}},
		{name: "legacy header", headers: map[string]string{"New-Api-Group": "default"}},
		{name: "endpoint group", cached: ModelRequest{Model: "wan-3.0", Group: "default"}},
		{name: "endpoint model suffix", cached: ModelRequest{Model: "wan-3.0@default"}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			body := &videoTierCountingBody{}
			probe := runSelectVideoTierGroupWithCounter(t, body, testCase.headers, testCase.cached, body)
			assert.Equal(t, http.StatusOK, probe.status)
			assert.Equal(t, "default", probe.usingGroup)
			assert.Equal(t, 0, probe.middlewareReads, "an explicit choice is decided without the body")
			assert.Greater(t, body.reads, 0, "the downstream handler does read it, so the assertion above is not vacuous")
		})
	}
}

// The skip is narrow: without a context model the body is still the only source
// of the model, and a declared tier still needs it for the resolution. Both
// cases must read, which is what makes the assertions above non-vacuous.
func TestSelectVideoTierGroupStillReadsBodyWhenNeeded(t *testing.T) {
	setupVideoTierGroupDefaults(t)

	t.Run("no context model", func(t *testing.T) {
		body := &videoTierCountingBody{}
		probe := runSelectVideoTierGroupWithCounter(t, body, nil, nil, body)
		assert.Greater(t, probe.middlewareReads, 0, "the body is the only source of the model here")
		assert.Equal(t, http.StatusOK, probe.status)
		assert.Equal(t, "default", probe.usingGroup)
	})

	t.Run("declared tier model", func(t *testing.T) {
		body := &videoTierCountingBody{}
		probe := runSelectVideoTierGroupWithCounter(t, body, nil, ModelRequest{Model: "wan-3.0"}, body)
		assert.Greater(t, probe.middlewareReads, 0, "a declared tier needs the body for its resolution")
		assert.Equal(t, http.StatusOK, probe.status, "an unreadable body is not a missing resolution")
	})
}

func TestVideoTierFromResolution(t *testing.T) {
	cases := []struct {
		resolution any
		size       any
		want       string
	}{
		{"720p", nil, "720p"},
		{"720P", nil, "720p"},
		{"1080p", nil, "1080p"},
		{"1080P", nil, "1080p"},
		{"1280x720", nil, "720p"},
		{"1920x1080", nil, "1080p"},
		{"720x1280", nil, "720p"},
		{"1080x1920", nil, "1080p"},
		{"1280×720", nil, "720p"},
		{"1280*720", nil, "720p"},
		{nil, "1280x720", "720p"},
		{nil, "1920x1080", "1080p"},
		{" 720p ", nil, "720p"},
		{float64(720), nil, "720p"},
		{float64(1080), nil, "1080p"},
		{720, nil, "720p"},
		{"720", nil, "720p"},
		{"1080", nil, "1080p"},
		{"480p", nil, ""},
		{"768p", nil, ""},
		{"2K", nil, ""},
		{"4k", nil, ""},
		{"16:9", nil, ""},
		{"", nil, ""},
		{nil, nil, ""},
		{"1080p24", nil, ""},
		{"1280x", nil, ""},
		{"x720", nil, ""},
		{"0x0", nil, ""},
		{true, nil, ""},
		{map[string]any{"height": 720}, nil, ""},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("%v/%v", tc.resolution, tc.size), func(t *testing.T) {
			got, ok := videoTierFromResolution(tc.resolution, tc.size)
			if tc.want == "" {
				assert.False(t, ok)
				assert.Empty(t, got)
				return
			}
			assert.True(t, ok)
			assert.Equal(t, tc.want, got)
		})
	}
}

// TestSelectVideoTierGroupDrivesChannelSelection proves the group resolved
// before Distribute() actually selects the tier's channel, so routing and the
// billing group cannot drift apart.
func TestSelectVideoTierGroupDrivesChannelSelection(t *testing.T) {
	setupVideoTierGroupDefaults(t)
	setupVideoTierChannelDB(t)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		common.SetContextKey(c, constant.ContextKeyUserId, 1)
		common.SetContextKey(c, constant.ContextKeyUserGroup, "default")
		common.SetContextKey(c, constant.ContextKeyUsingGroup, "default")
		common.SetContextKey(c, constant.ContextKeyTokenGroup, "default")
		c.Next()
	})
	router.Use(SelectVideoTierGroup())
	router.Use(Distribute())

	var selectedChannel int
	var usingGroup string
	var tokenGroup string
	router.POST("/v1/video/generations", func(c *gin.Context) {
		selectedChannel = common.GetContextKeyInt(c, constant.ContextKeyChannelId)
		usingGroup = common.GetContextKeyString(c, constant.ContextKeyUsingGroup)
		tokenGroup = common.GetContextKeyString(c, constant.ContextKeyTokenGroup)
		c.Status(http.StatusOK)
	})

	cases := []struct {
		name        string
		body        string
		wantGroup   string
		wantChannel int
	}{
		{"720p", `{"model":"wan-3.0","resolution":"720p","prompt":"cat"}`, "wan-3.0-720p", 71},
		{"1080p", `{"model":"wan-3.0","resolution":"1080p","prompt":"cat"}`, "wan-3.0-1080p", 72},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/v1/video/generations", bytes.NewReader([]byte(tc.body)))
			request.Header.Set("Content-Type", "application/json")
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)
			require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
			assert.Equal(t, tc.wantGroup, usingGroup)
			assert.Equal(t, tc.wantGroup, tokenGroup)
			assert.Equal(t, tc.wantChannel, selectedChannel, "the tier group must select the tier's channel")
		})
	}
}

func setupVideoTierChannelDB(t *testing.T) {
	t.Helper()
	_ = i18n.Init()
	originalDB := model.DB
	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	originalRetryTimes := common.RetryTimes

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Ability{}))
	model.DB = db
	common.MemoryCacheEnabled = true
	common.RetryTimes = 0

	priority := int64(0)
	weight := uint(100)
	for _, tier := range []struct {
		id    int
		group string
	}{
		{71, "wan-3.0-720p"},
		{72, "wan-3.0-1080p"},
	} {
		require.NoError(t, db.Create(&model.Channel{
			Id: tier.id, Type: constant.ChannelTypeOpenAI, Key: fmt.Sprintf("k%d", tier.id), Status: common.ChannelStatusEnabled,
			Name: "wan-3.0 " + tier.group, Weight: &weight, Models: "wan-3.0", Group: tier.group, Priority: &priority,
		}).Error)
		require.NoError(t, db.Create(&model.Ability{
			Group: tier.group, Model: "wan-3.0", ChannelId: tier.id, Enabled: true, Priority: &priority, Weight: weight,
		}).Error)
	}
	model.InitChannelCache()

	t.Cleanup(func() {
		model.DB = originalDB
		common.MemoryCacheEnabled = originalMemoryCacheEnabled
		common.RetryTimes = originalRetryTimes
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
}
