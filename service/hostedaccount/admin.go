package hostedaccount

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

// ReservedMonthlyPhoneID is the do-not-allocate monthly runner.
const ReservedMonthlyPhoneID = "630202623882428895"

func (s *Service) AdminListPool(_ context.Context) ([]map[string]any, error) {
	rows, err := model.ListAllHostedAccounts()
	if err != nil {
		return nil, err
	}
	usernames, err := hostedUsernames(rows)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(rows))
	for _, a := range rows {
		out = append(out, adminPoolRow(a, usernames[a.AssignedUserID]))
	}
	return out, nil
}

func (s *Service) AdminListOrders(_ context.Context) ([]map[string]any, error) {
	rows, err := model.ListAllHostedOrders()
	if err != nil {
		return nil, err
	}
	skus, err := skuByID()
	if err != nil {
		return nil, err
	}
	usernames, err := hostedOrderUsernames(rows)
	if err != nil {
		return nil, err
	}
	amounts, err := hostedOrderAmounts(rows)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(rows))
	for _, o := range rows {
		skuLabel := fmt.Sprintf("%d", o.SkuID)
		if sku := skus[o.SkuID]; sku != nil {
			skuLabel = sku.Platform + " / " + sku.Region
		}
		out = append(out, map[string]any{
			"id":          o.ID,
			"user":        usernames[o.UserID],
			"sku":         skuLabel,
			"kind":        o.Kind,
			"status":      o.Status,
			"quantity":    o.Quantity,
			"delivered":   o.DeliveredCount,
			"amount_usd":  amounts[o.ID],
			"deadline":    formatDeadlineUTC(o.DeadlineAt),
			"deadline_at": o.DeadlineAt,
		})
	}
	return out, nil
}

func (s *Service) AdminListTasks(_ context.Context) ([]map[string]any, error) {
	rows, err := model.ListAllHostedTasks()
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(rows))
	for _, t := range rows {
		out = append(out, adminTaskRow(t))
	}
	return out, nil
}

func (s *Service) AdminListSKUs(_ context.Context) ([]map[string]any, error) {
	rows, err := model.ListAllHostedSKUs()
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(rows))
	for _, sku := range rows {
		row, err := adminSKURow(sku)
		if err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, nil
}

func (s *Service) SetSKUEnabled(_ context.Context, id int64, enabled bool) (map[string]any, error) {
	if id <= 0 {
		return nil, ErrInvalidRequest
	}
	sku, err := model.GetHostedSKU(id)
	if err != nil {
		return nil, mapBillingErr(err)
	}
	if sku.Enabled != enabled {
		if err := model.SetHostedSKUEnabled(id, enabled); err != nil {
			return nil, mapBillingErr(err)
		}
		sku.Enabled = enabled
	}
	return adminSKURow(sku)
}

func (s *Service) AdminSync(ctx context.Context) (map[string]any, error) {
	n, err := s.SyncPhones(ctx)
	if err != nil {
		return nil, err
	}
	return map[string]any{"upserted": n}, nil
}

func (s *Service) AdminCancelTask(_ context.Context, id int64) (map[string]any, error) {
	if id <= 0 {
		return nil, ErrInvalidRequest
	}
	t, err := model.GetHostedTaskByID(id)
	if err != nil {
		return nil, mapBillingErr(err)
	}
	if err := cancelTaskRow(t); err != nil {
		return nil, err
	}
	return adminTaskRow(t), nil
}

func (s *Service) AdminRetryTask(_ context.Context, id int64) (map[string]any, error) {
	if id <= 0 {
		return nil, ErrInvalidRequest
	}
	if !s.GatewayReady() {
		return nil, ErrGatewayUnavailable
	}
	src, err := model.GetHostedTaskByID(id)
	if err != nil {
		return nil, mapBillingErr(err)
	}
	if src.Status != model.HostedTaskFailed {
		return nil, ErrInvalidRequest
	}
	now := common.GetTimestamp()
	retry := &model.HostedTask{
		AccountID:      src.AccountID,
		UserID:         src.UserID,
		Type:           src.Type,
		Status:         model.HostedTaskPending,
		ScheduledAt:    now,
		IdempotencyKey: fmt.Sprintf("retry:%d:%d", src.ID, now),
		MediaURL:       src.MediaURL,
		Payload:        src.Payload,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := model.DB.Create(retry).Error; err != nil {
		return nil, err
	}
	return adminTaskRow(retry), nil
}

func (s *Service) AdminGetAccount(_ context.Context, id int64) (map[string]any, error) {
	if id <= 0 {
		return nil, ErrInvalidRequest
	}
	acc, err := model.GetHostedAccountByID(id)
	if err != nil {
		return nil, mapBillingErr(err)
	}
	buyer := ""
	if acc.AssignedUserID > 0 {
		names, err := lookupUsernames([]int{acc.AssignedUserID})
		if err != nil {
			return nil, err
		}
		buyer = names[acc.AssignedUserID]
	}
	tasks, err := model.ListHostedTasksByAccount(id, 5)
	if err != nil {
		return nil, err
	}
	recent := make([]map[string]any, 0, len(tasks))
	for _, t := range tasks {
		recent = append(recent, adminTaskRow(t))
	}
	row := adminPoolRow(acc, buyer)
	row["tasks"] = recent
	return row, nil
}

type PutCredentialsInput struct {
	Username      string `json:"username"`
	Password      string `json:"password"`
	TOTPSecret    string `json:"totp_secret"`
	RecoveryEmail string `json:"recovery_email"`
}

// PutCredentials seals the vault envelope onto the account. Reserved phones may
// be written (SOP) but T02 still refuses outbound edits on them.
func (s *Service) PutCredentials(_ context.Context, id int64, in PutCredentialsInput) (map[string]any, error) {
	if id <= 0 {
		return nil, ErrInvalidRequest
	}
	acc, err := model.GetHostedAccountByID(id)
	if err != nil {
		return nil, mapBillingErr(err)
	}
	enc, err := Seal(CredentialPlain{
		Username:      strings.TrimSpace(in.Username),
		Password:      in.Password,
		TOTPSecret:    strings.TrimSpace(in.TOTPSecret),
		RecoveryEmail: strings.TrimSpace(in.RecoveryEmail),
	})
	if err != nil {
		return nil, err
	}
	now := common.GetTimestamp()
	if err := model.DB.Model(acc).Updates(map[string]any{
		"credentials_enc": enc,
		"updated_at":      now,
	}).Error; err != nil {
		return nil, err
	}
	return map[string]any{
		"ok":         true,
		"account_id": acc.ID,
	}, nil
}

func (s *Service) SetAllocatable(_ context.Context, id int64, allocatable bool) (map[string]any, error) {
	if id <= 0 {
		return nil, ErrInvalidRequest
	}
	acc, err := model.GetHostedAccountByID(id)
	if err != nil {
		return nil, mapBillingErr(err)
	}
	if acc.PhoneID == ReservedMonthlyPhoneID && allocatable {
		return nil, ErrReservedPhone
	}
	if acc.Allocatable == allocatable {
		return adminPoolRow(acc, ""), nil
	}
	if err := model.SetHostedAccountAllocatable(id, allocatable); err != nil {
		return nil, mapBillingErr(err)
	}
	acc.Allocatable = allocatable
	return adminPoolRow(acc, ""), nil
}

func adminPoolRow(a *model.HostedAccount, buyer string) map[string]any {
	remark := strings.TrimSpace(a.DisplayName)
	if remark == "" {
		remark = a.PhoneID
	}
	group, country := parsePoolTags(a.Tags)
	var buyerVal any
	if strings.TrimSpace(buyer) != "" {
		buyerVal = buyer
	}
	var brand any
	if nick := brandNick(a.BrandConfig); nick != "" {
		brand = nick
	}
	return map[string]any{
		"id":           a.ID,
		"phone_id":     a.PhoneID,
		"remark":       remark,
		"group":        group,
		"country_name": country,
		"platform":     a.Platform,
		"region":       a.Region,
		"status":       a.Status,
		"allocatable":  a.Allocatable,
		"readiness":    a.Readiness,
		"buyer":        buyerVal,
		"brand":        brand,
	}
}

func adminSKURow(sku *model.HostedSKU) (map[string]any, error) {
	stock, err := model.CountAvailableHostedAccounts(sku.Platform, sku.Region)
	if err != nil {
		return nil, err
	}
	open, err := model.CountOpenHostedOrdersBySKU(sku.ID)
	if err != nil {
		return nil, err
	}
	est := 0
	if stock == 0 {
		est = model.HostedSLAHours
	}
	return map[string]any{
		"id":                 sku.ID,
		"platform":           sku.Platform,
		"region":             sku.Region,
		"price_usd":          sku.PriceUSD,
		"enabled":            sku.Enabled,
		"stock":              stock,
		"open_orders":        open,
		"est_delivery_hours": est,
	}, nil
}

func adminTaskRow(t *model.HostedTask) map[string]any {
	note := strings.TrimSpace(t.ErrorMsg)
	if note == "" {
		note = t.ErrorCode
	}
	return map[string]any{
		"id":         t.ID,
		"channel_id": t.AccountID,
		"type":       t.Type,
		"status":     t.Status,
		"receipt":    taskReceipt(t.Metrics),
		"note":       note,
	}
}

func taskReceipt(raw string) any {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var parsed map[string]any
	if err := common.Unmarshal([]byte(raw), &parsed); err != nil {
		return nil
	}
	views, hasViews := asFloat(parsed["views"])
	likes, hasLikes := asFloat(parsed["likes"])
	comments, hasComments := asFloat(parsed["comments"])
	if !hasViews && !hasLikes && !hasComments {
		return nil
	}
	return map[string]any{
		"views":    views,
		"likes":    likes,
		"comments": comments,
	}
}

func asFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	default:
		return 0, false
	}
}

func parsePoolTags(tags string) (group, country string) {
	for _, part := range strings.Split(tags, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		key, value, ok := strings.Cut(part, "=")
		if !ok {
			if group == "" {
				group = part
			}
			continue
		}
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "group":
			group = strings.TrimSpace(value)
		case "country", "countryname":
			country = strings.TrimSpace(value)
		}
	}
	return group, country
}

func brandNick(raw string) string {
	decoded := decodeBrand(raw)
	obj, ok := decoded.(map[string]any)
	if !ok {
		return ""
	}
	nick, _ := obj["nick_name"].(string)
	return strings.TrimSpace(nick)
}

func formatDeadlineUTC(ts int64) string {
	if ts <= 0 {
		return ""
	}
	return time.Unix(ts, 0).UTC().Format("2006-01-02 15:04 UTC")
}

func hostedUsernames(rows []*model.HostedAccount) (map[int]string, error) {
	ids := make([]int, 0)
	seen := map[int]struct{}{}
	for _, a := range rows {
		if a.AssignedUserID <= 0 {
			continue
		}
		if _, ok := seen[a.AssignedUserID]; ok {
			continue
		}
		seen[a.AssignedUserID] = struct{}{}
		ids = append(ids, a.AssignedUserID)
	}
	return lookupUsernames(ids)
}

func hostedOrderUsernames(rows []*model.HostedOrder) (map[int]string, error) {
	ids := make([]int, 0)
	seen := map[int]struct{}{}
	for _, o := range rows {
		if o.UserID <= 0 {
			continue
		}
		if _, ok := seen[o.UserID]; ok {
			continue
		}
		seen[o.UserID] = struct{}{}
		ids = append(ids, o.UserID)
	}
	return lookupUsernames(ids)
}

func lookupUsernames(ids []int) (map[int]string, error) {
	out := map[int]string{}
	if len(ids) == 0 {
		return out, nil
	}
	var users []model.User
	if err := model.DB.Select("id", "username").Where("id IN ?", ids).Find(&users).Error; err != nil {
		return nil, err
	}
	for _, u := range users {
		out[u.Id] = u.Username
	}
	return out, nil
}

func skuByID() (map[int64]*model.HostedSKU, error) {
	rows, err := model.ListAllHostedSKUs()
	if err != nil {
		return nil, err
	}
	out := make(map[int64]*model.HostedSKU, len(rows))
	for _, sku := range rows {
		out[sku.ID] = sku
	}
	return out, nil
}

func hostedOrderAmounts(rows []*model.HostedOrder) (map[int64]float64, error) {
	out := map[int64]float64{}
	ids := make([]int, 0)
	seen := map[int]struct{}{}
	for _, o := range rows {
		if o.SubOrderID <= 0 {
			continue
		}
		if _, ok := seen[o.SubOrderID]; ok {
			continue
		}
		seen[o.SubOrderID] = struct{}{}
		ids = append(ids, o.SubOrderID)
	}
	if len(ids) == 0 {
		return out, nil
	}
	var receipts []model.SubscriptionOrder
	if err := model.DB.Select("id", "money").Where("id IN ?", ids).Find(&receipts).Error; err != nil {
		return nil, err
	}
	money := map[int]float64{}
	for _, r := range receipts {
		money[r.Id] = r.Money
	}
	for _, o := range rows {
		out[o.ID] = money[o.SubOrderID]
	}
	return out, nil
}
