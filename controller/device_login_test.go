package controller

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupDeviceLoginTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	gin.SetMode(gin.TestMode)
	previousDB := model.DB
	previousType := common.MainDatabaseType()
	previousRedis := common.RedisEnabled
	previousSecret := common.SessionSecret
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.AuthFlow{}))
	model.DB = db
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	common.RedisEnabled = false
	common.SessionSecret = "device-login-test-secret"
	t.Cleanup(func() {
		model.DB = previousDB
		common.SetMainDatabaseType(previousType)
		common.RedisEnabled = previousRedis
		common.SessionSecret = previousSecret
		sqlDB, dbErr := db.DB()
		if dbErr == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func deviceLoginJSON(t *testing.T, method, path string, body any, setup func(*gin.Context)) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		raw, err := json.Marshal(body)
		require.NoError(t, err)
		reader = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req
	if setup != nil {
		setup(c)
	}
	switch {
	case strings.HasSuffix(path, "/device/code"):
		CreateDeviceLoginCode(c)
	case strings.HasSuffix(path, "/device/token"):
		PollDeviceLoginToken(c)
	case strings.HasSuffix(path, "/device/approve"):
		ApproveDeviceLogin(c)
	case strings.HasSuffix(path, "/device/deny"):
		DenyDeviceLogin(c)
	default:
		t.Fatalf("unknown path %s", path)
	}
	return rec
}

func TestDeviceLoginPublicGrantLookupNoSecrets(t *testing.T) {
	setupDeviceLoginTestDB(t)
	created := deviceLoginJSON(t, "POST", "/device/code", map[string]string{"client_name": "omnimux-cli"}, nil)
	var body struct {
		Data map[string]any `json:"data"`
	}
	require.NoError(t, common.Unmarshal(created.Body.Bytes(), &body))
	require.NotNil(t, body.Data["verification_context"], "device grant must be discoverable before browser verification")
	lookup := deviceLoginJSON(t, "POST", "/device/code", map[string]string{"user_code": body.Data["user_code"].(string)}, nil)
	var lookedUp struct {
		Success bool           `json:"success"`
		Data    map[string]any `json:"data"`
	}
	require.NoError(t, common.Unmarshal(lookup.Body.Bytes(), &lookedUp))
	require.True(t, lookedUp.Success)
	assert.Equal(t, body.Data["verification_context"], lookedUp.Data["verification_context"])
	assert.NotContains(t, lookedUp.Data, "device_code")
	assert.NotContains(t, lookedUp.Data, "access_token")
}

func TestDeviceLoginHappyPathIssuesPATOnce(t *testing.T) {
	prevInterval := deviceLoginInterval
	deviceLoginInterval = 0
	t.Cleanup(func() { deviceLoginInterval = prevInterval })
	user, identity := setupSecurityEnrollmentTest(t)
	oldPAT := "old-pat-token-value-012345678901"
	require.NoError(t, model.UpdateLegacyUserAccessToken(user.Id, oldPAT))
	codeRec := deviceLoginJSON(t, http.MethodPost, "/api/user/device/code", map[string]string{
		"client_name": "omnimux-cli",
	}, nil)
	require.Equal(t, http.StatusOK, codeRec.Code)
	var codeBody map[string]any
	require.NoError(t, json.Unmarshal(codeRec.Body.Bytes(), &codeBody))
	require.True(t, codeBody["success"].(bool))
	data := codeBody["data"].(map[string]any)
	deviceCode := data["device_code"].(string)
	userCode := data["user_code"].(string)
	proof := issueSecurityEnrollmentProof(t, identity, deviceLoginTestOperation(t, userCode), service.VerificationMethodPassword)
	require.NotEmpty(t, deviceCode)
	require.Contains(t, userCode, "-")

	// Pending before approve
	pendingRec := deviceLoginJSON(t, http.MethodPost, "/api/user/device/token", map[string]string{
		"device_code": deviceCode,
		"grant_type":  deviceLoginGrantType,
	}, nil)
	var pendingBody map[string]any
	require.NoError(t, json.Unmarshal(pendingRec.Body.Bytes(), &pendingBody))
	assert.Equal(t, "authorization_pending", pendingBody["code"])

	// Wait past interval to avoid slow_down

	approveRec := deviceLoginJSON(t, http.MethodPost, "/api/user/device/approve", map[string]string{
		"user_code": userCode,
	}, func(c *gin.Context) {
		c.Set("id", user.Id)
		c.Set("session_id", identity.SessionID)
		c.Set("auth_version", identity.UserAuthVersion)
		c.Set("session_version", identity.SessionVersion)
		c.Request.Header.Set("X-Security-Proof", proof)
	})
	var approveBody map[string]any
	require.NoError(t, json.Unmarshal(approveRec.Body.Bytes(), &approveBody))
	require.True(t, approveBody["success"].(bool), approveRec.Body.String())

	tokenRec := deviceLoginJSON(t, http.MethodPost, "/api/user/device/token", map[string]string{
		"device_code": deviceCode,
		"grant_type":  deviceLoginGrantType,
	}, nil)
	var tokenBody map[string]any
	require.NoError(t, json.Unmarshal(tokenRec.Body.Bytes(), &tokenBody))
	require.True(t, tokenBody["success"].(bool), tokenRec.Body.String())
	tokenData := tokenBody["data"].(map[string]any)
	pat := tokenData["access_token"].(string)
	require.NotEmpty(t, pat)
	assert.NotEqual(t, oldPAT, pat)
	assert.EqualValues(t, float64(user.Id), tokenData["user_id"])
	assert.Equal(t, user.Username, tokenData["username"])

	// Second poll must fail
	second := deviceLoginJSON(t, http.MethodPost, "/api/user/device/token", map[string]string{
		"device_code": deviceCode,
	}, nil)
	var secondBody map[string]any
	require.NoError(t, json.Unmarshal(second.Body.Bytes(), &secondBody))
	assert.False(t, secondBody["success"].(bool))
	assert.Equal(t, "invalid_grant", secondBody["code"])

	// Native PAT validates; the replaced legacy credential cannot authenticate.
	validated, err := model.FindUserAccessTokenByHash(model.AccessTokenFingerprint(pat))
	require.NoError(t, err)
	require.NotNil(t, validated)
	assert.Equal(t, user.Id, validated.UserId)
	assert.Positive(t, validated.CreatedAt)
	assert.Positive(t, validated.ExpiresAt)
	assert.True(t, validated.DeviceSlot)
	assert.True(t, strings.HasPrefix(pat, "nap_"))
	assert.Equal(t, []string{"api_key:read", "api_key:reveal", "api_key:write", "files:read", "files:write", "hosted:read", "hosted:write", "influencer_creation:read", "influencer_creation:write", "inspiration:read", "inspiration:write", "profile:read", "social:read", "social:write"}, validated.GetScopes())
	var current model.User
	require.NoError(t, model.DB.First(&current, user.Id).Error)
	assert.Empty(t, current.GetAccessToken())
	assert.EqualValues(t, 2, current.AccessTokenGeneration)
	old, err := model.ValidateAccessToken(oldPAT)
	require.NoError(t, err)
	assert.Nil(t, old)
}

func TestDeviceLoginDenyReturnsAccessDenied(t *testing.T) {
	prevInterval := deviceLoginInterval
	deviceLoginInterval = 0
	t.Cleanup(func() { deviceLoginInterval = prevInterval })
	user, identity := setupSecurityEnrollmentTest(t)

	codeRec := deviceLoginJSON(t, http.MethodPost, "/api/user/device/code", map[string]string{}, nil)
	var codeBody map[string]any
	require.NoError(t, json.Unmarshal(codeRec.Body.Bytes(), &codeBody))
	data := codeBody["data"].(map[string]any)
	deviceCode := data["device_code"].(string)
	userCode := data["user_code"].(string)

	denyRec := deviceLoginJSON(t, http.MethodPost, "/api/user/device/deny", map[string]string{
		"user_code": userCode,
	}, func(c *gin.Context) {
		c.Set("id", user.Id)
		c.Set("session_id", identity.SessionID)
		c.Set("auth_version", identity.UserAuthVersion)
		c.Set("session_version", identity.SessionVersion)
	})
	var denyBody map[string]any
	require.NoError(t, json.Unmarshal(denyRec.Body.Bytes(), &denyBody))
	require.True(t, denyBody["success"].(bool), denyRec.Body.String())

	tokenRec := deviceLoginJSON(t, http.MethodPost, "/api/user/device/token", map[string]string{
		"device_code": deviceCode,
	}, nil)
	var tokenBody map[string]any
	require.NoError(t, json.Unmarshal(tokenRec.Body.Bytes(), &tokenBody))
	assert.Equal(t, "access_denied", tokenBody["code"])
}

func TestNormalizeDeviceUserCode(t *testing.T) {
	assert.Equal(t, "BCDF2345", normalizeDeviceUserCode("bcdf-2345"))
	assert.Equal(t, "BCDF2345", normalizeDeviceUserCode(" BCDF2345 "))
	assert.Equal(t, "", normalizeDeviceUserCode("ABC"))
	assert.Equal(t, "", normalizeDeviceUserCode("ABCD-IO10")) // invalid charset (I,O,1,0)
}

// deviceLoginTestCodes issues genuine paired flows through the public handler.
func deviceLoginTestCodes(t *testing.T) (string, string) {
	t.Helper()
	rec := deviceLoginJSON(t, "POST", "/device/code", map[string]string{}, nil)
	var body struct {
		Data struct {
			DeviceCode string `json:"device_code"`
			UserCode   string `json:"user_code"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(rec.Body.Bytes(), &body))
	require.NotEmpty(t, body.Data.DeviceCode)
	return body.Data.DeviceCode, body.Data.UserCode
}

func approveDeviceLoginTest(t *testing.T, identity service.AuthIdentity, code string) {
	t.Helper()
	proof := issueSecurityEnrollmentProof(t, identity, deviceLoginTestOperation(t, code), service.VerificationMethodPassword)
	body, err := common.Marshal(deviceUserCodeRequest{UserCode: code})
	require.NoError(t, err)
	rec := securityEnrollmentRequest("POST", "/device/approve", string(body), proof, identity, ApproveDeviceLogin)
	require.Contains(t, rec.Body.String(), `"approved":true`)
}

func TestDeviceLoginApprovalRequiresNativeProof(t *testing.T) {
	_, identity := setupSecurityEnrollmentTest(t)
	device, code := deviceLoginTestCodes(t)
	body, err := common.Marshal(deviceUserCodeRequest{UserCode: code})
	require.NoError(t, err)
	for _, test := range []struct {
		name, proof string
		identity    service.AuthIdentity
	}{
		{"missing", "", identity},
		{"PAT identity", "", service.AuthIdentity{UserID: identity.UserID}},
		{"scoped PAT identity", "", service.AuthIdentity{UserID: identity.UserID, SessionID: "pat:1", UserAuthVersion: identity.UserAuthVersion, SessionVersion: 1}},
		{"wrong scope", issueSecurityEnrollmentProof(t, identity, service.VerificationOperation{Scope: service.VerificationScopeTwoFASetup}, service.VerificationMethodPassword), identity},
	} {
		t.Run(test.name, func(t *testing.T) {
			rec := securityEnrollmentRequest("POST", "/device/approve", string(body), test.proof, test.identity, ApproveDeviceLogin)
			assert.Equal(t, http.StatusForbidden, rec.Code)
			flow, err := model.GetAuthFlow(device, model.AuthFlowMatch{Purpose: model.AuthFlowPurposeCliDevice})
			require.NoError(t, err)
			assert.Equal(t, model.AuthFlowIntentPending, flow.Intent)
		})
	}
	approveDeviceLoginTest(t, identity, code)
}

func TestDeviceLoginRejectsStaleAuthorization(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*testing.T, *model.User, service.AuthIdentity, *model.AuthFlow)
	}{
		{"session revoked", func(t *testing.T, u *model.User, i service.AuthIdentity, _ *model.AuthFlow) {
			_, err := model.RevokeUserSession(u.Id, i.SessionID, "test")
			require.NoError(t, err)
		}},
		{"session expired", func(t *testing.T, _ *model.User, i service.AuthIdentity, _ *model.AuthFlow) {
			require.NoError(t, model.DB.Model(&model.UserSession{}).Where("sid = ?", i.SessionID).Update("expires_at", time.Now().Unix()-1).Error)
		}},
		{"session version", func(t *testing.T, _ *model.User, i service.AuthIdentity, _ *model.AuthFlow) {
			require.NoError(t, model.DB.Model(&model.UserSession{}).Where("sid = ?", i.SessionID).Update("version", i.SessionVersion+1).Error)
		}},
		{"account version", func(t *testing.T, u *model.User, _ service.AuthIdentity, _ *model.AuthFlow) {
			require.NoError(t, model.DB.Model(u).Update("auth_version", u.AuthVersion+1).Error)
		}},
		{"disabled", func(t *testing.T, u *model.User, _ service.AuthIdentity, _ *model.AuthFlow) {
			require.NoError(t, model.DB.Model(u).Update("status", common.UserStatusDisabled).Error)
		}},
		{"deleted", func(t *testing.T, u *model.User, _ service.AuthIdentity, _ *model.AuthFlow) {
			require.NoError(t, model.DB.Delete(u).Error)
		}},
		{"PAT rotated", func(t *testing.T, u *model.User, _ service.AuthIdentity, _ *model.AuthFlow) {
			require.NoError(t, model.UpdateLegacyUserAccessToken(u.Id, "replacement"))
		}},
		{"PAT revoked", func(t *testing.T, u *model.User, _ service.AuthIdentity, _ *model.AuthFlow) {
			_, err := model.RevokeUserAccessToken(u.Id)
			require.NoError(t, err)
		}},
		{"empty issued revoked ABA", func(t *testing.T, u *model.User, _ service.AuthIdentity, _ *model.AuthFlow) {
			require.NoError(t, model.UpdateLegacyUserAccessToken(u.Id, "transient"))
			_, err := model.RevokeUserAccessToken(u.Id)
			require.NoError(t, err)
		}},
		{"legacy payload", func(t *testing.T, _ *model.User, _ service.AuthIdentity, f *model.AuthFlow) {
			require.NoError(t, model.DB.Model(f).Update("payload", `{}`).Error)
		}},
		{"corrupt payload", func(t *testing.T, _ *model.User, _ service.AuthIdentity, f *model.AuthFlow) {
			require.NoError(t, model.DB.Model(f).Update("payload", `{`).Error)
		}},
		{"different flow", func(t *testing.T, _ *model.User, _ service.AuthIdentity, f *model.AuthFlow) {
			var p deviceLoginDevicePayload
			require.NoError(t, common.UnmarshalJsonStr(f.Payload, &p))
			p.DeviceFlowID++
			b, err := common.Marshal(p)
			require.NoError(t, err)
			require.NoError(t, model.DB.Model(f).Update("payload", string(b)).Error)
		}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			u, identity := setupSecurityEnrollmentTest(t)
			device, code := deviceLoginTestCodes(t)
			approveDeviceLoginTest(t, identity, code)
			flow, err := model.GetAuthFlow(device, model.AuthFlowMatch{Purpose: model.AuthFlowPurposeCliDevice})
			require.NoError(t, err)
			test.mutate(t, u, identity, flow)
			var before model.User
			require.NoError(t, model.DB.Unscoped().First(&before, u.Id).Error)
			rec := deviceLoginJSON(t, "POST", "/device/token", deviceTokenRequest{DeviceCode: device}, nil)
			assert.Contains(t, rec.Body.String(), `"code":"invalid_grant"`)
			var after model.User
			require.NoError(t, model.DB.Unscoped().First(&after, u.Id).Error)
			assert.Equal(t, before.GetAccessToken(), after.GetAccessToken())
			assert.Equal(t, before.AccessTokenGeneration, after.AccessTokenGeneration)
		})
	}
}

func TestDeviceLoginUnrelatedRevocationAndRollback(t *testing.T) {
	u, identity := setupSecurityEnrollmentTest(t)
	other, err := service.CreateLoginSession(u.Id, "password", "127.0.0.1", "unrelated")
	require.NoError(t, err)
	otherIdentity, err := service.ParseAccessToken(other.AccessToken)
	require.NoError(t, err)
	device, code := deviceLoginTestCodes(t)
	approveDeviceLoginTest(t, identity, code)
	_, err = model.RevokeUserSession(u.Id, otherIdentity.SessionID, "test")
	require.NoError(t, err)
	require.NoError(t, model.DB.Callback().Update().Before("gorm:update").Register("device_write_failure", func(tx *gorm.DB) {
		if tx.Statement.Table == "users" {
			tx.AddError(errors.New("injected PAT failure"))
		}
	}))
	rec := deviceLoginJSON(t, "POST", "/device/token", deviceTokenRequest{DeviceCode: device}, nil)
	assert.Contains(t, rec.Body.String(), `"success":false`)
	flow, err := model.GetAuthFlow(device, model.AuthFlowMatch{Purpose: model.AuthFlowPurposeCliDevice})
	require.NoError(t, err)
	assert.Nil(t, flow.ConsumedAt)
	require.NoError(t, model.DB.Callback().Update().Remove("device_write_failure"))
	rec = deviceLoginJSON(t, "POST", "/device/token", deviceTokenRequest{DeviceCode: device}, nil)
	assert.Contains(t, rec.Body.String(), `"success":true`)
}

func TestDeviceLoginStalePollCannotEraseApproval(t *testing.T) {
	_, identity := setupSecurityEnrollmentTest(t)
	device, code := deviceLoginTestCodes(t)
	interleaved := false
	require.NoError(t, model.DB.Callback().Update().Before("gorm:begin_transaction").Register("approve_before_stale_poll", func(tx *gorm.DB) {
		updates, ok := tx.Statement.Dest.(map[string]interface{})
		if !interleaved && ok && len(updates) == 1 && updates["payload"] != nil {
			interleaved = true
			approveDeviceLoginTest(t, identity, code)
		}
	}))
	t.Cleanup(func() { _ = model.DB.Callback().Update().Remove("approve_before_stale_poll") })
	deviceLoginJSON(t, "POST", "/device/token", deviceTokenRequest{DeviceCode: device}, nil)
	require.True(t, interleaved)
	rec := deviceLoginJSON(t, "POST", "/device/token", deviceTokenRequest{DeviceCode: device}, nil)
	assert.Contains(t, rec.Body.String(), `"success":true`)
}

func TestDeviceLoginConcurrentRedemptionIssuesOnce(t *testing.T) {
	u, identity := setupSecurityEnrollmentTest(t)
	device, code := deviceLoginTestCodes(t)
	approveDeviceLoginTest(t, identity, code)
	var wg sync.WaitGroup
	responses := make(chan string, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			responses <- deviceLoginJSON(t, "POST", "/device/token", deviceTokenRequest{DeviceCode: device}, nil).Body.String()
		}()
	}
	wg.Wait()
	close(responses)
	successes := 0
	for response := range responses {
		if strings.Contains(response, `"success":true`) {
			successes++
		}
	}
	assert.Equal(t, 1, successes)
	var current model.User
	require.NoError(t, model.DB.First(&current, u.Id).Error)
	assert.EqualValues(t, 1, current.AccessTokenGeneration)
}

// TestDeviceLoginRevocationWinsRepeatableReadSnapshot pins the ordering that a
// locking read must resolve, not the transaction's earlier consistent snapshot.
//
// The consumed device flow is read before any locking read, which on MySQL
// REPEATABLE READ fixes the transaction snapshot at a generation that still
// predates the revocation. The revocation below therefore commits strictly
// between the snapshot read and the user-row lock. Only a current read of the
// locked row can observe the advanced generation; a snapshot read would still
// see the stale value, pass the comparison, and hand the caller a fresh PAT
// after the user revoked the credential. Real MySQL is required: SQLite has no
// consistent-snapshot/locking-read split to exercise.
//
// Measured on MySQL 8.0.46: with the stale-read variant the same schedule yields
// snapshotGeneration=1 against a locked generation of 2 and redemption returns
// success with a new access_token, so this test fails closed on that variant.
func TestDeviceLoginRevocationWinsRepeatableReadSnapshot(t *testing.T) {
	if os.Getenv("TEST_SECURITY_DIALECT") != "mysql" {
		t.Skip("requires MySQL: SQLite has no REPEATABLE READ snapshot/locking-read split")
	}
	u, identity := setupSecurityEnrollmentTest(t)
	require.Equal(t, common.DatabaseTypeMySQL, common.MainDatabaseType())
	oldPAT := "revoked-before-redemption-012345" // 32 chars: access_token is char(32)
	require.NoError(t, model.UpdateLegacyUserAccessToken(u.Id, oldPAT))
	device, code := deviceLoginTestCodes(t)
	approveDeviceLoginTest(t, identity, code)

	var approved model.User
	require.NoError(t, model.DB.First(&approved, u.Id).Error)
	generationAtApproval := approved.AccessTokenGeneration
	require.NotZero(t, generationAtApproval)

	// Barrier: the revoke commits after the redemption transaction has read the
	// consumed device flow (fixing its consistent-read snapshot) and strictly
	// before it takes the user-row lock. The redemption's flow claim and its
	// snapshot read both filter on intent and user_id; the outer pre-check does
	// not, and the barrier is armed only for redemption, so the ordering is
	// deterministic rather than timing-dependent.
	barrierArmed := false
	barrierFired := false
	var generationAfterRevoke int64
	require.NoError(t, model.DB.Callback().Query().After("gorm:query").Register("finding06_snapshot_barrier", func(tx *gorm.DB) {
		if !barrierArmed || barrierFired || tx.Statement.Table != "auth_flows" {
			return
		}
		sql := tx.Statement.SQL.String()
		if !strings.Contains(sql, "intent") || !strings.Contains(sql, "user_id") {
			return
		}
		barrierFired = true
		revoked, err := model.RevokeUserAccessToken(u.Id)
		require.NoError(t, err)
		require.Equal(t, model.AccessTokenFingerprint(oldPAT), revoked)
		var inside model.User
		require.NoError(t, model.DB.First(&inside, u.Id).Error)
		generationAfterRevoke = inside.AccessTokenGeneration
	}))
	t.Cleanup(func() { _ = model.DB.Callback().Query().Remove("finding06_snapshot_barrier") })

	barrierArmed = true
	response := deviceLoginJSON(t, "POST", "/device/token", deviceTokenRequest{DeviceCode: device}, nil).Body.String()

	require.True(t, barrierFired, "the revocation barrier must have run inside the redemption transaction")
	require.Greater(t, generationAfterRevoke, generationAtApproval, "revocation must advance the credential generation")

	assert.Contains(t, response, `"code":"invalid_grant"`, "a revocation committed before the user-row lock must block the approved flow")

	var final model.User
	require.NoError(t, model.DB.Unscoped().First(&final, u.Id).Error)
	assert.Empty(t, final.GetAccessToken(), "redemption must not re-issue a PAT after revocation")
	assert.Equal(t, generationAfterRevoke, final.AccessTokenGeneration, "redemption must not advance the generation")
}

// TestDeviceLoginRepeatableReadSnapshotControl is the control for the test
// above: it shows that an ordinary (non-locking) read taken before the
// revocation still returns the pre-revocation generation on MySQL after the
// revocation commits, while a locking read returns the current one. Without
// this control the main test would only prove that redemption rejects, not that
// the rejection came from reading the locked row rather than an older snapshot.
func TestDeviceLoginRepeatableReadSnapshotControl(t *testing.T) {
	if os.Getenv("TEST_SECURITY_DIALECT") != "mysql" {
		t.Skip("requires MySQL: SQLite has no REPEATABLE READ snapshot/locking-read split")
	}
	u, _ := setupSecurityEnrollmentTest(t)
	require.Equal(t, common.DatabaseTypeMySQL, common.MainDatabaseType())
	require.NoError(t, model.UpdateLegacyUserAccessToken(u.Id, "control-token-0123456789abcdef"))

	// A device flow must exist for the snapshot read to succeed.
	_, code := deviceLoginTestCodes(t)
	_ = code

	var start model.User
	require.NoError(t, model.DB.First(&start, u.Id).Error)
	snapshotGeneration := start.AccessTokenGeneration

	tx := model.DB.Begin()
	require.NoError(t, tx.Error)
	// Establish the consistent-read snapshot exactly where redemption does:
	// an ordinary read of the device flow row.
	var flow model.AuthFlow
	require.NoError(t, tx.Where("purpose = ?", model.AuthFlowPurposeCliDevice).First(&flow).Error)

	// The revocation commits strictly after the snapshot was established.
	_, err := model.RevokeUserAccessToken(u.Id)
	require.NoError(t, err)
	var afterRevoke model.User
	require.NoError(t, model.DB.First(&afterRevoke, u.Id).Error)
	require.Greater(t, afterRevoke.AccessTokenGeneration, snapshotGeneration)

	var stale model.User
	require.NoError(t, tx.First(&stale, u.Id).Error)
	assert.Equal(t, snapshotGeneration, stale.AccessTokenGeneration,
		"control: an ordinary read inside the open transaction must still see the pre-revocation generation")

	var current model.User
	require.NoError(t, model.LockForUpdate(tx).First(&current, u.Id).Error)
	assert.Equal(t, afterRevoke.AccessTokenGeneration, current.AccessTokenGeneration,
		"control: a locking read must see the committed revocation")

	require.NoError(t, tx.Rollback().Error)
}

func TestDeviceLoginCannotUseSessionOnlyVerification(t *testing.T) {
	_, identity := setupSecurityEnrollmentTest(t)
	device, code := deviceLoginTestCodes(t)
	operation := deviceLoginTestOperation(t, code)
	_, err := service.VerifySecurityInput(identity, service.VerificationInput{Scope: operation.Scope, Context: operation.Context, Method: service.VerificationMethodSession})
	require.ErrorIs(t, err, service.ErrProofMethod)
	_, err = service.VerifySecurityInput(identity, service.VerificationInput{Scope: operation.Scope, Context: operation.Context, Method: service.VerificationMethodPassword, Password: "wrong-password"})
	require.Error(t, err)
	proof, err := service.VerifySecurityInput(identity, service.VerificationInput{Scope: operation.Scope, Context: operation.Context, Method: service.VerificationMethodPassword, Password: "enrollment-password"})
	require.NoError(t, err)
	body, err := common.Marshal(deviceUserCodeRequest{UserCode: code})
	require.NoError(t, err)
	rec := securityEnrollmentRequest("POST", "/device/approve", string(body), proof.ProofToken, identity, ApproveDeviceLogin)
	require.Contains(t, rec.Body.String(), `"approved":true`)
	rec = securityEnrollmentRequest("POST", "/device/approve", string(body), proof.ProofToken, identity, ApproveDeviceLogin)
	assert.NotContains(t, rec.Body.String(), `"approved":true`)
	_, otherCode := deviceLoginTestCodes(t)
	body, err = common.Marshal(deviceUserCodeRequest{UserCode: otherCode})
	require.NoError(t, err)
	rec = securityEnrollmentRequest("POST", "/device/approve", string(body), proof.ProofToken, identity, ApproveDeviceLogin)
	assert.Contains(t, rec.Body.String(), "SECURITY_PROOF_CONTEXT_MISMATCH")
	_, err = service.ConsumeOperationProof(proof.ProofToken, identity, operation)
	assert.ErrorIs(t, err, service.ErrProofConsumed)
	rec = deviceLoginJSON(t, "POST", "/device/token", deviceTokenRequest{DeviceCode: device}, nil)
	assert.Contains(t, rec.Body.String(), `"success":true`)
}

func TestDeviceLoginValidatesFreshTransactionState(t *testing.T) {
	for _, mutation := range []string{"session", "authorization"} {
		t.Run(mutation, func(t *testing.T) {
			u, identity := setupSecurityEnrollmentTest(t)
			device, code := deviceLoginTestCodes(t)
			approveDeviceLoginTest(t, identity, code)
			flow, err := model.GetAuthFlow(device, model.AuthFlowMatch{Purpose: model.AuthFlowPurposeCliDevice})
			require.NoError(t, err)
			interleaved := false
			require.NoError(t, model.DB.Callback().Update().Before("gorm:begin_transaction").Register("invalidate_before_device_claim", func(tx *gorm.DB) {
				updates, ok := tx.Statement.Dest.(map[string]interface{})
				if interleaved || !ok || updates["consumed_at"] == nil {
					return
				}
				interleaved = true
				if mutation == "session" {
					require.NoError(t, model.DB.Model(&model.UserSession{}).Where("sid = ?", identity.SessionID).Update("version", identity.SessionVersion+1).Error)
				} else {
					var payload deviceLoginDevicePayload
					require.NoError(t, common.UnmarshalJsonStr(flow.Payload, &payload))
					payload.Authorization.ProofID++
					encoded, err := common.Marshal(payload)
					require.NoError(t, err)
					require.NoError(t, model.DB.Model(flow).Update("payload", string(encoded)).Error)
				}
			}))
			t.Cleanup(func() { _ = model.DB.Callback().Update().Remove("invalidate_before_device_claim") })
			rec := deviceLoginJSON(t, "POST", "/device/token", deviceTokenRequest{DeviceCode: device}, nil)
			require.True(t, interleaved)
			assert.Contains(t, rec.Body.String(), `"code":"invalid_grant"`)
			var current model.User
			require.NoError(t, model.DB.First(&current, u.Id).Error)
			assert.Empty(t, current.GetAccessToken())
			assert.Zero(t, current.AccessTokenGeneration)
		})
	}
}

// deviceLoginTestOperation obtains the grant through the same public lookup as
// the browser; expected permissions are asserted independently at issuance.
func deviceLoginTestOperation(t *testing.T, code string) service.VerificationOperation {
	t.Helper()
	rec := deviceLoginJSON(t, "POST", "/device/code", map[string]string{"user_code": code}, nil)
	var response struct {
		Success bool `json:"success"`
		Data    struct {
			Scope   string          `json:"verification_scope"`
			Context json.RawMessage `json:"verification_context"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(rec.Body.Bytes(), &response))
	require.True(t, response.Success)
	require.Equal(t, "cli.device.approve", response.Data.Scope)
	return service.VerificationOperation{Scope: response.Data.Scope, Context: response.Data.Context}
}

func redeemDeviceLoginTest(t *testing.T, device string) string {
	t.Helper()
	rec := deviceLoginJSON(t, "POST", "/device/token", deviceTokenRequest{DeviceCode: device}, nil)
	var result struct {
		Success bool `json:"success"`
		Data    struct {
			Token string `json:"access_token"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(rec.Body.Bytes(), &result))
	require.True(t, result.Success)
	require.NotEmpty(t, result.Data.Token)
	return result.Data.Token
}

func TestDeviceLoginSlotReplacementPreservesManualPAT(t *testing.T) {
	u, identity := setupSecurityEnrollmentTest(t)
	// A manual token with the same editable name must never become a CLI slot.
	manual := &model.UserAccessToken{Name: "OmniMux CLI", TokenHash: model.AccessTokenFingerprint("manual-contract-token"), ExpiresAt: time.Now().Unix() + 3600}
	require.NoError(t, manual.SetScopes([]string{"profile:read"}))
	require.NoError(t, model.CreateUserAccessToken(u.Id, manual, 20))
	first, code := deviceLoginTestCodes(t)
	approveDeviceLoginTest(t, identity, code)
	firstPAT := redeemDeviceLoginTest(t, first)
	firstEntry, err := model.FindUserAccessTokenByHash(model.AccessTokenFingerprint(firstPAT))
	require.NoError(t, err)
	require.NotNil(t, firstEntry)
	_, err = model.RenameUserAccessToken(u.Id, firstEntry.Id, "renamed device")
	require.NoError(t, err)
	second, code := deviceLoginTestCodes(t)
	approveDeviceLoginTest(t, identity, code)
	secondPAT := redeemDeviceLoginTest(t, second)
	assert.NotEqual(t, firstPAT, secondPAT)
	old, err := model.FindUserAccessTokenByHash(model.AccessTokenFingerprint(firstPAT))
	require.NoError(t, err)
	assert.Nil(t, old, "old slot credential must not remain valid")
	kept, err := model.GetUserAccessToken(u.Id, manual.Id)
	require.NoError(t, err)
	assert.Equal(t, manual.TokenHash, kept.TokenHash)
	var current model.User
	require.NoError(t, model.DB.First(&current, u.Id).Error)
	assert.EqualValues(t, 2, current.AccessTokenGeneration)
	entries, err := model.ListUserAccessTokens(u.Id)
	require.NoError(t, err)
	assert.Len(t, entries, 2)

	// An actual native-authenticated request succeeds with the minted grant.
	router := gin.New()
	router.GET("/api/user/self", middleware.UserAuth(), func(c *gin.Context) {
		assert.Equal(t, u.Id, c.GetInt("id"))
		c.Status(http.StatusNoContent)
	})
	req := httptest.NewRequest("GET", "/api/user/self", nil)
	req.Header.Set("Authorization", "Bearer "+secondPAT)
	req.Header.Set("New-Api-User", fmt.Sprint(u.Id))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestDeviceLoginNativeSlotRevokeBlocksPendingApproval(t *testing.T) {
	u, identity := setupSecurityEnrollmentTest(t)
	first, code := deviceLoginTestCodes(t)
	approveDeviceLoginTest(t, identity, code)
	pat := redeemDeviceLoginTest(t, first)
	entry, err := model.FindUserAccessTokenByHash(model.AccessTokenFingerprint(pat))
	require.NoError(t, err)
	pending, code := deviceLoginTestCodes(t)
	approveDeviceLoginTest(t, identity, code)
	_, err = model.DeleteUserAccessToken(u.Id, entry.Id)
	require.NoError(t, err)
	rec := deviceLoginJSON(t, "POST", "/device/token", deviceTokenRequest{DeviceCode: pending}, nil)
	assert.Contains(t, rec.Body.String(), `"code":"invalid_grant"`)
	var current model.User
	require.NoError(t, model.DB.First(&current, u.Id).Error)
	assert.EqualValues(t, 2, current.AccessTokenGeneration)
	entries, err := model.ListUserAccessTokens(u.Id)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestDeviceLoginNativeCreateFailureRollsBackReplacement(t *testing.T) {
	u, identity := setupSecurityEnrollmentTest(t)
	first, code := deviceLoginTestCodes(t)
	approveDeviceLoginTest(t, identity, code)
	pat := redeemDeviceLoginTest(t, first)
	pending, code := deviceLoginTestCodes(t)
	approveDeviceLoginTest(t, identity, code)
	require.NoError(t, model.DB.Callback().Create().Before("gorm:create").Register("device_native_create_failure", func(tx *gorm.DB) {
		if tx.Statement.Table == "user_access_tokens" {
			tx.AddError(errors.New("native PAT insertion failed"))
		}
	}))
	t.Cleanup(func() { _ = model.DB.Callback().Create().Remove("device_native_create_failure") })
	rec := deviceLoginJSON(t, "POST", "/device/token", deviceTokenRequest{DeviceCode: pending}, nil)
	assert.Contains(t, rec.Body.String(), `"success":false`)
	flow, err := model.GetAuthFlow(pending, model.AuthFlowMatch{Purpose: model.AuthFlowPurposeCliDevice})
	require.NoError(t, err)
	assert.Nil(t, flow.ConsumedAt)
	old, err := model.FindUserAccessTokenByHash(model.AccessTokenFingerprint(pat))
	require.NoError(t, err)
	require.NotNil(t, old, "failed creation must restore replaced slot")
	var current model.User
	require.NoError(t, model.DB.First(&current, u.Id).Error)
	assert.EqualValues(t, 1, current.AccessTokenGeneration)
	require.NoError(t, model.DB.Callback().Create().Remove("device_native_create_failure"))
	redeemDeviceLoginTest(t, pending)
}

func TestDeviceLoginProofBindsExactDeviceGrant(t *testing.T) {
	for _, change := range []string{"device", "user code", "scopes", "expiry", "manual scope"} {
		t.Run(change, func(t *testing.T) {
			_, identity := setupSecurityEnrollmentTest(t)
			device, code := deviceLoginTestCodes(t)
			operation := deviceLoginTestOperation(t, code)
			var grant service.CliDeviceApproveContext
			require.NoError(t, common.Unmarshal(operation.Context, &grant))
			switch change {
			case "device":
				grant.DeviceFlowID++
			case "user code":
				grant.UserCode = "BCDF2345"
			case "scopes":
				grant.Scopes = []string{"profile:read"}
			case "expiry":
				grant.ExpiresAt++
			case "manual scope":
				operation.Scope = service.VerificationScopeAccessTokenGenerate
			}
			var err error
			if change == "manual scope" {
				operation.Context, err = common.Marshal(service.AccessTokenGenerateContext{Scopes: grant.Scopes, ExpiresAt: grant.ExpiresAt})
			} else {
				operation.Context, err = common.Marshal(grant)
			}
			require.NoError(t, err)
			proof := issueSecurityEnrollmentProof(t, identity, operation, service.VerificationMethodPassword)
			body, err := common.Marshal(deviceUserCodeRequest{UserCode: code})
			require.NoError(t, err)
			rec := securityEnrollmentRequest("POST", "/device/approve", string(body), proof, identity, ApproveDeviceLogin)
			assert.Equal(t, http.StatusForbidden, rec.Code)
			flow, err := model.GetAuthFlow(device, model.AuthFlowMatch{Purpose: model.AuthFlowPurposeCliDevice})
			require.NoError(t, err)
			assert.Equal(t, model.AuthFlowIntentPending, flow.Intent)
		})
	}
}

func TestDeviceLoginExpiredSlotCleanupAdvancesGeneration(t *testing.T) {
	u, identity := setupSecurityEnrollmentTest(t)
	device, code := deviceLoginTestCodes(t)
	approveDeviceLoginTest(t, identity, code)
	pat := redeemDeviceLoginTest(t, device)
	entry, err := model.FindUserAccessTokenByHash(model.AccessTokenFingerprint(pat))
	require.NoError(t, err)
	require.NoError(t, model.DB.Model(entry).Update("expires_at", time.Now().Unix()-1).Error)
	require.NoError(t, model.DeleteExpiredUserAccessTokens(time.Now().Unix()))
	var current model.User
	require.NoError(t, model.DB.First(&current, u.Id).Error)
	assert.EqualValues(t, 2, current.AccessTokenGeneration, "native cleanup is a device-slot revocation writer")
}

func TestDeviceLoginLegacyGenerationWriterKeepsABAContract(t *testing.T) {
	u, _ := setupSecurityEnrollmentTest(t)
	for _, change := range []string{"empty revoke", "issue", "revoke", "empty revoke"} {
		var before model.User
		require.NoError(t, model.DB.First(&before, u.Id).Error)
		if change == "issue" {
			require.NoError(t, model.UpdateLegacyUserAccessToken(u.Id, "generation-writer-contract"))
		} else {
			_, err := model.RevokeUserAccessToken(u.Id)
			require.NoError(t, err)
		}
		var after model.User
		require.NoError(t, model.DB.First(&after, u.Id).Error)
		assert.Equal(t, before.AccessTokenGeneration+1, after.AccessTokenGeneration, change)
	}
}

func TestDeviceLoginOldPendingGrantCannotAcquireCreationScopes(t *testing.T) {
	setupDeviceLoginTestDB(t)
	device, code := deviceLoginTestCodes(t)
	flow, err := model.GetAuthFlow(device, model.AuthFlowMatch{Purpose: model.AuthFlowPurposeCliDevice})
	require.NoError(t, err)
	var payload deviceLoginDevicePayload
	require.NoError(t, common.UnmarshalJsonStr(flow.Payload, &payload))
	require.NotNil(t, payload.Grant)
	payload.Grant.Scopes = []string{"api_key:read", "api_key:reveal", "api_key:write", "files:read", "files:write", "hosted:read", "hosted:write", "inspiration:read", "inspiration:write", "profile:read", "social:read", "social:write"}
	encoded, err := common.Marshal(payload)
	require.NoError(t, err)
	require.NoError(t, model.UpdateAuthFlowFields(flow.Id, map[string]interface{}{"payload": string(encoded)}))
	lookup := deviceLoginJSON(t, http.MethodPost, "/api/user/device/code", map[string]string{"user_code": code}, nil)
	assert.Contains(t, lookup.Body.String(), `"success":false`)
	stored, err := model.GetAuthFlow(device, model.AuthFlowMatch{Purpose: model.AuthFlowPurposeCliDevice})
	require.NoError(t, err)
	assert.Equal(t, string(encoded), stored.Payload, "old grant is never silently rewritten with a broader scope")
}
