package operation_setting

import "github.com/QuantumNous/new-api/setting/config"

type RiskControlSetting struct {
	Enabled bool `json:"enabled"`

	HighRiskKeyRecreateRequired bool   `json:"high_risk_key_recreate_required"`
	HighRiskActivationRequired  bool   `json:"high_risk_activation_required"`
	HighRiskActivationSource    string `json:"high_risk_activation_source"`
	ActivationCodeTTLMinutes    int    `json:"activation_code_ttl_minutes"`

	SameIPSameDayOAuthRegisterEnabled bool   `json:"same_ip_same_day_oauth_register_enabled"`
	SameIPSameDayOAuthRegisterLimit   int    `json:"same_ip_same_day_oauth_register_limit"`
	SameIPRegisterBlockMessage        string `json:"same_ip_register_block_message"`

	RequestIPTrackingEnabled bool `json:"request_ip_tracking_enabled"`

	SameIPMultiAccountUsageEnabled       bool   `json:"same_ip_multi_account_usage_enabled"`
	SameIPMultiAccountUsageWindowMinutes int    `json:"same_ip_multi_account_usage_window_minutes"`
	SameIPMultiAccountUsageUserLimit     int    `json:"same_ip_multi_account_usage_user_limit"`
	SameIPMultiAccountUsageBlockMessage  string `json:"same_ip_multi_account_usage_block_message"`

	DynamicIPChurnEnabled         bool   `json:"dynamic_ip_churn_enabled"`
	DynamicIPChurnWindowMinutes   int    `json:"dynamic_ip_churn_window_minutes"`
	DynamicIPChurnDistinctIPLimit int    `json:"dynamic_ip_churn_distinct_ip_limit"`
	DynamicIPChurnBlockMessage    string `json:"dynamic_ip_churn_block_message"`

	BurstRegisterEnabled       bool   `json:"burst_register_enabled"`
	BurstRegisterWindowMinutes int    `json:"burst_register_window_minutes"`
	BurstRegisterLimit         int    `json:"burst_register_limit"`
	BurstRegisterBlockMessage  string `json:"burst_register_block_message"`

	InactiveTokenDisableEnabled bool   `json:"inactive_token_disable_enabled"`
	InactiveTokenDisableDays    int    `json:"inactive_token_disable_days"`
	InactiveTokenDisableReason  string `json:"inactive_token_disable_reason"`
}

var riskControlSetting = RiskControlSetting{
	Enabled: true,

	HighRiskKeyRecreateRequired: true,
	HighRiskActivationRequired:  true,
	HighRiskActivationSource:    "qq",
	ActivationCodeTTLMinutes:    10,

	SameIPSameDayOAuthRegisterEnabled: true,
	SameIPSameDayOAuthRegisterLimit:   1,
	SameIPRegisterBlockMessage:        "检测到同一 IP 当天已注册过账号，请更换 IP 后再试。",

	RequestIPTrackingEnabled: true,

	SameIPMultiAccountUsageEnabled:       false,
	SameIPMultiAccountUsageWindowMinutes: 60,
	SameIPMultiAccountUsageUserLimit:     1,
	SameIPMultiAccountUsageBlockMessage:  "检测到同一 IP 在短时间内切换多个账号访问，当前 Key 已触发风控，请更换独立网络后重试。",

	DynamicIPChurnEnabled:         false,
	DynamicIPChurnWindowMinutes:   30,
	DynamicIPChurnDistinctIPLimit: 6,
	DynamicIPChurnBlockMessage:    "检测到当前 Key 在短时间内频繁切换 IP，已触发风控，请完成账号校验后再试。",

	BurstRegisterEnabled:       false,
	BurstRegisterWindowMinutes: 10,
	BurstRegisterLimit:         3,
	BurstRegisterBlockMessage:  "检测到该 IP 在短时间内注册过多账号，请稍后更换 IP 后再试。",

	InactiveTokenDisableEnabled: false,
	InactiveTokenDisableDays:    7,
	InactiveTokenDisableReason:  "长时间未活跃的账号需重新创建 Key 并完成校验后再使用。",
}

func init() {
	config.GlobalConfig.Register("risk_control_setting", &riskControlSetting)
}

func GetRiskControlSetting() *RiskControlSetting {
	return &riskControlSetting
}
