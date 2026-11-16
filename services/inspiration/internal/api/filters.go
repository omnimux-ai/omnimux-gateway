package api

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"omnimux-inspiration/internal/errs"
	"omnimux-inspiration/internal/service"
)

func bindFilters(c *gin.Context, q *service.ListQuery) error {
	q.Country = c.Query("country")
	q.Category = c.Query("category")
	q.TrafficType = c.Query("traffic_type")
	values := c.Request.URL.Query()
	for _, param := range []struct {
		name string
		dest **int
	}{
		{"duration_min", &q.DurationMin}, {"duration_max", &q.DurationMax},
	} {
		if values.Has(param.name) {
			value, err := strconv.Atoi(values.Get(param.name))
			if err != nil {
				return errs.ErrInvalidParam
			}
			*param.dest = &value
		}
	}
	for _, param := range []struct {
		name string
		dest **int64
	}{
		{"views_min", &q.ViewsMin}, {"views_max", &q.ViewsMax},
	} {
		if values.Has(param.name) {
			value, err := strconv.ParseInt(values.Get(param.name), 10, 64)
			if err != nil {
				return errs.ErrInvalidParam
			}
			*param.dest = &value
		}
	}
	for _, param := range []struct {
		name  string
		dest  **time.Time
		upper bool
	}{
		{"posted_after", &q.PostedAfter, false}, {"posted_before", &q.PostedBefore, true},
	} {
		if values.Has(param.name) {
			value, err := service.ParsePostedBound(values.Get(param.name), param.upper)
			if err != nil {
				return err
			}
			*param.dest = value
		}
	}
	return nil
}
