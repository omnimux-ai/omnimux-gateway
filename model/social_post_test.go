package model

import (
	"fmt"
	"os"
	"sort"
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestSocialPostMonotonicTransitionsAcrossDatabases(t *testing.T) {
	engines := []struct {
		name string
		env  string
	}{
		{name: "sqlite"},
		{name: "mysql", env: "TEST_MYSQL_DSN"},
		{name: "postgres", env: "TEST_POSTGRES_DSN"},
	}

	for _, engine := range engines {
		t.Run(engine.name, func(t *testing.T) {
			var dialector gorm.Dialector
			switch engine.name {
			case "sqlite":
				dialector = sqlite.Open(":memory:")
			case "mysql":
				dsn := os.Getenv(engine.env)
				if dsn == "" {
					t.Skip("TEST_MYSQL_DSN is not configured")
				}
				dialector = mysql.Open(dsn)
			case "postgres":
				dsn := os.Getenv(engine.env)
				if dsn == "" {
					t.Skip("TEST_POSTGRES_DSN is not configured")
				}
				dialector = postgres.Open(dsn)
			}

			db, err := gorm.Open(dialector, &gorm.Config{})
			require.NoError(t, err)
			sqlDB, err := db.DB()
			require.NoError(t, err)
			if engine.name == "sqlite" {
				sqlDB.SetMaxOpenConns(1)
			} else {
				sqlDB.SetMaxOpenConns(8)
			}

			prevDB := DB
			prevType := common.MainDatabaseType()
			DB = db
			common.SetMainDatabaseType(common.DatabaseType(engine.name))
			t.Cleanup(func() {
				_ = db.Exec("DELETE FROM social_posts").Error
				if engine.name != "sqlite" {
					_ = db.Migrator().DropTable(&SocialPost{})
				}
				DB = prevDB
				common.SetMainDatabaseType(prevType)
				_ = sqlDB.Close()
			})

			require.NoError(t, db.AutoMigrate(&SocialPost{}))
			require.NoError(t, db.Exec("DELETE FROM social_posts").Error)

			var version string
			versionQuery := "SELECT version()"
			if engine.name == "sqlite" {
				versionQuery = "SELECT sqlite_version()"
			}
			require.NoError(t, db.Raw(versionQuery).Scan(&version).Error)
			t.Logf("database=%s version=%s", engine.name, version)

			t.Run("monotonic_progression_and_no_regression", func(t *testing.T) {
				post := &SocialPost{
					UserId:         101,
					Provider:       "tiktok_direct",
					ProviderPostID: "pub-mono-1",
					Status:         SocialPostStatusSubmitted,
				}
				require.NoError(t, CreateSocialPost(post))

				// submitted -> processing (PROCESSING_UPLOAD)
				updated, got, err := ApplySocialPostStateUpdateByID(post.ID, SocialPostStateUpdate{
					TargetStatus: SocialPostStatusProcessing,
					SnapshotFields: map[string]any{
						"status":      "PROCESSING_UPLOAD",
						"fail_reason": "",
						"publish_id":  "pub-mono-1",
					},
				})
				require.NoError(t, err)
				assert.True(t, updated)
				assert.Equal(t, SocialPostStatusProcessing, got.Status)
				assert.Contains(t, got.ResultSnapshot, `"status":"PROCESSING_UPLOAD"`)

				// Same non-terminal status refreshes snapshot when upstream detail changes.
				updated, got, err = ApplySocialPostStateUpdateByID(post.ID, SocialPostStateUpdate{
					TargetStatus: SocialPostStatusProcessing,
					SnapshotFields: map[string]any{
						"status":      "PROCESSING_DOWNLOAD",
						"fail_reason": "",
						"publish_id":  "pub-mono-1",
					},
				})
				require.NoError(t, err)
				assert.True(t, updated)
				assert.Equal(t, SocialPostStatusProcessing, got.Status)
				assert.Contains(t, got.ResultSnapshot, `"status":"PROCESSING_DOWNLOAD"`)

				// processing -> sent_to_inbox
				updated, got, err = ApplySocialPostStateUpdateByProviderPostID("tiktok_direct", "pub-mono-1", SocialPostStateUpdate{
					TargetStatus: SocialPostStatusSentToInbox,
					EventName:    "post.publish.inbox_delivered",
					EventTime:    1700000010,
					SnapshotFields: map[string]any{
						"status":       "SEND_TO_USER_INBOX",
						"publish_id":   "pub-mono-1",
						"publish_type": "INBOX_UPLOAD",
					},
				})
				require.NoError(t, err)
				assert.True(t, updated)
				assert.Equal(t, SocialPostStatusSentToInbox, got.Status)

				// Stale poll returning processing MUST NOT regress sent_to_inbox.
				updated, got, err = ApplySocialPostStateUpdateByID(post.ID, SocialPostStateUpdate{
					TargetStatus: SocialPostStatusProcessing,
					SnapshotFields: map[string]any{
						"status":      "PROCESSING_UPLOAD",
						"fail_reason": "",
						"publish_id":  "pub-mono-1",
					},
				})
				require.NoError(t, err)
				assert.False(t, updated)
				assert.Equal(t, SocialPostStatusSentToInbox, got.Status)

				// Webhook publicly_available advances sent_to_inbox -> published and records post_id.
				updated, got, err = ApplySocialPostStateUpdateByProviderPostID("tiktok_direct", "pub-mono-1", SocialPostStateUpdate{
					TargetStatus:                SocialPostStatusPublished,
					EventName:                   "post.publish.publicly_available",
					EventTime:                   1700000020,
					AddPostID:                   "7210402925882379563",
					AllowPublishedSnapshotMerge: true,
					SnapshotFields: map[string]any{
						"status":       "PUBLISH_COMPLETE",
						"publish_id":   "pub-mono-1",
						"publish_type": "DIRECT_POST",
					},
				})
				require.NoError(t, err)
				assert.True(t, updated)
				assert.Equal(t, SocialPostStatusPublished, got.Status)
				assert.Equal(t, []string{"7210402925882379563"}, readSnapshotStringSlice(t, got.ResultSnapshot, "post_ids"))

				// Stale poll (AllowPublishedSnapshotMerge=false) MUST NOT overwrite published snapshot or regress status.
				for _, pollStatus := range []string{SocialPostStatusProcessing, SocialPostStatusSentToInbox, SocialPostStatusPublished, SocialPostStatusFailed} {
					updated, got, err = ApplySocialPostStateUpdateByID(post.ID, SocialPostStateUpdate{
						TargetStatus: pollStatus,
						SnapshotFields: map[string]any{
							"status":      "POLL_STALE",
							"fail_reason": "stale",
							"publish_id":  "pub-mono-1",
						},
					})
					require.NoError(t, err)
					assert.False(t, updated, "poll status %s must not mutate published row", pollStatus)
					assert.Equal(t, SocialPostStatusPublished, got.Status)
					assert.Equal(t, []string{"7210402925882379563"}, readSnapshotStringSlice(t, got.ResultSnapshot, "post_ids"))
				}
			})

			t.Run("out_of_order_publicly_available_before_complete", func(t *testing.T) {
				post := &SocialPost{
					UserId:         102,
					Provider:       "tiktok_direct",
					ProviderPostID: "pub-ooo-1",
					Status:         SocialPostStatusSubmitted,
				}
				require.NoError(t, CreateSocialPost(post))

				// publicly_available arrives before complete.
				updated, got, err := ApplySocialPostStateUpdateByProviderPostID("tiktok_direct", "pub-ooo-1", SocialPostStateUpdate{
					TargetStatus:                SocialPostStatusPublished,
					EventName:                   "post.publish.publicly_available",
					EventTime:                   1700000200,
					AddPostID:                   "7210402925882379999",
					AllowPublishedSnapshotMerge: true,
					SnapshotFields: map[string]any{
						"status":       "PUBLISH_COMPLETE",
						"publish_id":   "pub-ooo-1",
						"publish_type": "DIRECT_POST",
					},
				})
				require.NoError(t, err)
				assert.True(t, updated)
				assert.Equal(t, SocialPostStatusPublished, got.Status)
				assert.Equal(t, []string{"7210402925882379999"}, readSnapshotStringSlice(t, got.ResultSnapshot, "post_ids"))

				// Delayed complete arrives afterwards: MUST preserve post_ids and not lose data.
				updated, got, err = ApplySocialPostStateUpdateByProviderPostID("tiktok_direct", "pub-ooo-1", SocialPostStateUpdate{
					TargetStatus:                SocialPostStatusPublished,
					EventName:                   "post.publish.complete",
					EventTime:                   1700000190,
					AllowPublishedSnapshotMerge: true,
					SnapshotFields: map[string]any{
						"status":           "PUBLISH_COMPLETE",
						"publish_id":       "pub-ooo-1",
						"publish_type":     "DIRECT_POST",
						"publish_complete": true,
					},
				})
				require.NoError(t, err)
				assert.True(t, updated)
				assert.Equal(t, SocialPostStatusPublished, got.Status)
				assert.Equal(t, []string{"7210402925882379999"}, readSnapshotStringSlice(t, got.ResultSnapshot, "post_ids"))
				snap := parseSocialPostSnapshot(got.ResultSnapshot)
				assert.Equal(t, true, snap["publish_complete"])
				assert.Equal(t, "post.publish.publicly_available", snap["last_event"])

				// Replaying delayed complete is a pure no-op.
				updated, _, err = ApplySocialPostStateUpdateByProviderPostID("tiktok_direct", "pub-ooo-1", SocialPostStateUpdate{
					TargetStatus:                SocialPostStatusPublished,
					EventName:                   "post.publish.complete",
					EventTime:                   1700000190,
					AllowPublishedSnapshotMerge: true,
					SnapshotFields: map[string]any{
						"status":           "PUBLISH_COMPLETE",
						"publish_id":       "pub-ooo-1",
						"publish_type":     "DIRECT_POST",
						"publish_complete": true,
					},
				})
				require.NoError(t, err)
				assert.False(t, updated)
			})

			t.Run("published_to_deleted_and_absorbing_terminals", func(t *testing.T) {
				// 1) published (single post) -> deleted, then deleted cannot resurrect.
				post := &SocialPost{
					UserId:         103,
					Provider:       "tiktok_direct",
					ProviderPostID: "pub-del-1",
					Status:         SocialPostStatusSubmitted,
				}
				require.NoError(t, CreateSocialPost(post))

				_, _, err := ApplySocialPostStateUpdateByProviderPostID("tiktok_direct", "pub-del-1", SocialPostStateUpdate{
					TargetStatus:                SocialPostStatusPublished,
					EventName:                   "post.publish.publicly_available",
					EventTime:                   1700001000,
					AddPostID:                   "888001",
					AllowPublishedSnapshotMerge: true,
					SnapshotFields:              map[string]any{"status": "PUBLISH_COMPLETE", "publish_id": "pub-del-1"},
				})
				require.NoError(t, err)

				updated, got, err := ApplySocialPostStateUpdateByProviderPostID("tiktok_direct", "pub-del-1", SocialPostStateUpdate{
					Takedown:       true,
					EventName:      "post.publish.no_longer_publicaly_available",
					EventTime:      1700001100,
					RemovePostID:   "888001",
					SnapshotFields: map[string]any{"publish_id": "pub-del-1"},
				})
				require.NoError(t, err)
				assert.True(t, updated)
				assert.Equal(t, SocialPostStatusDeleted, got.Status)
				assert.Equal(t, []string{"888001"}, readSnapshotStringSlice(t, got.ResultSnapshot, "unavailable_post_ids"))

				// deleted is strictly absorbing: no event or poll may resurrect it.
				attempts := []SocialPostStateUpdate{
					{TargetStatus: SocialPostStatusPublished, AddPostID: "888002", AllowPublishedSnapshotMerge: true, EventName: "post.publish.publicly_available", EventTime: 1700001200},
					{TargetStatus: SocialPostStatusPublished, AllowPublishedSnapshotMerge: true, EventName: "post.publish.complete", EventTime: 1700001201},
					{TargetStatus: SocialPostStatusSentToInbox, EventName: "post.publish.inbox_delivered", EventTime: 1700001202},
					{TargetStatus: SocialPostStatusProcessing, SnapshotFields: map[string]any{"status": "PROCESSING_UPLOAD"}},
					{TargetStatus: SocialPostStatusSubmitted},
					{TargetStatus: SocialPostStatusFailed, EventName: "post.publish.failed", EventTime: 1700001203},
					{Takedown: true, RemovePostID: "888002", EventName: "post.publish.no_longer_publicaly_available", EventTime: 1700001204},
				}
				for i, attempt := range attempts {
					updated, got, err = ApplySocialPostStateUpdateByID(post.ID, attempt)
					require.NoError(t, err, "attempt %d", i)
					assert.False(t, updated, "deleted must absorb attempt %d", i)
					assert.Equal(t, SocialPostStatusDeleted, got.Status)
				}

				// 2) non-published (submitted/processing/sent_to_inbox) + takedown -> deleted.
				for _, initStatus := range []string{SocialPostStatusSubmitted, SocialPostStatusProcessing, SocialPostStatusSentToInbox} {
					p := &SocialPost{
						UserId:         103,
						Provider:       "tiktok_direct",
						ProviderPostID: "pub-del-from-" + initStatus,
						Status:         initStatus,
					}
					require.NoError(t, CreateSocialPost(p))
					updated, got, err := ApplySocialPostStateUpdateByID(p.ID, SocialPostStateUpdate{
						Takedown:       true,
						EventName:      "post.publish.no_longer_publicaly_available",
						EventTime:      1700001300,
						RemovePostID:   "999",
						SnapshotFields: map[string]any{"publish_id": p.ProviderPostID},
					})
					require.NoError(t, err)
					assert.True(t, updated)
					assert.Equal(t, SocialPostStatusDeleted, got.Status)
				}

				// 3) failed and partial are irreversible absorbing terminals.
				for _, term := range []string{SocialPostStatusFailed, SocialPostStatusPartial} {
					p := &SocialPost{
						UserId:         103,
						Provider:       "tiktok_direct",
						ProviderPostID: "pub-term-" + term,
						Status:         term,
						ResultSnapshot: `{"status":"TERMINAL"}`,
					}
					require.NoError(t, CreateSocialPost(p))
					for _, upd := range []SocialPostStateUpdate{
						{TargetStatus: SocialPostStatusPublished, AddPostID: "123", AllowPublishedSnapshotMerge: true},
						{Takedown: true, RemovePostID: "123"},
						{TargetStatus: SocialPostStatusSentToInbox},
					} {
						updated, got, err := ApplySocialPostStateUpdateByID(p.ID, upd)
						require.NoError(t, err)
						assert.False(t, updated)
						assert.Equal(t, term, got.Status)
					}
				}
			})

			t.Run("multi_post_upload_takedown", func(t *testing.T) {
				post := &SocialPost{
					UserId:         104,
					Provider:       "tiktok_direct",
					ProviderPostID: "pub-multi-1",
					Status:         SocialPostStatusSentToInbox,
				}
				require.NoError(t, CreateSocialPost(post))

				for idx, postID := range []string{"7001", "7002"} {
					updated, got, err := ApplySocialPostStateUpdateByProviderPostID("tiktok_direct", "pub-multi-1", SocialPostStateUpdate{
						TargetStatus:                SocialPostStatusPublished,
						EventName:                   "post.publish.publicly_available",
						EventTime:                   int64(1700002000 + idx),
						AddPostID:                   postID,
						AllowPublishedSnapshotMerge: true,
						SnapshotFields:              map[string]any{"status": "PUBLISH_COMPLETE", "publish_id": "pub-multi-1"},
					})
					require.NoError(t, err)
					assert.True(t, updated)
					assert.Equal(t, SocialPostStatusPublished, got.Status)
				}

				// First post taken down: second post survives -> keep status=published.
				updated, got, err := ApplySocialPostStateUpdateByProviderPostID("tiktok_direct", "pub-multi-1", SocialPostStateUpdate{
					Takedown:       true,
					EventName:      "post.publish.no_longer_publicaly_available",
					EventTime:      1700002100,
					RemovePostID:   "7001",
					SnapshotFields: map[string]any{"publish_id": "pub-multi-1"},
				})
				require.NoError(t, err)
				assert.True(t, updated)
				assert.Equal(t, SocialPostStatusPublished, got.Status)
				assert.Equal(t, []string{"7001", "7002"}, readSnapshotStringSlice(t, got.ResultSnapshot, "post_ids"))
				assert.Equal(t, []string{"7001"}, readSnapshotStringSlice(t, got.ResultSnapshot, "unavailable_post_ids"))

				// Replay of first takedown is an idempotent no-op.
				updated, got, err = ApplySocialPostStateUpdateByProviderPostID("tiktok_direct", "pub-multi-1", SocialPostStateUpdate{
					Takedown:       true,
					EventName:      "post.publish.no_longer_publicaly_available",
					EventTime:      1700002100,
					RemovePostID:   "7001",
					SnapshotFields: map[string]any{"publish_id": "pub-multi-1"},
				})
				require.NoError(t, err)
				assert.False(t, updated)
				assert.Equal(t, SocialPostStatusPublished, got.Status)

				// Replay of publicly_available for removed post MUST NOT drop it from unavailable_post_ids.
				updated, got, err = ApplySocialPostStateUpdateByProviderPostID("tiktok_direct", "pub-multi-1", SocialPostStateUpdate{
					TargetStatus:                SocialPostStatusPublished,
					EventName:                   "post.publish.publicly_available",
					EventTime:                   1700002000,
					AddPostID:                   "7001",
					AllowPublishedSnapshotMerge: true,
					SnapshotFields:              map[string]any{"status": "PUBLISH_COMPLETE", "publish_id": "pub-multi-1"},
				})
				require.NoError(t, err)
				assert.False(t, updated)
				assert.Equal(t, []string{"7001"}, readSnapshotStringSlice(t, got.ResultSnapshot, "unavailable_post_ids"))

				// Last surviving post taken down -> transitions to deleted.
				updated, got, err = ApplySocialPostStateUpdateByProviderPostID("tiktok_direct", "pub-multi-1", SocialPostStateUpdate{
					Takedown:       true,
					EventName:      "post.publish.no_longer_publicly_available",
					EventTime:      1700002200,
					RemovePostID:   "7002",
					SnapshotFields: map[string]any{"publish_id": "pub-multi-1"},
				})
				require.NoError(t, err)
				assert.True(t, updated)
				assert.Equal(t, SocialPostStatusDeleted, got.Status)
				assert.Equal(t, []string{"7001", "7002"}, readSnapshotStringSlice(t, got.ResultSnapshot, "unavailable_post_ids"))
			})

			t.Run("concurrent_merge_and_poll_race", func(t *testing.T) {
				post := &SocialPost{
					UserId:         105,
					Provider:       "tiktok_direct",
					ProviderPostID: "pub-conc-1",
					Status:         SocialPostStatusSubmitted,
				}
				require.NoError(t, CreateSocialPost(post))

				const workers = 6
				var (
					wg   sync.WaitGroup
					errs = make(chan error, workers+2)
				)
				for i := 0; i < workers; i++ {
					i := i
					wg.Add(1)
					go func() {
						defer wg.Done()
						_, _, err := ApplySocialPostStateUpdateByProviderPostID("tiktok_direct", "pub-conc-1", SocialPostStateUpdate{
							TargetStatus:                SocialPostStatusPublished,
							EventName:                   "post.publish.publicly_available",
							EventTime:                   int64(1700003000 + i),
							AddPostID:                   fmt.Sprintf("900%d", i),
							AllowPublishedSnapshotMerge: true,
							SnapshotFields:              map[string]any{"status": "PUBLISH_COMPLETE", "publish_id": "pub-conc-1"},
						})
						if err != nil {
							errs <- err
						}
					}()
				}
				// Concurrent stale polls attempting non-terminal/published overwrites.
				for _, pollSt := range []string{SocialPostStatusProcessing, SocialPostStatusPublished} {
					pollSt := pollSt
					wg.Add(1)
					go func() {
						defer wg.Done()
						_, _, err := ApplySocialPostStateUpdateByID(post.ID, SocialPostStateUpdate{
							TargetStatus:   pollSt,
							SnapshotFields: map[string]any{"status": "POLL_RACE", "publish_id": "pub-conc-1"},
						})
						if err != nil {
							errs <- err
						}
					}()
				}
				wg.Wait()
				close(errs)
				for err := range errs {
					require.NoError(t, err)
				}

				finalPost, err := GetSocialPostByProviderPostID("tiktok_direct", "pub-conc-1")
				require.NoError(t, err)
				assert.Equal(t, SocialPostStatusPublished, finalPost.Status)
				gotIDs := readSnapshotStringSlice(t, finalPost.ResultSnapshot, "post_ids")
				sort.Strings(gotIDs)
				assert.Equal(t, []string{"9000", "9001", "9002", "9003", "9004", "9005"}, gotIDs)
			})

			t.Run("idempotent_replay_and_zero_rows_affected_safe", func(t *testing.T) {
				post := &SocialPost{
					UserId:         106,
					Provider:       "tiktok_direct",
					ProviderPostID: "pub-idem-1",
					Status:         SocialPostStatusSubmitted,
				}
				require.NoError(t, CreateSocialPost(post))

				upd := SocialPostStateUpdate{
					TargetStatus:                SocialPostStatusPublished,
					EventName:                   "post.publish.complete",
					EventTime:                   1700004000,
					AllowPublishedSnapshotMerge: true,
					SnapshotFields: map[string]any{
						"status":           "PUBLISH_COMPLETE",
						"publish_id":       "pub-idem-1",
						"publish_complete": true,
					},
				}
				updated, first, err := ApplySocialPostStateUpdateByProviderPostID("tiktok_direct", "pub-idem-1", upd)
				require.NoError(t, err)
				assert.True(t, updated)

				// Same-value replay skips UPDATE and returns nil error (no RowsAffected==0 error on MySQL).
				updated, second, err := ApplySocialPostStateUpdateByProviderPostID("tiktok_direct", "pub-idem-1", upd)
				require.NoError(t, err)
				assert.False(t, updated)
				assert.Equal(t, first.Status, second.Status)
				assert.Equal(t, first.ResultSnapshot, second.ResultSnapshot)

				// Even an explicit same-value UPDATE within the same second (MySQL RowsAffected == 0) must not error.
				require.NoError(t, UpdateSocialPostResult(first.ID, first.Status, first.ProviderPostID, first.ResultSnapshot))

				// Unknown publish_id returns gorm.ErrRecordNotFound cleanly.
				updated, missing, err := ApplySocialPostStateUpdateByProviderPostID("tiktok_direct", "pub-does-not-exist", upd)
				assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
				assert.False(t, updated)
				assert.Nil(t, missing)
			})
		})
	}
}

func readSnapshotStringSlice(t *testing.T, raw, key string) []string {
	t.Helper()
	return snapshotStringList(parseSocialPostSnapshot(raw), key)
}
