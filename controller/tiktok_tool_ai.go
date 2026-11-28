package controller

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

const (
	tiktokToolAICacheTTL      = 30 * time.Minute
	tiktokToolAICacheMaxItems = 500
	tiktokToolAIBurstWindow   = 30 * time.Second
	tiktokToolAIBurstLimit    = 4
	tiktokToolAIMaxInputCues  = 180
	tiktokToolAIMaxInputRunes = 10000
)

var tiktokToolVideoIDPattern = regexp.MustCompile(`^[0-9]{15,22}$`)

var tiktokToolAITargetLangs = map[string]string{
	"en":    "English",
	"zh-CN": "Simplified Chinese",
	"zh-TW": "Traditional Chinese",
	"ja":    "Japanese",
	"es":    "Spanish",
	"id":    "Indonesian",
}

var tiktokToolBreakdownStages = map[string]struct{}{
	"hook":          {},
	"product_intro": {},
	"demo_scene":    {},
	"proof_effect":  {},
	"authenticity":  {},
	"usage_detail":  {},
	"cta":           {},
	"pacing":        {},
}

type tiktokToolAISettings struct {
	model     string
	group     string
	userDaily int
}

func tiktokToolAIConfig() tiktokToolAISettings {
	return tiktokToolAISettings{
		model:     common.GetEnvOrDefaultString("TIKTOK_TOOL_AI_MODEL", service.InfluencerMotionPlannerModel),
		group:     common.GetEnvOrDefaultString("TIKTOK_TOOL_AI_GROUP", service.InfluencerMotionPlannerGroup),
		userDaily: max(1, common.GetEnvOrDefault("TIKTOK_TOOL_AI_USER_DAILY", 20)),
	}
}

type tiktokToolAICacheEntry struct {
	payload any
	expires time.Time
}

var tiktokToolAICache = struct {
	sync.Mutex
	entries map[string]tiktokToolAICacheEntry
}{entries: map[string]tiktokToolAICacheEntry{}}

type tiktokToolAIRequest struct {
	VideoID    string `json:"video_id"`
	TrackID    string `json:"track_id"`
	TargetLang string `json:"target_lang"`
}

type tiktokToolTranslateResult struct {
	VideoID    string                `json:"video_id"`
	TrackID    string                `json:"track_id"`
	TargetLang string                `json:"target_lang"`
	Cues       []tiktokTranscriptCue `json:"cues"`
}

type tiktokBreakdownSection struct {
	Stage    string `json:"stage"`
	Label    string `json:"label"`
	StartMs  int    `json:"start_ms"`
	EndMs    int    `json:"end_ms"`
	Quote    string `json:"quote"`
	Analysis string `json:"analysis"`
}

type tiktokBreakdownResult struct {
	VideoID    string                   `json:"video_id"`
	TrackID    string                   `json:"track_id"`
	TargetLang string                   `json:"target_lang"`
	Summary    string                   `json:"summary"`
	Sections   []tiktokBreakdownSection `json:"sections"`
}

type tiktokToolAICueInput struct {
	Index int    `json:"i"`
	Time  string `json:"t"`
	Text  string `json:"text"`
}

type tiktokToolPreparedContext struct {
	settings      tiktokToolAISettings
	userID        int
	videoID       string
	track         tiktokTranscriptTrack
	targetLang    string
	languageLabel string
	durationMs    int
	aiQuotaKey    string
	cacheKey      string
	now           time.Time
}

func formatAICueTimestamp(ms int) string {
	totalSeconds := max(0, ms/1000)
	minutes := totalSeconds / 60
	seconds := totalSeconds % 60
	return fmt.Sprintf("%02d:%02d", minutes, seconds)
}

func tiktokToolAICuesInput(cues []tiktokTranscriptCue) ([]tiktokTranscriptCue, string) {
	limit := min(len(cues), tiktokToolAIMaxInputCues)
	selected := make([]tiktokTranscriptCue, 0, limit)
	payload := make([]tiktokToolAICueInput, 0, limit)
	totalRunes := 0
	for i := range limit {
		cue := cues[i]
		text := strings.TrimSpace(cue.Text)
		if text == "" {
			continue
		}
		runeCount := len([]rune(text))
		if len(selected) > 0 && totalRunes+runeCount > tiktokToolAIMaxInputRunes {
			break
		}
		totalRunes += runeCount
		selected = append(selected, cue)
		payload = append(payload, tiktokToolAICueInput{
			Index: len(payload),
			Time:  formatAICueTimestamp(cue.StartMs),
			Text:  text,
		})
	}
	encoded, err := common.Marshal(payload)
	if err != nil {
		return nil, ""
	}
	return selected, string(encoded)
}

func prepareTikTokToolAIRequest(c *gin.Context, action string) (tiktokToolPreparedContext, any, bool) {
	c.Header("Cache-Control", "no-store")
	baseSettings := tiktokToolConfig()
	if !tiktokToolSameOrigin(c, baseSettings) {
		tiktokToolFail(c, http.StatusForbidden, "forbidden")
		return tiktokToolPreparedContext{}, nil, false
	}
	if !baseSettings.enabled || baseSettings.channelID <= 0 {
		tiktokToolFail(c, http.StatusServiceUnavailable, "unavailable_service")
		return tiktokToolPreparedContext{}, nil, false
	}
	userID := c.GetInt("id")
	if userID <= 0 {
		tiktokToolFail(c, http.StatusUnauthorized, "login_required")
		return tiktokToolPreparedContext{}, nil, false
	}

	var req tiktokToolAIRequest
	if err := common.DecodeJson(io.LimitReader(c.Request.Body, 16<<10), &req); err != nil {
		tiktokToolFail(c, http.StatusBadRequest, "invalid_param")
		return tiktokToolPreparedContext{}, nil, false
	}
	req.VideoID = strings.TrimSpace(req.VideoID)
	req.TrackID = strings.TrimSpace(req.TrackID)
	req.TargetLang = strings.TrimSpace(req.TargetLang)
	if req.TargetLang == "" {
		req.TargetLang = "en"
	}
	if !tiktokToolVideoIDPattern.MatchString(req.VideoID) {
		tiktokToolFail(c, http.StatusBadRequest, "invalid_param")
		return tiktokToolPreparedContext{}, nil, false
	}
	languageLabel, ok := tiktokToolAITargetLangs[req.TargetLang]
	if !ok {
		tiktokToolFail(c, http.StatusBadRequest, "invalid_param")
		return tiktokToolPreparedContext{}, nil, false
	}

	now := time.Now()
	tiktokToolCache.Lock()
	cachedPost, postHit := tiktokToolCache.entries[req.VideoID]
	tiktokToolCache.Unlock()
	if !postHit || now.After(cachedPost.expires) {
		tiktokToolFail(c, http.StatusGone, "cache_expired")
		return tiktokToolPreparedContext{}, nil, false
	}
	post := cachedPost.body
	if !post.transcriptsLoaded || len(post.transcripts) == 0 {
		tiktokToolFail(c, http.StatusBadRequest, "no_captions")
		return tiktokToolPreparedContext{}, nil, false
	}

	var selectedTrack tiktokTranscriptTrack
	foundTrack := false
	if req.TrackID == "" {
		selectedTrack = post.transcripts[0]
		foundTrack = true
	} else {
		for _, candidate := range post.transcripts {
			if candidate.ID == req.TrackID {
				selectedTrack = candidate
				foundTrack = true
				break
			}
		}
	}
	if !foundTrack || len(selectedTrack.Cues) == 0 {
		tiktokToolFail(c, http.StatusBadRequest, "no_captions")
		return tiktokToolPreparedContext{}, nil, false
	}

	cacheKey := action + ":" + req.VideoID + ":" + selectedTrack.ID + ":" + req.TargetLang
	tiktokToolAICache.Lock()
	cachedAI, aiHit := tiktokToolAICache.entries[cacheKey]
	tiktokToolAICache.Unlock()
	if aiHit && !now.After(cachedAI.expires) {
		return tiktokToolPreparedContext{}, cachedAI.payload, true
	}

	ctx := c.Request.Context()
	burstKey := fmt.Sprintf("tiktoktool:v1:ai_burst:%d", userID)
	burst, err := tiktokToolCounters.incr(ctx, burstKey, tiktokToolAIBurstWindow)
	if err != nil {
		tiktokToolFail(c, http.StatusServiceUnavailable, "busy")
		return tiktokToolPreparedContext{}, nil, false
	}
	if burst > tiktokToolAIBurstLimit {
		c.Header("Retry-After", strconv.Itoa(int(tiktokToolAIBurstWindow.Seconds())))
		tiktokToolFail(c, http.StatusTooManyRequests, "rate_limited")
		return tiktokToolPreparedContext{}, nil, false
	}

	aiSettings := tiktokToolAIConfig()
	aiQuotaKey := fmt.Sprintf("tiktoktool:v1:ai_user:%s:%d", tiktokToolDay(now), userID)
	used, err := tiktokToolCounters.get(ctx, aiQuotaKey)
	if err != nil {
		tiktokToolFail(c, http.StatusServiceUnavailable, "busy")
		return tiktokToolPreparedContext{}, nil, false
	}
	if used >= int64(aiSettings.userDaily) {
		tiktokToolFail(c, http.StatusTooManyRequests, "ai_quota_exceeded")
		return tiktokToolPreparedContext{}, nil, false
	}

	return tiktokToolPreparedContext{
		settings:      aiSettings,
		userID:        userID,
		videoID:       req.VideoID,
		track:         selectedTrack,
		targetLang:    req.TargetLang,
		languageLabel: languageLabel,
		durationMs:    post.durationMs,
		aiQuotaKey:    aiQuotaKey,
		cacheKey:      cacheKey,
		now:           now,
	}, nil, true
}

func storeTikTokToolAICache(cacheKey string, now time.Time, payload any) {
	tiktokToolAICache.Lock()
	defer tiktokToolAICache.Unlock()
	if len(tiktokToolAICache.entries) >= tiktokToolAICacheMaxItems {
		for k, entry := range tiktokToolAICache.entries {
			if now.After(entry.expires) {
				delete(tiktokToolAICache.entries, k)
			}
		}
		if len(tiktokToolAICache.entries) >= tiktokToolAICacheMaxItems {
			tiktokToolAICache.entries = map[string]tiktokToolAICacheEntry{}
		}
	}
	tiktokToolAICache.entries[cacheKey] = tiktokToolAICacheEntry{
		payload: payload,
		expires: now.Add(tiktokToolAICacheTTL),
	}
}

type tiktokTranslateModelReply struct {
	Cues []struct {
		Index int    `json:"i"`
		Text  string `json:"text"`
	} `json:"cues"`
}

func parseAndValidateTranslation(reply string, source []tiktokTranscriptCue) (map[int]string, error) {
	var parsed tiktokTranslateModelReply
	if err := common.UnmarshalJsonStr(service.StripJSONFence(reply), &parsed); err != nil {
		return nil, errors.New("invalid translation json")
	}
	if len(parsed.Cues) == 0 {
		return nil, errors.New("translation returned no cues")
	}
	translated := make(map[int]string, len(parsed.Cues))
	for _, item := range parsed.Cues {
		if item.Index < 0 || item.Index >= len(source) {
			continue
		}
		text := strings.TrimSpace(item.Text)
		if text != "" {
			translated[item.Index] = text
		}
	}
	required := max(1, (len(source)*8+9)/10)
	if len(translated) < required {
		return nil, errors.New("translation cue coverage too low")
	}
	return translated, nil
}

// TranslateTikTokToolTranscript POST /api/tools/tiktok/translate
func TranslateTikTokToolTranscript(c *gin.Context) {
	prepared, cachedPayload, ok := prepareTikTokToolAIRequest(c, "translate")
	if !ok {
		return
	}
	if cachedPayload != nil {
		c.JSON(http.StatusOK, gin.H{"success": true, "data": cachedPayload})
		return
	}

	sourceCues, encodedCues := tiktokToolAICuesInput(prepared.track.Cues)
	if len(sourceCues) == 0 || encodedCues == "" {
		tiktokToolFail(c, http.StatusBadRequest, "no_captions")
		return
	}

	systemPrompt := "You translate short-video spoken caption lines accurately and naturally. " +
		"Return strict JSON only matching {\"cues\":[{\"i\":0,\"text\":\"...\"}]}. " +
		"Keep every cue index aligned one-to-one with the input array, keep natural spoken cadence, and output zero markdown or extra keys."
	userPrompt := fmt.Sprintf("Target language: %s (%s)\nSource track language: %s\nTranslate every cue text in this JSON array:\n%s",
		prepared.languageLabel, prepared.targetLang, prepared.track.LanguageCode, encodedCues)

	reply, err := service.TikTokToolAIChat(
		c.Request.Context(),
		prepared.settings.model,
		prepared.settings.group,
		systemPrompt,
		userPrompt,
		func(raw string) error {
			_, checkErr := parseAndValidateTranslation(raw, sourceCues)
			return checkErr
		},
	)
	if err != nil {
		common.SysError(common.LogText("tiktok tool translate failed: %v", err))
		tiktokToolFail(c, http.StatusBadGateway, "upstream_failed")
		return
	}

	translatedMap, err := parseAndValidateTranslation(reply, sourceCues)
	if err != nil {
		tiktokToolFail(c, http.StatusBadGateway, "upstream_failed")
		return
	}

	outCues := make([]tiktokTranscriptCue, 0, len(sourceCues))
	for i, orig := range sourceCues {
		text := translatedMap[i]
		if text == "" {
			text = orig.Text
		}
		outCues = append(outCues, tiktokTranscriptCue{
			StartMs: orig.StartMs,
			EndMs:   orig.EndMs,
			Text:    text,
		})
	}

	result := tiktokToolTranslateResult{
		VideoID:    prepared.videoID,
		TrackID:    prepared.track.ID,
		TargetLang: prepared.targetLang,
		Cues:       outCues,
	}
	if _, incrErr := tiktokToolCounters.incr(c.Request.Context(), prepared.aiQuotaKey, tiktokToolSecondsToMidnight(prepared.now)); incrErr != nil {
		common.SysError(common.LogText("tiktok tool translate counter error: %v", incrErr))
	}
	storeTikTokToolAICache(prepared.cacheKey, prepared.now, result)
	c.JSON(http.StatusOK, gin.H{"success": true, "data": result})
}

type tiktokBreakdownModelReply struct {
	Summary  string                   `json:"summary"`
	Sections []tiktokBreakdownSection `json:"sections"`
}

func parseAndValidateBreakdown(reply string, maxMs int) (tiktokBreakdownModelReply, error) {
	var parsed tiktokBreakdownModelReply
	if err := common.UnmarshalJsonStr(service.StripJSONFence(reply), &parsed); err != nil {
		return tiktokBreakdownModelReply{}, errors.New("invalid breakdown json")
	}
	parsed.Summary = strings.TrimSpace(parsed.Summary)
	if parsed.Summary == "" || len(parsed.Sections) < 2 || len(parsed.Sections) > 8 {
		return tiktokBreakdownModelReply{}, errors.New("incomplete breakdown structure")
	}
	normalized := make([]tiktokBreakdownSection, 0, len(parsed.Sections))
	for _, sec := range parsed.Sections {
		stage := strings.ToLower(strings.TrimSpace(sec.Stage))
		if _, known := tiktokToolBreakdownStages[stage]; !known {
			stage = "pacing"
		}
		label := strings.TrimSpace(sec.Label)
		quote := strings.TrimSpace(sec.Quote)
		analysis := strings.TrimSpace(sec.Analysis)
		if label == "" || quote == "" || analysis == "" {
			return tiktokBreakdownModelReply{}, errors.New("empty breakdown section fields")
		}
		startMs := max(0, sec.StartMs)
		endMs := max(startMs, sec.EndMs)
		if maxMs > 0 {
			startMs = min(startMs, maxMs)
			endMs = min(max(endMs, startMs), maxMs)
		}
		normalized = append(normalized, tiktokBreakdownSection{
			Stage:    stage,
			Label:    label,
			StartMs:  startMs,
			EndMs:    endMs,
			Quote:    quote,
			Analysis: analysis,
		})
	}
	parsed.Sections = normalized
	return parsed, nil
}

// BreakdownTikTokToolTranscript POST /api/tools/tiktok/breakdown
func BreakdownTikTokToolTranscript(c *gin.Context) {
	prepared, cachedPayload, ok := prepareTikTokToolAIRequest(c, "breakdown")
	if !ok {
		return
	}
	if cachedPayload != nil {
		c.JSON(http.StatusOK, gin.H{"success": true, "data": cachedPayload})
		return
	}

	sourceCues, encodedCues := tiktokToolAICuesInput(prepared.track.Cues)
	if len(sourceCues) == 0 || encodedCues == "" {
		tiktokToolFail(c, http.StatusBadRequest, "no_captions")
		return
	}

	maxDurationMs := prepared.durationMs
	if last := sourceCues[len(sourceCues)-1].EndMs; last > maxDurationMs {
		maxDurationMs = last
	}
	maxTimeHint := make([]map[string]any, 0, len(sourceCues))
	for i, cue := range sourceCues {
		maxTimeHint = append(maxTimeHint, map[string]any{
			"i":        i,
			"start_ms": cue.StartMs,
			"end_ms":   cue.EndMs,
			"text":     cue.Text,
		})
	}
	cuesWithMsJSON, err := common.Marshal(maxTimeHint)
	if err != nil {
		tiktokToolFail(c, http.StatusBadRequest, "no_captions")
		return
	}

	systemPrompt := "You are a senior short-video creative strategist. Break a timed transcript into clear narrative beats in strict JSON only: " +
		"{\"summary\":\"...\",\"sections\":[{\"stage\":\"hook|product_intro|demo_scene|proof_effect|authenticity|usage_detail|cta|pacing\",\"label\":\"...\",\"start_ms\":0,\"end_ms\":4000,\"quote\":\"...\",\"analysis\":\"...\"}]}. " +
		"Produce 2 to 8 chronological sections grounded strictly in the provided transcript quotes and timestamps. " +
		"Write summary, label, and analysis in the requested target language; keep quote in the original transcript words. Return raw JSON only with zero markdown fences."
	userPrompt := fmt.Sprintf("Output language for summary/label/analysis: %s (%s)\nVideo duration_ms: %d\nTimed transcript cues:\n%s",
		prepared.languageLabel, prepared.targetLang, maxDurationMs, string(cuesWithMsJSON))

	reply, err := service.TikTokToolAIChat(
		c.Request.Context(),
		prepared.settings.model,
		prepared.settings.group,
		systemPrompt,
		userPrompt,
		func(raw string) error {
			_, checkErr := parseAndValidateBreakdown(raw, maxDurationMs)
			return checkErr
		},
	)
	if err != nil {
		common.SysError(common.LogText("tiktok tool breakdown failed: %v", err))
		tiktokToolFail(c, http.StatusBadGateway, "upstream_failed")
		return
	}

	parsed, err := parseAndValidateBreakdown(reply, maxDurationMs)
	if err != nil {
		tiktokToolFail(c, http.StatusBadGateway, "upstream_failed")
		return
	}

	result := tiktokBreakdownResult{
		VideoID:    prepared.videoID,
		TrackID:    prepared.track.ID,
		TargetLang: prepared.targetLang,
		Summary:    parsed.Summary,
		Sections:   parsed.Sections,
	}
	if _, incrErr := tiktokToolCounters.incr(c.Request.Context(), prepared.aiQuotaKey, tiktokToolSecondsToMidnight(prepared.now)); incrErr != nil {
		common.SysError(common.LogText("tiktok tool breakdown counter error: %v", incrErr))
	}
	storeTikTokToolAICache(prepared.cacheKey, prepared.now, result)
	c.JSON(http.StatusOK, gin.H{"success": true, "data": result})
}
