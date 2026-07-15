package service

import (
	"encoding/json"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"gorm.io/gorm"
)

type OpsControlPlaneSnapshot struct {
	SiteID      string                            `json:"site_id"`
	GeneratedAt int64                             `json:"generated_at"`
	Configured  map[string]map[string]interface{} `json:"configured"`
	Effective   map[string]map[string]interface{} `json:"effective"`
	Runtime     map[string]map[string]interface{} `json:"runtime"`
	SourceMap   map[string]OpsControlPlaneSource  `json:"source_map"`
	Drift       []OpsControlPlaneDrift            `json:"drift"`
}

type OpsControlPlaneSource struct {
	Field       string `json:"field"`
	Domain      string `json:"domain"`
	OptionKey   string `json:"option_key,omitempty"`
	Source      string `json:"source"`
	State       string `json:"state"`
	Message     string `json:"message"`
	DisplayName string `json:"display_name,omitempty"`
}

type OpsControlPlaneDrift struct {
	Domain     string      `json:"domain"`
	Field      string      `json:"field"`
	Severity   string      `json:"severity"`
	State      string      `json:"state"`
	Message    string      `json:"message"`
	Configured interface{} `json:"configured,omitempty"`
	Effective  interface{} `json:"effective,omitempty"`
	Runtime    interface{} `json:"runtime,omitempty"`
	Action     string      `json:"action,omitempty"`
}

type opsOptionRow struct {
	Key   string `gorm:"column:key"`
	Value string `gorm:"column:value"`
}

var opsControlPlaneKnownKeys = []string{
	"agent_setting.enabled",
	"agent_setting.site_id",
	"agent_setting.site_name",
	"agent_setting.public_base_url",
	"agent_setting.api_base_url",
	"agent_setting.llm_provider",
	"agent_setting.llm_model",
	"agent_setting.llm_base_url",
	"agent_setting.llm_api_key",
	"agent_setting.planner_provider",
	"agent_setting.hermes_base_url",
	"agent_setting.hermes_api_key",
	"agent_setting.director_enabled",
	"agent_setting.community_enabled",
	"agent_setting.growth_enabled",
	"agent_setting.activity_enabled",
	"agent_setting.game_enabled",
	"agent_setting.risk_enabled",
	"agent_setting.ops_enabled",
	"agent_setting.budget_enabled",
	"agent_setting.auto_execute_low_risk",
	"agent_setting.human_approval_enabled",
	"agent_setting.daily_budget_quota",
	"agent_setting.growth_budget_quota",
	"agent_setting.activity_budget_quota",
	"agent_setting.game_budget_quota",
	"agent_setting.ops_comp_budget_quota",
	"agent_setting.community_budget_quota",
	"agent_setting.single_action_limit_quota",
	"agent_setting.user_daily_limit_quota",
	"agent_setting.approval_threshold_quota",
	"agent_setting.risk_deny_threshold",
	"agent_setting.risk_review_threshold",
	"agent_setting.min_message_chars",
	"agent_setting.min_distinct_messages",
	"agent_setting.qq_bot_enabled",
	"agent_setting.qq_onebot_url",
	"agent_setting.qq_group_id",
	"agent_setting.qq_access_token",
	"agent_setting.tg_bot_enabled",
	"agent_setting.tg_bot_token",
	"agent_setting.tg_chat_id",
	"agent_setting.chatops_enabled",
	"agent_setting.chatops_webhook_secret",
	"agent_setting.chatops_admin_external_ids",
	"agent_setting.chatops_command_prefixes",
	"agent_setting.chatops_auto_reply",
	"agent_setting.chatops_allow_natural_language",
	"agent_setting.chatops_require_admin_for_ops",
	"agent_setting.chatops_trust_group_admin",
	"agent_setting.legacy_config_import_enabled",
	"agent_setting.legacy_config_import_reasons",
	"agent_setting.community_room_id",
	"agent_setting.community_host",
	"agent_setting.system_prompt",
	"agent_setting.site_knowledge",
	"agent_setting.welcome_template",
	"agent_setting.activity_policy",
	"agent_setting.risk_policy",
	"community_gate_setting.enabled",
	"community_gate_setting.provider_slug",
	"community_gate_setting.community_host",
	"community_gate_setting.room_id",
	"community_gate_setting.room_ids",
	"community_gate_setting.room_match_mode",
	"community_gate_setting.require_oauth_binding",
	"community_gate_setting.require_room_membership",
	"community_gate_setting.only_allow_provider_register",
	"community_gate_setting.disable_password_register",
	"community_gate_setting.disable_builtin_oauth_register",
	"community_gate_setting.auto_invite_on_login",
	"community_gate_setting.block_token_when_not_compliant",
	"community_gate_setting.allow_admin_bypass",
	"community_gate_setting.member_cache_ttl_seconds",
	"community_gate_setting.member_scan_limit",
	"community_gate_setting.audit_enabled",
	"community_gate_setting.token_disable_mode",
	"community_gate_setting.denied_message",
	"community_bot_setting.enabled",
	"community_bot_setting.community_host",
	"community_bot_setting.provider_slug",
	"community_bot_setting.room_id",
	"community_bot_setting.oauth_callback_url",
	"community_bot_setting.oauth_client_id",
	"community_bot_setting.oauth_client_secret",
	"community_bot_setting.oauth_state_secret",
	"community_bot_setting.oauth_verifier_secret",
	"community_bot_setting.bot_token",
	"community_bot_setting.bot_user_id",
	"community_bot_setting.bot_username",
	"community_bot_setting.auto_invite_enabled",
	"community_bot_setting.invite_on_oauth_login",
	"community_bot_setting.join_reward_enabled",
	"community_bot_setting.join_reward_min_quota",
	"community_bot_setting.join_reward_max_quota",
	"community_bot_setting.message_scan_interval_minutes",
	"community_bot_setting.message_scan_interval_seconds",
	"community_bot_setting.streaming_enabled",
	"community_bot_setting.command_burn_after_seconds",
	"community_bot_setting.message_lookback_minutes",
	"community_bot_setting.message_scan_limit",
	"community_bot_setting.notification_enabled",
	"membership_risk.enabled",
	"membership_risk.dry_run",
	"membership_risk.grace_hours",
	"membership_risk.auto_restore_on_rejoin",
	"membership_risk.paid_bypass_enabled",
	"membership_risk.event_secret",
	"membership_risk.freeze_community_tokens_after_grace",
	"membership_risk.revoke_community_access_after_grace",
	"membership_risk.block_checkin_on_left",
	"membership_risk.block_game_reward_on_left",
	"membership_risk.block_invite_reward_on_left",
	"membership_risk.block_campaign_bonus_on_left",
	"membership_risk.notify_user_on_left",
	"membership_risk.notify_admin_on_bulk_left",
	"membership_risk.qq_events_enabled",
	"membership_risk.tg_events_enabled",
	"membership_risk.scheduled_recheck_enabled",
	"membership_risk.scheduled_recheck_interval_hours",
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

func BuildOpsControlPlaneSite(siteID string) (*OpsControlPlaneSnapshot, error) {
	siteID = strings.TrimSpace(siteID)
	if siteID == "" {
		siteID = "default"
	}
	raw, loadErr := loadOpsOptionValues(opsControlPlaneKnownKeys)
	snapshot := &OpsControlPlaneSnapshot{
		SiteID:      siteID,
		GeneratedAt: time.Now().Unix(),
		Configured:  map[string]map[string]interface{}{},
		Effective:   map[string]map[string]interface{}{},
		Runtime:     map[string]map[string]interface{}{},
		SourceMap:   map[string]OpsControlPlaneSource{},
		Drift:       []OpsControlPlaneDrift{},
	}
	snapshot.Configured["agent"] = buildOpsConfiguredDomain(raw, "agent_setting.")
	snapshot.Configured["community_gate"] = buildOpsConfiguredDomain(raw, "community_gate_setting.")
	snapshot.Configured["community_bot"] = buildOpsConfiguredDomain(raw, "community_bot_setting.")
	snapshot.Configured["access_control"] = buildOpsConfiguredDomain(raw, "access_control_setting.")
	snapshot.Configured["membership_risk"] = buildOpsConfiguredDomain(raw, "membership_risk.")
	if loadErr != nil {
		snapshot.Drift = append(snapshot.Drift, OpsControlPlaneDrift{
			Domain:   "system",
			Field:    "options",
			Severity: "warning",
			State:    "option_load_failed",
			Message:  "读取配置表失败，本次只显示代码默认值与运行状态。",
			Action:   "检查 options 表和数据库连接。",
		})
	}
	fillOpsAccessControlTruth(snapshot, raw)
	fillOpsMembershipRiskTruth(snapshot, raw)
	fillOpsCommunityGateTruth(snapshot, raw)
	fillOpsCommunityBotTruth(snapshot, raw)
	fillOpsAgentTruth(snapshot, raw)
	return snapshot, nil
}

func loadOpsOptionValues(keys []string) (map[string]string, error) {
	values := map[string]string{}
	if len(keys) == 0 {
		return values, nil
	}
	var rows []opsOptionRow
	err := buildOpsOptionValuesQuery(model.DB, keys).
		Scan(&rows).Error
	if err != nil {
		return values, err
	}
	for _, row := range rows {
		values[strings.TrimSpace(row.Key)] = row.Value
	}
	return values, nil
}

func buildOpsOptionValuesQuery(db *gorm.DB, keys []string) *gorm.DB {
	return db.Model(&model.Option{}).
		Select("key", "value").
		Where("key IN ?", keys)
}

func buildOpsConfiguredDomain(raw map[string]string, prefix string) map[string]interface{} {
	out := map[string]interface{}{}
	for _, key := range opsControlPlaneKnownKeys {
		if !strings.HasPrefix(key, prefix) {
			continue
		}
		field := strings.TrimPrefix(key, prefix)
		value, ok := raw[key]
		if ok {
			out[field] = map[string]interface{}{
				"configured": true,
				"option_key": key,
				"value":      sanitizeOpsOptionValue(key, value),
			}
		} else {
			out[field] = map[string]interface{}{
				"configured": false,
				"option_key": key,
				"value":      nil,
			}
		}
	}
	return out
}

func fillOpsAccessControlTruth(snapshot *OpsControlPlaneSnapshot, raw map[string]string) {
	cfg := operation_setting.GetAccessControlSetting()
	effective := map[string]interface{}{
		"enabled":                       cfg.Enabled,
		"primary_platform":              cfg.PrimaryPlatform,
		"primary_group_ids":             cfg.PrimaryGroupIDs,
		"community_group_ids":           cfg.CommunityGroupIDs,
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
		"community_join_url":            cfg.CommunityJoinURL,
		"primary_join_url":              cfg.PrimaryJoinURL,
		"deny_message":                  cfg.DenyMessage,
		"upgrade_message":               cfg.UpgradeMessage,
		"reward_soft_floor_quota":       cfg.RewardSoftFloorQuota,
		"reward_hard_floor_quota":       cfg.RewardHardFloorQuota,
		"daily_site_reward_cap":         cfg.DailySiteRewardCap,
		"daily_user_reward_cap":         cfg.DailyUserRewardCap,
	}
	snapshot.Effective["access_control"] = effective
	snapshot.Runtime["access_control"] = map[string]interface{}{
		"enabled":              cfg.Enabled,
		"check_on_login":       cfg.CheckOnLogin,
		"enforce_request_time": cfg.EnforceRequestTime,
		"block_token_create":   cfg.BlockTokenCreate,
		"block_token_enable":   cfg.BlockTokenEnable,
	}
	opsAddSourcesForDomain(snapshot, raw, "access_control", "access_control_setting.", effective)
	if len(cfg.PrimaryGroupIDs) == 0 {
		snapshot.Drift = append(snapshot.Drift, OpsControlPlaneDrift{
			Domain:   "access_control",
			Field:    "primary_group_ids",
			Severity: "warning",
			State:    "missing",
			Message:  "主群解锁列表为空，当前无法明确哪些群拥有全量权限。",
			Action:   "在访问控制里保存 primary_group_ids。",
		})
	}
}

func fillOpsMembershipRiskTruth(snapshot *OpsControlPlaneSnapshot, raw map[string]string) {
	cfg := operation_setting.GetMembershipRiskSetting()
	effective := map[string]interface{}{
		"enabled":                             cfg.Enabled,
		"dry_run":                             cfg.DryRun,
		"grace_hours":                         cfg.GraceHours,
		"auto_restore_on_rejoin":              cfg.AutoRestoreOnRejoin,
		"paid_bypass_enabled":                 cfg.PaidBypassEnabled,
		"event_secret":                        cfg.EventSecret,
		"freeze_community_tokens_after_grace": cfg.FreezeCommunityTokensAfterGrace,
		"revoke_community_access_after_grace": cfg.RevokeCommunityAccessAfterGrace,
		"block_checkin_on_left":               cfg.BlockCheckinOnLeft,
		"block_game_reward_on_left":           cfg.BlockGameRewardOnLeft,
		"block_invite_reward_on_left":         cfg.BlockInviteRewardOnLeft,
		"block_campaign_bonus_on_left":        cfg.BlockCampaignBonusOnLeft,
		"notify_user_on_left":                 cfg.NotifyUserOnLeft,
		"notify_admin_on_bulk_left":           cfg.NotifyAdminOnBulkLeft,
		"qq_events_enabled":                   cfg.QQEventsEnabled,
		"tg_events_enabled":                   cfg.TGEventsEnabled,
		"scheduled_recheck_enabled":           cfg.ScheduledRecheckEnabled,
		"scheduled_recheck_interval_hours":    cfg.ScheduledRecheckIntervalHours,
	}
	snapshot.Effective["membership_risk"] = effective
	snapshot.Runtime["membership_risk"] = map[string]interface{}{
		"enabled": cfg.Enabled,
		"dry_run": cfg.DryRun,
		"traced_sources": []string{
			"/api/chat-membership/admin/overview",
			"/api/chat-membership/admin/states",
		},
	}
	opsAddSourcesForDomain(snapshot, raw, "membership_risk", "membership_risk.", effective)
	if cfg.Enabled && strings.TrimSpace(cfg.EventSecret) == "" {
		snapshot.Drift = append(snapshot.Drift, OpsControlPlaneDrift{
			Domain:   "membership_risk",
			Field:    "event_secret",
			Severity: "warning",
			State:    "missing",
			Message:  "成员资格风控已启用，但事件签名密钥为空。",
			Action:   "保存 membership_risk.event_secret 后再接收成员事件。",
		})
	}
}

func fillOpsCommunityGateTruth(snapshot *OpsControlPlaneSnapshot, raw map[string]string) {
	cfg, providerSlug, communityHost, primaryRoomID, roomIDs, roomMatchMode, botToken := communityGateEffectiveConfig()
	effective := map[string]interface{}{
		"enabled":                         cfg.Enabled,
		"provider_slug":                   providerSlug,
		"community_host":                  communityHost,
		"room_id":                         primaryRoomID,
		"room_ids":                        roomIDs,
		"room_match_mode":                 roomMatchMode,
		"require_oauth_binding":           cfg.RequireOAuthBinding,
		"require_room_membership":         cfg.RequireRoomMembership,
		"only_allow_provider_register":    cfg.OnlyAllowProviderRegister,
		"disable_password_register":       cfg.DisablePasswordRegister,
		"disable_builtin_oauth_register":  cfg.DisableBuiltinOAuthRegister,
		"auto_invite_on_login":            cfg.AutoInviteOnLogin,
		"block_token_when_not_compliant":  cfg.BlockTokenWhenNotCompliant,
		"allow_admin_bypass":              cfg.AllowAdminBypass,
		"member_cache_ttl_seconds":        cfg.MemberCacheTTLSeconds,
		"member_scan_limit":               cfg.MemberScanLimit,
		"audit_enabled":                   cfg.AuditEnabled,
		"token_disable_mode":              cfg.TokenDisableMode,
		"denied_message":                  cfg.DeniedMessage,
		"bot_token_configured_for_checks": botToken != "",
	}
	snapshot.Effective["community_gate"] = effective
	communityGateCache.RLock()
	snapshot.Runtime["community_gate"] = map[string]interface{}{
		"cached_rooms": len(communityGateCache.rooms),
		"cached_users": len(communityGateCache.results),
	}
	communityGateCache.RUnlock()
	opsAddSourcesForDomain(snapshot, raw, "community_gate", "community_gate_setting.", effective)
	opsAddFallbackDrift(snapshot, raw, "community_gate", "provider_slug", "community_gate_setting.provider_slug", providerSlug, "community_bot_setting.provider_slug")
	opsAddFallbackDrift(snapshot, raw, "community_gate", "community_host", "community_gate_setting.community_host", communityHost, "community_bot_setting.community_host")
	opsAddFallbackDrift(snapshot, raw, "community_gate", "room_id", "community_gate_setting.room_id", primaryRoomID, "community_bot_setting.room_id")
	hasSavedPrimaryRoom := strings.TrimSpace(raw["community_gate_setting.room_id"]) != ""
	hasSavedRoomList := strings.TrimSpace(raw["community_gate_setting.room_ids"]) != ""
	if !hasSavedRoomList && hasSavedPrimaryRoom && len(roomIDs) > 0 {
		path := "community_gate.room_ids"
		src := snapshot.SourceMap[path]
		src.Source = "derived"
		src.State = "derived_from_primary_room"
		src.Message = "多房间列表未单独保存，当前按主房间自动派生。"
		snapshot.SourceMap[path] = src
		snapshot.Drift = append(snapshot.Drift, OpsControlPlaneDrift{
			Domain:     "community_gate",
			Field:      "room_ids",
			Severity:   "warning",
			State:      "legacy_single_room_only",
			Message:    "当前仍只有旧版单房间配置；生效中的房间列表是由主房间自动派生出来的。",
			Configured: strings.TrimSpace(raw["community_gate_setting.room_id"]),
			Effective:  roomIDs,
			Action:     "把当前主房间同步保存到 community_gate_setting.room_ids，后续新增社区群直接维护多房间列表。",
		})
	}
	if _, ok := raw["community_gate_setting.room_match_mode"]; !ok && (hasSavedPrimaryRoom || hasSavedRoomList) && strings.TrimSpace(roomMatchMode) != "" {
		snapshot.Drift = append(snapshot.Drift, OpsControlPlaneDrift{
			Domain:     "community_gate",
			Field:      "room_match_mode",
			Severity:   "warning",
			State:      "default_room_match_mode_in_effect",
			Message:    "房间匹配模式还没单独保存，当前沿用代码默认值。",
			Configured: nil,
			Effective:  roomMatchMode,
			Action:     "显式保存 community_gate_setting.room_match_mode（建议 any_of 或 all_of），避免页面只显示默认值。",
		})
	}
}

func fillOpsCommunityBotTruth(snapshot *OpsControlPlaneSnapshot, raw map[string]string) {
	cfg := operation_setting.GetCommunityBotSetting()
	status := GetCommunityBotStatus()
	effective := map[string]interface{}{
		"enabled":                       cfg.Enabled,
		"community_host":                strings.TrimRight(strings.TrimSpace(cfg.CommunityHost), "/"),
		"provider_slug":                 strings.TrimSpace(cfg.ProviderSlug),
		"room_id":                       strings.TrimSpace(cfg.RoomID),
		"oauth_callback_url":            strings.TrimSpace(cfg.OAuthCallbackURL),
		"bot_token_configured":          strings.TrimSpace(cfg.BotToken) != "",
		"bot_user_id":                   strings.TrimSpace(cfg.BotUserID),
		"bot_username":                  strings.TrimSpace(cfg.BotUsername),
		"auto_invite_enabled":           cfg.AutoInviteEnabled,
		"invite_on_oauth_login":         cfg.InviteOnOAuthLogin,
		"join_reward_enabled":           cfg.JoinRewardEnabled,
		"join_reward_min_quota":         cfg.JoinRewardMinQuota,
		"join_reward_max_quota":         cfg.JoinRewardMaxQuota,
		"message_scan_interval_minutes": cfg.MessageScanIntervalMinutes,
		"message_scan_interval_seconds": cfg.MessageScanIntervalSeconds,
		"streaming_enabled":             cfg.StreamingEnabled,
		"command_burn_after_seconds":    cfg.CommandBurnAfterSeconds,
		"message_lookback_minutes":      cfg.MessageLookbackMinutes,
		"message_scan_limit":            cfg.MessageScanLimit,
		"notification_enabled":          cfg.NotificationEnabled,
	}
	snapshot.Effective["community_bot"] = effective
	snapshot.Runtime["community_bot"] = map[string]interface{}{
		"enabled":         status.Enabled,
		"configured":      status.Configured,
		"authorized":      status.Authorized,
		"bot_username":    status.BotUsername,
		"bot_user_id":     status.BotUserID,
		"room_id":         status.RoomID,
		"last_message_id": status.LastMessageID,
		"last_scanned_at": status.LastScannedAt,
	}
	opsAddSourcesForDomain(snapshot, raw, "community_bot", "community_bot_setting.", effective)
	if cfg.Enabled && !status.Authorized {
		snapshot.Drift = append(snapshot.Drift, OpsControlPlaneDrift{
			Domain:   "community_bot",
			Field:    "bot_token",
			Severity: "error",
			State:    "token_missing",
			Message:  "社区机器人已启用，但运行态没有可用 Bot Token。",
			Action:   "在社区机器人配置里保存 Bot Token 后再测试签到、验牌和邀请。",
		})
	}
}

func fillOpsAgentTruth(snapshot *OpsControlPlaneSnapshot, raw map[string]string) {
	cfg := operation_setting.GetAgentSetting()
	fields := buildOpsStructEffectiveMap("agent_setting.", cfg)
	for _, key := range opsControlPlaneKnownKeys {
		if !strings.HasPrefix(key, "agent_setting.") {
			continue
		}
		field := strings.TrimPrefix(key, "agent_setting.")
		if value, ok := raw[key]; ok {
			fields[field] = sanitizeOpsOptionValue(key, value)
		}
	}
	snapshot.Effective["agent"] = fields
	snapshot.Runtime["agent"] = map[string]interface{}{
		"truth": "agent runtime is supplied by /api/agent/dashboard; this endpoint only reconciles persisted agent_setting.* values.",
	}
	opsAddSourcesForDomain(snapshot, raw, "agent", "agent_setting.", fields)
	if cfg.QQBotEnabled && strings.TrimSpace(cfg.QQGroupID) == "" {
		snapshot.Drift = append(snapshot.Drift, OpsControlPlaneDrift{
			Domain:   "agent",
			Field:    "qq_group_id",
			Severity: "warning",
			State:    "important_config_missing",
			Message:  "QQ Bot 已启用，但主群 ID 为空，无法把 QQ 群事件稳定路由到目标群。",
			Action:   "在 Agent 运营中心保存 qq_group_id。",
		})
	}
	if cfg.TGBotEnabled && strings.TrimSpace(cfg.TGChatID) == "" {
		snapshot.Drift = append(snapshot.Drift, OpsControlPlaneDrift{
			Domain:   "agent",
			Field:    "tg_chat_id",
			Severity: "warning",
			State:    "important_config_missing",
			Message:  "TG Bot 已启用，但主聊天 ID 为空，无法把 TG 群事件稳定路由到目标群。",
			Action:   "在 Agent 运营中心保存 tg_chat_id。",
		})
	}
	if cfg.ChatOpsEnabled && strings.TrimSpace(cfg.ChatOpsWebhookSecret) == "" {
		snapshot.Drift = append(snapshot.Drift, OpsControlPlaneDrift{
			Domain:   "agent",
			Field:    "chatops_webhook_secret",
			Severity: "warning",
			State:    "important_config_missing",
			Message:  "ChatOps 已启用，但 webhook secret 为空，请求签名无法校验。",
			Action:   "在 Agent 运营中心保存 chatops_webhook_secret。",
		})
	}
}

func buildOpsStructEffectiveMap(prefix string, cfg interface{}) map[string]interface{} {
	out := map[string]interface{}{}
	value := reflect.ValueOf(cfg)
	if !value.IsValid() {
		return out
	}
	if value.Kind() == reflect.Ptr {
		if value.IsNil() {
			return out
		}
		value = value.Elem()
	}
	if value.Kind() != reflect.Struct {
		return out
	}
	valueType := value.Type()
	for i := 0; i < value.NumField(); i++ {
		structField := valueType.Field(i)
		jsonTag := strings.Split(structField.Tag.Get("json"), ",")[0]
		if jsonTag == "" || jsonTag == "-" {
			continue
		}
		out[jsonTag] = sanitizeOpsEffectiveValue(prefix+jsonTag, value.Field(i).Interface())
	}
	return out
}

func sanitizeOpsEffectiveValue(optionKey string, value interface{}) interface{} {
	switch typed := value.(type) {
	case string:
		return sanitizeOpsOptionValue(optionKey, typed)
	default:
		return value
	}
}

func opsAddSourcesForDomain(snapshot *OpsControlPlaneSnapshot, raw map[string]string, domain, prefix string, effective map[string]interface{}) {
	fields := make([]string, 0, len(effective))
	for field := range effective {
		fields = append(fields, field)
	}
	sort.Strings(fields)
	for _, field := range fields {
		optionKey := prefix + field
		_, configured := raw[optionKey]
		source := "default"
		state := "using_default"
		message := "未保存配置，当前使用代码默认值或上游合并值。"
		if configured {
			source = "configured"
			state = "in_effect"
			message = "配置已保存，并正在作为当前生效值使用。"
		}
		if strings.Contains(field, "token") || strings.Contains(field, "secret") || strings.Contains(field, "api_key") {
			if configured && strings.TrimSpace(raw[optionKey]) != "" {
				message = "密钥已保存，页面只显示是否配置，不回显原文。"
			}
		}
		path := domain + "." + field
		snapshot.SourceMap[path] = OpsControlPlaneSource{
			Field:       field,
			Domain:      domain,
			OptionKey:   optionKey,
			Source:      source,
			State:       state,
			Message:     message,
			DisplayName: opsHumanFieldName(field),
		}
	}
}

func opsAddFallbackDrift(snapshot *OpsControlPlaneSnapshot, raw map[string]string, domain, field, primaryKey string, effective interface{}, fallbackKey string) {
	primary := strings.TrimSpace(raw[primaryKey])
	if primary != "" {
		return
	}
	if strings.TrimSpace(raw[fallbackKey]) == "" {
		return
	}
	path := domain + "." + field
	src := snapshot.SourceMap[path]
	src.Source = "fallback"
	src.State = "using_compatible_value"
	src.Message = "主配置未保存，当前临时采用兼容配置值作为生效值。"
	snapshot.SourceMap[path] = src
	snapshot.Drift = append(snapshot.Drift, OpsControlPlaneDrift{
		Domain:     domain,
		Field:      field,
		Severity:   "warning",
		State:      "fallback_in_effect",
		Message:    "配置值为空，但运行仍从兼容配置取到值；这会让后台看起来像没配置。",
		Configured: primary,
		Effective:  effective,
		Action:     "把兼容值保存到主配置字段，收敛为配置值=生效值。",
	})
}

func sanitizeOpsOptionValue(key string, value string) interface{} {
	key = strings.ToLower(strings.TrimSpace(key))
	if strings.Contains(key, "secret") || strings.Contains(key, "token") || strings.Contains(key, "api_key") {
		return map[string]interface{}{
			"configured": strings.TrimSpace(value) != "",
			"masked":     true,
		}
	}
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	if strings.HasPrefix(trimmed, "[") || strings.HasPrefix(trimmed, "{") {
		var decoded interface{}
		if err := json.Unmarshal([]byte(trimmed), &decoded); err == nil {
			return decoded
		}
	}
	if parsed, err := strconv.ParseBool(strings.ToLower(trimmed)); err == nil {
		return parsed
	}
	if parsed, err := strconv.Atoi(trimmed); err == nil {
		return parsed
	}
	return value
}

func opsHumanFieldName(field string) string {
	field = strings.ReplaceAll(field, "_", " ")
	field = strings.TrimSpace(field)
	if field == "" {
		return ""
	}
	return strings.ToUpper(field[:1]) + field[1:]
}
