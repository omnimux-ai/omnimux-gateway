package fal

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHubToFalFixtures runs the real Hub guard and mapper from a supplied checkout.
func TestHubToFalFixtures(t *testing.T) {
	root := os.Getenv("OMNIMUX_DSH_FIXTURE_ROOT")
	if root == "" {
		t.Skip("set OMNIMUX_DSH_FIXTURE_ROOT to the isolated plugin checkout")
	}
	output, err := exec.Command("node", filepath.Join(root, "scripts/h3-cross-repo-fixtures.mjs")).CombinedOutput()
	require.NoError(t, err, string(output))
	var fixtures []struct {
		Name string         `json:"name"`
		Body map[string]any `json:"body"`
	}
	require.NoError(t, common.Unmarshal(output, &fixtures))
	require.Len(t, fixtures, 3)
	for _, fixture := range fixtures {
		t.Run(fixture.Name, func(t *testing.T) {
			data, err := common.Marshal(fixture.Body)
			require.NoError(t, err)
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/video/generations", bytes.NewReader(data))
			c.Request.Header.Set("Content-Type", "application/json")
			modelName, ok := fixture.Body["model"].(string)
			require.True(t, ok)
			info := &relaycommon.RelayInfo{OriginModelName: modelName}
			adaptor := &TaskAdaptor{}
			require.Nil(t, adaptor.ValidateRequestAndSetAction(c, info))
			endpoint := ModelMinimaxH3MaxImageToVideo
			if fixture.Name == "turbo-first" {
				endpoint = ModelMinimaxH3MaxTurboImageToVideo
			}
			if fixture.Name == "max-references" {
				endpoint = ModelMinimaxH3MaxReferenceToVideo
			}
			url, err := adaptor.BuildRequestURL(info)
			require.NoError(t, err)
			assert.Equal(t, "https://queue.fal.run/"+endpoint, url)
			reader, err := adaptor.BuildRequestBody(c, info)
			require.NoError(t, err)
			body, err := io.ReadAll(reader)
			require.NoError(t, err)
			expected := map[string]any{"prompt": "fixture", "duration": "5", "resolution": "768P", "aspect_ratio": "16:9", "prompt_optimizer": true}
			if fixture.Name == "max-references" {
				expected["reference_image_urls"] = []string{"https://example.invalid/second.png", "https://example.invalid/first.png"}
				expected["reference_video_urls"] = []string{"https://example.invalid/second.mp4", "https://example.invalid/first.mp4"}
			} else {
				expected["image_url"] = "https://example.invalid/first.png"
			}
			want, err := common.Marshal(expected)
			require.NoError(t, err)
			assert.JSONEq(t, string(want), string(body))
		})
	}
}
