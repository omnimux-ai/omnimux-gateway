package controller

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInfluencerPublishedReferenceSourcesMatchFrontendCatalog(t *testing.T) {
	referenceInspectionTestDB(t)
	data, err := os.ReadFile("../web/src/features/influencer/data/motion-explore.json")
	require.NoError(t, err)
	var catalog struct {
		Presets []struct {
			ReferenceVideoURL string `json:"referenceVideoUrl"`
		} `json:"presets"`
		Community []struct {
			ReferenceVideoURL string `json:"referenceVideoUrl"`
		} `json:"community"`
	}
	require.NoError(t, common.Unmarshal(data, &catalog))
	expected := make(map[string]struct{})
	for _, item := range append(catalog.Presets, catalog.Community...) {
		if item.ReferenceVideoURL != "" {
			expected[item.ReferenceVideoURL] = struct{}{}
		}
	}
	require.NotEmpty(t, expected, "the shipped catalog must provide full-quality reference sources")

	// Read string cases as syntax, without coupling the catalog gate to generated
	// whitespace. This also rejects stale extra URLs after a preset is removed.
	generated, err := parser.ParseFile(token.NewFileSet(), "influencer_preset_sources.gen.go", nil, 0)
	require.NoError(t, err)
	listed := make(map[string]struct{})
	for _, declaration := range generated.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Name.Name != "isPublishedInfluencerVideoReference" {
			continue
		}
		ast.Inspect(function.Body, func(node ast.Node) bool {
			clause, ok := node.(*ast.CaseClause)
			if !ok {
				return true
			}
			for _, expression := range clause.List {
				literal, ok := expression.(*ast.BasicLit)
				require.True(t, ok, "published sources must be explicit URL literals")
				require.Equal(t, token.STRING, literal.Kind)
				raw, err := strconv.Unquote(literal.Value)
				require.NoError(t, err)
				listed[raw] = struct{}{}
			}
			return false
		})
	}
	assert.Equal(t, expected, listed, "all and only shipped high-quality reference URLs must be admitted")

	for raw := range expected {
		t.Run(raw, func(t *testing.T) {
			assert.True(t, isPublishedInfluencerVideoReference(raw))
			allowed, err := influencerVideoReferenceSource(context.Background(), raw, 41)
			require.NoError(t, err)
			assert.True(t, allowed, "a published high-quality reference must pass the preflight source gate")

			parsed, err := url.Parse(raw)
			require.NoError(t, err)
			neighbor := *parsed
			neighbor.Path = path.Join(path.Dir(parsed.Path), "unpublished-reference-video.mp4")
			for _, unlisted := range []string{neighbor.String(), raw + "?download=1", raw + "#preview"} {
				assert.False(t, isPublishedInfluencerVideoReference(unlisted), unlisted)
				allowed, err := influencerVideoReferenceSource(context.Background(), unlisted, 41)
				require.NoError(t, err)
				assert.False(t, allowed, "sharing a CDN origin or directory grants no download authority: %s", unlisted)
			}
		})
	}
}

func TestInspectInfluencerMotionVideoAdmitsExactPublishedHDSourceOnly(t *testing.T) {
	referenceInspectionTestDB(t)
	const published = "https://cdn.higgsfield.ai/higgsfield_multiplier_video_explore_variant/55e656fb-4b00-5b62-bc1f-8098221211a8.mp4"
	previousInspector := influencerVideoReferenceInspector
	previousFetch := *system_setting.GetFetchSetting()
	previousAddress, previousTrusted := system_setting.ServerAddress, common.SessionCookieTrustedURLs
	t.Cleanup(func() {
		influencerVideoReferenceInspector = previousInspector
		*system_setting.GetFetchSetting() = previousFetch
		system_setting.ServerAddress, common.SessionCookieTrustedURLs = previousAddress, previousTrusted
	})
	// The source-gate test makes no outbound request or DNS lookup. Existing
	// dedicated media tests cover DNS and connection-level private-IP protection.
	system_setting.GetFetchSetting().ApplyIPFilterForDomain = false
	system_setting.ServerAddress = "https://app.example"
	common.SessionCookieTrustedURLs = nil
	calls := 0
	influencerVideoReferenceInspector = func(_ context.Context, raw, mode string) (service.InfluencerVideoMetadata, error) {
		calls++
		assert.Equal(t, published, raw)
		assert.Equal(t, "replace", mode)
		return service.InfluencerVideoMetadata{Duration: 9.708, Width: 1610, Height: 1288, FrameRate: 24, Container: "mp4", VideoCodec: "hevc"}, nil
	}
	engine := gin.New()
	engine.POST("/inspect", func(c *gin.Context) { c.Set("id", 41) }, InspectInfluencerMotionVideo)
	for _, tc := range []struct {
		name, raw string
		status    int
	}{
		{"published-source", published, http.StatusOK},
		{"same-cdn-unpublished-neighbor", "https://cdn.higgsfield.ai/higgsfield_multiplier_video_explore_variant/unpublished-reference-video.mp4", http.StatusForbidden},
		{"query-does-not-create-another-published-source", published + "?download=1", http.StatusForbidden},
	} {
		t.Run(tc.name, func(t *testing.T) {
			payload, err := common.Marshal(map[string]any{"video_url": tc.raw, "mode": "replace"})
			require.NoError(t, err)
			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/inspect", strings.NewReader(string(payload)))
			request.Header.Set("Content-Type", "application/json")
			engine.ServeHTTP(response, request)
			assert.Equal(t, tc.status, response.Code, response.Body.String())
		})
	}
	assert.Equal(t, 1, calls, "unpublished neighboring URLs must stop before media inspection")
}
