package middleware

import (
	"net/url"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func CORS() gin.HandlerFunc {
	config := cors.DefaultConfig()
	config.AllowCredentials = true
	config.AllowMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"}
	config.AllowHeaders = []string{
		"Origin",
		"Content-Length",
		"Content-Type",
		"Accept",
		"Authorization",
		"X-Requested-With",
		"Cache-Control",
		"New-API-User",
		"New-Api-User",
		"Turnstile-Token",
		"CF-Turnstile-Response",
	}
	config.ExposeHeaders = []string{
		"Content-Length",
		"Content-Type",
		"X-Oneapi-Request-Id",
		"X-New-Api-Version",
	}
	config.AllowOriginFunc = isAllowedCORSOrigin
	return cors.New(config)
}

func isAllowedCORSOrigin(origin string) bool {
	if origin == "" {
		return true
	}
	if common.IsSessionCookieTrustedOrigin(origin) {
		return true
	}
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.EscapedPath() != "" {
		return false
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	switch host {
	case "localhost", "127.0.0.1", "::1":
		return true
	}
	if scheme != "https" {
		return false
	}
	switch host {
	case "ai.prox.us.ci", "api.prox.us.ci":
		return true
	}
	return strings.HasSuffix(host, ".prox.us.ci")
}

func PoweredBy() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-New-Api-Version", common.Version)
		c.Next()
	}
}
