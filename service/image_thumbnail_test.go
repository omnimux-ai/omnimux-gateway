package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os/exec"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func thumbnailPNG(t *testing.T, width, height int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	img.SetNRGBA(0, 0, color.NRGBA{R: 255, A: 255})
	var out bytes.Buffer
	require.NoError(t, png.Encode(&out, img))
	return out.Bytes()
}

func TestImageThumbnailInputBoundaries(t *testing.T) {
	// A GIF header declares oversized dimensions without allocating pixels.
	oversizedPixels := append([]byte("GIF89a"), []byte{255, 255, 255, 255, 0, 0, 0}...)
	for _, tc := range []struct {
		name string
		data []byte
	}{
		{"empty", nil}, {"corrupt", []byte("not an image")},
		{"bytes", make([]byte, MaxTaskArtifactPersistBytes+1)}, {"pixels", oversizedPixels},
	} {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			_, err := generateImageThumbnail(context.Background(), tc.data, func(context.Context, []byte) ([]byte, error) { called = true; return nil, nil })
			require.Error(t, err)
			assert.False(t, called, "unsafe input must not reach decoder")
		})
	}
}

func TestImageThumbnailEncoderFailuresAndCancellation(t *testing.T) {
	data := thumbnailPNG(t, 8, 12)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	called := false
	_, err := generateImageThumbnail(ctx, data, func(context.Context, []byte) ([]byte, error) { called = true; return nil, nil })
	require.ErrorIs(t, err, context.Canceled)
	assert.False(t, called)
	expected := errors.New("encoder failed")
	_, err = generateImageThumbnail(context.Background(), data, func(context.Context, []byte) ([]byte, error) { return nil, expected })
	require.ErrorIs(t, err, expected)
	for _, output := range [][]byte{data, make([]byte, ImageThumbnailMaxBytes+1)} {
		_, err = generateImageThumbnail(context.Background(), data, func(context.Context, []byte) ([]byte, error) { return output, nil })
		require.Error(t, err)
	}
}

func TestImageThumbnailDimensionsAndSourceUnchanged(t *testing.T) {
	source := thumbnailPNG(t, 8, 12)
	original := bytes.Clone(source)
	webp, err := base64.StdEncoding.DecodeString("UklGRmAAAABXRUJQVlA4WAoAAAAQAAAABwAACwAAQUxQSAkAAAABBxAREYiI/gcAVlA4IDAAAADQAQCdASoIAAwAAUAmJaACdLoB+AADsAD+8ut//NgVzXPv9//S4P0uD9Lg/9KQAAA=")
	require.NoError(t, err)
	result, err := generateImageThumbnail(context.Background(), source, func(ctx context.Context, input []byte) ([]byte, error) {
		_, ok := ctx.Deadline()
		assert.True(t, ok)
		assert.Equal(t, source, input)
		return webp, nil
	})
	require.NoError(t, err)
	assert.Equal(t, 8, result.Width)
	assert.Equal(t, 12, result.Height)
	assert.Equal(t, original, source)
	key := TaskImageThumbnailKey("image-1")
	assert.True(t, IsTaskImageThumbnailKey(key))
	assert.NotEqual(t, key, TaskImageThumbnailKey("image-2"))
	assert.False(t, IsTaskImageThumbnailKey("image-1"))
}

func TestImageThumbnailRealEncoding(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg executable absent")
	}
	for _, tc := range []struct {
		name               string
		w, h, wantW, wantH int
	}{
		{"portrait", 940, 1672, 640, 1138}, {"landscape", 1280, 720, 640, 360}, {"small transparent", 8, 12, 8, 12},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := thumbnailPNG(t, tc.w, tc.h)
			original := bytes.Clone(source)
			result, err := GenerateImageThumbnail(context.Background(), source)
			require.NoError(t, err, "runtime must provide ffmpeg libwebp encoder")
			assert.Equal(t, tc.wantW, result.Width)
			assert.Equal(t, tc.wantH, result.Height)
			assert.Equal(t, original, source)
			decoded, _, err := image.Decode(bytes.NewReader(result.Data))
			require.NoError(t, err)
			_, _, _, alpha := decoded.At(result.Width-1, result.Height-1).RGBA()
			assert.Zero(t, alpha, "transparent background must survive")
		})
	}
}

func TestImageThumbnailRealEXIFOrientation(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg executable absent")
	}
	var source bytes.Buffer
	require.NoError(t, jpeg.Encode(&source, image.NewRGBA(image.Rect(0, 0, 12, 8)), nil))
	// APP1 EXIF little-endian TIFF orientation=6 (90 degrees clockwise).
	exif := []byte{0xff, 0xe1, 0, 34, 'E', 'x', 'i', 'f', 0, 0, 'I', 'I', 42, 0, 8, 0, 0, 0, 1, 0, 0x12, 1, 3, 0, 1, 0, 0, 0, 6, 0, 0, 0, 0, 0, 0, 0}
	original := source.Bytes()
	oriented := append(bytes.Clone(original[:2]), exif...)
	oriented = append(oriented, original[2:]...)
	result, err := GenerateImageThumbnail(context.Background(), oriented)
	require.NoError(t, err)
	assert.Equal(t, 8, result.Width)
	assert.Equal(t, 12, result.Height)
}

func TestImageThumbnailRealStorageRoundTrip(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg executable absent")
	}
	storage.SetGlobalDriver(storage.NewLocalStorageDriver(t.TempDir()))
	t.Cleanup(storage.ResetGlobalDriver)
	t.Cleanup(func() { SetTaskImageThumbnailScheduler(nil) })
	task := &model.Task{TaskID: "thumbnail-roundtrip"}
	source := thumbnailPNG(t, 8, 12)
	scheduled := 0
	SetTaskImageThumbnailScheduler(func(actual *model.Task, key string) {
		assert.Equal(t, task, actual)
		assert.Equal(t, "image", key)
		scheduled++
	})
	_, err := PersistTaskArtifact(t.Context(), task, "image", "image/png", source)
	require.NoError(t, err)
	assert.Equal(t, 1, scheduled)
	require.NoError(t, PersistTaskImageThumbnail(t.Context(), task, "image", source))
	assert.Equal(t, 1, scheduled, "derived persistence must not recursively enqueue")
	metadata, err := ResolveTaskImageThumbnail(t.Context(), task, "image")
	require.NoError(t, err)
	assert.Equal(t, 8, metadata.Width)
	assert.Equal(t, 12, metadata.Height)
	actual, _, err := ReadTaskArtifact(t.Context(), task, "image", MaxTaskArtifactPersistBytes)
	require.NoError(t, err)
	assert.Equal(t, source, actual)
	_, err = ResolveTaskImageThumbnail(t.Context(), &model.Task{TaskID: "other-owner"}, "image")
	require.Error(t, err, "different task must not resolve another task derivative")
}
