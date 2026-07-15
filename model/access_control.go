package model

import (
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	AccessLevelNone          = "none"
	AccessLevelCommunityOnly = "community_only"
	AccessLevelFullAccess    = "full_access"
	AccessLevelPaidBypass    = "paid_bypass"
	AccessLevelAdminBypass   = "admin_bypass"
	AccessLevelManual        = "manual_override"

	AccessOverrideNone       = ""
	AccessOverrideFullAccess = "full_access"
	AccessOverrideCommunity  = "community_only"
	AccessOverrideCustom     = "custom_groups"
	AccessOverrideDeny       = "none"

	AccessControlFreezeKey = "access_control"
)

type UserSiteAccessState struct {
	Id                    int    `json:"id"`
	SiteId                string `json:"site_id" gorm:"type:varchar(64);not null;uniqueIndex:ux_user_site_access_state;index"`
	UserId                int    `json:"user_id" gorm:"not null;uniqueIndex:ux_user_site_access_state;index"`
	AccessLevel           string `json:"access_level" gorm:"type:varchar(32);not null;default:none;index"`
	CommunityBound        bool   `json:"community_bound" gorm:"not null;default:false"`
	HasOAuthBinding       bool   `json:"has_oauth_binding" gorm:"column:has_oauth_binding;not null;default:false"`
	HasRoomMembership     bool   `json:"has_room_membership" gorm:"not null;default:false"`
	PrimaryBound          bool   `json:"primary_bound" gorm:"not null;default:false"`
	PrimaryPlatform       string `json:"primary_platform" gorm:"type:varchar(32);default:''"`
	MatchedPrimaryGroupId string `json:"matched_primary_group_id" gorm:"type:varchar(128);default:''"`
	ManualOverrideMode    string `json:"manual_override_mode" gorm:"type:varchar(32);default:''"`
	ManualOverrideGroups  string `json:"manual_override_groups" gorm:"type:text"`
	ManualOverrideReason  string `json:"manual_override_reason" gorm:"type:text"`
	EffectiveGroups       string `json:"effective_groups" gorm:"type:text"`
	ReasonCode            string `json:"reason_code" gorm:"type:varchar(64);default:'';index"`
	ReasonMessage         string `json:"reason_message" gorm:"type:text"`
	LastEvaluatedAt       int64  `json:"last_evaluated_at" gorm:"index"`
	CreatedAt             int64  `json:"created_at" gorm:"index"`
	UpdatedAt             int64  `json:"updated_at"`
}

func (UserSiteAccessState) TableName() string { return "user_site_access_states" }

func normalizeStringSlice(items []string) []string {
	if len(items) == 0 {
		return []string{}
	}
	seen := make(map[string]struct{}, len(items))
	out := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out
}

func marshalStringSlice(items []string) string {
	items = normalizeStringSlice(items)
	if len(items) == 0 {
		return "[]"
	}
	data, _ := json.Marshal(items)
	return string(data)
}

func parseStringSlice(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return []string{}
	}
	var out []string
	if err := json.Unmarshal([]byte(raw), &out); err == nil {
		return normalizeStringSlice(out)
	}
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == '\n' || r == '\r' || r == '\t'
	})
	return normalizeStringSlice(parts)
}

func (s *UserSiteAccessState) EffectiveGroupList() []string {
	if s == nil {
		return []string{}
	}
	return parseStringSlice(s.EffectiveGroups)
}

func (s *UserSiteAccessState) ManualOverrideGroupList() []string {
	if s == nil {
		return []string{}
	}
	return parseStringSlice(s.ManualOverrideGroups)
}

func UpsertUserSiteAccessState(state *UserSiteAccessState) error {
	if state == nil || state.UserId <= 0 {
		return errors.New("invalid user site access state")
	}
	state.SiteId = strings.TrimSpace(state.SiteId)
	if state.SiteId == "" {
		return errors.New("site_id is required")
	}
	state.AccessLevel = strings.TrimSpace(state.AccessLevel)
	if state.AccessLevel == "" {
		state.AccessLevel = AccessLevelNone
	}
	now := time.Now().Unix()
	if state.CreatedAt == 0 {
		state.CreatedAt = now
	}
	if state.LastEvaluatedAt == 0 {
		state.LastEvaluatedAt = now
	}
	state.UpdatedAt = now
	state.PrimaryPlatform = normalizeIdentityProvider(state.PrimaryPlatform)
	state.MatchedPrimaryGroupId = strings.TrimSpace(state.MatchedPrimaryGroupId)
	state.ManualOverrideMode = strings.TrimSpace(state.ManualOverrideMode)
	state.ManualOverrideGroups = marshalStringSlice(parseStringSlice(state.ManualOverrideGroups))
	state.EffectiveGroups = marshalStringSlice(parseStringSlice(state.EffectiveGroups))
	return DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "site_id"}, {Name: "user_id"}},
		DoUpdates: clause.Assignments(map[string]any{
			"access_level":             state.AccessLevel,
			"community_bound":          state.CommunityBound,
			"has_oauth_binding":        state.HasOAuthBinding,
			"has_room_membership":      state.HasRoomMembership,
			"primary_bound":            state.PrimaryBound,
			"primary_platform":         state.PrimaryPlatform,
			"matched_primary_group_id": state.MatchedPrimaryGroupId,
			"manual_override_mode":     state.ManualOverrideMode,
			"manual_override_groups":   state.ManualOverrideGroups,
			"manual_override_reason":   state.ManualOverrideReason,
			"effective_groups":         state.EffectiveGroups,
			"reason_code":              state.ReasonCode,
			"reason_message":           state.ReasonMessage,
			"last_evaluated_at":        state.LastEvaluatedAt,
			"updated_at":               state.UpdatedAt,
		}),
	}).Create(state).Error
}

func GetUserSiteAccessState(siteID string, userID int) (*UserSiteAccessState, error) {
	var row UserSiteAccessState
	err := DB.Where("site_id = ? AND user_id = ?", strings.TrimSpace(siteID), userID).First(&row).Error
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func DeleteUserSiteAccessState(siteID string, userID int) error {
	if userID <= 0 {
		return errors.New("invalid user id")
	}
	return DB.Where("site_id = ? AND user_id = ?", strings.TrimSpace(siteID), userID).Delete(&UserSiteAccessState{}).Error
}

func ListUserSiteAccessStates(siteID string, limit int) ([]UserSiteAccessState, error) {
	var rows []UserSiteAccessState
	tx := DB.Where("site_id = ?", strings.TrimSpace(siteID)).Order("updated_at desc, id desc")
	if limit > 0 {
		tx = tx.Limit(limit)
	}
	err := tx.Find(&rows).Error
	return rows, err
}

func CountUserSiteAccessByLevel(siteID string) (map[string]int64, error) {
	type row struct {
		AccessLevel string
		Count       int64
	}
	var rows []row
	err := DB.Model(&UserSiteAccessState{}).
		Select("access_level, count(*) as count").
		Where("site_id = ?", strings.TrimSpace(siteID)).
		Group("access_level").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := map[string]int64{}
	for _, item := range rows {
		out[strings.TrimSpace(item.AccessLevel)] = item.Count
	}
	return out, nil
}

func SetUserSiteAccessOverride(siteID string, userID int, mode string, groups []string, reason string) error {
	if userID <= 0 {
		return errors.New("invalid user id")
	}
	siteID = strings.TrimSpace(siteID)
	if siteID == "" {
		return errors.New("site_id is required")
	}
	mode = strings.TrimSpace(mode)
	now := time.Now().Unix()
	updates := map[string]any{
		"manual_override_mode":   mode,
		"manual_override_groups": marshalStringSlice(groups),
		"manual_override_reason": strings.TrimSpace(reason),
		"updated_at":             now,
	}
	res := DB.Model(&UserSiteAccessState{}).Where("site_id = ? AND user_id = ?", siteID, userID).Updates(updates)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected > 0 {
		return nil
	}
	row := &UserSiteAccessState{
		SiteId:               siteID,
		UserId:               userID,
		AccessLevel:          AccessLevelNone,
		ManualOverrideMode:   mode,
		ManualOverrideGroups: marshalStringSlice(groups),
		ManualOverrideReason: strings.TrimSpace(reason),
		LastEvaluatedAt:      0,
		CreatedAt:            now,
		UpdatedAt:            now,
	}
	return DB.Create(row).Error
}

func ClearUserSiteAccessOverride(siteID string, userID int) error {
	if userID <= 0 {
		return errors.New("invalid user id")
	}
	return DB.Model(&UserSiteAccessState{}).
		Where("site_id = ? AND user_id = ?", strings.TrimSpace(siteID), userID).
		Updates(map[string]any{
			"manual_override_mode":   "",
			"manual_override_groups": "[]",
			"manual_override_reason": "",
			"updated_at":             time.Now().Unix(),
		}).Error
}

func ListUserIdentityBindingsByUser(siteID string, provider string, userID int) ([]UserIdentityBinding, error) {
	var rows []UserIdentityBinding
	tx := DB.Where("site_id = ? AND user_id = ? AND status = ?", strings.TrimSpace(siteID), userID, "active").Order("updated_at desc, id desc")
	if strings.TrimSpace(provider) != "" {
		tx = tx.Where("provider = ?", normalizeIdentityProvider(provider))
	}
	err := tx.Find(&rows).Error
	return rows, err
}

func ListAgentChatBindingsByUser(siteID string, source string, userID int) ([]AgentChatBinding, error) {
	var rows []AgentChatBinding
	tx := DB.Where("site_id = ? AND new_api_user_id = ? AND enabled = ?", strings.TrimSpace(siteID), userID, true).Order("updated_at desc, id desc")
	source = normalizeIdentityProvider(source)
	if source != "" {
		tx = tx.Where("source = ?", source)
	}
	err := tx.Find(&rows).Error
	return rows, err
}

func AttachAccessControlFreezeMetadata(tokens []*Token) error {
	if len(tokens) == 0 {
		return nil
	}
	ids := make([]int, 0, len(tokens))
	for _, token := range tokens {
		if token != nil && token.Id > 0 {
			ids = append(ids, token.Id)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	var freezes []CommunityGateTokenFreeze
	if err := DB.Where("token_id IN ? AND freeze_key = ? AND restored_at = 0", ids, AccessControlFreezeKey).Find(&freezes).Error; err != nil {
		return err
	}
	byToken := make(map[int]CommunityGateTokenFreeze, len(freezes))
	for _, freeze := range freezes {
		byToken[freeze.TokenId] = freeze
	}
	for _, token := range tokens {
		if token == nil {
			continue
		}
		if freeze, ok := byToken[token.Id]; ok {
			token.AccessControlFrozen = true
			token.AccessControlFreezeReason = freeze.Reason
			token.AccessControlFrozenAt = freeze.FrozenAt
		}
	}
	return nil
}

func CountActiveAccessControlFreezes(userID int) (int64, error) {
	if userID <= 0 {
		return 0, nil
	}
	var count int64
	err := DB.Model(&CommunityGateTokenFreeze{}).
		Where("user_id = ? AND freeze_key = ? AND restored_at = 0", userID, AccessControlFreezeKey).
		Count(&count).Error
	return count, err
}

func CountDistinctActiveAccessControlFreezeUsers() (int64, error) {
	var count int64
	err := DB.Model(&CommunityGateTokenFreeze{}).
		Distinct("user_id").
		Where("freeze_key = ? AND restored_at = 0", AccessControlFreezeKey).
		Count(&count).Error
	return count, err
}

func CountActiveAccessControlFreezeTokens() (int64, error) {
	var count int64
	err := DB.Model(&CommunityGateTokenFreeze{}).
		Where("freeze_key = ? AND restored_at = 0", AccessControlFreezeKey).
		Count(&count).Error
	return count, err
}

func FreezeUserTokensForAccessControl(userID int, reason string, dryRun bool) (*CommunityGateTokenFreezeStats, error) {
	return freezeUserTokensByKey(userID, AccessControlFreezeKey, reason, dryRun)
}

func RestoreUserTokensForAccessControl(userID int) (*CommunityGateTokenFreezeStats, error) {
	return restoreUserTokensByKey(userID, AccessControlFreezeKey)
}

func freezeUserTokensByKey(userID int, freezeKey string, reason string, dryRun bool) (*CommunityGateTokenFreezeStats, error) {
	stats := &CommunityGateTokenFreezeStats{}
	if userID <= 0 {
		return stats, errors.New("invalid user id")
	}
	var user User
	if err := DB.Select("id", "role", "group").Where("id = ?", userID).First(&user).Error; err == nil {
		if user.Role >= common.RoleAdminUser {
			return stats, nil
		}
	}
	var tokens []Token
	if err := DB.Where("user_id = ? AND status = ?", userID, common.TokenStatusEnabled).Find(&tokens).Error; err != nil {
		return stats, err
	}
	stats.Eligible = len(tokens)
	if dryRun || len(tokens) == 0 {
		return stats, nil
	}
	now := time.Now().Unix()
	changedKeys := make([]string, 0, len(tokens))
	err := DB.Transaction(func(tx *gorm.DB) error {
		for _, token := range tokens {
			var activeCount int64
			if err := tx.Model(&CommunityGateTokenFreeze{}).
				Where("token_id = ? AND freeze_key = ? AND restored_at = 0", token.Id, freezeKey).
				Count(&activeCount).Error; err != nil {
				return err
			}
			if activeCount > 0 {
				stats.AlreadyFrozen++
			} else {
				freeze := CommunityGateTokenFreeze{
					UserId: userID, TokenId: token.Id, OldStatus: token.Status,
					NewStatus: common.TokenStatusDisabled, FreezeKey: freezeKey,
					Reason: strings.TrimSpace(reason), FrozenAt: now,
				}
				if err := tx.Create(&freeze).Error; err != nil {
					return err
				}
			}
			res := tx.Model(&Token{}).Where("id = ? AND status = ?", token.Id, token.Status).
				Update("status", common.TokenStatusDisabled)
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected > 0 {
				stats.Disabled++
				changedKeys = append(changedKeys, token.Key)
			}
		}
		return nil
	})
	if err != nil {
		return stats, err
	}
	if common.RedisEnabled {
		for _, key := range changedKeys {
			_ = cacheDeleteToken(key)
		}
	}
	return stats, nil
}

func restoreUserTokensByKey(userID int, freezeKey string) (*CommunityGateTokenFreezeStats, error) {
	stats := &CommunityGateTokenFreezeStats{}
	if userID <= 0 {
		return stats, errors.New("invalid user id")
	}
	var freezes []CommunityGateTokenFreeze
	if err := DB.Where("user_id = ? AND freeze_key = ? AND restored_at = 0", userID, freezeKey).
		Order("id asc").Find(&freezes).Error; err != nil {
		return stats, err
	}
	stats.ActiveFrozen = len(freezes)
	if len(freezes) == 0 {
		return stats, nil
	}
	now := time.Now().Unix()
	changedKeys := make([]string, 0, len(freezes))
	err := DB.Transaction(func(tx *gorm.DB) error {
		for _, freeze := range freezes {
			var token Token
			if err := tx.Unscoped().Where("id = ?", freeze.TokenId).First(&token).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					_ = tx.Model(&CommunityGateTokenFreeze{}).Where("id = ?", freeze.Id).Update("restored_at", now).Error
					continue
				}
				return err
			}
			if token.DeletedAt.Valid {
				_ = tx.Model(&CommunityGateTokenFreeze{}).Where("id = ?", freeze.Id).Update("restored_at", now).Error
				continue
			}
			res := tx.Model(&Token{}).Where("id = ? AND status = ?", token.Id, common.TokenStatusDisabled).
				Update("status", freeze.OldStatus)
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected > 0 {
				stats.Restored++
				changedKeys = append(changedKeys, token.Key)
			}
			if err := tx.Model(&CommunityGateTokenFreeze{}).Where("id = ?", freeze.Id).Update("restored_at", now).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return stats, err
	}
	if common.RedisEnabled {
		for _, key := range changedKeys {
			_ = cacheDeleteToken(key)
		}
	}
	return stats, nil
}

func SortedAccessLevels(stats map[string]int64) []string {
	levels := make([]string, 0, len(stats))
	for level := range stats {
		levels = append(levels, level)
	}
	sort.Strings(levels)
	return levels
}
