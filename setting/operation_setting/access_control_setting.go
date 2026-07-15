package operation_setting

import "github.com/QuantumNous/new-api/setting/config"

const defaultAccessControlStateCacheTTLSeconds = 900

type AccessControlSetting struct {
	Enabled bool `json:"enabled"`

	PrimaryPlatform   string   `json:"primary_platform"`
	PrimaryGroupIDs   []string `json:"primary_group_ids"`
	CommunityGroupIDs []string `json:"community_group_ids"`

	CommunityOnlyGroups []string `json:"community_only_groups"`
	FullAccessGroups    []string `json:"full_access_groups"`
	PaidBypassGroups    []string `json:"paid_bypass_groups"`
	PaidUserGroups      []string `json:"paid_user_groups"`

	AllowPaidBypass            bool `json:"allow_paid_bypass"`
	AllowAdminBypass           bool `json:"allow_admin_bypass"`
	CheckOnLogin               bool `json:"check_on_login"`
	BlockTokenCreate           bool `json:"block_token_create"`
	BlockTokenEnable           bool `json:"block_token_enable"`
	EnforceRequestTime         bool `json:"enforce_request_time"`
	FreezeLegacyTokens         bool `json:"freeze_legacy_tokens"`
	AutoRestoreCompliantTokens bool `json:"auto_restore_compliant_tokens"`
	StateCacheTTLSeconds       int  `json:"state_cache_ttl_seconds"`

	CommunityJoinURL string `json:"community_join_url"`
	PrimaryJoinURL   string `json:"primary_join_url"`
	DenyMessage      string `json:"deny_message"`
	UpgradeMessage   string `json:"upgrade_message"`

	RewardSoftFloorQuota int `json:"reward_soft_floor_quota"`
	RewardHardFloorQuota int `json:"reward_hard_floor_quota"`
	DailySiteRewardCap   int `json:"daily_site_reward_cap"`
	DailyUserRewardCap   int `json:"daily_user_reward_cap"`
}

var accessControlSetting = AccessControlSetting{
	Enabled: true,

	PrimaryPlatform:   "qq",
	PrimaryGroupIDs:   []string{},
	CommunityGroupIDs: []string{},

	CommunityOnlyGroups: []string{},
	FullAccessGroups:    []string{},
	PaidBypassGroups:    []string{},
	PaidUserGroups:      []string{},

	AllowPaidBypass:            false,
	AllowAdminBypass:           true,
	CheckOnLogin:               true,
	BlockTokenCreate:           true,
	BlockTokenEnable:           true,
	EnforceRequestTime:         true,
	FreezeLegacyTokens:         true,
	AutoRestoreCompliantTokens: true,
	StateCacheTTLSeconds:       defaultAccessControlStateCacheTTLSeconds,

	CommunityJoinURL: "",
	PrimaryJoinURL:   "",
	DenyMessage:      "请先完成社区授权绑定；仅绑定社区账号的用户只能使用社区分组，绑定主群账号后可解锁全站分组。",
	UpgradeMessage:   "完成主群绑定后可解锁全站分组与完整权益。",

	RewardSoftFloorQuota: 0,
	RewardHardFloorQuota: 0,
	DailySiteRewardCap:   0,
	DailyUserRewardCap:   0,
}

func init() {
	config.GlobalConfig.Register("access_control_setting", &accessControlSetting)
}

func GetAccessControlSetting() *AccessControlSetting {
	if accessControlSetting.StateCacheTTLSeconds <= 0 {
		accessControlSetting.StateCacheTTLSeconds = defaultAccessControlStateCacheTTLSeconds
	}
	if accessControlSetting.PrimaryPlatform == "" {
		accessControlSetting.PrimaryPlatform = "qq"
	}
	return &accessControlSetting
}
