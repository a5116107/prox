package service

import (
	"testing"

	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/require"
)

func TestGetAgentImageRuntimeConfigNormalizesValues(t *testing.T) {
	cfg := operation_setting.GetAgentSetting()
	original := *cfg
	t.Cleanup(func() { *cfg = original })

	cfg.ImageGenerationEnabled = true
	cfg.ImageAPIBaseURL = "https://images.example.test/v1/"
	cfg.ImageAPIKey = " image-secret "
	cfg.ImageModel = ""
	cfg.ImageSize = ""
	cfg.ImageTimeoutSeconds = 0
	cfg.ImageRetryLimit = 9
	cfg.ImageRetryBaseDelaySeconds = 5
	cfg.ImageRetryMaxDelaySeconds = 2
	cfg.ImageCooldownSeconds = -1
	cfg.ImageRequireBind = true

	got := GetAgentImageRuntimeConfig()
	require.True(t, got.Enabled)
	require.Equal(t, "https://images.example.test/v1", got.APIBaseURL)
	require.Equal(t, "image-secret", got.APIKey)
	require.True(t, got.APIKeyConfigured)
	require.Equal(t, "gpt-image-2", got.Model)
	require.Equal(t, "1024x1024", got.Size)
	require.Equal(t, 180, got.TimeoutSeconds)
	require.Equal(t, 5, got.RetryLimit)
	require.Equal(t, float64(5), got.RetryMaxDelaySeconds)
	require.Zero(t, got.CooldownSeconds)
	require.True(t, got.RequireBind)
}

func TestAgentSettingPublicMapNeverIncludesImageAPIKey(t *testing.T) {
	cfg := operation_setting.GetAgentSetting()
	original := *cfg
	t.Cleanup(func() { *cfg = original })
	cfg.ImageAPIKey = "must-not-leak"

	public := agentSettingPublicMap()
	require.NotContains(t, public, "image_api_key")
	require.Equal(t, true, public["image_api_key_configured"])
}
