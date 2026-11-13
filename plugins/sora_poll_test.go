package plugins_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/QuantumNous/new-api/pkg/jsplugin"
	builtinplugins "github.com/QuantumNous/new-api/plugins"
	"github.com/stretchr/testify/require"
)

// TestSoraImagePollRespondsToUpstreamEnvelopes replays the recorded upstream poll
// shapes for async image models through the embedded sora plugin.
//
// Regression: APIMart nests status/progress/result one level down
// ({"code":200,"data":{…}}), while the parser used to read body.status only. Every
// APIMart image task therefore failed the poll with "unrecognized status: " — and
// because the same parser is reached for nano-banana-pro / gpt-image-2.5 on
// OpenAI-type channels, the whole APIMart image line silently delivered nothing.
func TestSoraImagePollRespondsToUpstreamEnvelopes(t *testing.T) {
	source, err := builtinplugins.Source("sora")
	require.NoError(t, err)

	fixturePath := filepath.Join("tasks", "sora", "fixtures", "poll.json")
	fixture, err := os.ReadFile(fixturePath)
	require.NoErrorf(t, err, "read fixture %s", fixturePath)

	report, err := jsplugin.ReplayFixture(context.Background(), source, fixture)
	require.NoError(t, err)
	require.Equal(t, report.Total, report.Passed, "all sora poll fixture cases must pass")
	require.NotZero(t, report.Total)
}
