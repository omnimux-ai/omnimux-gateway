package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/service/social"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupUserPurgeTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	previousDB, previousLogDB := model.DB, model.LOG_DB
	previousRedisEnabled := common.RedisEnabled
	previousMainDatabaseType, previousLogDatabaseType := common.MainDatabaseType(), common.LogDatabaseType()
	previousSocial := socialSvc
	common.RedisEnabled = false
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB, model.LOG_DB = db, db
	require.NoError(t, db.AutoMigrate(
		&model.User{},
		&model.UserSession{},
		&model.UserAccessToken{},
		&model.Log{},
		&model.TwoFA{},
		&model.TwoFABackupCode{},
		&model.PasskeyCredential{},
		&model.Token{},
		&model.AuthFlow{},
		&model.UserOAuthBinding{},
		&model.ExternalIdentityClaim{},
		&model.AuditLog{},
		&model.SocialAccount{},
		&model.SocialSeat{},
	))

	t.Cleanup(func() {
		setSocialService(previousSocial)
		model.DB, model.LOG_DB = previousDB, previousLogDB
		common.RedisEnabled = previousRedisEnabled
		common.SetDatabaseTypes(previousMainDatabaseType, previousLogDatabaseType)
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func seedPurgeUser(t *testing.T, db *gorm.DB, username string, role int) *model.User {
	t.Helper()
	user := &model.User{
		Username:    username,
		Password:    "password1",
		Role:        role,
		Status:      common.UserStatusEnabled,
		Group:       "default",
		AuthVersion: 1,
		AffCode:     username + "-aff",
	}
	require.NoError(t, db.Create(user).Error)
	return user
}

func seedOccupiedSeat(t *testing.T, db *gorm.DB, userId int, providerAccountID string) *model.SocialSeat {
	t.Helper()
	acc := &model.SocialAccount{
		UserId:            userId,
		Provider:          social.ProviderZernio,
		ProviderAccountID: providerAccountID,
		Platform:          "twitter",
		Status:            model.SocialAccountStatusActive,
	}
	require.NoError(t, db.Create(acc).Error)
	seat := &model.SocialSeat{
		UserId:            userId,
		Status:            model.SocialSeatStatusOccupied,
		ExpiresAt:         time.Now().UTC().Add(10 * 24 * time.Hour),
		OccupiedAccountID: &acc.ID,
		OpenedQuota:       4_000_000,
	}
	require.NoError(t, db.Create(seat).Error)
	return seat
}

func performDeleteUser(t *testing.T, targetID, operatorRole int) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	idStr := strconv.Itoa(targetID)
	c.Request = httptest.NewRequest(http.MethodDelete, "/api/user/"+idStr, nil)
	c.Params = gin.Params{{Key: "id", Value: idStr}}
	operator := seedPurgeUser(t, model.DB, "purge-operator", operatorRole)
	bundle, err := service.CreateLoginSession(operator.Id, "password", "127.0.0.1", "purge-test")
	require.NoError(t, err)
	identity, err := service.ParseAccessToken(bundle.AccessToken)
	require.NoError(t, err)
	context, err := common.Marshal(service.AdminUserContext{UserID: targetID})
	require.NoError(t, err)
	proof := issueSecurityEnrollmentProof(t, identity, service.VerificationOperation{Scope: service.VerificationScopeAdminUserDelete, Context: context}, service.VerificationMethodPassword)
	c.Request.Header.Set("X-Security-Proof", proof)
	c.Set("id", identity.UserID)
	c.Set("role", operatorRole)
	c.Set("username", operator.Username)
	c.Set("session_id", identity.SessionID)
	c.Set("auth_version", identity.UserAuthVersion)
	c.Set("session_version", identity.SessionVersion)
	DeleteUser(c)
	return recorder
}

func performDeleteSelf(t *testing.T, userID int) *httptest.ResponseRecorder {
	t.Helper()
	bundle, err := service.CreateLoginSession(userID, "password", "127.0.0.1", "test")
	require.NoError(t, err)
	identity, err := service.ParseAccessToken(bundle.AccessToken)
	require.NoError(t, err)
	operation := service.VerificationOperation{Scope: service.VerificationScopeAccountDelete}
	proof := issueSecurityEnrollmentProof(t, identity, operation, service.VerificationMethodPassword)
	return securityEnrollmentRequest(http.MethodDelete, "/api/user/self", "", proof, identity, DeleteSelf)
}

func TestDeleteUserPurgesSocialSeatsBeforeHardDelete(t *testing.T) {
	db := setupUserPurgeTestDB(t)
	setSocialService(social.NewService(social.Config{}))

	target := seedPurgeUser(t, db, "purge-hard-delete", common.RoleCommonUser)
	seat := seedOccupiedSeat(t, db, target.Id, "acc_purge_hard")

	recorder := performDeleteUser(t, target.Id, common.RoleAdminUser)
	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"success":true`)

	var got model.SocialSeat
	require.NoError(t, db.First(&got, seat.ID).Error)
	assert.Equal(t, model.SocialSeatStatusExpired, got.Status)
	assert.Nil(t, got.OccupiedAccountID)

	var remaining int64
	require.NoError(t, db.Unscoped().Model(&model.User{}).Where("id = ?", target.Id).Count(&remaining).Error)
	assert.EqualValues(t, 0, remaining)
}

func TestDeleteSelfPurgesSocialSeatsBeforeSoftDelete(t *testing.T) {
	db := setupUserPurgeTestDB(t)
	setSocialService(social.NewService(social.Config{}))

	user := seedPurgeUser(t, db, "purge-self-delete", common.RoleCommonUser)
	seat := seedOccupiedSeat(t, db, user.Id, "acc_purge_self")

	recorder := performDeleteSelf(t, user.Id)
	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"success":true`)

	var got model.SocialSeat
	require.NoError(t, db.First(&got, seat.ID).Error)
	assert.Equal(t, model.SocialSeatStatusExpired, got.Status)
	assert.Nil(t, got.OccupiedAccountID)

	var deletedCount int64
	require.NoError(t, db.Unscoped().Model(&model.User{}).Where("id = ? AND deleted_at IS NOT NULL", user.Id).Count(&deletedCount).Error)
	assert.EqualValues(t, 1, deletedCount)
}

func TestDeleteUserPurgeFailureDoesNotBlockHardDelete(t *testing.T) {
	db := setupUserPurgeTestDB(t)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"error":"zernio down"}`))
	}))
	t.Cleanup(upstream.Close)
	setSocialService(social.NewService(social.Config{
		APIKey:  "test-key",
		BaseURL: upstream.URL,
	}))

	target := seedPurgeUser(t, db, "purge-fail-open", common.RoleCommonUser)
	seat := seedOccupiedSeat(t, db, target.Id, "acc_purge_fail")

	recorder := performDeleteUser(t, target.Id, common.RoleAdminUser)
	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"success":true`)

	var remaining int64
	require.NoError(t, db.Unscoped().Model(&model.User{}).Where("id = ?", target.Id).Count(&remaining).Error)
	assert.EqualValues(t, 0, remaining)

	var got model.SocialSeat
	require.NoError(t, db.First(&got, seat.ID).Error)
	assert.Equal(t, model.SocialSeatStatusOccupied, got.Status)
}

func TestDeleteUserWithoutProofDoesNotPurgeSeatsOrCallProvider(t *testing.T) {
	db := setupUserPurgeTestDB(t)
	var calls atomic.Int64
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(upstream.Close)
	setSocialService(social.NewService(social.Config{APIKey: "local-test-key", BaseURL: upstream.URL}))
	target := seedPurgeUser(t, db, "proof-block-target", common.RoleCommonUser)
	seat := seedOccupiedSeat(t, db, target.Id, "local-proof-account")
	operator := seedPurgeUser(t, db, "proof-block-operator", common.RoleAdminUser)
	bundle, err := service.CreateLoginSession(operator.Id, "password", "127.0.0.1", "proof-block-test")
	require.NoError(t, err)
	identity, err := service.ParseAccessToken(bundle.AccessToken)
	require.NoError(t, err)
	response := adminUserRequest(http.MethodDelete, "/api/user/"+strconv.Itoa(target.Id), "", "", identity, common.RoleAdminUser, gin.Params{{Key: "id", Value: strconv.Itoa(target.Id)}}, DeleteUser)
	assert.Equal(t, http.StatusForbidden, response.Code)
	assert.Contains(t, response.Body.String(), "SECURITY_PROOF_REQUIRED")
	assert.Zero(t, calls.Load(), "missing proof must stop external account cleanup")
	var got model.SocialSeat
	require.NoError(t, db.First(&got, seat.ID).Error)
	assert.Equal(t, model.SocialSeatStatusOccupied, got.Status)
	assert.Equal(t, seat.OccupiedAccountID, got.OccupiedAccountID)
	_, err = model.GetUserById(target.Id, false)
	assert.NoError(t, err)
}

func TestDeleteUserSkipsPurgeWhenRoleTooLow(t *testing.T) {
	db := setupUserPurgeTestDB(t)
	setSocialService(social.NewService(social.Config{}))

	target := seedPurgeUser(t, db, "purge-role-block", common.RoleAdminUser)
	seat := seedOccupiedSeat(t, db, target.Id, "acc_purge_role")

	recorder := performDeleteUser(t, target.Id, common.RoleAdminUser)
	assert.Contains(t, recorder.Body.String(), `"success":false`)

	var got model.SocialSeat
	require.NoError(t, db.First(&got, seat.ID).Error)
	assert.Equal(t, model.SocialSeatStatusOccupied, got.Status)

	var remaining int64
	require.NoError(t, db.Unscoped().Model(&model.User{}).Where("id = ?", target.Id).Count(&remaining).Error)
	assert.EqualValues(t, 1, remaining)
}
