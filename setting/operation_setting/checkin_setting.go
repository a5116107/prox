package operation_setting

import "github.com/QuantumNous/new-api/setting/config"

// CheckinChannelSetting 单个渠道的签到配置（独立预算池）。
// 字段为零值时回退到全局 CheckinSetting 的对应值，实现「独立但遵循后台配置」。
type CheckinChannelSetting struct {
	Enabled     *bool `json:"enabled"`      // nil=继承全局Enabled
	MinQuota    int   `json:"min_quota"`    // 0=继承全局MinQuota
	MaxQuota    int   `json:"max_quota"`    // 0=继承全局MaxQuota
	DailyBudget int   `json:"daily_budget"` // 该渠道每日发放总额上限(额度)，0=不限
}

// CheckinSetting 签到功能配置
type CheckinSetting struct {
	Enabled  bool `json:"enabled"`   // 是否启用签到功能
	MinQuota int  `json:"min_quota"` // 签到最小额度奖励
	MaxQuota int  `json:"max_quota"` // 签到最大额度奖励

	// JumpMode=direct 时，前端点击签到会跳转到外部群/社区入口；
	// JumpMode=api 时，继续使用站内 /api/user/checkin 直接签到。
	JumpMode string `json:"jump_mode"`

	// 外部签到入口，用于社区、QQ 或 TG 群签到跳转。
	// QQCheckinURL 保留为历史兼容字段；prox 优先使用社区入口。
	CommunityCheckinURL string `json:"community_checkin_url"`
	QQCheckinURL        string `json:"qq_checkin_url"`
	TGCheckinURL        string `json:"tg_checkin_url"`

	// 渠道独立配置（key: web/community/qq/tg）。各渠道签到相互独立、独立预算池，
	// 未配置的字段回退到上面的全局值。
	Channels map[string]CheckinChannelSetting `json:"channels"`
}

// 默认配置
var checkinSetting = CheckinSetting{
	Enabled:             false,
	MinQuota:            1000,
	MaxQuota:            10000,
	JumpMode:            "direct",
	CommunityCheckinURL: "",
	QQCheckinURL:        "",
	TGCheckinURL:        "",
	Channels:            map[string]CheckinChannelSetting{},
}

func init() {
	// 注册到全局配置管理器
	config.GlobalConfig.Register("checkin_setting", &checkinSetting)
}

// GetCheckinSetting 获取签到配置
func GetCheckinSetting() *CheckinSetting {
	return &checkinSetting
}

// IsCheckinEnabled 是否启用签到功能
func IsCheckinEnabled() bool {
	return checkinSetting.Enabled
}

// GetCheckinQuotaRange 获取签到额度范围
func GetCheckinQuotaRange() (min, max int) {
	return checkinSetting.MinQuota, checkinSetting.MaxQuota
}

// ChannelEnabled 指定渠道是否启用签到（渠道未配置 enabled 时继承全局 Enabled）
func (s *CheckinSetting) ChannelEnabled(channel string) bool {
	if s == nil || !s.Enabled {
		return false
	}
	if s.Channels != nil {
		if c, ok := s.Channels[channel]; ok && c.Enabled != nil {
			return *c.Enabled
		}
	}
	return true
}

// ChannelQuotaRange 返回指定渠道的签到额度范围（渠道未配置则回退全局）
func (s *CheckinSetting) ChannelQuotaRange(channel string) (int, int) {
	min, max := s.MinQuota, s.MaxQuota
	if s.Channels != nil {
		if c, ok := s.Channels[channel]; ok {
			if c.MinQuota > 0 {
				min = c.MinQuota
			}
			if c.MaxQuota > 0 {
				max = c.MaxQuota
			}
		}
	}
	if max < min {
		max = min
	}
	return min, max
}

// ChannelDailyBudget 返回指定渠道的每日发放总额上限（0=不限）
func (s *CheckinSetting) ChannelDailyBudget(channel string) int {
	if s == nil || s.Channels == nil {
		return 0
	}
	if c, ok := s.Channels[channel]; ok {
		return c.DailyBudget
	}
	return 0
}

func (s *CheckinSetting) EffectiveJumpMode() string {
	if s == nil {
		return "api"
	}
	switch s.JumpMode {
	case "api", "direct":
		return s.JumpMode
	default:
		return "direct"
	}
}

func (s *CheckinSetting) EffectiveCheckinURL(siteID string) string {
	if s == nil {
		return ""
	}
	switch siteID {
	case "prox":
		if s.CommunityCheckinURL != "" {
			return s.CommunityCheckinURL
		}
		return s.QQCheckinURL
	default:
		return s.CommunityCheckinURL
	}
}
