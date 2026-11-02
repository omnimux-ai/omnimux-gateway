package controller

import (
	"context"
	"reflect"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
)

// generationSubmissionQuote uses the native deep-copied billing context. Quota
// is the attempt's computed charge, never the existing session reservation.
// Requiring the same context also fixes units, counts and effective ratios for
// completion; equal rounded totals alone are insufficient.
type generationSubmissionQuote struct {
	Quota   int
	Billing *model.TaskBillingContext
}

func (quote *generationSubmissionQuote) Matches(quota int, billing *model.TaskBillingContext) bool {
	return quote != nil && quote.Quota == quota && reflect.DeepEqual(quote.Billing, billing)
}

func persistGenerationAttemptOutcome(c *gin.Context, task *model.Task, channelID int, outcome string) error {
	scanner, _ := c.Get("generation_queue_scanner")
	lease, _ := scanner.(*model.SystemTask)
	return model.PersistGenerationAttemptOutcome(c.Request.Context(), task, lease, c.GetString("generation_queue_runner"), channelID, outcome)
}

// Preserve a valid receipt even when cancellation or later pricing/persistence
// checks leave the public task awaiting reconciliation. This never publishes a
// terminal result or allows another send.
func persistGenerationAcceptedReceipt(c *gin.Context, task *model.Task, info *relaycommon.RelayInfo, result *relay.TaskSubmitResult, quote *generationSubmissionQuote) error {
	snapshot, err := taskFundingSnapshot(c, info, result.Platform)
	if err != nil {
		return err
	}
	if quote != nil {
		snapshot.PrivateData.BillingContext = quote.Billing
	}
	snapshot.PrivateData.UpstreamTaskID = result.UpstreamTaskID
	if task.HasVersionedSunburstImageInputs() {
		receipt := &model.GenerationAcceptedResult{}
		if immediate := result.Immediate; immediate != nil {
			receipt.Status, receipt.URL, receipt.Reason, receipt.Progress = model.TaskStatus(immediate.Status), immediate.Url, immediate.Reason, immediate.Progress
		}
		snapshot.PrivateData.GenerationAcceptedResult = receipt
	}
	snapshot.PrivateData.PluginState = append([]byte(nil), result.PluginState...)
	snapshot.Data = append([]byte(nil), result.TaskData...)
	scanner, _ := c.Get("generation_queue_scanner")
	lease, _ := scanner.(*model.SystemTask)
	ctx, cancel := context.WithTimeout(context.WithoutCancel(c.Request.Context()), 5*time.Second)
	defer cancel()
	if err := model.PersistGenerationSendSnapshot(ctx, task, snapshot, lease, c.GetString("generation_queue_runner")); err != nil {
		return err
	}
	return model.PersistGenerationAttemptOutcome(ctx, task, lease, c.GetString("generation_queue_runner"), snapshot.ChannelId, model.GenerationAttemptAccepted)
}
