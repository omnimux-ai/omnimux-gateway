package controller

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// callLogHandler runs a log handler as the given user and returns the JSON
// lines of the response.
func callLogHandler(t *testing.T, handler gin.HandlerFunc, userId int, role int, query string) []map[string]any {
	t.Helper()
	recorder := httptest.NewRecorder()
	runLogHandler(handler, recorder, context.Background(), userId, role, query)
	return decodeLogExportLines(t, recorder.Body.String())
}

func runLogHandler(handler gin.HandlerFunc, writer http.ResponseWriter, requestContext context.Context, userId int, role int, query string) {
	ctx, _ := gin.CreateTestContext(writer)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/log?"+query, nil).WithContext(requestContext)
	ctx.Set("id", userId)
	ctx.Set("role", role)
	handler(ctx)
}

func decodeLogExportLines(t *testing.T, body string) []map[string]any {
	t.Helper()
	if strings.TrimSpace(body) == "" {
		return nil
	}
	var lines []map[string]any
	for line := range strings.SplitSeq(strings.TrimSpace(body), "\n") {
		var value map[string]any
		require.NoError(t, common.UnmarshalJsonStr(line, &value))
		lines = append(lines, value)
	}
	return lines
}

func exportedRequestIds(lines []map[string]any) []any {
	ids := make([]any, 0, len(lines))
	for _, line := range lines[:len(lines)-1] {
		ids = append(ids, line["request_id"])
	}
	return ids
}

// seedExportLogs stores logs of two users: three in the last hour and one
// from 400 days ago.
func seedExportLogs(t *testing.T, mainDB *gorm.DB, logDB *gorm.DB, now int64) {
	t.Helper()
	require.NoError(t, mainDB.Create(&model.Channel{Id: 9, Name: "primary"}).Error)
	logs := []model.Log{
		{UserId: 1, Username: "alice", CreatedAt: now - 3600, Type: model.LogTypeConsume, ModelName: "gpt-a", ChannelId: 9, Quota: 10, RequestId: "a-1", Other: `{"model_price":1.25,"admin_info":{"use_channel":["9"]},"root_info":{"generation":18446744073709551615},"audit_info":{"method":"POST"},"channel_name":"legacy-private","reject_reason":"legacy-rejection"}`},
		{UserId: 2, Username: "bob", CreatedAt: now - 1800, Type: model.LogTypeConsume, ModelName: "gpt-b", ChannelId: 9, Quota: 20, RequestId: "b-1"},
		{UserId: 1, Username: "alice", CreatedAt: now - 60, Type: model.LogTypeError, ModelName: "gpt-a", RequestId: "a-2"},
		{UserId: 1, Username: "alice", CreatedAt: now - 400*24*3600, Type: model.LogTypeConsume, ModelName: "gpt-a", RequestId: "a-old"},
	}
	for i := range logs {
		require.NoError(t, logDB.Create(&logs[i]).Error)
	}
}

func useLogExportDatabases(t *testing.T, mainDB *gorm.DB, logDB *gorm.DB, mainType common.DatabaseType, logType common.DatabaseType) {
	t.Helper()
	previousDB, previousLogDB := model.DB, model.LOG_DB
	previousMain, previousLog := common.MainDatabaseType(), common.LogDatabaseType()
	previousRedis, previousMemory := common.RedisEnabled, common.MemoryCacheEnabled
	model.DB, model.LOG_DB = mainDB, logDB
	common.SetDatabaseTypes(mainType, logType)
	common.RedisEnabled, common.MemoryCacheEnabled = false, false
	t.Cleanup(func() {
		model.DB, model.LOG_DB = previousDB, previousLogDB
		common.SetDatabaseTypes(previousMain, previousLog)
		common.RedisEnabled, common.MemoryCacheEnabled = previousRedis, previousMemory
	})
}

func TestLogExportDatabaseMatrix(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previousMaster := common.IsMasterNode
	common.IsMasterNode = true
	t.Cleanup(func() { common.IsMasterNode = previousMaster })
	for _, tc := range []struct {
		kind, env string
		typ       common.DatabaseType
	}{
		{"sqlite", "", common.DatabaseTypeSQLite},
		{"mysql", "AUDIT_MYSQL_DSN", common.DatabaseTypeMySQL},
		{"postgres", "AUDIT_POSTGRES_DSN", common.DatabaseTypePostgreSQL},
		{"clickhouse", "AUDIT_CLICKHOUSE_DSN", common.DatabaseTypeClickHouse},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			dsn := os.Getenv(tc.env)
			if tc.env != "" && dsn == "" {
				t.Skip(tc.env + " is not configured")
			}
			db, isolatedDSN := newAuditTestDatabase(t, tc.kind, dsn)
			mainDB, mainType := db, tc.typ
			if tc.kind == "clickhouse" {
				mainDB, _ = newAuditTestDatabase(t, "sqlite", "")
				mainType = common.DatabaseTypeSQLite
			}
			useLogExportDatabases(t, mainDB, db, mainType, tc.typ)
			require.NoError(t, mainDB.AutoMigrate(&model.Channel{}))
			if tc.kind == "clickhouse" {
				t.Setenv("LOG_SQL_DSN", isolatedDSN)
				require.NoError(t, model.InitLogDB())
			} else {
				require.NoError(t, db.AutoMigrate(&model.Log{}))
			}
			versionSQL := "SELECT version()"
			if tc.kind == "sqlite" {
				versionSQL = "SELECT sqlite_version()"
			}
			var version string
			require.NoError(t, model.LOG_DB.Raw(versionSQL).Scan(&version).Error)
			t.Logf("log database: %s %s", tc.kind, version)
			now := time.Now().Unix()
			seedExportLogs(t, mainDB, model.LOG_DB, now)
			query := fmt.Sprintf("start_timestamp=%d&end_timestamp=%d", now-2*24*3600, now+3600)

			listed := callLogHandler(t, GetUserLogs, 1, common.RoleCommonUser, query)[0]
			require.Equal(t, true, listed["success"])
			assert.EqualValues(t, 2, listed["data"].(map[string]any)["total"])
			listed = callLogHandler(t, GetAllLogs, 3, common.RoleAdminUser, query+"&username=alice")[0]
			require.Equal(t, true, listed["success"])
			assert.EqualValues(t, 2, listed["data"].(map[string]any)["total"])

			own := callLogHandler(t, ExportUserLogs, 1, common.RoleCommonUser, query)
			require.Len(t, own, 3)
			assert.Equal(t, []any{"a-2", "a-1"}, exportedRequestIds(own))
			assert.Equal(t, map[string]any{"success": true, "message": "", "data": map[string]any{"count": float64(2)}}, own[2])
			assert.Equal(t, float64(1), own[0]["id"], "user exports carry list positions, not row ids")
			assert.Empty(t, own[1]["channel_name"])
			assert.NotContains(t, own[1]["other"], "admin_info")

			all := callLogHandler(t, ExportAllLogs, 3, common.RoleAdminUser, query+"&model_name=gpt-%25")
			require.Len(t, all, 4)
			assert.Equal(t, []any{"a-2", "b-1", "a-1"}, exportedRequestIds(all))
			assert.Equal(t, "primary", all[2]["channel_name"])
			assert.Contains(t, all[2]["other"], "admin_info")
			assert.Equal(t, true, all[3]["success"])
		})
	}
}

func TestLogExportSingleConnectionDatabase(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previousSetting := *operation_setting.GetLogSetting()
	operation_setting.GetLogSetting().UserExportEnabled = true
	t.Cleanup(func() { *operation_setting.GetLogSetting() = previousSetting })
	for _, separateLogDB := range []bool{false, true} {
		for _, tc := range []struct {
			name    string
			handler gin.HandlerFunc
			role    int
			count   int
		}{
			{"admin", ExportAllLogs, common.RoleAdminUser, 3},
			{"root", ExportAllLogs, common.RoleRootUser, 3},
			{"root personal view", ExportUserLogs, common.RoleRootUser, 2},
		} {
			t.Run(fmt.Sprintf("separate=%v/%s", separateLogDB, tc.name), func(t *testing.T) {
				mainDB, _ := newAuditTestDatabase(t, "sqlite", "")
				logDB := mainDB
				if separateLogDB {
					logDB, _ = newAuditTestDatabase(t, "sqlite", "")
				}
				useLogExportDatabases(t, mainDB, logDB, common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
				require.NoError(t, mainDB.AutoMigrate(&model.Channel{}))
				require.NoError(t, logDB.AutoMigrate(&model.Log{}))
				now := time.Now().Unix()
				seedExportLogs(t, mainDB, logDB, now)
				for _, db := range []*gorm.DB{mainDB, logDB} {
					pool, err := db.DB()
					require.NoError(t, err)
					pool.SetMaxOpenConns(1)
					pool.SetMaxIdleConns(1)
				}
				query := fmt.Sprintf("start_timestamp=%d&end_timestamp=%d", now-7200, now)
				ctx, cancel := context.WithTimeout(t.Context(), time.Second)
				defer cancel()
				recorder := httptest.NewRecorder()
				runLogHandler(tc.handler, recorder, ctx, 1, tc.role, query)
				require.NoError(t, ctx.Err(), "a shared single connection must complete without waiting for its own cursor")
				lines := decodeLogExportLines(t, recorder.Body.String())
				require.Len(t, lines, tc.count+1)
				assert.Equal(t, map[string]any{"success": true, "message": "", "data": map[string]any{"count": float64(tc.count)}}, lines[tc.count])
				for _, line := range lines[:tc.count] {
					if tc.name == "root personal view" {
						assert.EqualValues(t, 1, line["user_id"])
						assert.Empty(t, line["channel_name"])
						assert.NotContains(t, line["other"], "admin_info")
						assert.NotContains(t, line["other"], "root_info")
					} else if line["channel"] == float64(9) {
						assert.Equal(t, "primary", line["channel_name"])
						if line["request_id"] == "a-1" {
							assert.Contains(t, line["other"], "admin_info")
							assert.Equal(t, tc.role == common.RoleRootUser, strings.Contains(line["other"].(string), "root_info"))
						}
					}
				}

				canceledCtx, stop := context.WithCancel(t.Context())
				defer stop()
				writer := &logExportResponseWriter{ResponseRecorder: httptest.NewRecorder(), afterWrite: stop}
				runLogHandler(tc.handler, writer, canceledCtx, 1, tc.role, query)
				canceledLines := decodeLogExportLines(t, writer.Body.String())
				require.Len(t, canceledLines, 1)
				assert.NotContains(t, canceledLines[0], "success")
				for _, db := range []*gorm.DB{mainDB, logDB} {
					pool, err := db.DB()
					require.NoError(t, err)
					assert.Equal(t, 1, pool.Stats().MaxOpenConnections)
					assert.Zero(t, pool.Stats().InUse, "success and cancellation must return the only connection")
					probeCtx, finishProbe := context.WithTimeout(t.Context(), time.Second)
					err = pool.PingContext(probeCtx)
					finishProbe()
					require.NoError(t, err)
				}
				lock, acquired, err := service.AcquireLogExportLock(t.Context(), 1)
				require.NoError(t, err)
				require.True(t, acquired, "cancellation must release the export reservation")
				lock.Release()
			})
		}
	}
}

func TestLogExportRules(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, _ := newAuditTestDatabase(t, "sqlite", "")
	useLogExportDatabases(t, db, db, common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Log{}, &model.QuotaData{}))
	now := time.Now().Unix()
	seedExportLogs(t, db, db, now)
	logSetting := operation_setting.GetLogSetting()
	previousSetting := *logSetting
	t.Cleanup(func() { *logSetting = previousSetting })
	recent := fmt.Sprintf("start_timestamp=%d&end_timestamp=%d", now-2*24*3600, now)
	tooLong := fmt.Sprintf("start_timestamp=%d&end_timestamp=%d", now-366*24*3600, now)

	t.Run("time range", func(t *testing.T) {
		rejected := callLogHandler(t, ExportAllLogs, 3, common.RoleAdminUser, tooLong)
		require.Len(t, rejected, 1)
		assert.Equal(t, false, rejected[0]["success"])
		assert.Equal(t, "The time range cannot exceed {{days}} days", rejected[0]["message_key"])
		assert.Equal(t, map[string]any{"days": float64(365)}, rejected[0]["message_params"])

		assert.Equal(t, false, callLogHandler(t, GetUserLogs, 1, common.RoleCommonUser, tooLong)[0]["success"])
		assert.Equal(t, false, callLogHandler(t, GetLogsSelfStat, 1, common.RoleCommonUser, tooLong)[0]["success"])
		assert.Equal(t, true, callLogHandler(t, GetUserLogs, 3, common.RoleAdminUser, tooLong)[0]["success"], "administrators search their own logs without a limit")

		// The dashboard follows the same range instead of its old one-month limit.
		fortyDays := fmt.Sprintf("start_timestamp=%d&end_timestamp=%d", now-40*24*3600, now)
		for _, handler := range []gin.HandlerFunc{GetUserQuotaDates, GetUserFlowQuotaDates} {
			assert.Equal(t, "The time range cannot exceed {{days}} days", callLogHandler(t, handler, 1, common.RoleCommonUser, tooLong)[0]["message_key"])
			assert.Equal(t, true, callLogHandler(t, handler, 1, common.RoleCommonUser, fortyDays)[0]["success"])
		}

		// Without a start, a user's search reaches back the whole range.
		listed := callLogHandler(t, GetUserLogs, 1, common.RoleCommonUser, "")[0]
		assert.Equal(t, true, listed["success"])
		assert.EqualValues(t, 2, listed["data"].(map[string]any)["total"])

		logSetting.MaxRangeDays = 400
		assert.Equal(t, true, callLogHandler(t, GetUserLogs, 1, common.RoleCommonUser, tooLong)[0]["success"])
		logSetting.MaxRangeDays = previousSetting.MaxRangeDays
	})

	t.Run("user export switch", func(t *testing.T) {
		logSetting.UserExportEnabled = false
		t.Cleanup(func() { logSetting.UserExportEnabled = previousSetting.UserExportEnabled })
		recorder := httptest.NewRecorder()
		runLogHandler(ExportUserLogs, recorder, context.Background(), 1, common.RoleCommonUser, recent)
		assert.Equal(t, "application/json; charset=utf-8", recorder.Header().Get("Content-Type"))
		disabled := decodeLogExportLines(t, recorder.Body.String())
		require.Len(t, disabled, 1)
		assert.Equal(t, "Log export is disabled", disabled[0]["message_key"])
		for _, role := range []int{common.RoleAdminUser, common.RoleRootUser} {
			assert.Equal(t, true, callLogHandler(t, ExportUserLogs, 3, role, recent)[0]["success"], "administrators export their own logs even when users cannot")
		}
	})

	t.Run("one export per user", func(t *testing.T) {
		lock, ok, err := service.AcquireLogExportLock(t.Context(), 1)
		require.NoError(t, err)
		require.True(t, ok)
		busy := callLogHandler(t, ExportUserLogs, 1, common.RoleCommonUser, recent)
		require.Len(t, busy, 1)
		assert.Equal(t, "Another log export is running. Try again after it finishes.", busy[0]["message_key"])
		other := callLogHandler(t, ExportUserLogs, 2, common.RoleCommonUser, recent)
		assert.Equal(t, true, other[len(other)-1]["success"], "other users are not blocked")
		lock.Release()
		assert.Len(t, callLogHandler(t, ExportUserLogs, 1, common.RoleCommonUser, recent), 3)
	})
}

func TestLogExportOwnershipAndVisibility(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, _ := newAuditTestDatabase(t, "sqlite", "")
	useLogExportDatabases(t, db, db, common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Log{}))
	now := time.Now().Unix()
	seedExportLogs(t, db, db, now)
	query := fmt.Sprintf("start_timestamp=%d&end_timestamp=%d", now-7200, now)
	previousSetting := *operation_setting.GetLogSetting()
	operation_setting.GetLogSetting().UserExportEnabled = true
	t.Cleanup(func() { *operation_setting.GetLogSetting() = previousSetting })

	for _, role := range []int{common.RoleCommonUser, common.RoleAdminUser, common.RoleRootUser} {
		t.Run(fmt.Sprintf("self role %d", role), func(t *testing.T) {
			lines := callLogHandler(t, ExportUserLogs, 1, role, query+"&username=bob&channel=999&user_id=2")
			require.Len(t, lines, 3)
			assert.Equal(t, []any{"a-2", "a-1"}, exportedRequestIds(lines))
			for _, line := range lines[:2] {
				assert.EqualValues(t, 1, line["user_id"], "self export cannot change owner through query parameters or role")
				assert.Empty(t, line["channel_name"])
			}
			var other map[string]any
			require.NoError(t, common.UnmarshalJsonStr(lines[1]["other"].(string), &other))
			assert.Equal(t, 1.25, other["model_price"])
			for _, key := range []string{"admin_info", "root_info", "audit_info", "channel_name", "reject_reason"} {
				assert.NotContains(t, other, key)
			}
		})
	}

	for _, tc := range []struct {
		name string
		role int
		root bool
	}{
		{"admin", common.RoleAdminUser, false},
		{"root", common.RoleRootUser, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			lines := callLogHandler(t, ExportAllLogs, 3, tc.role, query+"&username=alice&channel=9&request_id=a-1")
			require.Len(t, lines, 2)
			assert.Equal(t, "a-1", lines[0]["request_id"])
			assert.Equal(t, "primary", lines[0]["channel_name"])
			var other map[string]any
			require.NoError(t, common.UnmarshalJsonStr(lines[0]["other"].(string), &other))
			assert.Contains(t, other, "admin_info")
			assert.Contains(t, other, "audit_info")
			assert.Equal(t, "legacy-rejection", other["admin_info"].(map[string]any)["reject_reason"])
			assert.NotContains(t, other, "reject_reason")
			_, hasRoot := other["root_info"]
			assert.Equal(t, tc.root, hasRoot)
			if tc.root {
				assert.Contains(t, lines[0]["other"], `"generation":18446744073709551615`, "export must preserve stored large integer lexemes")
			}
			assert.Equal(t, map[string]any{"success": true, "message": "", "data": map[string]any{"count": float64(1)}}, lines[1])
		})
	}
}

func TestLogExportRejectsBeforeStreaming(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, _ := newAuditTestDatabase(t, "sqlite", "")
	useLogExportDatabases(t, db, db, common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	require.NoError(t, db.AutoMigrate(&model.Log{}))
	previousSetting := *operation_setting.GetLogSetting()
	operation_setting.GetLogSetting().UserExportEnabled = true
	operation_setting.GetLogSetting().MaxRangeDays = 365
	t.Cleanup(func() { *operation_setting.GetLogSetting() = previousSetting })
	now := time.Now().Unix()
	for _, tc := range []struct {
		name    string
		handler gin.HandlerFunc
		query   string
	}{
		{"excess range", ExportAllLogs, fmt.Sprintf("start_timestamp=%d&end_timestamp=%d", now-366*86400, now)},
		{"minimum timestamp", ExportUserLogs, fmt.Sprintf("start_timestamp=%d&end_timestamp=%d", int64(math.MinInt64), now)},
		{"maximum start timestamp", ExportUserLogs, fmt.Sprintf("start_timestamp=%d", int64(math.MaxInt64))},
		{"negative end timestamp", ExportAllLogs, "end_timestamp=-1"},
		{"start after end", ExportAllLogs, fmt.Sprintf("start_timestamp=%d&end_timestamp=%d", now-60, now-120)},
		{"invalid model filter", ExportUserLogs, "model_name=" + url.QueryEscape("%")},
		{"invalid admin username", ExportAllLogs, "username=" + url.QueryEscape("%")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			runLogHandler(tc.handler, recorder, context.Background(), 1, common.RoleAdminUser, tc.query)
			assert.Equal(t, "application/json; charset=utf-8", recorder.Header().Get("Content-Type"), "validation must fail before starting an export response")
			assert.Empty(t, recorder.Header().Get("X-Accel-Buffering"))
			lines := decodeLogExportLines(t, recorder.Body.String())
			require.Len(t, lines, 1)
			assert.Equal(t, false, lines[0]["success"])
			assert.NotContains(t, lines[0], "data")
		})
	}
}

func TestLogExportRangeBoundaries(t *testing.T) {
	previousSetting := *operation_setting.GetLogSetting()
	operation_setting.GetLogSetting().MaxRangeDays = 365
	t.Cleanup(func() { *operation_setting.GetLogSetting() = previousSetting })
	// Keep the reference end in the past so the current clock cannot shorten it.
	end := time.Now().Unix() - 60
	start := end - 365*86400
	actual, err := limitLogTimeRange(start, end)
	require.NoError(t, err)
	assert.Equal(t, start, actual, "the full default 365-day boundary remains valid")
	_, err = limitLogTimeRange(start-1, end)
	assert.Error(t, err, "one second beyond the permitted range must be rejected")
	actual, err = limitLogTimeRange(0, end)
	require.NoError(t, err)
	assert.Equal(t, start, actual, "an omitted start covers the complete configured range")
	actual, err = limitLogTimeRange(time.Now().Unix()-60, math.MaxInt64)
	require.NoError(t, err)
	assert.Positive(t, actual, "future end timestamps are clipped to now without overflowing")
}

func TestLogExportDefaultRangeDoesNotDeleteLogs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, _ := newAuditTestDatabase(t, "sqlite", "")
	useLogExportDatabases(t, db, db, common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Log{}))
	previousSetting := *operation_setting.GetLogSetting()
	operation_setting.GetLogSetting().UserExportEnabled = true
	operation_setting.GetLogSetting().MaxRangeDays = 365
	t.Cleanup(func() { *operation_setting.GetLogSetting() = previousSetting })
	seedExportLogs(t, db, db, time.Now().Unix())

	lines := callLogHandler(t, ExportUserLogs, 1, common.RoleCommonUser, "")
	require.Len(t, lines, 3)
	assert.Equal(t, []any{"a-2", "a-1"}, exportedRequestIds(lines))
	var count int64
	require.NoError(t, db.Model(&model.Log{}).Count(&count).Error)
	assert.EqualValues(t, 4, count, "the search range does not configure retention or delete historical logs")
	old := callLogHandler(t, GetAllLogs, 3, common.RoleRootUser, "request_id=a-old")[0]
	require.Equal(t, true, old["success"])
	assert.EqualValues(t, 1, old["data"].(map[string]any)["total"])
}

type logExportResponseWriter struct {
	*httptest.ResponseRecorder
	afterWrite func()
	writeErr   error
	failWrite  int
	attempts   []string
}

func (w *logExportResponseWriter) Write(body []byte) (int, error) {
	w.attempts = append(w.attempts, string(body))
	if w.writeErr != nil && (w.failWrite == 0 || w.failWrite == len(w.attempts)) {
		return 0, w.writeErr
	}
	n, err := w.ResponseRecorder.Write(body)
	if w.afterWrite != nil {
		w.afterWrite()
	}
	return n, err
}

func TestLogExportTerminalOutcomes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name    string
		handler gin.HandlerFunc
	}{
		{"self", ExportUserLogs},
		{"admin", ExportAllLogs},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, _ := newAuditTestDatabase(t, "sqlite", "")
			useLogExportDatabases(t, db, db, common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
			require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Log{}))
			now := time.Now().Unix()
			seedExportLogs(t, db, db, now)
			query := fmt.Sprintf("start_timestamp=%d&end_timestamp=%d", now-7200, now)

			t.Run("empty completes with zero", func(t *testing.T) {
				lines := callLogHandler(t, tc.handler, 1, common.RoleAdminUser, query+"&request_id=missing")
				require.Len(t, lines, 1)
				assert.Equal(t, map[string]any{"success": true, "message": "", "data": map[string]any{"count": float64(0)}}, lines[0])
			})
			t.Run("cancellation has no success trailer", func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				writer := &logExportResponseWriter{ResponseRecorder: httptest.NewRecorder(), afterWrite: cancel}
				runLogHandler(tc.handler, writer, ctx, 1, common.RoleAdminUser, query)
				lines := decodeLogExportLines(t, writer.Body.String())
				require.Len(t, lines, 1, "cancellation stops immediately after the current record")
				assert.NotContains(t, lines[0], "success")
			})
			t.Run("write failure cannot claim success", func(t *testing.T) {
				writer := &logExportResponseWriter{ResponseRecorder: httptest.NewRecorder(), writeErr: io.ErrClosedPipe, failWrite: 2}
				runLogHandler(tc.handler, writer, context.Background(), 1, common.RoleAdminUser, query)
				require.NotEmpty(t, writer.attempts)
				for _, attempt := range writer.attempts {
					for _, line := range decodeLogExportLines(t, attempt) {
						assert.NotEqual(t, true, line["success"])
					}
				}
				lines := decodeLogExportLines(t, writer.Body.String())
				require.Len(t, lines, 2, "only the first completed record and a failure trailer can be delivered")
				assert.Equal(t, "a-2", lines[0]["request_id"])
				assert.Equal(t, false, lines[1]["success"])
				assert.NotContains(t, lines[1], "data")
			})
			t.Run("database failure has failure trailer", func(t *testing.T) {
				require.NoError(t, db.Migrator().DropTable(&model.Log{}))
				lines := callLogHandler(t, tc.handler, 1, common.RoleAdminUser, query)
				require.Len(t, lines, 1)
				assert.Equal(t, false, lines[0]["success"])
				assert.Equal(t, "Failed to export logs", lines[0]["message_key"])
				assert.NotContains(t, lines[0], "data")
			})
		})
	}
}

func TestLogExportLostLeaseCannotClaimSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, handler := range []struct {
		name string
		fn   gin.HandlerFunc
	}{
		{"self", ExportUserLogs},
		{"admin", ExportAllLogs},
	} {
		for _, failure := range []string{"deleted", "replaced", "redis error"} {
			t.Run(handler.name+"/"+failure, func(t *testing.T) {
				db, _ := newAuditTestDatabase(t, "sqlite", "")
				useLogExportDatabases(t, db, db, common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
				require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Log{}))
				now := time.Now().Unix()
				seedExportLogs(t, db, db, now)
				previousRedis, previousClient := common.RedisEnabled, common.RDB
				server := miniredis.RunT(t)
				common.RedisEnabled = true
				common.RDB = redis.NewClient(&redis.Options{Addr: server.Addr()})
				t.Cleanup(func() {
					require.NoError(t, common.RDB.Close())
					common.RedisEnabled, common.RDB = previousRedis, previousClient
				})
				require.NoError(t, common.RDB.Ping(t.Context()).Err())
				interrupted := false
				writer := &logExportResponseWriter{ResponseRecorder: httptest.NewRecorder()}
				writer.afterWrite = func() {
					if interrupted {
						return
					}
					interrupted = true
					switch failure {
					case "deleted":
						server.Del("log_export:1")
					case "replaced":
						require.NoError(t, server.Set("log_export:1", "replacement-owner"))
					case "redis error":
						server.SetError("ERR lease storage unavailable")
					}
					second, ok, err := service.AcquireLogExportLock(t.Context(), 1)
					assert.NoError(t, err)
					assert.False(t, ok, "losing Redis ownership must not release the still-writing local reservation")
					if second != nil {
						second.Release()
					}
				}
				query := fmt.Sprintf("start_timestamp=%d&end_timestamp=%d&request_id=a-2", now-7200, now)
				runLogHandler(handler.fn, writer, context.Background(), 1, common.RoleAdminUser, query)
				require.True(t, interrupted)
				lines := decodeLogExportLines(t, writer.Body.String())
				require.Len(t, lines, 2)
				assert.Equal(t, "a-2", lines[0]["request_id"])
				assert.Equal(t, false, lines[1]["success"])
				assert.Equal(t, "Failed to export logs", lines[1]["message_key"])
				assert.NotContains(t, lines[1], "data", "a partial export must not carry a successful row count")
				server.SetError("")
				if failure == "replaced" {
					owner, err := server.Get("log_export:1")
					require.NoError(t, err)
					assert.Equal(t, "replacement-owner", owner, "release must not delete another node's lease")
				}
				server.Del("log_export:1")
				next, ok, err := service.AcquireLogExportLock(t.Context(), 1)
				require.NoError(t, err)
				require.True(t, ok, "the local reservation is released after the old handler exits")
				next.Release()
			})
		}
	}
}

func TestLogStreamCancellationAndCallbackErrors(t *testing.T) {
	db, _ := newAuditTestDatabase(t, "sqlite", "")
	useLogExportDatabases(t, db, db, common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Log{}))
	now := time.Now().Unix()
	seedExportLogs(t, db, db, now)
	filter := model.LogFilter{StartTimestamp: now - 7200, EndTimestamp: now}
	for _, tc := range []struct {
		name   string
		stream func(context.Context, func(*model.Log) error) error
	}{
		{"self", func(ctx context.Context, fn func(*model.Log) error) error {
			return model.StreamUserLogs(ctx, 1, filter, fn)
		}},
		{"admin", func(ctx context.Context, fn func(*model.Log) error) error {
			return model.StreamAllLogs(ctx, filter, common.RoleAdminUser, fn)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Run("already canceled", func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				calls := 0
				err := tc.stream(ctx, func(*model.Log) error { calls++; return nil })
				assert.ErrorIs(t, err, context.Canceled)
				assert.Zero(t, calls)
			})
			t.Run("cancel after first record", func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				calls := 0
				err := tc.stream(ctx, func(*model.Log) error { calls++; cancel(); return nil })
				assert.ErrorIs(t, err, context.Canceled)
				assert.Equal(t, 1, calls)
			})
			t.Run("callback error is returned", func(t *testing.T) {
				failure := errors.New("export destination unavailable")
				calls := 0
				err := tc.stream(context.Background(), func(*model.Log) error { calls++; return failure })
				assert.ErrorIs(t, err, failure)
				assert.Equal(t, 1, calls)
			})
		})
	}
}

func TestAcquireLogExportLock(t *testing.T) {
	for _, useRedis := range []bool{false, true} {
		t.Run(fmt.Sprintf("redis=%v", useRedis), func(t *testing.T) {
			previousRedis, previousClient := common.RedisEnabled, common.RDB
			common.RedisEnabled = useRedis
			if useRedis {
				server := miniredis.RunT(t)
				common.RDB = redis.NewClient(&redis.Options{Addr: server.Addr()})
				require.NoError(t, common.RDB.Ping(context.Background()).Err())
			}
			t.Cleanup(func() { common.RedisEnabled, common.RDB = previousRedis, previousClient })

			lock, ok, err := service.AcquireLogExportLock(t.Context(), 1)
			require.NoError(t, err)
			require.True(t, ok)
			_, ok, err = service.AcquireLogExportLock(t.Context(), 1)
			require.NoError(t, err)
			assert.False(t, ok, "a second export of the same user must wait")
			otherLock, ok, err := service.AcquireLogExportLock(t.Context(), 2)
			require.NoError(t, err)
			assert.True(t, ok)
			otherLock.Release()
			lock.Release()
			nextLock, ok, err := service.AcquireLogExportLock(t.Context(), 1)
			require.NoError(t, err)
			assert.True(t, ok, "a finished export frees the user")
			nextLock.Release()
		})
	}
}
