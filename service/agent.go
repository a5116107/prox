package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"gorm.io/gorm"
)

type AgentTool struct {
	Name             string `json:"name"`
	Title            string `json:"title"`
	Category         string `json:"category"`
	Description      string `json:"description"`
	ApprovalRequired bool   `json:"approval_required"`
	RiskLevel        string `json:"risk_level"`
}

type AgentChannelSummary struct {
	Total             int `json:"total"`
	Enabled           int `json:"enabled"`
	ManuallyDisabled  int `json:"manually_disabled"`
	AutoDisabled      int `json:"auto_disabled"`
	AverageResponseMS int `json:"avg_response_ms"`
}

type AgentGroupState struct {
	Name             string   `json:"name"`
	Ratio            float64  `json:"ratio"`
	Available        bool     `json:"available"`
	Models           []string `json:"models"`
	HealthyChannels  int      `json:"healthy_channels"`
	DisabledChannels int      `json:"disabled_channels"`
	RecentErrorCount int      `json:"recent_error_count"`
}

type AgentRecentError struct {
	Id        int    `json:"id"`
	CreatedAt int64  `json:"created_at"`
	Content   string `json:"content"`
	ModelName string `json:"model_name"`
	Group     string `json:"group"`
	ChannelId int    `json:"channel_id"`
	RequestId string `json:"request_id"`
}

type AgentSiteState struct {
	SiteId       string              `json:"site_id"`
	SiteName     string              `json:"site_name"`
	SystemName   string              `json:"system_name"`
	Community    map[string]any      `json:"community"`
	Bots         map[string]any      `json:"bots"`
	Channels     AgentChannelSummary `json:"channels"`
	Groups       []AgentGroupState   `json:"groups"`
	RecentErrors []AgentRecentError  `json:"recent_errors"`
	GeneratedAt  int64               `json:"generated_at"`
}

type AgentDashboard struct {
	Setting     map[string]any              `json:"setting"`
	SiteState   *AgentSiteState             `json:"site_state"`
	Budgets     []model.AgentBudgetPool     `json:"budgets"`
	Actions     []model.AgentAction         `json:"actions"`
	Tasks       []model.AgentTask           `json:"tasks"`
	Approvals   []model.AgentActionApproval `json:"approvals"`
	Events      []model.AgentEvent          `json:"events"`
	Risks       []model.AgentRiskProfile    `json:"risks"`
	Tools       []AgentTool                 `json:"tools"`
	GeneratedAt int64                       `json:"generated_at"`
}

type AgentRiskEvaluation struct {
	UserId      int      `json:"user_id"`
	Score       int      `json:"score"`
	Level       string   `json:"level"`
	Decision    string   `json:"decision"`
	Reasons     []string `json:"reasons"`
	EvaluatedAt int64    `json:"evaluated_at"`
}

type AgentActionRequest struct {
	ActionType     string         `json:"action_type"`
	AgentName      string         `json:"agent_name"`
	TargetType     string         `json:"target_type"`
	TargetId       string         `json:"target_id"`
	UserId         int            `json:"user_id"`
	QuotaAmount    int            `json:"quota_amount"`
	BudgetPool     string         `json:"budget_pool"`
	Reason         string         `json:"reason"`
	Payload        map[string]any `json:"payload"`
	IdempotencyKey string         `json:"idempotency_key"`
	ForceApproval  bool           `json:"force_approval"`
}

func AgentSiteID() string {
	cfg := operation_setting.GetAgentSetting()
	return model.CanonicalSiteID(cfg.SiteID)
}

func agentSettingPublicMap() map[string]any {
	cfg := operation_setting.GetAgentSetting()
	communityCfg := operation_setting.GetCommunityBotSetting()
	return map[string]any{
		"enabled":                           cfg.Enabled,
		"site_id":                           AgentSiteID(),
		"site_name":                         cfg.SiteName,
		"public_base_url":                   cfg.PublicBaseURL,
		"api_base_url":                      cfg.APIBaseURL,
		"llm_provider":                      cfg.LLMProvider,
		"llm_model":                         cfg.LLMModel,
		"planner_provider":                  cfg.PlannerProvider,
		"hermes_base_url_configured":        strings.TrimSpace(cfg.HermesBaseURL) != "",
		"hermes_api_key_configured":         strings.TrimSpace(cfg.HermesAPIKey) != "",
		"llm_base_url_configured":           strings.TrimSpace(cfg.LLMBaseURL) != "",
		"llm_api_key_configured":            strings.TrimSpace(cfg.LLMAPIKey) != "",
		"director_enabled":                  cfg.DirectorEnabled,
		"community_enabled":                 cfg.CommunityEnabled,
		"growth_enabled":                    cfg.GrowthEnabled,
		"activity_enabled":                  cfg.ActivityEnabled,
		"game_enabled":                      cfg.GameEnabled,
		"risk_enabled":                      cfg.RiskEnabled,
		"ops_enabled":                       cfg.OpsEnabled,
		"budget_enabled":                    cfg.BudgetEnabled,
		"auto_execute_low_risk":             cfg.AutoExecuteLowRisk,
		"human_approval_enabled":            cfg.HumanApprovalEnabled,
		"daily_budget_quota":                cfg.DailyBudgetQuota,
		"growth_budget_quota":               cfg.GrowthBudgetQuota,
		"activity_budget_quota":             cfg.ActivityBudgetQuota,
		"game_budget_quota":                 cfg.GameBudgetQuota,
		"ops_comp_budget_quota":             cfg.OpsCompBudgetQuota,
		"community_budget_quota":            cfg.CommunityBudgetQuota,
		"daily_budget_reset_enabled":        cfg.DailyBudgetResetEnabled,
		"daily_fund_reset_enabled":          cfg.DailyFundResetEnabled,
		"ops_fund_daily_target_quota":       cfg.OpsFundDailyTargetQuota,
		"single_action_limit_quota":         cfg.SingleActionLimitQuota,
		"user_daily_limit_quota":            cfg.UserDailyLimitQuota,
		"approval_threshold_quota":          cfg.ApprovalThresholdQuota,
		"risk_deny_threshold":               cfg.RiskDenyThreshold,
		"risk_review_threshold":             cfg.RiskReviewThreshold,
		"min_message_chars":                 cfg.MinMessageChars,
		"min_distinct_messages":             cfg.MinDistinctMessages,
		"qq_bot_enabled":                    cfg.QQBotEnabled,
		"qq_onebot_url_configured":          strings.TrimSpace(cfg.QQOneBotURL) != "",
		"qq_group_id":                       cfg.QQGroupID,
		"qq_access_token_configured":        strings.TrimSpace(cfg.QQAccessToken) != "",
		"tg_bot_enabled":                    cfg.TGBotEnabled,
		"tg_bot_token_configured":           strings.TrimSpace(cfg.TGBotToken) != "",
		"tg_chat_id":                        cfg.TGChatID,
		"chatops_enabled":                   cfg.ChatOpsEnabled,
		"chatops_webhook_secret_configured": strings.TrimSpace(cfg.ChatOpsWebhookSecret) != "",
		"chatops_admin_external_ids":        cfg.ChatOpsAdminExternalIDs,
		"chatops_command_prefixes":          cfg.ChatOpsCommandPrefixes,
		"chatops_auto_reply":                cfg.ChatOpsAutoReply,
		"chatops_allow_natural_language":    cfg.ChatOpsAllowNaturalLanguage,
		"chatops_require_admin_for_ops":     cfg.ChatOpsRequireAdminForOps,
		"chatops_trust_group_admin":         cfg.ChatOpsTrustGroupAdmin,
		"chatops_group_moderation_enabled":  cfg.ChatOpsGroupModerationEnabled,
		"chatops_allow_direct_moderation":   cfg.ChatOpsAllowDirectModeration,
		"chatops_allow_kick":                cfg.ChatOpsAllowKick,
		"chatops_allow_ban":                 cfg.ChatOpsAllowBan,
		"chatops_max_mute_seconds":          cfg.ChatOpsMaxMuteSeconds,
		"memory_auto_capture_enabled":       cfg.MemoryAutoCaptureEnabled,
		"memory_noise_ttl_seconds":          cfg.MemoryNoiseTTLSeconds,
		"memory_candidate_ttl_seconds":      cfg.MemoryCandidateTTLSeconds,
		"memory_core_ttl_seconds":           cfg.MemoryCoreTTLSeconds,
		"memory_risk_ttl_seconds":           cfg.MemoryRiskTTLSeconds,
		"memory_noise_sample_rate":          cfg.MemoryNoiseSampleRate,
		"community_room_id":                 firstAgentNonEmpty(cfg.CommunityRoomID, communityCfg.RoomID),
		"community_host":                    firstAgentNonEmpty(cfg.CommunityHost, communityCfg.CommunityHost),
		"system_prompt":                     cfg.SystemPrompt,
		"site_knowledge":                    cfg.SiteKnowledge,
		"welcome_template":                  cfg.WelcomeTemplate,
		"activity_policy":                   cfg.ActivityPolicy,
		"risk_policy":                       cfg.RiskPolicy,
	}
}

func EnsureAgentRuntimeDefaults() error {
	siteID := AgentSiteID()
	if _, err := model.EnsureCurrentOpsDailyBudgetCapacity(siteID); err != nil {
		return err
	}
	if err := AgentEnsureDefaultMemories(); err != nil {
		return err
	}
	return nil
}

func GetAgentDashboard() (*AgentDashboard, error) {
	if err := EnsureAgentRuntimeDefaults(); err != nil {
		return nil, err
	}
	siteID := AgentSiteID()
	state, err := GetAgentSiteState()
	if err != nil {
		return nil, err
	}
	budgets, err := model.ListAgentBudgetPools(siteID, model.AgentBusinessDateAt(time.Now()))
	if err != nil {
		return nil, err
	}
	actions, err := model.ListAgentActions(siteID, 20)
	if err != nil {
		return nil, err
	}
	tasks, err := model.ListAgentTasks(siteID, "", 20)
	if err != nil {
		return nil, err
	}
	approvals, err := model.ListAgentApprovals(siteID, "", 20)
	if err != nil {
		return nil, err
	}
	events, err := model.ListAgentEvents(siteID, 20)
	if err != nil {
		return nil, err
	}
	risks, err := model.ListAgentRiskProfiles(siteID, 20)
	if err != nil {
		return nil, err
	}
	return &AgentDashboard{Setting: agentSettingPublicMap(), SiteState: state, Budgets: budgets, Actions: actions, Tasks: tasks, Approvals: approvals, Events: events, Risks: risks, Tools: AgentToolCatalog(), GeneratedAt: time.Now().Unix()}, nil
}

func GetAgentSiteState() (*AgentSiteState, error) {
	var channels []model.Channel
	if err := model.DB.Find(&channels).Error; err != nil {
		return nil, err
	}
	var abilities []model.Ability
	_ = model.DB.Find(&abilities).Error
	groups := map[string]*AgentGroupState{}
	ratioMap := ratio_setting.GetGroupRatioCopy()
	for group, ratio := range ratioMap {
		groups[group] = &AgentGroupState{Name: group, Ratio: ratio, Models: []string{}}
	}
	ensureGroup := func(name string) *AgentGroupState {
		name = strings.TrimSpace(name)
		if name == "" {
			name = "default"
		}
		if groups[name] == nil {
			groups[name] = &AgentGroupState{Name: name, Ratio: ratio_setting.GetGroupRatio(name), Models: []string{}}
		}
		return groups[name]
	}
	modelSet := map[string]map[string]struct{}{}
	for _, ability := range abilities {
		if !ability.Enabled {
			continue
		}
		g := ensureGroup(ability.Group)
		if modelSet[g.Name] == nil {
			modelSet[g.Name] = map[string]struct{}{}
		}
		if strings.TrimSpace(ability.Model) != "" {
			modelSet[g.Name][ability.Model] = struct{}{}
		}
	}
	respSum := 0
	respCount := 0
	for _, ch := range channels {
		if ch.ResponseTime > 0 {
			respSum += ch.ResponseTime
			respCount++
		}
		groupNames := splitAgentCSV(ch.Group)
		if len(groupNames) == 0 {
			groupNames = []string{"default"}
		}
		for _, groupName := range groupNames {
			g := ensureGroup(groupName)
			if ch.Status == common.ChannelStatusEnabled {
				g.HealthyChannels++
			} else {
				g.DisabledChannels++
			}
		}
	}
	summary := AgentChannelSummary{Total: len(channels)}
	for _, ch := range channels {
		switch ch.Status {
		case common.ChannelStatusEnabled:
			summary.Enabled++
		case common.ChannelStatusManuallyDisabled:
			summary.ManuallyDisabled++
		case common.ChannelStatusAutoDisabled:
			summary.AutoDisabled++
		}
	}
	if respCount > 0 {
		summary.AverageResponseMS = respSum / respCount
	}
	recentErrors, errorCounts := getAgentRecentErrors()
	for group, count := range errorCounts {
		ensureGroup(group).RecentErrorCount = count
	}
	rows := make([]AgentGroupState, 0, len(groups))
	for name, g := range groups {
		if set := modelSet[name]; len(set) > 0 {
			g.Models = make([]string, 0, len(set))
			for m := range set {
				g.Models = append(g.Models, m)
			}
			sort.Strings(g.Models)
		}
		g.Available = g.HealthyChannels > 0 && len(g.Models) > 0
		rows = append(rows, *g)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Name < rows[j].Name })
	cfg := operation_setting.GetAgentSetting()
	communityCfg := operation_setting.GetCommunityBotSetting()
	return &AgentSiteState{
		SiteId:     AgentSiteID(),
		SiteName:   firstAgentNonEmpty(cfg.SiteName, common.SystemName),
		SystemName: common.SystemName,
		Community: map[string]any{
			"host":                  firstAgentNonEmpty(cfg.CommunityHost, communityCfg.CommunityHost),
			"room_id":               firstAgentNonEmpty(cfg.CommunityRoomID, communityCfg.RoomID),
			"community_bot_enabled": communityCfg.Enabled,
			"auto_invite_enabled":   communityCfg.AutoInviteEnabled,
		},
		Bots: map[string]any{
			"qq_enabled":             cfg.QQBotEnabled,
			"qq_group_id":            cfg.QQGroupID,
			"tg_enabled":             cfg.TGBotEnabled,
			"tg_chat_id":             cfg.TGChatID,
			"community_bot_user_id":  communityCfg.BotUserID,
			"community_bot_username": communityCfg.BotUsername,
		},
		Channels:     summary,
		Groups:       rows,
		RecentErrors: recentErrors,
		GeneratedAt:  time.Now().Unix(),
	}, nil
}

func getAgentRecentErrors() ([]AgentRecentError, map[string]int) {
	logs := make([]model.Log, 0)
	errorCounts := map[string]int{}
	if model.LOG_DB == nil {
		return nil, errorCounts
	}
	err := model.LOG_DB.Model(&model.Log{}).Where("type = ?", model.LogTypeError).Order("created_at desc").Limit(20).Find(&logs).Error
	if err != nil {
		return nil, errorCounts
	}
	rows := make([]AgentRecentError, 0, len(logs))
	for _, l := range logs {
		group := strings.TrimSpace(l.Group)
		if group == "" {
			group = "default"
		}
		errorCounts[group]++
		rows = append(rows, AgentRecentError{Id: l.Id, CreatedAt: l.CreatedAt, Content: truncateAgentText(l.Content, 500), ModelName: l.ModelName, Group: group, ChannelId: l.ChannelId, RequestId: l.RequestId})
	}
	return rows, errorCounts
}

func AgentToolCatalog() []AgentTool {
	return []AgentTool{
		{Name: "site.state.read", Title: "读取站点状态", Category: "ops", Description: "读取分组、渠道、模型、近期错误、社区和机器人连接状态。", ApprovalRequired: false, RiskLevel: "low"},
		{Name: "site.logs.read", Title: "读取站点日志", Category: "ops", Description: "管理员读取最近站点日志，支持错误/系统/管理/消费/登录等类型和时间窗口过滤。", ApprovalRequired: false, RiskLevel: "low"},
		{Name: "chatops.logs.read", Title: "读取群聊运营日志", Category: "chatops", Description: "管理员读取 QQ/TG/社区 ChatOps 相关日志与失败原因。", ApprovalRequired: false, RiskLevel: "low"},
		{Name: "budget.check", Title: "检查预算", Category: "budget", Description: "读取今日活动、游戏、运营补偿等预算池使用情况。", ApprovalRequired: false, RiskLevel: "low"},
		{Name: "reward.grant.small", Title: "小额奖励发放", Category: "reward", Description: "按预算策略创建小额奖励动作；管理员私聊视为已授权执行。", ApprovalRequired: false, RiskLevel: "medium"},
		{Name: "reward.settlement.batch", Title: "批量结算", Category: "reward", Description: "原子批量结算多用户额度变化（游戏收支），整体成功或失败。", ApprovalRequired: false, RiskLevel: "medium"},
		{Name: "quiz.state.load", Title: "读取答题轮次", Category: "quiz", Description: "读取每日答题当前开放轮次、用户作答状态与当日轮次数。", ApprovalRequired: false, RiskLevel: "low"},
		{Name: "quiz.state.commit", Title: "提交答题轮次", Category: "quiz", Description: "提交每日答题轮次/作答状态，并在答对时与奖励发放原子落表。", ApprovalRequired: false, RiskLevel: "medium"},
		{Name: "quiz.question.draw", Title: "抽取题库题目", Category: "quiz", Description: "从已发布的数据库题库原子抽题并创建轮次，不向机器人暴露正确答案。", ApprovalRequired: false, RiskLevel: "low"},
		{Name: "quiz.round.load", Title: "恢复题库轮次", Category: "quiz", Description: "恢复当前用户或群组的开放答题轮次与作答次数。", ApprovalRequired: false, RiskLevel: "low"},
		{Name: "quiz.answer.submit", Title: "提交题库答案", Category: "quiz", Description: "在数据库事务中判题、累计次数、关闭轮次并幂等奖励。", ApprovalRequired: false, RiskLevel: "medium"},
		{Name: "fund.report.read", Title: "基金报表", Category: "budget", Description: "读取运营基金余额与近期流水。", ApprovalRequired: false, RiskLevel: "low"},
		{Name: "fund.topup", Title: "基金注资/调账", Category: "budget", Description: "管理员调整运营基金余额。仅 admin source。", ApprovalRequired: false, RiskLevel: "high"},
		{Name: "user.quota.read", Title: "读取用户余额", Category: "user", Description: "读取指定用户当前额度余额。", ApprovalRequired: false, RiskLevel: "low"},
		{Name: "agent.model.manage", Title: "Agent模型管理", Category: "agent", Description: "查看可用模型并切换 Agent/Hermes 使用模型。", ApprovalRequired: false, RiskLevel: "low"},
		{Name: "agent.skill.install", Title: "安装外部 Skill", Category: "agent", Description: "管理员从 GitHub 受控安装外部 skill 仓库到 Agent 技能区。", ApprovalRequired: false, RiskLevel: "medium"},
		{Name: "agent.skill.list", Title: "查看已装 Skill", Category: "agent", Description: "管理员查看当前已经安装的外部 skill 列表。", ApprovalRequired: false, RiskLevel: "low"},
		{Name: "risk.evaluate", Title: "风控评估", Category: "risk", Description: "对指定用户或事件进行风控评分并写入风控档案。", ApprovalRequired: false, RiskLevel: "low"},
		{Name: "task.chatops.record", Title: "记录 ChatOps 任务", Category: "chatops", Description: "记录来自 QQ/TG/社区的自然语言任务，等待后续规划或人工处理。", ApprovalRequired: false, RiskLevel: "low"},
		{Name: "message.community.send", Title: "社区群消息", Category: "community", Description: "通过社区机器人向房间发送运营通知。", ApprovalRequired: false, RiskLevel: "low"},
		{Name: "message.qq.send", Title: "QQ群消息", Category: "qq", Description: "通过 OneBot/LLoneBot 发送 QQ 群运营消息。", ApprovalRequired: false, RiskLevel: "low"},
		{Name: "message.tg.send", Title: "TG群消息", Category: "telegram", Description: "通过 Telegram Bot 发送 TG 群运营消息。", ApprovalRequired: false, RiskLevel: "low"},
		{Name: "group.message.delete", Title: "撤回群消息", Category: "moderation", Description: "管理员自然语言触发，撤回 QQ/TG 群内指定消息。", ApprovalRequired: false, RiskLevel: "medium"},
		{Name: "group.member.mute", Title: "禁言群成员", Category: "moderation", Description: "管理员自然语言触发，对 QQ/TG 群成员临时禁言，时长受后台上限控制。", ApprovalRequired: false, RiskLevel: "medium"},
		{Name: "group.member.unmute", Title: "解除禁言", Category: "moderation", Description: "管理员自然语言触发，解除 QQ/TG 群成员禁言。", ApprovalRequired: false, RiskLevel: "medium"},
		{Name: "group.member.kick", Title: "移出群成员", Category: "moderation", Description: "管理员自然语言触发，将 QQ/TG 群成员移出群组。", ApprovalRequired: false, RiskLevel: "high"},
		{Name: "group.member.ban", Title: "封禁群成员", Category: "moderation", Description: "管理员自然语言触发，QQ 侧踢出并拒绝再次加群，TG 侧 banChatMember。", ApprovalRequired: false, RiskLevel: "high"},
		{Name: "group.member.unban", Title: "解除封禁", Category: "moderation", Description: "管理员自然语言触发，TG 侧解除封禁；QQ 侧返回明确不支持原因。", ApprovalRequired: false, RiskLevel: "high"},
		{Name: "group.member.lookup", Title: "查询群成员", Category: "moderation", Description: "查询 QQ/TG 群成员资料、角色和状态。", ApprovalRequired: false, RiskLevel: "low"},
		{Name: "group.admin.lookup", Title: "查询群管理员", Category: "moderation", Description: "查询 QQ/TG 群管理员列表。", ApprovalRequired: false, RiskLevel: "low"},
		{Name: "admin.notice.publish", Title: "发布站点公告", Category: "admin", Description: "生成或发布站点公告，默认需要人工审批。", ApprovalRequired: true, RiskLevel: "high"},
		{Name: "admin.channel.suggest", Title: "渠道运维建议", Category: "ops", Description: "基于错误率、延迟和渠道状态生成启停或调权建议，默认需要人工审批。", ApprovalRequired: true, RiskLevel: "high"},
	}
}

func AgentRiskEvaluate(userId int, payload map[string]any) (*AgentRiskEvaluation, error) {
	cfg := operation_setting.GetAgentSetting()
	score := 0
	reasons := make([]string, 0)
	if userId <= 0 {
		score += 20
		reasons = append(reasons, "missing_user_id")
	}
	if payload != nil {
		if v, ok := payload["message_count"].(float64); ok && int(v) > 20 {
			score += 15
			reasons = append(reasons, "high_message_count")
		}
		if v, ok := payload["distinct_messages"].(float64); ok && int(v) < cfg.MinDistinctMessages {
			score += 25
			reasons = append(reasons, "low_distinct_messages")
		}
		if v, ok := payload["avg_message_chars"].(float64); ok && int(v) < cfg.MinMessageChars {
			score += 20
			reasons = append(reasons, "short_messages")
		}
		if v, ok := payload["invite_count"].(float64); ok && int(v) >= 5 {
			score += 20
			reasons = append(reasons, "high_invite_count")
		}
	}
	if score > 100 {
		score = 100
	}
	level := "low"
	decision := "allow"
	if score >= cfg.RiskDenyThreshold {
		level = "high"
		decision = "deny"
	} else if score >= cfg.RiskReviewThreshold {
		level = "medium"
		decision = "review"
	}
	if len(reasons) == 0 {
		reasons = append(reasons, "no_risk_signal")
	}
	res := &AgentRiskEvaluation{UserId: userId, Score: score, Level: level, Decision: decision, Reasons: reasons, EvaluatedAt: time.Now().Unix()}
	flags, _ := json.Marshal(reasons)
	_ = model.UpsertAgentRiskProfile(model.AgentRiskProfile{SiteId: AgentSiteID(), UserId: userId, RiskScore: score, RiskLevel: level, Reason: strings.Join(reasons, ","), FlagsJson: string(flags), LastEventAt: time.Now().Unix()})
	_ = model.DB.Create(&model.AgentRiskEvent{SiteId: AgentSiteID(), UserId: userId, RiskType: "manual_evaluate", RiskScore: score, RiskLevel: level, Source: "agent_api", PayloadJson: mustAgentJSON(payload)}).Error
	return res, nil
}

func AgentCreateAction(req AgentActionRequest, reviewerId int) (*model.AgentAction, error) {
	if strings.TrimSpace(req.ActionType) == "" {
		return nil, errors.New("action_type is required")
	}
	cfg := operation_setting.GetAgentSetting()
	tools := AgentToolCatalog()
	tool, ok := findAgentTool(req.ActionType, tools)
	if !ok {
		return nil, fmt.Errorf("unknown agent tool: %s", req.ActionType)
	}
	if req.AgentName == "" {
		req.AgentName = "director"
	}
	if req.BudgetPool == "" {
		req.BudgetPool = "daily"
	}
	payload := req.Payload
	if payload == nil {
		payload = map[string]any{}
	}
	if req.UserId > 0 {
		payload["user_id"] = req.UserId
	}
	if req.ForceApproval {
		payload["force_approval"] = true
	}
	gameAction := agentBoolPayload(payload, "game_action")
	if gameAction && req.ActionType == "reward.grant.small" {
		payload["admin_chatops_authorized"] = true
	}
	risk, _ := AgentRiskEvaluate(req.UserId, payload)
	adminAuthorized := agentBoolPayload(payload, "admin_chatops_authorized")
	forceApproval := req.ForceApproval || agentBoolPayload(payload, "force_approval", "requires_approval", "hermes_requires_approval")
	if adminAuthorized || gameAction {
		forceApproval = false
	}
	approvalRequired := !adminAuthorized && !gameAction && (forceApproval || (cfg.HumanApprovalEnabled && (tool.ApprovalRequired || req.QuotaAmount >= cfg.ApprovalThresholdQuota || (risk != nil && risk.Decision != "allow"))))
	forceExecute := agentBoolPayload(payload, "force_execute") || adminAuthorized || gameAction
	status := "pending"
	result := map[string]any{"mode": "control_plane", "message": "action recorded for agent workflow"}
	if !approvalRequired {
		status = "queued"
		result = map[string]any{"mode": "executor", "message": "action queued for guarded execution"}
	}
	idempotencyKey := strings.TrimSpace(req.IdempotencyKey)
	if idempotencyKey == "" {
		idempotencyKey = buildAgentIdempotencyKey(req)
	}
	action := &model.AgentAction{SiteId: AgentSiteID(), ActionType: req.ActionType, AgentName: req.AgentName, TargetType: req.TargetType, TargetId: req.TargetId, UserId: req.UserId, RiskLevel: tool.RiskLevel, QuotaAmount: req.QuotaAmount, BudgetPool: req.BudgetPool, ApprovalRequired: approvalRequired, Status: status, IdempotencyKey: idempotencyKey, Reason: req.Reason, PayloadJson: mustAgentJSON(payload), ResultJson: mustAgentJSON(result)}
	if risk != nil {
		action.RiskLevel = risk.Level
		action.RiskScore = risk.Score
	}
	var approval *model.AgentActionApproval
	if approvalRequired {
		approval = &model.AgentActionApproval{SiteId: AgentSiteID(), Status: "pending", RequestedBy: req.AgentName, ExpiresAt: time.Now().Add(72 * time.Hour).Unix()}
	}
	if err := model.CreateAgentActionWithApproval(action, approval); err != nil {
		return nil, err
	}
	if approvalRequired {
		go AgentNotifyApprovalRequired(context.Background(), action)
	}
	if !approvalRequired && (forceExecute || cfg.AutoExecuteLowRisk || tool.RiskLevel == "low") {
		executed, execErr := AgentExecuteActionByID(action.Id, "agent_api")
		if executed != nil {
			action = executed
		}
		if execErr != nil {
			common.SysLog(fmt.Sprintf("[Agent] action auto execution failed id=%d type=%s err=%s", action.Id, action.ActionType, execErr.Error()))
		}
	}
	_ = model.DB.Create(&model.AgentEvent{SiteId: AgentSiteID(), EventType: "action.created", Source: "agent_api", Severity: "info", Status: "closed", ActorType: "admin", ActorUserId: reviewerId, Title: req.ActionType, PayloadJson: action.PayloadJson, ResultJson: action.ResultJson}).Error
	return action, nil
}

func AgentDecideApproval(approvalId int, reviewerId int, decision string, comment string) error {
	decision = strings.ToLower(strings.TrimSpace(decision))
	if decision != "approved" && decision != "rejected" {
		return errors.New("decision must be approved or rejected")
	}
	var approval model.AgentActionApproval
	if err := model.DB.Where("id = ? AND site_id = ?", approvalId, AgentSiteID()).First(&approval).Error; err != nil {
		return err
	}
	if approval.Status != "pending" {
		return errors.New("approval is not pending")
	}
	status := decision
	now := time.Now().Unix()
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&approval).Updates(map[string]interface{}{"status": status, "decision": decision, "comment": comment, "reviewer_id": reviewerId, "decided_at": now, "updated_at": now}).Error; err != nil {
			return err
		}
		actionStatus := "approved"
		if decision == "rejected" {
			actionStatus = "rejected"
		}
		return tx.Model(&model.AgentAction{}).Where("id = ? AND site_id = ?", approval.ActionId, AgentSiteID()).Updates(map[string]interface{}{"status": actionStatus, "updated_at": now}).Error
	})
	if err != nil {
		return err
	}
	if decision == "approved" {
		_, err = AgentExecuteActionByID(approval.ActionId, "approval")
	}
	return err
}

func findAgentTool(name string, tools []AgentTool) (AgentTool, bool) {
	for _, t := range tools {
		if t.Name == name {
			return t, true
		}
	}
	return AgentTool{}, false
}

func buildAgentIdempotencyKey(req AgentActionRequest) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%s|%s|%d|%d|%d", AgentSiteID(), req.ActionType, req.TargetType, req.TargetId, req.UserId, req.QuotaAmount, time.Now().UnixNano())))
	return hex.EncodeToString(sum[:])[:32]
}

func splitAgentCSV(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	seen := map[string]struct{}{}
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	return out
}

func truncateAgentText(value string, limit int) string {
	value = strings.TrimSpace(value)
	if limit <= 0 {
		return value
	}
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit]) + "..."
}

func mustAgentJSON(v any) string {
	if v == nil {
		return "{}"
	}
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}

func firstAgentNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
