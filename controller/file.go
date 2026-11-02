package controller

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/storage"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	MaxUploadBytes                   = 100 * 1024 * 1024 // Ordinary files and Base64 remain capped at 100MiB.
	maxUploadMetadataBytes           = 1 << 20
	maxBase64UploadBodyBytes         = (MaxUploadBytes+2)/3*4 + maxUploadMetadataBytes
	maxMultipartUploadBodyBytes      = service.InfluencerVideoMaxBytes + maxUploadMetadataBytes
	influencerReferenceUploadPurpose = "influencer_reference_video"
	influencerReferenceUploadPath    = "influencer-reference/browser-upload"
)

// fork: reference videos use the provider's 200MiB budget without increasing
// the ordinary-file or Base64 limit. Declarations only set a reading budget;
// ordinary uploads above the limit must also identify an MP4/MOV container.
// The workbench browser-check purpose is handled separately without media reads.
func fileUploadSizeLimit(mimeType string, filenames ...string) int64 {
	mimeType = strings.ToLower(strings.TrimSpace(strings.SplitN(mimeType, ";", 2)[0]))
	if mimeType == "video/mp4" || mimeType == "video/quicktime" || mimeType == "video/x-quicktime" {
		return service.InfluencerVideoMaxBytes
	}
	for _, name := range filenames {
		ext := strings.ToLower(filepath.Ext(name))
		if ext == ".mp4" || ext == ".mov" {
			return service.InfluencerVideoMaxBytes
		}
	}
	return MaxUploadBytes
}

type uploadVideoEvidence struct {
	movieHeader  bool
	videoHandler bool
}

// inspectUploadVideoBoxes identifies BMFF brands and classic QuickTime movies.
// Box lengths stay inside the known object size. Front-indexed files finish
// after a small header; MP4/MOV can also place the movie index after media data.
// This is retained for ordinary uploads; browser-checked workbench references
// bypass server container inspection.
func inspectUploadVideoBoxes(reader io.Reader, size int64, path string, remainingBoxes *int) (uploadVideoEvidence, error) {
	var evidence uploadVideoEvidence
	for size >= 8 {
		if *remainingBoxes <= 0 {
			return uploadVideoEvidence{}, nil
		}
		*remainingBoxes -= 1
		var header [16]byte
		if _, err := io.ReadFull(reader, header[:8]); err != nil {
			return evidence, err
		}
		boxSize, headerSize := int64(binary.BigEndian.Uint32(header[:4])), int64(8)
		if boxSize == 1 {
			if size < 16 {
				return uploadVideoEvidence{}, nil
			}
			if _, err := io.ReadFull(reader, header[8:16]); err != nil {
				return evidence, err
			}
			extendedSize := binary.BigEndian.Uint64(header[8:16])
			if extendedSize > uint64(size) {
				return uploadVideoEvidence{}, nil
			}
			boxSize, headerSize = int64(extendedSize), 16
		} else if boxSize == 0 {
			boxSize = size
		}
		if boxSize < headerSize || boxSize > size {
			return uploadVideoEvidence{}, nil
		}
		payload := &io.LimitedReader{R: reader, N: boxSize - headerSize}
		boxType := string(header[4:8])
		switch {
		case path == "" && boxType == "ftyp":
			if payload.N < 8 || payload.N%4 != 0 {
				return uploadVideoEvidence{}, nil
			}
			prefix, err := io.ReadAll(io.LimitReader(payload, 512))
			if err != nil {
				return evidence, err
			}
			if len(prefix) < 8 {
				return uploadVideoEvidence{}, nil
			}
			majorBrand := string(prefix[:4])
			switch majorBrand {
			case "M4A ", "M4B ", "M4P ", "avif", "avis", "heic", "heix", "hevc", "hevx", "mif1", "msf1":
				return uploadVideoEvidence{}, nil
			}
			knownBrand := http.DetectContentType(append(header[:headerSize:headerSize], prefix...)) == "video/mp4"
			for offset := 0; offset+4 <= len(prefix); offset += 4 {
				if offset == 4 {
					continue // minor_version is not a compatible brand.
				}
				switch string(prefix[offset : offset+4]) {
				case "qt  ", "isom", "iso2", "iso3", "iso4", "iso5", "iso6", "iso7", "iso8", "iso9", "mp41", "mp42", "avc1", "hvc1", "hev1":
					knownBrand = true
				}
			}
			if !knownBrand {
				return uploadVideoEvidence{}, nil
			}
		case (path == "" && boxType == "moov") || (path == "moov" && boxType == "trak") || (path == "moov/trak" && boxType == "mdia"):
			childPath := boxType
			if path != "" {
				childPath = path + "/" + boxType
			}
			child, err := inspectUploadVideoBoxes(payload, payload.N, childPath, remainingBoxes)
			if err != nil {
				return evidence, err
			}
			evidence.movieHeader = evidence.movieHeader || child.movieHeader
			evidence.videoHandler = evidence.videoHandler || child.videoHandler
			if path == "" && evidence.movieHeader && evidence.videoHandler {
				return evidence, nil
			}
		case path == "moov" && boxType == "mvhd" && payload.N >= 100:
			var version [1]byte
			if _, err := io.ReadFull(payload, version[:]); err != nil {
				return evidence, err
			}
			evidence.movieHeader = version[0] == 0 || (version[0] == 1 && boxSize-headerSize >= 112)
		case path == "moov/trak/mdia" && boxType == "hdlr" && payload.N >= 24:
			var handler [12]byte
			if _, err := io.ReadFull(payload, handler[:]); err != nil {
				return evidence, err
			}
			evidence.videoHandler = string(handler[8:12]) == "vide"
		}
		if _, err := io.CopyN(io.Discard, payload, payload.N); err != nil {
			return evidence, err
		}
		size -= boxSize
	}
	return evidence, nil
}

func inspectUploadVideoContainer(reader io.Reader, size int64) (bool, error) {
	remainingBoxes := 4096
	evidence, err := inspectUploadVideoBoxes(reader, size, "", &remainingBoxes)
	return err == nil && evidence.movieHeader && evidence.videoHandler, err
}

func validFileUploadSize(data []byte, mimeType string, filenames ...string) bool {
	if int64(len(data)) > fileUploadSizeLimit(mimeType, filenames...) {
		return false
	}
	if len(data) <= MaxUploadBytes {
		return true
	}
	isVideo, err := inspectUploadVideoContainer(bytes.NewReader(data), int64(len(data)))
	return err == nil && isVideo
}

// browserReferenceUpload checks request declarations only. The reserved path is
// assigned by the server and persists the browser-check policy in the ticket;
// confirmation cannot opt a general upload out of its existing content check.
func browserReferenceUpload(purpose string, metadata *service.InfluencerVideoMetadata, mimeType string, size int64) (bool, error) {
	if purpose != influencerReferenceUploadPurpose {
		return false, nil
	}
	_, _, err := influencerBrowserVideoOutput(influencerMotionRequest{Mode: "motion", VideoMetadata: metadata}, "480p")
	if err != nil {
		return false, err
	}
	mimeType = strings.ToLower(strings.TrimSpace(strings.SplitN(mimeType, ";", 2)[0]))
	matches := metadata.Container == "mp4" && mimeType == "video/mp4" || metadata.Container == "mov" && (mimeType == "video/quicktime" || mimeType == "video/x-quicktime")
	if size != metadata.SizeBytes || !matches {
		return false, errors.New("Reference video upload information does not match the file")
	}
	return true, nil
}

// browserVideoMetadataJSON freezes a validated browser declaration for the
// file row and its response. Only the influencer-reference purpose carries one.
func browserVideoMetadataJSON(metadata *service.InfluencerVideoMetadata) (json.RawMessage, error) {
	if metadata == nil {
		return nil, nil
	}
	raw, err := common.Marshal(metadata)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(raw), nil
}

// fork: bound parsing at the shared handlers, including aliases without relay middleware.
func limitFileUploadBody(c *gin.Context, maxBytes int64) bool {
	if c.Request.ContentLength > maxBytes {
		fileUploadBodyTooLarge(c)
		return false
	}
	if c.Request.Body != nil {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
	}
	return true
}

func fileUploadBodyTooLarge(c *gin.Context) {
	c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, gin.H{
		"success": false, "code": http.StatusRequestEntityTooLarge, "msg": "上传内容超过大小上限",
	})
}

func decodeFileUploadBase64(data string, maxBytes int64) ([]byte, error) {
	var decodeErr error
	for _, encoding := range []*base64.Encoding{base64.StdEncoding, base64.URLEncoding} {
		decoded, err := io.ReadAll(io.LimitReader(base64.NewDecoder(encoding, strings.NewReader(data)), maxBytes+1))
		if int64(len(decoded)) > maxBytes {
			return nil, common.ErrRequestBodyTooLarge
		}
		if err == nil {
			return decoded, nil
		}
		decodeErr = err
	}
	return nil, decodeErr
}

// helper to generate unique file id
func generateFileID() string {
	id := strings.ReplaceAll(uuid.NewString(), "-", "")
	if len(id) > 16 {
		id = id[:16]
	}
	return fmt.Sprintf("file_%s", id)
}

// helper to resolve host URL
func buildDownloadURL(c *gin.Context, fileID string) string {
	scheme := "http"
	if c.Request.TLS != nil || strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}
	host := c.Request.Host
	if host == "" {
		host = "localhost"
	}
	return fmt.Sprintf("%s://%s/api/v1/files/download/%s", scheme, host, fileID)
}

// fork: public file delivery must not bypass the sandbox on an application origin.
func isIsolatedPublicFileURL(c *gin.Context, rawURL string) bool {
	publicHost, ok := canonicalFileDeliveryHostname(rawURL)
	if !ok {
		return false
	}

	applicationURLs := []string{buildDownloadURL(c, ""), system_setting.ServerAddress}
	applicationURLs = append(applicationURLs, common.SessionCookieTrustedURLs...)
	for _, rawApplicationURL := range applicationURLs {
		if strings.TrimSpace(rawApplicationURL) == "" {
			continue
		}
		applicationHost, ok := canonicalFileDeliveryHostname(rawApplicationURL)
		// Cookies are not isolated by port; even a different scheme/port on the
		// application host is unsuitable for publicly served untrusted files.
		if !ok || applicationHost == publicHost {
			return false
		}
	}
	return true
}

// Browsers canonicalize IDNs, numeric host aliases and padded ports differently
// from net/url. Keep ambiguous forms on the sandboxed gateway instead of
// treating two spellings of the same browser origin as separate origins.
func canonicalFileDeliveryHostname(rawURL string) (string, bool) {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.User != nil || parsed.Fragment != "" {
		return "", false
	}
	_, err = common.NormalizeOrigin(parsed.Scheme + "://" + parsed.Host)
	if err != nil {
		return "", false
	}
	if port := parsed.Port(); port != "" {
		number, err := strconv.Atoi(port)
		if err != nil || number > 65535 || strconv.Itoa(number) != port {
			return "", false
		}
	}
	hostname := parsed.Hostname()
	for _, char := range hostname {
		if char > 127 {
			return "", false
		}
	}
	hostname = strings.ToLower(hostname)
	if ip := net.ParseIP(hostname); ip != nil {
		if ip.String() != hostname {
			return "", false
		}
	} else {
		lastLabel := strings.TrimSuffix(hostname, ".")
		lastLabel = lastLabel[strings.LastIndex(lastLabel, ".")+1:]
		if strings.Contains(hostname, ":") || strings.Trim(lastLabel, "0123456789") == "" || strings.HasPrefix(lastLabel, "0x") {
			return "", false
		}
	}
	return strings.TrimSuffix(hostname, "."), true
}

// sanitizeFilename keeps only the base name and removes unsafe characters
func sanitizeFilename(name string) string {
	name = filepath.Base(filepath.Clean(name))
	name = strings.Trim(name, ". \t\r\n")
	if name == "" || name == "." {
		return ""
	}
	return name
}

// sanitizeUploadPath keeps path safe from traversal
func sanitizeUploadPath(p string, defaultCategory string) string {
	p = strings.TrimSpace(p)
	p = strings.Trim(p, "/\\")
	clean := filepath.Clean(p)
	if clean == "" || clean == "." || strings.HasPrefix(clean, "..") {
		return defaultCategory
	}
	clean = filepath.Clean(strings.ReplaceAll(clean, "\\", "/"))
	if clean == "task-artifacts" || strings.HasPrefix(clean, "task-artifacts/") || clean == "influencer-reference" || strings.HasPrefix(clean, "influencer-reference/") {
		return defaultCategory
	}
	return clean
}

// determineCategory determines default folder based on MIME type
func determineCategory(mimeType string) string {
	switch {
	case strings.HasPrefix(mimeType, "image/"):
		return "photos"
	case strings.HasPrefix(mimeType, "video/"):
		return "videos"
	case strings.HasPrefix(mimeType, "audio/"):
		return "audios"
	default:
		return "files"
	}
}

// determineExtension gets file extension from filename, MIME type or fallback
func determineExtension(filename string, mimeType string) string {
	if ext := filepath.Ext(filename); ext != "" {
		return strings.ToLower(strings.TrimPrefix(ext, "."))
	}
	switch mimeType {
	case "image/jpeg":
		return "jpg"
	case "image/png":
		return "png"
	case "image/gif":
		return "gif"
	case "image/webp":
		return "webp"
	case "video/mp4":
		return "mp4"
	case "video/quicktime", "video/x-quicktime":
		return "mov"
	case "audio/mpeg", "audio/mp3":
		return "mp3"
	case "audio/wav":
		return "wav"
	case "application/pdf":
		return "pdf"
	case "application/json":
		return "json"
	default:
		return "bin"
	}
}

// checkQuota verifies whether the user has exceeded the 2000 files quota limit
func checkQuota(userId int) error {
	count, err := model.CountActiveUserFiles(userId)
	if err != nil {
		return fmt.Errorf("failed to check file quota: %w", err)
	}
	if count >= model.DefaultUserMaxFiles {
		return fmt.Errorf("文件配额已耗尽，每个用户最多存储 %d 个文件", model.DefaultUserMaxFiles)
	}
	return nil
}

// saveAndRecordFile handles common storage uploading and database recording.
// videoMetadata is the validated browser declaration for the reserved
// influencer-reference purpose; every other caller passes nil.
func saveAndRecordFile(c *gin.Context, userId int, rawData []byte, mimeType string, rawOriginalName string, rawFileName string, rawUploadPath string, browserVideo bool, videoMetadata *service.InfluencerVideoMetadata) (*dto.FileInfoData, error) {
	if browserVideo && (len(rawData) == 0 || len(rawData) > service.InfluencerVideoMaxBytes) || !browserVideo && !validFileUploadSize(rawData, mimeType, rawOriginalName, rawFileName) {
		return nil, fmt.Errorf("文件大小超过上限（MP4/MOV 视频最大 200MiB，其他文件最大 100MiB）")
	}

	if err := checkQuota(userId); err != nil {
		return nil, err
	}

	ext := determineExtension(rawFileName, mimeType)
	if ext == "bin" && rawOriginalName != "" {
		ext = determineExtension(rawOriginalName, mimeType)
	}

	fileID := generateFileID()

	var fileName string
	cleanName := sanitizeFilename(rawFileName)
	if cleanName != "" {
		fileName = cleanName
	} else {
		fileName = fmt.Sprintf("%s.%s", fileID, ext)
	}

	originalName := sanitizeFilename(rawOriginalName)
	if originalName == "" {
		originalName = fileName
	}

	category := determineCategory(mimeType)
	uploadPath := sanitizeUploadPath(rawUploadPath, category)
	if browserVideo {
		uploadPath = influencerReferenceUploadPath
	}

	// Object key in storage: <uploadPath>/<fileID>_<fileName> to prevent collision across users
	storageKey := fmt.Sprintf("%s/%s_%s", uploadPath, fileID, fileName)

	driver := storage.GetStorageDriver()
	// fork: hiding an unsafe public URL cannot protect the raw object. Refuse
	// public application-host storage before any bytes are written.
	expectedPublicURL := driver.GetPublicURL(storageKey)
	if expectedPublicURL != "" && !isIsolatedPublicFileURL(c, expectedPublicURL) {
		return nil, errors.New("公开存储地址必须使用独立于本站的域名")
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()

	publicURL, err := driver.Upload(ctx, storageKey, rawData, mimeType)
	if err != nil {
		return nil, fmt.Errorf("上传至存储服务失败: %w", err)
	}
	if publicURL != expectedPublicURL || (publicURL != "" && !isIsolatedPublicFileURL(c, publicURL)) {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cleanupCancel()
		if err := driver.Delete(cleanupCtx, storageKey); err != nil {
			return nil, fmt.Errorf("存储服务返回的公开地址不一致，删除已上传文件失败: %w", err)
		}
		return nil, errors.New("存储服务返回的公开地址不一致，文件已删除")
	}

	downloadURL := buildDownloadURL(c, fileID)
	fileURL := publicURL
	if fileURL == "" {
		fileURL = downloadURL
	}

	now := time.Now()
	expiresAt := now.Add(model.DefaultFileExpireDuration)

	// The reserved browser-reference purpose persists the declaration it was
	// validated against; ordinary uploads keep the column NULL even if a caller
	// attached an unused metadata field.
	var storedMetadata json.RawMessage
	if browserVideo {
		raw, err := browserVideoMetadataJSON(videoMetadata)
		if err != nil {
			return nil, fmt.Errorf("记录参考视频信息失败: %w", err)
		}
		storedMetadata = raw
	}

	fileRecord := &model.File{
		FileId:        fileID,
		UserId:        userId,
		FileName:      fileName,
		OriginalName:  originalName,
		FileSize:      int64(len(rawData)),
		MimeType:      mimeType,
		StorageKey:    storageKey,
		UploadPath:    uploadPath,
		FileUrl:       fileURL,
		DownloadUrl:   downloadURL,
		CreatedAt:     now,
		ExpiresAt:     expiresAt,
		Status:        model.FileStatusActive,
		VideoMetadata: storedMetadata,
	}

	if err := model.CreateActiveFileWithinQuota(fileRecord); err != nil {
		_ = driver.Delete(context.Background(), storageKey)
		return nil, fmt.Errorf("记录文件信息失败: %w", err)
	}

	var echoedMetadata *service.InfluencerVideoMetadata
	if browserVideo {
		echoedMetadata = videoMetadata
	}

	return &dto.FileInfoData{
		FileID:        fileID,
		FileName:      fileName,
		OriginalName:  originalName,
		FileSize:      fileRecord.FileSize,
		MimeType:      fileRecord.MimeType,
		UploadPath:    uploadPath,
		FileURL:       fileURL,
		DownloadURL:   downloadURL,
		UploadTime:    now.Format(time.RFC3339),
		ExpiresAt:     expiresAt.Format(time.RFC3339),
		VideoMetadata: echoedMetadata,
	}, nil
}

// FileUploadBase64 POST /api/v1/files/upload/base64
func FileUploadBase64(c *gin.Context) {
	userId := c.GetInt("id")
	if userId <= 0 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"code":    http.StatusUnauthorized,
			"msg":     "未授权访问",
		})
		return
	}

	if !limitFileUploadBody(c, maxBase64UploadBodyBytes) {
		return
	}

	var req dto.FileUploadBase64Request
	if err := c.ShouldBindJSON(&req); err != nil {
		if common.IsRequestBodyTooLargeError(err) {
			fileUploadBodyTooLarge(c)
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"code":    http.StatusBadRequest,
			"msg":     "请求体格式错误: " + err.Error(),
		})
		return
	}

	base64Data := strings.TrimSpace(req.Base64Data)
	if base64Data == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"code":    http.StatusBadRequest,
			"msg":     "base64_data 不能为空",
		})
		return
	}

	var mimeType string
	cleanBase64 := base64Data
	if strings.HasPrefix(base64Data, "data:") {
		commaIdx := strings.Index(base64Data, ",")
		if commaIdx != -1 {
			header := base64Data[:commaIdx]
			cleanBase64 = base64Data[commaIdx+1:]
			if semiIdx := strings.Index(header, ";"); semiIdx != -1 {
				mimeType = strings.TrimPrefix(header[:semiIdx], "data:")
			}
		}
	}

	decodedBytes, err := decodeFileUploadBase64(cleanBase64, MaxUploadBytes)
	if err != nil {
		if common.IsRequestBodyTooLargeError(err) {
			fileUploadBodyTooLarge(c)
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false, "code": http.StatusBadRequest, "msg": "Base64 数据解码失败: " + err.Error(),
		})
		return
	}

	if mimeType == "" {
		mimeType = http.DetectContentType(decodedBytes)
		if idx := strings.Index(mimeType, ";"); idx != -1 {
			mimeType = strings.TrimSpace(mimeType[:idx])
		}
	}

	resData, err := saveAndRecordFile(c, userId, decodedBytes, mimeType, req.FileName, req.FileName, req.UploadPath, false, nil)
	if err != nil {
		code := http.StatusInternalServerError
		if strings.Contains(err.Error(), "配额已耗尽") {
			code = http.StatusTooManyRequests
		}
		c.JSON(code, gin.H{
			"success": false,
			"code":    code,
			"msg":     err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, dto.FileUploadResponse{
		Success: true,
		Code:    http.StatusOK,
		Msg:     "文件上传成功",
		Data:    resData,
	})
}

// FileUploadStream POST /api/v1/files/upload/stream
func FileUploadStream(c *gin.Context) {
	userId := c.GetInt("id")
	if userId <= 0 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"code":    http.StatusUnauthorized,
			"msg":     "未授权访问",
		})
		return
	}

	if !limitFileUploadBody(c, maxMultipartUploadBodyBytes) {
		return
	}
	defer func() {
		if c.Request.MultipartForm != nil {
			_ = c.Request.MultipartForm.RemoveAll()
		}
	}()

	fileHeader, err := c.FormFile("file")
	if err != nil {
		if common.IsRequestBodyTooLargeError(err) {
			fileUploadBodyTooLarge(c)
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"code":    http.StatusBadRequest,
			"msg":     "缺少 file 表单项: " + err.Error(),
		})
		return
	}

	uploadPath := c.PostForm("upload_path")
	if uploadPath == "" {
		uploadPath = c.PostForm("uploadPath")
	}
	fileName := c.PostForm("file_name")
	if fileName == "" {
		fileName = c.PostForm("fileName")
	}
	if fileName == "" {
		fileName = fileHeader.Filename
	}

	mimeType := fileHeader.Header.Get("Content-Type")
	var declared *service.InfluencerVideoMetadata
	if c.PostForm("purpose") == influencerReferenceUploadPurpose {
		if err := common.UnmarshalJsonStr(c.PostForm("video_metadata"), &declared); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "code": http.StatusBadRequest, "msg": "Reference video information is missing; add the video again"})
			return
		}
	}
	browserVideo, err := browserReferenceUpload(c.PostForm("purpose"), declared, mimeType, fileHeader.Size)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "code": http.StatusBadRequest, "msg": err.Error()})
		return
	}

	file, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"code":    http.StatusBadRequest,
			"msg":     "打开上传文件失败: " + err.Error(),
		})
		return
	}
	defer file.Close()

	rawData, err := io.ReadAll(io.LimitReader(file, fileUploadSizeLimit(mimeType, fileHeader.Filename, fileName)+1))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"code":    http.StatusBadRequest,
			"msg":     "读取文件内容失败: " + err.Error(),
		})
		return
	}
	if !browserVideo && !validFileUploadSize(rawData, mimeType, fileHeader.Filename, fileName) {
		fileUploadBodyTooLarge(c)
		return
	}

	if !browserVideo && (mimeType == "" || mimeType == "application/octet-stream") {
		mimeType = http.DetectContentType(rawData)
	}
	if idx := strings.Index(mimeType, ";"); idx != -1 {
		mimeType = strings.TrimSpace(mimeType[:idx])
	}

	resData, err := saveAndRecordFile(c, userId, rawData, mimeType, fileHeader.Filename, fileName, uploadPath, browserVideo, declared)
	if err != nil {
		code := http.StatusInternalServerError
		if strings.Contains(err.Error(), "配额已耗尽") {
			code = http.StatusTooManyRequests
		}
		c.JSON(code, gin.H{
			"success": false,
			"code":    code,
			"msg":     err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, dto.FileUploadResponse{
		Success: true,
		Code:    http.StatusOK,
		Msg:     "文件上传成功",
		Data:    resData,
	})
}

// FileUploadURL POST /api/v1/files/upload/url
func FileUploadURL(c *gin.Context) {
	userId := c.GetInt("id")
	if userId <= 0 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"code":    http.StatusUnauthorized,
			"msg":     "未授权访问",
		})
		return
	}

	if !limitFileUploadBody(c, maxUploadMetadataBytes) {
		return
	}

	var req dto.FileUploadURLRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		if common.IsRequestBodyTooLargeError(err) {
			fileUploadBodyTooLarge(c)
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"code":    http.StatusBadRequest,
			"msg":     "请求体格式错误: " + err.Error(),
		})
		return
	}

	rawURL := strings.TrimSpace(req.FileURL)
	if rawURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"code":    http.StatusBadRequest,
			"msg":     "file_url 不能为空",
		})
		return
	}

	parsedURL, err := url.Parse(rawURL)
	if err != nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"code":    http.StatusBadRequest,
			"msg":     "file_url 必须是有效的 HTTP 或 HTTPS 链接",
		})
		return
	}

	// Download remote file using service.DoDownloadRequest
	resp, err := service.DoDownloadRequest(rawURL, "file_upload_url")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"code":    http.StatusBadRequest,
			"msg":     "下载远程文件失败: " + err.Error(),
		})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"code":    http.StatusBadRequest,
			"msg":     fmt.Sprintf("下载远程文件失败，状态码: %d", resp.StatusCode),
		})
		return
	}

	mimeType := resp.Header.Get("Content-Type")
	urlFileName := filepath.Base(parsedURL.Path)
	if urlFileName == "/" || urlFileName == "." {
		urlFileName = ""
	}
	fileName := req.FileName
	if fileName == "" {
		fileName = urlFileName
	}
	rawData, err := io.ReadAll(io.LimitReader(resp.Body, fileUploadSizeLimit(mimeType, urlFileName, fileName)+1))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"code":    http.StatusBadRequest,
			"msg":     "读取远程文件内容失败: " + err.Error(),
		})
		return
	}
	if !validFileUploadSize(rawData, mimeType, urlFileName, fileName) {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"code":    http.StatusBadRequest,
			"msg":     "文件大小超过上限（MP4/MOV 视频最大 200MiB，其他文件最大 100MiB）",
		})
		return
	}

	if mimeType == "" || mimeType == "application/octet-stream" {
		mimeType = http.DetectContentType(rawData)
	}
	if idx := strings.Index(mimeType, ";"); idx != -1 {
		mimeType = strings.TrimSpace(mimeType[:idx])
	}

	resData, err := saveAndRecordFile(c, userId, rawData, mimeType, urlFileName, fileName, req.UploadPath, false, nil)
	if err != nil {
		code := http.StatusInternalServerError
		if strings.Contains(err.Error(), "配额已耗尽") {
			code = http.StatusTooManyRequests
		}
		c.JSON(code, gin.H{
			"success": false,
			"code":    code,
			"msg":     err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, dto.FileUploadResponse{
		Success: true,
		Code:    http.StatusOK,
		Msg:     "文件上传成功",
		Data:    resData,
	})
}

// fork: r2-file-storage-presign — direct upload tickets.
//
// The relay route makes the gateway carry every byte of a shared asset. These
// two routes split that into a signed ticket and a verification step: the caller
// puts the object straight into the bucket, and the gateway still ends up with
// the same file record, quota accounting and public address the relay produces.
const (
	fileUploadPresignDefaultTTLSeconds = 900
	fileUploadPresignMaxTTLSeconds     = 60 * 60
	// fileUploadConfirmMargin keeps a pending row alive past its ticket, so the
	// expiration sweep can never delete an object while its upload is running.
	fileUploadConfirmMargin = 15 * time.Minute
	fileUploadPresignTTLEnv = "FILE_UPLOAD_PRESIGN_TTL"
)

// fileUploadPresignTTL resolves the ticket lifetime; an out-of-range value keeps
// the default rather than handing out an unbounded or already-expired ticket.
func fileUploadPresignTTL() time.Duration {
	seconds := common.GetEnvOrDefault(fileUploadPresignTTLEnv, fileUploadPresignDefaultTTLSeconds)
	if seconds <= 0 || seconds > fileUploadPresignMaxTTLSeconds {
		seconds = fileUploadPresignDefaultTTLSeconds
	}
	return time.Duration(seconds) * time.Second
}

// fileUploadPresignUnsupported answers the only recovery a caller has when this
// deployment cannot hand out tickets: fall back to the relay route.
func fileUploadPresignUnsupported(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{
		"success": false,
		"code":    http.StatusNotImplemented,
		"msg":     "当前存储后端不支持直传，请改用 /upload/stream",
	})
}

// FilePresignUpload POST /api/v1/files/upload/presign
func FilePresignUpload(c *gin.Context) {
	userId := c.GetInt("id")
	if userId <= 0 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"code":    http.StatusUnauthorized,
			"msg":     "未授权访问",
		})
		return
	}

	if !limitFileUploadBody(c, maxUploadMetadataBytes) {
		return
	}
	var req struct {
		dto.FilePresignRequest
		Purpose       string                           `json:"purpose"`
		VideoMetadata *service.InfluencerVideoMetadata `json:"video_metadata"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"code":    http.StatusBadRequest,
			"msg":     "请求数据格式错误: " + err.Error(),
		})
		return
	}

	fileName := sanitizeFilename(req.FileName)
	if fileName == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"code":    http.StatusBadRequest,
			"msg":     "file_name 不能为空",
		})
		return
	}
	if req.FileSize <= 0 || req.FileSize > fileUploadSizeLimit(req.MimeType, fileName) {
		fileUploadBodyTooLarge(c)
		return
	}

	// The content type is part of the ticket signature, so it must be known
	// before the ticket is issued rather than sniffed from the bytes later.
	mimeType := strings.TrimSpace(req.MimeType)
	if idx := strings.Index(mimeType, ";"); idx != -1 {
		mimeType = strings.TrimSpace(mimeType[:idx])
	}
	mimeType = strings.ToLower(mimeType)
	if mimeType == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"code":    http.StatusBadRequest,
			"msg":     "mime_type 不能为空",
		})
		return
	}

	browserVideo, err := browserReferenceUpload(req.Purpose, req.VideoMetadata, mimeType, req.FileSize)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "code": http.StatusBadRequest, "msg": err.Error()})
		return
	}

	if err := checkQuota(userId); err != nil {
		c.JSON(http.StatusTooManyRequests, gin.H{
			"success": false,
			"code":    http.StatusTooManyRequests,
			"msg":     err.Error(),
		})
		return
	}

	driver := storage.GetStorageDriver()
	fileID := generateFileID()
	category := determineCategory(mimeType)
	uploadPath := sanitizeUploadPath(req.UploadPath, category)
	if browserVideo {
		uploadPath = influencerReferenceUploadPath
	}
	// Same key shape as the relay route: the client never chooses its own key.
	storageKey := fmt.Sprintf("%s/%s_%s", uploadPath, fileID, fileName)

	// An unconfigured public origin is not a capability gap: the relay route
	// publishes through the gateway download address in that case, and direct
	// upload keeps that same fallback. An origin that IS configured must still be
	// isolated from the application host.
	expectedPublicURL := driver.GetPublicURL(storageKey)
	if expectedPublicURL != "" && !isIsolatedPublicFileURL(c, expectedPublicURL) {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"code":    http.StatusBadRequest,
			"msg":     "公开存储地址必须使用独立于本站的域名",
		})
		return
	}

	ttl := fileUploadPresignTTL()
	ticket, err := driver.PresignPut(c.Request.Context(), storageKey, mimeType, ttl)
	if err != nil {
		if errors.Is(err, storage.ErrPresignUnsupported) {
			fileUploadPresignUnsupported(c)
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"code":    http.StatusInternalServerError,
			"msg":     "签发上传凭证失败: " + err.Error(),
		})
		return
	}

	now := time.Now()
	downloadURL := buildDownloadURL(c, fileID)
	// The published address is whatever the relay route would have recorded: the
	// storage origin when one is configured, the gateway download address when
	// the deployment serves objects through itself.
	resourceURL := expectedPublicURL
	if resourceURL == "" {
		resourceURL = downloadURL
	}
	// Freeze the declaration the ticket was validated against so confirmation
	// and later consumers reuse it instead of re-reading the media.
	var videoMetadata json.RawMessage
	if browserVideo {
		raw, err := browserVideoMetadataJSON(req.VideoMetadata)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"code":    http.StatusInternalServerError,
				"msg":     "记录上传凭证失败: " + err.Error(),
			})
			return
		}
		videoMetadata = raw
	}
	record := &model.File{
		FileId:        fileID,
		UserId:        userId,
		FileName:      fileName,
		OriginalName:  fileName,
		FileSize:      req.FileSize,
		MimeType:      mimeType,
		StorageKey:    storageKey,
		UploadPath:    uploadPath,
		FileUrl:       resourceURL,
		DownloadUrl:   downloadURL,
		CreatedAt:     now,
		ExpiresAt:     now.Add(ttl + fileUploadConfirmMargin),
		Status:        model.FileStatusUploadPending,
		VideoMetadata: videoMetadata,
	}
	if err := model.CreateFile(record); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"code":    http.StatusInternalServerError,
			"msg":     "记录上传凭证失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, dto.FilePresignResponse{
		Success: true,
		Code:    http.StatusOK,
		Msg:     "上传凭证已签发",
		Data: &dto.FilePresignInfoData{
			FileID:        fileID,
			UploadURL:     ticket.URL,
			UploadMethod:  ticket.Method,
			UploadHeaders: ticket.Headers,
			ResourceURL:   resourceURL,
			DownloadURL:   downloadURL,
			ExpiresAt:     ticket.ExpiresAt.UTC().Format(time.RFC3339),
		},
	})
}

// FileConfirmUpload POST /api/v1/files/upload/confirm
//
// An issued ticket is a promise, not a fact: this is where the promise is
// checked against what storage actually holds. Anything that does not match is
// refused and released, so no orphan object and no phantom quota survive it.
func FileConfirmUpload(c *gin.Context) {
	userId := c.GetInt("id")
	if userId <= 0 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"code":    http.StatusUnauthorized,
			"msg":     "未授权访问",
		})
		return
	}

	if !limitFileUploadBody(c, maxUploadMetadataBytes) {
		return
	}
	var req dto.FileConfirmUploadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"code":    http.StatusBadRequest,
			"msg":     "请求数据格式错误: " + err.Error(),
		})
		return
	}
	fileID := strings.TrimSpace(req.FileID)
	if fileID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"code":    http.StatusBadRequest,
			"msg":     "file_id 不能为空",
		})
		return
	}

	record, err := model.GetFileByFileId(fileID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"code":    http.StatusNotFound,
				"msg":     "文件不存在或已删除",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"code":    http.StatusInternalServerError,
			"msg":     "查询文件信息失败",
		})
		return
	}
	// Someone else's ticket is reported as missing, not as forbidden.
	if record.UserId != userId {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"code":    http.StatusNotFound,
			"msg":     "文件不存在或已删除",
		})
		return
	}
	if record.Status != model.FileStatusUploadPending {
		c.JSON(http.StatusConflict, gin.H{
			"success": false,
			"code":    http.StatusConflict,
			"msg":     "该上传凭证已确认或已失效",
		})
		return
	}
	if time.Now().After(record.ExpiresAt) {
		_, _ = model.RejectPendingFileUpload(record.Id)
		c.JSON(http.StatusConflict, gin.H{
			"success": false,
			"code":    http.StatusConflict,
			"msg":     "上传凭证已过期，请重新申请",
		})
		return
	}
	if err := checkQuota(userId); err != nil {
		_, _ = model.RejectPendingFileUpload(record.Id)
		c.JSON(http.StatusTooManyRequests, gin.H{
			"success": false,
			"code":    http.StatusTooManyRequests,
			"msg":     err.Error(),
		})
		return
	}

	driver := storage.GetStorageDriver()
	// Best-effort cleanup: a rejected upload must not leave an object behind.
	discardObject := func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()
		if err := driver.Delete(cleanupCtx, record.StorageKey); err != nil {
			common.SysError(fmt.Sprintf("failed to delete rejected direct upload (key=%s): %v", record.StorageKey, err))
		}
	}
	rejectUpload := func(message string) {
		if rejected, rejectErr := model.RejectPendingFileUpload(record.Id); rejectErr == nil && rejected {
			discardObject()
		}
		c.JSON(http.StatusConflict, gin.H{
			"success": false,
			"code":    http.StatusConflict,
			"msg":     message,
		})
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	// R2 temporary read failures leave the same ticket eligible for confirmation.
	metadata, err := storage.InspectObject(ctx, driver, record.StorageKey)
	if err != nil {
		if _, metadataOnly := driver.(storage.MetadataReader); metadataOnly && !errors.Is(err, storage.ErrObjectNotFound) {
			c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "code": http.StatusServiceUnavailable, "msg": "存储暂时不可用，请重试确认上传"})
			return
		}
		_, _ = model.RejectPendingFileUpload(record.Id)
		c.JSON(http.StatusConflict, gin.H{
			"success": false,
			"code":    http.StatusConflict,
			"msg":     "未检测到已上传的文件，请重新申请上传凭证",
		})
		return
	}
	size, storedMimeType := metadata.Size, metadata.ContentType

	if size <= 0 || size > fileUploadSizeLimit(record.MimeType, record.FileName) {
		rejectUpload("上传内容大小不合法，请重新上传")
		return
	}
	if record.FileSize != size || (req.FileSize > 0 && req.FileSize != record.FileSize) {
		rejectUpload("实际上传大小与声明不一致，请重新上传")
		return
	}

	finalMimeType := strings.TrimSpace(storedMimeType)
	if finalMimeType != record.MimeType {
		rejectUpload("实际上传类型与声明不一致，请重新上传")
		return
	}
	if size > MaxUploadBytes && record.UploadPath != influencerReferenceUploadPath {
		reader, _, actualSize, readErr := driver.Get(ctx, record.StorageKey)
		if readErr != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "code": http.StatusServiceUnavailable, "msg": "存储暂时不可用，请重试确认上传"})
			return
		}
		isVideo, inspectErr := inspectUploadVideoContainer(reader, size)
		_ = reader.Close()
		if inspectErr != nil && !errors.Is(inspectErr, io.EOF) && !errors.Is(inspectErr, io.ErrUnexpectedEOF) {
			c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "code": http.StatusServiceUnavailable, "msg": "存储暂时不可用，请重试确认上传"})
			return
		}
		if actualSize != size || inspectErr != nil || !isVideo {
			rejectUpload("超过 100MiB 的文件必须是有效的 MP4/MOV 视频，请重新上传")
			return
		}
	}

	// A browser-reference ticket promises a checked declaration. A pending row
	// without one (for example issued by a process from before the declaration
	// was persisted) cannot confirm as checked; the caller must upload again.
	if record.UploadPath == influencerReferenceUploadPath && len(record.VideoMetadata) == 0 {
		rejectUpload("Reference video information is missing; add the video again")
		return
	}

	var videoMetadata *service.InfluencerVideoMetadata
	if len(record.VideoMetadata) > 0 {
		var persisted service.InfluencerVideoMetadata
		if err := common.Unmarshal(record.VideoMetadata, &persisted); err == nil {
			videoMetadata = &persisted
		}
	}

	now := time.Now()
	expiresAt := now.Add(model.DefaultFileExpireDuration)
	if err := model.ConfirmDirectFileUpload(record.Id, userId, size, finalMimeType, expiresAt); err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "配额已耗尽") {
			status = http.StatusTooManyRequests
		}
		if err.Error() == "upload is no longer pending" {
			status = http.StatusConflict
		}
		c.JSON(status, gin.H{
			"success": false,
			"code":    status,
			"msg":     "记录文件信息失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, dto.FileUploadResponse{
		Success: true,
		Code:    http.StatusOK,
		Msg:     "文件上传成功",
		Data: &dto.FileInfoData{
			FileID:        record.FileId,
			FileName:      record.FileName,
			OriginalName:  record.OriginalName,
			FileSize:      size,
			MimeType:      finalMimeType,
			UploadPath:    record.UploadPath,
			FileURL:       record.FileUrl,
			DownloadURL:   record.DownloadUrl,
			UploadTime:    now.Format(time.RFC3339),
			ExpiresAt:     expiresAt.Format(time.RFC3339),
			VideoMetadata: videoMetadata,
		},
	})
}

// FileGetQuota GET /api/v1/files/quota
func FileGetQuota(c *gin.Context) {
	userId := c.GetInt("id")
	if userId <= 0 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"code":    http.StatusUnauthorized,
			"msg":     "未授权访问",
		})
		return
	}

	usedFiles, err := model.CountActiveUserFiles(userId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"code":    http.StatusInternalServerError,
			"msg":     "获取配额信息失败: " + err.Error(),
		})
		return
	}

	maxFiles := int64(model.DefaultUserMaxFiles)
	remainFiles := maxFiles - usedFiles
	if remainFiles < 0 {
		remainFiles = 0
	}

	userGroup := c.GetString("group")
	if userGroup == "" {
		userGroup = "default"
	}

	c.JSON(http.StatusOK, dto.FileQuotaResponse{
		Success: true,
		Code:    http.StatusOK,
		Data: &dto.FileQuotaData{
			UserGroup:   userGroup,
			UsedFiles:   usedFiles,
			MaxFiles:    maxFiles,
			RemainFiles: remainFiles,
			IsCustom:    false,
			QuotaReason: "",
		},
	})
}

// FileDownload GET /api/v1/files/download/:file_id
func FileDownload(c *gin.Context) {
	fileID := strings.TrimSpace(c.Param("file_id"))
	if fileID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"code":    http.StatusBadRequest,
			"msg":     "file_id 不能为空",
		})
		return
	}

	file, err := model.GetFileByFileId(fileID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"code":    http.StatusNotFound,
				"msg":     "文件不存在或已删除",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"code":    http.StatusInternalServerError,
			"msg":     "查询文件信息失败",
		})
		return
	}

	// Expiration check: 72 hours
	if file.Status != model.FileStatusActive || time.Now().After(file.ExpiresAt) {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"code":    http.StatusNotFound,
			"msg":     "文件已过期",
		})
		return
	}

	driver := storage.GetStorageDriver()

	if c.Request.Method == http.MethodGet && c.Query("delivery") == "direct" && driver.IsR2() {
		if signer, ok := driver.(storage.DownloadPresigner); ok {
			ticket, signErr := signer.PresignGet(c.Request.Context(), file.StorageKey, file.MimeType, fmt.Sprintf("attachment; filename=%q", sanitizeFilename(file.FileName)), 5*time.Minute)
			if signErr == nil && ticket != nil && ticket.Method == http.MethodGet && strings.HasPrefix(ticket.URL, "https://") && isIsolatedPublicFileURL(c, ticket.URL) {
				c.Header("Cache-Control", "private, no-store")
				c.Header("Referrer-Policy", "no-referrer")
				c.Redirect(http.StatusTemporaryRedirect, ticket.URL)
				return
			}
		}
	}
	// Preserve R2 public delivery only outside application and trusted session origins.
	if driver.IsR2() {
		publicURL := driver.GetPublicURL(file.StorageKey)
		if isIsolatedPublicFileURL(c, publicURL) {
			c.Redirect(http.StatusFound, publicURL)
			return
		}
	}

	// Stream from storage
	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()

	reader, mimeType, size, err := driver.Get(ctx, file.StorageKey)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"code":    http.StatusNotFound,
			"msg":     "读取文件存储内容失败",
		})
		return
	}
	defer reader.Close()

	finalMimeType := file.MimeType
	if finalMimeType == "" || finalMimeType == "application/octet-stream" {
		if mimeType != "" && mimeType != "application/octet-stream" {
			finalMimeType = mimeType
		}
	}
	if finalMimeType == "" {
		finalMimeType = "application/octet-stream"
	}

	// fork: every stored MIME, including legacy records, is untrusted browser content.
	setUntrustedMediaResponseSecurityHeaders(c.Writer.Header())
	c.Header("Content-Type", finalMimeType)
	c.Header("Content-Disposition", fmt.Sprintf("inline; filename=%q", file.FileName))
	if size > 0 {
		c.Header("Content-Length", fmt.Sprintf("%d", size))
	}
	_, _ = io.Copy(c.Writer, reader)
}
