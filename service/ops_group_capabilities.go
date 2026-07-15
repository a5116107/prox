package service

import (
	"sort"
	"strings"
	"time"
)

type OpsGroupCapabilityMatrixView struct {
	SiteID      string                         `json:"site_id"`
	Filters     map[string]string              `json:"filters"`
	Summary     map[string]any                 `json:"summary"`
	Items       []OpsGroupCapabilityMatrixItem `json:"items"`
	GeneratedAt int64                          `json:"generated_at"`
}

type OpsGroupCapabilityMatrixItem struct {
	ID                int               `json:"id"`
	SiteID            string            `json:"site_id"`
	Platform          string            `json:"platform"`
	PlatformFamily    string            `json:"platform_family"`
	GroupID           string            `json:"group_id"`
	GroupName         string            `json:"group_name"`
	Role              string            `json:"role"`
	Status            string            `json:"status"`
	Enabled           bool              `json:"enabled"`
	CapabilityPolicy  map[string]any    `json:"capability_policy"`
	RewardPolicy      map[string]any    `json:"reward_policy"`
	GamePolicy        map[string]any    `json:"game_policy"`
	GameConfigs       []map[string]any  `json:"game_configs"`
	AccessQualifiers  map[string]any    `json:"access_qualifiers"`
	RuntimeConnectors map[string]any    `json:"runtime_connectors"`
	LatestMetrics     map[string]any    `json:"latest_metrics"`
	SourceTables      map[string]string `json:"source_tables"`
}

func GetOpsGroupCapabilityMatrix(siteID string, platform string, role string, status string) (*OpsGroupCapabilityMatrixView, error) {
	resolvedSiteID := resolveOpsSiteID(siteID)
	groups, err := ListOpsGroups(resolvedSiteID, platform, role, status)
	if err != nil {
		return nil, err
	}
	items := make([]OpsGroupCapabilityMatrixItem, 0, len(groups))
	summary := map[string]any{
		"total_groups":              0,
		"enabled_groups":            0,
		"configured_chatops_groups": 0,
		"checkin_enabled_groups":    0,
		"verify_enabled_groups":     0,
		"invite_enabled_groups":     0,
		"primary_groups":            0,
		"community_groups":          0,
		"matched_primary_groups":    0,
		"matched_community_groups":  0,
		"game_rule_groups":          0,
		"enabled_game_rules":        0,
		"disabled_game_rules":       0,
		"platform_breakdown":        map[string]int{},
		"role_breakdown":            map[string]int{},
		"budget_pools":              []string{},
		"enabled_game_codes":        []string{},
		"disabled_game_codes":       []string{},
		"reward_totals":             map[string]int{"checkin_quota": 0, "invite_reward_quota": 0, "invitee_reward_quota": 0, "daily_group_reward_limit": 0},
	}
	budgetPools := map[string]struct{}{}
	enabledGameCodes := map[string]struct{}{}
	disabledGameCodes := map[string]struct{}{}

	for _, group := range groups {
		capabilityPolicy := normalizeOpsGroupCapabilityPolicy(group.Capabilities)
		gamePolicy := buildOpsGroupGamePolicy(group.GameConfigs)
		rewardPolicy := buildOpsGroupRewardPolicy(capabilityPolicy, gamePolicy)
		item := OpsGroupCapabilityMatrixItem{
			ID:                group.ID,
			SiteID:            group.SiteID,
			Platform:          group.Platform,
			PlatformFamily:    group.PlatformFamily,
			GroupID:           group.GroupID,
			GroupName:         group.GroupName,
			Role:              group.Role,
			Status:            group.Status,
			Enabled:           group.Enabled,
			CapabilityPolicy:  capabilityPolicy,
			RewardPolicy:      rewardPolicy,
			GamePolicy:        gamePolicy,
			GameConfigs:       group.GameConfigs,
			AccessQualifiers:  group.AccessQualifiers,
			RuntimeConnectors: group.RuntimeConnectors,
			LatestMetrics:     group.LatestMetrics,
			SourceTables:      group.SourceTables,
		}
		items = append(items, item)
		opsSummaryAddInt(summary, "total_groups", 1)
		if group.Enabled {
			opsSummaryAddInt(summary, "enabled_groups", 1)
		}
		if capabilityPolicy["configured"] == true {
			opsSummaryAddInt(summary, "configured_chatops_groups", 1)
		}
		if opsBoolAny(capabilityPolicy["checkin_enabled"]) {
			opsSummaryAddInt(summary, "checkin_enabled_groups", 1)
		}
		if opsBoolAny(capabilityPolicy["verify_enabled"]) {
			opsSummaryAddInt(summary, "verify_enabled_groups", 1)
		}
		if opsBoolAny(capabilityPolicy["invite_enabled"]) {
			opsSummaryAddInt(summary, "invite_enabled_groups", 1)
		}
		if group.Role == "primary_mainfield" {
			opsSummaryAddInt(summary, "primary_groups", 1)
		}
		if group.PlatformFamily == "community" || group.Role == "community_intake" {
			opsSummaryAddInt(summary, "community_groups", 1)
		}
		if opsBoolAny(group.AccessQualifiers["qualifies_primary_binding"]) {
			opsSummaryAddInt(summary, "matched_primary_groups", 1)
		}
		if opsBoolAny(group.AccessQualifiers["qualifies_community_binding"]) {
			opsSummaryAddInt(summary, "matched_community_groups", 1)
		}
		opsIncrementMapCounter(summary["platform_breakdown"], group.PlatformFamily)
		opsIncrementMapCounter(summary["role_breakdown"], group.Role)
		gameConfigured := opsIntAny(gamePolicy["configured_count"])
		if gameConfigured > 0 {
			opsSummaryAddInt(summary, "game_rule_groups", 1)
		}
		opsSummaryAddInt(summary, "enabled_game_rules", opsIntAny(gamePolicy["enabled_count"]))
		opsSummaryAddInt(summary, "disabled_game_rules", opsIntAny(gamePolicy["disabled_count"]))
		for _, pool := range opsStringSliceAny(gamePolicy["budget_pools"]) {
			budgetPools[pool] = struct{}{}
		}
		for _, code := range opsStringSliceAny(gamePolicy["enabled_game_codes"]) {
			enabledGameCodes[code] = struct{}{}
		}
		for _, code := range opsStringSliceAny(gamePolicy["disabled_game_codes"]) {
			disabledGameCodes[code] = struct{}{}
		}
		if totals, ok := summary["reward_totals"].(map[string]int); ok {
			totals["checkin_quota"] += opsIntAny(rewardPolicy["checkin_quota"])
			totals["invite_reward_quota"] += opsIntAny(rewardPolicy["invite_reward_quota"])
			totals["invitee_reward_quota"] += opsIntAny(rewardPolicy["invitee_reward_quota"])
			totals["daily_group_reward_limit"] += opsIntAny(rewardPolicy["daily_group_reward_limit"])
		}
	}

	summary["budget_pools"] = opsSortedStringKeys(budgetPools)
	summary["enabled_game_codes"] = opsSortedStringKeys(enabledGameCodes)
	summary["disabled_game_codes"] = opsSortedStringKeys(disabledGameCodes)

	return &OpsGroupCapabilityMatrixView{
		SiteID: resolvedSiteID,
		Filters: map[string]string{
			"platform": platform,
			"role":     role,
			"status":   status,
		},
		Summary:     summary,
		Items:       items,
		GeneratedAt: time.Now().Unix(),
	}, nil
}

func normalizeOpsGroupCapabilityPolicy(raw map[string]any) map[string]any {
	configured := false
	if raw != nil {
		if v, ok := raw["configured"].(bool); ok {
			configured = v
		}
		if !configured && len(raw) > 1 {
			configured = true
		}
	}
	return map[string]any{
		"configured":               configured,
		"checkin_enabled":          opsBoolAny(raw["checkin_enabled"]),
		"verify_enabled":           opsBoolAny(raw["verify_enabled"]),
		"invite_enabled":           opsBoolAny(raw["invite_enabled"]),
		"checkin_quota":            opsIntAny(raw["checkin_quota"]),
		"verify_min_quota":         opsIntAny(raw["verify_min_quota"]),
		"invite_reward_quota":      opsIntAny(raw["invite_reward_quota"]),
		"invitee_reward_quota":     opsIntAny(raw["invitee_reward_quota"]),
		"daily_group_reward_limit": opsIntAny(raw["daily_group_reward_limit"]),
		"rule":                     opsMapAny(raw["rule"]),
	}
}

func buildOpsGroupGamePolicy(gameConfigs []map[string]any) map[string]any {
	out := map[string]any{
		"configured_count":    len(gameConfigs),
		"enabled_count":       0,
		"disabled_count":      0,
		"enabled_game_codes":  []string{},
		"disabled_game_codes": []string{},
		"budget_pools":        []string{},
	}
	budgetPools := map[string]struct{}{}
	enabledCodes := make([]string, 0)
	disabledCodes := make([]string, 0)
	for _, row := range gameConfigs {
		code := firstOpsNonEmpty(opsStringAny(row["game_code"]), "unknown")
		pool := firstOpsNonEmpty(opsStringAny(row["budget_pool"]), "game")
		budgetPools[pool] = struct{}{}
		if opsBoolAny(row["enabled"]) {
			out["enabled_count"] = opsIntAny(out["enabled_count"]) + 1
			enabledCodes = append(enabledCodes, code)
		} else {
			out["disabled_count"] = opsIntAny(out["disabled_count"]) + 1
			disabledCodes = append(disabledCodes, code)
		}
	}
	sort.Strings(enabledCodes)
	sort.Strings(disabledCodes)
	out["enabled_game_codes"] = enabledCodes
	out["disabled_game_codes"] = disabledCodes
	out["budget_pools"] = opsSortedStringKeys(budgetPools)
	return out
}

func buildOpsGroupRewardPolicy(capabilityPolicy map[string]any, gamePolicy map[string]any) map[string]any {
	return map[string]any{
		"checkin_quota":            opsIntAny(capabilityPolicy["checkin_quota"]),
		"verify_min_quota":         opsIntAny(capabilityPolicy["verify_min_quota"]),
		"invite_reward_quota":      opsIntAny(capabilityPolicy["invite_reward_quota"]),
		"invitee_reward_quota":     opsIntAny(capabilityPolicy["invitee_reward_quota"]),
		"daily_group_reward_limit": opsIntAny(capabilityPolicy["daily_group_reward_limit"]),
		"budget_pools":             opsStringSliceAny(gamePolicy["budget_pools"]),
	}
}

func opsSummaryAddInt(summary map[string]any, key string, delta int) {
	summary[key] = opsIntAny(summary[key]) + delta
}

func opsIncrementMapCounter(target any, key string) {
	if key == "" {
		key = "unknown"
	}
	if counters, ok := target.(map[string]int); ok {
		counters[key]++
	}
}

func opsBoolAny(v any) bool {
	if b, ok := v.(bool); ok {
		return b
	}
	return false
}

func opsIntAny(v any) int {
	switch value := v.(type) {
	case int:
		return value
	case int64:
		return int(value)
	case int32:
		return int(value)
	case float64:
		return int(value)
	case float32:
		return int(value)
	default:
		return 0
	}
}

func opsStringAny(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func opsStringSliceAny(v any) []string {
	switch items := v.(type) {
	case []string:
		out := make([]string, 0, len(items))
		seen := map[string]struct{}{}
		for _, item := range items {
			item = strings.TrimSpace(item)
			if item == "" {
				continue
			}
			if _, ok := seen[item]; ok {
				continue
			}
			seen[item] = struct{}{}
			out = append(out, item)
		}
		sort.Strings(out)
		return out
	case []any:
		out := make([]string, 0, len(items))
		seen := map[string]struct{}{}
		for _, raw := range items {
			item := strings.TrimSpace(opsStringAny(raw))
			if item == "" {
				continue
			}
			if _, ok := seen[item]; ok {
				continue
			}
			seen[item] = struct{}{}
			out = append(out, item)
		}
		sort.Strings(out)
		return out
	default:
		return []string{}
	}
}

func opsSortedStringKeys(items map[string]struct{}) []string {
	out := make([]string, 0, len(items))
	for item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		out = append(out, item)
	}
	sort.Strings(out)
	return out
}
