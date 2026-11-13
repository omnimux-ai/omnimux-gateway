package service

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInfluencerVideoInspectionStatus(t *testing.T) {
	for _, item := range []struct {
		err    error
		status int
	}{
		{context.Canceled, http.StatusRequestTimeout},
		{context.DeadlineExceeded, http.StatusRequestTimeout},
		{ErrInfluencerVideoBusy, http.StatusServiceUnavailable},
		{ErrInfluencerVideoUnavailable, http.StatusServiceUnavailable},
		{ErrInfluencerVideoOutputLimit, http.StatusBadRequest},
		{errors.New("invalid media"), http.StatusBadRequest},
	} {
		assert.Equal(t, item.status, InfluencerVideoInspectionStatus(item.err))
	}
}
