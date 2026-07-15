package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

type ChatMembershipAdminBatchRequest struct {
	Limit int `json:"limit"`
}

type ChatMembershipBackfillResult struct {
	Action              string `json:"action"`
	Limit               int    `json:"limit"`
	StateRows           int64  `json:"state_rows"`
	EventRows           int64  `json:"event_rows"`
	ActivatedStates     int64  `json:"activated_states"`
	RemainingUnresolved int64  `json:"remaining_unresolved"`
}

type ChatMembershipDemoteResult struct {
	Action                    string `json:"action"`
	Limit                     int    `json:"limit"`
	DemotedStates             int64  `json:"demoted_states"`
	RemainingUnresolvedActive int64  `json:"remaining_unresolved_active"`
}

type ChatMembershipUnresolvedIdentityCandidate struct {
	UserID                  int    `json:"user_id"`
	Username                string `json:"username"`
	CommunityExternalUserID string `json:"community_external_user_id"`
	MatchedBy               string `json:"matched_by"`
	MatchType               string `json:"match_type"`
}

type ChatMembershipUnresolvedBindingHint struct {
	Source       string `json:"source"`
	RoomID       string `json:"room_id"`
	NewAPIUserID int    `json:"new_api_user_id"`
	Username     string `json:"username"`
	Enabled      bool   `json:"enabled"`
	Remark       string `json:"remark"`
}

type ChatMembershipUnresolvedRecord struct {
	StateID          int                                         `json:"state_id"`
	Source           string                                      `json:"source"`
	RoomID           string                                      `json:"room_id"`
	ExternalUserID   string                                      `json:"external_user_id"`
	Status           string                                      `json:"status"`
	UpdatedAt        int64                                       `json:"updated_at"`
	LatestEventType  string                                      `json:"latest_event_type"`
	LatestEventAt    int64                                       `json:"latest_event_at"`
	IdentityHints    []string                                    `json:"identity_hints"`
	MatchCandidates  []ChatMembershipUnresolvedIdentityCandidate `json:"match_candidates"`
	ExistingBindings []ChatMembershipUnresolvedBindingHint       `json:"existing_bindings"`
	SuggestedAction  string                                      `json:"suggested_action"`
	Reason           string                                      `json:"reason"`
}

type ChatMembershipUnresolvedOverview struct {
	Limit                int                              `json:"limit"`
	UnresolvedStateCount int64                            `json:"unresolved_state_count"`
	UnresolvedEventCount int64                            `json:"unresolved_event_count"`
	Records              []ChatMembershipUnresolvedRecord `json:"records"`
}

func normalizeChatMembershipBatchLimit(limit int) int {
	if limit <= 0 || limit > 2000 {
		return 500
	}
	return limit
}

func GetChatMembershipUnresolvedOverview(limit int) (*ChatMembershipUnresolvedOverview, error) {
	limit = normalizeChatMembershipBatchLimit(limit)
	unresolvedStateCount, err := model.CountChatMembershipStatesByStatusAndUserLink("", false)
	if err != nil {
		return nil, err
	}
	unresolvedEventCount, err := model.CountChatMembershipEventsWithoutUser()
	if err != nil {
		return nil, err
	}
	states, err := model.ListChatMembershipStatesByStatusAndUserLink("", false, limit)
	if err != nil {
		return nil, err
	}
	records := make([]ChatMembershipUnresolvedRecord, 0, len(states))
	for _, state := range states {
		record, buildErr := buildChatMembershipUnresolvedRecord(state)
		if buildErr != nil {
			return nil, buildErr
		}
		records = append(records, record)
	}
	return &ChatMembershipUnresolvedOverview{
		Limit:                limit,
		UnresolvedStateCount: unresolvedStateCount,
		UnresolvedEventCount: unresolvedEventCount,
		Records:              records,
	}, nil
}

func buildChatMembershipUnresolvedRecord(state model.ChatMembershipState) (ChatMembershipUnresolvedRecord, error) {
	record := ChatMembershipUnresolvedRecord{
		StateID:        state.Id,
		Source:         strings.TrimSpace(state.Source),
		RoomID:         strings.TrimSpace(state.RoomId),
		ExternalUserID: strings.TrimSpace(state.ExternalUserId),
		Status:         strings.TrimSpace(state.Status),
		UpdatedAt:      state.UpdatedAt,
	}
	metadata := membershipAdminDecodeJSONMap(state.MetadataJson)
	rawPayload := map[string]any{}
	if latestEvent, err := model.GetLatestChatMembershipEventByExternal(state.Source, state.RoomId, state.ExternalUserId); err == nil && latestEvent != nil {
		record.LatestEventType = strings.TrimSpace(latestEvent.EventType)
		record.LatestEventAt = latestEvent.EventAt
		rawPayload = membershipAdminDecodeJSONMap(latestEvent.RawPayloadJson)
	}
	candidateHints := membershipIdentityCandidates(ChatMembershipEventRequest{
		Source:         state.Source,
		RoomID:         state.RoomId,
		ExternalUserID: state.ExternalUserId,
		Metadata:       metadata,
		RawPayload:     rawPayload,
	}, membershipBindingSubject{})
	record.IdentityHints = candidateHints
	matchCandidates, ambiguousHints, err := buildMembershipUnresolvedIdentityCandidates(candidateHints)
	if err != nil {
		return record, err
	}
	record.MatchCandidates = matchCandidates
	existingBindings, err := buildMembershipUnresolvedBindingHints(state)
	if err != nil {
		return record, err
	}
	record.ExistingBindings = existingBindings
	record.SuggestedAction, record.Reason = chooseMembershipUnresolvedAction(matchCandidates, ambiguousHints, existingBindings, candidateHints)
	return record, nil
}

func membershipAdminDecodeJSONMap(raw string) map[string]any {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return map[string]any{}
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(raw), &out); err != nil || out == nil {
		return map[string]any{}
	}
	return out
}

func buildMembershipUnresolvedIdentityCandidates(candidates []string) ([]ChatMembershipUnresolvedIdentityCandidate, []string, error) {
	type accumulator struct {
		row       ChatMembershipUnresolvedIdentityCandidate
		matchedBy map[string]struct{}
	}
	byUserID := make(map[int]*accumulator)
	ambiguousHints := make([]string, 0)
	for _, candidate := range candidates {
		rows, err := model.ListActiveUserIdentityBindingsByUsername(AgentSiteID(), candidate, "community")
		if err != nil {
			return nil, nil, err
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
		if len(uniqueUsers) > 1 {
			ambiguousHints = append(ambiguousHints, candidate)
		}
		for userID, row := range uniqueUsers {
			acc, ok := byUserID[userID]
			if !ok {
				acc = &accumulator{
					row: ChatMembershipUnresolvedIdentityCandidate{
						UserID:                  row.UserId,
						Username:                strings.TrimSpace(row.Username),
						CommunityExternalUserID: strings.TrimSpace(row.ExternalUserId),
						MatchedBy:               strings.TrimSpace(candidate),
						MatchType:               "community_username",
					},
					matchedBy: map[string]struct{}{},
				}
				byUserID[userID] = acc
			}
			acc.matchedBy[strings.TrimSpace(candidate)] = struct{}{}
		}
	}
	result := make([]ChatMembershipUnresolvedIdentityCandidate, 0, len(byUserID))
	for _, acc := range byUserID {
		keys := make([]string, 0, len(acc.matchedBy))
		for key := range acc.matchedBy {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		acc.row.MatchedBy = strings.Join(keys, ", ")
		result = append(result, acc.row)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].UserID == result[j].UserID {
			return result[i].Username < result[j].Username
		}
		return result[i].UserID < result[j].UserID
	})
	sort.Strings(ambiguousHints)
	return result, ambiguousHints, nil
}

func buildMembershipUnresolvedBindingHints(state model.ChatMembershipState) ([]ChatMembershipUnresolvedBindingHint, error) {
	rows, err := model.ListAgentChatBindingsByExternal(AgentSiteID(), state.Source, state.ExternalUserId)
	if err != nil {
		return nil, err
	}
	out := make([]ChatMembershipUnresolvedBindingHint, 0, len(rows))
	for _, row := range rows {
		out = append(out, ChatMembershipUnresolvedBindingHint{
			Source:       strings.TrimSpace(row.Source),
			RoomID:       strings.TrimSpace(row.RoomId),
			NewAPIUserID: row.NewAPIUserId,
			Username:     strings.TrimSpace(row.Username),
			Enabled:      row.Enabled,
			Remark:       strings.TrimSpace(row.Remark),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].RoomID == out[j].RoomID {
			return out[i].NewAPIUserID < out[j].NewAPIUserID
		}
		return out[i].RoomID < out[j].RoomID
	})
	return out, nil
}

func chooseMembershipUnresolvedAction(
	matchCandidates []ChatMembershipUnresolvedIdentityCandidate,
	ambiguousHints []string,
	existingBindings []ChatMembershipUnresolvedBindingHint,
	identityHints []string,
) (string, string) {
	if len(existingBindings) > 0 {
		return "rerun_binding_backfill", "已存在同源绑定，但这条成员记录仍未挂到用户，先复查绑定/回填流水。"
	}
	if len(matchCandidates) == 1 {
		return "review_then_backfill", fmt.Sprintf("社区绑定唯一命中用户 #%d，确认后可回填成员资格。", matchCandidates[0].UserID)
	}
	if len(matchCandidates) > 1 {
		if len(ambiguousHints) > 0 {
			return "manual_pick_user", fmt.Sprintf("候选昵称 %s 命中多个社区账号，需要人工选定用户。", strings.Join(ambiguousHints, " / "))
		}
		return "manual_pick_user", "存在多个候选用户，需要人工确认再回填。"
	}
	if len(identityHints) > 0 {
		return "ask_user_to_finish_binding", fmt.Sprintf("已拿到昵称/卡片线索（%s），但还没有唯一社区绑定，需引导用户先完成绑定。", strings.Join(identityHints, " / "))
	}
	return "collect_identity_evidence", "当前只有外部 ID，没有足够线索自动挂用户，需要运营补充身份映射证据。"
}

func ActivateResolvedUnboundMembershipStates(limit int) (int64, error) {
	limit = normalizeChatMembershipBatchLimit(limit)
	states, err := model.ListChatMembershipStatesByStatusAndUserLink(model.ChatMembershipStatusUnboundObserved, true, limit)
	if err != nil {
		return 0, err
	}
	var activated int64
	now := time.Now().Unix()
	ctx := context.Background()
	for _, state := range states {
		if state.NewAPIUserId <= 0 {
			continue
		}
		updated, err := model.UpdateChatMembershipStateStatus(state.Id, model.ChatMembershipStatusActive, map[string]interface{}{
			"left_at":       int64(0),
			"grace_until":   int64(0),
			"restored_at":   now,
			"bypass_reason": "",
			"bypass_until":  int64(0),
			"metadata_json": buildMembershipAdminMetadata("backfill_activate", "resolved_unbound_membership", 0),
		})
		if err != nil {
			return activated, err
		}
		if _, err := model.UpsertAgentChatBindingFromMembership(
			AgentSiteID(),
			updated.Source,
			updated.RoomId,
			updated.ExternalUserId,
			updated.NewAPIUserId,
			"",
			"member",
			"membership_backfill",
		); err != nil {
			return activated, err
		}
		if _, err := model.SetAgentChatBindingsEnabled(AgentSiteID(), updated.Source, updated.RoomId, updated.ExternalUserId, true, updated.NewAPIUserId); err != nil {
			return activated, err
		}
		if updated.NewAPIUserId > 0 {
			_, _ = EvaluateUserAccessControl(ctx, updated.NewAPIUserId, true)
		}
		activated++
	}
	return activated, nil
}

func AdminBackfillHistoricalMembership(limit int) (*ChatMembershipBackfillResult, error) {
	limit = normalizeChatMembershipBatchLimit(limit)
	stateRows, eventRows, err := BackfillHistoricalMembershipUserLinks(limit)
	if err != nil {
		return nil, err
	}
	activatedStates, err := ActivateResolvedUnboundMembershipStates(limit)
	if err != nil {
		return nil, err
	}
	remainingUnresolved, err := model.CountChatMembershipStatesByStatusAndUserLink(model.ChatMembershipStatusUnboundObserved, false)
	if err != nil {
		return nil, err
	}
	common.SysLog(fmt.Sprintf(
		"[MembershipRisk] admin backfill unresolved limit=%d states=%d events=%d activated=%d remaining=%d",
		limit,
		stateRows,
		eventRows,
		activatedStates,
		remainingUnresolved,
	))
	return &ChatMembershipBackfillResult{
		Action:              "backfill",
		Limit:               limit,
		StateRows:           stateRows,
		EventRows:           eventRows,
		ActivatedStates:     activatedStates,
		RemainingUnresolved: remainingUnresolved,
	}, nil
}

func AdminDemoteUnresolvedActiveMembership(limit int) (*ChatMembershipDemoteResult, error) {
	limit = normalizeChatMembershipBatchLimit(limit)
	states, err := model.ListChatMembershipStatesByStatusAndUserLink(model.ChatMembershipStatusActive, false, limit)
	if err != nil {
		return nil, err
	}
	var demoted int64
	for _, state := range states {
		if _, err := model.UpdateChatMembershipStateStatus(state.Id, model.ChatMembershipStatusUnboundObserved, map[string]interface{}{
			"restored_at":   int64(0),
			"metadata_json": buildMembershipAdminMetadata("demote_unresolved_active", strings.TrimSpace(state.ExternalUserId), 0),
		}); err != nil {
			return nil, err
		}
		demoted++
	}
	remainingUnresolvedActive, err := model.CountChatMembershipStatesByStatusAndUserLink(model.ChatMembershipStatusActive, false)
	if err != nil {
		return nil, err
	}
	common.SysLog(fmt.Sprintf(
		"[MembershipRisk] admin demote unresolved active limit=%d demoted=%d remaining=%d",
		limit,
		demoted,
		remainingUnresolvedActive,
	))
	return &ChatMembershipDemoteResult{
		Action:                    "demote_unresolved",
		Limit:                     limit,
		DemotedStates:             demoted,
		RemainingUnresolvedActive: remainingUnresolvedActive,
	}, nil
}
