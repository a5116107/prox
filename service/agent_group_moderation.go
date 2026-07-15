package service

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/setting/operation_setting"
)

var agentGroupModerationActionSet = map[string]bool{
	"group.message.delete": true,
	"group.member.mute":    true,
	"group.member.unmute":  true,
	"group.member.kick":    true,
	"group.member.ban":     true,
	"group.member.unban":   true,
	"group.member.lookup":  true,
	"group.admin.lookup":   true,
}

func isAgentGroupModerationAction(actionType string) bool {
	return agentGroupModerationActionSet[strings.TrimSpace(actionType)]
}

func isAgentGroupModerationCommand(command string) bool {
	lower := strings.ToLower(strings.TrimSpace(command))
	return hasAny(lower,
		"撤回", "删除消息", "删消息", "delete message",
		"禁言", "解除禁言", "解禁言", "mute", "unmute",
		"踢", "移出", "踢出", "kick",
		"封禁", "解封", "ban", "unban",
		"查成员", "成员信息", "群管理员", "管理员列表")
}

func buildAgentGroupModerationActionFromCommand(command string, req AgentChatOpsRequest, base AgentActionRequest) (AgentActionRequest, error) {
	lower := strings.ToLower(strings.TrimSpace(command))
	actionType := "group.member.lookup"
	switch {
	case hasAny(lower, "管理员列表", "群管理员", "admin list", "admins"):
		actionType = "group.admin.lookup"
	case hasAny(lower, "撤回", "删除消息", "删消息", "delete message"):
		actionType = "group.message.delete"
	case hasAny(lower, "解除禁言", "解禁言", "unmute"):
		actionType = "group.member.unmute"
	case hasAny(lower, "禁言", "mute"):
		actionType = "group.member.mute"
	case hasAny(lower, "解除封禁", "解封", "unban"):
		actionType = "group.member.unban"
	case hasAny(lower, "封禁", "ban"):
		actionType = "group.member.ban"
	case hasAny(lower, "踢出", "移出", "踢", "kick"):
		actionType = "group.member.kick"
	case hasAny(lower, "查成员", "成员信息", "lookup"):
		actionType = "group.member.lookup"
	}
	base.ActionType = actionType
	base.Reason = command
	base.Payload["reason"] = command
	if target := extractAgentGroupModerationTarget(command, req); target != "" {
		base.Payload["target_external_id"] = target
	}
	if messageID := extractAgentGroupModerationMessageID(req); messageID != "" {
		base.Payload["target_message_id"] = messageID
	}
	if actionType == "group.member.mute" {
		base.Payload["duration_seconds"] = extractAgentGroupModerationDuration(command)
	}
	targetType, targetID, _, err := applyAgentGroupModerationDefaults(actionType, base.Payload, req)
	if err != nil {
		return base, err
	}
	base.TargetType = targetType
	base.TargetId = targetID
	return base, nil
}

func applyAgentGroupModerationDefaults(actionType string, payload map[string]any, req AgentChatOpsRequest) (string, string, int, error) {
	cfg := operation_setting.GetAgentSetting()
	if cfg == nil || !cfg.ChatOpsGroupModerationEnabled {
		return "", "", 0, errors.New("群管理能力未开启，请在 Agent 运营配置里启用 chatops_group_moderation_enabled")
	}
	if !isAgentGroupModerationAction(actionType) {
		return "", "", 0, fmt.Errorf("不是群管理动作：%s", actionType)
	}
	source := normalizeAgentModerationSource(firstAgentNonEmpty(agentPayloadString(payload, "chatops_source", "source", "platform"), req.Source))
	payload["chatops_source"] = source
	roomID := firstAgentNonEmpty(agentPayloadString(payload, "room_id", "group_id", "chat_id"), req.RoomID)
	if roomID == "" {
		if source == "qq" {
			roomID = cfg.QQGroupID
		} else if source == "tg" {
			roomID = cfg.TGChatID
		}
	}
	if roomID == "" && actionType != "group.message.delete" {
		return "", "", 0, errors.New("缺少群号/聊天室 ID")
	}
	payload["room_id"] = roomID
	payload["group_id"] = roomID
	payload["chat_id"] = roomID

	targetUser := firstAgentNonEmpty(
		agentPayloadString(payload, "target_external_id", "target_user_id", "member_id", "user_external_id", "user_id", "qq", "tg_user_id"),
		extractAgentGroupModerationTarget("", req),
	)
	messageID := firstAgentNonEmpty(agentPayloadString(payload, "target_message_id", "reply_message_id", "message_id"), extractAgentGroupModerationMessageID(req))
	if actionType == "group.message.delete" {
		if messageID == "" {
			return "", "", 0, errors.New("撤回消息需要 target_message_id 或 reply_message_id")
		}
		payload["target_message_id"] = messageID
		return "group_message", messageID, 0, nil
	}
	if actionType == "group.admin.lookup" {
		return "group_admins", roomID, 0, nil
	}
	if targetUser == "" {
		return "", "", 0, errors.New("群成员操作需要 target_external_id；可以 @ 对方、回复对方消息，或说出 QQ/TG 用户 ID")
	}
	payload["target_external_id"] = targetUser
	duration := 0
	if actionType == "group.member.mute" {
		duration = int(agentPayloadFloat(payload, "duration_seconds", "duration", "mute_seconds"))
		if duration <= 0 {
			duration = 600
		}
		maxMute := cfg.ChatOpsMaxMuteSeconds
		if maxMute <= 0 {
			maxMute = 3600
		}
		if duration > maxMute {
			duration = maxMute
		}
		payload["duration_seconds"] = duration
	}
	return "group_member", targetUser, duration, nil
}

func normalizeAgentModerationSource(source string) string {
	source = strings.ToLower(strings.TrimSpace(source))
	if source == "telegram" {
		return "tg"
	}
	return source
}

func extractAgentGroupModerationTarget(command string, req AgentChatOpsRequest) string {
	for _, key := range []string{"target_external_id", "target_user_id", "reply_to_user_id", "member_id"} {
		if req.Raw != nil {
			if v := jsonString(req.Raw[key]); v != "" {
				return v
			}
		}
	}
	text := strings.TrimSpace(command)
	if text == "" && req.Raw != nil {
		text = firstAgentNonEmpty(jsonString(req.Raw["raw_message"]), jsonString(req.Raw["message"]), jsonString(req.Raw["text"]))
	}
	patterns := []string{
		`\[CQ:at,qq=(\d{5,20})\]`,
		`(?:用户|成员|账号|QQ号|qq号)\s*(?:id|号)?\s*[:=：#]?\s*(\d{5,20})`,
		`(?i)\b(?:qq|user|uid|member)\s*(?:id|no)?\s*[:=：#]?\s*(\d{5,20})\b`,
		`@(\d{5,20})\b`,
	}
	for _, p := range patterns {
		re := regexp.MustCompile(p)
		if m := re.FindStringSubmatch(text); len(m) >= 2 {
			return strings.TrimSpace(m[1])
		}
	}
	generic := regexp.MustCompile(`\b(\d{5,20})\b`).FindAllStringSubmatch(text, -1)
	if len(generic) > 0 {
		return strings.TrimSpace(generic[len(generic)-1][1])
	}
	return ""
}

func extractAgentGroupModerationMessageID(req AgentChatOpsRequest) string {
	if req.Raw != nil {
		for _, key := range []string{"target_message_id", "reply_message_id"} {
			if v := jsonString(req.Raw[key]); v != "" {
				return v
			}
		}
		rawText := firstAgentNonEmpty(jsonString(req.Raw["raw_message"]), jsonString(req.Raw["message"]))
		if rawText != "" {
			if m := regexp.MustCompile(`\[CQ:reply,id=(-?\d+)\]`).FindStringSubmatch(rawText); len(m) >= 2 {
				return m[1]
			}
		}
	}
	return ""
}

func extractAgentGroupModerationDuration(command string) int {
	lower := strings.ToLower(strings.TrimSpace(command))
	re := regexp.MustCompile(`(\d+)\s*(秒|s|sec|分钟|分|m|min|小时|时|h|天|d)`)
	if m := re.FindStringSubmatch(lower); len(m) >= 3 {
		n, _ := strconv.Atoi(m[1])
		switch m[2] {
		case "秒", "s", "sec":
			return n
		case "分钟", "分", "m", "min":
			return n * 60
		case "小时", "时", "h":
			return n * 3600
		case "天", "d":
			return n * 86400
		}
	}
	return 600
}

func executeAgentGroupModerationAction(actionType string, payload map[string]any) (map[string]any, error) {
	cfg := operation_setting.GetAgentSetting()
	if cfg == nil || !cfg.ChatOpsGroupModerationEnabled {
		return nil, errors.New("群管理能力未开启")
	}
	if !cfg.ChatOpsAllowKick && actionType == "group.member.kick" {
		return nil, errors.New("移出群成员能力未开启")
	}
	if !cfg.ChatOpsAllowBan && (actionType == "group.member.ban" || actionType == "group.member.unban") {
		return nil, errors.New("封禁/解封能力未开启")
	}
	source := normalizeAgentModerationSource(firstAgentNonEmpty(agentPayloadString(payload, "chatops_source", "source", "platform"), "qq"))
	roomID := firstAgentNonEmpty(agentPayloadString(payload, "room_id", "group_id", "chat_id"), cfg.QQGroupID)
	if source == "tg" {
		roomID = firstAgentNonEmpty(agentPayloadString(payload, "chat_id", "room_id", "group_id"), cfg.TGChatID)
	}
	targetUser := agentPayloadString(payload, "target_external_id", "target_user_id", "member_id", "user_external_id", "user_id")
	messageID := agentPayloadString(payload, "target_message_id", "reply_message_id", "message_id")
	duration := int(agentPayloadFloat(payload, "duration_seconds", "duration", "mute_seconds"))
	reason := firstAgentNonEmpty(agentPayloadString(payload, "reason"), "群管理")

	if agentBoolPayload(payload, "dry_run", "mock_execute", "test_mode") {
		return map[string]any{
			"ok":                 true,
			"dry_run":            true,
			"summary":            humanAgentGroupModerationDryRunSummary(actionType, source, roomID, targetUser, messageID, duration),
			"action_type":        actionType,
			"chatops_source":     source,
			"room_id":            roomID,
			"target_external_id": targetUser,
			"target_message_id":  messageID,
			"duration_seconds":   duration,
			"reason":             reason,
		}, nil
	}

	switch source {
	case "qq":
		return executeAgentQQModeration(actionType, roomID, targetUser, messageID, duration, reason)
	case "tg":
		return executeAgentTelegramModeration(actionType, roomID, targetUser, messageID, duration, reason)
	default:
		return nil, fmt.Errorf("群管理暂不支持来源：%s", source)
	}
}

func humanAgentGroupModerationDryRunSummary(actionType, source, roomID, targetUser, messageID string, duration int) string {
	switch actionType {
	case "group.message.delete":
		return fmt.Sprintf("已预演：将撤回 %s 群 %s 的消息 %s。", strings.ToUpper(source), roomID, messageID)
	case "group.member.mute":
		return fmt.Sprintf("已预演：将禁言 %s 群 %s 的成员 %s %s。", strings.ToUpper(source), roomID, targetUser, humanAgentDuration(duration))
	case "group.member.unmute":
		return fmt.Sprintf("已预演：将解除 %s 群 %s 的成员 %s 禁言。", strings.ToUpper(source), roomID, targetUser)
	case "group.member.kick":
		return fmt.Sprintf("已预演：将移出 %s 群 %s 的成员 %s。", strings.ToUpper(source), roomID, targetUser)
	case "group.member.ban":
		return fmt.Sprintf("已预演：将封禁 %s 群 %s 的成员 %s。", strings.ToUpper(source), roomID, targetUser)
	case "group.member.unban":
		return fmt.Sprintf("已预演：将解除 %s 群 %s 的成员 %s 封禁。", strings.ToUpper(source), roomID, targetUser)
	case "group.member.lookup":
		return fmt.Sprintf("已预演：将查询 %s 群 %s 的成员 %s。", strings.ToUpper(source), roomID, targetUser)
	case "group.admin.lookup":
		return fmt.Sprintf("已预演：将查询 %s 群 %s 的管理员列表。", strings.ToUpper(source), roomID)
	default:
		return fmt.Sprintf("已预演：%s source=%s room=%s target=%s message=%s。", actionType, source, roomID, targetUser, messageID)
	}
}

func executeAgentQQModeration(actionType, groupID, targetUser, messageID string, duration int, reason string) (map[string]any, error) {
	ctx := context.Background()
	switch actionType {
	case "group.message.delete":
		if err := AgentQQDeleteMessage(ctx, messageID); err != nil {
			return nil, err
		}
		return map[string]any{"ok": true, "summary": "消息已撤回。", "message_id": messageID}, nil
	case "group.member.mute":
		if err := AgentQQSetGroupBan(ctx, groupID, targetUser, duration); err != nil {
			return nil, err
		}
		return map[string]any{"ok": true, "summary": fmt.Sprintf("已禁言 %s %s。", targetUser, humanAgentDuration(duration)), "group_id": groupID, "target_external_id": targetUser, "duration_seconds": duration}, nil
	case "group.member.unmute":
		if err := AgentQQSetGroupBan(ctx, groupID, targetUser, 0); err != nil {
			return nil, err
		}
		return map[string]any{"ok": true, "summary": fmt.Sprintf("已解除 %s 的禁言。", targetUser), "group_id": groupID, "target_external_id": targetUser}, nil
	case "group.member.kick":
		if err := AgentQQKickGroupMember(ctx, groupID, targetUser, false); err != nil {
			return nil, err
		}
		return map[string]any{"ok": true, "summary": fmt.Sprintf("已将 %s 移出 QQ 群。", targetUser), "group_id": groupID, "target_external_id": targetUser, "reason": reason}, nil
	case "group.member.ban":
		if err := AgentQQKickGroupMember(ctx, groupID, targetUser, true); err != nil {
			return nil, err
		}
		return map[string]any{"ok": true, "summary": fmt.Sprintf("已将 %s 移出，并拒绝再次加群。", targetUser), "group_id": groupID, "target_external_id": targetUser, "reason": reason}, nil
	case "group.member.unban":
		return nil, errors.New("QQ OneBot 不支持从群黑名单直接解除封禁，请在 QQ 群管理界面处理")
	case "group.member.lookup":
		info, err := AgentQQGetGroupMemberInfo(ctx, groupID, targetUser)
		if err != nil {
			return nil, err
		}
		return map[string]any{"ok": true, "summary": "已查到 QQ 群成员信息。", "member": info, "group_id": groupID, "target_external_id": targetUser}, nil
	case "group.admin.lookup":
		admins, err := AgentQQGetGroupAdmins(ctx, groupID)
		if err != nil {
			return nil, err
		}
		return map[string]any{"ok": true, "summary": fmt.Sprintf("当前 QQ 群管理员 %d 人。", len(admins)), "admins": admins, "group_id": groupID}, nil
	}
	return nil, fmt.Errorf("未知 QQ 群管理动作：%s", actionType)
}

func executeAgentTelegramModeration(actionType, chatID, targetUser, messageID string, duration int, reason string) (map[string]any, error) {
	ctx := context.Background()
	switch actionType {
	case "group.message.delete":
		if err := AgentTelegramDeleteMessage(ctx, chatID, messageID); err != nil {
			return nil, err
		}
		return map[string]any{"ok": true, "summary": "消息已撤回。", "chat_id": chatID, "message_id": messageID}, nil
	case "group.member.mute":
		if err := AgentTelegramRestrictMember(ctx, chatID, targetUser, duration, false); err != nil {
			return nil, err
		}
		return map[string]any{"ok": true, "summary": fmt.Sprintf("已禁言 %s %s。", targetUser, humanAgentDuration(duration)), "chat_id": chatID, "target_external_id": targetUser, "duration_seconds": duration}, nil
	case "group.member.unmute":
		if err := AgentTelegramRestrictMember(ctx, chatID, targetUser, 0, true); err != nil {
			return nil, err
		}
		return map[string]any{"ok": true, "summary": fmt.Sprintf("已解除 %s 的禁言。", targetUser), "chat_id": chatID, "target_external_id": targetUser}, nil
	case "group.member.kick":
		if err := AgentTelegramBanMember(ctx, chatID, targetUser, 0); err != nil {
			return nil, err
		}
		_ = AgentTelegramUnbanMember(ctx, chatID, targetUser, true)
		return map[string]any{"ok": true, "summary": fmt.Sprintf("已将 %s 移出 TG 群。", targetUser), "chat_id": chatID, "target_external_id": targetUser, "reason": reason}, nil
	case "group.member.ban":
		if err := AgentTelegramBanMember(ctx, chatID, targetUser, 0); err != nil {
			return nil, err
		}
		return map[string]any{"ok": true, "summary": fmt.Sprintf("已封禁 %s。", targetUser), "chat_id": chatID, "target_external_id": targetUser, "reason": reason}, nil
	case "group.member.unban":
		if err := AgentTelegramUnbanMember(ctx, chatID, targetUser, false); err != nil {
			return nil, err
		}
		return map[string]any{"ok": true, "summary": fmt.Sprintf("已解除 %s 的封禁。", targetUser), "chat_id": chatID, "target_external_id": targetUser}, nil
	case "group.member.lookup":
		info, err := AgentTelegramGetChatMember(ctx, chatID, targetUser)
		if err != nil {
			return nil, err
		}
		return map[string]any{"ok": true, "summary": "已查到 TG 群成员信息。", "member": info, "chat_id": chatID, "target_external_id": targetUser}, nil
	case "group.admin.lookup":
		admins, err := AgentTelegramGetAdministrators(ctx, chatID)
		if err != nil {
			return nil, err
		}
		return map[string]any{"ok": true, "summary": fmt.Sprintf("当前 TG 群管理员 %d 人。", len(admins)), "admins": admins, "chat_id": chatID}, nil
	}
	return nil, fmt.Errorf("未知 TG 群管理动作：%s", actionType)
}

func AgentQQDeleteMessage(ctx context.Context, messageID string) error {
	return postAgentOneBot(ctx, "delete_msg", map[string]any{"message_id": parseAgentNumberish(messageID)}, nil)
}

func AgentQQSetGroupBan(ctx context.Context, groupID, userID string, duration int) error {
	return postAgentOneBot(ctx, "set_group_ban", map[string]any{"group_id": parseAgentNumberish(groupID), "user_id": parseAgentNumberish(userID), "duration": duration}, nil)
}

func AgentQQKickGroupMember(ctx context.Context, groupID, userID string, rejectAddRequest bool) error {
	return postAgentOneBot(ctx, "set_group_kick", map[string]any{"group_id": parseAgentNumberish(groupID), "user_id": parseAgentNumberish(userID), "reject_add_request": rejectAddRequest}, nil)
}

func AgentQQGetGroupMemberInfo(ctx context.Context, groupID, userID string) (map[string]any, error) {
	var out map[string]any
	if err := postAgentOneBot(ctx, "get_group_member_info", map[string]any{"group_id": parseAgentNumberish(groupID), "user_id": parseAgentNumberish(userID), "no_cache": false}, &out); err != nil {
		return nil, err
	}
	if data, ok := out["data"].(map[string]any); ok {
		return data, nil
	}
	return out, nil
}

func AgentQQGetGroupAdmins(ctx context.Context, groupID string) ([]map[string]any, error) {
	var out map[string]any
	if err := postAgentOneBot(ctx, "get_group_member_list", map[string]any{"group_id": parseAgentNumberish(groupID)}, &out); err != nil {
		return nil, err
	}
	admins := []map[string]any{}
	if rows, ok := out["data"].([]any); ok {
		for _, item := range rows {
			row, ok := item.(map[string]any)
			if !ok {
				continue
			}
			role := strings.ToLower(fmt.Sprint(row["role"]))
			if role == "owner" || role == "admin" || role == "administrator" {
				admins = append(admins, row)
			}
		}
	}
	return admins, nil
}

func postAgentOneBot(ctx context.Context, action string, payload map[string]any, out *map[string]any) error {
	cfg := operation_setting.GetAgentSetting()
	if !cfg.QQBotEnabled {
		return errors.New("qq bot is disabled")
	}
	baseURL := strings.TrimSpace(cfg.QQOneBotURL)
	if baseURL == "" {
		return errors.New("qq onebot url is empty")
	}
	result := map[string]any{}
	if err := postAgentJSON(ctx, strings.TrimRight(baseURL, "/")+"/"+action, cfg.QQAccessToken, payload, &result); err != nil {
		return err
	}
	if ret := int(agentPayloadFloat(result, "retcode")); ret != 0 {
		return fmt.Errorf("OneBot %s retcode=%d message=%s", action, ret, firstAgentNonEmpty(agentPayloadString(result, "message", "wording"), "执行失败"))
	}
	if out != nil {
		*out = result
	}
	return nil
}

func AgentTelegramDeleteMessage(ctx context.Context, chatID, messageID string) error {
	return postAgentTelegram(ctx, "deleteMessage", map[string]any{"chat_id": parseAgentNumberish(chatID), "message_id": parseAgentNumberish(messageID)}, nil)
}

func AgentTelegramRestrictMember(ctx context.Context, chatID, userID string, duration int, allow bool) error {
	perms := map[string]any{
		"can_send_messages":         allow,
		"can_send_audios":           allow,
		"can_send_documents":        allow,
		"can_send_photos":           allow,
		"can_send_videos":           allow,
		"can_send_video_notes":      allow,
		"can_send_voice_notes":      allow,
		"can_send_polls":            allow,
		"can_send_other_messages":   allow,
		"can_add_web_page_previews": allow,
	}
	payload := map[string]any{"chat_id": parseAgentNumberish(chatID), "user_id": parseAgentNumberish(userID), "permissions": perms}
	if !allow && duration > 0 {
		payload["until_date"] = time.Now().Add(time.Duration(duration) * time.Second).Unix()
	}
	return postAgentTelegram(ctx, "restrictChatMember", payload, nil)
}

func AgentTelegramBanMember(ctx context.Context, chatID, userID string, until int64) error {
	payload := map[string]any{"chat_id": parseAgentNumberish(chatID), "user_id": parseAgentNumberish(userID)}
	if until > 0 {
		payload["until_date"] = until
	}
	return postAgentTelegram(ctx, "banChatMember", payload, nil)
}

func AgentTelegramUnbanMember(ctx context.Context, chatID, userID string, onlyIfBanned bool) error {
	return postAgentTelegram(ctx, "unbanChatMember", map[string]any{"chat_id": parseAgentNumberish(chatID), "user_id": parseAgentNumberish(userID), "only_if_banned": onlyIfBanned}, nil)
}

func AgentTelegramGetChatMember(ctx context.Context, chatID, userID string) (map[string]any, error) {
	var out map[string]any
	if err := postAgentTelegram(ctx, "getChatMember", map[string]any{"chat_id": parseAgentNumberish(chatID), "user_id": parseAgentNumberish(userID)}, &out); err != nil {
		return nil, err
	}
	if data, ok := out["result"].(map[string]any); ok {
		return data, nil
	}
	return out, nil
}

func AgentTelegramGetAdministrators(ctx context.Context, chatID string) ([]map[string]any, error) {
	var out map[string]any
	if err := postAgentTelegram(ctx, "getChatAdministrators", map[string]any{"chat_id": parseAgentNumberish(chatID)}, &out); err != nil {
		return nil, err
	}
	admins := []map[string]any{}
	if rows, ok := out["result"].([]any); ok {
		for _, item := range rows {
			if row, ok := item.(map[string]any); ok {
				admins = append(admins, row)
			}
		}
	}
	return admins, nil
}

func postAgentTelegram(ctx context.Context, method string, payload map[string]any, out *map[string]any) error {
	cfg := operation_setting.GetAgentSetting()
	if !cfg.TGBotEnabled {
		return errors.New("telegram bot is disabled")
	}
	token := strings.TrimSpace(cfg.TGBotToken)
	if token == "" {
		return errors.New("telegram token is empty")
	}
	result := map[string]any{}
	if err := postAgentJSON(ctx, "https://api.telegram.org/bot"+url.PathEscape(token)+"/"+method, "", payload, &result); err != nil {
		return err
	}
	if ok, _ := result["ok"].(bool); !ok {
		return fmt.Errorf("Telegram %s failed: %s", method, firstAgentNonEmpty(agentPayloadString(result, "description", "error"), "执行失败"))
	}
	if out != nil {
		*out = result
	}
	return nil
}

func humanAgentDuration(seconds int) string {
	if seconds <= 0 {
		return "0 秒"
	}
	if seconds%86400 == 0 {
		return fmt.Sprintf("%d 天", seconds/86400)
	}
	if seconds%3600 == 0 {
		return fmt.Sprintf("%d 小时", seconds/3600)
	}
	if seconds%60 == 0 {
		return fmt.Sprintf("%d 分钟", seconds/60)
	}
	return fmt.Sprintf("%d 秒", seconds)
}
