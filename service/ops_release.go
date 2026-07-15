package service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

type OpsReleasePublishRequest struct {
	ReleaseLabel string `json:"release_label"`
	Note         string `json:"note"`
}

type OpsReleaseRollbackRequest struct {
	ReleaseID int    `json:"release_id"`
	Note      string `json:"note"`
}

type opsReleasePayload struct {
	Version           int                        `json:"version"`
	SiteID            string                     `json:"site_id"`
	GeneratedAt       int64                      `json:"generated_at"`
	RawOptions        map[string]string          `json:"raw_options"`
	MissingOptionKeys []string                   `json:"missing_option_keys"`
	Groups            []model.ChatGroup          `json:"groups"`
	GroupChatOps      []model.GroupChatOpsConfig `json:"group_chatops_configs"`
	GroupGameConfigs  []model.GroupGameConfig    `json:"group_game_configs"`
	ControlPlane      *OpsControlPlaneSnapshot   `json:"control_plane"`
}

type OpsUnifiedAuditEvent struct {
	Id            string         `json:"id"`
	Domain        string         `json:"domain"`
	EventType     string         `json:"event_type"`
	Title         string         `json:"title"`
	Subject       string         `json:"subject"`
	Status        string         `json:"status"`
	Severity      string         `json:"severity"`
	ReasonCode    string         `json:"reason_code"`
	ReasonMessage string         `json:"reason_message"`
	Actor         string         `json:"actor"`
	ActorUserId   int            `json:"actor_user_id,omitempty"`
	UserId        int            `json:"user_id,omitempty"`
	RoomId        string         `json:"room_id,omitempty"`
	ProviderSlug  string         `json:"provider_slug,omitempty"`
	AccessLevel   string         `json:"access_level,omitempty"`
	At            int64          `json:"at"`
	Raw           map[string]any `json:"raw"`
}

type OpsUnifiedAuditSummary struct {
	TotalEvents         int            `json:"total_events"`
	CountsByDomain      map[string]int `json:"counts_by_domain"`
	DeniedGateCount     int            `json:"denied_gate_count"`
	ManualOverrideCount int            `json:"manual_override_count"`
	OpenRiskCount       int            `json:"open_risk_count"`
}

func GetOpsReleaseOverview(siteID string) (map[string]any, error) {
	siteID = resolveOpsSiteID(siteID)
	snapshot, err := BuildOpsControlPlaneSite(siteID)
	if err != nil {
		return nil, err
	}
	audits, err := model.ListAdminConfigAuditsBySite(siteID, 50)
	if err != nil {
		return nil, err
	}
	releases, err := model.ListOpsReleaseSnapshotsBySite(siteID, 12)
	if err != nil {
		return nil, err
	}
	scopeCounts := map[string]int{}
	latestChangeAt := int64(0)
	for _, audit := range audits {
		scope := strings.TrimSpace(audit.ConfigScope)
		if scope == "" {
			scope = "unknown"
		}
		scopeCounts[scope]++
		if audit.CreatedAt > latestChangeAt {
			latestChangeAt = audit.CreatedAt
		}
	}
	recentReleaseRows := make([]map[string]any, 0, len(releases))
	for _, row := range releases {
		recentReleaseRows = append(recentReleaseRows, opsReleaseSummary(&row))
		if row.AppliedAt > latestChangeAt {
			latestChangeAt = row.AppliedAt
		}
		if row.CreatedAt > latestChangeAt {
			latestChangeAt = row.CreatedAt
		}
	}
	releaseMode := "audit_only"
	if len(releases) > 0 {
		if strings.TrimSpace(strings.ToLower(releases[0].Action)) == "rollback" {
			releaseMode = "rolled_back"
		} else {
			releaseMode = "published"
		}
	}
	return map[string]any{
		"site_id":            siteID,
		"release_mode":       releaseMode,
		"publish_supported":  true,
		"rollback_supported": len(releases) > 0,
		"latest_change_at":   latestChangeAt,
		"audit_scope_counts": scopeCounts,
		"recent_audits":      audits,
		"recent_releases":    recentReleaseRows,
		"current_release": func() map[string]any {
			if len(releases) == 0 {
				return nil
			}
			return opsReleaseSummary(&releases[0])
		}(),
		"control_plane": snapshot,
		"generated_at":  time.Now().Unix(),
	}, nil
}

func GetOpsReleaseImpactPreview(siteID string) (map[string]any, error) {
	siteID = resolveOpsSiteID(siteID)
	currentPayload, currentHash, err := buildOpsReleasePayload(siteID)
	if err != nil {
		return nil, err
	}
	rows, err := model.ListOpsReleaseSnapshotsBySite(siteID, 1)
	if err != nil {
		return nil, err
	}
	var previousRelease *model.OpsReleaseSnapshot
	var previousPayload *opsReleasePayload
	if len(rows) > 0 {
		previousRelease = &rows[0]
		previousPayload, err = parseOpsReleasePayload(previousRelease.PayloadJson)
		if err != nil {
			return nil, err
		}
	}

	optionDiff := opsReleaseOptionDiff(
		currentPayload.RawOptions,
		opsReleasePayloadOptions(previousPayload),
	)
	groupDiff := opsReleaseStructuredDiff(
		opsReleaseGroupEntries(currentPayload.Groups),
		opsReleaseGroupEntries(opsReleasePayloadGroups(previousPayload)),
	)
	chatOpsDiff := opsReleaseStructuredDiff(
		opsReleaseChatOpsEntries(currentPayload.GroupChatOps),
		opsReleaseChatOpsEntries(opsReleasePayloadChatOps(previousPayload)),
	)
	gameDiff := opsReleaseStructuredDiff(
		opsReleaseGameEntries(currentPayload.GroupGameConfigs),
		opsReleaseGameEntries(opsReleasePayloadGameConfigs(previousPayload)),
	)

	previousHash := ""
	if previousRelease != nil {
		previousHash = strings.TrimSpace(strings.ToLower(previousRelease.SnapshotHash))
	}
	return map[string]any{
		"site_id":          siteID,
		"current_hash":     currentHash,
		"previous_hash":    previousHash,
		"current_state":    opsReleasePayloadSummary(currentPayload, currentHash),
		"previous_release": opsReleaseSummary(previousRelease),
		"has_changes":      previousHash == "" || currentHash != previousHash,
		"diff_summary": map[string]any{
			"option_changes":        opsReleaseChangeCount(optionDiff),
			"group_changes":         opsReleaseChangeCount(groupDiff),
			"group_chatops_changes": opsReleaseChangeCount(chatOpsDiff),
			"group_game_changes":    opsReleaseChangeCount(gameDiff),
		},
		"changes": map[string]any{
			"options":            optionDiff,
			"groups":             groupDiff,
			"group_chatops":      chatOpsDiff,
			"group_game_configs": gameDiff,
		},
		"generated_at": time.Now().Unix(),
	}, nil
}

func PublishOpsRelease(siteID string, actorUserID int, actorUsername string, req OpsReleasePublishRequest) (map[string]any, error) {
	siteID = resolveOpsSiteID(siteID)
	payload, hash, err := buildOpsReleasePayload(siteID)
	if err != nil {
		return nil, err
	}
	currentRows, err := model.ListOpsReleaseSnapshotsBySite(siteID, 1)
	if err != nil {
		return nil, err
	}
	row := &model.OpsReleaseSnapshot{
		SiteId:             siteID,
		Action:             "publish",
		ReleaseLabel:       firstReleaseNonEmpty(strings.TrimSpace(req.ReleaseLabel), buildOpsReleaseLabel(siteID, "publish")),
		ActorUserId:        actorUserID,
		ActorUsername:      strings.TrimSpace(actorUsername),
		Note:               strings.TrimSpace(req.Note),
		SnapshotHash:       hash,
		OptionCount:        len(payload.RawOptions),
		MissingOptionCount: len(payload.MissingOptionKeys),
		GroupCount:         len(payload.Groups),
		GroupChatOpsCount:  len(payload.GroupChatOps),
		GroupGameCount:     len(payload.GroupGameConfigs),
		PayloadJson:        mustMarshalOpsReleasePayload(payload),
		CreatedAt:          time.Now().Unix(),
		AppliedAt:          time.Now().Unix(),
	}
	if err := model.CreateOpsReleaseSnapshot(row); err != nil {
		return nil, err
	}
	_ = createOpsReleaseAudit(
		siteID,
		actorUserID,
		"ops_release.publish",
		fmt.Sprintf("release:%d", row.Id),
		opsReleaseAuditSnapshot(currentRows),
		opsReleaseSummary(row),
		firstReleaseNonEmpty(strings.TrimSpace(req.Note), row.ReleaseLabel),
	)
	return map[string]any{
		"site_id":      siteID,
		"release":      opsReleaseSummary(row),
		"generated_at": time.Now().Unix(),
	}, nil
}

func RollbackOpsRelease(siteID string, actorUserID int, actorUsername string, req OpsReleaseRollbackRequest) (map[string]any, error) {
	siteID = resolveOpsSiteID(siteID)
	if req.ReleaseID <= 0 {
		return nil, fmt.Errorf("release_id is required")
	}
	target, err := model.GetOpsReleaseSnapshotByID(siteID, req.ReleaseID)
	if err != nil {
		return nil, err
	}
	payload, err := parseOpsReleasePayload(target.PayloadJson)
	if err != nil {
		return nil, err
	}
	currentRows, err := model.ListOpsReleaseSnapshotsBySite(siteID, 1)
	if err != nil {
		return nil, err
	}
	rollbackRow := &model.OpsReleaseSnapshot{
		SiteId:             siteID,
		Action:             "rollback",
		ReleaseLabel:       buildOpsReleaseLabel(siteID, "rollback"),
		ActorUserId:        actorUserID,
		ActorUsername:      strings.TrimSpace(actorUsername),
		Note:               firstReleaseNonEmpty(strings.TrimSpace(req.Note), fmt.Sprintf("rollback to %s", target.ReleaseLabel)),
		SnapshotHash:       target.SnapshotHash,
		OptionCount:        target.OptionCount,
		MissingOptionCount: target.MissingOptionCount,
		GroupCount:         target.GroupCount,
		GroupChatOpsCount:  target.GroupChatOpsCount,
		GroupGameCount:     target.GroupGameCount,
		SourceReleaseId:    target.Id,
		PayloadJson:        target.PayloadJson,
		CreatedAt:          time.Now().Unix(),
		AppliedAt:          time.Now().Unix(),
	}
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		if err := restoreOpsReleasePayloadTx(tx, siteID, payload); err != nil {
			return err
		}
		if err := model.CreateOpsReleaseSnapshotWithDB(tx, rollbackRow); err != nil {
			return err
		}
		return createOpsReleaseAuditWithDB(
			tx,
			siteID,
			actorUserID,
			"ops_release.rollback",
			fmt.Sprintf("release:%d", target.Id),
			opsReleaseAuditSnapshot(currentRows),
			opsReleaseSummary(rollbackRow),
			rollbackRow.Note,
		)
	})
	if err != nil {
		return nil, err
	}
	model.ReloadOptionsFromDatabase()
	refreshedControlPlane, refreshErr := BuildOpsControlPlaneSite(siteID)
	if refreshErr != nil {
		return nil, refreshErr
	}
	return map[string]any{
		"site_id":       siteID,
		"target":        opsReleaseSummary(target),
		"release":       opsReleaseSummary(rollbackRow),
		"control_plane": refreshedControlPlane,
		"generated_at":  time.Now().Unix(),
	}, nil
}

func GetOpsAuditOverview(siteID string, limit int) (map[string]any, error) {
	siteID = resolveOpsSiteID(siteID)
	adminAudits, err := model.ListAdminConfigAuditsBySite(siteID, limit)
	if err != nil {
		return nil, err
	}
	communityAudits, err := model.ListCommunityGateAudits(limit)
	if err != nil {
		return nil, err
	}
	recentAccessStates, err := model.ListUserSiteAccessStates(siteID, limit)
	if err != nil {
		return nil, err
	}
	riskAudits, err := listRecentOpsAccountRiskAuditsBySite(siteID, limit)
	if err != nil {
		return nil, err
	}
	events, summary := buildUnifiedOpsAuditEvents(adminAudits, communityAudits, recentAccessStates, riskAudits)
	return map[string]any{
		"site_id":      siteID,
		"summary":      summary,
		"events":       events,
		"generated_at": time.Now().Unix(),
	}, nil
}

func listRecentOpsAccountRiskAuditsBySite(siteID string, limit int) ([]model.OpsAccountRiskAudit, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	var rows []model.OpsAccountRiskAudit
	err := model.DB.Where("site_id = ?", strings.TrimSpace(siteID)).
		Order("updated_at desc, id desc").
		Limit(limit).
		Find(&rows).Error
	return rows, err
}

func buildUnifiedOpsAuditEvents(
	adminAudits []model.AdminConfigAudit,
	communityAudits []model.CommunityGateAudit,
	accessStates []model.UserSiteAccessState,
	riskAudits []model.OpsAccountRiskAudit,
) ([]OpsUnifiedAuditEvent, OpsUnifiedAuditSummary) {
	events := make([]OpsUnifiedAuditEvent, 0, len(adminAudits)+len(communityAudits)+len(accessStates)+len(riskAudits))
	summary := OpsUnifiedAuditSummary{CountsByDomain: map[string]int{}}

	for _, audit := range adminAudits {
		scope := strings.TrimSpace(audit.ConfigScope)
		if scope == "" {
			scope = "config"
		}
		targetKey := strings.TrimSpace(audit.TargetKey)
		if targetKey == "" {
			targetKey = fmt.Sprintf("audit:%d", audit.Id)
		}
		events = append(events, OpsUnifiedAuditEvent{
			Id:            fmt.Sprintf("admin:%d", audit.Id),
			Domain:        "admin_config",
			EventType:     "config_change",
			Title:         scope,
			Subject:       targetKey,
			Status:        "changed",
			ReasonCode:    scope,
			ReasonMessage: strings.TrimSpace(audit.Reason),
			Actor:         fmt.Sprintf("UID %d", audit.ActorUserId),
			ActorUserId:   audit.ActorUserId,
			At:            audit.CreatedAt,
			Raw:           opsAuditRawRecord(audit),
		})
		summary.CountsByDomain["admin_config"]++
	}

	for _, audit := range communityAudits {
		status := "denied"
		if audit.Compliant {
			status = "compliant"
		}
		if !audit.Compliant {
			summary.DeniedGateCount++
		}
		events = append(events, OpsUnifiedAuditEvent{
			Id:            fmt.Sprintf("community:%d", audit.Id),
			Domain:        "community_gate",
			EventType:     "membership_gate",
			Title:         strings.TrimSpace(audit.ProviderSlug),
			Subject:       firstNonEmptyString(strings.TrimSpace(audit.Username), fmt.Sprintf("UID %d", audit.UserId)),
			Status:        status,
			ReasonCode:    strings.TrimSpace(audit.ReasonCode),
			ReasonMessage: strings.TrimSpace(audit.Reason),
			UserId:        audit.UserId,
			RoomId:        strings.TrimSpace(audit.RoomId),
			ProviderSlug:  strings.TrimSpace(audit.ProviderSlug),
			At:            audit.CheckedAt,
			Raw:           opsAuditRawRecord(audit),
		})
		summary.CountsByDomain["community_gate"]++
	}

	for _, row := range accessStates {
		overrideMode := strings.TrimSpace(row.ManualOverrideMode)
		if overrideMode != "" {
			summary.ManualOverrideCount++
		}
		reasonCode := strings.TrimSpace(row.ReasonCode)
		if reasonCode == "" {
			reasonCode = strings.TrimSpace(row.AccessLevel)
		}
		events = append(events, OpsUnifiedAuditEvent{
			Id:            fmt.Sprintf("access:%d", row.Id),
			Domain:        "access_control",
			EventType:     "access_state",
			Title:         firstNonEmptyString(strings.TrimSpace(row.AccessLevel), "access"),
			Subject:       fmt.Sprintf("UID %d", row.UserId),
			Status:        strings.TrimSpace(row.AccessLevel),
			ReasonCode:    reasonCode,
			ReasonMessage: strings.TrimSpace(row.ReasonMessage),
			UserId:        row.UserId,
			RoomId:        strings.TrimSpace(row.MatchedPrimaryGroupId),
			ProviderSlug:  strings.TrimSpace(row.PrimaryPlatform),
			AccessLevel:   strings.TrimSpace(row.AccessLevel),
			At:            maxInt64(row.UpdatedAt, row.LastEvaluatedAt),
			Raw:           opsAuditRawRecord(row),
		})
		summary.CountsByDomain["access_control"]++
	}

	for _, row := range riskAudits {
		status := strings.TrimSpace(row.Status)
		if status == "" {
			status = OpsRiskStatusOpen
		}
		if status == OpsRiskStatusOpen {
			summary.OpenRiskCount++
		}
		events = append(events, OpsUnifiedAuditEvent{
			Id:            fmt.Sprintf("risk:%d", row.Id),
			Domain:        "risk_control",
			EventType:     "risk_audit",
			Title:         strings.TrimSpace(row.RiskType),
			Subject:       strings.TrimSpace(row.Subject),
			Status:        status,
			Severity:      strings.TrimSpace(row.Severity),
			ReasonCode:    strings.TrimSpace(row.RiskType),
			ReasonMessage: strings.TrimSpace(row.Ip),
			At:            maxInt64(row.UpdatedAt, row.CreatedAt),
			Raw:           opsAuditRawRecord(row),
		})
		summary.CountsByDomain["risk_control"]++
	}

	sort.SliceStable(events, func(i, j int) bool {
		if events[i].At == events[j].At {
			return events[i].Id > events[j].Id
		}
		return events[i].At > events[j].At
	})
	summary.TotalEvents = len(events)
	return events, summary
}

func opsAuditRawRecord(value any) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return map[string]any{}
	}
	out := map[string]any{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return map[string]any{}
	}
	return out
}

func maxInt64(values ...int64) int64 {
	var out int64
	for _, value := range values {
		if value > out {
			out = value
		}
	}
	return out
}

func buildOpsReleasePayload(siteID string) (*opsReleasePayload, string, error) {
	controlPlane, err := BuildOpsControlPlaneSite(siteID)
	if err != nil {
		return nil, "", err
	}
	rawOptions, err := loadOpsOptionValues(opsControlPlaneKnownKeys)
	if err != nil {
		return nil, "", err
	}
	missingOptionKeys := make([]string, 0)
	for _, key := range opsControlPlaneKnownKeys {
		if _, ok := rawOptions[key]; !ok {
			missingOptionKeys = append(missingOptionKeys, key)
		}
	}
	sort.Strings(missingOptionKeys)

	groups, err := model.ListChatGroupsBySite(siteID, "", "", "")
	if err != nil {
		return nil, "", err
	}
	groupChatOps, err := model.ListGroupChatOpsConfigsBySite(siteID)
	if err != nil {
		return nil, "", err
	}
	groupGames, err := model.ListGroupGameConfigsBySite(siteID)
	if err != nil {
		return nil, "", err
	}
	payload := &opsReleasePayload{
		Version:           1,
		SiteID:            siteID,
		GeneratedAt:       time.Now().Unix(),
		RawOptions:        rawOptions,
		MissingOptionKeys: missingOptionKeys,
		Groups:            groups,
		GroupChatOps:      groupChatOps,
		GroupGameConfigs:  groupGames,
		ControlPlane:      controlPlane,
	}
	sum := sha256.Sum256([]byte(mustMarshalOpsReleasePayload(payload)))
	return payload, hex.EncodeToString(sum[:]), nil
}

func parseOpsReleasePayload(raw string) (*opsReleasePayload, error) {
	var payload opsReleasePayload
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil, fmt.Errorf("invalid release payload: %w", err)
	}
	if payload.Version == 0 {
		payload.Version = 1
	}
	return &payload, nil
}

func opsReleasePayloadSummary(payload *opsReleasePayload, hash string) map[string]any {
	if payload == nil {
		return map[string]any{}
	}
	return map[string]any{
		"site_id":              payload.SiteID,
		"hash":                 hash,
		"generated_at":         payload.GeneratedAt,
		"option_count":         len(payload.RawOptions),
		"missing_option_count": len(payload.MissingOptionKeys),
		"group_count":          len(payload.Groups),
		"group_chatops_count":  len(payload.GroupChatOps),
		"group_game_count":     len(payload.GroupGameConfigs),
	}
}

func opsReleasePayloadOptions(payload *opsReleasePayload) map[string]string {
	if payload == nil || payload.RawOptions == nil {
		return map[string]string{}
	}
	return payload.RawOptions
}

func opsReleasePayloadGroups(payload *opsReleasePayload) []model.ChatGroup {
	if payload == nil {
		return nil
	}
	return payload.Groups
}

func opsReleasePayloadChatOps(payload *opsReleasePayload) []model.GroupChatOpsConfig {
	if payload == nil {
		return nil
	}
	return payload.GroupChatOps
}

func opsReleasePayloadGameConfigs(payload *opsReleasePayload) []model.GroupGameConfig {
	if payload == nil {
		return nil
	}
	return payload.GroupGameConfigs
}

func opsReleaseOptionDiff(current map[string]string, previous map[string]string) map[string]any {
	added := make([]string, 0)
	updated := make([]string, 0)
	removed := make([]string, 0)

	keys := make(map[string]struct{}, len(current)+len(previous))
	for key := range current {
		keys[key] = struct{}{}
	}
	for key := range previous {
		keys[key] = struct{}{}
	}
	sortedKeys := make([]string, 0, len(keys))
	for key := range keys {
		sortedKeys = append(sortedKeys, key)
	}
	sort.Strings(sortedKeys)
	for _, key := range sortedKeys {
		currentValue, currentOK := current[key]
		previousValue, previousOK := previous[key]
		switch {
		case currentOK && !previousOK:
			added = append(added, key)
		case !currentOK && previousOK:
			removed = append(removed, key)
		case currentValue != previousValue:
			updated = append(updated, key)
		}
	}
	return map[string]any{
		"added_count":   len(added),
		"updated_count": len(updated),
		"removed_count": len(removed),
		"added_keys":    opsReleaseLimitStrings(added, 12),
		"updated_keys":  opsReleaseLimitStrings(updated, 12),
		"removed_keys":  opsReleaseLimitStrings(removed, 12),
	}
}

func opsReleaseGroupEntries(rows []model.ChatGroup) map[string]map[string]any {
	out := make(map[string]map[string]any, len(rows))
	for _, row := range rows {
		key := strings.TrimSpace(row.Platform) + "|" + strings.TrimSpace(row.GroupId)
		config := opsJSONMap(row.ConfigJson)
		out[key] = map[string]any{
			"platform":               row.Platform,
			"group_id":               row.GroupId,
			"group_name":             strings.TrimSpace(row.GroupName),
			"invite_target_group_id": opsGroupInviteTargetGroupID(&row, config),
			"role":                   strings.TrimSpace(row.Role),
			"status":                 strings.TrimSpace(row.Status),
			"language":               strings.TrimSpace(row.Language),
			"timezone":               strings.TrimSpace(row.Timezone),
			"config_json":            strings.TrimSpace(row.ConfigJson),
		}
	}
	return out
}

func opsReleaseChatOpsEntries(rows []model.GroupChatOpsConfig) map[string]map[string]any {
	out := make(map[string]map[string]any, len(rows))
	for _, row := range rows {
		key := strings.TrimSpace(row.Platform) + "|" + strings.TrimSpace(row.GroupId)
		out[key] = map[string]any{
			"platform":                 row.Platform,
			"group_id":                 row.GroupId,
			"checkin_enabled":          row.CheckinEnabled,
			"verify_enabled":           row.VerifyEnabled,
			"invite_enabled":           row.InviteEnabled,
			"checkin_quota":            row.CheckinQuota,
			"verify_min_quota":         row.VerifyMinQuota,
			"invite_reward_quota":      row.InviteRewardQuota,
			"invitee_reward_quota":     row.InviteeRewardQuota,
			"daily_group_reward_limit": row.DailyGroupRewardLimit,
			"rule_json":                strings.TrimSpace(row.RuleJson),
		}
	}
	return out
}

func opsReleaseGameEntries(rows []model.GroupGameConfig) map[string]map[string]any {
	out := make(map[string]map[string]any, len(rows))
	for _, row := range rows {
		key := strings.TrimSpace(row.Platform) + "|" + strings.TrimSpace(row.GroupId) + "|" + strings.TrimSpace(row.GameCode)
		out[key] = map[string]any{
			"platform":    row.Platform,
			"group_id":    row.GroupId,
			"game_code":   row.GameCode,
			"enabled":     row.Enabled,
			"budget_pool": strings.TrimSpace(row.BudgetPool),
			"rule_json":   strings.TrimSpace(row.RuleJson),
		}
	}
	return out
}

func opsReleaseStructuredDiff(current map[string]map[string]any, previous map[string]map[string]any) map[string]any {
	added := make([]map[string]any, 0)
	updated := make([]map[string]any, 0)
	removed := make([]map[string]any, 0)

	keys := make(map[string]struct{}, len(current)+len(previous))
	for key := range current {
		keys[key] = struct{}{}
	}
	for key := range previous {
		keys[key] = struct{}{}
	}
	sortedKeys := make([]string, 0, len(keys))
	for key := range keys {
		sortedKeys = append(sortedKeys, key)
	}
	sort.Strings(sortedKeys)
	for _, key := range sortedKeys {
		currentValue, currentOK := current[key]
		previousValue, previousOK := previous[key]
		switch {
		case currentOK && !previousOK:
			added = append(added, currentValue)
		case !currentOK && previousOK:
			removed = append(removed, previousValue)
		case currentOK && previousOK && opsReleaseComparableJSON(currentValue) != opsReleaseComparableJSON(previousValue):
			updated = append(updated, map[string]any{
				"key":    key,
				"before": previousValue,
				"after":  currentValue,
			})
		}
	}
	return map[string]any{
		"added_count":   len(added),
		"updated_count": len(updated),
		"removed_count": len(removed),
		"added":         opsReleaseLimitMaps(added, 8),
		"updated":       opsReleaseLimitMaps(updated, 8),
		"removed":       opsReleaseLimitMaps(removed, 8),
	}
}

func opsReleaseComparableJSON(value any) string {
	raw, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(raw)
}

func opsReleaseLimitStrings(values []string, limit int) []string {
	if len(values) <= limit {
		return values
	}
	return values[:limit]
}

func opsReleaseLimitMaps(values []map[string]any, limit int) []map[string]any {
	if len(values) <= limit {
		return values
	}
	return values[:limit]
}

func opsReleaseChangeCount(diff map[string]any) int {
	return opsIntValue(diff["added_count"]) + opsIntValue(diff["updated_count"]) + opsIntValue(diff["removed_count"])
}

func opsIntValue(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	default:
		return 0
	}
}

func mustMarshalOpsReleasePayload(payload *opsReleasePayload) string {
	if payload == nil {
		return "{}"
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "{}"
	}
	return string(raw)
}

func opsReleaseSummary(row *model.OpsReleaseSnapshot) map[string]any {
	if row == nil {
		return nil
	}
	return map[string]any{
		"id":                   row.Id,
		"site_id":              row.SiteId,
		"action":               row.Action,
		"release_label":        row.ReleaseLabel,
		"actor_user_id":        row.ActorUserId,
		"actor_username":       row.ActorUsername,
		"note":                 row.Note,
		"snapshot_hash":        row.SnapshotHash,
		"option_count":         row.OptionCount,
		"missing_option_count": row.MissingOptionCount,
		"group_count":          row.GroupCount,
		"group_chatops_count":  row.GroupChatOpsCount,
		"group_game_count":     row.GroupGameCount,
		"source_release_id":    row.SourceReleaseId,
		"created_at":           row.CreatedAt,
		"applied_at":           row.AppliedAt,
	}
}

func opsReleaseAuditSnapshot(rows []model.OpsReleaseSnapshot) map[string]any {
	if len(rows) == 0 {
		return map[string]any{}
	}
	return opsReleaseSummary(&rows[0])
}

func createOpsReleaseAudit(siteID string, actorUserID int, scope string, targetKey string, before map[string]any, after map[string]any, reason string) error {
	if model.DB == nil {
		return nil
	}
	return createOpsReleaseAuditWithDB(model.DB, siteID, actorUserID, scope, targetKey, before, after, reason)
}

func createOpsReleaseAuditWithDB(tx *gorm.DB, siteID string, actorUserID int, scope string, targetKey string, before map[string]any, after map[string]any, reason string) error {
	if tx == nil {
		return gorm.ErrInvalidDB
	}
	return tx.Create(&model.AdminConfigAudit{
		SiteId:      strings.TrimSpace(siteID),
		ActorUserId: actorUserID,
		ConfigScope: strings.TrimSpace(scope),
		TargetKey:   strings.TrimSpace(targetKey),
		BeforeJson:  mustMarshalOpsAuditJSON(before),
		AfterJson:   mustMarshalOpsAuditJSON(after),
		Reason:      strings.TrimSpace(reason),
		CreatedAt:   time.Now().Unix(),
	}).Error
}

func mustMarshalOpsAuditJSON(value map[string]any) string {
	if value == nil {
		return "{}"
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(raw)
}

func buildOpsReleaseLabel(siteID string, action string) string {
	return fmt.Sprintf("%s-%s-%s", siteID, strings.TrimSpace(action), time.Now().Format("20060102-150405"))
}

func firstReleaseNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func restoreOpsReleasePayloadTx(tx *gorm.DB, siteID string, payload *opsReleasePayload) error {
	if tx == nil {
		return gorm.ErrInvalidDB
	}
	if payload == nil {
		return fmt.Errorf("release payload is nil")
	}
	if err := model.ReplaceOptionsBulkTx(tx, payload.RawOptions, payload.MissingOptionKeys); err != nil {
		return err
	}
	canonicalSiteID := model.CanonicalSiteID(siteID)
	if err := tx.Where("site_id = ?", canonicalSiteID).Delete(&model.GroupGameConfig{}).Error; err != nil {
		return err
	}
	if err := tx.Where("site_id = ?", canonicalSiteID).Delete(&model.GroupChatOpsConfig{}).Error; err != nil {
		return err
	}
	if err := tx.Where("site_id = ?", canonicalSiteID).Delete(&model.ChatGroup{}).Error; err != nil {
		return err
	}
	for _, row := range payload.Groups {
		cloned := row
		cloned.SiteId = canonicalSiteID
		if err := model.InsertChatGroupSnapshotWithDB(tx, &cloned); err != nil {
			return err
		}
	}
	for _, row := range payload.GroupChatOps {
		cloned := row
		cloned.SiteId = canonicalSiteID
		if err := model.InsertGroupChatOpsSnapshotWithDB(tx, &cloned); err != nil {
			return err
		}
	}
	for _, row := range payload.GroupGameConfigs {
		cloned := row
		cloned.SiteId = canonicalSiteID
		if err := model.InsertGroupGameSnapshotWithDB(tx, &cloned); err != nil {
			return err
		}
	}
	return nil
}
