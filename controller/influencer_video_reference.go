package controller

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/storage"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

var influencerVideoReferenceInspector = service.InspectInfluencerVideoReference

// Published preset filenames are content-derived keys. Uploaded files always
// include a file_ prefix, so an arbitrary upload cannot impersonate this namespace.
var influencerBuiltinVideoPath = regexp.MustCompile(`^/influencer/motion/[0-9a-f]{16}\.mp4$`)

// InspectInfluencerMotionVideo checks an authenticated user's controlled material
// without creating a generation, frozen copy, file record, or billing operation.
func InspectInfluencerMotionVideo(c *gin.Context) {
	userID := c.GetInt("id")
	if userID <= 0 {
		influencerError(c, http.StatusUnauthorized, "unauthorized", "未授权访问")
		return
	}
	var req struct {
		VideoURL string `json:"video_url"`
		Mode     string `json:"mode"`
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 8192)
	if err := common.UnmarshalBodyReusable(c, &req); err != nil {
		influencerAbort(c, "参考视频检查请求无效")
		return
	}
	if _, err := service.GetInfluencerVideoInputRules(req.Mode); err != nil {
		influencerAbort(c, err.Error())
		return
	}
	videoURL := strings.TrimSpace(req.VideoURL)
	if !isInfluencerMediaURL(videoURL) {
		influencerAbort(c, "参考视频地址不符合安全要求")
		return
	}
	inspectionURL, ok := resolveInfluencerVideoReferenceSource(c, videoURL, userID)
	if !ok {
		return
	}
	metadata, err := influencerVideoReferenceInspector(c.Request.Context(), inspectionURL, req.Mode)
	if err != nil {
		influencerError(c, service.InfluencerVideoInspectionStatus(err), "reference_video_invalid", err.Error())
		return
	}
	common.ApiSuccess(c, metadata)
}

// resolveInfluencerVideoReferenceSource authorizes the existing material
// capability without fetching or inspecting the video bytes.
func resolveInfluencerVideoReferenceSource(c *gin.Context, videoURL string, userID int) (string, bool) {
	inspectionURL, artifact, err := resolveInfluencerVideoArtifactReference(c, videoURL, userID)
	if err != nil {
		influencerError(c, http.StatusServiceUnavailable, "reference_video_unavailable", "参考素材记录暂不可用，请稍后重试")
		return "", false
	}
	if !artifact {
		if !isIsolatedPublicFileURL(c, videoURL) || service.ValidateSSRFProtectedFetchURL(videoURL) != nil {
			influencerAbort(c, "参考视频地址不符合安全要求")
			return "", false
		}
		allowed, err := influencerVideoReferenceSource(c.Request.Context(), videoURL, userID)
		if err != nil {
			influencerError(c, http.StatusServiceUnavailable, "reference_video_unavailable", "参考素材记录暂不可用，请稍后重试")
			return "", false
		}
		if !allowed {
			influencerError(c, http.StatusForbidden, "reference_video_forbidden", "请选择已上传的本人视频或视频预设")
			return "", false
		}
		inspectionURL = videoURL
	}
	if artifact && service.ValidateSSRFProtectedFetchURL(inspectionURL) != nil {
		influencerAbort(c, "参考视频地址不符合安全要求")
		return "", false
	}
	return inspectionURL, true
}

// History exposes signed task-artifact URLs, including on the application
// origin. Resolve only an exact server-issued video capability owned by this
// session. Stored bytes use their own isolated public origin; the fallback is
// the existing artifact handler, which resolves unpersisted provider content.
func resolveInfluencerVideoArtifactReference(c *gin.Context, raw string, userID int) (string, bool, error) {
	candidate, err := url.Parse(raw)
	if err != nil || candidate.Scheme != "https" || candidate.User != nil || candidate.Fragment != "" {
		return "", false, nil
	}
	parts := strings.Split(candidate.EscapedPath(), "/")
	if len(parts) < 7 {
		return "", false, nil
	}
	suffix := parts[len(parts)-6:]
	if suffix[0] != "v1" || suffix[1] != "tasks" || suffix[3] != "artifacts" || suffix[5] != "content" {
		return "", false, nil
	}
	taskID, taskErr := url.PathUnescape(suffix[2])
	artifactKey, keyErr := url.PathUnescape(suffix[4])
	if taskErr != nil || keyErr != nil || taskID == "" || len(taskID) > 191 || !taskArtifactKeyPattern.MatchString(artifactKey) {
		return "", false, nil
	}
	expected, err := service.BuildTaskArtifactContentURL(taskID, artifactKey)
	if err != nil || raw != expected || !service.VerifyTaskArtifactAccess(candidate.Query().Get(service.TaskArtifactAccessQueryParameter), taskID, artifactKey) {
		return "", false, nil
	}
	if model.DB == nil {
		return "", false, errors.New("task records unavailable")
	}
	task, exists, err := model.GetUniqueByOnlyTaskId(taskID)
	if err != nil || !exists || task == nil {
		return "", false, err
	}
	if task.UserId != userID || task.Status != model.TaskStatusSuccess || !task.ResultRetrievable() {
		return "", false, nil
	}
	owner, err := model.GetUserCache(userID)
	if err != nil {
		return "", false, err
	}
	if owner == nil || owner.Status != common.UserStatusEnabled {
		return "", false, nil
	}
	artifacts, legacyURL, err := resolveTaskArtifacts(task)
	if err != nil {
		return "", false, err
	}
	video := artifactKey == "video" && legacyURL == expected
	for _, item := range artifacts {
		if item.Key == artifactKey && item.Type == "video" && item.ContentURL == expected {
			video = true
			break
		}
	}
	if !video {
		return "", false, nil
	}
	if stored, err := service.GetTaskArtifactStore().Resolve(task, artifactKey); err == nil && stored != nil {
		publicURL := storage.GetStorageDriver().GetPublicURL(stored.ObjectKey)
		if isInfluencerMediaURL(publicURL) && isIsolatedPublicFileURL(c, publicURL) {
			return publicURL, true, nil
		}
	}
	return expected, true, nil
}

// A public URL alone grants no free server-side download. Inputs must be a
// published preset, an active owned upload, or an exact material URL previously
// recorded by one of this user's motion tasks. Expired/deleted uploads stay denied.
func influencerVideoReferenceSource(ctx context.Context, raw string, userID int) (bool, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return false, nil
	}
	if parsed.Host == "files.omnimux.ai" && parsed.RawQuery == "" && parsed.RawPath == "" && influencerBuiltinVideoPath.MatchString(parsed.Path) {
		return true, nil
	}
	if isPublishedInfluencerVideoReference(raw) {
		return true, nil
	}
	if model.DB == nil {
		return false, errors.New("file records unavailable")
	}
	var files []model.File
	if err := model.DB.WithContext(ctx).Where("file_url = ?", raw).Limit(2).Find(&files).Error; err != nil {
		return false, err
	}
	if len(files) > 0 {
		return len(files) == 1 && files[0].UserId == userID && files[0].Status == model.FileStatusActive && files[0].ExpiresAt.After(time.Now()), nil
	}
	var tasks []model.Task
	matched := false
	stop := errors.New("material found")
	err = model.DB.WithContext(ctx).Select("id", "properties").Where("user_id = ? AND action = ?", userID, ActionInfluencerMotion).FindInBatches(&tasks, 100, func(_ *gorm.DB, _ int) error {
		for _, task := range tasks {
			var input struct {
				VideoURL         string `json:"video_url"`
				PreparedVideoURL string `json:"prepared_video_url"`
			}
			if common.UnmarshalJsonStr(task.Properties.Input, &input) == nil && (input.VideoURL == raw || input.PreparedVideoURL == raw) {
				matched = true
				return stop
			}
		}
		return nil
	}).Error
	if errors.Is(err, stop) {
		err = nil
	}
	return matched, err
}
