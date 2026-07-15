package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func CleanupLegacySessionCookies(currentCookieName string, cookieDomain string) gin.HandlerFunc {
	return func(c *gin.Context) {
		clearLegacyCookie(c, "session", "")
		if cookieDomain != "" {
			clearLegacyCookie(c, "session", cookieDomain)
		}

		trimmedCurrent := strings.TrimSpace(currentCookieName)
		if trimmedCurrent != "" && trimmedCurrent != "session" {
			// Clear stale host-only cookies left by older deployments. This keeps
			// the current domain-scoped cookie intact while removing same-name
			// host-only duplicates that browsers may send first.
			clearLegacyCookie(c, trimmedCurrent, "")
		}

		c.Next()
	}
}

func clearLegacyCookie(c *gin.Context, name string, domain string) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     "/",
		Domain:   domain,
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})
}
