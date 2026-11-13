package social

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/service/social/zernio"
	"gorm.io/gorm"
)

var (
	ErrSeatQuota   = model.ErrSocialSeatQuota
	ErrSeatCap     = model.ErrSocialSeatCap
	ErrSeatExpired = model.ErrSocialSeatExpired
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

type SeatView struct {
	BillingMode      string  `json:"billing_mode"`
	SeatsPurchased   int     `json:"seats_purchased"`
	SeatsOccupied    int     `json:"seats_occupied"`
	SeatsVacant      int     `json:"seats_vacant"`
	SeatsCap         int     `json:"seats_cap"`
	NextExpiry       *string `json:"next_expiry"`
	SeatPriceUSD     float64 `json:"seat_price_usd"`
	SeatPriceCredits int     `json:"seat_price_credits"`
	WalletOK         bool    `json:"wallet_ok"`
	GrandfatherUntil *string `json:"grandfather_until"`
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
	if err := s.lazyExpireAndRenew(context.Background(), userId); err != nil {
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
	if err := s.requireReady(); err != nil {
		return nil, err
	}
	if seatBillingOn() {
		_ = s.lazyExpireAndRenew(ctx, userId)
		_ = s.grandfatherActiveAccounts(userId)
	}
	now := nowUTC()
	counts, err := model.CountUserSeatState(userId, now)
	if err != nil {
		return nil, err
	}
	price, err := seatPriceRaw()
	if err != nil {
		return nil, err
	}
	walletOK := true
	if seatBillingOn() && counts.Vacant == 0 {
		q, qerr := model.GetUserQuota(userId, true)
		if qerr != nil {
			return nil, qerr
		}
		walletOK = q >= price
	}
	view := &SeatView{
		BillingMode:      operation_setting.EffectiveSocialSeatBillingMode(),
		SeatsPurchased:   counts.Purchased,
		SeatsOccupied:    counts.Occupied,
		SeatsVacant:      counts.Vacant,
		SeatsCap:         operation_setting.SocialSeatsCap(),
		SeatPriceUSD:     operation_setting.SocialSeatPriceUSD(),
		SeatPriceCredits: operation_setting.SocialSeatPriceCredits(),
		WalletOK:         walletOK,
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

func (s *Service) grandfatherActiveAccounts(userId int) error {
	if !seatBillingOn() {
		return nil
	}
	rows, err := model.ListActiveAccountsWithoutSeat(userId)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return nil
	}
	now := nowUTC()
	until := now.Add(time.Duration(operation_setting.SocialSeatGrandfatherDays()) * 24 * time.Hour)
	for _, acc := range rows {
		var created *model.SocialSeat
		err := model.DB.Transaction(func(tx *gorm.DB) error {
			seat, e := model.OpenOccupiedSeatTx(tx, userId, acc.ID, 0, now, &until)
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
			model.RecordSocialSeatLog(userId, model.SocialSeatActionOpen, created.ID, 0,
				fmt.Sprintf("grandfather account_id=%d", acc.ID))
		}
	}
	return nil
}

func (s *Service) lazyExpireAndRenew(ctx context.Context, userId int) error {
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
	occupied, err := model.ListOccupiedExpiredSeats(userId, now)
	if err != nil {
		return err
	}
	price, err := seatPriceRaw()
	if err != nil {
		return err
	}
	graceDays := operation_setting.SocialSeatGraceDays()
	for _, seat := range occupied {
		if seat.GrandfatherUntil != nil && seat.GrandfatherUntil.After(now) {
			continue
		}
		charged := false
		err := model.DB.Transaction(func(tx *gorm.DB) error {
			if e := model.ChargeSocialSeatTx(tx, userId, price); e != nil {
				return e
			}
			charged = true
			return nil
		})
		if err == nil && charged {
			if e := model.RenewSocialSeat(seat, price, now); e != nil {
				return e
			}
			continue
		}
		if !errors.Is(err, model.ErrSocialSeatQuota) && err != nil {
			return err
		}
		graceEnd := seat.ExpiresAt.Add(time.Duration(graceDays) * 24 * time.Hour)
		if now.Before(graceEnd) || now.Equal(graceEnd) {
			continue
		}
		if seat.OccupiedAccountID != nil {
			acc, aerr := model.GetSocialAccountByIDForUser(*seat.OccupiedAccountID, userId)
			if aerr == nil && s.client != nil && acc.ProviderAccountID != "" {
				_ = s.client.DisconnectAccount(ctx, acc.ProviderAccountID)
			}
			if aerr == nil {
				_ = model.MarkSocialAccountDisconnected(acc.ID, userId)
			}
		}
		if e := model.MarkSocialSeatExpired(seat, "renew_failed_grace_elapsed"); e != nil {
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

	if err := s.lazyExpireAndRenew(ctx, userId); err != nil {
		return nil, err
	}
	if err := s.grandfatherActiveAccounts(userId); err != nil {
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
		if compensateDisconnect && s.client != nil {
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

	type pendingLog struct {
		action string
		seatID int64
		quota  int
		extra  string
	}
	var result *model.SocialAccount
	var logAfter *pendingLog
	txErr := model.DB.Transaction(func(tx *gorm.DB) error {
		if existing != nil && existing.UserId == userId {
			seat, serr := model.GetOccupiedSeatForAccountTx(tx, userId, existing.ID)
			if serr == nil && (seat.ExpiresAt.After(now) || (seat.GrandfatherUntil != nil && seat.GrandfatherUntil.After(now))) {
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
		}

		vacant, e := model.CountVacantUnexpiredTx(tx, userId, now)
		if e != nil {
			return e
		}
		if vacant > 0 {
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
			seat, e := model.OccupyEarliestVacantTx(tx, userId, acc.ID, now)
			if e != nil {
				return e
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

		purchased, e := model.CountPurchasedSeatsTx(tx, userId, now)
		if e != nil {
			return e
		}
		if purchased >= operation_setting.SocialSeatsCap() {
			return ErrSeatCap
		}
		if e := model.ChargeSocialSeatTx(tx, userId, price); e != nil {
			return e
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
		seat, e := model.OpenOccupiedSeatTx(tx, userId, acc.ID, price, now, nil)
		if e != nil {
			return e
		}
		sid := seat.ID
		if e := tx.Model(acc).Updates(map[string]any{"seat_id": sid, "updated_at": now}).Error; e != nil {
			return e
		}
		acc.SeatID = &sid
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
	_ = s.lazyExpireAndRenew(context.Background(), userId)
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
		if s.client != nil && acc.ProviderAccountID != "" {
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
