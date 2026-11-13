package hostedaccount

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/service/hostedgateway"
)

func RegisterWorkers() {
	service.RegisterSystemTaskHandler(syncHandler{})
	service.RegisterSystemTaskHandler(deliverHandler{})
	service.RegisterSystemTaskHandler(warmupHandler{})
	service.RegisterSystemTaskHandler(pollHandler{})
}

func workersEnabled() bool {
	return common.GetEnvOrDefaultBool("HOSTED_WORKERS_ENABLED", true)
}

type syncHandler struct{}

func (syncHandler) Type() string { return model.SystemTaskTypeHostedSync }
func (syncHandler) Enabled() bool {
	return workersEnabled() && hostedgateway.NewFromEnv().Ready()
}
func (syncHandler) Interval() time.Duration { return 5 * time.Minute }
func (syncHandler) NewPayload() any         { return nil }
func (syncHandler) Run(ctx context.Context, task *model.SystemTask, runnerID string) {
	n, err := DefaultService().SyncPhones(ctx)
	finishWorker(task, runnerID, map[string]any{"upserted": n}, err)
}

type deliverHandler struct{}

func (deliverHandler) Type() string { return model.SystemTaskTypeHostedDeliver }
func (deliverHandler) Enabled() bool {
	return workersEnabled()
}
func (deliverHandler) Interval() time.Duration { return 30 * time.Second }
func (deliverHandler) NewPayload() any         { return nil }
func (deliverHandler) Run(_ context.Context, task *model.SystemTask, runnerID string) {
	n, err := DefaultService().DeliverOpenOrders()
	finishWorker(task, runnerID, map[string]any{"orders": n}, err)
}

type warmupHandler struct{}

func (warmupHandler) Type() string { return model.SystemTaskTypeHostedWarmup }
func (warmupHandler) Enabled() bool {
	return workersEnabled()
}
func (warmupHandler) Interval() time.Duration {
	if warmupFastForward() {
		return 30 * time.Second
	}
	return 24 * time.Hour
}
func (warmupHandler) NewPayload() any { return nil }
func (warmupHandler) Run(ctx context.Context, task *model.SystemTask, runnerID string) {
	_, _ = model.ApplyHostedGraceSweep(common.GetTimestamp())
	n, err := DefaultService().RunWarmupTick(ctx)
	finishWorker(task, runnerID, map[string]any{"warmup": n}, err)
}

type pollHandler struct{}

func (pollHandler) Type() string { return model.SystemTaskTypeHostedPoll }
func (pollHandler) Enabled() bool {
	return workersEnabled() && hostedgateway.NewFromEnv().Ready()
}
func (pollHandler) Interval() time.Duration { return 30 * time.Second }
func (pollHandler) NewPayload() any         { return nil }
func (pollHandler) Run(ctx context.Context, task *model.SystemTask, runnerID string) {
	svc := DefaultService()
	dispatched, err := svc.DispatchDueTasks(ctx)
	polled := 0
	if err == nil {
		polled, err = svc.PollGatewayJobs(ctx)
	}
	finishWorker(task, runnerID, map[string]any{"dispatched": dispatched, "polled": polled}, err)
}

func finishWorker(task *model.SystemTask, runnerID string, result any, runErr error) {
	msg := ""
	status := model.SystemTaskStatusSucceeded
	if runErr != nil {
		msg = runErr.Error()
		status = model.SystemTaskStatusFailed
		logger.LogWarn(context.Background(), "hosted worker: "+msg)
	}
	_ = model.FinishSystemTask(task.TaskID, runnerID, status, result, msg)
}

func MapRegion(groupName, countryName string) (platform, region string, ok bool) {
	g := strings.ToLower(strings.TrimSpace(groupName))
	c := strings.ToLower(strings.TrimSpace(countryName))
	if strings.Contains(g, "美区") || c == "usa" || c == "united states" || c == "us" {
		return model.HostedPlatformTikTok, model.HostedRegionUS, true
	}
	return "", "", false
}

func DoNotAllocate(phoneID string) bool {
	raw := strings.TrimSpace(os.Getenv("HOSTED_DO_NOT_ALLOCATE"))
	if raw == "" {
		return false
	}
	for _, id := range strings.Split(raw, ",") {
		if strings.TrimSpace(id) == phoneID {
			return true
		}
	}
	return false
}

func (s *Service) SyncPhones(ctx context.Context) (int, error) {
	if !s.GatewayReady() {
		return 0, ErrGatewayUnavailable
	}
	page, err := s.gw.PhoneList(ctx, 1, 50)
	if err != nil {
		return 0, err
	}
	now := common.GetTimestamp()
	n := 0
	for _, item := range page.Items {
		phoneID, _ := item["id"].(string)
		if phoneID == "" {
			if v, ok := item["envId"].(string); ok {
				phoneID = v
			}
		}
		if phoneID == "" {
			continue
		}
		groupName := nestedString(item, "group", "name")
		country := nestedString(item, "equipmentInfo", "countryName")
		platform, region, mapped := MapRegion(groupName, country)
		status := model.HostedAccountAvailable
		if !mapped {
			status = model.HostedAccountProcuring
		}
		allocatable := mapped && !DoNotAllocate(phoneID)
		tags := ""
		if raw, ok := item["tags"].(string); ok {
			tags = raw
		}
		var existing model.HostedAccount
		qerr := model.DB.Where("phone_id = ?", phoneID).First(&existing).Error
		if qerr != nil {
			acc := &model.HostedAccount{
				PhoneID:     phoneID,
				Platform:    platform,
				Region:      region,
				Status:      status,
				Allocatable: allocatable,
				Tags:        tags,
				LastSyncAt:  now,
				CreatedAt:   now,
				UpdatedAt:   now,
			}
			if err := model.DB.Create(acc).Error; err == nil {
				n++
			}
			continue
		}
		updates := map[string]any{
			"tags":         tags,
			"last_sync_at": now,
			"updated_at":   now,
		}
		if existing.AssignedUserID == 0 {
			updates["platform"] = platform
			updates["region"] = region
			updates["allocatable"] = allocatable
			if existing.Status == model.HostedAccountProcuring && mapped {
				updates["status"] = model.HostedAccountAvailable
			}
		}
		if err := model.DB.Model(&existing).Updates(updates).Error; err == nil {
			n++
		}
	}
	return n, nil
}

func (s *Service) PollGatewayJobs(ctx context.Context) (int, error) {
	if !s.GatewayReady() {
		return 0, nil
	}
	var tasks []*model.HostedTask
	err := model.DB.Where("status IN ? AND gateway_job_id <> ''", []string{
		model.HostedTaskQueued, model.HostedTaskRunning,
	}).Find(&tasks).Error
	if err != nil {
		return 0, err
	}
	n := 0
	for _, t := range tasks {
		job, err := s.gw.JobsGet(ctx, t.GatewayJobID, "")
		now := common.GetTimestamp()
		if err != nil {
			if isJobsGetNotFound(err) && strings.TrimSpace(t.GatewayJobID) != "" {
				_ = model.DB.Model(t).Updates(map[string]any{
					"status":     model.HostedTaskSucceeded,
					"updated_at": now,
				}).Error
				n++
			}
			continue
		}
		status := t.Status
		updates := map[string]any{
			"error_code": job.ErrorCode,
			"error_msg":  job.ErrorMsg,
			"updated_at": now,
		}
		switch strings.ToLower(job.Status) {
		case "succeeded", "success":
			status = model.HostedTaskSucceeded
		case "failed", "error":
			status = model.HostedTaskFailed
			t.Status = status
			mergeRefund(t, updates)
		case "cancelled", "canceled":
			status = model.HostedTaskCancelled
			t.Status = status
			mergeRefund(t, updates)
		case "running":
			status = model.HostedTaskRunning
		default:
			status = model.HostedTaskQueued
		}
		updates["status"] = status
		_ = model.DB.Model(t).Updates(updates)
		n++
	}
	return n, nil
}

func nestedString(m map[string]any, keys ...string) string {
	cur := any(m)
	for i, k := range keys {
		obj, ok := cur.(map[string]any)
		if !ok {
			return ""
		}
		v, ok := obj[k]
		if !ok {
			return ""
		}
		if i == len(keys)-1 {
			s, _ := v.(string)
			return s
		}
		cur = v
	}
	return ""
}
