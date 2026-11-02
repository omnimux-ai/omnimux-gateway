package router

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/storage"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/service/authz"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// accessTokenExemptRoutes are /api routes that never reach dashboard
// authentication, so a personal access token has no rule to satisfy there.
var accessTokenExemptRoutes = []string{
	// Anonymous pages and setup.
	"GET /api/setup",
	"POST /api/setup",
	"GET /api/status",
	"GET /api/uptime/status",
	"GET /api/notice",
	"GET /api/user-agreement",
	"GET /api/privacy-policy",
	"GET /api/about",
	"GET /api/home_page_content",
	"GET /api/ratio_config",
	"GET /api/user/groups",

	// Anonymous account entry points.
	"GET /api/verification",
	"GET /api/reset_password",
	"POST /api/user/reset",
	"POST /api/user/register",
	"GET /api/user/login/encryption-key",
	"POST /api/user/login",
	"POST /api/user/login/2fa",
	"POST /api/user/login/verify",
	"POST /api/user/login/passkey/begin",
	"POST /api/user/login/passkey/finish",
	"POST /api/user/passkey/login/begin",
	"POST /api/user/passkey/login/finish",
	"GET /api/oauth/wechat",
	"GET /api/oauth/telegram/login",
	"GET /api/oauth/telegram/bind/:flow_token",

	// Browser session cookie endpoints.
	"POST /api/user/auth/refresh",
	"POST /api/user/auth/logout",

	// Payment callbacks.
	"POST /api/stripe/webhook",
	"POST /api/creem/webhook",
	"POST /api/waffo/webhook",
	"POST /api/waffo-pancake/webhook/:env",
	"GET /api/user/epay/notify",
	"POST /api/user/epay/notify",
	"GET /api/subscription/epay/notify",
	"POST /api/subscription/epay/notify",
	"GET /api/subscription/epay/return",
	"POST /api/subscription/epay/return",

	// fork: exact anonymous business entry points, never a prefix exemption.
	"POST /api/user/device/code",
	"POST /api/user/device/token",
	"GET /api/social/v1/status",
	"GET /api/social/v1/oauth/tiktok/callback",
	"POST /api/social/v1/oauth/tiktok/callback",
	"GET /api/social/v1/public/media/:token",
	"GET /api/inspiration/v1/status",
	"GET /api/inspiration/v1/categories",
	"GET /api/inspiration/v1/share/:id",
	"GET /api/inspiration/v1/public/categories",
	"GET /api/inspiration/v1/public/inspirations",
	"GET /api/inspiration/v1/public/media/*key",
	"GET /api/hosted/v1/status",
	// fork: public TikTok tool pages; handler-level Turnstile pass and quotas.
	"GET /api/tools/tiktok/config",
	"POST /api/tools/tiktok/verify",
	"POST /api/tools/tiktok/resolve",
	"GET /api/tools/tiktok/file/:kind",
	"GET /api/tools/tiktok/zip",
	// fork: guest browsing — the workbench option dataset and the published
	// inspiration library list are public read; writes still need a session.
	"GET /api/influencer/taxonomy",
	"GET /api/influencer/presets",
	"GET /api/influencer/presets/:id/asset/:role",
	"GET /api/v1/files/download/:file_id",
	"GET /v1/files/download/:file_id",

	// Native relay-key-only file placeholders, not the dual-auth uploads.
	"DELETE /v1/files/:id",
	"GET /v1/files/:id",
	"GET /v1/files/:id/content",

	// API key authentication (TokenAuthReadOnly).
	"GET /api/usage/token/",
	"GET /api/log/token",
}

// accessTokenHelperRoutes are declared through handlePermissionRoute outside
// channelPermissionRoutes.
var accessTokenHelperRoutes = map[string]authz.Permission{
	"GET /api/task_plugin_options": authz.TaskPluginBind,
	"GET /api/audit":               authz.AuditRead,
}

func newAccessTokenScopeTestEngine(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	// A permission route that is also in the static table panics here.
	require.NotPanics(t, func() {
		SetApiRouter(engine)
		SetDashboardRouter(engine)
		SetRelayRouter(engine)
		SetTaskPluginProtocolRouter(engine)
		SetVideoRouter(engine)
		SetTaskRouter(engine)
	})
	return engine
}

func TestAccessTokenRouteRulesCoverEveryDashboardRoute(t *testing.T) {
	engine := newAccessTokenScopeTestEngine(t)
	exempt := map[string]bool{}
	for _, key := range accessTokenExemptRoutes {
		exempt[key] = true
	}
	registered := map[string]bool{}
	panel := map[string]bool{}
	for _, route := range engine.Routes() {
		if !strings.HasPrefix(route.Path, "/api/") && !strings.HasPrefix(route.Path, "/pg/") && !strings.HasPrefix(route.Path, "/v1/files/") {
			continue
		}
		key := route.Method + " " + route.Path
		registered[key] = true
		if !exempt[key] {
			panel[key] = true
		}
	}
	for _, key := range accessTokenExemptRoutes {
		assert.True(t, registered[key], "exempt route %s is no longer registered", key)
	}

	declared := map[string]int{}
	for _, key := range middleware.AccessTokenRouteRuleKeys() {
		declared[key]++
	}
	for key := range panel {
		assert.Equal(t, 1, declared[key], "panel route %s must be declared exactly once", key)
	}
	for key := range declared {
		assert.True(t, panel[key], "declared route %s is not a registered panel route", key)
	}

	permissionRoutes := map[string]authz.Permission{}
	for _, route := range channelPermissionRoutes {
		permissionRoutes[route.method+" "+joinPaths("/api/channel", route.path)] = route.permission
	}
	for key, permission := range accessTokenHelperRoutes {
		permissionRoutes[key] = permission
	}
	for key, permission := range permissionRoutes {
		rule, ok := middleware.AccessTokenRouteRule(key)
		require.True(t, ok, key)
		assert.Equal(t, "scope", rule.Kind(), key)
		assert.Equal(t, service.AccessTokenScopeOf(permission), rule.Scope(), key)
	}
}

func TestAccessTokenCatalogCoversEveryPermission(t *testing.T) {
	catalog := service.AccessTokenCatalog(1, common.RoleRootUser)
	scopes := map[string]bool{}
	resources := map[string]int{}
	for _, group := range catalog {
		for _, resource := range group.Resources {
			resources[resource.Resource]++
			for _, action := range resource.Actions {
				scopes[resource.Resource+":"+action.Action] = true
			}
		}
	}
	for _, permission := range authz.AllPermissions() {
		assert.True(t, scopes[service.AccessTokenScopeOf(permission)], "root catalog lacks %s", service.AccessTokenScopeOf(permission))
	}
	for resource, count := range resources {
		assert.Equal(t, 1, count, "resource %s appears in more than one catalog entry", resource)
	}
}

func TestAccessTokenVerificationScopesAreAllListed(t *testing.T) {
	files, err := filepath.Glob("../service/*.go")
	require.NoError(t, err)
	fileSet := token.NewFileSet()
	var scopes []string
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		parsed, err := parser.ParseFile(fileSet, file, nil, 0)
		require.NoError(t, err)
		for _, declaration := range parsed.Decls {
			general, ok := declaration.(*ast.GenDecl)
			if !ok || general.Tok != token.CONST {
				continue
			}
			for _, spec := range general.Specs {
				value := spec.(*ast.ValueSpec)
				for i, name := range value.Names {
					if !strings.HasPrefix(name.Name, "VerificationScope") || i >= len(value.Values) {
						continue
					}
					literal, ok := value.Values[i].(*ast.BasicLit)
					require.True(t, ok, name.Name)
					scope, err := strconv.Unquote(literal.Value)
					require.NoError(t, err)
					scopes = append(scopes, scope)
				}
			}
		}
	}
	require.NotEmpty(t, scopes)
	for _, scope := range scopes {
		_, listed := service.AccessTokenVerificationScope(scope)
		assert.True(t, listed, "verification scope %s has no access token rule", scope)
	}
}

func newRouterScopedCredential(t *testing.T, userID int, scopes ...string) (string, *model.UserAccessToken) {
	t.Helper()
	suffix, err := common.GenerateRandomCharsKey(43)
	require.NoError(t, err)
	raw := model.AccessTokenPrefix + suffix
	token := &model.UserAccessToken{Name: "route contract", TokenHash: model.AccessTokenFingerprint(raw), TokenHint: model.AccessTokenHint(raw)}
	require.NoError(t, token.SetScopes(scopes))
	require.NoError(t, model.CreateUserAccessToken(userID, token, service.AccessTokenMaxPerUser))
	return raw, token
}

func TestAccessTokenLogExportsRespectScopesAndAccountRole(t *testing.T) {
	previousDB, previousLogDB := model.DB, model.LOG_DB
	previousMain, previousLog := common.MainDatabaseType(), common.LogDatabaseType()
	previousRedis, previousMemory := common.RedisEnabled, common.MemoryCacheEnabled
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	model.DB, model.LOG_DB = db, db
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	common.RedisEnabled, common.MemoryCacheEnabled = false, false
	t.Cleanup(func() {
		assert.NoError(t, sqlDB.Close())
		model.DB, model.LOG_DB = previousDB, previousLogDB
		common.SetDatabaseTypes(previousMain, previousLog)
		common.RedisEnabled, common.MemoryCacheEnabled = previousRedis, previousMemory
	})
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.UserSession{}, &model.UserAccessToken{}, &model.Option{}, &model.AuditLog{}, &model.Log{}, &model.Channel{}))
	require.NoError(t, model.EnsureLegacyAccessTokenRetireAt(time.Now().Unix()))
	t.Setenv("LOG_SQL_DSN", "")
	previousMaster := common.IsMasterNode
	common.IsMasterNode = false
	err = model.InitLogDB()
	common.IsMasterNode = previousMaster
	require.NoError(t, err)
	logSetting := operation_setting.GetLogSetting()
	previousLogSetting := *logSetting
	logSetting.UserExportEnabled = true
	t.Cleanup(func() { *logSetting = previousLogSetting })

	user := &model.User{Username: "log-export-owner", Password: "placeholder", AffCode: "export-owner-aff", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: "default", AuthVersion: 1}
	admin := &model.User{Username: "log-export-admin", Password: "placeholder", AffCode: "export-admin-aff", Role: common.RoleAdminUser, Status: common.UserStatusEnabled, Group: "default", AuthVersion: 1}
	require.NoError(t, db.Create(user).Error)
	require.NoError(t, db.Create(admin).Error)
	usage, _ := newRouterScopedCredential(t, user.Id, "usage:read")
	profile, _ := newRouterScopedCredential(t, user.Id, "profile:read")
	// Model insertion intentionally simulates a stale or forged admin grant:
	// the request must still check the account's current role.
	forgedLog, _ := newRouterScopedCredential(t, user.Id, "log:read")
	adminLog, _ := newRouterScopedCredential(t, admin.Id, "log:read")
	adminUsage, _ := newRouterScopedCredential(t, admin.Id, "usage:read")
	now := time.Now().Unix()
	for _, log := range []model.Log{
		{UserId: user.Id, Username: user.Username, Type: model.LogTypeConsume, CreatedAt: now - 10, RequestId: "export-own-row", Group: "default"},
		{UserId: admin.Id, Username: admin.Username, Type: model.LogTypeConsume, CreatedAt: now - 5, RequestId: "export-admin-row", Group: "default"},
	} {
		require.NoError(t, db.Create(&log).Error)
	}
	engine := newAccessTokenScopeTestEngine(t)
	request := func(path, credential string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		if credential != "" {
			req.Header.Set("Authorization", "Bearer "+credential)
		}
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, req)
		return rec
	}
	for _, tc := range []struct {
		name, path, credential, code string
		status                       int
		requestIDs                   []string
	}{
		{name: "personal export requires authentication", path: "/api/log/self/export", status: http.StatusUnauthorized},
		{name: "all export requires authentication", path: "/api/log/export", status: http.StatusUnauthorized},
		{name: "personal usage grant exports own logs", path: "/api/log/self/export", credential: usage, status: http.StatusOK, requestIDs: []string{"export-own-row"}},
		{name: "personal export rejects unrelated grant", path: "/api/log/self/export", credential: profile, status: http.StatusForbidden, code: "ACCESS_TOKEN_SCOPE_DENIED"},
		{name: "personal export does not accept management grant", path: "/api/log/self/export", credential: forgedLog, status: http.StatusForbidden, code: "ACCESS_TOKEN_SCOPE_DENIED"},
		{name: "personal usage grant cannot export all logs", path: "/api/log/export", credential: usage, status: http.StatusForbidden, code: "AUTH_INSUFFICIENT_PRIVILEGE"},
		{name: "ordinary account cannot elevate with management grant", path: "/api/log/export", credential: forgedLog, status: http.StatusForbidden, code: "AUTH_INSUFFICIENT_PRIVILEGE"},
		{name: "administrator still needs management grant", path: "/api/log/export", credential: adminUsage, status: http.StatusForbidden, code: "ACCESS_TOKEN_SCOPE_DENIED"},
		{name: "administrator management grant exports all logs", path: "/api/log/export", credential: adminLog, status: http.StatusOK, requestIDs: []string{"export-admin-row", "export-own-row"}},
		{name: "administrator personal grant stays personal", path: "/api/log/self/export", credential: adminUsage, status: http.StatusOK, requestIDs: []string{"export-admin-row"}},
		{name: "administrator management grant cannot replace personal grant", path: "/api/log/self/export", credential: adminLog, status: http.StatusForbidden, code: "ACCESS_TOKEN_SCOPE_DENIED"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := request(tc.path, tc.credential)
			require.Equal(t, tc.status, rec.Code, rec.Body.String())
			if tc.status != http.StatusOK {
				if tc.code != "" {
					assert.Contains(t, rec.Body.String(), `"code":"`+tc.code+`"`)
				}
				assert.NotContains(t, rec.Body.String(), "export-own-row")
				assert.NotContains(t, rec.Body.String(), "export-admin-row")
				return
			}
			assert.Equal(t, "application/x-ndjson; charset=utf-8", rec.Header().Get("Content-Type"))
			assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
			lines := strings.Split(strings.TrimSpace(rec.Body.String()), "\n")
			require.Len(t, lines, len(tc.requestIDs)+1, rec.Body.String())
			for i, id := range tc.requestIDs {
				var entry model.Log
				require.NoError(t, common.UnmarshalJsonStr(lines[i], &entry))
				assert.Equal(t, id, entry.RequestId)
			}
			var result struct {
				Success bool `json:"success"`
				Data    struct {
					Count int `json:"count"`
				} `json:"data"`
			}
			require.NoError(t, common.UnmarshalJsonStr(lines[len(lines)-1], &result))
			assert.True(t, result.Success)
			assert.Equal(t, len(tc.requestIDs), result.Data.Count)
		})
	}

	require.NoError(t, db.Model(admin).Update("role", common.RoleCommonUser).Error)
	rec := request("/api/log/export", adminLog)
	assert.Equal(t, http.StatusForbidden, rec.Code, "existing token cannot retain a revoked administrator role")
	assert.Contains(t, rec.Body.String(), "AUTH_INSUFFICIENT_PRIVILEGE")
	assert.NotContains(t, rec.Body.String(), "export-own-row")
	assert.NotContains(t, rec.Body.String(), "export-admin-row")
}

func TestAccessTokenRealFileAliasesAndZeroSideEffects(t *testing.T) {
	previousDB, previousLogDB, previousRedis, previousType := model.DB, model.LOG_DB, common.RedisEnabled, common.MainDatabaseType()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	model.DB, model.LOG_DB, common.RedisEnabled = db, db, false
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, sqlDB.Close())
		model.DB, model.LOG_DB, common.RedisEnabled = previousDB, previousLogDB, previousRedis
		common.SetMainDatabaseType(previousType)
	})
	require.NoError(t, model.DB.AutoMigrate(&model.User{}, &model.Token{}, &model.UserAccessToken{}, &model.Option{}, &model.AuditLog{}, &model.File{}, &model.UserSession{}))
	require.NoError(t, model.EnsureLegacyAccessTokenRetireAt(time.Now().Unix()))
	previousMaster := common.IsMasterNode
	common.IsMasterNode = false
	require.NoError(t, model.InitLogDB()) // Initialize native reserved-column quoting.
	common.IsMasterNode = previousMaster
	previousDriver, previousAddress := storage.GetStorageDriver(), system_setting.ServerAddress
	previousSecret := common.SessionSecret
	storageDir := t.TempDir()
	storage.SetGlobalDriver(storage.NewLocalStorageDriver(storageDir))
	system_setting.ServerAddress = "https://scope.example"
	common.SessionSecret = "scope-router-isolated-secret"
	t.Cleanup(func() {
		storage.SetGlobalDriver(previousDriver)
		system_setting.ServerAddress = previousAddress
		common.SessionSecret = previousSecret
	})
	legacy := "scope-real-legacy"
	user := &model.User{Username: "scope-real-owner", Password: "placeholder", AffCode: "scope-aff", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: "default", AuthVersion: 1, AccessToken: &legacy, Quota: 12345}
	require.NoError(t, model.DB.Create(user).Error)
	read, readRow := newRouterScopedCredential(t, user.Id, "files:read")
	write, writeRow := newRouterScopedCredential(t, user.Id, "files:write")
	profile, _ := newRouterScopedCredential(t, user.Id, "profile:read")
	session := &model.UserSession{SID: "scope-file-session", UserID: user.Id, Version: 1, UserAuthVersion: 1, Status: model.UserSessionStatusActive, RefreshHash: "scope-refresh", LoginMethod: "password", LastActiveAt: time.Now().Unix(), ExpiresAt: time.Now().Unix() + 3600}
	require.NoError(t, model.CreateUserSession(session))
	identity := service.AuthIdentity{UserID: user.Id, SessionID: session.SID, UserAuthVersion: 1, SessionVersion: 1}
	browser, _, err := service.IssueAccessToken(identity)
	require.NoError(t, err)
	relayKey := "scopefilenativerelay"
	require.NoError(t, model.DB.Create(&model.Token{UserId: user.Id, Key: relayKey, Status: common.TokenStatusEnabled, ExpiredTime: -1, UnlimitedQuota: true}).Error)
	engine := newAccessTokenScopeTestEngine(t)
	request := func(method, path, credential, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if credential != "" {
			req.Header.Set("Authorization", "Bearer "+credential)
		}
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, req)
		return rec
	}
	// Real registered private handlers are rejected before request decoding,
	// sidecar access, task distribution, or native accounting.
	for _, route := range []struct{ method, path string }{
		{http.MethodPost, "/api/social/v1/posts"}, {http.MethodPost, "/api/hosted/v1/orders"},
		{http.MethodPost, "/api/inspiration/v1/share"}, {http.MethodPost, "/api/inspiration/v1/publish"},
		{http.MethodPost, "/api/inspiration/v1/inspirations/1/share"},
	} {
		rec := request(route.method, route.path, profile, `{}`)
		assert.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
		assert.Contains(t, rec.Body.String(), "ACCESS_TOKEN_SCOPE_DENIED")
	}
	influencer, _ := newRouterScopedCredential(t, user.Id, "influencer:read")
	rec := request(http.MethodGet, "/api/influencer/taxonomy", influencer, "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"success":true`)
	// Guest browsing: the read endpoints answer without any credential too.
	// The preset store may not exist in the test DB, so anonymous access only
	// asserts the request is not refused for missing auth.
	rec = request(http.MethodGet, "/api/influencer/taxonomy", "", "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"success":true`)
	rec = request(http.MethodGet, "/api/influencer/presets", "", "")
	assert.NotContains(t, []int{http.StatusUnauthorized, http.StatusForbidden}, rec.Code, rec.Body.String())
	for _, path := range []string{"/pg/chat/completions", "/pg/video/generations", "/pg/images/generations", "/pg/images/edits", "/pg/influencer/sheet", "/pg/influencer/multiview", "/pg/influencer/motion", "/api/user/device/approve", "/api/user/device/deny"} {
		for _, credential := range []string{write, legacy} {
			rec := request(http.MethodPost, path, credential, `{}`)
			assert.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
			assert.Contains(t, rec.Body.String(), "AUTH_SESSION_REQUIRED")
		}
	}
	for _, path := range []string{"/api/social/v1/status", "/api/hosted/v1/status", "/api/inspiration/v1/status"} {
		assert.Equal(t, http.StatusOK, request(http.MethodGet, path, "", "").Code)
	}
	payload := `{"base64_data":"aGVsbG8=","file_name":"contract.txt"}`
	for _, prefix := range []string{"/api/v1/files", "/v1/files"} {
		for _, credential := range []string{read, legacy, browser, relayKey} {
			rec := request(http.MethodGet, prefix+"/quota", credential, "")
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			assert.Contains(t, rec.Body.String(), `"success":true`)
		}
		rec := request(http.MethodPost, prefix+"/upload/base64", write, payload)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		assert.Contains(t, rec.Body.String(), `"success":true`)
		for _, denied := range []struct{ method, suffix, credential string }{
			{http.MethodGet, "/quota", profile},
			{http.MethodPost, "/upload/base64", read},
			{http.MethodPost, "/upload/stream", read},
			{http.MethodPost, "/upload/url", read},
			{http.MethodPost, "/upload/presign", read},
			{http.MethodPost, "/upload/confirm", read},
		} {
			rec = request(denied.method, prefix+denied.suffix, denied.credential, payload)
			require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
			assert.Contains(t, rec.Body.String(), "ACCESS_TOKEN_SCOPE_DENIED")
		}
	}
	var filesBefore int64
	require.NoError(t, model.DB.Model(&model.File{}).Count(&filesBefore).Error)
	assert.EqualValues(t, 2, filesBefore)
	objectCount := 0
	require.NoError(t, filepath.WalkDir(storageDir, func(_ string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			objectCount++
		}
		return err
	}))
	assert.Equal(t, 2, objectCount, "denied uploads must not touch storage")
	var current model.User
	require.NoError(t, model.DB.First(&current, user.Id).Error)
	assert.Equal(t, user.Quota, current.Quota, "auth rejection cannot charge quota")
	require.NoError(t, model.DB.First(readRow, readRow.Id).Error)
	assert.Positive(t, readRow.LastUsedAt)
	require.NoError(t, model.DB.First(writeRow, writeRow.Id).Error)
	assert.Positive(t, writeRow.LastUsedAt)
	for _, prefix := range []string{"/api/v1/files", "/v1/files"} {
		var audit model.AuditLog
		require.NoError(t, model.DB.Where("category = ? AND token_ref = ? AND route = ?", model.AuditCategoryAccessToken, readRow.TokenHash, prefix+"/upload/confirm").Order("id desc").First(&audit).Error)
		assert.Equal(t, http.StatusForbidden, audit.Status)
		require.NotNil(t, audit.Other.Op)
		params, err := common.Marshal(audit.Other.Op.Params)
		require.NoError(t, err)
		assert.Contains(t, string(params), `"failure_reason":"scope_denied"`)
		assert.NotContains(t, string(params), read)
	}
	// Revocation and expiry deny before the same real handler on both aliases.
	_, err = model.DeleteUserAccessToken(user.Id, writeRow.Id)
	require.NoError(t, err)
	require.NoError(t, model.DB.Model(readRow).Update("expires_at", time.Now().Unix()-1).Error)
	for _, prefix := range []string{"/api/v1/files", "/v1/files"} {
		assert.Equal(t, http.StatusUnauthorized, request(http.MethodPost, prefix+"/upload/base64", write, payload).Code)
		assert.Equal(t, http.StatusUnauthorized, request(http.MethodGet, prefix+"/quota", read, "").Code)
	}
	var filesAfter int64
	require.NoError(t, model.DB.Model(&model.File{}).Count(&filesAfter).Error)
	assert.Equal(t, filesBefore, filesAfter)
	// The global legacy deadline must not retire unrelated native relay keys.
	require.NoError(t, model.DB.Model(&model.Option{}).Where("key = ?", "LegacyAccessTokenRetireAt").Update("value", time.Now().Unix()-1).Error)
	require.NoError(t, model.EnsureLegacyAccessTokenRetireAt(time.Now().Unix()))
	for _, prefix := range []string{"/api/v1/files", "/v1/files"} {
		assert.Equal(t, http.StatusOK, request(http.MethodGet, prefix+"/quota", relayKey, "").Code)
		rec := request(http.MethodGet, prefix+"/quota", legacy, "")
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
		assert.Contains(t, rec.Body.String(), "ACCESS_TOKEN_LEGACY_RETIRED")
		req := httptest.NewRequest(http.MethodGet, prefix+"/quota", nil)
		req.Header.Set("mj-api-secret", relayKey)
		rec = httptest.NewRecorder()
		engine.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	}
}

func TestAccessTokenPersonalGrantsNeverIncludeAdminAndPlayground(t *testing.T) {
	for _, role := range []int{common.RoleCommonUser, common.RoleAdminUser} {
		for _, scope := range []string{"social:read", "social:write", "hosted:read", "hosted:write", "files:read", "files:write", "inspiration:read", "inspiration:write", "influencer:read"} {
			granted, err := service.NormalizeAccessTokenScopes(1, role, []string{scope})
			require.NoError(t, err)
			assert.Equal(t, []string{scope}, granted)
		}
	}
	for _, scope := range []string{"hosted_admin:read", "hosted_admin:write", "inspiration_admin:write"} {
		_, err := service.NormalizeAccessTokenScopes(1, common.RoleCommonUser, []string{scope})
		assert.ErrorIs(t, err, service.ErrAccessTokenScopeForbidden)
	}
	for _, key := range []string{"POST /pg/chat/completions", "POST /pg/video/generations", "POST /pg/images/generations", "POST /pg/images/edits", "POST /pg/influencer/sheet", "POST /pg/influencer/multiview", "POST /pg/influencer/motion", "POST /api/user/device/approve", "POST /api/user/device/deny"} {
		rule, ok := middleware.AccessTokenRouteRule(key)
		require.True(t, ok)
		assert.Equal(t, "session", rule.Kind())
	}
}
