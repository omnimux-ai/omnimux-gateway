package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBrowserVideoPreparationFreezesDeclaredMediaWithoutProbe(t *testing.T) {
	for _, withAudio := range []bool{false, true} {
		t.Run(map[bool]string{false: "no-audio", true: "with-audio"}[withAudio], func(t *testing.T) {
			original := []byte("browser-declared-media-is-not-server-inspected")
			if withAudio {
				original = frozenRealVideo(t, "4", true)
			}
			driver, source, gets := frozenTestEnvironment(t, original)
			oldProbe := influencerVideoPreparationProbe
			t.Cleanup(func() { influencerVideoPreparationProbe = oldProbe })
			probes := 0
			influencerVideoPreparationProbe = func(context.Context, string) (InfluencerVideoMetadata, error) {
				probes++
				return InfluencerVideoMetadata{}, errors.New("unexpected server inspection")
			}
			declared := InfluencerVideoMetadata{Duration: 4, VideoDuration: 4, Width: 720, Height: 1280, FrameRate: 24, VideoCodec: "h264", Container: "mp4", SizeBytes: int64(len(original))}
			if withAudio {
				declared.AudioCodec = "aac"
			}
			prepared, err := InfluencerBrowserVideoPreparer(context.Background(), source, 41, declared, frozenPublicURL)
			require.NoError(t, err)
			assert.Zero(t, probes, "browser workbench must not run server media inspection")
			assert.Equal(t, int32(1), gets.Load(), "one read is retained solely for the immutable copy")
			assert.Equal(t, declared, prepared.Metadata, "client declarations must not be relabelled as server observations")
			assert.Equal(t, original, driver.objects[strings.TrimPrefix(prepared.URL, "https://files.example/")])
			assert.Len(t, driver.objects, 1, "the frozen video is the only stored reference; no separate audio attachment")
			prepared.Discard()
			assert.Empty(t, driver.objects)
		})
	}
}

func TestBrowserVideoPCMDeclarationMatchesMOVContainerOnly(t *testing.T) {
	for _, container := range []string{"mp4", "mov"} {
		t.Run(container, func(t *testing.T) {
			metadata := InfluencerVideoMetadata{Duration: 4, VideoDuration: 4, Width: 720, Height: 1280, FrameRate: 24, VideoCodec: "h264", AudioCodec: "pcm", Container: container, SizeBytes: 1234}
			err := ValidateInfluencerVideoReferenceMetadata(metadata, "motion")
			if container == "mov" {
				require.NoError(t, err, "browser parser uses the documented pcm family label")
			} else {
				require.Error(t, err, "MP4 must retain its existing audio codec restriction")
			}
		})
	}
}
