package hostedaccount

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service/hostedgateway"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func gwClient(t *testing.T, handler http.Handler) *hostedgateway.Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return hostedgateway.NewClient(hostedgateway.Config{BaseURL: srv.URL, APIKey: "k", TimeoutMS: 3000})
}

func envelopeOK(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "msg": "ok", "data": data})
}

func TestDispatchReservedPhoneZeroHTTP(t *testing.T) {
	clearHosted(t)
	u, _ := seedUserSKU(t, 1000)
	acc := &model.HostedAccount{
		PhoneID: ReservedMonthlyPhoneID, Platform: model.HostedPlatformTikTok, Region: model.HostedRegionUS,
		Status: model.HostedAccountWarmingUp, AssignedUserID: u.Id,
	}
	require.NoError(t, model.DB.Create(acc).Error)
	now := common.GetTimestamp()
	task := &model.HostedTask{
		AccountID: acc.ID, UserID: u.Id, Type: model.HostedTaskProfileEdit,
		Status: model.HostedTaskPending, ScheduledAt: now - 1, IdempotencyKey: "res-edit",
		CreatedAt: now, UpdatedAt: now,
	}
	require.NoError(t, model.DB.Create(task).Error)

	var hits atomic.Int32
	svc := NewService(gwClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	})))
	n, err := svc.DispatchDueTasks(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	assert.Equal(t, int32(0), hits.Load())

	var stored model.HostedTask
	require.NoError(t, model.DB.First(&stored, task.ID).Error)
	assert.Equal(t, model.HostedTaskFailed, stored.Status)
	assert.Equal(t, "HOSTED_RESERVED_PHONE", stored.ErrorCode)
}

func TestDispatchTypeMapping(t *testing.T) {
	clearHosted(t)
	u, _ := seedUserSKU(t, 1000)
	acc := &model.HostedAccount{
		PhoneID: "phone-map", Platform: model.HostedPlatformTikTok, Region: model.HostedRegionUS,
		Status: model.HostedAccountReady, AssignedUserID: u.Id,
		BrandConfig: `{"nick_name":"n","site":"https://example.com"}`,
	}
	require.NoError(t, model.DB.Create(acc).Error)
	now := common.GetTimestamp()
	edit := &model.HostedTask{
		AccountID: acc.ID, UserID: u.Id, Type: model.HostedTaskProfileEdit,
		Status: model.HostedTaskPending, ScheduledAt: now - 1, IdempotencyKey: "map-edit",
		CreatedAt: now, UpdatedAt: now,
	}
	distVideo := &model.HostedTask{
		AccountID: acc.ID, UserID: u.Id, Type: model.HostedTaskDistribute,
		Status: model.HostedTaskPending, ScheduledAt: now - 1, IdempotencyKey: "map-v",
		MediaURL: "https://cdn.example/v.mp4", CreatedAt: now, UpdatedAt: now,
	}
	distAlbum := &model.HostedTask{
		AccountID: acc.ID, UserID: u.Id, Type: model.HostedTaskDistribute,
		Status: model.HostedTaskPending, ScheduledAt: now - 1, IdempotencyKey: "map-a",
		CreatedAt: now, UpdatedAt: now,
	}
	warmup := &model.HostedTask{
		AccountID: acc.ID, UserID: 0, Type: model.HostedTaskInternalWarmup,
		Status: model.HostedTaskPending, ScheduledAt: now - 1, IdempotencyKey: "map-w",
		Payload: `{"action":"browse","duration_sec":50}`, CreatedAt: now, UpdatedAt: now,
	}
	require.NoError(t, model.DB.Create(edit).Error)
	require.NoError(t, model.DB.Create(distVideo).Error)
	require.NoError(t, model.DB.Create(distAlbum).Error)
	require.NoError(t, model.DB.Create(warmup).Error)

	var paths []string
	var types []float64
	svc := NewService(gwClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		_ = json.Unmarshal(raw, &body)
		if tt, ok := body["taskType"].(float64); ok {
			types = append(types, tt)
		}
		switch r.URL.Path {
		case "/api/v1/rpa/tiktok/edit":
			envelopeOK(w, map[string]any{"jobId": "job-edit"})
		case "/api/v1/automation/task/add":
			envelopeOK(w, map[string]any{"taskIds": []string{"tid-" + r.URL.Path}})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})))

	n, err := svc.DispatchDueTasks(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 4, n)
	assert.Contains(t, paths, "/api/v1/rpa/tiktok/edit")
	assert.Contains(t, paths, "/api/v1/automation/task/add")
	assert.Contains(t, types, float64(1))
	assert.Contains(t, types, float64(2))
	assert.Contains(t, types, float64(3))

	var storedEdit model.HostedTask
	require.NoError(t, model.DB.First(&storedEdit, edit.ID).Error)
	assert.Equal(t, model.HostedTaskQueued, storedEdit.Status)
	assert.Equal(t, "job-edit", storedEdit.GatewayJobID)
}

func TestDispatchFailureRefundsQuota(t *testing.T) {
	clearHosted(t)
	u, _ := seedUserSKU(t, 500)
	acc := &model.HostedAccount{
		PhoneID: "phone-fail", Platform: model.HostedPlatformTikTok, Region: model.HostedRegionUS,
		Status: model.HostedAccountReady, AssignedUserID: u.Id,
	}
	require.NoError(t, model.DB.Create(acc).Error)
	now := common.GetTimestamp()
	task := &model.HostedTask{
		AccountID: acc.ID, UserID: u.Id, Type: model.HostedTaskDistribute,
		Status: model.HostedTaskPending, ScheduledAt: now - 1, IdempotencyKey: "fail-d",
		MediaURL: "https://cdn.example/v.mp4", PreConsumedQuota: 40, CreatedAt: now, UpdatedAt: now,
	}
	require.NoError(t, model.DB.Create(task).Error)

	svc := NewService(gwClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"code":"GW_DOWN","msg":"upstream down"}`))
	})))
	n, err := svc.DispatchDueTasks(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, n)

	var stored model.HostedTask
	require.NoError(t, model.DB.First(&stored, task.ID).Error)
	assert.Equal(t, model.HostedTaskFailed, stored.Status)
	assert.Equal(t, "GW_DOWN", stored.ErrorCode)
	assert.Equal(t, int64(0), stored.PreConsumedQuota)

	var user model.User
	require.NoError(t, model.DB.First(&user, u.Id).Error)
	assert.Equal(t, 540, user.Quota)
}

func TestDispatchDoesNotResubmitQueued(t *testing.T) {
	clearHosted(t)
	u, _ := seedUserSKU(t, 1000)
	acc := &model.HostedAccount{
		PhoneID: "phone-cas", Platform: model.HostedPlatformTikTok, Region: model.HostedRegionUS,
		Status: model.HostedAccountReady, AssignedUserID: u.Id,
	}
	require.NoError(t, model.DB.Create(acc).Error)
	now := common.GetTimestamp()
	task := &model.HostedTask{
		AccountID: acc.ID, UserID: u.Id, Type: model.HostedTaskDistribute,
		Status: model.HostedTaskPending, ScheduledAt: now - 1, IdempotencyKey: "cas-1",
		MediaURL: "https://cdn.example/v.mp4", CreatedAt: now, UpdatedAt: now,
	}
	require.NoError(t, model.DB.Create(task).Error)

	var hits atomic.Int32
	svc := NewService(gwClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		envelopeOK(w, map[string]any{"taskIds": []string{"tid-cas"}})
	})))
	n, err := svc.DispatchDueTasks(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	n, err = svc.DispatchDueTasks(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, n)
	assert.Equal(t, int32(1), hits.Load())

	var stored model.HostedTask
	require.NoError(t, model.DB.First(&stored, task.ID).Error)
	assert.Equal(t, model.HostedTaskQueued, stored.Status)
	assert.Equal(t, "tid-cas", stored.GatewayJobID)
	assert.Equal(t, "tid-cas", stored.UpstreamTaskID)
}

func TestDispatchSkipsFutureScheduled(t *testing.T) {
	clearHosted(t)
	u, _ := seedUserSKU(t, 1000)
	acc := &model.HostedAccount{
		PhoneID: "phone-future", Platform: model.HostedPlatformTikTok, Region: model.HostedRegionUS,
		Status: model.HostedAccountReady, AssignedUserID: u.Id,
	}
	require.NoError(t, model.DB.Create(acc).Error)
	now := common.GetTimestamp()
	task := &model.HostedTask{
		AccountID: acc.ID, UserID: u.Id, Type: model.HostedTaskDistribute,
		Status: model.HostedTaskPending, ScheduledAt: now + 3600, IdempotencyKey: "future-1",
		MediaURL: "https://cdn.example/v.mp4", CreatedAt: now, UpdatedAt: now,
	}
	require.NoError(t, model.DB.Create(task).Error)
	var hits atomic.Int32
	svc := NewService(gwClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	})))
	n, err := svc.DispatchDueTasks(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, n)
	assert.Equal(t, int32(0), hits.Load())
	var stored model.HostedTask
	require.NoError(t, model.DB.First(&stored, task.ID).Error)
	assert.Equal(t, model.HostedTaskPending, stored.Status)
}

func TestDispatchProfileEditInjectsOpenedCredentials(t *testing.T) {
	clearHosted(t)
	t.Setenv("HOSTED_CRED_KEY", testCredKey())
	u, _ := seedUserSKU(t, 1000)
	enc, err := Seal(CredentialPlain{Username: "tik.user", Password: "vault-pw", TOTPSecret: "TOTPSECRET"})
	require.NoError(t, err)
	acc := &model.HostedAccount{
		PhoneID: "phone-vault", Platform: model.HostedPlatformTikTok, Region: model.HostedRegionUS,
		Status: model.HostedAccountWarmingUp, AssignedUserID: u.Id,
		BrandConfig: `{"nick_name":"n","site":"https://example.com"}`, CredentialsEnc: enc,
	}
	require.NoError(t, model.DB.Create(acc).Error)
	now := common.GetTimestamp()
	task := &model.HostedTask{
		AccountID: acc.ID, UserID: u.Id, Type: model.HostedTaskProfileEdit,
		Status: model.HostedTaskPending, ScheduledAt: now - 1, IdempotencyKey: "edit-vault",
		CreatedAt: now, UpdatedAt: now,
	}
	require.NoError(t, model.DB.Create(task).Error)

	var body map[string]any
	svc := NewService(gwClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/rpa/tiktok/edit", r.URL.Path)
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		envelopeOK(w, map[string]any{"jobId": "job-vault"})
	})))
	n, err := svc.DispatchDueTasks(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	assert.Equal(t, "tik.user", body["username"])
	assert.Equal(t, "vault-pw", body["password"])
	assert.Equal(t, "TOTPSECRET", body["totp"])
	assert.Equal(t, "phone-vault", body["envId"])
}

func TestPollJobsGet404Succeeded(t *testing.T) {
	clearHosted(t)
	u, _ := seedUserSKU(t, 1000)
	acc := &model.HostedAccount{
		PhoneID: "phone-404", Platform: model.HostedPlatformTikTok, Region: model.HostedRegionUS,
		Status: model.HostedAccountReady, AssignedUserID: u.Id,
	}
	require.NoError(t, model.DB.Create(acc).Error)
	now := common.GetTimestamp()
	task := &model.HostedTask{
		AccountID: acc.ID, UserID: u.Id, Type: model.HostedTaskDistribute,
		Status: model.HostedTaskQueued, GatewayJobID: "native-1", ScheduledAt: now,
		IdempotencyKey: "poll-404", CreatedAt: now, UpdatedAt: now,
	}
	require.NoError(t, model.DB.Create(task).Error)

	svc := NewService(gwClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"code":"NOT_FOUND","msg":"no job"}`))
	})))
	n, err := svc.PollGatewayJobs(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	var stored model.HostedTask
	require.NoError(t, model.DB.First(&stored, task.ID).Error)
	assert.Equal(t, model.HostedTaskSucceeded, stored.Status)
}
