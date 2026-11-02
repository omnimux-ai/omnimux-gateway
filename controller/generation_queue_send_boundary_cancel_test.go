package controller

import (
	"context"
	"io"
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Only the response reader is wrapped: RoundTrip still performs a real HTTP POST.
// Cancelling on the first body read guarantees native status classification has run.
type generationCancelBody struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (b *generationCancelBody) Read(p []byte) (int, error) { b.cancel(); return b.ReadCloser.Read(p) }

type generationCancelTransport struct {
	native http.RoundTripper
	cancel context.CancelFunc
}

func (r generationCancelTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := r.native.RoundTrip(req)
	if resp != nil {
		resp.Body = &generationCancelBody{ReadCloser: resp.Body, cancel: r.cancel}
	}
	return resp, err
}

func TestGenerationQueueHTTP400CancellationRetainsNativeRejectionEvidence(t *testing.T) {
	var calls atomic.Int32
	_, user, scanner := generationCorrectionFixture(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"rejected"}`))
	}))
	service.InitHttpClient()
	client := service.GetHttpClient()
	native := client.Transport
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	client.Transport = generationCancelTransport{native: native, cancel: cancel}
	defer func() { client.Transport = native }()
	task := insertCorrectionTask(t, user, 1, false)
	claimed, err := model.ClaimGenerationTasks(context.Background(), user.Id, scanner.TaskID)
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	dispatchInfluencerGeneration(ctx, claimed[0], scanner, "correction")
	require.ErrorIs(t, ctx.Err(), context.Canceled)
	stored, found, err := model.GetByTaskId(user.Id, task.TaskID)
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, model.TaskStatus(model.TaskStatusFailure), stored.Status)
	assert.Zero(t, stored.Quota)
	account, err := model.GetUserById(user.Id, false)
	require.NoError(t, err)
	assert.Equal(t, 100000, account.Quota)
	assert.Zero(t, account.UsedQuota)
	assert.Equal(t, int32(1), calls.Load())
}
