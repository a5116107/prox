package service

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

type effectiveGroupGamePolicy struct {
	Found      bool
	Enabled    bool
	BudgetPool string
	Rules      map[string]any
}

func resolveEffectiveGroupGamePolicyTx(db *gorm.DB, siteID, source, roomID, gameCode string) (effectiveGroupGamePolicy, error) {
	policy := effectiveGroupGamePolicy{Enabled: true, Rules: map[string]any{}}
	if db == nil {
		return policy, nil
	}
	siteID = model.CanonicalSiteID(siteID)
	roomID = strings.TrimSpace(roomID)
	gameCode = strings.ToLower(strings.TrimSpace(gameCode))
	platforms := chatOpsPlatformCandidates(source, roomID)
	platformRank := map[string]int{"*": 0}
	for index := len(platforms) - 1; index >= 0; index-- {
		platformRank[platforms[index]] = len(platforms) - index
	}
	queryPlatforms := append([]string{"*"}, platforms...)
	queryGroups := []string{"", "*"}
	if roomID != "" {
		queryGroups = append(queryGroups, roomID)
	}

	var rows []model.GroupGameConfig
	if err := db.Where(
		"site_id = ? AND game_code = ? AND platform IN ? AND group_id IN ?",
		siteID, gameCode, queryPlatforms, queryGroups,
	).Find(&rows).Error; err != nil {
		return policy, err
	}
	sort.SliceStable(rows, func(i, j int) bool {
		leftPlatform := platformRank[strings.ToLower(strings.TrimSpace(rows[i].Platform))]
		rightPlatform := platformRank[strings.ToLower(strings.TrimSpace(rows[j].Platform))]
		if leftPlatform != rightPlatform {
			return leftPlatform < rightPlatform
		}
		leftGroup := gamePolicyGroupRank(rows[i].GroupId, roomID)
		rightGroup := gamePolicyGroupRank(rows[j].GroupId, roomID)
		if leftGroup != rightGroup {
			return leftGroup < rightGroup
		}
		return rows[i].Id < rows[j].Id
	})
	for _, row := range rows {
		policy.Found = true
		policy.Enabled = row.Enabled
		if pool := strings.ToLower(strings.TrimSpace(row.BudgetPool)); pool != "" {
			if err := model.ValidateOpsPoolType(pool); err != nil {
				return policy, fmt.Errorf("invalid %s budget pool for %s/%s: %w", gameCode, row.Platform, row.GroupId, err)
			}
			policy.BudgetPool = pool
		}
		rawRules := strings.TrimSpace(row.RuleJson)
		if rawRules == "" {
			continue
		}
		var rules map[string]any
		if err := json.Unmarshal([]byte(rawRules), &rules); err != nil || rules == nil {
			return policy, fmt.Errorf("invalid %s rules for %s/%s", gameCode, row.Platform, row.GroupId)
		}
		policy.Rules = mergeGamePolicyMaps(policy.Rules, rules)
	}
	return policy, nil
}

func gamePolicyGroupRank(groupID, roomID string) int {
	switch strings.TrimSpace(groupID) {
	case "":
		return 0
	case "*":
		return 1
	case strings.TrimSpace(roomID):
		return 2
	default:
		return -1
	}
}

func mergeGamePolicyMaps(base, override map[string]any) map[string]any {
	out := make(map[string]any, len(base)+len(override))
	for key, value := range base {
		out[key] = value
	}
	for key, value := range override {
		if nestedOverride, ok := value.(map[string]any); ok {
			if nestedBase, ok := out[key].(map[string]any); ok {
				out[key] = mergeGamePolicyMaps(nestedBase, nestedOverride)
				continue
			}
		}
		out[key] = value
	}
	return out
}

func gamePolicyString(rules map[string]any, key string) string {
	if rules == nil {
		return ""
	}
	value, ok := rules[key]
	if !ok {
		return ""
	}
	text, _ := value.(string)
	return strings.TrimSpace(text)
}

func gamePolicyInt(rules map[string]any, key string) (int, bool) {
	if rules == nil {
		return 0, false
	}
	value, ok := rules[key]
	if !ok || value == nil {
		return 0, false
	}
	switch typed := value.(type) {
	case int:
		return typed, true
	case int64:
		return int(typed), true
	case float64:
		return int(typed), true
	case json.Number:
		parsed, err := strconv.Atoi(typed.String())
		return parsed, err == nil
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(typed))
		return parsed, err == nil
	default:
		parsed, err := strconv.Atoi(strings.TrimSpace(fmt.Sprint(typed)))
		return parsed, err == nil
	}
}

func effectivePolicyBudgetPool(policy effectiveGroupGamePolicy, rules map[string]any, fallback string) (string, error) {
	pool := strings.ToLower(strings.TrimSpace(policy.BudgetPool))
	if configured := strings.ToLower(gamePolicyString(rules, "budget_pool")); configured != "" {
		pool = configured
	}
	if pool == "" {
		pool = strings.ToLower(strings.TrimSpace(fallback))
	}
	if err := model.ValidateOpsPoolType(pool); err != nil {
		return "", err
	}
	return pool, nil
}
