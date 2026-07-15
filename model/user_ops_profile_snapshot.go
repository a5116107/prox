package model

import (
	"errors"
	"strings"
	"time"

	"gorm.io/gorm/clause"
)

type UserOpsProfileSnapshot struct {
	Id                         int    `json:"id"`
	SiteId                     string `json:"site_id" gorm:"type:varchar(64);not null;uniqueIndex:ux_user_ops_profile_snapshot;index"`
	UserId                     int    `json:"user_id" gorm:"not null;uniqueIndex:ux_user_ops_profile_snapshot;index"`
	Username                   string `json:"username" gorm:"type:varchar(255);default:'';index"`
	DisplayName                string `json:"display_name" gorm:"type:varchar(255);default:''"`
	Email                      string `json:"email" gorm:"type:varchar(255);default:'';index"`
	BaseGroup                  string `json:"base_group" gorm:"column:base_group;type:varchar(64);default:'';index"`
	Role                       int    `json:"role" gorm:"not null;default:0;index"`
	Status                     int    `json:"status" gorm:"not null;default:0;index"`
	Quota                      int    `json:"quota" gorm:"type:int;default:0"`
	UsedQuota                  int    `json:"used_quota" gorm:"type:int;default:0"`
	RequestCount               int    `json:"request_count" gorm:"type:int;default:0"`
	LastLoginAt                int64  `json:"last_login_at" gorm:"default:0;index"`
	CreatedAtUnix              int64  `json:"created_at_unix" gorm:"column:created_at_unix;default:0"`
	InviterId                  int    `json:"inviter_id" gorm:"default:0;index"`
	InviterUsername            string `json:"inviter_username" gorm:"type:varchar(255);default:''"`
	InviterDisplayName         string `json:"inviter_display_name" gorm:"type:varchar(255);default:''"`
	InviteeCount               int    `json:"invitee_count" gorm:"default:0"`
	AffCount                   int    `json:"aff_count" gorm:"default:0"`
	AffQuota                   int    `json:"aff_quota" gorm:"default:0"`
	AffHistoryQuota            int    `json:"aff_history_quota" gorm:"default:0"`
	CommunityBound             bool   `json:"community_bound" gorm:"not null;default:false;index"`
	HasCommunityOAuthBinding   bool   `json:"has_community_oauth_binding" gorm:"column:has_community_o_auth_binding;not null;default:false"`
	HasCommunityRoomMembership bool   `json:"has_community_room_membership" gorm:"not null;default:false;index"`
	CommunityRoomIds           string `json:"community_room_ids" gorm:"type:text"`
	CommunityExternalUserId    string `json:"community_external_user_id" gorm:"type:varchar(255);default:''"`
	CommunityUsername          string `json:"community_username" gorm:"type:varchar(255);default:''"`
	QQBound                    bool   `json:"qq_bound" gorm:"not null;default:false;index"`
	QQExternalUserId           string `json:"qq_external_user_id" gorm:"type:varchar(255);default:''"`
	QQUsername                 string `json:"qq_username" gorm:"type:varchar(255);default:''"`
	QQBoundGroupIds            string `json:"qq_bound_group_ids" gorm:"type:text"`
	TGBound                    bool   `json:"tg_bound" gorm:"not null;default:false;index"`
	TGExternalUserId           string `json:"tg_external_user_id" gorm:"type:varchar(255);default:''"`
	TGUsername                 string `json:"tg_username" gorm:"type:varchar(255);default:''"`
	TGBoundGroupIds            string `json:"tg_bound_group_ids" gorm:"type:text"`
	AccessLevel                string `json:"access_level" gorm:"type:varchar(32);default:'';index"`
	EffectiveGroups            string `json:"effective_groups" gorm:"type:text"`
	ReasonCode                 string `json:"reason_code" gorm:"type:varchar(64);default:'';index"`
	ReasonMessage              string `json:"reason_message" gorm:"type:text"`
	PrimaryBound               bool   `json:"primary_bound" gorm:"not null;default:false;index"`
	PrimaryPlatform            string `json:"primary_platform" gorm:"type:varchar(32);default:''"`
	MatchedPrimaryGroupId      string `json:"matched_primary_group_id" gorm:"type:varchar(128);default:''"`
	ManualOverrideMode         string `json:"manual_override_mode" gorm:"type:varchar(32);default:'';index"`
	ManualOverrideGroups       string `json:"manual_override_groups" gorm:"type:text"`
	ManualOverrideReason       string `json:"manual_override_reason" gorm:"type:text"`
	ActiveFrozenKeyCount       int    `json:"active_frozen_key_count" gorm:"default:0"`
	AccessControlFrozenKeys    int    `json:"access_control_frozen_keys" gorm:"default:0"`
	CommunityGateFrozenKeys    int    `json:"community_gate_frozen_keys" gorm:"default:0"`
	HasActiveFrozenKeys        bool   `json:"has_active_frozen_keys" gorm:"not null;default:false;index"`
	CanRestore                 bool   `json:"can_restore" gorm:"not null;default:false"`
	CommunitySiteId            string `json:"community_site_id" gorm:"type:varchar(64);default:''"`
	LastLoginIP                string `json:"last_login_ip" gorm:"type:varchar(255);default:''"`
	LastLoginSource            string `json:"last_login_source" gorm:"type:varchar(64);default:''"`
	LastLoginIPAt              int64  `json:"last_login_ip_at" gorm:"default:0"`
	InviteePreviewJson         string `json:"invitee_preview_json" gorm:"type:text"`
	AccessControlStatusJson    string `json:"access_control_status_json" gorm:"type:text"`
	CommunityGateStatusJson    string `json:"community_gate_status_json" gorm:"type:text"`
	IdentityBindingsJson       string `json:"identity_bindings_json" gorm:"type:text"`
	AgentChatBindingsJson      string `json:"agent_chat_bindings_json" gorm:"type:text"`
	ChatMembershipStatesJson   string `json:"chat_membership_states_json" gorm:"type:text"`
	ProfileJSON                string `json:"profile_json" gorm:"column:profile_json;type:text"`
	ProfileSyncedAt            int64  `json:"profile_synced_at" gorm:"index"`
	CreatedAt                  int64  `json:"created_at" gorm:"index"`
	UpdatedAt                  int64  `json:"updated_at"`
}

func (UserOpsProfileSnapshot) TableName() string { return "user_ops_profile_snapshots" }

func UpsertUserOpsProfileSnapshot(snapshot *UserOpsProfileSnapshot) error {
	if snapshot == nil || snapshot.UserId <= 0 {
		return errors.New("invalid user ops profile snapshot")
	}
	snapshot.SiteId = strings.TrimSpace(snapshot.SiteId)
	if snapshot.SiteId == "" {
		return errors.New("site_id is required")
	}
	snapshot.Username = strings.TrimSpace(snapshot.Username)
	snapshot.DisplayName = strings.TrimSpace(snapshot.DisplayName)
	snapshot.Email = strings.TrimSpace(snapshot.Email)
	snapshot.BaseGroup = strings.TrimSpace(snapshot.BaseGroup)
	snapshot.InviterUsername = strings.TrimSpace(snapshot.InviterUsername)
	snapshot.InviterDisplayName = strings.TrimSpace(snapshot.InviterDisplayName)
	snapshot.CommunityExternalUserId = strings.TrimSpace(snapshot.CommunityExternalUserId)
	snapshot.CommunityUsername = strings.TrimSpace(snapshot.CommunityUsername)
	snapshot.QQExternalUserId = strings.TrimSpace(snapshot.QQExternalUserId)
	snapshot.QQUsername = strings.TrimSpace(snapshot.QQUsername)
	snapshot.TGExternalUserId = strings.TrimSpace(snapshot.TGExternalUserId)
	snapshot.TGUsername = strings.TrimSpace(snapshot.TGUsername)
	snapshot.AccessLevel = strings.TrimSpace(snapshot.AccessLevel)
	snapshot.ReasonCode = strings.TrimSpace(snapshot.ReasonCode)
	snapshot.ReasonMessage = strings.TrimSpace(snapshot.ReasonMessage)
	snapshot.PrimaryPlatform = normalizeIdentityProvider(snapshot.PrimaryPlatform)
	snapshot.MatchedPrimaryGroupId = strings.TrimSpace(snapshot.MatchedPrimaryGroupId)
	snapshot.ManualOverrideMode = strings.TrimSpace(snapshot.ManualOverrideMode)
	snapshot.ManualOverrideReason = strings.TrimSpace(snapshot.ManualOverrideReason)
	snapshot.CommunitySiteId = strings.TrimSpace(snapshot.CommunitySiteId)
	snapshot.LastLoginIP = strings.TrimSpace(snapshot.LastLoginIP)
	snapshot.LastLoginSource = strings.TrimSpace(snapshot.LastLoginSource)
	snapshot.CommunityRoomIds = marshalStringSlice(parseStringSlice(snapshot.CommunityRoomIds))
	snapshot.QQBoundGroupIds = marshalStringSlice(parseStringSlice(snapshot.QQBoundGroupIds))
	snapshot.TGBoundGroupIds = marshalStringSlice(parseStringSlice(snapshot.TGBoundGroupIds))
	snapshot.EffectiveGroups = marshalStringSlice(parseStringSlice(snapshot.EffectiveGroups))
	snapshot.ManualOverrideGroups = marshalStringSlice(parseStringSlice(snapshot.ManualOverrideGroups))
	if strings.TrimSpace(snapshot.InviteePreviewJson) == "" {
		snapshot.InviteePreviewJson = "[]"
	}
	if strings.TrimSpace(snapshot.AccessControlStatusJson) == "" {
		snapshot.AccessControlStatusJson = "{}"
	}
	if strings.TrimSpace(snapshot.CommunityGateStatusJson) == "" {
		snapshot.CommunityGateStatusJson = "{}"
	}
	if strings.TrimSpace(snapshot.IdentityBindingsJson) == "" {
		snapshot.IdentityBindingsJson = "[]"
	}
	if strings.TrimSpace(snapshot.AgentChatBindingsJson) == "" {
		snapshot.AgentChatBindingsJson = "[]"
	}
	if strings.TrimSpace(snapshot.ChatMembershipStatesJson) == "" {
		snapshot.ChatMembershipStatesJson = "[]"
	}
	if strings.TrimSpace(snapshot.ProfileJSON) == "" {
		snapshot.ProfileJSON = "{}"
	}
	now := time.Now().Unix()
	if snapshot.CreatedAt == 0 {
		snapshot.CreatedAt = now
	}
	if snapshot.ProfileSyncedAt == 0 {
		snapshot.ProfileSyncedAt = now
	}
	snapshot.UpdatedAt = now
	return DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "site_id"}, {Name: "user_id"}},
		DoUpdates: clause.Assignments(map[string]any{
			"username":                      snapshot.Username,
			"display_name":                  snapshot.DisplayName,
			"email":                         snapshot.Email,
			"base_group":                    snapshot.BaseGroup,
			"role":                          snapshot.Role,
			"status":                        snapshot.Status,
			"quota":                         snapshot.Quota,
			"used_quota":                    snapshot.UsedQuota,
			"request_count":                 snapshot.RequestCount,
			"last_login_at":                 snapshot.LastLoginAt,
			"created_at_unix":               snapshot.CreatedAtUnix,
			"inviter_id":                    snapshot.InviterId,
			"inviter_username":              snapshot.InviterUsername,
			"inviter_display_name":          snapshot.InviterDisplayName,
			"invitee_count":                 snapshot.InviteeCount,
			"aff_count":                     snapshot.AffCount,
			"aff_quota":                     snapshot.AffQuota,
			"aff_history_quota":             snapshot.AffHistoryQuota,
			"community_bound":               snapshot.CommunityBound,
			"has_community_o_auth_binding":  snapshot.HasCommunityOAuthBinding,
			"has_community_room_membership": snapshot.HasCommunityRoomMembership,
			"community_room_ids":            snapshot.CommunityRoomIds,
			"community_external_user_id":    snapshot.CommunityExternalUserId,
			"community_username":            snapshot.CommunityUsername,
			"qq_bound":                      snapshot.QQBound,
			"qq_external_user_id":           snapshot.QQExternalUserId,
			"qq_username":                   snapshot.QQUsername,
			"qq_bound_group_ids":            snapshot.QQBoundGroupIds,
			"tg_bound":                      snapshot.TGBound,
			"tg_external_user_id":           snapshot.TGExternalUserId,
			"tg_username":                   snapshot.TGUsername,
			"tg_bound_group_ids":            snapshot.TGBoundGroupIds,
			"access_level":                  snapshot.AccessLevel,
			"effective_groups":              snapshot.EffectiveGroups,
			"reason_code":                   snapshot.ReasonCode,
			"reason_message":                snapshot.ReasonMessage,
			"primary_bound":                 snapshot.PrimaryBound,
			"primary_platform":              snapshot.PrimaryPlatform,
			"matched_primary_group_id":      snapshot.MatchedPrimaryGroupId,
			"manual_override_mode":          snapshot.ManualOverrideMode,
			"manual_override_groups":        snapshot.ManualOverrideGroups,
			"manual_override_reason":        snapshot.ManualOverrideReason,
			"active_frozen_key_count":       snapshot.ActiveFrozenKeyCount,
			"access_control_frozen_keys":    snapshot.AccessControlFrozenKeys,
			"community_gate_frozen_keys":    snapshot.CommunityGateFrozenKeys,
			"has_active_frozen_keys":        snapshot.HasActiveFrozenKeys,
			"can_restore":                   snapshot.CanRestore,
			"community_site_id":             snapshot.CommunitySiteId,
			"last_login_ip":                 snapshot.LastLoginIP,
			"last_login_source":             snapshot.LastLoginSource,
			"last_login_ip_at":              snapshot.LastLoginIPAt,
			"invitee_preview_json":          snapshot.InviteePreviewJson,
			"access_control_status_json":    snapshot.AccessControlStatusJson,
			"community_gate_status_json":    snapshot.CommunityGateStatusJson,
			"identity_bindings_json":        snapshot.IdentityBindingsJson,
			"agent_chat_bindings_json":      snapshot.AgentChatBindingsJson,
			"chat_membership_states_json":   snapshot.ChatMembershipStatesJson,
			"profile_json":                  snapshot.ProfileJSON,
			"profile_synced_at":             snapshot.ProfileSyncedAt,
			"updated_at":                    snapshot.UpdatedAt,
		}),
	}).Create(snapshot).Error
}

func GetUserOpsProfileSnapshot(siteID string, userID int) (*UserOpsProfileSnapshot, error) {
	var row UserOpsProfileSnapshot
	err := DB.Where("site_id = ? AND user_id = ?", strings.TrimSpace(siteID), userID).First(&row).Error
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func ListUserOpsProfileSnapshots(siteID string, userIDs []int) ([]UserOpsProfileSnapshot, error) {
	rows := make([]UserOpsProfileSnapshot, 0)
	if len(userIDs) == 0 {
		return rows, nil
	}
	err := DB.Where("site_id = ? AND user_id IN ?", strings.TrimSpace(siteID), userIDs).
		Order("updated_at desc, id desc").
		Find(&rows).Error
	return rows, err
}
