package operation_setting

import "github.com/QuantumNous/new-api/setting/config"

// AgentSetting stores site-local operations-agent settings.
// Each New API site owns an independent copy through the options table.
type AgentSetting struct {
	Enabled bool `json:"enabled"`

	SiteID        string `json:"site_id"`
	SiteName      string `json:"site_name"`
	PublicBaseURL string `json:"public_base_url"`
	APIBaseURL    string `json:"api_base_url"`

	LLMProvider     string `json:"llm_provider"`
	LLMModel        string `json:"llm_model"`
	LLMBaseURL      string `json:"llm_base_url"`
	LLMAPIKey       string `json:"llm_api_key"`
	PlannerProvider string `json:"planner_provider"`
	HermesBaseURL   string `json:"hermes_base_url"`
	HermesAPIKey    string `json:"hermes_api_key"`

	DirectorEnabled      bool `json:"director_enabled"`
	CommunityEnabled     bool `json:"community_enabled"`
	GrowthEnabled        bool `json:"growth_enabled"`
	ActivityEnabled      bool `json:"activity_enabled"`
	GameEnabled          bool `json:"game_enabled"`
	RiskEnabled          bool `json:"risk_enabled"`
	OpsEnabled           bool `json:"ops_enabled"`
	BudgetEnabled        bool `json:"budget_enabled"`
	AutoExecuteLowRisk   bool `json:"auto_execute_low_risk"`
	HumanApprovalEnabled bool `json:"human_approval_enabled"`

	DailyBudgetQuota        int  `json:"daily_budget_quota"`
	GrowthBudgetQuota       int  `json:"growth_budget_quota"`
	ActivityBudgetQuota     int  `json:"activity_budget_quota"`
	GameBudgetQuota         int  `json:"game_budget_quota"`
	OpsCompBudgetQuota      int  `json:"ops_comp_budget_quota"`
	CommunityBudgetQuota    int  `json:"community_budget_quota"`
	DailyBudgetResetEnabled bool `json:"daily_budget_reset_enabled"`
	DailyFundResetEnabled   bool `json:"daily_fund_reset_enabled"`
	OpsFundDailyTargetQuota int  `json:"ops_fund_daily_target_quota"`
	SingleActionLimitQuota  int  `json:"single_action_limit_quota"`
	UserDailyLimitQuota     int  `json:"user_daily_limit_quota"`
	ApprovalThresholdQuota  int  `json:"approval_threshold_quota"`

	RiskDenyThreshold   int `json:"risk_deny_threshold"`
	RiskReviewThreshold int `json:"risk_review_threshold"`
	MinMessageChars     int `json:"min_message_chars"`
	MinDistinctMessages int `json:"min_distinct_messages"`

	QQBotEnabled  bool   `json:"qq_bot_enabled"`
	QQOneBotURL   string `json:"qq_onebot_url"`
	QQGroupID     string `json:"qq_group_id"`
	QQAccessToken string `json:"qq_access_token"`

	TGBotEnabled bool   `json:"tg_bot_enabled"`
	TGBotToken   string `json:"tg_bot_token"`
	TGChatID     string `json:"tg_chat_id"`

	ChatOpsEnabled                bool   `json:"chatops_enabled"`
	ChatOpsWebhookSecret          string `json:"chatops_webhook_secret"`
	ChatOpsAdminExternalIDs       string `json:"chatops_admin_external_ids"`
	ChatOpsCommandPrefixes        string `json:"chatops_command_prefixes"`
	ChatOpsAutoReply              bool   `json:"chatops_auto_reply"`
	ChatOpsAllowNaturalLanguage   bool   `json:"chatops_allow_natural_language"`
	ChatOpsRequireAdminForOps     bool   `json:"chatops_require_admin_for_ops"`
	ChatOpsTrustGroupAdmin        bool   `json:"chatops_trust_group_admin"`
	ChatOpsGroupModerationEnabled bool   `json:"chatops_group_moderation_enabled"`
	ChatOpsAllowDirectModeration  bool   `json:"chatops_allow_direct_moderation"`
	ChatOpsAllowKick              bool   `json:"chatops_allow_kick"`
	ChatOpsAllowBan               bool   `json:"chatops_allow_ban"`
	ChatOpsMaxMuteSeconds         int    `json:"chatops_max_mute_seconds"`
	LegacyConfigImportEnabled     bool   `json:"legacy_config_import_enabled"`
	LegacyConfigImportReasons     string `json:"legacy_config_import_reasons"`

	MemoryAutoCaptureEnabled  bool `json:"memory_auto_capture_enabled"`
	MemoryNoiseTTLSeconds     int  `json:"memory_noise_ttl_seconds"`
	MemoryCandidateTTLSeconds int  `json:"memory_candidate_ttl_seconds"`
	MemoryCoreTTLSeconds      int  `json:"memory_core_ttl_seconds"`
	MemoryRiskTTLSeconds      int  `json:"memory_risk_ttl_seconds"`
	MemoryNoiseSampleRate     int  `json:"memory_noise_sample_rate"`

	CommunityRoomID string `json:"community_room_id"`
	CommunityHost   string `json:"community_host"`

	SystemPrompt    string `json:"system_prompt"`
	SiteKnowledge   string `json:"site_knowledge"`
	WelcomeTemplate string `json:"welcome_template"`
	ActivityPolicy  string `json:"activity_policy"`
	RiskPolicy      string `json:"risk_policy"`
}

var agentSetting = AgentSetting{
	Enabled:                       false,
	SiteID:                        "default",
	SiteName:                      "New API",
	PublicBaseURL:                 "",
	APIBaseURL:                    "",
	LLMProvider:                   "openai-compatible",
	LLMModel:                      "gpt-4.1-mini",
	LLMBaseURL:                    "",
	LLMAPIKey:                     "",
	PlannerProvider:               "builtin",
	HermesBaseURL:                 "",
	HermesAPIKey:                  "",
	DirectorEnabled:               true,
	CommunityEnabled:              true,
	GrowthEnabled:                 true,
	ActivityEnabled:               true,
	GameEnabled:                   true,
	RiskEnabled:                   true,
	OpsEnabled:                    true,
	BudgetEnabled:                 true,
	AutoExecuteLowRisk:            false,
	HumanApprovalEnabled:          true,
	DailyBudgetQuota:              25000000,
	GrowthBudgetQuota:             10000000,
	ActivityBudgetQuota:           10000000,
	GameBudgetQuota:               5000000,
	OpsCompBudgetQuota:            5000000,
	CommunityBudgetQuota:          10000000,
	DailyBudgetResetEnabled:       true,
	DailyFundResetEnabled:         true,
	OpsFundDailyTargetQuota:       0,
	SingleActionLimitQuota:        2000000,
	UserDailyLimitQuota:           1000000,
	ApprovalThresholdQuota:        5000000,
	RiskDenyThreshold:             85,
	RiskReviewThreshold:           60,
	MinMessageChars:               6,
	MinDistinctMessages:           3,
	QQBotEnabled:                  false,
	QQOneBotURL:                   "",
	QQGroupID:                     "",
	QQAccessToken:                 "",
	TGBotEnabled:                  false,
	TGBotToken:                    "",
	TGChatID:                      "",
	ChatOpsEnabled:                false,
	ChatOpsWebhookSecret:          "",
	ChatOpsAdminExternalIDs:       "",
	ChatOpsCommandPrefixes:        "/agent,@agent",
	ChatOpsAutoReply:              true,
	ChatOpsAllowNaturalLanguage:   true,
	ChatOpsRequireAdminForOps:     true,
	ChatOpsTrustGroupAdmin:        false,
	ChatOpsGroupModerationEnabled: true,
	ChatOpsAllowDirectModeration:  true,
	ChatOpsAllowKick:              true,
	ChatOpsAllowBan:               false,
	ChatOpsMaxMuteSeconds:         3600,
	LegacyConfigImportEnabled:     false,
	LegacyConfigImportReasons:     "",
	MemoryAutoCaptureEnabled:      true,
	MemoryNoiseTTLSeconds:         86400,
	MemoryCandidateTTLSeconds:     7 * 86400,
	MemoryCoreTTLSeconds:          0,
	MemoryRiskTTLSeconds:          90 * 86400,
	MemoryNoiseSampleRate:         10,
	CommunityRoomID:               "",
	CommunityHost:                 "https://dc.hhhl.cc",
	SystemPrompt:                  "你是站点运营、社区和运维 Agent。所有动作必须遵守预算、风控和审批策略；高风险动作只提交审批建议。",
	SiteKnowledge:                 "",
	WelcomeTemplate:               "欢迎加入社区。可查看站点公告、模型列表、价格说明和活动规则。",
	ActivityPolicy:                "签到、抽奖、小游戏和社区活动需要先检查预算；单用户奖励受每日上限限制。",
	RiskPolicy:                    "过滤重复刷屏、低质量水贴、多账号薅羊毛、异常邀请和异常消耗；达到复核阈值进入人工审批。",
}

func init() {
	config.GlobalConfig.Register("agent_setting", &agentSetting)
}

func GetAgentSetting() *AgentSetting {
	return &agentSetting
}
