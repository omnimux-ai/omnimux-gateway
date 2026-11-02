package controller

import (
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service/hostedaccount"
	"github.com/gin-gonic/gin"
)

// HostedAdminListPool GET /api/hosted/admin/v1/pool
func HostedAdminListPool(c *gin.Context) {
	rows, err := hostedSvc.AdminListPool(c.Request.Context())
	if err != nil {
		hostedError(c, err)
		return
	}
	common.ApiSuccess(c, rows)
}

// HostedAdminListOrders GET /api/hosted/admin/v1/orders
func HostedAdminListOrders(c *gin.Context) {
	rows, err := hostedSvc.AdminListOrders(c.Request.Context())
	if err != nil {
		hostedError(c, err)
		return
	}
	common.ApiSuccess(c, rows)
}

// HostedAdminListTasks GET /api/hosted/admin/v1/tasks
func HostedAdminListTasks(c *gin.Context) {
	rows, err := hostedSvc.AdminListTasks(c.Request.Context())
	if err != nil {
		hostedError(c, err)
		return
	}
	common.ApiSuccess(c, rows)
}

// HostedAdminListSKUs GET /api/hosted/admin/v1/skus
func HostedAdminListSKUs(c *gin.Context) {
	rows, err := hostedSvc.AdminListSKUs(c.Request.Context())
	if err != nil {
		hostedError(c, err)
		return
	}
	common.ApiSuccess(c, rows)
}

type hostedAllocatableRequest struct {
	Allocatable *bool `json:"allocatable"`
}

// HostedAdminSetAllocatable PATCH /api/hosted/admin/v1/accounts/:id/allocatable
func HostedAdminSetAllocatable(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	var req hostedAllocatableRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Allocatable == nil {
		hostedError(c, hostedaccount.ErrInvalidRequest)
		return
	}
	row, err := hostedSvc.SetAllocatable(c.Request.Context(), id, *req.Allocatable)
	if err != nil {
		hostedError(c, err)
		return
	}
	adminID := c.GetInt("id")
	adminName := c.GetString("username")
	phone, _ := row["phone_id"].(string)
	model.RecordLogWithAdminInfo(adminID, model.LogTypeManage,
		fmt.Sprintf("hosted allocatable phone=%s id=%d set=%t", phone, id, *req.Allocatable),
		&model.AuditAdminInfo{AdminID: adminID, AdminUsername: adminName}, nil, c,
	)
	common.ApiSuccess(c, row)
}

type hostedSKUEnabledRequest struct {
	Enabled *bool `json:"enabled"`
}

// HostedAdminSetSKUEnabled PATCH /api/hosted/admin/v1/skus/:id
func HostedAdminSetSKUEnabled(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	var req hostedSKUEnabledRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Enabled == nil {
		hostedError(c, hostedaccount.ErrInvalidRequest)
		return
	}
	row, err := hostedSvc.SetSKUEnabled(c.Request.Context(), id, *req.Enabled)
	if err != nil {
		hostedError(c, err)
		return
	}
	adminID := c.GetInt("id")
	adminName := c.GetString("username")
	model.RecordLogWithAdminInfo(adminID, model.LogTypeManage,
		fmt.Sprintf("hosted sku id=%d enabled=%t", id, *req.Enabled),
		&model.AuditAdminInfo{AdminID: adminID, AdminUsername: adminName}, nil, c,
	)
	common.ApiSuccess(c, row)
}

// HostedAdminSync POST /api/hosted/admin/v1/sync
func HostedAdminSync(c *gin.Context) {
	row, err := hostedSvc.AdminSync(c.Request.Context())
	if err != nil {
		hostedError(c, err)
		return
	}
	adminID := c.GetInt("id")
	adminName := c.GetString("username")
	upserted, _ := row["upserted"].(int)
	model.RecordLogWithAdminInfo(adminID, model.LogTypeManage,
		fmt.Sprintf("hosted sync upserted=%d", upserted),
		&model.AuditAdminInfo{AdminID: adminID, AdminUsername: adminName}, nil, c,
	)
	common.ApiSuccess(c, row)
}

type hostedCredentialsRequest struct {
	Username      string `json:"username"`
	Password      string `json:"password"`
	TOTPSecret    string `json:"totp_secret"`
	RecoveryEmail string `json:"recovery_email"`
}

// HostedAdminPutCredentials POST /api/hosted/admin/v1/accounts/:id/credentials
func HostedAdminPutCredentials(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	var req hostedCredentialsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		hostedError(c, hostedaccount.ErrInvalidRequest)
		return
	}
	row, err := hostedSvc.PutCredentials(c.Request.Context(), id, hostedaccount.PutCredentialsInput{
		Username:      req.Username,
		Password:      req.Password,
		TOTPSecret:    req.TOTPSecret,
		RecoveryEmail: req.RecoveryEmail,
	})
	if err != nil {
		hostedError(c, err)
		return
	}
	adminID := c.GetInt("id")
	adminName := c.GetString("username")
	model.RecordLogWithAdminInfo(adminID, model.LogTypeManage,
		fmt.Sprintf("hosted credentials written id=%d", id),
		&model.AuditAdminInfo{AdminID: adminID, AdminUsername: adminName}, nil, c,
	)
	common.ApiSuccess(c, row)
}

// HostedAdminGetAccount GET /api/hosted/admin/v1/accounts/:id
func HostedAdminGetAccount(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	row, err := hostedSvc.AdminGetAccount(c.Request.Context(), id)
	if err != nil {
		hostedError(c, err)
		return
	}
	common.ApiSuccess(c, row)
}

// HostedAdminCancelTask POST /api/hosted/admin/v1/tasks/:id/cancel
func HostedAdminCancelTask(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	row, err := hostedSvc.AdminCancelTask(c.Request.Context(), id)
	if err != nil {
		hostedError(c, err)
		return
	}
	adminID := c.GetInt("id")
	adminName := c.GetString("username")
	model.RecordLogWithAdminInfo(adminID, model.LogTypeManage,
		fmt.Sprintf("hosted task cancel id=%d", id),
		&model.AuditAdminInfo{AdminID: adminID, AdminUsername: adminName}, nil, c,
	)
	common.ApiSuccess(c, row)
}

// HostedAdminRetryTask POST /api/hosted/admin/v1/tasks/:id/retry
func HostedAdminRetryTask(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	row, err := hostedSvc.AdminRetryTask(c.Request.Context(), id)
	if err != nil {
		hostedError(c, err)
		return
	}
	adminID := c.GetInt("id")
	adminName := c.GetString("username")
	retryID, _ := row["id"].(int64)
	model.RecordLogWithAdminInfo(adminID, model.LogTypeManage,
		fmt.Sprintf("hosted task retry id=%d new=%d", id, retryID),
		&model.AuditAdminInfo{AdminID: adminID, AdminUsername: adminName}, nil, c,
	)
	common.ApiSuccess(c, row)
}
