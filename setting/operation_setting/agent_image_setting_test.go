package operation_setting

import "testing"

func TestValidateAgentImageOption(t *testing.T) {
	t.Parallel()
	valid := map[string]string{
		"agent_setting.image_generation_enabled":       "true",
		"agent_setting.image_api_base_url":             "http://new-api:3000/v1",
		"agent_setting.image_model":                    "gpt-image-2",
		"agent_setting.image_size":                     "1024x1536",
		"agent_setting.image_timeout_seconds":          "180",
		"agent_setting.image_retry_limit":              "3",
		"agent_setting.image_retry_base_delay_seconds": "1.5",
		"agent_setting.image_retry_max_delay_seconds":  "15",
		"agent_setting.image_cooldown_seconds":         "45",
		"agent_setting.image_require_bind":             "false",
	}
	for key, value := range valid {
		if err := ValidateAgentImageOption(key, value); err != nil {
			t.Fatalf("expected %s=%q to be valid: %v", key, value, err)
		}
	}

	invalid := map[string]string{
		"agent_setting.image_api_base_url":            "file:///tmp/image",
		"agent_setting.image_size":                    "8000x8000",
		"agent_setting.image_timeout_seconds":         "0",
		"agent_setting.image_retry_limit":             "8",
		"agent_setting.image_retry_max_delay_seconds": "-1",
		"agent_setting.image_cooldown_seconds":        "86401",
	}
	for key, value := range invalid {
		if err := ValidateAgentImageOption(key, value); err == nil {
			t.Fatalf("expected %s=%q to be rejected", key, value)
		}
	}
}
