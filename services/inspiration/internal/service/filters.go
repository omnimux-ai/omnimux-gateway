package service

import (
	"bytes"
	"encoding/json"
	"math"
	"time"
	"unicode/utf8"

	"omnimux-inspiration/internal/errs"
	"omnimux-inspiration/internal/model"
)

// OptionalTime distinguishes an omitted PATCH field from an explicit JSON null.
type OptionalTime struct {
	Set   bool
	Value *time.Time
}

func (v *OptionalTime) UnmarshalJSON(data []byte) error {
	v.Set = true
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		v.Value = nil
		return nil
	}
	var parsed time.Time
	if err := json.Unmarshal(data, &parsed); err != nil {
		return err
	}
	v.Value = &parsed
	return nil
}

func validFilterFields(country, category, traffic string, duration int, views int64, posted *time.Time) bool {
	return utf8.RuneCountInString(country) <= 8 && utf8.RuneCountInString(category) <= 64 &&
		(traffic == "" || traffic == "ad" || traffic == "organic") &&
		duration >= 0 && uint64(duration) <= math.MaxUint32 && views >= 0 && validPostedAt(posted)
}

func validPostedAt(value *time.Time) bool {
	return value == nil || (value.UTC().Year() >= 1000 && value.UTC().Year() <= 9999)
}

func normalizedPostedAt(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	utc := value.UTC().Truncate(time.Millisecond)
	return &utc
}

func (req *UpdateInspirationReq) filterUpdates() (map[string]any, error) {
	updates := map[string]any{}
	country, category, traffic := "", "", ""
	duration, views := 0, int64(0)
	if req.CountryCode != nil {
		country = *req.CountryCode
		updates["country_code"] = country
	}
	if req.Category != nil {
		category = *req.Category
		updates["category"] = category
	}
	if req.Duration != nil {
		duration = *req.Duration
		updates["duration"] = duration
	}
	if req.Views != nil {
		views = *req.Views
		updates["views"] = views
	}
	if req.TrafficType != nil {
		traffic = *req.TrafficType
		if traffic == "" {
			return nil, errs.ErrInvalidParam
		}
		updates["traffic_type"] = traffic
	}
	if !validFilterFields(country, category, traffic, duration, views, req.PostedAt.Value) {
		return nil, errs.ErrInvalidParam
	}
	if req.PostedAt.Set {
		updates["posted_at"] = model.PostedAtValue(normalizedPostedAt(req.PostedAt.Value))
	}
	return updates, nil
}

func (q *ListQuery) validateFilters() error {
	if !validFilterFields(q.Country, q.Category, q.TrafficType, 0, 0, q.PostedAfter) || !validPostedAt(q.PostedBefore) {
		return errs.ErrInvalidParam
	}
	for _, v := range []*int{q.DurationMin, q.DurationMax} {
		if v != nil && (*v < 0 || uint64(*v) > math.MaxUint32) {
			return errs.ErrInvalidParam
		}
	}
	for _, v := range []*int64{q.ViewsMin, q.ViewsMax} {
		if v != nil && *v < 0 {
			return errs.ErrInvalidParam
		}
	}
	if q.DurationMin != nil && q.DurationMax != nil && *q.DurationMin > *q.DurationMax {
		return errs.ErrInvalidParam
	}
	if q.ViewsMin != nil && q.ViewsMax != nil && *q.ViewsMin > *q.ViewsMax {
		return errs.ErrInvalidParam
	}
	if q.PostedAfter != nil && q.PostedBefore != nil && q.PostedAfter.After(*q.PostedBefore) {
		return errs.ErrInvalidParam
	}
	return nil
}

// ParsePostedBound accepts an instant or a UTC calendar date. A date upper bound
// includes its final millisecond, matching the stored DATETIME(3) precision.
func ParsePostedBound(raw string, upper bool) (*time.Time, error) {
	value, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		value, err = time.Parse(time.DateOnly, raw)
		if err != nil {
			return nil, errs.ErrInvalidParam
		}
		if upper {
			value = value.AddDate(0, 0, 1).Add(-time.Millisecond)
		}
	}
	if !validPostedAt(&value) {
		return nil, errs.ErrInvalidParam
	}
	value = value.UTC()
	return &value, nil
}
