package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"image/png"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/storage"
	"github.com/QuantumNous/new-api/setting/system_setting"
)

const influencerFrozenImagePrefix = "influencer-image/frozen"

// ResolveInfluencerImageSourceExpiry preserves ownership for managed material,
// while allowing public external images to enter protected image validation.
// The external lifetime applies only to the newly frozen copy, not to a claim
// that an external URL or its content remains available for that lifetime.
func ResolveInfluencerImageSourceExpiry(ctx context.Context, userID int, rawURL string) (time.Time, error) {
	if err := ctx.Err(); err != nil {
		return time.Time{}, err
	}
	if userID <= 0 || model.DB == nil {
		return time.Time{}, errors.New("image material ownership is unavailable")
	}
	if err := influencerReferenceURL(rawURL); err != nil {
		return time.Time{}, err
	}
	candidate, err := url.Parse(rawURL)
	if err != nil || candidate.Opaque != "" {
		return time.Time{}, errors.New("image material URL is invalid")
	}
	var files []model.File
	if err := model.DB.WithContext(ctx).Where("file_url = ?", rawURL).Limit(2).Find(&files).Error; err != nil {
		return time.Time{}, errors.New("image material file records are unavailable")
	}
	driver := storage.GetStorageDriver()
	if len(files) > 0 {
		if len(files) != 1 {
			return time.Time{}, errors.New("image material file URL is ambiguous")
		}
		file := files[0]
		if file.UserId != userID || file.Status != model.FileStatusActive || !file.ExpiresAt.After(time.Now()) || driver == nil || driver.GetPublicURL(file.StorageKey) != rawURL {
			return time.Time{}, errors.New("image material does not belong to this account or has expired")
		}
		return file.ExpiresAt, nil
	}
	for _, rawBase := range []string{system_setting.TaskPublicAddress, system_setting.ServerAddress} {
		relative, managed := influencerManagedImagePath(candidate, rawBase)
		if !managed || relative != "/v1/tasks" && !strings.HasPrefix(relative, "/v1/tasks/") {
			continue
		}
		base, _ := url.Parse(strings.TrimSpace(rawBase))
		escapedPrefix := strings.TrimRight(base.EscapedPath(), "/")
		parts := strings.Split(strings.TrimPrefix(candidate.EscapedPath(), escapedPrefix+"/"), "/")
		if len(parts) != 6 || parts[0] != "v1" || parts[1] != "tasks" || parts[3] != "artifacts" || parts[5] != "content" || candidate.Fragment != "" {
			return time.Time{}, errors.New("native image artifact URL is invalid")
		}
		taskID, taskErr := url.PathUnescape(parts[2])
		artifactKey, keyErr := url.PathUnescape(parts[4])
		query, queryErr := url.ParseQuery(candidate.RawQuery)
		if taskErr != nil || keyErr != nil || queryErr != nil || len(query) != 1 || len(query[TaskArtifactAccessQueryParameter]) != 1 || !VerifyTaskArtifactAccess(query.Get(TaskArtifactAccessQueryParameter), taskID, artifactKey) {
			return time.Time{}, errors.New("native image artifact access is invalid")
		}
		expectedURL, err := BuildTaskArtifactContentURL(taskID, artifactKey)
		if err != nil {
			return time.Time{}, errors.New("native image artifact URL is unavailable")
		}
		expected, err := url.Parse(expectedURL)
		if err != nil || candidate.Scheme != expected.Scheme || !strings.EqualFold(candidate.Host, expected.Host) || candidate.EscapedPath() != expected.EscapedPath() {
			return time.Time{}, errors.New("native image artifact URL does not match its configured origin")
		}
		task, exists, err := model.GetUniqueByOnlyTaskId(taskID)
		if err != nil || !exists || task.UserId != userID || task.Status != model.TaskStatusSuccess || !task.ResultRetrievable() {
			return time.Time{}, errors.New("native image artifact does not belong to this account or is unavailable")
		}
		if err := ctx.Err(); err != nil {
			return time.Time{}, err
		}
		return time.Now().Add(model.DefaultFileExpireDuration), nil
	}
	if driver != nil {
		if _, managed := influencerManagedImagePath(candidate, driver.GetPublicURL("")); managed {
			return time.Time{}, errors.New("managed image material has no active owned file record")
		}
	}
	return time.Now().Add(model.DefaultFileExpireDuration), nil
}

// influencerManagedImagePath recognizes origin/path boundaries before any
// external fallback. Decoded path normalization also catches forged task paths;
// accepted native capabilities must additionally match their exact escaped URL.
func influencerManagedImagePath(candidate *url.URL, rawBase string) (string, bool) {
	base, err := url.Parse(strings.TrimSpace(rawBase))
	if err != nil || base == nil || base.Host == "" || base.User != nil || base.Opaque != "" || !strings.EqualFold(candidate.Scheme, base.Scheme) || !strings.EqualFold(candidate.Hostname(), base.Hostname()) {
		return "", false
	}
	candidatePort, basePort := candidate.Port(), base.Port()
	if candidatePort == "" && strings.EqualFold(candidate.Scheme, "https") {
		candidatePort = "443"
	}
	if basePort == "" && strings.EqualFold(base.Scheme, "https") {
		basePort = "443"
	}
	if candidatePort != basePort {
		return "", false
	}
	basePath := strings.TrimRight(path.Clean("/"+strings.TrimPrefix(base.Path, "/")), "/")
	candidatePath := path.Clean("/" + strings.TrimPrefix(candidate.Path, "/"))
	if candidatePath != basePath && !strings.HasPrefix(candidatePath, basePath+"/") {
		return "", false
	}
	return strings.TrimPrefix(candidatePath, basePath), true
}

// InfluencerFrozenImage identifies one server-owned immutable input. A queue may
// restore this snapshot only after ReadInfluencerFrozenImage verifies its bytes.
type InfluencerFrozenImage = model.GenerationImageReference

// PreparedInfluencerImage owns a frozen copy until its task has been accepted.
type PreparedInfluencerImage struct {
	InfluencerFrozenImage
	file   *model.File
	driver storage.StorageDriver
}

// Discard reclaims only this server-created copy. An unsuccessful delete keeps
// its original expiry record available to the existing expiration service.
func (prepared *PreparedInfluencerImage) Discard() {
	if prepared == nil || prepared.driver == nil || prepared.file == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := prepared.driver.Delete(ctx, prepared.file.StorageKey); err != nil {
		common.SysError("frozen image object cleanup failed: " + err.Error())
		return
	}
	if err := model.DeleteFilePermanently(prepared.file.Id); err != nil {
		common.SysError("frozen image record cleanup failed: " + err.Error())
		return
	}
	prepared.file = nil
}

// FreezeInfluencerImage downloads and validates exactly once. Managed ownership
// is checked for the initial URL and every redirect. The caller's verified
// retention bound may only shrink when a redirect has an earlier actual expiry.
func FreezeInfluencerImage(ctx context.Context, userID int, rawURL string, expiresAt time.Time, client *http.Client) (PreparedInfluencerImage, error) {
	var prepared PreparedInfluencerImage
	if err := ctx.Err(); err != nil {
		return prepared, err
	}
	if userID <= 0 || !expiresAt.After(time.Now()) || expiresAt.Unix() <= time.Now().Unix() {
		return prepared, errors.New("image owner or original expiry is invalid")
	}
	data, config, format, err := readInfluencerReference(ctx, rawURL, client, func(checkCtx context.Context, target string) error {
		retainedUntil, err := ResolveInfluencerImageSourceExpiry(checkCtx, userID, target)
		if err != nil {
			return err
		}
		if retainedUntil.Before(expiresAt) {
			expiresAt = retainedUntil
		}
		return nil
	})
	if err != nil {
		return prepared, err
	}
	if err := ctx.Err(); err != nil {
		return prepared, err
	}
	prepared.driver = storage.GetStorageDriver()
	if prepared.driver == nil {
		return PreparedInfluencerImage{}, errors.New("image storage is unavailable")
	}
	id := common.GetUUID()
	extension := format
	if extension == "jpeg" {
		extension = "jpg"
	}
	name := id + "." + extension
	key := influencerFrozenImagePrefix + "/" + name
	publicURL := prepared.driver.GetPublicURL(key)
	if err := influencerReferenceURL(publicURL); err != nil {
		return PreparedInfluencerImage{}, errors.New("image storage must expose a public HTTPS URL")
	}
	now := time.Now()
	if !expiresAt.After(now) {
		return PreparedInfluencerImage{}, errors.New("original image has expired")
	}
	mime := "image/" + format
	record := &model.File{FileId: "file_" + id, UserId: userID, FileName: name, OriginalName: name, FileSize: int64(len(data)), MimeType: mime, StorageKey: key, UploadPath: influencerFrozenImagePrefix, FileUrl: publicURL, CreatedAt: now, ExpiresAt: expiresAt, Status: model.FileStatusUploadPending}
	if err := model.CreateFile(record); err != nil {
		return PreparedInfluencerImage{}, errors.New("image file record could not be created")
	}
	prepared.file = record
	success := false
	defer func() {
		if !success {
			prepared.Discard()
		}
	}()
	uploaded, err := prepared.driver.Upload(ctx, key, data, mime)
	if ctx.Err() != nil {
		return PreparedInfluencerImage{}, ctx.Err()
	}
	if err != nil {
		return PreparedInfluencerImage{}, errors.New("image could not be frozen")
	}
	if uploaded != publicURL {
		return PreparedInfluencerImage{}, errors.New("frozen image storage URL is inconsistent")
	}
	if err := model.ConfirmDirectFileUpload(record.Id, userID, int64(len(data)), mime, expiresAt); err != nil {
		return PreparedInfluencerImage{}, errors.New("frozen image could not be confirmed")
	}
	if err := ctx.Err(); err != nil {
		return PreparedInfluencerImage{}, err
	}
	if expiresAt.Unix() <= time.Now().Unix() {
		return PreparedInfluencerImage{}, errors.New("original image expired while freezing")
	}
	digest := sha256.Sum256(data)
	prepared.InfluencerFrozenImage = InfluencerFrozenImage{URL: publicURL, FileID: record.FileId, SHA256: fmt.Sprintf("%x", digest), Width: config.Width, Height: config.Height, Format: format, ExpiresAt: expiresAt.Unix()}
	success = true
	return prepared, nil
}

// ReadInfluencerFrozenImage reads through the storage driver, never the original
// URL. Ownership, retention, immutable identity and decoded content are rechecked
// before a restored task can send any input to a supplier.
func ReadInfluencerFrozenImage(parent context.Context, userID int, snapshot InfluencerFrozenImage) ([]byte, error) {
	if err := parent.Err(); err != nil {
		return nil, err
	}
	digest, err := hex.DecodeString(snapshot.SHA256)
	if userID <= 0 || snapshot.FileID == "" || len(digest) != sha256.Size || err != nil || strings.ToLower(snapshot.SHA256) != snapshot.SHA256 || snapshot.Width <= 0 || snapshot.Height <= 0 || snapshot.ExpiresAt <= time.Now().Unix() {
		return nil, errors.New("frozen image snapshot is invalid or expired")
	}
	ctx, cancel := context.WithTimeout(parent, 15*time.Second)
	defer cancel()
	file, err := model.GetUniqueFileByURL(ctx, snapshot.URL)
	if err != nil {
		return nil, errors.New("frozen image file is unavailable")
	}
	driver := storage.GetStorageDriver()
	if driver == nil || file.FileId != snapshot.FileID || file.UserId != userID || file.Status != model.FileStatusActive || !file.ExpiresAt.After(time.Now()) || file.ExpiresAt.Unix() != snapshot.ExpiresAt || file.UploadPath != influencerFrozenImagePrefix || path.Clean(file.StorageKey) != file.StorageKey || path.Dir(file.StorageKey) != influencerFrozenImagePrefix || path.Base(file.StorageKey) != file.FileName || driver.GetPublicURL(file.StorageKey) != snapshot.URL || file.MimeType != "image/"+snapshot.Format || file.FileSize <= 0 || file.FileSize > influencerReferenceMaxBytes {
		return nil, errors.New("frozen image ownership or storage identity is invalid")
	}
	if err := influencerReferenceURL(snapshot.URL); err != nil {
		return nil, errors.New("frozen image URL is not public HTTPS")
	}
	reader, _, size, err := driver.Get(ctx, file.StorageKey)
	if err != nil {
		return nil, errors.New("frozen image could not be read")
	}
	if size > influencerReferenceMaxBytes || size >= 0 && size != file.FileSize {
		_ = reader.Close()
		return nil, errors.New("frozen image storage size is inconsistent")
	}
	data, readErr := io.ReadAll(io.LimitReader(reader, influencerReferenceMaxBytes+1))
	closeErr := reader.Close()
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if readErr != nil || closeErr != nil || int64(len(data)) != file.FileSize {
		return nil, errors.New("frozen image bytes are unavailable or inconsistent")
	}
	actual := sha256.Sum256(data)
	if !bytes.Equal(actual[:], digest) {
		return nil, errors.New("frozen image content has changed")
	}
	config, format, err := validateInfluencerImageBytes(data)
	if err != nil || format != snapshot.Format || config.Width != snapshot.Width || config.Height != snapshot.Height {
		return nil, errors.New("frozen image metadata is inconsistent")
	}
	if !file.ExpiresAt.After(time.Now()) || snapshot.ExpiresAt <= time.Now().Unix() {
		return nil, errors.New("frozen image expired while reading")
	}
	return data, nil
}

// ValidateInfluencerFrozenImageMask requires a real edit selection: transparent
// pixels are editable and fully opaque pixels protect the retained image.
func ValidateInfluencerFrozenImageMask(ctx context.Context, userID int, source, mask InfluencerFrozenImage) error {
	sourceData, err := ReadInfluencerFrozenImage(ctx, userID, source)
	if err != nil {
		return err
	}
	if source.Format != "png" {
		return errors.New("masked image editing requires a PNG source")
	}
	hasExif, err := influencerPNGHasExif(sourceData)
	if err != nil || hasExif {
		return errors.New("masked image source must not contain PNG EXIF orientation metadata")
	}
	data, err := ReadInfluencerFrozenImage(ctx, userID, mask)
	if err != nil {
		return err
	}
	// fork: keep the shared mask contract below the stricter supplier limit;
	// source images retain the independent 5 MiB allowance.
	if len(data) >= 4_000_000 {
		return errors.New("image mask must be smaller than 4,000,000 bytes")
	}
	if mask.Format != "png" || mask.Width != source.Width || mask.Height != source.Height {
		return errors.New("image mask must be PNG with the same dimensions as its source")
	}
	hasExif, err = influencerPNGHasExif(data)
	if err != nil || hasExif {
		return errors.New("image mask must not contain PNG EXIF orientation metadata")
	}
	decoded, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return errors.New("image mask could not be decoded")
	}
	transparent, opaque := false, false
	for y := decoded.Bounds().Min.Y; y < decoded.Bounds().Max.Y; y++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		for x := decoded.Bounds().Min.X; x < decoded.Bounds().Max.X; x++ {
			_, _, _, alpha := decoded.At(x, y).RGBA()
			transparent = transparent || alpha == 0
			opaque = opaque || alpha == 0xffff
			if transparent && opaque {
				return nil
			}
		}
	}
	return errors.New("image mask must contain both transparent edit pixels and opaque protected pixels")
}

// influencerPNGHasExif examines actual PNG chunks through IEND, never matching
// compressed pixels or ancillary text that happens to contain the same letters.
// Full PNG decode/CRC validation must precede this additional orientation check.
func influencerPNGHasExif(data []byte) (bool, error) {
	if len(data) < 8 || !bytes.Equal(data[:8], []byte("\x89PNG\r\n\x1a\n")) {
		return false, errors.New("invalid PNG signature")
	}
	for offset := 8; offset < len(data); {
		if len(data)-offset < 12 {
			return false, errors.New("truncated PNG chunk")
		}
		size := binary.BigEndian.Uint32(data[offset : offset+4])
		if uint64(size) > uint64(len(data)-offset-12) {
			return false, errors.New("invalid PNG chunk length")
		}
		kind := string(data[offset+4 : offset+8])
		if kind == "eXIf" {
			return true, nil
		}
		offset += 12 + int(size)
		if kind == "IEND" {
			return false, nil
		}
	}
	return false, errors.New("PNG has no IEND chunk")
}
