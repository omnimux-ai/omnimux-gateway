package router

import (
	"strings"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/relay"
	"github.com/QuantumNous/new-api/relaykit/types"

	"github.com/gin-gonic/gin"
)

func SetRelayRouter(router *gin.Engine) {
	router.Use(middleware.CORS())
	router.Use(middleware.DecompressRequestMiddleware())
	router.Use(middleware.BodyStorageCleanup()) // 清理请求体存储
	router.Use(middleware.StatsMiddleware())
	// https://platform.openai.com/docs/api-reference/introduction
	// fork: r2-file-storage — Evolink & OpenAI-compatible /v1/files upload & quota
	v1FileRoute := router.Group("/v1/files")
	{
		v1FileRoute.GET("/download/:file_id", controller.FileDownload)
		v1FileAuthed := v1FileRoute.Group("/")
		v1FileAuthed.Use(middleware.TokenOrUserAuth())
		{
			v1FileAuthed.POST("/upload/base64", middleware.UploadRateLimit(), controller.FileUploadBase64)
			v1FileAuthed.POST("/upload/stream", middleware.UploadRateLimit(), controller.FileUploadStream)
			v1FileAuthed.POST("/upload/url", middleware.UploadRateLimit(), controller.FileUploadURL)
			// fork: r2-file-storage-presign — same ticket + verification on the v1 alias.
			v1FileAuthed.POST("/upload/presign", middleware.UploadRateLimit(), controller.FilePresignUpload)
			v1FileAuthed.POST("/upload/confirm", middleware.UploadRateLimit(), controller.FileConfirmUpload)
			v1FileAuthed.GET("/quota", controller.FileGetQuota)
		}
	}

	modelsRouter := router.Group("/v1/models")
	modelsRouter.Use(middleware.RouteTag("relay"))
	modelsRouter.Use(middleware.TokenAuth())
	{
		modelsRouter.GET("", func(c *gin.Context) {
			switch {
			case c.GetHeader("x-api-key") != "" && c.GetHeader("anthropic-version") != "":
				controller.ListModels(c, constant.ChannelTypeAnthropic)
			case c.GetHeader("x-goog-api-key") != "" || c.Query("key") != "": // 单独的适配
				controller.ListModels(c, constant.ChannelTypeGemini)
			default:
				controller.ListModels(c, constant.ChannelTypeOpenAI)
			}
		})

		modelsRouter.GET("/:model", func(c *gin.Context) {
			switch {
			case c.GetHeader("x-api-key") != "" && c.GetHeader("anthropic-version") != "":
				controller.RetrieveModel(c, constant.ChannelTypeAnthropic)
			default:
				controller.RetrieveModel(c, constant.ChannelTypeOpenAI)
			}
		})
	}

	geminiRouter := router.Group("/v1beta/models")
	geminiRouter.Use(middleware.RouteTag("relay"))
	geminiRouter.Use(middleware.TokenAuth())
	{
		geminiRouter.GET("", func(c *gin.Context) {
			controller.ListModels(c, constant.ChannelTypeGemini)
		})
	}

	geminiCompatibleRouter := router.Group("/v1beta/openai/models")
	geminiCompatibleRouter.Use(middleware.RouteTag("relay"))
	geminiCompatibleRouter.Use(middleware.TokenAuth())
	{
		geminiCompatibleRouter.GET("", func(c *gin.Context) {
			controller.ListModels(c, constant.ChannelTypeOpenAI)
		})
	}

	playgroundRouter := router.Group("/pg")
	playgroundRouter.Use(middleware.RouteTag("relay"))
	playgroundRouter.Use(middleware.SystemPerformanceCheck())
	playgroundRouter.Use(middleware.UserAuth(), middleware.Distribute())
	{
		playgroundRouter.POST("/chat/completions", controller.Playground)
	}

	// Generation endpoints for the browser playground. The video route takes the
	// same task-plugin chain, resolution tier selection, channel distribution and
	// native precharge/settle path as /v1/video/generations, with the session
	// cookie (UserAuth) standing in for a personal access token. Registered as
	// its own group so it inherits neither the chat group's Distribute nor a
	// second distribution pass.
	playgroundTaskRouter := router.Group("/pg")
	playgroundTaskRouter.Use(middleware.RouteTag("relay"))
	playgroundTaskRouter.Use(middleware.SystemPerformanceCheck())
	playgroundTaskRouter.Use(middleware.UserAuth())
	{
		playgroundTaskRouter.POST(
			"/video/generations",
			middleware.PinTaskPluginEndpoint(),
			middleware.TaskPluginEndpointOnly(middleware.ModelRequestRateLimit()),
			middleware.PrepareTaskPluginEndpoint(),
			// fork: wan30-resolution-tier-group — the browser playground submits to
			// this route, so the tier group must be pinned here too, ahead of
			// distribution. The image routes below serve models without tiers.
			middleware.SelectVideoTierGroup(),
			middleware.Distribute(),
			controller.PlaygroundTask,
		)
		playgroundTaskRouter.POST(
			"/images/generations",
			middleware.PinTaskPluginEndpoint(),
			middleware.TaskPluginEndpointOnly(middleware.ModelRequestRateLimit()),
			middleware.PrepareTaskPluginEndpoint(),
			middleware.Distribute(),
			controller.PlaygroundTask,
		)
		// The edit surface takes the same chain: a reference-carrying image
		// submission differs only in the upstream path the adaptor selects.
		playgroundTaskRouter.POST(
			"/images/edits",
			middleware.PinTaskPluginEndpoint(),
			middleware.TaskPluginEndpointOnly(middleware.ModelRequestRateLimit()),
			middleware.PrepareTaskPluginEndpoint(),
			middleware.Distribute(),
			controller.PlaygroundTask,
		)
		// fork: influencer workbench — the controller validates the structured
		// sheet payload, assembles the prompt server-side, rewrites the body to
		// the canonical image submit shape and re-enters RelayTask, so billing,
		// queueing and the result proxy stay native.
		playgroundTaskRouter.POST(
			"/influencer/sheet",
			controller.PrepareInfluencerSheet,
			middleware.ModelRequestRateLimit(),
			middleware.Distribute(),
			controller.PostInfluencerSheet,
		)
		// fork: influencer workbench — a turnaround sheet derived from an
		// existing sheet task. Same chain as the sheet action: the controller
		// reads the character description from the persisted parent, so only the
		// model, group and reference image come from the caller.
		playgroundTaskRouter.POST(
			"/influencer/multiview",
			controller.PrepareInfluencerMultiView,
			middleware.ModelRequestRateLimit(),
			middleware.Distribute(),
			controller.PostInfluencerMultiView,
		)
		// fork: influencer motion video — the form is validated and rewritten to
		// the canonical video submit body before distribution, so the channel is
		// picked from the tier group the resolution implies; the handler then
		// adds the reference clip's own sound and re-enters RelayTask.
		playgroundTaskRouter.POST(
			"/influencer/motion",
			controller.PrepareInfluencerMotion,
			middleware.ModelRequestRateLimit(),
			middleware.Distribute(),
			controller.PostInfluencerMotion,
		)
		// fork: read-only capability surface for the motion form; keeps the
		// offered output framing honest to the bound supplier lines.
		playgroundTaskRouter.GET(
			"/influencer/motion/options",
			controller.InfluencerMotionOptions,
		)
		playgroundTaskRouter.POST(
			"/influencer/motion/inspect",
			middleware.UploadRateLimit(),
			middleware.UserCriticalRateLimit("influencer_motion_inspect"),
			controller.InspectInfluencerMotionVideo,
		)
		// fork: influencer viral recreation — the reference clip is decomposed
		// here and only the compiled prompt plus the breakdown return; no model or
		// group is selected by the caller and no video task is created.
		playgroundTaskRouter.POST(
			"/influencer/recreate/analyze",
			middleware.UserCriticalRateLimit("influencer_recreate_analyze"),
			controller.PostInfluencerRecreateAnalyze,
		)
		// The decomposition runs in the generation queue; the workbench polls the
		// owner-scoped job instead of holding a proxied request open for minutes.
		playgroundTaskRouter.GET(
			"/influencer/recreate/:task_id",
			controller.GetInfluencerRecreateAnalysis,
		)
	}
	relayV1Router := router.Group("/v1")
	relayV1Router.Use(middleware.RouteTag("relay"))
	relayV1Router.Use(middleware.SystemPerformanceCheck())
	relayV1Router.Use(middleware.TokenAuth())
	{
		// Responses WebSocket route. Channel selection happens after the first
		// response.create event; each event runs the ordinary request limiter.
		relayV1Router.GET("/responses", controller.ResponsesWebSocket)
	}
	relayV1Router.Use(middleware.ModelRequestRateLimit())
	{
		// WebSocket 路由（统一到 Relay）
		wsRouter := relayV1Router.Group("")
		wsRouter.Use(middleware.Distribute())
		wsRouter.GET("/realtime", func(c *gin.Context) {
			controller.Relay(c, types.RelayFormatOpenAIRealtime)
		})
	}
	{
		//http router
		httpRouter := relayV1Router.Group("")
		httpRouter.Use(middleware.Distribute())

		// claude related routes
		// TODO: /messages/count_tokens is disabled. The current controller.CountClaudeTokens
		// httpRouter.POST("/messages/count_tokens", controller.CountClaudeTokens)
		httpRouter.POST("/messages", func(c *gin.Context) {
			controller.Relay(c, types.RelayFormatClaude)
		})

		// chat related routes
		httpRouter.POST("/completions", func(c *gin.Context) {
			controller.Relay(c, types.RelayFormatOpenAI)
		})
		httpRouter.POST("/chat/completions", func(c *gin.Context) {
			controller.Relay(c, types.RelayFormatOpenAI)
		})

		// response related routes
		httpRouter.POST("/responses/compact", func(c *gin.Context) {
			controller.Relay(c, types.RelayFormatOpenAIResponsesCompaction)
		})

		// alpha search related routes (Codex standalone web search)
		httpRouter.POST("/alpha/search", func(c *gin.Context) {
			controller.Relay(c, types.RelayFormatOpenAIAlphaSearch)
		})

		// fork: image host protocols own /images/generations and /images/edits;
		// their unclaimed-model fallback retains RelayImage's async routing.
		// Async image task poll (GoEasy / future image task platforms)
		httpRouter.GET("/images/generations/:task_id", controller.RelayTaskFetch)
		httpRouter.GET("/images/tasks/:task_id", controller.RelayTaskFetch)

		// embedding related routes
		httpRouter.POST("/embeddings", func(c *gin.Context) {
			controller.Relay(c, types.RelayFormatEmbedding)
		})

		// audio related routes
		httpRouter.POST("/audio/transcriptions", func(c *gin.Context) {
			controller.Relay(c, types.RelayFormatOpenAIAudio)
		})
		httpRouter.POST("/audio/translations", func(c *gin.Context) {
			controller.Relay(c, types.RelayFormatOpenAIAudio)
		})
		httpRouter.POST("/audio/speech", func(c *gin.Context) {
			controller.Relay(c, types.RelayFormatOpenAIAudio)
		})

		// rerank related routes
		httpRouter.POST("/rerank", func(c *gin.Context) {
			controller.Relay(c, types.RelayFormatRerank)
		})

		// jina reader route (URL -> LLM-friendly text)
		httpRouter.POST("/reader", func(c *gin.Context) {
			controller.Relay(c, types.RelayFormatReader)
		})

		// TypeSafe AI System One (Jev) native protocol.
		httpRouter.POST("/systemone", func(c *gin.Context) {
			controller.Relay(c, types.RelayFormatSystemOne)
		})

		// gemini relay routes
		httpRouter.POST("/engines/:model/embeddings", func(c *gin.Context) {
			controller.Relay(c, types.RelayFormatGemini)
		})
		httpRouter.POST("/models/*path", func(c *gin.Context) {
			controller.Relay(c, types.RelayFormatGemini)
		})

		// other relay routes
		httpRouter.POST("/moderations", func(c *gin.Context) {
			controller.Relay(c, types.RelayFormatOpenAI)
		})

		// not implemented
		httpRouter.POST("/images/variations", controller.RelayNotImplemented)
		httpRouter.GET("/files", controller.RelayNotImplemented)
		httpRouter.POST("/files", controller.RelayNotImplemented)
		httpRouter.DELETE("/files/:id", controller.RelayNotImplemented)
		httpRouter.GET("/files/:id", controller.RelayNotImplemented)
		httpRouter.GET("/files/:id/content", controller.RelayNotImplemented)
		httpRouter.POST("/fine-tunes", controller.RelayNotImplemented)
		httpRouter.GET("/fine-tunes", controller.RelayNotImplemented)
		httpRouter.GET("/fine-tunes/:id", controller.RelayNotImplemented)
		httpRouter.POST("/fine-tunes/:id/cancel", controller.RelayNotImplemented)
		httpRouter.GET("/fine-tunes/:id/events", controller.RelayNotImplemented)
		httpRouter.DELETE("/models/:model", controller.RelayNotImplemented)
	}

	relayMjRouter := router.Group("/mj")
	relayMjRouter.Use(middleware.RouteTag("relay"))
	relayMjRouter.Use(middleware.SystemPerformanceCheck())
	registerMjRouterGroup(relayMjRouter)

	relayMjModeRouter := router.Group("/:mode/mj")
	relayMjModeRouter.Use(middleware.RouteTag("relay"))
	relayMjModeRouter.Use(middleware.SystemPerformanceCheck())
	registerMjRouterGroup(relayMjModeRouter)
	//relayMjRouter.Use()

	relayGeminiRouter := router.Group("/v1beta")
	// :countTokens is not implemented. Answer it like an unregistered route
	// before auth/channel selection instead of silently relaying it as
	// generateContent (#7283).
	relayGeminiRouter.Use(func(c *gin.Context) {
		if strings.HasSuffix(c.Request.URL.Path, ":countTokens") {
			controller.RelayNotFound(c)
			c.Abort()
		}
	})
	relayGeminiRouter.Use(middleware.RouteTag("relay"))
	relayGeminiRouter.Use(middleware.SystemPerformanceCheck())
	relayGeminiRouter.Use(middleware.TokenAuth())
	relayGeminiRouter.Use(middleware.ModelRequestRateLimit())
	relayGeminiRouter.Use(middleware.Distribute())
	{
		// Gemini API 路径格式: /v1beta/models/{model_name}:{action}
		relayGeminiRouter.POST("/models/*path", func(c *gin.Context) {
			controller.Relay(c, types.RelayFormatGemini)
		})
	}
}

func registerMjRouterGroup(relayMjRouter *gin.RouterGroup) {
	relayMjRouter.GET("/image/:id", relay.RelayMidjourneyImage)
	relayMjRouter.Use(middleware.TokenAuth(), middleware.Distribute())
	{
		relayMjRouter.POST("/submit/action", controller.RelayMidjourney)
		relayMjRouter.POST("/submit/shorten", controller.RelayMidjourney)
		relayMjRouter.POST("/submit/modal", controller.RelayMidjourney)
		relayMjRouter.POST("/submit/imagine", controller.RelayMidjourney)
		relayMjRouter.POST("/submit/change", controller.RelayMidjourney)
		relayMjRouter.POST("/submit/simple-change", controller.RelayMidjourney)
		relayMjRouter.POST("/submit/describe", controller.RelayMidjourney)
		relayMjRouter.POST("/submit/blend", controller.RelayMidjourney)
		relayMjRouter.POST("/submit/edits", controller.RelayMidjourney)
		relayMjRouter.POST("/submit/video", controller.RelayMidjourney)
		//relayMjRouter.POST("/notify", controller.RelayMidjourney)
		relayMjRouter.GET("/task/:id/fetch", controller.RelayMidjourney)
		relayMjRouter.GET("/task/:id/image-seed", controller.RelayMidjourney)
		relayMjRouter.POST("/task/list-by-condition", controller.RelayMidjourney)
		relayMjRouter.POST("/insight-face/swap", controller.RelayMidjourney)
		relayMjRouter.POST("/submit/upload-discord-images", controller.RelayMidjourney)
	}
}
