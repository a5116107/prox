package operation_setting

import (
	"os"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/setting/config"
)

type MembershipRiskSetting struct {
	Enabled                         bool   `json:"enabled"`
	DryRun                          bool   `json:"dry_run"`
	GraceHours                      int    `json:"grace_hours"`
	AutoRestoreOnRejoin             bool   `json:"auto_restore_on_rejoin"`
	PaidBypassEnabled               bool   `json:"paid_bypass_enabled"`
	EventSecret                     string `json:"event_secret"`
	FreezeCommunityTokensAfterGrace bool   `json:"freeze_community_tokens_after_grace"`
	RevokeCommunityAccessAfterGrace bool   `json:"revoke_community_access_after_grace"`
	BlockCheckinOnLeft              bool   `json:"block_checkin_on_left"`
	BlockGameRewardOnLeft           bool   `json:"block_game_reward_on_left"`
	BlockInviteRewardOnLeft         bool   `json:"block_invite_reward_on_left"`
	BlockCampaignBonusOnLeft        bool   `json:"block_campaign_bonus_on_left"`
	NotifyUserOnLeft                bool   `json:"notify_user_on_left"`
	NotifyAdminOnBulkLeft           bool   `json:"notify_admin_on_bulk_left"`
	QQEventsEnabled                 bool   `json:"qq_events_enabled"`
	TGEventsEnabled                 bool   `json:"tg_events_enabled"`
	ScheduledRecheckEnabled         bool   `json:"scheduled_recheck_enabled"`
	ScheduledRecheckIntervalHours   int    `json:"scheduled_recheck_interval_hours"`
}

var membershipRiskSetting = MembershipRiskSetting{
	Enabled:                         false,
	DryRun:                          true,
	GraceHours:                      24,
	AutoRestoreOnRejoin:             true,
	PaidBypassEnabled:               true,
	FreezeCommunityTokensAfterGrace: true,
	RevokeCommunityAccessAfterGrace: true,
	BlockCheckinOnLeft:              true,
	BlockGameRewardOnLeft:           true,
	BlockInviteRewardOnLeft:         true,
	BlockCampaignBonusOnLeft:        true,
	NotifyUserOnLeft:                true,
	NotifyAdminOnBulkLeft:           true,
	QQEventsEnabled:                 true,
	TGEventsEnabled:                 true,
	ScheduledRecheckEnabled:         true,
	ScheduledRecheckIntervalHours:   12,
}

func init() {
	config.GlobalConfig.Register("membership_risk", &membershipRiskSetting)
}

func GetMembershipRiskSetting() *MembershipRiskSetting {
	applyMembershipRiskEnvOverrides(&membershipRiskSetting)
	if membershipRiskSetting.GraceHours <= 0 {
		membershipRiskSetting.GraceHours = 24
	}
	if membershipRiskSetting.ScheduledRecheckIntervalHours <= 0 {
		membershipRiskSetting.ScheduledRecheckIntervalHours = 12
	}
	return &membershipRiskSetting
}

func applyMembershipRiskEnvOverrides(target *MembershipRiskSetting) {
	if target == nil {
		return
	}
	overrideMembershipRiskBool(&target.Enabled, "MEMBERSHIP_RISK_ENABLED")
	overrideMembershipRiskBool(&target.DryRun, "MEMBERSHIP_RISK_DRY_RUN")
	overrideMembershipRiskInt(&target.GraceHours, "MEMBERSHIP_RISK_GRACE_HOURS")
	overrideMembershipRiskBool(&target.AutoRestoreOnRejoin, "MEMBERSHIP_RISK_AUTO_RESTORE_ON_REJOIN")
	overrideMembershipRiskBool(&target.PaidBypassEnabled, "MEMBERSHIP_RISK_PAID_BYPASS_ENABLED")
	overrideMembershipRiskString(&target.EventSecret, "MEMBERSHIP_RISK_EVENT_SECRET", "NEW_API_MEMBERSHIP_EVENT_SECRET", "CHAT_MEMBERSHIP_EVENT_SECRET")
	overrideMembershipRiskBool(&target.FreezeCommunityTokensAfterGrace, "MEMBERSHIP_RISK_FREEZE_COMMUNITY_TOKENS_AFTER_GRACE")
	overrideMembershipRiskBool(&target.RevokeCommunityAccessAfterGrace, "MEMBERSHIP_RISK_REVOKE_COMMUNITY_ACCESS_AFTER_GRACE")
	overrideMembershipRiskBool(&target.BlockCheckinOnLeft, "MEMBERSHIP_RISK_BLOCK_CHECKIN_ON_LEFT")
	overrideMembershipRiskBool(&target.BlockGameRewardOnLeft, "MEMBERSHIP_RISK_BLOCK_GAME_REWARD_ON_LEFT")
	overrideMembershipRiskBool(&target.BlockInviteRewardOnLeft, "MEMBERSHIP_RISK_BLOCK_INVITE_REWARD_ON_LEFT")
	overrideMembershipRiskBool(&target.BlockCampaignBonusOnLeft, "MEMBERSHIP_RISK_BLOCK_CAMPAIGN_BONUS_ON_LEFT")
	overrideMembershipRiskBool(&target.NotifyUserOnLeft, "MEMBERSHIP_RISK_NOTIFY_USER_ON_LEFT")
	overrideMembershipRiskBool(&target.NotifyAdminOnBulkLeft, "MEMBERSHIP_RISK_NOTIFY_ADMIN_ON_BULK_LEFT")
	overrideMembershipRiskBool(&target.QQEventsEnabled, "MEMBERSHIP_RISK_QQ_EVENTS_ENABLED")
	overrideMembershipRiskBool(&target.TGEventsEnabled, "MEMBERSHIP_RISK_TG_EVENTS_ENABLED")
	overrideMembershipRiskBool(&target.ScheduledRecheckEnabled, "MEMBERSHIP_RISK_SCHEDULED_RECHECK_ENABLED")
	overrideMembershipRiskInt(&target.ScheduledRecheckIntervalHours, "MEMBERSHIP_RISK_SCHEDULED_RECHECK_INTERVAL_HOURS")
}

func overrideMembershipRiskString(target *string, keys ...string) {
	if target == nil {
		return
	}
	for _, key := range keys {
		if value, ok := os.LookupEnv(key); ok {
			trimmed := strings.TrimSpace(value)
			if trimmed != "" {
				*target = trimmed
				return
			}
		}
	}
}

func overrideMembershipRiskBool(target *bool, key string) {
	if target == nil {
		return
	}
	value, ok := os.LookupEnv(key)
	if !ok {
		return
	}
	parsed, err := strconv.ParseBool(strings.TrimSpace(value))
	if err == nil {
		*target = parsed
	}
}

func overrideMembershipRiskInt(target *int, key string) {
	if target == nil {
		return
	}
	value, ok := os.LookupEnv(key)
	if !ok {
		return
	}
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err == nil {
		*target = parsed
	}
}
