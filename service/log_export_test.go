package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogExportLeaseOwnershipAndRenewal(t *testing.T) {
	for _, failure := range []string{"renewed", "expired", "replaced", "redis error"} {
		t.Run(failure, func(t *testing.T) {
			server := miniredis.RunT(t)
			previousEnabled, previousClient := common.RedisEnabled, common.RDB
			common.RedisEnabled = true
			common.RDB = redis.NewClient(&redis.Options{Addr: server.Addr(), MaxRetries: -1})
			t.Cleanup(func() { common.RDB.Close(); common.RedisEnabled, common.RDB = previousEnabled, previousClient })
			lock, ok, err := AcquireLogExportLock(t.Context(), 81)
			require.NoError(t, err)
			require.True(t, ok)
			defer lock.Release()
			key := "log_export:81"
			switch failure {
			case "renewed":
				server.FastForward(40 * time.Second)
				require.NoError(t, lock.Check())
				server.FastForward(40 * time.Second)
				assert.True(t, server.Exists(key), "the validated export renewed its reservation")
			case "expired":
				server.FastForward(logExportLockTTL)
			case "replaced":
				require.NoError(t, server.Set(key, "another-owner"))
			case "redis error":
				server.SetError("ERR temporarily unavailable")
			}
			if failure != "renewed" {
				require.ErrorIs(t, lock.Check(), ErrLogExportLockLost)
				require.ErrorIs(t, context.Cause(lock.Context()), ErrLogExportLockLost)
			}
			next, acquired, err := AcquireLogExportLock(t.Context(), 81)
			require.NoError(t, err)
			assert.False(t, acquired)
			assert.Nil(t, next, "the handler keeps its local slot until it stops")
			server.SetError("")
			lock.Release()
			lock.Release()
			if failure == "replaced" {
				value, err := server.Get(key)
				require.NoError(t, err)
				assert.Equal(t, "another-owner", value, "an old owner must not delete its successor")
				server.Del(key)
			}
			next, acquired, err = AcquireLogExportLock(t.Context(), 81)
			require.NoError(t, err)
			require.True(t, acquired)
			lock.Release()
			_, acquired, err = AcquireLogExportLock(t.Context(), 81)
			require.NoError(t, err)
			assert.False(t, acquired, "double release cannot remove the new owner's local reservation")
			next.Release()
		})
	}
}

func TestLogExportLeaseAcquisitionFailureAndCancellation(t *testing.T) {
	server := miniredis.RunT(t)
	previousEnabled, previousClient := common.RedisEnabled, common.RDB
	common.RedisEnabled = true
	common.RDB = redis.NewClient(&redis.Options{Addr: server.Addr(), MaxRetries: -1})
	t.Cleanup(func() { common.RDB.Close(); common.RedisEnabled, common.RDB = previousEnabled, previousClient })
	server.SetError("ERR unavailable")
	lock, ok, err := AcquireLogExportLock(t.Context(), 82)
	require.Error(t, err)
	assert.False(t, ok)
	assert.Nil(t, lock)
	server.SetError("")
	ctx, cancel := context.WithCancel(t.Context())
	lock, ok, err = AcquireLogExportLock(ctx, 82)
	require.NoError(t, err)
	require.True(t, ok)
	cancel()
	require.ErrorIs(t, lock.Check(), context.Canceled)
	lock.Release()
	assert.False(t, server.Exists(fmt.Sprintf("log_export:%d", 82)))
}

func TestLogExportHeartbeatCancelsOnRenewFailure(t *testing.T) {
	server := miniredis.RunT(t)
	previousEnabled, previousClient := common.RedisEnabled, common.RDB
	common.RedisEnabled = true
	common.RDB = redis.NewClient(&redis.Options{Addr: server.Addr(), MaxRetries: -1})
	t.Cleanup(func() { common.RDB.Close(); common.RedisEnabled, common.RDB = previousEnabled, previousClient })
	lock, ok, err := AcquireLogExportLock(t.Context(), 83)
	require.NoError(t, err)
	require.True(t, ok)
	defer lock.Release()
	server.SetError("ERR renewal unavailable")
	select {
	case <-lock.Context().Done():
		require.ErrorIs(t, context.Cause(lock.Context()), ErrLogExportLockLost)
	case <-time.After(logExportLockTTL/3 + 10*time.Second):
		t.Fatal("the heartbeat failed to cancel an export whose lease cannot renew")
	}
	_, ok, err = AcquireLogExportLock(t.Context(), 83)
	require.NoError(t, err)
	assert.False(t, ok, "the canceled handler must keep its local slot until release")
	server.SetError("")
}
