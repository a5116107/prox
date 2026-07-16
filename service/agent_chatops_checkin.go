package service

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

// chatOpsChannel 将 chatops 来源(source)映射为签到渠道。
func chatOpsChannel(source string) string {
	switch source {
	case "qq":
		return model.CheckinChannelQQ
	case "tg", "telegram":
		return model.CheckinChannelTG
	case "community":
		return model.CheckinChannelCommunity
	default:
		return model.CheckinChannelWeb
	}
}

// chatOpsFormatUSD 将额度按美元(2位小数)格式化，用于群内结果展示。
func chatOpsFormatUSD(quota int) string {
	return fmt.Sprintf("%.2f 美刀", float64(quota)/common.QuotaPerUnit)
}

func chatOpsPlatformKey(source string, roomID string) string {
	s := strings.ToLower(strings.TrimSpace(source))
	roomID = strings.TrimSpace(roomID)
	switch s {
	case "community", "dc", "hhhl":
		return "community"
	case "telegram", "tg":
		if roomID != "" {
			return "tg_group"
		}
		return "tg"
	case "", "qq":
		if roomID != "" {
			return "qq_group"
		}
		return "qq"
	}
	if s == "telegram" {
		return "tg"
	}
	return s
}

func chatOpsPlatformCandidates(source string, roomID string) []string {
	candidates := make([]string, 0, 2)
	appendPlatform := func(platform string) {
		platform = strings.TrimSpace(platform)
		if platform == "" {
			return
		}
		for _, existing := range candidates {
			if existing == platform {
				return
			}
		}
		candidates = append(candidates, platform)
	}
	primary := chatOpsPlatformKey(source, roomID)
	appendPlatform(primary)
	if strings.TrimSpace(roomID) != "" {
		switch primary {
		case "qq_group":
			appendPlatform("qq")
		case "tg_group":
			appendPlatform("tg")
		}
	}
	return candidates
}

func chatOpsGroupConfigByScopeFromDB(db *gorm.DB, source string, roomID string) (*model.GroupChatOpsConfig, bool) {
	roomID = strings.TrimSpace(roomID)
	if roomID == "" || db == nil {
		return nil, false
	}
	candidates := chatOpsPlatformCandidates(source, roomID)
	var cfg model.GroupChatOpsConfig
	for _, platform := range candidates {
		if err := db.Where("site_id = ? AND platform = ? AND group_id = ?", AgentSiteID(), platform, roomID).First(&cfg).Error; err == nil {
			return &cfg, true
		}
	}
	for _, platform := range candidates {
		if err := db.Where("site_id = ? AND platform = ? AND group_id = ''", AgentSiteID(), platform).First(&cfg).Error; err == nil {
			return &cfg, true
		}
	}
	return nil, false
}

func chatOpsGroupConfigByScope(source string, roomID string) (*model.GroupChatOpsConfig, bool) {
	return chatOpsGroupConfigByScopeFromDB(model.DB, source, roomID)
}

func chatOpsGroupConfig(req AgentChatOpsRequest) (*model.GroupChatOpsConfig, bool) {
	return chatOpsGroupConfigByScope(req.Source, req.RoomID)
}

func chatOpsJSONMap(text string) map[string]any {
	text = strings.TrimSpace(text)
	if text == "" {
		return map[string]any{}
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		return map[string]any{}
	}
	return out
}

func chatOpsNestedMap(v any) map[string]any {
	if value, ok := v.(map[string]any); ok && value != nil {
		return value
	}
	return map[string]any{}
}

func chatOpsFirstInt(values ...any) int {
	for _, value := range values {
		switch v := value.(type) {
		case int:
			if v != 0 {
				return v
			}
		case int64:
			if v != 0 {
				return int(v)
			}
		case float64:
			if int(v) != 0 {
				return int(v)
			}
		case string:
			v = strings.TrimSpace(v)
			if v == "" {
				continue
			}
			var parsed int
			if _, err := fmt.Sscanf(v, "%d", &parsed); err == nil && parsed != 0 {
				return parsed
			}
		}
	}
	return 0
}

func chatOpsBool(value any) bool {
	switch v := value.(type) {
	case bool:
		return v
	case string:
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "1", "true", "yes", "on":
			return true
		}
	case float64:
		return v != 0
	case int:
		return v != 0
	case int64:
		return v != 0
	}
	return false
}

func chatOpsCheckinRule(cfg *model.GroupChatOpsConfig, policy effectiveGroupGamePolicy) (*model.CheckinRule, int, bool, string, error) {
	checkin := policy.Rules
	if checkin == nil {
		checkin = map[string]any{}
	}
	if cfg != nil {
		rules := chatOpsJSONMap(cfg.RuleJson)
		checkin = mergeGamePolicyMaps(checkin, chatOpsNestedMap(rules["checkin"]))
	}
	rule := &model.CheckinRule{
		FixedQuota: 0,
		RewardMin:  chatOpsFirstInt(checkin["reward_min"]),
		RewardMax:  chatOpsFirstInt(checkin["reward_max"]),
		BonusDays:  chatOpsFirstInt(checkin["bonus_days"]),
		BonusExtra: chatOpsFirstInt(checkin["bonus_extra"]),
	}
	if cfg != nil {
		rule.FixedQuota = cfg.CheckinQuota
	}
	if rule.FixedQuota <= 0 {
		rule.FixedQuota = chatOpsFirstInt(checkin["checkin_quota"], checkin["quota"], checkin["fixed_quota"])
	}
	dailyLimit := 0
	if cfg != nil {
		dailyLimit = cfg.DailyGroupRewardLimit
	}
	if dailyLimit <= 0 {
		dailyLimit = chatOpsFirstInt(checkin["daily_group_reward_limit"], checkin["daily_reward_limit"])
	}
	requireVerify := chatOpsBool(checkin["require_verify"])
	pool, err := effectivePolicyBudgetPool(policy, checkin, "activity")
	return rule, dailyLimit, requireVerify, pool, err
}

func chatOpsCheckinBudgetExceeded(source string, roomID string, limit int, incoming int) bool {
	roomID = strings.TrimSpace(roomID)
	if limit <= 0 || incoming <= 0 || roomID == "" || model.DB == nil {
		return false
	}
	date := model.AgentBusinessDateAt(time.Now())
	platform := chatOpsPlatformKey(source, roomID)
	var used int64
	model.DB.Model(&model.GroupMetricsDaily{}).
		Where("site_id = ? AND platform = ? AND group_id = ? AND metric_date = ?", AgentSiteID(), platform, roomID, date).
		Select("COALESCE(reward_cost_quota, 0)").Scan(&used)
	return int(used)+incoming > limit
}

func recordChatOpsGroupMetric(source string, roomID string, field string, rewardCost int) {
	roomID = strings.TrimSpace(roomID)
	if roomID == "" || model.DB == nil {
		return
	}
	now := time.Now().Unix()
	date := model.AgentBusinessDateAt(time.Now())
	platform := chatOpsPlatformKey(source, roomID)
	metric := model.GroupMetricsDaily{SiteId: AgentSiteID(), Platform: platform, GroupId: roomID, MetricDate: date, CreatedAt: now, UpdatedAt: now}
	_ = model.DB.Where("site_id = ? AND platform = ? AND group_id = ? AND metric_date = ?", AgentSiteID(), platform, roomID, date).FirstOrCreate(&metric).Error
	updates := map[string]any{"updated_at": now}
	field = strings.TrimSpace(field)
	if field != "" {
		switch field {
		case "invite_links", "joins", "binds", "verifies", "checkins":
			updates[field] = gorm.Expr(field+" + ?", 1)
		default:
			return
		}
	}
	if rewardCost != 0 {
		updates["reward_cost_quota"] = gorm.Expr("reward_cost_quota + ?", rewardCost)
	}
	if len(updates) == 1 {
		return
	}
	_ = model.DB.Model(&model.GroupMetricsDaily{}).Where("site_id = ? AND platform = ? AND group_id = ? AND metric_date = ?", AgentSiteID(), platform, roomID, date).Updates(updates).Error
}

func recordChatOpsActionLog(req AgentChatOpsRequest, identity AgentChatOpsIdentity, logType int, content string, opts model.LogEventOptions) {
	tags := append([]string{"chatops", strings.TrimSpace(req.Source)}, opts.Tags...)
	if opts.Source == "" {
		opts.Source = strings.TrimSpace(req.Source)
	}
	if opts.Category == "" {
		opts.Category = "chatops"
	}
	if opts.SiteId == "" {
		opts.SiteId = AgentSiteID()
	}
	if opts.RoomId == "" {
		opts.RoomId = strings.TrimSpace(req.RoomID)
	}
	if opts.ExternalUserId == "" {
		opts.ExternalUserId = strings.TrimSpace(req.UserExternalID)
	}
	opts.Tags = tags
	other := opts.Other
	if other == nil {
		other = map[string]interface{}{}
	}
	other["chatops"] = map[string]interface{}{
		"room_id":          strings.TrimSpace(req.RoomID),
		"message_id":       strings.TrimSpace(req.MessageID),
		"external_user_id": strings.TrimSpace(req.UserExternalID),
		"username":         strings.TrimSpace(req.Username),
		"user_bound":       identity.UserBound,
		"identity_reason":  identity.Reason,
		"source":           strings.TrimSpace(req.Source),
	}
	opts.Other = other
	model.RecordLogEvent(identity.NewAPIUserID, logType, content, opts)
}

// AgentChatOpsCheckinResult 签到结果（供 controller 返回 + bot 展示）。
type AgentChatOpsCheckinResult struct {
	Success      bool   `json:"success"`
	UserBound    bool   `json:"user_bound"`
	NewAPIUserID int    `json:"new_api_user_id"`
	Channel      string `json:"channel"`
	QuotaAwarded int    `json:"quota_awarded"`
	CurrentQuota int    `json:"current_quota"`
	Reply        string `json:"reply"`
}

// HandleAgentChatOpsCheckin 处理来自 QQ/TG/社区 的签到命令：
// 解析身份 -> 按渠道调用 UserCheckinByChannel 真实发额度并落 checkins 台账 -> 返回真实结果。
func HandleAgentChatOpsCheckin(req AgentChatOpsRequest) AgentChatOpsCheckinResult {
	channel := chatOpsChannel(req.Source)
	display := firstNonEmptyAgent(req.Username, req.UserExternalID)

	cfg, hasGroupCfg := chatOpsGroupConfig(req)
	gamePolicy, policyErr := resolveEffectiveGroupGamePolicyTx(model.DB, AgentSiteID(), req.Source, req.RoomID, "checkin")
	if policyErr != nil {
		return AgentChatOpsCheckinResult{Success: false, Channel: channel, Reply: fmt.Sprintf("@%s 签到配置读取失败，请稍后重试。", display)}
	}
	if (hasGroupCfg && !cfg.CheckinEnabled) || (gamePolicy.Found && !gamePolicy.Enabled) {
		return AgentChatOpsCheckinResult{
			Success: false, Channel: channel,
			Reply: fmt.Sprintf("@%s 本群签到暂未开启，请关注管理员公告。", display),
		}
	}

	identity := ResolveAgentChatOpsIdentity(req)
	if !identity.UserBound || identity.NewAPIUserID <= 0 {
		recordChatOpsActionLog(req, identity, model.LogTypeSystem, "chatops checkin denied: user not bound", model.LogEventOptions{
			Action: "checkin",
			Status: "denied",
			Group:  channel,
			Other: map[string]interface{}{
				"checkin": map[string]interface{}{
					"channel": channel,
					"reason":  "not_bound",
				},
			},
		})
		return AgentChatOpsCheckinResult{
			Success:   false,
			UserBound: false,
			Channel:   channel,
			Reply:     fmt.Sprintf("@%s 你还未绑定本站账号。请先在站点生成绑定码，回到群里发送「绑定 <绑定码>」完成绑定后再签到。", display),
		}
	}

	var checkinRule *model.CheckinRule
	dailyLimit := 0
	requireVerify := false
	budgetPool := "activity"
	if hasGroupCfg || gamePolicy.Found {
		var ruleErr error
		checkinRule, dailyLimit, requireVerify, budgetPool, ruleErr = chatOpsCheckinRule(cfg, gamePolicy)
		if ruleErr != nil {
			return AgentChatOpsCheckinResult{Success: false, UserBound: true, NewAPIUserID: identity.NewAPIUserID, Channel: channel, Reply: fmt.Sprintf("@%s 签到预算配置有误，请联系管理员。", display)}
		}
	}
	if requireVerify && !identity.UserBound {
		return AgentChatOpsCheckinResult{
			Success:      false,
			UserBound:    false,
			NewAPIUserID: identity.NewAPIUserID,
			Channel:      channel,
			Reply:        fmt.Sprintf("@%s 请先完成验牌后再签到。", display),
		}
	}

	preview, err := model.PreviewCheckinAwardByChannel(identity.NewAPIUserID, channel, checkinRule)
	if err != nil {
		recordChatOpsActionLog(req, identity, model.LogTypeSystem, "chatops checkin preview failed", model.LogEventOptions{
			Action:     "checkin",
			Status:     "failed",
			Group:      channel,
			BudgetPool: budgetPool,
			Other: map[string]interface{}{
				"checkin": map[string]interface{}{
					"channel": channel,
					"reason":  err.Error(),
				},
			},
		})
		return AgentChatOpsCheckinResult{
			Success:      false,
			UserBound:    true,
			NewAPIUserID: identity.NewAPIUserID,
			Channel:      channel,
			Reply:        fmt.Sprintf("@%s 签到失败：%s", display, err.Error()),
		}
	}
	if chatOpsCheckinBudgetExceeded(req.Source, req.RoomID, dailyLimit, preview.QuotaAwarded) {
		recordChatOpsActionLog(req, identity, model.LogTypeSystem, "chatops checkin denied: group daily limit reached", model.LogEventOptions{
			Action: "checkin",
			Status: "denied",
			Group:  channel,
			Other: map[string]interface{}{
				"checkin": map[string]interface{}{
					"channel":             channel,
					"reason":              "group_daily_reward_limit",
					"daily_group_limit":   dailyLimit,
					"quota_awarded_guess": preview.QuotaAwarded,
				},
			},
		})
		return AgentChatOpsCheckinResult{
			Success:      false,
			UserBound:    true,
			NewAPIUserID: identity.NewAPIUserID,
			Channel:      channel,
			Reply:        fmt.Sprintf("@%s 本群今日签到奖励池已发完，请明天再来。", display),
		}
	}
	checkin, err := model.UserCheckinByChannelWithQuotaFromPool(identity.NewAPIUserID, channel, preview.QuotaAwarded, budgetPool)
	if err != nil {
		recordChatOpsActionLog(req, identity, model.LogTypeSystem, "chatops checkin failed", model.LogEventOptions{
			Action:     "checkin",
			Status:     "failed",
			Group:      channel,
			BudgetPool: budgetPool,
			Other: map[string]interface{}{
				"checkin": map[string]interface{}{
					"channel": channel,
					"reason":  err.Error(),
				},
			},
		})
		return AgentChatOpsCheckinResult{
			Success:      false,
			UserBound:    true,
			NewAPIUserID: identity.NewAPIUserID,
			Channel:      channel,
			Reply:        fmt.Sprintf("@%s 签到失败：%s", display, err.Error()),
		}
	}

	current := checkin.QuotaAwarded
	if u, e := model.GetUserById(identity.NewAPIUserID, false); e == nil && u != nil {
		current = u.Quota
	}

	model.RecordCommunityBotRewardLog(identity.NewAPIUserID,
		fmt.Sprintf("%s签到奖励 %s", channel, logger.LogQuota(checkin.QuotaAwarded)),
		checkin.QuotaAwarded, 0, req.RoomID, "checkin_"+channel)
	recordChatOpsGroupMetric(req.Source, req.RoomID, "checkins", checkin.QuotaAwarded)
	recordChatOpsActionLog(req, identity, model.LogTypeSystem, "chatops checkin success", model.LogEventOptions{
		Quota:      checkin.QuotaAwarded,
		Action:     "checkin",
		Status:     "success",
		Group:      channel,
		BudgetPool: budgetPool,
		RewardType: "checkin_" + channel,
		Other: map[string]interface{}{
			"checkin": map[string]interface{}{
				"channel":       channel,
				"quota_awarded": checkin.QuotaAwarded,
				"current_quota": current,
				"base_quota":    preview.BaseQuota,
				"bonus_quota":   preview.BonusQuota,
				"streak_after":  preview.StreakAfter,
			},
		},
	})

	streakLine := ""
	if preview.StreakAfter > 0 {
		streakLine = fmt.Sprintf("\n🔥 连续签到：%d 天", preview.StreakAfter)
	}
	bonusLine := ""
	if preview.BonusQuota > 0 {
		bonusLine = fmt.Sprintf("\n🎁 连签加赠：%s", chatOpsFormatUSD(preview.BonusQuota))
	}

	return AgentChatOpsCheckinResult{
		Success:      true,
		UserBound:    true,
		NewAPIUserID: identity.NewAPIUserID,
		Channel:      channel,
		QuotaAwarded: checkin.QuotaAwarded,
		CurrentQuota: current,
		Reply: fmt.Sprintf("🎉 签到成功！@%s\n获得 %s ！%s%s\n💵 当前额度：%s",
			display, chatOpsFormatUSD(checkin.QuotaAwarded), streakLine, bonusLine, chatOpsFormatUSD(current)),
	}
}

// AgentChatOpsVerifyResult 验牌结果（3项检查）。
type AgentChatOpsVerifyResult struct {
	UserBound     bool   `json:"user_bound"`
	NewAPIUserID  int    `json:"new_api_user_id"`
	HasAPIKey     bool   `json:"has_api_key"`
	HasQuota      bool   `json:"has_quota"`
	CurrentQuota  int    `json:"current_quota"`
	RequiredQuota int    `json:"required_quota"`
	Passed        bool   `json:"passed"`
	Reply         string `json:"reply"`
}

// HandleAgentChatOpsVerify 处理验牌命令（身份验证，3项检查）：
//
//	① 已绑定账号 ② 已开通 API Key ③ 额度 > 0，三项全过才算验牌通过。
//
// 验牌通过是参与 P2P 对战和系统坐庄类游戏的前提。
func HandleAgentChatOpsVerify(req AgentChatOpsRequest) AgentChatOpsVerifyResult {
	display := firstNonEmptyAgent(req.Username, req.UserExternalID)
	cfg, hasGroupCfg := chatOpsGroupConfig(req)
	if hasGroupCfg && !cfg.VerifyEnabled {
		return AgentChatOpsVerifyResult{Reply: fmt.Sprintf("@%s 本群验牌暂未开启，请关注管理员公告。", display)}
	}

	identity := ResolveAgentChatOpsIdentity(req)
	if !identity.UserBound || identity.NewAPIUserID <= 0 {
		recordChatOpsActionLog(req, identity, model.LogTypeSystem, "chatops verify denied: user not bound", model.LogEventOptions{
			Action: "verify",
			Status: "denied",
			Other: map[string]interface{}{
				"verify": map[string]interface{}{
					"passed":         false,
					"reason":         "not_bound",
					"reason_code":    "not_bound",
					"reason_message": "未绑定站点账号",
				},
			},
		})
		return AgentChatOpsVerifyResult{
			UserBound: false,
			Reply:     fmt.Sprintf("🛡️ 验牌结果 @%s\n① 账号绑定：❌ 未绑定\n② API Key：—\n③ 额度状态：—\n\n请先在站点生成绑定码，回到群里发送「绑定 <绑定码>」完成绑定。", display),
		}
	}

	u, err := model.GetUserById(identity.NewAPIUserID, false)
	if err != nil || u == nil {
		recordChatOpsActionLog(req, identity, model.LogTypeSystem, "chatops verify failed: user query failed", model.LogEventOptions{
			Action: "verify",
			Status: "failed",
			Other: map[string]interface{}{
				"verify": map[string]interface{}{
					"passed":         false,
					"reason":         "user_query_failed",
					"reason_code":    "user_query_failed",
					"reason_message": "查询用户失败",
				},
			},
		})
		return AgentChatOpsVerifyResult{UserBound: true, NewAPIUserID: identity.NewAPIUserID, Reply: fmt.Sprintf("@%s 验牌暂时失败，请稍后再试。", display)}
	}

	activeTokens, _ := model.CountUserEnabledTokens(identity.NewAPIUserID)
	hasKey := activeTokens > 0
	requiredQuota := 1
	if hasGroupCfg && cfg.VerifyMinQuota > 0 {
		requiredQuota = cfg.VerifyMinQuota
	}
	hasQuota := u.Quota >= requiredQuota

	keyMark := "✅ 已开通"
	if !hasKey {
		keyMark = "❌ 未开通"
	}
	quotaMark := fmt.Sprintf("✅ %s", chatOpsFormatUSD(u.Quota))
	if !hasQuota {
		quotaMark = fmt.Sprintf("❌ 额度不足（需 ≥ %s）", chatOpsFormatUSD(requiredQuota))
	}

	passed := hasKey && hasQuota
	reasonCode, reasonMessage := agentChatOpsVerifyFailureReason(hasKey, hasQuota)
	var reply string
	if passed {
		reply = fmt.Sprintf("🛡️ 验牌结果 @%s\n① 账号绑定：✅ 已绑定\n② API Key：%s\n③ 额度状态：%s\n\n验牌通过 ✅ 你已满足参与 P2P 对战与系统坐庄类游戏的条件。", display, keyMark, quotaMark)
	} else {
		tip := ""
		if !hasKey {
			tip = "请先在站点开通 API Key。"
		} else if !hasQuota {
			tip = "请先为账号充值或获取额度（可发送「签到」领取）。"
		}
		reply = fmt.Sprintf("🛡️ 验牌结果 @%s\n① 账号绑定：✅ 已绑定\n② API Key：%s\n③ 额度状态：%s\n\n验牌未通过：%s", display, keyMark, quotaMark, tip)
	}
	recordChatOpsActionLog(req, identity, model.LogTypeSystem, "chatops verify result", model.LogEventOptions{
		Action: "verify",
		Status: func() string {
			if passed {
				return "success"
			}
			return "failed"
		}(),
		Other: map[string]interface{}{
			"verify": map[string]interface{}{
				"passed":         passed,
				"reason":         reasonCode,
				"reason_code":    reasonCode,
				"reason_message": reasonMessage,
				"has_api_key":    hasKey,
				"has_quota":      hasQuota,
				"current_quota":  u.Quota,
				"required_quota": requiredQuota,
			},
		},
	})

	if passed {
		recordChatOpsGroupMetric(req.Source, req.RoomID, "verifies", 0)
	}

	return AgentChatOpsVerifyResult{
		UserBound:     true,
		NewAPIUserID:  identity.NewAPIUserID,
		HasAPIKey:     hasKey,
		HasQuota:      hasQuota,
		CurrentQuota:  u.Quota,
		RequiredQuota: requiredQuota,
		Passed:        passed,
		Reply:         reply,
	}
}

func agentChatOpsVerifyFailureReason(hasKey bool, hasQuota bool) (string, string) {
	switch {
	case !hasKey && !hasQuota:
		return "missing_api_key_and_quota", "未开通 API Key 且额度不足"
	case !hasKey:
		return "missing_api_key", "未开通 API Key"
	case !hasQuota:
		return "insufficient_quota", "额度不足"
	default:
		return "ok", "验牌通过"
	}
}

// firstNonEmptyAgent 返回第一个非空字符串。
func firstNonEmptyAgent(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
