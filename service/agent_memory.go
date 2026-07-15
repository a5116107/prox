package service

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"gorm.io/gorm"
)

const (
	agentMemoryDefaultAgent = "director"
	agentMemoryMaxValueLen  = 2000
)

var agentMemoryScopes = map[string]bool{
	"site": true, "user": true, "group": true, "ops": true, "risk": true, "game": true, "policy": true,
}

type AgentMemoryUpsertRequest struct {
	AgentName  string         `json:"agent_name"`
	Scope      string         `json:"scope"`
	Key        string         `json:"key"`
	Value      string         `json:"value"`
	Source     string         `json:"source"`
	Confidence float64        `json:"confidence"`
	TaskID     int            `json:"task_id,omitempty"`
	TTLSeconds int64          `json:"ttl_seconds,omitempty"`
	Extra      map[string]any `json:"extra,omitempty"`
}

type AgentMemoryPayload struct {
	Value       string         `json:"value"`
	Source      string         `json:"source"`
	Confidence  float64        `json:"confidence"`
	ValidatedAt int64          `json:"validated_at"`
	TaskID      int            `json:"task_id,omitempty"`
	Extra       map[string]any `json:"extra,omitempty"`
}

type AgentMemoryContextItem struct {
	Scope      string  `json:"scope"`
	Key        string  `json:"key"`
	Value      string  `json:"value"`
	Source     string  `json:"source"`
	Confidence float64 `json:"confidence"`
	UpdatedAt  int64   `json:"updated_at"`
}

func AgentEnsureDefaultMemories() error {
	var existing model.AgentMemory
	if err := model.DB.Where("site_id = ? AND agent_name = ? AND scope = ? AND memory_key = ?", AgentSiteID(), agentMemoryDefaultAgent, "site", "architecture.agent-boundary").First(&existing).Error; err == nil {
		return nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	_, err := AgentUpsertMemory(AgentMemoryUpsertRequest{
		AgentName:  agentMemoryDefaultAgent,
		Scope:      "site",
		Key:        "architecture.agent-boundary",
		Value:      "本站的社区、QQ/TG 群运营分别是独立运营 Agent；New API 只是该站点 Agent 的工具与数据面，用于风控、额度、发奖、状态与审计。记忆、策略和任务按站点独立沉淀，不跨站共享。",
		Source:     "system_baseline",
		Confidence: 1,
		Extra: map[string]any{
			"site_id": AgentSiteID(),
			"scopes":  []string{"site", "user", "group", "ops", "risk", "game", "policy"},
		},
	})
	return err
}

func AgentUpsertMemory(req AgentMemoryUpsertRequest) (*model.AgentMemory, error) {
	siteID := AgentSiteID()
	agentName := firstAgentNonEmpty(strings.TrimSpace(req.AgentName), agentMemoryDefaultAgent)
	scope, err := normalizeAgentMemoryScope(req.Scope)
	if err != nil {
		return nil, err
	}
	key := normalizeAgentMemoryKey(req.Key)
	if key == "" {
		return nil, errors.New("memory key is required")
	}
	value := strings.TrimSpace(req.Value)
	if value == "" {
		return nil, errors.New("memory value is required")
	}
	if agentMemoryLooksSecret(value) || agentMemoryLooksSecret(key) {
		return nil, errors.New("安全策略已拦截：长期记忆不保存 token、密码、密钥或 Cookie")
	}
	value = truncateAgentText(value, agentMemoryMaxValueLen)
	confidence := req.Confidence
	if confidence <= 0 {
		confidence = 0.7
	}
	if confidence > 1 {
		confidence = 1
	}
	now := time.Now().Unix()
	expiresAt := int64(0)
	if req.TTLSeconds > 0 {
		expiresAt = now + req.TTLSeconds
	}
	payload, _ := json.Marshal(AgentMemoryPayload{Value: value, Source: firstAgentNonEmpty(req.Source, "agent"), Confidence: confidence, ValidatedAt: now, TaskID: req.TaskID, Extra: req.Extra})
	row := model.AgentMemory{}
	err = model.DB.Where("site_id = ? AND agent_name = ? AND scope = ? AND memory_key = ?", siteID, agentName, scope, key).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		row = model.AgentMemory{SiteId: siteID, AgentName: agentName, Scope: scope, MemoryKey: key, MemoryValue: string(payload), ExpiresAt: expiresAt}
		if err := model.DB.Create(&row).Error; err != nil {
			return nil, err
		}
		_ = model.DB.Create(&model.AgentEvent{SiteId: siteID, EventType: "memory.saved", Source: firstAgentNonEmpty(req.Source, "agent"), Severity: "info", Status: "closed", Title: scope + ":" + key, PayloadJson: mustAgentJSON(map[string]any{"scope": scope, "key": key, "confidence": confidence, "task_id": req.TaskID})}).Error
		return &row, nil
	}
	if err != nil {
		return nil, err
	}
	updates := map[string]any{"memory_value": string(payload), "expires_at": expiresAt, "updated_at": now}
	if err := model.DB.Model(&model.AgentMemory{}).Where("id = ? AND site_id = ?", row.Id, siteID).Updates(updates).Error; err != nil {
		return nil, err
	}
	_ = model.DB.Where("id = ? AND site_id = ?", row.Id, siteID).First(&row).Error
	_ = model.DB.Create(&model.AgentEvent{SiteId: siteID, EventType: "memory.updated", Source: firstAgentNonEmpty(req.Source, "agent"), Severity: "info", Status: "closed", Title: scope + ":" + key, PayloadJson: mustAgentJSON(map[string]any{"scope": scope, "key": key, "confidence": confidence, "task_id": req.TaskID})}).Error
	return &row, nil
}

func AgentListMemories(agentName string, scope string, limit int) ([]model.AgentMemory, error) {
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	agentName = firstAgentNonEmpty(strings.TrimSpace(agentName), agentMemoryDefaultAgent)
	now := time.Now().Unix()
	tx := model.DB.Where("site_id = ? AND agent_name = ? AND (expires_at = 0 OR expires_at > ?)", AgentSiteID(), agentName, now).Order("updated_at desc, id desc").Limit(limit)
	if strings.TrimSpace(scope) != "" {
		normalized, err := normalizeAgentMemoryScope(scope)
		if err != nil {
			return nil, err
		}
		tx = tx.Where("scope = ?", normalized)
	}
	var rows []model.AgentMemory
	err := tx.Find(&rows).Error
	return rows, err
}

func AgentForgetMemory(agentName string, scope string, key string) (int64, error) {
	agentName = firstAgentNonEmpty(strings.TrimSpace(agentName), agentMemoryDefaultAgent)
	scope, err := normalizeAgentMemoryScope(scope)
	if err != nil {
		return 0, err
	}
	key = normalizeAgentMemoryKey(key)
	if key == "" {
		return 0, errors.New("memory key is required")
	}
	res := model.DB.Where("site_id = ? AND agent_name = ? AND scope = ? AND memory_key = ?", AgentSiteID(), agentName, scope, key).Delete(&model.AgentMemory{})
	if res.Error != nil {
		return 0, res.Error
	}
	_ = model.DB.Create(&model.AgentEvent{SiteId: AgentSiteID(), EventType: "memory.forgotten", Source: "admin", Severity: "info", Status: "closed", Title: scope + ":" + key, PayloadJson: mustAgentJSON(map[string]any{"scope": scope, "key": key, "deleted": res.RowsAffected})}).Error
	return res.RowsAffected, nil
}

func AgentPromoteMemory(agentName string, scope string, key string, source string) (*model.AgentMemory, error) {
	agentName = firstAgentNonEmpty(strings.TrimSpace(agentName), agentMemoryDefaultAgent)
	scope, err := normalizeAgentMemoryScope(scope)
	if err != nil {
		return nil, err
	}
	key = normalizeAgentMemoryKey(key)
	var row model.AgentMemory
	if err := model.DB.Where("site_id = ? AND agent_name = ? AND scope = ? AND memory_key = ?", AgentSiteID(), agentName, scope, key).First(&row).Error; err != nil {
		return nil, err
	}
	payload := decodeAgentMemoryPayload(row)
	payload.Confidence = 1
	payload.Source = firstAgentNonEmpty(source, "admin_promote")
	payload.ValidatedAt = time.Now().Unix()
	body, _ := json.Marshal(payload)
	if err := model.DB.Model(&model.AgentMemory{}).Where("id = ? AND site_id = ?", row.Id, AgentSiteID()).Updates(map[string]any{"memory_value": string(body), "updated_at": time.Now().Unix()}).Error; err != nil {
		return nil, err
	}
	_ = model.DB.Where("id = ? AND site_id = ?", row.Id, AgentSiteID()).First(&row).Error
	_ = model.DB.Create(&model.AgentEvent{SiteId: AgentSiteID(), EventType: "memory.promoted", Source: payload.Source, Severity: "info", Status: "closed", Title: scope + ":" + key}).Error
	return &row, nil
}

func AgentMemoryContextForChatOps(req AgentChatOpsRequest) map[string]any {
	items := agentMemoryContextItems(req, 14)
	summary := formatAgentMemorySummary(items)
	return map[string]any{
		"site_id":   AgentSiteID(),
		"agent":     agentMemoryDefaultAgent,
		"summary":   summary,
		"items":     items,
		"isolation": "memory is isolated by site_id, agent_name, scope and memory_key; do not reuse memories across sites",
	}
}

func AgentHandleMemoryCommand(req AgentChatOpsRequest, command string, taskID int) (string, error) {
	cmd := strings.TrimSpace(command)
	lower := strings.ToLower(cmd)
	if lower == "memory" || lower == "memory list" || cmd == "记忆" || hasAny(lower, "memory list", "记忆列表", "列出记忆", "查看记忆") {
		scope := firstAgentNonEmpty(agentMemoryCommandField(cmd, "scope", "范围"), inferScopeWord(cmd))
		rows, err := AgentListMemories(agentMemoryDefaultAgent, scope, 30)
		if err != nil {
			return "记忆读取失败：" + err.Error(), err
		}
		return formatAgentMemoryList(rows), nil
	}
	if hasAny(lower, "memory forget", "memory delete", "忘记", "删除记忆") {
		scope := firstAgentNonEmpty(agentMemoryCommandField(cmd, "scope", "范围"), inferScopeWord(cmd), "site")
		key := firstAgentNonEmpty(agentMemoryCommandField(cmd, "key", "键"), lastAgentMemoryWord(cmd))
		deleted, err := AgentForgetMemory(agentMemoryDefaultAgent, scope, key)
		if err != nil {
			return "记忆删除失败：" + err.Error(), err
		}
		return fmt.Sprintf("已删除 %d 条记忆：%s/%s", deleted, scope, normalizeAgentMemoryKey(key)), nil
	}
	if hasAny(lower, "memory promote", "确认记忆", "提升记忆") {
		scope := firstAgentNonEmpty(agentMemoryCommandField(cmd, "scope", "范围"), inferScopeWord(cmd), "site")
		key := firstAgentNonEmpty(agentMemoryCommandField(cmd, "key", "键"), lastAgentMemoryWord(cmd))
		row, err := AgentPromoteMemory(agentMemoryDefaultAgent, scope, key, "admin_promote")
		if err != nil {
			return "记忆确认失败：" + err.Error(), err
		}
		return fmt.Sprintf("已确认记忆：%s/%s", row.Scope, row.MemoryKey), nil
	}
	value := firstAgentNonEmpty(agentMemoryCommandTextValue(cmd), trimAgentMemorySaveHead(cmd))
	scope := firstAgentNonEmpty(agentMemoryCommandField(cmd, "scope", "范围"), inferAgentMemoryScope(value), "site")
	key := firstAgentNonEmpty(agentMemoryCommandField(cmd, "key", "键"), "note:"+shortAgentMemoryHash(value))
	row, err := AgentUpsertMemory(AgentMemoryUpsertRequest{AgentName: agentMemoryDefaultAgent, Scope: scope, Key: key, Value: value, Source: "admin_chatops", Confidence: 1, TaskID: taskID, Extra: map[string]any{"source": req.Source, "room_id": req.RoomID, "user_external_id": req.UserExternalID, "username": req.Username}})
	if err != nil {
		return "记忆保存失败：" + err.Error(), err
	}
	return fmt.Sprintf("已保存记忆：%s/%s\n%s", row.Scope, row.MemoryKey, truncateAgentText(value, 300)), nil
}

func AgentLearnFromFinishedTask(task *model.AgentTask, status string, reply string, action *model.AgentAction, taskErr error) {
	if task == nil || taskErr != nil || isAgentMemoryCommand(task.Command) || isHelpCommand(task.Command) {
		return
	}
	if task.IssuerRole != "admin" {
		return
	}
	status = strings.ToLower(strings.TrimSpace(status))
	if status != "completed" && status != "queued" {
		return
	}
	command := strings.TrimSpace(firstAgentNonEmpty(task.Command, task.Text))
	if command == "" || agentMemoryLooksSecret(command) {
		return
	}
	now := time.Now().Unix()
	if hasAny(strings.ToLower(command), "纠正", "以后", "规则", "policy", "偏好", "人设", "口径", "不要再", "统一按") {
		scope := inferAgentMemoryScope(command)
		_, _ = AgentUpsertMemory(AgentMemoryUpsertRequest{AgentName: task.AgentName, Scope: scope, Key: "admin-note:" + shortAgentMemoryHash(command), Value: command, Source: "admin_correction", Confidence: 0.95, TaskID: task.Id, Extra: map[string]any{"source": task.Source, "room_id": task.RoomId, "message_id": task.MessageId}})
	}
	if action != nil && action.Id > 0 {
		value := fmt.Sprintf("%s 成功执行动作 %s，目标=%s/%s，回复=%s", time.Unix(now, 0).Format("2006-01-02 15:04"), action.ActionType, action.TargetType, action.TargetId, truncateAgentText(reply, 240))
		_, _ = AgentUpsertMemory(AgentMemoryUpsertRequest{AgentName: task.AgentName, Scope: "ops", Key: fmt.Sprintf("last-success:%s:%d", strings.ToLower(task.Source), task.Id), Value: value, Source: "task_success", Confidence: 0.8, TaskID: task.Id, TTLSeconds: 30 * 24 * 3600, Extra: map[string]any{"action_id": action.Id, "action_type": action.ActionType, "source": task.Source}})
	}
	if strings.TrimSpace(task.RoomId) != "" {
		value := fmt.Sprintf("管理员 %s 在 %s 群/房间 %s 完成任务：%s", firstAgentNonEmpty(task.IssuerUsername, task.IssuerExternalId), task.Source, task.RoomId, truncateAgentText(command, 240))
		_, _ = AgentUpsertMemory(AgentMemoryUpsertRequest{AgentName: task.AgentName, Scope: "group", Key: fmt.Sprintf("group:%s:%s:last-admin-task", strings.ToLower(task.Source), task.RoomId), Value: value, Source: "group_ops", Confidence: 0.75, TaskID: task.Id, TTLSeconds: 14 * 24 * 3600, Extra: map[string]any{"source": task.Source, "room_id": task.RoomId}})
	}
}

type agentChatMemoryClassification struct {
	Category      string
	Scope         string
	Key           string
	TTLSeconds    int64
	Confidence    float64
	Reason        string
	SaveMemory    bool
	SaveCandidate bool
}

type agentMemoryRuntimePolicy struct {
	AutoCaptureEnabled  bool
	NoiseTTLSeconds     int
	CandidateTTLSeconds int
	CoreTTLSeconds      int
	RiskTTLSeconds      int
	NoiseSampleRate     int
	Source              string
	PolicySource        string
	PolicyRoomID        string
}

func AgentCaptureChatMemory(req AgentChatOpsRequest, taskID int) {
	cfg := operation_setting.GetAgentSetting()
	if cfg == nil {
		return
	}
	policy := resolveAgentMemoryRuntimePolicy(req, cfg)
	if !policy.AutoCaptureEnabled {
		return
	}
	text := strings.TrimSpace(req.Text)
	if text == "" || agentMemoryLooksSecret(text) {
		return
	}
	classification := classifyAgentChatMemory(req, text, policy)
	if classification.Category == "" {
		return
	}
	now := time.Now().Unix()
	var memoryID int
	var candidateID int
	if classification.SaveMemory {
		row, err := AgentUpsertMemory(AgentMemoryUpsertRequest{
			AgentName:  agentMemoryDefaultAgent,
			Scope:      classification.Scope,
			Key:        classification.Key,
			Value:      text,
			Source:     "chat_auto_capture",
			Confidence: classification.Confidence,
			TaskID:     taskID,
			TTLSeconds: classification.TTLSeconds,
			Extra: map[string]any{
				"category":         classification.Category,
				"reason":           classification.Reason,
				"source":           req.Source,
				"room_id":          req.RoomID,
				"message_id":       req.MessageID,
				"user_external_id": req.UserExternalID,
				"username":         req.Username,
				"is_admin":         req.IsAdmin,
				"policy_source":    policy.PolicySource,
				"policy_room_id":   policy.PolicyRoomID,
			},
		})
		if err == nil && row != nil {
			memoryID = row.Id
		}
	}
	if classification.SaveCandidate {
		expiresAt := int64(0)
		if classification.TTLSeconds > 0 {
			expiresAt = now + classification.TTLSeconds
		}
		candidate := &model.AgentMemoryCandidate{
			SiteId:         AgentSiteID(),
			AgentName:      agentMemoryDefaultAgent,
			Source:         req.Source,
			RoomId:         req.RoomID,
			UserExternalId: req.UserExternalID,
			Username:       req.Username,
			Category:       classification.Category,
			Scope:          classification.Scope,
			MemoryKey:      classification.Key,
			Text:           truncateAgentText(text, agentMemoryMaxValueLen),
			Reason:         classification.Reason,
			Confidence:     classification.Confidence,
			Status:         agentMemoryCandidateStatus(classification),
			PayloadJson:    mustAgentJSON(map[string]any{"task_id": taskID, "message_id": req.MessageID, "is_admin": req.IsAdmin, "raw_source": req.Source, "policy_source": policy.PolicySource, "policy_room_id": policy.PolicyRoomID}),
			ExpiresAt:      expiresAt,
		}
		if err := model.DB.Create(candidate).Error; err == nil {
			candidateID = candidate.Id
		}
	}
	_ = model.DB.Create(&model.AgentMemoryEvent{
		SiteId:         AgentSiteID(),
		AgentName:      agentMemoryDefaultAgent,
		EventType:      "chat.memory." + classification.Category,
		Category:       classification.Category,
		Source:         req.Source,
		RoomId:         req.RoomID,
		UserExternalId: req.UserExternalID,
		MemoryId:       memoryID,
		CandidateId:    candidateID,
		Reason:         classification.Reason,
		PayloadJson:    mustAgentJSON(map[string]any{"task_id": taskID, "message_id": req.MessageID, "saved_memory": memoryID > 0, "saved_candidate": candidateID > 0, "policy_source": policy.PolicySource, "policy_room_id": policy.PolicyRoomID}),
	}).Error
}

func resolveAgentMemoryRuntimePolicy(req AgentChatOpsRequest, cfg *operation_setting.AgentSetting) agentMemoryRuntimePolicy {
	policy := agentMemoryRuntimePolicy{
		AutoCaptureEnabled:  cfg != nil && cfg.MemoryAutoCaptureEnabled,
		NoiseTTLSeconds:     86400,
		CandidateTTLSeconds: 7 * 86400,
		CoreTTLSeconds:      0,
		RiskTTLSeconds:      90 * 86400,
		NoiseSampleRate:     10,
		Source:              "agent_setting",
		PolicySource:        "agent_setting.global",
	}
	if cfg != nil {
		policy.NoiseTTLSeconds = firstPositiveInt(cfg.MemoryNoiseTTLSeconds, policy.NoiseTTLSeconds)
		policy.CandidateTTLSeconds = firstPositiveInt(cfg.MemoryCandidateTTLSeconds, policy.CandidateTTLSeconds)
		policy.CoreTTLSeconds = cfg.MemoryCoreTTLSeconds
		policy.RiskTTLSeconds = firstPositiveInt(cfg.MemoryRiskTTLSeconds, policy.RiskTTLSeconds)
		policy.NoiseSampleRate = cfg.MemoryNoiseSampleRate
	}
	if model.DB == nil {
		return policy
	}

	siteID := AgentSiteID()
	source := strings.ToLower(strings.TrimSpace(req.Source))
	roomID := strings.TrimSpace(req.RoomID)
	candidates := [][2]string{
		{source, roomID},
		{source, ""},
		{"", ""},
	}
	seen := map[string]bool{}
	for _, candidate := range candidates {
		key := candidate[0] + "\x00" + candidate[1]
		if seen[key] {
			continue
		}
		seen[key] = true
		var row model.AgentMemoryPolicy
		err := model.DB.Where("site_id = ? AND source = ? AND room_id = ?", siteID, candidate[0], candidate[1]).First(&row).Error
		if err != nil {
			continue
		}
		policy.AutoCaptureEnabled = row.AutoCaptureEnabled
		policy.NoiseTTLSeconds = firstPositiveInt(row.NoiseTTLSeconds, policy.NoiseTTLSeconds)
		policy.CandidateTTLSeconds = firstPositiveInt(row.CandidateTTLSeconds, policy.CandidateTTLSeconds)
		policy.CoreTTLSeconds = row.CoreTTLSeconds
		policy.RiskTTLSeconds = firstPositiveInt(row.RiskTTLSeconds, policy.RiskTTLSeconds)
		policy.NoiseSampleRate = row.NoiseSampleRate
		policy.Source = "agent_memory_policies"
		switch {
		case candidate[0] != "" && candidate[1] != "":
			policy.PolicySource = "agent_memory_policies.source_room"
		case candidate[0] != "":
			policy.PolicySource = "agent_memory_policies.source_default"
		default:
			policy.PolicySource = "agent_memory_policies.global"
		}
		policy.PolicyRoomID = candidate[1]
		return policy
	}
	return policy
}

func classifyAgentChatMemory(req AgentChatOpsRequest, text string, policy agentMemoryRuntimePolicy) agentChatMemoryClassification {
	lower := strings.ToLower(strings.TrimSpace(text))
	source := strings.ToLower(strings.TrimSpace(req.Source))
	roomID := strings.TrimSpace(req.RoomID)
	userID := strings.TrimSpace(req.UserExternalID)
	scope := "group"
	if roomID == "" {
		scope = "user"
	}
	keyPrefix := "group:" + source + ":" + roomID
	if scope == "user" {
		keyPrefix = "user:" + source + ":" + userID
	}
	coreTTL := int64(policy.CoreTTLSeconds)
	riskTTL := int64(firstPositiveInt(policy.RiskTTLSeconds, 90*86400))
	candidateTTL := int64(firstPositiveInt(policy.CandidateTTLSeconds, 7*86400))
	noiseTTL := int64(firstPositiveInt(policy.NoiseTTLSeconds, 86400))
	if chatMemoryIsAdminPolicy(req, lower) {
		policyScope := "policy"
		if roomID != "" && hasAny(lower, "本群", "这个群", "群里", "房间") {
			policyScope = "group"
		}
		return agentChatMemoryClassification{
			Category:      "core",
			Scope:         policyScope,
			Key:           keyPrefix + ":policy:" + shortAgentMemoryHash(text),
			TTLSeconds:    coreTTL,
			Confidence:    0.96,
			Reason:        "管理员给出长期规则/运营口径",
			SaveMemory:    true,
			SaveCandidate: true,
		}
	}
	if hasAny(lower, "广告", "引流", "诈骗", "私聊交易", "卖号", "换u", "违规", "刷屏", "辱骂", "黑产", "羊毛", "薅") {
		return agentChatMemoryClassification{
			Category:      "risk",
			Scope:         "risk",
			Key:           keyPrefix + ":risk:" + shortAgentMemoryHash(text),
			TTLSeconds:    riskTTL,
			Confidence:    0.86,
			Reason:        "群聊出现风控/违规线索",
			SaveMemory:    true,
			SaveCandidate: true,
		}
	}
	if chatMemoryLooksNoise(lower) {
		if !sampleAgentNoiseMemory(text, policy.NoiseSampleRate) {
			return agentChatMemoryClassification{Category: "noise", Scope: scope, Key: keyPrefix + ":noise:" + shortAgentMemoryHash(text), TTLSeconds: noiseTTL, Confidence: 0.2, Reason: "低价值水聊，未命中抽样", SaveMemory: false, SaveCandidate: false}
		}
		return agentChatMemoryClassification{
			Category:      "noise",
			Scope:         scope,
			Key:           keyPrefix + ":noise:" + shortAgentMemoryHash(text),
			TTLSeconds:    noiseTTL,
			Confidence:    0.25,
			Reason:        "低价值水聊抽样短期保留",
			SaveMemory:    true,
			SaveCandidate: true,
		}
	}
	if chatMemoryLooksUseful(lower) {
		return agentChatMemoryClassification{
			Category:      "candidate",
			Scope:         scope,
			Key:           keyPrefix + ":candidate:" + shortAgentMemoryHash(text),
			TTLSeconds:    candidateTTL,
			Confidence:    0.62,
			Reason:        "可能有运营/使用答疑价值，先进入候选记忆",
			SaveMemory:    false,
			SaveCandidate: true,
		}
	}
	return agentChatMemoryClassification{}
}

func chatMemoryIsAdminPolicy(req AgentChatOpsRequest, lower string) bool {
	role := strings.ToLower(strings.TrimSpace(req.UserRole))
	isAdmin := req.IsAdmin || role == "admin" || role == "owner" || role == "administrator" || role == "creator"
	if !isAdmin {
		return false
	}
	return hasAny(lower, "记住", "以后", "规则", "统一", "口径", "必须", "默认", "不要再", "遇到", "一律", "policy", "rule")
}

func chatMemoryLooksNoise(lower string) bool {
	s := strings.TrimSpace(lower)
	if s == "" {
		return true
	}
	if len([]rune(s)) <= 4 {
		return true
	}
	return hasAny(s, "哈哈", "hhh", "233", "666", "水一下", "路过", "签到", "打卡") && len([]rune(s)) <= 20
}

func chatMemoryLooksUseful(lower string) bool {
	if len([]rune(lower)) >= 80 {
		return true
	}
	return hasAny(lower, "怎么", "为什么", "多少钱", "价格", "额度", "充值", "模型", "绑定", "签到", "验牌", "邀请", "规则", "报错", "失败", "教程", "入口", "链接", "公告", "活动")
}

func sampleAgentNoiseMemory(text string, rate int) bool {
	if rate <= 0 {
		return false
	}
	if rate >= 100 {
		return true
	}
	sum := sha1.Sum([]byte(strings.TrimSpace(text)))
	return int(sum[0])%100 < rate
}

func agentMemoryCandidateStatus(c agentChatMemoryClassification) string {
	if c.Category == "core" || c.Category == "risk" {
		return "promoted"
	}
	if c.Category == "noise" {
		return "ephemeral"
	}
	return "open"
}

func firstPositiveInt(values ...int) int {
	for _, v := range values {
		if v > 0 {
			return v
		}
	}
	return 0
}

func isAgentMemoryCommand(command string) bool {
	cmd := strings.TrimSpace(command)
	lower := strings.ToLower(cmd)
	return strings.HasPrefix(lower, "memory") || strings.HasPrefix(lower, "learn") || strings.HasPrefix(cmd, "记忆") || strings.HasPrefix(cmd, "记住") || strings.HasPrefix(cmd, "学习") || strings.HasPrefix(cmd, "忘记") || strings.HasPrefix(cmd, "删除记忆")
}

func agentMemoryContextItems(req AgentChatOpsRequest, limit int) []AgentMemoryContextItem {
	if limit <= 0 {
		limit = 12
	}
	now := time.Now().Unix()
	var rows []model.AgentMemory
	_ = model.DB.Where("site_id = ? AND agent_name = ? AND (expires_at = 0 OR expires_at > ?) AND scope IN ?", AgentSiteID(), agentMemoryDefaultAgent, now, []string{"site", "policy", "ops", "group", "user", "risk", "game"}).Order("updated_at desc, id desc").Limit(80).Find(&rows).Error
	items := make([]AgentMemoryContextItem, 0, limit)
	groupPrefix := "group:" + strings.ToLower(strings.TrimSpace(req.Source)) + ":" + strings.TrimSpace(req.RoomID)
	userPrefix := "user:" + strings.ToLower(strings.TrimSpace(req.Source)) + ":" + strings.TrimSpace(req.UserExternalID)
	for _, row := range rows {
		if row.Scope == "group" && groupPrefix != "group::" && !strings.HasPrefix(row.MemoryKey, groupPrefix) {
			continue
		}
		if row.Scope == "user" && userPrefix != "user::" && !strings.HasPrefix(row.MemoryKey, userPrefix) {
			continue
		}
		if row.Scope == "risk" && strings.HasPrefix(row.MemoryKey, "group:") && groupPrefix != "group::" && !strings.HasPrefix(row.MemoryKey, groupPrefix) {
			continue
		}
		payload := decodeAgentMemoryPayload(row)
		items = append(items, AgentMemoryContextItem{Scope: row.Scope, Key: row.MemoryKey, Value: truncateAgentText(payload.Value, 500), Source: payload.Source, Confidence: payload.Confidence, UpdatedAt: row.UpdatedAt})
		if len(items) >= limit {
			break
		}
	}
	return items
}

func formatAgentMemorySummary(items []AgentMemoryContextItem) string {
	if len(items) == 0 {
		return "暂无可用长期记忆；仅使用当前请求、站点状态和后台配置判断。"
	}
	parts := make([]string, 0, len(items))
	for _, item := range items {
		parts = append(parts, fmt.Sprintf("[%s/%s c=%.2f] %s", item.Scope, item.Key, item.Confidence, item.Value))
	}
	return strings.Join(parts, "\n")
}

func formatAgentMemoryList(rows []model.AgentMemory) string {
	if len(rows) == 0 {
		return "当前站点还没有可展示的长期记忆。"
	}
	lines := []string{fmt.Sprintf("当前站点长期记忆共显示 %d 条：", len(rows))}
	for _, row := range rows {
		payload := decodeAgentMemoryPayload(row)
		lines = append(lines, fmt.Sprintf("- %s/%s c=%.2f source=%s：%s", row.Scope, row.MemoryKey, payload.Confidence, payload.Source, truncateAgentText(payload.Value, 180)))
	}
	return strings.Join(lines, "\n")
}

func decodeAgentMemoryPayload(row model.AgentMemory) AgentMemoryPayload {
	payload := AgentMemoryPayload{}
	if err := json.Unmarshal([]byte(row.MemoryValue), &payload); err != nil || strings.TrimSpace(payload.Value) == "" {
		payload.Value = strings.TrimSpace(row.MemoryValue)
	}
	if payload.Confidence <= 0 {
		payload.Confidence = 0.5
	}
	if payload.Source == "" {
		payload.Source = "legacy"
	}
	return payload
}

func normalizeAgentMemoryScope(scope string) (string, error) {
	scope = strings.ToLower(strings.TrimSpace(scope))
	scope = strings.Trim(scope, " ：:=，,。")
	aliases := map[string]string{"站点": "site", "用户": "user", "群": "group", "群组": "group", "房间": "group", "运维": "ops", "运营": "ops", "风控": "risk", "游戏": "game", "策略": "policy", "规则": "policy"}
	if aliases[scope] != "" {
		scope = aliases[scope]
	}
	if scope == "" {
		scope = "site"
	}
	if !agentMemoryScopes[scope] {
		return "", fmt.Errorf("unsupported memory scope: %s", scope)
	}
	return scope, nil
}

func normalizeAgentMemoryKey(key string) string {
	key = strings.ToLower(strings.TrimSpace(key))
	key = strings.Trim(key, " ：:=，,。")
	key = regexp.MustCompile(`[^a-z0-9_.:\-]+`).ReplaceAllString(key, "-")
	key = strings.Trim(key, "-")
	return truncateAgentText(key, 120)
}

func inferAgentMemoryScope(value string) string {
	lower := strings.ToLower(value)
	switch {
	case hasAny(lower, "风控", "risk", "注册", "滥用", "spam"):
		return "risk"
	case hasAny(lower, "游戏", "签到", "抽奖", "game"):
		return "game"
	case hasAny(lower, "规则", "策略", "倍率", "定价", "policy"):
		return "policy"
	case hasAny(lower, "运维", "部署", "日志", "服务", "重启", "ops"):
		return "ops"
	default:
		return "site"
	}
}

func inferScopeWord(command string) string {
	lower := strings.ToLower(command)
	scopes := []string{"site", "policy", "ops", "risk", "game", "group", "user", "站点", "规则", "策略", "运维", "风控", "游戏", "群组", "用户"}
	for _, scope := range scopes {
		if strings.Contains(lower, strings.ToLower(scope)) {
			if normalized, err := normalizeAgentMemoryScope(scope); err == nil {
				return normalized
			}
		}
	}
	return ""
}

func agentMemoryCommandField(command string, keys ...string) string {
	for _, key := range keys {
		patterns := []string{
			`(?i)` + regexp.QuoteMeta(key) + `\s*[:=：]\s*([^\s，,]+)`,
			`(?i)` + regexp.QuoteMeta(key) + `\s+([^\s，,]+)`,
		}
		for _, pattern := range patterns {
			re := regexp.MustCompile(pattern)
			if m := re.FindStringSubmatch(command); len(m) >= 2 {
				return strings.TrimSpace(m[1])
			}
		}
	}
	return ""
}

func agentMemoryCommandTextValue(command string) string {
	for _, key := range []string{"value", "内容", "text"} {
		re := regexp.MustCompile(`(?is)` + regexp.QuoteMeta(key) + `\s*[:=：]\s*(.+)$`)
		if m := re.FindStringSubmatch(command); len(m) >= 2 {
			return strings.TrimSpace(m[1])
		}
	}
	return ""
}

func trimAgentMemorySaveHead(command string) string {
	out := strings.TrimSpace(command)
	for _, head := range []string{"memory save", "memory", "learn", "记忆 保存", "记忆", "记住", "学习"} {
		if strings.HasPrefix(strings.ToLower(out), strings.ToLower(head)) {
			out = strings.TrimSpace(out[len(head):])
			break
		}
	}
	out = regexp.MustCompile(`(?i)(scope|key|范围|键)\s*[:=：]\s*[^\s，,]+`).ReplaceAllString(out, "")
	return strings.TrimSpace(out)
}

func lastAgentMemoryWord(command string) string {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}

func shortAgentMemoryHash(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return strconv.FormatInt(time.Now().UnixNano(), 36)
	}
	sum := sha1.Sum([]byte(value))
	return hex.EncodeToString(sum[:])[:12]
}

func agentMemoryLooksSecret(value string) bool {
	lower := strings.ToLower(value)
	secretWords := []string{"sk-", "token", "secret", "password", "passwd", "cookie", "authorization", "bearer ", "api_key", "apikey", "密钥", "密码", "令牌", "私钥"}
	for _, word := range secretWords {
		if strings.Contains(lower, strings.ToLower(word)) {
			return true
		}
	}
	return false
}

func sortAgentMemoryRows(rows []model.AgentMemory) {
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].UpdatedAt == rows[j].UpdatedAt {
			return rows[i].Id > rows[j].Id
		}
		return rows[i].UpdatedAt > rows[j].UpdatedAt
	})
}
