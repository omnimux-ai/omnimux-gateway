package controller

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service/hostedaccount"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
)

var hostedSvc = hostedaccount.DefaultService()

func setHostedService(s *hostedaccount.Service) {
	hostedSvc = s
}

func hostedError(c *gin.Context, err error) {
	if err == nil {
		return
	}
	code := "HOSTED_ERROR"
	status := http.StatusBadRequest
	switch {
	case errors.Is(err, hostedaccount.ErrGatewayUnavailable):
		code, status = "HOSTED_GATEWAY_UNAVAILABLE", http.StatusServiceUnavailable
	case errors.Is(err, hostedaccount.ErrChannelNotReady), errors.Is(err, hostedaccount.ErrNotReadyToDistribute):
		code, status = "CHANNEL_NOT_READY", http.StatusConflict
	case errors.Is(err, hostedaccount.ErrChannelSuspended):
		code, status = "CHANNEL_SUSPENDED", http.StatusForbidden
	case errors.Is(err, hostedaccount.ErrChannelReplacing):
		code, status = "CHANNEL_REPLACING", http.StatusConflict
	case errors.Is(err, hostedaccount.ErrOrderLimit):
		code, status = "HOSTED_ORDER_LIMIT", http.StatusTooManyRequests
	case errors.Is(err, hostedaccount.ErrIdempotentConflict):
		code, status = "HOSTED_TASK_IDEMPOTENT_CONFLICT", http.StatusConflict
	case errors.Is(err, hostedaccount.ErrReservedPhone):
		code, status = "HOSTED_RESERVED_PHONE", http.StatusForbidden
	case errors.Is(err, hostedaccount.ErrCredKeyMissing):
		code, status = "HOSTED_CRED_KEY_MISSING", http.StatusServiceUnavailable
	case errors.Is(err, hostedaccount.ErrCredInvalid):
		code, status = "HOSTED_CRED_INVALID", http.StatusBadRequest
	case errors.Is(err, hostedaccount.ErrChannelNotFound),
		errors.Is(err, hostedaccount.ErrOrderNotFound),
		errors.Is(err, hostedaccount.ErrTaskNotFound):
		code, status = "HOSTED_NOT_FOUND", http.StatusNotFound
	case errors.Is(err, hostedaccount.ErrBalanceInsufficient):
		code, status = "HOSTED_BALANCE_INSUFFICIENT", http.StatusPaymentRequired
	case errors.Is(err, model.ErrHostedInvalidQuantity),
		errors.Is(err, hostedaccount.ErrInvalidRequest),
		errors.Is(err, hostedaccount.ErrSKUDisabled):
		code, status = "HOSTED_INVALID_REQUEST", http.StatusBadRequest
	}
	c.JSON(status, gin.H{"success": false, "code": code, "message": err.Error()})
}

func parseIDParam(c *gin.Context, name string) (int64, bool) {
	id, err := strconv.ParseInt(c.Param(name), 10, 64)
	if err != nil || id <= 0 {
		hostedError(c, hostedaccount.ErrInvalidRequest)
		return 0, false
	}
	return id, true
}

// HostedStatus GET /api/hosted/v1/status
func HostedStatus(c *gin.Context) {
	common.ApiSuccess(c, hostedSvc.Status())
}

// HostedListSKUs GET /api/hosted/v1/skus
func HostedListSKUs(c *gin.Context) {
	rows, err := hostedSvc.ListSKUs(c.Request.Context())
	if err != nil {
		hostedError(c, err)
		return
	}
	common.ApiSuccess(c, rows)
}

type hostedOrderRequest struct {
	SkuID          int64                    `json:"sku_id"`
	Quantity       int                      `json:"quantity"`
	IdempotencyKey string                   `json:"idempotency_key"`
	Brand          hostedaccount.BrandInput `json:"brand"`
}

// HostedCreateOrder POST /api/hosted/v1/orders
func HostedCreateOrder(c *gin.Context) {
	if !operation_setting.IsPaymentComplianceConfirmed() {
		hostedError(c, hostedaccount.ErrInvalidRequest)
		return
	}
	var req hostedOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		hostedError(c, hostedaccount.ErrInvalidRequest)
		return
	}
	res, err := hostedSvc.CreateOrder(c.Request.Context(), c.GetInt("id"), hostedaccount.CreateOrderInput{
		SkuID:          req.SkuID,
		Quantity:       req.Quantity,
		IdempotencyKey: req.IdempotencyKey,
		Brand:          req.Brand,
	})
	if err != nil {
		hostedError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"order_id":    res.Order.ID,
		"status":      res.Order.Status,
		"quantity":    res.Order.Quantity,
		"deadline_at": res.Order.DeadlineAt,
		"sub_id":      res.SubOrderID,
		"amount_usd":  res.AmountUSD,
	})
}

// HostedGetOrder GET /api/hosted/v1/orders/:id
func HostedGetOrder(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	out, err := hostedSvc.GetOrder(c.Request.Context(), c.GetInt("id"), id)
	if err != nil {
		hostedError(c, err)
		return
	}
	common.ApiSuccess(c, out)
}

type hostedAddonRequest struct {
	SkuID          int64  `json:"sku_id"`
	Quantity       int    `json:"quantity"`
	Confirm        bool   `json:"confirm"`
	IdempotencyKey string `json:"idempotency_key"`
}

// HostedAddon POST /api/hosted/v1/subscriptions/:id/addons
func HostedAddon(c *gin.Context) {
	if !operation_setting.IsPaymentComplianceConfirmed() {
		hostedError(c, hostedaccount.ErrInvalidRequest)
		return
	}
	skuID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	var req hostedAddonRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		hostedError(c, hostedaccount.ErrInvalidRequest)
		return
	}
	if req.SkuID == 0 {
		req.SkuID = skuID
	}
	if !req.Confirm {
		quote, err := hostedSvc.QuoteAddon(c.Request.Context(), c.GetInt("id"), req.SkuID, req.Quantity)
		if err != nil {
			hostedError(c, err)
			return
		}
		common.ApiSuccess(c, quote)
		return
	}
	res, err := hostedSvc.ConfirmAddon(c.Request.Context(), c.GetInt("id"), req.SkuID, req.Quantity, req.IdempotencyKey)
	if err != nil {
		hostedError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"order_id":   res.Order.ID,
		"status":     res.Order.Status,
		"quantity":   res.Order.Quantity,
		"amount_usd": res.AmountUSD,
		"expire_at":  res.Entitlement.ExpireAt,
	})
}

type hostedRenewRequest struct {
	SkuID          int64  `json:"sku_id"`
	IdempotencyKey string `json:"idempotency_key"`
}

// HostedRenew POST /api/hosted/v1/subscriptions/:id/renew
func HostedRenew(c *gin.Context) {
	if !operation_setting.IsPaymentComplianceConfirmed() {
		hostedError(c, hostedaccount.ErrInvalidRequest)
		return
	}
	skuID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	var req hostedRenewRequest
	_ = c.ShouldBindJSON(&req)
	if req.SkuID == 0 {
		req.SkuID = skuID
	}
	res, err := hostedSvc.Renew(c.Request.Context(), c.GetInt("id"), req.SkuID, req.IdempotencyKey)
	if err != nil {
		hostedError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"order_id":   res.Order.ID,
		"status":     res.Order.Status,
		"quantity":   res.Order.Quantity,
		"amount_usd": res.AmountUSD,
		"expire_at":  res.Entitlement.ExpireAt,
	})
}

// HostedListChannels GET /api/hosted/v1/channels
func HostedListChannels(c *gin.Context) {
	rows, err := hostedSvc.ListChannels(c.Request.Context(), c.GetInt("id"), c.Query("status"))
	if err != nil {
		hostedError(c, err)
		return
	}
	common.ApiSuccess(c, rows)
}

// HostedGetChannel GET /api/hosted/v1/channels/:id
func HostedGetChannel(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	out, err := hostedSvc.GetChannel(c.Request.Context(), c.GetInt("id"), id)
	if err != nil {
		hostedError(c, err)
		return
	}
	common.ApiSuccess(c, out)
}

// HostedPatchBrand PATCH /api/hosted/v1/channels/:id/brand
func HostedPatchBrand(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	var brand hostedaccount.BrandInput
	if err := c.ShouldBindJSON(&brand); err != nil {
		hostedError(c, hostedaccount.ErrInvalidRequest)
		return
	}
	if err := hostedSvc.PatchBrand(c.Request.Context(), c.GetInt("id"), id, brand); err != nil {
		hostedError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"ok": true})
}

type hostedTaskRequest struct {
	Type           string         `json:"type"`
	ScheduledAt    int64          `json:"scheduled_at"`
	MediaURL       string         `json:"media_url"`
	Caption        string         `json:"caption"`
	Payload        map[string]any `json:"payload"`
	IdempotencyKey string         `json:"idempotency_key"`
}

// HostedCreateTask POST /api/hosted/v1/channels/:id/tasks
func HostedCreateTask(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	var req hostedTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		hostedError(c, hostedaccount.ErrInvalidRequest)
		return
	}
	task, err := hostedSvc.CreateTask(c.Request.Context(), c.GetInt("id"), id, hostedaccount.CreateTaskInput{
		Type:           req.Type,
		ScheduledAt:    req.ScheduledAt,
		MediaURL:       req.MediaURL,
		Caption:        req.Caption,
		Payload:        req.Payload,
		IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		hostedError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"task_id": task.ID, "status": task.Status})
}

type hostedBatchRequest struct {
	ChannelIDs     []int64        `json:"channel_ids"`
	MediaURL       string         `json:"media_url"`
	Caption        string         `json:"caption"`
	Payload        map[string]any `json:"payload"`
	ScheduledAt    int64          `json:"scheduled_at"`
	IdempotencyKey string         `json:"idempotency_key"`
}

// HostedBatchTasks POST /api/hosted/v1/tasks/batch
func HostedBatchTasks(c *gin.Context) {
	var req hostedBatchRequest
	if err := c.ShouldBindJSON(&req); err != nil || len(req.ChannelIDs) == 0 {
		hostedError(c, hostedaccount.ErrInvalidRequest)
		return
	}
	out := hostedSvc.BatchDistribute(c.Request.Context(), c.GetInt("id"), req.ChannelIDs, hostedaccount.CreateTaskInput{
		Type:           model.HostedTaskDistribute,
		ScheduledAt:    req.ScheduledAt,
		MediaURL:       req.MediaURL,
		Caption:        req.Caption,
		Payload:        req.Payload,
		IdempotencyKey: req.IdempotencyKey,
	})
	common.ApiSuccess(c, out)
}

// HostedListChannelTasks GET /api/hosted/v1/channels/:id/tasks
func HostedListChannelTasks(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	rows, err := hostedSvc.ListTasks(c.Request.Context(), c.GetInt("id"), id)
	if err != nil {
		hostedError(c, err)
		return
	}
	common.ApiSuccess(c, rows)
}

// HostedGetTask GET /api/hosted/v1/tasks/:id
func HostedGetTask(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	out, err := hostedSvc.GetTask(c.Request.Context(), c.GetInt("id"), id)
	if err != nil {
		hostedError(c, err)
		return
	}
	common.ApiSuccess(c, out)
}

// HostedCancelTask POST /api/hosted/v1/tasks/:id/cancel
func HostedCancelTask(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	if err := hostedSvc.CancelTask(c.Request.Context(), c.GetInt("id"), id); err != nil {
		hostedError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"ok": true})
}

type hostedPresignRequest struct {
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
}

// HostedPresignMedia POST /api/hosted/v1/media/presign
func HostedPresignMedia(c *gin.Context) {
	var req hostedPresignRequest
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Filename) == "" {
		hostedError(c, hostedaccount.ErrInvalidRequest)
		return
	}
	out, err := hostedSvc.PresignMedia(c.Request.Context(), req.Filename, req.ContentType)
	if err != nil {
		hostedError(c, err)
		return
	}
	common.ApiSuccess(c, out)
}
