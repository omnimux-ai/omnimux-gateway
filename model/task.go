package model

import (
	"bytes"
	"context"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	commonRelay "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"gorm.io/gorm"
)

type TaskStatus string

func (t TaskStatus) ToVideoStatus() string {
	var status string
	switch t {
	case TaskStatusWaiting, TaskStatusClaimed, TaskStatusDispatching, TaskStatusNotStart, TaskStatusQueued, TaskStatusSubmitted:
		status = dto.VideoStatusQueued
	case TaskStatusInProgress:
		status = dto.VideoStatusInProgress
	case TaskStatusSuccess:
		status = dto.VideoStatusCompleted
	case TaskStatusFailure:
		status = dto.VideoStatusFailed
	default:
		status = dto.VideoStatusUnknown // Default fallback
	}
	return status
}

const (
	// fork: local generation states have no pollable upstream task.
	TaskStatusWaiting     TaskStatus = "WAITING"
	TaskStatusClaimed     TaskStatus = "CLAIMED"
	TaskStatusDispatching TaskStatus = "DISPATCHING"
	TaskStatusNotStart    TaskStatus = "NOT_START"
	TaskStatusSubmitted              = "SUBMITTED"
	TaskStatusQueued                 = "QUEUED"
	TaskStatusInProgress             = "IN_PROGRESS"
	TaskStatusFailure                = "FAILURE"
	TaskStatusSuccess                = "SUCCESS"
	TaskStatusUnknown                = "UNKNOWN"
)

// TaskRefundLegacyCutoff separates tasks created before timeout refunds were
// introduced. Those legacy tasks are failed without an automatic refund.
const TaskRefundLegacyCutoff int64 = 1771718400 // 2026-02-22 00:00:00 UTC

type Task struct {
	ID         int64                 `json:"id" gorm:"primary_key;AUTO_INCREMENT"`
	CreatedAt  int64                 `json:"created_at" gorm:"index"`
	UpdatedAt  int64                 `json:"updated_at"`
	TaskID     string                `json:"task_id" gorm:"type:varchar(191);index"` // 第三方id，不一定有/ song id\ Task id
	Platform   constant.TaskPlatform `json:"platform" gorm:"type:varchar(30);index"` // 平台
	UserId     int                   `json:"user_id" gorm:"index"`
	Group      string                `json:"group" gorm:"type:varchar(50)"` // 修正计费用
	ChannelId  int                   `json:"channel_id" gorm:"index"`
	Quota      int                   `json:"quota"`
	Action     string                `json:"action" gorm:"type:varchar(40);index"` // 任务类型, song, lyrics, description-mode
	Status     TaskStatus            `json:"status" gorm:"type:varchar(20);index"` // 任务状态
	FailReason string                `json:"fail_reason"`
	SubmitTime int64                 `json:"submit_time" gorm:"index"`
	StartTime  int64                 `json:"start_time" gorm:"index"`
	FinishTime int64                 `json:"finish_time" gorm:"index"`
	Progress   string                `json:"progress" gorm:"type:varchar(20);index"`
	Properties Properties            `json:"properties" gorm:"type:json"`
	Username   string                `json:"username,omitempty" gorm:"-"`
	// 禁止返回给用户，内部可能包含key等隐私信息
	PrivateData TaskPrivateData `json:"-" gorm:"column:private_data;type:json"`
	Data        json.RawMessage `json:"data" gorm:"type:json"`
	// OutputVideoMetadata holds the server-observed media facts of a generated
	// output video, written once from bytes the server already buffered. It is
	// nullable with no default: historical tasks keep NULL and clients fall back
	// to their own read, so absent metadata never changes an existing response.
	// The JSON serializer keeps TEXT portable across SQLite, MySQL and PostgreSQL
	// while the value stays a json.RawMessage in Go.
	OutputVideoMetadata json.RawMessage `json:"-" gorm:"column:output_video_metadata;type:text;serializer:json"`
}

func (t *Task) SetData(data any) {
	b, _ := common.Marshal(data)
	t.Data = json.RawMessage(b)
}

func (t *Task) GetData(v any) error {
	return common.Unmarshal(t.Data, &v)
}

type Properties struct {
	Input             string `json:"input"`
	UpstreamModelName string `json:"upstream_model_name,omitempty"`
	OriginModelName   string `json:"origin_model_name,omitempty"`
}

func (m *Properties) Scan(val any) error {
	bytesValue := jsonScanBytes(val)
	if len(bytesValue) == 0 {
		*m = Properties{}
		return nil
	}
	return common.Unmarshal(bytesValue, m)
}

func (m Properties) Value() (driver.Value, error) {
	if m == (Properties{}) {
		return nil, nil
	}
	// 必须返回 string 而非 []byte:PG simple protocol 下 []byte 按 bytea 编码,
	// 写 json 列会触发 SQLSTATE 22P02。
	b, err := common.Marshal(m)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

type TaskPrivateData struct {
	GenerationQueue *GenerationQueueRequest `json:"generation_queue,omitempty"`
	// ObjectSwapPreparation survives queue handoff; a recorded understanding
	// request must never be repeated merely because its worker was interrupted.
	ObjectSwapPreparation *GenerationObjectSwapPreparation `json:"object_swap_preparation,omitempty"`
	// GenerationRouting retains private attempt evidence after the queue hands
	// the task to the native poller; it never restores a pending queue request.
	GenerationRouting *GenerationRoutingPolicy `json:"generation_routing,omitempty"`
	// ImageInputs retains the original frozen source/mask identity after the
	// native queue hands off an accepted image request. It is never a retry job.
	ImageInputs *GenerationImageInputs `json:"image_inputs,omitempty"`
	// GenerationAcceptedResult records a trusted submit receipt before normal
	// handoff. A terminal receipt is consumed locally, never queried by a
	// synthetic public task ID.
	GenerationAcceptedResult *GenerationAcceptedResult `json:"generation_accepted_result,omitempty"`
	// GenerationReservationUnlogged marks a handed-off reservation whose usage
	// is not committed yet; failure releases funding without undoing usage.
	GenerationReservationUnlogged bool `json:"generation_reservation_unlogged,omitempty"`
	// GenerationConsumptionLogState separates atomic primary-DB usage from
	// a possibly independent log database. started/unknown never replay a write.
	GenerationConsumptionLogState string `json:"generation_consumption_log_state,omitempty"`
	Key                           string `json:"key,omitempty"`
	UpstreamTaskID                string `json:"upstream_task_id,omitempty"` // 上游真实 task ID
	ResultURL                     string `json:"result_url,omitempty"`       // 任务成功后的结果 URL（视频地址等）
	// Execution records safe, immutable request provenance. It lives next to
	// other private task state so public task DTOs cannot expose it by accident.
	Execution *TaskExecutionSnapshot `json:"execution,omitempty"`
	// 计费上下文：用于异步退款/差额结算（轮询阶段读取）
	BillingSource  string `json:"billing_source,omitempty"`  // "wallet" 或 "subscription"
	SubscriptionId int    `json:"subscription_id,omitempty"` // 订阅 ID，用于订阅退款
	// fork: wallet share of this task's subscription settlement, for exact refunds.
	SubscriptionWalletConsumed int64 `json:"subscription_wallet_consumed,omitempty"`
	// fork: allowance generation this task's subscription quota was charged in, and how
	// much of that quota landed in it. 0 means the task predates generation tracking,
	// so a late refund can never be applied to a newer allowance generation.
	SubscriptionAllowanceEpoch     int64               `json:"subscription_allowance_epoch,omitempty"`
	SubscriptionAllowanceAllocated int64               `json:"subscription_allowance_allocated,omitempty"`
	TokenId                        int                 `json:"token_id,omitempty"`        // 令牌 ID，用于令牌额度退款
	NodeName                       string              `json:"node_name,omitempty"`       // 发起任务的节点名，轮询结算阶段据此归属日志而非最后查询节点
	BillingContext                 *TaskBillingContext `json:"billing_context,omitempty"` // 计费参数快照（用于轮询阶段重新计算）
	// ResponsesBackground records that the openai_responses create request
	// asked for background:true. Every task is durable and survives client
	// disconnect regardless; this only echoes the protocol-level request
	// attribute back on retrieval snapshots.
	ResponsesBackground bool `json:"responses_background,omitempty"`
	// PluginState is plugin-owned cross-round data. Unlike Task.Data it is
	// only replaced when a hook explicitly returns state.
	PluginState json.RawMessage `json:"plugin_state,omitempty"`
	// PollFailures counts consecutive unrecognized or transient poll outcomes.
	PollFailures int `json:"poll_failures,omitempty"`
	// ImageResultUnverified retains an accepted, versioned image request for
	// original-task reconciliation; it never authorizes a new submission.
	ImageResultUnverified bool `json:"image_result_unverified,omitempty"`
	// ResultDiscarded marks an immediate terminal result whose submit route
	// declared retainResult: false. The upstream snapshot was never written
	// and every retrieval surface treats the task as not found. The zero
	// value keeps historical rows retained and retrievable.
	ResultDiscarded bool `json:"result_discarded,omitempty"`
}

type TaskExecutionSnapshot struct {
	RequestID   string              `json:"request_id,omitempty"`
	RequestPath string              `json:"request_path,omitempty"`
	TaskPlugin  *TaskPluginSnapshot `json:"task_plugin,omitempty"`
}

// TaskPluginSnapshot contains credential-free identity only. Plugin source,
// request/response payloads, and channel secrets must never be added here.
type TaskPluginSnapshot struct {
	Key        string                    `json:"key"`
	Name       string                    `json:"name"`
	Version    string                    `json:"version"`
	Author     *TaskPluginAuthorSnapshot `json:"author,omitempty"`
	APIVersion int                       `json:"api_version"`
	Generation uint64                    `json:"generation"`
}

type TaskPluginAuthorSnapshot struct {
	Name string `json:"name"`
	URL  string `json:"url,omitempty"`
}

// TaskBillingContext 记录任务提交时的计费参数，以便轮询阶段可以重新计算额度。
type TaskBillingContext struct {
	ModelPrice      float64                      `json:"model_price,omitempty"`       // 模型单价
	GroupRatio      float64                      `json:"group_ratio,omitempty"`       // 分组倍率
	ModelRatio      float64                      `json:"model_ratio,omitempty"`       // 模型倍率
	OtherRatios     map[string]float64           `json:"other_ratios,omitempty"`      // 附加倍率（时长、分辨率等）
	OriginModelName string                       `json:"origin_model_name,omitempty"` // 模型名称，必须为OriginModelName
	PerCallBilling  bool                         `json:"per_call_billing,omitempty"`  // 按次计费：跳过轮询阶段的差额结算
	TieredSnapshot  *billingexpr.BillingSnapshot `json:"tiered_snapshot,omitempty"`
}

// ResultRetrievable reports whether retrieval surfaces (native query routes,
// protocol retrieve endpoints, artifact projection) may serve this task.
func (t *Task) ResultRetrievable() bool {
	return !t.PrivateData.ResultDiscarded
}

// GetUpstreamTaskID 获取上游真实 task ID（用于与 provider 通信）
// 旧数据没有 UpstreamTaskID 时，TaskID 本身就是上游 ID
func (t *Task) GetUpstreamTaskID() string {
	if t.PrivateData.UpstreamTaskID != "" {
		return t.PrivateData.UpstreamTaskID
	}
	return t.TaskID
}

// GetRawUpstreamURL 获取任务内部存储的真实上游结果 URL（真实真源）
func (t *Task) GetRawUpstreamURL() string {
	if t.PrivateData.ResultURL != "" {
		return t.PrivateData.ResultURL
	}
	return t.FailReason
}

// IsImageTask 判断该任务是否为图片任务
func (t *Task) IsImageTask() bool {
	if t.Platform == constant.TaskPlatformMidjourney {
		return true
	}
	modelName := strings.ToLower(t.Properties.OriginModelName)
	if modelName == "" {
		modelName = strings.ToLower(t.Properties.UpstreamModelName)
	}
	if strings.Contains(modelName, "image") {
		return true
	}
	if t.Action == "image" || strings.Contains(strings.ToLower(t.Action), "image") {
		return true
	}
	rawURL := strings.ToLower(t.GetRawUpstreamURL())
	for _, ext := range []string{".png", ".jpg", ".jpeg", ".webp", ".gif"} {
		if strings.Contains(rawURL, ext) {
			return true
		}
	}
	return false
}

// BuildProxyURL constructs the video proxy URL using the public task ID.
// e.g., "https://your-server.com/v1/videos/task_xxxx/content"
func BuildProxyURL(taskID string) string {
	return fmt.Sprintf("%s/v1/videos/%s/content", system_setting.ServerAddress, taskID)
}

// BuildImageProxyURL constructs the image proxy URL using the public task ID.
// e.g., "https://your-server.com/v1/images/task_xxxx/content"
func BuildImageProxyURL(taskID string) string {
	return fmt.Sprintf("%s/v1/images/%s/content", system_setting.ServerAddress, taskID)
}

// isLocalProxyURL 检查 URL 是否已经是本站的代理地址
func isLocalProxyURL(u string, taskID string) bool {
	trimmed := strings.TrimSpace(u)
	if trimmed == "" {
		return false
	}
	if taskID != "" {
		if strings.Contains(trimmed, "/v1/videos/"+taskID+"/content") ||
			strings.Contains(trimmed, "/v1/images/"+taskID+"/content") {
			return true
		}
	}
	return strings.Contains(trimmed, "/v1/videos/") && strings.HasSuffix(trimmed, "/content") ||
		strings.Contains(trimmed, "/v1/images/") && strings.HasSuffix(trimmed, "/content")
}

// GetResultURL 获取对外暴露的任务结果 URL（视频/图片等）
// 当 setting.TaskProxyEnabled 为 true 时，根据任务类型输出本地反代 URL：
// - 视频：/v1/videos/{task_id}/content
// - 图片：/v1/images/{task_id}/content
// 当 TaskProxyEnabled 为 false 时，返回原始真实 URL。
func (t *Task) GetResultURL() string {
	rawURL := t.GetRawUpstreamURL()
	if rawURL == "" {
		return ""
	}
	if !setting.TaskProxyEnabled {
		return rawURL
	}
	if isLocalProxyURL(rawURL, t.TaskID) {
		return rawURL
	}
	if t.IsImageTask() {
		return BuildImageProxyURL(t.TaskID)
	}
	return BuildProxyURL(t.TaskID)
}

// GenerateTaskID 生成对外暴露的 task_xxxx 格式 ID
func GenerateTaskID() string {
	key, _ := common.GenerateRandomCharsKey(32)
	return "task_" + key
}

func (p *TaskPrivateData) Scan(val any) error {
	bytesValue := jsonScanBytes(val)
	if len(bytesValue) == 0 {
		return nil
	}
	return common.Unmarshal(bytesValue, p)
}

func (p TaskPrivateData) Value() (driver.Value, error) {
	if p.GenerationQueue == nil && p.GenerationRouting == nil && p.ImageInputs == nil && p.GenerationAcceptedResult == nil && !p.GenerationReservationUnlogged && p.GenerationConsumptionLogState == "" && p.Key == "" && p.UpstreamTaskID == "" && p.ResultURL == "" &&
		p.Execution == nil && p.BillingSource == "" && p.SubscriptionId == 0 && p.SubscriptionWalletConsumed == 0 &&
		p.SubscriptionAllowanceEpoch == 0 && p.SubscriptionAllowanceAllocated == 0 &&
		p.TokenId == 0 && p.NodeName == "" && p.BillingContext == nil &&
		!p.ResponsesBackground && len(p.PluginState) == 0 && p.PollFailures == 0 && !p.ImageResultUnverified &&
		!p.ResultDiscarded {
		return nil, nil
	}
	// 同 Properties.Value:string 避免 PG simple protocol 的 bytea 编码。
	b, err := common.Marshal(p)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

// SyncTaskQueryParams 用于包含所有搜索条件的结构体，可以根据需求添加更多字段
type SyncTaskQueryParams struct {
	Platform       constant.TaskPlatform
	ChannelID      string
	TaskID         string
	UserID         string
	Action         string
	Status         string
	StartTimestamp int64
	EndTimestamp   int64
	UserIDs        []int
	// WithArtifacts retains the internal snapshot and channel for media projection, never for the list response.
	WithArtifacts bool
}

func InitTask(platform constant.TaskPlatform, relayInfo *commonRelay.RelayInfo) *Task {
	properties := Properties{}
	privateData := TaskPrivateData{}
	if relayInfo != nil && relayInfo.ChannelMeta != nil {
		// Native task credentials bind polling to the account that submitted it.
		// The reviewed type-1 image path may also use an existing multi-key pool;
		// preserve only the single key already selected for this attempt.
		reviewedImageIdentity := platform == "image_task_v1" && relayInfo.OriginModelName == "gpt-image-2.5-sunburst" && relayInfo.ChannelMeta.ChannelType == constant.ChannelTypeOpenAI
		if reviewedImageIdentity || relayInfo.ChannelMeta.ChannelType == constant.ChannelTypeGemini ||
			relayInfo.ChannelMeta.ChannelType == constant.ChannelTypeVertexAi ||
			relayInfo.ChannelMeta.ChannelType == constant.ChannelTypeGxgenAI ||
			relayInfo.ChannelMeta.ChannelType == constant.ChannelTypeGoEasyAPI ||
			relayInfo.ChannelMeta.ChannelType == constant.ChannelTypeNewAPI {
			privateData.Key = relayInfo.ChannelMeta.ApiKey
		}
		if relayInfo.UpstreamModelName != "" {
			properties.UpstreamModelName = relayInfo.UpstreamModelName
		}
		if relayInfo.OriginModelName != "" {
			properties.OriginModelName = relayInfo.OriginModelName
		}
	}

	// 使用预生成的公开 ID（如果有），否则新生成
	taskID := ""
	if relayInfo.TaskRelayInfo != nil && relayInfo.TaskRelayInfo.PublicTaskID != "" {
		taskID = relayInfo.TaskRelayInfo.PublicTaskID
	} else {
		taskID = GenerateTaskID()
	}

	t := &Task{
		TaskID:      taskID,
		UserId:      relayInfo.UserId,
		Group:       relayInfo.UsingGroup,
		SubmitTime:  time.Now().Unix(),
		Status:      TaskStatusNotStart,
		Progress:    "0%",
		ChannelId:   relayInfo.ChannelId,
		Platform:    platform,
		Properties:  properties,
		PrivateData: privateData,
	}
	return t
}

func TaskGetAllUserTask(userId int, startIdx int, num int, queryParams SyncTaskQueryParams) []*Task {
	var tasks []*Task
	var err error

	// 初始化查询构建器
	query := DB.Where("user_id = ?", userId)

	if queryParams.TaskID != "" {
		query = query.Where("task_id = ?", queryParams.TaskID)
	}
	if queryParams.Action != "" {
		query = query.Where("action = ?", queryParams.Action)
	}
	if queryParams.Status != "" {
		query = query.Where("status = ?", queryParams.Status)
	}
	if queryParams.Platform != "" {
		query = query.Where("platform = ?", queryParams.Platform)
	}
	if queryParams.StartTimestamp != 0 {
		// 假设您已将前端传来的时间戳转换为数据库所需的时间格式，并处理了时间戳的验证和解析
		query = query.Where("submit_time >= ?", queryParams.StartTimestamp)
	}
	if queryParams.EndTimestamp != 0 {
		query = query.Where("submit_time <= ?", queryParams.EndTimestamp)
	}

	// Status-only lists stay small; explicit artifact reads need the persisted
	// snapshot and channel internally to project and preheat the correct media.
	if !queryParams.WithArtifacts {
		query = query.Omit("channel_id", "data")
	}
	err = query.Order("id desc").Limit(num).Offset(startIdx).Find(&tasks).Error
	if err != nil {
		return nil
	}

	return tasks
}

func TaskGetAllTasks(startIdx int, num int, queryParams SyncTaskQueryParams) []*Task {
	var tasks []*Task
	var err error

	// 初始化查询构建器
	query := DB

	// 添加过滤条件
	if queryParams.ChannelID != "" {
		query = query.Where("channel_id = ?", queryParams.ChannelID)
	}
	if queryParams.Platform != "" {
		query = query.Where("platform = ?", queryParams.Platform)
	}
	if queryParams.UserID != "" {
		query = query.Where("user_id = ?", queryParams.UserID)
	}
	if len(queryParams.UserIDs) != 0 {
		query = query.Where("user_id in (?)", queryParams.UserIDs)
	}
	if queryParams.TaskID != "" {
		query = query.Where("task_id = ?", queryParams.TaskID)
	}
	if queryParams.Action != "" {
		query = query.Where("action = ?", queryParams.Action)
	}
	if queryParams.Status != "" {
		query = query.Where("status = ?", queryParams.Status)
	}
	if queryParams.StartTimestamp != 0 {
		query = query.Where("submit_time >= ?", queryParams.StartTimestamp)
	}
	if queryParams.EndTimestamp != 0 {
		query = query.Where("submit_time <= ?", queryParams.EndTimestamp)
	}

	if !queryParams.WithArtifacts {
		query = query.Omit("data")
	}
	err = query.Order("id desc").Limit(num).Offset(startIdx).Find(&tasks).Error
	if err != nil {
		return nil
	}

	return tasks
}

func GetTimedOutUnfinishedTasks(cutoffUnix int64, limit int) []*Task {
	var tasks []*Task
	err := DB.Where("progress != ?", "100%").
		Where("status NOT IN ?", []TaskStatus{TaskStatusFailure, TaskStatusSuccess, TaskStatusWaiting, TaskStatusClaimed, TaskStatusDispatching}).
		Where("submit_time < ?", cutoffUnix).
		Order("submit_time").
		Limit(limit).
		Find(&tasks).Error
	if err != nil {
		return nil
	}
	return tasks
}

func GetAllUnFinishSyncTasks(limit int) []*Task {
	var tasks []*Task
	var err error
	// get all tasks progress is not 100%
	err = DB.Where("progress != ?", "100%").Where("status != ?", TaskStatusFailure).Where("status != ?", TaskStatusSuccess).Where("status NOT IN ?", []TaskStatus{TaskStatusWaiting, TaskStatusClaimed, TaskStatusDispatching}).Limit(limit).Order("id").Find(&tasks).Error
	if err != nil {
		return nil
	}
	return tasks
}

// HasUnfinishedSyncTasks reports whether at least one async (Suno/video) task is
// still in progress. It is a cheap existence check (LIMIT 1) used to decide
// whether the async_task_poll system task needs to run; when no task is pending
// the scheduler skips creating a row entirely.
func HasUnfinishedSyncTasks() bool {
	var id int64
	err := DB.Model(&Task{}).
		Where("progress != ?", "100%").
		Where("status != ?", TaskStatusFailure).
		Where("status != ?", TaskStatusSuccess).
		Where("status NOT IN ?", []TaskStatus{TaskStatusWaiting, TaskStatusClaimed, TaskStatusDispatching}).
		Limit(1).
		Pluck("id", &id).Error
	return err == nil && id != 0
}

func GetByOnlyTaskId(taskId string) (*Task, bool, error) {
	if taskId == "" {
		return nil, false, nil
	}
	var task *Task
	var err error
	err = DB.Where("task_id = ?", taskId).First(&task).Error
	exist, err := RecordExist(err)
	if err != nil {
		return nil, false, err
	}
	return task, exist, err
}

// GetUniqueByOnlyTaskId resolves a public task identifier only when exactly one
// row owns it. Historical task identifiers were not globally unique, so
// capability-based reads must fail closed instead of selecting an arbitrary
// tenant's row.
func GetUniqueByOnlyTaskId(taskId string) (*Task, bool, error) {
	if taskId == "" {
		return nil, false, nil
	}
	var tasks []*Task
	if err := DB.Where("task_id = ?", taskId).Order("id").Limit(2).Find(&tasks).Error; err != nil {
		return nil, false, err
	}
	if len(tasks) != 1 {
		return nil, false, nil
	}
	return tasks[0], true, nil
}

func GetByTaskId(userId int, taskId string) (*Task, bool, error) {
	if taskId == "" {
		return nil, false, nil
	}
	var task *Task
	var err error
	err = DB.Where("user_id = ? and task_id = ?", userId, taskId).
		First(&task).Error
	exist, err := RecordExist(err)
	if err != nil {
		return nil, false, err
	}
	return task, exist, err
}

func GetByTaskIdsForPlatforms(userID int, platforms []constant.TaskPlatform, taskIDs []string) ([]*Task, error) {
	if len(platforms) == 0 || len(taskIDs) == 0 {
		return nil, nil
	}
	var tasks []*Task
	err := DB.
		Where("user_id = ? AND platform IN ? AND task_id IN ?", userID, platforms, taskIDs).
		Find(&tasks).Error
	if err != nil {
		return nil, err
	}
	return tasks, nil
}

// GetTaskForProtocolObservation reloads one public task through the ownership
// boundary used by long-lived plugin protocol observers. A missing task,
// foreign user, and wrong plugin platform are deliberately indistinguishable.
func GetTaskForProtocolObservation(ctx context.Context, userID int, platform constant.TaskPlatform, taskID string) (*Task, bool, error) {
	if taskID == "" {
		return nil, false, nil
	}
	var task Task
	err := DB.WithContext(ctx).
		Where("user_id = ? AND platform = ? AND task_id = ?", userID, platform, taskID).
		First(&task).Error
	exists, err := RecordExist(err)
	if err != nil || !exists {
		return nil, exists, err
	}
	return &task, true, nil
}

func (Task *Task) Insert() error {
	return Task.InsertWithContext(context.Background())
}

// InsertWithContext creates the row. omitColumns are left out of the INSERT
// (for example "data" when the submit route discards the upstream snapshot)
// while the in-memory task keeps its values for presentation.
func (Task *Task) InsertWithContext(ctx context.Context, omitColumns ...string) error {
	tx := DB.WithContext(ctx)
	if len(omitColumns) > 0 {
		tx = tx.Omit(omitColumns...)
	}
	return tx.Create(Task).Error
}

type taskSnapshot struct {
	Status                TaskStatus
	Progress              string
	StartTime             int64
	FinishTime            int64
	FailReason            string
	ResultURL             string
	Data                  json.RawMessage
	PluginState           json.RawMessage
	PollFailures          int
	ImageResultUnverified bool
}

func (s taskSnapshot) Equal(other taskSnapshot) bool {
	return s.Status == other.Status &&
		s.Progress == other.Progress &&
		s.StartTime == other.StartTime &&
		s.FinishTime == other.FinishTime &&
		s.FailReason == other.FailReason &&
		s.ResultURL == other.ResultURL &&
		bytes.Equal(s.Data, other.Data) &&
		bytes.Equal(s.PluginState, other.PluginState) &&
		s.PollFailures == other.PollFailures && s.ImageResultUnverified == other.ImageResultUnverified
}

func (t *Task) Snapshot() taskSnapshot {
	return taskSnapshot{
		Status:                t.Status,
		Progress:              t.Progress,
		StartTime:             t.StartTime,
		FinishTime:            t.FinishTime,
		FailReason:            t.FailReason,
		ResultURL:             t.PrivateData.ResultURL,
		Data:                  t.Data,
		PluginState:           t.PrivateData.PluginState,
		PollFailures:          t.PrivateData.PollFailures,
		ImageResultUnverified: t.PrivateData.ImageResultUnverified,
	}
}

func (Task *Task) Update() error {
	var err error
	err = DB.Save(Task).Error
	return err
}

func (t *Task) UpdateQuota() error {
	return DB.Model(t).Update("quota", t.Quota).Error
}

// UpdateTaskOutputVideoMetadata stores observed output media facts in their own
// column. It is a single-column UPDATE without hooks or updated_at side effects,
// so it can never lose a concurrent task-row update and task polling never has
// to read-modify-write this field.
func UpdateTaskOutputVideoMetadata(taskID int64, metadata []byte) error {
	if taskID == 0 || len(metadata) == 0 {
		return nil
	}
	// string avoids PostgreSQL simple-protocol bytea encoding for a text column.
	return DB.Model(&Task{}).
		Where("id = ?", taskID).
		UpdateColumn("output_video_metadata", string(metadata)).Error
}

// UpdateWithStatus performs a conditional UPDATE guarded by fromStatus (CAS).
// Returns (true, nil) if this caller won the update, (false, nil) if
// another process already moved the task out of fromStatus. MySQL commonly
// reports changed rows rather than matched rows, so a same-value no-op update
// can also return false even when the status predicate still matched.
//
// Uses Model().Select("*").Updates() instead of Save() because GORM's Save
// falls back to INSERT ON CONFLICT when the WHERE-guarded UPDATE matches
// zero rows, which silently bypasses the CAS guard.
func (t *Task) UpdateWithStatus(fromStatus TaskStatus) (bool, error) {
	if t.HasVersionedSunburstImageInputs() {
		var won bool
		err := DB.Transaction(func(tx *gorm.DB) error {
			if common.UsingMainDatabase(common.DatabaseTypeSQLite) {
				if err := tx.Model(&Task{}).Where("id = ?", t.ID).UpdateColumn("status", gorm.Expr("status")).Error; err != nil {
					return err
				}
			}
			var current Task
			if err := lockForUpdate(tx).First(&current, t.ID).Error; err != nil {
				return err
			}
			if current.Status != fromStatus || current.UserId != t.UserId || current.TaskID != t.TaskID {
				return nil
			}
			// A poller may have fetched its snapshot before the submitter's
			// accounting transaction. Never overwrite committed native stages.
			t.PrivateData.GenerationReservationUnlogged = current.PrivateData.GenerationReservationUnlogged
			t.PrivateData.GenerationConsumptionLogState = current.PrivateData.GenerationConsumptionLogState
			result := tx.Model(t).Where("status = ?", fromStatus).Select("*").Updates(t)
			won = result.RowsAffected > 0
			return result.Error
		})
		return won, err
	}
	result := DB.Model(t).Where("status = ?", fromStatus).Select("*").Updates(t)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

// TaskBulkUpdateByID performs an unconditional bulk UPDATE by primary key IDs.
// WARNING: This function has NO CAS (Compare-And-Swap) guard — it will overwrite
// any concurrent status changes. DO NOT use in billing/quota lifecycle flows
// (e.g., timeout, success, failure transitions that trigger refunds or settlements).
// For status transitions that involve billing, use Task.UpdateWithStatus() instead.
func TaskBulkUpdateByID(ids []int64, params map[string]any) error {
	if len(ids) == 0 {
		return nil
	}
	return DB.Model(&Task{}).
		Where("id in (?)", ids).
		Updates(params).Error
}

type TaskQuotaUsage struct {
	Mode  string  `json:"mode"`
	Count float64 `json:"count"`
}

// TaskCountAllTasks returns total tasks that match the given query params (admin usage)
func TaskCountAllTasks(queryParams SyncTaskQueryParams) int64 {
	var total int64
	query := DB.Model(&Task{})
	if queryParams.ChannelID != "" {
		query = query.Where("channel_id = ?", queryParams.ChannelID)
	}
	if queryParams.Platform != "" {
		query = query.Where("platform = ?", queryParams.Platform)
	}
	if queryParams.UserID != "" {
		query = query.Where("user_id = ?", queryParams.UserID)
	}
	if len(queryParams.UserIDs) != 0 {
		query = query.Where("user_id in (?)", queryParams.UserIDs)
	}
	if queryParams.TaskID != "" {
		query = query.Where("task_id = ?", queryParams.TaskID)
	}
	if queryParams.Action != "" {
		query = query.Where("action = ?", queryParams.Action)
	}
	if queryParams.Status != "" {
		query = query.Where("status = ?", queryParams.Status)
	}
	if queryParams.StartTimestamp != 0 {
		query = query.Where("submit_time >= ?", queryParams.StartTimestamp)
	}
	if queryParams.EndTimestamp != 0 {
		query = query.Where("submit_time <= ?", queryParams.EndTimestamp)
	}
	_ = query.Count(&total).Error
	return total
}

// TaskCountAllUserTask returns total tasks for given user
func TaskCountAllUserTask(userId int, queryParams SyncTaskQueryParams) int64 {
	var total int64
	query := DB.Model(&Task{}).Where("user_id = ?", userId)
	if queryParams.TaskID != "" {
		query = query.Where("task_id = ?", queryParams.TaskID)
	}
	if queryParams.Action != "" {
		query = query.Where("action = ?", queryParams.Action)
	}
	if queryParams.Status != "" {
		query = query.Where("status = ?", queryParams.Status)
	}
	if queryParams.Platform != "" {
		query = query.Where("platform = ?", queryParams.Platform)
	}
	if queryParams.StartTimestamp != 0 {
		query = query.Where("submit_time >= ?", queryParams.StartTimestamp)
	}
	if queryParams.EndTimestamp != 0 {
		query = query.Where("submit_time <= ?", queryParams.EndTimestamp)
	}
	_ = query.Count(&total).Error
	return total
}
func (t *Task) ToOpenAIVideo() *dto.OpenAIVideo {
	openAIVideo := dto.NewOpenAIVideo()
	openAIVideo.ID = t.TaskID
	openAIVideo.Status = t.Status.ToVideoStatus()
	openAIVideo.Model = t.Properties.OriginModelName
	openAIVideo.SetProgressStr(t.Progress)
	openAIVideo.CreatedAt = t.CreatedAt
	if t.Status == TaskStatusSuccess {
		if t.FinishTime != 0 {
			openAIVideo.CompletedAt = t.FinishTime
		} else {
			openAIVideo.CompletedAt = t.UpdatedAt
		}
	}
	return openAIVideo
}
