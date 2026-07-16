package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSetUpstreamRateLimitResponseHeaders(t *testing.T) {
	originalCooldown := common.ChannelRoutingRateLimitCooldownSeconds
	common.ChannelRoutingRateLimitCooldownSeconds = 17
	t.Cleanup(func() {
		common.ChannelRoutingRateLimitCooldownSeconds = originalCooldown
	})

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	setUpstreamRateLimitResponseHeaders(ctx, http.StatusTooManyRequests)

	require.Equal(t, "upstream-channel", recorder.Header().Get("X-RateLimit-Scope"))
	require.Equal(t, "17", recorder.Header().Get("Retry-After"))
}

func TestSetUpstreamRateLimitResponseHeadersPreservesProviderRetryAfter(t *testing.T) {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Header("Retry-After", "42")

	setUpstreamRateLimitResponseHeaders(ctx, http.StatusTooManyRequests)

	require.Equal(t, "42", recorder.Header().Get("Retry-After"))
}
