package social

// fork: tiktok-webhooks — TikTok Content Posting webhook verification and
// idempotent monotonic status transitions.

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

const (
	TikTokWebhookRejectMissingHeader        = "missing_header"
	TikTokWebhookRejectMalformedSignature   = "malformed_signature"
	TikTokWebhookRejectBadHMAC              = "bad_hmac"
	TikTokWebhookRejectTimestampOutOfWindow = "timestamp_out_of_window"
	TikTokWebhookRejectClientKeyMismatch    = "client_key_mismatch"

	TikTokWebhookEventInboxDelivered    = "post.publish.inbox_delivered"
	TikTokWebhookEventComplete          = "post.publish.complete"
	TikTokWebhookEventFailed            = "post.publish.failed"
	TikTokWebhookEventPubliclyAvailable = "post.publish.publicly_available"
	// TikTok docs use the single-"l" spelling "publicaly"; accept both spellings defensively.
	TikTokWebhookEventNoLongerPublicaly = "post.publish.no_longer_publicaly_available"
	TikTokWebhookEventNoLongerPublicly  = "post.publish.no_longer_publicly_available"

	tikTokWebhookMaxFutureSkewSeconds int64 = 60
)

var (
	ErrTikTokWebhookDisabled   = errors.New("tiktok webhook is disabled")
	ErrTikTokWebhookBadPayload = errors.New("invalid tiktok webhook payload")
)

// TikTokWebhookAuthError reports a verification failure. HTTP callers MUST return
// a uniform 401 response without reflecting secrets, headers, or payload bytes,
// while logging Reason (and safe numeric age/tolerance for window rejections).
type TikTokWebhookAuthError struct {
	Reason           string
	SigAgeSeconds    int64
	ToleranceSeconds int64
}

func (e *TikTokWebhookAuthError) Error() string {
	if e == nil {
		return "tiktok webhook unauthorized"
	}
	return "tiktok webhook unauthorized: " + e.Reason
}

// TikTokWebhookResult summarizes how a verified webhook delivery was handled.
type TikTokWebhookResult struct {
	Event     string
	PublishID string
	Action    string
	Status    string
}

type tikTokWebhookEnvelope struct {
	ClientKey  string            `json:"client_key"`
	Event      string            `json:"event"`
	CreateTime int64             `json:"create_time"`
	UserOpenID string            `json:"user_openid"`
	Content    common.RawMessage `json:"content"`
}

// tikTokWebhookPostID preserves 64-bit post_id values sent as either JSON strings or numbers.
type tikTokWebhookPostID string

func (id *tikTokWebhookPostID) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		*id = ""
		return nil
	}
	if trimmed[0] == '"' {
		var s string
		if err := common.Unmarshal(trimmed, &s); err != nil {
			return err
		}
		*id = tikTokWebhookPostID(strings.TrimSpace(s))
		return nil
	}
	var num json.Number
	if err := common.Unmarshal(trimmed, &num); err != nil {
		return err
	}
	*id = tikTokWebhookPostID(strings.TrimSpace(num.String()))
	return nil
}

type tikTokPublishEventContent struct {
	PublishID   string              `json:"publish_id"`
	PublishType string              `json:"publish_type"`
	Reason      string              `json:"reason"`
	PostID      tikTokWebhookPostID `json:"post_id"`
}

// TikTokWebhookReady reports whether the webhook endpoint is enabled and configured.
func (s *Service) TikTokWebhookReady() bool {
	return s != nil && s.cfg.TikTokWebhookActive()
}

// HandleTikTokWebhook verifies TikTok-Signature, timestamp window, and client_key,
// then applies Content Posting status transitions monotonically and idempotently.
func (s *Service) HandleTikTokWebhook(ctx context.Context, sigHeaders []string, rawBody []byte) (*TikTokWebhookResult, error) {
	return s.handleTikTokWebhookAt(ctx, sigHeaders, rawBody, time.Now())
}

func (s *Service) handleTikTokWebhookAt(ctx context.Context, sigHeaders []string, rawBody []byte, now time.Time) (*TikTokWebhookResult, error) {
	if !s.TikTokWebhookReady() {
		return nil, ErrTikTokWebhookDisabled
	}
	expectedClientKey, clientSecret := s.cfg.TikTokCredentials()
	if expectedClientKey == "" || clientSecret == "" {
		return nil, ErrTikTokWebhookDisabled
	}

	tStr, ts, sigBytes, authErr := parseTikTokSignatureHeader(sigHeaders)
	if authErr != nil {
		return nil, authErr
	}

	// Verify HMAC-SHA256 over "<t>.<raw_body>" before checking timestamp or parsing JSON.
	mac := hmac.New(sha256.New, []byte(clientSecret))
	mac.Write([]byte(tStr))
	mac.Write([]byte{'.'})
	mac.Write(rawBody)
	if !hmac.Equal(sigBytes, mac.Sum(nil)) {
		return nil, &TikTokWebhookAuthError{Reason: TikTokWebhookRejectBadHMAC}
	}

	// Check replay window: past <= tolerance, future <= 60s.
	toleranceSeconds := int64(s.cfg.TikTokWebhookTolerance() / time.Second)
	ageSeconds := now.Unix() - ts
	if ageSeconds > toleranceSeconds || ageSeconds < -tikTokWebhookMaxFutureSkewSeconds {
		return nil, &TikTokWebhookAuthError{
			Reason:           TikTokWebhookRejectTimestampOutOfWindow,
			SigAgeSeconds:    ageSeconds,
			ToleranceSeconds: toleranceSeconds,
		}
	}

	// Parse JSON envelope only after HMAC + window pass.
	trimmedBody := bytes.TrimSpace(rawBody)
	if len(trimmedBody) == 0 || trimmedBody[0] != '{' {
		return nil, ErrTikTokWebhookBadPayload
	}
	var env tikTokWebhookEnvelope
	if err := common.Unmarshal(trimmedBody, &env); err != nil {
		return nil, ErrTikTokWebhookBadPayload
	}

	// Constant-time top-level client_key binding check.
	if subtle.ConstantTimeCompare([]byte(env.ClientKey), []byte(expectedClientKey)) != 1 {
		return nil, &TikTokWebhookAuthError{Reason: TikTokWebhookRejectClientKeyMismatch}
	}

	eventName := strings.TrimSpace(env.Event)
	if !isTikTokPublishWebhookEvent(eventName) {
		return &TikTokWebhookResult{
			Event:  eventName,
			Action: "ignored_event",
		}, nil
	}

	content, err := parseTikTokPublishEventContent(env.Content)
	if err != nil {
		return nil, ErrTikTokWebhookBadPayload
	}

	publishID := strings.TrimSpace(content.PublishID)
	if publishID == "" {
		logger.LogWarn(ctx, common.LogText("TikTok webhook ignored reason=missing_publish_id event=%s", eventName))
		return &TikTokWebhookResult{
			Event:  eventName,
			Action: "ignored_missing_publish_id",
		}, nil
	}

	post, err := model.GetSocialPostByProviderPostID(ProviderTikTokDirect, publishID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			logger.LogWarn(ctx, common.LogText("TikTok webhook ignored reason=unknown_publish_id event=%s publish_id=%s", eventName, publishID))
			return &TikTokWebhookResult{
				Event:     eventName,
				PublishID: publishID,
				Action:    "ignored_unknown_post",
			}, nil
		}
		logger.LogWarn(ctx, common.LogText("TikTok webhook processing failed event=%s publish_id=%s error=%q", eventName, publishID, err.Error()))
		return nil, err
	}

	if eventOpenID := strings.TrimSpace(env.UserOpenID); eventOpenID != "" {
		localOpenID, openErr := resolvePostTikTokOpenID(post)
		if openErr != nil {
			logger.LogWarn(ctx, common.LogText("TikTok webhook processing failed event=%s publish_id=%s error=%q", eventName, publishID, openErr.Error()))
			return nil, openErr
		}
		if localOpenID != "" && subtle.ConstantTimeCompare([]byte(eventOpenID), []byte(localOpenID)) != 1 {
			logger.LogWarn(ctx, common.LogText("TikTok webhook ignored reason=user_openid_mismatch event=%s publish_id=%s", eventName, publishID))
			return &TikTokWebhookResult{
				Event:     eventName,
				PublishID: publishID,
				Action:    "ignored_openid_mismatch",
				Status:    post.Status,
			}, nil
		}
	}

	upd := buildTikTokWebhookStateUpdate(eventName, env.CreateTime, publishID, content)
	updated, finalPost, err := model.ApplySocialPostStateUpdateByID(post.ID, upd)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			logger.LogWarn(ctx, common.LogText("TikTok webhook ignored reason=unknown_publish_id event=%s publish_id=%s", eventName, publishID))
			return &TikTokWebhookResult{
				Event:     eventName,
				PublishID: publishID,
				Action:    "ignored_unknown_post",
			}, nil
		}
		logger.LogWarn(ctx, common.LogText("TikTok webhook processing failed event=%s publish_id=%s error=%q", eventName, publishID, err.Error()))
		return nil, err
	}

	action := "noop"
	if updated {
		action = "updated"
	}
	return &TikTokWebhookResult{
		Event:     eventName,
		PublishID: publishID,
		Action:    action,
		Status:    finalPost.Status,
	}, nil
}

func parseTikTokSignatureHeader(headers []string) (tStr string, ts int64, sigBytes []byte, authErr *TikTokWebhookAuthError) {
	if len(headers) == 0 {
		return "", 0, nil, &TikTokWebhookAuthError{Reason: TikTokWebhookRejectMissingHeader}
	}
	if len(headers) > 1 {
		return "", 0, nil, &TikTokWebhookAuthError{Reason: TikTokWebhookRejectMalformedSignature}
	}
	raw := strings.TrimSpace(headers[0])
	if raw == "" {
		return "", 0, nil, &TikTokWebhookAuthError{Reason: TikTokWebhookRejectMissingHeader}
	}

	var (
		hasT, hasS bool
		sStr       string
	)
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			return "", 0, nil, &TikTokWebhookAuthError{Reason: TikTokWebhookRejectMalformedSignature}
		}
		k, v, ok := strings.Cut(part, "=")
		if !ok {
			return "", 0, nil, &TikTokWebhookAuthError{Reason: TikTokWebhookRejectMalformedSignature}
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		if k == "" || v == "" {
			return "", 0, nil, &TikTokWebhookAuthError{Reason: TikTokWebhookRejectMalformedSignature}
		}
		switch k {
		case "t":
			if hasT {
				return "", 0, nil, &TikTokWebhookAuthError{Reason: TikTokWebhookRejectMalformedSignature}
			}
			hasT = true
			tStr = v
		case "s":
			if hasS {
				return "", 0, nil, &TikTokWebhookAuthError{Reason: TikTokWebhookRejectMalformedSignature}
			}
			hasS = true
			sStr = v
		default:
			return "", 0, nil, &TikTokWebhookAuthError{Reason: TikTokWebhookRejectMalformedSignature}
		}
	}
	if !hasT || !hasS {
		return "", 0, nil, &TikTokWebhookAuthError{Reason: TikTokWebhookRejectMalformedSignature}
	}
	for i := 0; i < len(tStr); i++ {
		if tStr[i] < '0' || tStr[i] > '9' {
			return "", 0, nil, &TikTokWebhookAuthError{Reason: TikTokWebhookRejectMalformedSignature}
		}
	}
	parsedTS, err := strconv.ParseInt(tStr, 10, 64)
	if err != nil || parsedTS <= 0 {
		return "", 0, nil, &TikTokWebhookAuthError{Reason: TikTokWebhookRejectMalformedSignature}
	}
	if len(sStr) != sha256.Size*2 {
		return "", 0, nil, &TikTokWebhookAuthError{Reason: TikTokWebhookRejectMalformedSignature}
	}
	decoded, err := hex.DecodeString(sStr)
	if err != nil || len(decoded) != sha256.Size {
		return "", 0, nil, &TikTokWebhookAuthError{Reason: TikTokWebhookRejectMalformedSignature}
	}
	return tStr, parsedTS, decoded, nil
}

func isTikTokPublishWebhookEvent(event string) bool {
	switch event {
	case TikTokWebhookEventInboxDelivered,
		TikTokWebhookEventComplete,
		TikTokWebhookEventFailed,
		TikTokWebhookEventPubliclyAvailable,
		TikTokWebhookEventNoLongerPublicaly,
		TikTokWebhookEventNoLongerPublicly:
		return true
	default:
		return false
	}
}

func parseTikTokPublishEventContent(raw common.RawMessage) (*tikTokPublishEventContent, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil, ErrTikTokWebhookBadPayload
	}
	var payloadBytes []byte
	switch trimmed[0] {
	case '"':
		var serialized string
		if err := common.Unmarshal(trimmed, &serialized); err != nil {
			return nil, ErrTikTokWebhookBadPayload
		}
		payloadBytes = bytes.TrimSpace([]byte(serialized))
		if len(payloadBytes) == 0 || payloadBytes[0] != '{' {
			return nil, ErrTikTokWebhookBadPayload
		}
	case '{':
		payloadBytes = trimmed
	default:
		return nil, ErrTikTokWebhookBadPayload
	}
	var content tikTokPublishEventContent
	if err := common.Unmarshal(payloadBytes, &content); err != nil {
		return nil, ErrTikTokWebhookBadPayload
	}
	return &content, nil
}

func resolvePostTikTokOpenID(post *model.SocialPost) (string, error) {
	if post == nil || strings.TrimSpace(post.RequestSnapshot) == "" {
		return "", nil
	}
	var snap struct {
		AccountIDs []int64 `json:"account_ids"`
	}
	if err := common.Unmarshal([]byte(post.RequestSnapshot), &snap); err != nil || len(snap.AccountIDs) == 0 {
		return "", nil
	}
	acc, err := model.GetSocialAccountByIDForUser(snap.AccountIDs[0], post.UserId)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", nil
		}
		return "", err
	}
	if acc == nil || acc.Provider != ProviderTikTokDirect {
		return "", nil
	}
	return strings.TrimSpace(acc.ProviderAccountID), nil
}

func buildTikTokWebhookStateUpdate(eventName string, createTime int64, publishID string, content *tikTokPublishEventContent) model.SocialPostStateUpdate {
	fields := map[string]any{
		"publish_id": publishID,
	}
	if pt := strings.TrimSpace(content.PublishType); pt != "" {
		fields["publish_type"] = pt
	}
	switch eventName {
	case TikTokWebhookEventInboxDelivered:
		fields["status"] = "SEND_TO_USER_INBOX"
		return model.SocialPostStateUpdate{
			TargetStatus:   model.SocialPostStatusSentToInbox,
			EventName:      eventName,
			EventTime:      createTime,
			SnapshotFields: fields,
		}
	case TikTokWebhookEventComplete:
		fields["status"] = "PUBLISH_COMPLETE"
		fields["publish_complete"] = true
		return model.SocialPostStateUpdate{
			TargetStatus:                model.SocialPostStatusPublished,
			EventName:                   eventName,
			EventTime:                   createTime,
			SnapshotFields:              fields,
			AllowPublishedSnapshotMerge: true,
		}
	case TikTokWebhookEventPubliclyAvailable:
		fields["status"] = "PUBLISH_COMPLETE"
		return model.SocialPostStateUpdate{
			TargetStatus:                model.SocialPostStatusPublished,
			EventName:                   eventName,
			EventTime:                   createTime,
			SnapshotFields:              fields,
			AddPostID:                   strings.TrimSpace(string(content.PostID)),
			AllowPublishedSnapshotMerge: true,
		}
	case TikTokWebhookEventFailed:
		reason := strings.TrimSpace(content.Reason)
		fields["status"] = "FAILED"
		fields["fail_reason"] = reason
		fields["reason"] = reason
		return model.SocialPostStateUpdate{
			TargetStatus:   model.SocialPostStatusFailed,
			EventName:      eventName,
			EventTime:      createTime,
			SnapshotFields: fields,
		}
	case TikTokWebhookEventNoLongerPublicaly, TikTokWebhookEventNoLongerPublicly:
		return model.SocialPostStateUpdate{
			Takedown:       true,
			EventName:      eventName,
			EventTime:      createTime,
			SnapshotFields: fields,
			RemovePostID:   strings.TrimSpace(string(content.PostID)),
		}
	default:
		return model.SocialPostStateUpdate{}
	}
}
