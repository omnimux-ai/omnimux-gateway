package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/storage"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

// StoredArtifactRef describes a persisted artifact object.
type StoredArtifactRef struct {
	Backend   string
	Bucket    string
	ObjectKey string
	MimeType  string
	Size      int64
}

// TaskArtifactStore is the persistence boundary for generated artifact bytes.
// types.TaskArtifact is re-exported by relay/channel as channel.TaskArtifact.
type TaskArtifactStore interface {
	Enabled() bool
	Resolve(task *model.Task, artifactKey string) (*StoredArtifactRef, error)
	Persist(ctx context.Context, task *model.Task, artifact types.TaskArtifact, content io.Reader) (*StoredArtifactRef, error)
	Serve(c *gin.Context, task *model.Task, ref *StoredArtifactRef) error
}

var ErrTaskArtifactStoreDisabled = errors.New("task artifact store is disabled")
var errTaskArtifactTooLarge = errors.New("task artifact exceeds persistence size limit")

// MaxTaskArtifactPersistBytes caps a single artifact persistence read. Larger
// objects keep the upstream proxy path instead of being buffered into memory.
const MaxTaskArtifactPersistBytes int64 = 64 << 20

// taskArtifactStoreProbeTimeout bounds an existence probe so a listing can ask
// about many artifacts without waiting on a stalled object store.
const taskArtifactStoreProbeTimeout = 5 * time.Second

// TaskArtifactObjectPrefix is the storage namespace for persisted artifacts:
// task-artifacts/<task_id>/<artifact_key>. Object keys are the resolution
// truth, so blue-green colors and fresh containers share one set without a
// database side table.
const TaskArtifactObjectPrefix = "task-artifacts"

var taskArtifactObjectKeyPartPattern = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,191}$`)

func taskArtifactObjectKey(task *model.Task, artifactKey string) string {
	if task == nil {
		return ""
	}
	return TaskArtifactObjectPrefix + "/" + task.TaskID + "/" + artifactKey
}

func validArtifactStoreTarget(task *model.Task, artifactKey string) bool {
	if task == nil ||
		!taskArtifactObjectKeyPartPattern.MatchString(task.TaskID) ||
		!taskArtifactObjectKeyPartPattern.MatchString(artifactKey) {
		return false
	}
	// Bare dot segments pass the charset check but are path specials.
	for _, part := range []string{task.TaskID, artifactKey} {
		if part == "." || part == ".." {
			return false
		}
	}
	return true
}

// storageArtifactStore persists artifact bytes through the shared pkg/storage
// driver: Cloudflare R2 when R2_* env is configured, local disk otherwise.
type storageArtifactStore struct{}

func (storageArtifactStore) Enabled() bool { return true }

// Resolve reports the stored object when it exists. Missing or unreadable
// objects return (nil, nil) so callers fall through to upstream proxying.
func (storageArtifactStore) Resolve(task *model.Task, artifactKey string) (*StoredArtifactRef, error) {
	if !validArtifactStoreTarget(task, artifactKey) {
		return nil, nil
	}
	objectKey := taskArtifactObjectKey(task, artifactKey)
	ctx, cancel := context.WithTimeout(context.Background(), taskArtifactStoreProbeTimeout)
	defer cancel()
	metadata, err := storage.InspectObject(ctx, storage.GetStorageDriver(), objectKey)
	if err != nil {
		return nil, nil
	}
	return &StoredArtifactRef{
		Backend:   artifactStoreBackendName(),
		ObjectKey: objectKey,
		MimeType:  metadata.ContentType,
		Size:      metadata.Size,
	}, nil
}

func (storageArtifactStore) Persist(ctx context.Context, task *model.Task, artifact types.TaskArtifact, content io.Reader) (*StoredArtifactRef, error) {
	if !validArtifactStoreTarget(task, artifact.Key) || content == nil {
		return nil, ErrTaskArtifactStoreDisabled
	}
	data, err := io.ReadAll(io.LimitReader(content, MaxTaskArtifactPersistBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > MaxTaskArtifactPersistBytes {
		return nil, errTaskArtifactTooLarge
	}
	mimeType := strings.TrimSpace(artifact.MimeType)
	if mimeType == "" {
		mimeType = http.DetectContentType(data)
	}
	objectKey := taskArtifactObjectKey(task, artifact.Key)
	if _, err := storage.GetStorageDriver().Upload(ctx, objectKey, data, mimeType); err != nil {
		return nil, err
	}
	scheduleTaskImageThumbnail(task, artifact.Key, mimeType)
	return &StoredArtifactRef{
		Backend:   artifactStoreBackendName(),
		ObjectKey: objectKey,
		MimeType:  mimeType,
		Size:      int64(len(data)),
	}, nil
}

// PersistTaskArtifact writes artifact bytes at the canonical object key and
// reports the ref. Convenience wrapper for callers that already have bytes.
func PersistTaskArtifact(ctx context.Context, task *model.Task, artifactKey, mimeType string, data []byte) (*StoredArtifactRef, error) {
	return (&storageArtifactStore{}).Persist(ctx, task, types.TaskArtifact{
		Key: artifactKey, Type: "file", MimeType: mimeType,
	}, bytes.NewReader(data))
}

func (storageArtifactStore) Serve(c *gin.Context, task *model.Task, ref *StoredArtifactRef) error {
	if ref == nil || ref.ObjectKey == "" {
		return ErrTaskArtifactStoreDisabled
	}
	driver := storage.GetStorageDriver()
	if c.Request.Method == http.MethodGet && c.Query("delivery") == "direct" && driver.IsR2() {
		if signer, ok := driver.(storage.DownloadPresigner); ok {
			contentType := ref.MimeType
			extension := ""
			switch contentType {
			case "image/png":
				extension = ".png"
			case "image/jpeg":
				extension = ".jpg"
			case "image/webp":
				extension = ".webp"
			case "video/mp4":
				extension = ".mp4"
			case "video/quicktime":
				extension = ".mov"
			}
			if extension != "" && task != nil {
				disposition := "inline"
				if c.Query("download") == "1" {
					disposition = fmt.Sprintf("attachment; filename=%q", task.TaskID+extension)
				}
				ticket, signErr := signer.PresignGet(c.Request.Context(), ref.ObjectKey, contentType, disposition, 5*time.Minute)
				var destination *url.URL
				if ticket != nil {
					destination, _ = url.Parse(ticket.URL)
				}
				if signErr == nil && ticket != nil && ticket.Method == http.MethodGet && destination != nil && destination.Scheme == "https" && destination.User == nil && strings.HasSuffix(destination.Hostname(), ".r2.cloudflarestorage.com") {
					c.Header("Cache-Control", "private, no-store")
					c.Header("Referrer-Policy", "no-referrer")
					c.Redirect(http.StatusTemporaryRedirect, ticket.URL)
					return nil
				}
			}
		}
	}
	reader, mimeType, size, err := driver.Get(c.Request.Context(), ref.ObjectKey)
	if err != nil {
		return err
	}
	defer reader.Close()
	if ref.MimeType != "" {
		mimeType = ref.MimeType
	}
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	c.Header("Cache-Control", "private, max-age=31536000, immutable")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Accept-Ranges", "bytes")
	// Persisted videos are replayed by a media element, which seeks: honouring
	// a single range keeps seeking off the full-download path without buffering
	// the object in memory.
	start, length, partial, rangeErr := parseTaskArtifactRange(c.Request.Header.Get("Range"), size)
	if rangeErr != nil {
		c.Header("Content-Range", fmt.Sprintf("bytes */%d", size))
		c.Status(http.StatusRequestedRangeNotSatisfiable)
		c.Writer.WriteHeaderNow()
		return nil
	}
	if !partial {
		c.DataFromReader(http.StatusOK, size, mimeType, reader, nil)
		return nil
	}
	if start > 0 {
		if _, err := io.CopyN(io.Discard, reader, start); err != nil {
			return err
		}
	}
	c.Header("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, start+length-1, size))
	// DataFromReader only sets Content-Length; the reader itself must stop at
	// the requested range or the client would receive the rest of the object.
	c.DataFromReader(http.StatusPartialContent, length, mimeType, io.LimitReader(reader, length), nil)
	return nil
}

// parseTaskArtifactRange resolves one `bytes=` range against a known object
// size. A missing or unsupported range reports partial=false so the caller
// serves the whole object; a syntactically valid but unsatisfiable range
// reports an error so the caller can answer 416.
func parseTaskArtifactRange(header string, size int64) (start, length int64, partial bool, err error) {
	value := strings.TrimSpace(header)
	if value == "" || size <= 0 {
		return 0, 0, false, nil
	}
	if !strings.HasPrefix(value, "bytes=") {
		return 0, 0, false, nil
	}
	spec := strings.TrimSpace(strings.TrimPrefix(value, "bytes="))
	if spec == "" || strings.Contains(spec, ",") {
		return 0, 0, false, nil
	}
	first, last, found := strings.Cut(spec, "-")
	if !found {
		return 0, 0, false, nil
	}
	first = strings.TrimSpace(first)
	last = strings.TrimSpace(last)
	if first == "" {
		// A suffix range asks for the final N bytes.
		suffix, parseErr := strconv.ParseInt(last, 10, 64)
		if parseErr != nil || suffix <= 0 {
			return 0, 0, false, nil
		}
		if suffix > size {
			suffix = size
		}
		return size - suffix, suffix, true, nil
	}
	start, parseErr := strconv.ParseInt(first, 10, 64)
	if parseErr != nil || start < 0 {
		return 0, 0, false, nil
	}
	if start >= size {
		return 0, 0, false, errors.New("range start is past the end of the object")
	}
	end := size - 1
	if last != "" {
		parsedEnd, endErr := strconv.ParseInt(last, 10, 64)
		if endErr != nil || parsedEnd < start {
			return 0, 0, false, nil
		}
		if parsedEnd < end {
			end = parsedEnd
		}
	}
	return start, end - start + 1, true, nil
}

func artifactStoreBackendName() string {
	if storage.GetStorageDriver().IsR2() {
		return "r2"
	}
	return "local"
}

// IsTaskArtifactStored reports whether the artifact object already exists.
// The probe is bounded: a slow or unreachable object store answers "absent"
// instead of holding the caller.
func IsTaskArtifactStored(task *model.Task, artifactKey string) bool {
	if !validArtifactStoreTarget(task, artifactKey) {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), taskArtifactStoreProbeTimeout)
	defer cancel()
	_, err := storage.InspectObject(ctx, storage.GetStorageDriver(), taskArtifactObjectKey(task, artifactKey))
	return err == nil
}

// ReadTaskArtifact loads a persisted artifact object into memory. Callers pass
// the largest payload they are willing to hold; a larger object is refused
// rather than truncated.
func ReadTaskArtifact(ctx context.Context, task *model.Task, artifactKey string, maxBytes int64) ([]byte, string, error) {
	if !validArtifactStoreTarget(task, artifactKey) {
		return nil, "", ErrTaskArtifactStoreDisabled
	}
	reader, mimeType, _, err := storage.GetStorageDriver().Get(ctx, taskArtifactObjectKey(task, artifactKey))
	if err != nil {
		return nil, "", err
	}
	defer reader.Close()
	data, err := io.ReadAll(io.LimitReader(reader, maxBytes+1))
	if err != nil {
		return nil, "", err
	}
	if int64(len(data)) > maxBytes {
		return nil, "", errTaskArtifactTooLarge
	}
	return data, mimeType, nil
}

var taskArtifactStore TaskArtifactStore = &storageArtifactStore{}

func init() {
	_ = system_setting.LoadTaskArtifactStoreConfig()
}

// GetTaskArtifactStore returns the process-wide artifact storage backend.
func GetTaskArtifactStore() TaskArtifactStore {
	return taskArtifactStore
}
