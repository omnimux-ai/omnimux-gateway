package router

import (
	"net/http"

	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/service/authz"

	// Import oauth package to register providers via init()
	_ "github.com/QuantumNous/new-api/oauth"

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
)

func SetApiRouter(router *gin.Engine) {
	apiRouter := router.Group("/api")
	apiRouter.Use(middleware.RouteTag("api"))
	apiRouter.Use(gzip.Gzip(gzip.DefaultCompression, gzip.WithExcludedPaths([]string{"/api/inspiration/v1/media", "/api/inspiration/v1/public/media", "/api/social/v1/public/media"})))
	apiRouter.Use(middleware.AccessTokenAudit())
	apiRouter.Use(middleware.BodyStorageCleanup()) // 清理请求体存储
	apiRouter.Use(middleware.GlobalAPIRateLimit())
	anonymousRequestBodyLimit := middleware.AnonymousRequestBodyLimit()
	{
		apiRouter.GET("/setup", controller.GetSetup)
		apiRouter.POST("/setup", anonymousRequestBodyLimit, controller.PostSetup)
		apiRouter.GET("/status", controller.GetStatus)
		apiRouter.GET("/uptime/status", controller.GetUptimeKumaStatus)
		apiRouter.GET("/models", middleware.UserAuth(), controller.DashboardListModels)
		apiRouter.GET("/status/test", middleware.AdminAuth(), controller.TestStatus)
		apiRouter.GET("/notice", controller.GetNotice)
		apiRouter.GET("/user-agreement", controller.GetUserAgreement)
		apiRouter.GET("/privacy-policy", controller.GetPrivacyPolicy)
		apiRouter.GET("/about", controller.GetAbout)
		//apiRouter.GET("/midjourney", controller.GetMidjourney)
		apiRouter.GET("/home_page_content", controller.GetHomePageContent)
		apiRouter.GET("/pricing", middleware.HeaderNavModuleAuth("pricing"), controller.GetPricing)

		// fork: OmniMux — the same contract `/pricing` publishes per row, addressed
		// by public model id so a client can read one model's accepted parameters
		// before submitting instead of maintaining its own table.
		// See FORK_CUSTOMIZATIONS.md.
		apiRouter.GET("/model_contract/:model", middleware.HeaderNavModuleAuth("pricing"), controller.GetModelContract)
		// fork: influencer workbench option dataset (embedded, versioned).
		// Guest browsing: the read endpoints are public so visitors can explore
		// the workbench before signing in; writes stay behind UserAuth/AdminAuth.
		apiRouter.GET("/influencer/taxonomy", middleware.GlobalWebRateLimit(nil), controller.GetInfluencerTaxonomy)
		// fork: influencer-preset-publish — an administrator promotes a finished
		// character from their history into the inspiration library; the library
		// list is public window content, only promote/delete stay privileged.
		apiRouter.GET("/influencer/presets", middleware.GlobalWebRateLimit(nil), controller.ListInfluencerPresets)
		apiRouter.GET("/influencer/presets/:id/asset/:role", middleware.GlobalWebRateLimit(nil), controller.InfluencerPresetAsset)
		apiRouter.POST("/influencer/presets", middleware.AdminAuth(), middleware.CriticalRateLimit(), controller.PublishInfluencerPreset)
		apiRouter.DELETE("/influencer/presets/:id", middleware.AdminAuth(), middleware.CriticalRateLimit(), controller.DeleteInfluencerPreset)
		// fork: creator CLI uses explicit personal grants without opening /pg.
		influencerCLI := apiRouter.Group("/influencer/v1", middleware.UserAuth(), middleware.DisableCache())
		controller.RegisterInfluencerCLIRoutes(influencerCLI)
		perfMetricsRoute := apiRouter.Group("/perf-metrics")
		perfMetricsRoute.Use(middleware.HeaderNavModulePublicOrUserAuth("pricing"))
		{
			perfMetricsRoute.GET("/summary", controller.GetPerfMetricsSummary)
			perfMetricsRoute.GET("", controller.GetPerfMetrics)
		}
		apiRouter.GET("/rankings", middleware.HeaderNavModuleAuth("rankings"), controller.GetRankings)
		apiRouter.GET("/verification", middleware.EmailVerificationRateLimit(), middleware.TurnstileCheck(), controller.SendEmailVerification)
		apiRouter.GET("/reset_password", middleware.CriticalRateLimit(), middleware.TurnstileCheck(), controller.SendPasswordResetEmail)
		apiRouter.POST("/user/reset", middleware.CriticalRateLimit(), anonymousRequestBodyLimit, controller.ResetPassword)
		// OAuth routes - specific routes must come before :provider wildcard
		apiRouter.POST("/oauth/state", middleware.CriticalRateLimit(), middleware.DisableCache(), middleware.TryUserAuth(), anonymousRequestBodyLimit, controller.GenerateOAuthCode)
		apiRouter.POST("/oauth/email/bind/start", middleware.UserAuth(), middleware.CriticalRateLimit(), middleware.UserCriticalRateLimit("account-security"), middleware.EmailVerificationRateLimit(), middleware.DisableCache(), controller.EmailBindStart)
		apiRouter.POST("/oauth/email/bind/resend", middleware.UserAuth(), middleware.CriticalRateLimit(), middleware.UserCriticalRateLimit("account-security"), middleware.EmailVerificationRateLimit(), middleware.DisableCache(), controller.EmailBindResend)
		apiRouter.POST("/oauth/email/bind", middleware.UserAuth(), middleware.CriticalRateLimit(), middleware.UserCriticalRateLimit("account-security"), middleware.DisableCache(), controller.EmailBind)
		// WeChat uses its existing authorization-code service.
		apiRouter.GET("/oauth/wechat", middleware.CriticalRateLimit(), middleware.DisableCache(), controller.WeChatAuth)
		apiRouter.POST("/oauth/wechat/bind", middleware.UserAuth(), middleware.CriticalRateLimit(), controller.WeChatBind)
		apiRouter.GET("/oauth/telegram/login", middleware.CriticalRateLimit(), middleware.DisableCache(), controller.TelegramLegacyAuth)
		apiRouter.POST("/oauth/telegram/bind/start", middleware.UserAuth(), middleware.CriticalRateLimit(), middleware.DisableCache(), controller.TelegramLegacyAuth)
		apiRouter.GET("/oauth/telegram/bind/:flow_token", middleware.CriticalRateLimit(), middleware.DisableCache(), controller.TelegramLegacyAuth)
		// Standard OAuth providers (GitHub, Discord, OIDC, LinuxDO, Telegram) - unified route
		apiRouter.GET("/oauth/:provider", middleware.CriticalRateLimit(), middleware.DisableCache(), middleware.TryUserAuth(), controller.HandleOAuth)
		apiRouter.GET("/ratio_config", middleware.CriticalRateLimit(), controller.GetRatioConfig)

		apiRouter.POST("/stripe/webhook", anonymousRequestBodyLimit, controller.StripeWebhook)
		apiRouter.POST("/creem/webhook", anonymousRequestBodyLimit, controller.CreemWebhook)
		apiRouter.POST("/waffo/webhook", anonymousRequestBodyLimit, controller.WaffoWebhook)
		// :env separates test vs prod URLs so the operator can register each
		// in Pancake's matching webhook slot; handler enforces env match.
		apiRouter.POST("/waffo-pancake/webhook/:env", anonymousRequestBodyLimit, controller.WaffoPancakeWebhook)

		// Universal secure verification routes
		apiRouter.GET("/verify/methods", middleware.UserAuth(), middleware.DisableCache(), controller.GetVerificationMethods)
		apiRouter.POST("/verify", middleware.UserAuth(), middleware.CriticalRateLimit(), middleware.UserCriticalRateLimit("security-verification"), middleware.DisableCache(), controller.UniversalVerify)

		userRoute := apiRouter.Group("/user")
		{
			userRoute.POST("/auth/refresh", middleware.SessionCookieOriginGuard(), middleware.CriticalRateLimit(), middleware.DisableCache(), controller.RefreshAuth)
			userRoute.POST("/auth/logout", middleware.SessionCookieOriginGuard(), middleware.CriticalRateLimit(), middleware.DisableCache(), controller.AuthLogout)
			userRoute.POST("/register", middleware.CriticalRateLimit(), anonymousRequestBodyLimit, middleware.TurnstileCheck(), controller.Register)
			userRoute.GET("/login/encryption-key", middleware.DisableCache(), controller.GetPasswordEncryptionKey)
			userRoute.POST("/login", middleware.CriticalRateLimit(), middleware.DisableCache(), anonymousRequestBodyLimit, middleware.TurnstileCheck(), controller.Login)
			// CLI device-code login (RFC 8628-style)
			userRoute.POST("/device/code", middleware.CriticalRateLimit(), middleware.DisableCache(), anonymousRequestBodyLimit, controller.CreateDeviceLoginCode)
			userRoute.POST("/device/token", middleware.CriticalRateLimit(), middleware.DisableCache(), anonymousRequestBodyLimit, controller.PollDeviceLoginToken)
			userRoute.POST("/login/2fa", middleware.CriticalRateLimit(), middleware.DisableCache(), anonymousRequestBodyLimit, controller.Verify2FALogin)
			userRoute.POST("/login/verify", middleware.CriticalRateLimit(), middleware.DisableCache(), anonymousRequestBodyLimit, controller.VerifyLogin)
			userRoute.POST("/login/passkey/begin", middleware.CriticalRateLimit(), middleware.DisableCache(), anonymousRequestBodyLimit, controller.LoginPasskeyBegin)
			userRoute.POST("/login/passkey/finish", middleware.CriticalRateLimit(), middleware.DisableCache(), anonymousRequestBodyLimit, controller.LoginPasskeyFinish)
			userRoute.POST("/passkey/login/begin", middleware.CriticalRateLimit(), middleware.DisableCache(), anonymousRequestBodyLimit, controller.PasskeyLoginBegin)
			userRoute.POST("/passkey/login/finish", middleware.CriticalRateLimit(), middleware.DisableCache(), anonymousRequestBodyLimit, controller.PasskeyLoginFinish)
			//userRoute.POST("/tokenlog", middleware.CriticalRateLimit(), controller.TokenLog)
			userRoute.POST("/epay/notify", anonymousRequestBodyLimit, controller.EpayNotify)
			userRoute.GET("/epay/notify", controller.EpayNotify)
			userRoute.GET("/groups", controller.GetUserGroups)

			selfRoute := userRoute.Group("/")
			selfRoute.Use(middleware.DisableCache(), middleware.UserAuth())
			{
				selfRoute.GET("/sessions", middleware.DisableCache(), controller.GetLoginSessions)
				selfRoute.DELETE("/sessions/:sid", middleware.DisableCache(), controller.DeleteLoginSession)
				selfRoute.POST("/sessions/revoke-others", middleware.DisableCache(), controller.RevokeOtherLoginSessions)
				selfRoute.GET("/self/groups", controller.GetUserGroups)
				selfRoute.GET("/self", controller.GetSelf)
				selfRoute.GET("/models", controller.GetUserModels)
				selfRoute.PUT("/self", middleware.CriticalRateLimit(), middleware.DisableCache(), controller.UpdateSelf)
				selfRoute.DELETE("/self", middleware.DisableCache(), controller.DeleteSelf)
				// fork: CLI device-code login confirm/deny (browser while logged in)
				selfRoute.POST("/device/approve", middleware.CriticalRateLimit(), middleware.DisableCache(), controller.ApproveDeviceLogin)
				selfRoute.POST("/device/deny", middleware.CriticalRateLimit(), middleware.DisableCache(), controller.DenyDeviceLogin)
				accessTokenRoute := selfRoute.Group("/access_tokens")
				accessTokenRoute.Use(middleware.DisableCache())
				{
					accessTokenRoute.GET("", controller.ListAccessTokens)
					accessTokenRoute.GET("/catalog", controller.GetAccessTokenCatalog)
					accessTokenRoute.GET("/scopes", controller.GetAccessTokenScopes)
					accessTokenRoute.POST("", middleware.CriticalRateLimit(), middleware.UserCriticalRateLimit("access-token"), controller.CreateAccessToken)
					accessTokenRoute.PATCH("/:id", middleware.CriticalRateLimit(), middleware.UserCriticalRateLimit("access-token"), controller.UpdateAccessToken)
					accessTokenRoute.DELETE("/:id", middleware.CriticalRateLimit(), middleware.UserCriticalRateLimit("access-token"), controller.DeleteAccessToken)
					accessTokenRoute.DELETE("/legacy", middleware.CriticalRateLimit(), middleware.UserCriticalRateLimit("access-token"), controller.RevokeLegacyAccessToken)
				}
				selfRoute.GET("/passkey", controller.PasskeyStatus)
				selfRoute.POST("/passkey/register/begin", middleware.UserCriticalRateLimit("security-verification"), middleware.DisableCache(), controller.PasskeyRegisterBegin)
				selfRoute.POST("/passkey/register/finish", middleware.UserCriticalRateLimit("security-verification"), middleware.DisableCache(), controller.PasskeyRegisterFinish)
				selfRoute.POST("/passkey/verify/begin", middleware.UserCriticalRateLimit("security-verification"), middleware.DisableCache(), controller.PasskeyVerifyBegin)
				selfRoute.POST("/passkey/verify/finish", middleware.UserCriticalRateLimit("security-verification"), middleware.DisableCache(), controller.PasskeyVerifyFinish)
				selfRoute.DELETE("/passkey", middleware.DisableCache(), controller.PasskeyDelete)
				selfRoute.GET("/aff", controller.GetAffCode)
				selfRoute.GET("/topup/info", controller.GetTopUpInfo)
				selfRoute.GET("/topup/self", controller.GetUserTopUps)
				selfRoute.POST("/topup", middleware.CriticalRateLimit(), controller.TopUp)
				selfRoute.POST("/pay", middleware.CriticalRateLimit(), controller.RequestEpay)
				selfRoute.POST("/amount", controller.RequestAmount)
				selfRoute.POST("/stripe/pay", middleware.CriticalRateLimit(), controller.RequestStripePay)
				selfRoute.POST("/stripe/amount", controller.RequestStripeAmount)
				selfRoute.POST("/creem/pay", middleware.CriticalRateLimit(), controller.RequestCreemPay)
				selfRoute.POST("/waffo/amount", controller.RequestWaffoAmount)
				selfRoute.POST("/waffo/pay", middleware.CriticalRateLimit(), controller.RequestWaffoPay)
				selfRoute.POST("/waffo-pancake/amount", controller.RequestWaffoPancakeAmount)
				selfRoute.POST("/waffo-pancake/pay", middleware.CriticalRateLimit(), controller.RequestWaffoPancakePay)
				selfRoute.POST("/aff_transfer", middleware.UserCriticalRateLimit("aff-transfer"), controller.TransferAffQuota)
				selfRoute.PUT("/setting", controller.UpdateUserSetting)

				// 2FA routes
				selfRoute.GET("/2fa/status", controller.Get2FAStatus)
				selfRoute.POST("/2fa/setup", middleware.UserCriticalRateLimit("security-verification"), middleware.DisableCache(), controller.Setup2FA)
				selfRoute.POST("/2fa/enable", middleware.UserCriticalRateLimit("security-verification"), middleware.DisableCache(), controller.Enable2FA)
				selfRoute.POST("/2fa/disable", middleware.DisableCache(), controller.Disable2FA)
				selfRoute.POST("/2fa/backup_codes", middleware.DisableCache(), controller.RegenerateBackupCodes)

				// Check-in routes
				selfRoute.GET("/checkin", controller.GetCheckinStatus)
				selfRoute.POST("/checkin", middleware.TurnstileCheck(), controller.DoCheckin)

				// Custom OAuth bindings
				selfRoute.GET("/oauth/bindings", controller.GetUserOAuthBindings)
				selfRoute.DELETE("/oauth/bindings/:provider_id", controller.UnbindCustomOAuth)
			}

			adminRoute := userRoute.Group("/")
			adminRoute.Use(middleware.AdminAuth())
			{
				adminRoute.GET("/", controller.GetAllUsers)
				adminRoute.GET("/topup", controller.GetAllTopUps)
				adminRoute.POST("/topup/complete", controller.AdminCompleteTopUp)
				adminRoute.GET("/search", controller.SearchUsers)
				adminRoute.GET("/:id/oauth/bindings", controller.GetUserOAuthBindingsByAdmin)
				adminRoute.DELETE("/:id/oauth/bindings/:provider_id", controller.UnbindCustomOAuthByAdmin)
				adminRoute.DELETE("/:id/bindings/:binding_type", controller.AdminClearUserBinding)
				adminRoute.GET("/:id", controller.GetUser)
				adminRoute.POST("/", controller.CreateUser)
				adminRoute.POST("/manage", controller.ManageUser)
				adminRoute.PUT("/", controller.UpdateUser)
				adminRoute.DELETE("/:id", controller.DeleteUser)
				adminRoute.DELETE("/:id/reset_passkey", controller.AdminResetPasskey)

				// Admin 2FA routes
				adminRoute.GET("/2fa/stats", controller.Admin2FAStats)
				adminRoute.DELETE("/:id/2fa", controller.AdminDisable2FA)
			}
		}

		// Subscription billing (plans, purchase, admin management)
		subscriptionRoute := apiRouter.Group("/subscription")
		subscriptionRoute.Use(middleware.UserAuth())
		{
			subscriptionRoute.GET("/plans", controller.GetSubscriptionPlans)
			subscriptionRoute.GET("/self", controller.GetSubscriptionSelf)
			subscriptionRoute.PUT("/self/preference", controller.UpdateSubscriptionPreference)
			subscriptionRoute.POST("/balance/pay", middleware.CriticalRateLimit(), controller.SubscriptionRequestBalancePay)
			subscriptionRoute.POST("/epay/pay", middleware.CriticalRateLimit(), controller.SubscriptionRequestEpay)
			subscriptionRoute.POST("/stripe/pay", middleware.CriticalRateLimit(), controller.SubscriptionRequestStripePay)
			subscriptionRoute.POST("/creem/pay", middleware.CriticalRateLimit(), controller.SubscriptionRequestCreemPay)
			subscriptionRoute.POST("/waffo-pancake/pay", middleware.CriticalRateLimit(), controller.SubscriptionRequestWaffoPancakePay)
		}
		subscriptionAdminRoute := apiRouter.Group("/subscription/admin")
		subscriptionAdminRoute.Use(middleware.AdminAuth())
		{
			subscriptionAdminRoute.GET("/plans", controller.AdminListSubscriptionPlans)
			subscriptionAdminRoute.POST("/plans", controller.AdminCreateSubscriptionPlan)
			subscriptionAdminRoute.PUT("/plans/:id", controller.AdminUpdateSubscriptionPlan)
			subscriptionAdminRoute.PATCH("/plans/:id", controller.AdminUpdateSubscriptionPlanStatus)
			subscriptionAdminRoute.POST("/bind", controller.AdminBindSubscription)
			subscriptionAdminRoute.POST("/plans/:id/subscriptions/reset", controller.AdminResetPlanSubscriptions)

			// User subscription management (admin)
			subscriptionAdminRoute.GET("/users/:id/subscriptions", controller.AdminListUserSubscriptions)
			subscriptionAdminRoute.POST("/users/:id/subscriptions", controller.AdminCreateUserSubscription)
			subscriptionAdminRoute.POST("/users/:id/subscriptions/reset", controller.AdminResetUserSubscriptionsByPlan)
			subscriptionAdminRoute.POST("/user_subscriptions/:id/invalidate", controller.AdminInvalidateUserSubscription)
			subscriptionAdminRoute.DELETE("/user_subscriptions/:id", controller.AdminDeleteUserSubscription)
		}

		// Subscription payment callbacks (no auth)
		apiRouter.POST("/subscription/epay/notify", anonymousRequestBodyLimit, controller.SubscriptionEpayNotify)
		apiRouter.GET("/subscription/epay/notify", controller.SubscriptionEpayNotify)
		apiRouter.GET("/subscription/epay/return", controller.SubscriptionEpayReturn)

		// Social Ops (Zernio multi-tenant publish). Isolated from relay; default off via SOCIAL_OPS_ENABLED.
		// fork: social-ops-zernio
		socialRoute := apiRouter.Group("/social/v1")
		{
			socialRoute.GET("/status", controller.SocialStatus)
			// fork: tiktok-direct — official OAuth callback is anonymous (state binds it to the user).
			socialRoute.GET("/oauth/tiktok/callback", controller.SocialTikTokCallback)
			socialRoute.POST("/oauth/tiktok/callback", controller.SocialTikTokCallback)
			socialRoute.GET("/public/media/:token", middleware.DownloadRateLimit(), controller.SocialPublicMediaProxy)
			socialAuthed := socialRoute.Group("/")
			socialAuthed.Use(middleware.UserAuth())
			{
				socialAuthed.GET("/seats", controller.SocialSeats)
				socialAuthed.GET("/accounts", controller.SocialListAccounts)
				socialAuthed.GET("/accounts/:id/creator-info", controller.SocialAccountCreatorInfo)
				socialAuthed.POST("/connect", middleware.CriticalRateLimit(), controller.SocialConnect)
				socialAuthed.DELETE("/accounts/:id", middleware.CriticalRateLimit(), controller.SocialDisconnectAccount)
				socialAuthed.POST("/posts", middleware.CriticalRateLimit(), controller.SocialCreatePost)
				socialAuthed.GET("/posts/:id", controller.SocialGetPost)
				socialAuthed.POST("/media/presign", middleware.CriticalRateLimit(), controller.SocialPresignMedia)

				// Analytics & Insights (Zernio)
				socialAuthed.GET("/analytics/daily-metrics", controller.SocialDailyMetrics)
				socialAuthed.GET("/analytics/best-time-to-post", controller.SocialBestTimeToPost)
				socialAuthed.GET("/analytics/posting-frequency", controller.SocialPostingFrequency)
				socialAuthed.GET("/analytics/content-decay", controller.SocialContentDecay)
				socialAuthed.GET("/accounts/follower-stats", controller.SocialFollowerStats)
				socialAuthed.GET("/analytics/posts", controller.SocialPostsAnalytics)
				socialAuthed.POST("/analytics/sync-external-posts", middleware.CriticalRateLimit(), controller.SocialSyncExternalPosts)

				// Inbox Analytics (Zernio)
				socialAuthed.GET("/inbox-analytics/volume", controller.SocialInboxVolume)
				socialAuthed.GET("/inbox-analytics/response-time", controller.SocialInboxResponseTime)
				socialAuthed.GET("/inbox-analytics/heatmap", controller.SocialInboxHeatmap)
				socialAuthed.GET("/inbox-analytics/source-breakdown", controller.SocialInboxSourceBreakdown)
			}
		}

		// fork: public TikTok tool pages. Anonymous-capable; Turnstile pass,
		// quotas, budget and signed media links are enforced in the handlers.
		tiktokTool := apiRouter.Group("/tools/tiktok")
		tiktokTool.Use(middleware.DisableCache(), middleware.TryUserAuth())
		{
			tiktokTool.GET("/config", middleware.GlobalWebRateLimit(nil), controller.GetTikTokToolConfig)
			tiktokTool.POST("/verify", middleware.TikTokToolVerifyRateLimit(), anonymousRequestBodyLimit, controller.VerifyTikTokToolTurnstile)
			tiktokTool.POST("/resolve", anonymousRequestBodyLimit, controller.ResolveTikTokTool)
			tiktokTool.GET("/file/:kind", middleware.DownloadRateLimit(), controller.GetTikTokToolFile)
			tiktokTool.GET("/zip", middleware.DownloadRateLimit(), controller.GetTikTokToolZip)
		}

		// Inspiration sidecar relay. Default off (INSPIRATION_ENABLED=false); fail-closed when unconfigured.
		// fork: inspiration-gateway
		inspirationRoute := apiRouter.Group("/inspiration/v1")
		{
			inspirationRoute.GET("/status", controller.InspirationStatus)
			// fork: inspiration-categories — public category dictionary & required/optional rules
			inspirationRoute.GET("/categories", middleware.GlobalWebRateLimit(nil), controller.InspirationListCategories)
			// fork: inspiration-publish — direct-link accessible inspiration share lookup
			inspirationRoute.GET("/share/:id", middleware.GlobalWebRateLimit(nil), controller.InspirationGetShare)

			// Anonymous read-only projection for the public inspiration-community
			// pages. Rate limited by the global web limiter; the handlers validate
			// every parameter and project every field.
			inspirationPublic := inspirationRoute.Group("/public")
			inspirationPublic.Use(middleware.GlobalWebRateLimit(nil))
			{
				inspirationPublic.GET("/categories", controller.InspirationListCategories)
				inspirationPublic.GET("/inspirations", controller.InspirationPublicList)
				inspirationPublic.GET("/media/*key", controller.InspirationPublicMedia)
			}
			inspirationAuthed := inspirationRoute.Group("/")
			inspirationAuthed.Use(middleware.UserAuth())
			{
				inspirationAuthed.POST("/query", controller.InspirationQuery)
				inspirationAuthed.GET("/inspirations", controller.InspirationList)
				inspirationAuthed.GET("/inspirations/:id", controller.InspirationGet)
				inspirationAuthed.GET("/tags", controller.InspirationListTags)
				inspirationAuthed.GET("/media/*key", controller.InspirationMedia)
			}
			inspirationAdmin := inspirationRoute.Group("/")
			inspirationAdmin.Use(middleware.AdminAuth())
			{
				inspirationAdmin.POST("/inspirations", middleware.CriticalRateLimit(), controller.InspirationCreate)
				inspirationAdmin.POST("/inspirations/bulk", middleware.CriticalRateLimit(), controller.InspirationBulk)
				inspirationAdmin.PATCH("/inspirations/:id", middleware.CriticalRateLimit(), controller.InspirationUpdate)
				inspirationAdmin.DELETE("/inspirations/:id", middleware.CriticalRateLimit(), controller.InspirationDelete)
				inspirationAdmin.POST("/tags", middleware.CriticalRateLimit(), controller.InspirationCreateTag)
				inspirationAdmin.DELETE("/tags/:id", middleware.CriticalRateLimit(), controller.InspirationDeleteTag)
				inspirationAdmin.POST("/media", middleware.CriticalRateLimit(), controller.InspirationUploadMedia)
			}

			// fork: inspiration-share-convergence — one share capability, two
			// sources (a row already in the inspiration library, or data the
			// caller brings). The two older paths below stay registered as
			// compatibility aliases onto the same implementation.
			inspirationPublishGroup := inspirationRoute.Group("/")
			inspirationPublishGroup.Use(middleware.TokenOrUserAuth())
			{
				inspirationPublishGroup.POST("/share", middleware.CriticalRateLimit(), controller.InspirationShare)
				// fork: inspiration-publish — compatibility aliases.
				inspirationPublishGroup.POST("/publish", middleware.CriticalRateLimit(), controller.InspirationPublish)
				// turn a community inspiration into a direct-link share (the
				// shape the DSH inspiration vertical calls).
				inspirationPublishGroup.POST("/inspirations/:id/share", middleware.CriticalRateLimit(), controller.InspirationCreateShare)
			}
		}

		// Hosted distribution channels (GeeLark). Parallel to Social Ops; quantity is hosted_entitlements.
		// fork: hosted-accounts-domain
		hostedRoute := apiRouter.Group("/hosted/v1")
		{
			hostedRoute.GET("/status", controller.HostedStatus)
			hostedAuthed := hostedRoute.Group("/")
			hostedAuthed.Use(middleware.UserAuth())
			{
				hostedAuthed.GET("/skus", controller.HostedListSKUs)
				hostedAuthed.POST("/orders", middleware.CriticalRateLimit(), controller.HostedCreateOrder)
				hostedAuthed.GET("/orders/:id", controller.HostedGetOrder)
				hostedAuthed.POST("/subscriptions/:id/addons", middleware.CriticalRateLimit(), controller.HostedAddon)
				hostedAuthed.POST("/subscriptions/:id/renew", middleware.CriticalRateLimit(), controller.HostedRenew)
				hostedAuthed.GET("/channels", controller.HostedListChannels)
				hostedAuthed.GET("/channels/:id", controller.HostedGetChannel)
				hostedAuthed.PATCH("/channels/:id/brand", middleware.CriticalRateLimit(), controller.HostedPatchBrand)
				hostedAuthed.POST("/channels/:id/tasks", middleware.CriticalRateLimit(), controller.HostedCreateTask)
				hostedAuthed.GET("/channels/:id/tasks", controller.HostedListChannelTasks)
				hostedAuthed.POST("/tasks/batch", middleware.CriticalRateLimit(), controller.HostedBatchTasks)
				hostedAuthed.GET("/tasks/:id", controller.HostedGetTask)
				hostedAuthed.POST("/tasks/:id/cancel", middleware.CriticalRateLimit(), controller.HostedCancelTask)
				hostedAuthed.POST("/media/presign", middleware.CriticalRateLimit(), controller.HostedPresignMedia)
			}
		}
		// fork: hosted-admin-api
		hostedAdminRoute := apiRouter.Group("/hosted/admin/v1")
		hostedAdminRoute.Use(middleware.AdminAuth())
		{
			hostedAdminRoute.GET("/pool", controller.HostedAdminListPool)
			hostedAdminRoute.GET("/orders", controller.HostedAdminListOrders)
			hostedAdminRoute.GET("/tasks", controller.HostedAdminListTasks)
			hostedAdminRoute.POST("/tasks/:id/cancel", middleware.CriticalRateLimit(), controller.HostedAdminCancelTask)
			hostedAdminRoute.POST("/tasks/:id/retry", middleware.CriticalRateLimit(), controller.HostedAdminRetryTask)
			hostedAdminRoute.GET("/skus", controller.HostedAdminListSKUs)
			hostedAdminRoute.PATCH("/skus/:id", middleware.CriticalRateLimit(), controller.HostedAdminSetSKUEnabled)
			hostedAdminRoute.POST("/sync", middleware.CriticalRateLimit(), controller.HostedAdminSync)
			hostedAdminRoute.GET("/accounts/:id", controller.HostedAdminGetAccount)
			hostedAdminRoute.PATCH("/accounts/:id/allocatable", middleware.CriticalRateLimit(), controller.HostedAdminSetAllocatable)
			hostedAdminRoute.POST("/accounts/:id/credentials", middleware.CriticalRateLimit(), controller.HostedAdminPutCredentials)
		}
		// fork: r2-file-storage — Evolink-compatible file upload & quota endpoints
		fileRoute := apiRouter.Group("/v1/files")
		{
			fileRoute.GET("/download/:file_id", controller.FileDownload)
			fileAuthed := fileRoute.Group("/")
			fileAuthed.Use(middleware.TokenOrUserAuth())
			{
				fileAuthed.POST("/upload/base64", middleware.UploadRateLimit(), controller.FileUploadBase64)
				fileAuthed.POST("/upload/stream", middleware.UploadRateLimit(), controller.FileUploadStream)
				fileAuthed.POST("/upload/url", middleware.UploadRateLimit(), controller.FileUploadURL)
				// fork: r2-file-storage-presign — ticket + verification for direct upload.
				fileAuthed.POST("/upload/presign", middleware.UploadRateLimit(), controller.FilePresignUpload)
				fileAuthed.POST("/upload/confirm", middleware.UploadRateLimit(), controller.FileConfirmUpload)
				fileAuthed.GET("/quota", controller.FileGetQuota)
			}
		}

		apiRouter.POST("/subscription/epay/return", anonymousRequestBodyLimit, controller.SubscriptionEpayReturn)
		optionRoute := apiRouter.Group("/option")
		optionRoute.Use(middleware.RootAuth())
		{
			optionRoute.GET("/", controller.GetOptions)
			optionRoute.GET("/request_policy", controller.GetRequestPolicy)
			optionRoute.PATCH("/request_policy", controller.UpdateRequestPolicy)
			optionRoute.PUT("/", controller.UpdateOption)
			optionRoute.PUT("/passkey/domains", controller.UpdatePasskeyDomains)
			optionRoute.GET("/model_pricing", controller.GetModelPricingConfig)
			optionRoute.PATCH("/model_pricing", controller.UpdateModelPricingConfig)
			optionRoute.POST("/model_pricing/convert", controller.PreviewModelPricingConversion)
			optionRoute.POST("/model_pricing/preview", controller.PreviewModelPricing)
			optionRoute.POST("/payment_compliance", controller.ConfirmPaymentCompliance)
			optionRoute.GET("/channel_affinity_cache", controller.GetChannelAffinityCacheStats)
			optionRoute.DELETE("/channel_affinity_cache", controller.ClearChannelAffinityCache)
			optionRoute.POST("/rest_model_ratio", controller.ResetModelRatio)
			optionRoute.GET("/waffo-pancake/catalog", controller.ListWaffoPancakeCatalog)
			optionRoute.POST("/waffo-pancake/pair", controller.CreateWaffoPancakePair)
			optionRoute.POST("/waffo-pancake/save", controller.SaveWaffoPancake)
			optionRoute.POST("/waffo-pancake/subscription-product", controller.CreateWaffoPancakeSubscriptionProduct)
			optionRoute.GET("/waffo-pancake/subscription-product-options", controller.ListWaffoPancakeSubscriptionProductOptions)
		}

		// Custom OAuth provider management (root only)
		customOAuthRoute := apiRouter.Group("/custom-oauth-provider")
		customOAuthRoute.Use(middleware.RootAuth())
		{
			customOAuthRoute.POST("/discovery", controller.FetchCustomOAuthDiscovery)
			customOAuthRoute.GET("/", controller.GetCustomOAuthProviders)
			customOAuthRoute.GET("/:id", controller.GetCustomOAuthProvider)
			customOAuthRoute.POST("/", controller.CreateCustomOAuthProvider)
			customOAuthRoute.PUT("/:id", controller.UpdateCustomOAuthProvider)
			customOAuthRoute.DELETE("/:id", controller.DeleteCustomOAuthProvider)
		}
		performanceRoute := apiRouter.Group("/performance")
		performanceRoute.Use(middleware.RootAuth())
		{
			performanceRoute.GET("/stats", controller.GetPerformanceStats)
			performanceRoute.DELETE("/disk_cache", controller.ClearDiskCache)
			performanceRoute.POST("/reset_stats", controller.ResetPerformanceStats)
			performanceRoute.POST("/gc", controller.ForceGC)
			performanceRoute.GET("/logs", controller.GetLogFiles)
			performanceRoute.DELETE("/logs", controller.CleanupLogFiles)
		}
		ratioSyncRoute := apiRouter.Group("/ratio_sync")
		ratioSyncRoute.Use(middleware.RootAuth())
		{
			ratioSyncRoute.GET("/channels", controller.GetSyncableChannels)
			ratioSyncRoute.POST("/fetch", controller.FetchUpstreamRatios)
		}
		taskPluginRoute := apiRouter.Group("/plugin/task")
		taskPluginRoute.Use(middleware.RootAuth())
		{
			taskPluginRoute.GET("", controller.ListTaskPlugins)
			taskPluginRoute.POST("", controller.UploadTaskPlugin)
			taskPluginRoute.PUT("", controller.UploadTaskPlugin)
			taskPluginRoute.GET("/runtime/status", controller.GetTaskPluginRuntime)
			taskPluginRoute.GET("/marketplace/sources", controller.GetTaskPluginMarketplaceSources)
			taskPluginRoute.PUT("/marketplace/sources", controller.UpdateTaskPluginMarketplaceSources)
			taskPluginRoute.GET("/:key", controller.GetTaskPlugin)
			taskPluginRoute.GET("/:key/icon", controller.GetTaskPluginIcon)
			taskPluginRoute.GET("/:key/versions", controller.GetTaskPluginVersions)
			taskPluginRoute.POST("/:key/activate", controller.ActivateTaskPlugin)
			taskPluginRoute.POST("/:key/status", controller.SetTaskPluginStatus)
			taskPluginRoute.POST("/:key/dryrun", controller.DryRunTaskPlugin)
			taskPluginRoute.DELETE("/:key/versions/:version", controller.DeleteTaskPluginVersion)
		}
		handlePermissionRoute(apiRouter.Group("", middleware.AdminAuth()), http.MethodGet, "/task_plugin_options", authz.TaskPluginBind, controller.GetTaskPluginOptions)
		registerChannelRoutes(apiRouter)
		registerAuthzRoutes(apiRouter)
		tokenRoute := apiRouter.Group("/token")
		tokenRoute.Use(middleware.UserAuth())
		tokenRoute.Use(middleware.TokenOperationAudit())
		{
			tokenRoute.GET("/", controller.GetAllTokens)
			tokenRoute.GET("/search", middleware.SearchRateLimit(), controller.SearchTokens)
			tokenRoute.GET("/auto-groups", controller.GetTokenAutoGroups)
			tokenRoute.GET("/:id", controller.GetToken)
			tokenRoute.POST("/:id/key", middleware.CriticalRateLimit(), middleware.DisableCache(), controller.GetTokenKey)
			tokenRoute.POST("/", controller.AddToken)
			tokenRoute.PUT("/", controller.UpdateToken)
			tokenRoute.DELETE("/:id", controller.DeleteToken)
			tokenRoute.POST("/batch", controller.DeleteTokenBatch)
			tokenRoute.POST("/batch/keys", middleware.CriticalRateLimit(), middleware.DisableCache(), controller.GetTokenKeysBatch)
		}

		usageRoute := apiRouter.Group("/usage")
		usageRoute.Use(middleware.CORS(), middleware.CriticalRateLimit())
		{
			tokenUsageRoute := usageRoute.Group("/token")
			tokenUsageRoute.Use(middleware.TokenAuthReadOnly())
			{
				tokenUsageRoute.GET("/", controller.GetTokenUsage)
			}
		}

		redemptionRoute := apiRouter.Group("/redemption")
		redemptionRoute.Use(middleware.AdminAuth())
		{
			redemptionRoute.GET("/", controller.GetAllRedemptions)
			redemptionRoute.GET("/search", controller.SearchRedemptions)
			redemptionRoute.GET("/:id", controller.GetRedemption)
			redemptionRoute.POST("/", controller.AddRedemption)
			redemptionRoute.POST("/batch", controller.DeleteRedemptionBatch)
			redemptionRoute.PUT("/", controller.UpdateRedemption)
			redemptionRoute.DELETE("/invalid", controller.DeleteInvalidRedemption)
			redemptionRoute.DELETE("/:id", controller.DeleteRedemption)
		}
		handlePermissionRoute(apiRouter.Group("", middleware.DisableCache(), middleware.AdminAuth()), http.MethodGet, "/audit", authz.AuditRead, controller.GetAuditLogs)
		apiRouter.GET("/audit/self", middleware.DisableCache(), middleware.UserAuth(), controller.GetAuditLogs)
		logRoute := apiRouter.Group("/log")
		logRoute.GET("/", middleware.AdminAuth(), controller.GetAllLogs)
		logRoute.GET("/stat", middleware.AdminAuth(), controller.GetLogsStat)
		logRoute.GET("/self/stat", middleware.UserAuth(), controller.GetLogsSelfStat)
		logRoute.GET("/channel_affinity_usage_cache", middleware.AdminAuth(), controller.GetChannelAffinityUsageCacheStats)
		logRoute.GET("/search", middleware.AdminAuth(), controller.SearchAllLogs)
		logRoute.GET("/self", middleware.UserAuth(), controller.GetUserLogs)
		logRoute.GET("/export", middleware.AdminAuth(), controller.ExportAllLogs)
		logRoute.GET("/self/export", middleware.UserAuth(), controller.ExportUserLogs)
		logRoute.GET("/self/search", middleware.UserAuth(), middleware.SearchRateLimit(), controller.SearchUserLogs)

		systemTaskRoute := apiRouter.Group("/system-task")
		systemTaskRoute.Use(middleware.RootAuth())
		{
			systemTaskRoute.POST("/log-cleanup", controller.CreateLogCleanupSystemTask)
			systemTaskRoute.POST("/image-thumbnail-backfill", controller.ImageThumbnailBackfill)
			systemTaskRoute.GET("/list", controller.ListSystemTasks)
			systemTaskRoute.DELETE("/history", controller.DeleteSystemTaskHistory)
			systemTaskRoute.GET("/current", controller.GetCurrentSystemTask)
			systemTaskRoute.GET("/:task_id", controller.GetSystemTask)
		}
		systemInfoRoute := apiRouter.Group("/system-info")
		systemInfoRoute.Use(middleware.RootAuth())
		{
			systemInfoRoute.GET("/instances", controller.ListSystemInstances)
			systemInfoRoute.DELETE("/stale-instances", controller.DeleteStaleSystemInstances)
			systemInfoRoute.DELETE("/instances/:node_name", controller.DeleteStaleSystemInstance)
		}

		dataRoute := apiRouter.Group("/data")
		dataRoute.GET("/", middleware.AdminAuth(), controller.GetAllQuotaDates)
		dataRoute.GET("/users", middleware.AdminAuth(), controller.GetQuotaDatesByUser)
		dataRoute.GET("/self", middleware.UserAuth(), controller.GetUserQuotaDates)
		dataRoute.GET("/flow", middleware.AdminAuth(), controller.GetAllFlowQuotaDates)
		dataRoute.GET("/flow/self", middleware.UserAuth(), controller.GetUserFlowQuotaDates)

		logRoute.Use(middleware.CORS(), middleware.CriticalRateLimit())
		{
			logRoute.GET("/token", middleware.TokenAuthReadOnly(), controller.GetLogByKey)
		}
		groupRoute := apiRouter.Group("/group")
		groupRoute.Use(middleware.AdminAuth())
		{
			groupRoute.GET("/", controller.GetGroups)
		}

		prefillGroupRoute := apiRouter.Group("/prefill_group")
		prefillGroupRoute.Use(middleware.AdminAuth())
		{
			prefillGroupRoute.GET("/", controller.GetPrefillGroups)
			prefillGroupRoute.POST("/", controller.CreatePrefillGroup)
			prefillGroupRoute.PUT("/", controller.UpdatePrefillGroup)
			prefillGroupRoute.DELETE("/:id", controller.DeletePrefillGroup)
		}

		mjRoute := apiRouter.Group("/mj")
		mjRoute.GET("/self", middleware.UserAuth(), controller.GetUserMidjourney)
		mjRoute.GET("/", middleware.AdminAuth(), controller.GetAllMidjourney)

		taskRoute := apiRouter.Group("/task")
		{
			taskRoute.GET("/self", middleware.UserAuth(), controller.GetUserTask)
			taskRoute.GET("", middleware.AdminAuth(), controller.GetAllTask)
			taskRoute.GET("/:task_id/artifacts", middleware.UserAuth(), controller.GetDashboardTaskArtifacts)
		}

		vendorRoute := apiRouter.Group("/vendors")
		vendorRoute.Use(middleware.AdminAuth())
		{
			vendorRoute.POST("/operations/preview", controller.PreviewVendorOperation)
			vendorRoute.POST("/operations", controller.ApplyVendorOperation)
			vendorRoute.GET("/", controller.GetAllVendors)
			vendorRoute.GET("/search", controller.SearchVendors)
			vendorRoute.GET("/:id", controller.GetVendorMeta)
			vendorRoute.POST("/", controller.CreateVendorMeta)
			vendorRoute.PUT("/", controller.UpdateVendorMeta)
			vendorRoute.DELETE("/:id", controller.DeleteVendorMeta)
		}

		modelsRoute := apiRouter.Group("/models")
		modelsRoute.Use(middleware.AdminAuth())
		{
			modelsRoute.GET("/sync_upstream/preview", controller.SyncUpstreamPreview)
			modelsRoute.POST("/sync_upstream", controller.SyncUpstreamModels)
			modelsRoute.POST("/delete", controller.BatchDeleteModelMeta)
			modelsRoute.GET("/missing", controller.GetMissingModels)
			modelsRoute.GET("/", controller.GetAllModelsMeta)
			modelsRoute.GET("/search", controller.SearchModelsMeta)
			modelsRoute.GET("/:id", controller.GetModelMeta)
			modelsRoute.POST("/", controller.CreateModelMeta)
			modelsRoute.PUT("/", controller.UpdateModelMeta)
			modelsRoute.DELETE("/:id", controller.DeleteModelMeta)
		}

		// Deployments (model deployment management)
		deploymentsRoute := apiRouter.Group("/deployments")
		deploymentsRoute.Use(middleware.AdminAuth())
		{
			deploymentsRoute.GET("/settings", controller.GetModelDeploymentSettings)
			deploymentsRoute.POST("/settings/test-connection", controller.TestIoNetConnection)
			deploymentsRoute.GET("/", controller.GetAllDeployments)
			deploymentsRoute.GET("/search", controller.SearchDeployments)
			deploymentsRoute.POST("/test-connection", controller.TestIoNetConnection)
			deploymentsRoute.GET("/hardware-types", controller.GetHardwareTypes)
			deploymentsRoute.GET("/locations", controller.GetLocations)
			deploymentsRoute.GET("/available-replicas", controller.GetAvailableReplicas)
			deploymentsRoute.POST("/price-estimation", controller.GetPriceEstimation)
			deploymentsRoute.GET("/check-name", controller.CheckClusterNameAvailability)
			deploymentsRoute.POST("/", controller.CreateDeployment)

			deploymentsRoute.GET("/:id", controller.GetDeployment)
			deploymentsRoute.GET("/:id/logs", controller.GetDeploymentLogs)
			deploymentsRoute.GET("/:id/containers", controller.ListDeploymentContainers)
			deploymentsRoute.GET("/:id/containers/:container_id", controller.GetContainerDetails)
			deploymentsRoute.PUT("/:id", controller.UpdateDeployment)
			deploymentsRoute.PUT("/:id/name", controller.UpdateDeploymentName)
			deploymentsRoute.POST("/:id/extend", controller.ExtendDeployment)
			deploymentsRoute.DELETE("/:id", controller.DeleteDeployment)
		}
	}
}
