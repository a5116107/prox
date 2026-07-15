package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"gorm.io/gorm"
)

type AgentChatOpsRequest struct {
	Source         string         `json:"source"`
	RoomID         string         `json:"room_id"`
	MessageID      string         `json:"message_id"`
	UserExternalID string         `json:"user_external_id"`
	Username       string         `json:"username"`
	UserRole       string         `json:"user_role"`
	IsAdmin        bool           `json:"is_admin"`
	Text           string         `json:"text"`
	NewAPIUserID   int            `json:"new_api_user_id"`
	UserBound      bool           `json:"user_bound"`
	Raw            map[string]any `json:"raw"`
}

type AgentChatOpsBindConfirmRequest struct {
	Source         string `json:"source"`
	RoomID         string `json:"room_id"`
	UserExternalID string `json:"user_external_id"`
	Username       string `json:"username"`
	Code           string `json:"code"`
	Scope          string `json:"scope"`
}

const (
	agentChatOpsBindScopeRoom              = "room"
	agentChatOpsBindReasonRoomScopeRequire = "room_scope_required"
	agentChatOpsBindReasonBadScope         = "unsupported_bind_scope"
)

type AgentChatOpsResult struct {
	Ignored      bool                 `json:"ignored"`
	Reply        string               `json:"reply"`
	NewAPIUserID int                  `json:"new_api_user_id,omitempty"`
	UserBound    bool                 `json:"user_bound"`
	Task         *model.AgentTask     `json:"task,omitempty"`
	Action       *model.AgentAction   `json:"action,omitempty"`
	Actions      []*model.AgentAction `json:"actions,omitempty"`
}

func AgentChatOpsSecretOK(source string, authHeader string, queryToken string, telegramSecret string) bool {
	cfg := operation_setting.GetAgentSetting()
	if cfg == nil {
		return false
	}
	source = strings.ToLower(strings.TrimSpace(source))
	if source == "telegram" {
		source = "tg"
	}
	expected := strings.TrimSpace(cfg.ChatOpsWebhookSecret)
	bearerToken := ""
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(authHeader)), "bearer ") {
		bearerToken = strings.TrimSpace(authHeader)[7:]
	}
	matches := func(candidate, secret string) bool {
		candidate = strings.TrimSpace(candidate)
		secret = strings.TrimSpace(secret)
		return candidate != "" && secret != "" && subtle.ConstantTimeCompare([]byte(candidate), []byte(secret)) == 1
	}
	switch source {
	case "tg":
		return matches(telegramSecret, expected) || matches(bearerToken, expected)
	case "qq":
		qqExpected := strings.TrimSpace(cfg.QQAccessToken)
		return matches(bearerToken, expected) || matches(bearerToken, qqExpected) ||
			matches(queryToken, expected) || matches(queryToken, qqExpected)
	default:
		return matches(bearerToken, expected) || matches(queryToken, expected)
	}
}

func HandleAgentChatOps(ctx context.Context, req AgentChatOpsRequest) (*AgentChatOpsResult, error) {
	cfg := operation_setting.GetAgentSetting()
	source := strings.ToLower(strings.TrimSpace(req.Source))
	if source == "telegram" {
		source = "tg"
	}
	req.Source = source
	if cfg == nil || !cfg.Enabled || !cfg.ChatOpsEnabled {
		return &AgentChatOpsResult{Ignored: true, Reply: "agent chatops disabled"}, nil
	}
	_ = AgentEnsureDefaultMemories()
	if err := validateChatOpsSource(cfg, req); err != nil {
		return nil, err
	}
	isAdmin := isAgentChatOpsAdmin(ctx, cfg, req)
	req.IsAdmin = isAdmin
	identity := ResolveAgentChatOpsIdentity(req)
	req.NewAPIUserID = identity.NewAPIUserID
	req.UserBound = identity.UserBound
	text := strings.TrimSpace(req.Text)
	command, triggered := extractChatOpsCommand(cfg, text)
	status := "received"
	if !triggered {
		status = "ignored"
	}
	task := &model.AgentTask{SiteId: AgentSiteID(), TaskType: "chatops", AgentName: "director", Source: source, RoomId: strings.TrimSpace(req.RoomID), MessageId: strings.TrimSpace(req.MessageID), IssuerExternalId: strings.TrimSpace(req.UserExternalID), IssuerUsername: strings.TrimSpace(req.Username), IssuerRole: chatOpsRoleName(req.UserRole, isAdmin), Text: text, Command: truncateAgentText(command, 128), Status: status, PayloadJson: mustAgentJSON(req.Raw)}
	if err := model.CreateAgentTask(task); err != nil {
		return nil, err
	}
	_ = model.DB.Create(&model.AgentEvent{SiteId: AgentSiteID(), EventType: "chatops.message", Source: source, Severity: "info", Status: "closed", ActorType: "chatops", ActorExternalId: req.UserExternalID, Title: truncateAgentText(command, 120), PayloadJson: mustAgentJSON(map[string]any{"room_id": req.RoomID, "username": req.Username, "role": req.UserRole, "text": text, "new_api_user_id": req.NewAPIUserID, "user_bound": req.UserBound})}).Error
	AgentCaptureChatMemory(req, task.Id)
	if !triggered {
		return &AgentChatOpsResult{Ignored: true, Task: task}, nil
	}
	if command == "" || isHelpCommand(command) {
		reply := agentChatOpsHelp(isAdmin)
		_ = finishAgentTask(task, "completed", reply, nil, nil)
		_ = sendChatOpsAutoReply(ctx, cfg, req, reply)
		return &AgentChatOpsResult{Reply: reply, Task: task}, nil
	}
	if isAgentMemoryCommand(command) {
		if !isAdmin {
			reply := "需要管理员权限才能管理 Agent 长期记忆。"
			_ = finishAgentTask(task, "denied", reply, nil, nil)
			_ = sendChatOpsAutoReply(ctx, cfg, req, reply)
			return &AgentChatOpsResult{Reply: reply, Task: task}, nil
		}
		reply, err := AgentHandleMemoryCommand(req, command, task.Id)
		status := "completed"
		if err != nil {
			status = "failed"
		}
		_ = finishAgentTask(task, status, reply, nil, err)
		_ = sendChatOpsAutoReply(ctx, cfg, req, reply)
		return &AgentChatOpsResult{Reply: reply, Task: task}, err
	}
	if approvalID, decision, ok := parseApprovalCommand(command); ok {
		if !isAdmin {
			reply := "需要管理员权限才能审批 Agent 动作。"
			_ = finishAgentTask(task, "denied", reply, nil, nil)
			_ = sendChatOpsAutoReply(ctx, cfg, req, reply)
			return &AgentChatOpsResult{Reply: reply, Task: task}, nil
		}
		if err := AgentDecideApproval(approvalID, 0, decision, "chatops:"+req.Source+":"+req.UserExternalID); err != nil {
			reply := "审批失败：" + err.Error()
			_ = finishAgentTask(task, "failed", reply, nil, err)
			_ = sendChatOpsAutoReply(ctx, cfg, req, reply)
			return &AgentChatOpsResult{Reply: reply, Task: task}, err
		}
		reply := fmt.Sprintf("审批已处理：#%d %s", approvalID, decision)
		_ = finishAgentTask(task, "completed", reply, nil, nil)
		_ = sendChatOpsAutoReply(ctx, cfg, req, reply)
		return &AgentChatOpsResult{Reply: reply, Task: task}, nil
	}
	if !isAdmin && isSensitiveChatOpsCommand(command) {
		reply := publicSafeChatOpsReply(command)
		_ = model.DB.Create(&model.AgentEvent{SiteId: AgentSiteID(), EventType: "chatops.sensitive_denied", Source: req.Source, Severity: "info", Status: "closed", ActorType: "chatops", ActorExternalId: req.UserExternalID, Title: truncateAgentText(command, 120), PayloadJson: mustAgentJSON(map[string]any{"room_id": req.RoomID, "username": req.Username})}).Error
		_ = finishAgentTask(task, "denied", reply, nil, nil)
		_ = sendChatOpsAutoReply(ctx, cfg, req, reply)
		return &AgentChatOpsResult{Reply: reply, Task: task}, nil
	}

	actionReq, adminRequired, err := buildChatOpsAction(command, req, task.Id)
	if err != nil {
		reply := humanChatOpsParseError(command, err)
		_ = finishAgentTask(task, "failed", reply, nil, err)
		_ = sendChatOpsAutoReply(ctx, cfg, req, reply)
		return &AgentChatOpsResult{Reply: reply, Task: task}, nil
	}
	if actionReq.ActionType == "task.chatops.record" && AgentHermesEnabled(cfg) {
		plan, hermesErr := AgentPlanWithHermes(ctx, cfg, req, command, task.Id, isAdmin)
		if hermesErr == nil && plan != nil {
			if !shouldExecuteHermesActionsForChatOps(command, plan) {
				reply := sanitizeHermesChatReply(command, plan.Reply, isAdmin)
				if reply == "" {
					reply = naturalAgentReply(command, isAdmin)
				}
				_ = model.DB.Create(&model.AgentEvent{SiteId: AgentSiteID(), EventType: "hermes.chat_reply", Source: req.Source, Severity: "info", Status: "closed", ActorType: "chatops", ActorExternalId: req.UserExternalID, Title: truncateAgentText(command, 120), PayloadJson: mustAgentJSON(map[string]any{"risk": plan.Risk, "requires_approval_ignored": plan.RequiresApproval, "actions_ignored": len(plan.Actions), "notes": plan.Notes})}).Error
				_ = finishAgentTask(task, "completed", reply, nil, nil)
				_ = sendChatOpsAutoReply(ctx, cfg, req, reply)
				return &AgentChatOpsResult{Reply: reply, Task: task}, nil
			}
			createdActions, pendingActions, actionErrors := createHermesChatOpsActions(plan, req, task.Id, isAdmin)
			reply := strings.TrimSpace(plan.Reply)
			if reply == "" {
				reply = "Hermes 已收到任务并完成分析。"
			}
			if len(createdActions) > 0 || len(actionErrors) > 0 {
				reply = appendHermesActionSummary(reply, createdActions, pendingActions, actionErrors)
			}
			status := "completed"
			if pendingActions > 0 {
				status = "pending"
			}
			severity := "info"
			eventStatus := "closed"
			if len(actionErrors) > 0 {
				severity = "warning"
				eventStatus = "open"
			}
			primaryAction := firstHermesCreatedAction(createdActions)
			_ = model.DB.Create(&model.AgentEvent{SiteId: AgentSiteID(), EventType: "hermes.plan", Source: req.Source, Severity: severity, Status: eventStatus, ActorType: "chatops", ActorExternalId: req.UserExternalID, Title: truncateAgentText(command, 120), PayloadJson: mustAgentJSON(map[string]any{"risk": plan.Risk, "requires_approval": plan.RequiresApproval, "actions": plan.Actions, "created_action_ids": hermesActionIDs(createdActions), "action_errors": actionErrors, "notes": plan.Notes})}).Error
			_ = finishAgentTask(task, status, reply, primaryAction, nil)
			_ = sendChatOpsAutoReply(ctx, cfg, req, reply)
			return &AgentChatOpsResult{Reply: reply, Task: task, Action: primaryAction, Actions: createdActions}, nil
		}
		failure := "unknown hermes error"
		if hermesErr != nil {
			failure = hermesErr.Error()
		}
		_ = model.DB.Create(&model.AgentEvent{SiteId: AgentSiteID(), EventType: "hermes.plan_failed", Source: req.Source, Severity: "warning", Status: "open", ActorType: "chatops", ActorExternalId: req.UserExternalID, Title: truncateAgentText(command, 120), ResultJson: mustAgentJSON(map[string]any{"error": failure})}).Error
	}

	if adminRequired && !isAdmin {
		if actionReq.ActionType == "task.chatops.record" && !isSensitiveChatOpsCommand(command) {
			reply := naturalAgentReply(command, false)
			_ = model.DB.Create(&model.AgentEvent{SiteId: AgentSiteID(), EventType: "chatops.public_reply", Source: req.Source, Severity: "info", Status: "closed", ActorType: "chatops", ActorExternalId: req.UserExternalID, Title: truncateAgentText(command, 120), PayloadJson: mustAgentJSON(map[string]any{"room_id": req.RoomID, "username": req.Username})}).Error
			_ = finishAgentTask(task, "completed", reply, nil, nil)
			_ = sendChatOpsAutoReply(ctx, cfg, req, reply)
			return &AgentChatOpsResult{Reply: reply, Task: task}, nil
		}
		reply := "需要管理员权限才能执行该 Agent 任务。"
		_ = finishAgentTask(task, "denied", reply, nil, nil)
		_ = sendChatOpsAutoReply(ctx, cfg, req, reply)
		return &AgentChatOpsResult{Reply: reply, Task: task}, nil
	}
	action, err := AgentCreateAction(actionReq, 0)
	if action != nil {
		task.ActionId = action.Id
		task.ApprovalId = action.ApprovalId
	}
	if err != nil {
		reply := "Agent 动作创建失败：" + err.Error()
		_ = finishAgentTask(task, "failed", reply, action, err)
		_ = sendChatOpsAutoReply(ctx, cfg, req, reply)
		return &AgentChatOpsResult{Reply: reply, Task: task, Action: action}, err
	}
	reply := replyForAgentAction(action)
	_ = finishAgentTask(task, action.Status, reply, action, nil)
	_ = sendChatOpsAutoReply(ctx, cfg, req, reply)
	return &AgentChatOpsResult{Reply: reply, Task: task, Action: action}, nil
}

func validateChatOpsSource(cfg *operation_setting.AgentSetting, req AgentChatOpsRequest) error {
	switch strings.ToLower(strings.TrimSpace(req.Source)) {
	case "qq":
		if !cfg.QQBotEnabled {
			return errors.New("qq bot is disabled")
		}
	case "tg":
		if !cfg.TGBotEnabled {
			return errors.New("telegram bot is disabled")
		}
	case "community":
		return nil
	default:
		return errors.New("unsupported chatops source")
	}
	return nil
}

func isAgentChatOpsAdmin(ctx context.Context, cfg *operation_setting.AgentSetting, req AgentChatOpsRequest) bool {
	id := strings.ToLower(strings.TrimSpace(req.UserExternalID))
	name := strings.ToLower(strings.TrimSpace(req.Username))
	for _, item := range splitAgentCSV(cfg.ChatOpsAdminExternalIDs) {
		item = strings.ToLower(item)
		if item != "" && (item == id || item == name) {
			return true
		}
	}
	role := strings.ToLower(strings.TrimSpace(req.UserRole))
	return cfg.ChatOpsTrustGroupAdmin && (role == "owner" || role == "admin" || role == "administrator" || role == "creator")
}

func extractChatOpsCommand(cfg *operation_setting.AgentSetting, text string) (string, bool) {
	text = strings.TrimSpace(text)
	for _, prefix := range splitAgentCSV(cfg.ChatOpsCommandPrefixes) {
		if prefix != "" && strings.HasPrefix(strings.ToLower(text), strings.ToLower(prefix)) {
			return strings.TrimSpace(text[len(prefix):]), true
		}
	}
	if cfg.ChatOpsAllowNaturalLanguage && strings.Contains(strings.ToLower(text), "@agent") {
		return strings.TrimSpace(strings.ReplaceAll(text, "@agent", "")), true
	}
	return "", false
}

func buildChatOpsAction(command string, req AgentChatOpsRequest, taskID int) (AgentActionRequest, bool, error) {
	cmd := strings.TrimSpace(command)
	lower := strings.ToLower(cmd)
	payload := map[string]any{"chatops_task_id": taskID, "chatops_source": req.Source, "room_id": req.RoomID, "issuer_external_id": req.UserExternalID, "issuer_username": req.Username, "force_execute": true}
	base := AgentActionRequest{AgentName: "director", TargetType: "chatops", TargetId: fmt.Sprintf("task:%d", taskID), Payload: payload, IdempotencyKey: fmt.Sprintf("chatops-%s-%d", AgentSiteID(), taskID)}
	if isAgentSkillListCommand(cmd) {
		base.ActionType = "agent.skill.list"
		base.TargetType = "skill_registry"
		base.TargetId = AgentSiteID()
		base.Reason = cmd
		return base, true, nil
	}
	if isAgentSkillInstallCommand(cmd) {
		repoURL, branch, skillName := extractAgentSkillInstallRequest(cmd)
		base.ActionType = "agent.skill.install"
		base.TargetType = "skill_repo"
		base.TargetId = repoURL
		base.Reason = cmd
		payload["repo_url"] = repoURL
		if branch != "" {
			payload["branch"] = branch
		}
		if skillName != "" {
			payload["skill_name"] = skillName
		}
		if repoURL == "" {
			return base, true, errors.New("安装 GitHub skill 需要仓库链接，例如：安装这个 skill https://github.com/owner/repo branch=main")
		}
		return base, true, nil
	}
	if hasAny(lower, "余额", "额度", "quota balance", "balance") && !hasAny(lower, "加", "给", "奖励", "发奖", "grant", "reward") {
		base.ActionType = "user.quota.read"
		base.UserId = extractUserID(cmd)
		base.TargetType = "user"
		base.TargetId = strconv.Itoa(base.UserId)
		base.Reason = cmd
		payload["user_id"] = base.UserId
		if base.UserId <= 0 {
			return base, true, errors.New("查余额需要指定用户 ID，例如：查用户2余额")
		}
		return base, true, nil
	}
	if hasAny(lower, "模型", "model") {
		base.ActionType = "agent.model.manage"
		base.TargetType = "agent_model"
		modelName := extractAgentModelName(cmd)
		if modelName != "" {
			payload["model"] = modelName
			base.TargetId = modelName
		} else {
			base.TargetId = "list"
		}
		base.Reason = cmd
		return base, true, nil
	}
	if hasAny(lower, "状态", "status", "健康", "health") {
		base.ActionType = "site.state.read"
		base.Reason = cmd
		return base, false, nil
	}
	if hasAny(lower, "日志", "log", "logs", "报错", "错误记录") {
		base.ActionType = "site.logs.read"
		if hasAny(lower, "chatops", "群聊", "qq", "tg", "telegram", "社区") {
			base.ActionType = "chatops.logs.read"
		}
		base.TargetType = "logs"
		base.TargetId = AgentSiteID()
		base.Reason = cmd
		payload["limit"] = extractAgentLogLimit(cmd)
		payload["minutes"] = extractAgentLogMinutes(cmd)
		payload["log_type"] = inferAgentLogTypeFromCommand(lower)
		if base.ActionType == "chatops.logs.read" {
			payload["source"] = req.Source
			payload["room_id"] = req.RoomID
		}
		return base, true, nil
	}
	if hasAny(lower, "基金", "fund", "运营资金", "运营基金") {
		base.ActionType = "fund.report.read"
		base.TargetType = "ops_fund"
		base.TargetId = AgentSiteID()
		base.Reason = cmd
		return base, true, nil
	}
	if hasAny(lower, "预算", "budget") {
		base.ActionType = "budget.check"
		base.Reason = cmd
		return base, true, nil
	}
	if hasAny(lower, "风控", "risk") {
		base.ActionType = "risk.evaluate"
		base.UserId = extractUserID(cmd)
		base.Reason = cmd
		payload["user_id"] = base.UserId
		return base, true, nil
	}
	if hasAny(lower, "奖励", "reward", "发奖", "加额度", "加", "充值", "补偿") {
		base.ActionType = "reward.grant.small"
		base.UserId = extractUserID(cmd)
		base.QuotaAmount = extractQuota(cmd)
		base.BudgetPool = "activity"
		base.Reason = cmd
		payload["user_id"] = base.UserId
		payload["quota"] = base.QuotaAmount
		if base.UserId <= 0 || base.QuotaAmount <= 0 {
			return base, true, errors.New("发额度需要同时指定用户 ID 和金额，例如：给用户2加5刀 或 /agent 奖励 user=2 amount=2500000")
		}
		return base, true, nil
	}
	if isAgentGroupModerationCommand(cmd) {
		actionReq, err := buildAgentGroupModerationActionFromCommand(cmd, req, base)
		return actionReq, true, err
	}
	if hasAny(lower, "社区消息", "community", "发送社区") {
		base.ActionType = "message.community.send"
		payload["text"] = trimCommandHead(cmd, "社区消息", "发送社区", "community")
		base.Reason = fmt.Sprint(payload["text"])
		return base, true, nil
	}
	if hasAny(lower, "qq", "qq群") {
		base.ActionType = "message.qq.send"
		payload["text"] = trimCommandHead(cmd, "qq群", "qq")
		payload["group_id"] = firstAgentNonEmpty(req.RoomID, operation_setting.GetAgentSetting().QQGroupID)
		base.TargetId = fmt.Sprint(payload["group_id"])
		base.Reason = fmt.Sprint(payload["text"])
		return base, true, nil
	}
	if hasAny(lower, "tg", "telegram") {
		base.ActionType = "message.tg.send"
		payload["text"] = trimCommandHead(cmd, "telegram", "tg")
		payload["chat_id"] = firstAgentNonEmpty(req.RoomID, operation_setting.GetAgentSetting().TGChatID)
		base.TargetId = fmt.Sprint(payload["chat_id"])
		base.Reason = fmt.Sprint(payload["text"])
		return base, true, nil
	}
	if hasAny(lower, "公告", "notice") {
		base.ActionType = "admin.notice.publish"
		payload["text"] = trimCommandHead(cmd, "公告", "notice")
		base.Reason = cmd
		return base, true, nil
	}
	base.ActionType = "task.chatops.record"
	base.Reason = cmd
	payload["natural_task"] = cmd
	return base, true, nil
}

func createHermesChatOpsActions(plan *AgentHermesPlan, req AgentChatOpsRequest, taskID int, isAdmin bool) ([]*model.AgentAction, int, []string) {
	if plan == nil || len(plan.Actions) == 0 {
		return nil, 0, nil
	}
	created := make([]*model.AgentAction, 0, len(plan.Actions))
	errorsOut := make([]string, 0)
	pending := 0
	for idx, hAction := range plan.Actions {
		actionReq, err := buildAgentActionFromHermes(plan, hAction, req, taskID, idx, isAdmin)
		if err != nil {
			errorsOut = append(errorsOut, err.Error())
			_ = model.DB.Create(&model.AgentEvent{SiteId: AgentSiteID(), EventType: "hermes.action_rejected", Source: req.Source, Severity: "warning", Status: "closed", ActorType: "chatops", ActorExternalId: req.UserExternalID, Title: truncateAgentText(hAction.Type, 120), PayloadJson: mustAgentJSON(map[string]any{"error": err.Error(), "action": hAction})}).Error
			continue
		}
		action, err := AgentCreateAction(actionReq, 0)
		if err != nil {
			errorsOut = append(errorsOut, fmt.Sprintf("%s: %s", actionReq.ActionType, err.Error()))
			continue
		}
		created = append(created, action)
		if action.ApprovalRequired || action.Status == "pending" || action.Status == "approved" {
			pending++
		}
	}
	return created, pending, errorsOut
}

func buildAgentActionFromHermes(plan *AgentHermesPlan, hAction AgentHermesAction, req AgentChatOpsRequest, taskID int, idx int, isAdmin bool) (AgentActionRequest, error) {
	actionType := strings.TrimSpace(hAction.Type)
	if actionType == "" {
		return AgentActionRequest{}, fmt.Errorf("hermes action #%d missing type", idx+1)
	}
	if !isHermesAllowedAction(actionType) {
		return AgentActionRequest{}, fmt.Errorf("hermes action type not allowed: %s", actionType)
	}
	if !isAdmin && !isHermesNonAdminAction(actionType) {
		return AgentActionRequest{}, fmt.Errorf("hermes action requires admin: %s", actionType)
	}
	payload := cloneAgentPayload(hAction.Payload)
	payload["hermes_plan"] = true
	payload["hermes_risk"] = plan.Risk
	payload["hermes_requires_approval"] = plan.RequiresApproval
	payload["chatops_task_id"] = taskID
	payload["chatops_source"] = req.Source
	payload["room_id"] = req.RoomID
	payload["message_id"] = req.MessageID
	payload["issuer_external_id"] = req.UserExternalID
	payload["issuer_username"] = req.Username

	cfg := operation_setting.GetAgentSetting()
	reason := firstAgentNonEmpty(strings.TrimSpace(hAction.Reason), agentPayloadString(payload, "reason"), fmt.Sprintf("Hermes action from chatops task #%d", taskID))
	targetType := firstAgentNonEmpty(agentPayloadString(payload, "target_type"), "chatops")
	targetID := firstAgentNonEmpty(agentPayloadString(payload, "target_id"), fmt.Sprintf("task:%d", taskID))
	userID := int(agentPayloadFloat(payload, "user_id", "uid"))
	quota := int(agentPayloadFloat(payload, "quota", "amount", "quota_amount"))
	budgetPool := firstAgentNonEmpty(agentPayloadString(payload, "budget_pool", "pool"), "daily")

	switch actionType {
	case "agent.skill.install":
		targetType = "skill_repo"
		repoURL, branch, skillName := extractAgentSkillInstallRequest(firstAgentNonEmpty(reason, req.RawCommand()))
		payload["repo_url"] = firstAgentNonEmpty(agentPayloadString(payload, "repo_url", "url", "repo"), repoURL)
		payload["branch"] = firstAgentNonEmpty(agentPayloadString(payload, "branch", "ref"), branch)
		payload["skill_name"] = firstAgentNonEmpty(agentPayloadString(payload, "skill_name", "name"), skillName)
		targetID = firstAgentNonEmpty(agentPayloadString(payload, "repo_url", "url", "repo"), targetID)
		if strings.TrimSpace(targetID) == "" {
			return AgentActionRequest{}, errors.New("hermes skill install action requires repo_url")
		}
	case "agent.skill.list":
		targetType = "skill_registry"
		targetID = AgentSiteID()
	case "message.qq.send":
		targetType = "qq_group"
		targetID = firstAgentNonEmpty(agentPayloadString(payload, "group_id", "room_id"), req.RoomID, cfg.QQGroupID)
		payload["group_id"] = targetID
	case "message.tg.send":
		targetType = "telegram_chat"
		targetID = firstAgentNonEmpty(agentPayloadString(payload, "chat_id", "room_id"), req.RoomID, cfg.TGChatID)
		payload["chat_id"] = targetID
	case "message.community.send":
		targetType = "community_room"
		targetID = firstAgentNonEmpty(agentPayloadString(payload, "to_room_id", "room_id"), cfg.CommunityRoomID)
		payload["room_id"] = targetID
	case "group.message.delete", "group.member.mute", "group.member.unmute", "group.member.kick", "group.member.ban", "group.member.unban", "group.member.lookup", "group.admin.lookup":
		tType, tID, _, err := applyAgentGroupModerationDefaults(actionType, payload, req)
		if err != nil {
			return AgentActionRequest{}, err
		}
		targetType = tType
		targetID = tID
	case "reward.grant.small":
		targetType = "user"
		budgetPool = firstAgentNonEmpty(agentPayloadString(payload, "budget_pool", "pool"), "activity")
		if userID <= 0 || quota <= 0 {
			return AgentActionRequest{}, errors.New("hermes reward action requires positive user_id and quota")
		}
		targetID = strconv.Itoa(userID)
	case "risk.evaluate":
		targetType = "user"
		if userID > 0 {
			targetID = strconv.Itoa(userID)
		}
	case "budget.check":
		targetType = "budget"
		targetID = firstAgentNonEmpty(agentPayloadString(payload, "budget_pool", "pool"), "today")
	case "fund.report.read":
		targetType = "ops_fund"
		targetID = AgentSiteID()
	case "site.state.read":
		targetType = "site"
		targetID = AgentSiteID()
	case "site.logs.read", "chatops.logs.read":
		targetType = "logs"
		targetID = AgentSiteID()
		if actionType == "chatops.logs.read" {
			payload["source"] = firstAgentNonEmpty(agentPayloadString(payload, "source", "log_source"), req.Source)
			payload["room_id"] = firstAgentNonEmpty(agentPayloadString(payload, "room_id", "chat_id", "group_id"), req.RoomID)
		}
	case "task.chatops.record":
		targetType = "chatops"
		targetID = fmt.Sprintf("task:%d", taskID)
	case "admin.notice.publish":
		targetType = "notice"
		targetID = firstAgentNonEmpty(agentPayloadString(payload, "notice_id"), "draft")
	}

	forceApproval := false
	if !isAdmin {
		forceApproval = plan.RequiresApproval || isHermesAlwaysApprovalAction(actionType)
	}
	if forceApproval {
		payload["force_approval"] = true
	} else if isAdmin {
		if isAgentGroupModerationAction(actionType) && cfg != nil && !cfg.ChatOpsAllowDirectModeration {
			forceApproval = true
			payload["force_approval"] = true
		} else {
			payload["admin_chatops_authorized"] = true
			payload["force_execute"] = true
		}
	}
	return AgentActionRequest{
		ActionType:     actionType,
		AgentName:      "hermes",
		TargetType:     targetType,
		TargetId:       targetID,
		UserId:         userID,
		QuotaAmount:    quota,
		BudgetPool:     budgetPool,
		Reason:         reason,
		Payload:        payload,
		ForceApproval:  forceApproval,
		IdempotencyKey: fmt.Sprintf("hermes-%s-%d-%02d-%s", AgentSiteID(), taskID, idx+1, strings.ReplaceAll(actionType, ".", "-")),
	}, nil
}

func isHermesAllowedAction(actionType string) bool {
	if isAgentGroupModerationAction(actionType) {
		return true
	}
	switch actionType {
	case "message.qq.send", "message.tg.send", "message.community.send", "admin.notice.publish", "reward.grant.small", "reward.settlement.batch", "quiz.question.draw", "quiz.round.load", "quiz.answer.submit", "fund.report.read", "risk.evaluate", "budget.check", "site.state.read", "site.logs.read", "chatops.logs.read", "task.chatops.record", "user.quota.read", "agent.model.manage", "agent.skill.install", "agent.skill.list":
		return true
	default:
		return false
	}
}

func isHermesNonAdminAction(actionType string) bool {
	switch actionType {
	case "site.state.read", "task.chatops.record":
		return true
	default:
		return false
	}
}

// isHermesAlwaysApprovalAction 仅对 admin.* 类高危动作强制审批。
// 小额奖励 reward.grant.small 不再无条件强制审批，改为由金额阈值（ApprovalThresholdQuota）
// 和单动作上限统一判定——小额自动放行、超阈值才需��批，避免日常运营被频繁打断。
func isHermesAlwaysApprovalAction(actionType string) bool {
	if isAgentGroupModerationAction(actionType) {
		return false
	}
	return strings.HasPrefix(actionType, "admin.")
}

func cloneAgentPayload(in map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range in {
		out[k] = v
	}
	return out
}

func sanitizeHermesChatReply(command string, reply string, isAdmin bool) string {
	reply = strings.TrimSpace(reply)
	if !isPlainChatOpsConversation(command) {
		return reply
	}
	lowerReply := strings.ToLower(reply)
	if reply == "" || hasAny(lowerReply, "审批", "批准", "确认是否", "立即执行", "执行一次", "new api 执行", "创建动作", "受控动作", "写入审计", "权限或格式", "后台执行", "请确认", "是否继续") {
		return naturalAgentReply(command, isAdmin)
	}
	return reply
}

func isPlainChatOpsConversation(command string) bool {
	lower := strings.ToLower(strings.TrimSpace(command))
	if lower == "" {
		return true
	}
	if isAgentSkillInstallCommand(lower) || isAgentSkillListCommand(lower) {
		return false
	}
	mutationKeywords := []string{"new api", "newapi", "系统", "额度", "奖励", "发奖", "公告", "社区消息", "发送社区", "tg", "telegram", "qq", "qq群", "风控", "禁用", "启用", "渠道", "分组", "配置", "删除", "撤回", "禁言", "解禁", "踢", "移出", "封禁", "ban", "mute", "kick", "限流", "重启", "部署", "查用户", "余额", "充值", "补偿", "日志", "log", "logs"}
	if hasAny(lower, mutationKeywords...) {
		return false
	}
	return true
}

func shouldExecuteHermesActionsForChatOps(command string, plan *AgentHermesPlan) bool {
	if plan == nil || len(plan.Actions) == 0 {
		return false
	}
	lower := strings.ToLower(strings.TrimSpace(command))
	if lower == "" {
		return false
	}
	if isAgentSkillInstallCommand(lower) || isAgentSkillListCommand(lower) {
		return true
	}
	// 纯对话、调研、人设、确认、yolo 等不改站点状态，不创建动作、不审批。
	if hasAny(lower, "你好", "同意", "是的", "可以", "收到", "人设", "人格", "skill", "联网搜索", "搜索", "资料", "yolo", "模式", "角色", "设定", "聊天", "讨论", "解释", "规划", "方案") && !hasAny(lower, "new api", "newapi", "系统", "额度", "奖励", "发奖", "公告", "社区消息", "tg", "telegram", "qq", "渠道", "分组", "配置", "删除", "封禁", "限流", "重启", "部署", "撤回", "禁言", "解禁", "踢", "移出", "ban", "mute", "kick", "广告", "引流", "日志", "log", "logs") {
		return false
	}
	return hasAny(lower, "new api", "newapi", "系统", "额度", "奖励", "发奖", "公告", "社区消息", "发送社区", "tg", "telegram", "qq", "qq群", "风控", "禁用", "启用", "渠道", "分组", "配置", "删除", "封禁", "限流", "重启", "部署", "撤回", "禁言", "解禁", "踢", "移出", "ban", "mute", "kick", "广告", "引流", "日志", "log", "logs")
}

func agentSiteDisplayName() string {
	if cfg := operation_setting.GetAgentSetting(); cfg != nil {
		if name := strings.TrimSpace(cfg.SiteName); name != "" {
			return name
		}
	}
	return "本站"
}

func agentModelExampleName(models []string, current string) string {
	current = strings.TrimSpace(current)
	for _, modelName := range models {
		modelName = strings.TrimSpace(modelName)
		if modelName != "" && !strings.EqualFold(modelName, current) {
			return modelName
		}
	}
	if current != "" {
		return current
	}
	for _, modelName := range models {
		modelName = strings.TrimSpace(modelName)
		if modelName != "" {
			return modelName
		}
	}
	return "模型名"
}

func naturalAgentReply(command string, isAdmin bool) string {
	lower := strings.ToLower(strings.TrimSpace(command))
	siteName := agentSiteDisplayName()
	if hasAny(lower, "yolo") {
		return fmt.Sprintf("收到。YOLO 我会只当成说话风格：更直接、少打断；但不会绕过 %s 的额度、公告、渠道和系统配置边界。", siteName)
	}
	if lower == "同意" || lower == "是的" || lower == "可以" || lower == "收到" {
		return "收到。这个确认不会触发审批。需要我执行具体操作时，直接说明目标和参数即可。"
	}
	if hasAny(lower, "联网搜索", "搜索", "资料", "人设", "人格", "skill") {
		return "收到。我会按调研/讨论任务处理，不会创建 New API 或系统动作，也不会触发审批。请把要沉淀的人设方向、语气、边界和示例继续发给我。"
	}
	if isAdmin {
		return fmt.Sprintf("收到。这类内容我先按普通对话/规划处理，不会擅自改 %s 的真实配置或运行态。你把目标和动作说清楚，我再直接执行。", siteName)
	}
	return fmt.Sprintf("收到。这条消息我先按普通对话处理，不会擅自改 %s 的站点状态。", siteName)
}

func firstHermesCreatedAction(actions []*model.AgentAction) *model.AgentAction {
	if len(actions) == 0 {
		return nil
	}
	return actions[0]
}

func hermesActionIDs(actions []*model.AgentAction) []int {
	ids := make([]int, 0, len(actions))
	for _, action := range actions {
		if action != nil {
			ids = append(ids, action.Id)
		}
	}
	return ids
}

func appendHermesActionSummary(reply string, actions []*model.AgentAction, pending int, actionErrors []string) string {
	parts := []string{strings.TrimSpace(reply)}
	if len(actions) > 0 {
		if hermesActionsAllGroupModeration(actions) && pending == 0 {
			for _, action := range actions {
				if action != nil && action.Status == "completed" {
					if s := strings.TrimSpace(actionSummaryFromJSON(action.ResultJson)); s != "" {
						parts = append(parts, s)
					}
				}
			}
		} else {
			parts = append(parts, fmt.Sprintf("已生成 %d 个受控动作，其中 %d 个待审批/待执行。", len(actions), pending))
		}
	}
	if len(actionErrors) > 0 {
		parts = append(parts, fmt.Sprintf("有 %d 个动作没处理成，原因已写入审计日志。", len(actionErrors)))
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

func hermesActionsAllGroupModeration(actions []*model.AgentAction) bool {
	if len(actions) == 0 {
		return false
	}
	for _, action := range actions {
		if action == nil || !isAgentGroupModerationAction(action.ActionType) {
			return false
		}
	}
	return true
}

func replyForAgentAction(action *model.AgentAction) string {
	if action == nil {
		return "已收到，我已经记录这项任务。"
	}
	if action.ApprovalRequired || action.Status == "pending" || action.Status == "approved" {
		return fmt.Sprintf("这项操作需要管理员确认。审批编号：#%d。\n直接私聊回复：/agent 审批 %d 通过，或 /agent 审批 %d 拒绝。", action.ApprovalId, action.ApprovalId, action.ApprovalId)
	}
	summary := actionSummaryFromJSON(action.ResultJson)
	if action.ActionType == "site.state.read" {
		return humanAgentStateSummary(summary)
	}
	if isAgentGroupModerationAction(action.ActionType) {
		if action.Status == "completed" {
			return summary
		}
		if action.Status == "failed" {
			return "这次没处理成功：" + summary
		}
		return fmt.Sprintf("群管理动作状态：%s。%s", action.Status, summary)
	}
	if action.ActionType == "user.quota.read" || action.ActionType == "agent.model.manage" || action.ActionType == "agent.skill.install" || action.ActionType == "agent.skill.list" || action.ActionType == "reward.grant.small" || action.ActionType == "budget.check" || action.ActionType == "fund.report.read" || action.ActionType == "site.logs.read" || action.ActionType == "chatops.logs.read" {
		return summary
	}
	if action.Status == "completed" {
		return "已完成：" + summary
	}
	return fmt.Sprintf("当前状态：%s。%s", action.Status, summary)
}

func humanAgentStateSummary(summary string) string {
	re := regexp.MustCompile(`(.+?) channels=(\d+)/(\d+) groups=(\d+)/(\d+) avg=(\d+)ms recent_errors=(\d+)`)
	m := re.FindStringSubmatch(summary)
	if len(m) != 8 {
		return "站点状态：" + summary
	}
	return fmt.Sprintf("站点状态：%s\n- 可用渠道：%s/%s\n- 可用分组：%s/%s\n- 平均响应：%sms\n- 最近错误：%s 条", m[1], m[2], m[3], m[4], m[5], m[6], m[7])
}

func actionSummaryFromJSON(raw string) string {
	m := map[string]any{}
	_ = json.Unmarshal([]byte(raw), &m)
	if s, _ := m["summary"].(string); s != "" {
		return s
	}
	if s, _ := m["error"].(string); s != "" {
		return s
	}
	return truncateAgentText(raw, 160)
}

func finishAgentTask(task *model.AgentTask, status string, reply string, action *model.AgentAction, err error) error {
	if task == nil || task.Id <= 0 {
		return nil
	}
	updates := map[string]interface{}{"status": status, "result_json": mustAgentJSON(map[string]any{"reply": reply})}
	if action != nil {
		updates["action_id"] = action.Id
		updates["approval_id"] = action.ApprovalId
		updates["risk_level"] = action.RiskLevel
		updates["risk_score"] = action.RiskScore
	}
	if err != nil {
		updates["error"] = err.Error()
	}
	AgentLearnFromFinishedTask(task, status, reply, action, err)
	if updateErr := model.UpdateAgentTaskResult(task.Id, AgentSiteID(), updates); updateErr != nil {
		return updateErr
	}
	task.Status = status
	task.ResultJson = fmt.Sprint(updates["result_json"])
	if action != nil {
		task.ActionId = action.Id
		task.ApprovalId = action.ApprovalId
		task.RiskLevel = action.RiskLevel
		task.RiskScore = action.RiskScore
	}
	if err != nil {
		task.Error = err.Error()
	}
	return nil
}

// isSensitiveChatOpsCommand 仅判定真正高危/涉密的运维与管理动作。
// 普通的余额/额度/模型/状态/预算等问询不再算敏感，允许普通用户走 Hermes 正常对话答疑；
// 真正高危操作（密钥、数据库、重启部署、封禁删除、发奖审批、改配置等）仍仅对管理员开放。
func isSensitiveChatOpsCommand(command string) bool {
	lower := strings.ToLower(strings.TrimSpace(command))
	if isAgentSkillInstallCommand(lower) || isAgentSkillListCommand(lower) {
		return true
	}
	return hasAny(lower,
		"发奖", "发放奖励", "充值", "补偿", "审批", "approve", "approval",
		"封禁", "封号", "解封", "删除", "撤回", "禁言", "解禁", "踢", "移出", "ban", "mute", "kick", "限流", "重启", "部署", "deploy", "restart",
		"key", "token", "secret", "密钥", "数据库", "sql",
		"改配置", "改渠道", "改分组")
}

func publicSafeChatOpsReply(command string) string {
	lower := strings.ToLower(strings.TrimSpace(command))
	siteName := agentSiteDisplayName()
	if isAgentSkillInstallCommand(lower) || isAgentSkillListCommand(lower) {
		return "外部 skill 的安装和已装列表只对管理员开放；你要是管理员，直接私聊我一句“安装这个 skill <GitHub 链接>”或“看看现在装了哪些 skill”就行。"
	}
	if hasAny(lower, "模型", "model") {
		return fmt.Sprintf("%s 的模型可用情况和切换策略，请以前台展示和管理员公告为准；管理级模型与渠道细节只对管理员开放。", siteName)
	}
	if hasAny(lower, "状态", "status", "健康", "health", "日志", "log", "错误", "error", "渠道", "channel", "配置", "config") {
		return fmt.Sprintf("%s 的运行状态、日志、渠道和配置只对管理员开放。你可以继续问公开规则、使用教程、充值/额度说明或常见问题。", siteName)
	}
	if hasAny(lower, "余额", "额度", "quota", "balance", "用户", "user", "key", "token") {
		return "账户、额度、Key 等信息只允许本人在站点内查看；群内不会展示敏感信息。"
	}
	return "这个请求涉及管理或敏感信息，群内不会展示。你可以咨询公开规则、使用教程和常见问题。"
}

func sendChatOpsAutoReply(ctx context.Context, cfg *operation_setting.AgentSetting, req AgentChatOpsRequest, reply string) error {
	if !cfg.ChatOpsAutoReply || strings.TrimSpace(reply) == "" {
		return nil
	}
	source := strings.ToLower(strings.TrimSpace(req.Source))
	if source == "qq" || source == "tg" || source == "telegram" {
		if shouldPrivateReplyChatOps(req) && strings.TrimSpace(req.UserExternalID) != "" {
			// 管理员/敏感运维对话只私聊；私聊失败也不回群，避免泄露。
			_ = AgentSendChatOpsReplyTarget(ctx, req.Source, "", req.UserExternalID, reply)
			return nil
		}
		return AgentSendChatOpsReplyTarget(ctx, req.Source, req.RoomID, req.UserExternalID, reply)
	}
	// 社区房间没有可靠私聊入口；敏感/管理员类请求不回显管理正文。
	if shouldPrivateReplyChatOps(req) {
		return AgentSendChatOpsReplyTarget(ctx, req.Source, req.RoomID, req.UserExternalID, publicSafeChatOpsReply(req.RawCommand()))
	}
	return AgentSendChatOpsReplyTarget(ctx, req.Source, req.RoomID, req.UserExternalID, reply)
}

func shouldPrivateReplyChatOps(req AgentChatOpsRequest) bool {
	if req.IsAdmin {
		return true
	}
	lower := strings.ToLower(strings.TrimSpace(req.Text + " " + req.RawCommand()))
	if isAgentSkillInstallCommand(lower) || isAgentSkillListCommand(lower) {
		return true
	}
	return hasAny(lower,
		"余额", "额度", "quota", "balance", "模型", "model", "状态", "status", "健康", "health", "预算", "budget", "基金", "fund", "风控", "risk",
		"奖励", "reward", "发奖", "充值", "补偿", "公告", "notice", "审批", "approve", "approval", "配置", "config", "渠道", "channel",
		"分组", "group", "用户", "user", "封禁", "删除", "限流", "重启", "部署", "日志", "log", "错误", "error", "key", "token", "secret", "数据库", "db", "sql")
}

func (req AgentChatOpsRequest) RawCommand() string {
	text := strings.TrimSpace(req.Text)
	if strings.HasPrefix(strings.ToLower(text), "/agent") {
		return strings.TrimSpace(text[len("/agent"):])
	}
	if strings.HasPrefix(strings.ToLower(text), "@agent") {
		return strings.TrimSpace(text[len("@agent"):])
	}
	return text
}

func agentChatOpsHelp(isAdmin bool) string {
	models := agentEnabledModelNames()
	currentModel := ""
	if cfg := operation_setting.GetAgentSetting(); cfg != nil {
		currentModel = strings.TrimSpace(cfg.LLMModel)
	}
	exampleModel := agentModelExampleName(models, currentModel)
	if isAdmin {
		return fmt.Sprintf(`我是本站 Agent 管理助手。请优先在 QQ/TG 私聊中处理管理、运营和运维任务；普通群聊只用于公开客服问答。

管理员常用命令：
- /agent 状态：查看站点、渠道、分组是否正常
- /agent 预算：查看奖励和活动预算
- /agent 基金：查看运营基金余额、今日收支和最近流水
- /agent 模型：查看 Agent 当前模型和可切换模型
- /agent 切换模型 %s：切换 Agent/Hermes 使用的模型
- /agent 查用户2余额：查看用户余额
- /agent 社区消息 <内容>：发送社区群消息
- /agent tg <内容>：发送 Telegram 群消息
- /agent qq <内容>：发送 QQ 群消息
- /agent 风控 user=123：查看某个用户风险
- /agent 奖励 user=123 amount=500000 reason=补偿：给用户发奖励
- /agent 安装 skill https://github.com/owner/repo [branch=main]：安装 GitHub 外部 skill
- /agent 查看 skills：查看当前已装的 skill
- /agent 审批 12 通过/拒绝：处理审批
- /agent memory list：查看本站 Agent 长期记忆
- /agent 记住 scope=policy key=pricing.note 内容=国模保持官方定价：保存运营口径
- /agent 删除记忆 scope=policy key=pricing.note：删除过期记忆

你也可以直接用普通话私聊我，例如：
“帮我看看今天站点是否正常”
“给用户 123 发 0.5 刀补偿”
“切换模型 %s”
“发一条社区公告说明今晚维护”
“安装这个 skill https://github.com/owner/repo”
`, exampleModel, exampleModel)
	}
	return `我是本站 Agent 助手。你可以在群里咨询公开规则、使用教程、充值/额度说明和常见问题。

普通用户可用：
- /agent 使用教程
- /agent 规则
- /agent 怎么注册
- /agent 额度说明

账户与站点管理类敏感信息不会在群内展示。`
}
func isHelpCommand(command string) bool {
	lower := strings.ToLower(strings.TrimSpace(command))
	return lower == "help" || lower == "帮助" || lower == "?" || lower == "菜单"
}

func parseApprovalCommand(command string) (int, string, bool) {
	lower := strings.ToLower(command)
	if !hasAny(lower, "审批", "approve", "approval") {
		return 0, "", false
	}
	id, _ := strconv.Atoi(regexp.MustCompile(`\d+`).FindString(command))
	if id <= 0 {
		return 0, "", false
	}
	decision := "approved"
	if hasAny(lower, "拒绝", "reject", "rejected", "deny") {
		decision = "rejected"
	}
	return id, decision, true
}

func hasAny(value string, needles ...string) bool {
	for _, n := range needles {
		if strings.Contains(value, strings.ToLower(n)) || strings.Contains(value, n) {
			return true
		}
	}
	return false
}
func trimCommandHead(command string, heads ...string) string {
	out := strings.TrimSpace(command)
	lower := strings.ToLower(out)
	for _, head := range heads {
		if strings.HasPrefix(lower, strings.ToLower(head)) {
			return strings.TrimSpace(out[len(head):])
		}
	}
	parts := strings.Fields(out)
	if len(parts) > 1 {
		return strings.Join(parts[1:], " ")
	}
	return ""
}

func extractAgentLogLimit(command string) int {
	for _, re := range []*regexp.Regexp{
		regexp.MustCompile(`(?i)(?:limit|count|num)\s*[=:：]?\s*(\d+)`),
		regexp.MustCompile(`(?:最近|最新|取|看|查)\s*(\d+)\s*(?:条|行)?`),
	} {
		if m := re.FindStringSubmatch(command); len(m) > 1 {
			if n, _ := strconv.Atoi(m[1]); n > 0 {
				if n > 20 {
					return 20
				}
				return n
			}
		}
	}
	return 8
}

func extractAgentLogMinutes(command string) int {
	lower := strings.ToLower(command)
	for _, re := range []*regexp.Regexp{
		regexp.MustCompile(`(?i)(?:minutes|mins|minute)\s*[=:：]?\s*(\d+)`),
		regexp.MustCompile(`最近\s*(\d+)\s*(?:分钟|分)`),
	} {
		if m := re.FindStringSubmatch(command); len(m) > 1 {
			if n, _ := strconv.Atoi(m[1]); n > 0 {
				return n
			}
		}
	}
	if m := regexp.MustCompile(`最近\s*(\d+)\s*(?:小时|钟头)`).FindStringSubmatch(command); len(m) > 1 {
		if n, _ := strconv.Atoi(m[1]); n > 0 {
			return n * 60
		}
	}
	if hasAny(lower, "半小时", "半个钟", "30分钟", "30 分钟") {
		return 30
	}
	if hasAny(lower, "一天", "24小时", "24 小时") {
		return 24 * 60
	}
	return 60
}

func inferAgentLogTypeFromCommand(lower string) string {
	if hasAny(lower, "错误", "失败", "报错", "error", "errors") {
		return "error"
	}
	if hasAny(lower, "系统", "system", "签到", "验牌", "绑定", "门禁") {
		return "system"
	}
	if hasAny(lower, "管理", "后台", "admin", "manage", "操作") {
		return "manage"
	}
	if hasAny(lower, "消费", "请求", "扣费", "usage", "consume") {
		return "consume"
	}
	if hasAny(lower, "充值", "topup", "recharge") {
		return "topup"
	}
	if hasAny(lower, "退款", "refund") {
		return "refund"
	}
	if hasAny(lower, "登录", "login") {
		return "login"
	}
	return ""
}

func extractUserID(command string) int {
	for _, key := range []string{"user_id", "user", "uid", "用户ID", "用户id", "用户"} {
		if v := extractValue(command, key); v != "" {
			id, _ := strconv.Atoi(v)
			if id > 0 {
				return id
			}
		}
	}
	patterns := []string{
		`(?i)(?:user|uid|user_id)\s*#?\s*(\d+)`,
		`(?:用户|用户ID|用户id)\s*#?\s*(\d+)`,
		`#(\d+)`,
	}
	for _, pattern := range patterns {
		if m := regexp.MustCompile(pattern).FindStringSubmatch(command); len(m) >= 2 {
			id, _ := strconv.Atoi(m[1])
			if id > 0 {
				return id
			}
		}
	}
	return 0
}
func extractQuota(command string) int {
	for _, key := range []string{"quota", "amount", "额度"} {
		if v := extractValue(command, key); v != "" {
			q, _ := strconv.Atoi(v)
			return q
		}
	}
	if m := regexp.MustCompile(`([0-9]+(?:\.[0-9]+)?)\s*(刀|usd|美元)`).FindStringSubmatch(strings.ToLower(command)); len(m) >= 2 {
		f, _ := strconv.ParseFloat(m[1], 64)
		return int(f * common.QuotaPerUnit)
	}
	return 0
}
func extractValue(command string, keys ...string) string {
	for _, key := range keys {
		re := regexp.MustCompile(`(?i)` + regexp.QuoteMeta(key) + `\s*[:=： ]\s*([A-Za-z0-9_.-]+)`)
		if m := re.FindStringSubmatch(command); len(m) >= 2 {
			return strings.TrimSpace(m[1])
		}
	}
	return ""
}
func chatOpsRoleName(role string, admin bool) string {
	if admin {
		return "admin"
	}
	if strings.TrimSpace(role) == "" {
		return "member"
	}
	return strings.TrimSpace(role)
}

func humanChatOpsParseError(command string, err error) string {
	msg := strings.TrimSpace(err.Error())
	if msg == "" {
		msg = "命令格式不完整"
	}
	return msg
}

func extractAgentModelName(command string) string {
	cmd := strings.TrimSpace(command)
	for _, key := range []string{"model", "模型", "切换模型", "使用模型", "换模型"} {
		if v := extractValue(cmd, key); v != "" {
			return strings.TrimSpace(v)
		}
	}
	parts := strings.Fields(cmd)
	for i, part := range parts {
		p := strings.ToLower(strings.Trim(part, " ：:=，,。"))
		if (p == "model" || p == "模型" || p == "切换模型" || p == "使用模型" || p == "换模型") && i+1 < len(parts) {
			return strings.Trim(parts[i+1], " ：:=，,。")
		}
	}
	return ""
}

func agentQuotaUSDText(quota int) string {
	return fmt.Sprintf("$%.2f", float64(quota)/common.QuotaPerUnit)
}

func readAgentUserQuota(userID int) (string, error) {
	if userID <= 0 {
		return "", errors.New("用户 ID 无效")
	}
	var user model.User
	if err := model.DB.Where("id = ?", userID).First(&user).Error; err != nil {
		return "", err
	}
	return fmt.Sprintf("用户 #%d（%s）当前余额：%s。原始额度=%d", user.Id, user.Username, agentQuotaUSDText(user.Quota), user.Quota), nil
}

func agentEnabledModelNames() []string {
	models := model.GetEnabledModels()
	seen := map[string]bool{}
	out := make([]string, 0, len(models))
	for _, name := range models {
		name = strings.TrimSpace(name)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func agentModelManageReply(modelName string) (string, error) {
	cfg := operation_setting.GetAgentSetting()
	current := strings.TrimSpace(cfg.LLMModel)
	models := agentEnabledModelNames()
	if strings.TrimSpace(modelName) == "" {
		limit := len(models)
		if limit > 30 {
			limit = 30
		}
		list := strings.Join(models[:limit], "\n- ")
		if list != "" {
			list = "\n- " + list
		}
		more := ""
		if len(models) > limit {
			more = fmt.Sprintf("\n……还有 %d 个模型未显示。", len(models)-limit)
		}
		exampleModel := agentModelExampleName(models, current)
		return fmt.Sprintf("当前 Agent 模型：%s\n可切换模型共 %d 个：%s%s\n\n切换示例：/agent 切换模型 %s", current, len(models), list, more, exampleModel), nil
	}
	found := false
	for _, m := range models {
		if strings.EqualFold(m, modelName) {
			modelName = m
			found = true
			break
		}
	}
	if !found {
		return "", fmt.Errorf("模型 %s 当前没有启用渠道。先用 /agent 模型 查看可切换列表", modelName)
	}
	if err := model.UpdateOption("agent_setting.llm_model", modelName); err != nil {
		return "", err
	}
	cfg.LLMModel = modelName
	return fmt.Sprintf("已切换 Agent 模型：%s → %s。后续 Agent/Hermes 对话会使用新模型。", current, modelName), nil
}

func NormalizeAgentChatOpsQQ(raw map[string]any) AgentChatOpsRequest {
	roomID := jsonNumberString(raw["group_id"])
	userID := jsonNumberString(raw["user_id"])
	messageID := jsonNumberString(raw["message_id"])
	text := firstAgentNonEmpty(jsonString(raw["raw_message"]), jsonString(raw["message"]))
	username := ""
	role := ""
	if sender, ok := raw["sender"].(map[string]any); ok {
		username = firstAgentNonEmpty(jsonString(sender["card"]), jsonString(sender["nickname"]), jsonString(sender["user_id"]))
		role = jsonString(sender["role"])
	}
	return AgentChatOpsRequest{Source: "qq", RoomID: roomID, MessageID: messageID, UserExternalID: userID, Username: username, UserRole: role, Text: text, Raw: raw}
}

func NormalizeAgentChatOpsTelegram(raw map[string]any) AgentChatOpsRequest {
	msg, _ := raw["message"].(map[string]any)
	if msg == nil {
		msg, _ = raw["edited_message"].(map[string]any)
	}
	chat, _ := msg["chat"].(map[string]any)
	from, _ := msg["from"].(map[string]any)
	username := firstAgentNonEmpty(jsonString(from["username"]), strings.TrimSpace(jsonString(from["first_name"])+" "+jsonString(from["last_name"])))
	return AgentChatOpsRequest{Source: "tg", RoomID: jsonNumberString(chat["id"]), MessageID: jsonNumberString(msg["message_id"]), UserExternalID: jsonNumberString(from["id"]), Username: username, Text: jsonString(msg["text"]), Raw: raw}
}

func jsonString(v any) string {
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case float64:
		return strconv.FormatInt(int64(t), 10)
	case int:
		return strconv.Itoa(t)
	case json.Number:
		return t.String()
	default:
		return ""
	}
}
func jsonNumberString(v any) string { return jsonString(v) }
func DecodeAgentJSONRequest(r *http.Request) (map[string]any, error) {
	defer r.Body.Close()
	var raw map[string]any
	dec := json.NewDecoder(r.Body)
	dec.UseNumber()
	err := dec.Decode(&raw)
	return raw, err
}

type AgentChatOpsIdentity struct {
	NewAPIUserID    int    `json:"new_api_user_id"`
	UserBound       bool   `json:"user_bound"`
	Source          string `json:"source"`
	Provider        string `json:"provider"`
	Reason          string `json:"reason"`
	ActivatedTokens int    `json:"activated_tokens"`
}

func ResolveAgentChatOpsIdentity(req AgentChatOpsRequest) AgentChatOpsIdentity {
	source := strings.ToLower(strings.TrimSpace(req.Source))
	if source == "telegram" {
		source = "tg"
	}
	out := AgentChatOpsIdentity{Source: source}
	if req.NewAPIUserID > 0 {
		out.NewAPIUserID = req.NewAPIUserID
		out.UserBound = true
		out.Reason = "request"
		return out
	}
	uid := strings.TrimSpace(req.UserExternalID)
	roomID := strings.TrimSpace(req.RoomID)
	if uid == "" {
		out.Reason = "empty_external_user_id"
		return out
	}
	if binding, err := model.GetAgentChatBinding(AgentSiteID(), source, roomID, uid); err == nil && binding != nil && binding.NewAPIUserId > 0 {
		out.NewAPIUserID = binding.NewAPIUserId
		out.UserBound = true
		out.Provider = "agent_chat_bindings"
		out.Reason = "agent_chat_binding"
		return out
	}
	if binding, err := model.GetUserIdentityBindingByExternal(AgentSiteID(), source, uid); err == nil && binding != nil && binding.UserId > 0 {
		out.NewAPIUserID = binding.UserId
		out.UserBound = true
		out.Provider = "user_identity_bindings"
		out.Reason = "identity_binding"
		return out
	}
	if source == "tg" {
		var user model.User
		if err := model.DB.Where("telegram_id = ?", uid).First(&user).Error; err == nil && user.Id > 0 {
			out.NewAPIUserID = user.Id
			out.UserBound = true
			out.Provider = "telegram_id"
			out.Reason = "user.telegram_id"
			return out
		}
	}
	if source == "community" {
		if cfg := operation_setting.GetCommunityBotSetting(); cfg != nil {
			if userID, err := model.FindCommunityBoundUser(cfg.ProviderSlug, uid, req.Username); err == nil && userID > 0 {
				out.NewAPIUserID = userID
				out.UserBound = true
				out.Provider = "community_oauth"
				out.Reason = "community_bound_user"
				return out
			}
		}
	}
	out.Reason = "not_bound"
	return out
}

func ExecuteAgentChatOpsGameAction(ctx context.Context, req AgentChatOpsRequest, action map[string]any) (*model.AgentAction, error) {
	if action == nil {
		return nil, errors.New("action is empty")
	}
	identity := ResolveAgentChatOpsIdentity(req)
	actionPayload := normalizeGameActionPayload(action)
	actionType := strings.TrimSpace(agentPayloadString(actionPayload, "type", "action_type"))
	if actionType == "" {
		return nil, errors.New("game action missing type")
	}
	if isAgentGroupModerationAction(actionType) {
		cfg := operation_setting.GetAgentSetting()
		isAdmin := req.IsAdmin || isAgentChatOpsAdmin(ctx, cfg, req)
		if !isAdmin {
			return nil, errors.New("群管理动作需要管理员权限")
		}
		payload := cloneAgentPayload(actionPayload)
		payload["chatops_source"] = req.Source
		payload["room_id"] = req.RoomID
		payload["message_id"] = req.MessageID
		payload["issuer_external_id"] = req.UserExternalID
		payload["issuer_username"] = req.Username
		payload["issuer_role"] = req.UserRole
		payload["admin_chatops_authorized"] = true
		payload["force_execute"] = true
		if cfg != nil && !cfg.ChatOpsAllowDirectModeration {
			delete(payload, "admin_chatops_authorized")
			delete(payload, "force_execute")
			payload["force_approval"] = true
		}
		targetType, targetID, _, err := applyAgentGroupModerationDefaults(actionType, payload, req)
		if err != nil {
			return nil, err
		}
		return AgentCreateAction(AgentActionRequest{
			ActionType:     actionType,
			AgentName:      "chatops-moderator",
			TargetType:     targetType,
			TargetId:       targetID,
			Reason:         firstAgentNonEmpty(agentPayloadString(payload, "reason"), fmt.Sprintf("chatops moderation %s", actionType)),
			Payload:        payload,
			ForceApproval:  agentBoolPayload(payload, "force_approval"),
			IdempotencyKey: firstAgentNonEmpty(agentPayloadString(payload, "idempotency_key", "idem_key"), fmt.Sprintf("moderation-%s-%s-%d", AgentSiteID(), strings.ReplaceAll(actionType, ".", "-"), time.Now().UnixNano())),
		}, 0)
	}
	if actionType != "reward.grant.small" && actionType != "agent.model.manage" && actionType != "user.quota.read" && actionType != "budget.check" && actionType != "reward.settlement.batch" && actionType != "quiz.state.load" && actionType != "quiz.state.commit" && actionType != "quiz.question.draw" && actionType != "quiz.round.load" && actionType != "quiz.answer.submit" && actionType != "fund.report.read" && actionType != "fund.topup" && actionType != "site.logs.read" && actionType != "chatops.logs.read" {
		return nil, fmt.Errorf("game action type not allowed: %s", actionType)
	}
	if actionType == "site.logs.read" || actionType == "chatops.logs.read" {
		cfg := operation_setting.GetAgentSetting()
		if !(req.IsAdmin || isAgentChatOpsAdmin(ctx, cfg, req)) {
			return nil, errors.New("日志读取动作需要管理员权限")
		}
	}
	payload := cloneAgentPayload(actionPayload)
	payload["chatops_source"] = req.Source
	payload["room_id"] = req.RoomID
	payload["issuer_external_id"] = req.UserExternalID
	payload["issuer_username"] = req.Username
	payload["issuer_new_api_user_id"] = identity.NewAPIUserID
	payload["force_execute"] = true
	payload["game_action"] = true

	quizStateAction := actionType == "quiz.state.load" || actionType == "quiz.state.commit" || actionType == "quiz.question.draw" || actionType == "quiz.round.load" || actionType == "quiz.answer.submit"
	allowUnboundTarget := actionType == "budget.check" || actionType == "fund.report.read" || actionType == "fund.topup" || actionType == "site.logs.read" || actionType == "chatops.logs.read" || quizStateAction
	userID := identity.NewAPIUserID
	targetExternalID := strings.TrimSpace(agentPayloadString(actionPayload, "target_external_id", "external_user_id"))
	if quizStateAction && targetExternalID == "" {
		targetExternalID = strings.TrimSpace(req.UserExternalID)
	}
	var err error
	if !allowUnboundTarget {
		userID, targetExternalID, err = resolveGameActionTargetUserID(req, identity, actionPayload)
		if err != nil {
			return nil, err
		}
		if userID <= 0 {
			userID = identity.NewAPIUserID
		}
	}
	if actionType == "reward.grant.small" && userID <= 0 {
		return nil, errors.New("reward game action requires bound New API user")
	}
	if actionType == "reward.grant.small" && !identity.UserBound && strings.TrimSpace(targetExternalID) == "" {
		return nil, errors.New("reward game action requires a verified issuer or target binding")
	}
	quota := int(agentPayloadFloat(actionPayload, "quota_amount", "quota", "amount"))
	if actionType == "reward.grant.small" && quota == 0 {
		return nil, errors.New("reward game action requires non-zero quota")
	}
	budgetPool := firstAgentNonEmpty(agentPayloadString(actionPayload, "budget_pool", "pool"), "game")
	reason := firstAgentNonEmpty(agentPayloadString(actionPayload, "reason"), fmt.Sprintf("game action %s", actionType))
	if actionType == "reward.grant.small" {
		benefitType := detectAgentChatOpsRewardBenefitType(actionPayload, reason)
		benefitExternalID := firstAgentNonEmpty(targetExternalID, req.UserExternalID)
		if ok, gateReason := CanReceiveCommunityBenefit(userID, req.Source, req.RoomID, benefitExternalID, benefitType); !ok {
			return nil, fmt.Errorf("membership blocked for %s: %s", benefitType, gateReason)
		}
	}
	targetType := firstAgentNonEmpty(agentPayloadString(actionPayload, "target_type"), "user")
	if allowUnboundTarget && (targetType == "" || targetType == "user") {
		if quizStateAction {
			targetType = "quiz_state"
		} else if actionType == "fund.report.read" {
			targetType = "ops_fund"
		} else if actionType == "site.logs.read" || actionType == "chatops.logs.read" {
			targetType = "logs"
		} else {
			targetType = "budget"
		}
	}
	targetID := firstAgentNonEmpty(agentPayloadString(actionPayload, "target_id"), targetExternalID, strconv.Itoa(userID))
	if quizStateAction && (targetID == "" || targetID == "0") {
		targetID = firstAgentNonEmpty(agentPayloadString(actionPayload, "round_key"), req.RoomID, req.UserExternalID, AgentSiteID())
	}
	if allowUnboundTarget && targetID == "0" {
		targetID = AgentSiteID()
	}
	payload["user_id"] = userID
	payload["resolved_new_api_user_id"] = userID
	if targetExternalID != "" {
		payload["target_external_id"] = targetExternalID
	}
	idemSeed := strings.TrimSpace(agentPayloadString(actionPayload, "idempotency_key", "idem_key"))
	if idemSeed == "" && (actionType == "reward.grant.small" || actionType == "reward.settlement.batch") {
		idemSeed = defaultGameActionIdempotencyKey(req, actionPayload, actionType, userID, targetExternalID, quota, reason)
	}
	return AgentCreateAction(AgentActionRequest{
		ActionType:     actionType,
		AgentName:      "game-director",
		TargetType:     targetType,
		TargetId:       targetID,
		UserId:         userID,
		QuotaAmount:    quota,
		BudgetPool:     budgetPool,
		Reason:         reason,
		Payload:        payload,
		IdempotencyKey: idemSeed,
	}, 0)
}

func detectAgentChatOpsRewardBenefitType(actionPayload map[string]any, reason string) string {
	benefitType := strings.TrimSpace(strings.ToLower(firstAgentNonEmpty(
		agentPayloadString(actionPayload, "benefit_type", "reward_type", "source_type"),
	)))
	switch {
	case benefitType == "":
	case strings.Contains(benefitType, "invite"):
		return "invite_reward"
	case strings.Contains(benefitType, "campaign"):
		return "campaign_bonus"
	case strings.Contains(benefitType, "checkin"):
		return "checkin"
	case strings.Contains(benefitType, "game"):
		return "game_reward"
	default:
		return benefitType
	}
	reasonLower := strings.ToLower(strings.TrimSpace(reason))
	switch {
	case strings.Contains(reasonLower, "campaign"), strings.Contains(reasonLower, "bonus"), strings.Contains(reasonLower, "活动"):
		return "campaign_bonus"
	default:
		return "game_reward"
	}
}

func defaultGameActionIdempotencyKey(req AgentChatOpsRequest, actionPayload map[string]any, actionType string, userID int, targetExternalID string, quota int, reason string) string {
	period := firstAgentNonEmpty(agentPayloadString(actionPayload, "period", "date"), model.AgentBusinessDateAt(time.Now()))
	game := firstAgentNonEmpty(agentPayloadString(actionPayload, "game", "event"), reason, actionType)
	target := firstAgentNonEmpty(targetExternalID, req.UserExternalID, strconv.Itoa(userID))
	return fmt.Sprintf("game-%s-%s-%s-%s-%d-%s-%d-%s-%s", AgentSiteID(), strings.TrimSpace(req.Source), strings.TrimSpace(req.RoomID), target, userID, actionType, quota, game, period)
}

func normalizeGameActionPayload(action map[string]any) map[string]any {
	out := cloneAgentPayload(action)
	if nested, ok := action["payload"].(map[string]any); ok {
		for k, v := range nested {
			if _, exists := out[k]; !exists || out[k] == nil || fmt.Sprint(out[k]) == "" {
				out[k] = v
			}
		}
	}
	return out
}

func resolveGameActionTargetUserID(req AgentChatOpsRequest, issuer AgentChatOpsIdentity, action map[string]any) (int, string, error) {
	if explicit := int(agentPayloadFloat(action, "new_api_user_id", "target_new_api_user_id")); explicit > 0 {
		return explicit, "", nil
	}
	targetExternalID := strings.TrimSpace(agentPayloadString(action, "target_external_id", "external_user_id"))
	if targetExternalID == "" {
		maybeTarget := strings.TrimSpace(agentPayloadString(action, "target_id"))
		if maybeTarget != "" {
			if resolved, ok := resolveExternalChatUserID(req, maybeTarget); ok {
				return resolved, maybeTarget, nil
			}
			if id, ok := parsePositiveInt(maybeTarget); ok && model.DB.Where("id = ?", id).First(&model.User{}).Error == nil {
				return id, "", nil
			}
			targetExternalID = maybeTarget
		}
	}
	if targetExternalID != "" {
		if resolved, ok := resolveExternalChatUserID(req, targetExternalID); ok {
			return resolved, targetExternalID, nil
		}
	}
	candidate := strings.TrimSpace(agentPayloadString(action, "user_id", "uid"))
	if candidate != "" {
		if id, ok := parsePositiveInt(candidate); ok && issuer.UserBound && id == issuer.NewAPIUserID {
			return id, "", nil
		}
		if resolved, ok := resolveExternalChatUserID(req, candidate); ok {
			return resolved, candidate, nil
		}
		if id, ok := parsePositiveInt(candidate); ok && model.DB.Where("id = ?", id).First(&model.User{}).Error == nil {
			return id, "", nil
		}
	}
	if issuer.UserBound && issuer.NewAPIUserID > 0 {
		return issuer.NewAPIUserID, "", nil
	}
	return 0, firstAgentNonEmpty(targetExternalID, candidate), errors.New("game action target user is not bound")
}

func parsePositiveInt(value string) (int, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false
	}
	id64, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id64 <= 0 {
		return 0, false
	}
	return int(id64), true
}

func resolveExternalChatUserID(req AgentChatOpsRequest, externalID string) (int, bool) {
	externalID = strings.TrimSpace(externalID)
	if externalID == "" {
		return 0, false
	}
	probe := req
	probe.UserExternalID = externalID
	probe.NewAPIUserID = 0
	probe.UserBound = false
	ident := ResolveAgentChatOpsIdentity(probe)
	return ident.NewAPIUserID, ident.UserBound && ident.NewAPIUserID > 0
}

func PrepareAgentChatOpsBindCodeWithTTL(userID int, ttlMinutes int) (string, string, int64, error) {
	if userID <= 0 {
		return "", "", 0, errors.New("invalid user id")
	}
	code, err := randomAgentBindCode(8)
	if err != nil {
		return "", "", 0, err
	}
	if ttlMinutes <= 0 {
		ttlMinutes = 10
	}
	expiresAt := time.Now().Add(time.Duration(ttlMinutes) * time.Minute).Unix()
	codeHash := hashAgentBindCode(AgentSiteID(), code)
	return code, codeHash, expiresAt, nil
}

func CreateAgentChatOpsBindCodeWithTTL(userID int, ttlMinutes int) (string, string, int64, error) {
	code, codeHash, expiresAt, err := PrepareAgentChatOpsBindCodeWithTTL(userID, ttlMinutes)
	if err != nil {
		return "", "", 0, err
	}
	if err := model.CreateAgentChatBindCode(AgentSiteID(), userID, codeHash, expiresAt); err != nil {
		return "", "", 0, err
	}
	return code, codeHash, expiresAt, nil
}

func CreateAgentChatOpsBindCode(userID int) (string, int64, error) {
	code, _, expiresAt, err := CreateAgentChatOpsBindCodeWithTTL(userID, 10)
	if err != nil {
		return "", 0, err
	}
	return code, expiresAt, nil
}

func ConfirmAgentChatOpsBindCode(req AgentChatOpsBindConfirmRequest) (AgentChatOpsIdentity, error) {
	source := strings.ToLower(strings.TrimSpace(req.Source))
	if source == "telegram" {
		source = "tg"
	}
	if source == "" {
		source = "qq"
	}
	uid := strings.TrimSpace(req.UserExternalID)
	if uid == "" {
		return AgentChatOpsIdentity{Source: source, Reason: "empty_external_user_id"}, errors.New("missing external user id")
	}
	scope, roomID, scopeErr := normalizeAgentChatOpsBindScope(req.Scope, req.RoomID)
	if scopeErr != nil {
		reason := agentChatOpsBindReasonRoomScopeRequire
		if !errors.Is(scopeErr, errAgentChatOpsBindRoomRequired) {
			reason = agentChatOpsBindReasonBadScope
		}
		return AgentChatOpsIdentity{Source: source, Reason: reason}, scopeErr
	}
	code := strings.ToUpper(strings.TrimSpace(req.Code))
	if len(code) < 4 || len(code) > 24 {
		return AgentChatOpsIdentity{Source: source, Reason: "bad_code"}, errors.New("invalid bind code")
	}
	var row *model.AgentChatBindCode
	failureReason := "binding_write_failed"
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		var consumeErr error
		row, consumeErr = model.ConsumeAgentChatBindCodeWithTx(tx, AgentSiteID(), hashAgentBindCode(AgentSiteID(), code))
		if consumeErr != nil {
			failureReason = "code_not_found_or_expired"
			return consumeErr
		}
		binding := &model.AgentChatBinding{
			SiteId:         AgentSiteID(),
			Source:         source,
			RoomId:         roomID,
			ExternalUserId: uid,
			Username:       strings.TrimSpace(req.Username),
			NewAPIUserId:   row.UserId,
			Role:           "member",
			Enabled:        true,
			Remark:         mustAgentJSON(map[string]any{"reason": "chatops_self_bind", "room_id": strings.TrimSpace(req.RoomID), "scope": scope, "source": source}),
		}
		if err := model.UpsertAgentChatBindingWithTx(tx, binding); err != nil {
			failureReason = "binding_write_failed"
			return err
		}
		if err := model.UpsertUserIdentityBindingWithTx(tx, AgentSiteID(), row.UserId, source, uid, strings.TrimSpace(req.Username)); err != nil {
			failureReason = "identity_binding_write_failed"
			return err
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, model.ErrIdentityBindingConflict) {
			return AgentChatOpsIdentity{Source: source, Reason: "identity_binding_conflict"}, errors.New("该外部账号已绑定其他站点账号，请先解除原绑定")
		}
		if failureReason == "code_not_found_or_expired" {
			return AgentChatOpsIdentity{Source: source, Reason: failureReason}, errors.New("绑定码无效、已使用或已过期")
		}
		return AgentChatOpsIdentity{Source: source, Reason: failureReason}, err
	}
	if _, _, err := BackfillMembershipUserLinks(source, roomID, uid, row.UserId); err != nil {
		common.SysLog(fmt.Sprintf("[MembershipRisk] bind confirm backfill failed source=%s room=%s external=%s user_id=%d err=%s", source, roomID, uid, row.UserId, err.Error()))
	}
	_, _ = EvaluateUserAccessControl(context.Background(), row.UserId, true)
	activatedTokens, err := ActivateRiskTokensOnBind(req, row.UserId, code)
	if err != nil {
		return AgentChatOpsIdentity{Source: source, Reason: "risk_activation_failed"}, err
	}
	return AgentChatOpsIdentity{
		NewAPIUserID:    row.UserId,
		UserBound:       true,
		Source:          source,
		Provider:        "agent_chat_bind_code",
		Reason:          "bind_code_confirmed",
		ActivatedTokens: activatedTokens,
	}, nil
}

var errAgentChatOpsBindRoomRequired = errors.New("当前绑定必须在群聊内发起，请进入目标群后发送“绑定 绑定码”或“验牌 绑定码”完成绑定。")

func normalizeAgentChatOpsBindScope(rawScope string, rawRoomID string) (string, string, error) {
	scope := strings.ToLower(strings.TrimSpace(rawScope))
	roomID := strings.TrimSpace(rawRoomID)
	if scope == "" && roomID != "" {
		scope = agentChatOpsBindScopeRoom
	}
	if scope != "" && scope != agentChatOpsBindScopeRoom {
		return scope, roomID, fmt.Errorf("unsupported bind scope: %s", scope)
	}
	if roomID == "" {
		return scope, roomID, errAgentChatOpsBindRoomRequired
	}
	return agentChatOpsBindScopeRoom, roomID, nil
}

func randomAgentBindCode(n int) (string, error) {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	if n <= 0 {
		return "", nil
	}
	for attempt := 0; attempt < 16; attempt++ {
		buf := make([]byte, n)
		raw := make([]byte, n)
		if _, err := rand.Read(raw); err != nil {
			return "", err
		}
		hasLetter := false
		for i := range raw {
			ch := alphabet[int(raw[i])%len(alphabet)]
			buf[i] = ch
			if ch >= 'A' && ch <= 'Z' {
				hasLetter = true
			}
		}
		if hasLetter {
			return string(buf), nil
		}
	}
	return "", errors.New("failed to generate bind code with alphabetic guard")
}

func hashAgentBindCode(siteID string, code string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(siteID) + ":" + strings.ToUpper(strings.TrimSpace(code))))
	return hex.EncodeToString(sum[:])
}
