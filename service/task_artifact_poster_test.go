package service

import (
	"bytes"
	"image/jpeg"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVideoPosterDecodesMP4WithTrailingIndex(t *testing.T) {
	path := filepath.Join(t.TempDir(), "trailing-index.mp4")
	output, err := exec.Command("ffmpeg", "-nostdin", "-v", "error", "-f", "lavfi", "-i", "testsrc2=s=480x854:r=24", "-t", "6", "-c:v", "libx264", "-threads", "1", path).CombinedOutput()
	require.NoError(t, err, string(output))
	video, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Greater(t, bytes.Index(video, []byte("moov")), bytes.Index(video, []byte("mdat")), "fixture must require seeking back after the trailing index")
	frame, err := extractVideoPosterFrame(t.Context(), video)
	require.NoError(t, err)
	config, err := jpeg.DecodeConfig(bytes.NewReader(frame))
	require.NoError(t, err)
	assert.Equal(t, 480, config.Width)
	assert.Equal(t, 854, config.Height)
}
