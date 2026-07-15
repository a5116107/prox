package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

type AccessControlDecision struct {
	State      *model.UserSiteAccessState
	Usable     map[string]string
	JoinURL    string
	PrimaryURL string
	DeniedMsg  string
	UpgradeMsg string
	Compliant  bool
}

type accessControlBindingInfo struct {
	PrimaryBound          bool
	MatchedPrimaryGroupID string
}

type accessControlPersistenceContextKey struct{}

func withAccessControlPersistence(ctx context.Context, persist bool) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, accessControlPersistenceContextKey{}, persist)
}

func accessControlPersistenceEnabled(ctx context.Context) bool {
	if ctx == nil {
		return true
	}
	persist, ok := ctx.Value(accessControlPersistenceContextKey{}).(bool)
	return !ok || persist
}

type AccessControlViolation struct {
	Action         string
	Code           string
	Message        string
	State          *model.UserSiteAccessState
	RequestedGroup string
	AllowedGroups  []string
}

func (e *AccessControlViolation) Error() string {
	if e == nil {
		return ""
	}
	return strings.TrimSpace(e.Message)
}

func IsAccessControlViolation(err error) (*AccessControlViolation, bool) {
	var target *AccessControlViolation
	if errors.As(err, &target) {
		return target, true
	}
	return nil, false
}

func accessControlEnabled() bool {
	cfg := operation_setting.GetAccessControlSetting()
	return cfg != nil && cfg.Enabled
}

func accessControlTTL() time.Duration {
	cfg := operation_setting.GetAccessControlSetting()
	if cfg == nil || cfg.StateCacheTTLSeconds <= 0 {
		return time.Duration(operation_setting.GetAccessControlSetting().StateCacheTTLSeconds) * time.Second
	}
	return time.Duration(cfg.StateCacheTTLSeconds) * time.Second
}

func accessControlPrimaryPlatform() string {
	cfg := operation_setting.GetAccessControlSetting()
	if cfg == nil {
		return "qq"
	}
	platform := strings.ToLower(strings.TrimSpace(cfg.PrimaryPlatform))
	if platform == "" {
		return "qq"
	}
	if platform == "telegram" {
		return "tg"
	}
	return platform
}

func normalizeAccessPlatform(platform string) string {
	platform = strings.ToLower(strings.TrimSpace(platform))
	switch platform {
	case "telegram":
		return "tg"
	case "discord", "dc", "hhhl":
		return "community"
	case "":
		return "qq"
	default:
		return platform
	}
}

func accessControlJoinURL(joinURL string, roomID string) string {
	joinURL = strings.TrimRight(strings.TrimSpace(joinURL), "/")
	if strings.Contains(joinURL, "/chat/room/") {
		return joinURL
	}
	roomID = strings.TrimSpace(roomID)
	if joinURL == "" || roomID == "" {
		return ""
	}
	return joinURL + "/chat/room/" + roomID
}

func accessControlStateFresh(row *model.UserSiteAccessState) bool {
	if row == nil {
		return false
	}
	return time.Since(time.Unix(row.LastEvaluatedAt, 0)) < accessControlTTL()
}

func splitAccessControlGroups(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return []string{}
	}
	var parsed []string
	if err := json.Unmarshal([]byte(raw), &parsed); err == nil {
		out := make([]string, 0, len(parsed))
		seen := map[string]struct{}{}
		for _, item := range parsed {
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
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == '\n' || r == '\r' || r == '\t'
	})
	out := make([]string, 0, len(parts))
	seen := map[string]struct{}{}
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if _, ok := seen[part]; ok {
			continue
		}
		seen[part] = struct{}{}
		out = append(out, part)
	}
	return out
}

func accessControlFilterGroups(base map[string]string, candidates []string) map[string]string {
	if len(base) == 0 || len(candidates) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string)
	for _, candidate := range candidates {
		if desc, ok := base[strings.TrimSpace(candidate)]; ok {
			out[strings.TrimSpace(candidate)] = desc
		}
	}
	return out
}

func accessControlOrderedGroups(base map[string]string, candidates []string) []string {
	if len(candidates) > 0 {
		out := make([]string, 0, len(candidates))
		seen := map[string]struct{}{}
		for _, candidate := range candidates {
			candidate = strings.TrimSpace(candidate)
			if candidate == "" {
				continue
			}
			if _, ok := base[candidate]; !ok {
				continue
			}
			if _, ok := seen[candidate]; ok {
				continue
			}
			seen[candidate] = struct{}{}
			out = append(out, candidate)
		}
		return out
	}
	out := make([]string, 0, len(base))
	for group := range base {
		out = append(out, group)
	}
	sort.Strings(out)
	return out
}

func accessControlPrimaryBinding(ctx context.Context, userID int) (bool, string, error) {
	cfg := operation_setting.GetAccessControlSetting()
	if cfg == nil {
		return false, "", nil
	}
	platform := accessControlPrimaryPlatform()
	primaryGroupIDs, _ := opsAccessPolicyResolvedBindingGroups(AgentSiteID(), cfg)
	allowedRooms := make(map[string]struct{}, len(primaryGroupIDs))
	for _, roomID := range primaryGroupIDs {
		roomID = strings.TrimSpace(roomID)
		if roomID != "" {
			allowedRooms[roomID] = struct{}{}
		}
	}
	bindings, err := model.ListAgentChatBindingsByUser(AgentSiteID(), platform, userID)
	if err != nil {
		return false, "", err
	}
	for _, binding := range bindings {
		roomID := strings.TrimSpace(binding.RoomId)
		if roomID == "" {
			continue
		}
		if len(allowedRooms) > 0 {
			if _, ok := allowedRooms[roomID]; !ok {
				continue
			}
		}
		return true, roomID, nil
	}
	return false, "", nil
}

func accessControlCommunityBinding(ctx context.Context, userID int, refresh bool) (*model.UserSiteAccessState, error) {
	cfg := operation_setting.GetCommunityGateSetting()
	if cfg == nil || !cfg.Enabled {
		return &model.UserSiteAccessState{
			CommunityBound:    true,
			HasOAuthBinding:   true,
			HasRoomMembership: true,
		}, nil
	}
	gate, err := EvaluateCommunityGate(ctx, userID, refresh)
	if err != nil {
		return nil, err
	}
	if gate == nil {
		return nil, errors.New("community gate returned empty result")
	}
	return &model.UserSiteAccessState{
		CommunityBound:    gate.Compliant,
		HasOAuthBinding:   gate.HasOAuthBinding,
		HasRoomMembership: gate.HasRoomMembership,
	}, nil
}

func accessControlAllowedGroups(base map[string]string, state *model.UserSiteAccessState, cfg *operation_setting.AccessControlSetting) map[string]string {
	if cfg == nil || state == nil {
		return base
	}
	overrideMode := strings.TrimSpace(state.ManualOverrideMode)
	overrideGroups := splitAccessControlGroups(state.ManualOverrideGroups)
	switch overrideMode {
	case model.AccessOverrideDeny:
		return map[string]string{}
	case model.AccessOverrideCommunity:
		candidates := overrideGroups
		if len(candidates) == 0 {
			candidates = cfg.CommunityOnlyGroups
		}
		return accessControlFilterGroups(base, candidates)
	case model.AccessOverrideCustom:
		return accessControlFilterGroups(base, overrideGroups)
	case model.AccessOverrideFullAccess:
		candidates := overrideGroups
		if len(candidates) == 0 {
			candidates = cfg.FullAccessGroups
		}
		if len(candidates) == 0 {
			return base
		}
		return accessControlFilterGroups(base, candidates)
	}
	switch state.AccessLevel {
	case model.AccessLevelAdminBypass, model.AccessLevelPaidBypass, model.AccessLevelFullAccess:
		candidates := cfg.FullAccessGroups
		if len(candidates) == 0 {
			return base
		}
		return accessControlFilterGroups(base, candidates)
	case model.AccessLevelCommunityOnly:
		candidates := cfg.CommunityOnlyGroups
		if len(candidates) == 0 {
			return map[string]string{}
		}
		return accessControlFilterGroups(base, candidates)
	default:
		return map[string]string{}
	}
}

func accessControlReasonMessage(state *model.UserSiteAccessState, cfg *operation_setting.AccessControlSetting) string {
	if state == nil {
		return accessControlAppendSteps("当前账号尚未完成访问绑定", accessControlNextSteps(nil))
	}
	if strings.TrimSpace(state.ManualOverrideReason) != "" {
		return strings.TrimSpace(state.ManualOverrideReason)
	}
	if state.AccessLevel == model.AccessLevelNone {
		base := "当前账号尚未获得可用访问权限"
		if cfg != nil && strings.TrimSpace(cfg.DenyMessage) != "" {
			base = strings.TrimSpace(cfg.DenyMessage)
		}
		return accessControlAppendSteps(base, accessControlNextSteps(state))
	}
	if state.AccessLevel == model.AccessLevelCommunityOnly {
		base := "当前账号已完成社区授权，仅解锁社区分组"
		if cfg != nil && strings.TrimSpace(cfg.UpgradeMessage) != "" {
			base = strings.TrimSpace(cfg.UpgradeMessage)
		}
		steps := []string{}
		if !state.PrimaryBound {
			steps = append(steps, fmt.Sprintf("绑定 %s 主群以解锁更多分组", accessControlPrimaryPlatformLabel(state.PrimaryPlatform)))
		}
		return accessControlAppendSteps(base, steps)
	}
	return ""
}

func accessControlReasonCode(state *model.UserSiteAccessState) string {
	if state == nil {
		return "unknown"
	}
	if strings.TrimSpace(state.ManualOverrideMode) != "" {
		return "manual_override"
	}
	if state.AccessLevel == model.AccessLevelNone {
		return "not_bound"
	}
	return state.AccessLevel
}

func accessControlActionLabel(action string) string {
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "create":
		return "创建 API Key"
	case "enable":
		return "启用 API Key"
	case "update":
		return "更新 API Key 分组"
	case "request":
		return "使用当前 API Key"
	default:
		return "执行当前操作"
	}
}

func accessControlPrimaryPlatformLabel(platform string) string {
	switch normalizeAccessPlatform(platform) {
	case "tg":
		return "Telegram"
	case "community":
		return "社区"
	default:
		return "QQ"
	}
}

func accessControlNextSteps(state *model.UserSiteAccessState) []string {
	steps := make([]string, 0, 3)
	addStep := func(step string) {
		step = strings.TrimSpace(step)
		if step == "" {
			return
		}
		for _, item := range steps {
			if item == step {
				return
			}
		}
		steps = append(steps, step)
	}
	platformLabel := accessControlPrimaryPlatformLabel(accessControlPrimaryPlatform())
	if state != nil && strings.TrimSpace(state.PrimaryPlatform) != "" {
		platformLabel = accessControlPrimaryPlatformLabel(state.PrimaryPlatform)
	}
	if state == nil || !state.HasOAuthBinding {
		addStep("先通过 HHHL 社区授权登录")
	}
	if state == nil || !state.HasRoomMembership {
		addStep("加入当前站点要求的社区群")
	}
	if state == nil || !state.PrimaryBound {
		addStep(fmt.Sprintf("绑定 %s 主群", platformLabel))
	}
	return steps
}

func BuildAccessControlNextSteps(state *model.UserSiteAccessState) []string {
	return accessControlNextSteps(state)
}

func accessControlAllowedGroupsSummary(allowed []string) string {
	if len(allowed) == 0 {
		return "暂无"
	}
	ordered := make([]string, 0, len(allowed))
	seen := map[string]struct{}{}
	for _, group := range allowed {
		group = strings.TrimSpace(group)
		if group == "" {
			continue
		}
		if _, ok := seen[group]; ok {
			continue
		}
		seen[group] = struct{}{}
		ordered = append(ordered, group)
	}
	sort.Strings(ordered)
	if len(ordered) == 0 {
		return "暂无"
	}
	if len(ordered) > 6 {
		return strings.Join(ordered[:6], "、") + " 等"
	}
	return strings.Join(ordered, "、")
}

func accessControlAppendSteps(base string, steps []string) string {
	base = strings.TrimSpace(base)
	filtered := make([]string, 0, len(steps))
	for _, step := range steps {
		step = strings.TrimSpace(step)
		if step != "" {
			filtered = append(filtered, step)
		}
	}
	if len(filtered) == 0 {
		return strings.TrimRight(base, "；。 ")
	}
	stepText := strings.Join(filtered, "；")
	if base == "" {
		return fmt.Sprintf("当前还需：%s。", stepText)
	}
	base = strings.TrimRight(base, "；。 ")
	return fmt.Sprintf("%s。当前还需：%s。", base, stepText)
}

func accessControlSignificantStateChanged(prev *model.UserSiteAccessState, next *model.UserSiteAccessState) bool {
	if next == nil {
		return false
	}
	if prev == nil {
		return true
	}
	return prev.AccessLevel != next.AccessLevel ||
		prev.CommunityBound != next.CommunityBound ||
		prev.HasOAuthBinding != next.HasOAuthBinding ||
		prev.HasRoomMembership != next.HasRoomMembership ||
		prev.PrimaryBound != next.PrimaryBound ||
		strings.TrimSpace(prev.PrimaryPlatform) != strings.TrimSpace(next.PrimaryPlatform) ||
		strings.TrimSpace(prev.MatchedPrimaryGroupId) != strings.TrimSpace(next.MatchedPrimaryGroupId) ||
		strings.TrimSpace(prev.ManualOverrideMode) != strings.TrimSpace(next.ManualOverrideMode) ||
		strings.TrimSpace(prev.ManualOverrideGroups) != strings.TrimSpace(next.ManualOverrideGroups) ||
		strings.TrimSpace(prev.ManualOverrideReason) != strings.TrimSpace(next.ManualOverrideReason) ||
		strings.TrimSpace(prev.EffectiveGroups) != strings.TrimSpace(next.EffectiveGroups) ||
		strings.TrimSpace(prev.ReasonCode) != strings.TrimSpace(next.ReasonCode) ||
		strings.TrimSpace(prev.ReasonMessage) != strings.TrimSpace(next.ReasonMessage)
}

func recordAccessControlLevelChange(user *model.User, prev *model.UserSiteAccessState, next *model.UserSiteAccessState) {
	if user == nil || next == nil || !accessControlSignificantStateChanged(prev, next) {
		return
	}
	previousAccessLevel := model.AccessLevelNone
	previousReasonCode := "unknown"
	previousGroups := []string{}
	if prev != nil {
		previousAccessLevel = firstNonEmptyAccessState(prev.AccessLevel, model.AccessLevelNone)
		previousReasonCode = firstNonEmptyAccessState(prev.ReasonCode, "unknown")
		previousGroups = prev.EffectiveGroupList()
	}
	other := map[string]any{
		"access_control": map[string]any{
			"site_id":                   next.SiteId,
			"previous_access_level":     previousAccessLevel,
			"current_access_level":      next.AccessLevel,
			"previous_reason_code":      previousReasonCode,
			"current_reason_code":       next.ReasonCode,
			"previous_effective_groups": previousGroups,
			"current_effective_groups":  next.EffectiveGroupList(),
			"primary_bound":             next.PrimaryBound,
			"community_bound":           next.CommunityBound,
			"matched_primary_group_id":  next.MatchedPrimaryGroupId,
		},
	}
	model.RecordLogEvent(user.Id, model.LogTypeSystem, fmt.Sprintf("访问权限状态已更新：%s → %s", previousAccessLevel, next.AccessLevel), model.LogEventOptions{
		Username: user.Username,
		SiteId:   next.SiteId,
		Category: "access_control",
		Source:   "system",
		Action:   "level_changed",
		Status:   "success",
		Other:    other,
	})
}

func accessControlHasPaidBypass(user *model.User, cfg *operation_setting.AccessControlSetting) bool {
	if user == nil || cfg == nil || !cfg.AllowPaidBypass {
		return false
	}
	userGroup := strings.TrimSpace(user.Group)
	if userGroup == "" {
		return false
	}
	candidates := cfg.PaidUserGroups
	if len(candidates) == 0 {
		candidates = cfg.PaidBypassGroups
	}
	for _, groupName := range candidates {
		groupName = strings.TrimSpace(groupName)
		if groupName != "" && strings.EqualFold(groupName, userGroup) {
			return true
		}
	}
	return false
}

func accessControlMutationEnforced(cfg *operation_setting.AccessControlSetting, action string) bool {
	if cfg == nil || !cfg.Enabled {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "create":
		return cfg.BlockTokenCreate
	case "enable":
		return cfg.BlockTokenEnable
	default:
		return true
	}
}

func accessControlViolationMessage(action string, requestedGroup string, state *model.UserSiteAccessState, allowed []string, cfg *operation_setting.AccessControlSetting) string {
	action = strings.ToLower(strings.TrimSpace(action))
	requestedGroup = strings.TrimSpace(requestedGroup)
	verb := accessControlActionLabel(action)
	allowedText := accessControlAllowedGroupsSummary(allowed)
	if state == nil || state.AccessLevel == model.AccessLevelNone {
		return fmt.Sprintf("%s，本次%s已拒绝。", strings.TrimRight(accessControlReasonMessage(state, cfg), "。"), verb)
	}
	if state.AccessLevel == model.AccessLevelCommunityOnly {
		if requestedGroup != "" {
			return fmt.Sprintf("%s。当前仅开放社区分组，请求分组：%s；当前可用分组：%s。", strings.TrimRight(accessControlReasonMessage(state, cfg), "。"), requestedGroup, allowedText)
		}
		return fmt.Sprintf("%s。当前可用分组：%s。", strings.TrimRight(accessControlReasonMessage(state, cfg), "。"), allowedText)
	}
	if requestedGroup != "" {
		return fmt.Sprintf("当前账号无权访问 %s 分组；当前可用分组：%s。", requestedGroup, allowedText)
	}
	return fmt.Sprintf("%s失败：当前账号暂无可用分组。", verb)
}

func newAccessControlViolation(action string, code string, requestedGroup string, state *model.UserSiteAccessState, allowed map[string]string, cfg *operation_setting.AccessControlSetting) *AccessControlViolation {
	allowedList := accessControlOrderedGroups(allowed, nil)
	message := accessControlViolationMessage(action, requestedGroup, state, allowedList, cfg)
	if strings.TrimSpace(code) == "empty_group" {
		message = "Key 未配置有效分组，已拒绝启用；请先重新完成社区认证并选择获准分组。"
	}
	return &AccessControlViolation{
		Action:         strings.ToLower(strings.TrimSpace(action)),
		Code:           strings.TrimSpace(code),
		Message:        message,
		State:          state,
		RequestedGroup: strings.TrimSpace(requestedGroup),
		AllowedGroups:  allowedList,
	}
}

func BuildAccessControlLogMessage(action string, requestedGroup string, state *model.UserSiteAccessState, allowed []string, denied bool) string {
	cfg := operation_setting.GetAccessControlSetting()
	verb := accessControlActionLabel(action)
	requestedGroup = strings.TrimSpace(requestedGroup)
	allowedSummary := accessControlAllowedGroupsSummary(allowed)
	if denied {
		return fmt.Sprintf("访问控制拒绝%s：%s", verb, accessControlViolationMessage(action, requestedGroup, state, allowed, cfg))
	}
	if requestedGroup != "" {
		switch {
		case state != nil && state.AccessLevel == model.AccessLevelCommunityOnly:
			return fmt.Sprintf("访问控制允许%s：当前账号为社区权限，已通过 %s 分组校验；当前可用分组：%s。", verb, requestedGroup, allowedSummary)
		case len(allowed) > 0:
			return fmt.Sprintf("访问控制允许%s：已通过 %s 分组校验；当前可用分组：%s。", verb, requestedGroup, allowedSummary)
		default:
			return fmt.Sprintf("访问控制允许%s：已通过 %s 分组校验。", verb, requestedGroup)
		}
	}
	if len(allowed) > 0 {
		return fmt.Sprintf("访问控制允许%s：当前可用分组 %s。", verb, allowedSummary)
	}
	return fmt.Sprintf("访问控制允许%s。", verb)
}

func EvaluateUserAccessControl(ctx context.Context, userID int, refresh bool) (*model.UserSiteAccessState, error) {
	return evaluateUserAccessControl(ctx, userID, refresh, true)
}

func evaluateUserAccessControl(ctx context.Context, userID int, refresh bool, persist bool) (*model.UserSiteAccessState, error) {
	ctx = withAccessControlPersistence(ctx, persist)
	cfg := operation_setting.GetAccessControlSetting()
	siteID := AgentSiteID()
	now := time.Now().Unix()
	if cfg == nil || !cfg.Enabled {
		row := &model.UserSiteAccessState{
			SiteId:          siteID,
			UserId:          userID,
			AccessLevel:     model.AccessLevelFullAccess,
			EffectiveGroups: "[]",
			ReasonCode:      "disabled",
			ReasonMessage:   "access control disabled",
			LastEvaluatedAt: now,
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		if persist {
			_ = model.UpsertUserSiteAccessState(row)
		}
		return row, nil
	}
	user, err := model.GetUserById(userID, false)
	if err != nil {
		return nil, err
	}
	if !refresh {
		if cached, err := model.GetUserSiteAccessState(siteID, userID); err == nil && accessControlStateFresh(cached) {
			return cached, nil
		}
	}
	var existing *model.UserSiteAccessState
	if cached, err := model.GetUserSiteAccessState(siteID, userID); err == nil {
		existing = cached
	}
	state := &model.UserSiteAccessState{
		SiteId:               siteID,
		UserId:               userID,
		PrimaryPlatform:      accessControlPrimaryPlatform(),
		ManualOverrideMode:   "",
		ManualOverrideGroups: "[]",
		ReasonCode:           "not_bound",
		ReasonMessage:        "",
		LastEvaluatedAt:      now,
		CreatedAt:            now,
		UpdatedAt:            now,
	}
	if existing != nil {
		state.ManualOverrideMode = strings.TrimSpace(existing.ManualOverrideMode)
		if strings.TrimSpace(existing.ManualOverrideGroups) != "" {
			state.ManualOverrideGroups = existing.ManualOverrideGroups
		}
		state.ManualOverrideReason = strings.TrimSpace(existing.ManualOverrideReason)
	}
	if cfg.AllowAdminBypass && user.Role >= common.RoleAdminUser {
		state.AccessLevel = model.AccessLevelAdminBypass
		state.PrimaryBound = true
		state.ReasonCode = "admin_bypass"
		state.ReasonMessage = "administrator bypass"
	} else if state.ManualOverrideMode != "" {
		switch state.ManualOverrideMode {
		case model.AccessOverrideFullAccess:
			state.AccessLevel = model.AccessLevelManual
			state.ReasonCode = "manual_full_access"
		case model.AccessOverrideCommunity:
			state.AccessLevel = model.AccessLevelManual
			state.ReasonCode = "manual_community_only"
		case model.AccessOverrideCustom:
			state.AccessLevel = model.AccessLevelManual
			state.ReasonCode = "manual_custom_groups"
		case model.AccessOverrideDeny:
			state.AccessLevel = model.AccessLevelNone
			state.ReasonCode = "manual_deny"
		default:
			state.AccessLevel = model.AccessLevelManual
			state.ReasonCode = "manual_override"
		}
		state.ReasonMessage = accessControlReasonMessage(state, cfg)
	} else {
		if accessControlHasPaidBypass(user, cfg) {
			state.AccessLevel = model.AccessLevelPaidBypass
			state.ReasonCode = "paid_bypass"
			state.ReasonMessage = "paid user bypass"
		}
		if state.AccessLevel == "" {
			primaryBound, matchedGroupID, bindErr := accessControlPrimaryBinding(ctx, userID)
			if bindErr != nil {
				return nil, bindErr
			}
			communityState, communityErr := accessControlCommunityBinding(ctx, userID, refresh)
			if communityErr != nil {
				return nil, communityErr
			}
			state.PrimaryBound = primaryBound
			state.MatchedPrimaryGroupId = matchedGroupID
			state.CommunityBound = communityState.CommunityBound
			state.HasOAuthBinding = communityState.HasOAuthBinding
			state.HasRoomMembership = communityState.HasRoomMembership
			switch {
			case primaryBound:
				state.AccessLevel = model.AccessLevelFullAccess
				state.ReasonCode = "primary_bound"
				state.ReasonMessage = "primary group bound"
			case state.CommunityBound:
				state.AccessLevel = model.AccessLevelCommunityOnly
				state.ReasonCode = "community_bound"
				state.ReasonMessage = "community only"
			default:
				state.AccessLevel = model.AccessLevelNone
				state.ReasonCode = "not_bound"
				state.ReasonMessage = accessControlReasonMessage(state, cfg)
			}
		}
	}
	baseGroups := GetUserUsableGroups(user.Group)
	allowedGroups := accessControlAllowedGroups(baseGroups, state, cfg)
	if state.AccessLevel == model.AccessLevelPaidBypass || state.AccessLevel == model.AccessLevelAdminBypass || state.AccessLevel == model.AccessLevelFullAccess {
		if len(cfg.FullAccessGroups) == 0 && len(allowedGroups) == 0 {
			allowedGroups = baseGroups
		}
	}
	if state.AccessLevel == model.AccessLevelCommunityOnly && len(allowedGroups) == 0 && len(cfg.CommunityOnlyGroups) == 0 {
		allowedGroups = map[string]string{}
	}
	effectiveList := make([]string, 0, len(allowedGroups))
	for groupName := range allowedGroups {
		effectiveList = append(effectiveList, groupName)
	}
	sort.Strings(effectiveList)
	state.EffectiveGroups = marshalAccessControlGroups(effectiveList)
	if state.ReasonMessage == "" {
		state.ReasonMessage = accessControlReasonMessage(state, cfg)
	}
	if state.ReasonCode == "" {
		state.ReasonCode = accessControlReasonCode(state)
	}
	state.UpdatedAt = now
	if state.CreatedAt == 0 {
		state.CreatedAt = now
	}
	if persist {
		if err := model.UpsertUserSiteAccessState(state); err != nil {
			return nil, err
		}
		recordAccessControlLevelChange(user, existing, state)
	}
	return state, nil
}

func marshalAccessControlGroups(groups []string) string {
	if len(groups) == 0 {
		return "[]"
	}
	uniq := make([]string, 0, len(groups))
	seen := map[string]struct{}{}
	for _, group := range groups {
		group = strings.TrimSpace(group)
		if group == "" {
			continue
		}
		if _, ok := seen[group]; ok {
			continue
		}
		seen[group] = struct{}{}
		uniq = append(uniq, group)
	}
	data, _ := json.Marshal(uniq)
	return string(data)
}

func GetUserAccessControlStatus(ctx context.Context, userID int, refresh bool) (map[string]any, error) {
	state, err := EvaluateUserAccessControl(ctx, userID, refresh)
	if err != nil {
		return nil, err
	}
	cfg := operation_setting.GetAccessControlSetting()
	communityJoin := ""
	primaryJoin := ""
	if cfg != nil {
		firstCommunityRoomID := ""
		_, communityGroupIDs := opsAccessPolicyResolvedBindingGroups(AgentSiteID(), cfg)
		if len(communityGroupIDs) > 0 {
			firstCommunityRoomID = strings.TrimSpace(communityGroupIDs[0])
		}
		communityJoin = accessControlJoinURL(cfg.CommunityJoinURL, firstCommunityRoomID)
		primaryJoin = strings.TrimRight(strings.TrimSpace(cfg.PrimaryJoinURL), "/")
	}
	activeFrozen, _ := model.CountActiveAccessControlFreezes(userID)
	allowedGroups := state.EffectiveGroupList()
	return map[string]any{
		"state":                    state,
		"compliant":                state != nil && state.AccessLevel != model.AccessLevelNone,
		"access_level":             state.AccessLevel,
		"reason_code":              state.ReasonCode,
		"reason_message":           state.ReasonMessage,
		"next_steps":               accessControlNextSteps(state),
		"join_url":                 communityJoin,
		"primary_join_url":         primaryJoin,
		"bind_url":                 "/profile?tab=bindings",
		"effective_groups":         allowedGroups,
		"has_active_frozen_keys":   activeFrozen > 0,
		"active_frozen_keys":       activeFrozen,
		"can_restore":              activeFrozen > 0 && state.AccessLevel != model.AccessLevelNone,
		"community_bound":          state.CommunityBound,
		"has_oauth_binding":        state.HasOAuthBinding,
		"has_room_membership":      state.HasRoomMembership,
		"primary_bound":            state.PrimaryBound,
		"primary_platform":         state.PrimaryPlatform,
		"matched_primary_group_id": state.MatchedPrimaryGroupId,
		"denied_message":           accessControlReasonMessage(state, cfg),
		"upgrade_message":          accessControlUpgradeMessage(state, cfg),
		"check_on_login": func() bool {
			if cfg != nil {
				return cfg.CheckOnLogin
			}
			return false
		}(),
		"block_token_create": func() bool {
			if cfg != nil {
				return cfg.BlockTokenCreate
			}
			return false
		}(),
		"block_token_enable": func() bool {
			if cfg != nil {
				return cfg.BlockTokenEnable
			}
			return false
		}(),
	}, nil
}

func accessControlUpgradeMessage(state *model.UserSiteAccessState, cfg *operation_setting.AccessControlSetting) string {
	if cfg == nil {
		return ""
	}
	if state != nil && state.AccessLevel == model.AccessLevelCommunityOnly && strings.TrimSpace(cfg.UpgradeMessage) != "" {
		return strings.TrimSpace(cfg.UpgradeMessage)
	}
	return ""
}

func firstNonEmptyAccessState(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func GetAccessControlAdminStatus() (map[string]any, error) {
	cfg := operation_setting.GetAccessControlSetting()
	siteID := AgentSiteID()
	counts, err := model.CountUserSiteAccessByLevel(siteID)
	if err != nil {
		return nil, err
	}
	recent, err := model.ListUserSiteAccessStates(siteID, 50)
	if err != nil {
		return nil, err
	}
	activeFrozenUsers, _ := model.CountDistinctActiveAccessControlFreezeUsers()
	activeFrozenTokens, _ := model.CountActiveAccessControlFreezeTokens()
	return map[string]any{
		"enabled":          accessControlEnabled(),
		"primary_platform": accessControlPrimaryPlatform(),
		"community_join_url": func() string {
			if cfg != nil {
				return strings.TrimRight(strings.TrimSpace(cfg.CommunityJoinURL), "/")
			}
			return ""
		}(),
		"primary_join_url": func() string {
			if cfg != nil {
				return strings.TrimRight(strings.TrimSpace(cfg.PrimaryJoinURL), "/")
			}
			return ""
		}(),
		"deny_message": func() string {
			if cfg != nil {
				return cfg.DenyMessage
			}
			return ""
		}(),
		"upgrade_message": func() string {
			if cfg != nil {
				return cfg.UpgradeMessage
			}
			return ""
		}(),
		"reward_soft_floor_quota": func() int {
			if cfg != nil {
				return cfg.RewardSoftFloorQuota
			}
			return 0
		}(),
		"reward_hard_floor_quota": func() int {
			if cfg != nil {
				return cfg.RewardHardFloorQuota
			}
			return 0
		}(),
		"daily_site_reward_cap": func() int {
			if cfg != nil {
				return cfg.DailySiteRewardCap
			}
			return 0
		}(),
		"daily_user_reward_cap": func() int {
			if cfg != nil {
				return cfg.DailyUserRewardCap
			}
			return 0
		}(),
		"counts":               counts,
		"recent_states":        recent,
		"recent_frozen_users":  activeFrozenUsers,
		"active_frozen_users":  activeFrozenUsers,
		"active_frozen_tokens": activeFrozenTokens,
	}, nil
}

func ScanAccessControlAndFreeze(ctx context.Context, dryRun bool, limit int) (*struct {
	DryRun         bool             `json:"dry_run"`
	ScannedUsers   int              `json:"scanned_users"`
	CompliantUsers int              `json:"compliant_users"`
	BlockedUsers   int              `json:"blocked_users"`
	ErrorUsers     int              `json:"error_users"`
	TokensEligible int              `json:"tokens_eligible"`
	TokensDisabled int              `json:"tokens_disabled"`
	Users          []map[string]any `json:"users"`
}, error) {
	cfg := operation_setting.GetAccessControlSetting()
	if cfg == nil || !cfg.Enabled {
		return &struct {
			DryRun         bool             `json:"dry_run"`
			ScannedUsers   int              `json:"scanned_users"`
			CompliantUsers int              `json:"compliant_users"`
			BlockedUsers   int              `json:"blocked_users"`
			ErrorUsers     int              `json:"error_users"`
			TokensEligible int              `json:"tokens_eligible"`
			TokensDisabled int              `json:"tokens_disabled"`
			Users          []map[string]any `json:"users"`
		}{DryRun: dryRun, Users: []map[string]any{}}, nil
	}
	if limit <= 0 || limit > 5000 {
		limit = 1000
	}
	users, err := model.ListCommunityGateUsers(limit)
	if err != nil {
		return nil, err
	}
	out := &struct {
		DryRun         bool             `json:"dry_run"`
		ScannedUsers   int              `json:"scanned_users"`
		CompliantUsers int              `json:"compliant_users"`
		BlockedUsers   int              `json:"blocked_users"`
		ErrorUsers     int              `json:"error_users"`
		TokensEligible int              `json:"tokens_eligible"`
		TokensDisabled int              `json:"tokens_disabled"`
		Users          []map[string]any `json:"users"`
	}{DryRun: dryRun, Users: make([]map[string]any, 0, len(users))}
	for _, user := range users {
		out.ScannedUsers++
		state, evalErr := evaluateUserAccessControl(ctx, user.Id, true, !dryRun)
		if evalErr != nil {
			out.ErrorUsers++
			out.Users = append(out.Users, map[string]any{
				"user_id": user.Id, "username": user.Username, "error": evalErr.Error(),
			})
			continue
		}
		item := map[string]any{
			"user_id":         user.Id,
			"username":        user.Username,
			"access_level":    state.AccessLevel,
			"reason_code":     state.ReasonCode,
			"reason_message":  state.ReasonMessage,
			"compliant":       state.AccessLevel != model.AccessLevelNone,
			"primary_bound":   state.PrimaryBound,
			"community_bound": state.CommunityBound,
		}
		if state.AccessLevel == model.AccessLevelNone {
			out.BlockedUsers++
			stats, freezeErr := model.FreezeUserTokensForAccessControl(user.Id, accessControlReasonMessage(state, cfg), dryRun)
			if freezeErr != nil {
				out.ErrorUsers++
				item["freeze_error"] = freezeErr.Error()
			} else {
				item["tokens_eligible"] = stats.Eligible
				item["tokens_disabled"] = stats.Disabled
				out.TokensEligible += stats.Eligible
				out.TokensDisabled += stats.Disabled
				if !dryRun && stats.Disabled > 0 {
					model.RecordLogEvent(user.Id, model.LogTypeSystem, fmt.Sprintf("访问控制已冻结不合规 API Key：冻结 %d 个，原因：%s", stats.Disabled, accessControlReasonMessage(state, cfg)), model.LogEventOptions{
						Username: user.Username,
						SiteId:   AgentSiteID(),
						Category: "access_control",
						Source:   "system",
						Action:   "legacy_keys_frozen",
						Status:   "success",
						Other: map[string]any{
							"access_control": map[string]any{
								"tokens_disabled": stats.Disabled,
								"reason_code":     state.ReasonCode,
								"reason_message":  state.ReasonMessage,
							},
						},
					})
				}
			}
		} else {
			out.CompliantUsers++
			if cfg.AutoRestoreCompliantTokens {
				stats, restoreErr := model.RestoreUserTokensForAccessControl(user.Id)
				if restoreErr != nil {
					item["restore_error"] = restoreErr.Error()
				} else {
					item["restored"] = stats.Restored
					if !dryRun && stats.Restored > 0 {
						model.RecordLogEvent(user.Id, model.LogTypeSystem, fmt.Sprintf("访问控制已恢复合规 API Key：恢复 %d 个", stats.Restored), model.LogEventOptions{
							Username: user.Username,
							SiteId:   AgentSiteID(),
							Category: "access_control",
							Source:   "system",
							Action:   "legacy_keys_restored",
							Status:   "success",
							Other: map[string]any{
								"access_control": map[string]any{
									"restored_tokens": stats.Restored,
									"access_level":    state.AccessLevel,
									"reason_code":     state.ReasonCode,
								},
							},
						})
					}
				}
			}
		}
		out.Users = append(out.Users, item)
	}
	return out, nil
}

func RestoreAccessControlUserTokensIfCompliant(ctx context.Context, userID int) (*model.CommunityGateTokenFreezeStats, *model.UserSiteAccessState, error) {
	state, err := EvaluateUserAccessControl(ctx, userID, true)
	if err != nil {
		return nil, nil, err
	}
	if state == nil || state.AccessLevel == model.AccessLevelNone {
		return nil, state, errors.New(accessControlReasonMessage(state, operation_setting.GetAccessControlSetting()))
	}
	stats, err := model.RestoreUserTokensForAccessControl(userID)
	if err == nil && stats != nil && stats.Restored > 0 {
		model.RecordLogEvent(userID, model.LogTypeSystem, fmt.Sprintf("访问控制已恢复当前账号的 API Key：恢复 %d 个", stats.Restored), model.LogEventOptions{
			SiteId:   AgentSiteID(),
			Category: "access_control",
			Source:   "web",
			Action:   "restore_self",
			Status:   "success",
			Other: map[string]any{
				"access_control": map[string]any{
					"restored_tokens": stats.Restored,
					"access_level":    state.AccessLevel,
					"reason_code":     state.ReasonCode,
				},
			},
		})
	}
	return stats, state, err
}

func EnsureUserCanCreateOrEnableToken(ctx context.Context, userID int, requestedGroup string, action string) (string, *model.UserSiteAccessState, error) {
	requestedGroup = strings.TrimSpace(requestedGroup)
	if strings.EqualFold(strings.TrimSpace(action), "enable") && requestedGroup == "" {
		return "", nil, newAccessControlViolation(action, "empty_group", requestedGroup, nil, nil, operation_setting.GetAccessControlSetting())
	}
	user, err := model.GetUserById(userID, false)
	if err != nil {
		return "", nil, err
	}
	cfg := operation_setting.GetAccessControlSetting()
	state, err := EvaluateUserAccessControl(ctx, userID, false)
	if err != nil {
		return "", nil, err
	}
	baseUsable := GetUserUsableGroups(user.Group)
	usable := baseUsable
	if accessControlMutationEnforced(cfg, action) {
		usable = accessControlAllowedGroups(baseUsable, state, cfg)
		if len(usable) == 0 && state != nil && (state.AccessLevel == model.AccessLevelFullAccess || state.AccessLevel == model.AccessLevelPaidBypass || state.AccessLevel == model.AccessLevelAdminBypass || state.ManualOverrideMode == model.AccessOverrideFullAccess) {
			usable = baseUsable
		}
	}
	if len(usable) == 0 {
		return "", state, newAccessControlViolation(action, "no_usable_groups", requestedGroup, state, usable, cfg)
	}
	if requestedGroup != "" {
		if _, ok := usable[requestedGroup]; !ok {
			return "", state, newAccessControlViolation(action, "group_denied", requestedGroup, state, usable, cfg)
		}
		return requestedGroup, state, nil
	}
	ordered := accessControlOrderedGroups(usable, nil)
	if len(ordered) == 0 {
		return "", state, newAccessControlViolation(action, "no_usable_groups", requestedGroup, state, usable, cfg)
	}
	if _, ok := usable[user.Group]; ok {
		return user.Group, state, nil
	}
	return ordered[0], state, nil
}

func GetUserUsableGroupsWithAccess(ctx context.Context, userID int, userGroup string, refresh bool) (map[string]string, *model.UserSiteAccessState, error) {
	base := GetUserUsableGroups(userGroup)
	state, err := EvaluateUserAccessControl(ctx, userID, refresh)
	if err != nil {
		return nil, nil, err
	}
	cfg := operation_setting.GetAccessControlSetting()
	usable := base
	if cfg != nil && cfg.Enabled {
		usable = accessControlAllowedGroups(base, state, cfg)
	}
	if len(usable) == 0 && state != nil && (state.AccessLevel == model.AccessLevelFullAccess || state.AccessLevel == model.AccessLevelPaidBypass || state.AccessLevel == model.AccessLevelAdminBypass || state.ManualOverrideMode == model.AccessOverrideFullAccess) {
		usable = base
	}
	return usable, state, nil
}

func GetUserAutoGroupWithAccess(ctx context.Context, userID int, userGroup string, refresh bool) ([]string, error) {
	usable, _, err := GetUserUsableGroupsWithAccess(ctx, userID, userGroup, refresh)
	if err != nil {
		return nil, err
	}
	return GetUserAutoGroupFromUsableGroups(usable), nil
}

func GetUserAutoGroupFromUsableGroups(groups map[string]string) []string {
	if _, ok := groups["auto"]; ok {
		return []string{"auto"}
	}
	return []string{}
}
