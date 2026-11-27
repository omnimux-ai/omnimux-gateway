package service

import (
	"context"
	"encoding/binary"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProbeInfluencerVideoFullCapacity(t *testing.T) {
	path := filepath.Join(t.TempDir(), "reference.mp4")
	runInfluencerFormatFFmpeg(t, "-f", "lavfi", "-i", "color=c=black:s=1000x800:r=24", "-t", "2", "-an", "-c:v", "libx264", "-threads", "1", path)
	file, err := os.OpenFile(path, os.O_RDWR, 0600)
	require.NoError(t, err)
	info, err := file.Stat()
	require.NoError(t, err)
	_, err = file.Seek(info.Size(), 0)
	require.NoError(t, err)
	// A legal free box fills the container without inventing additional video frames.
	var header [8]byte
	binary.BigEndian.PutUint32(header[:4], uint32(InfluencerVideoMaxBytes-info.Size()))
	copy(header[4:], "free")
	_, err = file.Write(header[:])
	require.NoError(t, err)
	require.NoError(t, file.Truncate(InfluencerVideoMaxBytes))
	require.NoError(t, file.Close())
	metadata, err := ProbeInfluencerVideoFile(context.Background(), path)
	require.NoError(t, err, "a complete 200MiB input must not fail at the old 100MiB probe/read-ahead ceiling")
	assert.Equal(t, int64(InfluencerVideoMaxBytes), metadata.SizeBytes)
	assert.Equal(t, 2.0, metadata.Duration)
	require.NoError(t, os.Truncate(path, InfluencerVideoMaxBytes+1))
	_, err = ProbeInfluencerVideoFile(context.Background(), path)
	require.Error(t, err, "actual bytes over the contract must still fail before media decoding")
}

func TestInspectInfluencerVideoReferenceHasNoStorageSideEffects(t *testing.T) {
	path := filepath.Join(t.TempDir(), "reference.mov")
	runInfluencerFormatFFmpeg(t, "-f", "lavfi", "-i", "color=c=black:s=1000x800:r=24", "-t", "2", "-an", "-c:v", "libx264", "-threads", "1", path)
	original, err := os.ReadFile(path)
	require.NoError(t, err)
	for _, mode := range []string{"motion", "replace"} {
		t.Run(mode, func(t *testing.T) {
			driver, source, gets := frozenTestEnvironment(t, original)
			temporary := t.TempDir()
			t.Setenv("TMPDIR", temporary)
			metadata, err := InspectInfluencerVideoReference(context.Background(), source, mode)
			if mode == "replace" {
				require.ErrorContains(t, err, "4至30秒")
				assert.Equal(t, http.StatusBadRequest, InfluencerVideoInspectionStatus(err))
			} else {
				require.NoError(t, err)
				assert.Equal(t, 2.0, metadata.Duration)
				assert.Equal(t, 1000, metadata.Width)
				assert.Equal(t, 800, metadata.Height)
				assert.Equal(t, "mov", metadata.Container, "real container determines acceptance regardless of URL suffix")
				assert.Equal(t, int64(len(original)), metadata.SizeBytes)
			}
			assert.Equal(t, int32(1), gets.Load(), "preflight checks one actual source copy")
			assert.Empty(t, driver.objects, "preflight must never publish objects")
			var count int64
			require.NoError(t, model.DB.Model(&model.File{}).Count(&count).Error)
			assert.Zero(t, count, "preflight must not reserve file records")
			remaining, err := os.ReadDir(temporary)
			require.NoError(t, err)
			assert.Empty(t, remaining, "temporary source must be removed before response")
		})
	}
}

func TestInspectInfluencerVideoReferenceRejectsWithoutLeakingResources(t *testing.T) {
	t.Run("bad media", func(t *testing.T) {
		driver, source, _ := frozenTestEnvironment(t, []byte("invalid video"))
		temporary := t.TempDir()
		t.Setenv("TMPDIR", temporary)
		_, err := InspectInfluencerVideoReference(context.Background(), source, "motion")
		require.Error(t, err)
		assert.Equal(t, http.StatusBadRequest, InfluencerVideoInspectionStatus(err))
		assert.Empty(t, driver.objects)
		remaining, err := os.ReadDir(temporary)
		require.NoError(t, err)
		assert.Empty(t, remaining)
		assert.Empty(t, influencerVideoProbeSlots)
	})
	t.Run("cancelled request", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := InspectInfluencerVideoReference(ctx, "https://files.example/reference.mp4", "motion")
		require.ErrorIs(t, err, context.Canceled)
		assert.Equal(t, http.StatusRequestTimeout, InfluencerVideoInspectionStatus(err))
	})
	t.Run("busy check can retry", func(t *testing.T) {
		for range cap(influencerVideoProbeSlots) {
			influencerVideoProbeSlots <- struct{}{}
		}
		t.Cleanup(func() {
			for range cap(influencerVideoProbeSlots) {
				<-influencerVideoProbeSlots
			}
		})
		_, err := InspectInfluencerVideoReference(context.Background(), "https://files.example/reference.mp4", "motion")
		require.ErrorIs(t, err, ErrInfluencerVideoBusy)
		assert.Equal(t, http.StatusServiceUnavailable, InfluencerVideoInspectionStatus(err))
	})
}
