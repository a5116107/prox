package controller

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestShouldRetryUpstreamDiskStorageFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		statusCode int
		message    string
		want       bool
	}{
		{
			name:       "disk free-space floor reached",
			statusCode: http.StatusBadRequest,
			message:    "Invalid request: disk storage creation failed: failed to write to temp file: disk free-space floor reached: free=5699096576 reserved=0 requested=1048576 minimum=10737418240",
			want:       true,
		},
		{
			name:       "case insensitive provider message",
			statusCode: http.StatusBadRequest,
			message:    "DISK STORAGE CREATION FAILED: DISK FREE-SPACE FLOOR REACHED",
			want:       true,
		},
		{
			name:       "ordinary invalid request",
			statusCode: http.StatusBadRequest,
			message:    "Invalid request: input is required",
			want:       false,
		},
		{
			name:       "free-space phrase alone",
			statusCode: http.StatusBadRequest,
			message:    "disk free-space floor reached",
			want:       false,
		},
		{
			name:       "storage creation phrase alone",
			statusCode: http.StatusBadRequest,
			message:    "disk storage creation failed",
			want:       false,
		},
		{
			name:       "rate limited",
			statusCode: http.StatusTooManyRequests,
			message:    "rate limited",
			want:       true,
		},
		{
			name:       "server error",
			statusCode: http.StatusInternalServerError,
			message:    "upstream unavailable",
			want:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			err := types.WithOpenAIError(types.OpenAIError{
				Message: tt.message,
				Type:    "new_api_error",
				Code:    "upstream_error",
			}, tt.statusCode)

			require.Equal(t, tt.want, shouldRetry(ctx, err, 1))
		})
	}
}

func TestShouldRetryParsedUpstreamDiskStorageFailure(t *testing.T) {
	body := `{"error":{"message":"Invalid request: Invalid request: disk storage creation failed: failed to write to temp file: disk free-space floor reached: free=5699096576 reserved=0 requested=1048576 minimum=10737418240","type":"new_api_error","param":"","code":"***"}}`
	resp := &http.Response{
		StatusCode: http.StatusBadRequest,
		Body:       io.NopCloser(strings.NewReader(body)),
	}
	upstreamError := service.RelayErrorHandler(context.Background(), resp, false)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())

	require.Equal(t, http.StatusBadRequest, upstreamError.StatusCode)
	require.True(t, shouldRetry(ctx, upstreamError, 1))
}

func TestShouldRetryUpstreamDiskStorageFailureOverridesAffinityOnly(t *testing.T) {
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Set("channel_affinity_skip_retry_on_failure", true)

	diskError := types.WithOpenAIError(types.OpenAIError{
		Message: "disk storage creation failed: disk free-space floor reached",
		Type:    "new_api_error",
		Code:    "upstream_error",
	}, http.StatusBadRequest)
	require.True(t, shouldRetry(ctx, diskError, 1))

	ordinaryError := types.WithOpenAIError(types.OpenAIError{
		Message: "upstream unavailable",
		Type:    "server_error",
		Code:    "server_error",
	}, http.StatusInternalServerError)
	require.False(t, shouldRetry(ctx, ordinaryError, 1))
}

func TestShouldRetryUpstreamDiskStorageFailureHonorsRetryGuards(t *testing.T) {
	newDiskError := func(options ...types.NewAPIErrorOptions) *types.NewAPIError {
		return types.WithOpenAIError(types.OpenAIError{
			Message: "disk storage creation failed: disk free-space floor reached",
			Type:    "new_api_error",
			Code:    "upstream_error",
		}, http.StatusBadRequest, options...)
	}

	t.Run("retry budget exhausted", func(t *testing.T) {
		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		require.False(t, shouldRetry(ctx, newDiskError(), 0))
	})

	t.Run("specific channel", func(t *testing.T) {
		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		ctx.Set("specific_channel_id", 42)
		require.False(t, shouldRetry(ctx, newDiskError(), 1))
	})

	t.Run("skip retry option", func(t *testing.T) {
		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		require.False(t, shouldRetry(ctx, newDiskError(types.ErrOptionWithSkipRetry()), 1))
	})
}
