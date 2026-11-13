package hostedaccount

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service/hostedgateway"
)

const dispatchBatchLimit = 50

// DispatchDueTasks CAS-claims due pending rows and submits them to the gateway.
// CreateTask stays pending; this is the only outbound path for hosted tasks.
func (s *Service) DispatchDueTasks(ctx context.Context) (int, error) {
	if s == nil || !s.GatewayReady() {
		return 0, nil
	}
	now := common.GetTimestamp()
	var tasks []*model.HostedTask
	err := model.DB.Where("status = ? AND scheduled_at <= ?", model.HostedTaskPending, now).
		Order("id asc").
		Limit(dispatchBatchLimit).
		Find(&tasks).Error
	if err != nil {
		return 0, err
	}
	n := 0
	for _, t := range tasks {
		if s.dispatchOne(ctx, t, now) {
			n++
		}
	}
	return n, nil
}

func (s *Service) dispatchOne(ctx context.Context, t *model.HostedTask, now int64) bool {
	claimed := model.DB.Model(&model.HostedTask{}).
		Where("id = ? AND status = ?", t.ID, model.HostedTaskPending).
		Updates(map[string]any{
			"status":     model.HostedTaskQueued,
			"updated_at": now,
		})
	if claimed.Error != nil || claimed.RowsAffected != 1 {
		return false
	}
	t.Status = model.HostedTaskQueued

	acc, err := model.GetHostedAccountByID(t.AccountID)
	if err != nil {
		failDispatchedTask(t, "HOSTED_NOT_FOUND", err.Error(), now)
		return true
	}
	if isReservedPhone(acc.PhoneID) {
		failDispatchedTask(t, "HOSTED_RESERVED_PHONE", ErrReservedPhone.Error(), now)
		return true
	}

	jobID, upstreamID, rpcErr := s.submitGateway(ctx, t, acc)
	if rpcErr != nil {
		code, msg := gatewayErrFields(rpcErr)
		failDispatchedTask(t, code, msg, now)
		return true
	}
	updates := map[string]any{"updated_at": now, "status": model.HostedTaskQueued}
	if jobID == "" && upstreamID == "" {
		// Local / sync task/add path with no jobs id: treat as delivered (H2).
		updates["status"] = model.HostedTaskSucceeded
		t.Status = model.HostedTaskSucceeded
	} else {
		if jobID != "" {
			updates["gateway_job_id"] = jobID
			t.GatewayJobID = jobID
		}
		if upstreamID != "" {
			updates["upstream_task_id"] = upstreamID
			t.UpstreamTaskID = upstreamID
			if jobID == "" {
				updates["gateway_job_id"] = upstreamID
				t.GatewayJobID = upstreamID
			}
		}
	}
	_ = model.DB.Model(t).Updates(updates).Error
	return true
}

func (s *Service) submitGateway(ctx context.Context, t *model.HostedTask, acc *model.HostedAccount) (jobID, upstreamID string, err error) {
	switch t.Type {
	case model.HostedTaskProfileEdit:
		env, e := s.gw.TiktokEdit(ctx, tiktokEditBody(t, acc))
		if e != nil {
			return "", "", e
		}
		id := jobIDFromData(env.Data)
		return id, id, nil
	case model.HostedTaskDistribute, model.HostedTaskInternalWarmup, model.HostedTaskInternalMetrics:
		res, e := s.gw.TaskAdd(ctx, taskAddBody(t, acc))
		if e != nil {
			return "", "", e
		}
		id := ""
		if res != nil && len(res.TaskIDs) > 0 {
			id = res.TaskIDs[0]
		}
		return id, id, nil
	default:
		return "", "", ErrInvalidRequest
	}
}

func tiktokEditBody(t *model.HostedTask, acc *model.HostedAccount) map[string]any {
	brand := decodeBrandMap(acc.BrandConfig)
	body := map[string]any{
		"id":       acc.PhoneID,
		"envId":    acc.PhoneID,
		"nickName": brandString(brand, "nick_name"),
		"avatar":   brandString(brand, "avatar_url"),
		"bio":      brandString(brand, "bio"),
		"site":     brandString(brand, "site"),
	}
	if acc.CredentialsEnc != "" {
		plain, err := Open(acc.CredentialsEnc)
		if err == nil {
			if plain.Username != "" {
				body["username"] = plain.Username
			}
			if plain.Password != "" {
				body["password"] = plain.Password
			}
			if plain.TOTPSecret != "" {
				body["totp"] = plain.TOTPSecret
			}
		} else {
			logger.LogWarn(context.Background(), "hosted dispatch: credential envelope unreadable account_id="+strconv.FormatInt(acc.ID, 10))
		}
	}
	_ = t
	return body
}

func taskAddBody(t *model.HostedTask, acc *model.HostedAccount) map[string]any {
	payload := decodePayloadMap(t.Payload)
	brand := decodeBrandMap(acc.BrandConfig)
	caption, _ := payload["caption"].(string)
	if strings.TrimSpace(caption) == "" {
		caption = brandString(brand, "site")
	}
	taskType := 1
	switch t.Type {
	case model.HostedTaskInternalWarmup:
		if payloadKind(payload) == "d7_clip" {
			taskType = 1
		} else {
			taskType = 2
		}
	case model.HostedTaskDistribute:
		if strings.TrimSpace(t.MediaURL) == "" {
			taskType = 3
		} else {
			taskType = 1
		}
	default:
		taskType = 1
	}
	body := map[string]any{
		"envId":     acc.PhoneID,
		"taskType":  taskType,
		"videoDesc": caption,
	}
	if t.MediaURL != "" {
		body["videoUrl"] = t.MediaURL
		body["video"] = t.MediaURL
	}
	if taskType == 2 {
		if action, _ := payload["action"].(string); action != "" {
			body["action"] = action
		}
		if dur, ok := asInt(payload["duration_sec"]); ok {
			body["duration"] = dur
		}
		if kws, ok := payload["keywords"].([]any); ok {
			body["keywords"] = kws
		} else if ks, ok := payload["keywords"].([]string); ok {
			body["keywords"] = ks
		}
	}
	return body
}

func payloadKind(payload map[string]any) string {
	k, _ := payload["kind"].(string)
	return strings.TrimSpace(k)
}

func failDispatchedTask(t *model.HostedTask, code, msg string, now int64) {
	if len(msg) > 255 {
		msg = msg[:255]
	}
	updates := map[string]any{
		"status":     model.HostedTaskFailed,
		"error_code": code,
		"error_msg":  msg,
		"updated_at": now,
	}
	mergeRefund(t, updates)
	_ = model.DB.Model(t).Updates(updates).Error
	t.Status = model.HostedTaskFailed
	t.ErrorCode = code
	t.ErrorMsg = msg
}

func mergeRefund(t *model.HostedTask, updates map[string]any) {
	if t.UserID > 0 && t.PreConsumedQuota > 0 && t.SettledQuota == 0 {
		if err := model.IncreaseUserQuota(t.UserID, int(t.PreConsumedQuota), true); err != nil {
			logger.LogWarn(context.Background(), "hosted refund failed task_id="+strconv.FormatInt(t.ID, 10))
			return
		}
		updates["pre_consumed_quota"] = 0
		updates["settled_quota"] = 0
		t.PreConsumedQuota = 0
		t.SettledQuota = 0
	}
}

func gatewayErrFields(err error) (code, msg string) {
	if err == nil {
		return "HOSTED_ERROR", ""
	}
	var api *hostedgateway.APIError
	if errors.As(err, &api) && api != nil {
		code = strings.TrimSpace(api.Code)
		if code == "" {
			code = strconv.Itoa(api.Status)
		}
		msg = api.Message
		if msg == "" {
			msg = api.Error()
		}
		return code, msg
	}
	return "HOSTED_ERROR", err.Error()
}

func isJobsGetNotFound(err error) bool {
	var api *hostedgateway.APIError
	if errors.As(err, &api) && api != nil {
		return api.Status == http.StatusNotFound
	}
	return false
}

func isReservedPhone(phoneID string) bool {
	return strings.TrimSpace(phoneID) == ReservedMonthlyPhoneID
}

func jobIDFromData(data any) string {
	if data == nil {
		return ""
	}
	raw, err := common.Marshal(data)
	if err != nil {
		return ""
	}
	var box struct {
		JobID  string `json:"jobId"`
		TaskID string `json:"taskId"`
		ID     string `json:"id"`
	}
	if err := common.Unmarshal(raw, &box); err != nil {
		return ""
	}
	if box.JobID != "" {
		return box.JobID
	}
	if box.TaskID != "" {
		return box.TaskID
	}
	return box.ID
}

func decodePayloadMap(raw string) map[string]any {
	if strings.TrimSpace(raw) == "" {
		return map[string]any{}
	}
	var out map[string]any
	if err := common.Unmarshal([]byte(raw), &out); err != nil || out == nil {
		return map[string]any{}
	}
	return out
}

func decodeBrandMap(raw string) map[string]any {
	decoded := decodeBrand(raw)
	obj, ok := decoded.(map[string]any)
	if !ok || obj == nil {
		return map[string]any{}
	}
	return obj
}

func brandString(brand map[string]any, key string) string {
	s, _ := brand[key].(string)
	return strings.TrimSpace(s)
}

func asInt(v any) (int, bool) {
	switch n := v.(type) {
	case float64:
		return int(n), true
	case int:
		return n, true
	case int64:
		return int(n), true
	default:
		return 0, false
	}
}
