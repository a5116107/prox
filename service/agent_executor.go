package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func AgentExecuteActionByID(actionID int, executor string) (*model.AgentAction, error) {
	var action model.AgentAction
	if err := model.DB.Where("id = ? AND site_id = ?", actionID, AgentSiteID()).First(&action).Error; err != nil {
		return nil, err
	}
	if action.Status == "completed" || action.Status == "rejected" {
		return &action, nil
	}
	now := time.Now().Unix()
	_ = model.DB.Model(&action).Updates(map[string]interface{}{"status": "running", "updated_at": now}).Error
	result, err := executeAgentAction(&action)
	updates := map[string]interface{}{"updated_at": time.Now().Unix()}
	if err != nil {
		updates["status"] = "failed"
		updates["result_json"] = mustAgentJSON(map[string]any{"ok": false, "error": err.Error()})
		_ = model.DB.Model(&model.AgentAction{}).Where("id = ? AND site_id = ?", action.Id, action.SiteId).Updates(updates).Error
		_ = model.DB.Create(&model.AgentEvent{SiteId: action.SiteId, EventType: "action.failed", Source: firstAgentNonEmpty(executor, "agent_executor"), Severity: "error", Status: "open", Title: action.ActionType, PayloadJson: action.PayloadJson, ResultJson: updates["result_json"].(string)}).Error
		_ = model.DB.Where("id = ? AND site_id = ?", action.Id, action.SiteId).First(&action).Error
		return &action, err
	}
	updates["status"] = "completed"
	updates["executed_at"] = time.Now().Unix()
	updates["result_json"] = mustAgentJSON(result)
	if err := model.DB.Model(&model.AgentAction{}).Where("id = ? AND site_id = ?", action.Id, action.SiteId).Updates(updates).Error; err != nil {
		return nil, err
	}
	_ = model.DB.Create(&model.AgentEvent{SiteId: action.SiteId, EventType: "action.completed", Source: firstAgentNonEmpty(executor, "agent_executor"), Severity: "info", Status: "closed", Title: action.ActionType, PayloadJson: action.PayloadJson, ResultJson: updates["result_json"].(string)}).Error
	_ = model.DB.Where("id = ? AND site_id = ?", action.Id, action.SiteId).First(&action).Error
	return &action, nil
}

func executeAgentAction(action *model.AgentAction) (map[string]any, error) {
	payload := map[string]any{}
	_ = json.Unmarshal([]byte(action.PayloadJson), &payload)
	switch action.ActionType {
	case "site.state.read":
		state, err := GetAgentSiteState()
		if err != nil {
			return nil, err
		}
		return map[string]any{"ok": true, "summary": agentStateSummary(state), "site_state": state}, nil
	case "budget.check":
		if err := EnsureAgentRuntimeDefaults(); err != nil {
			return nil, err
		}
		pools, err := model.ListAgentBudgetPools(AgentSiteID(), model.AgentBusinessDateAt(time.Now()))
		return map[string]any{"ok": err == nil, "summary": humanAgentBudgetSummary(pools), "budgets": pools}, err
	case "site.logs.read", "chatops.logs.read":
		return executeAgentSiteLogsRead(action.ActionType, payload)
	case "message.community.send":
		text := firstAgentNonEmpty(agentPayloadString(payload, "text", "message", "content"), action.Reason)
		if text == "" {
			return nil, errors.New("message text is empty")
		}
		if err := CommunitySendRoomMessage(context.Background(), text); err != nil {
			return nil, err
		}
		return map[string]any{"ok": true, "summary": "community message sent", "text": text}, nil
	case "message.qq.send":
		text := firstAgentNonEmpty(agentPayloadString(payload, "text", "message", "content"), action.Reason)
		groupID := firstAgentNonEmpty(agentPayloadString(payload, "group_id", "room_id"), action.TargetId)
		if err := AgentSendQQGroupMessage(context.Background(), groupID, text); err != nil {
			return nil, err
		}
		return map[string]any{"ok": true, "summary": "qq group message sent", "group_id": groupID}, nil
	case "message.tg.send":
		text := firstAgentNonEmpty(agentPayloadString(payload, "text", "message", "content"), action.Reason)
		chatID := firstAgentNonEmpty(agentPayloadString(payload, "chat_id", "room_id"), action.TargetId)
		if err := AgentSendTelegramMessage(context.Background(), chatID, text); err != nil {
			return nil, err
		}
		return map[string]any{"ok": true, "summary": "telegram message sent", "chat_id": chatID}, nil
	case "group.message.delete", "group.member.mute", "group.member.unmute", "group.member.kick", "group.member.ban", "group.member.unban", "group.member.lookup", "group.admin.lookup":
		return executeAgentGroupModerationAction(action.ActionType, payload)
	case "user.quota.read":
		userID := int(agentPayloadFloat(payload, "user_id", "uid"))
		if userID <= 0 {
			userID = action.UserId
		}
		text, err := readAgentUserQuota(userID)
		if err != nil {
			return nil, err
		}
		inviterID := 0
		quotaBalance := 0
		username := ""
		var _qu model.User
		if qerr := model.DB.Select("id", "username", "quota", "inviter_id").Where("id = ?", userID).First(&_qu).Error; qerr == nil {
			inviterID = _qu.InviterId
			quotaBalance = _qu.Quota
			username = _qu.Username
		}
		return map[string]any{"ok": true, "summary": text, "user_id": userID, "username": username, "inviter_id": inviterID, "quota_balance": quotaBalance, "balance_quota": quotaBalance, "quota_usd": fmt.Sprintf("%.4f", float64(quotaBalance)/float64(common.QuotaPerUnit))}, nil
	case "agent.model.manage":
		if strings.EqualFold(strings.TrimSpace(agentPayloadString(payload, "action", "operation")), "ensure_token_creation_enabled") {
			userID := int(agentPayloadFloat(payload, "user_id", "uid", "resolved_new_api_user_id"))
			if userID <= 0 {
				userID = action.UserId
			}
			if userID <= 0 {
				return nil, errors.New("ensure token creation requires user_id")
			}
			var user model.User
			if err := model.DB.Select("id", "status").Where("id = ?", userID).First(&user).Error; err != nil {
				return nil, err
			}
			if user.Status != 1 {
				if err := model.DB.Model(&model.User{}).Where("id = ?", userID).Update("status", 1).Error; err != nil {
					return nil, err
				}
			}
			return map[string]any{"ok": true, "summary": fmt.Sprintf("用户 #%d 已确认可创建 API Key。", userID), "user_id": userID, "operation": "ensure_token_creation_enabled"}, nil
		}
		modelName := firstAgentNonEmpty(agentPayloadString(payload, "model", "model_name", "llm_model"), action.TargetId)
		if strings.EqualFold(strings.TrimSpace(modelName), "list") {
			modelName = ""
		}
		text, err := agentModelManageReply(modelName)
		if err != nil {
			return nil, err
		}
		return map[string]any{"ok": true, "summary": text, "model": modelName}, nil
	case "agent.skill.install":
		return executeAgentSkillInstall(action, payload)
	case "agent.skill.list":
		return executeAgentSkillList(payload)
	case "reward.grant.small":
		userID := int(agentPayloadFloat(payload, "user_id", "uid", "resolved_new_api_user_id"))
		if userID <= 0 {
			userID = action.UserId
		}
		quota := action.QuotaAmount
		if quota == 0 {
			quota = int(agentPayloadFloat(payload, "quota", "amount", "quota_amount"))
		}
		if userID <= 0 || quota == 0 {
			return nil, errors.New("reward requires positive user_id and non-zero quota")
		}
		if err := grantAgentQuota(action, userID, quota); err != nil {
			return nil, err
		}
		verb := "增加"
		if quota < 0 {
			verb = "扣除"
		}
		return map[string]any{"ok": true, "summary": fmt.Sprintf("已给用户 #%d %s %s 额度。", userID, verb, agentQuotaUSDText(absAgentInt(quota))), "user_id": userID, "quota": quota}, nil
	case "reward.settlement.batch":
		settlementID := firstAgentNonEmpty(agentPayloadString(payload, "settlement_id", "settlement"), fmt.Sprintf("settle-%d", action.Id))
		rawMuts, _ := payload["mutations"].([]any)
		var muts []model.QuotaMutation
		defaultPoolType := normalizeAgentStringFallback(action.BudgetPool, "game")
		for _, rm := range rawMuts {
			m, ok := rm.(map[string]any)
			if !ok {
				continue
			}
			muts = append(muts, model.QuotaMutation{
				UserID:         int(agentPayloadFloat(m, "user_id", "uid")),
				Delta:          int(agentPayloadFloat(m, "delta", "quota_amount")),
				PoolType:       normalizeAgentStringFallback(firstAgentNonEmpty(agentPayloadString(m, "pool_type", "budget_pool"), defaultPoolType), "game"),
				SourceType:     firstAgentNonEmpty(agentPayloadString(m, "source_type"), "game_payout"),
				IdempotencyKey: agentPayloadString(m, "idempotency_key"),
				Remark:         agentPayloadString(m, "remark", "reason"),
				ActionID:       action.Id,
				MetadataJson:   agentPayloadString(m, "metadata_json"),
			})
		}
		siteID := AgentSiteID()
		if err := model.DB.Transaction(func(tx *gorm.DB) error {
			if err := model.ApplyQuotaSettlementBatchTx(tx, siteID, settlementID, muts); err != nil {
				return err
			}
			return recordGameSettlementAuditTx(tx, action, payload, siteID, settlementID, muts)
		}); err != nil {
			return nil, err
		}
		return map[string]any{"ok": true, "summary": fmt.Sprintf("settlement %s: %d mutations", settlementID, len(muts)), "settlement_id": settlementID}, nil
	case "quiz.state.load":
		return executeQuizStateLoad(action, payload)
	case "quiz.state.commit":
		return executeQuizStateCommit(action, payload)
	case "quiz.question.draw":
		return executeQuizQuestionDraw(action, payload)
	case "quiz.round.load":
		return executeQuizRoundLoad(action, payload)
	case "quiz.answer.submit":
		return executeQuizAnswerSubmit(action, payload)
	case "fund.report.read":
		siteID := AgentSiteID()
		var acc model.OpsFundAccount
		if err := model.DB.Where("site_id = ? AND fund_type = ?", siteID, "operations").First(&acc).Error; err != nil {
			return nil, err
		}
		var recent []model.OpsFundLedger
		model.DB.Where("site_id = ?", siteID).Order("id desc").Limit(10).Find(&recent)
		dayStart := model.AgentBusinessDayStartAt(time.Now())
		var today []model.OpsFundLedger
		model.DB.Where("site_id = ? AND created_at >= ?", siteID, dayStart.Unix()).Find(&today)
		recentItems := make([]map[string]any, 0, len(recent))
		recentIncome := 0
		recentExpense := 0
		for _, item := range recent {
			if item.DeltaQuota >= 0 {
				recentIncome += item.DeltaQuota
			} else {
				recentExpense += -item.DeltaQuota
			}
			recentItems = append(recentItems, map[string]any{
				"id":              item.Id,
				"delta_quota":     item.DeltaQuota,
				"delta_usd":       fmt.Sprintf("%.4f", float64(item.DeltaQuota)/float64(common.QuotaPerUnit)),
				"balance_after":   item.BalanceAfter,
				"source_type":     item.SourceType,
				"source_pool":     item.SourcePoolType,
				"user_id":         item.UserId,
				"remark":          item.Remark,
				"created_at":      item.CreatedAt,
				"created_at_text": time.Unix(item.CreatedAt, 0).Format("2006-01-02 15:04:05"),
			})
		}
		todayIncome := 0
		todayExpense := 0
		for _, item := range today {
			if item.DeltaQuota >= 0 {
				todayIncome += item.DeltaQuota
			} else {
				todayExpense += -item.DeltaQuota
			}
		}
		observability := agentBuildFundObservability(siteID, dayStart)
		summary := humanAgentFundSummary(acc.BalanceQuota, todayIncome, todayExpense, len(recentItems))
		if extra := agentFundObservabilitySummary(observability); extra != "" {
			summary += "；" + extra
		}
		return map[string]any{
			"ok":                     true,
			"summary":                summary,
			"balance_quota":          acc.BalanceQuota,
			"balance_usd":            fmt.Sprintf("%.4f", float64(acc.BalanceQuota)/float64(common.QuotaPerUnit)),
			"today_income_quota":     todayIncome,
			"today_expense_quota":    todayExpense,
			"recent_income_quota":    recentIncome,
			"recent_expense_quota":   recentExpense,
			"recent_count":           len(recentItems),
			"recent":                 recentItems,
			"observability":          observability,
			"source_breakdown":       observability["source_breakdown"],
			"source_breakdown_today": observability["source_breakdown_today"],
			"game_audit":             observability["game_audit"],
			"invite_audit":           observability["invite_audit"],
			"group_metrics_today":    observability["group_metrics_today"],
			"jackpot_audit":          observability["jackpot_audit"],
			"checkin_audit_today":    observability["checkin_audit_today"],
		}, nil
	case "fund.topup":
		if agentPayloadString(payload, "chatops_source") != "admin" {
			return nil, errors.New("fund.topup requires admin source")
		}
		delta := int(agentPayloadFloat(payload, "delta", "delta_quota", "quota_amount"))
		if delta == 0 {
			return nil, errors.New("delta must be non-zero")
		}
		remark := agentPayloadString(payload, "remark", "reason")
		siteID := AgentSiteID()
		var acc model.OpsFundAccount
		if err := model.DB.Where("site_id = ? AND fund_type = ?", siteID, "operations").First(&acc).Error; err != nil {
			return nil, err
		}
		newBal := acc.BalanceQuota + delta
		if newBal < 0 {
			return nil, errors.New("resulting balance would be negative")
		}
		if err := model.DB.Model(&model.OpsFundAccount{}).Where("id = ?", acc.Id).Update("balance_quota", newBal).Error; err != nil {
			return nil, err
		}
		sourceType := "fund_topup"
		if delta < 0 {
			sourceType = "fund_adjustment"
		}
		ledger := model.OpsFundLedger{SiteId: siteID, FundAccountId: acc.Id, DeltaQuota: delta, BalanceAfter: newBal, SourceType: sourceType, IdempotencyKey: action.IdempotencyKey, Remark: strings.TrimSpace(remark)}
		if res := model.DB.Create(&ledger); res.Error != nil {
			return nil, res.Error
		}
		return map[string]any{"ok": true, "summary": fmt.Sprintf("基金调整 %s，新余额 %s", agentQuotaUSDText(delta), agentQuotaUSDText(newBal)), "balance_quota": newBal}, nil
	case "risk.evaluate":
		userID := int(agentPayloadFloat(payload, "user_id", "uid"))
		if userID <= 0 {
			userID = action.UserId
		}
		res, err := AgentRiskEvaluate(userID, payload)
		if err != nil {
			return nil, err
		}
		return map[string]any{"ok": true, "summary": fmt.Sprintf("risk=%s score=%d", res.Level, res.Score), "risk": res}, nil
	case "admin.notice.publish", "admin.channel.suggest", "task.chatops.record":
		return map[string]any{"ok": true, "summary": "suggestion recorded for human review", "payload": payload}, nil
	default:
		return nil, fmt.Errorf("unsupported agent action: %s", action.ActionType)
	}
}

func executeAgentSiteLogsRead(actionType string, payload map[string]any) (map[string]any, error) {
	if model.LOG_DB == nil {
		return nil, errors.New("log database is not initialized")
	}
	limit := int(agentPayloadFloat(payload, "limit", "num", "count"))
	if limit <= 0 {
		limit = 8
	}
	if limit > 20 {
		limit = 20
	}
	logType := agentLogTypeFromPayload(payload)
	minutes := int(agentPayloadFloat(payload, "minutes", "recent_minutes", "window_minutes"))
	if minutes <= 0 {
		minutes = 60
	}
	if minutes > 24*60 {
		minutes = 24 * 60
	}
	filter := model.LogQueryFilter{
		LogType:        logType,
		StartTimestamp: time.Now().Add(-time.Duration(minutes) * time.Minute).Unix(),
		EndTimestamp:   time.Now().Unix(),
		StartIdx:       0,
		Num:            limit,
		SiteId:         firstAgentNonEmpty(agentPayloadString(payload, "site_id"), AgentSiteID()),
		Group:          agentPayloadString(payload, "group", "api_group"),
		Username:       agentPayloadString(payload, "username"),
		RequestId:      agentPayloadString(payload, "request_id"),
		RoomId:         agentPayloadString(payload, "room_id", "chat_id", "group_id"),
		Category:       agentPayloadString(payload, "category"),
		Source:         agentPayloadString(payload, "source", "log_source"),
		Action:         agentPayloadString(payload, "action"),
		Status:         agentPayloadString(payload, "status"),
		RiskLevel:      agentPayloadString(payload, "risk_level"),
	}
	logs, total, err := model.GetAllLogs(filter)
	if err != nil {
		return nil, err
	}
	items := make([]map[string]any, 0, len(logs))
	for _, l := range logs {
		if l == nil {
			continue
		}
		items = append(items, map[string]any{
			"id":                  l.Id,
			"type":                l.Type,
			"type_text":           agentLogTypeText(l.Type),
			"created_at":          l.CreatedAt,
			"created_at_text":     time.Unix(l.CreatedAt, 0).Format("2006-01-02 15:04:05"),
			"user_id":             l.UserId,
			"username":            l.Username,
			"content":             truncateAgentText(l.Content, 220),
			"model_name":          l.ModelName,
			"group":               l.Group,
			"channel_id":          l.ChannelId,
			"channel_name":        l.ChannelName,
			"quota":               l.Quota,
			"quota_usd":           fmt.Sprintf("%.4f", float64(l.Quota)/float64(common.QuotaPerUnit)),
			"request_id":          l.RequestId,
			"upstream_request_id": l.UpstreamRequestId,
			"status":              l.Status,
			"source":              l.Source,
			"action":              l.Action,
			"room_id":             l.RoomId,
			"risk_level":          l.RiskLevel,
			"tags":                l.Tags,
		})
	}
	typeText := "全部"
	if logType != model.LogTypeUnknown {
		typeText = agentLogTypeText(logType)
	}
	summary := fmt.Sprintf("最近 %d 分钟%s日志共 %d 条，已取最新 %d 条。", minutes, typeText, total, len(items))
	if len(items) > 0 {
		parts := make([]string, 0, len(items))
		for i, item := range items {
			parts = append(parts, fmt.Sprintf("%d. [%s] %s %s", i+1, item["type_text"], item["created_at_text"], truncateAgentText(fmt.Sprint(item["content"]), 80)))
		}
		summary += "\n" + strings.Join(parts, "\n")
	}
	return map[string]any{"ok": true, "summary": summary, "total": total, "limit": limit, "minutes": minutes, "log_type": logType, "log_type_text": typeText, "logs": items, "action_type": actionType}, nil
}

func agentLogTypeFromPayload(payload map[string]any) int {
	raw := strings.ToLower(strings.TrimSpace(agentPayloadString(payload, "log_type", "type", "kind")))
	switch raw {
	case "1", "topup", "recharge", "充值":
		return model.LogTypeTopup
	case "2", "consume", "request", "usage", "消费", "请求":
		return model.LogTypeConsume
	case "3", "manage", "admin", "管理", "后台":
		return model.LogTypeManage
	case "4", "system", "sys", "系统":
		return model.LogTypeSystem
	case "5", "error", "errors", "错误", "失败", "报错":
		return model.LogTypeError
	case "6", "refund", "退款":
		return model.LogTypeRefund
	case "7", "login", "登录":
		return model.LogTypeLogin
	default:
		return model.LogTypeUnknown
	}
}

func agentLogTypeText(t int) string {
	switch t {
	case model.LogTypeTopup:
		return "充值"
	case model.LogTypeConsume:
		return "消费"
	case model.LogTypeManage:
		return "管理"
	case model.LogTypeSystem:
		return "系统"
	case model.LogTypeError:
		return "错误"
	case model.LogTypeRefund:
		return "退款"
	case model.LogTypeLogin:
		return "登录"
	default:
		return "全部"
	}
}

func recordGameSettlementAuditTx(tx *gorm.DB, action *model.AgentAction, payload map[string]any, siteID string, settlementID string, muts []model.QuotaMutation) error {
	if tx == nil || len(muts) == 0 || strings.TrimSpace(settlementID) == "" {
		return nil
	}
	now := time.Now().Unix()
	platform := firstAgentNonEmpty(agentPayloadString(payload, "platform", "chatops_source", "source"), "unknown")
	if strings.EqualFold(platform, "telegram") {
		platform = "tg"
	}
	groupID := firstAgentNonEmpty(agentPayloadString(payload, "room_id", "group_id", "chat_id"), "unknown")
	gameCode := normalizeAgentGameCode(firstAgentNonEmpty(agentPayloadString(payload, "game_code", "game"), firstAgentGameCodeFromMutations(muts), "game"))
	roundKey := firstAgentNonEmpty(agentPayloadString(payload, "round_key", "round"), fmt.Sprintf("%s:%s", siteID, settlementID))
	totalStake, totalPayout := 0, 0
	players := map[int]bool{}
	for _, m := range muts {
		if m.UserID > 0 {
			players[m.UserID] = true
		}
		if m.Delta < 0 {
			totalStake += -m.Delta
		} else if m.Delta > 0 {
			totalPayout += m.Delta
		}
	}
	commissionQuota := int(agentPayloadFloat(payload, "commission_quota", "platform_fee_quota", "fee_quota"))
	if commissionQuota < 0 {
		commissionQuota = 0
	}
	if commissionQuota == 0 {
		// Legacy fallback for adapter versions that did not pass explicit fee events.
		if retained := totalStake - totalPayout; retained > 0 {
			commissionQuota = retained
		}
	}
	jackpotDelta := 0
	if agentGameUsesJackpot(gameCode) {
		jackpotDelta = totalStake - totalPayout - commissionQuota
	}
	resultJson := mustAgentJSON(map[string]any{
		"settlement_id":    settlementID,
		"platform":         platform,
		"group_id":         groupID,
		"game_code":        gameCode,
		"mutation_count":   len(muts),
		"player_count":     len(players),
		"stake_quota":      totalStake,
		"payout_quota":     totalPayout,
		"commission_quota": commissionQuota,
		"jackpot_delta":    jackpotDelta,
		"agent_action_id":  action.Id,
		"idempotency_key":  action.IdempotencyKey,
	})
	round := model.GameRound{
		SiteId: siteID, Platform: platform, GroupId: groupID, GameCode: gameCode,
		RoundKey: roundKey, Status: "closed", RandomSeed: action.IdempotencyKey,
		RuleJson: "{}", ResultJson: resultJson, OpenedAt: now, ClosedAt: now, CreatedAt: now, UpdatedAt: now,
	}
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&round).Error; err != nil {
		return err
	}
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("round_key = ?", roundKey).First(&round).Error; err != nil {
		return err
	}
	if err := tx.Model(&model.GameRound{}).Where("id = ?", round.Id).Updates(map[string]any{
		"status": "closed", "closed_at": now, "result_json": resultJson, "updated_at": now,
	}).Error; err != nil {
		return err
	}

	remainingCommission := commissionQuota
	remainingStake := totalStake
	createdSettlements := 0
	firstSettlementID := 0
	firstLedgerID := 0
	for i, m := range muts {
		settlementKey := fmt.Sprintf("%s:%s:%d:%d:%d", siteID, settlementID, i, m.UserID, m.Delta)
		var existing model.GameSettlement
		if err := tx.Where("settlement_key = ?", settlementKey).First(&existing).Error; err == nil {
			if firstSettlementID == 0 {
				firstSettlementID = existing.Id
			}
			continue
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		var ledger model.OpsFundLedger
		ledgerID := 0
		if err := tx.Where("site_id = ? AND settlement_id = ? AND mutation_index = ?", siteID, settlementID, i).First(&ledger).Error; err == nil {
			ledgerID = ledger.Id
			if firstLedgerID == 0 {
				firstLedgerID = ledger.Id
			}
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		stake, payout := 0, 0
		if m.Delta < 0 {
			stake = -m.Delta
		} else if m.Delta > 0 {
			payout = m.Delta
		}
		mutationCommission := 0
		if stake > 0 && totalStake > 0 && commissionQuota > 0 {
			if remainingStake <= stake {
				mutationCommission = remainingCommission
			} else {
				mutationCommission = commissionQuota * stake / totalStake
				if mutationCommission > remainingCommission {
					mutationCommission = remainingCommission
				}
			}
			remainingStake -= stake
			remainingCommission -= mutationCommission
		}
		entryID := 0
		metaJson := gameMutationAuditJSON(action, payload, m, i, gameCode, platform, groupID)
		if stake > 0 {
			entry := model.GameEntry{SiteId: siteID, RoundId: round.Id, UserId: m.UserID, StakeQuota: stake, EntryPayloadJson: metaJson, Status: "entered", CreatedAt: now, UpdatedAt: now}
			if err := tx.Create(&entry).Error; err != nil {
				return err
			}
			entryID = entry.Id
		}
		settlement := model.GameSettlement{
			SiteId: siteID, RoundId: round.Id, EntryId: entryID, UserId: m.UserID, SettlementKey: settlementKey,
			StakeQuota: stake, PayoutQuota: payout, CommissionQuota: mutationCommission, RefundQuota: 0,
			OpsFundLedgerId: ledgerID, Status: "closed", MetadataJson: metaJson, CreatedAt: now, UpdatedAt: now,
		}
		if err := tx.Create(&settlement).Error; err != nil {
			return err
		}
		createdSettlements++
		if firstSettlementID == 0 {
			firstSettlementID = settlement.Id
		}
	}
	if createdSettlements == 0 {
		return nil
	}
	if commissionQuota > 0 {
		var existingCommission int64
		if err := tx.Model(&model.GameCommission{}).Where("site_id = ? AND round_id = ? AND game_code = ?", siteID, round.Id, gameCode).Count(&existingCommission).Error; err != nil {
			return err
		}
		if existingCommission == 0 {
			commission := model.GameCommission{SiteId: siteID, RoundId: round.Id, SettlementId: firstSettlementID, UserId: 0, GameCode: gameCode, GroupId: groupID, CommissionQuota: commissionQuota, RateBps: 0, OpsFundLedgerId: firstLedgerID, CreatedAt: now}
			if err := tx.Create(&commission).Error; err != nil {
				return err
			}
		}
	}
	if agentGameUsesJackpot(gameCode) && jackpotDelta != 0 {
		if err := applyAgentGameJackpotDeltaTx(tx, siteID, platform, groupID, gameCode, round.Id, firstSettlementID, jackpotDelta, "settlement:"+settlementID, now); err != nil {
			return err
		}
	}
	return updateAgentGroupGameMetricsTx(tx, siteID, platform, groupID, len(players), totalStake, totalPayout, commissionQuota, now)
}

func gameMutationAuditJSON(action *model.AgentAction, payload map[string]any, m model.QuotaMutation, index int, gameCode string, platform string, groupID string) string {
	meta := map[string]any{}
	if strings.TrimSpace(m.MetadataJson) != "" {
		_ = json.Unmarshal([]byte(m.MetadataJson), &meta)
	}
	meta["agent_action_id"] = action.Id
	meta["settlement_mutation_index"] = index
	meta["game_code"] = gameCode
	meta["platform"] = platform
	meta["group_id"] = groupID
	meta["source_type"] = m.SourceType
	meta["pool_type"] = m.PoolType
	meta["delta"] = m.Delta
	meta["user_id"] = m.UserID
	meta["room_id"] = firstAgentNonEmpty(agentPayloadString(payload, "room_id", "group_id", "chat_id"), groupID)
	return mustAgentJSON(meta)
}

func firstAgentGameCodeFromMutations(muts []model.QuotaMutation) string {
	for _, m := range muts {
		meta := map[string]any{}
		if strings.TrimSpace(m.MetadataJson) != "" {
			_ = json.Unmarshal([]byte(m.MetadataJson), &meta)
		}
		if g := normalizeAgentGameCode(firstAgentNonEmpty(agentPayloadString(meta, "game_code", "game"), m.Remark)); g != "" && g != "game" {
			return g
		}
	}
	return ""
}

func normalizeAgentGameCode(raw string) string {
	s := strings.ToLower(strings.TrimSpace(raw))
	s = strings.ReplaceAll(s, " ", "_")
	known := []string{"banker_guess", "duel_compare", "duel_idiom", "duel_rps", "redpacket", "treasure", "lottery", "predict", "bounty", "dice", "wheel", "fortune", "quiz", "checkin", "verify", "invite", "leaderboard"}
	for _, k := range known {
		if strings.HasPrefix(s, k) {
			return k
		}
	}
	if s == "" {
		return "game"
	}
	if idx := strings.Index(s, "_"); idx > 0 {
		return s[:idx]
	}
	return s
}

func agentGameUsesJackpot(gameCode string) bool {
	switch normalizeAgentGameCode(gameCode) {
	case "lottery", "treasure":
		return true
	default:
		return false
	}
}

func applyAgentGameJackpotDeltaTx(tx *gorm.DB, siteID string, platform string, groupID string, gameCode string, roundID int, settlementID int, delta int, remark string, now int64) error {
	acc := model.GameJackpotAccount{SiteId: siteID, Platform: platform, GroupId: groupID, GameCode: gameCode, BalanceQuota: 0, Status: "active", CreatedAt: now, UpdatedAt: now}
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&acc).Error; err != nil {
		return err
	}
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("site_id = ? AND platform = ? AND group_id = ? AND game_code = ?", siteID, platform, groupID, gameCode).First(&acc).Error; err != nil {
		return err
	}
	newBalance := acc.BalanceQuota + delta
	if err := tx.Model(&model.GameJackpotAccount{}).Where("id = ?", acc.Id).Updates(map[string]any{"balance_quota": newBalance, "updated_at": now}).Error; err != nil {
		return err
	}
	ledger := model.GameJackpotLedger{SiteId: siteID, JackpotAccountId: acc.Id, RoundId: roundID, SettlementId: settlementID, DeltaQuota: delta, BalanceAfter: newBalance, SourceType: "game_jackpot_delta", Remark: remark, CreatedAt: now}
	return tx.Create(&ledger).Error
}

func updateAgentGroupGameMetricsTx(tx *gorm.DB, siteID string, platform string, groupID string, playerCount int, stakeQuota int, payoutQuota int, commissionQuota int, now int64) error {
	if strings.TrimSpace(groupID) == "" {
		groupID = "unknown"
	}
	metricDate := time.Unix(now, 0).Format("2006-01-02")
	metric := model.GroupMetricsDaily{SiteId: siteID, Platform: platform, GroupId: groupID, MetricDate: metricDate, CreatedAt: now, UpdatedAt: now}
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&metric).Error; err != nil {
		return err
	}
	return tx.Model(&model.GroupMetricsDaily{}).Where("site_id = ? AND platform = ? AND group_id = ? AND metric_date = ?", siteID, platform, groupID, metricDate).Updates(map[string]any{
		"game_rounds":       gorm.Expr("game_rounds + ?", 1),
		"game_players":      gorm.Expr("game_players + ?", playerCount),
		"stake_quota":       gorm.Expr("stake_quota + ?", stakeQuota),
		"payout_quota":      gorm.Expr("payout_quota + ?", payoutQuota),
		"commission_quota":  gorm.Expr("commission_quota + ?", commissionQuota),
		"reward_cost_quota": gorm.Expr("reward_cost_quota + ?", payoutQuota),
		"updated_at":        now,
	}).Error
}

func grantAgentQuota(action *model.AgentAction, userID int, quota int) error {
	cfg := operation_setting.GetAgentSetting()
	if quota == 0 {
		return errors.New("quota must be non-zero")
	}
	gameAction := false
	adminAuthorized := false
	rawPayload := map[string]any{}
	if strings.TrimSpace(action.PayloadJson) != "" {
		if err := json.Unmarshal([]byte(action.PayloadJson), &rawPayload); err == nil {
			gameAction = agentBoolPayload(rawPayload, "game_action")
			adminAuthorized = agentBoolPayload(rawPayload, "admin_chatops_authorized")
		}
	}
	if cfg.SingleActionLimitQuota > 0 && absAgentInt(quota) > cfg.SingleActionLimitQuota && !gameAction && !adminAuthorized {
		return fmt.Errorf("quota exceeds single action limit: %d > %d", absAgentInt(quota), cfg.SingleActionLimitQuota)
	}
	poolType := normalizeAgentStringFallback(action.BudgetPool, "daily")
	idem := firstAgentNonEmpty(action.IdempotencyKey, fmt.Sprintf("agent-action-%d", action.Id))
	sourceType := agentQuotaSourceType(action, rawPayload, quota, poolType, gameAction)
	settlementID := agentQuotaSettlementID(action, rawPayload)
	mutationIndex := agentQuotaMutationIndex(rawPayload)
	metadataJson := agentQuotaMetadataJSON(action, rawPayload, userID, quota, poolType, sourceType, settlementID, mutationIndex)
	if err := model.DB.Transaction(func(tx *gorm.DB) error {
		return model.ApplyQuotaMutationTx(tx, action.SiteId, userID, quota, poolType, sourceType, idem, action.Reason, action.Id, settlementID, mutationIndex, metadataJson)
	}); err != nil {
		return err
	}
	recordGameQuotaLog(userID, quota, normalizeAgentStringFallback(action.BudgetPool, "game"), firstAgentNonEmpty(action.Reason, "game_quota"))
	return nil
}

// recordGameQuotaLog 把游戏额度奖励/扣款写入使用日志。
func recordGameQuotaLog(userID int, quota int, pool, reason string) {
	content := fmt.Sprintf("游戏额度奖励 +%s（%s）", agentQuotaUSDText(absAgentInt(quota)), reason)
	logType := model.LogTypeTopup
	if quota < 0 {
		content = fmt.Sprintf("游戏额度扣除 -%s（%s）", agentQuotaUSDText(absAgentInt(quota)), reason)
		logType = model.LogTypeManage
	}
	l := &model.Log{
		UserId:    userID,
		CreatedAt: time.Now().Unix(),
		Type:      logType,
		Content:   content,
		ModelName: "游戏@" + pool,
		Quota:     quota,
	}
	if err := model.LOG_DB.Create(l).Error; err != nil {
		fmt.Printf("[agent] record game quota log failed: %v\n", err)
	}
}

func humanAgentBudgetSummary(pools []model.AgentBudgetPool) string {
	if len(pools) == 0 {
		return "今日预算（日控）还没有生成任何池。"
	}
	parts := make([]string, 0, len(pools))
	for _, pool := range pools {
		remaining := pool.TotalQuota - pool.UsedQuota - pool.FrozenQuota
		parts = append(parts, fmt.Sprintf("%s 已用 %s / 上限 %s，剩余 %s", agentPoolDisplayName(pool.PoolType), agentQuotaUSDText(pool.UsedQuota), agentQuotaUSDText(pool.TotalQuota), agentQuotaUSDText(remaining)))
	}
	return "今日预算（日控）：\n- " + strings.Join(parts, "\n- ")
}

func humanAgentFundSummary(balanceQuota int, todayIncome int, todayExpense int, recentCount int) string {
	return fmt.Sprintf("运营基金余额 %s；今日收入 %s，今日支出 %s；已返回最近 %d 条流水。", agentQuotaUSDText(balanceQuota), agentQuotaUSDText(todayIncome), agentQuotaUSDText(todayExpense), recentCount)
}

func agentAnyInt64(v any) int64 {
	switch t := v.(type) {
	case int:
		return int64(t)
	case int8:
		return int64(t)
	case int16:
		return int64(t)
	case int32:
		return int64(t)
	case int64:
		return t
	case uint:
		return int64(t)
	case uint32:
		return int64(t)
	case uint64:
		return int64(t)
	case float32:
		return int64(t)
	case float64:
		return int64(t)
	case json.Number:
		if i, err := t.Int64(); err == nil {
			return i
		}
	case string:
		if i, err := strconv.ParseInt(strings.TrimSpace(t), 10, 64); err == nil {
			return i
		}
	}
	return 0
}

func agentAnyString(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return strings.TrimSpace(s)
	}
	return strings.TrimSpace(fmt.Sprint(v))
}

func agentQuotaUSDText64(quota int64) string {
	return fmt.Sprintf("$%.2f", float64(quota)/float64(common.QuotaPerUnit))
}

func agentFundObservabilitySummary(obs map[string]any) string {
	parts := make([]string, 0, 6)
	if game, ok := obs["game_audit"].(map[string]any); ok {
		rounds := agentAnyInt64(game["round_count"])
		commission := agentAnyInt64(game["commission_quota"])
		if rounds > 0 || commission != 0 {
			parts = append(parts, fmt.Sprintf("游戏 %d 局 / 抽佣 %s", rounds, agentQuotaUSDText64(commission)))
		}
	}
	if invite, ok := obs["invite_audit"].(map[string]any); ok {
		links := agentAnyInt64(invite["link_count"])
		edges := agentAnyInt64(invite["edge_count"])
		claims := agentAnyInt64(invite["claim_count"])
		if links > 0 || edges > 0 || claims > 0 {
			parts = append(parts, fmt.Sprintf("邀请 link %d / join %d / claim %d", links, edges, claims))
		}
	}
	if groups, ok := obs["group_metrics_today"].([]map[string]any); ok && len(groups) > 0 {
		top := groups[0]
		parts = append(parts, fmt.Sprintf("群组 %s/%s 佣金 %s", agentAnyString(top["platform"]), agentAnyString(top["group_id"]), agentQuotaUSDText64(agentAnyInt64(top["commission_quota"]))))
	}
	if jackpot, ok := obs["jackpot_audit"].(map[string]any); ok {
		accounts := agentAnyInt64(jackpot["account_count"])
		balance := agentAnyInt64(jackpot["balance_quota"])
		if accounts > 0 || balance > 0 {
			parts = append(parts, fmt.Sprintf("奖池 %d 个 / 余额 %s", accounts, agentQuotaUSDText64(balance)))
		}
	}
	if checks, ok := obs["checkin_audit_today"].([]map[string]any); ok && len(checks) > 0 {
		parts = append(parts, fmt.Sprintf("签到渠道 %s", agentAnyString(checks[0]["channel"])))
	}
	return strings.Join(parts, "；")
}

func agentLedgerSourceBreakdown(siteID string, since int64) []map[string]any {
	type row struct {
		SourceType   string
		Count        int64
		IncomeQuota  int64
		ExpenseQuota int64
		NetQuota     int64
	}
	var rows []row
	q := model.DB.Model(&model.OpsFundLedger{}).Select(
		"COALESCE(NULLIF(source_type, ''), 'unknown') AS source_type, COUNT(*) AS count, COALESCE(SUM(CASE WHEN delta_quota > 0 THEN delta_quota ELSE 0 END), 0) AS income_quota, COALESCE(SUM(CASE WHEN delta_quota < 0 THEN -delta_quota ELSE 0 END), 0) AS expense_quota, COALESCE(SUM(delta_quota), 0) AS net_quota",
	).Where("site_id = ?", siteID)
	if since > 0 {
		q = q.Where("created_at >= ?", since)
	}
	if err := q.Group("COALESCE(NULLIF(source_type, ''), 'unknown')").Order("net_quota desc").Scan(&rows).Error; err != nil {
		return nil
	}
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		out = append(out, map[string]any{
			"source_type":   row.SourceType,
			"count":         row.Count,
			"income_quota":  row.IncomeQuota,
			"income_usd":    fmt.Sprintf("%.4f", float64(row.IncomeQuota)/float64(common.QuotaPerUnit)),
			"expense_quota": row.ExpenseQuota,
			"expense_usd":   fmt.Sprintf("%.4f", float64(row.ExpenseQuota)/float64(common.QuotaPerUnit)),
			"net_quota":     row.NetQuota,
			"net_usd":       fmt.Sprintf("%.4f", float64(row.NetQuota)/float64(common.QuotaPerUnit)),
		})
	}
	return out
}

func agentGameAudit(siteID string) map[string]any {
	type commissionRow struct {
		GameCode        string
		Count           int64
		CommissionQuota int64
	}
	type groupRow struct {
		Platform        string
		GroupID         string
		GameRounds      int64
		GamePlayers     int64
		StakeQuota      int64
		PayoutQuota     int64
		CommissionQuota int64
		RewardCostQuota int64
	}
	var roundCount, entryCount, settlementCount, commissionCount int64
	_ = model.DB.Model(&model.GameRound{}).Where("site_id = ?", siteID).Count(&roundCount).Error
	_ = model.DB.Model(&model.GameEntry{}).Where("site_id = ?", siteID).Count(&entryCount).Error
	_ = model.DB.Model(&model.GameSettlement{}).Where("site_id = ?", siteID).Count(&settlementCount).Error
	_ = model.DB.Model(&model.GameCommission{}).Where("site_id = ?", siteID).Count(&commissionCount).Error
	var aggs struct {
		StakeQuota      int64
		PayoutQuota     int64
		CommissionQuota int64
	}
	_ = model.DB.Model(&model.GameSettlement{}).Select("COALESCE(SUM(stake_quota), 0) AS stake_quota, COALESCE(SUM(payout_quota), 0) AS payout_quota, COALESCE(SUM(commission_quota), 0) AS commission_quota").Where("site_id = ?", siteID).Scan(&aggs).Error
	var topCommissions []commissionRow
	_ = model.DB.Model(&model.GameCommission{}).Select("game_code AS game_code, COUNT(*) AS count, COALESCE(SUM(commission_quota), 0) AS commission_quota").Where("site_id = ?", siteID).Group("game_code").Order("commission_quota desc").Limit(10).Scan(&topCommissions).Error
	var topGroups []groupRow
	today := model.AgentBusinessDateAt(time.Now())
	_ = model.DB.Model(&model.GroupMetricsDaily{}).Select("platform, group_id, SUM(game_rounds) AS game_rounds, SUM(game_players) AS game_players, SUM(stake_quota) AS stake_quota, SUM(payout_quota) AS payout_quota, SUM(commission_quota) AS commission_quota, SUM(reward_cost_quota) AS reward_cost_quota").Where("site_id = ? AND metric_date = ?", siteID, today).Group("platform, group_id").Order("commission_quota desc").Limit(10).Scan(&topGroups).Error
	commissions := make([]map[string]any, 0, len(topCommissions))
	for _, row := range topCommissions {
		commissions = append(commissions, map[string]any{
			"game_code":        row.GameCode,
			"count":            row.Count,
			"commission_quota": row.CommissionQuota,
			"commission_usd":   fmt.Sprintf("%.4f", float64(row.CommissionQuota)/float64(common.QuotaPerUnit)),
		})
	}
	groups := make([]map[string]any, 0, len(topGroups))
	for _, row := range topGroups {
		groups = append(groups, map[string]any{
			"platform":          row.Platform,
			"group_id":          row.GroupID,
			"game_rounds":       row.GameRounds,
			"game_players":      row.GamePlayers,
			"stake_quota":       row.StakeQuota,
			"payout_quota":      row.PayoutQuota,
			"commission_quota":  row.CommissionQuota,
			"reward_cost_quota": row.RewardCostQuota,
			"commission_usd":    fmt.Sprintf("%.4f", float64(row.CommissionQuota)/float64(common.QuotaPerUnit)),
		})
	}
	return map[string]any{
		"round_count":      roundCount,
		"entry_count":      entryCount,
		"settlement_count": settlementCount,
		"commission_count": commissionCount,
		"stake_quota":      aggs.StakeQuota,
		"payout_quota":     aggs.PayoutQuota,
		"commission_quota": aggs.CommissionQuota,
		"top_commissions":  commissions,
		"top_groups_today": groups,
	}
}

func agentInviteAudit(siteID string) map[string]any {
	type statusRow struct {
		Status string
		Count  int64
		Quota  int64
	}
	type eventRow struct {
		EventType string
		Count     int64
	}
	var campaignCount, linkCount, edgeCount, eventCount, claimCount int64
	_ = model.DB.Model(&model.InviteCampaign{}).Where("site_id = ?", siteID).Count(&campaignCount).Error
	_ = model.DB.Model(&model.InviteLink{}).Where("site_id = ?", siteID).Count(&linkCount).Error
	_ = model.DB.Model(&model.InviteEdge{}).Where("site_id = ?", siteID).Count(&edgeCount).Error
	_ = model.DB.Model(&model.InviteEvent{}).Where("site_id = ?", siteID).Count(&eventCount).Error
	_ = model.DB.Model(&model.InviteRewardClaim{}).Where("site_id = ?", siteID).Count(&claimCount).Error
	var claimRows []statusRow
	_ = model.DB.Model(&model.InviteRewardClaim{}).Select("status AS status, COUNT(*) AS count, COALESCE(SUM(quota), 0) AS quota").Where("site_id = ?", siteID).Group("status").Order("quota desc").Scan(&claimRows).Error
	var edgeRows []statusRow
	_ = model.DB.Model(&model.InviteEdge{}).Select("status AS status, COUNT(*) AS count, 0 AS quota").Where("site_id = ?", siteID).Group("status").Order("count desc").Scan(&edgeRows).Error
	var eventRows []eventRow
	_ = model.DB.Model(&model.InviteEvent{}).Select("event_type AS event_type, COUNT(*) AS count").Where("site_id = ?", siteID).Group("event_type").Order("count desc").Limit(10).Scan(&eventRows).Error
	claims := make([]map[string]any, 0, len(claimRows))
	for _, row := range claimRows {
		claims = append(claims, map[string]any{"status": row.Status, "count": row.Count, "quota": row.Quota, "quota_usd": fmt.Sprintf("%.4f", float64(row.Quota)/float64(common.QuotaPerUnit))})
	}
	edges := make([]map[string]any, 0, len(edgeRows))
	for _, row := range edgeRows {
		edges = append(edges, map[string]any{"status": row.Status, "count": row.Count})
	}
	events := make([]map[string]any, 0, len(eventRows))
	for _, row := range eventRows {
		events = append(events, map[string]any{"event_type": row.EventType, "count": row.Count})
	}
	return map[string]any{
		"campaign_count": campaignCount,
		"link_count":     linkCount,
		"edge_count":     edgeCount,
		"event_count":    eventCount,
		"claim_count":    claimCount,
		"claim_statuses": claims,
		"edge_statuses":  edges,
		"events_top":     events,
	}
}

func agentJackpotAudit(siteID string) map[string]any {
	type row struct {
		Platform     string
		GroupID      string
		GameCode     string
		BalanceQuota int64
	}
	var accountCount int64
	_ = model.DB.Model(&model.GameJackpotAccount{}).Where("site_id = ?", siteID).Count(&accountCount).Error
	var totals struct {
		BalanceQuota int64
	}
	_ = model.DB.Model(&model.GameJackpotAccount{}).Select("COALESCE(SUM(balance_quota), 0) AS balance_quota").Where("site_id = ?", siteID).Scan(&totals).Error
	var rows []row
	_ = model.DB.Model(&model.GameJackpotAccount{}).Select("platform, group_id, game_code, balance_quota").Where("site_id = ?", siteID).Order("balance_quota desc").Limit(10).Scan(&rows).Error
	top := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		top = append(top, map[string]any{"platform": row.Platform, "group_id": row.GroupID, "game_code": row.GameCode, "balance_quota": row.BalanceQuota, "balance_usd": fmt.Sprintf("%.4f", float64(row.BalanceQuota)/float64(common.QuotaPerUnit))})
	}
	return map[string]any{"account_count": accountCount, "balance_quota": totals.BalanceQuota, "balance_usd": fmt.Sprintf("%.4f", float64(totals.BalanceQuota)/float64(common.QuotaPerUnit)), "top_accounts": top}
}

func agentCheckinAudit(todayDate string) []map[string]any {
	type row struct {
		Channel      string
		Count        int64
		QuotaAwarded int64
	}
	var rows []row
	_ = model.DB.Model(&model.Checkin{}).Select("channel, COUNT(*) AS count, COALESCE(SUM(quota_awarded), 0) AS quota_awarded").Where("checkin_date = ?", todayDate).Group("channel").Order("count desc").Scan(&rows).Error
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		out = append(out, map[string]any{"channel": row.Channel, "count": row.Count, "quota_awarded": row.QuotaAwarded, "quota_usd": fmt.Sprintf("%.4f", float64(row.QuotaAwarded)/float64(common.QuotaPerUnit))})
	}
	return out
}

func agentGroupMetricsToday(siteID string, todayDate string) []map[string]any {
	type row struct {
		Platform        string
		GroupID         string
		GameRounds      int64
		GamePlayers     int64
		StakeQuota      int64
		PayoutQuota     int64
		CommissionQuota int64
		RewardCostQuota int64
	}
	var rows []row
	_ = model.DB.Model(&model.GroupMetricsDaily{}).Select("platform, group_id, SUM(game_rounds) AS game_rounds, SUM(game_players) AS game_players, SUM(stake_quota) AS stake_quota, SUM(payout_quota) AS payout_quota, SUM(commission_quota) AS commission_quota, SUM(reward_cost_quota) AS reward_cost_quota").Where("site_id = ? AND metric_date = ?", siteID, todayDate).Group("platform, group_id").Order("commission_quota desc, game_rounds desc").Limit(10).Scan(&rows).Error
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		out = append(out, map[string]any{"platform": row.Platform, "group_id": row.GroupID, "game_rounds": row.GameRounds, "game_players": row.GamePlayers, "stake_quota": row.StakeQuota, "payout_quota": row.PayoutQuota, "commission_quota": row.CommissionQuota, "reward_cost_quota": row.RewardCostQuota, "commission_usd": fmt.Sprintf("%.4f", float64(row.CommissionQuota)/float64(common.QuotaPerUnit))})
	}
	return out
}

func agentBuildFundObservability(siteID string, dayStart time.Time) map[string]any {
	todayDate := dayStart.Format("2006-01-02")
	obs := map[string]any{
		"source_breakdown":       agentLedgerSourceBreakdown(siteID, 0),
		"source_breakdown_today": agentLedgerSourceBreakdown(siteID, dayStart.Unix()),
		"game_audit":             agentGameAudit(siteID),
		"invite_audit":           agentInviteAudit(siteID),
		"group_metrics_today":    agentGroupMetricsToday(siteID, todayDate),
		"jackpot_audit":          agentJackpotAudit(siteID),
		"checkin_audit_today":    agentCheckinAudit(todayDate),
	}
	obs["summary"] = agentFundObservabilitySummary(obs)
	return obs
}

func agentPoolDisplayName(pool string) string {
	switch strings.TrimSpace(pool) {
	case "game":
		return "游戏池"
	case "activity":
		return "活动池"
	case "growth":
		return "增长池"
	case "community":
		return "社区池"
	case "ops_comp":
		return "运营补偿池"
	case "daily":
		return "日池"
	default:
		return firstAgentNonEmpty(strings.TrimSpace(pool), "预算池")
	}
}

func agentQuotaSourceType(action *model.AgentAction, payload map[string]any, quota int, poolType string, gameAction bool) string {
	if explicit := strings.TrimSpace(agentPayloadString(payload, "source_type")); explicit != "" {
		return explicit
	}
	if quota < 0 {
		return "game_stake"
	}
	if gameAction {
		return "game_payout"
	}
	switch strings.TrimSpace(poolType) {
	case "ops_comp":
		return "ops_reward"
	case "community":
		return "community_reward"
	case "growth":
		return "growth_reward"
	case "activity":
		return "activity_reward"
	default:
		return "reward_grant"
	}
}

func agentQuotaSettlementID(action *model.AgentAction, payload map[string]any) string {
	if settlementID := strings.TrimSpace(agentPayloadString(payload, "settlement_id", "settlement")); settlementID != "" {
		if len(settlementID) > 64 {
			return settlementID[:64]
		}
		return settlementID
	}
	if action != nil {
		if idem := strings.TrimSpace(action.IdempotencyKey); idem != "" {
			return agentStableSettlementID(idem)
		}
	}
	if action != nil && action.Id > 0 {
		return fmt.Sprintf("action-%d", action.Id)
	}
	return ""
}

func agentStableSettlementID(seed string) string {
	seed = strings.TrimSpace(seed)
	if seed == "" {
		return ""
	}
	if len(seed) <= 64 {
		return seed
	}
	sum := sha256.Sum256([]byte(seed))
	prefix := seed
	if len(prefix) > 47 {
		prefix = prefix[:47]
	}
	return prefix + "-" + hex.EncodeToString(sum[:])[:16]
}

func agentQuotaMutationIndex(payload map[string]any) int {
	index := int(agentPayloadFloat(payload, "mutation_index", "settlement_mutation_index"))
	if index < 0 {
		return 0
	}
	return index
}

func agentQuotaMetadataJSON(action *model.AgentAction, payload map[string]any, userID int, quota int, poolType string, sourceType string, settlementID string, mutationIndex int) string {
	meta := map[string]any{
		"action_type":               action.ActionType,
		"agent_action_id":           action.Id,
		"budget_pool":               poolType,
		"source_type":               sourceType,
		"user_id":                   userID,
		"quota":                     quota,
		"reason":                    action.Reason,
		"settlement_id":             settlementID,
		"mutation_index":            mutationIndex,
		"settlement_mutation_index": mutationIndex,
	}
	if roomID := firstAgentNonEmpty(agentPayloadString(payload, "room_id", "group_id", "chat_id")); roomID != "" {
		meta["room_id"] = roomID
	}
	if gameCode := strings.TrimSpace(agentPayloadString(payload, "game_code", "game")); gameCode != "" {
		meta["game_code"] = normalizeAgentGameCode(gameCode)
	}
	if roundKey := strings.TrimSpace(agentPayloadString(payload, "round_key", "round")); roundKey != "" {
		meta["round_key"] = roundKey
	}
	if idem := strings.TrimSpace(action.IdempotencyKey); idem != "" {
		meta["idempotency_key"] = idem
	}
	if source := firstAgentNonEmpty(agentPayloadString(payload, "chatops_source", "source", "platform")); source != "" {
		meta["source"] = source
	}
	if targetExternalID := strings.TrimSpace(agentPayloadString(payload, "target_external_id")); targetExternalID != "" {
		meta["target_external_id"] = targetExternalID
	}
	if issuer := strings.TrimSpace(agentPayloadString(payload, "issuer_external_id")); issuer != "" {
		meta["issuer_external_id"] = issuer
	}
	return mustAgentJSON(meta)
}

func absAgentInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func AgentSendChatOpsReply(ctx context.Context, source string, roomID string, text string) error {
	return AgentSendChatOpsReplyTarget(ctx, source, roomID, "", text)
}

func AgentSendChatOpsReplyTarget(ctx context.Context, source string, roomID string, userExternalID string, text string) error {
	if text == "" {
		return nil
	}
	switch strings.ToLower(strings.TrimSpace(source)) {
	case "qq":
		if strings.TrimSpace(roomID) == "" && strings.TrimSpace(userExternalID) != "" {
			return AgentSendQQPrivateMessage(ctx, userExternalID, text)
		}
		return AgentSendQQGroupMessage(ctx, roomID, text)
	case "tg", "telegram":
		return AgentSendTelegramMessage(ctx, firstAgentNonEmpty(roomID, userExternalID), text)
	case "community":
		return CommunitySendRoomMessage(ctx, text)
	default:
		return nil
	}
}

func AgentSendQQGroupMessage(ctx context.Context, groupID string, text string) error {
	cfg := operation_setting.GetAgentSetting()
	if !cfg.QQBotEnabled {
		return errors.New("qq bot is disabled")
	}
	baseURL := strings.TrimSpace(cfg.QQOneBotURL)
	if baseURL == "" {
		return errors.New("qq onebot url is empty")
	}
	groupID = firstAgentNonEmpty(groupID, cfg.QQGroupID)
	if groupID == "" {
		return errors.New("qq group id is empty")
	}
	payload := map[string]any{"group_id": parseAgentNumberish(groupID), "message": text}
	return postAgentJSON(ctx, strings.TrimRight(baseURL, "/")+"/send_group_msg", cfg.QQAccessToken, payload, nil)
}
func AgentSendQQPrivateMessage(ctx context.Context, userID string, text string) error {
	cfg := operation_setting.GetAgentSetting()
	if !cfg.QQBotEnabled {
		return errors.New("qq bot is disabled")
	}
	baseURL := strings.TrimSpace(cfg.QQOneBotURL)
	if baseURL == "" {
		return errors.New("qq onebot url is empty")
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return errors.New("qq private user id is empty")
	}
	payload := map[string]any{"user_id": parseAgentNumberish(userID), "message": text}
	return postAgentJSON(ctx, strings.TrimRight(baseURL, "/")+"/send_private_msg", cfg.QQAccessToken, payload, nil)
}

func AgentSendTelegramMessage(ctx context.Context, chatID string, text string) error {
	cfg := operation_setting.GetAgentSetting()
	if !cfg.TGBotEnabled {
		return errors.New("telegram bot is disabled")
	}
	token := strings.TrimSpace(cfg.TGBotToken)
	chatID = firstAgentNonEmpty(chatID, cfg.TGChatID)
	if token == "" || chatID == "" {
		return errors.New("telegram token or chat id is empty")
	}
	payload := map[string]any{"chat_id": parseAgentNumberish(chatID), "text": text, "disable_web_page_preview": true}
	return postAgentJSON(ctx, "https://api.telegram.org/bot"+url.PathEscape(token)+"/sendMessage", "", payload, nil)
}

func postAgentJSON(ctx context.Context, endpoint string, bearer string, payload any, out any) error {
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if strings.TrimSpace(bearer) != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(bearer))
	}
	client := http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("agent http post status=%d body=%s", resp.StatusCode, truncateAgentText(string(respBody), 500))
	}
	if out != nil && len(respBody) > 0 {
		return json.Unmarshal(respBody, out)
	}
	return nil
}

func agentBoolPayload(payload map[string]any, keys ...string) bool {
	for _, key := range keys {
		if v, ok := payload[key]; ok {
			switch t := v.(type) {
			case bool:
				return t
			case string:
				return t == "true" || t == "1" || strings.EqualFold(t, "yes")
			case float64:
				return t != 0
			}
		}
	}
	return false
}

func agentPayloadString(payload map[string]any, keys ...string) string {
	for _, key := range keys {
		if v, ok := payload[key]; ok {
			switch t := v.(type) {
			case string:
				if normalized := normalizeAgentString(t); normalized != "" {
					return normalized
				}
			case float64:
				return strconv.FormatInt(int64(t), 10)
			case int:
				return strconv.Itoa(t)
			}
		}
	}
	return ""
}

func normalizeAgentString(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || value == "<nil>" || strings.EqualFold(value, "nil") {
		return ""
	}
	return value
}

func normalizeAgentStringFallback(value string, fallback string) string {
	if normalized := normalizeAgentString(value); normalized != "" {
		return normalized
	}
	return normalizeAgentString(fallback)
}

func agentPayloadFloat(payload map[string]any, keys ...string) float64 {
	for _, key := range keys {
		if v, ok := payload[key]; ok {
			switch t := v.(type) {
			case float64:
				return t
			case int:
				return float64(t)
			case string:
				f, _ := strconv.ParseFloat(strings.TrimSpace(t), 64)
				return f
			}
		}
	}
	return 0
}

func parseAgentNumberish(value string) any {
	value = strings.TrimSpace(value)
	if i, err := strconv.ParseInt(value, 10, 64); err == nil {
		return i
	}
	return value
}

func agentStateSummary(state *AgentSiteState) string {
	if state == nil {
		return "site state unavailable"
	}
	availableGroups := 0
	for _, g := range state.Groups {
		if g.Available {
			availableGroups++
		}
	}
	return fmt.Sprintf("%s channels=%d/%d groups=%d/%d avg=%dms recent_errors=%d", state.SiteName, state.Channels.Enabled, state.Channels.Total, availableGroups, len(state.Groups), state.Channels.AverageResponseMS, len(state.RecentErrors))
}
func AgentNotifyApprovalRequired(ctx context.Context, action *model.AgentAction) {
	if action == nil || action.ApprovalId <= 0 {
		return
	}
	cfg := operation_setting.GetAgentSetting()
	msg := buildAgentApprovalPrivateMessage(action)
	if strings.TrimSpace(msg) == "" {
		return
	}
	payload := map[string]any{}
	_ = json.Unmarshal([]byte(action.PayloadJson), &payload)
	source := firstAgentNonEmpty(agentPayloadString(payload, "chatops_source"), "qq")
	issuerID := firstAgentNonEmpty(agentPayloadString(payload, "issuer_external_id"))
	sent := false
	trySend := func(target string) {
		if sent || strings.TrimSpace(target) == "" {
			return
		}
		if err := AgentSendChatOpsReplyTarget(ctx, source, "", target, msg); err == nil {
			sent = true
		} else {
			_ = model.DB.Create(&model.AgentEvent{SiteId: action.SiteId, EventType: "approval.notify_failed", Source: source, Severity: "warning", Status: "open", ActorExternalId: target, Title: action.ActionType, PayloadJson: action.PayloadJson, ResultJson: mustAgentJSON(map[string]any{"error": err.Error(), "approval_id": action.ApprovalId})}).Error
		}
	}
	trySend(issuerID)
	if !sent {
		for _, adminID := range splitAgentCSV(cfg.ChatOpsAdminExternalIDs) {
			trySend(adminID)
		}
	}
	status := "sent"
	if !sent {
		status = "no_private_admin_target"
	}
	_ = model.DB.Create(&model.AgentEvent{SiteId: action.SiteId, EventType: "approval.notify", Source: "chatops", Severity: "info", Status: "closed", Title: action.ActionType, PayloadJson: action.PayloadJson, ResultJson: mustAgentJSON(map[string]any{"approval_id": action.ApprovalId, "status": status})}).Error
}

func buildAgentApprovalPrivateMessage(action *model.AgentAction) string {
	payload := map[string]any{}
	_ = json.Unmarshal([]byte(action.PayloadJson), &payload)
	issuer := firstAgentNonEmpty(agentPayloadString(payload, "issuer_username"), agentPayloadString(payload, "issuer_external_id"), "unknown")
	source := firstAgentNonEmpty(agentPayloadString(payload, "chatops_source"), "agent")
	text := firstAgentNonEmpty(agentPayloadString(payload, "text", "message", "content"), action.Reason)
	return fmt.Sprintf(`【Agent 私聊审批】
站点：%s
审批ID：#%d
动作：%s
来源：%s / %s
风险：%s score=%d
目标：%s %s
额度：%d
原因：%s

通过：/agent 审批 %d 通过
拒绝：/agent 审批 %d 拒绝`, AgentSiteID(), action.ApprovalId, action.ActionType, source, issuer, action.RiskLevel, action.RiskScore, action.TargetType, action.TargetId, action.QuotaAmount, truncateAgentText(text, 500), action.ApprovalId, action.ApprovalId)
}

func isAgentNumericID(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	for _, r := range value {
		if r == '-' {
			continue
		}
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func executeQuizStateLoad(action *model.AgentAction, payload map[string]any) (map[string]any, error) {
	return loadQuizStateByPayload(AgentSiteID(), payload)
}

func executeQuizStateCommit(action *model.AgentAction, payload map[string]any) (map[string]any, error) {
	siteID := AgentSiteID()
	platform := agentQuizPlatform(payload)
	groupID := agentQuizGroupID(payload)
	scopeMode := agentQuizScopeMode(payload)
	roundPayload := agentQuizPayloadMap(payload["round"])
	if len(roundPayload) == 0 {
		return nil, errors.New("quiz round payload is empty")
	}
	roundKey := strings.TrimSpace(agentPayloadString(roundPayload, "round_key"))
	if roundKey == "" {
		roundKey = strings.TrimSpace(agentPayloadString(payload, "round_key"))
	}
	if roundKey == "" {
		return nil, errors.New("quiz round_key is required")
	}
	entryPayload := agentQuizPayloadMap(payload["entry"])
	resultPayload := agentQuizPayloadMap(payload["result"])
	closeRound := agentBoolPayload(payload, "close_round", "close")
	userID := int(agentPayloadFloat(payload, "user_id", "new_api_user_id", "resolved_new_api_user_id"))
	externalUserID := firstAgentNonEmpty(agentPayloadString(payload, "target_external_id", "external_user_id", "user_external_id"), action.TargetId)
	grantQuota := int(agentPayloadFloat(payload, "grant_quota", "reward_quota"))
	grantPool := firstAgentNonEmpty(agentPayloadString(payload, "budget_pool", "pool"), "activity")
	grantReason := firstAgentNonEmpty(agentPayloadString(payload, "grant_reason", "reason"), "quiz_correct")
	grantIdempotencyKey := strings.TrimSpace(agentPayloadString(payload, "grant_idempotency_key", "reward_idempotency_key"))
	now := time.Now().Unix()
	resultJSON := mustAgentJSON(resultPayload)
	if resultJSON == "{}" && closeRound {
		resultJSON = mustAgentJSON(map[string]any{"status": "closed", "round_key": roundKey, "closed_at": now})
	}
	if err := model.DB.Transaction(func(tx *gorm.DB) error {
		var round model.GameRound
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("round_key = ?", roundKey).First(&round).Error
		ruleJSON := mustAgentJSON(roundPayload)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			round = model.GameRound{SiteId: siteID, Platform: platform, GroupId: groupID, GameCode: "quiz", RoundKey: roundKey, Status: "open", RandomSeed: action.IdempotencyKey, RuleJson: ruleJSON, ResultJson: "{}", OpenedAt: now, CreatedAt: now, UpdatedAt: now}
			if closeRound {
				round.Status = "closed"
				round.ClosedAt = now
				round.ResultJson = resultJSON
			}
			if err := tx.Create(&round).Error; err != nil {
				return err
			}
		} else if err != nil {
			return err
		} else {
			updates := map[string]any{"rule_json": ruleJSON, "updated_at": now}
			if closeRound {
				updates["status"] = "closed"
				updates["closed_at"] = now
				updates["result_json"] = resultJSON
			} else if strings.TrimSpace(round.Status) == "" {
				updates["status"] = "open"
			}
			if err := tx.Model(&model.GameRound{}).Where("id = ?", round.Id).Updates(updates).Error; err != nil {
				return err
			}
		}
		if len(entryPayload) > 0 && (userID > 0 || externalUserID != "") {
			var entry model.GameEntry
			entryQuery := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("round_id = ?", round.Id)
			switch {
			case userID > 0 && externalUserID != "":
				entryQuery = entryQuery.Where("(user_id = ? OR external_user_id = ?)", userID, externalUserID)
			case userID > 0:
				entryQuery = entryQuery.Where("user_id = ?", userID)
			default:
				entryQuery = entryQuery.Where("external_user_id = ?", externalUserID)
			}
			err := entryQuery.Order("id desc").First(&entry).Error
			entryStatus := firstAgentNonEmpty(agentPayloadString(entryPayload, "status"), "active")
			entryJSON := mustAgentJSON(entryPayload)
			if errors.Is(err, gorm.ErrRecordNotFound) {
				entry = model.GameEntry{SiteId: siteID, RoundId: round.Id, UserId: userID, ExternalUserId: externalUserID, StakeQuota: 0, EntryPayloadJson: entryJSON, Status: entryStatus, CreatedAt: now, UpdatedAt: now}
				if err := tx.Create(&entry).Error; err != nil {
					return err
				}
			} else if err != nil {
				return err
			} else {
				updates := map[string]any{"status": entryStatus, "entry_payload_json": entryJSON, "updated_at": now}
				if userID > 0 {
					updates["user_id"] = userID
				}
				if externalUserID != "" {
					updates["external_user_id"] = externalUserID
				}
				if err := tx.Model(&model.GameEntry{}).Where("id = ?", entry.Id).Updates(updates).Error; err != nil {
					return err
				}
			}
		}
		if grantQuota != 0 {
			if userID <= 0 {
				return errors.New("quiz reward requires bound user")
			}
			if grantIdempotencyKey == "" {
				grantIdempotencyKey = fmt.Sprintf("quiz-reward:%s:%d", roundKey, userID)
			}
			metaJSON := mustAgentJSON(map[string]any{"game_code": "quiz", "round_key": roundKey, "platform": platform, "group_id": groupID, "scope_mode": scopeMode, "external_user_id": externalUserID})
			if err := model.ApplyQuotaMutationTx(tx, siteID, userID, grantQuota, grantPool, "quiz_reward", grantIdempotencyKey, grantReason, action.Id, roundKey, 0, metaJSON); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}
	dayStart := model.AgentBusinessDayStartAt(time.Now())
	var todayRounds int64
	_ = model.DB.Model(&model.GameRound{}).Where("site_id = ? AND platform = ? AND group_id = ? AND game_code = ? AND created_at >= ?", siteID, platform, groupID, "quiz", dayStart.Unix()).Count(&todayRounds).Error
	todayUserRounds, _ := loadQuizTodayUserRounds(siteID, platform, groupID, userID, externalUserID, dayStart.Unix())
	if closeRound {
		return map[string]any{"ok": true, "round_key": roundKey, "closed": true, "today_rounds": todayRounds, "today_user_rounds": todayUserRounds}, nil
	}
	data, err := loadQuizStateByPayload(siteID, payload)
	if err != nil {
		return map[string]any{"ok": true, "round_key": roundKey, "today_rounds": todayRounds, "today_user_rounds": todayUserRounds}, nil
	}
	data["grant_quota"] = grantQuota
	data["today_user_rounds"] = todayUserRounds
	return data, nil
}

func loadQuizTodayUserRounds(siteID, platform, groupID string, userID int, externalUserID string, dayStart int64) (int64, error) {
	if userID <= 0 && strings.TrimSpace(externalUserID) == "" {
		return 0, nil
	}
	query := model.DB.Model(&model.GameRound{}).Distinct("game_rounds.id").Joins("JOIN game_entries ge ON ge.round_id = game_rounds.id").Where("game_rounds.site_id = ? AND game_rounds.platform = ? AND game_rounds.group_id = ? AND game_rounds.game_code = ? AND game_rounds.created_at >= ?", siteID, platform, groupID, "quiz", dayStart)
	switch {
	case userID > 0 && strings.TrimSpace(externalUserID) != "":
		query = query.Where("(ge.user_id = ? OR ge.external_user_id = ?)", userID, externalUserID)
	case userID > 0:
		query = query.Where("ge.user_id = ?", userID)
	default:
		query = query.Where("ge.external_user_id = ?", externalUserID)
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func loadQuizStateByPayload(siteID string, payload map[string]any) (map[string]any, error) {
	platform := agentQuizPlatform(payload)
	groupID := agentQuizGroupID(payload)
	scopeMode := agentQuizScopeMode(payload)
	userID := int(agentPayloadFloat(payload, "user_id", "new_api_user_id", "resolved_new_api_user_id"))
	externalUserID := strings.TrimSpace(agentPayloadString(payload, "target_external_id", "external_user_id", "user_external_id"))
	dayStart := model.AgentBusinessDayStartAt(time.Now())
	out := map[string]any{"ok": true, "scope_mode": scopeMode, "entries": []map[string]any{}, "today_rounds": int64(0), "today_user_rounds": int64(0)}
	var todayRounds int64
	if err := model.DB.Model(&model.GameRound{}).Where("site_id = ? AND platform = ? AND group_id = ? AND game_code = ? AND created_at >= ?", siteID, platform, groupID, "quiz", dayStart.Unix()).Count(&todayRounds).Error; err == nil {
		out["today_rounds"] = todayRounds
	}
	if todayUserRounds, err := loadQuizTodayUserRounds(siteID, platform, groupID, userID, externalUserID, dayStart.Unix()); err == nil {
		out["today_user_rounds"] = todayUserRounds
	}
	query := model.DB.Model(&model.GameRound{}).Where("game_rounds.site_id = ? AND game_rounds.platform = ? AND game_rounds.group_id = ? AND game_rounds.game_code = ? AND game_rounds.status = ?", siteID, platform, groupID, "quiz", "open")
	if scopeMode == "per_group" {
		query = query.Order("updated_at desc, id desc")
	} else {
		if userID > 0 && externalUserID != "" {
			query = query.Joins("JOIN game_entries ge ON ge.round_id = game_rounds.id").Where("(ge.user_id = ? OR ge.external_user_id = ?)", userID, externalUserID)
		} else if userID > 0 {
			query = query.Joins("JOIN game_entries ge ON ge.round_id = game_rounds.id").Where("ge.user_id = ?", userID)
		} else if externalUserID != "" {
			query = query.Joins("JOIN game_entries ge ON ge.round_id = game_rounds.id").Where("ge.external_user_id = ?", externalUserID)
		} else {
			return out, nil
		}
		query = query.Order("game_rounds.updated_at desc, game_rounds.id desc")
	}
	var round model.GameRound
	if err := query.First(&round).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return out, nil
		}
		return nil, err
	}
	out["round"] = agentQuizRoundData(round)
	var rows []model.GameEntry
	if err := model.DB.Where("round_id = ?", round.Id).Order("id asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	latest := map[string]model.GameEntry{}
	order := make([]string, 0, len(rows))
	for _, row := range rows {
		key := agentQuizEntryIdentityKey(row)
		if key == "" {
			continue
		}
		if _, ok := latest[key]; !ok {
			order = append(order, key)
		}
		latest[key] = row
	}
	entries := make([]map[string]any, 0, len(order))
	answeredCount := 0
	lockedCount := 0
	for _, key := range order {
		entryMap := agentQuizEntryData(latest[key])
		entries = append(entries, entryMap)
		payloadMap := agentQuizPayloadMap(entryMap["payload"])
		status := strings.ToLower(strings.TrimSpace(firstAgentNonEmpty(agentPayloadString(entryMap, "status"), agentPayloadString(payloadMap, "status"))))
		if status == "answered" || agentBoolPayload(payloadMap, "answered") {
			answeredCount++
		}
		if status == "locked" || agentBoolPayload(payloadMap, "locked") {
			lockedCount++
		}
	}
	out["entries"] = entries
	out["answered_count"] = answeredCount
	out["locked_count"] = lockedCount
	return out, nil
}

func agentQuizPlatform(payload map[string]any) string {
	platform := firstAgentNonEmpty(agentPayloadString(payload, "platform", "chatops_source", "source"), "unknown")
	if strings.EqualFold(platform, "telegram") {
		return "tg"
	}
	return platform
}

func agentQuizGroupID(payload map[string]any) string {
	return firstAgentNonEmpty(agentPayloadString(payload, "room_id", "group_id", "chat_id"), "dm")
}

func agentQuizScopeMode(payload map[string]any) string {
	scope := strings.ToLower(strings.TrimSpace(agentPayloadString(payload, "scope_mode", "question_scope")))
	switch scope {
	case "group", "per_group", "shared", "room":
		return "per_group"
	default:
		return "per_user"
	}
}

func agentQuizPayloadMap(value any) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	if out, ok := value.(map[string]any); ok {
		return out
	}
	if out, ok := value.(map[string]interface{}); ok {
		return map[string]any(out)
	}
	if s, ok := value.(string); ok && strings.TrimSpace(s) != "" {
		var out map[string]any
		if err := json.Unmarshal([]byte(s), &out); err == nil && out != nil {
			return out
		}
	}
	return map[string]any{}
}
func agentQuizRoundData(round model.GameRound) map[string]any {
	return map[string]any{
		"id":         round.Id,
		"site_id":    round.SiteId,
		"platform":   round.Platform,
		"group_id":   round.GroupId,
		"game_code":  round.GameCode,
		"round_key":  round.RoundKey,
		"status":     round.Status,
		"opened_at":  round.OpenedAt,
		"closed_at":  round.ClosedAt,
		"created_at": round.CreatedAt,
		"updated_at": round.UpdatedAt,
		"rule":       agentQuizPayloadMap(round.RuleJson),
		"result":     agentQuizPayloadMap(round.ResultJson),
	}
}

func agentQuizEntryIdentityKey(entry model.GameEntry) string {
	if strings.TrimSpace(entry.ExternalUserId) != "" {
		return "ext:" + strings.TrimSpace(entry.ExternalUserId)
	}
	if entry.UserId > 0 {
		return fmt.Sprintf("uid:%d", entry.UserId)
	}
	return fmt.Sprintf("row:%d", entry.Id)
}

func agentQuizEntryData(entry model.GameEntry) map[string]any {
	payload := agentQuizPayloadMap(entry.EntryPayloadJson)
	return map[string]any{
		"id":               entry.Id,
		"round_id":         entry.RoundId,
		"user_id":          entry.UserId,
		"external_user_id": entry.ExternalUserId,
		"status":           entry.Status,
		"attempts":         agentPayloadFloat(payload, "attempts"),
		"created_at":       entry.CreatedAt,
		"updated_at":       entry.UpdatedAt,
		"payload":          payload,
	}
}
