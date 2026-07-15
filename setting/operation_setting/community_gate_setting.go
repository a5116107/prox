package operation_setting

import "github.com/QuantumNous/new-api/setting/config"

type CommunityGateSetting struct {
	Enabled bool `json:"enabled"`

	ProviderSlug  string   `json:"provider_slug"`
	CommunityHost string   `json:"community_host"`
	RoomID        string   `json:"room_id"`
	RoomIDs       []string `json:"room_ids"`
	RoomMatchMode string   `json:"room_match_mode"`

	RequireOAuthBinding         bool   `json:"require_oauth_binding"`
	RequireRoomMembership       bool   `json:"require_room_membership"`
	OnlyAllowProviderRegister   bool   `json:"only_allow_provider_register"`
	DisablePasswordRegister     bool   `json:"disable_password_register"`
	DisableBuiltinOAuthRegister bool   `json:"disable_builtin_oauth_register"`
	AutoInviteOnLogin           bool   `json:"auto_invite_on_login"`
	BlockTokenWhenNotCompliant  bool   `json:"block_token_when_not_compliant"`
	AllowAdminBypass            bool   `json:"allow_admin_bypass"`
	MemberCacheTTLSeconds       int    `json:"member_cache_ttl_seconds"`
	MemberScanLimit             int    `json:"member_scan_limit"`
	AuditEnabled                bool   `json:"audit_enabled"`
	TokenDisableMode            string `json:"token_disable_mode"`
	DeniedMessage               string `json:"denied_message"`
}

var communityGateSetting = CommunityGateSetting{
	Enabled: true,

	ProviderSlug:  "dc.hhhl.cc",
	CommunityHost: "https://dc.hhhl.cc",
	RoomID:        "",
	RoomIDs:       []string{},
	RoomMatchMode: "any_of",

	RequireOAuthBinding:         true,
	RequireRoomMembership:       true,
	OnlyAllowProviderRegister:   true,
	DisablePasswordRegister:     true,
	DisableBuiltinOAuthRegister: true,
	AutoInviteOnLogin:           true,
	BlockTokenWhenNotCompliant:  true,
	AllowAdminBypass:            true,
	MemberCacheTTLSeconds:       600,
	MemberScanLimit:             3000,
	AuditEnabled:                true,
	TokenDisableMode:            "temporary_disable",
	DeniedMessage:               "请先使用 dc.hhhl.cc 社区授权登录，并加入本站社区群聊后再使用 API Key。",
}

func init() {
	config.GlobalConfig.Register("community_gate_setting", &communityGateSetting)
}

func GetCommunityGateSetting() *CommunityGateSetting {
	return &communityGateSetting
}
