package model

import (
	"net/url"
	"os"
	"strings"
	"unicode"

	"github.com/QuantumNous/new-api/setting/operation_setting"
)

var ignoredSiteIDHostLabels = map[string]struct{}{
	"":        {},
	"www":     {},
	"api":     {},
	"ai":      {},
	"app":     {},
	"web":     {},
	"admin":   {},
	"console": {},
	"panel":   {},
}

func sanitizeSiteIDCandidate(raw string) string {
	raw = strings.TrimSpace(strings.ToLower(raw))
	if raw == "" || raw == "default" {
		return ""
	}
	if strings.Contains(raw, "://") || strings.Contains(raw, "/") {
		if inferred := inferSiteIDFromURL(raw); inferred != "" {
			return inferred
		}
	}
	var b strings.Builder
	for _, r := range raw {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' {
			b.WriteRune(unicode.ToLower(r))
		}
	}
	return strings.Trim(b.String(), "-_")
}

func inferSiteIDFromURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	host := strings.TrimSpace(strings.ToLower(parsed.Hostname()))
	if host == "" {
		return ""
	}
	for _, part := range strings.Split(host, ".") {
		part = strings.TrimSpace(strings.ToLower(part))
		if _, skip := ignoredSiteIDHostLabels[part]; skip {
			continue
		}
		if len(part) <= 2 {
			continue
		}
		if normalized := sanitizeSiteIDCandidate(part); normalized != "" {
			return normalized
		}
	}
	return ""
}

func CanonicalSiteID(explicit string) string {
	candidates := []string{
		explicit,
		strings.TrimSpace(os.Getenv("COMMUNITY_SITE_ID")),
		strings.TrimSpace(os.Getenv("SITE_ID")),
		strings.TrimSpace(os.Getenv("AGENT_SITE_ID")),
	}
	if cfg := operation_setting.GetAgentSetting(); cfg != nil {
		candidates = append(candidates,
			strings.TrimSpace(cfg.SiteID),
			strings.TrimSpace(cfg.APIBaseURL),
			strings.TrimSpace(cfg.PublicBaseURL),
		)
	}
	for _, candidate := range candidates {
		if normalized := sanitizeSiteIDCandidate(candidate); normalized != "" {
			return normalized
		}
	}
	return "default"
}

func CommunityIdentitySiteID() string {
	return CanonicalSiteID(strings.TrimSpace(os.Getenv("COMMUNITY_SITE_ID")))
}
