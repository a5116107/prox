package service

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var defaultLegacyConfigImportReasons = []string{
	"bootstrap_local_config",
	"manual_legacy_import",
	"manual_legacy_recovery",
}

type AgentConfigImportGuard struct {
	Allowed         bool     `json:"allowed"`
	SiteID          string   `json:"site_id"`
	Mode            string   `json:"mode"`
	RequestedReason string   `json:"requested_reason,omitempty"`
	ReasonCode      string   `json:"reason_code"`
	Message         string   `json:"message"`
	AllowedReasons  []string `json:"allowed_reasons,omitempty"`
}

func ExportAgentGameConfigSnapshot(siteID string) (map[string]any, error) {
	siteID = strings.TrimSpace(siteID)
	if siteID == "" {
		siteID = AgentSiteID()
	}
	site, err := exportAgentGameConfigSite(siteID)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"_version":     2,
		"source":       "newapi_db",
		"generated_at": time.Now().Unix(),
		"sites": map[string]any{
			siteID: site,
		},
	}, nil
}

func exportAgentGameConfigSite(siteID string) (map[string]any, error) {
	out := map[string]any{
		"site_name":  siteID,
		"platforms":  map[string]any{},
		"config_src": "newapi_db",
	}
	if model.DB == nil {
		return out, nil
	}
	platforms := out["platforms"].(map[string]any)
	var groups []model.ChatGroup
	var games []model.GroupGameConfig
	var chatops []model.GroupChatOpsConfig
	if err := model.DB.Where("site_id = ?", siteID).Order("platform asc, group_id asc, id asc").Find(&groups).Error; err != nil {
		return nil, err
	}
	if err := model.DB.Where("site_id = ?", siteID).Order("platform asc, group_id asc, game_code asc, id asc").Find(&games).Error; err != nil {
		return nil, err
	}
	if err := model.DB.Where("site_id = ?", siteID).Order("platform asc, group_id asc, id asc").Find(&chatops).Error; err != nil {
		return nil, err
	}
	for _, row := range groups {
		plat := ensureAgentPlatformSnapshot(platforms, row.Platform)
		group := ensureAgentGroupSnapshot(plat, row.GroupId)
		extras := agentSyncJSONMap(row.ConfigJson)
		for k, v := range extras {
			group[k] = v
		}
		group["label"] = firstSyncNonEmpty(groupString(group["label"]), strings.TrimSpace(row.GroupName), row.GroupId)
		group["role"] = firstSyncNonEmpty(groupString(group["role"]), strings.TrimSpace(row.Role))
		if _, ok := group["primary"]; !ok {
			group["primary"] = strings.Contains(strings.ToLower(strings.TrimSpace(row.Role)), "primary")
		}
		group["enabled"] = strings.TrimSpace(row.Status) != "disabled"
		if row.Language != "" {
			group["locale"] = row.Language
		}
		if row.Timezone != "" {
			group["timezone"] = row.Timezone
		}
		ensureGroupGamesMap(group)
	}
	for _, row := range games {
		plat := ensureAgentPlatformSnapshot(platforms, row.Platform)
		cfg := agentSyncJSONMap(row.RuleJson)
		cfg["enabled"] = row.Enabled
		if row.BudgetPool != "" && cfg["budget_pool"] == nil {
			cfg["budget_pool"] = row.BudgetPool
		}
		if row.GroupId == "" {
			mergeIntoGameMap(plat["games"].(map[string]any), row.GameCode, cfg)
			continue
		}
		group := ensureAgentGroupSnapshot(plat, row.GroupId)
		mergeIntoGameMap(ensureGroupGamesMap(group), row.GameCode, cfg)
	}
	for _, row := range chatops {
		plat := ensureAgentPlatformSnapshot(platforms, row.Platform)
		targetGames := plat["games"].(map[string]any)
		if row.GroupId != "" {
			group := ensureAgentGroupSnapshot(plat, row.GroupId)
			targetGames = ensureGroupGamesMap(group)
		}
		rules := agentSyncJSONMap(row.RuleJson)
		checkinCfg := agentSyncMergeMap(map[string]any{"enabled": row.CheckinEnabled}, agentSyncJSONMap(rules["checkin"]))
		if row.CheckinQuota > 0 && checkinCfg["checkin_quota"] == nil && checkinCfg["quota"] == nil {
			checkinCfg["checkin_quota"] = row.CheckinQuota
		}
		verifyCfg := agentSyncMergeMap(map[string]any{"enabled": row.VerifyEnabled, "min_quota_required": row.VerifyMinQuota}, agentSyncJSONMap(rules["verify"]))
		if row.VerifyMinQuota <= 0 {
			verifyCfg["min_quota_required"] = 1
		}
		inviteCfg := agentSyncMergeMap(map[string]any{"enabled": row.InviteEnabled}, agentSyncJSONMap(rules["invite"]))
		if row.InviteRewardQuota > 0 && inviteCfg["inviter_reward_quota"] == nil && inviteCfg["reward_quota"] == nil {
			inviteCfg["inviter_reward_quota"] = row.InviteRewardQuota
		}
		if row.InviteeRewardQuota > 0 && inviteCfg["invitee_reward_quota"] == nil {
			inviteCfg["invitee_reward_quota"] = row.InviteeRewardQuota
		}
		mergeIntoGameMap(targetGames, "checkin", checkinCfg)
		mergeIntoGameMap(targetGames, "verify", verifyCfg)
		mergeIntoGameMap(targetGames, "invite", inviteCfg)
	}
	return out, nil
}

func ImportAgentGameConfigSnapshot(snapshot map[string]any, siteID string, actorUserID int, reason string) (map[string]any, error) {
	siteID = strings.TrimSpace(firstSyncNonEmpty(siteID, AgentSiteID()))
	guard := GetAgentConfigImportGuard(siteID, reason)
	if !guard.Allowed {
		recordAgentConfigImportGuard(actorUserID, guard)
		common.SysLog(fmt.Sprintf(
			"[AgentConfigSync] legacy import blocked site=%s requested_reason=%s code=%s",
			guard.SiteID,
			guard.RequestedReason,
			guard.ReasonCode,
		))
		return map[string]any{
			"ok":               false,
			"blocked":          true,
			"db_truth_locked":  guard.Mode == "db_truth_locked",
			"site_id":          guard.SiteID,
			"mode":             guard.Mode,
			"reason_code":      guard.ReasonCode,
			"message":          guard.Message,
			"requested_reason": guard.RequestedReason,
			"allowed_reasons":  guard.AllowedReasons,
		}, nil
	}
	if len(snapshot) == 0 {
		return nil, fmt.Errorf("snapshot is empty")
	}
	before, _ := ExportAgentGameConfigSnapshot(siteID)
	root := snapshot
	if nested := agentSyncJSONMap(snapshot["snapshot"]); len(nested) > 0 {
		root = nested
	}
	sites := agentSyncJSONMap(root["sites"])
	if len(sites) == 0 {
		if directPlatforms := agentSyncJSONMap(root["platforms"]); len(directPlatforms) > 0 {
			sites = map[string]any{siteID: map[string]any{"site_name": siteID, "platforms": directPlatforms}}
		}
	}
	if len(sites) == 0 {
		return nil, fmt.Errorf("snapshot.sites is required")
	}
	now := time.Now().Unix()
	processedSites := make([]string, 0, 1)
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		for rawSiteID, rawSite := range sites {
			targetSiteID := strings.TrimSpace(rawSiteID)
			if siteID != "" && targetSiteID != siteID {
				continue
			}
			siteMap := agentSyncJSONMap(rawSite)
			platforms := agentSyncJSONMap(siteMap["platforms"])
			if err := importAgentSiteSnapshotTx(tx, targetSiteID, platforms, now); err != nil {
				return err
			}
			processedSites = append(processedSites, targetSiteID)
		}
		if len(processedSites) == 0 {
			return fmt.Errorf("site_id=%s not found in snapshot", siteID)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	after, _ := ExportAgentGameConfigSnapshot(siteID)
	_ = createAgentConfigAudit(siteID, actorUserID, "game_config_sync", siteID, before, after, firstSyncNonEmpty(reason, "adapter_game_config_sync"))
	return map[string]any{
		"ok":              true,
		"blocked":         false,
		"db_truth_locked": false,
		"site_id":         siteID,
		"mode":            "legacy_bootstrap_window",
		"processed_sites": processedSites,
		"reason":          firstSyncNonEmpty(reason, "adapter_game_config_sync"),
		"actor_user_id":   actorUserID,
	}, nil
}

func GetAgentConfigImportGuard(siteID string, reason string) AgentConfigImportGuard {
	siteID = strings.TrimSpace(firstSyncNonEmpty(siteID, AgentSiteID()))
	requestedReason := normalizeAgentConfigImportReason(reason)
	allowedReasons := agentConfigImportAllowedReasons()
	cfg := operation_setting.GetAgentSetting()
	if cfg == nil || !cfg.LegacyConfigImportEnabled {
		return AgentConfigImportGuard{
			Allowed:         false,
			SiteID:          siteID,
			Mode:            "db_truth_locked",
			RequestedReason: requestedReason,
			ReasonCode:      "legacy_import_disabled",
			Message:         "旧版玩法快照导入已关闭。当前主真值在 New API 群组/玩法配置数据库中；如需一次性迁移旧 adapter 本地配置，请先在 Agent 设置中临时开启“允许旧版玩法快照导入”。",
			AllowedReasons:  allowedReasons,
		}
	}
	if requestedReason == "" {
		return AgentConfigImportGuard{
			Allowed:         false,
			SiteID:          siteID,
			Mode:            "legacy_bootstrap_window",
			RequestedReason: requestedReason,
			ReasonCode:      "legacy_import_reason_required",
			Message:         "旧版玩法快照导入需要明确原因。请填写允许原因，例如 manual_legacy_import 或 manual_legacy_recovery；未命中白名单的请求会被拒绝并写入审计。",
			AllowedReasons:  allowedReasons,
		}
	}
	allowedSet := make(map[string]struct{}, len(allowedReasons))
	for _, item := range allowedReasons {
		allowedSet[item] = struct{}{}
	}
	if _, ok := allowedSet[requestedReason]; !ok {
		return AgentConfigImportGuard{
			Allowed:         false,
			SiteID:          siteID,
			Mode:            "legacy_bootstrap_window",
			RequestedReason: requestedReason,
			ReasonCode:      "legacy_import_reason_not_allowed",
			Message:         fmt.Sprintf("旧版玩法快照导入已被拦截：原因 %q 不在允许列表中。请改用群组注册表/玩法配置后台，或仅在一次性迁移窗口内使用白名单原因。", requestedReason),
			AllowedReasons:  allowedReasons,
		}
	}
	return AgentConfigImportGuard{
		Allowed:         true,
		SiteID:          siteID,
		Mode:            "legacy_bootstrap_window",
		RequestedReason: requestedReason,
		ReasonCode:      "allowed",
		Message:         fmt.Sprintf("允许按兼容链路导入旧版玩法快照：reason=%s。导入后请尽快切回数据库真值模式。", requestedReason),
		AllowedReasons:  allowedReasons,
	}
}

func recordAgentConfigImportGuard(actorUserID int, guard AgentConfigImportGuard) {
	before := map[string]any{
		"site_id":          guard.SiteID,
		"requested_reason": guard.RequestedReason,
		"mode":             guard.Mode,
		"allowed_reasons":  guard.AllowedReasons,
	}
	after := map[string]any{
		"allowed":          guard.Allowed,
		"reason_code":      guard.ReasonCode,
		"message":          guard.Message,
		"requested_reason": guard.RequestedReason,
	}
	_ = createAgentConfigAudit(
		guard.SiteID,
		actorUserID,
		"legacy_game_config_import",
		guard.SiteID,
		before,
		after,
		firstSyncNonEmpty(guard.ReasonCode, "legacy_adapter_import_attempt"),
	)
}

func agentConfigImportAllowedReasons() []string {
	cfg := operation_setting.GetAgentSetting()
	if cfg == nil {
		return append([]string(nil), defaultLegacyConfigImportReasons...)
	}
	values := splitAgentConfigImportReasons(cfg.LegacyConfigImportReasons)
	if len(values) == 0 {
		return append([]string(nil), defaultLegacyConfigImportReasons...)
	}
	return values
}

func splitAgentConfigImportReasons(raw string) []string {
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		switch r {
		case ',', '\n', '\r', '\t', ';', '|':
			return true
		default:
			return false
		}
	})
	seen := map[string]struct{}{}
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		item := normalizeAgentConfigImportReason(part)
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

func normalizeAgentConfigImportReason(reason string) string {
	return strings.ToLower(strings.TrimSpace(reason))
}

func ExportAgentChatBindings(provider string) ([]map[string]any, error) {
	provider = normalizeSyncProvider(provider)
	rows, err := model.ListUserIdentityBindings(AgentSiteID(), provider)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		if row.UserId <= 0 || strings.TrimSpace(row.ExternalUserId) == "" {
			continue
		}
		user, err := model.GetUserById(row.UserId, false)
		if err != nil || user == nil || strings.TrimSpace(user.AffCode) == "" {
			continue
		}
		out = append(out, map[string]any{
			"provider":         provider,
			"external_user_id": row.ExternalUserId,
			"newapi_user_id":   row.UserId,
			"newapi_aff_code":  strings.TrimSpace(user.AffCode),
			"newapi_username":  firstSyncNonEmpty(row.Username, user.Username),
			"bound_at":         row.BoundAt,
			"updated_at":       row.UpdatedAt,
		})
	}
	return out, nil
}

func importAgentSiteSnapshotTx(tx *gorm.DB, siteID string, platforms map[string]any, now int64) error {
	expectedPlatforms := make([]string, 0, len(platforms))
	explicitPlatforms := make(map[string]struct{}, len(platforms))
	for rawPlatform := range platforms {
		platform := normalizeSyncPlatform(rawPlatform)
		if platform != "" {
			explicitPlatforms[platform] = struct{}{}
		}
	}
	for rawPlatform, rawPlatformCfg := range platforms {
		platform := normalizeSyncPlatform(rawPlatform)
		if platform == "" {
			continue
		}
		expectedPlatforms = syncAppendPlatform(expectedPlatforms, platform)
		platformCfg := agentSyncJSONMap(rawPlatformCfg)
		platformGames := agentSyncJSONMap(platformCfg["games"])
		groupMap := agentSyncJSONMap(platformCfg["groups"])
		groupPlatform := normalizeSyncGroupPlatform(platform)
		if len(groupMap) > 0 {
			expectedPlatforms = syncAppendPlatform(expectedPlatforms, groupPlatform)
		}
		if err := syncAgentPlatformGamesTx(tx, siteID, platform, "", platformGames, now); err != nil {
			return err
		}
		groupIDs := make([]string, 0, len(groupMap))
		for rawGroupID, rawGroupCfg := range groupMap {
			groupID := strings.TrimSpace(rawGroupID)
			if groupID == "" {
				continue
			}
			groupIDs = append(groupIDs, groupID)
			groupCfg := agentSyncJSONMap(rawGroupCfg)
			if err := upsertAgentChatGroupTx(tx, siteID, groupPlatform, groupID, groupCfg, now); err != nil {
				return err
			}
			groupGames := agentSyncJSONMap(groupCfg["games"])
			if err := syncAgentPlatformGamesTx(tx, siteID, groupPlatform, groupID, groupGames, now); err != nil {
				return err
			}
		}
		if len(groupIDs) == 0 {
			_, hasExplicitGroupPlatform := explicitPlatforms[groupPlatform]
			if groupPlatform == platform || !hasExplicitGroupPlatform {
				if err := tx.Where("site_id = ? AND platform = ?", siteID, groupPlatform).Delete(&model.ChatGroup{}).Error; err != nil {
					return err
				}
				if err := tx.Where("site_id = ? AND platform = ? AND group_id <> ''", siteID, groupPlatform).Delete(&model.GroupGameConfig{}).Error; err != nil {
					return err
				}
				if err := tx.Where("site_id = ? AND platform = ? AND group_id <> ''", siteID, groupPlatform).Delete(&model.GroupChatOpsConfig{}).Error; err != nil {
					return err
				}
			}
		} else {
			if err := tx.Where("site_id = ? AND platform = ? AND group_id NOT IN ?", siteID, groupPlatform, groupIDs).Delete(&model.ChatGroup{}).Error; err != nil {
				return err
			}
			if err := tx.Where("site_id = ? AND platform = ? AND group_id <> '' AND group_id NOT IN ?", siteID, groupPlatform, groupIDs).Delete(&model.GroupGameConfig{}).Error; err != nil {
				return err
			}
			if err := tx.Where("site_id = ? AND platform = ? AND group_id <> '' AND group_id NOT IN ?", siteID, groupPlatform, groupIDs).Delete(&model.GroupChatOpsConfig{}).Error; err != nil {
				return err
			}
		}
	}
	if len(expectedPlatforms) == 0 {
		if err := tx.Where("site_id = ?", siteID).Delete(&model.ChatGroup{}).Error; err != nil {
			return err
		}
		if err := tx.Where("site_id = ?", siteID).Delete(&model.GroupGameConfig{}).Error; err != nil {
			return err
		}
		if err := tx.Where("site_id = ?", siteID).Delete(&model.GroupChatOpsConfig{}).Error; err != nil {
			return err
		}
		return nil
	}
	if err := tx.Where("site_id = ? AND platform NOT IN ?", siteID, expectedPlatforms).Delete(&model.ChatGroup{}).Error; err != nil {
		return err
	}
	if err := tx.Where("site_id = ? AND platform NOT IN ?", siteID, expectedPlatforms).Delete(&model.GroupGameConfig{}).Error; err != nil {
		return err
	}
	if err := tx.Where("site_id = ? AND platform NOT IN ?", siteID, expectedPlatforms).Delete(&model.GroupChatOpsConfig{}).Error; err != nil {
		return err
	}
	return nil
}

func syncAgentPlatformGamesTx(tx *gorm.DB, siteID, platform, groupID string, games map[string]any, now int64) error {
	expected := make([]string, 0, len(games))
	for rawGameCode, rawCfg := range games {
		gameCode := strings.TrimSpace(rawGameCode)
		if gameCode == "" {
			continue
		}
		cfg := agentSyncJSONMap(rawCfg)
		expected = append(expected, gameCode)
		if err := upsertAgentGameConfigTx(tx, siteID, platform, groupID, gameCode, cfg, now); err != nil {
			return err
		}
	}
	if len(expected) == 0 {
		if err := tx.Where("site_id = ? AND platform = ? AND group_id = ?", siteID, platform, groupID).Delete(&model.GroupGameConfig{}).Error; err != nil {
			return err
		}
	} else {
		if err := tx.Where("site_id = ? AND platform = ? AND group_id = ? AND game_code NOT IN ?", siteID, platform, groupID, expected).Delete(&model.GroupGameConfig{}).Error; err != nil {
			return err
		}
	}
	return upsertOrDeleteAgentChatOpsConfigTx(tx, siteID, platform, groupID, games, now)
}

func upsertAgentChatGroupTx(tx *gorm.DB, siteID, platform, groupID string, groupCfg map[string]any, now int64) error {
	extras := agentSyncCopyMap(groupCfg)
	delete(extras, "games")
	delete(extras, "label")
	delete(extras, "role")
	delete(extras, "primary")
	delete(extras, "enabled")
	delete(extras, "locale")
	delete(extras, "timezone")
	row := model.ChatGroup{
		SiteId:     siteID,
		Platform:   platform,
		GroupId:    groupID,
		GroupName:  firstSyncNonEmpty(groupString(groupCfg["label"]), groupID),
		Role:       firstSyncNonEmpty(groupString(groupCfg["role"]), "secondary"),
		Status:     "active",
		Language:   firstSyncNonEmpty(groupString(groupCfg["locale"]), "zh-CN"),
		Timezone:   firstSyncNonEmpty(groupString(groupCfg["timezone"]), "Asia/Shanghai"),
		ConfigJson: mustSyncJSON(extras),
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if enabled, ok := groupCfg["enabled"].(bool); ok && !enabled {
		row.Status = "disabled"
	}
	return tx.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "site_id"}, {Name: "platform"}, {Name: "group_id"}},
		DoUpdates: clause.Assignments(map[string]any{
			"group_name":  row.GroupName,
			"role":        row.Role,
			"status":      row.Status,
			"language":    row.Language,
			"timezone":    row.Timezone,
			"config_json": row.ConfigJson,
			"updated_at":  now,
		}),
	}).Create(&row).Error
}

func upsertAgentGameConfigTx(tx *gorm.DB, siteID, platform, groupID, gameCode string, cfg map[string]any, now int64) error {
	rule := agentSyncCopyMap(cfg)
	enabled := syncBoolDefault(rule, "enabled", true)
	delete(rule, "enabled")
	budgetPool := strings.TrimSpace(firstSyncNonEmpty(groupString(cfg["budget_pool"]), groupString(cfg["pool"])))
	if budgetPool == "" {
		switch gameCode {
		case "invite":
			budgetPool = "community"
		case "checkin":
			budgetPool = "activity"
		default:
			budgetPool = "game"
		}
	}
	row := model.GroupGameConfig{
		SiteId:     siteID,
		Platform:   platform,
		GroupId:    groupID,
		GameCode:   gameCode,
		Enabled:    enabled,
		BudgetPool: budgetPool,
		RuleJson:   mustSyncJSON(rule),
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	return tx.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "site_id"}, {Name: "platform"}, {Name: "group_id"}, {Name: "game_code"}},
		DoUpdates: clause.Assignments(map[string]any{
			"enabled":     row.Enabled,
			"budget_pool": row.BudgetPool,
			"rule_json":   row.RuleJson,
			"updated_at":  now,
		}),
	}).Create(&row).Error
}

func upsertOrDeleteAgentChatOpsConfigTx(tx *gorm.DB, siteID, platform, groupID string, games map[string]any, now int64) error {
	checkin := agentSyncJSONMap(games["checkin"])
	verify := agentSyncJSONMap(games["verify"])
	invite := agentSyncJSONMap(games["invite"])
	if len(checkin) == 0 && len(verify) == 0 && len(invite) == 0 {
		return tx.Where("site_id = ? AND platform = ? AND group_id = ?", siteID, platform, groupID).Delete(&model.GroupChatOpsConfig{}).Error
	}
	row := model.GroupChatOpsConfig{
		SiteId:                siteID,
		Platform:              platform,
		GroupId:               groupID,
		CheckinEnabled:        syncBoolDefault(checkin, "enabled", true),
		VerifyEnabled:         syncBoolDefault(verify, "enabled", true),
		InviteEnabled:         syncBoolDefault(invite, "enabled", true),
		CheckinQuota:          syncFirstInt(checkin, "checkin_quota", "quota", "fixed_quota"),
		VerifyMinQuota:        syncFirstIntDefault(1, verify, "min_quota_required", "verify_min_quota", "min_quota"),
		InviteRewardQuota:     syncFirstInt(invite, "inviter_reward_quota", "reward_quota"),
		InviteeRewardQuota:    syncFirstInt(invite, "invitee_reward_quota"),
		DailyGroupRewardLimit: syncFirstInt(checkin, "daily_group_reward_limit", "daily_reward_limit"),
		RuleJson: mustSyncJSON(map[string]any{
			"checkin": checkin,
			"verify":  verify,
			"invite":  invite,
		}),
		CreatedAt: now,
		UpdatedAt: now,
	}
	return tx.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "site_id"}, {Name: "platform"}, {Name: "group_id"}},
		DoUpdates: clause.Assignments(map[string]any{
			"checkin_enabled":          row.CheckinEnabled,
			"verify_enabled":           row.VerifyEnabled,
			"invite_enabled":           row.InviteEnabled,
			"checkin_quota":            row.CheckinQuota,
			"verify_min_quota":         row.VerifyMinQuota,
			"invite_reward_quota":      row.InviteRewardQuota,
			"invitee_reward_quota":     row.InviteeRewardQuota,
			"daily_group_reward_limit": row.DailyGroupRewardLimit,
			"rule_json":                row.RuleJson,
			"updated_at":               now,
		}),
	}).Create(&row).Error
}

func createAgentConfigAudit(siteID string, actorUserID int, scope, targetKey string, before, after map[string]any, reason string) error {
	if model.DB == nil {
		return nil
	}
	return model.DB.Create(&model.AdminConfigAudit{
		SiteId:      strings.TrimSpace(siteID),
		ActorUserId: actorUserID,
		ConfigScope: strings.TrimSpace(scope),
		TargetKey:   strings.TrimSpace(targetKey),
		BeforeJson:  mustSyncJSON(before),
		AfterJson:   mustSyncJSON(after),
		Reason:      strings.TrimSpace(reason),
		CreatedAt:   time.Now().Unix(),
	}).Error
}

func ensureAgentPlatformSnapshot(platforms map[string]any, platform string) map[string]any {
	if current, ok := platforms[platform].(map[string]any); ok && current != nil {
		if _, ok := current["games"].(map[string]any); !ok {
			current["games"] = map[string]any{}
		}
		if _, ok := current["groups"].(map[string]any); !ok {
			current["groups"] = map[string]any{}
		}
		if _, ok := current["label"]; !ok {
			current["label"] = platform
		}
		return current
	}
	current := map[string]any{
		"label":  platform,
		"games":  map[string]any{},
		"groups": map[string]any{},
	}
	platforms[platform] = current
	return current
}

func ensureAgentGroupSnapshot(platform map[string]any, groupID string) map[string]any {
	groups, _ := platform["groups"].(map[string]any)
	if groups == nil {
		groups = map[string]any{}
		platform["groups"] = groups
	}
	if current, ok := groups[groupID].(map[string]any); ok && current != nil {
		ensureGroupGamesMap(current)
		if _, ok := current["label"]; !ok {
			current["label"] = groupID
		}
		return current
	}
	current := map[string]any{
		"label":   groupID,
		"enabled": true,
		"games":   map[string]any{},
	}
	groups[groupID] = current
	return current
}

func ensureGroupGamesMap(group map[string]any) map[string]any {
	if current, ok := group["games"].(map[string]any); ok && current != nil {
		return current
	}
	current := map[string]any{}
	group["games"] = current
	return current
}

func mergeIntoGameMap(games map[string]any, gameCode string, cfg map[string]any) {
	current, _ := games[gameCode].(map[string]any)
	games[gameCode] = agentSyncMergeMap(current, cfg)
}

func agentSyncJSONMap(v any) map[string]any {
	switch value := v.(type) {
	case map[string]any:
		return agentSyncCopyMap(value)
	case string:
		text := strings.TrimSpace(value)
		if text == "" {
			return map[string]any{}
		}
		var out map[string]any
		if err := json.Unmarshal([]byte(text), &out); err == nil {
			return out
		}
	}
	return map[string]any{}
}

func agentSyncCopyMap(src map[string]any) map[string]any {
	if src == nil {
		return map[string]any{}
	}
	out := make(map[string]any, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}

func agentSyncMergeMap(base, override map[string]any) map[string]any {
	out := agentSyncCopyMap(base)
	for k, v := range override {
		if child, ok := v.(map[string]any); ok {
			if baseChild, ok2 := out[k].(map[string]any); ok2 {
				out[k] = agentSyncMergeMap(baseChild, child)
				continue
			}
		}
		out[k] = v
	}
	return out
}

func mustSyncJSON(v any) string {
	raw, _ := json.Marshal(v)
	return string(raw)
}

func normalizeSyncProvider(provider string) string {
	provider = strings.ToLower(strings.TrimSpace(provider))
	switch provider {
	case "telegram":
		return "tg"
	case "discord", "dc", "hhhl":
		return "community"
	case "":
		return "qq"
	default:
		return provider
	}
}

func normalizeSyncPlatform(platform string) string {
	platform = strings.ToLower(strings.TrimSpace(platform))
	switch platform {
	case "telegram":
		return "tg"
	default:
		return platform
	}
}

func normalizeSyncGroupPlatform(platform string) string {
	platform = normalizeSyncPlatform(platform)
	switch platform {
	case "qq":
		return "qq_group"
	case "tg":
		return "tg_group"
	default:
		return platform
	}
}

func syncAppendPlatform(platforms []string, platform string) []string {
	platform = strings.TrimSpace(platform)
	if platform == "" {
		return platforms
	}
	for _, existing := range platforms {
		if existing == platform {
			return platforms
		}
	}
	return append(platforms, platform)
}

func syncBoolDefault(cfg map[string]any, key string, def bool) bool {
	value, ok := cfg[key]
	if !ok {
		return def
	}
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		lower := strings.ToLower(strings.TrimSpace(typed))
		return lower == "1" || lower == "true" || lower == "yes" || lower == "on"
	case float64:
		return typed != 0
	case int:
		return typed != 0
	default:
		return def
	}
}

func syncFirstInt(cfg map[string]any, keys ...string) int {
	return syncFirstIntDefault(0, cfg, keys...)
}

func syncFirstIntDefault(def int, cfg map[string]any, keys ...string) int {
	for _, key := range keys {
		if value, ok := cfg[key]; ok {
			switch typed := value.(type) {
			case float64:
				return int(typed)
			case int:
				return typed
			case int64:
				return int(typed)
			case json.Number:
				if i64, err := typed.Int64(); err == nil {
					return int(i64)
				}
			case string:
				var num json.Number = json.Number(strings.TrimSpace(typed))
				if i64, err := num.Int64(); err == nil {
					return int(i64)
				}
			}
		}
	}
	return def
}

func firstSyncNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func groupString(v any) string {
	switch value := v.(type) {
	case string:
		return strings.TrimSpace(value)
	case json.Number:
		return value.String()
	default:
		return strings.TrimSpace(fmt.Sprint(value))
	}
}
