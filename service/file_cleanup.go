package service

import (
	"context"
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/storage"
)

const (
	fileExpirationCleanupInterval = 10 * time.Minute
	fileExpirationBatchSize       = 100
)

// StartFileExpirationCleanup starts the background worker that periodically purges
// files that have passed their 72-hour expiration window.
func StartFileExpirationCleanup() {
	if !common.IsMasterNode {
		return
	}
	go func() {
		cleanupExpiredFiles()
		ticker := time.NewTicker(fileExpirationCleanupInterval)
		defer ticker.Stop()
		for range ticker.C {
			cleanupExpiredFiles()
		}
	}()
}

func cleanupExpiredFiles() {
	driver := storage.GetStorageDriver()
	for {
		expiredFiles, err := model.GetExpiredFiles(fileExpirationBatchSize)
		if err != nil {
			common.SysError(fmt.Sprintf("failed to query expired files: %v", err))
			return
		}
		if len(expiredFiles) == 0 {
			break
		}

		cleanupFailed := false
		for _, f := range expiredFiles {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			err := driver.Delete(ctx, f.StorageKey)
			cancel()
			if err != nil {
				common.SysError(fmt.Sprintf("failed to delete expired file from storage (key=%s, id=%d): %v", f.StorageKey, f.Id, err))
				cleanupFailed = true
				continue
			}
			if err := model.MarkFileStatus(f.Id, model.FileStatusExpired); err != nil {
				common.SysError(fmt.Sprintf("failed to mark file expired in db (id=%d): %v", f.Id, err))
				cleanupFailed = true
			}
		}

		// A transient failure remains eligible next tick, never a tight retry loop.
		if cleanupFailed || len(expiredFiles) < fileExpirationBatchSize {
			break
		}
	}
}
