package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func configureRateLimitTest(t *testing.T) {
	t.Helper()

	originalRedisEnabled := common.RedisEnabled
	originalWebEnabled := common.GlobalWebRateLimitEnable
	originalWebNum := common.GlobalWebRateLimitNum
	originalWebDuration := common.GlobalWebRateLimitDuration
	originalAPIEnabled := common.GlobalApiRateLimitEnable
	originalAPINum := common.GlobalApiRateLimitNum
	originalAPIDuration := common.GlobalApiRateLimitDuration

	common.RedisEnabled = false
	common.GlobalWebRateLimitEnable = true
	common.GlobalWebRateLimitNum = 1
	common.GlobalWebRateLimitDuration = 60
	common.GlobalApiRateLimitEnable = true
	common.GlobalApiRateLimitNum = 1
	common.GlobalApiRateLimitDuration = 60

	t.Cleanup(func() {
		common.RedisEnabled = originalRedisEnabled
		common.GlobalWebRateLimitEnable = originalWebEnabled
		common.GlobalWebRateLimitNum = originalWebNum
		common.GlobalWebRateLimitDuration = originalWebDuration
		common.GlobalApiRateLimitEnable = originalAPIEnabled
		common.GlobalApiRateLimitNum = originalAPINum
		common.GlobalApiRateLimitDuration = originalAPIDuration
	})
}

func requestRateLimitedRouter(t *testing.T, router *gin.Engine, path string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, path, nil)
	request.RemoteAddr = "203.0.113.10:43210"
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	router.ServeHTTP(recorder, request)
	return recorder
}

func TestGlobalWebRateLimitDoesNotChargeStaticAssets(t *testing.T) {
	configureRateLimitTest(t)
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(GlobalWebRateLimit())
	router.NoRoute(func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	staticPaths := []string{
		"/static/js/async/4303.bff14cc141.js",
		"/assets/app.12345678.css",
		"/logo.png?v=release",
		"/favicon.ico",
		"/fonts/inter.woff2",
	}
	for _, path := range staticPaths {
		require.Equal(t, http.StatusOK, requestRateLimitedRouter(t, router, path, nil).Code, path)
	}

	require.Equal(t, http.StatusOK, requestRateLimitedRouter(t, router, "/console", nil).Code)
	limited := requestRateLimitedRouter(t, router, "/console/usage-monitor", nil)
	require.Equal(t, http.StatusTooManyRequests, limited.Code)
	require.Equal(t, "60", limited.Header().Get("Retry-After"))
}

func TestGlobalAPIRateLimitUsesCredentialBucketWithIPUmbrella(t *testing.T) {
	configureRateLimitTest(t)
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(GlobalAPIRateLimit())
	router.NoRoute(func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	firstCredential := map[string]string{"Authorization": "Bearer credential-a"}
	secondCredential := map[string]string{"Authorization": "Bearer credential-b"}
	require.Equal(t, http.StatusOK, requestRateLimitedRouter(t, router, "/api/status", firstCredential).Code)
	require.Equal(t, http.StatusTooManyRequests, requestRateLimitedRouter(t, router, "/api/status", firstCredential).Code)
	require.Equal(t, http.StatusOK, requestRateLimitedRouter(t, router, "/api/status", secondCredential).Code)

	for _, credential := range []string{"credential-c", "credential-d"} {
		headers := map[string]string{"Authorization": "Bearer " + credential}
		require.Equal(t, http.StatusOK, requestRateLimitedRouter(t, router, "/api/status", headers).Code)
	}
	umbrellaLimited := requestRateLimitedRouter(t, router, "/api/status", map[string]string{
		"Authorization": "Bearer credential-e",
	})
	require.Equal(t, http.StatusTooManyRequests, umbrellaLimited.Code)
}
