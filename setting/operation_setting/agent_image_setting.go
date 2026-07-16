package operation_setting

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

var agentImageSizePattern = regexp.MustCompile(`^(\d{2,5})x(\d{2,5})$`)

// ValidateAgentImageOption validates values accepted by the generic option API.
func ValidateAgentImageOption(key, value string) error {
	value = strings.TrimSpace(value)
	switch key {
	case "agent_setting.image_generation_enabled", "agent_setting.image_require_bind":
		if _, err := strconv.ParseBool(value); err != nil {
			return fmt.Errorf("image setting must be true or false")
		}
	case "agent_setting.image_api_base_url":
		if value == "" {
			return nil
		}
		parsed, err := url.ParseRequestURI(value)
		if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.User != nil {
			return fmt.Errorf("image API base URL must be an HTTP(S) URL without embedded credentials")
		}
	case "agent_setting.image_model":
		if len(value) > 128 || strings.ContainsAny(value, "\r\n\t") {
			return fmt.Errorf("image model must be at most 128 characters")
		}
	case "agent_setting.image_size":
		if value == "" || strings.EqualFold(value, "auto") {
			return nil
		}
		parts := agentImageSizePattern.FindStringSubmatch(value)
		if len(parts) != 3 {
			return fmt.Errorf("image size must be auto or WIDTHxHEIGHT")
		}
		width, _ := strconv.Atoi(parts[1])
		height, _ := strconv.Atoi(parts[2])
		if width < 256 || width > 4096 || height < 256 || height > 4096 {
			return fmt.Errorf("image dimensions must be between 256 and 4096 pixels")
		}
	case "agent_setting.image_timeout_seconds":
		return validateAgentImageInt(value, 1, 600, "image timeout")
	case "agent_setting.image_retry_limit":
		return validateAgentImageInt(value, 1, 5, "image retry limit")
	case "agent_setting.image_cooldown_seconds":
		return validateAgentImageInt(value, 0, 86400, "image cooldown")
	case "agent_setting.image_retry_base_delay_seconds", "agent_setting.image_retry_max_delay_seconds":
		parsed, err := strconv.ParseFloat(value, 64)
		if err != nil || parsed < 0 || parsed > 300 {
			return fmt.Errorf("image retry delay must be between 0 and 300 seconds")
		}
	}
	return nil
}

func validateAgentImageInt(value string, minValue, maxValue int, label string) error {
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < minValue || parsed > maxValue {
		return fmt.Errorf("%s must be between %d and %d", label, minValue, maxValue)
	}
	return nil
}
