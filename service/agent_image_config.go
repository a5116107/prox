package service

import (
	"strings"

	"github.com/QuantumNous/new-api/setting/operation_setting"
)

type AgentImageRuntimeConfig struct {
	Enabled               bool    `json:"enabled"`
	APIBaseURL            string  `json:"api_base_url"`
	APIKey                string  `json:"api_key,omitempty"`
	APIKeyConfigured      bool    `json:"api_key_configured"`
	Model                 string  `json:"model"`
	Size                  string  `json:"size"`
	TimeoutSeconds        int     `json:"timeout_seconds"`
	RetryLimit            int     `json:"retry_limit"`
	RetryBaseDelaySeconds float64 `json:"retry_base_delay_seconds"`
	RetryMaxDelaySeconds  float64 `json:"retry_max_delay_seconds"`
	CooldownSeconds       int     `json:"cooldown_seconds"`
	RequireBind           bool    `json:"require_bind"`
}

func GetAgentImageRuntimeConfig() AgentImageRuntimeConfig {
	cfg := operation_setting.GetAgentSetting()
	if cfg == nil {
		return AgentImageRuntimeConfig{}
	}
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.ImageAPIBaseURL), "/")
	modelName := strings.TrimSpace(cfg.ImageModel)
	if modelName == "" {
		modelName = "gpt-image-2"
	}
	size := strings.TrimSpace(cfg.ImageSize)
	if size == "" {
		size = "1024x1024"
	}
	timeout := cfg.ImageTimeoutSeconds
	if timeout <= 0 {
		timeout = 180
	}
	retryLimit := cfg.ImageRetryLimit
	if retryLimit < 1 {
		retryLimit = 1
	} else if retryLimit > 5 {
		retryLimit = 5
	}
	baseDelay := cfg.ImageRetryBaseDelaySeconds
	if baseDelay < 0 {
		baseDelay = 0
	}
	maxDelay := cfg.ImageRetryMaxDelaySeconds
	if maxDelay < baseDelay {
		maxDelay = baseDelay
	}
	cooldown := cfg.ImageCooldownSeconds
	if cooldown < 0 {
		cooldown = 0
	}
	apiKey := strings.TrimSpace(cfg.ImageAPIKey)
	return AgentImageRuntimeConfig{
		Enabled:               cfg.ImageGenerationEnabled,
		APIBaseURL:            baseURL,
		APIKey:                apiKey,
		APIKeyConfigured:      apiKey != "",
		Model:                 modelName,
		Size:                  size,
		TimeoutSeconds:        timeout,
		RetryLimit:            retryLimit,
		RetryBaseDelaySeconds: baseDelay,
		RetryMaxDelaySeconds:  maxDelay,
		CooldownSeconds:       cooldown,
		RequireBind:           cfg.ImageRequireBind,
	}
}
