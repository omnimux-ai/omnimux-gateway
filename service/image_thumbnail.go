package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"github.com/QuantumNous/new-api/pkg/storage"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os/exec"
	"regexp"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/model"
	_ "golang.org/x/image/webp"
)

const (
	ImageThumbnailWidth           = 640
	ImageThumbnailMaxPixels int64 = 64_000_000
	ImageThumbnailMaxBytes        = 8 << 20
	imageThumbnailTimeout         = 30 * time.Second
)

// ImageThumbnail contains a WebP derivative; the source bytes remain untouched.
type ImageThumbnail struct {
	Data   []byte
	Width  int
	Height int
}

var imageThumbnailSlots = make(chan struct{}, 2)

type imageThumbnailEncoder func(context.Context, []byte) ([]byte, error)

// GenerateImageThumbnail bounds input decoding and processing before producing
// a versioned, proportional derivative. Encoding never changes the original.
func GenerateImageThumbnail(ctx context.Context, data []byte) (*ImageThumbnail, error) {
	return generateImageThumbnail(ctx, data, encodeImageThumbnail)
}

func generateImageThumbnail(ctx context.Context, data []byte, encode imageThumbnailEncoder) (*ImageThumbnail, error) {
	if len(data) == 0 || int64(len(data)) > MaxTaskArtifactPersistBytes {
		return nil, fmt.Errorf("image thumbnail input exceeds byte limit or is empty")
	}
	config, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("image thumbnail header: %w", err)
	}
	if config.Width <= 0 || config.Height <= 0 || int64(config.Width) > ImageThumbnailMaxPixels/int64(config.Height) {
		return nil, fmt.Errorf("image thumbnail input exceeds pixel limit")
	}
	ctx, cancel := context.WithTimeout(ctx, imageThumbnailTimeout)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	select {
	case imageThumbnailSlots <- struct{}{}:
		defer func() { <-imageThumbnailSlots }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	encoded, err := encode(ctx, data)
	if err != nil {
		return nil, err
	}
	if len(encoded) == 0 || len(encoded) > ImageThumbnailMaxBytes {
		return nil, fmt.Errorf("image thumbnail output exceeds byte limit or is empty")
	}
	result, format, err := image.DecodeConfig(bytes.NewReader(encoded))
	if err != nil || format != "webp" {
		return nil, fmt.Errorf("image thumbnail encoder returned invalid WebP")
	}
	if result.Width <= 0 || result.Height <= 0 || result.Width > ImageThumbnailWidth || int64(result.Width) > ImageThumbnailMaxPixels/int64(result.Height) {
		return nil, fmt.Errorf("image thumbnail encoder returned invalid dimensions")
	}
	return &ImageThumbnail{Data: encoded, Width: result.Width, Height: result.Height}, nil
}

func encodeImageThumbnail(ctx context.Context, data []byte) ([]byte, error) {
	command := exec.CommandContext(ctx, "ffmpeg", "-nostdin", "-hide_banner", "-loglevel", "error", "-threads", "1", "-i", "pipe:0", "-frames:v", "1", "-vf", "scale=w='min(640,iw)':h=-1", "-c:v", "libwebp", "-quality", "82", "-threads", "1", "-f", "webp", "pipe:1")
	command.Stdin = bytes.NewReader(data)
	output := &thumbnailBoundedBuffer{limit: ImageThumbnailMaxBytes}
	diagnostic := &thumbnailBoundedBuffer{limit: 4096}
	command.Stdout = output
	command.Stderr = diagnostic
	if err := command.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("image thumbnail encoding failed: %w", err)
	}
	if output.overflow {
		return nil, fmt.Errorf("image thumbnail output exceeds byte limit")
	}
	return output.Bytes(), nil
}

// Bound command output without breaking stderr draining or allowing a child
// process to allocate unbounded Go memory.
type thumbnailBoundedBuffer struct {
	bytes.Buffer
	limit    int
	overflow bool
}

func (b *thumbnailBoundedBuffer) Write(p []byte) (int, error) {
	count := len(p)
	remaining := b.limit - b.Len()
	if count > remaining {
		p = p[:remaining]
		b.overflow = true
	}
	_, _ = b.Buffer.Write(p)
	return count, nil
}

var _ io.Writer = (*thumbnailBoundedBuffer)(nil)

// TaskImageThumbnailKey stays within artifact-key limits even for long source
// keys, and changes when derivative encoding rules are revised.
func TaskImageThumbnailKey(artifactKey string) string {
	return fmt.Sprintf("thumb-v1-%x.webp", sha256.Sum256([]byte(artifactKey)))
}

// PersistTaskImageThumbnail writes only the derivative, never the source key.
// Callers own asynchronous scheduling and bounded retries.
func PersistTaskImageThumbnail(ctx context.Context, task *model.Task, artifactKey string, data []byte) error {
	if !validArtifactStoreTarget(task, artifactKey) {
		return ErrTaskArtifactStoreDisabled
	}
	thumbnail, err := GenerateImageThumbnail(ctx, data)
	if err != nil {
		return err
	}
	_, err = PersistTaskArtifact(ctx, task, TaskImageThumbnailKey(artifactKey), "image/webp", thumbnail.Data)
	return err
}

// ImageThumbnailMetadata describes a ready derivative from a bounded header read.
type ImageThumbnailMetadata struct {
	Width  int
	Height int
}

func ResolveTaskImageThumbnail(ctx context.Context, task *model.Task, artifactKey string) (*ImageThumbnailMetadata, error) {
	if !validArtifactStoreTarget(task, artifactKey) {
		return nil, ErrTaskArtifactStoreDisabled
	}
	reader, _, _, err := storage.GetStorageDriver().Get(ctx, taskArtifactObjectKey(task, TaskImageThumbnailKey(artifactKey)))
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	config, format, err := image.DecodeConfig(io.LimitReader(reader, 64<<10))
	if err != nil {
		return nil, err
	}
	if format != "webp" || config.Width <= 0 || config.Width > ImageThumbnailWidth || config.Height <= 0 {
		return nil, fmt.Errorf("invalid stored image thumbnail")
	}
	return &ImageThumbnailMetadata{Width: config.Width, Height: config.Height}, nil
}

var thumbnailKeyPattern = regexp.MustCompile(`^thumb-v1-[0-9a-f]{64}\.webp$`)

func IsTaskImageThumbnailKey(key string) bool { return thumbnailKeyPattern.MatchString(key) }

var thumbnailScheduler struct {
	sync.RWMutex
	enqueue func(*model.Task, string)
}

// SetTaskImageThumbnailScheduler connects storage completion to the bounded
// application queue without making the service depend on controllers.
func SetTaskImageThumbnailScheduler(enqueue func(*model.Task, string)) {
	thumbnailScheduler.Lock()
	defer thumbnailScheduler.Unlock()
	thumbnailScheduler.enqueue = enqueue
}

func scheduleTaskImageThumbnail(task *model.Task, key, mimeType string) {
	if IsTaskImageThumbnailKey(key) || len(mimeType) < 6 || mimeType[:6] != "image/" {
		return
	}
	thumbnailScheduler.RLock()
	enqueue := thumbnailScheduler.enqueue
	thumbnailScheduler.RUnlock()
	if enqueue != nil {
		enqueue(task, key)
	}
}
