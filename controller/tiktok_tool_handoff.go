package controller

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
)

// fork: tool-to-workbench — hand one parsed TikTok video from the tool page to
// the workbench's viral-recreate slot.
//
//   - Issue (POST /api/tools/tiktok/handoff) needs the tool pass and a post this
//     process resolved in the last 30 minutes. It stores {work id, owner} in
//     Redis under the hash of a random token; the token carries no link, id or
//     media address. Guests are bound to their /24 (/48) network and user agent,
//     signed-in visitors to their user id.
//   - Redeem (POST /api/tools/tiktok/handoff/redeem) needs same-origin, a
//     session and the same owner; it is rate limited per user and per IP, runs
//     one transfer per user at a time across instances (a Redis lock with a
//     TTL), and is single use (an atomic compare-and-delete; a refused owner
//     check leaves the token). It re-reads the post by work id (never the
//     page's 10-minute signed media path), copies the video through the
//     SSRF-protected transfer into the redeemer's own upload library, and
//     returns that upload. Tokens exist only in Redis with a TTL.

const (
	tiktokHandoffTTL         = 30 * time.Minute
	tiktokHandoffKeyPrefix   = "tiktoktool:v1:handoff:"
	tiktokHandoffIssueWindow = time.Minute
	tiktokHandoffIssueLimit  = 10

	// Redeem attempts per signed-in user and per client IP in one window,
	// counted before the token is read so refused attempts never touch it.
	tiktokHandoffRedeemWindow    = 10 * time.Minute
	tiktokHandoffRedeemUserLimit = 6
	tiktokHandoffRedeemIPLimit   = 20

	// One transfer per user at a time across instances. The lock outlives the
	// transfer's own 90-second timeout plus storing, and expires on its own if
	// an instance dies mid-transfer.
	tiktokHandoffLockPrefix = "tiktoktool:v1:handoff-lock:user:"
	tiktokHandoffLockTTL    = 3 * time.Minute
)

// tiktokHandoffCompareAndDelete deletes KEYS[1] only while it still holds
// ARGV[1], in one Redis step. It consumes a token (exactly one concurrent
// redeemer gets 1) and releases a lock only by its holder.
var tiktokHandoffCompareAndDelete = redis.NewScript(`if redis.call('GET', KEYS[1]) == ARGV[1] then return redis.call('DEL', KEYS[1]) end return 0`)

// tiktokHandoffBeforeConsume is a test seam between the owner check and the
// atomic consume; production leaves it nil.
var tiktokHandoffBeforeConsume func()

var (
	tiktokHandoffWorkID = regexp.MustCompile(`^[0-9]{1,32}$`)
	tiktokHandoffToken  = regexp.MustCompile(`^[A-Za-z0-9_-]{43}$`)
)

type tiktokHandoffRecord struct {
	WorkID  string `json:"work_id"`
	UserID  int    `json:"user_id,omitempty"`
	Network string `json:"network,omitempty"`
	Agent   string `json:"agent,omitempty"`
}

// tiktokHandoffTransfer and tiktokHandoffStore are swapped by tests; production
// uses the SSRF-protected transfer and the native upload record (owner, quota,
// isolated public host, expiry sweep).
var (
	tiktokHandoffTransfer = service.TransferTikTokHandoffVideo
	tiktokHandoffStore    = func(c *gin.Context, userID int, data []byte, name string, media *service.InfluencerVideoMetadata) (*dto.FileInfoData, error) {
		return saveAndRecordFile(c, userID, data, "video/mp4", name, name, "", true, media)
	}
)

func tiktokHandoffRedis() bool {
	return common.RedisEnabled && common.RDB != nil
}

func tiktokHandoffKey(token string) string {
	sum := sha256.Sum256([]byte(token))
	return tiktokHandoffKeyPrefix + hex.EncodeToString(sum[:])
}

func tiktokHandoffCachedVideo(workID string, now time.Time) (tiktokPost, bool) {
	tiktokToolCache.Lock()
	cached, hit := tiktokToolCache.entries[workID]
	tiktokToolCache.Unlock()
	if !hit || now.After(cached.expires) || cached.body.kind != "video" || cached.body.videoURL == "" {
		return tiktokPost{}, false
	}
	return cached.body, true
}

func tiktokHandoffDurationAllowed(post tiktokPost) bool {
	return service.TikTokHandoffDurationAllowed(float64(post.durationMs) / 1000)
}

// IssueTikTokToolHandoff POST /api/tools/tiktok/handoff
func IssueTikTokToolHandoff(c *gin.Context) {
	settings := tiktokToolConfig()
	ctx := c.Request.Context()
	now := time.Now()
	c.Header("Cache-Control", "no-store")
	if !settings.enabled || settings.channelID <= 0 || !tiktokHandoffRedis() {
		tiktokToolFail(c, http.StatusServiceUnavailable, "handoff_unavailable")
		return
	}
	if !tiktokToolSameOrigin(c, settings) {
		tiktokToolFail(c, http.StatusForbidden, "forbidden")
		return
	}
	if !tiktokToolPassValid(c, now) {
		tiktokToolFail(c, http.StatusForbidden, "verification_required")
		return
	}
	var body struct {
		ID string `json:"id"`
	}
	if err := common.DecodeJson(io.LimitReader(c.Request.Body, 1<<10), &body); err != nil || !tiktokHandoffWorkID.MatchString(body.ID) {
		tiktokToolFail(c, http.StatusBadRequest, "invalid_request")
		return
	}
	issued, err := tiktokToolCounters.incr(ctx, "tiktoktool:v1:handoff-issue:"+tiktokToolClientIP(c), tiktokHandoffIssueWindow)
	if err != nil {
		tiktokToolFail(c, http.StatusServiceUnavailable, "busy")
		return
	}
	if issued > tiktokHandoffIssueLimit {
		tiktokToolFail(c, http.StatusTooManyRequests, "rate_limited")
		return
	}
	post, ok := tiktokHandoffCachedVideo(body.ID, now)
	if !ok {
		tiktokToolFail(c, http.StatusGone, "handoff_expired")
		return
	}
	if !tiktokHandoffDurationAllowed(post) {
		tiktokToolFail(c, http.StatusUnprocessableEntity, "handoff_duration")
		return
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		tiktokToolFail(c, http.StatusServiceUnavailable, "handoff_unavailable")
		return
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	record := tiktokHandoffRecord{WorkID: post.id}
	if userID := c.GetInt("id"); userID > 0 {
		record.UserID = userID
	} else {
		record.Network, record.Agent = tiktokToolNetwork(tiktokToolClientIP(c)), tiktokToolAgent(c)
	}
	value, err := common.Marshal(record)
	if err != nil {
		tiktokToolFail(c, http.StatusServiceUnavailable, "handoff_unavailable")
		return
	}
	if err := common.RDB.Set(ctx, tiktokHandoffKey(token), value, tiktokHandoffTTL).Err(); err != nil {
		tiktokToolFail(c, http.StatusServiceUnavailable, "handoff_unavailable")
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{
		"token": token, "expires_in": int(tiktokHandoffTTL.Seconds()),
	}})
}

// tiktokHandoffOwns reports whether this request belongs to the token's owner.
// A guest token stays redeemable by the same browser after it signs in.
func tiktokHandoffOwns(c *gin.Context, record tiktokHandoffRecord, userID int) bool {
	if record.UserID > 0 {
		return record.UserID == userID
	}
	return record.Network != "" && record.Network == tiktokToolNetwork(tiktokToolClientIP(c)) && record.Agent == tiktokToolAgent(c)
}

// RedeemTikTokToolHandoff POST /api/tools/tiktok/handoff/redeem
func RedeemTikTokToolHandoff(c *gin.Context) {
	settings := tiktokToolConfig()
	ctx := c.Request.Context()
	c.Header("Cache-Control", "no-store")
	if !settings.enabled || settings.channelID <= 0 || !tiktokHandoffRedis() {
		tiktokToolFail(c, http.StatusServiceUnavailable, "handoff_unavailable")
		return
	}
	if !tiktokToolSameOrigin(c, settings) {
		tiktokToolFail(c, http.StatusForbidden, "forbidden")
		return
	}
	var body struct {
		Token string `json:"token"`
	}
	if err := common.DecodeJson(io.LimitReader(c.Request.Body, 1<<10), &body); err != nil || !tiktokHandoffToken.MatchString(body.Token) {
		tiktokToolFail(c, http.StatusBadRequest, "invalid_request")
		return
	}
	userID := c.GetInt("id")
	if userID <= 0 {
		// The token stays: the same tab redeems it after signing in.
		tiktokToolFail(c, http.StatusUnauthorized, "login_required")
		return
	}
	if limited, err := tiktokHandoffRedeemLimited(ctx, userID, tiktokToolClientIP(c)); err != nil {
		tiktokToolFail(c, http.StatusServiceUnavailable, "busy")
		return
	} else if limited {
		tiktokToolFail(c, http.StatusTooManyRequests, "rate_limited")
		return
	}
	key := tiktokHandoffKey(body.Token)
	raw, err := common.RDB.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		tiktokToolFail(c, http.StatusGone, "handoff_expired")
		return
	}
	if err != nil {
		tiktokToolFail(c, http.StatusServiceUnavailable, "handoff_unavailable")
		return
	}
	var record tiktokHandoffRecord
	if err := common.UnmarshalJsonStr(raw, &record); err != nil || !tiktokHandoffWorkID.MatchString(record.WorkID) {
		_ = common.RDB.Del(ctx, key).Err()
		tiktokToolFail(c, http.StatusGone, "handoff_expired")
		return
	}
	if !tiktokHandoffOwns(c, record, userID) {
		tiktokToolFail(c, http.StatusForbidden, "handoff_forbidden")
		return
	}
	release, held, err := tiktokHandoffLock(ctx, userID)
	if err != nil {
		tiktokToolFail(c, http.StatusServiceUnavailable, "handoff_unavailable")
		return
	}
	if !held {
		tiktokToolFail(c, http.StatusTooManyRequests, "busy")
		return
	}
	defer release()
	if tiktokHandoffBeforeConsume != nil {
		tiktokHandoffBeforeConsume()
	}
	// Single use: the record is deleted only if it is still the one read above,
	// atomically, and only the request that deleted it may transfer.
	consumed, err := tiktokHandoffCompareAndDelete.Run(ctx, common.RDB, []string{key}, raw).Int()
	if err != nil {
		tiktokToolFail(c, http.StatusServiceUnavailable, "handoff_unavailable")
		return
	}
	if consumed != 1 {
		tiktokToolFail(c, http.StatusGone, "handoff_expired")
		return
	}
	post, ok := tiktokHandoffCachedVideo(record.WorkID, time.Now())
	if !ok {
		fetched, _, err := tiktokPostUpstream(ctx, settings.channelID, record.WorkID)
		if err != nil || fetched.kind != "video" || fetched.videoURL == "" {
			tiktokToolFail(c, http.StatusGone, "handoff_expired")
			return
		}
		post = fetched
	}
	if !tiktokHandoffDurationAllowed(post) {
		tiktokToolFail(c, http.StatusUnprocessableEntity, "handoff_duration")
		return
	}
	data, media, err := tiktokHandoffTransfer(ctx, post.videoURL)
	switch {
	case errors.Is(err, service.ErrTikTokHandoffDuration):
		tiktokToolFail(c, http.StatusUnprocessableEntity, "handoff_duration")
		return
	case errors.Is(err, service.ErrTikTokHandoffTooLarge):
		tiktokToolFail(c, http.StatusRequestEntityTooLarge, "handoff_too_large")
		return
	case err != nil:
		common.SysError(common.LogText("tiktok tool handoff transfer failed: %v", err))
		tiktokToolFail(c, http.StatusBadGateway, "handoff_unavailable")
		return
	}
	name := "tiktok-" + post.id + ".mp4"
	info, err := tiktokHandoffStore(c, userID, data, name, &media)
	if err != nil {
		if strings.Contains(err.Error(), "配额已耗尽") {
			tiktokToolFail(c, http.StatusTooManyRequests, "storage_quota")
			return
		}
		common.SysError(common.LogText("tiktok tool handoff store failed: %v", err))
		tiktokToolFail(c, http.StatusServiceUnavailable, "handoff_unavailable")
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{
		"url": info.FileURL, "preview_url": info.FileURL, "name": name, "metadata": media,
	}})
}

// tiktokHandoffRedeemLimited counts one redeem attempt for the user and for
// the client IP in Redis and reports whether either is over its limit.
func tiktokHandoffRedeemLimited(ctx context.Context, userID int, ip string) (bool, error) {
	byUser, err := tiktokToolCounters.incr(ctx, "tiktoktool:v1:handoff-redeem:user:"+strconv.Itoa(userID), tiktokHandoffRedeemWindow)
	if err != nil {
		return false, err
	}
	byIP, err := tiktokToolCounters.incr(ctx, "tiktoktool:v1:handoff-redeem:ip:"+ip, tiktokHandoffRedeemWindow)
	if err != nil {
		return false, err
	}
	return byUser > tiktokHandoffRedeemUserLimit || byIP > tiktokHandoffRedeemIPLimit, nil
}

// tiktokHandoffLock takes the user's cross-instance transfer lock. held is
// false while another transfer for the user runs; release frees only this
// holder's lock, even after the request context ends.
func tiktokHandoffLock(ctx context.Context, userID int) (release func(), held bool, err error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return nil, false, err
	}
	key, holder := tiktokHandoffLockPrefix+strconv.Itoa(userID), hex.EncodeToString(raw)
	held, err = common.RDB.SetNX(ctx, key, holder, tiktokHandoffLockTTL).Result()
	if err != nil || !held {
		return nil, false, err
	}
	return func() {
		releaseCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = tiktokHandoffCompareAndDelete.Run(releaseCtx, common.RDB, []string{key}, holder).Err()
	}, true, nil
}
