package controller

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/storage"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/service/influencer"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// fork: influencer-preset-publish
//
// An administrator promotes one finished character from their history into the
// inspiration library. The character's parameters are read from the task record
// — never from the request — and the artwork the browser is already showing is
// stored under a key this gateway owns, so the library entry outlives the
// upstream render. Ordinary users only read the library.

const (
	// influencerPresetMaxAssetBytes bounds one uploaded artwork. The shared
	// upload limit is 100 MiB; a character render is a few megabytes, so the
	// tighter bound keeps a publish request cheap.
	influencerPresetMaxAssetBytes = 10 << 20
	influencerPresetNameMaxLength = 120
	influencerPresetKeyPrefix     = "influencer/presets/"
)

// influencerPresetAssetExtensions is both the accepted-type allowlist and the
// extension table. The type is sniffed from the bytes, not taken from the
// multipart header.
var influencerPresetAssetExtensions = map[string]string{
	"image/png":  "png",
	"image/jpeg": "jpg",
	"image/webp": "webp",
}

// ListInfluencerPresets handles GET /api/influencer/presets.
// Every signed-in user sees the published characters; only the projection is
// exposed, so storage keys and uploader ids stay server-side.
func ListInfluencerPresets(c *gin.Context) {
	presets, err := model.ListInfluencerPresets()
	if err != nil {
		influencerError(c, http.StatusInternalServerError, "internal_error", "failed to read the inspiration library")
		return
	}
	items := make([]gin.H, 0, len(presets))
	for i := range presets {
		items = append(items, influencerPresetProjection(&presets[i]))
		if !influencerPresetThumbnailReady(&presets[i]) && presets[i].ThumbnailAttempts < 3 {
			enqueueInfluencerPresetThumbnail(presets[i].Id)
		}
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"items": items}})
}

// PublishInfluencerPreset handles POST /api/influencer/presets.
// The caller supplies only the artwork bytes and a display name: the tier,
// option selection, brief and seed come from the task the row points at, so a
// published character always reproduces what was actually generated.
func PublishInfluencerPreset(c *gin.Context) {
	if !limitFileUploadBody(c, 2*influencerPresetMaxAssetBytes+(1<<20)) {
		return
	}

	taskID := strings.TrimSpace(c.PostForm("task_id"))
	name := strings.TrimSpace(c.PostForm("name"))
	if taskID == "" {
		influencerError(c, http.StatusBadRequest, "invalid_param", "task_id is required")
		return
	}
	if name == "" {
		influencerError(c, http.StatusBadRequest, "invalid_param", "a preset name is required")
		return
	}
	if len([]rune(name)) > influencerPresetNameMaxLength {
		influencerError(c, http.StatusBadRequest, "invalid_param", "the preset name is too long")
		return
	}

	var task model.Task
	err := model.DB.Where("task_id = ? AND user_id = ?", taskID, c.GetInt("id")).First(&task).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		influencerError(c, http.StatusNotFound, "task_not_found", "task not found")
		return
	}
	if err != nil {
		influencerError(c, http.StatusInternalServerError, "internal_error", "failed to read the task")
		return
	}
	if task.Status != model.TaskStatusSuccess || task.Action != ActionInfluencerSheet {
		influencerError(c, http.StatusBadRequest, "task_not_publishable", "this record is not a finished character")
		return
	}
	meta, ok := parseInfluencerTaskMeta(task.Properties.Input)
	// Only original character sheets can seed a library entry. Animal types
	// carry a complete default look even without additional feature picks.
	if !ok || (len(meta.Selection) == 0 && !influencer.IsAnimalTier(meta.Tier)) || meta.Kind == influencerTaskKindMultiView {
		influencerError(c, http.StatusBadRequest, "task_not_publishable", "this record carries no character parameters")
		return
	}
	if meta.Tier == "" {
		meta.Tier = "normal"
	}
	if !influencer.ValidTier(meta.Tier) {
		influencerError(c, http.StatusBadRequest, "task_not_publishable", "saved character type is unsupported")
		return
	}
	if influencer.IsAnimalTier(meta.Tier) {
		selection, err := influencer.ValidateSelection(meta.Tier, meta.Selection)
		if err != nil {
			influencerError(c, http.StatusBadRequest, "task_not_publishable", err.Error())
			return
		}
		meta.Selection = selection
	}
	published, err := model.GetInfluencerPresetBySourceTask(taskID)
	if err != nil {
		influencerError(c, http.StatusInternalServerError, "internal_error", "failed to read the inspiration library")
		return
	}
	if published != nil {
		influencerError(c, http.StatusConflict, "already_published", "this character is already in the inspiration library")
		return
	}

	previewFile, err := c.FormFile("preview")
	if err != nil {
		influencerError(c, http.StatusBadRequest, "invalid_param", "the character artwork is required")
		return
	}
	sheetFile := previewFile
	if candidate, formErr := c.FormFile("sheet"); formErr == nil && candidate != nil {
		sheetFile = candidate
	}

	preview, err := readInfluencerPresetAsset(previewFile)
	if err != nil {
		influencerError(c, http.StatusBadRequest, "invalid_param", "the character artwork was rejected: "+err.Error())
		return
	}
	sheet := preview
	if sheetFile != previewFile {
		sheet, err = readInfluencerPresetAsset(sheetFile)
		if err != nil {
			influencerError(c, http.StatusBadRequest, "invalid_param", "the character sheet was rejected: "+err.Error())
			return
		}
	}

	driver := storage.GetStorageDriver()
	if driver == nil {
		influencerError(c, http.StatusInternalServerError, "storage_failed", "no storage backend is available")
		return
	}
	ctx := c.Request.Context()

	previewKey := influencerPresetObjectKey(taskID, "preview", preview)
	if _, err = driver.Upload(ctx, previewKey, preview.data, preview.mime); err != nil {
		common.SysError("influencer preset upload failed: " + err.Error())
		influencerError(c, http.StatusInternalServerError, "storage_failed", "failed to store the character artwork")
		return
	}
	uploaded := []string{previewKey}

	// The board is only a second object when it is different artwork; otherwise
	// the card and the sheet share one stored render.
	sheetKey := previewKey
	if !bytes.Equal(sheet.data, preview.data) {
		sheetKey = influencerPresetObjectKey(taskID, "sheet", sheet)
		if _, err = driver.Upload(ctx, sheetKey, sheet.data, sheet.mime); err != nil {
			common.SysError("influencer preset upload failed: " + err.Error())
			influencerPresetDeleteObjects(ctx, driver, uploaded)
			influencerError(c, http.StatusInternalServerError, "storage_failed", "failed to store the character sheet")
			return
		}
		uploaded = append(uploaded, sheetKey)
	}

	previewWidth, previewHeight := influencerPresetImageSize(preview.data)
	sheetWidth, sheetHeight := influencerPresetImageSize(sheet.data)

	preset := &model.InfluencerPreset{
		Name:          name,
		Tier:          meta.Tier,
		Brief:         meta.Brief,
		Seed:          meta.Seed,
		PreviewKey:    previewKey,
		PreviewMime:   preview.mime,
		PreviewWidth:  previewWidth,
		PreviewHeight: previewHeight,
		SheetKey:      sheetKey,
		SheetMime:     sheet.mime,
		SheetWidth:    sheetWidth,
		SheetHeight:   sheetHeight,
		SourceTaskID:  taskID,
		CreatedBy:     c.GetInt("id"),
	}
	if err = preset.SetSelectionMap(meta.Selection); err != nil {
		influencerPresetDeleteObjects(ctx, driver, uploaded)
		influencerError(c, http.StatusInternalServerError, "internal_error", "failed to encode the character parameters")
		return
	}
	if err = model.CreateInfluencerPreset(preset); err != nil {
		common.SysError("influencer preset insert failed: " + err.Error())
		influencerPresetDeleteObjects(ctx, driver, uploaded)
		influencerError(c, http.StatusInternalServerError, "internal_error", "failed to publish the character")
		return
	}

	enqueueInfluencerPresetThumbnail(preset.Id)
	c.JSON(http.StatusOK, gin.H{"success": true, "data": influencerPresetProjection(preset)})
}

// DeleteInfluencerPreset handles DELETE /api/influencer/presets/:id.
// The row goes first: an orphaned object is invisible, an orphaned row is a
// broken card. Object removal is best-effort.
// influencerPresetRowID reads the row id from a route parameter.
//
// The library hands the client the same public identifier the grid shows
// ("pub-<row>"), so both that form and the bare row id are accepted: a client
// must be able to feed back the identifier it was given.
func influencerPresetRowID(raw string) (int64, bool) {
	value := strings.TrimPrefix(strings.TrimSpace(raw), "pub-")
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}

func DeleteInfluencerPreset(c *gin.Context) {
	id, ok := influencerPresetRowID(c.Param("id"))
	if !ok {
		influencerError(c, http.StatusBadRequest, "invalid_param", "invalid preset id")
		return
	}
	lock := &influencerPresetThumbnailLocks[uint64(id)%uint64(len(influencerPresetThumbnailLocks))]
	lock.Lock()
	defer lock.Unlock()
	preset, err := model.GetInfluencerPreset(id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		influencerError(c, http.StatusNotFound, "not_found", "preset not found")
		return
	}
	if err != nil {
		influencerError(c, http.StatusInternalServerError, "internal_error", "failed to read the preset")
		return
	}
	if err = model.DeleteInfluencerPreset(id); err != nil {
		influencerError(c, http.StatusInternalServerError, "internal_error", "failed to remove the preset")
		return
	}
	if driver := storage.GetStorageDriver(); driver != nil {
		influencerPresetDeleteObjects(c.Request.Context(), driver, []string{preset.PreviewKey, preset.SheetKey, preset.ThumbnailKey})
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// InfluencerPresetAsset handles GET /api/influencer/presets/:id/asset/:role.
// The gallery needs a stable image address, so the row's key is served here:
// an isolated object-storage origin is used directly, everything else streams
// through the gateway. Keys are never exposed to the client.
func InfluencerPresetAsset(c *gin.Context) {
	id, ok := influencerPresetRowID(c.Param("id"))
	if !ok {
		writeInfluencerPresetAssetError(c)
		return
	}
	role := strings.TrimSpace(c.Param("role"))
	if role != "preview" && role != "sheet" && role != "thumbnail" {
		writeInfluencerPresetAssetError(c)
		return
	}
	preset, err := model.GetInfluencerPreset(id)
	if err != nil || preset == nil {
		writeInfluencerPresetAssetError(c)
		return
	}

	key := preset.PreviewKey
	mimeType := preset.PreviewMime
	if role == "sheet" {
		key = preset.SheetKey
		mimeType = preset.SheetMime
	}
	if role == "thumbnail" {
		key = preset.ThumbnailKey
		mimeType = preset.ThumbnailMime
	}
	driver := storage.GetStorageDriver()
	if driver == nil || strings.TrimSpace(key) == "" {
		writeInfluencerPresetAssetError(c)
		return
	}

	// Preserve object-storage delivery only outside application and trusted
	// session origins, exactly like the shared file download route.
	if driver.IsR2() {
		if publicURL := driver.GetPublicURL(key); isIsolatedPublicFileURL(c, publicURL) {
			c.Redirect(http.StatusFound, publicURL)
			return
		}
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), time.Minute)
	defer cancel()
	reader, storedMime, size, err := driver.Get(ctx, key)
	if err != nil {
		writeInfluencerPresetAssetError(c)
		return
	}
	defer reader.Close()
	if strings.TrimSpace(mimeType) == "" {
		mimeType = storedMime
	}
	c.Header("Cache-Control", "public, max-age=31536000, immutable")
	c.Header("X-Content-Type-Options", "nosniff")
	c.DataFromReader(http.StatusOK, size, mimeType, reader, nil)
}

// influencerPresetProjection is the wire shape the inspiration library renders:
// the same fields the mirrored snapshot uses, plus the publication marker.
func influencerPresetProjection(preset *model.InfluencerPreset) gin.H {
	selection, err := preset.SelectionMap()
	if err != nil {
		common.SysError("influencer preset selection is unreadable: " + err.Error())
		selection = map[string][]string{}
	}
	projection := gin.H{
		"id":        "pub-" + strconv.FormatInt(preset.Id, 10),
		"name":      preset.Name,
		"tier":      preset.Tier,
		"selection": selection,
		"brief":     preset.Brief,
		"seed":      preset.Seed,
		"preview": gin.H{
			"path":   influencerPresetAssetPath(preset.Id, "preview"),
			"width":  preset.PreviewWidth,
			"height": preset.PreviewHeight,
		},
		"sheet": gin.H{
			"path":   influencerPresetAssetPath(preset.Id, "sheet"),
			"width":  preset.SheetWidth,
			"height": preset.SheetHeight,
		},
		"created_at": preset.CreatedAt,
		"published":  true,
	}
	if influencerPresetThumbnailReady(preset) {
		projection["thumbnail"] = gin.H{"path": influencerPresetAssetPath(preset.Id, "thumbnail") + "?v=" + influencerPresetDigest([]byte(preset.ThumbnailKey)), "width": preset.ThumbnailWidth, "height": preset.ThumbnailHeight}
	}
	return projection
}

func influencerPresetAssetPath(id int64, role string) string {
	return "/api/influencer/presets/" + strconv.FormatInt(id, 10) + "/asset/" + role
}

// influencerPresetAsset is one uploaded artwork, already type-checked.
type influencerPresetAsset struct {
	data []byte
	mime string
}

// readInfluencerPresetAsset reads one multipart part under the size bound and
// decides its type from the bytes, so a mislabelled upload cannot be stored as
// something the browser would then trust.
func readInfluencerPresetAsset(file *multipart.FileHeader) (*influencerPresetAsset, error) {
	if file == nil {
		return nil, errors.New("the artwork is missing")
	}
	if file.Size > influencerPresetMaxAssetBytes {
		return nil, fmt.Errorf("the artwork exceeds %d bytes", influencerPresetMaxAssetBytes)
	}
	source, err := file.Open()
	if err != nil {
		return nil, errors.New("the artwork could not be read")
	}
	defer source.Close()
	data, err := io.ReadAll(io.LimitReader(source, influencerPresetMaxAssetBytes+1))
	if err != nil {
		return nil, errors.New("the artwork could not be read")
	}
	if len(data) == 0 {
		return nil, errors.New("the artwork is empty")
	}
	if len(data) > influencerPresetMaxAssetBytes {
		return nil, fmt.Errorf("the artwork exceeds %d bytes", influencerPresetMaxAssetBytes)
	}
	mimeType := strings.ToLower(strings.TrimSpace(strings.Split(http.DetectContentType(data), ";")[0]))
	if _, ok := influencerPresetAssetExtensions[mimeType]; !ok {
		return nil, fmt.Errorf("unsupported artwork type %q", mimeType)
	}
	return &influencerPresetAsset{data: data, mime: mimeType}, nil
}

// influencerPresetObjectKey is content addressed, like the mirrored option
// artwork: the same character published twice would reuse the same object.
func influencerPresetObjectKey(taskID, role string, asset *influencerPresetAsset) string {
	extension := influencerPresetAssetExtensions[asset.mime]
	if extension == "" {
		extension = "bin"
	}
	return influencerPresetKeyPrefix +
		influencerPresetDigest([]byte(strings.TrimSpace(taskID))) + "/" +
		role + "-" + influencerPresetDigest(asset.data) + "." + extension
}

func influencerPresetDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])[:16]
}

// influencerPresetImageSize reads the pixel size from the image header. A
// format the decoder does not know keeps 0/0 and the gallery falls back to
// natural sizing.
func influencerPresetImageSize(data []byte) (int, int) {
	config, _, err := service.DecodeImageConfig(data)
	if err != nil {
		return 0, 0
	}
	return config.Width, config.Height
}

func influencerPresetDeleteObjects(ctx context.Context, driver storage.StorageDriver, keys []string) {
	seen := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		if strings.TrimSpace(key) == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		if err := driver.Delete(ctx, key); err != nil {
			common.SysError("influencer preset object cleanup failed: " + err.Error())
		}
	}
}

func writeInfluencerPresetAssetError(c *gin.Context) {
	c.Header("Cache-Control", "private, no-store")
	c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "asset not found"})
}
