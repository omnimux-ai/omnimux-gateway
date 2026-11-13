package service

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/storage"
)

var ErrInfluencerVideoBusy = errors.New("视频检查繁忙，请稍后重试")
var ErrInfluencerVideoUnavailable = errors.New("视频准备暂不可用，请稍后重试")

// PreparedInfluencerVideo owns server-created references, separate from the user's material identity.
type PreparedInfluencerVideo struct {
	Metadata InfluencerVideoMetadata
	URL      string
	SHA256   string
	files    []*model.File
	driver   storage.StorageDriver
}

// Discard reclaims preparation that was rejected before task submission. Failed deletion
// retains the existing expiry record so the file-expiration service can retry.
func (prepared *PreparedInfluencerVideo) Discard() {
	if prepared.driver == nil {
		return
	}
	for _, file := range prepared.files {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		err := prepared.driver.Delete(ctx, file.StorageKey)
		cancel()
		if err == nil {
			if err := model.DeleteFilePermanently(file.Id); err != nil {
				common.SysError("reference media record cleanup failed: " + err.Error())
			}
		} else {
			common.SysError("reference media object cleanup failed: " + err.Error())
		}
	}
	prepared.files = nil
}

// InfluencerVideoInspector retains the legacy product API media inspection boundary.
// validateURL reuses the controller's established public-file origin isolation policy.
var InfluencerVideoInspector = inspectInfluencerVideo

// InfluencerBrowserVideoPreparer freezes the already browser-checked source.
// The metadata remains a client declaration; this path never invokes the
// server media inspector.
var InfluencerBrowserVideoPreparer = prepareInfluencerBrowserVideo

var influencerVideoPreparationProbe = probeInfluencerVideoFile

// InspectInfluencerVideoReference performs a read-only preflight. Its caller must
// establish file ownership and the allowed source before supplying a URL. Temporary
// media and process slots are reclaimed before returning; no objects or model tasks are created.
func InspectInfluencerVideoReference(parent context.Context, videoURL, mode string) (InfluencerVideoMetadata, error) {
	var empty InfluencerVideoMetadata
	if err := parent.Err(); err != nil {
		return empty, err
	}
	if _, err := GetInfluencerVideoInputRules(mode); err != nil {
		return empty, err
	}
	select {
	case influencerVideoProbeSlots <- struct{}{}:
		defer func() { <-influencerVideoProbeSlots }()
	default:
		return empty, ErrInfluencerVideoBusy
	}
	ctx, cancel := context.WithTimeout(parent, influencerVideoPreparationTimeout)
	defer cancel()
	directory, err := os.MkdirTemp("", "influencer-video-preflight-*")
	if err != nil {
		return empty, ErrInfluencerVideoUnavailable
	}
	defer os.RemoveAll(directory)
	path := filepath.Join(directory, "reference")
	if err := downloadInfluencerVideo(ctx, videoURL, path); err != nil {
		return empty, err
	}
	metadata, err := probeInfluencerVideoFile(ctx, path)
	if err != nil {
		return empty, err
	}
	if err := ValidateInfluencerVideoReferenceMetadata(metadata, mode); err != nil {
		return empty, err
	}
	return metadata, nil
}

func inspectInfluencerVideo(parent context.Context, videoURL string, userID int, validateURL func(string) bool) (PreparedInfluencerVideo, error) {
	return prepareInfluencerVideo(parent, videoURL, userID, nil, validateURL)
}

func prepareInfluencerBrowserVideo(parent context.Context, videoURL string, userID int, metadata InfluencerVideoMetadata, validateURL func(string) bool) (PreparedInfluencerVideo, error) {
	return prepareInfluencerVideo(parent, videoURL, userID, &metadata, validateURL)
}

// prepareInfluencerVideo shares bounded download and immutable storage. Only
// legacy callers without browser declarations inspect media.
func prepareInfluencerVideo(parent context.Context, videoURL string, userID int, declared *InfluencerVideoMetadata, validateURL func(string) bool) (PreparedInfluencerVideo, error) {
	var prepared PreparedInfluencerVideo
	if err := parent.Err(); err != nil {
		return prepared, err
	}
	if userID <= 0 {
		return prepared, errors.New("未授权访问")
	}
	if validateURL == nil {
		return prepared, ErrInfluencerVideoUnavailable
	}
	select {
	case influencerVideoProbeSlots <- struct{}{}:
		defer func() { <-influencerVideoProbeSlots }()
	default:
		return prepared, ErrInfluencerVideoBusy
	}
	ctx, cancel := context.WithTimeout(parent, influencerVideoPreparationTimeout)
	defer cancel()
	directory, err := os.MkdirTemp("", "influencer-video-check-*")
	if err != nil {
		return prepared, ErrInfluencerVideoUnavailable
	}
	defer os.RemoveAll(directory)
	path := filepath.Join(directory, "reference")
	if err := downloadInfluencerVideo(ctx, videoURL, path); err != nil {
		return prepared, err
	}
	var metadata InfluencerVideoMetadata
	if declared != nil {
		metadata = *declared
	} else {
		metadata, err = influencerVideoPreparationProbe(ctx, path)
		if err != nil {
			if ctx.Err() != nil {
				return prepared, ctx.Err()
			}
			return prepared, err
		}
	}
	prepared.Metadata = metadata
	if err := ctx.Err(); err != nil {
		return prepared, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return prepared, ErrInfluencerVideoUnavailable
	}
	prepared.driver = storage.GetStorageDriver()
	if prepared.driver == nil {
		return prepared, ErrInfluencerVideoUnavailable
	}
	success := false
	defer func() {
		if !success {
			prepared.Discard()
		}
	}()
	mime := "video/mp4"
	if metadata.Container == "mov" {
		mime = "video/quicktime"
	}
	prepared.URL, err = prepared.store(ctx, userID, data, metadata.Container, mime, validateURL)
	if err != nil {
		return PreparedInfluencerVideo{}, err
	}
	digest := sha256.Sum256(data)
	prepared.SHA256 = fmt.Sprintf("%x", digest)
	if err := ctx.Err(); err != nil {
		return PreparedInfluencerVideo{}, err
	}
	success = true
	return prepared, nil
}

// store writes only server-owned unique keys; no user-supplied upload path or key is accepted.
func (prepared *PreparedInfluencerVideo) store(ctx context.Context, userID int, data []byte, extension, mime string, validateURL func(string) bool) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if len(data) == 0 || len(data) > InfluencerVideoMaxBytes {
		return "", errors.New("参考媒体文件无效或超过200MiB")
	}
	count, err := model.CountActiveUserFiles(userID)
	if err != nil {
		return "", fmt.Errorf("%w: 文件记录不可用", ErrInfluencerVideoUnavailable)
	}
	if count >= model.DefaultUserMaxFiles {
		return "", errors.New("文件配额已耗尽")
	}
	id := common.GetUUID()
	name := id + "." + extension
	key := "influencer-reference/frozen/" + name
	expected := prepared.driver.GetPublicURL(key)
	if !validateURL(expected) {
		return "", fmt.Errorf("%w: 公开存储地址必须为独立公网HTTPS域名", ErrInfluencerVideoUnavailable)
	}
	now := time.Now()
	record := &model.File{FileId: "file_" + id, UserId: userID, FileName: name, OriginalName: name, FileSize: int64(len(data)), MimeType: mime, StorageKey: key, UploadPath: "influencer-reference/frozen", FileUrl: expected, CreatedAt: now, ExpiresAt: now.Add(model.DefaultFileExpireDuration), Status: model.FileStatusUploadPending}
	// Record before upload, so even a partial upload or failed cleanup remains expirable.
	if err := model.CreateFile(record); err != nil {
		return "", fmt.Errorf("%w: 记录文件信息失败", ErrInfluencerVideoUnavailable)
	}
	prepared.files = append(prepared.files, record)
	uploaded, err := prepared.driver.Upload(ctx, key, data, mime)
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	if err != nil {
		return "", fmt.Errorf("%w: 上传文件失败", ErrInfluencerVideoUnavailable)
	}
	if uploaded != expected || !validateURL(uploaded) {
		return "", fmt.Errorf("%w: 存储地址不一致", ErrInfluencerVideoUnavailable)
	}
	if err := model.ConfirmDirectFileUpload(record.Id, userID, int64(len(data)), mime, record.ExpiresAt); err != nil {
		return "", fmt.Errorf("%w: 确认文件信息失败", ErrInfluencerVideoUnavailable)
	}
	return expected, nil
}

func InfluencerVideoInspectionStatus(err error) int {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return http.StatusRequestTimeout
	}
	if errors.Is(err, ErrInfluencerVideoBusy) || errors.Is(err, ErrInfluencerVideoUnavailable) {
		return http.StatusServiceUnavailable
	}
	return http.StatusBadRequest
}
