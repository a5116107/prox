package service

import (
	"context"
	"encoding/json"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

type OpsAccessPolicySaveRequest struct {
	Enabled                    *bool    `json:"enabled"`
	PrimaryPlatform            *string  `json:"primary_platform"`
	PrimaryGroupIDs            []string `json:"primary_group_ids"`
	CommunityGroupIDs          []string `json:"community_group_ids"`
	CommunityOnlyGroups        []string `json:"community_only_groups"`
	FullAccessGroups           []string `json:"full_access_groups"`
	PaidBypassGroups           []string `json:"paid_bypass_groups"`
	PaidUserGroups             []string `json:"paid_user_groups"`
	AllowPaidBypass            *bool    `json:"allow_paid_bypass"`
	AllowAdminBypass           *bool    `json:"allow_admin_bypass"`
	CheckOnLogin               *bool    `json:"check_on_login"`
	BlockTokenCreate           *bool    `json:"block_token_create"`
	BlockTokenEnable           *bool    `json:"block_token_enable"`
	EnforceRequestTime         *bool    `json:"enforce_request_time"`
	FreezeLegacyTokens         *bool    `json:"freeze_legacy_tokens"`
	AutoRestoreCompliantTokens *bool    `json:"auto_restore_compliant_tokens"`
	StateCacheTTLSeconds       *int     `json:"state_cache_ttl_seconds"`
	CommunityJoinURL           *string  `json:"community_join_url"`
	PrimaryJoinURL             *string  `json:"primary_join_url"`
	DenyMessage                *string  `json:"deny_message"`
	UpgradeMessage             *string  `json:"upgrade_message"`
	RewardSoftFloorQuota       *int     `json:"reward_soft_floor_quota"`
	RewardHardFloorQuota       *int     `json:"reward_hard_floor_quota"`
	DailySiteRewardCap         *int     `json:"daily_site_reward_cap"`
	DailyUserRewardCap         *int     `json:"daily_user_reward_cap"`
}

type OpsAccessPolicyExplainRequest struct {
	UserID         int    `json:"user_id"`
	RequestedGroup string `json:"requested_group"`
	Refresh        *bool  `json:"refresh"`
}

func ExplainOpsAccessPolicyUser(ctx context.Context, siteID string, req OpsAccessPolicyExplainRequest) (map[string]any, error) {
	resolvedSiteID := resolveOpsSiteID(siteID)
	refresh := true
	if req.Refresh != nil {
		refresh = *req.Refresh
	}
	user, err := model.GetUserById(req.UserID, false)
	if err != nil {
		return nil, err
	}
	status, err := GetUserAccessControlStatus(ctx, req.UserID, refresh)
	if err != nil {
		return nil, err
	}
	usable, state, err := GetUserUsableGroupsWithAccess(ctx, req.UserID, user.Group, refresh)
	if err != nil {
		return nil, err
	}
	baseGroups := GetUserUsableGroups(user.Group)
	requestedGroup := strings.TrimSpace(req.RequestedGroup)
	allowedGroups := accessControlOrderedGroups(usable, nil)
	baseGroupNames := accessControlOrderedGroups(baseGroups, nil)
	allowed := len(allowedGroups) > 0
	if requestedGroup != "" {
		_, allowed = usable[requestedGroup]
	}
	cfg := operation_setting.GetAccessControlSetting()
	policy, policyErr := buildOpsAccessPolicyView(resolvedSiteID, cfg, nil)
	if policyErr != nil {
		return nil, policyErr
	}
	decision := "allowed"
	if !allowed {
		decision = "denied"
	}
	reasonCode := ""
	reasonMessage := ""
	if state != nil {
		reasonCode = state.ReasonCode
		reasonMessage = state.ReasonMessage
	}
	if strings.TrimSpace(reasonMessage) == "" {
		reasonMessage = accessControlReasonMessage(state, cfg)
	}
	return map[string]any{
		"site_id":      resolvedSiteID,
		"generated_at": time.Now().Unix(),
		"user": map[string]any{
			"id":           user.Id,
			"username":     user.Username,
			"display_name": user.DisplayName,
			"base_group":   user.Group,
			"role":         user.Role,
			"status":       user.Status,
		},
		"requested_group":        requestedGroup,
		"decision":               decision,
		"allowed":                allowed,
		"reason_code":            reasonCode,
		"reason_message":         reasonMessage,
		"human_message":          opsAccessPolicyExplainHumanMessage(requestedGroup, allowed, allowedGroups, state, cfg),
		"next_steps":             accessControlNextSteps(state),
		"base_groups":            baseGroupNames,
		"effective_groups":       allowedGroups,
		"requested_group_source": opsAccessPolicyGroupSource(requestedGroup, cfg, baseGroups),
		"status":                 status,
		"policy":                 policy,
	}, nil
}

func opsAccessPolicyExplainHumanMessage(requestedGroup string, allowed bool, allowedGroups []string, state *model.UserSiteAccessState, cfg *operation_setting.AccessControlSetting) string {
	allowedSummary := accessControlAllowedGroupsSummary(allowedGroups)
	if allowed {
		if requestedGroup != "" {
			return "允许使用分组 " + requestedGroup + "：当前绑定/套餐/手工授权满足访问策略；当前可用分组：" + allowedSummary + "。"
		}
		return "允许继续操作：当前可用分组为 " + allowedSummary + "。"
	}
	base := accessControlReasonMessage(state, cfg)
	if requestedGroup != "" {
		return "拒绝使用分组 " + requestedGroup + "：该分组不在当前账号可用范围内；当前可用分组：" + allowedSummary + "。" + base
	}
	return "拒绝继续操作：当前账号没有可用 API 分组。" + base
}

func opsAccessPolicyGroupSource(group string, cfg *operation_setting.AccessControlSetting, base map[string]string) map[string]any {
	group = strings.TrimSpace(group)
	if group == "" {
		return map[string]any{"provided": false}
	}
	return map[string]any{
		"provided":                 true,
		"in_user_base_group":       base != nil && base[group] != "",
		"in_community_only_groups": opsStringInList(group, cfg.CommunityOnlyGroups),
		"in_full_access_groups":    opsStringInList(group, cfg.FullAccessGroups),
		"in_paid_bypass_groups":    opsStringInList(group, cfg.PaidBypassGroups),
	}
}

type OpsAccessPolicyDryRunRequest struct {
	Policy *OpsAccessPolicySaveRequest `json:"policy"`
	UserID int                         `json:"user_id"`
}

type OpsAccessPolicyGroupRef struct {
	ID       int    `json:"id"`
	Platform string `json:"platform"`
	Family   string `json:"family"`
	GroupID  string `json:"group_id"`
	Name     string `json:"name"`
	Role     string `json:"role"`
	Enabled  bool   `json:"enabled"`
}

type OpsAccessPolicyView struct {
	SiteID           string                               `json:"site_id"`
	GeneratedAt      int64                                `json:"generated_at"`
	Configured       map[string]interface{}               `json:"configured"`
	Effective        map[string]interface{}               `json:"effective"`
	Runtime          map[string]interface{}               `json:"runtime"`
	SourceMap        map[string]string                    `json:"source_map"`
	RegisteredGroups map[string][]OpsAccessPolicyGroupRef `json:"registered_groups"`
	Coverage         map[string]interface{}               `json:"coverage"`
	Warnings         []string                             `json:"warnings"`
	HumanSummary     []string                             `json:"human_summary"`
}

var opsAccessPolicyOptionKeys = []string{
	"access_control_setting.enabled",
	"access_control_setting.primary_platform",
	"access_control_setting.primary_group_ids",
	"access_control_setting.community_group_ids",
	"access_control_setting.community_only_groups",
	"access_control_setting.full_access_groups",
	"access_control_setting.paid_bypass_groups",
	"access_control_setting.paid_user_groups",
	"access_control_setting.allow_paid_bypass",
	"access_control_setting.allow_admin_bypass",
	"access_control_setting.check_on_login",
	"access_control_setting.block_token_create",
	"access_control_setting.block_token_enable",
	"access_control_setting.enforce_request_time",
	"access_control_setting.freeze_legacy_tokens",
	"access_control_setting.auto_restore_compliant_tokens",
	"access_control_setting.state_cache_ttl_seconds",
	"access_control_setting.community_join_url",
	"access_control_setting.primary_join_url",
	"access_control_setting.deny_message",
	"access_control_setting.upgrade_message",
	"access_control_setting.reward_soft_floor_quota",
	"access_control_setting.reward_hard_floor_quota",
	"access_control_setting.daily_site_reward_cap",
	"access_control_setting.daily_user_reward_cap",
}

func GetOpsAccessPolicy(siteID string) (*OpsAccessPolicyView, error) {
	return buildOpsAccessPolicyView(resolveOpsSiteID(siteID), operation_setting.GetAccessControlSetting(), nil)
}

func SaveOpsAccessPolicy(siteID string, req OpsAccessPolicySaveRequest) (*OpsAccessPolicyView, error) {
	cfg := operation_setting.GetAccessControlSetting()
	next := *cfg
	applyOpsAccessPolicySaveRequest(&next, req)
	values := opsAccessPolicyOptionValues(&next)
	if err := model.UpdateOptionsBulk(values); err != nil {
		return nil, err
	}
	return buildOpsAccessPolicyView(resolveOpsSiteID(siteID), operation_setting.GetAccessControlSetting(), nil)
}

func DryRunOpsAccessPolicy(ctx context.Context, siteID string, req OpsAccessPolicyDryRunRequest) (map[string]any, error) {
	cfg := operation_setting.GetAccessControlSetting()
	draft := *cfg
	if req.Policy != nil {
		applyOpsAccessPolicySaveRequest(&draft, *req.Policy)
	}
	policy, err := buildOpsAccessPolicyView(resolveOpsSiteID(siteID), &draft, nil)
	if err != nil {
		return nil, err
	}
	out := map[string]any{
		"site_id":      policy.SiteID,
		"generated_at": time.Now().Unix(),
		"policy":       policy,
		"would_write":  opsAccessPolicyOptionValues(&draft),
		"warnings":     policy.Warnings,
	}
	if req.UserID > 0 {
		status, statusErr := GetUserAccessControlStatus(ctx, req.UserID, true)
		if statusErr != nil {
			out["user_error"] = statusErr.Error()
		} else {
			out["current_user_status"] = status
			out["note"] = "user_status uses current saved runtime policy; draft policy is validated without mutating runtime"
		}
	}
	return out, nil
}

func applyOpsAccessPolicySaveRequest(cfg *operation_setting.AccessControlSetting, req OpsAccessPolicySaveRequest) {
	if cfg == nil {
		return
	}
	if req.Enabled != nil {
		cfg.Enabled = *req.Enabled
	}
	if req.PrimaryPlatform != nil {
		cfg.PrimaryPlatform = normalizeAccessPlatform(*req.PrimaryPlatform)
	}
	if req.PrimaryGroupIDs != nil {
		cfg.PrimaryGroupIDs = normalizeOpsAccessPolicyStringList(req.PrimaryGroupIDs)
	}
	if req.CommunityGroupIDs != nil {
		cfg.CommunityGroupIDs = normalizeOpsAccessPolicyStringList(req.CommunityGroupIDs)
	}
	if req.CommunityOnlyGroups != nil {
		cfg.CommunityOnlyGroups = normalizeOpsAccessPolicyStringList(req.CommunityOnlyGroups)
	}
	if req.FullAccessGroups != nil {
		cfg.FullAccessGroups = normalizeOpsAccessPolicyStringList(req.FullAccessGroups)
	}
	if req.PaidBypassGroups != nil {
		cfg.PaidBypassGroups = normalizeOpsAccessPolicyStringList(req.PaidBypassGroups)
	}
	if req.PaidUserGroups != nil {
		cfg.PaidUserGroups = normalizeOpsAccessPolicyStringList(req.PaidUserGroups)
	}
	if req.AllowPaidBypass != nil {
		cfg.AllowPaidBypass = *req.AllowPaidBypass
	}
	if req.AllowAdminBypass != nil {
		cfg.AllowAdminBypass = *req.AllowAdminBypass
	}
	if req.CheckOnLogin != nil {
		cfg.CheckOnLogin = *req.CheckOnLogin
	}
	if req.BlockTokenCreate != nil {
		cfg.BlockTokenCreate = *req.BlockTokenCreate
	}
	if req.BlockTokenEnable != nil {
		cfg.BlockTokenEnable = *req.BlockTokenEnable
	}
	if req.EnforceRequestTime != nil {
		cfg.EnforceRequestTime = *req.EnforceRequestTime
	}
	if req.FreezeLegacyTokens != nil {
		cfg.FreezeLegacyTokens = *req.FreezeLegacyTokens
	}
	if req.AutoRestoreCompliantTokens != nil {
		cfg.AutoRestoreCompliantTokens = *req.AutoRestoreCompliantTokens
	}
	if req.StateCacheTTLSeconds != nil {
		cfg.StateCacheTTLSeconds = maxOpsNonNegative(*req.StateCacheTTLSeconds)
	}
	if req.CommunityJoinURL != nil {
		cfg.CommunityJoinURL = strings.TrimSpace(*req.CommunityJoinURL)
	}
	if req.PrimaryJoinURL != nil {
		cfg.PrimaryJoinURL = strings.TrimSpace(*req.PrimaryJoinURL)
	}
	if req.DenyMessage != nil {
		cfg.DenyMessage = strings.TrimSpace(*req.DenyMessage)
	}
	if req.UpgradeMessage != nil {
		cfg.UpgradeMessage = strings.TrimSpace(*req.UpgradeMessage)
	}
	if req.RewardSoftFloorQuota != nil {
		cfg.RewardSoftFloorQuota = maxOpsNonNegative(*req.RewardSoftFloorQuota)
	}
	if req.RewardHardFloorQuota != nil {
		cfg.RewardHardFloorQuota = maxOpsNonNegative(*req.RewardHardFloorQuota)
	}
	if req.DailySiteRewardCap != nil {
		cfg.DailySiteRewardCap = maxOpsNonNegative(*req.DailySiteRewardCap)
	}
	if req.DailyUserRewardCap != nil {
		cfg.DailyUserRewardCap = maxOpsNonNegative(*req.DailyUserRewardCap)
	}
	if strings.TrimSpace(cfg.PrimaryPlatform) == "" {
		cfg.PrimaryPlatform = "qq"
	}
}

func buildOpsAccessPolicyView(siteID string, cfg *operation_setting.AccessControlSetting, rawOverride map[string]string) (*OpsAccessPolicyView, error) {
	if cfg == nil {
		cfg = operation_setting.GetAccessControlSetting()
	}
	raw := rawOverride
	if raw == nil {
		var err error
		raw, err = loadOpsOptionValues(opsAccessPolicyOptionKeys)
		if err != nil {
			raw = map[string]string{}
		}
	}
	groups, err := ListOpsGroups(siteID, "", "", "")
	if err != nil {
		return nil, err
	}
	registered := opsAccessPolicyRegisteredGroups(groups, cfg)
	coverage, warnings := opsAccessPolicyCoverage(groups, cfg)
	view := &OpsAccessPolicyView{
		SiteID:           siteID,
		GeneratedAt:      time.Now().Unix(),
		Configured:       buildOpsConfiguredDomain(raw, "access_control_setting."),
		Effective:        opsAccessPolicyEffectiveMap(cfg),
		Runtime:          opsAccessPolicyRuntimeMap(cfg),
		SourceMap:        opsAccessPolicySourceMap(),
		RegisteredGroups: registered,
		Coverage:         coverage,
		Warnings:         warnings,
		HumanSummary:     opsAccessPolicyHumanSummary(cfg, coverage, warnings),
	}
	return view, nil
}

func opsAccessPolicyEffectiveMap(cfg *operation_setting.AccessControlSetting) map[string]interface{} {
	primaryGroupIDs, communityGroupIDs := opsAccessPolicyResolvedBindingGroups(AgentSiteID(), cfg)
	return map[string]interface{}{
		"enabled":                       cfg.Enabled,
		"primary_platform":              cfg.PrimaryPlatform,
		"primary_group_ids":             primaryGroupIDs,
		"community_group_ids":           communityGroupIDs,
		"community_only_groups":         cfg.CommunityOnlyGroups,
		"full_access_groups":            cfg.FullAccessGroups,
		"paid_bypass_groups":            cfg.PaidBypassGroups,
		"paid_user_groups":              cfg.PaidUserGroups,
		"allow_paid_bypass":             cfg.AllowPaidBypass,
		"allow_admin_bypass":            cfg.AllowAdminBypass,
		"check_on_login":                cfg.CheckOnLogin,
		"block_token_create":            cfg.BlockTokenCreate,
		"block_token_enable":            cfg.BlockTokenEnable,
		"enforce_request_time":          cfg.EnforceRequestTime,
		"freeze_legacy_tokens":          cfg.FreezeLegacyTokens,
		"auto_restore_compliant_tokens": cfg.AutoRestoreCompliantTokens,
		"state_cache_ttl_seconds":       cfg.StateCacheTTLSeconds,
		"community_join_url":            strings.TrimSpace(cfg.CommunityJoinURL),
		"primary_join_url":              strings.TrimSpace(cfg.PrimaryJoinURL),
		"deny_message":                  strings.TrimSpace(cfg.DenyMessage),
		"upgrade_message":               strings.TrimSpace(cfg.UpgradeMessage),
		"reward_soft_floor_quota":       cfg.RewardSoftFloorQuota,
		"reward_hard_floor_quota":       cfg.RewardHardFloorQuota,
		"daily_site_reward_cap":         cfg.DailySiteRewardCap,
		"daily_user_reward_cap":         cfg.DailyUserRewardCap,
	}
}

func opsAccessPolicyRuntimeMap(cfg *operation_setting.AccessControlSetting) map[string]interface{} {
	primaryGroupIDs, communityGroupIDs := opsAccessPolicyResolvedBindingGroups(AgentSiteID(), cfg)
	return map[string]interface{}{
		"token_mutation_enforced":      cfg.Enabled && (cfg.BlockTokenCreate || cfg.BlockTokenEnable),
		"request_time_enforced":        cfg.Enabled && cfg.EnforceRequestTime,
		"paid_bypass_enabled":          cfg.Enabled && cfg.AllowPaidBypass,
		"admin_bypass_enabled":         cfg.Enabled && cfg.AllowAdminBypass,
		"primary_unlocks_all_groups":   len(cfg.FullAccessGroups) == 0,
		"community_requires_allowlist": len(cfg.CommunityOnlyGroups) > 0,
		"legacy_token_freeze_enabled":  cfg.Enabled && cfg.FreezeLegacyTokens,
		"compliant_token_auto_restore": cfg.Enabled && cfg.AutoRestoreCompliantTokens,
		"primary_groups_source":        opsAccessPolicyBindingGroupSource(cfg.PrimaryGroupIDs, primaryGroupIDs),
		"community_groups_source":      opsAccessPolicyBindingGroupSource(cfg.CommunityGroupIDs, communityGroupIDs),
	}
}

func opsAccessPolicyOptionValues(cfg *operation_setting.AccessControlSetting) map[string]string {
	return map[string]string{
		"access_control_setting.enabled":                       opsAccessPolicyStringValue(cfg.Enabled),
		"access_control_setting.primary_platform":              opsAccessPolicyStringValue(cfg.PrimaryPlatform),
		"access_control_setting.primary_group_ids":             opsAccessPolicyStringValue(cfg.PrimaryGroupIDs),
		"access_control_setting.community_group_ids":           opsAccessPolicyStringValue(cfg.CommunityGroupIDs),
		"access_control_setting.community_only_groups":         opsAccessPolicyStringValue(cfg.CommunityOnlyGroups),
		"access_control_setting.full_access_groups":            opsAccessPolicyStringValue(cfg.FullAccessGroups),
		"access_control_setting.paid_bypass_groups":            opsAccessPolicyStringValue(cfg.PaidBypassGroups),
		"access_control_setting.paid_user_groups":              opsAccessPolicyStringValue(cfg.PaidUserGroups),
		"access_control_setting.allow_paid_bypass":             opsAccessPolicyStringValue(cfg.AllowPaidBypass),
		"access_control_setting.allow_admin_bypass":            opsAccessPolicyStringValue(cfg.AllowAdminBypass),
		"access_control_setting.check_on_login":                opsAccessPolicyStringValue(cfg.CheckOnLogin),
		"access_control_setting.block_token_create":            opsAccessPolicyStringValue(cfg.BlockTokenCreate),
		"access_control_setting.block_token_enable":            opsAccessPolicyStringValue(cfg.BlockTokenEnable),
		"access_control_setting.enforce_request_time":          opsAccessPolicyStringValue(cfg.EnforceRequestTime),
		"access_control_setting.freeze_legacy_tokens":          opsAccessPolicyStringValue(cfg.FreezeLegacyTokens),
		"access_control_setting.auto_restore_compliant_tokens": opsAccessPolicyStringValue(cfg.AutoRestoreCompliantTokens),
		"access_control_setting.state_cache_ttl_seconds":       opsAccessPolicyStringValue(cfg.StateCacheTTLSeconds),
		"access_control_setting.community_join_url":            opsAccessPolicyStringValue(strings.TrimSpace(cfg.CommunityJoinURL)),
		"access_control_setting.primary_join_url":              opsAccessPolicyStringValue(strings.TrimSpace(cfg.PrimaryJoinURL)),
		"access_control_setting.deny_message":                  opsAccessPolicyStringValue(strings.TrimSpace(cfg.DenyMessage)),
		"access_control_setting.upgrade_message":               opsAccessPolicyStringValue(strings.TrimSpace(cfg.UpgradeMessage)),
		"access_control_setting.reward_soft_floor_quota":       opsAccessPolicyStringValue(cfg.RewardSoftFloorQuota),
		"access_control_setting.reward_hard_floor_quota":       opsAccessPolicyStringValue(cfg.RewardHardFloorQuota),
		"access_control_setting.daily_site_reward_cap":         opsAccessPolicyStringValue(cfg.DailySiteRewardCap),
		"access_control_setting.daily_user_reward_cap":         opsAccessPolicyStringValue(cfg.DailyUserRewardCap),
	}
}

func opsAccessPolicyStringValue(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case bool:
		if v {
			return "true"
		}
		return "false"
	case int:
		return strconv.Itoa(v)
	case []string:
		b, _ := json.Marshal(normalizeOpsAccessPolicyStringList(v))
		return string(b)
	default:
		b, _ := json.Marshal(v)
		return string(b)
	}
}

func opsAccessPolicySourceMap() map[string]string {
	out := map[string]string{}
	for _, key := range opsAccessPolicyOptionKeys {
		out[strings.TrimPrefix(key, "access_control_setting.")] = key
	}
	return out
}

func normalizeOpsAccessPolicyStringList(items []string) []string {
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
	return out
}

func opsAccessPolicyRegisteredGroups(groups []OpsGroupView, cfg *operation_setting.AccessControlSetting) map[string][]OpsAccessPolicyGroupRef {
	out := map[string][]OpsAccessPolicyGroupRef{
		"primary_candidates":   {},
		"community_candidates": {},
		"all":                  {},
	}
	primaryFamily := opsAccessPolicyPrimaryFamily(cfg.PrimaryPlatform)
	for _, group := range groups {
		ref := OpsAccessPolicyGroupRef{ID: group.ID, Platform: group.Platform, Family: group.PlatformFamily, GroupID: group.GroupID, Name: group.GroupName, Role: group.Role, Enabled: group.Enabled}
		out["all"] = append(out["all"], ref)
		if group.Enabled && group.PlatformFamily == primaryFamily && group.Role == "primary_mainfield" {
			out["primary_candidates"] = append(out["primary_candidates"], ref)
		}
		if group.Enabled && (group.PlatformFamily == "community" || group.Role == "community_intake") {
			out["community_candidates"] = append(out["community_candidates"], ref)
		}
	}
	return out
}

func opsAccessPolicyCoverage(groups []OpsGroupView, cfg *operation_setting.AccessControlSetting) (map[string]interface{}, []string) {
	primaryEffective, communityEffective := opsAccessPolicyResolvedBindingGroups(AgentSiteID(), cfg)
	registeredByID := map[string]OpsGroupView{}
	for _, group := range groups {
		registeredByID[strings.TrimSpace(group.GroupID)] = group
	}
	primaryMatched, primaryMissing := opsAccessPolicyMatchConfiguredGroups(cfg.PrimaryGroupIDs, registeredByID)
	communityMatched, communityMissing := opsAccessPolicyMatchConfiguredGroups(cfg.CommunityGroupIDs, registeredByID)
	warnings := []string{}
	if len(primaryEffective) == 0 {
		warnings = append(warnings, "尚未配置主场群：未绑定 QQ/TG 主群的用户不会解锁全站分组")
	}
	if len(communityEffective) == 0 {
		warnings = append(warnings, "尚未配置社区准入群：社区 key 分组无法精确按房间解释")
	}
	if len(cfg.CommunityOnlyGroups) == 0 {
		warnings = append(warnings, "尚未配置社区可用 API 分组：仅社区绑定用户会显示无可用分组")
	}
	if len(primaryMissing) > 0 {
		warnings = append(warnings, "部分主场群 ID 未登记到群组注册表："+strings.Join(primaryMissing, ", "))
	}
	if len(communityMissing) > 0 {
		warnings = append(warnings, "部分社区群/房间 ID 未登记到群组注册表："+strings.Join(communityMissing, ", "))
	}
	return map[string]interface{}{
		"primary_configured":               cfg.PrimaryGroupIDs,
		"community_configured":             cfg.CommunityGroupIDs,
		"primary_effective":                primaryEffective,
		"community_effective":              communityEffective,
		"primary_matched":                  primaryMatched,
		"community_matched":                communityMatched,
		"primary_missing":                  primaryMissing,
		"community_missing":                communityMissing,
		"community_only_groups_configured": len(cfg.CommunityOnlyGroups) > 0,
		"full_access_groups_configured":    len(cfg.FullAccessGroups) > 0,
	}, warnings
}

func opsAccessPolicyMatchConfiguredGroups(configured []string, registered map[string]OpsGroupView) ([]OpsAccessPolicyGroupRef, []string) {
	matched := []OpsAccessPolicyGroupRef{}
	missing := []string{}
	for _, groupID := range normalizeOpsAccessPolicyStringList(configured) {
		if group, ok := registered[groupID]; ok {
			matched = append(matched, OpsAccessPolicyGroupRef{ID: group.ID, Platform: group.Platform, Family: group.PlatformFamily, GroupID: group.GroupID, Name: group.GroupName, Role: group.Role, Enabled: group.Enabled})
		} else {
			missing = append(missing, groupID)
		}
	}
	return matched, missing
}

func opsAccessPolicyHumanSummary(cfg *operation_setting.AccessControlSetting, coverage map[string]interface{}, warnings []string) []string {
	out := []string{}
	if cfg.Enabled {
		out = append(out, "访问控制已开启：用户创建/启用 Key 与请求时会按绑定状态收敛可用分组。")
	} else {
		out = append(out, "访问控制未开启：所有用户按原套餐组使用，绑定状态只用于展示。")
	}
	primaryEffective, _ := coverage["primary_effective"].([]string)
	communityEffective, _ := coverage["community_effective"].([]string)
	primarySource := opsAccessPolicyBindingGroupSource(cfg.PrimaryGroupIDs, primaryEffective)
	communitySource := opsAccessPolicyBindingGroupSource(cfg.CommunityGroupIDs, communityEffective)
	if primarySource != "configured" || communitySource != "configured" {
		out = append(out, "配置值只保存管理员显式填写的群组 ID；运行值会按群组注册表自动补齐。主场来源："+opsAccessPolicyBindingSourceLabel(primarySource)+"；社区来源："+opsAccessPolicyBindingSourceLabel(communitySource)+"。")
	}
	out = append(out, "主场平台为 "+accessControlPrimaryPlatformLabel(cfg.PrimaryPlatform)+"；命中主场群后可使用 full_access_groups，未限制时等同套餐全部分组。")
	out = append(out, "仅社区绑定用户只能使用 community_only_groups；未配置该列表时不会给社区用户额外 API 分组。")
	if cfg.AllowPaidBypass {
		out = append(out, "付费套餐组命中 paid_user_groups 后拥有最高通行级别。")
	}
	if len(warnings) > 0 {
		out = append(out, "当前仍有配置缺口需要处理："+strings.Join(warnings, "；"))
	}
	return out
}

func opsAccessPolicyPrimaryFamily(platform string) string {
	switch normalizeAccessPlatform(platform) {
	case "tg":
		return "tg"
	case "community":
		return "community"
	default:
		return "qq"
	}
}

func opsAccessPolicyResolvedBindingGroups(siteID string, cfg *operation_setting.AccessControlSetting) ([]string, []string) {
	if cfg == nil {
		return []string{}, []string{}
	}
	configuredPrimary := normalizeOpsAccessPolicyStringList(cfg.PrimaryGroupIDs)
	configuredCommunity := normalizeOpsAccessPolicyStringList(cfg.CommunityGroupIDs)
	rows, err := model.ListChatGroupsBySite(resolveOpsSiteID(siteID), "", "", "")
	if err != nil {
		return configuredPrimary, configuredCommunity
	}
	primaryFamily := opsAccessPolicyPrimaryFamily(cfg.PrimaryPlatform)
	derivedPrimary := make([]string, 0, len(rows))
	derivedCommunity := make([]string, 0, len(rows))
	for _, row := range rows {
		status := normalizeOpsGroupStatus(row.Status)
		if status == "disabled" || status == "archived" {
			continue
		}
		role := strings.TrimSpace(strings.ToLower(row.Role))
		family := opsPlatformFamily(row.Platform)
		if family == primaryFamily && (role == "primary_mainfield" || role == "primary") {
			derivedPrimary = append(derivedPrimary, row.GroupId)
		}
		if family == "community" || role == "community_intake" || role == "community" {
			derivedCommunity = append(derivedCommunity, row.GroupId)
		}
	}
	primaryEffective := normalizeOpsAccessPolicyStringList(
		append(append([]string{}, configuredPrimary...), derivedPrimary...),
	)
	communityEffective := normalizeOpsAccessPolicyStringList(
		append(append([]string{}, configuredCommunity...), derivedCommunity...),
	)
	return primaryEffective, communityEffective
}

func opsAccessPolicyBindingGroupSource(configured []string, effective []string) string {
	if len(configured) == 0 && len(effective) == 0 {
		return "missing"
	}
	if len(configured) == 0 && len(effective) > 0 {
		return "registry"
	}
	if len(configured) > 0 && len(effective) > len(normalizeOpsAccessPolicyStringList(configured)) {
		return "merged"
	}
	return "configured"
}

func opsAccessPolicyBindingSourceLabel(source string) string {
	switch strings.TrimSpace(source) {
	case "registry":
		return "仅注册表推导"
	case "merged":
		return "显式配置 + 注册表合并"
	case "configured":
		return "仅显式配置"
	default:
		return "未配置"
	}
}

func sortedOpsAccessPolicyKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
