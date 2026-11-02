package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
)

const logExportLockTTL = time.Minute
const logExportLockOperationTimeout = 5 * time.Second

var ErrLogExportLockLost = errors.New("log export lock lost")

const logExportLockRenewScript = `
if redis.call('GET', KEYS[1]) == ARGV[1] then
  return redis.call('PEXPIRE', KEYS[1], ARGV[2])
end
return 0`

const logExportLockReleaseScript = `
if redis.call('GET', KEYS[1]) == ARGV[1] then
  return redis.call('DEL', KEYS[1])
end
return 0`

// The local reservation also stays held when Redis ownership is lost, until
// the canceled handler has stopped writing and releases it.
var logExportLocks sync.Map

type LogExportLock struct {
	ctx     context.Context
	cancel  context.CancelCauseFunc
	renew   func() error
	release func()
}

func (l *LogExportLock) Context() context.Context { return l.ctx }
func (l *LogExportLock) Release()                 { l.release() }

// Check verifies ownership before streaming and before reporting success.
// A renewal error cancels the stream rather than letting an expired owner
// keep exporting or claim that its partial output is complete.
func (l *LogExportLock) Check() error {
	if err := context.Cause(l.ctx); err != nil {
		return err
	}
	if l.renew == nil {
		return nil
	}
	if err := l.renew(); err != nil {
		l.cancel(err)
		return err
	}
	return context.Cause(l.ctx)
}

// AcquireLogExportLock reserves one running export per user. Redis ownership
// spans nodes, while the local reservation remains until the handler stops.
func AcquireLogExportLock(parent context.Context, userId int) (*LogExportLock, bool, error) {
	if err := parent.Err(); err != nil {
		return nil, false, err
	}
	owner := common.GetRandomString(16)
	if _, running := logExportLocks.LoadOrStore(userId, owner); running {
		return nil, false, nil
	}
	unlockLocal := func() { logExportLocks.CompareAndDelete(userId, owner) }
	ctx, cancel := context.WithCancelCause(parent)
	lock := &LogExportLock{ctx: ctx, cancel: cancel}
	var once sync.Once
	if !common.RedisEnabled {
		lock.release = func() { once.Do(func() { cancel(context.Canceled); unlockLocal() }) }
		return lock, true, nil
	}

	client := common.RDB
	if client == nil {
		cancel(ErrLogExportLockLost)
		unlockLocal()
		return nil, false, ErrLogExportLockLost
	}
	key := fmt.Sprintf("log_export:%d", userId)
	acquireCtx, stopAcquire := context.WithTimeout(ctx, logExportLockOperationTimeout)
	acquired, err := client.SetNX(acquireCtx, key, owner, logExportLockTTL).Result()
	stopAcquire()
	if err != nil || !acquired {
		cancel(context.Canceled)
		unlockLocal()
		return nil, false, err
	}
	lock.renew = func() error {
		renewCtx, stopRenew := context.WithTimeout(ctx, logExportLockOperationTimeout)
		defer stopRenew()
		owned, err := client.Eval(renewCtx, logExportLockRenewScript, []string{key}, owner, logExportLockTTL.Milliseconds()).Int64()
		if err != nil {
			return fmt.Errorf("%w: %v", ErrLogExportLockLost, err)
		}
		if owned != 1 {
			return ErrLogExportLockLost
		}
		return nil
	}
	stop, done := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(logExportLockTTL / 3)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				if lock.Check() != nil {
					return
				}
			}
		}
	}()
	lock.release = func() {
		once.Do(func() {
			close(stop)
			cancel(context.Canceled)
			<-done
			releaseCtx, stopRelease := context.WithTimeout(context.Background(), logExportLockOperationTimeout)
			defer stopRelease()
			client.Eval(releaseCtx, logExportLockReleaseScript, []string{key}, owner)
			unlockLocal()
		})
	}
	return lock, true, nil
}
