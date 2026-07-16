package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"gorm.io/gorm"
)

type OpsGroupView struct {
	ID                  int               `json:"id"`
	SiteID              string            `json:"site_id"`
	Platform            string            `json:"platform"`
	PlatformFamily      string            `json:"platform_family"`
	GroupID             string            `json:"group_id"`
	GroupName           string            `json:"group_name"`
	InviteTargetGroupID string            `json:"invite_target_group_id"`
	Role                string            `json:"role"`
	Status              string            `json:"status"`
	Enabled             bool              `json:"enabled"`
	Language            string            `json:"language"`
	Timezone            string            `json:"timezone"`
	Config              map[string]any    `json:"config"`
	Capabilities        map[string]any    `json:"capabilities"`
	GameConfigs         []map[string]any  `json:"game_configs"`
	LatestMetrics       map[string]any    `json:"latest_metrics"`
	SourceTables        map[string]string `json:"source_tables"`
	AccessQualifiers    map[string]any    `json:"access_qualifiers"`
	RuntimeConnectors   map[string]any    `json:"runtime_connectors"`
	GeneratedAt         int64             `json:"generated_at"`
}

type OpsGroupChatOpsUpdateRequest struct {
	CheckinEnabled        *bool          `json:"checkin_enabled"`
	VerifyEnabled         *bool          `json:"verify_enabled"`
	InviteEnabled         *bool          `json:"invite_enabled"`
	CheckinQuota          *int           `json:"checkin_quota"`
	VerifyMinQuota        *int           `json:"verify_min_quota"`
	InviteRewardQuota     *int           `json:"invite_reward_quota"`
	InviteeRewardQuota    *int           `json:"invitee_reward_quota"`
	DailyGroupRewardLimit *int           `json:"daily_group_reward_limit"`
	Rule                  map[string]any `json:"rule"`
}

type OpsGroupGameUpdateItem struct {
	GameCode   string         `json:"game_code"`
	Enabled    *bool          `json:"enabled"`
	BudgetPool string         `json:"budget_pool"`
	Rule       map[string]any `json:"rule"`
}

type OpsGroupGamesUpdateRequest struct {
	Games []OpsGroupGameUpdateItem `json:"games"`
}

type OpsGroupSaveRequest struct {
	SiteID              string         `json:"site_id"`
	Platform            *string        `json:"platform"`
	GroupID             *string        `json:"group_id"`
	GroupName           *string        `json:"group_name"`
	InviteTargetGroupID *string        `json:"invite_target_group_id"`
	Role                *string        `json:"role"`
	Status              *string        `json:"status"`
	Language            *string        `json:"language"`
	Timezone            *string        `json:"timezone"`
	Config              map[string]any `json:"config"`
	CopyChatOps         *bool          `json:"copy_chatops"`
	CopyGameRule        *bool          `json:"copy_game_rule"`
}

func ListOpsGroups(siteID string, platform string, role string, status string) ([]OpsGroupView, error) {
	resolvedSiteID := resolveOpsSiteID(siteID)
	rows, err := model.ListChatGroupsBySite(resolvedSiteID, platform, role, status)
	if err != nil {
		return nil, err
	}
	out := make([]OpsGroupView, 0, len(rows))
	for _, row := range rows {
		view, viewErr := buildOpsGroupView(resolvedSiteID, &row)
		if viewErr != nil {
			return nil, viewErr
		}
		out = append(out, *view)
	}
	return out, nil
}

func GetOpsGroup(siteID string, id int) (*OpsGroupView, error) {
	group, err := model.GetChatGroupByID(resolveOpsSiteID(siteID), id)
	if err != nil {
		return nil, err
	}
	return buildOpsGroupView(resolveOpsSiteID(siteID), group)
}

func CreateOpsGroup(siteID string, req OpsGroupSaveRequest) (*OpsGroupView, error) {
	resolvedSiteID := resolveOpsGroupRequestSiteID(siteID, req.SiteID)
	row, err := buildChatGroupFromCreateRequest(resolvedSiteID, req)
	if err != nil {
		return nil, err
	}
	if err := model.CreateChatGroup(row); err != nil {
		return nil, err
	}
	return buildOpsGroupView(resolvedSiteID, row)
}

func UpdateOpsGroup(siteID string, id int, req OpsGroupSaveRequest) (*OpsGroupView, error) {
	resolvedSiteID := resolveOpsGroupRequestSiteID(siteID, req.SiteID)
	row, err := model.GetChatGroupByID(resolvedSiteID, id)
	if err != nil {
		return nil, err
	}
	if err := applyOpsGroupSaveRequest(row, req, false); err != nil {
		return nil, err
	}
	if err := model.UpdateChatGroup(row); err != nil {
		return nil, err
	}
	return buildOpsGroupView(resolvedSiteID, row)
}

func CloneOpsGroup(siteID string, id int, req OpsGroupSaveRequest) (*OpsGroupView, error) {
	resolvedSiteID := resolveOpsGroupRequestSiteID(siteID, req.SiteID)
	source, err := model.GetChatGroupByID(resolvedSiteID, id)
	if err != nil {
		return nil, err
	}
	row := &model.ChatGroup{
		SiteId:              resolvedSiteID,
		Platform:            source.Platform,
		GroupName:           source.GroupName,
		InviteTargetGroupId: opsGroupInviteTargetGroupID(source, opsJSONMap(source.ConfigJson)),
		Role:                source.Role,
		Status:              source.Status,
		Language:            source.Language,
		Timezone:            source.Timezone,
		ConfigJson:          source.ConfigJson,
	}
	if req.GroupID == nil || strings.TrimSpace(*req.GroupID) == "" {
		return nil, errors.New("group_id is required when cloning a group")
	}
	if err := applyOpsGroupSaveRequest(row, req, true); err != nil {
		return nil, err
	}
	copyChatOps := req.CopyChatOps == nil || *req.CopyChatOps
	copyGameRule := req.CopyGameRule == nil || *req.CopyGameRule
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		if err := model.CreateChatGroupWithDB(tx, row); err != nil {
			return err
		}
		if copyChatOps {
			if cfg, err := model.GetGroupChatOpsConfigByGroupWithDB(tx, resolvedSiteID, source.Platform, source.GroupId); err == nil && cfg != nil {
				cloned := *cfg
				cloned.Id = 0
				cloned.Platform = row.Platform
				cloned.GroupId = row.GroupId
				if err := model.UpsertGroupChatOpsConfigWithDB(tx, &cloned); err != nil {
					return err
				}
			} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
		}
		if copyGameRule {
			games, err := model.ListGroupGameConfigsByGroupWithDB(tx, resolvedSiteID, source.Platform, source.GroupId)
			if err != nil {
				return err
			}
			for _, game := range games {
				cloned := game
				cloned.Id = 0
				cloned.Platform = row.Platform
				cloned.GroupId = row.GroupId
				if err := model.UpsertGroupGameConfigWithDB(tx, &cloned); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return buildOpsGroupView(resolvedSiteID, row)
}

func GetOpsGroupEffective(siteID string, id int) (map[string]any, error) {
	view, err := GetOpsGroup(siteID, id)
	if err != nil {
		return nil, err
	}
	impact, err := GetOpsGroupImpactPreview(siteID, id)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"site_id":        view.SiteID,
		"group":          view,
		"impact_preview": impact,
		"generated_at":   time.Now().Unix(),
	}, nil
}

func buildChatGroupFromCreateRequest(siteID string, req OpsGroupSaveRequest) (*model.ChatGroup, error) {
	row := &model.ChatGroup{
		SiteId:   siteID,
		Status:   "active",
		Language: "zh-CN",
		Timezone: "Asia/Shanghai",
	}
	if err := applyOpsGroupSaveRequest(row, req, true); err != nil {
		return nil, err
	}
	if row.GroupName == "" {
		row.GroupName = row.GroupId
	}
	return row, nil
}

func applyOpsGroupSaveRequest(row *model.ChatGroup, req OpsGroupSaveRequest, requireIdentity bool) error {
	if row == nil {
		return errors.New("group is nil")
	}
	if req.Platform != nil {
		row.Platform = normalizeOpsGroupPlatform(*req.Platform)
	}
	if req.GroupID != nil {
		row.GroupId = strings.TrimSpace(*req.GroupID)
	}
	if requireIdentity {
		if strings.TrimSpace(row.Platform) == "" {
			return errors.New("platform is required")
		}
		if strings.TrimSpace(row.GroupId) == "" {
			return errors.New("group_id is required")
		}
	}
	if req.GroupName != nil {
		row.GroupName = strings.TrimSpace(*req.GroupName)
	}
	explicitInviteTargetProvided := req.InviteTargetGroupID != nil
	if explicitInviteTargetProvided {
		row.InviteTargetGroupId = strings.TrimSpace(*req.InviteTargetGroupID)
	}
	if req.Role != nil {
		row.Role = normalizeOpsGroupRole(*req.Role)
	}
	if req.Status != nil {
		row.Status = normalizeOpsGroupStatus(*req.Status)
	}
	if req.Language != nil {
		row.Language = firstOpsNonEmpty(strings.TrimSpace(*req.Language), "zh-CN")
	}
	if req.Timezone != nil {
		row.Timezone = firstOpsNonEmpty(strings.TrimSpace(*req.Timezone), "Asia/Shanghai")
	}
	if req.Config != nil {
		config := opsCopyMap(req.Config)
		if !explicitInviteTargetProvided {
			if legacyTarget := opsConfigString(config, "invite_target_group_id"); legacyTarget != "" {
				row.InviteTargetGroupId = legacyTarget
			}
		}
		delete(config, "invite_target_group_id")
		b, err := json.Marshal(config)
		if err != nil {
			return fmt.Errorf("invalid config: %w", err)
		}
		row.ConfigJson = string(b)
	}
	row.Platform = normalizeOpsGroupPlatform(row.Platform)
	row.Role = normalizeOpsGroupRole(row.Role)
	row.Status = normalizeOpsGroupStatus(row.Status)
	row.Language = firstOpsNonEmpty(row.Language, "zh-CN")
	row.Timezone = firstOpsNonEmpty(row.Timezone, "Asia/Shanghai")
	return nil
}

func resolveOpsGroupRequestSiteID(querySiteID string, bodySiteID string) string {
	if strings.TrimSpace(querySiteID) != "" {
		return resolveOpsSiteID(querySiteID)
	}
	return resolveOpsSiteID(bodySiteID)
}

func normalizeOpsGroupPlatform(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	switch value {
	case "qq", "qq-group", "qq_group":
		return "qq_group"
	case "tg", "telegram", "tg-group", "tg_group":
		return "tg_group"
	case "community", "community_room", "community-room", "hhhl", "dc", "discord":
		return "community"
	default:
		return value
	}
}

func normalizeOpsGroupRole(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	switch value {
	case "", "secondary":
		return "ops_secondary"
	case "primary", "main", "mainfield", "primary_mainfield":
		return "primary_mainfield"
	case "community", "intake", "community_intake":
		return "community_intake"
	case "campaign":
		return "campaign"
	case "backup":
		return "backup"
	case "manual_whitelist", "manual-whitelist", "whitelist":
		return "manual_whitelist"
	default:
		return value
	}
}

func normalizeOpsGroupStatus(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	switch value {
	case "", "enabled", "active":
		return "active"
	case "disabled", "inactive":
		return "disabled"
	case "archived":
		return "archived"
	default:
		return value
	}
}

func GetOpsGroupImpactPreview(siteID string, id int) (map[string]any, error) {
	group, err := model.GetChatGroupByID(resolveOpsSiteID(siteID), id)
	if err != nil {
		return nil, err
	}
	accessCfg := operation_setting.GetAccessControlSetting()
	communityCfg := operation_setting.GetCommunityGateSetting()
	agentCfg := operation_setting.GetAgentSetting()
	family := opsPlatformFamily(group.Platform)
	primaryGroupIDs, communityGroupIDs := opsAccessPolicyResolvedBindingGroups(siteID, accessCfg)
	primaryPlatform := ""
	if accessCfg != nil {
		primaryPlatform = accessCfg.PrimaryPlatform
	}
	primaryGroup := accessCfg != nil && family == strings.TrimSpace(strings.ToLower(primaryPlatform)) && opsStringInList(group.GroupId, primaryGroupIDs)
	communityGroup := opsStringInList(group.GroupId, communityGroupIDs)
	requiredRoomIDs := opsCommunityGateRoomIDs(communityCfg)
	return map[string]any{
		"site_id":         resolveOpsSiteID(siteID),
		"group_id":        group.GroupId,
		"platform":        group.Platform,
		"platform_family": family,
		"access_control": map[string]any{
			"primary_platform":            primaryPlatform,
			"primary_group_ids":           primaryGroupIDs,
			"community_group_ids":         communityGroupIDs,
			"community_only_groups":       accessCfg.CommunityOnlyGroups,
			"full_access_groups":          accessCfg.FullAccessGroups,
			"paid_bypass_groups":          accessCfg.PaidBypassGroups,
			"paid_user_groups":            accessCfg.PaidUserGroups,
			"qualifies_primary_binding":   primaryGroup,
			"qualifies_community_binding": communityGroup || opsStringInList(group.GroupId, requiredRoomIDs),
		},
		"community_gate": map[string]any{
			"provider_slug":           communityCfg.ProviderSlug,
			"community_host":          communityCfg.CommunityHost,
			"required_room_ids":       requiredRoomIDs,
			"room_match_mode":         communityCfg.RoomMatchMode,
			"matches_required_room":   opsStringInList(group.GroupId, requiredRoomIDs),
			"require_oauth_binding":   communityCfg.RequireOAuthBinding,
			"require_room_membership": communityCfg.RequireRoomMembership,
			"block_token_when_denied": communityCfg.BlockTokenWhenNotCompliant,
			"token_disable_mode":      communityCfg.TokenDisableMode,
		},
		"runtime_connectors": map[string]any{
			"qq_group_id":               strings.TrimSpace(agentCfg.QQGroupID),
			"tg_chat_id":                strings.TrimSpace(agentCfg.TGChatID),
			"community_room_id":         strings.TrimSpace(agentCfg.CommunityRoomID),
			"matches_runtime_connector": opsGroupMatchesRuntimeConnector(group, agentCfg),
		},
		"generated_at": time.Now().Unix(),
	}, nil
}

func SaveOpsGroupChatOps(siteID string, id int, req OpsGroupChatOpsUpdateRequest) (*OpsGroupView, error) {
	resolvedSiteID := resolveOpsSiteID(siteID)
	group, err := model.GetChatGroupByID(resolvedSiteID, id)
	if err != nil {
		return nil, err
	}
	current, err := model.GetGroupChatOpsConfigByGroup(resolvedSiteID, group.Platform, group.GroupId)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	row := &model.GroupChatOpsConfig{
		SiteId:                resolvedSiteID,
		Platform:              group.Platform,
		GroupId:               group.GroupId,
		CheckinEnabled:        true,
		VerifyEnabled:         true,
		InviteEnabled:         true,
		VerifyMinQuota:        1,
		DailyGroupRewardLimit: 0,
	}
	if current != nil {
		*row = *current
	}
	if req.CheckinEnabled != nil {
		row.CheckinEnabled = *req.CheckinEnabled
	}
	if req.VerifyEnabled != nil {
		row.VerifyEnabled = *req.VerifyEnabled
	}
	if req.InviteEnabled != nil {
		row.InviteEnabled = *req.InviteEnabled
	}
	if req.CheckinQuota != nil {
		row.CheckinQuota = maxOpsNonNegative(*req.CheckinQuota)
	}
	if req.VerifyMinQuota != nil {
		row.VerifyMinQuota = maxOpsNonNegative(*req.VerifyMinQuota)
	}
	if req.InviteRewardQuota != nil {
		row.InviteRewardQuota = maxOpsNonNegative(*req.InviteRewardQuota)
	}
	if req.InviteeRewardQuota != nil {
		row.InviteeRewardQuota = maxOpsNonNegative(*req.InviteeRewardQuota)
	}
	if req.DailyGroupRewardLimit != nil {
		row.DailyGroupRewardLimit = maxOpsNonNegative(*req.DailyGroupRewardLimit)
	}
	if req.Rule != nil {
		b, _ := json.Marshal(req.Rule)
		row.RuleJson = string(b)
	}
	if err := model.UpsertGroupChatOpsConfig(row); err != nil {
		return nil, err
	}
	return buildOpsGroupView(resolvedSiteID, group)
}

func SaveOpsGroupGames(siteID string, id int, req OpsGroupGamesUpdateRequest) (*OpsGroupView, error) {
	resolvedSiteID := resolveOpsSiteID(siteID)
	group, err := model.GetChatGroupByID(resolvedSiteID, id)
	if err != nil {
		return nil, err
	}
	if len(req.Games) == 0 {
		return nil, errors.New("at least one game rule is required")
	}
	rows := make([]model.GroupGameConfig, 0, len(req.Games))
	seen := make(map[string]struct{}, len(req.Games))
	for _, item := range req.Games {
		gameCode := strings.TrimSpace(strings.ToLower(item.GameCode))
		if gameCode == "" {
			return nil, errors.New("game_code is required")
		}
		if _, exists := seen[gameCode]; exists {
			return nil, fmt.Errorf("duplicate game_code: %s", gameCode)
		}
		seen[gameCode] = struct{}{}
		budgetPool := strings.TrimSpace(strings.ToLower(item.BudgetPool))
		if budgetPool == "" {
			budgetPool = "game"
		}
		if err := model.ValidateOpsPoolType(budgetPool); err != nil {
			return nil, fmt.Errorf("invalid budget_pool for %s: %w", gameCode, err)
		}
		enabled := true
		if item.Enabled != nil {
			enabled = *item.Enabled
		}
		ruleJSON := "{}"
		if item.Rule != nil {
			if b, marshalErr := json.Marshal(item.Rule); marshalErr == nil {
				ruleJSON = string(b)
			} else {
				return nil, fmt.Errorf("invalid game rule for %s: %w", gameCode, marshalErr)
			}
		}
		rows = append(rows, model.GroupGameConfig{
			SiteId:     resolvedSiteID,
			Platform:   group.Platform,
			GroupId:    group.GroupId,
			GameCode:   gameCode,
			Enabled:    enabled,
			BudgetPool: budgetPool,
			RuleJson:   ruleJSON,
		})
	}
	if err := model.DB.Transaction(func(tx *gorm.DB) error {
		for index := range rows {
			if err := model.UpsertGroupGameConfigWithDB(tx, &rows[index]); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return buildOpsGroupView(resolvedSiteID, group)
}

func buildOpsGroupView(siteID string, group *model.ChatGroup) (*OpsGroupView, error) {
	if group == nil {
		return nil, errors.New("group is nil")
	}
	config := opsJSONMap(group.ConfigJson)
	inviteTargetGroupID := opsGroupInviteTargetGroupID(group, config)
	if inviteTargetGroupID != "" {
		config["invite_target_group_id"] = inviteTargetGroupID
	} else {
		delete(config, "invite_target_group_id")
	}
	chatopsCfg, chatopsErr := model.GetGroupChatOpsConfigByGroup(siteID, group.Platform, group.GroupId)
	if chatopsErr != nil && !errors.Is(chatopsErr, gorm.ErrRecordNotFound) {
		return nil, chatopsErr
	}
	gameCfgs, err := model.ListGroupGameConfigsByGroup(siteID, group.Platform, group.GroupId)
	if err != nil {
		return nil, err
	}
	metrics, metricsErr := model.GetLatestGroupMetricsDaily(siteID, group.Platform, group.GroupId)
	if metricsErr != nil && !errors.Is(metricsErr, gorm.ErrRecordNotFound) {
		return nil, metricsErr
	}
	impact, err := GetOpsGroupImpactPreview(siteID, group.Id)
	if err != nil {
		return nil, err
	}
	view := &OpsGroupView{
		ID:                  group.Id,
		SiteID:              siteID,
		Platform:            group.Platform,
		PlatformFamily:      opsPlatformFamily(group.Platform),
		GroupID:             group.GroupId,
		GroupName:           firstOpsNonEmpty(strings.TrimSpace(group.GroupName), group.GroupId),
		InviteTargetGroupID: inviteTargetGroupID,
		Role:                group.Role,
		Status:              group.Status,
		Enabled:             strings.TrimSpace(group.Status) != "disabled",
		Language:            group.Language,
		Timezone:            group.Timezone,
		Config:              config,
		GameConfigs:         opsGameConfigList(gameCfgs),
		SourceTables: map[string]string{
			"group":   "chat_groups",
			"chatops": "group_chatops_configs",
			"games":   "group_game_configs",
			"metrics": "group_metrics_daily",
		},
		AccessQualifiers:  opsMapAny(impact["access_control"]),
		RuntimeConnectors: opsMapAny(impact["runtime_connectors"]),
		GeneratedAt:       time.Now().Unix(),
	}
	if chatopsCfg != nil {
		view.Capabilities = map[string]any{
			"checkin_enabled":          chatopsCfg.CheckinEnabled,
			"verify_enabled":           chatopsCfg.VerifyEnabled,
			"invite_enabled":           chatopsCfg.InviteEnabled,
			"checkin_quota":            chatopsCfg.CheckinQuota,
			"verify_min_quota":         chatopsCfg.VerifyMinQuota,
			"invite_reward_quota":      chatopsCfg.InviteRewardQuota,
			"invitee_reward_quota":     chatopsCfg.InviteeRewardQuota,
			"daily_group_reward_limit": chatopsCfg.DailyGroupRewardLimit,
			"rule":                     opsJSONMap(chatopsCfg.RuleJson),
		}
	} else {
		view.Capabilities = map[string]any{
			"configured": false,
		}
	}
	if metrics != nil {
		view.LatestMetrics = map[string]any{
			"metric_date":       metrics.MetricDate,
			"invite_links":      metrics.InviteLinks,
			"joins":             metrics.Joins,
			"binds":             metrics.Binds,
			"verifies":          metrics.Verifies,
			"checkins":          metrics.Checkins,
			"game_players":      metrics.GamePlayers,
			"game_rounds":       metrics.GameRounds,
			"stake_quota":       metrics.StakeQuota,
			"payout_quota":      metrics.PayoutQuota,
			"commission_quota":  metrics.CommissionQuota,
			"reward_cost_quota": metrics.RewardCostQuota,
			"metadata":          opsJSONMap(metrics.MetadataJson),
		}
	} else {
		view.LatestMetrics = map[string]any{}
	}
	return view, nil
}

func maxOpsNonNegative(v int) int {
	if v < 0 {
		return 0
	}
	return v
}

func opsGameConfigList(rows []model.GroupGameConfig) []map[string]any {
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		out = append(out, map[string]any{
			"game_code":   row.GameCode,
			"enabled":     row.Enabled,
			"budget_pool": row.BudgetPool,
			"rule":        opsJSONMap(row.RuleJson),
			"updated_at":  row.UpdatedAt,
			"created_at":  row.CreatedAt,
		})
	}
	return out
}

func resolveOpsSiteID(explicit string) string {
	explicit = strings.TrimSpace(explicit)
	if explicit == "" {
		return AgentSiteID()
	}
	return model.CanonicalSiteID(explicit)
}

func opsJSONMap(raw string) map[string]any {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return map[string]any{}
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return map[string]any{"raw": raw}
	}
	if out == nil {
		return map[string]any{}
	}
	return out
}

func opsGroupInviteTargetGroupID(group *model.ChatGroup, config map[string]any) string {
	if group != nil {
		if value := strings.TrimSpace(group.InviteTargetGroupId); value != "" {
			return value
		}
	}
	return opsConfigString(config, "invite_target_group_id")
}

func opsConfigString(config map[string]any, key string) string {
	if len(config) == 0 {
		return ""
	}
	return opsNormalizedConfigString(config[key])
}

func opsNormalizedConfigString(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(typed)
	default:
		return strings.TrimSpace(fmt.Sprint(typed))
	}
}

func opsCopyMap(input map[string]any) map[string]any {
	if len(input) == 0 {
		return map[string]any{}
	}
	out := make(map[string]any, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

func opsPlatformFamily(platform string) string {
	platform = strings.TrimSpace(strings.ToLower(platform))
	switch platform {
	case "qq_group":
		return "qq"
	case "tg_group", "telegram", "tg":
		return "tg"
	case "discord", "dc", "hhhl":
		return "community"
	default:
		return platform
	}
}

func opsStringInList(value string, items []string) bool {
	value = strings.TrimSpace(value)
	for _, item := range items {
		if value != "" && value == strings.TrimSpace(item) {
			return true
		}
	}
	return false
}

func opsCommunityGateRoomIDs(cfg *operation_setting.CommunityGateSetting) []string {
	if cfg == nil {
		return []string{}
	}
	out := make([]string, 0, len(cfg.RoomIDs)+1)
	seen := map[string]struct{}{}
	appendOne := func(v string) {
		v = strings.TrimSpace(v)
		if v == "" {
			return
		}
		if _, ok := seen[v]; ok {
			return
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	appendOne(cfg.RoomID)
	for _, roomID := range cfg.RoomIDs {
		appendOne(roomID)
	}
	return out
}

func opsGroupMatchesRuntimeConnector(group *model.ChatGroup, agentCfg *operation_setting.AgentSetting) bool {
	if group == nil || agentCfg == nil {
		return false
	}
	switch opsPlatformFamily(group.Platform) {
	case "qq":
		return strings.TrimSpace(group.GroupId) != "" && strings.TrimSpace(group.GroupId) == strings.TrimSpace(agentCfg.QQGroupID)
	case "tg":
		return strings.TrimSpace(group.GroupId) != "" && strings.TrimSpace(group.GroupId) == strings.TrimSpace(agentCfg.TGChatID)
	case "community":
		return strings.TrimSpace(group.GroupId) != "" && strings.TrimSpace(group.GroupId) == strings.TrimSpace(agentCfg.CommunityRoomID)
	default:
		return false
	}
}

func firstOpsNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func opsMapAny(value any) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	if out, ok := value.(map[string]any); ok && out != nil {
		return out
	}
	return map[string]any{}
}
