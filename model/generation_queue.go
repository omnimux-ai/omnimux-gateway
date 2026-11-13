package model

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"reflect"
	"slices"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const generationDispatchExpiryBatch = 100

const SystemTaskTypeGenerationQueue = "generation_queue"
const GenerationConcurrency = 5

// GenerationQueueRequest contains only server-normalized generation parameters,
// never client headers, authentication, or channel credentials.
type GenerationQueueRequest struct {
	Path               string          `json:"path"`
	Body               json.RawMessage `json:"body"`
	ReferencePreflight string          `json:"reference_preflight,omitempty"`
	ClaimOwner         string          `json:"claim_owner,omitempty"`
	SendStartedAt      int64           `json:"send_started_at,omitempty"`
	// ReleaseStartedAt is written before the submitting worker releases its
	// in-memory reservation; expiry never refunds a row that carries it.
	ReleaseStartedAt int64                    `json:"release_started_at,omitempty"`
	Routing          *GenerationRoutingPolicy `json:"routing,omitempty"`
	// ImageInputs freezes only new, versioned workbench image submissions.
	// A nil snapshot never upgrades or resubmits a historical queue row.
	ImageInputs *GenerationImageInputs `json:"image_inputs,omitempty"`
}

type GenerationImageInputs struct {
	Version int                       `json:"version"`
	Mode    string                    `json:"mode"`
	Source  *GenerationImageReference `json:"source,omitempty"`
	Mask    *GenerationImageReference `json:"mask,omitempty"`
}

// GenerationAcceptedResult contains only trusted supplier result facts. Empty
// Status means an asynchronous receipt; terminal results retain their genuine
// original URL even when the client response has been rewritten to a proxy.
type GenerationAcceptedResult struct {
	Status   TaskStatus `json:"status,omitempty"`
	URL      string     `json:"url,omitempty"`
	Reason   string     `json:"reason,omitempty"`
	Progress string     `json:"progress,omitempty"`
}

func (task *Task) HasVersionedSunburstImageInputs() bool {
	if task == nil || task.Action != "influencer_sheet" || task.Properties.OriginModelName != "gpt-image-2.5-sunburst" {
		return false
	}
	inputs := task.PrivateData.ImageInputs
	if inputs == nil && task.PrivateData.GenerationQueue != nil {
		inputs = task.PrivateData.GenerationQueue.ImageInputs
	}
	return inputs != nil && inputs.Version == 1
}

// GenerationImageReference identifies original, server-frozen material without
// storing credentials or trusting a user-provided digest.
type GenerationImageReference struct {
	URL       string `json:"url"`
	FileID    string `json:"file_id"`
	SHA256    string `json:"sha256"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	Format    string `json:"format"`
	ExpiresAt int64  `json:"expires_at"`
}

const (
	GenerationRoutingAuto     = "auto"
	GenerationRoutingGroup    = "group"
	GenerationAttemptPending  = "pending"
	GenerationAttemptRejected = "rejected"
	GenerationAttemptAccepted = "accepted"
	GenerationAttemptUnknown  = "unknown"
)

// GenerationRoutingPolicy is a server-owned snapshot. A nil policy keeps old
// queued tasks on the original single-submission reconciliation path.
type GenerationRoutingPolicy struct {
	Mode        string                   `json:"mode"`
	Groups      []string                 `json:"groups"`
	MaxAttempts int                      `json:"max_attempts"`
	Attempts    []GenerationRouteAttempt `json:"attempts,omitempty"`
}

type GenerationRouteAttempt struct {
	ChannelID int    `json:"channel_id"`
	Group     string `json:"group"`
	Outcome   string `json:"outcome"`
}

var generationActions = []string{"influencer_sheet", "influencer_motion", "influencer_recreate_analysis"}

func GenerationWaitingUsers(ctx context.Context) ([]int, error) {
	var ids []int
	err := DB.WithContext(ctx).Model(&Task{}).Where("action IN ? AND status = ?", generationActions, TaskStatusWaiting).Distinct().Pluck("user_id", &ids).Error
	return ids, err
}

// ClaimGenerationTasks serializes every media kind on the existing user row.
// SQLite first takes a write lock; server databases use the shared row-lock helper.
func ClaimGenerationTasks(ctx context.Context, userID int, owner string) ([]*Task, error) {
	var claimed []*Task
	err := DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if common.UsingMainDatabase(common.DatabaseTypeSQLite) {
			if err := tx.Model(&User{}).Where("id = ?", userID).UpdateColumn("status", gorm.Expr("status")).Error; err != nil {
				return err
			}
		}
		var user User
		if err := lockForUpdate(tx).First(&user, userID).Error; err != nil {
			return err
		}
		if user.Status != common.UserStatusEnabled {
			return nil
		}
		var active int64
		if err := tx.Model(&Task{}).Where("user_id = ? AND action IN ? AND status NOT IN ?", userID, generationActions, []TaskStatus{TaskStatusWaiting, TaskStatusSuccess, TaskStatusFailure}).Count(&active).Error; err != nil {
			return err
		}
		if active >= GenerationConcurrency {
			return nil
		}
		var waiting []*Task
		if err := tx.Where("user_id = ? AND action IN ? AND status = ?", userID, generationActions, TaskStatusWaiting).Order("id").Limit(GenerationConcurrency - int(active)).Find(&waiting).Error; err != nil {
			return err
		}
		for _, task := range waiting {
			if task.PrivateData.GenerationQueue == nil {
				return errors.New("generation request is missing")
			}
			task.PrivateData.GenerationQueue.ClaimOwner = owner
			task.Status = TaskStatusClaimed
			result := tx.Model(task).Where("status = ?", TaskStatusWaiting).Select("status", "private_data").Updates(task)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return errors.New("generation claim lost")
			}
			claimed = append(claimed, task)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return claimed, nil
}

// RecoverGenerationClaims is called only by the current global scanner lease.
// CLAIMED has not entered the billing/submission chain. DISPATCHING is never reset.
func RecoverGenerationClaims(ctx context.Context, scanner *SystemTask, runnerID string) error {
	return DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var lease SystemTaskLock
		if err := lockForUpdate(tx).Where("type = ? AND task_id = ? AND locked_by = ? AND locked_until >= ?", SystemTaskTypeGenerationQueue, scanner.TaskID, runnerID, common.GetTimestamp()).First(&lease).Error; err != nil {
			return err
		}
		var dispatching []Task
		if err := lockForUpdate(tx).Where("action IN ? AND status = ?", generationActions, TaskStatusDispatching).Find(&dispatching).Error; err != nil {
			return err
		}
		for _, task := range dispatching {
			if task.HasVersionedSunburstImageInputs() {
				if err := recoverVersionedImageDispatch(tx, &task); err != nil {
					return err
				}
				continue
			}
			if task.PrivateData.GenerationQueue != nil && task.PrivateData.GenerationQueue.ClaimOwner == scanner.TaskID {
				continue
			}
			// A missing prior worker or interrupted preparation is not evidence
			// that funding was never reserved or that a POST can safely be replayed.
			if err := tx.Model(&Task{}).Where("id = ? AND status = ? AND (fail_reason = ? OR fail_reason IS NULL)", task.ID, TaskStatusDispatching, "").Update("fail_reason", "提交或资金状态待核对，不会自动重发").Error; err != nil {
				return err
			}
		}
		return tx.Model(&Task{}).Where("action IN ? AND status = ?", generationActions, TaskStatusClaimed).Update("status", TaskStatusWaiting).Error
	})
}

// ExpireGenerationDispatches fails DISPATCHING rows that entered submission
// before cutoff, releasing their concurrency slots. fork: an unconfirmed
// submission must not hold a slot indefinitely. inFlight lists rows whose
// worker in this scanner is still submitting; they are never expired. It
// never resends and never settles funding; the returned rows carry a persisted
// reservation that no worker has started to release, which the caller returns.
func ExpireGenerationDispatches(ctx context.Context, scanner *SystemTask, runnerID string, cutoff int64, timeoutMinutes int, inFlight []int64) ([]*Task, error) {
	if err := DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return lockGenerationScannerLease(tx, scanner, runnerID)
	}); err != nil {
		return nil, err
	}
	query := DB.WithContext(ctx).Model(&Task{}).Where("action IN ? AND status = ? AND start_time > 0 AND start_time < ?", generationActions, TaskStatusDispatching, cutoff)
	if len(inFlight) > 0 {
		query = query.Where("id NOT IN ?", inFlight)
	}
	var candidates []int64
	if err := query.Order("id").Limit(generationDispatchExpiryBatch).Pluck("id", &candidates).Error; err != nil {
		return nil, err
	}
	var refundable []*Task
	for _, id := range candidates {
		expired, err := expireGenerationDispatch(ctx, id, scanner, runnerID, timeoutMinutes)
		if err != nil {
			return refundable, err
		}
		if expired != nil {
			refundable = append(refundable, expired)
		}
	}
	return refundable, nil
}

func lockGenerationScannerLease(tx *gorm.DB, scanner *SystemTask, runnerID string) error {
	var lease SystemTaskLock
	return lockForUpdate(tx).Where("type = ? AND task_id = ? AND locked_by = ? AND locked_until >= ?", SystemTaskTypeGenerationQueue, scanner.TaskID, runnerID, common.GetTimestamp()).First(&lease).Error
}

// expireGenerationDispatch returns the expired row when its reservation is
// the caller's to release. The lease is rechecked per row so a scanner that
// loses it mid-batch stops before touching rows a newer scanner may own.
func expireGenerationDispatch(ctx context.Context, id int64, scanner *SystemTask, runnerID string, timeoutMinutes int) (*Task, error) {
	var expired *Task
	err := DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if common.UsingMainDatabase(common.DatabaseTypeSQLite) {
			if err := tx.Model(&Task{}).Where("id = ?", id).UpdateColumn("status", gorm.Expr("status")).Error; err != nil {
				return err
			}
		}
		if err := lockGenerationScannerLease(tx, scanner, runnerID); err != nil {
			return err
		}
		var current Task
		if err := lockForUpdate(tx).First(&current, id).Error; err != nil {
			return err
		}
		if current.Status != TaskStatusDispatching {
			return nil
		}
		queue := current.PrivateData.GenerationQueue
		if current.HasVersionedSunburstImageInputs() {
			knownRejected := queue != nil && queue.ReleaseStartedAt == 0 && current.PrivateData.UpstreamTaskID == "" && queue.Routing != nil && len(queue.Routing.Attempts) > 0 && queue.Routing.Attempts[len(queue.Routing.Attempts)-1].Outcome == GenerationAttemptRejected
			if !knownRejected {
				return recoverVersionedImageDispatch(tx, &current)
			}
		}
		// A row without a send snapshot may have been pre-charged without a
		// record, and a row whose worker began releasing may already be
		// refunded; neither quota proves an outstanding reservation.
		refundable := queue != nil && queue.SendStartedAt > 0 && queue.ReleaseStartedAt == 0
		reason := fmt.Sprintf("提交结果超过 %d 分钟未确认，已判定失败；扣费待人工核对", timeoutMinutes)
		if refundable {
			reason = fmt.Sprintf("提交结果超过 %d 分钟未确认，已判定失败，预留积分按任务记录退回", timeoutMinutes)
		}
		if current.FailReason != "" {
			reason += "（原记录：" + current.FailReason + "）"
		}
		now := time.Now().Unix()
		updates := map[string]any{"status": TaskStatusFailure, "progress": "100%", "finish_time": now, "fail_reason": reason}
		if current.HasVersionedSunburstImageInputs() {
			current.PrivateData.ImageResultUnverified = false
			updates["private_data"] = current.PrivateData
		}
		result := tx.Model(&Task{}).Where("id = ? AND user_id = ? AND task_id = ? AND status = ?", current.ID, current.UserId, current.TaskID, TaskStatusDispatching).
			Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("generation expiry ownership lost")
		}
		if !refundable {
			common.SysLog(fmt.Sprintf("generation dispatch %s expired without a refundable reservation; funding requires manual reconciliation", current.TaskID))
			return nil
		}
		current.Status = TaskStatusFailure
		current.Progress = "100%"
		current.FinishTime = now
		current.FailReason = reason
		expired = &current
		return nil
	})
	return expired, err
}

// recoverVersionedImageDispatch resumes original-result observation only. It
// never makes a row executable again or performs a funding adjustment.
func recoverVersionedImageDispatch(tx *gorm.DB, task *Task) error {
	queue := task.PrivateData.GenerationQueue
	if queue == nil || task.Status != TaskStatusDispatching {
		return nil
	}
	private := task.PrivateData
	private.ImageResultUnverified = true
	updates := map[string]any{"private_data": private, "fail_reason": "提交或资金状态待核对，不会自动重发"}
	receipt := private.GenerationAcceptedResult
	terminalReceipt := receipt != nil && (receipt.Status == TaskStatusSuccess || receipt.Status == TaskStatusFailure)
	// A synthesized public ID from an old synchronous response cannot be
	// queried as though it were a real supplier job.
	hasOriginalReceipt := private.UpstreamTaskID != "" && (private.UpstreamTaskID != task.TaskID || terminalReceipt)
	if queue.SendStartedAt > 0 && queue.ReleaseStartedAt == 0 && hasOriginalReceipt {
		private.GenerationRouting, private.ImageInputs = queue.Routing, queue.ImageInputs
		private.GenerationQueue = nil
		private.GenerationReservationUnlogged = true
		private.GenerationConsumptionLogState = TaskConsumptionLogPending
		updates["private_data"], updates["status"], updates["progress"], updates["fail_reason"] = private, TaskStatusSubmitted, "0%", ""
	}
	return tx.Model(&Task{}).Where("id = ? AND status = ?", task.ID, TaskStatusDispatching).Updates(updates).Error
}

// MarkGenerationReleaseStarted persists, before a worker releases its
// in-memory reservation, that this release has begun. It succeeds only while
// the row is still DISPATCHING under the same claim, so a row already failed
// by expiry (and refunded there) is never released a second time.
func MarkGenerationReleaseStarted(ctx context.Context, reserved *Task) error {
	return DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if common.UsingMainDatabase(common.DatabaseTypeSQLite) {
			if err := tx.Model(&Task{}).Where("id = ?", reserved.ID).UpdateColumn("status", gorm.Expr("status")).Error; err != nil {
				return err
			}
		}
		var current Task
		if err := lockForUpdate(tx).First(&current, reserved.ID).Error; err != nil {
			return err
		}
		if current.UserId != reserved.UserId || current.TaskID != reserved.TaskID || current.Status != TaskStatusDispatching || current.PrivateData.GenerationQueue == nil || reserved.PrivateData.GenerationQueue == nil || current.PrivateData.GenerationQueue.ClaimOwner != reserved.PrivateData.GenerationQueue.ClaimOwner {
			return errors.New("generation release ownership lost")
		}
		private := current.PrivateData
		private.GenerationQueue.ReleaseStartedAt = common.GetTimestamp()
		result := tx.Model(&Task{}).Where("id = ? AND status = ?", current.ID, TaskStatusDispatching).Update("private_data", private)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("generation release ownership lost")
		}
		return nil
	})
}

func BeginGenerationDispatch(ctx context.Context, task *Task, scanner *SystemTask, runnerID string) (bool, error) {
	var begun bool
	err := DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if common.UsingMainDatabase(common.DatabaseTypeSQLite) {
			if err := tx.Model(&User{}).Where("id = ?", task.UserId).UpdateColumn("status", gorm.Expr("status")).Error; err != nil {
				return err
			}
		}
		var lease SystemTaskLock
		if err := lockForUpdate(tx).Where("type = ? AND task_id = ? AND locked_by = ? AND locked_until >= ?", SystemTaskTypeGenerationQueue, scanner.TaskID, runnerID, common.GetTimestamp()).First(&lease).Error; err != nil {
			return err
		}
		var user User
		if err := lockForUpdate(tx).First(&user, task.UserId).Error; err != nil {
			return err
		}
		if user.Status != common.UserStatusEnabled {
			return errors.New("generation account is disabled")
		}
		var current Task
		if err := lockForUpdate(tx).First(&current, task.ID).Error; err != nil {
			return err
		}
		if current.Status != TaskStatusClaimed || current.PrivateData.GenerationQueue == nil || current.PrivateData.GenerationQueue.ClaimOwner != scanner.TaskID {
			return nil
		}
		result := tx.Model(&Task{}).Where("id = ? AND status = ?", task.ID, TaskStatusClaimed).Updates(map[string]any{"status": TaskStatusDispatching, "start_time": time.Now().Unix(), "fail_reason": ""})
		begun = result.RowsAffected == 1
		return result.Error
	})
	return begun, err
}

// RejectGenerationDispatch cannot overwrite a claim acquired by a newer scan.
func RejectGenerationDispatch(ctx context.Context, task *Task, reason string, refundedSnapshot ...*Task) error {
	return DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if common.UsingMainDatabase(common.DatabaseTypeSQLite) {
			if err := tx.Model(&Task{}).Where("id = ?", task.ID).UpdateColumn("status", gorm.Expr("status")).Error; err != nil {
				return err
			}
		}
		var current Task
		if err := lockForUpdate(tx).First(&current, task.ID).Error; err != nil {
			return err
		}
		if current.Status != TaskStatusClaimed && current.Status != TaskStatusDispatching {
			return nil
		}
		if current.UserId != task.UserId || current.TaskID != task.TaskID || current.PrivateData.GenerationQueue == nil || task.PrivateData.GenerationQueue == nil || current.PrivateData.GenerationQueue.ClaimOwner != task.PrivateData.GenerationQueue.ClaimOwner {
			if len(refundedSnapshot) > 0 {
				return errors.New("generation rejection ownership lost")
			}
			return nil
		}
		updates := map[string]any{"status": TaskStatusFailure, "progress": "100%", "finish_time": time.Now().Unix(), "fail_reason": reason}
		if len(refundedSnapshot) > 0 && refundedSnapshot[0] != nil {
			snapshot := refundedSnapshot[0]
			private := current.PrivateData
			if current.HasVersionedSunburstImageInputs() {
				private.ImageResultUnverified = false
			}
			private.BillingSource = snapshot.PrivateData.BillingSource
			private.SubscriptionId = snapshot.PrivateData.SubscriptionId
			private.SubscriptionWalletConsumed = snapshot.PrivateData.SubscriptionWalletConsumed
			private.SubscriptionAllowanceEpoch = snapshot.PrivateData.SubscriptionAllowanceEpoch
			private.SubscriptionAllowanceAllocated = snapshot.PrivateData.SubscriptionAllowanceAllocated
			private.TokenId = snapshot.PrivateData.TokenId
			if private.BillingContext == nil {
				private.BillingContext = snapshot.PrivateData.BillingContext
			}
			updates["private_data"] = private
			updates["quota"] = 0
		}
		result := tx.Model(&Task{}).Where("id = ? AND user_id = ? AND task_id = ? AND status = ?", task.ID, task.UserId, task.TaskID, current.Status).Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("generation rejection ownership lost")
		}
		return nil
	})
}

// PersistGenerationSendSnapshot commits actual native funding before possible transmission.
// The transaction fences the current scanner, account and task owner, never the network.
func PersistGenerationSendSnapshot(ctx context.Context, reserved, snapshot *Task, scanner *SystemTask, runnerID string, startAttempt ...bool) error {
	if reserved == nil || snapshot == nil || scanner == nil || snapshot.UserId != reserved.UserId || snapshot.TaskID != reserved.TaskID || snapshot.Quota < 0 || snapshot.Quota > common.MaxQuota {
		return errors.New("invalid generation funding snapshot")
	}
	private := snapshot.PrivateData
	if private.BillingSource == "subscription" && (private.SubscriptionId <= 0 || private.SubscriptionAllowanceEpoch <= 0) || private.BillingSource != "subscription" && private.SubscriptionId != 0 {
		return errors.New("invalid generation funding source")
	}
	var savedQueue *GenerationQueueRequest
	err := DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if common.UsingMainDatabase(common.DatabaseTypeSQLite) {
			if err := tx.Model(&User{}).Where("id = ?", reserved.UserId).UpdateColumn("status", gorm.Expr("status")).Error; err != nil {
				return err
			}
		}
		var lease SystemTaskLock
		if err := lockForUpdate(tx).Where("type = ? AND task_id = ? AND locked_by = ? AND locked_until >= ?", SystemTaskTypeGenerationQueue, scanner.TaskID, runnerID, common.GetTimestamp()).First(&lease).Error; err != nil {
			return err
		}
		var user User
		if err := lockForUpdate(tx).First(&user, reserved.UserId).Error; err != nil {
			return err
		}
		if user.Status != common.UserStatusEnabled {
			return errors.New("generation account is disabled")
		}
		var current Task
		if err := lockForUpdate(tx).First(&current, reserved.ID).Error; err != nil {
			return err
		}
		if current.UserId != reserved.UserId || current.TaskID != reserved.TaskID || current.Status != TaskStatusDispatching || current.PrivateData.GenerationQueue == nil || reserved.PrivateData.GenerationQueue == nil || current.PrivateData.GenerationQueue.ClaimOwner != reserved.PrivateData.GenerationQueue.ClaimOwner || current.PrivateData.GenerationQueue.ClaimOwner != scanner.TaskID {
			return errors.New("generation snapshot ownership lost")
		}
		private := current.PrivateData
		if routing := private.GenerationQueue.Routing; routing != nil && len(startAttempt) > 0 && startAttempt[0] {
			if private.GenerationQueue.ReleaseStartedAt != 0 || routing.MaxAttempts < 1 || routing.MaxAttempts > 2 || len(routing.Attempts) >= routing.MaxAttempts {
				return errors.New("generation send budget is exhausted or released")
			}
			if routing.Mode != GenerationRoutingAuto && routing.Mode != GenerationRoutingGroup || routing.Mode == GenerationRoutingGroup && len(routing.Groups) != 1 {
				return errors.New("generation routing policy is invalid")
			}
			if snapshot.ChannelId <= 0 || !slices.Contains(routing.Groups, snapshot.Group) {
				return errors.New("generation route is outside the frozen policy")
			}
			for _, attempt := range routing.Attempts {
				if attempt.ChannelID == snapshot.ChannelId || attempt.Outcome != GenerationAttemptRejected {
					return errors.New("generation previous submission is not safely rejected")
				}
			}
			if len(routing.Attempts) > 0 {
				original, next := current.PrivateData, snapshot.PrivateData
				fundingChanged := current.Quota != snapshot.Quota || original.BillingSource != next.BillingSource || original.TokenId != next.TokenId
				subscriptionChanged := original.SubscriptionId != next.SubscriptionId || original.SubscriptionWalletConsumed != next.SubscriptionWalletConsumed || original.SubscriptionAllowanceEpoch != next.SubscriptionAllowanceEpoch || original.SubscriptionAllowanceAllocated != next.SubscriptionAllowanceAllocated
				if fundingChanged || subscriptionChanged || !reflect.DeepEqual(original.BillingContext, next.BillingContext) {
					return errors.New("generation failover funding snapshot changed")
				}
			}
			routing.Attempts = append(routing.Attempts, GenerationRouteAttempt{ChannelID: snapshot.ChannelId, Group: snapshot.Group, Outcome: GenerationAttemptPending})
		}
		private.Key = snapshot.PrivateData.Key
		private.Execution = snapshot.PrivateData.Execution
		private.BillingSource = snapshot.PrivateData.BillingSource
		private.SubscriptionId = snapshot.PrivateData.SubscriptionId
		private.SubscriptionWalletConsumed = snapshot.PrivateData.SubscriptionWalletConsumed
		private.SubscriptionAllowanceEpoch = snapshot.PrivateData.SubscriptionAllowanceEpoch
		private.SubscriptionAllowanceAllocated = snapshot.PrivateData.SubscriptionAllowanceAllocated
		private.TokenId = snapshot.PrivateData.TokenId
		private.NodeName = snapshot.PrivateData.NodeName
		private.BillingContext = snapshot.PrivateData.BillingContext
		updates := map[string]any{}
		if snapshot.PrivateData.UpstreamTaskID != "" {
			if private.UpstreamTaskID != "" && private.UpstreamTaskID != snapshot.PrivateData.UpstreamTaskID {
				return errors.New("generation receipt conflicts with the accepted task")
			}
			private.UpstreamTaskID = snapshot.PrivateData.UpstreamTaskID
			if receipt := snapshot.PrivateData.GenerationAcceptedResult; receipt != nil && current.HasVersionedSunburstImageInputs() {
				if receipt.Status != "" && receipt.Status != TaskStatusSuccess && receipt.Status != TaskStatusFailure {
					return errors.New("invalid generation accepted result status")
				}
				if receipt.Status == TaskStatusSuccess {
					parsed, err := url.Parse(receipt.URL)
					if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil || parsed.Fragment != "" || receipt.URL == "" {
						return errors.New("invalid generation original image result")
					}
				}
				if private.GenerationAcceptedResult != nil && !reflect.DeepEqual(private.GenerationAcceptedResult, receipt) {
					return errors.New("generation accepted result conflicts with original receipt")
				}
				private.GenerationAcceptedResult = receipt
				if receipt.Status == TaskStatusSuccess {
					private.ResultURL = receipt.URL
				}
				// Receipt facts and accepted outcome share one durable barrier.
				if routing := private.GenerationQueue.Routing; routing != nil && len(routing.Attempts) > 0 {
					attempt := &routing.Attempts[len(routing.Attempts)-1]
					if attempt.ChannelID != snapshot.ChannelId || (attempt.Outcome != GenerationAttemptPending && attempt.Outcome != GenerationAttemptUnknown && attempt.Outcome != GenerationAttemptAccepted) {
						return errors.New("generation receipt does not match the live attempt")
					}
					attempt.Outcome = GenerationAttemptAccepted
				}
			}
			if len(snapshot.PrivateData.PluginState) > 0 {
				private.PluginState = snapshot.PrivateData.PluginState
			}
			if len(snapshot.Data) > 0 {
				updates["data"] = snapshot.Data
			}
		}
		if private.GenerationQueue.SendStartedAt == 0 {
			private.GenerationQueue.SendStartedAt = common.GetTimestamp()
		}
		properties := snapshot.Properties
		properties.Input = current.Properties.Input
		updates["quota"], updates["channel_id"], updates["platform"], updates["group"] = snapshot.Quota, snapshot.ChannelId, snapshot.Platform, snapshot.Group
		updates["properties"], updates["private_data"], updates["fail_reason"] = properties, private, ""
		result := tx.Model(&Task{}).Where("id = ? AND user_id = ? AND task_id = ? AND status = ?", current.ID, reserved.UserId, reserved.TaskID, TaskStatusDispatching).Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		// Ownership is fenced by the locked row above. MySQL reports zero changed
		// rows when the post-submit funding snapshot matches the pre-send snapshot.
		savedQueue = private.GenerationQueue
		return nil
	})
	if err == nil {
		reserved.PrivateData.GenerationQueue = savedQueue
	}
	return err
}

// PersistGenerationAttemptOutcome records only the last live send. It keeps the
// task DISPATCHING and fences the scanner lease, account, and original owner.
func PersistGenerationAttemptOutcome(ctx context.Context, reserved *Task, scanner *SystemTask, runnerID string, channelID int, outcome string) error {
	if reserved == nil || scanner == nil || channelID <= 0 || !slices.Contains([]string{GenerationAttemptRejected, GenerationAttemptAccepted, GenerationAttemptUnknown}, outcome) {
		return errors.New("invalid generation attempt outcome")
	}
	var savedQueue *GenerationQueueRequest
	err := DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if common.UsingMainDatabase(common.DatabaseTypeSQLite) {
			if err := tx.Model(&User{}).Where("id = ?", reserved.UserId).UpdateColumn("status", gorm.Expr("status")).Error; err != nil {
				return err
			}
		}
		if err := lockGenerationScannerLease(tx, scanner, runnerID); err != nil {
			return err
		}
		var current Task
		if err := lockForUpdate(tx).First(&current, reserved.ID).Error; err != nil {
			return err
		}
		queue := current.PrivateData.GenerationQueue
		if current.UserId != reserved.UserId || current.TaskID != reserved.TaskID || current.Status != TaskStatusDispatching || queue == nil || reserved.PrivateData.GenerationQueue == nil || queue.ClaimOwner != reserved.PrivateData.GenerationQueue.ClaimOwner || queue.ClaimOwner != scanner.TaskID || queue.ReleaseStartedAt != 0 || queue.Routing == nil || len(queue.Routing.Attempts) == 0 {
			return errors.New("generation attempt ownership lost")
		}
		attempt := &queue.Routing.Attempts[len(queue.Routing.Attempts)-1]
		if attempt.ChannelID != channelID || (attempt.Outcome != GenerationAttemptPending && attempt.Outcome != outcome) {
			return errors.New("generation attempt outcome is already final")
		}
		attempt.Outcome = outcome
		if err := tx.Model(&Task{}).Where("id = ? AND status = ?", current.ID, TaskStatusDispatching).Update("private_data", current.PrivateData).Error; err != nil {
			return err
		}
		savedQueue = queue
		return nil
	})
	if err == nil {
		reserved.PrivateData.GenerationQueue = savedQueue
	}
	return err
}

// MarkGenerationSubmissionUnknown preserves funding and the slot for explicit reconciliation.
func MarkGenerationSubmissionUnknown(ctx context.Context, reserved *Task, reason string) error {
	return DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if common.UsingMainDatabase(common.DatabaseTypeSQLite) {
			if err := tx.Model(&Task{}).Where("id = ?", reserved.ID).UpdateColumn("status", gorm.Expr("status")).Error; err != nil {
				return err
			}
		}
		var current Task
		if err := lockForUpdate(tx).First(&current, reserved.ID).Error; err != nil {
			return err
		}
		if current.UserId != reserved.UserId || current.TaskID != reserved.TaskID || current.Status != TaskStatusDispatching || current.PrivateData.GenerationQueue == nil || reserved.PrivateData.GenerationQueue == nil || current.PrivateData.GenerationQueue.ClaimOwner != reserved.PrivateData.GenerationQueue.ClaimOwner {
			return errors.New("generation reconciliation ownership lost")
		}
		// A live worker that has finished with a durable receipt can hand
		// observation off immediately, even while its scanner runs other jobs.
		if current.HasVersionedSunburstImageInputs() && current.PrivateData.UpstreamTaskID != "" {
			return recoverVersionedImageDispatch(tx, &current)
		}
		updates := map[string]any{"fail_reason": reason}
		if current.HasVersionedSunburstImageInputs() {
			current.PrivateData.ImageResultUnverified = true
			updates["private_data"] = current.PrivateData
		}
		return tx.Model(&Task{}).Where("id = ? AND status = ?", current.ID, TaskStatusDispatching).Updates(updates).Error
	})
}

// PersistGenerationSubmission updates the reserved public row, never inserts a
// second row. The structured history snapshot survives the native submission.
func PersistGenerationSubmission(ctx context.Context, task *Task, reserved *Task) error {
	task.ID = reserved.ID
	task.TaskID = reserved.TaskID
	task.Action = reserved.Action
	task.CreatedAt = reserved.CreatedAt
	task.Properties.Input = reserved.Properties.Input
	return DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if common.UsingMainDatabase(common.DatabaseTypeSQLite) {
			if err := tx.Model(&Task{}).Where("id = ?", reserved.ID).UpdateColumn("status", gorm.Expr("status")).Error; err != nil {
				return err
			}
		}
		var current Task
		if err := lockForUpdate(tx).First(&current, reserved.ID).Error; err != nil {
			return err
		}
		if current.PrivateData.GenerationQueue != nil || reserved.PrivateData.GenerationQueue != nil {
			if current.PrivateData.GenerationQueue == nil || reserved.PrivateData.GenerationQueue == nil || current.PrivateData.GenerationQueue.ClaimOwner != reserved.PrivateData.GenerationQueue.ClaimOwner {
				return errors.New("generation persistence ownership lost")
			}
			if current.PrivateData.GenerationQueue.Routing != nil {
				task.PrivateData.GenerationRouting = current.PrivateData.GenerationQueue.Routing
				task.PrivateData.ImageInputs = current.PrivateData.GenerationQueue.ImageInputs
				task.PrivateData.GenerationAcceptedResult = current.PrivateData.GenerationAcceptedResult
				task.PrivateData.GenerationQueue = nil
			}
		}
		if current.HasVersionedSunburstImageInputs() {
			task.PrivateData.GenerationReservationUnlogged = true
			task.PrivateData.GenerationConsumptionLogState = TaskConsumptionLogPending
		}
		task.PrivateData.ObjectSwapPreparation = current.PrivateData.ObjectSwapPreparation
		result := tx.Model(&Task{}).Where("id = ? AND user_id = ? AND status = ?", reserved.ID, reserved.UserId, TaskStatusDispatching).Select("*").Updates(task)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("generation persistence ownership lost")
		}
		return nil
	})
}
