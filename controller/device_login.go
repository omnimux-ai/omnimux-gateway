package controller

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	deviceLoginExpiresIn     = 900
	deviceLoginUserCodeChars = "BCDFGHJKLMNPQRSTVWXZ23456789"
	deviceLoginUserCodeLen   = 8
	deviceLoginGrantType     = "urn:ietf:params:oauth:grant-type:device_code"
)

// deviceLoginInterval is the default poll interval in seconds (RFC 8628).
// Tests may lower it to avoid multi-second sleeps.
var deviceLoginInterval = 5

type deviceLoginDevicePayload struct {
	Interval              int                              `json:"interval"`
	LastPollAt            int64                            `json:"last_poll_at,omitempty"`
	ClientName            string                           `json:"client_name,omitempty"`
	Authorization         *model.AuthFlowAuthorization     `json:"authorization,omitempty"`
	DeviceFlowID          int64                            `json:"device_flow_id,omitempty"`
	AccessTokenGeneration *int64                           `json:"access_token_generation,omitempty"`
	Grant                 *service.CliDeviceApproveContext `json:"grant,omitempty"`
}

type deviceLoginUserPayload struct {
	DeviceFlowID int64 `json:"device_flow_id"`
}

type deviceCodeRequest struct {
	ClientName string  `json:"client_name"`
	UserCode   *string `json:"user_code"`
}

type deviceTokenRequest struct {
	DeviceCode string `json:"device_code"`
	GrantType  string `json:"grant_type"`
}

type deviceUserCodeRequest struct {
	UserCode string `json:"user_code"`
}

// CreateDeviceLoginCode issues a device_code + user_code pair for CLI login.
func CreateDeviceLoginCode(c *gin.Context) {
	setAuthNoStore(c)
	var req deviceCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid request"})
		return
	}
	if req.UserCode != nil {
		_, _, grant, err := lookupDeviceLoginGrant(*req.UserCode)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": "invalid or expired user code"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{
			"verification_scope":   service.VerificationScopeCliDeviceApprove,
			"verification_context": grant,
		}})
		return
	}

	expiresAt := time.Now().Add(time.Duration(deviceLoginExpiresIn) * time.Second)
	devicePayload, err := common.Marshal(deviceLoginDevicePayload{
		Interval:   deviceLoginInterval,
		ClientName: strings.TrimSpace(req.ClientName),
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}

	deviceToken, deviceFlow, err := model.CreateAuthFlow(model.AuthFlowCreate{
		Purpose:   model.AuthFlowPurposeCliDevice,
		Intent:    model.AuthFlowIntentPending,
		Payload:   string(devicePayload),
		ExpiresAt: expiresAt,
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}

	userCode, userCodeDisplay, err := createDeviceUserCodeFlow(deviceFlow.Id, expiresAt)
	if err != nil {
		_ = model.DB.Delete(&model.AuthFlow{}, deviceFlow.Id).Error
		common.ApiError(c, err)
		return
	}

	grant := &service.CliDeviceApproveContext{
		DeviceFlowID: deviceFlow.Id, UserCode: userCode, Scopes: deviceLoginScopes(),
		ExpiresAt: time.Now().Unix() + int64(service.AccessTokenDefaultExpiryDays)*24*60*60,
	}
	var pending deviceLoginDevicePayload
	if err := common.UnmarshalJsonStr(deviceFlow.Payload, &pending); err != nil {
		common.ApiError(c, err)
		return
	}
	pending.Grant = grant
	encoded, err := common.Marshal(pending)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.UpdateAuthFlowFields(deviceFlow.Id, map[string]interface{}{"payload": string(encoded)}); err != nil {
		common.ApiError(c, err)
		return
	}
	verificationURI := deviceLoginVerificationURI(c)
	complete := verificationURI + "?user_code=" + url.QueryEscape(userCodeDisplay)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"device_code":               deviceToken,
			"user_code":                 userCodeDisplay,
			"verification_uri":          verificationURI,
			"verification_uri_complete": complete,
			"expires_in":                deviceLoginExpiresIn,
			"interval":                  deviceLoginInterval,
			"verification_scope":        service.VerificationScopeCliDeviceApprove,
			"verification_context":      grant,
		},
	})
}

// PollDeviceLoginToken exchanges an approved device_code for a scoped PAT.
func PollDeviceLoginToken(c *gin.Context) {
	setAuthNoStore(c)
	var req deviceTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.DeviceCode) == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"code":    "invalid_grant",
			"message": "device_code is required",
		})
		return
	}
	if grant := strings.TrimSpace(req.GrantType); grant != "" && grant != deviceLoginGrantType {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"code":    "invalid_grant",
			"message": "unsupported grant_type",
		})
		return
	}

	deviceCode := strings.TrimSpace(req.DeviceCode)
	flow, err := model.GetAuthFlow(deviceCode, model.AuthFlowMatch{
		Purpose: model.AuthFlowPurposeCliDevice,
	})
	if err != nil {
		code := "invalid_grant"
		if errors.Is(err, model.ErrAuthFlowExpired) {
			code = "expired_token"
		}
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"code":    code,
			"message": "device code is not usable",
		})
		return
	}

	var payload deviceLoginDevicePayload
	if flow.Payload != "" {
		_ = common.UnmarshalJsonStr(flow.Payload, &payload)
	}
	if payload.Interval <= 0 {
		payload.Interval = deviceLoginInterval
	}

	now := time.Now()
	if payload.LastPollAt > 0 {
		elapsed := now.Unix() - payload.LastPollAt
		if elapsed < int64(payload.Interval) {
			c.JSON(http.StatusOK, gin.H{
				"success":  false,
				"code":     "slow_down",
				"message":  "polling too frequently",
				"interval": payload.Interval,
			})
			return
		}
	}
	payload.LastPollAt = now.Unix()
	if payloadBytes, marshalErr := common.Marshal(payload); marshalErr == nil && (flow.Intent == model.AuthFlowIntentPending || flow.Intent == "") {
		// A pending poll must never overwrite a concurrent approval's authorization.
		if err := model.DB.Model(&model.AuthFlow{}).
			Where("id = ? AND payload = ? AND consumed_at IS NULL AND expires_at > ?", flow.Id, flow.Payload, now).
			Where("intent = ? OR intent = ''", model.AuthFlowIntentPending).
			Update("payload", string(payloadBytes)).Error; err != nil {
			common.ApiError(c, err)
			return
		}
	}

	switch flow.Intent {
	case model.AuthFlowIntentPending, "":
		c.JSON(http.StatusOK, gin.H{
			"success":  false,
			"code":     "authorization_pending",
			"message":  "authorization pending",
			"interval": payload.Interval,
		})
		return
	case model.AuthFlowIntentDenied:
		_, _ = model.ConsumeAuthFlow(deviceCode, model.AuthFlowMatch{
			Purpose: model.AuthFlowPurposeCliDevice,
		})
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"code":    "access_denied",
			"message": "authorization denied",
		})
		return
	case model.AuthFlowIntentApproved:
		// continue below
	default:
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"code":    "invalid_grant",
			"message": "device code is not usable",
		})
		return
	}

	if flow.UserId <= 0 {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"code":    "invalid_grant",
			"message": "device code is not usable",
		})
		return
	}

	operation, grantErr := deviceLoginOperation(flow, payload.Grant)
	if grantErr != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "code": "invalid_grant", "message": "device code is not usable"})
		return
	}
	if payload.Authorization == nil || payload.DeviceFlowID != flow.Id || payload.AccessTokenGeneration == nil ||
		payload.Authorization.UserID != flow.UserId || payload.Authorization.SessionID != flow.SessionId ||
		strings.HasPrefix(payload.Authorization.SessionID, model.AccessTokenSessionPrefix) ||
		service.ValidateFlowAuthorization(payload.Authorization.AuthSessionIdentity, operation, payload.Authorization) != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "code": "invalid_grant", "message": "device code is not usable"})
		return
	}
	userID := flow.UserId
	var issuedPAT string
	var username string
	_, err = model.ConsumeAuthFlowWithAction(deviceCode, model.AuthFlowMatch{
		Purpose:   model.AuthFlowPurposeCliDevice,
		UserId:    userID,
		Intent:    model.AuthFlowIntentApproved,
		SessionId: flow.SessionId,
	}, func(tx *gorm.DB, consumed *model.AuthFlow) error {
		var fresh deviceLoginDevicePayload
		if common.UnmarshalJsonStr(consumed.Payload, &fresh) != nil || fresh.Authorization == nil || fresh.AccessTokenGeneration == nil ||
			fresh.DeviceFlowID != consumed.Id || *fresh.Authorization != *payload.Authorization ||
			fresh.Grant == nil || !slices.Equal(fresh.Grant.Scopes, payload.Grant.Scopes) ||
			fresh.Grant.DeviceFlowID != payload.Grant.DeviceFlowID || fresh.Grant.UserCode != payload.Grant.UserCode ||
			fresh.Grant.ExpiresAt != payload.Grant.ExpiresAt ||
			*fresh.AccessTokenGeneration != *payload.AccessTokenGeneration {
			return model.ErrAuthFlowInvalid
		}
		user, err := model.ValidateAuthSessionUserWithTx(tx, fresh.Authorization.AuthSessionIdentity)
		if err != nil {
			if errors.Is(err, model.ErrUserSessionInactive) || errors.Is(err, gorm.ErrRecordNotFound) {
				return model.ErrAuthFlowInvalid
			}
			return err
		}
		if user.AccessTokenGeneration != *fresh.AccessTokenGeneration {
			return model.ErrAuthFlowInvalid
		}
		scopes, err := service.NormalizeAccessTokenScopes(user.Id, common.RoleCommonUser, fresh.Grant.Scopes)
		if err != nil {
			return err
		}
		pat, rotateErr := rotateDeviceAccessTokenWithTx(tx, user, scopes, fresh.Grant.ExpiresAt)
		if rotateErr != nil {
			return rotateErr
		}
		issuedPAT = pat
		username = user.Username
		return nil
	})
	if err != nil {
		if errors.Is(err, model.ErrAuthFlowConsumed) || errors.Is(err, model.ErrAuthFlowInvalid) || errors.Is(err, model.ErrAuthFlowExpired) {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"code":    "invalid_grant",
				"message": "device code is not usable",
			})
			return
		}
		common.ApiError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"access_token": issuedPAT,
			"token_type":   "Bearer",
			"user_id":      userID,
			"username":     username,
		},
	})
}

// ApproveDeviceLogin binds the pending device code to the authenticated user.
func ApproveDeviceLogin(c *gin.Context) {
	identity, ok := requireBrowserSession(c)
	if !ok {
		return
	}
	if strings.HasPrefix(identity.SessionID, model.AccessTokenSessionPrefix) {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "code": "AUTH_SESSION_REQUIRED", "message": "a dashboard login session is required"})
		return
	}
	userID := identity.UserID
	var req deviceUserCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "user_code is required"})
		return
	}
	normalized := normalizeDeviceUserCode(req.UserCode)
	if normalized == "" {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "user_code is required"})
		return
	}

	_, deviceFlow, grant, err := lookupDeviceLoginGrant(normalized)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "invalid or expired user code"})
		return
	}
	operation, err := deviceLoginOperation(deviceFlow, grant)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "invalid or expired user code"})
		return
	}
	authorization := middleware.RequireSecurityProof(c, operation)
	if authorization == nil {
		return
	}
	if authorization.AuthSessionIdentity != identity {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "unauthorized"})
		return
	}

	_, err = model.ConsumeAuthFlowWithAction(normalized, model.AuthFlowMatch{
		Purpose: model.AuthFlowPurposeCliDeviceUser,
	}, func(tx *gorm.DB, _ *model.AuthFlow) error {
		result := tx.Model(&model.AuthFlow{}).
			Where("id = ? AND consumed_at IS NULL AND expires_at > ?", deviceFlow.Id, time.Now()).
			Where("(intent = ? OR intent = '' OR intent IS NULL)", model.AuthFlowIntentPending).
			Updates(map[string]interface{}{
				"user_id":    userID,
				"intent":     model.AuthFlowIntentApproved,
				"session_id": authorization.SessionID,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return model.ErrAuthFlowInvalid
		}
		// Lock order matches redemption: device flow, then user and session.
		user, err := model.ValidateAuthSessionUserWithTx(tx, authorization.AuthSessionIdentity)
		if err != nil {
			return err
		}
		var freshFlow model.AuthFlow
		if err := tx.First(&freshFlow, deviceFlow.Id).Error; err != nil {
			return err
		}
		var payload deviceLoginDevicePayload
		if err := common.UnmarshalJsonStr(freshFlow.Payload, &payload); err != nil {
			return err
		}
		freshOperation, err := deviceLoginOperation(&freshFlow, payload.Grant)
		if err != nil {
			return err
		}
		binding, err := service.BindVerificationOperation(freshOperation)
		if err != nil || binding.ContextHash != authorization.ContextHash {
			return model.ErrAuthFlowInvalid
		}
		payload.Authorization = authorization
		payload.DeviceFlowID = freshFlow.Id
		payload.AccessTokenGeneration = &user.AccessTokenGeneration
		encoded, err := common.Marshal(payload)
		if err != nil {
			return err
		}
		return tx.Model(&freshFlow).Update("payload", string(encoded)).Error
	})
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "invalid or expired user code"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"approved": true,
			"user_id":  userID,
		},
	})
}

// DenyDeviceLogin rejects a pending device login request.
func DenyDeviceLogin(c *gin.Context) {
	identity, ok := requireBrowserSession(c)
	if !ok {
		return
	}
	if strings.HasPrefix(identity.SessionID, model.AccessTokenSessionPrefix) {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "code": "AUTH_SESSION_REQUIRED", "message": "a dashboard login session is required"})
		return
	}
	userID := identity.UserID
	var req deviceUserCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "user_code is required"})
		return
	}
	normalized := normalizeDeviceUserCode(req.UserCode)
	if normalized == "" {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "user_code is required"})
		return
	}

	userFlow, err := model.GetAuthFlow(normalized, model.AuthFlowMatch{
		Purpose: model.AuthFlowPurposeCliDeviceUser,
	})
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "invalid or expired user code"})
		return
	}
	var userPayload deviceLoginUserPayload
	if err := common.UnmarshalJsonStr(userFlow.Payload, &userPayload); err != nil || userPayload.DeviceFlowID <= 0 {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "invalid or expired user code"})
		return
	}
	deviceFlow, err := model.GetAuthFlowByID(userPayload.DeviceFlowID)
	if err != nil || deviceFlow.Purpose != model.AuthFlowPurposeCliDevice {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "invalid or expired user code"})
		return
	}

	_, err = model.ConsumeAuthFlowWithAction(normalized, model.AuthFlowMatch{
		Purpose: model.AuthFlowPurposeCliDeviceUser,
	}, func(tx *gorm.DB, _ *model.AuthFlow) error {
		result := tx.Model(&model.AuthFlow{}).
			Where("id = ? AND consumed_at IS NULL AND expires_at > ?", deviceFlow.Id, time.Now()).
			Where("intent = ? OR intent = '' OR intent IS NULL", model.AuthFlowIntentPending).
			Updates(map[string]interface{}{
				"intent":  model.AuthFlowIntentDenied,
				"user_id": userID,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return model.ErrAuthFlowInvalid
		}
		return nil
	})
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "invalid or expired user code"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    gin.H{"denied": true},
	})
}

func createDeviceUserCodeFlow(deviceFlowID int64, expiresAt time.Time) (normalized string, display string, err error) {
	userPayload, err := common.Marshal(deviceLoginUserPayload{DeviceFlowID: deviceFlowID})
	if err != nil {
		return "", "", err
	}
	for attempt := 0; attempt < 8; attempt++ {
		normalized, display, err = generateDeviceUserCode()
		if err != nil {
			return "", "", err
		}
		_, _, err = model.CreateAuthFlow(model.AuthFlowCreate{
			Purpose:   model.AuthFlowPurposeCliDeviceUser,
			Intent:    model.AuthFlowIntentPending,
			Payload:   string(userPayload),
			ExpiresAt: expiresAt,
			Token:     normalized,
		})
		if err == nil {
			return normalized, display, nil
		}
		lower := strings.ToLower(err.Error())
		if !strings.Contains(lower, "unique") && !strings.Contains(lower, "duplicate") {
			return "", "", err
		}
	}
	return "", "", fmt.Errorf("failed to allocate unique user code")
}

func generateDeviceUserCode() (normalized string, display string, err error) {
	var b strings.Builder
	b.Grow(deviceLoginUserCodeLen)
	max := big.NewInt(int64(len(deviceLoginUserCodeChars)))
	for i := 0; i < deviceLoginUserCodeLen; i++ {
		n, randErr := rand.Int(rand.Reader, max)
		if randErr != nil {
			return "", "", randErr
		}
		b.WriteByte(deviceLoginUserCodeChars[n.Int64()])
	}
	normalized = b.String()
	display = normalized[:4] + "-" + normalized[4:]
	return normalized, display, nil
}

func normalizeDeviceUserCode(raw string) string {
	s := strings.ToUpper(strings.TrimSpace(raw))
	s = strings.ReplaceAll(s, "-", "")
	s = strings.ReplaceAll(s, " ", "")
	if len(s) != deviceLoginUserCodeLen {
		return ""
	}
	for i := 0; i < len(s); i++ {
		if !strings.ContainsRune(deviceLoginUserCodeChars, rune(s[i])) {
			return ""
		}
	}
	return s
}

func deviceLoginVerificationURI(c *gin.Context) string {
	base := strings.TrimRight(strings.TrimSpace(system_setting.ServerAddress), "/")
	if base == "" {
		scheme := "https"
		if c.Request.TLS == nil {
			if proto := c.GetHeader("X-Forwarded-Proto"); proto != "" {
				scheme = strings.TrimSpace(strings.Split(proto, ",")[0])
			} else {
				scheme = "http"
			}
		}
		host := c.Request.Host
		if host == "" {
			host = "localhost"
		}
		base = scheme + "://" + host
	}
	return base + "/cli/login"
}

// deviceLoginScopes is the fixed personal grant used by the user CLI.
func deviceLoginScopes() []string {
	return []string{
		"api_key:read", "api_key:reveal", "api_key:write", "files:read", "files:write",
		"hosted:read", "hosted:write", "influencer_creation:read", "influencer_creation:write",
		"inspiration:read", "inspiration:write", "profile:read",
		"social:read", "social:write",
	}
}

func deviceLoginOperation(flow *model.AuthFlow, grant *service.CliDeviceApproveContext) (service.VerificationOperation, error) {
	if flow == nil || grant == nil || grant.DeviceFlowID != flow.Id || normalizeDeviceUserCode(grant.UserCode) != grant.UserCode ||
		grant.ExpiresAt <= time.Now().Unix() || !slices.Equal(grant.Scopes, deviceLoginScopes()) {
		return service.VerificationOperation{}, model.ErrAuthFlowInvalid
	}
	context, err := common.Marshal(grant)
	return service.VerificationOperation{Scope: service.VerificationScopeCliDeviceApprove, Context: context}, err
}

func lookupDeviceLoginGrant(code string) (*model.AuthFlow, *model.AuthFlow, *service.CliDeviceApproveContext, error) {
	normalized := normalizeDeviceUserCode(code)
	if normalized == "" {
		return nil, nil, nil, model.ErrAuthFlowInvalid
	}
	userFlow, err := model.GetAuthFlow(normalized, model.AuthFlowMatch{Purpose: model.AuthFlowPurposeCliDeviceUser})
	if err != nil {
		return nil, nil, nil, err
	}
	var userPayload deviceLoginUserPayload
	if common.UnmarshalJsonStr(userFlow.Payload, &userPayload) != nil || userPayload.DeviceFlowID <= 0 {
		return nil, nil, nil, model.ErrAuthFlowInvalid
	}
	deviceFlow, err := model.GetAuthFlowByID(userPayload.DeviceFlowID)
	if err != nil {
		return nil, nil, nil, err
	}
	var payload deviceLoginDevicePayload
	if deviceFlow.Purpose != model.AuthFlowPurposeCliDevice || (deviceFlow.Intent != model.AuthFlowIntentPending && deviceFlow.Intent != "") ||
		common.UnmarshalJsonStr(deviceFlow.Payload, &payload) != nil || payload.Grant == nil || payload.Grant.UserCode != normalized {
		return nil, nil, nil, model.ErrAuthFlowInvalid
	}
	if _, err := deviceLoginOperation(deviceFlow, payload.Grant); err != nil {
		return nil, nil, nil, err
	}
	return userFlow, deviceFlow, payload.Grant, nil
}

func rotateDeviceAccessTokenWithTx(tx *gorm.DB, user *model.User, scopes []string, expiresAt int64) (string, error) {
	suffix, err := common.GenerateRandomCharsKey(43)
	if err != nil {
		return "", err
	}
	raw := model.AccessTokenPrefix + suffix
	token := &model.UserAccessToken{
		Name: "OmniMux CLI", DeviceSlot: true,
		TokenHash: model.AccessTokenFingerprint(raw), TokenHint: model.AccessTokenHint(raw),
		ExpiresAt: expiresAt,
	}
	if err := token.SetScopes(scopes); err != nil {
		return "", err
	}
	if err := model.ReplaceDeviceAccessTokenWithTx(tx, user.Id, token, service.AccessTokenMaxPerUser); err != nil {
		return "", err
	}
	return raw, nil
}
