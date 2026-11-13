package hostedaccount

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service/hostedgateway"
	"gorm.io/gorm"
)

type Service struct {
	gw *hostedgateway.Client
}

func NewService(gw *hostedgateway.Client) *Service {
	return &Service{gw: gw}
}

func DefaultService() *Service {
	return NewService(hostedgateway.NewFromEnv())
}

func (s *Service) GatewayReady() bool {
	return s != nil && s.gw != nil && s.gw.Ready()
}

func (s *Service) Status() map[string]any {
	return map[string]any{
		"enabled":       true,
		"gateway_ready": s.GatewayReady(),
		"billing_mode":  "wallet",
		"sku_platform":  model.HostedPlatformTikTok,
		"sku_region":    model.HostedRegionUS,
		"price_usd":     99,
	}
}

type BrandInput struct {
	NickName  string   `json:"nick_name"`
	AvatarURL string   `json:"avatar_url"`
	Bio       string   `json:"bio"`
	Site      string   `json:"site"`
	Keywords  []string `json:"keywords"`
}

type CreateOrderInput struct {
	SkuID          int64
	Quantity       int
	IdempotencyKey string
	Brand          BrandInput
}

func encodeBrand(b BrandInput) string {
	raw, err := common.Marshal(b)
	if err != nil {
		return ""
	}
	return string(raw)
}

func (s *Service) ListSKUs(_ context.Context) ([]map[string]any, error) {
	skus, err := model.ListEnabledHostedSKUs()
	if err != nil {
		return nil, err
	}
	if len(skus) == 0 {
		if err := model.EnsureDefaultHostedSKU(); err != nil {
			return nil, err
		}
		skus, err = model.ListEnabledHostedSKUs()
		if err != nil {
			return nil, err
		}
	}
	out := make([]map[string]any, 0, len(skus))
	for _, sku := range skus {
		stock, err := model.CountAvailableHostedAccounts(sku.Platform, sku.Region)
		if err != nil {
			return nil, err
		}
		est := 0
		if stock == 0 {
			est = model.HostedSLAHours
		}
		out = append(out, map[string]any{
			"id":                 sku.ID,
			"platform":           sku.Platform,
			"region":             sku.Region,
			"price_usd":          sku.PriceUSD,
			"stock":              stock,
			"est_delivery_hours": est,
		})
	}
	return out, nil
}

func (s *Service) CreateOrder(_ context.Context, userID int, in CreateOrderInput) (*model.HostedPurchaseResult, error) {
	if userID <= 0 || in.SkuID <= 0 {
		return nil, ErrInvalidRequest
	}
	res, err := model.PurchaseHostedChannelsWithBalance(userID, in.SkuID, in.Quantity, 0, model.HostedOrderKindInitial, in.IdempotencyKey, encodeBrand(in.Brand))
	if err != nil {
		return nil, mapBillingErr(err)
	}
	if !res.Idempotent {
		_ = s.DeliverPending(userID, res.Order.ID)
		if ent, err := model.GetHostedEntitlement(userID, in.SkuID); err == nil {
			res.Entitlement = ent
		}
		if order, err := model.GetHostedOrderForUser(res.Order.ID, userID); err == nil {
			res.Order = order
		}
	}
	return res, nil
}

func (s *Service) GetOrder(_ context.Context, userID int, orderID int64) (map[string]any, error) {
	order, err := model.GetHostedOrderForUser(orderID, userID)
	if err != nil {
		return nil, mapBillingErr(err)
	}
	allocs, err := model.ListHostedAllocations(order.ID)
	if err != nil {
		return nil, err
	}
	channels := make([]map[string]any, 0, len(allocs))
	for _, a := range allocs {
		acc, err := model.GetHostedAccountForUser(a.AccountID, userID)
		if err != nil {
			continue
		}
		channels = append(channels, map[string]any{
			"account_id": acc.ID,
			"status":     acc.Status,
			"readiness":  acc.Readiness,
		})
	}
	return map[string]any{
		"order_id":        order.ID,
		"status":          order.Status,
		"quantity":        order.Quantity,
		"delivered_count": order.DeliveredCount,
		"allocated_count": order.AllocatedCount,
		"deadline_at":     order.DeadlineAt,
		"kind":            order.Kind,
		"channels":        channels,
	}, nil
}

func (s *Service) QuoteAddon(_ context.Context, userID int, skuID int64, qty int) (map[string]any, error) {
	if err := validateQty(qty); err != nil {
		return nil, err
	}
	sku, err := model.GetHostedSKU(skuID)
	if err != nil {
		return nil, mapBillingErr(err)
	}
	ent, err := model.GetHostedEntitlement(userID, skuID)
	if err != nil {
		return nil, mapBillingErr(err)
	}
	days := remainingDays(ent.ExpireAt, common.GetTimestamp())
	amount, err := model.HostedAddonAmountUSD(sku.PriceUSD, qty, days)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"amount_usd":     amount,
		"proration_days": days,
		"expire_at":      ent.ExpireAt,
		"quantity":       qty,
	}, nil
}

func (s *Service) ConfirmAddon(_ context.Context, userID int, skuID int64, qty int, idempotencyKey string) (*model.HostedPurchaseResult, error) {
	ent, err := model.GetHostedEntitlement(userID, skuID)
	if err != nil {
		return nil, mapBillingErr(err)
	}
	days := remainingDays(ent.ExpireAt, common.GetTimestamp())
	res, err := model.PurchaseHostedChannelsWithBalance(userID, skuID, qty, days, model.HostedOrderKindAddon, idempotencyKey, "")
	if err != nil {
		return nil, mapBillingErr(err)
	}
	if !res.Idempotent {
		_ = s.DeliverPending(userID, res.Order.ID)
		if ent, err := model.GetHostedEntitlement(userID, skuID); err == nil {
			res.Entitlement = ent
		}
	}
	return res, nil
}

func (s *Service) Renew(_ context.Context, userID int, skuID int64, idempotencyKey string) (*model.HostedPurchaseResult, error) {
	res, err := model.RenewHostedEntitlement(userID, skuID, idempotencyKey)
	return res, mapBillingErr(err)
}

func (s *Service) ListChannels(_ context.Context, userID int, status string) ([]map[string]any, error) {
	rows, err := model.ListHostedAccountsForUser(userID, status)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(rows))
	for _, a := range rows {
		out = append(out, publicChannel(a, entitlementExpire(userID, a)))
	}
	return out, nil
}

func (s *Service) GetChannel(_ context.Context, userID int, id int64) (map[string]any, error) {
	a, err := model.GetHostedAccountForUser(id, userID)
	if err != nil {
		return nil, mapBillingErr(err)
	}
	view := publicChannel(a, entitlementExpire(userID, a))
	view["brand"] = decodeBrand(a.BrandConfig)
	return view, nil
}

func (s *Service) PatchBrand(_ context.Context, userID int, id int64, brand BrandInput) error {
	a, err := model.GetHostedAccountForUser(id, userID)
	if err != nil {
		return mapBillingErr(err)
	}
	now := common.GetTimestamp()
	return model.DB.Model(a).Updates(map[string]any{
		"brand_config": encodeBrand(brand),
		"updated_at":   now,
	}).Error
}

type CreateTaskInput struct {
	Type           string
	ScheduledAt    int64
	MediaURL       string
	Caption        string
	Payload        map[string]any
	IdempotencyKey string
}

func (s *Service) CreateTask(_ context.Context, userID int, accountID int64, in CreateTaskInput) (*model.HostedTask, error) {
	a, err := model.GetHostedAccountForUser(accountID, userID)
	if err != nil {
		return nil, mapBillingErr(err)
	}
	taskType := strings.TrimSpace(in.Type)
	if taskType != model.HostedTaskDistribute && taskType != model.HostedTaskProfileEdit {
		return nil, ErrInvalidRequest
	}
	if taskType == model.HostedTaskDistribute {
		if err := ChannelUserFacingBlocked(a.Status); err != nil {
			return nil, err
		}
	}
	key := strings.TrimSpace(in.IdempotencyKey)
	if key == "" {
		key = fmt.Sprintf("%d:%s:%d", accountID, taskType, common.GetTimestamp())
	}
	if existing, err := model.GetHostedTaskByIdempotency(key); err == nil {
		if existing.UserID != userID || existing.AccountID != accountID || existing.Type != taskType {
			return nil, ErrIdempotentConflict
		}
		return existing, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	payload := in.Payload
	if payload == nil {
		payload = map[string]any{}
	}
	if in.Caption != "" {
		payload["caption"] = in.Caption
	}
	raw, err := common.Marshal(payload)
	if err != nil {
		return nil, err
	}
	now := common.GetTimestamp()
	scheduled := in.ScheduledAt
	if scheduled <= 0 {
		scheduled = now
	}
	task := &model.HostedTask{
		AccountID:      accountID,
		UserID:         userID,
		Type:           taskType,
		Status:         model.HostedTaskPending,
		ScheduledAt:    scheduled,
		IdempotencyKey: key,
		MediaURL:       in.MediaURL,
		Payload:        string(raw),
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := model.DB.Create(task).Error; err != nil {
		return nil, err
	}
	return task, nil
}

func (s *Service) BatchDistribute(ctx context.Context, userID int, accountIDs []int64, in CreateTaskInput) map[string]any {
	accepted := make([]map[string]any, 0)
	rejected := make([]map[string]any, 0)
	in.Type = model.HostedTaskDistribute
	for _, id := range accountIDs {
		taskIn := in
		if in.IdempotencyKey != "" {
			taskIn.IdempotencyKey = fmt.Sprintf("%s:%d", in.IdempotencyKey, id)
		}
		task, err := s.CreateTask(ctx, userID, id, taskIn)
		if err != nil {
			rejected = append(rejected, map[string]any{"channel_id": id, "code": errCode(err), "message": err.Error()})
			continue
		}
		accepted = append(accepted, map[string]any{"channel_id": id, "task_id": task.ID, "status": task.Status})
	}
	return map[string]any{"accepted": accepted, "rejected": rejected}
}

func (s *Service) GetTask(_ context.Context, userID int, taskID int64) (map[string]any, error) {
	t, err := model.GetHostedTaskForUser(taskID, userID)
	if err != nil {
		return nil, mapBillingErr(err)
	}
	return publicTask(t), nil
}

func (s *Service) ListTasks(_ context.Context, userID int, accountID int64) ([]map[string]any, error) {
	if _, err := model.GetHostedAccountForUser(accountID, userID); err != nil {
		return nil, mapBillingErr(err)
	}
	rows, err := model.ListHostedTasksForAccount(accountID, userID)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(rows))
	for _, t := range rows {
		out = append(out, publicTask(t))
	}
	return out, nil
}

func (s *Service) CancelTask(_ context.Context, userID int, taskID int64) error {
	t, err := model.GetHostedTaskForUser(taskID, userID)
	if err != nil {
		return mapBillingErr(err)
	}
	return cancelTaskRow(t)
}

func cancelTaskRow(t *model.HostedTask) error {
	switch t.Status {
	case model.HostedTaskSucceeded, model.HostedTaskFailed, model.HostedTaskCancelled:
		return ErrInvalidRequest
	}
	now := common.GetTimestamp()
	updates := map[string]any{"status": model.HostedTaskCancelled, "updated_at": now}
	if t.PreConsumedQuota > 0 && t.SettledQuota == 0 {
		if err := model.IncreaseUserQuota(t.UserID, int(t.PreConsumedQuota), true); err != nil {
			return err
		}
		updates["settled_quota"] = 0
		updates["pre_consumed_quota"] = 0
	}
	if err := model.DB.Model(t).Updates(updates).Error; err != nil {
		return err
	}
	t.Status = model.HostedTaskCancelled
	t.UpdatedAt = now
	if t.PreConsumedQuota > 0 && t.SettledQuota == 0 {
		t.PreConsumedQuota = 0
		t.SettledQuota = 0
	}
	return nil
}

func (s *Service) PresignMedia(ctx context.Context, filename, contentType string) (map[string]any, error) {
	if !s.GatewayReady() {
		return nil, ErrGatewayUnavailable
	}
	filename = strings.TrimSpace(filename)
	if filename == "" {
		return nil, ErrInvalidRequest
	}
	url, err := s.gw.MaterialGetUploadURL(ctx, map[string]any{
		"fileName":    filename,
		"contentType": contentType,
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"upload_url":   url.UploadURL,
		"resource_url": url.ResourceURL,
	}, nil
}

func entitlementExpire(userID int, a *model.HostedAccount) int64 {
	var sku model.HostedSKU
	if err := model.DB.Where("platform = ? AND region = ?", a.Platform, a.Region).First(&sku).Error; err != nil {
		return 0
	}
	ent, err := model.GetHostedEntitlement(userID, sku.ID)
	if err != nil {
		return 0
	}
	return ent.ExpireAt
}

func publicChannel(a *model.HostedAccount, expireAt int64) map[string]any {
	return map[string]any{
		"id":           a.ID,
		"platform":     a.Platform,
		"region":       a.Region,
		"status":       a.Status,
		"readiness":    a.Readiness,
		"expire_at":    expireAt,
		"display_name": a.DisplayName,
	}
}

func publicTask(t *model.HostedTask) map[string]any {
	metrics := any(nil)
	if strings.TrimSpace(t.Metrics) != "" {
		var parsed any
		if err := common.Unmarshal([]byte(t.Metrics), &parsed); err == nil {
			metrics = parsed
		}
	}
	return map[string]any{
		"id":         t.ID,
		"account_id": t.AccountID,
		"type":       t.Type,
		"status":     t.Status,
		"metrics":    metrics,
		"error_code": t.ErrorCode,
		"created_at": t.CreatedAt,
	}
}

func decodeBrand(raw string) any {
	if strings.TrimSpace(raw) == "" {
		return map[string]any{}
	}
	var out any
	if err := common.Unmarshal([]byte(raw), &out); err != nil {
		return map[string]any{}
	}
	return out
}

func validateQty(qty int) error {
	if qty < 1 || qty > model.HostedMaxOpenQuantity {
		return model.ErrHostedInvalidQuantity
	}
	return nil
}

func remainingDays(expireAt, now int64) int {
	if expireAt <= now {
		return 0
	}
	d := int((expireAt - now + 86399) / 86400)
	if d > model.HostedPeriodDays {
		return model.HostedPeriodDays
	}
	return d
}

func mapBillingErr(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, model.ErrHostedSKUNotFound),
		errors.Is(err, model.ErrHostedSKUDisabled):
		return ErrSKUDisabled
	case errors.Is(err, model.ErrHostedBalanceInsufficient):
		return ErrBalanceInsufficient
	case errors.Is(err, model.ErrHostedOrderLimit):
		return ErrOrderLimit
	case errors.Is(err, model.ErrHostedOrderNotFound):
		return ErrOrderNotFound
	case errors.Is(err, model.ErrHostedAccountNotFound):
		return ErrChannelNotFound
	case errors.Is(err, model.ErrHostedTaskNotFound):
		return ErrTaskNotFound
	case errors.Is(err, model.ErrHostedIdempotentConflict):
		return ErrIdempotentConflict
	case errors.Is(err, model.ErrHostedEntitlementNotFound):
		return ErrOrderNotFound
	default:
		return err
	}
}

func errCode(err error) string {
	switch {
	case errors.Is(err, ErrChannelNotReady):
		return "CHANNEL_NOT_READY"
	case errors.Is(err, ErrChannelSuspended):
		return "CHANNEL_SUSPENDED"
	case errors.Is(err, ErrChannelReplacing):
		return "CHANNEL_REPLACING"
	case errors.Is(err, ErrGatewayUnavailable):
		return "HOSTED_GATEWAY_UNAVAILABLE"
	case errors.Is(err, ErrOrderLimit):
		return "HOSTED_ORDER_LIMIT"
	case errors.Is(err, ErrIdempotentConflict):
		return "HOSTED_TASK_IDEMPOTENT_CONFLICT"
	default:
		return "HOSTED_ERROR"
	}
}
