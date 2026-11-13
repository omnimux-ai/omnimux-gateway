package service

import (
	"context"
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

// PersistTaskOutputVideoMetadata records the media facts of a generated output
// video from bytes the server already buffered. It is best-effort by design:
// callers run it off the request path, and any probe/storage failure only
// leaves the column NULL so the client keeps its existing fallback read. It
// never changes the task status, settlement, or completion path.
func PersistTaskOutputVideoMetadata(ctx context.Context, task *model.Task, video []byte) error {
	if task == nil || task.ID == 0 || len(video) == 0 {
		return nil
	}
	// The column is written once per task; a list reload may still carry an
	// earlier result, so skip the probe instead of repeating it.
	if len(task.OutputVideoMetadata) != 0 {
		return nil
	}
	metadata, err := ProbeInfluencerVideoPayload(ctx, video)
	if err != nil {
		return err
	}
	raw, err := common.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("marshal output video metadata: %w", err)
	}
	return model.UpdateTaskOutputVideoMetadata(task.ID, raw)
}
