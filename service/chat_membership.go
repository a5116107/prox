package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/bytedance/gopkg/util/gopool"
)

type ChatMembershipEventRequest struct {
	EventID        string         `json:"event_id"`
	Source         string         `json:"source"`
	RoomID         string         `json:"room_id"`
	ExternalUserID string         `json:"external_user_id"`
	NewAPIUserID   int            `json:"new_api_user_id"`
	EventType      string         `json:"event_type"`
	EventAt        int64          `json:"event_at"`
	Metadata       map[string]any `json:"metadata"`
	RawPayload     map[string]any `json:"raw_payload"`
}

type ChatMembershipEventResult struct {
	EventID   string                     `json:"event_id"`
	Accepted  bool                       `json:"accepted"`
	Duplicate bool                       `json:"duplicate"`
	DryRun    bool                       `json:"dry_run"`
	State     *model.ChatMembershipState `json:"state,omitempty"`
}

type ChatMembershipOverview struct {
	Enabled bool             `json:"enabled"`
	DryRun  bool             `json:"dry_run"`
	Counts  map[string]int64 `json:"counts"`
}

type ChatMembershipDryRunResult struct {
	GraceToExpire int64 `json:"grace_to_expire"`
	DryRun        bool  `json:"dry_run"`
}

type ChatMembershipExpireResult struct {
	Expired          int64 `json:"expired"`
	ReevaluatedUsers int   `json:"reevaluated_users"`
	TokensEligible   int   `json:"tokens_eligible"`
	TokensDisabled   int   `json:"tokens_disabled"`
	DryRun           bool  `json:"dry_run"`
}

type ChatMembershipAdminActionRequest struct {
	Reason     string `json:"reason"`
	UntilHours int    `json:"until_hours"`
}

type ChatMembershipAdminActionResult struct {
	Action string                     `json:"action"`
	State  *model.ChatMembershipState `json:"state"`
}

const chatMembershipMaintenanceTickInterval = 12 * time.Hour

var (
	chatMembershipMaintenanceOnce    sync.Once
	chatMembershipMaintenanceRunning atomic.Bool
)

func VerifyMembershipEventSignature(body []byte, signature string) bool {
	secret := strings.TrimSpace(operation_setting.GetMembershipRiskSetting().EventSecret)
	if secret == "" {
		secret = strings.TrimSpace(common.SessionSecret)
	}
	if secret == "" {
		return false
	}
	signature = strings.TrimSpace(signature)
	if signature == "" {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	signature = strings.TrimPrefix(signature, "sha256=")
	return hmac.Equal([]byte(expected), []byte(signature))
}

func HandleChatMembershipEvent(req ChatMembershipEventRequest) (*ChatMembershipEventResult, error) {
	cfg := operation_setting.GetMembershipRiskSetting()
	if cfg == nil || !cfg.Enabled {
		return nil, errors.New("membership risk is disabled")
	}
	req.Source = strings.TrimSpace(strings.ToLower(req.Source))
	req.EventType = strings.TrimSpace(strings.ToLower(req.EventType))
	req.RoomID = strings.TrimSpace(req.RoomID)
	req.ExternalUserID = strings.TrimSpace(req.ExternalUserID)
	if req.Source == "" || req.RoomID == "" || req.ExternalUserID == "" || req.EventType == "" {
		return nil, errors.New("source, room_id, external_user_id and event_type are required")
	}
	if req.Source == "qq" && !cfg.QQEventsEnabled {
		return nil, errors.New("qq membership events are disabled")
	}
	if req.Source == "tg" && !cfg.TGEventsEnabled {
		return nil, errors.New("tg membership events are disabled")
	}
	if req.EventAt <= 0 {
		req.EventAt = time.Now().Unix()
	}
	if req.EventID == "" {
		req.EventID = fmt.Sprintf("%s:%s:%s:%s:%d", req.Source, req.RoomID, req.ExternalUserID, req.EventType, req.EventAt)
	}
	subject := resolveMembershipBindingSubject(req)
	if req.NewAPIUserID <= 0 && subject.UserID > 0 {
		req.NewAPIUserID = subject.UserID
	}
	raw := req.RawPayload
	if raw == nil {
		raw = map[string]any{}
	}
	rawBytes, _ := json.Marshal(raw)
	metadataBytes, _ := json.Marshal(req.Metadata)
	graceUntil := int64(0)
	switch req.EventType {
	case model.ChatMembershipEventLeave, model.ChatMembershipEventKick, model.ChatMembershipEventScanAbsent:
		graceUntil = req.EventAt + int64(cfg.GraceHours)*3600
	case model.ChatMembershipEventJoin, model.ChatMembershipEventScanPresent:
	default:
		return nil, fmt.Errorf("unsupported event_type: %s", req.EventType)
	}
	event, inserted, err := model.UpsertChatMembershipEvent(model.ChatMembershipEventInput{
		EventId:        req.EventID,
		Source:         req.Source,
		RoomId:         req.RoomID,
		ExternalUserId: req.ExternalUserID,
		NewAPIUserId:   req.NewAPIUserID,
		EventType:      req.EventType,
		EventAt:        req.EventAt,
		RawPayloadJson: string(rawBytes),
		MetadataJson:   string(metadataBytes),
		GraceUntil:     graceUntil,
	})
	if err != nil {
		return nil, err
	}
	if err := applyMembershipSideEffects(req, cfg, subject); err != nil {
		return nil, err
	}
	state, _ := resolveMembershipStateForResult(req, subject.UserID)
	common.SysLog(fmt.Sprintf("[MembershipRisk] event=%s source=%s room=%s external=%s inserted=%v status=%s", event.EventType, event.Source, event.RoomId, event.ExternalUserId, inserted, firstMembershipStatus(state)))
	return &ChatMembershipEventResult{
		EventID:   event.EventId,
		Accepted:  inserted,
		Duplicate: !inserted,
		DryRun:    cfg.DryRun,
		State:     state,
	}, nil
}

func GetChatMembershipOverview() (*ChatMembershipOverview, error) {
	counts, err := model.CountChatMembershipStatesByStatus()
	if err != nil {
		return nil, err
	}
	cfg := operation_setting.GetMembershipRiskSetting()
	return &ChatMembershipOverview{Enabled: cfg.Enabled, DryRun: cfg.DryRun, Counts: counts}, nil
}

func GetChatMembershipUnresolved(limit int) (*ChatMembershipUnresolvedOverview, error) {
	return GetChatMembershipUnresolvedOverview(limit)
}

func ListChatMembershipStates(status string, source string, roomID string, limit int) ([]model.ChatMembershipState, error) {
	return model.ListChatMembershipStates(status, source, roomID, limit)
}

func DryRunExpireChatMembershipGrace() (*ChatMembershipDryRunResult, error) {
	n, err := model.ExpireChatMembershipGrace(time.Now().Unix(), true, 1000)
	if err != nil {
		return nil, err
	}
	return &ChatMembershipDryRunResult{GraceToExpire: n, DryRun: true}, nil
}

func ExpireChatMembershipGrace() (int64, error) {
	expiryOutcome, err := ExpireChatMembershipGraceAndEnforce()
	if err != nil {
		return 0, err
	}
	return expiryOutcome.Expired, nil
}

func ExpireChatMembershipGraceAndEnforce() (*ChatMembershipExpireResult, error) {
	cfg := operation_setting.GetMembershipRiskSetting()
	now := time.Now().Unix()
	dueStates, err := model.ListChatMembershipStates(model.ChatMembershipStatusGrace, "", "", 1000)
	if err != nil {
		return nil, err
	}
	dueUserIDs := map[int]struct{}{}
	for _, state := range dueStates {
		if state.GraceUntil > 0 && state.GraceUntil <= now && state.NewAPIUserId > 0 {
			dueUserIDs[state.NewAPIUserId] = struct{}{}
		}
	}
	expired, err := model.ExpireChatMembershipGrace(now, cfg.DryRun, 1000)
	if err != nil {
		return nil, err
	}
	expiryOutcome := &ChatMembershipExpireResult{Expired: expired, DryRun: cfg.DryRun}
	if expired <= 0 || cfg.DryRun {
		return expiryOutcome, nil
	}
	for userID := range dueUserIDs {
		if _, err := EvaluateUserAccessControl(context.Background(), userID, true); err != nil {
			logger.LogWarn(context.Background(), fmt.Sprintf("membership grace access reevaluate failed user=%d: %v", userID, err))
			continue
		}
		expiryOutcome.ReevaluatedUsers++
		if cfg.FreezeCommunityTokensAfterGrace || cfg.RevokeCommunityAccessAfterGrace {
			stats, err := model.FreezeUserTokensForAccessControl(userID, "membership grace expired", false)
			if err != nil {
				logger.LogWarn(context.Background(), fmt.Sprintf("membership grace token freeze failed user=%d: %v", userID, err))
				continue
			}
			expiryOutcome.TokensEligible += stats.Eligible
			expiryOutcome.TokensDisabled += stats.Disabled
			if stats.Disabled > 0 {
				model.RecordLogEvent(userID, model.LogTypeSystem, fmt.Sprintf("群成员资格过期，已冻结社区权益 API Key：%d 个", stats.Disabled), model.LogEventOptions{
					SiteId:     AgentSiteID(),
					Category:   "membership_risk",
					Source:     "system",
					Action:     "membership_expired_freeze",
					Status:     "success",
					BudgetPool: "community",
					Other: map[string]interface{}{
						"membership_risk": map[string]interface{}{
							"reason":          "grace_expired",
							"tokens_disabled": stats.Disabled,
						},
					},
				})
			}
		}
	}
	return expiryOutcome, nil
}

func BackfillMembershipUserLinks(source string, roomID string, externalUserID string, userID int) (int64, int64, error) {
	source = strings.TrimSpace(strings.ToLower(source))
	roomID = strings.TrimSpace(roomID)
	externalUserID = strings.TrimSpace(externalUserID)
	if source == "" || externalUserID == "" || userID <= 0 {
		return 0, 0, nil
	}

	stateRows, eventRows, err := model.BackfillChatMembershipUserByExternal(source, externalUserID, userID)
	if err != nil {
		return 0, 0, err
	}
	if stateRows > 0 || eventRows > 0 {
		common.SysLog(fmt.Sprintf(
			"[MembershipRisk] bind backfill source=%s room=%s external=%s user_id=%d states=%d events=%d",
			source,
			roomID,
			externalUserID,
			userID,
			stateRows,
			eventRows,
		))
	}
	return stateRows, eventRows, nil
}

func BackfillHistoricalMembershipUserLinks(limit int) (int64, int64, error) {
	stateRows, eventRows, err := model.BackfillChatMembershipUsersFromBindings(limit)
	if err != nil {
		return 0, 0, err
	}
	crossStateRows, crossEventRows, err := backfillCrossSourceMembershipUserLinks(limit)
	if err != nil {
		return stateRows, eventRows, err
	}
	stateRows += crossStateRows
	eventRows += crossEventRows
	if stateRows > 0 || eventRows > 0 {
		common.SysLog(fmt.Sprintf(
			"[MembershipRisk] maintenance backfill states=%d events=%d limit=%d",
			stateRows,
			eventRows,
			limit,
		))
	}
	return stateRows, eventRows, nil
}

func backfillCrossSourceMembershipUserLinks(limit int) (int64, int64, error) {
	states, err := model.ListChatMembershipStatesWithoutUser(limit)
	if err != nil {
		return 0, 0, err
	}
	seen := make(map[string]struct{}, len(states))
	var totalStateRows int64
	var totalEventRows int64
	for _, state := range states {
		key := fmt.Sprintf("%s|%s|%s", state.Source, state.RoomId, state.ExternalUserId)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		identity, candidate, err := resolveCrossSourceMembershipIdentity(ChatMembershipEventRequest{
			Source:         state.Source,
			RoomID:         state.RoomId,
			ExternalUserID: state.ExternalUserId,
		}, membershipBindingSubject{})
		if err != nil {
			return totalStateRows, totalEventRows, err
		}
		if identity == nil || identity.UserId <= 0 {
			continue
		}
		stateRows, eventRows, err := model.BackfillChatMembershipUserByExternal(state.Source, state.ExternalUserId, identity.UserId)
		if err != nil {
			return totalStateRows, totalEventRows, err
		}
		if stateRows > 0 || eventRows > 0 {
			common.SysLog(fmt.Sprintf(
				"[MembershipRisk] cross-source backfill matched source=%s room=%s external=%s candidate=%s provider=%s user_id=%d states=%d events=%d",
				state.Source,
				state.RoomId,
				state.ExternalUserId,
				candidate,
				identity.Provider,
				identity.UserId,
				stateRows,
				eventRows,
			))
		}
		totalStateRows += stateRows
		totalEventRows += eventRows
	}
	return totalStateRows, totalEventRows, nil
}

func RestoreChatMembershipState(id int, reason string) (*ChatMembershipAdminActionResult, error) {
	now := time.Now().Unix()
	state, err := model.UpdateChatMembershipStateStatus(id, model.ChatMembershipStatusRestored, map[string]interface{}{
		"restored_at":   now,
		"last_seen_at":  now,
		"left_at":       int64(0),
		"grace_until":   int64(0),
		"bypass_reason": strings.TrimSpace(reason),
		"bypass_until":  int64(0),
		"metadata_json": buildMembershipAdminMetadata("restore", reason, 0),
	})
	if err != nil {
		return nil, err
	}
	if _, err := model.SetAgentChatBindingsEnabled(AgentSiteID(), state.Source, state.RoomId, state.ExternalUserId, true, state.NewAPIUserId); err != nil {
		return nil, err
	}
	if state.NewAPIUserId > 0 {
		_, _ = EvaluateUserAccessControl(context.Background(), state.NewAPIUserId, true)
	}
	common.SysLog(fmt.Sprintf("[MembershipRisk] admin restore state_id=%d reason=%s", id, strings.TrimSpace(reason)))
	return &ChatMembershipAdminActionResult{Action: "restore", State: state}, nil
}

func BypassChatMembershipState(id int, reason string, untilHours int) (*ChatMembershipAdminActionResult, error) {
	if untilHours < 0 {
		untilHours = 0
	}
	var bypassUntil int64
	if untilHours > 0 {
		bypassUntil = time.Now().Add(time.Duration(untilHours) * time.Hour).Unix()
	}
	state, err := model.UpdateChatMembershipStateStatus(id, model.ChatMembershipStatusManualBypass, map[string]interface{}{
		"bypass_reason": strings.TrimSpace(reason),
		"bypass_until":  bypassUntil,
		"metadata_json": buildMembershipAdminMetadata("bypass", reason, bypassUntil),
	})
	if err != nil {
		return nil, err
	}
	common.SysLog(fmt.Sprintf("[MembershipRisk] admin bypass state_id=%d until=%d reason=%s", id, bypassUntil, strings.TrimSpace(reason)))
	return &ChatMembershipAdminActionResult{Action: "bypass", State: state}, nil
}

func ClearChatMembershipBypass(id int, reason string) (*ChatMembershipAdminActionResult, error) {
	now := time.Now().Unix()
	current, err := model.GetChatMembershipStateByID(id)
	if err != nil {
		return nil, err
	}
	nextStatus := model.ChatMembershipStatusRestored
	if current.LeftAt > 0 && current.JoinedAt <= current.LeftAt {
		if current.GraceUntil > now {
			nextStatus = model.ChatMembershipStatusGrace
		} else if current.GraceUntil > 0 {
			nextStatus = model.ChatMembershipStatusLeftExpired
		}
	}
	state, err := model.UpdateChatMembershipStateStatus(id, nextStatus, map[string]interface{}{
		"restored_at":   time.Now().Unix(),
		"bypass_reason": strings.TrimSpace(reason),
		"bypass_until":  int64(0),
		"metadata_json": buildMembershipAdminMetadata("clear_bypass", reason, 0),
	})
	if err != nil {
		return nil, err
	}
	common.SysLog(fmt.Sprintf("[MembershipRisk] admin clear bypass state_id=%d reason=%s", id, strings.TrimSpace(reason)))
	return &ChatMembershipAdminActionResult{Action: "clear_bypass", State: state}, nil
}

func UserHasActivePaidEntitlement(userID int) bool {
	if userID <= 0 {
		return false
	}
	ok, err := model.HasActiveUserSubscription(userID)
	if err == nil && ok {
		return true
	}
	user, err := model.GetUserById(userID, false)
	if err == nil && user != nil && strings.TrimSpace(user.Group) != "" && strings.TrimSpace(user.Group) != "default" {
		return true
	}
	return false
}

func communityBenefitScope(source string, roomID string) bool {
	normalizedSource := strings.TrimSpace(strings.ToLower(source))
	switch normalizedSource {
	case "community", "dc", "hhhl":
		return true
	}
	roomID = strings.TrimSpace(roomID)
	if roomID == "" {
		return false
	}
	_, _, _, _, configuredRoomIDs, _, _ := communityGateEffectiveConfig()
	return communityGateContainsRoom(configuredRoomIDs, roomID)
}

func communityGateAllowsBenefit(userID int) bool {
	if userID <= 0 {
		return false
	}
	if cached, err := model.GetUserSiteAccessState(AgentSiteID(), userID); err == nil && cached != nil {
		if accessControlStateFresh(cached) && cached.CommunityBound && cached.HasRoomMembership {
			return true
		}
	}
	state, err := accessControlCommunityBinding(context.Background(), userID, false)
	if err != nil || state == nil {
		return false
	}
	return state.CommunityBound && state.HasRoomMembership
}

func normalizeMembershipBenefitType(benefitType string) string {
	normalized := strings.TrimSpace(strings.ToLower(benefitType))
	switch {
	case normalized == "":
		return ""
	case strings.Contains(normalized, "paid"):
		return "paid_access"
	case strings.Contains(normalized, "invitee"):
		return "invite_invitee_reward"
	case strings.Contains(normalized, "inviter"):
		return "invite_inviter_reward"
	case strings.Contains(normalized, "invite"):
		return "invite_reward"
	case strings.Contains(normalized, "campaign"), strings.Contains(normalized, "bonus"), strings.Contains(normalized, "活动"):
		return "campaign_bonus"
	case strings.Contains(normalized, "game"):
		return "game_reward"
	case strings.Contains(normalized, "daily_message"), strings.Contains(normalized, "message_reward"), strings.Contains(normalized, "checkin"), strings.Contains(normalized, "join_reward"), strings.Contains(normalized, "签到"):
		return "checkin"
	default:
		return normalized
	}
}

func membershipBenefitGuardEnabled(cfg *operation_setting.MembershipRiskSetting, benefitType string) bool {
	if cfg == nil {
		return false
	}
	switch normalizeMembershipBenefitType(benefitType) {
	case "paid_access":
		return true
	case "checkin":
		return cfg.BlockCheckinOnLeft
	case "game_reward":
		return cfg.BlockGameRewardOnLeft
	case "invite_reward", "invite_inviter_reward", "invite_invitee_reward":
		return cfg.BlockInviteRewardOnLeft
	case "campaign_bonus":
		return cfg.BlockCampaignBonusOnLeft
	default:
		return cfg.BlockCheckinOnLeft
	}
}

func CanReceiveCommunityBenefit(userID int, source string, roomID string, externalUserID string, benefitType string) (bool, string) {
	cfg := operation_setting.GetMembershipRiskSetting()
	if cfg == nil || !cfg.Enabled {
		return true, "membership_risk_disabled"
	}
	if cfg.DryRun {
		return true, "dry_run_allow"
	}
	normalizedBenefitType := normalizeMembershipBenefitType(benefitType)
	if !membershipBenefitGuardEnabled(cfg, normalizedBenefitType) {
		return true, "benefit_gate_disabled"
	}
	if cfg.PaidBypassEnabled && UserHasActivePaidEntitlement(userID) && normalizedBenefitType == "paid_access" {
		return true, "paid_bypass"
	}
	states, err := loadMembershipStatesForBenefit(userID, source, externalUserID)
	now := time.Now().Unix()
	var preferred *model.ChatMembershipState
	if err == nil && len(states) > 0 {
		preferred = selectPreferredMembershipState(states, strings.TrimSpace(strings.ToLower(source)), strings.TrimSpace(roomID), strings.TrimSpace(externalUserID))
		if preferred != nil {
			if allowed, reason := membershipStateAllowsBenefit(preferred, now); allowed {
				return true, reason
			} else if !communityBenefitScope(source, roomID) {
				return false, reason
			}
		}
	}
	if communityBenefitScope(source, roomID) && communityGateAllowsBenefit(userID) {
		return true, "community_bound"
	}
	if err != nil || len(states) == 0 || preferred == nil {
		return false, "membership_missing"
	}
	_, reason := membershipStateAllowsBenefit(preferred, now)
	return false, reason
}

func CanCurrentUserReceiveCheckinBenefit(userID int) bool {
	allowed, _ := CanCurrentUserReceiveCheckinBenefitReason(userID)
	return allowed
}

func CanCurrentUserReceiveCheckinBenefitReason(userID int) (bool, string) {
	cfg := operation_setting.GetMembershipRiskSetting()
	if cfg == nil || !cfg.Enabled || !cfg.BlockCheckinOnLeft {
		return true, "membership_risk_disabled"
	}
	if cfg.DryRun {
		return true, "dry_run_allow"
	}
	if cfg.PaidBypassEnabled && UserHasActivePaidEntitlement(userID) {
		return true, "paid_bypass"
	}

	if gate, err := EvaluateCommunityGate(context.Background(), userID, false); err == nil && gate != nil {
		if gate.Compliant {
			return true, "community_bound"
		}
		if reason := strings.TrimSpace(gate.ReasonCode); reason != "" {
			return false, reason
		}
	}

	states, err := model.ListChatMembershipStatesByUser(userID, 200)
	if err != nil {
		common.SysLog(fmt.Sprintf("[MembershipRisk] checkin membership check failed user_id=%d err=%s", userID, err.Error()))
		return false, "membership_check_failed"
	}
	now := time.Now().Unix()
	bestReason := ""
	for i := range states {
		if !communityBenefitScope(states[i].Source, states[i].RoomId) {
			continue
		}
		if allowed, reason := membershipStateAllowsBenefit(&states[i], now); allowed {
			return true, reason
		} else if bestReason == "" {
			bestReason = reason
		}
	}
	if bestReason != "" {
		return false, bestReason
	}
	return false, "membership_missing"
}

func firstMembershipStatus(state *model.ChatMembershipState) string {
	if state == nil {
		return ""
	}
	return state.Status
}

type membershipBindingSubject struct {
	Binding  *model.AgentChatBinding
	UserID   int
	Username string
	Role     string
	Remark   string
}

func membershipIdentityCandidates(req ChatMembershipEventRequest, subject membershipBindingSubject) []string {
	out := make([]string, 0, 8)
	seen := make(map[string]struct{}, 8)
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" || value == "<nil>" {
			return
		}
		key := strings.ToLower(value)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		out = append(out, value)
	}
	add(req.ExternalUserID)
	add(subject.Username)
	for _, m := range []map[string]any{req.Metadata, req.RawPayload} {
		for _, key := range []string{"username", "user_name", "nickname", "card", "sender_nick", "name", "user_id"} {
			if value, ok := m[key]; ok {
				add(fmt.Sprint(value))
			}
		}
	}
	return out
}

func resolveCrossSourceMembershipIdentity(req ChatMembershipEventRequest, subject membershipBindingSubject) (*model.UserIdentityBinding, string, error) {
	if subject.UserID > 0 {
		return nil, "", nil
	}
	source := strings.TrimSpace(strings.ToLower(req.Source))
	if source != "qq" && source != "tg" {
		return nil, "", nil
	}
	candidates := membershipIdentityCandidates(req, subject)
	if len(candidates) == 0 {
		return nil, "", nil
	}
	matches := make(map[int]model.UserIdentityBinding)
	matchCandidate := ""
	for _, candidate := range candidates {
		rows, err := model.ListActiveUserIdentityBindingsByUsername(AgentSiteID(), candidate, "community")
		if err != nil {
			return nil, "", err
		}
		if len(rows) == 0 {
			continue
		}
		uniqueUsers := make(map[int]model.UserIdentityBinding)
		for _, row := range rows {
			if row.UserId > 0 {
				uniqueUsers[row.UserId] = row
			}
		}
		if len(uniqueUsers) == 0 {
			continue
		}
		if len(uniqueUsers) > 1 {
			common.SysLog(fmt.Sprintf(
				"[MembershipRisk] cross-source identity ambiguous source=%s room=%s external=%s candidate=%s matches=%d",
				req.Source,
				req.RoomID,
				req.ExternalUserID,
				candidate,
				len(uniqueUsers),
			))
			return nil, "", nil
		}
		for userID, row := range uniqueUsers {
			matches[userID] = row
			if matchCandidate == "" {
				matchCandidate = candidate
			}
		}
	}
	if len(matches) == 0 {
		return nil, "", nil
	}
	if len(matches) > 1 {
		common.SysLog(fmt.Sprintf(
			"[MembershipRisk] cross-source identity conflicting source=%s room=%s external=%s candidates=%v users=%d",
			req.Source,
			req.RoomID,
			req.ExternalUserID,
			candidates,
			len(matches),
		))
		return nil, "", nil
	}
	for _, row := range matches {
		copy := row
		return &copy, matchCandidate, nil
	}
	return nil, "", nil
}

func resolveMembershipBindingSubject(req ChatMembershipEventRequest) membershipBindingSubject {
	subject := membershipBindingSubject{
		UserID:   req.NewAPIUserID,
		Username: membershipUsernameFromRequest(req),
		Role:     "member",
		Remark:   "membership_event",
	}
	if binding, err := model.GetAnyAgentChatBinding(AgentSiteID(), req.Source, req.RoomID, req.ExternalUserID); err == nil && binding != nil {
		subject.Binding = binding
		if subject.UserID <= 0 && binding.NewAPIUserId > 0 {
			subject.UserID = binding.NewAPIUserId
		}
		if subject.Username == "" {
			subject.Username = strings.TrimSpace(binding.Username)
		}
		if strings.TrimSpace(binding.Role) != "" {
			subject.Role = strings.TrimSpace(binding.Role)
		}
		if strings.TrimSpace(binding.Remark) != "" {
			subject.Remark = strings.TrimSpace(binding.Remark)
		}
	}
	if subject.UserID <= 0 {
		if identity, err := model.GetUserIdentityBindingByExternal(AgentSiteID(), req.Source, req.ExternalUserID); err == nil && identity != nil {
			subject.UserID = identity.UserId
			if subject.Username == "" {
				subject.Username = strings.TrimSpace(identity.Username)
			}
		}
	}
	if subject.UserID <= 0 {
		if state, err := model.GetChatMembershipState(req.Source, req.RoomID, req.ExternalUserID); err == nil && state != nil && state.NewAPIUserId > 0 {
			subject.UserID = state.NewAPIUserId
			if subject.Remark == "" || subject.Remark == "membership_event" {
				subject.Remark = "membership_event state_link"
			} else if !strings.Contains(subject.Remark, "state_link") {
				subject.Remark = strings.TrimSpace(subject.Remark + " state_link")
			}
		}
	}
	if subject.UserID <= 0 {
		if identity, candidate, err := resolveCrossSourceMembershipIdentity(req, subject); err == nil && identity != nil {
			subject.UserID = identity.UserId
			if subject.Username == "" {
				subject.Username = strings.TrimSpace(identity.Username)
			}
			if subject.Remark == "" || subject.Remark == "membership_event" {
				subject.Remark = "membership_event cross_source_identity"
			} else if !strings.Contains(subject.Remark, "cross_source_identity") {
				subject.Remark = strings.TrimSpace(subject.Remark + " cross_source_identity")
			}
			common.SysLog(fmt.Sprintf(
				"[MembershipRisk] cross-source identity matched source=%s room=%s external=%s candidate=%s provider=%s user_id=%d",
				req.Source,
				req.RoomID,
				req.ExternalUserID,
				candidate,
				identity.Provider,
				identity.UserId,
			))
		} else if err != nil {
			common.SysLog(fmt.Sprintf(
				"[MembershipRisk] cross-source identity lookup failed source=%s room=%s external=%s err=%v",
				req.Source,
				req.RoomID,
				req.ExternalUserID,
				err,
			))
		}
	}
	return subject
}

func applyMembershipSideEffects(req ChatMembershipEventRequest, cfg *operation_setting.MembershipRiskSetting, subject membershipBindingSubject) error {
	userID := subject.UserID
	switch req.EventType {
	case model.ChatMembershipEventLeave, model.ChatMembershipEventKick, model.ChatMembershipEventScanAbsent:
		if _, err := model.SetAgentChatBindingsEnabled(AgentSiteID(), req.Source, req.RoomID, req.ExternalUserID, false, userID); err != nil {
			return err
		}
		updates := map[string]interface{}{
			"status":        model.ChatMembershipStatusGrace,
			"left_at":       req.EventAt,
			"grace_until":   req.EventAt + int64(cfg.GraceHours)*3600,
			"last_seen_at":  req.EventAt,
			"last_event_id": req.EventID,
		}
		if userID > 0 {
			updates["new_api_user_id"] = userID
		}
		if _, err := model.UpdateChatMembershipStatesByExternal(req.Source, req.RoomID, req.ExternalUserID, updates); err != nil {
			return err
		}
		if userID > 0 && !cfg.DryRun {
			CommunityGateInvalidateMembership(userID, req.RoomID)
			if err := model.DeleteUserSiteAccessState(AgentSiteID(), userID); err != nil {
				return err
			}
			if err := model.InvalidateUserCache(userID); err != nil {
				return err
			}
			if err := model.InvalidateUserTokensCache(userID); err != nil {
				return err
			}
			stats, err := model.FreezeUserTokensForAccessControl(userID, "membership revoked: "+req.EventType, false)
			if err != nil {
				return err
			}
			if stats != nil && stats.Disabled > 0 {
				model.RecordLogEvent(userID, model.LogTypeSystem, fmt.Sprintf("群成员资格已失效，已即时冻结 API Key：%d 个", stats.Disabled), model.LogEventOptions{
					SiteId:   AgentSiteID(),
					Category: "membership_risk",
					Source:   req.Source,
					Action:   "membership_immediate_freeze",
					Status:   "success",
					Other: map[string]interface{}{
						"membership_risk": map[string]interface{}{
							"event_type":      req.EventType,
							"room_id":         req.RoomID,
							"tokens_disabled": stats.Disabled,
						},
					},
				})
			}
		}
	case model.ChatMembershipEventJoin, model.ChatMembershipEventScanPresent:
		if userID > 0 {
			CommunityGateInvalidateMembership(userID, req.RoomID)
			_ = model.DeleteUserSiteAccessState(AgentSiteID(), userID)
			_ = model.InvalidateUserCache(userID)
			_ = model.InvalidateUserTokensCache(userID)
		}
		nextStatus := model.ChatMembershipStatusActive
		if userID > 0 {
			if _, err := model.UpsertAgentChatBindingFromMembership(AgentSiteID(), req.Source, req.RoomID, req.ExternalUserID, userID, subject.Username, subject.Role, subject.Remark); err != nil {
				return err
			}
			if _, err := model.SetAgentChatBindingsEnabled(AgentSiteID(), req.Source, req.RoomID, req.ExternalUserID, true, userID); err != nil {
				return err
			}
		} else {
			nextStatus = model.ChatMembershipStatusUnboundObserved
			common.SysLog(fmt.Sprintf(
				"[MembershipRisk] skip binding upsert without resolved user source=%s room=%s external=%s event=%s",
				req.Source,
				req.RoomID,
				req.ExternalUserID,
				req.EventType,
			))
		}
		updates := map[string]interface{}{
			"status":        nextStatus,
			"joined_at":     req.EventAt,
			"left_at":       int64(0),
			"grace_until":   int64(0),
			"last_seen_at":  req.EventAt,
			"last_event_id": req.EventID,
		}
		if nextStatus == model.ChatMembershipStatusActive {
			updates["restored_at"] = req.EventAt
		} else {
			updates["restored_at"] = int64(0)
		}
		if userID > 0 {
			updates["new_api_user_id"] = userID
		}
		if _, err := model.UpdateChatMembershipStatesByExternal(req.Source, req.RoomID, req.ExternalUserID, updates); err != nil {
			return err
		}
	default:
		return nil
	}
	if userID > 0 {
		_, _ = EvaluateUserAccessControl(context.Background(), userID, true)
	}
	return nil
}

func resolveMembershipStateForResult(req ChatMembershipEventRequest, userID int) (*model.ChatMembershipState, error) {
	if state, err := model.GetChatMembershipState(req.Source, req.RoomID, req.ExternalUserID); err == nil && state != nil {
		return state, nil
	}
	if userID > 0 {
		if states, err := model.ListChatMembershipStatesByUser(userID, 100); err == nil {
			for i := range states {
				if strings.TrimSpace(states[i].Source) == strings.TrimSpace(req.Source) &&
					strings.TrimSpace(states[i].RoomId) == strings.TrimSpace(req.RoomID) &&
					strings.TrimSpace(states[i].ExternalUserId) == strings.TrimSpace(req.ExternalUserID) {
					return &states[i], nil
				}
			}
		}
	}
	return nil, nil
}

func membershipUsernameFromRequest(req ChatMembershipEventRequest) string {
	candidates := []map[string]any{req.Metadata, req.RawPayload}
	keys := []string{"username", "user_name", "nickname", "card", "sender_nick", "name"}
	for _, m := range candidates {
		for _, key := range keys {
			if value, ok := m[key]; ok {
				if text := strings.TrimSpace(fmt.Sprint(value)); text != "" && text != "<nil>" {
					return text
				}
			}
		}
	}
	return ""
}

func membershipStateAllowsBenefit(state *model.ChatMembershipState, now int64) (bool, string) {
	if state == nil {
		return false, "membership_missing"
	}
	if state.Status == model.ChatMembershipStatusManualBypass && (state.BypassUntil == 0 || state.BypassUntil > now) {
		return true, "manual_bypass"
	}
	switch state.Status {
	case model.ChatMembershipStatusActive, model.ChatMembershipStatusRestored, model.ChatMembershipStatusRejoinPending:
		return true, state.Status
	case model.ChatMembershipStatusGrace:
		if state.GraceUntil > 0 && state.GraceUntil <= now {
			return false, "grace_expired"
		}
		return false, "grace"
	default:
		return false, state.Status
	}
}

func loadMembershipStatesForBenefit(userID int, source string, externalUserID string) ([]model.ChatMembershipState, error) {
	if userID > 0 {
		return model.ListChatMembershipStatesByUser(userID, 200)
	}
	if strings.TrimSpace(source) != "" && strings.TrimSpace(externalUserID) != "" {
		return model.ListChatMembershipStatesByExternal(source, externalUserID, 200)
	}
	return []model.ChatMembershipState{}, nil
}

func selectPreferredMembershipState(states []model.ChatMembershipState, source string, roomID string, externalUserID string) *model.ChatMembershipState {
	source = strings.TrimSpace(strings.ToLower(source))
	roomID = strings.TrimSpace(roomID)
	externalUserID = strings.TrimSpace(externalUserID)

	exactMatch := func(targetRoom string) *model.ChatMembershipState {
		for i := range states {
			state := &states[i]
			if source != "" && strings.TrimSpace(strings.ToLower(state.Source)) != source {
				continue
			}
			if externalUserID != "" && strings.TrimSpace(state.ExternalUserId) != externalUserID {
				continue
			}
			if strings.TrimSpace(state.RoomId) != targetRoom {
				continue
			}
			return state
		}
		return nil
	}

	if roomID != "" {
		return exactMatch(roomID)
	}
	if state := exactMatch(""); state != nil {
		return state
	}
	for i := range states {
		state := &states[i]
		if source != "" && strings.TrimSpace(strings.ToLower(state.Source)) != source {
			continue
		}
		if externalUserID != "" && strings.TrimSpace(state.ExternalUserId) != externalUserID {
			continue
		}
		return state
	}
	return nil
}

func StartChatMembershipMaintenanceTask() {
	chatMembershipMaintenanceOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}
		gopool.Go(func() {
			logger.LogInfo(context.Background(), fmt.Sprintf("chat membership maintenance task started: tick=%s", chatMembershipMaintenanceTickInterval))
			ticker := time.NewTicker(chatMembershipMaintenanceTickInterval)
			defer ticker.Stop()

			runChatMembershipMaintenance()
			for range ticker.C {
				runChatMembershipMaintenance()
			}
		})
	})
}

func runChatMembershipMaintenance() {
	if !chatMembershipMaintenanceRunning.CompareAndSwap(false, true) {
		return
	}
	defer chatMembershipMaintenanceRunning.Store(false)
	ctx := context.Background()
	if stateRows, eventRows, err := BackfillHistoricalMembershipUserLinks(500); err != nil {
		logger.LogWarn(ctx, fmt.Sprintf("chat membership user backfill failed: %v", err))
	} else if stateRows > 0 || eventRows > 0 {
		logger.LogInfo(ctx, fmt.Sprintf("chat membership user backfill repaired: states=%d events=%d", stateRows, eventRows))
	}
	totalExpired, err := ExpireChatMembershipGrace()
	if err != nil {
		logger.LogWarn(ctx, fmt.Sprintf("chat membership grace expiration failed: %v", err))
		return
	}
	if totalExpired > 0 {
		logger.LogInfo(ctx, fmt.Sprintf("chat membership grace expired: count=%d", totalExpired))
	}
}

func buildMembershipAdminMetadata(action string, reason string, until int64) string {
	payload := map[string]any{
		"admin_action": action,
		"reason":       strings.TrimSpace(reason),
		"until":        until,
		"at":           time.Now().Unix(),
	}
	b, _ := json.Marshal(payload)
	return string(b)
}
