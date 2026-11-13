package hostedaccount

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service/hostedgateway"
	"gorm.io/gorm"
)

const (
	warmupPlaybook       = "warmup7_v1"
	warmupReadyPlayCount = 500
	warmupMaxRounds      = 2
)

type warmupDayPlan struct {
	Day         int      `json:"day"`
	Action      string   `json:"action"`
	Keywords    []string `json:"keywords"`
	DurationSec int      `json:"duration_sec"`
}

func warmupFastForward() bool {
	v := strings.TrimSpace(os.Getenv("HOSTED_WARMUP_FAST_FORWARD"))
	return v == "1" || strings.EqualFold(v, "true")
}

func warmupClipURL() string {
	u := strings.TrimSpace(os.Getenv("HOSTED_WARMUP_CLIP_URL"))
	if u != "" {
		return u
	}
	return "https://example.invalid/hosted-warmup-d7.mp4"
}

// RunWarmupTick generates the day's internal_warmup rows and (when due) checks D7 playCount.
func (s *Service) RunWarmupTick(ctx context.Context) (int, error) {
	var accounts []*model.HostedAccount
	err := model.DB.Where("status = ?", model.HostedAccountWarmingUp).Find(&accounts).Error
	if err != nil {
		return 0, err
	}
	n := 0
	for _, acc := range accounts {
		if isReservedPhone(acc.PhoneID) {
			continue
		}
		touched, err := s.advanceWarmupAccount(ctx, acc)
		if err != nil {
			continue
		}
		if touched {
			n++
		}
	}
	return n, nil
}

func (s *Service) advanceWarmupAccount(ctx context.Context, acc *model.HostedAccount) (bool, error) {
	var plan model.HostedWarmupPlan
	err := model.DB.Where("account_id = ?", acc.ID).
		Where("playbook_code = ?", warmupPlaybook).
		Order("id desc").
		First(&plan).Error
	if err != nil {
		if errorsIsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	switch plan.State {
	case "running", "generating", "verifying", "extend_round1", "extend_round2":
	default:
		if strings.HasPrefix(plan.State, "extend_round") {
			break
		}
		return false, nil
	}

	now := common.GetTimestamp()
	if strings.TrimSpace(plan.Schedule) == "" {
		sched := defaultWarmupSchedule(acc)
		raw, mErr := common.Marshal(sched)
		if mErr != nil {
			return false, mErr
		}
		if err := model.DB.Model(&plan).Updates(map[string]any{
			"schedule":   string(raw),
			"state":      "running",
			"updated_at": now,
		}).Error; err != nil {
			return false, err
		}
		plan.Schedule = string(raw)
		plan.State = "running"
	}

	days := parseWarmupSchedule(plan.Schedule)
	if len(days) == 0 {
		days = defaultWarmupSchedule(acc)
	}
	fast := warmupFastForward()
	created := false

	if plan.State != "verifying" && !strings.HasPrefix(plan.State, "extend_round") {
		start := plan.CurrentDay
		if start < 1 {
			start = 1
		}
		end := start
		if fast {
			end = 7
		} else if start <= 7 {
			// Idempotent re-tick: if today's internal_warmup already exists, advance one day.
			key := warmupIdempotencyKey(acc.ID, start)
			if _, err := model.GetHostedTaskByIdempotency(key); err == nil {
				if start < 7 {
					start++
				}
				end = start
			}
		}
		for day := start; day <= end && day <= 7; day++ {
			spec := daySpec(days, day)
			if ensureWarmupTask(acc, spec, day, now) {
				created = true
			}
			plan.CurrentDay = day
		}
		updates := map[string]any{"current_day": plan.CurrentDay, "updated_at": now, "state": "running"}
		if fast || plan.CurrentDay >= 7 {
			if ensureD7ClipTask(acc, now) {
				created = true
			}
			updates["state"] = "verifying"
			plan.State = "verifying"
		}
		_ = model.DB.Model(&plan).Updates(updates).Error
	}

	if plan.State == "verifying" || strings.HasPrefix(plan.State, "extend_round") {
		if err := s.evaluateD7(ctx, acc, &plan, now); err != nil {
			return created, nil
		}
		created = true
	}
	return created, nil
}

func (s *Service) evaluateD7(ctx context.Context, acc *model.HostedAccount, plan *model.HostedWarmupPlan, now int64) error {
	if s == nil || !s.GatewayReady() {
		return nil
	}
	page, err := s.gw.AnalyticsData(ctx, map[string]any{
		"account": acc.PhoneID,
		"envId":   acc.PhoneID,
	})
	if err != nil {
		return err
	}
	play, stale := aggregatePlayCount(page)
	if stale || play < 0 {
		// Must not treat -1 as 0 or as ready.
		_ = model.DB.Model(plan).Updates(map[string]any{
			"state":      "verifying",
			"updated_at": now,
		}).Error
		return nil
	}
	if play >= warmupReadyPlayCount {
		return markWarmupReady(acc, plan, now)
	}
	round := plan.Round + 1
	state := "failed_manual"
	accWarmup := model.HostedWarmupFailedManual
	if round <= warmupMaxRounds {
		state = fmt.Sprintf("extend_round%d", round)
		accWarmup = model.HostedWarmupInProgress
	}
	_ = model.DB.Model(plan).Updates(map[string]any{
		"round":      round,
		"state":      state,
		"updated_at": now,
	}).Error
	_ = model.DB.Model(acc).Updates(map[string]any{
		"warmup_status": accWarmup,
		"updated_at":    now,
	}).Error
	return nil
}

func markWarmupReady(acc *model.HostedAccount, plan *model.HostedWarmupPlan, now int64) error {
	if err := model.DB.Model(plan).Updates(map[string]any{
		"state":      "passed",
		"updated_at": now,
	}).Error; err != nil {
		return err
	}
	if err := model.DB.Model(acc).Updates(map[string]any{
		"status":        model.HostedAccountReady,
		"warmup_status": model.HostedWarmupReady,
		"readiness":     100,
		"updated_at":    now,
	}).Error; err != nil {
		return err
	}
	acc.Status = model.HostedAccountReady
	acc.WarmupStatus = model.HostedWarmupReady
	acc.Readiness = 100
	plan.State = "passed"
	return nil
}

func aggregatePlayCount(page *hostedgateway.AnalyticsDataPage) (play int64, stale bool) {
	if page == nil || len(page.Items) == 0 {
		return -1, true
	}
	var max int64
	seen := false
	for _, it := range page.Items {
		if it.PlayCountStale || it.PlayCount < 0 {
			stale = true
			continue
		}
		if !seen || it.PlayCount > max {
			max = it.PlayCount
			seen = true
		}
	}
	if !seen {
		return -1, true
	}
	// Any stale or negative row poisons the aggregate; max>=500 must not clear it.
	return max, stale
}

func ensureWarmupTask(acc *model.HostedAccount, spec warmupDayPlan, day int, now int64) bool {
	key := warmupIdempotencyKey(acc.ID, day)
	if _, err := model.GetHostedTaskByIdempotency(key); err == nil {
		return false
	}
	payload, _ := common.Marshal(map[string]any{
		"action":       spec.Action,
		"keywords":     spec.Keywords,
		"duration_sec": spec.DurationSec,
		"day":          day,
	})
	task := &model.HostedTask{
		AccountID:      acc.ID,
		UserID:         0,
		Type:           model.HostedTaskInternalWarmup,
		Status:         model.HostedTaskPending,
		ScheduledAt:    now,
		IdempotencyKey: key,
		Payload:        string(payload),
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := model.DB.Create(task).Error; err != nil {
		return false
	}
	return true
}

func ensureD7ClipTask(acc *model.HostedAccount, now int64) bool {
	key := fmt.Sprintf("%d:d7_clip", acc.ID)
	if _, err := model.GetHostedTaskByIdempotency(key); err == nil {
		return false
	}
	payload, _ := common.Marshal(map[string]any{"kind": "d7_clip", "day": 7})
	task := &model.HostedTask{
		AccountID:        acc.ID,
		UserID:           0,
		Type:             model.HostedTaskInternalWarmup,
		Status:           model.HostedTaskPending,
		ScheduledAt:      now,
		IdempotencyKey:   key,
		MediaURL:         warmupClipURL(),
		Payload:          string(payload),
		PreConsumedQuota: 0,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := model.DB.Create(task).Error; err != nil {
		return false
	}
	return true
}

func warmupIdempotencyKey(accountID int64, day int) string {
	if warmupFastForward() {
		return fmt.Sprintf("%d:internal_warmup:fast-%d", accountID, day)
	}
	return fmt.Sprintf("%d:internal_warmup:%d", accountID, day)
}

func defaultWarmupSchedule(acc *model.HostedAccount) []warmupDayPlan {
	keywords := brandKeywords(acc.BrandConfig)
	out := make([]warmupDayPlan, 0, 7)
	jitter := int(acc.ID%17) + 45
	for d := 1; d <= 7; d++ {
		out = append(out, warmupDayPlan{
			Day:         d,
			Action:      "browse",
			Keywords:    keywords,
			DurationSec: jitter + d,
		})
	}
	return out
}

func parseWarmupSchedule(raw string) []warmupDayPlan {
	var days []warmupDayPlan
	if err := common.Unmarshal([]byte(raw), &days); err != nil {
		return nil
	}
	return days
}

func daySpec(days []warmupDayPlan, day int) warmupDayPlan {
	for _, d := range days {
		if d.Day == day {
			if d.Action == "" {
				d.Action = "browse"
			}
			return d
		}
	}
	return warmupDayPlan{Day: day, Action: "browse", DurationSec: 60}
}

func brandKeywords(raw string) []string {
	brand := decodeBrandMap(raw)
	switch v := brand["keywords"].(type) {
	case []any:
		out := make([]string, 0, len(v))
		for _, it := range v {
			s, _ := it.(string)
			if strings.TrimSpace(s) != "" {
				out = append(out, s)
			}
		}
		if len(out) > 0 {
			return out
		}
	case []string:
		if len(v) > 0 {
			return v
		}
	}
	nick := brandString(brand, "nick_name")
	if nick != "" {
		return []string{nick}
	}
	return []string{"fyp"}
}

func errorsIsNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}
