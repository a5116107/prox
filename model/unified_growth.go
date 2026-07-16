package model

// ChatGroup records each QQ/TG/community room that participates in operations.
type ChatGroup struct {
	Id                  int    `json:"id"`
	SiteId              string `json:"site_id" gorm:"type:varchar(64);not null;uniqueIndex:ux_chat_group_site_platform_group;index"`
	Platform            string `json:"platform" gorm:"type:varchar(32);not null;uniqueIndex:ux_chat_group_site_platform_group;index"`
	GroupId             string `json:"group_id" gorm:"type:varchar(128);not null;uniqueIndex:ux_chat_group_site_platform_group;index"`
	GroupName           string `json:"group_name" gorm:"type:varchar(128)"`
	InviteTargetGroupId string `json:"invite_target_group_id" gorm:"type:varchar(128);index"`
	Role                string `json:"role" gorm:"type:varchar(32);not null;default:secondary;index"`
	Status              string `json:"status" gorm:"type:varchar(32);not null;default:active;index"`
	Language            string `json:"language" gorm:"type:varchar(16);not null;default:zh-CN"`
	Timezone            string `json:"timezone" gorm:"type:varchar(64);not null;default:Asia/Shanghai"`
	ConfigJson          string `json:"config_json" gorm:"type:text"`
	CreatedAt           int64  `json:"created_at" gorm:"index"`
	UpdatedAt           int64  `json:"updated_at"`
}

func (ChatGroup) TableName() string { return "chat_groups" }

// GroupGameConfig is the group-level source of truth for game switches and rules.
type GroupGameConfig struct {
	Id         int    `json:"id"`
	SiteId     string `json:"site_id" gorm:"type:varchar(64);not null;uniqueIndex:ux_group_game_config;index"`
	Platform   string `json:"platform" gorm:"type:varchar(32);not null;uniqueIndex:ux_group_game_config;index"`
	GroupId    string `json:"group_id" gorm:"type:varchar(128);not null;uniqueIndex:ux_group_game_config;index"`
	GameCode   string `json:"game_code" gorm:"type:varchar(64);not null;uniqueIndex:ux_group_game_config;index"`
	Enabled    bool   `json:"enabled" gorm:"not null"`
	BudgetPool string `json:"budget_pool" gorm:"type:varchar(32);not null;default:game"`
	RuleJson   string `json:"rule_json" gorm:"type:text"`
	CreatedAt  int64  `json:"created_at" gorm:"index"`
	UpdatedAt  int64  `json:"updated_at"`
}

func (GroupGameConfig) TableName() string { return "group_game_configs" }

// GroupChatOpsConfig controls checkin/verify/invite behavior per group.
type GroupChatOpsConfig struct {
	Id                    int    `json:"id"`
	SiteId                string `json:"site_id" gorm:"type:varchar(64);not null;uniqueIndex:ux_group_chatops_config;index"`
	Platform              string `json:"platform" gorm:"type:varchar(32);not null;uniqueIndex:ux_group_chatops_config;index"`
	GroupId               string `json:"group_id" gorm:"type:varchar(128);not null;uniqueIndex:ux_group_chatops_config;index"`
	CheckinEnabled        bool   `json:"checkin_enabled" gorm:"not null"`
	VerifyEnabled         bool   `json:"verify_enabled" gorm:"not null"`
	InviteEnabled         bool   `json:"invite_enabled" gorm:"not null"`
	CheckinQuota          int    `json:"checkin_quota" gorm:"not null;default:0"`
	VerifyMinQuota        int    `json:"verify_min_quota" gorm:"not null;default:1"`
	InviteRewardQuota     int    `json:"invite_reward_quota" gorm:"not null;default:0"`
	InviteeRewardQuota    int    `json:"invitee_reward_quota" gorm:"not null;default:0"`
	DailyGroupRewardLimit int    `json:"daily_group_reward_limit" gorm:"not null;default:0"`
	RuleJson              string `json:"rule_json" gorm:"type:text"`
	CreatedAt             int64  `json:"created_at" gorm:"index"`
	UpdatedAt             int64  `json:"updated_at"`
}

func (GroupChatOpsConfig) TableName() string { return "group_chatops_configs" }

// GroupI18nTemplate keeps bot/admin copy in one place.
type GroupI18nTemplate struct {
	Id          int    `json:"id"`
	SiteId      string `json:"site_id" gorm:"type:varchar(64);not null;uniqueIndex:ux_group_i18n_template;index"`
	Platform    string `json:"platform" gorm:"type:varchar(32);not null;uniqueIndex:ux_group_i18n_template;index"`
	GroupId     string `json:"group_id" gorm:"type:varchar(128);not null;uniqueIndex:ux_group_i18n_template;index"`
	Locale      string `json:"locale" gorm:"type:varchar(16);not null;uniqueIndex:ux_group_i18n_template;index"`
	TemplateKey string `json:"template_key" gorm:"type:varchar(128);not null;uniqueIndex:ux_group_i18n_template;index"`
	Template    string `json:"template" gorm:"type:text"`
	CreatedAt   int64  `json:"created_at" gorm:"index"`
	UpdatedAt   int64  `json:"updated_at"`
}

func (GroupI18nTemplate) TableName() string { return "group_i18n_templates" }

// UserIdentityBinding is the unified identity map for QQ/TG/community OAuth.
type UserIdentityBinding struct {
	Id             int    `json:"id"`
	SiteId         string `json:"site_id" gorm:"type:varchar(64);not null;uniqueIndex:ux_identity_provider_external;index"`
	UserId         int    `json:"user_id" gorm:"not null;index"`
	Provider       string `json:"provider" gorm:"type:varchar(32);not null;uniqueIndex:ux_identity_provider_external;index"`
	ExternalUserId string `json:"external_user_id" gorm:"type:varchar(128);not null;uniqueIndex:ux_identity_provider_external;index"`
	Username       string `json:"username" gorm:"type:varchar(128)"`
	BindCodeHash   string `json:"-" gorm:"type:varchar(128);index"`
	Status         string `json:"status" gorm:"type:varchar(32);not null;default:active;index"`
	BoundAt        int64  `json:"bound_at" gorm:"index"`
	LastSeenAt     int64  `json:"last_seen_at" gorm:"index"`
	CreatedAt      int64  `json:"created_at" gorm:"index"`
	UpdatedAt      int64  `json:"updated_at"`
}

func (UserIdentityBinding) TableName() string { return "user_identity_bindings" }

// InviteCampaign defines a growth campaign and its target primary room.
type InviteCampaign struct {
	Id             int    `json:"id"`
	SiteId         string `json:"site_id" gorm:"type:varchar(64);not null;index"`
	CampaignCode   string `json:"campaign_code" gorm:"type:varchar(64);not null;uniqueIndex:ux_invite_campaign"`
	Name           string `json:"name" gorm:"type:varchar(128)"`
	SourcePlatform string `json:"source_platform" gorm:"type:varchar(32);index"`
	SourceGroupId  string `json:"source_group_id" gorm:"type:varchar(128);index"`
	TargetPlatform string `json:"target_platform" gorm:"type:varchar(32);index"`
	TargetGroupId  string `json:"target_group_id" gorm:"type:varchar(128);index"`
	Status         string `json:"status" gorm:"type:varchar(32);not null;default:active;index"`
	RuleJson       string `json:"rule_json" gorm:"type:text"`
	CreatedAt      int64  `json:"created_at" gorm:"index"`
	UpdatedAt      int64  `json:"updated_at"`
}

func (InviteCampaign) TableName() string { return "invite_campaigns" }

type InviteLink struct {
	Id             int    `json:"id"`
	SiteId         string `json:"site_id" gorm:"type:varchar(64);not null;uniqueIndex:ux_invite_link;index"`
	CampaignId     int    `json:"campaign_id" gorm:"not null;uniqueIndex:ux_invite_link;index"`
	InviterUserId  int    `json:"inviter_user_id" gorm:"not null;uniqueIndex:ux_invite_link;index"`
	Provider       string `json:"provider" gorm:"type:varchar(32);not null;index"`
	ExternalUserId string `json:"external_user_id" gorm:"type:varchar(128);index"`
	InviteCode     string `json:"invite_code" gorm:"type:varchar(128);not null;uniqueIndex;index"`
	InviteUrl      string `json:"invite_url" gorm:"type:text"`
	Status         string `json:"status" gorm:"type:varchar(32);not null;default:active;index"`
	CreatedAt      int64  `json:"created_at" gorm:"index"`
	UpdatedAt      int64  `json:"updated_at"`
}

func (InviteLink) TableName() string { return "invite_links" }

type InviteEdge struct {
	Id                int    `json:"id"`
	SiteId            string `json:"site_id" gorm:"type:varchar(64);not null;uniqueIndex:ux_invite_edge;index"`
	CampaignId        int    `json:"campaign_id" gorm:"not null;index"`
	InviteLinkId      int    `json:"invite_link_id" gorm:"index"`
	InviterUserId     int    `json:"inviter_user_id" gorm:"not null;index"`
	InviteeUserId     int    `json:"invitee_user_id" gorm:"uniqueIndex:ux_invite_edge;index"`
	InviteeProvider   string `json:"invitee_provider" gorm:"type:varchar(32);uniqueIndex:ux_invite_edge;index"`
	InviteeExternalId string `json:"invitee_external_id" gorm:"type:varchar(128);uniqueIndex:ux_invite_edge;index"`
	Stage             string `json:"stage" gorm:"type:varchar(32);not null;default:join;index"`
	Status            string `json:"status" gorm:"type:varchar(32);not null;default:pending;index"`
	CreatedAt         int64  `json:"created_at" gorm:"index"`
	UpdatedAt         int64  `json:"updated_at"`
}

func (InviteEdge) TableName() string { return "invite_edges" }

type InviteEvent struct {
	Id             int    `json:"id"`
	SiteId         string `json:"site_id" gorm:"type:varchar(64);not null;index"`
	CampaignId     int    `json:"campaign_id" gorm:"index"`
	InviteLinkId   int    `json:"invite_link_id" gorm:"index"`
	InviteEdgeId   int    `json:"invite_edge_id" gorm:"index"`
	EventType      string `json:"event_type" gorm:"type:varchar(32);not null;index"`
	Provider       string `json:"provider" gorm:"type:varchar(32);index"`
	ExternalUserId string `json:"external_user_id" gorm:"type:varchar(128);index"`
	UserId         int    `json:"user_id" gorm:"index"`
	GroupId        string `json:"group_id" gorm:"type:varchar(128);index"`
	MetadataJson   string `json:"metadata_json" gorm:"type:text"`
	CreatedAt      int64  `json:"created_at" gorm:"index"`
}

func (InviteEvent) TableName() string { return "invite_events" }

type InviteRewardClaim struct {
	Id              int    `json:"id"`
	SiteId          string `json:"site_id" gorm:"type:varchar(64);not null;index"`
	CampaignId      int    `json:"campaign_id" gorm:"index"`
	InviteEdgeId    int    `json:"invite_edge_id" gorm:"not null;uniqueIndex:ux_invite_reward_stage;index"`
	RewardStage     string `json:"reward_stage" gorm:"type:varchar(32);not null;uniqueIndex:ux_invite_reward_stage;index"`
	RewardUserId    int    `json:"reward_user_id" gorm:"not null;index"`
	Quota           int    `json:"quota" gorm:"not null"`
	Status          string `json:"status" gorm:"type:varchar(32);not null;default:pending;index"`
	OpsFundLedgerId int    `json:"ops_fund_ledger_id" gorm:"index"`
	Error           string `json:"error" gorm:"type:text"`
	CreatedAt       int64  `json:"created_at" gorm:"index"`
	UpdatedAt       int64  `json:"updated_at"`
}

func (InviteRewardClaim) TableName() string { return "invite_reward_claims" }

type InviteRiskFlag struct {
	Id           int    `json:"id"`
	SiteId       string `json:"site_id" gorm:"type:varchar(64);not null;index"`
	InviteEdgeId int    `json:"invite_edge_id" gorm:"index"`
	FlagType     string `json:"flag_type" gorm:"type:varchar(64);not null;index"`
	Severity     string `json:"severity" gorm:"type:varchar(16);not null;default:warning;index"`
	Status       string `json:"status" gorm:"type:varchar(32);not null;default:open;index"`
	DetailJson   string `json:"detail_json" gorm:"type:text"`
	CreatedAt    int64  `json:"created_at" gorm:"index"`
	UpdatedAt    int64  `json:"updated_at"`
}

func (InviteRiskFlag) TableName() string { return "invite_risk_flags" }

type GameRound struct {
	Id         int    `json:"id"`
	SiteId     string `json:"site_id" gorm:"type:varchar(64);not null;index"`
	Platform   string `json:"platform" gorm:"type:varchar(32);not null;index"`
	GroupId    string `json:"group_id" gorm:"type:varchar(128);not null;index"`
	GameCode   string `json:"game_code" gorm:"type:varchar(64);not null;index"`
	RoundKey   string `json:"round_key" gorm:"type:varchar(128);not null;uniqueIndex"`
	Status     string `json:"status" gorm:"type:varchar(32);not null;default:open;index"`
	RandomSeed string `json:"random_seed" gorm:"type:varchar(128)"`
	RuleJson   string `json:"rule_json" gorm:"type:text"`
	ResultJson string `json:"result_json" gorm:"type:text"`
	OpenedAt   int64  `json:"opened_at" gorm:"index"`
	ClosedAt   int64  `json:"closed_at" gorm:"index"`
	CreatedAt  int64  `json:"created_at" gorm:"index"`
	UpdatedAt  int64  `json:"updated_at"`
}

func (GameRound) TableName() string { return "game_rounds" }

type GameEntry struct {
	Id               int    `json:"id"`
	SiteId           string `json:"site_id" gorm:"type:varchar(64);not null;index"`
	RoundId          int    `json:"round_id" gorm:"not null;index"`
	UserId           int    `json:"user_id" gorm:"not null;index"`
	ExternalUserId   string `json:"external_user_id" gorm:"type:varchar(128);index"`
	StakeQuota       int    `json:"stake_quota" gorm:"not null;default:0"`
	EntryPayloadJson string `json:"entry_payload_json" gorm:"type:text"`
	Status           string `json:"status" gorm:"type:varchar(32);not null;default:entered;index"`
	CreatedAt        int64  `json:"created_at" gorm:"index"`
	UpdatedAt        int64  `json:"updated_at"`
}

func (GameEntry) TableName() string { return "game_entries" }

type GameSettlement struct {
	Id              int    `json:"id"`
	SiteId          string `json:"site_id" gorm:"type:varchar(64);not null;index"`
	RoundId         int    `json:"round_id" gorm:"not null;index"`
	EntryId         int    `json:"entry_id" gorm:"index"`
	UserId          int    `json:"user_id" gorm:"not null;index"`
	SettlementKey   string `json:"settlement_key" gorm:"type:varchar(128);not null;uniqueIndex"`
	StakeQuota      int    `json:"stake_quota" gorm:"not null;default:0"`
	PayoutQuota     int    `json:"payout_quota" gorm:"not null;default:0"`
	CommissionQuota int    `json:"commission_quota" gorm:"not null;default:0"`
	RefundQuota     int    `json:"refund_quota" gorm:"not null;default:0"`
	OpsFundLedgerId int    `json:"ops_fund_ledger_id" gorm:"index"`
	Status          string `json:"status" gorm:"type:varchar(32);not null;default:pending;index"`
	MetadataJson    string `json:"metadata_json" gorm:"type:text"`
	CreatedAt       int64  `json:"created_at" gorm:"index"`
	UpdatedAt       int64  `json:"updated_at"`
}

func (GameSettlement) TableName() string { return "game_settlements" }

type GameCommission struct {
	Id              int    `json:"id"`
	SiteId          string `json:"site_id" gorm:"type:varchar(64);not null;index"`
	RoundId         int    `json:"round_id" gorm:"not null;index"`
	SettlementId    int    `json:"settlement_id" gorm:"index"`
	UserId          int    `json:"user_id" gorm:"index"`
	GameCode        string `json:"game_code" gorm:"type:varchar(64);not null;index"`
	GroupId         string `json:"group_id" gorm:"type:varchar(128);index"`
	CommissionQuota int    `json:"commission_quota" gorm:"not null"`
	RateBps         int    `json:"rate_bps" gorm:"not null;default:0"`
	OpsFundLedgerId int    `json:"ops_fund_ledger_id" gorm:"index"`
	CreatedAt       int64  `json:"created_at" gorm:"index"`
}

func (GameCommission) TableName() string { return "game_commissions" }

type GameJackpotAccount struct {
	Id           int    `json:"id"`
	SiteId       string `json:"site_id" gorm:"type:varchar(64);not null;uniqueIndex:ux_game_jackpot_account;index"`
	Platform     string `json:"platform" gorm:"type:varchar(32);not null;uniqueIndex:ux_game_jackpot_account;index"`
	GroupId      string `json:"group_id" gorm:"type:varchar(128);not null;uniqueIndex:ux_game_jackpot_account;index"`
	GameCode     string `json:"game_code" gorm:"type:varchar(64);not null;uniqueIndex:ux_game_jackpot_account;index"`
	BalanceQuota int    `json:"balance_quota" gorm:"not null;default:0"`
	Status       string `json:"status" gorm:"type:varchar(32);not null;default:active;index"`
	CreatedAt    int64  `json:"created_at" gorm:"index"`
	UpdatedAt    int64  `json:"updated_at"`
}

func (GameJackpotAccount) TableName() string { return "game_jackpot_accounts" }

type GameJackpotLedger struct {
	Id               int    `json:"id"`
	SiteId           string `json:"site_id" gorm:"type:varchar(64);not null;index"`
	JackpotAccountId int    `json:"jackpot_account_id" gorm:"not null;index"`
	RoundId          int    `json:"round_id" gorm:"index"`
	SettlementId     int    `json:"settlement_id" gorm:"index"`
	DeltaQuota       int    `json:"delta_quota" gorm:"not null"`
	BalanceAfter     int    `json:"balance_after" gorm:"not null"`
	SourceType       string `json:"source_type" gorm:"type:varchar(64);not null;index"`
	Remark           string `json:"remark" gorm:"type:text"`
	CreatedAt        int64  `json:"created_at" gorm:"index"`
}

func (GameJackpotLedger) TableName() string { return "game_jackpot_ledgers" }

type GroupMetricsDaily struct {
	Id              int    `json:"id"`
	SiteId          string `json:"site_id" gorm:"type:varchar(64);not null;uniqueIndex:ux_group_metrics_daily;index"`
	Platform        string `json:"platform" gorm:"type:varchar(32);not null;uniqueIndex:ux_group_metrics_daily;index"`
	GroupId         string `json:"group_id" gorm:"type:varchar(128);not null;uniqueIndex:ux_group_metrics_daily;index"`
	MetricDate      string `json:"metric_date" gorm:"type:varchar(16);not null;uniqueIndex:ux_group_metrics_daily;index"`
	InviteLinks     int    `json:"invite_links"`
	Joins           int    `json:"joins"`
	Binds           int    `json:"binds"`
	Verifies        int    `json:"verifies"`
	Checkins        int    `json:"checkins"`
	GamePlayers     int    `json:"game_players"`
	GameRounds      int    `json:"game_rounds"`
	StakeQuota      int    `json:"stake_quota"`
	PayoutQuota     int    `json:"payout_quota"`
	CommissionQuota int    `json:"commission_quota"`
	RewardCostQuota int    `json:"reward_cost_quota"`
	MetadataJson    string `json:"metadata_json" gorm:"type:text"`
	CreatedAt       int64  `json:"created_at" gorm:"index"`
	UpdatedAt       int64  `json:"updated_at"`
}

func (GroupMetricsDaily) TableName() string { return "group_metrics_daily" }

type AdminConfigAudit struct {
	Id          int    `json:"id"`
	SiteId      string `json:"site_id" gorm:"type:varchar(64);not null;index"`
	ActorUserId int    `json:"actor_user_id" gorm:"index"`
	ConfigScope string `json:"config_scope" gorm:"type:varchar(64);not null;index"`
	TargetKey   string `json:"target_key" gorm:"type:varchar(256);not null;index"`
	BeforeJson  string `json:"before_json" gorm:"type:text"`
	AfterJson   string `json:"after_json" gorm:"type:text"`
	Reason      string `json:"reason" gorm:"type:text"`
	CreatedAt   int64  `json:"created_at" gorm:"index"`
}

func (AdminConfigAudit) TableName() string { return "admin_config_audits" }
