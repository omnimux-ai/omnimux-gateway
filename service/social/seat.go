package social

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service/social/zernio"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"gorm.io/gorm"
)

var (
	ErrSeatQuota      = model.ErrSocialSeatQuota
	ErrSeatCap        = model.ErrSocialSeatCap
	ErrSeatExpired    = model.ErrSocialSeatExpired
	ErrSeatRenewLimit = model.ErrSocialSeatRenewLimit
	ErrSeatSwapLimit  = model.ErrSocialSeatSwapLimit
)

const beijingTZName = "Asia/Shanghai"

func beijingLocation() *time.Location {
	loc, err := time.LoadLocation(beijingTZName)
	if err != nil {
		return time.FixedZone("CST", 8*3600)
	}
	return loc
}

func FormatBeijingExpiry(t time.Time) string {
	return t.In(beijingLocation()).Format("2006-01-02 15:04")
}

func nowUTC() time.Time {
	return time.Now().UTC()
}

func seatPriceRaw() (int, error) {
	return model.SocialUSDToQuota(operation_setting.SocialSeatPriceUSD())
}

func seatBillingOn() bool {
	return operation_setting.SocialSeatBillingEnabled()
}

type VacantSeatView struct {
	ID              int64  `json:"id"`
	ExpiresAt       string `json:"expires_at"`
	SwapsRemaining  int    `json:"swaps_remaining"`
	SameAccountOnly bool   `json:"same_account_only"`
}

type AccountSeatView struct {
	ID                 int64  `json:"id"`
	Status             string `json:"status"`
	ExpiresAt          string `json:"expires_at"`
	GraceUntil         string `json:"grace_until"`
	SwapCount          int    `json:"swap_count"`
	SwapsRemaining     int    `json:"swaps_remaining"`
	CanRenew           bool   `json:"can_renew"`
	RenewBlockedReason string `json:"renew_blocked_reason"`
	PriceCredits       int    `json:"price_credits"`
}

type SeatView struct {
	BillingMode       string           `json:"billing_mode"`
	SeatsPurchased    int              `json:"seats_purchased"`
	SeatsOccupied     int              `json:"seats_occupied"`
	SeatsVacant       int              `json:"seats_vacant"`
	SeatsCap          int              `json:"seats_cap"`
	NextExpiry        *string          `json:"next_expiry"`
	SeatPriceUSD      float64          `json:"seat_price_usd"`
	SeatPriceCredits  int              `json:"seat_price_credits"`
	WalletOK          bool             `json:"wallet_ok"`
	GrandfatherUntil  *string          `json:"grandfather_until"`
	MaxValidityDays   int              `json:"max_validity_days"`
	MaxSwapsPerWindow int              `json:"max_swaps_per_window"`
	GraceDays         int              `json:"grace_days"`
	VacantSeats       []VacantSeatView `json:"vacant_seats"`
}

func publicSeatCatalog() map[string]any {
	return map[string]any{
		"billing_mode":       operation_setting.EffectiveSocialSeatBillingMode(),
		"seat_price_usd":     operation_setting.SocialSeatPriceUSD(),
		"seat_price_credits": operation_setting.SocialSeatPriceCredits(),
		"seats_cap":          operation_setting.SocialSeatsCap(),
	}
}

func NeedCreditsForPrice() int {
	return operation_setting.SocialSeatPriceCredits()
}

// PreflightConnect blocks GetConnectURL when seat mode has no vacant pit and
// the user is over cap or cannot afford a new window.
func (s *Service) PreflightConnect(userId int) error {
	if !seatBillingOn() {
		return nil
	}
	if err := s.reconcileExpiredSeats(context.Background(), userId); err != nil {
		return err
	}
	now := nowUTC()
	vacant, err := model.CountVacantUnexpired(userId, now)
	if err != nil {
		return err
	}
	if vacant > 0 {
		return nil
	}
	purchased, err := model.CountPurchasedSeats(userId, now)
	if err != nil {
		return err
	}
	if purchased >= operation_setting.SocialSeatsCap() {
		return ErrSeatCap
	}
	price, err := seatPriceRaw()
	if err != nil {
		return err
	}
	quota, err := model.GetUserQuota(userId, true)
	if err != nil {
		return err
	}
	if quota < price {
		return ErrSeatQuota
	}
	return nil
}

func (s *Service) SeatsView(ctx context.Context, userId int) (*SeatView, error) {
	// fork: zernio-retire — seats are local billing state, not an upstream call.
	if !s.cfg.Enabled {
		return nil, ErrDisabled
	}
	if seatBillingOn() {
		_ = s.reconcileExpiredSeats(ctx, userId)
	}
	now := nowUTC()
	counts, err := model.CountUserSeatState(userId, now)
	if err != nil {
		return nil, err
	}
	maxSwaps := operation_setting.SocialSeatMaxSwapsPerWindow()
	vacantRows, err := model.ListVacantUnexpiredSeats(userId, now)
	if err != nil {
		return nil, err
	}
	vacantViews := make([]VacantSeatView, 0, len(vacantRows))
	hasSwappableVacant := false
	for _, row := range vacantRows {
		rem := max(0, maxSwaps-row.SwapCount)
		sameOnly := row.LastAccountID != nil && rem == 0
		if !sameOnly {
			hasSwappableVacant = true
		}
		vacantViews = append(vacantViews, VacantSeatView{
			ID:              row.ID,
			ExpiresAt:       row.ExpiresAt.UTC().Format(time.RFC3339),
			SwapsRemaining:  rem,
			SameAccountOnly: sameOnly,
		})
	}
	price, err := seatPriceRaw()
	if err != nil {
		return nil, err
	}
	walletOK := true
	if seatBillingOn() && !hasSwappableVacant {
		q, qerr := model.GetUserQuota(userId, true)
		if qerr != nil {
			return nil, qerr
		}
		walletOK = q >= price
	}
	view := &SeatView{
		BillingMode:       operation_setting.EffectiveSocialSeatBillingMode(),
		SeatsPurchased:    counts.Purchased,
		SeatsOccupied:     counts.Occupied,
		SeatsVacant:       counts.Vacant,
		SeatsCap:          operation_setting.SocialSeatsCap(),
		SeatPriceUSD:      operation_setting.SocialSeatPriceUSD(),
		SeatPriceCredits:  operation_setting.SocialSeatPriceCredits(),
		WalletOK:          walletOK,
		MaxValidityDays:   operation_setting.SocialSeatMaxValidityDays(),
		MaxSwapsPerWindow: maxSwaps,
		GraceDays:         operation_setting.SocialSeatGraceDays(),
		VacantSeats:       vacantViews,
	}
	if counts.NextExpiry != nil {
		s := FormatBeijingExpiry(*counts.NextExpiry)
		view.NextExpiry = &s
	}
	if counts.GrandfatherUntil != nil {
		s := FormatBeijingExpiry(*counts.GrandfatherUntil)
		view.GrandfatherUntil = &s
	}
	return view, nil
}

func buildAccountSeatView(seat *model.SocialSeat, now time.Time, graceDays, maxValidityDays, maxSwaps, priceCredits int) *AccountSeatView {
	if seat == nil {
		graceUntil := now.Add(time.Duration(graceDays) * 24 * time.Hour)
		return &AccountSeatView{
			ID:                 0,
			Status:             "unactivated",
			ExpiresAt:          now.UTC().Format(time.RFC3339),
			GraceUntil:         graceUntil.UTC().Format(time.RFC3339),
			SwapCount:          0,
			SwapsRemaining:     maxSwaps,
			CanRenew:           true,
			RenewBlockedReason: "",
			PriceCredits:       priceCredits,
		}
	}
	graceUntil := seat.ExpiresAt.Add(time.Duration(graceDays) * 24 * time.Hour)
	active := seat.ExpiresAt.After(now) || (seat.GrandfatherUntil != nil && seat.GrandfatherUntil.After(now))
	status := "frozen"
	switch {
	case active:
		status = "active"
	case seat.OpenedQuota == 0 && seat.GrandfatherUntil == nil:
		status = "unactivated"
	}
	swapsRemaining := max(0, maxSwaps-seat.SwapCount)
	canRenew := true
	renewBlockedReason := ""
	if now.After(graceUntil) {
		canRenew = false
		renewBlockedReason = "SOCIAL_SEAT_EXPIRED"
	} else {
		var next time.Time
		if seat.ExpiresAt.After(now) {
			next = seat.ExpiresAt.Add(model.SocialSeatWindow())
		} else {
			next = now.Add(model.SocialSeatWindow())
		}
		if next.After(now.Add(time.Duration(maxValidityDays) * 24 * time.Hour)) {
			canRenew = false
			renewBlockedReason = "SOCIAL_SEAT_RENEW_LIMIT"
		}
	}
	return &AccountSeatView{
		ID:                 seat.ID,
		Status:             status,
		ExpiresAt:          seat.ExpiresAt.UTC().Format(time.RFC3339),
		GraceUntil:         graceUntil.UTC().Format(time.RFC3339),
		SwapCount:          seat.SwapCount,
		SwapsRemaining:     swapsRemaining,
		CanRenew:           canRenew,
		RenewBlockedReason: renewBlockedReason,
		PriceCredits:       priceCredits,
	}
}

// BuildAccountSeatViews returns per-account seat status views when seat billing is enabled.
func (s *Service) BuildAccountSeatViews(userId int, accounts []*model.SocialAccount) (map[int64]*AccountSeatView, error) {
	if !seatBillingOn() || len(accounts) == 0 {
		return nil, nil
	}
	ids := make([]int64, 0, len(accounts))
	for _, a := range accounts {
		ids = append(ids, a.ID)
	}
	seatMap, err := model.ListOccupiedSeatsByAccountIDs(userId, ids)
	if err != nil {
		return nil, err
	}
	now := nowUTC()
	graceDays := operation_setting.SocialSeatGraceDays()
	maxValidityDays := operation_setting.SocialSeatMaxValidityDays()
	maxSwaps := operation_setting.SocialSeatMaxSwapsPerWindow()
	priceCredits := operation_setting.SocialSeatPriceCredits()
	out := make(map[int64]*AccountSeatView, len(accounts))
	for _, a := range accounts {
		out[a.ID] = buildAccountSeatView(seatMap[a.ID], now, graceDays, maxValidityDays, maxSwaps, priceCredits)
	}
	return out, nil
}

// RenewAccountSeat manually renews an occupied seat or activates an unactivated account.
func (s *Service) RenewAccountSeat(ctx context.Context, userId int, provider string, accountID int64) (*AccountSeatView, error) {
	if err := validateProvider(provider, s.cfg.ZernioActive()); err != nil {
		return nil, err
	}
	if !s.cfg.Enabled {
		return nil, ErrDisabled
	}
	if !seatBillingOn() {
		return nil, errors.New("social seat billing is not enabled")
	}
	if err := s.reconcileExpiredSeats(ctx, userId); err != nil {
		return nil, err
	}
	acc, err := model.GetSocialAccountByIDForUser(accountID, userId)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAccountMissing
		}
		return nil, err
	}
	if err := validateAccountProvider(provider, []*model.SocialAccount{acc}); err != nil {
		return nil, err
	}
	if acc.Status != model.SocialAccountStatusActive {
		return nil, ErrSeatExpired
	}
	price, err := seatPriceRaw()
	if err != nil {
		return nil, err
	}
	now := nowUTC()
	graceDays := operation_setting.SocialSeatGraceDays()
	maxValidityDays := operation_setting.SocialSeatMaxValidityDays()
	maxSwaps := operation_setting.SocialSeatMaxSwapsPerWindow()
	seatsCap := operation_setting.SocialSeatsCap()

	var res *model.RenewSocialSeatResult
	txErr := model.DB.Transaction(func(tx *gorm.DB) error {
		var e error
		res, e = model.RenewOrActivateSeatTx(tx, userId, acc.ID, price, graceDays, maxValidityDays, maxSwaps, seatsCap, now)
		return e
	})
	if txErr != nil {
		return nil, txErr
	}
	if res.ChargedQuota > 0 {
		model.SyncSocialSeatDebitCache(userId, res.ChargedQuota, res.Action)
	}
	model.RecordSocialSeatLog(userId, res.Action, res.Seat.ID, res.ChargedQuota, fmt.Sprintf("account_id=%d", acc.ID))
	return buildAccountSeatView(res.Seat, now, graceDays, maxValidityDays, maxSwaps, operation_setting.SocialSeatPriceCredits()), nil
}

// reconcileExpiredSeats performs pure cleanup and materialization without silent auto-charging:
// 1. Expires past-due vacant pits.
// 2. Materializes seats for active accounts without one (grandfather or immediate frozen unactivated).
// 3. Disconnects and expires occupied seats whose grace window has elapsed.
func (s *Service) reconcileExpiredSeats(ctx context.Context, userId int) error {
	if !seatBillingOn() {
		return nil
	}
	now := nowUTC()
	vacants, err := model.ListVacantExpiredSeats(userId, now)
	if err != nil {
		return err
	}
	for _, seat := range vacants {
		if err := model.MarkSocialSeatExpired(seat, "vacant_expired"); err != nil {
			return err
		}
	}

	unseated, err := model.ListActiveAccountsWithoutSeat(userId)
	if err != nil {
		return err
	}
	gfDays := operation_setting.SocialSeatGrandfatherDays()
	for _, acc := range unseated {
		var created *model.SocialSeat
		var extra string
		err := model.DB.Transaction(func(tx *gorm.DB) error {
			if _, existingErr := model.GetOccupiedSeatForAccountTx(tx, userId, acc.ID); existingErr == nil {
				return nil
			} else if !errors.Is(existingErr, gorm.ErrRecordNotFound) {
				return existingErr
			}
			var seat *model.SocialSeat
			var e error
			if gfDays > 0 {
				until := now.Add(time.Duration(gfDays) * 24 * time.Hour)
				seat, e = model.OpenOccupiedSeatTx(tx, userId, acc.ID, 0, now, &until)
				extra = fmt.Sprintf("grandfather account_id=%d", acc.ID)
			} else {
				seat, e = model.OpenFrozenUnactivatedSeatTx(tx, userId, acc.ID, now)
				extra = fmt.Sprintf("unactivated account_id=%d", acc.ID)
			}
			if e != nil {
				return e
			}
			created = seat
			sid := seat.ID
			return tx.Model(acc).Updates(map[string]any{
				"seat_id":    sid,
				"updated_at": now,
			}).Error
		})
		if err != nil {
			return err
		}
		if created != nil {
			model.RecordSocialSeatLog(userId, model.SocialSeatActionOpen, created.ID, 0, extra)
		}
	}

	occupied, err := model.ListOccupiedExpiredSeats(userId, now)
	if err != nil {
		return err
	}
	graceDays := operation_setting.SocialSeatGraceDays()
	for _, seat := range occupied {
		if seat.GrandfatherUntil != nil && seat.GrandfatherUntil.After(now) {
			continue
		}
		graceEnd := seat.ExpiresAt.Add(time.Duration(graceDays) * 24 * time.Hour)
		if !now.After(graceEnd) {
			continue
		}
		if seat.OccupiedAccountID != nil {
			acc, aerr := model.GetSocialAccountByIDForUser(*seat.OccupiedAccountID, userId)
			if aerr == nil {
				if acc.Provider == ProviderTikTokDirect {
					s.revokeTikTokAccountToken(ctx, acc)
				} else if s.cfg.ZernioActive() && s.client != nil && acc.ProviderAccountID != "" {
					_ = s.client.DisconnectAccount(ctx, acc.ProviderAccountID)
				}
				_ = model.MarkSocialAccountDisconnected(acc.ID, userId)
			}
		}
		if e := model.MarkSocialSeatExpired(seat, "grace_elapsed"); e != nil {
			return e
		}
	}
	return nil
}

type ingestDraft struct {
	UserId            int
	Provider          string
	ProviderAccountID string
	ProviderProfileID string
	Platform          string
	Username          string
	DisplayName       string
}

func (s *Service) ingestUpstreamAccount(ctx context.Context, userId int, tenant *model.SocialTenant, upstream zernio.Account) (*model.SocialAccount, error) {
	if upstream.ID == "" {
		return nil, nil
	}
	profileID := upstream.ProfileID.String()
	if profileID != "" && tenant != nil && profileID != tenant.ProviderProfileID {
		return nil, nil
	}
	username := upstream.Username
	if username == "" {
		username = upstream.Name
	}
	display := upstream.DisplayName
	if display == "" {
		display = upstream.Name
	}
	if display == "" {
		display = username
	}
	draft := ingestDraft{
		UserId:            userId,
		Provider:          ProviderZernio,
		ProviderAccountID: upstream.ID,
		ProviderProfileID: "",
		Platform:          NormalizePlatform(upstream.Platform),
		Username:          username,
		DisplayName:       display,
	}
	if tenant != nil {
		draft.ProviderProfileID = tenant.ProviderProfileID
	}
	if !seatBillingOn() {
		acc := &model.SocialAccount{
			UserId:            draft.UserId,
			Provider:          draft.Provider,
			ProviderAccountID: draft.ProviderAccountID,
			ProviderProfileID: draft.ProviderProfileID,
			Platform:          draft.Platform,
			Username:          draft.Username,
			DisplayName:       draft.DisplayName,
			Status:            model.SocialAccountStatusActive,
		}
		if err := model.UpsertSocialAccount(acc); err != nil {
			return nil, err
		}
		return model.GetSocialAccountByProviderRef(draft.Provider, draft.ProviderAccountID)
	}

	if err := s.reconcileExpiredSeats(ctx, userId); err != nil {
		return nil, err
	}

	existing, err := model.GetSocialAccountByProviderRef(draft.Provider, draft.ProviderAccountID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if err == nil && existing.UserId != userId {
		return nil, errors.New("social account already bound to another user")
	}

	wasLocallyActive := err == nil && existing.Status == model.SocialAccountStatusActive
	compensateDisconnect := !wasLocallyActive

	acc, gateErr := s.applySeatGate(ctx, userId, draft, existing)
	if gateErr != nil {
		if compensateDisconnect && s.cfg.ZernioActive() && s.client != nil {
			_ = s.client.DisconnectAccount(ctx, draft.ProviderAccountID)
		}
		return nil, gateErr
	}
	return acc, nil
}

func loadSocialAccountByProviderRefTx(tx *gorm.DB, provider, providerAccountID string) (*model.SocialAccount, error) {
	var got model.SocialAccount
	if err := tx.Where("provider = ? AND provider_account_id = ?", provider, providerAccountID).First(&got).Error; err != nil {
		return nil, err
	}
	return &got, nil
}

func (s *Service) applySeatGate(ctx context.Context, userId int, draft ingestDraft, existing *model.SocialAccount) (*model.SocialAccount, error) {
	_ = ctx
	now := nowUTC()
	price, err := seatPriceRaw()
	if err != nil {
		return nil, err
	}
	maxSwaps := operation_setting.SocialSeatMaxSwapsPerWindow()
	seatsCap := operation_setting.SocialSeatsCap()

	type pendingLog struct {
		action string
		seatID int64
		quota  int
		extra  string
	}
	var result *model.SocialAccount
	var logAfter *pendingLog
	chargedQuota := 0
	txErr := model.DB.Transaction(func(tx *gorm.DB) error {
		if err := model.LockSocialSeatUserTx(tx, userId); err != nil {
			return err
		}

		var staleFrozenSeat *model.SocialSeat
		if existing != nil && existing.UserId == userId {
			seat, serr := model.GetOccupiedSeatForAccountTx(tx, userId, existing.ID)
			if serr == nil {
				if seat.ExpiresAt.After(now) || (seat.GrandfatherUntil != nil && seat.GrandfatherUntil.After(now)) {
					existing.ProviderProfileID = draft.ProviderProfileID
					existing.Platform = draft.Platform
					existing.Username = draft.Username
					existing.DisplayName = draft.DisplayName
					existing.Status = model.SocialAccountStatusActive
					sid := seat.ID
					existing.SeatID = &sid
					if e := model.UpsertSocialAccountTx(tx, existing); e != nil {
						return e
					}
					result = existing
					return nil
				}
				// Re-authorizing an account whose seat is expired/frozen must not bypass the seat gate.
				staleFrozenSeat = seat
			} else if !errors.Is(serr, gorm.ErrRecordNotFound) {
				return serr
			}
		}

		acc := &model.SocialAccount{
			UserId:            draft.UserId,
			Provider:          draft.Provider,
			ProviderAccountID: draft.ProviderAccountID,
			ProviderProfileID: draft.ProviderProfileID,
			Platform:          draft.Platform,
			Username:          draft.Username,
			DisplayName:       draft.DisplayName,
			Status:            model.SocialAccountStatusActive,
		}
		if e := model.UpsertSocialAccountTx(tx, acc); e != nil {
			return e
		}
		if acc.ID == 0 {
			got, ge := loadSocialAccountByProviderRefTx(tx, draft.Provider, draft.ProviderAccountID)
			if ge != nil {
				return ge
			}
			acc = got
		}

		seat, occErr := model.OccupyVacantForAccountTx(tx, userId, acc.ID, maxSwaps, now)
		if occErr == nil {
			if staleFrozenSeat != nil {
				if e := model.ExpireSocialSeatTx(tx, staleFrozenSeat.ID, userId, now); e != nil {
					return e
				}
			}
			sid := seat.ID
			acc.SeatID = &sid
			if e := tx.Model(acc).Updates(map[string]any{"seat_id": sid, "updated_at": now}).Error; e != nil {
				return e
			}
			logAfter = &pendingLog{
				action: model.SocialSeatActionOccupy,
				seatID: seat.ID,
				quota:  0,
				extra:  fmt.Sprintf("account_id=%d", acc.ID),
			}
			result = acc
			return nil
		}
		if !errors.Is(occErr, gorm.ErrRecordNotFound) {
			return occErr
		}

		purchased, e := model.CountPurchasedSeatsTx(tx, userId, now)
		if e != nil {
			return e
		}
		if purchased >= seatsCap {
			vacantCount, ve := model.CountVacantUnexpiredTx(tx, userId, now)
			if ve == nil && vacantCount > 0 {
				return ErrSeatSwapLimit
			}
			return ErrSeatCap
		}
		if e := model.ChargeSocialSeatTx(tx, userId, price); e != nil {
			return e
		}
		if staleFrozenSeat != nil {
			if e := model.ExpireSocialSeatTx(tx, staleFrozenSeat.ID, userId, now); e != nil {
				return e
			}
		}
		seat, e = model.OpenOccupiedSeatTx(tx, userId, acc.ID, price, now, nil)
		if e != nil {
			return e
		}
		sid := seat.ID
		if e := tx.Model(acc).Updates(map[string]any{"seat_id": sid, "updated_at": now}).Error; e != nil {
			return e
		}
		acc.SeatID = &sid
		chargedQuota = price
		logAfter = &pendingLog{
			action: model.SocialSeatActionOpen,
			seatID: seat.ID,
			quota:  price,
			extra:  fmt.Sprintf("account_id=%d", acc.ID),
		}
		result = acc
		return nil
	})
	if txErr != nil {
		return nil, txErr
	}
	if chargedQuota > 0 {
		model.SyncSocialSeatDebitCache(userId, chargedQuota, model.SocialSeatActionOpen)
	}
	if logAfter != nil {
		model.RecordSocialSeatLog(userId, logAfter.action, logAfter.seatID, logAfter.quota, logAfter.extra)
	}
	return result, nil
}

func (s *Service) vacateAfterDisconnect(userId int, accountID int64) error {
	if !seatBillingOn() {
		return nil
	}
	_, err := model.VacateOccupiedSeatForAccount(userId, accountID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return nil
}

func (s *Service) guardSeatsForAccounts(userId int, accounts []*model.SocialAccount) error {
	if !seatBillingOn() {
		return nil
	}
	_ = s.reconcileExpiredSeats(context.Background(), userId)
	now := nowUTC()
	grace := operation_setting.SocialSeatGraceDays()
	for _, a := range accounts {
		if err := model.AccountSeatUsable(userId, a.ID, now, grace); err != nil {
			return ErrSeatExpired
		}
	}
	return nil
}

// PurgeUserSeats disconnects occupied upstream accounts and expires every seat.
// Wired from controller.DeleteUser / DeleteSelf as best-effort offboard:
// failures are logged and do not block OmniMux user delete. Not called from
// model.(*User).HardDelete / Delete (model must not depend on service).
func (s *Service) PurgeUserSeats(ctx context.Context, userId int) error {
	if userId <= 0 {
		return errors.New("invalid user")
	}
	occupied, err := model.ListOccupiedSeatsByUser(userId)
	if err != nil {
		return err
	}
	for _, seat := range occupied {
		if seat.OccupiedAccountID == nil {
			continue
		}
		acc, aerr := model.GetSocialAccountByIDForUser(*seat.OccupiedAccountID, userId)
		if aerr != nil {
			continue
		}
		if acc.Provider == ProviderTikTokDirect {
			s.revokeTikTokAccountToken(ctx, acc)
		} else if s.client != nil && acc.ProviderAccountID != "" {
			// fork: zernio-retire — explicit user offboarding must still attempt the
			// disconnect against a configured Zernio client. ZernioEnabled only gates
			// *new* publishing, not cleanup: skipping the call here would expire the
			// seat without ever proving the upstream account is gone, and would turn a
			// failed purge into a silent data loss instead of the fail-open "leave the
			// seat occupied" behaviour DeleteUser relies on.
			derr := s.client.DisconnectAccount(ctx, acc.ProviderAccountID)
			if derr != nil {
				var apiErr *zernio.APIError
				if !(errors.As(derr, &apiErr) && (apiErr.Status == 404 || apiErr.Status == 410)) {
					return derr
				}
			}
		}
		_ = model.MarkSocialAccountDisconnected(acc.ID, userId)
	}
	return model.ExpireAllUserSeats(userId)
}
