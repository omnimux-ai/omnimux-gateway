package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ErrPresignUnsupported reports that the active driver cannot hand out an
// upload ticket: either the backend has no presigning support, or it cannot
// expose the stored object afterwards. Callers fail closed instead of pointing
// a client at an unverified write target.
var ErrPresignUnsupported = errors.New("storage driver does not support presigned uploads")
var ErrObjectNotFound = errors.New("storage object not found")

// PresignResult is one short-lived upload ticket for one object key.
type PresignResult struct {
	URL       string
	Method    string
	Headers   map[string]string
	ExpiresAt time.Time
}

// ObjectMetadata can be inspected without transferring an object's body.
type ObjectMetadata struct {
	ContentType string
	Size        int64
}

// MetadataReader is optional so existing local and third-party drivers remain usable.
type MetadataReader interface {
	Head(ctx context.Context, key string) (*ObjectMetadata, error)
}

// DownloadPresigner issues object-scoped read tickets without application credentials.
type DownloadPresigner interface {
	PresignGet(ctx context.Context, key, contentType, disposition string, ttl time.Duration) (*PresignResult, error)
}

// InspectObject uses metadata-only reads when available.
func InspectObject(ctx context.Context, driver StorageDriver, key string) (*ObjectMetadata, error) {
	if metadata, ok := driver.(MetadataReader); ok {
		return metadata.Head(ctx, key)
	}
	reader, contentType, size, err := driver.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	_ = reader.Close()
	return &ObjectMetadata{ContentType: contentType, Size: size}, nil
}

// StorageDriver defines the interface for object storage backends.
type StorageDriver interface {
	Upload(ctx context.Context, key string, data []byte, contentType string) (string, error)
	Get(ctx context.Context, key string) (io.ReadCloser, string, int64, error)
	Delete(ctx context.Context, key string) error
	GetPublicURL(key string) string
	IsR2() bool
	PresignPut(ctx context.Context, key string, contentType string, ttl time.Duration) (*PresignResult, error)
}

var (
	globalDriver StorageDriver
	driverMu     sync.RWMutex
)

// SetGlobalDriver overrides the global storage driver (mainly for testing).
func SetGlobalDriver(driver StorageDriver) {
	driverMu.Lock()
	defer driverMu.Unlock()
	globalDriver = driver
}

// ResetGlobalDriver resets the global storage driver to auto-detect.
func ResetGlobalDriver() {
	driverMu.Lock()
	defer driverMu.Unlock()
	globalDriver = nil
}

// GetStorageDriver returns the initialized storage driver.
func GetStorageDriver() StorageDriver {
	driverMu.RLock()
	if globalDriver != nil {
		defer driverMu.RUnlock()
		return globalDriver
	}
	driverMu.RUnlock()

	driverMu.Lock()
	defer driverMu.Unlock()
	if globalDriver != nil {
		return globalDriver
	}

	// Try initializing Cloudflare R2
	r2Driver, err := NewR2StorageDriverFromEnv()
	if err == nil && r2Driver != nil {
		globalDriver = r2Driver
		return globalDriver
	}

	// Fallback to local storage driver
	globalDriver = NewLocalStorageDriver()
	return globalDriver
}

// LocalStorageDriver stores files on local disk
type LocalStorageDriver struct {
	baseDir       string
	publicBaseURL string
}

// NewLocalStorageDriver creates a new local disk storage driver.
func NewLocalStorageDriver(baseDirs ...string) *LocalStorageDriver {
	dir := "data/storage"
	if len(baseDirs) > 0 && baseDirs[0] != "" {
		dir = baseDirs[0]
	}
	_ = os.MkdirAll(dir, 0755)
	publicURL := strings.TrimRight(os.Getenv("FILE_PUBLIC_BASE_URL"), "/")
	return &LocalStorageDriver{
		baseDir:       dir,
		publicBaseURL: publicURL,
	}
}

func (l *LocalStorageDriver) Upload(ctx context.Context, key string, data []byte, contentType string) (string, error) {
	cleanKey := filepath.Clean(key)
	if strings.HasPrefix(cleanKey, "..") {
		return "", fmt.Errorf("invalid storage key: path traversal rejected")
	}
	filePath := filepath.Join(l.baseDir, cleanKey)
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}
	return l.GetPublicURL(key), nil
}

func (l *LocalStorageDriver) Get(ctx context.Context, key string) (io.ReadCloser, string, int64, error) {
	cleanKey := filepath.Clean(key)
	if strings.HasPrefix(cleanKey, "..") {
		return nil, "", 0, fmt.Errorf("invalid storage key: path traversal rejected")
	}
	filePath := filepath.Join(l.baseDir, cleanKey)
	f, err := os.Open(filePath)
	if err != nil {
		return nil, "", 0, err
	}
	fi, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, "", 0, err
	}
	return f, "application/octet-stream", fi.Size(), nil
}

func (l *LocalStorageDriver) Delete(ctx context.Context, key string) error {
	cleanKey := filepath.Clean(key)
	if strings.HasPrefix(cleanKey, "..") {
		return fmt.Errorf("invalid storage key: path traversal rejected")
	}
	filePath := filepath.Join(l.baseDir, cleanKey)
	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// publicObjectURL encodes object names as path data, preserving directory separators.
// The configured base remains untouched so callers can reject unsafe origins.
func publicObjectURL(base, key string) string {
	if base == "" {
		return ""
	}
	parts := strings.Split(strings.TrimLeft(key, "/"), "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	return base + "/" + strings.Join(parts, "/")
}

func (l *LocalStorageDriver) GetPublicURL(key string) string {
	return publicObjectURL(l.publicBaseURL, key)
}

func (l *LocalStorageDriver) IsR2() bool {
	return false
}

// PresignPut refuses: a local directory has no way to accept a write that the
// gateway did not see, and its public URL is the application host.
func (l *LocalStorageDriver) PresignPut(context.Context, string, string, time.Duration) (*PresignResult, error) {
	return nil, ErrPresignUnsupported
}
