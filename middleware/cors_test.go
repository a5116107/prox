package middleware

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestAllowedCORSOriginUsesExactTrustedOrigins(t *testing.T) {
	oldTrustedOrigins := common.SessionCookieTrustedURLs
	common.SessionCookieTrustedURLs = []string{"https://console.example.net:8443"}
	t.Cleanup(func() { common.SessionCookieTrustedURLs = oldTrustedOrigins })

	require.True(t, isAllowedCORSOrigin("https://console.example.net:8443"))
	require.False(t, isAllowedCORSOrigin("https://console.example.net"))
	require.False(t, isAllowedCORSOrigin("https://console.example.net.evil.test:8443"))
	require.False(t, isAllowedCORSOrigin("http://console.example.net:8443"))
}

func TestAllowedCORSOriginSeparatesLocalAndProductionSchemes(t *testing.T) {
	oldTrustedOrigins := common.SessionCookieTrustedURLs
	common.SessionCookieTrustedURLs = nil
	t.Cleanup(func() { common.SessionCookieTrustedURLs = oldTrustedOrigins })

	require.True(t, isAllowedCORSOrigin(""))
	require.True(t, isAllowedCORSOrigin("http://localhost:3000"))
	require.True(t, isAllowedCORSOrigin("https://admin.prox.us.ci"))
	require.False(t, isAllowedCORSOrigin("https://console.other-site.example"))
	require.False(t, isAllowedCORSOrigin("http://admin.prox.us.ci"))
	require.False(t, isAllowedCORSOrigin("chrome-extension://ai.prox.us.ci"))
	require.False(t, isAllowedCORSOrigin("https://prox.us.ci.evil.test"))
	require.False(t, isAllowedCORSOrigin("https://admin.prox.us.ci/path"))
}
