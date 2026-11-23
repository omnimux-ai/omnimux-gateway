package middleware

import (
	"fmt"
	"net/url"
	"path"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

const RouteTagKey = "route_tag"

const (
	socialOAuthLogPrefix       = "/api/social/v1/oauth/"
	socialPublicMediaLogPrefix = "/api/social/v1/public/media/"
	redactedLogPlaceholder     = "[REDACTED]"
)

var sensitiveLogQueryKeys = map[string]struct{}{
	"ticket":        {},
	"code":          {},
	"state":         {},
	"access_token":  {},
	"refresh_token": {},
	"id_token":      {},
}

func RouteTag(tag string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set(RouteTagKey, tag)
		c.Next()
	}
}

func SetUpLogger(server *gin.Engine) {
	server.Use(redactTaskArtifactAccessQuery())
	server.Use(gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		var requestID string
		if param.Keys != nil {
			requestID, _ = param.Keys[common.RequestIdKey].(string)
		}
		tag, _ := param.Keys[RouteTagKey].(string)
		if tag == "" {
			tag = "web"
		}
		// Redact sensitive path segments and query params in the log line only;
		// HTTP handlers continue receiving the intact original path and query.
		logPath := redactAccessLogParam(param)
		return fmt.Sprintf("[GIN] %s | %s | %s | %3d | %13v | %15s | %7s %s\n",
			param.TimeStamp.Format("2006/01/02 - 15:04:05"),
			tag,
			requestID,
			param.StatusCode,
			param.Latency,
			param.ClientIP,
			param.Method,
			logPath,
		)
	}))
}

func redactAccessLogParam(param gin.LogFormatterParams) string {
	if param.Request != nil && param.Request.URL != nil {
		if rawQuery := param.Request.URL.RawQuery; rawQuery != "" {
			suffix := "?" + rawQuery
			if strings.HasSuffix(param.Path, suffix) {
				pathPart := param.Path[:len(param.Path)-len(suffix)]
				return redactAccessLogPathParts(pathPart, rawQuery, true)
			}
		}
	}
	return redactAccessLogPath(param.Path)
}

func redactAccessLogPath(raw string) string {
	pathPart, queryPart, hasQuery := strings.Cut(raw, "?")
	return redactAccessLogPathParts(pathPart, queryPart, hasQuery)
}

func redactAccessLogPathParts(pathPart, queryPart string, hasQuery bool) string {
	collapsed := pathPart
	for strings.Contains(collapsed, "//") {
		collapsed = strings.ReplaceAll(collapsed, "//", "/")
	}
	cleaned := path.Clean(pathPart)

	// Signed public media proxy URLs carry a bearer-like token in the path.
	// Redact the token segment(s) and drop any query string.
	if strings.HasPrefix(collapsed, socialPublicMediaLogPrefix) {
		rest := strings.TrimPrefix(collapsed, socialPublicMediaLogPrefix)
		if strings.Trim(rest, "/") != "" {
			return socialPublicMediaLogPrefix + redactedLogPlaceholder
		}
		return socialPublicMediaLogPrefix
	}
	if strings.HasPrefix(cleaned, socialPublicMediaLogPrefix) {
		return socialPublicMediaLogPrefix + redactedLogPlaceholder
	}

	// OAuth launch and callback routes carry one-time tickets, codes, state, and scopes in query params.
	if hasOAuthLogPrefix(collapsed) || hasOAuthLogPrefix(cleaned) {
		return pathPart
	}

	if !hasQuery || queryPart == "" {
		if hasQuery {
			return pathPart + "?"
		}
		return pathPart
	}
	redactedQuery, changed := redactSensitiveQueryString(queryPart)
	if !changed {
		return pathPart + "?" + queryPart
	}
	return pathPart + "?" + redactedQuery
}

func hasOAuthLogPrefix(p string) bool {
	return strings.HasPrefix(p, "/api/oauth/") ||
		strings.HasPrefix(p, "/oauth/") ||
		strings.HasPrefix(p, socialOAuthLogPrefix)
}

func redactSensitiveQueryString(rawQuery string) (string, bool) {
	changed := false
	parts := strings.Split(rawQuery, "&")
	for i, part := range parts {
		if part == "" {
			continue
		}
		if strings.Contains(part, ";") {
			subParts := strings.Split(part, ";")
			subChanged := false
			for j, sub := range subParts {
				if redacted, ok := redactSensitiveQueryParam(sub); ok {
					subParts[j] = redacted
					subChanged = true
				}
			}
			if subChanged {
				parts[i] = strings.Join(subParts, ";")
				changed = true
			}
			continue
		}
		if redacted, ok := redactSensitiveQueryParam(part); ok {
			parts[i] = redacted
			changed = true
		}
	}
	if !changed {
		return rawQuery, false
	}
	return strings.Join(parts, "&"), true
}

func redactSensitiveQueryParam(part string) (string, bool) {
	rawKey, _, hasEq := strings.Cut(part, "=")
	if !hasEq {
		return part, false
	}
	decodedKey, err := url.QueryUnescape(rawKey)
	if err != nil {
		decodedKey = rawKey
	}
	if _, sensitive := sensitiveLogQueryKeys[strings.ToLower(strings.TrimSpace(decodedKey))]; !sensitive {
		return part, false
	}
	return rawKey + "=" + redactedLogPlaceholder, true
}
