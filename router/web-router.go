package router

import (
	"embed"
	"net/http"
	"os"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
)

// WebAssets holds the embedded dashboard frontend assets.
type WebAssets struct {
	BuildFS   embed.FS
	IndexPage []byte
}

func SetWebRouter(router *gin.Engine, assets WebAssets, pluginDispatcher gin.HandlerFunc) {
	frontendFS := common.EmbedFolder(assets.BuildFS, "web/dist")
	compress := gzip.Gzip(gzip.DefaultCompression)
	// fork: assemble all page types before injecting the existing Umami script.
	searchManifest, manifestErr := assets.BuildFS.ReadFile("web/dist/search-pages.json")
	searchPages, pageErr := buildWebSearchPages(assets.IndexPage, searchManifest)
	searchPages.injectUmami(os.Getenv("UMAMI_WEBSITE_ID"), os.Getenv("UMAMI_SCRIPT_URL"))
	if manifestErr != nil {
		common.SysError(common.LogText("frontend search-page manifest unavailable; SPA pages are noindex: %s", manifestErr.Error()))
	} else if pageErr != nil {
		common.SysError(common.LogText("frontend search-page manifest invalid; unavailable pages are noindex: %s", pageErr.Error()))
	}
	appManifest, appManifestErr := assets.BuildFS.ReadFile("web/dist/app-routes.json")
	appRoutes, appRoutesErr := buildWebAppRoutes(appManifest)
	if appManifestErr != nil {
		common.SysError(common.LogText("frontend application-route manifest unavailable; retaining noindex SPA fallback: %s", appManifestErr.Error()))
	} else if appRoutesErr != nil {
		common.SysError(common.LogText("frontend application-route manifest invalid; retaining noindex SPA fallback: %s", appRoutesErr.Error()))
	}
	servePage := appRoutes.serve(searchPages)

	// CLI install one-liner: always serve /install.sh from the embedded
	// frontend public asset when present; otherwise redirect to the public
	// release-repo copy so curl -fsSL https://omnimux.ai/install.sh works
	// even if SPA NoRoute would otherwise swallow the path.
	router.GET("/install.sh", func(c *gin.Context) {
		c.Set(middleware.RouteTagKey, "web")
		data, err := assets.BuildFS.ReadFile("web/dist/install.sh")
		if err != nil {
			c.Redirect(http.StatusFound, "https://raw.githubusercontent.com/laozhong86/omnimux-cli/main/install.sh")
			return
		}
		c.Header("Cache-Control", "no-cache")
		c.Header("Content-Type", "text/x-shellscript; charset=utf-8")
		c.Data(http.StatusOK, "text/x-shellscript; charset=utf-8", data)
	})

	router.GET("/install.ps1", func(c *gin.Context) {
		c.Set(middleware.RouteTagKey, "web")
		data, err := assets.BuildFS.ReadFile("web/dist/install.ps1")
		if err != nil {
			c.Redirect(http.StatusFound, "https://raw.githubusercontent.com/laozhong86/omnimux-cli/main/install.ps1")
			return
		}
		c.Header("Cache-Control", "no-cache")
		c.Data(http.StatusOK, "text/plain; charset=utf-8", data)
	})

	router.NoRoute(
		pluginDispatcher,
		middleware.RouteTag("web"),
		func(c *gin.Context) {
			// ServeFrontendFiles sends build files already compressed.
			if !frontendFS.Exists("/", c.Request.URL.Path) {
				compress(c)
			}
		},
		middleware.AccessTokenAudit(),
		middleware.GlobalWebRateLimit(frontendFS),
		middleware.Cache(),
		func(c *gin.Context) {
			switch c.Request.URL.Path {
			case "/index.html":
				location := "/"
				if c.Request.URL.RawQuery != "" {
					location += "?" + c.Request.URL.RawQuery
				}
				c.Header("Cache-Control", "no-cache")
				c.Redirect(http.StatusMovedPermanently, location)
				c.Abort()
			case "/search-pages.json", "/app-routes.json":
				c.Header("Cache-Control", "no-cache")
				c.Header("X-Robots-Tag", "noindex, follow")
				c.AbortWithStatus(http.StatusNotFound)
			case "/robots.txt", "/sitemap.xml":
				c.Header("Cache-Control", "no-cache")
			default:
				if strings.HasSuffix(c.Request.URL.Path, ".html") {
					c.Header("X-Robots-Tag", "noindex, follow")
				}
			}
		},
		middleware.ServeFrontendFiles(frontendFS),
		func(c *gin.Context) {
			if strings.HasPrefix(c.Request.RequestURI, "/v1") || strings.HasPrefix(c.Request.RequestURI, "/api") || strings.HasPrefix(c.Request.RequestURI, "/assets") {
				controller.RelayNotFound(c)
				return
			}
			servePage(c)
		},
	)
}
