package model

import (
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func ListChatGroupsBySite(siteID string, platform string, role string, status string) ([]ChatGroup, error) {
	siteID = CanonicalSiteID(siteID)
	query := DB.Where("site_id = ?", siteID).Order("platform asc, group_id asc, id asc")
	if platform = strings.TrimSpace(platform); platform != "" {
		query = query.Where("platform = ?", strings.ToLower(platform))
	}
	if role = strings.TrimSpace(role); role != "" {
		query = query.Where("role = ?", role)
	}
	if status = strings.TrimSpace(status); status != "" {
		query = query.Where("status = ?", status)
	}
	var rows []ChatGroup
	return rows, query.Find(&rows).Error
}

func GetChatGroupByID(siteID string, id int) (*ChatGroup, error) {
	if id <= 0 {
		return nil, gorm.ErrRecordNotFound
	}
	var row ChatGroup
	err := DB.Where("site_id = ? AND id = ?", CanonicalSiteID(siteID), id).First(&row).Error
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func CreateChatGroup(row *ChatGroup) error {
	return CreateChatGroupWithDB(DB, row)
}

func CreateChatGroupWithDB(db *gorm.DB, row *ChatGroup) error {
	if row == nil {
		return gorm.ErrInvalidData
	}
	normalizeChatGroup(row)
	now := time.Now().Unix()
	if row.CreatedAt == 0 {
		row.CreatedAt = now
	}
	row.UpdatedAt = now
	return db.Create(row).Error
}

func UpdateChatGroup(row *ChatGroup) error {
	if row == nil || row.Id <= 0 {
		return gorm.ErrInvalidData
	}
	normalizeChatGroup(row)
	row.UpdatedAt = time.Now().Unix()
	return DB.Save(row).Error
}

func normalizeChatGroup(row *ChatGroup) {
	row.SiteId = CanonicalSiteID(row.SiteId)
	row.Platform = strings.TrimSpace(strings.ToLower(row.Platform))
	row.GroupId = strings.TrimSpace(row.GroupId)
	row.GroupName = strings.TrimSpace(row.GroupName)
	row.InviteTargetGroupId = strings.TrimSpace(row.InviteTargetGroupId)
	row.Role = strings.TrimSpace(row.Role)
	row.Status = strings.TrimSpace(row.Status)
	row.Language = strings.TrimSpace(row.Language)
	row.Timezone = strings.TrimSpace(row.Timezone)
	row.ConfigJson = strings.TrimSpace(row.ConfigJson)
}

func GetGroupChatOpsConfigByGroup(siteID string, platform string, groupID string) (*GroupChatOpsConfig, error) {
	return GetGroupChatOpsConfigByGroupWithDB(DB, siteID, platform, groupID)
}

func GetGroupChatOpsConfigByGroupWithDB(db *gorm.DB, siteID string, platform string, groupID string) (*GroupChatOpsConfig, error) {
	var row GroupChatOpsConfig
	err := db.Where("site_id = ? AND platform = ? AND group_id = ?",
		CanonicalSiteID(siteID),
		strings.TrimSpace(platform),
		strings.TrimSpace(groupID),
	).First(&row).Error
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func UpsertGroupChatOpsConfig(row *GroupChatOpsConfig) error {
	return UpsertGroupChatOpsConfigWithDB(DB, row)
}

func UpsertGroupChatOpsConfigWithDB(db *gorm.DB, row *GroupChatOpsConfig) error {
	if row == nil {
		return gorm.ErrInvalidData
	}
	row.SiteId = CanonicalSiteID(row.SiteId)
	row.Platform = strings.TrimSpace(row.Platform)
	row.GroupId = strings.TrimSpace(row.GroupId)
	now := time.Now().Unix()
	if row.CreatedAt == 0 {
		row.CreatedAt = now
	}
	row.UpdatedAt = now
	return db.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "site_id"},
			{Name: "platform"},
			{Name: "group_id"},
		},
		DoUpdates: clause.AssignmentColumns([]string{
			"checkin_enabled",
			"verify_enabled",
			"invite_enabled",
			"checkin_quota",
			"verify_min_quota",
			"invite_reward_quota",
			"invitee_reward_quota",
			"daily_group_reward_limit",
			"rule_json",
			"updated_at",
		}),
	}).Create(row).Error
}

func UpsertGroupGameConfig(row *GroupGameConfig) error {
	return UpsertGroupGameConfigWithDB(DB, row)
}

func UpsertGroupGameConfigWithDB(db *gorm.DB, row *GroupGameConfig) error {
	if row == nil {
		return gorm.ErrInvalidData
	}
	row.SiteId = CanonicalSiteID(row.SiteId)
	row.Platform = strings.TrimSpace(row.Platform)
	row.GroupId = strings.TrimSpace(row.GroupId)
	row.GameCode = strings.TrimSpace(row.GameCode)
	now := time.Now().Unix()
	if row.CreatedAt == 0 {
		row.CreatedAt = now
	}
	row.UpdatedAt = now
	return db.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "site_id"},
			{Name: "platform"},
			{Name: "group_id"},
			{Name: "game_code"},
		},
		DoUpdates: clause.AssignmentColumns([]string{
			"enabled",
			"budget_pool",
			"rule_json",
			"updated_at",
		}),
	}).Create(row).Error
}

func ListGroupGameConfigsByGroup(siteID string, platform string, groupID string) ([]GroupGameConfig, error) {
	return ListGroupGameConfigsByGroupWithDB(DB, siteID, platform, groupID)
}

func ListGroupGameConfigsByGroupWithDB(db *gorm.DB, siteID string, platform string, groupID string) ([]GroupGameConfig, error) {
	var rows []GroupGameConfig
	err := db.Where("site_id = ? AND platform = ? AND group_id = ?",
		CanonicalSiteID(siteID),
		strings.TrimSpace(platform),
		strings.TrimSpace(groupID),
	).Order("game_code asc, id asc").Find(&rows).Error
	return rows, err
}

func GetLatestGroupMetricsDaily(siteID string, platform string, groupID string) (*GroupMetricsDaily, error) {
	var row GroupMetricsDaily
	err := DB.Where("site_id = ? AND platform = ? AND group_id = ?",
		CanonicalSiteID(siteID),
		strings.TrimSpace(platform),
		strings.TrimSpace(groupID),
	).Order("metric_date desc, id desc").First(&row).Error
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func ListAdminConfigAuditsBySite(siteID string, limit int) ([]AdminConfigAudit, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	var rows []AdminConfigAudit
	err := DB.Where("site_id = ?", CanonicalSiteID(siteID)).
		Order("created_at desc, id desc").
		Limit(limit).
		Find(&rows).Error
	return rows, err
}
