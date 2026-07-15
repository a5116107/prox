package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

type AdminUserOpsGridFilters struct {
	Keyword          string
	Group            string
	Role             *int
	Status           *int
	AccessLevel      string
	CommunityBound   *bool
	HasCommunityRoom *bool
	QQBound          *bool
	TGBound          *bool
	PrimaryBound     *bool
	HasFrozenKeys    *bool
	OverrideMode     string
	Page             int
	PageSize         int
}

func ListAdminUserOpsGrid(ctx context.Context, siteID string, filters AdminUserOpsGridFilters) ([]map[string]any, int64, error) {
	siteID = resolveOpsSiteID(siteID)
	query := model.DB.Model(&model.User{}).
		Unscoped().
		Joins("LEFT JOIN user_site_access_states usas ON usas.site_id = ? AND usas.user_id = users.id", siteID)

	if keyword := strings.TrimSpace(filters.Keyword); keyword != "" {
		likePattern := "%" + keyword + "%"
		if keywordID, err := strconv.Atoi(keyword); err == nil {
			query = query.Where("(users.id = ? OR users.username LIKE ? OR users.email LIKE ? OR users.display_name LIKE ?)", keywordID, likePattern, likePattern, likePattern)
		} else {
			query = query.Where("(users.username LIKE ? OR users.email LIKE ? OR users.display_name LIKE ?)", likePattern, likePattern, likePattern)
		}
	}
	if group := strings.TrimSpace(filters.Group); group != "" {
		query = query.Where("users.`group` = ?", group)
	}
	if filters.Role != nil {
		query = query.Where("users.role = ?", *filters.Role)
	}
	if filters.Status != nil {
		if *filters.Status == -1 {
			query = query.Where("users.deleted_at IS NOT NULL")
		} else {
			query = query.Where("users.deleted_at IS NULL").Where("users.status = ?", *filters.Status)
		}
	} else {
		query = query.Where("users.deleted_at IS NULL")
	}
	if accessLevel := strings.TrimSpace(filters.AccessLevel); accessLevel != "" {
		query = query.Where("COALESCE(usas.access_level, '') = ?", accessLevel)
	}
	if filters.CommunityBound != nil {
		query = query.Where("COALESCE(usas.community_bound, false) = ?", *filters.CommunityBound)
	}
	if filters.HasCommunityRoom != nil {
		query = query.Where("COALESCE(usas.has_room_membership, false) = ?", *filters.HasCommunityRoom)
	}
	if filters.PrimaryBound != nil {
		query = query.Where("COALESCE(usas.primary_bound, false) = ?", *filters.PrimaryBound)
	}
	if overrideMode := strings.TrimSpace(filters.OverrideMode); overrideMode != "" {
		if overrideMode == "none" {
			query = query.Where("COALESCE(usas.manual_override_mode, '') = ''")
		} else {
			query = query.Where("COALESCE(usas.manual_override_mode, '') = ?", overrideMode)
		}
	}
	if filters.QQBound != nil {
		query = applyExistsFilter(query, *filters.QQBound, model.DB.Model(&model.AgentChatBinding{}).
			Select("1").
			Where("site_id = ? AND new_api_user_id = users.id AND source = ? AND enabled = ?", siteID, "qq", true))
	}
	if filters.TGBound != nil {
		query = applyExistsFilter(query, *filters.TGBound, model.DB.Model(&model.AgentChatBinding{}).
			Select("1").
			Where("site_id = ? AND new_api_user_id = users.id AND source = ? AND enabled = ?", siteID, "tg", true))
	}
	if filters.HasFrozenKeys != nil {
		freezeQuery := model.DB.Model(&model.CommunityGateTokenFreeze{}).
			Select("1").
			Where("user_id = users.id AND freeze_key IN ? AND restored_at = 0", []string{"community_gate", model.AccessControlFreezeKey})
		riskQuery := model.DB.Model(&model.RiskUserControl{}).
			Select("1").
			Where("site_id = ? AND user_id = users.id AND enabled = ?", siteID, true)
		if *filters.HasFrozenKeys {
			query = query.Where("EXISTS (?) OR EXISTS (?)", freezeQuery, riskQuery)
		} else {
			query = query.Where("NOT EXISTS (?) AND NOT EXISTS (?)", freezeQuery, riskQuery)
		}
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	page := filters.Page
	if page <= 0 {
		page = 1
	}
	pageSize := filters.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	var users []model.User
	if err := query.Select("users.*").Order("users.id desc").Limit(pageSize).Offset((page - 1) * pageSize).Find(&users).Error; err != nil {
		return nil, 0, err
	}
	userIDs := make([]int, 0, len(users))
	for i := range users {
		if users[i].Id > 0 {
			userIDs = append(userIDs, users[i].Id)
		}
	}
	snapshotMap, err := loadAdminUserOpsSnapshotMap(siteID, userIDs)
	if err != nil {
		return nil, 0, err
	}
	accessUpdatedAtMap, err := loadUserSiteAccessUpdatedAtMap(siteID, userIDs)
	if err != nil {
		return nil, 0, err
	}
	bindingUpdatedAtMap, err := loadAdminUserBindingUpdatedAtMap(siteID, userIDs)
	if err != nil {
		return nil, 0, err
	}
	accessFrozenMap, err := loadAdminActiveFreezeCountMap(userIDs, model.AccessControlFreezeKey)
	if err != nil {
		return nil, 0, err
	}
	communityFrozenMap, err := loadAdminActiveFreezeCountMap(userIDs, "community_gate")
	if err != nil {
		return nil, 0, err
	}
	riskRestoreMap, err := loadAdminRiskRestoreStateMap(siteID, userIDs)
	if err != nil {
		return nil, 0, err
	}
	profiles := make([]map[string]any, 0, len(users))
	for i := range users {
		var (
			profile map[string]any
			err     error
		)
		if snapshot := snapshotMap[users[i].Id]; adminUserOpsGridSnapshotUsable(snapshot, &users[i], accessUpdatedAtMap[users[i].Id], bindingUpdatedAtMap[users[i].Id]) {
			profile, err = adminUserOpsProfileFromSnapshot(snapshot)
		} else {
			profile, err = buildAndPersistAdminUserOpsProfile(ctx, siteID, &users[i], false)
		}
		if err != nil {
			return nil, 0, err
		}
		mergeAdminUserOpsGridUserFields(profile, siteID, &users[i])
		profiles = append(profiles, profile)
	}
	mergeAdminUserOpsGridRestoreState(profiles, accessFrozenMap, communityFrozenMap, riskRestoreMap)
	if err := attachAdminUserRelationshipFields(siteID, profiles); err != nil {
		return nil, 0, err
	}
	items := make([]map[string]any, 0, len(profiles))
	for _, profile := range profiles {
		items = append(items, adminUserOpsGridItem(profile))
	}
	return items, total, nil
}

func GetAdminUserOpsProfile(ctx context.Context, siteID string, userID int, refresh bool) (map[string]any, error) {
	user, err := model.GetUserById(userID, false)
	if err != nil {
		return nil, err
	}
	return BuildAdminUserOpsProfile(ctx, resolveOpsSiteID(siteID), user, refresh)
}

func BuildAdminUserOpsProfile(ctx context.Context, siteID string, user *model.User, refresh bool) (map[string]any, error) {
	if user == nil {
		return nil, gorm.ErrRecordNotFound
	}
	siteID = resolveOpsSiteID(siteID)
	if !refresh {
		accessUpdatedAtMap, err := loadUserSiteAccessUpdatedAtMap(siteID, []int{user.Id})
		if err == nil {
			bindingUpdatedAtMap, bindingErr := loadAdminUserBindingUpdatedAtMap(siteID, []int{user.Id})
			if bindingErr == nil {
				if snapshot, snapshotErr := model.GetUserOpsProfileSnapshot(siteID, user.Id); snapshotErr == nil && adminUserOpsSnapshotFresh(snapshot, user, accessUpdatedAtMap[user.Id], bindingUpdatedAtMap[user.Id]) {
					if profile, decodeErr := adminUserOpsProfileFromSnapshot(snapshot); decodeErr == nil {
						if restoreErr := refreshAdminUserRestoreState(siteID, []map[string]any{profile}); restoreErr != nil {
							return nil, restoreErr
						}
						return profile, nil
					}
				}
			}
		}
	}
	return buildAndPersistAdminUserOpsProfile(ctx, siteID, user, refresh)
}

func buildAndPersistAdminUserOpsProfile(ctx context.Context, siteID string, user *model.User, refresh bool) (map[string]any, error) {
	profile, err := buildAdminUserOpsProfileLive(ctx, siteID, user, refresh)
	if err != nil {
		return nil, err
	}
	if err := attachAdminUserRelationshipFields(siteID, []map[string]any{profile}); err != nil {
		return nil, err
	}
	if err := persistAdminUserOpsProfileSnapshot(siteID, profile); err != nil {
		return nil, err
	}
	return profile, nil
}

func buildAdminUserOpsProfileLive(ctx context.Context, siteID string, user *model.User, refresh bool) (map[string]any, error) {
	if user == nil {
		return nil, gorm.ErrRecordNotFound
	}
	siteID = resolveOpsSiteID(siteID)
	accessStatus, accessErr := GetUserAccessControlStatus(ctx, user.Id, refresh)
	if accessErr != nil {
		return nil, accessErr
	}
	communityStatus, communityErr := GetCommunityGateUserStatus(ctx, user.Id, refresh)
	if communityErr != nil {
		return nil, communityErr
	}
	accessState, _ := accessStatus["state"].(*model.UserSiteAccessState)
	identityBindings, agentBindings, membershipStates, err := loadAdminUserBindings(siteID, user.Id)
	if err != nil {
		return nil, err
	}
	communityBinding := pickLatestIdentityBinding(identityBindings, "community")
	qqIdentityBinding := pickLatestIdentityBinding(identityBindings, "qq")
	tgIdentityBinding := pickLatestIdentityBinding(identityBindings, "tg")
	qqBinding := pickLatestAgentBinding(agentBindings, "qq")
	tgBinding := pickLatestAgentBinding(agentBindings, "tg")
	communityRoomIDs := membershipRoomIDs(membershipStates, "community")
	qqRoomIDs := append(uniqueRoomIDsFromAgentBindings(agentBindings, "qq"), membershipRoomIDs(membershipStates, "qq")...)
	tgRoomIDs := append(uniqueRoomIDsFromAgentBindings(agentBindings, "tg"), membershipRoomIDs(membershipStates, "tg")...)
	qqRoomIDs = uniqueStrings(qqRoomIDs)
	tgRoomIDs = uniqueStrings(tgRoomIDs)
	activeAccessFrozen, _ := model.CountActiveAccessControlFreezes(user.Id)
	activeCommunityFrozen, _ := model.CountActiveCommunityGateFreezes(user.Id)
	riskRestoreMap, err := loadAdminRiskRestoreStateMap(siteID, []int{user.Id})
	if err != nil {
		return nil, err
	}
	riskRestore := riskRestoreMap[user.Id]
	restorableCount := int64(activeAccessFrozen) + int64(activeCommunityFrozen) + riskRestore.RestorableTokenCount
	effectiveGroups := toStringSlice(accessStatus["effective_groups"])
	matchedPrimaryGroupID := ""
	if accessState != nil {
		matchedPrimaryGroupID = accessState.MatchedPrimaryGroupId
	}
	profile := map[string]any{
		"site_id":           siteID,
		"user_id":           user.Id,
		"username":          user.Username,
		"display_name":      user.DisplayName,
		"email":             user.Email,
		"base_group":        user.Group,
		"role":              user.Role,
		"status":            user.Status,
		"quota":             user.Quota,
		"used_quota":        user.UsedQuota,
		"request_count":     user.RequestCount,
		"last_login_at":     user.LastLoginAt,
		"created_at":        user.CreatedAt,
		"inviter_id":        user.InviterId,
		"aff_count":         user.AffCount,
		"aff_quota":         user.AffQuota,
		"aff_history_quota": user.AffHistoryQuota,
		"access_level":      accessStatus["access_level"],
		"effective_groups":  effectiveGroups,
		"reason_code":       accessStatus["reason_code"],
		"reason_message":    accessStatus["reason_message"],
		"manual_override_mode": firstAnyString(accessStatus["state"], func(v any) string {
			if s, ok := v.(*model.UserSiteAccessState); ok && s != nil {
				return s.ManualOverrideMode
			}
			return ""
		}),
		"manual_override_reason": firstAnyString(accessStatus["state"], func(v any) string {
			if s, ok := v.(*model.UserSiteAccessState); ok && s != nil {
				return s.ManualOverrideReason
			}
			return ""
		}),
		"manual_override_groups": func() []string {
			if accessState != nil {
				return accessState.ManualOverrideGroupList()
			}
			return []string{}
		}(),
		"community_bound":               toBool(accessStatus["community_bound"]),
		"has_community_oauth_binding":   toBool(accessStatus["has_oauth_binding"]),
		"has_community_room_membership": toBool(accessStatus["has_room_membership"]),
		"community_room_ids":            uniqueStrings(communityRoomIDs),
		"community_external_user_id":    bindingExternalUserID(communityBinding),
		"community_username":            bindingUsername(communityBinding),
		"qq_bound":                      len(qqRoomIDs) > 0 || qqBinding != nil || qqIdentityBinding != nil,
		"qq_external_user_id":           preferredBindingExternalUserID(qqBinding, qqIdentityBinding),
		"qq_username":                   preferredBindingUsername(qqBinding, qqIdentityBinding),
		"qq_bound_group_ids":            qqRoomIDs,
		"tg_bound":                      len(tgRoomIDs) > 0 || tgBinding != nil || tgIdentityBinding != nil,
		"tg_external_user_id":           preferredBindingExternalUserID(tgBinding, tgIdentityBinding),
		"tg_username":                   preferredBindingUsername(tgBinding, tgIdentityBinding),
		"tg_bound_group_ids":            tgRoomIDs,
		"primary_bound":                 toBool(accessStatus["primary_bound"]),
		"primary_platform":              anyString(accessStatus["primary_platform"]),
		"matched_primary_group_id":      matchedPrimaryGroupID,
		"active_frozen_key_count":       restorableCount,
		"access_control_frozen_keys":    activeAccessFrozen,
		"community_gate_frozen_keys":    activeCommunityFrozen,
		"risk_controlled_keys":          riskRestore.RestorableTokenCount,
		"has_active_risk_control":       riskRestore.ActiveControl,
		"has_active_frozen_keys":        restorableCount > 0,
		"can_restore":                   restorableCount > 0 || riskRestore.ActiveControl,
		"access_control_status":         accessStatus,
		"community_gate_status":         communityStatus,
		"identity_bindings":             identityBindings,
		"agent_chat_bindings":           agentBindings,
		"chat_membership_states":        membershipStates,
	}
	return profile, nil
}

func persistAdminUserOpsProfileSnapshot(siteID string, profile map[string]any) error {
	if len(profile) == 0 {
		return nil
	}
	now := time.Now().Unix()
	profile["profile_synced_at"] = now
	row := &model.UserOpsProfileSnapshot{
		SiteId:                     resolveOpsSiteID(firstNonEmptyString(anyString(profile["site_id"]), siteID)),
		UserId:                     anyInt(profile["user_id"]),
		Username:                   anyString(profile["username"]),
		DisplayName:                anyString(profile["display_name"]),
		Email:                      anyString(profile["email"]),
		BaseGroup:                  anyString(profile["base_group"]),
		Role:                       anyInt(profile["role"]),
		Status:                     anyInt(profile["status"]),
		Quota:                      anyInt(profile["quota"]),
		UsedQuota:                  anyInt(profile["used_quota"]),
		RequestCount:               anyInt(profile["request_count"]),
		LastLoginAt:                anyInt64(profile["last_login_at"]),
		CreatedAtUnix:              anyInt64(profile["created_at"]),
		InviterId:                  anyInt(profile["inviter_id"]),
		InviterUsername:            anyString(profile["inviter_username"]),
		InviterDisplayName:         anyString(profile["inviter_display_name"]),
		InviteeCount:               anyInt(profile["invitee_count"]),
		AffCount:                   anyInt(profile["aff_count"]),
		AffQuota:                   anyInt(profile["aff_quota"]),
		AffHistoryQuota:            anyInt(profile["aff_history_quota"]),
		CommunityBound:             toBool(profile["community_bound"]),
		HasCommunityOAuthBinding:   toBool(profile["has_community_oauth_binding"]),
		HasCommunityRoomMembership: toBool(profile["has_community_room_membership"]),
		CommunityRoomIds:           adminUserOpsMarshalJSON(toStringSlice(profile["community_room_ids"]), "[]"),
		CommunityExternalUserId:    anyString(profile["community_external_user_id"]),
		CommunityUsername:          anyString(profile["community_username"]),
		QQBound:                    toBool(profile["qq_bound"]),
		QQExternalUserId:           anyString(profile["qq_external_user_id"]),
		QQUsername:                 anyString(profile["qq_username"]),
		QQBoundGroupIds:            adminUserOpsMarshalJSON(toStringSlice(profile["qq_bound_group_ids"]), "[]"),
		TGBound:                    toBool(profile["tg_bound"]),
		TGExternalUserId:           anyString(profile["tg_external_user_id"]),
		TGUsername:                 anyString(profile["tg_username"]),
		TGBoundGroupIds:            adminUserOpsMarshalJSON(toStringSlice(profile["tg_bound_group_ids"]), "[]"),
		AccessLevel:                anyString(profile["access_level"]),
		EffectiveGroups:            adminUserOpsMarshalJSON(toStringSlice(profile["effective_groups"]), "[]"),
		ReasonCode:                 anyString(profile["reason_code"]),
		ReasonMessage:              anyString(profile["reason_message"]),
		PrimaryBound:               toBool(profile["primary_bound"]),
		PrimaryPlatform:            anyString(profile["primary_platform"]),
		MatchedPrimaryGroupId:      anyString(profile["matched_primary_group_id"]),
		ManualOverrideMode:         anyString(profile["manual_override_mode"]),
		ManualOverrideGroups:       adminUserOpsMarshalJSON(toStringSlice(profile["manual_override_groups"]), "[]"),
		ManualOverrideReason:       anyString(profile["manual_override_reason"]),
		ActiveFrozenKeyCount:       anyInt(profile["active_frozen_key_count"]),
		AccessControlFrozenKeys:    anyInt(profile["access_control_frozen_keys"]),
		CommunityGateFrozenKeys:    anyInt(profile["community_gate_frozen_keys"]),
		HasActiveFrozenKeys:        toBool(profile["has_active_frozen_keys"]),
		CanRestore:                 toBool(profile["can_restore"]),
		CommunitySiteId:            anyString(profile["community_site_id"]),
		LastLoginIP:                anyString(profile["last_login_ip"]),
		LastLoginSource:            anyString(profile["last_login_source"]),
		LastLoginIPAt:              anyInt64(profile["last_login_ip_at"]),
		InviteePreviewJson:         adminUserOpsMarshalJSON(profile["invitee_preview"], "[]"),
		AccessControlStatusJson:    adminUserOpsMarshalJSON(profile["access_control_status"], "{}"),
		CommunityGateStatusJson:    adminUserOpsMarshalJSON(profile["community_gate_status"], "{}"),
		IdentityBindingsJson:       adminUserOpsMarshalJSON(profile["identity_bindings"], "[]"),
		AgentChatBindingsJson:      adminUserOpsMarshalJSON(profile["agent_chat_bindings"], "[]"),
		ChatMembershipStatesJson:   adminUserOpsMarshalJSON(profile["chat_membership_states"], "[]"),
		ProfileJSON:                adminUserOpsMarshalJSON(profile, "{}"),
		ProfileSyncedAt:            now,
	}
	return model.UpsertUserOpsProfileSnapshot(row)
}

func adminUserOpsProfileFromSnapshot(row *model.UserOpsProfileSnapshot) (map[string]any, error) {
	if row == nil {
		return nil, gorm.ErrRecordNotFound
	}
	if profile, ok := adminUserOpsUnmarshalJSONMap(row.ProfileJSON); ok {
		profile["site_id"] = firstNonEmptyString(anyString(profile["site_id"]), row.SiteId)
		profile["community_site_id"] = firstNonEmptyString(anyString(profile["community_site_id"]), row.CommunitySiteId)
		profile["user_id"] = anyInt(profile["user_id"])
		profile["profile_synced_at"] = row.ProfileSyncedAt
		profile["snapshot_updated_at"] = row.UpdatedAt
		return profile, nil
	}
	profile := map[string]any{
		"site_id":                       row.SiteId,
		"community_site_id":             row.CommunitySiteId,
		"user_id":                       row.UserId,
		"username":                      row.Username,
		"display_name":                  row.DisplayName,
		"email":                         row.Email,
		"quota":                         row.Quota,
		"used_quota":                    row.UsedQuota,
		"request_count":                 row.RequestCount,
		"base_group":                    row.BaseGroup,
		"group":                         row.BaseGroup,
		"role":                          row.Role,
		"status":                        row.Status,
		"access_level":                  row.AccessLevel,
		"effective_groups":              adminUserOpsParseStringSlice(row.EffectiveGroups),
		"community_bound":               row.CommunityBound,
		"has_community_oauth_binding":   row.HasCommunityOAuthBinding,
		"has_community_room_membership": row.HasCommunityRoomMembership,
		"community_room_ids":            adminUserOpsParseStringSlice(row.CommunityRoomIds),
		"community_external_user_id":    row.CommunityExternalUserId,
		"community_username":            row.CommunityUsername,
		"qq_bound":                      row.QQBound,
		"qq_bound_group_ids":            adminUserOpsParseStringSlice(row.QQBoundGroupIds),
		"qq_external_user_id":           row.QQExternalUserId,
		"qq_username":                   row.QQUsername,
		"tg_bound":                      row.TGBound,
		"tg_bound_group_ids":            adminUserOpsParseStringSlice(row.TGBoundGroupIds),
		"tg_external_user_id":           row.TGExternalUserId,
		"tg_username":                   row.TGUsername,
		"primary_bound":                 row.PrimaryBound,
		"primary_platform":              row.PrimaryPlatform,
		"matched_primary_group_id":      row.MatchedPrimaryGroupId,
		"manual_override_mode":          row.ManualOverrideMode,
		"manual_override_groups":        adminUserOpsParseStringSlice(row.ManualOverrideGroups),
		"manual_override_reason":        row.ManualOverrideReason,
		"active_frozen_key_count":       row.ActiveFrozenKeyCount,
		"access_control_frozen_keys":    row.AccessControlFrozenKeys,
		"community_gate_frozen_keys":    row.CommunityGateFrozenKeys,
		"has_active_frozen_keys":        row.HasActiveFrozenKeys,
		"can_restore":                   row.CanRestore,
		"inviter_id":                    row.InviterId,
		"inviter_username":              row.InviterUsername,
		"inviter_display_name":          row.InviterDisplayName,
		"invitee_count":                 row.InviteeCount,
		"aff_count":                     row.AffCount,
		"aff_quota":                     row.AffQuota,
		"aff_history_quota":             row.AffHistoryQuota,
		"last_login_at":                 row.LastLoginAt,
		"last_login_ip":                 row.LastLoginIP,
		"last_login_source":             row.LastLoginSource,
		"last_login_ip_at":              row.LastLoginIPAt,
		"reason_code":                   row.ReasonCode,
		"reason_message":                row.ReasonMessage,
		"created_at":                    row.CreatedAtUnix,
		"profile_synced_at":             row.ProfileSyncedAt,
		"snapshot_updated_at":           row.UpdatedAt,
	}
	adminUserOpsSetJSONField(profile, "invitee_preview", row.InviteePreviewJson, []any{})
	adminUserOpsSetJSONField(profile, "access_control_status", row.AccessControlStatusJson, map[string]any{})
	adminUserOpsSetJSONField(profile, "community_gate_status", row.CommunityGateStatusJson, map[string]any{})
	adminUserOpsSetJSONField(profile, "identity_bindings", row.IdentityBindingsJson, []any{})
	adminUserOpsSetJSONField(profile, "agent_chat_bindings", row.AgentChatBindingsJson, []any{})
	adminUserOpsSetJSONField(profile, "chat_membership_states", row.ChatMembershipStatesJson, []any{})
	return profile, nil
}

func adminUserOpsProfileSnapshotTTL() time.Duration {
	ttl := accessControlTTL()
	if ttl <= 0 {
		return 5 * time.Minute
	}
	if ttl > 15*time.Minute {
		return 15 * time.Minute
	}
	return ttl
}

func adminUserOpsSnapshotFresh(row *model.UserOpsProfileSnapshot, user *model.User, accessUpdatedAt int64, bindingUpdatedAt int64) bool {
	if row == nil || user == nil {
		return false
	}
	if row.UserId != user.Id || strings.TrimSpace(row.SiteId) == "" || strings.TrimSpace(row.ProfileJSON) == "" || row.ProfileSyncedAt <= 0 {
		return false
	}
	if time.Since(time.Unix(row.ProfileSyncedAt, 0)) > adminUserOpsProfileSnapshotTTL() {
		return false
	}
	if accessUpdatedAt > row.ProfileSyncedAt {
		return false
	}
	if bindingUpdatedAt > row.ProfileSyncedAt {
		return false
	}
	return row.Username == user.Username &&
		row.DisplayName == user.DisplayName &&
		row.Email == user.Email &&
		row.BaseGroup == user.Group &&
		row.Role == user.Role &&
		row.Status == user.Status &&
		row.Quota == user.Quota &&
		row.UsedQuota == user.UsedQuota &&
		row.RequestCount == user.RequestCount &&
		row.LastLoginAt == user.LastLoginAt &&
		row.CreatedAtUnix == user.CreatedAt &&
		row.InviterId == user.InviterId &&
		row.AffCount == user.AffCount &&
		row.AffQuota == user.AffQuota &&
		row.AffHistoryQuota == user.AffHistoryQuota
}

func adminUserOpsGridSnapshotUsable(row *model.UserOpsProfileSnapshot, user *model.User, accessUpdatedAt int64, bindingUpdatedAt int64) bool {
	if row == nil || user == nil {
		return false
	}
	if row.UserId != user.Id || strings.TrimSpace(row.SiteId) == "" || strings.TrimSpace(row.ProfileJSON) == "" || row.ProfileSyncedAt <= 0 {
		return false
	}
	if time.Since(time.Unix(row.ProfileSyncedAt, 0)) > 6*time.Hour {
		return false
	}
	if accessUpdatedAt > row.ProfileSyncedAt {
		return false
	}
	if bindingUpdatedAt > row.ProfileSyncedAt {
		return false
	}
	return row.Username == user.Username &&
		row.DisplayName == user.DisplayName &&
		row.Email == user.Email &&
		row.BaseGroup == user.Group &&
		row.Role == user.Role &&
		row.Status == user.Status &&
		row.CreatedAtUnix == user.CreatedAt &&
		row.InviterId == user.InviterId &&
		row.AffCount == user.AffCount &&
		row.AffQuota == user.AffQuota &&
		row.AffHistoryQuota == user.AffHistoryQuota
}

func mergeAdminUserOpsGridUserFields(profile map[string]any, siteID string, user *model.User) {
	if len(profile) == 0 || user == nil {
		return
	}
	communitySiteID := model.CommunityIdentitySiteID()
	if communitySiteID == "" {
		communitySiteID = siteID
	}
	profile["site_id"] = siteID
	profile["community_site_id"] = communitySiteID
	profile["user_id"] = user.Id
	profile["username"] = user.Username
	profile["display_name"] = user.DisplayName
	profile["email"] = user.Email
	profile["base_group"] = user.Group
	profile["group"] = user.Group
	profile["role"] = user.Role
	profile["status"] = user.Status
	profile["quota"] = user.Quota
	profile["used_quota"] = user.UsedQuota
	profile["request_count"] = user.RequestCount
	profile["last_login_at"] = user.LastLoginAt
	profile["created_at"] = user.CreatedAt
	profile["inviter_id"] = user.InviterId
	profile["aff_count"] = user.AffCount
	profile["aff_quota"] = user.AffQuota
	profile["aff_history_quota"] = user.AffHistoryQuota
}

type adminUserCountSnapshot struct {
	UserID int   `gorm:"column:user_id"`
	Count  int64 `gorm:"column:count"`
}

func loadAdminActiveFreezeCountMap(userIDs []int, freezeKey string) (map[int]int64, error) {
	out := make(map[int]int64, len(userIDs))
	if len(userIDs) == 0 {
		return out, nil
	}
	var rows []adminUserCountSnapshot
	if err := model.DB.Model(&model.CommunityGateTokenFreeze{}).
		Select("user_id, COUNT(*) AS count").
		Where("user_id IN ? AND freeze_key = ? AND restored_at = 0", uniqueInts(userIDs), strings.TrimSpace(freezeKey)).
		Group("user_id").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		if row.UserID > 0 {
			out[row.UserID] = row.Count
		}
	}
	return out, nil
}

func refreshAdminUserRestoreState(siteID string, profiles []map[string]any) error {
	userIDs := make([]int, 0, len(profiles))
	for _, profile := range profiles {
		userIDs = append(userIDs, anyInt(profile["user_id"]))
	}
	accessFrozenMap, err := loadAdminActiveFreezeCountMap(userIDs, model.AccessControlFreezeKey)
	if err != nil {
		return err
	}
	communityFrozenMap, err := loadAdminActiveFreezeCountMap(userIDs, "community_gate")
	if err != nil {
		return err
	}
	riskRestoreMap, err := loadAdminRiskRestoreStateMap(siteID, userIDs)
	if err != nil {
		return err
	}
	mergeAdminUserOpsGridRestoreState(profiles, accessFrozenMap, communityFrozenMap, riskRestoreMap)
	return nil
}

func mergeAdminUserOpsGridRestoreState(profiles []map[string]any, accessFrozenMap, communityFrozenMap map[int]int64, riskRestoreMap map[int]adminRiskRestoreState) {
	for _, profile := range profiles {
		userID := anyInt(profile["user_id"])
		accessCount := accessFrozenMap[userID]
		communityCount := communityFrozenMap[userID]
		riskRestore := riskRestoreMap[userID]
		total := accessCount + communityCount + riskRestore.RestorableTokenCount
		profile["access_control_frozen_keys"] = int(accessCount)
		profile["community_gate_frozen_keys"] = int(communityCount)
		profile["risk_controlled_keys"] = int(riskRestore.RestorableTokenCount)
		profile["has_active_risk_control"] = riskRestore.ActiveControl
		profile["active_frozen_key_count"] = int(total)
		profile["has_active_frozen_keys"] = total > 0
		profile["can_restore"] = total > 0 || riskRestore.ActiveControl
	}
}

func GetAdminUserBindings(siteID string, userID int) (map[string]any, error) {
	identityBindings, agentBindings, membershipStates, err := loadAdminUserBindings(resolveOpsSiteID(siteID), userID)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"site_id":                resolveOpsSiteID(siteID),
		"user_id":                userID,
		"identity_bindings":      identityBindings,
		"agent_chat_bindings":    agentBindings,
		"chat_membership_states": membershipStates,
	}, nil
}

func RecomputeAdminUserMembership(ctx context.Context, siteID string, userID int) (map[string]any, error) {
	profile, err := GetAdminUserOpsProfile(ctx, siteID, userID, true)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"user_id":     userID,
		"site_id":     resolveOpsSiteID(siteID),
		"recomputed":  true,
		"ops_profile": profile,
	}, nil
}

func UpdateAdminUserAccessOverride(ctx context.Context, siteID string, userID int, mode string, groups []string, reason string) (map[string]any, error) {
	siteID = resolveOpsSiteID(siteID)
	mode = strings.TrimSpace(mode)
	if mode == "" || mode == "clear" {
		if err := model.ClearUserSiteAccessOverride(siteID, userID); err != nil {
			return nil, err
		}
	} else {
		if err := model.SetUserSiteAccessOverride(siteID, userID, mode, groups, reason); err != nil {
			return nil, err
		}
	}
	accessStatus, err := GetUserAccessControlStatus(ctx, userID, true)
	if err != nil {
		return nil, err
	}
	profile, err := GetAdminUserOpsProfile(ctx, siteID, userID, true)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"user_id":       userID,
		"site_id":       siteID,
		"mode":          mode,
		"groups":        uniqueStrings(groups),
		"reason":        strings.TrimSpace(reason),
		"access_status": accessStatus,
		"ops_profile":   profile,
	}, nil
}

func RestoreAdminUserKeys(ctx context.Context, siteID string, userID int, operatorID int) (map[string]any, error) {
	siteID = resolveOpsSiteID(siteID)
	accessStats, accessState, accessErr := RestoreAccessControlUserTokensIfCompliant(ctx, userID)
	communityStats, communityGate, communityErr := RestoreCommunityGateUserTokensIfCompliant(ctx, userID)
	riskStats, riskErr := RestoreAdminRiskControlledTokens(siteID, userID, operatorID, "管理员从用户管理恢复 Key 使用权限")
	profile, profileErr := GetAdminUserOpsProfile(ctx, siteID, userID, true)
	if profileErr != nil {
		return nil, profileErr
	}
	result := map[string]any{
		"user_id": userID,
		"site_id": siteID,
		"access_control": map[string]any{
			"stats": accessStats,
			"state": accessState,
		},
		"community_gate": map[string]any{
			"stats": communityStats,
			"gate":  communityGate,
		},
		"risk_control": riskStats,
		"ops_profile":  profile,
	}
	var errs []string
	if accessErr != nil {
		result["access_control"].(map[string]any)["error"] = accessErr.Error()
		errs = append(errs, "access_control: "+accessErr.Error())
	}
	if communityErr != nil {
		result["community_gate"].(map[string]any)["error"] = communityErr.Error()
		errs = append(errs, "community_gate: "+communityErr.Error())
	}
	if riskErr != nil {
		result["risk_control"] = map[string]any{"stats": riskStats, "error": riskErr.Error()}
		errs = append(errs, "risk_control: "+riskErr.Error())
	}
	if len(errs) > 0 {
		return result, fmt.Errorf("%s", strings.Join(errs, " | "))
	}
	return result, nil
}

func loadAdminUserBindings(siteID string, userID int) ([]model.UserIdentityBinding, []model.AgentChatBinding, []model.ChatMembershipState, error) {
	identityBindings, err := model.ListUserIdentityBindingsByUser(siteID, "", userID)
	if err != nil {
		return nil, nil, nil, err
	}
	communitySiteID := model.CommunityIdentitySiteID()
	if communitySiteID != "" && communitySiteID != siteID {
		communityBindings, communityErr := model.ListUserIdentityBindingsByUser(communitySiteID, "community", userID)
		if communityErr != nil {
			return nil, nil, nil, communityErr
		}
		identityBindings = uniqueIdentityBindings(append(identityBindings, communityBindings...))
	}
	agentBindings, err := model.ListAgentChatBindingsByUser(siteID, "", userID)
	if err != nil {
		return nil, nil, nil, err
	}
	membershipStates, err := model.ListChatMembershipStatesByUser(userID, 200)
	if err != nil {
		return nil, nil, nil, err
	}
	return identityBindings, agentBindings, membershipStates, nil
}

type adminUserInviteePreview struct {
	UserID      int    `json:"user_id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name,omitempty"`
	Email       string `json:"email,omitempty"`
	Status      int    `json:"status"`
	LastLoginAt int64  `json:"last_login_at"`
	CreatedAt   int64  `json:"created_at"`
}

type adminUserLoginSnapshot struct {
	UserID    int    `gorm:"column:user_id"`
	IP        string `gorm:"column:ip"`
	Source    string `gorm:"column:source"`
	CreatedAt int64  `gorm:"column:created_at"`
}

type adminUserAccessSnapshot struct {
	UserID    int   `gorm:"column:user_id"`
	UpdatedAt int64 `gorm:"column:updated_at"`
}

func attachAdminUserRelationshipFields(siteID string, profiles []map[string]any) error {
	if len(profiles) == 0 {
		return nil
	}
	userIDs := make([]int, 0, len(profiles))
	inviterIDs := make([]int, 0, len(profiles))
	for _, profile := range profiles {
		userID := anyInt(profile["user_id"])
		if userID > 0 {
			userIDs = append(userIDs, userID)
		}
		inviterID := anyInt(profile["inviter_id"])
		if inviterID > 0 {
			inviterIDs = append(inviterIDs, inviterID)
		}
	}
	inviterMap, err := loadAdminUserBasicMap(uniqueInts(inviterIDs))
	if err != nil {
		return err
	}
	loginMap, err := loadAdminLatestLoginSnapshots(uniqueInts(userIDs))
	if err != nil {
		return err
	}
	inviteeMap, err := loadAdminInviteePreviewMap(uniqueInts(userIDs), 5)
	if err != nil {
		return err
	}
	communitySiteID := model.CommunityIdentitySiteID()
	if communitySiteID == "" {
		communitySiteID = siteID
	}
	for _, profile := range profiles {
		userID := anyInt(profile["user_id"])
		profile["site_id"] = siteID
		profile["community_site_id"] = communitySiteID
		if inviterUser, ok := inviterMap[anyInt(profile["inviter_id"])]; ok {
			profile["inviter_username"] = inviterUser.Username
			profile["inviter_display_name"] = inviterUser.DisplayName
		} else {
			profile["inviter_username"] = ""
			profile["inviter_display_name"] = ""
		}
		if login, ok := loginMap[userID]; ok {
			profile["last_login_ip"] = strings.TrimSpace(login.IP)
			profile["last_login_source"] = strings.TrimSpace(login.Source)
			profile["last_login_ip_at"] = login.CreatedAt
			if anyInt64(profile["last_login_at"]) <= 0 && login.CreatedAt > 0 {
				profile["last_login_at"] = login.CreatedAt
			}
		} else {
			profile["last_login_ip"] = ""
			profile["last_login_source"] = ""
			profile["last_login_ip_at"] = int64(0)
		}
		invitees := inviteeMap[userID]
		if invitees == nil {
			invitees = []adminUserInviteePreview{}
		}
		profile["invitee_preview"] = invitees
		inviteeCount := anyInt(profile["aff_count"])
		if len(invitees) > inviteeCount {
			inviteeCount = len(invitees)
		}
		profile["invitee_count"] = inviteeCount
		profile["can_restore"] = anyInt(profile["active_frozen_key_count"]) > 0 || toBool(profile["has_active_risk_control"])
	}
	return nil
}

func loadAdminUserBasicMap(userIDs []int) (map[int]model.User, error) {
	out := make(map[int]model.User, len(userIDs))
	if len(userIDs) == 0 {
		return out, nil
	}
	var users []model.User
	if err := model.DB.Unscoped().
		Select("id, username, display_name, email, status, last_login_at").
		Where("id IN ?", userIDs).
		Find(&users).Error; err != nil {
		return nil, err
	}
	for _, user := range users {
		out[user.Id] = user
	}
	return out, nil
}

func loadAdminLatestLoginSnapshots(userIDs []int) (map[int]adminUserLoginSnapshot, error) {
	out := make(map[int]adminUserLoginSnapshot, len(userIDs))
	if len(userIDs) == 0 {
		return out, nil
	}
	subQuery := model.LOG_DB.Model(&model.Log{}).
		Select("MAX(id) AS id").
		Where("type = ? AND user_id IN ?", model.LogTypeLogin, userIDs).
		Group("user_id")
	var rows []adminUserLoginSnapshot
	if err := model.LOG_DB.Table("logs AS l").
		Select("l.user_id, l.ip, l.source, l.created_at").
		Joins("JOIN (?) latest ON latest.id = l.id", subQuery).
		Order("l.id desc").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		if row.UserID <= 0 {
			continue
		}
		out[row.UserID] = row
	}
	return out, nil
}

func loadAdminInviteePreviewMap(inviterIDs []int, limit int) (map[int][]adminUserInviteePreview, error) {
	out := make(map[int][]adminUserInviteePreview, len(inviterIDs))
	if len(inviterIDs) == 0 || limit <= 0 {
		return out, nil
	}
	var invitees []model.User
	if err := model.DB.Unscoped().
		Select("id, username, display_name, email, status, inviter_id, last_login_at, created_at").
		Where("inviter_id IN ?", inviterIDs).
		Order("inviter_id asc, created_at desc, id desc").
		Find(&invitees).Error; err != nil {
		return nil, err
	}
	counts := make(map[int]int, len(inviterIDs))
	for _, invitee := range invitees {
		inviterID := invitee.InviterId
		if inviterID <= 0 {
			continue
		}
		if counts[inviterID] >= limit {
			continue
		}
		out[inviterID] = append(out[inviterID], adminUserInviteePreview{
			UserID:      invitee.Id,
			Username:    invitee.Username,
			DisplayName: invitee.DisplayName,
			Email:       invitee.Email,
			Status:      invitee.Status,
			LastLoginAt: invitee.LastLoginAt,
			CreatedAt:   invitee.CreatedAt,
		})
		counts[inviterID]++
	}
	return out, nil
}

func loadAdminUserOpsSnapshotMap(siteID string, userIDs []int) (map[int]*model.UserOpsProfileSnapshot, error) {
	out := make(map[int]*model.UserOpsProfileSnapshot, len(userIDs))
	if len(userIDs) == 0 {
		return out, nil
	}
	rows, err := model.ListUserOpsProfileSnapshots(resolveOpsSiteID(siteID), uniqueInts(userIDs))
	if err != nil {
		return nil, err
	}
	for i := range rows {
		row := rows[i]
		if _, exists := out[row.UserId]; exists {
			continue
		}
		copyRow := row
		out[row.UserId] = &copyRow
	}
	return out, nil
}

func loadUserSiteAccessUpdatedAtMap(siteID string, userIDs []int) (map[int]int64, error) {
	out := make(map[int]int64, len(userIDs))
	if len(userIDs) == 0 {
		return out, nil
	}
	var rows []adminUserAccessSnapshot
	if err := model.DB.Model(&model.UserSiteAccessState{}).
		Select("user_id, updated_at").
		Where("site_id = ? AND user_id IN ?", resolveOpsSiteID(siteID), uniqueInts(userIDs)).
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		if row.UserID > 0 {
			out[row.UserID] = row.UpdatedAt
		}
	}
	return out, nil
}

func loadAdminUserBindingUpdatedAtMap(siteID string, userIDs []int) (map[int]int64, error) {
	out := make(map[int]int64, len(userIDs))
	if len(userIDs) == 0 {
		return out, nil
	}
	siteID = resolveOpsSiteID(siteID)
	uniqueUserIDs := uniqueInts(userIDs)
	identitySiteIDs := []string{siteID}
	if communitySiteID := strings.TrimSpace(model.CommunityIdentitySiteID()); communitySiteID != "" && communitySiteID != siteID {
		identitySiteIDs = append(identitySiteIDs, communitySiteID)
	}
	var rows []adminUserAccessSnapshot
	if err := model.DB.Model(&model.UserIdentityBinding{}).
		Select("user_id, MAX(updated_at) AS updated_at").
		Where("site_id IN ? AND user_id IN ?", uniqueStrings(identitySiteIDs), uniqueUserIDs).
		Group("user_id").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	mergeAdminUserUpdatedAtRows(out, rows)
	rows = nil
	if err := model.DB.Model(&model.AgentChatBinding{}).
		Select("new_api_user_id AS user_id, MAX(updated_at) AS updated_at").
		Where("site_id = ? AND new_api_user_id IN ?", siteID, uniqueUserIDs).
		Group("new_api_user_id").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	mergeAdminUserUpdatedAtRows(out, rows)
	rows = nil
	if err := model.DB.Model(&model.ChatMembershipState{}).
		Select("new_api_user_id AS user_id, MAX(updated_at) AS updated_at").
		Where("site_id = ? AND new_api_user_id IN ?", siteID, uniqueUserIDs).
		Group("new_api_user_id").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	mergeAdminUserUpdatedAtRows(out, rows)
	return out, nil
}

func mergeAdminUserUpdatedAtRows(target map[int]int64, rows []adminUserAccessSnapshot) {
	for _, row := range rows {
		if row.UserID <= 0 {
			continue
		}
		if row.UpdatedAt > target[row.UserID] {
			target[row.UserID] = row.UpdatedAt
		}
	}
}

func adminUserOpsGridItem(profile map[string]any) map[string]any {
	return map[string]any{
		"site_id":                       profile["site_id"],
		"community_site_id":             profile["community_site_id"],
		"user_id":                       profile["user_id"],
		"username":                      profile["username"],
		"display_name":                  profile["display_name"],
		"email":                         profile["email"],
		"quota":                         profile["quota"],
		"used_quota":                    profile["used_quota"],
		"request_count":                 profile["request_count"],
		"base_group":                    profile["base_group"],
		"group":                         profile["base_group"],
		"role":                          profile["role"],
		"status":                        profile["status"],
		"access_level":                  profile["access_level"],
		"effective_groups":              profile["effective_groups"],
		"community_bound":               profile["community_bound"],
		"has_community_room_membership": profile["has_community_room_membership"],
		"community_external_user_id":    profile["community_external_user_id"],
		"community_username":            profile["community_username"],
		"qq_bound":                      profile["qq_bound"],
		"qq_bound_group_ids":            profile["qq_bound_group_ids"],
		"qq_external_user_id":           profile["qq_external_user_id"],
		"qq_username":                   profile["qq_username"],
		"tg_bound":                      profile["tg_bound"],
		"tg_bound_group_ids":            profile["tg_bound_group_ids"],
		"tg_external_user_id":           profile["tg_external_user_id"],
		"tg_username":                   profile["tg_username"],
		"primary_bound":                 profile["primary_bound"],
		"primary_platform":              profile["primary_platform"],
		"matched_primary_group_id":      profile["matched_primary_group_id"],
		"manual_override_mode":          profile["manual_override_mode"],
		"manual_override_groups":        profile["manual_override_groups"],
		"manual_override_reason":        profile["manual_override_reason"],
		"active_frozen_key_count":       profile["active_frozen_key_count"],
		"access_control_frozen_keys":    profile["access_control_frozen_keys"],
		"community_gate_frozen_keys":    profile["community_gate_frozen_keys"],
		"risk_controlled_keys":          profile["risk_controlled_keys"],
		"has_active_risk_control":       profile["has_active_risk_control"],
		"has_active_frozen_keys":        profile["has_active_frozen_keys"],
		"can_restore":                   profile["can_restore"],
		"inviter_id":                    profile["inviter_id"],
		"inviter_username":              profile["inviter_username"],
		"inviter_display_name":          profile["inviter_display_name"],
		"invitee_count":                 profile["invitee_count"],
		"invitee_preview":               profile["invitee_preview"],
		"aff_count":                     profile["aff_count"],
		"aff_quota":                     profile["aff_quota"],
		"aff_history_quota":             profile["aff_history_quota"],
		"last_login_at":                 profile["last_login_at"],
		"last_login_ip":                 profile["last_login_ip"],
		"last_login_source":             profile["last_login_source"],
		"last_login_ip_at":              profile["last_login_ip_at"],
		"reason_code":                   profile["reason_code"],
		"reason_message":                profile["reason_message"],
	}
}

func applyExistsFilter(query *gorm.DB, expected bool, subquery *gorm.DB) *gorm.DB {
	if expected {
		return query.Where("EXISTS (?)", subquery)
	}
	return query.Where("NOT EXISTS (?)", subquery)
}

func pickLatestIdentityBinding(rows []model.UserIdentityBinding, provider string) *model.UserIdentityBinding {
	provider = strings.TrimSpace(strings.ToLower(provider))
	var filtered []model.UserIdentityBinding
	for _, row := range rows {
		if strings.TrimSpace(strings.ToLower(row.Provider)) == provider {
			filtered = append(filtered, row)
		}
	}
	if len(filtered) == 0 {
		return nil
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		if filtered[i].UpdatedAt == filtered[j].UpdatedAt {
			return filtered[i].Id > filtered[j].Id
		}
		return filtered[i].UpdatedAt > filtered[j].UpdatedAt
	})
	row := filtered[0]
	return &row
}

func pickLatestAgentBinding(rows []model.AgentChatBinding, source string) *model.AgentChatBinding {
	source = strings.TrimSpace(strings.ToLower(source))
	var filtered []model.AgentChatBinding
	for _, row := range rows {
		if strings.TrimSpace(strings.ToLower(row.Source)) == source {
			filtered = append(filtered, row)
		}
	}
	if len(filtered) == 0 {
		return nil
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		if filtered[i].UpdatedAt == filtered[j].UpdatedAt {
			return filtered[i].Id > filtered[j].Id
		}
		return filtered[i].UpdatedAt > filtered[j].UpdatedAt
	})
	row := filtered[0]
	return &row
}

func uniqueRoomIDsFromAgentBindings(rows []model.AgentChatBinding, source string) []string {
	source = strings.TrimSpace(strings.ToLower(source))
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		if strings.TrimSpace(strings.ToLower(row.Source)) != source {
			continue
		}
		if roomID := strings.TrimSpace(row.RoomId); roomID != "" {
			out = append(out, roomID)
		}
	}
	return uniqueStrings(out)
}

func membershipRoomIDs(rows []model.ChatMembershipState, source string) []string {
	source = strings.TrimSpace(strings.ToLower(source))
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		if strings.TrimSpace(strings.ToLower(row.Source)) != source {
			continue
		}
		if strings.TrimSpace(strings.ToLower(row.Status)) == "left" {
			continue
		}
		if roomID := strings.TrimSpace(row.RoomId); roomID != "" {
			out = append(out, roomID)
		}
	}
	return uniqueStrings(out)
}

func uniqueStrings(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func uniqueIdentityBindings(rows []model.UserIdentityBinding) []model.UserIdentityBinding {
	seen := map[string]struct{}{}
	out := make([]model.UserIdentityBinding, 0, len(rows))
	for _, row := range rows {
		key := strings.ToLower(strings.TrimSpace(row.Provider)) + "|" + strings.TrimSpace(row.ExternalUserId)
		if key == "|" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, row)
	}
	return out
}

func toBool(value any) bool {
	switch v := value.(type) {
	case bool:
		return v
	default:
		return false
	}
}

func anyString(value any) string {
	if value == nil {
		return ""
	}
	if v, ok := value.(string); ok {
		return strings.TrimSpace(v)
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func toStringSlice(value any) []string {
	switch v := value.(type) {
	case []string:
		return uniqueStrings(v)
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			out = append(out, anyString(item))
		}
		return uniqueStrings(out)
	default:
		return []string{}
	}
}

func anyInt(value any) int {
	switch v := value.(type) {
	case int:
		return v
	case int8:
		return int(v)
	case int16:
		return int(v)
	case int32:
		return int(v)
	case int64:
		return int(v)
	case float32:
		return int(v)
	case float64:
		return int(v)
	case string:
		parsed, _ := strconv.Atoi(strings.TrimSpace(v))
		return parsed
	default:
		return 0
	}
}

func anyInt64(value any) int64 {
	switch v := value.(type) {
	case int:
		return int64(v)
	case int8:
		return int64(v)
	case int16:
		return int64(v)
	case int32:
		return int64(v)
	case int64:
		return v
	case float32:
		return int64(v)
	case float64:
		return int64(v)
	case string:
		parsed, _ := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		return parsed
	default:
		return 0
	}
}

func uniqueInts(values []int) []int {
	seen := make(map[int]struct{}, len(values))
	out := make([]int, 0, len(values))
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Ints(out)
	return out
}

func bindingExternalUserID(binding any) string {
	switch v := binding.(type) {
	case *model.UserIdentityBinding:
		if v != nil {
			return strings.TrimSpace(v.ExternalUserId)
		}
	case *model.AgentChatBinding:
		if v != nil {
			return strings.TrimSpace(v.ExternalUserId)
		}
	}
	return ""
}

func bindingUsername(binding any) string {
	switch v := binding.(type) {
	case *model.UserIdentityBinding:
		if v != nil {
			return strings.TrimSpace(v.Username)
		}
	case *model.AgentChatBinding:
		if v != nil {
			return strings.TrimSpace(v.Username)
		}
	}
	return ""
}

func preferredBindingExternalUserID(primary any, fallback any) string {
	if value := bindingExternalUserID(primary); value != "" {
		return value
	}
	return bindingExternalUserID(fallback)
}

func adminUserOpsMarshalJSON(value any, empty string) string {
	if value == nil {
		return empty
	}
	data, err := json.Marshal(value)
	if err != nil || len(data) == 0 {
		return empty
	}
	return string(data)
}

func adminUserOpsUnmarshalJSONMap(raw string) (map[string]any, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, false
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(raw), &out); err != nil || out == nil {
		return nil, false
	}
	return out, true
}

func adminUserOpsSetJSONField(profile map[string]any, key string, raw string, empty any) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		if empty != nil {
			profile[key] = empty
		}
		return
	}
	var out any
	if err := json.Unmarshal([]byte(raw), &out); err == nil {
		profile[key] = out
		return
	}
	if empty != nil {
		profile[key] = empty
	}
}

func adminUserOpsParseStringSlice(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return []string{}
	}
	var out []string
	if err := json.Unmarshal([]byte(raw), &out); err == nil {
		return uniqueStrings(out)
	}
	return uniqueStrings(strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == '\n' || r == '\r' || r == '\t'
	}))
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func preferredBindingUsername(primary any, fallback any) string {
	if value := bindingUsername(primary); value != "" {
		return value
	}
	return bindingUsername(fallback)
}

func firstAnyString(value any, mapper func(any) string) string {
	if mapper == nil {
		return ""
	}
	return strings.TrimSpace(mapper(value))
}
