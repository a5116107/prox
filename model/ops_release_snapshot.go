package model

import (
	"strings"
	"time"

	"gorm.io/gorm"
)

type OpsReleaseSnapshot struct {
	Id                 int    `json:"id"`
	SiteId             string `json:"site_id" gorm:"type:varchar(64);not null;index"`
	Action             string `json:"action" gorm:"type:varchar(32);not null;index"`
	ReleaseLabel       string `json:"release_label" gorm:"type:varchar(128);not null;index"`
	ActorUserId        int    `json:"actor_user_id" gorm:"index"`
	ActorUsername      string `json:"actor_username" gorm:"type:varchar(128)"`
	Note               string `json:"note" gorm:"type:text"`
	SnapshotHash       string `json:"snapshot_hash" gorm:"type:char(64);index"`
	OptionCount        int    `json:"option_count"`
	MissingOptionCount int    `json:"missing_option_count"`
	GroupCount         int    `json:"group_count"`
	GroupChatOpsCount  int    `json:"group_chatops_count"`
	GroupGameCount     int    `json:"group_game_count"`
	SourceReleaseId    int    `json:"source_release_id" gorm:"index"`
	PayloadJson        string `json:"payload_json" gorm:"type:text"`
	CreatedAt          int64  `json:"created_at" gorm:"index"`
	AppliedAt          int64  `json:"applied_at" gorm:"index"`
}

func (OpsReleaseSnapshot) TableName() string { return "ops_release_snapshots" }

func CreateOpsReleaseSnapshot(row *OpsReleaseSnapshot) error {
	return CreateOpsReleaseSnapshotWithDB(DB, row)
}

func CreateOpsReleaseSnapshotWithDB(db *gorm.DB, row *OpsReleaseSnapshot) error {
	if row == nil {
		return gorm.ErrInvalidData
	}
	row.SiteId = CanonicalSiteID(row.SiteId)
	row.Action = strings.TrimSpace(strings.ToLower(row.Action))
	row.ReleaseLabel = strings.TrimSpace(row.ReleaseLabel)
	row.ActorUsername = strings.TrimSpace(row.ActorUsername)
	row.Note = strings.TrimSpace(row.Note)
	row.SnapshotHash = strings.TrimSpace(strings.ToLower(row.SnapshotHash))
	if row.ReleaseLabel == "" {
		row.ReleaseLabel = row.SiteId + "-" + row.Action
	}
	now := time.Now().Unix()
	if row.CreatedAt == 0 {
		row.CreatedAt = now
	}
	if row.AppliedAt == 0 {
		row.AppliedAt = now
	}
	return db.Create(row).Error
}

func ListOpsReleaseSnapshotsBySite(siteID string, limit int) ([]OpsReleaseSnapshot, error) {
	if limit <= 0 || limit > 200 {
		limit = 20
	}
	var rows []OpsReleaseSnapshot
	err := DB.Where("site_id = ?", CanonicalSiteID(siteID)).
		Order("applied_at desc, id desc").
		Limit(limit).
		Find(&rows).Error
	return rows, err
}

func GetOpsReleaseSnapshotByID(siteID string, id int) (*OpsReleaseSnapshot, error) {
	if id <= 0 {
		return nil, gorm.ErrRecordNotFound
	}
	var row OpsReleaseSnapshot
	err := DB.Where("site_id = ? AND id = ?", CanonicalSiteID(siteID), id).First(&row).Error
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func ListGroupChatOpsConfigsBySite(siteID string) ([]GroupChatOpsConfig, error) {
	return ListGroupChatOpsConfigsBySiteWithDB(DB, siteID)
}

func ListGroupChatOpsConfigsBySiteWithDB(db *gorm.DB, siteID string) ([]GroupChatOpsConfig, error) {
	var rows []GroupChatOpsConfig
	err := db.Where("site_id = ?", CanonicalSiteID(siteID)).
		Order("platform asc, group_id asc, id asc").
		Find(&rows).Error
	return rows, err
}

func ListGroupGameConfigsBySite(siteID string) ([]GroupGameConfig, error) {
	return ListGroupGameConfigsBySiteWithDB(DB, siteID)
}

func ListGroupGameConfigsBySiteWithDB(db *gorm.DB, siteID string) ([]GroupGameConfig, error) {
	var rows []GroupGameConfig
	err := db.Where("site_id = ?", CanonicalSiteID(siteID)).
		Order("platform asc, group_id asc, game_code asc, id asc").
		Find(&rows).Error
	return rows, err
}

func InsertChatGroupSnapshotWithDB(db *gorm.DB, row *ChatGroup) error {
	if row == nil {
		return gorm.ErrInvalidData
	}
	normalizeChatGroup(row)
	return db.Create(row).Error
}

func InsertGroupChatOpsSnapshotWithDB(db *gorm.DB, row *GroupChatOpsConfig) error {
	if row == nil {
		return gorm.ErrInvalidData
	}
	row.SiteId = CanonicalSiteID(row.SiteId)
	row.Platform = strings.TrimSpace(row.Platform)
	row.GroupId = strings.TrimSpace(row.GroupId)
	row.RuleJson = strings.TrimSpace(row.RuleJson)
	return db.Create(row).Error
}

func InsertGroupGameSnapshotWithDB(db *gorm.DB, row *GroupGameConfig) error {
	if row == nil {
		return gorm.ErrInvalidData
	}
	row.SiteId = CanonicalSiteID(row.SiteId)
	row.Platform = strings.TrimSpace(row.Platform)
	row.GroupId = strings.TrimSpace(row.GroupId)
	row.GameCode = strings.TrimSpace(row.GameCode)
	row.BudgetPool = strings.TrimSpace(row.BudgetPool)
	row.RuleJson = strings.TrimSpace(row.RuleJson)
	return db.Create(row).Error
}

func ReloadOptionsFromDatabase() {
	loadOptionsFromDatabase()
}

func ReplaceOptionsBulkTx(tx *gorm.DB, values map[string]string, deleteKeys []string) error {
	if tx == nil {
		return gorm.ErrInvalidDB
	}
	activeKeys := map[string]struct{}{}
	for key, value := range values {
		trimmedKey := strings.TrimSpace(key)
		if trimmedKey == "" {
			continue
		}
		activeKeys[trimmedKey] = struct{}{}
		option := Option{Key: trimmedKey}
		if err := tx.FirstOrCreate(&option, Option{Key: trimmedKey}).Error; err != nil {
			return err
		}
		option.Value = value
		if err := tx.Save(&option).Error; err != nil {
			return err
		}
	}
	cleanDeleteKeys := make([]string, 0, len(deleteKeys))
	for _, key := range deleteKeys {
		trimmedKey := strings.TrimSpace(key)
		if trimmedKey == "" {
			continue
		}
		if _, exists := activeKeys[trimmedKey]; exists {
			continue
		}
		cleanDeleteKeys = append(cleanDeleteKeys, trimmedKey)
	}
	if len(cleanDeleteKeys) > 0 {
		if err := tx.Where("key IN ?", cleanDeleteKeys).Delete(&Option{}).Error; err != nil {
			return err
		}
	}
	return nil
}
