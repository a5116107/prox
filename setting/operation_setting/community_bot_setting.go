package operation_setting

import "github.com/QuantumNous/new-api/setting/config"

type CommunityBotSetting struct {
	Enabled bool `json:"enabled"`

	CommunityHost    string `json:"community_host"`
	ProviderSlug     string `json:"provider_slug"`
	RoomID           string `json:"room_id"`
	OAuthCallbackURL string `json:"oauth_callback_url"`

	OAuthClientID       string `json:"oauth_client_id"`
	OAuthClientSecret   string `json:"oauth_client_secret"`
	OAuthStateSecret    string `json:"oauth_state_secret"`
	OAuthVerifierSecret string `json:"oauth_verifier_secret"`
	BotToken            string `json:"bot_token"`
	BotUserID           string `json:"bot_user_id"`
	BotUsername         string `json:"bot_username"`

	AutoInviteEnabled  bool `json:"auto_invite_enabled"`
	InviteOnOAuthLogin bool `json:"invite_on_oauth_login"`

	JoinRewardEnabled  bool `json:"join_reward_enabled"`
	JoinRewardMinQuota int  `json:"join_reward_min_quota"`
	JoinRewardMaxQuota int  `json:"join_reward_max_quota"`

	DailyMessageRewardEnabled bool `json:"daily_message_reward_enabled"`
	DailyMessageThreshold     int  `json:"daily_message_threshold"`
	DailyRewardMinQuota       int  `json:"daily_reward_min_quota"`
	DailyRewardMaxQuota       int  `json:"daily_reward_max_quota"`
	DailyMaxRewardsPerUser    int  `json:"daily_max_rewards_per_user"`

	MessageScanIntervalMinutes int  `json:"message_scan_interval_minutes"`
	MessageScanIntervalSeconds int  `json:"message_scan_interval_seconds"` // 优先于分钟级；用于命令近实时响应（如 30 秒）。最小 10 秒。
	StreamingEnabled           bool `json:"streaming_enabled"`             // 启用 Misskey streaming 实时监听（亚秒级响应）；轮询作为兜底保留。
	CommandBurnAfterSeconds    int  `json:"command_burn_after_seconds"`    // 签到/验牌结果及用户命令消息阅后即焚秒数，<=0 关闭。
	MessageLookbackMinutes     int  `json:"message_lookback_minutes"`
	MessageScanLimit           int  `json:"message_scan_limit"`

	// 签到/验牌文案模板（两站可各自定制，实现差异化）。支持占位符：
	// {username} {amount} 本次额度 {balance} 当前额度 {key} API Key 状态 {quota} 额度状态 {tip} 引导文案
	CheckinSuccessTemplate string `json:"checkin_success_template"` // 签到成功
	CheckinFailedTemplate  string `json:"checkin_failed_template"`  // 签到失败（含今日已签）
	VerifyPassTemplate     string `json:"verify_pass_template"`     // 验牌通过
	VerifyFailedTemplate   string `json:"verify_failed_template"`   // 验牌未通过
	BindGuideTemplate      string `json:"bind_guide_template"`      // 未绑定引导

	AntiSpamMinChars         int  `json:"anti_spam_min_chars"`
	AntiSpamMinDistinctTexts int  `json:"anti_spam_min_distinct_texts"`
	AntiSpamIgnoreBot        bool `json:"anti_spam_ignore_bot"`

	NotificationEnabled        bool   `json:"notification_enabled"`
	NotifyOnInvite             bool   `json:"notify_on_invite"`
	NotifyOnJoinReward         bool   `json:"notify_on_join_reward"`
	NotifyOnDailyReward        bool   `json:"notify_on_daily_reward"`
	NotifyOnOpsAlert           bool   `json:"notify_on_ops_alert"`
	InviteNotificationTemplate string `json:"invite_notification_template"`
	JoinRewardTemplate         string `json:"join_reward_template"`
	DailyRewardTemplate        string `json:"daily_reward_template"`
	OpsAlertTemplate           string `json:"ops_alert_template"`
}

var communityBotSetting = CommunityBotSetting{
	Enabled:          false,
	CommunityHost:    "https://dc.hhhl.cc",
	ProviderSlug:     "dc.hhhl.cc",
	RoomID:           "",
	OAuthCallbackURL: "",

	OAuthClientID:       "",
	OAuthClientSecret:   "",
	OAuthStateSecret:    "",
	OAuthVerifierSecret: "",
	BotToken:            "",
	BotUserID:           "",
	BotUsername:         "",

	AutoInviteEnabled:  true,
	InviteOnOAuthLogin: true,

	JoinRewardEnabled:  false,
	JoinRewardMinQuota: 2500000,
	JoinRewardMaxQuota: 2500000,

	DailyMessageRewardEnabled: true,
	DailyMessageThreshold:     5,
	DailyRewardMinQuota:       500000,
	DailyRewardMaxQuota:       500000,
	DailyMaxRewardsPerUser:    1,

	MessageScanIntervalMinutes: 5,
	MessageScanIntervalSeconds: 30, // 命令近实时响应：每 30 秒扫描一次（优先于分钟级）。
	StreamingEnabled:           true, // 默认启用 streaming 实时监听（亚秒级）；轮询兜底。
	CommandBurnAfterSeconds:    15,   // 默认 15 秒后撤回签到/验牌结果与用户命令，保持房间清爽。
	MessageLookbackMinutes:     1440,
	MessageScanLimit:           100,

	AntiSpamMinChars:         2,
	AntiSpamMinDistinctTexts: 3,
	AntiSpamIgnoreBot:        true,

	NotificationEnabled:        true,
	NotifyOnInvite:             true,
	NotifyOnJoinReward:         true,
	NotifyOnDailyReward:        true,
	NotifyOnOpsAlert:           true,
	InviteNotificationTemplate: "已邀请 @{username} 加入社区群聊，欢迎来群里交流。",
	JoinRewardTemplate:         "欢迎 @{username} 加入社区群聊，已发放社区奖励 {amount}。",
	DailyRewardTemplate:        "@{username} 今日有效发言已达 {count} 条，已自动发放 {amount}。",
	OpsAlertTemplate:           "社区管家提醒：{message}",
}

func init() {
	config.GlobalConfig.Register("community_bot_setting", &communityBotSetting)
}

func GetCommunityBotSetting() *CommunityBotSetting {
	return &communityBotSetting
}
