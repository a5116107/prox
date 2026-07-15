package model

import (
	"errors"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type CommunityGateAudit struct {
	Id                int    `json:"id" gorm:"primaryKey"`
	UserId            int    `json:"user_id" gorm:"not null;index;uniqueIndex:ux_community_gate_audit"`
	Username          string `json:"username" gorm:"type:varchar(191);not null;default:''"`
	ProviderSlug      string `json:"provider_slug" gorm:"type:varchar(128);not null;index;uniqueIndex:ux_community_gate_audit"`
	ProviderUserId    string `json:"provider_user_id" gorm:"type:varchar(256);not null;default:'';index"`
	RoomId            string `json:"room_id" gorm:"type:varchar(64);not null;index;uniqueIndex:ux_community_gate_audit"`
	Compliant         bool   `json:"compliant" gorm:"not null;default:false;index"`
	HasOAuthBinding   bool   `json:"has_oauth_binding" gorm:"column:has_oauth_binding;not null;default:false"`
	HasRoomMembership bool   `json:"has_room_membership" gorm:"not null;default:false"`
	ReasonCode        string `json:"reason_code" gorm:"type:varchar(64);not null;default:'';index"`
	Reason            string `json:"reason" gorm:"type:text"`
	CheckedAt         int64  `json:"checked_at" gorm:"not null;default:0;index"`
	CreatedAt         int64  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt         int64  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (CommunityGateAudit) TableName() string { return "community_gate_audits" }

type CommunityGateTokenFreeze struct {
	Id         int    `json:"id" gorm:"primaryKey"`
	UserId     int    `json:"user_id" gorm:"not null;index"`
	TokenId    int    `json:"token_id" gorm:"not null;index"`
	OldStatus  int    `json:"old_status" gorm:"not null"`
	NewStatus  int    `json:"new_status" gorm:"not null"`
	FreezeKey  string `json:"freeze_key" gorm:"type:varchar(64);not null;default:'community_gate';index"`
	Reason     string `json:"reason" gorm:"type:text"`
	FrozenAt   int64  `json:"frozen_at" gorm:"not null;default:0;index"`
	RestoredAt int64  `json:"restored_at" gorm:"not null;default:0;index"`
	CreatedAt  int64  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt  int64  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (CommunityGateTokenFreeze) TableName() string { return "community_gate_token_freezes" }

type CommunityGateTokenFreezeStats struct {
	Eligible      int `json:"eligible"`
	Disabled      int `json:"disabled"`
	AlreadyFrozen int `json:"already_frozen"`
	ActiveFrozen  int `json:"active_frozen"`
	Restored      int `json:"restored"`
}

func GetCommunityGateBinding(providerSlug string, userId int) (providerId int, providerUserId string, err error) {
	providerSlug = strings.TrimSpace(providerSlug)
	if providerSlug == "" || userId <= 0 {
		return 0, "", gorm.ErrRecordNotFound
	}
	provider, err := GetCustomOAuthProviderBySlug(providerSlug)
	if err != nil {
		return 0, "", err
	}
	binding, err := GetUserOAuthBinding(userId, provider.Id)
	if err != nil {
		return provider.Id, "", err
	}
	return provider.Id, strings.TrimSpace(binding.ProviderUserId), nil
}

func UpsertCommunityGateAudit(a *CommunityGateAudit) error {
	if a == nil || a.UserId <= 0 {
		return errors.New("invalid community gate audit")
	}
	a.ProviderSlug = strings.TrimSpace(a.ProviderSlug)
	a.RoomId = strings.TrimSpace(a.RoomId)
	if a.ProviderSlug == "" {
		a.ProviderSlug = "dc.hhhl.cc"
	}
	if a.RoomId == "" {
		a.RoomId = "default"
	}
	if a.CheckedAt == 0 {
		a.CheckedAt = time.Now().Unix()
	}
	return DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "user_id"}, {Name: "provider_slug"}, {Name: "room_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"username", "provider_user_id", "compliant", "has_oauth_binding",
			"has_room_membership", "reason_code", "reason", "checked_at", "updated_at",
		}),
	}).Create(a).Error
}

func AttachCommunityGateFreezeMetadata(tokens []*Token) error {
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
	if err := DB.Where("token_id IN ? AND freeze_key = ? AND restored_at = 0", ids, "community_gate").Find(&freezes).Error; err != nil {
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
			token.CommunityGateFrozen = true
			token.CommunityGateFreezeReason = freeze.Reason
			token.CommunityGateFrozenAt = freeze.FrozenAt
		}
	}
	return nil
}

func CountActiveCommunityGateFreezes(userId int) (int64, error) {
	if userId <= 0 {
		return 0, nil
	}
	var count int64
	err := DB.Model(&CommunityGateTokenFreeze{}).
		Where("user_id = ? AND freeze_key = ? AND restored_at = 0", userId, "community_gate").
		Count(&count).Error
	return count, err
}

func ListCommunityGateAudits(limit int) ([]CommunityGateAudit, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	var audits []CommunityGateAudit
	err := DB.Order("checked_at desc, id desc").Limit(limit).Find(&audits).Error
	return audits, err
}

func ListCommunityGateUsers(limit int) ([]User, error) {
	var users []User
	query := DB.Select("id", "username", "role", "status").
		Where("status = ?", common.UserStatusEnabled).
		Where("role < ?", common.RoleAdminUser).
		Order("id asc")
	if limit > 0 {
		query = query.Limit(limit)
	}
	return users, query.Find(&users).Error
}

func FreezeUserTokensForCommunityGate(userId int, reason string, dryRun bool) (*CommunityGateTokenFreezeStats, error) {
	stats := &CommunityGateTokenFreezeStats{}
	if userId <= 0 {
		return stats, errors.New("invalid user id")
	}
	var tokens []Token
	if err := DB.Where("user_id = ? AND status = ?", userId, common.TokenStatusEnabled).Find(&tokens).Error; err != nil {
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
				Where("token_id = ? AND freeze_key = ? AND restored_at = 0", token.Id, "community_gate").
				Count(&activeCount).Error; err != nil {
				return err
			}
			if activeCount > 0 {
				stats.AlreadyFrozen++
			} else {
				freeze := CommunityGateTokenFreeze{
					UserId: userId, TokenId: token.Id, OldStatus: token.Status,
					NewStatus: common.TokenStatusDisabled, FreezeKey: "community_gate",
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

func RestoreCommunityGateUserTokens(userId int) (*CommunityGateTokenFreezeStats, error) {
	stats := &CommunityGateTokenFreezeStats{}
	if userId <= 0 {
		return stats, errors.New("invalid user id")
	}
	var freezes []CommunityGateTokenFreeze
	if err := DB.Where("user_id = ? AND freeze_key = ? AND restored_at = 0", userId, "community_gate").
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
