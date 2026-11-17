package middleware

import (
	"github.com/gin-gonic/gin"
)

const (
	// PublicHTMLCacheControl allows shared caches to keep a short public HTML
	// copy while browsers revalidate. The edge TTL is intentionally short so a
	// deploy is visible after at most one window; ops may tune s-maxage.
	PublicHTMLCacheControl = "public, max-age=0, s-maxage=300, stale-while-revalidate=86400"
	// PrivateHTMLCacheControl keeps application shells and private pages out of
	// shared caches. API handlers use the stronger DisableCache policy.
	PrivateHTMLCacheControl = "no-cache"
)

// SetHTMLCacheHeaders applies the public/private HTML cache boundary and keeps
// the content-encoding variant explicit for shared caches.
func SetHTMLCacheHeaders(c *gin.Context, public bool) {
	if public {
		c.Header("Cache-Control", PublicHTMLCacheControl)
	} else {
		c.Header("Cache-Control", PrivateHTMLCacheControl)
	}
	c.Header("Vary", "Accept-Encoding")
}

func Cache() func(c *gin.Context) {
	return func(c *gin.Context) {
		if c.Request.RequestURI == "/" {
			SetHTMLCacheHeaders(c, true)
		} else {
			// Static assets keep their existing one-week browser cache. HTML
			// handlers below override this with the public/private policy.
			c.Header("Cache-Control", "max-age=604800")
		}
		c.Header("Cache-Version", "b688f2fb5be447c25e5aa3bd063087a83db32a288bf6a4f35f2d8db310e40b14")
		c.Next()
	}
}
