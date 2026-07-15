package model

import (
	"errors"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	RiskTokenActivationPending   = "pending"
	RiskTokenActivationActivated = "activated"
	RiskTokenActivationExpired   = "expired"
	RiskTokenActivationCancelled = "cancelled"

	OpsAccountRiskActionStatus  = "status"
	OpsAccountRiskActionDisable = "disable"
	OpsAccountRiskActionRestore = "restore"
)

type RiskUserControl struct {
	Id                     int    `json:"id" gorm:"primaryKey"`
	SiteId                 string `json:"site_id" gorm:"type:varchar(64);not null;index;uniqueIndex:ux_risk_user_control"`
	UserId                 int    `json:"user_id" gorm:"not null;index;uniqueIndex:ux_risk_user_control"`
	RiskLevel              string `json:"risk_level" gorm:"type:varchar(32);not null;default:'high';index"`
	ReasonCode             string `json:"reason_code" gorm:"type:varchar(64);not null;default:'';index"`
	Reason                 string `json:"reason" gorm:"type:text"`
	Enabled                bool   `json:"enabled" gorm:"not null;default:true;index"`
	KeyRecreateRequired    bool   `json:"key_recreate_required" gorm:"not null;default:true"`
	ActivationRequired     bool   `json:"activation_required" gorm:"not null;default:true"`
	ActivationSource       string `json:"activation_source" gorm:"type:varchar(32);not null;default:'qq'"`
	ExistingKeysDisabledAt int64  `json:"existing_keys_disabled_at" gorm:"not null;default:0"`
	UpdatedAt              int64  `json:"updated_at" gorm:"autoUpdateTime"`
	CreatedAt              int64  `json:"created_at" gorm:"autoCreateTime;index"`
}

func (RiskUserControl) TableName() string { return "risk_user_controls" }

type RiskTokenActivation struct {
	Id               int    `json:"id" gorm:"primaryKey"`
	SiteId           string `json:"site_id" gorm:"type:varchar(64);not null;index"`
	UserId           int    `json:"user_id" gorm:"not null;index"`
	TokenId          int    `json:"token_id" gorm:"not null;index"`
	ActivationSource string `json:"activation_source" gorm:"type:varchar(32);not null;default:'qq';index"`
	ExternalUserId   string `json:"external_user_id" gorm:"type:varchar(128);not null;default:'';index"`
	CodeHash         string `json:"-" gorm:"type:varchar(128);not null;default:'';index"`
	Status           string `json:"status" gorm:"type:varchar(32);not null;default:'pending';index"`
	ReasonCode       string `json:"reason_code" gorm:"type:varchar(64);not null;default:'';index"`
	Reason           string `json:"reason" gorm:"type:text"`
	ExpiresAt        int64  `json:"expires_at" gorm:"not null;default:0;index"`
	ActivatedAt      int64  `json:"activated_at" gorm:"not null;default:0;index"`
	UpdatedAt        int64  `json:"updated_at" gorm:"autoUpdateTime"`
	CreatedAt        int64  `json:"created_at" gorm:"autoCreateTime;index"`
}

func (RiskTokenActivation) TableName() string { return "risk_token_activations" }

type OAuthRegisterAttempt struct {
	Id           int    `json:"id" gorm:"primaryKey"`
	SiteId       string `json:"site_id" gorm:"type:varchar(64);not null;index"`
	ProviderSlug string `json:"provider_slug" gorm:"type:varchar(128);not null;default:'';index"`
	UserId       int    `json:"user_id" gorm:"not null;index"`
	ClientIP     string `json:"client_ip" gorm:"type:varchar(128);not null;index"`
	RegisterDate string `json:"register_date" gorm:"type:varchar(16);not null;index"`
	CreatedAt    int64  `json:"created_at" gorm:"autoCreateTime;index"`
	UpdatedAt    int64  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (OAuthRegisterAttempt) TableName() string { return "oauth_register_attempts" }

type RiskRequestFingerprint struct {
	Id          int    `json:"id" gorm:"primaryKey"`
	SiteId      string `json:"site_id" gorm:"type:varchar(64);not null;index;uniqueIndex:ux_risk_request_fp"`
	UserId      int    `json:"user_id" gorm:"not null;index;uniqueIndex:ux_risk_request_fp"`
	TokenId     int    `json:"token_id" gorm:"not null;index;uniqueIndex:ux_risk_request_fp"`
	ClientIP    string `json:"client_ip" gorm:"type:varchar(128);not null;index;uniqueIndex:ux_risk_request_fp"`
	HitCount    int64  `json:"hit_count" gorm:"not null;default:0"`
	FirstSeenAt int64  `json:"first_seen_at" gorm:"not null;default:0;index"`
	LastSeenAt  int64  `json:"last_seen_at" gorm:"not null;default:0;index"`
	CreatedAt   int64  `json:"created_at" gorm:"autoCreateTime;index"`
	UpdatedAt   int64  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (RiskRequestFingerprint) TableName() string { return "risk_request_fingerprints" }

type OpsAccountRiskAudit struct {
	Id        int64  `json:"id" gorm:"primaryKey"`
	SiteId    string `json:"site_id" gorm:"type:varchar(64);not null;index;uniqueIndex:ux_ops_account_risk_audit_subject"`
	RiskType  string `json:"risk_type" gorm:"type:varchar(96);not null;index;uniqueIndex:ux_ops_account_risk_audit_subject"`
	Severity  string `json:"severity" gorm:"type:varchar(32);not null;index"`
	Subject   string `json:"subject" gorm:"type:varchar(256);not null;uniqueIndex:ux_ops_account_risk_audit_subject"`
	UserIds   string `json:"user_ids" gorm:"type:text;not null;default:'[]'"`
	TokenIds  string `json:"token_ids" gorm:"type:text;not null;default:'[]'"`
	Ip        string `json:"ip" gorm:"type:text"`
	Evidence  string `json:"evidence" gorm:"type:text;not null;default:'{}'"`
	Status    string `json:"status" gorm:"type:varchar(32);not null;default:'open';index"`
	CreatedAt int64  `json:"created_at" gorm:"autoCreateTime;index"`
	UpdatedAt int64  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (OpsAccountRiskAudit) TableName() string { return "ops_account_risk_audit" }

type OpsAccountRiskAction struct {
	Id                  int64  `json:"id" gorm:"primaryKey"`
	SiteId              string `json:"site_id" gorm:"type:varchar(64);not null;index;uniqueIndex:ux_ops_account_risk_action_key,priority:1"`
	AuditId             int64  `json:"audit_id" gorm:"not null;index;uniqueIndex:ux_ops_account_risk_action_key,priority:2"`
	ActionKey           string `json:"action_key" gorm:"type:varchar(160);not null;uniqueIndex:ux_ops_account_risk_action_key,priority:3"`
	ActionType          string `json:"action_type" gorm:"type:varchar(32);not null;index"`
	TokenId             *int   `json:"token_id,omitempty" gorm:"index"`
	UserId              *int   `json:"user_id,omitempty" gorm:"index"`
	SourceActionId      *int64 `json:"source_action_id,omitempty" gorm:"index;uniqueIndex:ux_ops_account_risk_action_source"`
	TokenName           string `json:"token_name" gorm:"type:varchar(191);not null;default:''"`
	TokenGroup          string `json:"group" gorm:"column:token_group;type:varchar(64);not null;default:''"`
	PreviousTokenStatus int    `json:"previous_token_status" gorm:"not null;default:0"`
	NewTokenStatus      int    `json:"new_token_status" gorm:"not null;default:0"`
	PreviousAuditStatus string `json:"previous_audit_status" gorm:"type:varchar(32);not null;default:''"`
	NewAuditStatus      string `json:"new_audit_status" gorm:"type:varchar(32);not null;default:''"`
	OperatorId          int    `json:"operator_id" gorm:"not null;default:0;index"`
	Reason              string `json:"reason" gorm:"type:text"`
	CreatedAt           int64  `json:"created_at" gorm:"autoCreateTime;index"`
	UpdatedAt           int64  `json:"updated_at" gorm:"autoUpdateTime"`

	Audit        *OpsAccountRiskAudit  `json:"-" gorm:"foreignKey:AuditId;references:Id;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT"`
	Token        *Token                `json:"-" gorm:"foreignKey:TokenId;references:Id;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT"`
	User         *User                 `json:"-" gorm:"foreignKey:UserId;references:Id;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT"`
	SourceAction *OpsAccountRiskAction `json:"-" gorm:"foreignKey:SourceActionId;references:Id;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT"`
}

func (OpsAccountRiskAction) TableName() string { return "ops_account_risk_actions" }

type RiskMaintenanceUser struct {
	Id          int    `json:"id"`
	Username    string `json:"username"`
	Role        int    `json:"role"`
	Status      int    `json:"status"`
	CreatedAt   int64  `json:"created_at"`
	LastLoginAt int64  `json:"last_login_at"`
}

func riskCurrentSiteID() string {
	cfg := operation_setting.GetAgentSetting()
	if cfg != nil && strings.TrimSpace(cfg.SiteID) != "" {
		return strings.TrimSpace(cfg.SiteID)
	}
	return "default"
}

func GetRiskUserControl(siteId string, userId int) (*RiskUserControl, error) {
	var row RiskUserControl
	err := DB.Where("site_id = ? AND user_id = ?", strings.TrimSpace(siteId), userId).First(&row).Error
	return &row, err
}

func UpsertRiskUserControlWithTx(tx *gorm.DB, row *RiskUserControl) error {
	if tx == nil {
		tx = DB
	}
	if row == nil || row.UserId <= 0 {
		return errors.New("invalid risk user control")
	}
	row.SiteId = strings.TrimSpace(row.SiteId)
	if row.SiteId == "" {
		row.SiteId = riskCurrentSiteID()
	}
	if strings.TrimSpace(row.ActivationSource) == "" {
		row.ActivationSource = "qq"
	}
	return tx.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "site_id"}, {Name: "user_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"risk_level", "reason_code", "reason", "enabled", "key_recreate_required",
			"activation_required", "activation_source", "existing_keys_disabled_at", "updated_at",
		}),
	}).Create(row).Error
}

func UpsertRiskUserControl(row *RiskUserControl) error {
	return UpsertRiskUserControlWithTx(DB, row)
}

func CreateRiskTokenActivationWithTx(tx *gorm.DB, row *RiskTokenActivation) error {
	if tx == nil {
		tx = DB
	}
	if row == nil || row.UserId <= 0 || row.TokenId <= 0 {
		return errors.New("invalid risk token activation")
	}
	row.SiteId = strings.TrimSpace(row.SiteId)
	if row.SiteId == "" {
		row.SiteId = riskCurrentSiteID()
	}
	row.ActivationSource = strings.TrimSpace(row.ActivationSource)
	if row.ActivationSource == "" {
		row.ActivationSource = "qq"
	}
	if row.Status == "" {
		row.Status = RiskTokenActivationPending
	}
	now := time.Now().Unix()
	if err := tx.Model(&RiskTokenActivation{}).
		Where("site_id = ? AND token_id = ? AND status = ?", row.SiteId, row.TokenId, RiskTokenActivationPending).
		Updates(map[string]interface{}{
			"status":     RiskTokenActivationCancelled,
			"reason":     "superseded by newer activation code",
			"updated_at": now,
		}).Error; err != nil {
		return err
	}
	return tx.Create(row).Error
}

func CreateRiskTokenActivation(row *RiskTokenActivation) error {
	return CreateRiskTokenActivationWithTx(DB, row)
}

func GetLatestRiskTokenActivation(siteId string, userId int, tokenId int) (*RiskTokenActivation, error) {
	var row RiskTokenActivation
	err := DB.Where("site_id = ? AND user_id = ? AND token_id = ?", strings.TrimSpace(siteId), userId, tokenId).
		Order("id desc").First(&row).Error
	return &row, err
}

func ListLatestRiskTokenActivationsByToken(siteId string, tokenIds []int) (map[int]RiskTokenActivation, error) {
	out := make(map[int]RiskTokenActivation, len(tokenIds))
	if len(tokenIds) == 0 {
		return out, nil
	}
	var rows []RiskTokenActivation
	if err := DB.Where("site_id = ? AND token_id IN ?", strings.TrimSpace(siteId), tokenIds).
		Order("token_id asc, id desc").Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		if _, ok := out[row.TokenId]; !ok {
			out[row.TokenId] = row
		}
	}
	return out, nil
}

func GetPendingRiskTokenActivationsByCode(siteId string, userId int, activationSource string, codeHash string, now int64) ([]RiskTokenActivation, error) {
	var rows []RiskTokenActivation
	err := DB.Where(
		"site_id = ? AND user_id = ? AND activation_source = ? AND code_hash = ? AND status = ? AND expires_at > ?",
		strings.TrimSpace(siteId),
		userId,
		strings.TrimSpace(activationSource),
		strings.TrimSpace(codeHash),
		RiskTokenActivationPending,
		now,
	).Order("id asc").Find(&rows).Error
	return rows, err
}

func UpdateRiskTokenActivations(ids []int, updates map[string]interface{}) error {
	if len(ids) == 0 {
		return nil
	}
	if updates == nil {
		updates = map[string]interface{}{}
	}
	updates["updated_at"] = time.Now().Unix()
	return DB.Model(&RiskTokenActivation{}).Where("id IN ?", ids).Updates(updates).Error
}

func CountOAuthRegisterAttempts(siteId string, clientIP string, registerDate string) (int64, error) {
	var count int64
	err := DB.Model(&OAuthRegisterAttempt{}).
		Where("site_id = ? AND client_ip = ? AND register_date = ?", strings.TrimSpace(siteId), strings.TrimSpace(clientIP), strings.TrimSpace(registerDate)).
		Count(&count).Error
	return count, err
}

func CreateOAuthRegisterAttempt(row *OAuthRegisterAttempt) error {
	if row == nil {
		return errors.New("invalid oauth register attempt")
	}
	row.SiteId = strings.TrimSpace(row.SiteId)
	row.ProviderSlug = strings.TrimSpace(row.ProviderSlug)
	row.ClientIP = strings.TrimSpace(row.ClientIP)
	row.RegisterDate = strings.TrimSpace(row.RegisterDate)
	if row.SiteId == "" {
		row.SiteId = riskCurrentSiteID()
	}
	if row.RegisterDate == "" {
		row.RegisterDate = AgentBusinessDateAt(time.Now())
	}
	return DB.Create(row).Error
}

func CountOAuthRegisterAttemptsSince(siteId string, clientIP string, since int64) (int64, error) {
	var count int64
	err := DB.Model(&OAuthRegisterAttempt{}).
		Where("site_id = ? AND client_ip = ? AND created_at >= ?", strings.TrimSpace(siteId), strings.TrimSpace(clientIP), since).
		Count(&count).Error
	return count, err
}

func UpsertRiskRequestFingerprint(siteId string, userId int, tokenId int, clientIP string, seenAt int64) error {
	if userId <= 0 || tokenId <= 0 || strings.TrimSpace(clientIP) == "" {
		return errors.New("invalid risk request fingerprint")
	}
	if seenAt <= 0 {
		seenAt = time.Now().Unix()
	}
	siteId = strings.TrimSpace(siteId)
	if siteId == "" {
		siteId = riskCurrentSiteID()
	}
	row := &RiskRequestFingerprint{
		SiteId:      siteId,
		UserId:      userId,
		TokenId:     tokenId,
		ClientIP:    strings.TrimSpace(clientIP),
		HitCount:    1,
		FirstSeenAt: seenAt,
		LastSeenAt:  seenAt,
	}
	return DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "site_id"},
			{Name: "user_id"},
			{Name: "token_id"},
			{Name: "client_ip"},
		},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"hit_count":    gorm.Expr(`"risk_request_fingerprints"."hit_count" + 1`),
			"last_seen_at": seenAt,
			"updated_at":   seenAt,
		}),
	}).Create(row).Error
}

func CountDistinctRiskUsersByIPSince(siteId string, clientIP string, since int64, excludeUserId int) (int64, error) {
	var count int64
	tx := DB.Model(&RiskRequestFingerprint{}).
		Distinct("user_id").
		Where("site_id = ? AND client_ip = ? AND last_seen_at >= ?", strings.TrimSpace(siteId), strings.TrimSpace(clientIP), since)
	if excludeUserId > 0 {
		tx = tx.Where("user_id <> ?", excludeUserId)
	}
	err := tx.Count(&count).Error
	return count, err
}

func CountDistinctRiskIPsByTokenSince(siteId string, tokenId int, since int64) (int64, error) {
	var count int64
	err := DB.Model(&RiskRequestFingerprint{}).
		Distinct("client_ip").
		Where("site_id = ? AND token_id = ? AND last_seen_at >= ?", strings.TrimSpace(siteId), tokenId, since).
		Count(&count).Error
	return count, err
}

func GetLatestRiskRequestSeenAt(userId int) (int64, error) {
	var ts int64
	err := DB.Model(&RiskRequestFingerprint{}).
		Where("user_id = ?", userId).
		Select("COALESCE(MAX(last_seen_at), 0)").
		Scan(&ts).Error
	return ts, err
}

func ListRiskMaintenanceUsers(limit int, offset int) ([]RiskMaintenanceUser, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}
	rows := make([]RiskMaintenanceUser, 0, limit)
	err := DB.Table("users").
		Select("users.id, users.username, users.role, users.status, users.created_at, users.last_login_at").
		Joins("JOIN tokens ON tokens.user_id = users.id AND tokens.status = ? AND tokens.deleted_at IS NULL", common.TokenStatusEnabled).
		Where("users.status = ? AND users.role < ?", common.UserStatusEnabled, common.RoleAdminUser).
		Group("users.id, users.username, users.role, users.status, users.created_at, users.last_login_at").
		Order("users.id asc").
		Limit(limit).
		Offset(offset).
		Scan(&rows).Error
	return rows, err
}

func DeleteOldRiskRequestFingerprints(cutoff int64) error {
	if cutoff <= 0 {
		return nil
	}
	return DB.Where("last_seen_at > 0 AND last_seen_at < ?", cutoff).Delete(&RiskRequestFingerprint{}).Error
}

func AttachRiskTokenActivationMetadata(tokens []*Token) error {
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
	rows, err := ListLatestRiskTokenActivationsByToken(riskCurrentSiteID(), ids)
	if err != nil {
		return err
	}
	now := common.GetTimestamp()
	for _, token := range tokens {
		if token == nil {
			continue
		}
		row, ok := rows[token.Id]
		if !ok {
			continue
		}
		status := row.Status
		if status == RiskTokenActivationPending && row.ExpiresAt > 0 && row.ExpiresAt <= now {
			status = RiskTokenActivationExpired
		}
		if status == RiskTokenActivationActivated {
			continue
		}
		token.RiskActivationRequired = true
		token.RiskActivationSource = row.ActivationSource
		token.RiskActivationStatus = status
		token.RiskActivationExpiresAt = row.ExpiresAt
		token.RiskActivationReason = row.Reason
	}
	return nil
}
