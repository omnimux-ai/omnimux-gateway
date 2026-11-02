package model

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

const (
	HostedPlatformTikTok = "tiktok"
	HostedRegionUS       = "us"

	HostedOrderKindInitial = "initial"
	HostedOrderKindAddon   = "addon"
	HostedOrderKindRenew   = "renew"

	HostedOrderCreated            = "created"
	HostedOrderPaid               = "paid"
	HostedOrderAllocated          = "allocated"
	HostedOrderPendingFulfillment = "pending_fulfillment"
	HostedOrderDelivered          = "delivered"
	HostedOrderCancelled          = "cancelled"
	HostedOrderRefundPartial      = "refund_partial"

	HostedAllocReserved  = "reserved"
	HostedAllocDelivered = "delivered"
	HostedAllocReleased  = "released"
	HostedAllocReplaced  = "replaced"

	HostedAccountProcuring   = "procuring"
	HostedAccountAvailable   = "available"
	HostedAccountReserved    = "reserved"
	HostedAccountAssigned    = "assigned"
	HostedAccountWarmingUp   = "warming_up"
	HostedAccountReady       = "ready"
	HostedAccountActive      = "active"
	HostedAccountSuspended   = "suspended"
	HostedAccountQuarantined = "quarantined"
	HostedAccountRecycled    = "recycled"

	HostedWarmupNone         = "none"
	HostedWarmupInProgress   = "in_progress"
	HostedWarmupReady        = "ready"
	HostedWarmupFailedManual = "failed_manual"

	HostedTaskProfileEdit     = "profile_edit"
	HostedTaskDistribute      = "distribute"
	HostedTaskInternalWarmup  = "internal_warmup"
	HostedTaskInternalMetrics = "internal_metrics"

	HostedTaskPending   = "pending"
	HostedTaskQueued    = "queued"
	HostedTaskRunning   = "running"
	HostedTaskSucceeded = "succeeded"
	HostedTaskFailed    = "failed"
	HostedTaskCancelled = "cancelled"

	HostedEntitlementActive    = "active"
	HostedEntitlementGrace     = "grace"
	HostedEntitlementExpired   = "expired"
	HostedEntitlementCancelled = "cancelled"

	HostedMaxOpenQuantity = 5
	HostedPeriodDays      = 30
	HostedGraceDays       = 7
	HostedRecycleDays     = 23
	HostedSLAHours        = 72
	HostedReserveMinutes  = 30
)

var (
	ErrHostedSKUNotFound         = errors.New("hosted sku not found")
	ErrHostedSKUDisabled         = errors.New("hosted sku disabled")
	ErrHostedInvalidQuantity     = errors.New("hosted quantity must be 1-5")
	ErrHostedBalanceInsufficient = errors.New("余额不足")
	ErrHostedOrderLimit          = errors.New("undelivered hosted quantity exceeds limit")
	ErrHostedOrderNotFound       = errors.New("hosted order not found")
	ErrHostedEntitlementNotFound = errors.New("hosted entitlement not found")
	ErrHostedAccountNotFound     = errors.New("hosted account not found")
	ErrHostedTaskNotFound        = errors.New("hosted task not found")
	ErrHostedIdempotentConflict  = errors.New("hosted idempotency conflict")
	ErrHostedInvalidAmount       = errors.New("hosted amount invalid")
)

// HostedSKU is a quantity-based hosted channel offer. Do not put quantity on UserSubscription.
type HostedSKU struct {
	ID        int64   `json:"id" gorm:"primaryKey;autoIncrement"`
	Platform  string  `json:"platform" gorm:"type:varchar(32);index"`
	Region    string  `json:"region" gorm:"type:varchar(32);index"`
	PriceUSD  float64 `json:"price_usd" gorm:"type:numeric"`
	PlanId    int     `json:"plan_id" gorm:"index"`
	Enabled   bool    `json:"enabled"`
	SortOrder int     `json:"sort_order"`
	CreatedAt int64   `json:"created_at"`
	UpdatedAt int64   `json:"updated_at"`
}

func (HostedSKU) TableName() string { return "hosted_skus" }

type HostedAccount struct {
	ID             int64  `json:"id" gorm:"primaryKey;autoIncrement"`
	PhoneID        string `json:"phone_id" gorm:"type:varchar(64);uniqueIndex"`
	Platform       string `json:"platform" gorm:"type:varchar(32);index"`
	Region         string `json:"region" gorm:"type:varchar(32);index"`
	Status         string `json:"status" gorm:"type:varchar(32);index"`
	WarmupStatus   string `json:"warmup_status" gorm:"type:varchar(32)"`
	Readiness      int    `json:"readiness"`
	Allocatable    bool   `json:"allocatable"`
	AssignedUserID int    `json:"assigned_user_id" gorm:"index"`
	AssignedSubID  int    `json:"assigned_sub_id" gorm:"index"`
	BrandConfig    string `json:"-" gorm:"type:text"`
	CredentialsEnc string `json:"-" gorm:"type:text"`
	DisplayName    string `json:"display_name" gorm:"type:varchar(191)"`
	Tags           string `json:"tags" gorm:"type:varchar(255)"`
	MonthlyExpire  int64  `json:"monthly_expire"`
	LastSyncAt     int64  `json:"last_sync_at"`
	CreatedAt      int64  `json:"created_at"`
	UpdatedAt      int64  `json:"updated_at"`
}

func (HostedAccount) TableName() string { return "hosted_accounts" }

type HostedOrder struct {
	ID             int64  `json:"id" gorm:"primaryKey;autoIncrement"`
	UserID         int    `json:"user_id" gorm:"index"`
	SkuID          int64  `json:"sku_id"`
	SubOrderID     int    `json:"sub_order_id" gorm:"index"`
	SubID          int    `json:"sub_id" gorm:"index"`
	Kind           string `json:"kind" gorm:"type:varchar(16)"`
	Quantity       int    `json:"quantity"`
	Status         string `json:"status" gorm:"type:varchar(32);index"`
	AllocatedCount int    `json:"allocated_count"`
	DeliveredCount int    `json:"delivered_count"`
	ProrationDays  int    `json:"proration_days"`
	DeadlineAt     int64  `json:"deadline_at"`
	IdempotencyKey string `json:"idempotency_key" gorm:"type:varchar(128);uniqueIndex"`
	BrandConfig    string `json:"-" gorm:"type:text"`
	CreatedAt      int64  `json:"created_at"`
	UpdatedAt      int64  `json:"updated_at"`
}

func (HostedOrder) TableName() string { return "hosted_orders" }

type HostedAllocation struct {
	ID             int64  `json:"id" gorm:"primaryKey;autoIncrement"`
	OrderID        int64  `json:"order_id" gorm:"index"`
	AccountID      int64  `json:"account_id" gorm:"index"`
	State          string `json:"state" gorm:"type:varchar(32)"`
	ReservedAt     int64  `json:"reserved_at"`
	DeliveredAt    int64  `json:"delivered_at"`
	ReplacedFromID int64  `json:"replaced_from_id"`
}

func (HostedAllocation) TableName() string { return "hosted_allocations" }

type HostedWarmupPlan struct {
	ID            int64  `json:"id" gorm:"primaryKey;autoIncrement"`
	AccountID     int64  `json:"account_id" gorm:"index"`
	PlaybookCode  string `json:"playbook_code" gorm:"type:varchar(32)"`
	BrandKeywords string `json:"brand_keywords" gorm:"type:text"`
	Schedule      string `json:"schedule" gorm:"type:text"`
	CurrentDay    int    `json:"current_day"`
	State         string `json:"state" gorm:"type:varchar(32)"`
	Round         int    `json:"round"`
	CreatedAt     int64  `json:"created_at"`
	UpdatedAt     int64  `json:"updated_at"`
}

func (HostedWarmupPlan) TableName() string { return "hosted_warmup_plans" }

type HostedTask struct {
	ID               int64  `json:"id" gorm:"primaryKey;autoIncrement"`
	AccountID        int64  `json:"account_id" gorm:"index"`
	UserID           int    `json:"user_id" gorm:"index"`
	Type             string `json:"type" gorm:"type:varchar(32)"`
	Status           string `json:"status" gorm:"type:varchar(32);index"`
	ScheduledAt      int64  `json:"scheduled_at"`
	IdempotencyKey   string `json:"idempotency_key" gorm:"type:varchar(128);uniqueIndex"`
	GatewayJobID     string `json:"gateway_job_id" gorm:"type:varchar(64)"`
	UpstreamTaskID   string `json:"upstream_task_id" gorm:"type:varchar(64)"`
	MediaURL         string `json:"media_url" gorm:"type:text"`
	Payload          string `json:"payload" gorm:"type:text"`
	Metrics          string `json:"metrics" gorm:"type:text"`
	PreConsumedQuota int64  `json:"pre_consumed_quota"`
	SettledQuota     int64  `json:"settled_quota"`
	ErrorCode        string `json:"error_code" gorm:"type:varchar(64)"`
	ErrorMsg         string `json:"error_msg" gorm:"type:varchar(255)"`
	CreatedAt        int64  `json:"created_at"`
	UpdatedAt        int64  `json:"updated_at"`
}

func (HostedTask) TableName() string { return "hosted_tasks" }

type HostedEntitlement struct {
	ID         int64  `json:"id" gorm:"primaryKey;autoIncrement"`
	UserID     int    `json:"user_id" gorm:"uniqueIndex:uk_hosted_entitlement,priority:1"`
	SkuID      int64  `json:"sku_id" gorm:"uniqueIndex:uk_hosted_entitlement,priority:2"`
	Quantity   int    `json:"quantity"`
	ExpireAt   int64  `json:"expire_at"`
	GraceUntil int64  `json:"grace_until"`
	Status     string `json:"status" gorm:"type:varchar(32);index"`
	CreatedAt  int64  `json:"created_at"`
	UpdatedAt  int64  `json:"updated_at"`
}

func (HostedEntitlement) TableName() string { return "hosted_entitlements" }

func hostedNow() int64 { return common.GetTimestamp() }

func GetHostedSKU(id int64) (*HostedSKU, error) {
	var sku HostedSKU
	if err := DB.First(&sku, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrHostedSKUNotFound
		}
		return nil, err
	}
	return &sku, nil
}

func ListEnabledHostedSKUs() ([]*HostedSKU, error) {
	var rows []*HostedSKU
	err := DB.Where("enabled = ?", true).Order("sort_order asc, id asc").Find(&rows).Error
	return rows, err
}

func CountAvailableHostedAccounts(platform, region string) (int64, error) {
	var n int64
	err := DB.Model(&HostedAccount{}).
		Where("platform = ? AND region = ? AND status = ? AND allocatable = ? AND assigned_user_id = 0",
			platform, region, HostedAccountAvailable, true).
		Count(&n).Error
	return n, err
}

func CountOpenHostedQuantity(userID int, skuID int64) (int, error) {
	var sum int
	err := DB.Model(&HostedOrder{}).
		Select("COALESCE(SUM(quantity - delivered_count), 0)").
		Where("user_id = ? AND sku_id = ? AND status IN ?", userID, skuID, []string{
			HostedOrderPaid, HostedOrderAllocated, HostedOrderPendingFulfillment,
		}).
		Scan(&sum).Error
	return sum, err
}

func GetHostedOrderForUser(id int64, userID int) (*HostedOrder, error) {
	var o HostedOrder
	err := DB.Where("id = ? AND user_id = ?", id, userID).First(&o).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrHostedOrderNotFound
	}
	return &o, err
}

func GetHostedOrderByIdempotency(userID int, key string) (*HostedOrder, error) {
	if key == "" {
		return nil, gorm.ErrRecordNotFound
	}
	var o HostedOrder
	err := DB.Where("user_id = ? AND idempotency_key = ?", userID, key).First(&o).Error
	return &o, err
}

func GetHostedAccountForUser(id int64, userID int) (*HostedAccount, error) {
	var a HostedAccount
	err := DB.Where("id = ? AND assigned_user_id = ?", id, userID).First(&a).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrHostedAccountNotFound
	}
	return &a, err
}

func ListHostedAccountsForUser(userID int, status string) ([]*HostedAccount, error) {
	var rows []*HostedAccount
	q := DB.Where("assigned_user_id = ?", userID)
	if status != "" {
		q = q.Where("status = ?", status)
	}
	err := q.Order("id desc").Find(&rows).Error
	return rows, err
}

func GetHostedTaskForUser(id int64, userID int) (*HostedTask, error) {
	var t HostedTask
	err := DB.Where("id = ? AND user_id = ?", id, userID).First(&t).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrHostedTaskNotFound
	}
	return &t, err
}

func ListHostedTasksForAccount(accountID int64, userID int) ([]*HostedTask, error) {
	var rows []*HostedTask
	err := DB.Where("account_id = ? AND user_id = ?", accountID, userID).Order("id desc").Find(&rows).Error
	return rows, err
}

func GetHostedTaskByIdempotency(key string) (*HostedTask, error) {
	var t HostedTask
	err := DB.Where("idempotency_key = ?", key).First(&t).Error
	return &t, err
}

func GetHostedEntitlement(userID int, skuID int64) (*HostedEntitlement, error) {
	var e HostedEntitlement
	err := DB.Where("user_id = ? AND sku_id = ?", userID, skuID).First(&e).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrHostedEntitlementNotFound
	}
	return &e, err
}

func ListHostedAllocations(orderID int64) ([]*HostedAllocation, error) {
	var rows []*HostedAllocation
	err := DB.Where("order_id = ?", orderID).Order("id asc").Find(&rows).Error
	return rows, err
}

func HostedUSDToQuota(amountUSD float64) (int, *common.QuotaClamp, error) {
	if amountUSD < 0 {
		return 0, nil, ErrHostedInvalidAmount
	}
	if amountUSD == 0 {
		return 0, nil, nil
	}
	if common.QuotaPerUnit <= 0 {
		return 0, nil, errors.New("额度单位配置错误")
	}
	d := decimal.NewFromFloat(amountUSD).Mul(decimal.NewFromFloat(common.QuotaPerUnit)).Ceil()
	quota, clamp := common.QuotaFromDecimalChecked(d)
	if clamp != nil {
		return 0, clamp, clamp
	}
	if quota < 0 {
		return 0, nil, ErrHostedInvalidAmount
	}
	return quota, nil, nil
}

func HostedAddonAmountUSD(unitPrice float64, qty, remainingDays int) (float64, error) {
	if unitPrice < 0 || qty <= 0 {
		return 0, ErrHostedInvalidAmount
	}
	if remainingDays < 0 {
		remainingDays = 0
	}
	d := decimal.NewFromFloat(unitPrice).
		Mul(decimal.NewFromInt(int64(qty))).
		Mul(decimal.NewFromInt(int64(remainingDays))).
		Div(decimal.NewFromInt(HostedPeriodDays)).
		Ceil()
	f, _ := d.Float64()
	if f < 0 {
		return 0, ErrHostedInvalidAmount
	}
	return f, nil
}

func remainingHostedDays(expireAt, now int64) int {
	if expireAt <= now {
		return 0
	}
	d := int((expireAt - now + 86399) / 86400)
	if d > HostedPeriodDays {
		return HostedPeriodDays
	}
	return d
}

func HostedModels() []any {
	return []any{
		&HostedSKU{},
		&HostedAccount{},
		&HostedOrder{},
		&HostedAllocation{},
		&HostedWarmupPlan{},
		&HostedTask{},
		&HostedEntitlement{},
	}
}

func EnsureDefaultHostedSKU() error {
	var n int64
	if err := DB.Model(&HostedSKU{}).Count(&n).Error; err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	now := hostedNow()
	sku := &HostedSKU{
		Platform:  HostedPlatformTikTok,
		Region:    HostedRegionUS,
		PriceUSD:  99,
		Enabled:   true,
		SortOrder: 1,
		CreatedAt: now,
		UpdatedAt: now,
	}
	return DB.Create(sku).Error
}

func GetOrCreateHostedEntitlementTx(tx *gorm.DB, userID int, skuID int64) (*HostedEntitlement, error) {
	var e HostedEntitlement
	err := tx.Where("user_id = ? AND sku_id = ?", userID, skuID).First(&e).Error
	if err == nil {
		return &e, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	now := hostedNow()
	e = HostedEntitlement{
		UserID:    userID,
		SkuID:     skuID,
		Quantity:  0,
		Status:    HostedEntitlementActive,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := tx.Create(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

func TransitionHostedAccount(id int64, from, to string) error {
	now := hostedNow()
	res := DB.Model(&HostedAccount{}).
		Where("id = ? AND status = ?", id, from).
		Updates(map[string]any{"status": to, "updated_at": now})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		return fmt.Errorf("hosted account %d: cannot %s→%s", id, from, to)
	}
	return nil
}

func NormalizeHostedSKU(s *HostedSKU) {
	if s == nil {
		return
	}
	s.Platform = strings.ToLower(strings.TrimSpace(s.Platform))
	s.Region = strings.ToLower(strings.TrimSpace(s.Region))
	if s.CreatedAt == 0 {
		s.CreatedAt = hostedNow()
	}
	s.UpdatedAt = hostedNow()
}

func (s *HostedSKU) BeforeCreate(_ *gorm.DB) error {
	NormalizeHostedSKU(s)
	return nil
}

func ListAllHostedAccounts() ([]*HostedAccount, error) {
	var rows []*HostedAccount
	err := DB.Order("id asc").Find(&rows).Error
	return rows, err
}

func ListAllHostedOrders() ([]*HostedOrder, error) {
	var rows []*HostedOrder
	err := DB.Order("id desc").Find(&rows).Error
	return rows, err
}

func ListAllHostedTasks() ([]*HostedTask, error) {
	var rows []*HostedTask
	err := DB.Order("id desc").Find(&rows).Error
	return rows, err
}

func GetHostedTaskByID(id int64) (*HostedTask, error) {
	var t HostedTask
	err := DB.First(&t, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrHostedTaskNotFound
	}
	return &t, err
}

func ListAllHostedSKUs() ([]*HostedSKU, error) {
	var rows []*HostedSKU
	err := DB.Order("sort_order asc, id asc").Find(&rows).Error
	return rows, err
}

func GetHostedAccountByID(id int64) (*HostedAccount, error) {
	var a HostedAccount
	err := DB.First(&a, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrHostedAccountNotFound
	}
	return &a, err
}

func SetHostedAccountAllocatable(id int64, allocatable bool) error {
	now := hostedNow()
	res := DB.Model(&HostedAccount{}).Where("id = ?", id).
		Updates(map[string]any{"allocatable": allocatable, "updated_at": now})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		return ErrHostedAccountNotFound
	}
	return nil
}

func CountOpenHostedOrdersBySKU(skuID int64) (int64, error) {
	var n int64
	err := DB.Model(&HostedOrder{}).
		Where("sku_id = ? AND delivered_count < quantity AND status <> ?", skuID, HostedOrderCancelled).
		Count(&n).Error
	return n, err
}

func SetHostedSKUEnabled(id int64, enabled bool) error {
	now := hostedNow()
	res := DB.Model(&HostedSKU{}).Where("id = ?", id).
		Updates(map[string]any{"enabled": enabled, "updated_at": now})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		return ErrHostedSKUNotFound
	}
	return nil
}

func ListHostedTasksByAccount(accountID int64, limit int) ([]*HostedTask, error) {
	if limit <= 0 {
		limit = 5
	}
	var rows []*HostedTask
	err := DB.Where("account_id = ?", accountID).Order("id desc").Limit(limit).Find(&rows).Error
	return rows, err
}

func HostedPeriodSeconds() int64 { return int64(HostedPeriodDays) * 24 * int64(time.Hour/time.Second) }

func HostedGraceSeconds() int64 { return int64(HostedGraceDays) * 24 * int64(time.Hour/time.Second) }

func HostedSLASeconds() int64 { return int64(HostedSLAHours) * int64(time.Hour/time.Second) }
