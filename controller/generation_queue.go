package controller

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/jsplugin"
	"github.com/QuantumNous/new-api/pkg/storage"
	"github.com/QuantumNous/new-api/relay"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

// fork: accepted workbench submissions are durable before any generation billing.
func enqueueInfluencerGeneration(c *gin.Context, path, action string, out, meta map[string]any) bool {
	if _, worker := c.Get("generation_queue_reserved_task"); worker {
		return false
	}
	if action != ActionInfluencerSheet && action != ActionInfluencerMotion {
		return false
	}
	if c.GetBool("use_access_token") || c.GetInt("id") <= 0 {
		influencerError(c, http.StatusUnauthorized, "unauthorized", "a dashboard login session is required")
		return true
	}
	group := c.GetHeader("X-Omnimux-Group")
	if group == "" {
		group = common.GetContextKeyString(c, constant.ContextKeyUsingGroup)
	}
	if group == "" || group == "auto" {
		influencerError(c, 400, "explicit_group_required", "an explicit supplier-aligned group is required")
		return true
	}
	task, err := buildInfluencerGenerationTask(c.GetInt("id"), group, path, action, out, meta, c.GetString("influencer_reference_preflight"))
	if err != nil {
		influencerError(c, 500, "internal_error", "failed to save generation request")
		return true
	}
	modelName := task.Properties.OriginModelName
	task.PrivateData.GenerationQueue.Routing = generationRequestRouting(c, group, path)
	attachInfluencerImageInputs(c, task)
	if apiErr := checkInfluencerGenerationFunding(c, task, out, meta); apiErr != nil {
		writeInfluencerFundingError(c, apiErr)
		return true
	}
	if action == ActionInfluencerMotion && !influencerRecreateQueuedDailyAllow(c, meta) {
		return true
	}
	if err := task.InsertWithContext(c.Request.Context()); err != nil {
		influencerError(c, 500, "generation_queue_unavailable", "failed to save generation request")
		return true
	}
	if action == ActionInfluencerMotion {
		c.Set(ctxInfluencerMotionSubmitting, true)
	}
	service.TrackFirstVideoStart(task, "workbench")
	c.Set("influencer_image_inputs_retained", true)
	// The scheduled scanner recovers this row even when this best-effort wakeup fails.
	if _, _, err := service.EnqueueSystemTask(model.SystemTaskTypeGenerationQueue, nil); err != nil {
		common.SysError("generation scanner wakeup failed")
	}
	c.JSON(http.StatusOK, gin.H{"id": task.TaskID, "task_id": task.TaskID, "status": "queued", "queue_status": task.Status, "created_at": task.SubmitTime, "model": modelName})
	return true
}

// checkInfluencerGenerationFunding prices the product input on each eligible
// route. If a route needs facts that are only available at dispatch, leave the
// funding decision to the worker.
func checkInfluencerGenerationFunding(c *gin.Context, task *model.Task, out, meta map[string]any) *types.NewAPIError {
	request := task.PrivateData.GenerationQueue
	user, err := model.GetUserById(task.UserId, false)
	if err != nil {
		return types.NewError(err, types.ErrorCodeQueryDataError, types.ErrOptionWithSkipRetry())
	}
	body := request.Body
	planPending := false
	if task.Action == ActionInfluencerMotion {
		quoteBody := influencerMotionTaskRequestBody(out)
		switch meta["mode"] {
		case influencerMotionModeRecreate:
			// Only Web still needs a prompt from the reference clip. Use a
			// placeholder for native validation of the declared output fields.
			planPending = !c.GetBool("influencer_creation_api")
			if planPending {
				quoteBody["prompt"] = "Generate the requested scene"
			}
		case "motion", "replace":
			planPending = true
		}
		body, err = common.Marshal(quoteBody)
		if err != nil {
			return types.NewError(err, types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
		}
	}

	canonicalPath := "/v1" + strings.TrimPrefix(request.Path, "/pg")
	auto := request.Routing.Mode == model.GenerationRoutingAuto
	groups := request.Routing.Groups
	if auto {
		groups = service.FilterUserTokenAutoGroups(user.Group, groups)
	}
	filters := service.GetChannelConstraints(c).Filters
	var group string
	var visited []int
	var candidate *relaycommon.RelayInfo
	var routing service.GenerationQuoteRouting
	var needsFetch bool
	quoteContext := jsplugin.WithFetcher(c.Request.Context(), func(context.Context, jsplugin.FetchRequest) (jsplugin.FetchResponse, error) {
		needsFetch = true
		return jsplugin.FetchResponse{}, errors.New("fetch is unavailable during a task quote")
	})
	engine := gin.New()
	engine.Use(func(quote *gin.Context) {
		defer common.CleanupBodyStorage(quote)
		quote.Set("id", user.Id)
		quote.Set("group", user.Group)
		user.ToBaseUser().WriteContext(quote)
		quote.Set("relay_mode", relayconstant.RelayModeVideoSubmit)
		quote.Set(service.GenerationQuoteRoutingKey, &routing)
		quote.Set("generation_queue_native_routing", true)
		if request.ImageInputs != nil {
			quote.Set(ctxInfluencerImageInputs, request.ImageInputs)
			applyInfluencerImageInputs(quote, make(map[string]any), request.ImageInputs)
		}
		if request.ReferencePreflight != "" {
			quote.Set("influencer_reference_preflight", request.ReferencePreflight)
		}
		constraints := service.GetChannelConstraints(quote)
		for _, filter := range filters {
			if filter.Kind == dto.FilterExcludeChannels {
				constraints.AddFilter(filter)
			}
		}
		if len(visited) > 0 {
			constraints.AddFilter(dto.ChannelFilter{Kind: dto.FilterExcludeChannels, ExcludeChannelIDs: visited})
		}
		applyInfluencerRoutingContext(quote, request.Routing)
		if auto {
			common.SetContextKey(quote, constant.ContextKeyTokenAutoGroups, []string{group})
		}
		quote.Next()
	})
	engine.POST(canonicalPath, middleware.PinTaskPluginEndpoint(), middleware.PrepareTaskPluginEndpoint(), middleware.Distribute(), func(quote *gin.Context) {
		if common.GetContextKeyInt(quote, constant.ContextKeyChannelId) <= 0 {
			return
		}
		info, err := relaycommon.GenRelayInfo(quote, types.RelayFormatTask, nil, nil)
		if err != nil {
			return
		}
		info.Action = quote.GetString("task_action")
		info.PublicTaskID = task.TaskID
		info.IsPlayground = true
		if task.Action == ActionInfluencerMotion {
			var input map[string]any
			if err := common.UnmarshalBodyReusable(quote, &input); err != nil {
				return
			}
			setInfluencerMotionTaskRequest(quote, input)
		}
		if taskErr := relay.EstimateTaskSubmissionPrice(quote, info, planPending); taskErr != nil {
			logger.LogDebug(c, "generation quote deferred to dispatch: code=%s", taskErr.Code)
			return
		}
		candidate = info
	})

	var cheapest, invalidQuote *relaycommon.RelayInfo
	for _, quoteGroup := range groups {
		group, visited = quoteGroup, nil
		for {
			routing.HasAlternative, routing.NoEligibleChannel, routing.LookupFailed = false, false, false
			candidate, needsFetch = nil, false
			// A fresh context preserves per-channel plugin ownership and
			// decoding when the next channel uses a different adaptor.
			probe := httptest.NewRequestWithContext(quoteContext, http.MethodPost, canonicalPath, bytes.NewReader(body))
			probe.Header.Set("Content-Type", "application/json")
			engine.ServeHTTP(httptest.NewRecorder(), probe)
			// A decoder or usage hook may catch a blocked fetch. Its fallback
			// cannot establish the cost of this route.
			if needsFetch || routing.LookupFailed {
				return nil
			}
			if routing.NoEligibleChannel {
				break
			}
			if candidate == nil {
				return nil
			}
			price := candidate.PriceData
			if price.FreeModel {
				return service.CheckTaskFundingAvailability(c, candidate)
			}
			if candidate.QuotaClamp != nil || price.Quota < 0 {
				invalidQuote = candidate
			} else if cheapest == nil || price.Quota < cheapest.PriceData.Quota {
				cheapest = candidate
			}
			if !routing.HasAlternative {
				break
			}
			visited = append(visited, candidate.ChannelId)
		}
	}
	// The same account and funding preference apply to every candidate. If
	// the cheapest valid paid quote cannot be covered, no higher one can be.
	if cheapest == nil {
		cheapest = invalidQuote
	}
	if cheapest == nil {
		return nil
	}
	return service.CheckTaskFundingAvailability(c, cheapest)
}

func writeInfluencerFundingError(c *gin.Context, apiErr *types.NewAPIError) {
	if apiErr.GetErrorCode() == types.ErrorCodeInsufficientUserQuota {
		influencerError(c, http.StatusForbidden, string(types.ErrorCodeInsufficientUserQuota), "Insufficient user quota")
		return
	}
	influencerError(c, http.StatusServiceUnavailable, "generation_quote_unavailable", "The pricing service is temporarily unavailable. Please try again.")
}

// buildInfluencerGenerationTask only captures normalized business data; authorization and
// durable acceptance remain the caller's responsibility.
func buildInfluencerGenerationTask(userID int, group, path, action string, out, meta map[string]any, referencePreflight string) (*model.Task, error) {
	body, err := common.Marshal(out)
	if err != nil {
		return nil, err
	}
	snapshot, err := common.Marshal(meta)
	if err != nil {
		return nil, err
	}
	modelName, _ := out["model"].(string)
	return &model.Task{TaskID: model.GenerateTaskID(), UserId: userID, Group: group, Action: action, Status: model.TaskStatusWaiting, Progress: "0%", SubmitTime: time.Now().Unix(), Properties: model.Properties{Input: string(snapshot), OriginModelName: modelName}, PrivateData: model.TaskPrivateData{GenerationQueue: &model.GenerationQueueRequest{Path: path, Body: body, ReferencePreflight: referencePreflight}}}, nil
}

type generationQueueHandler struct{}

func (generationQueueHandler) Type() string            { return model.SystemTaskTypeGenerationQueue }
func (generationQueueHandler) Enabled() bool           { return true }
func (generationQueueHandler) Interval() time.Duration { return 15 * time.Second }
func (generationQueueHandler) NewPayload() any         { return nil }
func (generationQueueHandler) Run(ctx context.Context, scanner *model.SystemTask, runnerID string) {
	if err := model.RecoverGenerationClaims(ctx, scanner, runnerID); err != nil {
		finishSystemTaskHandler(scanner, runnerID, model.SystemTaskStatusFailed, nil, err)
		return
	}
	workerCtx, cancel := context.WithCancel(ctx)
	var wg sync.WaitGroup
	var runErr error
	defer func() {
		cancel()
		wg.Wait()
		status := model.SystemTaskStatusSucceeded
		if runErr != nil {
			status = model.SystemTaskStatusFailed
		}
		finishSystemTaskHandler(scanner, runnerID, status, nil, runErr)
	}()
	// Keep the existing lease heartbeat alive while synchronous submissions run.
	// Completion wakes the scan immediately; polling also notices upstream tasks
	// completed by the native poller and newly accepted requests on other nodes.
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	completed := make(chan int64)
	// Only rows whose worker is still running are protected from expiry; a row
	// this scanner left unconfirmed must still expire while the scan lives on.
	running := map[int64]struct{}{}
	var lastExpiry time.Time
	for {
		if ctx.Err() != nil {
			runErr = ctx.Err()
			return
		}
		// The scan stays alive while any user waits, so expiry runs inside the
		// loop rather than once at start. A failed pass is retried on the next
		// interval instead of cancelling submissions that are still running.
		if time.Since(lastExpiry) >= generationDispatchExpiryInterval {
			if err := expireGenerationDispatches(ctx, scanner, runnerID, running); err != nil {
				common.SysError("generation dispatch expiry failed: " + err.Error())
			}
			lastExpiry = time.Now()
		}
		users, err := model.GenerationWaitingUsers(ctx)
		if err != nil {
			runErr = err
			return
		}
		for _, id := range users {
			tasks, err := model.ClaimGenerationTasks(ctx, id, scanner.TaskID)
			if err != nil {
				runErr = err
				return
			}
			for _, task := range tasks {
				running[task.ID] = struct{}{}
				wg.Add(1)
				go func(task *model.Task) {
					defer wg.Done()
					dispatchInfluencerGeneration(workerCtx, task, scanner, runnerID)
					select {
					case completed <- task.ID:
					case <-workerCtx.Done():
					}
				}(task)
			}
		}
		if len(running) == 0 && len(users) == 0 {
			return
		}
		select {
		case <-ctx.Done():
			runErr = ctx.Err()
			return
		case id := <-completed:
			delete(running, id)
		case <-ticker.C:
		}
	}
}

const generationDispatchExpiryInterval = 15 * time.Second

// expireGenerationDispatches fails unconfirmed dispatches past the configured
// timeout and returns each persisted reservation once. A failed release keeps
// the task quota for reconciliation and is never retried here.
func expireGenerationDispatches(ctx context.Context, scanner *model.SystemTask, runnerID string, running map[int64]struct{}) error {
	minutes := constant.GenerationDispatchTimeoutMinutes
	if minutes <= 0 {
		return nil
	}
	inFlight := make([]int64, 0, len(running))
	for id := range running {
		inFlight = append(inFlight, id)
	}
	cutoff := time.Now().Unix() - int64(minutes)*60
	expired, err := model.ExpireGenerationDispatches(ctx, scanner, runnerID, cutoff, minutes, inFlight)
	for _, task := range expired {
		if !service.ReleaseUnloggedTaskReservation(ctx, task, task.FailReason) {
			common.SysError("generation dispatch " + task.TaskID + " expired but its reservation release failed")
		}
	}
	return err
}

func generationWorkerContext(ctx context.Context, task *model.Task, scanner *model.SystemTask, runnerID string) (*gin.Context, error) {
	request := task.PrivateData.GenerationQueue
	if request == nil {
		return nil, errors.New("generation request is missing")
	}
	switch request.Path {
	case "/pg/images/generations", "/pg/images/edits", "/pg/video/generations":
	default:
		return nil, errors.New("invalid generation path")
	}
	user, err := model.GetUserById(task.UserId, false)
	if err != nil {
		return nil, err
	}
	if user.Status != common.UserStatusEnabled {
		return nil, errors.New("generation account is disabled")
	}
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, request.Path, bytes.NewReader(request.Body)).WithContext(ctx)
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("X-Omnimux-Group", task.Group)
	c.Set("id", user.Id)
	c.Set("username", user.Username)
	c.Set("role", user.Role)
	c.Set("group", user.Group)
	c.Set("user_group", user.Group)
	user.ToBaseUser().WriteContext(c)
	var body map[string]any
	if err := common.Unmarshal(request.Body, &body); err != nil {
		return nil, err
	}
	sunburst := task.Properties.OriginModelName == influencerImageModel || body["model"] == influencerImageModel
	if sunburst {
		if task.Action != ActionInfluencerSheet || task.Properties.OriginModelName != influencerImageModel || task.Group != influencerImageGroup {
			return nil, errors.New("image queue owner contract is invalid")
		}
		if err := validateInfluencerImageInputs(c, request, body); err != nil {
			return nil, err
		}
		if (request.ImageInputs.Mode == "text" && request.Path != "/pg/images/generations") || (request.ImageInputs.Mode != "text" && request.Path != "/pg/images/edits") {
			return nil, errors.New("image queue path does not match mode")
		}
	}
	if routing := request.Routing; routing != nil {
		if routing.MaxAttempts < 1 || routing.MaxAttempts > 2 || len(routing.Attempts) > 0 || len(routing.Groups) == 0 || (routing.Mode != model.GenerationRoutingAuto && routing.Mode != model.GenerationRoutingGroup) || (routing.Mode == model.GenerationRoutingGroup && (len(routing.Groups) != 1 || routing.Groups[0] != task.Group)) {
			return nil, errors.New("generation routing policy is invalid or already sent")
		}
		reference, _ := body["image"].(string)
		_, hasImage := body["image"]
		_, hasImages := body["images"]
		_, hasImageURLs := body["image_urls"]
		_, hasInputReference := body["input_reference"]
		additionalReferences := hasImages || hasImageURLs || hasInputReference
		autoReference := request.Path == "/pg/images/edits" && request.ReferencePreflight != "" && request.ReferencePreflight == reference && body["model"] == "gpt-image-2.5" && !additionalReferences
		plainImage := request.Path == "/pg/images/generations" && request.ReferencePreflight == "" && !hasImage && !additionalReferences && body["model"] == "gpt-image-2.5"
		if routing.Mode == model.GenerationRoutingAuto && !sunburst && (task.Action != ActionInfluencerSheet || (!plainImage && !autoReference) || task.Properties.OriginModelName != "gpt-image-2.5" || task.Group != influencerReferenceGroup) {
			return nil, errors.New("generation auto policy is outside its supported contract")
		}
		liveRouting := *routing
		if routing.Mode == model.GenerationRoutingAuto {
			currentGroups, err := influencerImageAutoGroups(c, task.Properties.OriginModelName, task.Group)
			if (autoReference || sunburst) && err == nil {
				currentGroups, err = influencerReferenceAutoGroups(c, task.Properties.OriginModelName, currentGroups)
				if request.ReferencePreflight != "" {
					c.Set("influencer_reference_preflight", request.ReferencePreflight)
				}
			}
			if err != nil {
				return nil, err
			}
			liveRouting.Groups = nil
			for _, group := range routing.Groups {
				if slices.Contains(currentGroups, group) {
					liveRouting.Groups = append(liveRouting.Groups, group)
				}
			}
			if len(liveRouting.Groups) == 0 {
				return nil, errors.New("generation routing permissions are no longer available")
			}
		}
		c.Set(ctxInfluencerRouting, &liveRouting)
		if routing.Mode == model.GenerationRoutingAuto {
			c.Set(ctxInfluencerQuoteGroup, task.Group)
		}
		applyInfluencerRoutingContext(c, &liveRouting)
	}
	c.Set("generation_queue_reserved_task", task)
	c.Set("generation_queue_scanner", scanner)
	c.Set("generation_queue_runner", runnerID)
	c.Set("relay_mode", relayconstant.RelayModeVideoSubmit)
	return c, nil
}

func dispatchInfluencerGeneration(ctx context.Context, task *model.Task, scanner *model.SystemTask, runnerID string) {
	if task == nil || task.PrivateData.GenerationQueue == nil || task.Status != model.TaskStatusClaimed || task.PrivateData.GenerationQueue.SendStartedAt != 0 {
		return
	}
	current, exists, err := model.GetByTaskId(task.UserId, task.TaskID)
	if err != nil || !exists || current.Status != model.TaskStatusClaimed || current.PrivateData.GenerationQueue == nil || current.PrivateData.GenerationQueue.SendStartedAt != 0 || current.PrivateData.GenerationQueue.ClaimOwner != task.PrivateData.GenerationQueue.ClaimOwner {
		return
	}
	// fork: a viral-recreation analysis job runs planning in this worker instead of
	// submitting to a provider. It has no channel, no billing and no upstream body,
	// so it never enters the generation context below.
	if task.Action == ActionInfluencerRecreateAnalysis {
		runInfluencerRecreateAnalysis(ctx, task)
		return
	}
	// Every request is freshly distributed; no auth or selected channel survives
	// enqueue, and an old scanner cannot enter submission after losing its lease.
	c, err := generationWorkerContext(ctx, task, scanner, runnerID)
	if err != nil {
		failGenerationBeforeDispatch(ctx, task, "generation account or request is unavailable")
		return
	}
	request := task.PrivateData.GenerationQueue
	// Protocol ownership is declared on canonical API paths. Billing still
	// identifies this as an accepted playground task in executeTaskSubmission.
	c.Request.URL.Path = "/v1" + strings.TrimPrefix(request.Path, "/pg")
	engine := gin.New()
	identityKeys := c.Keys
	engine.Use(func(worker *gin.Context) {
		for key, value := range identityKeys {
			worker.Set(key, value)
		}
	})
	engine.POST(c.Request.URL.Path, middleware.PinTaskPluginEndpoint(), middleware.PrepareTaskPluginEndpoint(), middleware.Distribute(), func(c *gin.Context) {
		var out map[string]any
		if err := common.Unmarshal(request.Body, &out); err != nil {
			influencerError(c, 400, "invalid_request", "invalid generation request")
			return
		}
		meta, parsed := parseInfluencerTaskMeta(task.Properties.Input)
		if parsed && meta.Kind == influencerTaskKindMultiView {
			parent, exists, err := model.GetByTaskId(task.UserId, meta.Parent)
			if err != nil || !exists || parent.Action != ActionInfluencerSheet || parent.Status != model.TaskStatusSuccess || !parent.ResultRetrievable() {
				c.Set("generation_queue_preflight_failure", "父任务不可用，任务未开始")
				influencerError(c, 400, "generation_material_unavailable", "父任务不可用，任务未开始")
				return
			}
			parentMeta, parsed := parseInfluencerTaskMeta(parent.Properties.Input)
			imageURL, _ := out["image"].(string)
			if request.ImageInputs != nil {
				imageURL = meta.ImageURL
			}
			if !parsed || parentMeta.Kind == influencerTaskKindMultiView || imageURL != meta.ImageURL || (request.ImageInputs == nil && request.ReferencePreflight != "" && request.ReferencePreflight != imageURL) || !influencerParentImageArtifact(parent, imageURL) {
				c.Set("generation_queue_preflight_failure", "父任务成品不匹配，任务未开始")
				influencerError(c, 400, "generation_material_unavailable", "父任务成品不匹配，任务未开始")
				return
			}
			if request.ReferencePreflight == "" {
				if err := service.ValidateInfluencerReference(ctx, imageURL, influencerReferenceHTTPClient); err != nil {
					c.Set("generation_queue_preflight_failure", "父任务成品暂不可读取，任务未开始")
					influencerError(c, 400, "generation_material_unavailable", "父任务成品暂不可读取，任务未开始")
					return
				}
			}
		}
		if task.Action == ActionInfluencerMotion {
			if err := preflightInfluencerFrozenMedia(ctx, task.UserId, out); err != nil {
				c.Set("generation_queue_preflight_failure", "参考材料不可用或已失效，任务未开始")
				influencerError(c, 400, "generation_material_unavailable", "参考材料不可用或已失效，任务未开始")
				return
			}
			if err := prepareInfluencerVideoPrompt(ctx, task, out, scanner, runnerID); err != nil {
				c.Set("generation_queue_preflight_failure", err.Error())
				influencerError(c, 503, "generation_material_understanding_failed", err.Error())
				return
			}
			// Native adaptors validate and build from body storage; without this
			// rewrite the planner's prompt/duration only existed in task_request
			// and the provider received the empty acceptance-time body.
			if err := writeInfluencerMotionTaskRequestBody(c, out); err != nil {
				c.Set("generation_queue_preflight_failure", "failed to build upstream request")
				influencerError(c, http.StatusInternalServerError, "internal_error", "failed to build upstream request")
				return
			}
			setInfluencerMotionTaskRequest(c, out)
		}
		if request.ReferencePreflight != "" {
			if _, valid := influencerReferenceSelectedChannel(c); !valid {
				influencerError(c, 503, "reference_route_unavailable", "reference image route is unavailable")
				return
			}
			if request.ImageInputs == nil {
				if err := service.ValidateInfluencerReference(ctx, request.ReferencePreflight, influencerReferenceHTTPClient); err != nil {
					influencerError(c, 400, "invalid_reference_image", "reference image is unavailable")
					return
				}
			}
			c.Set("influencer_reference_preflight", request.ReferencePreflight)
			submitInfluencerReferenceTask(c)
		} else {
			RelayTask(c)
		}
	})
	// HandleContext preserves only the reconstructed business keys above and
	// executes the same router middleware, never a saved login context.
	engine.HandleContext(c)
	if !c.GetBool("generation_queue_attempt_started") || c.GetBool("generation_queue_safe_failure") {
		reason := c.GetString("generation_queue_preflight_failure")
		if reason == "" {
			reason = "generation could not start; check account, balance and available route"
		}
		failGenerationBeforeDispatch(ctx, task, reason)
	}
	// Once submission starts, an unrecognized response remains DISPATCHING.
	// Native submission owns reserve/settle/refund; the scanner never repeats it.
}

// Frozen videos and audio retain their accepted bytes and original expiry.
func preflightInfluencerFrozenMedia(ctx context.Context, userID int, out map[string]any) error {
	driver := storage.GetStorageDriver()
	for _, field := range []string{"video_urls", "audio_urls", "videos", "audios"} {
		if _, exists := out[field]; !exists {
			continue
		}
		urls, ok := influencerMediaURLs(out, field, "")
		if !ok || len(urls) == 0 {
			return errors.New("invalid frozen media references")
		}
		for _, raw := range urls {
			file, err := model.GetUniqueFileByURL(ctx, raw)
			if err != nil {
				return err
			}
			if file.UserId != userID || file.Status != model.FileStatusActive || !file.ExpiresAt.After(time.Now()) || !strings.HasPrefix(file.StorageKey, "influencer-reference/frozen/") || path.Clean(file.StorageKey) != file.StorageKey || path.Dir(file.StorageKey) != "influencer-reference/frozen" || driver.GetPublicURL(file.StorageKey) != raw {
				return errors.New("frozen media is unavailable")
			}
			reader, _, _, err := driver.Get(ctx, file.StorageKey)
			if err != nil {
				return err
			}
			_, readErr := io.CopyN(io.Discard, reader, 1)
			closeErr := reader.Close()
			if readErr != nil || closeErr != nil {
				return errors.New("frozen media cannot be read")
			}
		}
	}
	return nil
}

func failGenerationBeforeDispatch(ctx context.Context, task *model.Task, reason string) {
	err := model.RejectGenerationDispatch(ctx, task, reason)
	if err != nil {
		common.SysError("generation rejection could not be saved")
	}
}
