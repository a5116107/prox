package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	OpsRiskStatusActive    = "active"
	OpsRiskStatusOpen      = "open"
	OpsRiskStatusReviewing = "reviewing"
	OpsRiskStatusIgnored   = "ignored"
	OpsRiskStatusClosed    = "closed"
)

func opsRiskStatusesForQuery(status string) []string {
	status = strings.ToLower(strings.TrimSpace(status))
	if status == "" {
		return nil
	}
	if status == OpsRiskStatusActive {
		return []string{OpsRiskStatusOpen, OpsRiskStatusReviewing}
	}
	return []string{status}
}

type OpsAccountRiskSummary struct {
	TotalOpen       int64            `json:"total_open"`
	HighRiskOpen    int64            `json:"high_risk_open"`
	KeyRiskOpen     int64            `json:"key_risk_open"`
	AdminCandidates int64            `json:"admin_candidates"`
	ByType          map[string]int64 `json:"by_type"`
	BySeverity      map[string]int64 `json:"by_severity"`
}

type OpsAccountRiskListResult struct {
	Items    []model.OpsAccountRiskAudit `json:"items"`
	Total    int64                       `json:"total"`
	Page     int                         `json:"page"`
	PageSize int                         `json:"page_size"`
	Summary  OpsAccountRiskSummary       `json:"summary"`
}

type OpsAccountRiskQuery struct {
	RiskType string
	Severity string
	Status   string
	Keyword  string
	Page     int
	PageSize int
}

type OpsRiskTokenDetail struct {
	Id     int    `json:"id"`
	UserId int    `json:"user_id"`
	Name   string `json:"name"`
	Status int    `json:"status"`
	Group  string `json:"group"`
}

func normalizeOpsRiskStatus(status string) (string, error) {
	status = strings.ToLower(strings.TrimSpace(status))
	switch status {
	case OpsRiskStatusOpen:
		return OpsRiskStatusOpen, nil
	case OpsRiskStatusReviewing, OpsRiskStatusIgnored, OpsRiskStatusClosed:
		return status, nil
	default:
		return "", fmt.Errorf("invalid risk audit status: %s", status)
	}
}

func validateOpsRiskStatusTransition(current, target string) error {
	if current == target {
		return nil
	}
	allowed := map[string]map[string]bool{
		OpsRiskStatusOpen: {
			OpsRiskStatusReviewing: true,
			OpsRiskStatusIgnored:   true,
			OpsRiskStatusClosed:    true,
		},
		OpsRiskStatusReviewing: {
			OpsRiskStatusOpen:    true,
			OpsRiskStatusIgnored: true,
			OpsRiskStatusClosed:  true,
		},
		OpsRiskStatusIgnored: {
			OpsRiskStatusOpen:   true,
			OpsRiskStatusClosed: true,
		},
		OpsRiskStatusClosed: {},
	}
	transitions, ok := allowed[current]
	if !ok || !transitions[target] {
		return fmt.Errorf("invalid risk audit status transition: %s -> %s", current, target)
	}
	return nil
}

func normalizeOpsRiskPage(page, pageSize int) (int, int) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 200 {
		pageSize = 200
	}
	return page, pageSize
}

func buildOpsRiskAuditQuery(q OpsAccountRiskQuery) *gorm.DB {
	tx := model.DB.Model(&model.OpsAccountRiskAudit{}).Where("site_id = ?", AgentSiteID())
	if v := strings.TrimSpace(q.RiskType); v != "" {
		tx = tx.Where("risk_type = ?", v)
	}
	if v := strings.TrimSpace(q.Severity); v != "" {
		tx = tx.Where("severity = ?", v)
	}
	if statuses := opsRiskStatusesForQuery(q.Status); len(statuses) == 1 {
		tx = tx.Where("status = ?", statuses[0])
	} else if len(statuses) > 1 {
		tx = tx.Where("status IN ?", statuses)
	}
	if kw := strings.TrimSpace(q.Keyword); kw != "" {
		like := "%" + kw + "%"
		tx = tx.Where("subject LIKE ? OR user_ids LIKE ? OR token_ids LIKE ? OR ip LIKE ? OR evidence LIKE ?", like, like, like, like, like)
	}
	return tx
}

func ListOpsAccountRiskAudits(q OpsAccountRiskQuery) (*OpsAccountRiskListResult, error) {
	page, pageSize := normalizeOpsRiskPage(q.Page, q.PageSize)
	var total int64
	base := buildOpsRiskAuditQuery(q)
	if err := base.Count(&total).Error; err != nil {
		return nil, err
	}
	var rows []model.OpsAccountRiskAudit
	if err := buildOpsRiskAuditQuery(q).Order("updated_at desc, id desc").Limit(pageSize).Offset((page - 1) * pageSize).Find(&rows).Error; err != nil {
		return nil, err
	}
	summary, err := GetOpsAccountRiskSummary()
	if err != nil {
		return nil, err
	}
	return &OpsAccountRiskListResult{Items: rows, Total: total, Page: page, PageSize: pageSize, Summary: *summary}, nil
}

func GetOpsAccountRiskSummary() (*OpsAccountRiskSummary, error) {
	out := &OpsAccountRiskSummary{ByType: map[string]int64{}, BySeverity: map[string]int64{}}
	siteID := AgentSiteID()
	if err := model.DB.Model(&model.OpsAccountRiskAudit{}).Where("site_id = ? AND status = ?", siteID, OpsRiskStatusOpen).Count(&out.TotalOpen).Error; err != nil {
		return nil, err
	}
	if err := model.DB.Model(&model.OpsAccountRiskAudit{}).Where("site_id = ? AND status = ? AND severity IN ?", siteID, OpsRiskStatusOpen, []string{"high", "critical"}).Count(&out.HighRiskOpen).Error; err != nil {
		return nil, err
	}
	if err := model.DB.Model(&model.OpsAccountRiskAudit{}).Where("site_id = ? AND status = ? AND (risk_type LIKE ? OR token_ids <> ?)", siteID, OpsRiskStatusOpen, "%token%", "[]").Count(&out.KeyRiskOpen).Error; err != nil {
		return nil, err
	}
	if err := model.DB.Model(&model.OpsAccountRiskAudit{}).Where("site_id = ? AND status = ? AND risk_type = ?", siteID, OpsRiskStatusOpen, "admin_candidate").Count(&out.AdminCandidates).Error; err != nil {
		return nil, err
	}
	type countRow struct {
		Key   string
		Count int64
	}
	var byType []countRow
	if err := model.DB.Model(&model.OpsAccountRiskAudit{}).Select("risk_type AS key, count(*) AS count").Where("site_id = ? AND status = ?", siteID, OpsRiskStatusOpen).Group("risk_type").Scan(&byType).Error; err != nil {
		return nil, err
	}
	for _, row := range byType {
		out.ByType[row.Key] = row.Count
	}
	var bySeverity []countRow
	if err := model.DB.Model(&model.OpsAccountRiskAudit{}).Select("severity AS key, count(*) AS count").Where("site_id = ? AND status = ?", siteID, OpsRiskStatusOpen).Group("severity").Scan(&bySeverity).Error; err != nil {
		return nil, err
	}
	for _, row := range bySeverity {
		out.BySeverity[row.Key] = row.Count
	}
	return out, nil
}

func UpdateOpsAccountRiskAuditStatus(id int64, status string, operatorID int, reason string) (*model.OpsAccountRiskAudit, error) {
	normalized, err := normalizeOpsRiskStatus(status)
	if err != nil {
		return nil, err
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "管理员手动更新风控审计状态"
	}
	var row model.OpsAccountRiskAudit
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		loaded, loadErr := getOpsRiskAudit(tx, id, true)
		if loadErr != nil {
			return loadErr
		}
		row = *loaded
		if err := validateOpsRiskStatusTransition(row.Status, normalized); err != nil {
			return err
		}
		if row.Status == normalized {
			return nil
		}
		previous := row.Status
		now := time.Now()
		if err := setOpsRiskAuditStatusWithTx(tx, &row, normalized, now.Unix()); err != nil {
			return err
		}
		action := model.OpsAccountRiskAction{
			SiteId: row.SiteId, AuditId: row.Id,
			ActionKey:           fmt.Sprintf("status:%s:%s:%d", previous, normalized, now.UnixNano()),
			ActionType:          model.OpsAccountRiskActionStatus,
			PreviousAuditStatus: previous, NewAuditStatus: normalized,
			OperatorId: operatorID, Reason: reason,
		}
		return tx.Create(&action).Error
	})
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func DisableKeysForOpsRiskAudit(id int64, operatorID int, reason string, dryRun bool) (map[string]interface{}, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "群资格或账号风控不合规，需重新完成社区/QQ/TG 认证后恢复对应分组"
	}
	if dryRun {
		row, err := getOpsRiskAudit(model.DB, id, false)
		if err != nil {
			return nil, err
		}
		if err := validateOpsRiskDestructiveStatus(row.Status, false); err != nil {
			return nil, err
		}
		tokenIDs, err := requiredOpsRiskTokenIDs(row)
		if err != nil {
			return nil, err
		}
		tokens, err := listOpsRiskDisableCandidates(model.DB, row, tokenIDs)
		if err != nil {
			return nil, err
		}
		return newOpsRiskTokenResult(row, tokenIDs, tokens, true, "disabled_tokens"), nil
	}

	var result map[string]interface{}
	changedUsers := make([]int, 0)
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		row, err := getOpsRiskAudit(tx, id, true)
		if err != nil {
			return err
		}
		if err := validateOpsRiskDestructiveStatus(row.Status, false); err != nil {
			return err
		}
		tokenIDs, err := requiredOpsRiskTokenIDs(row)
		if err != nil {
			return err
		}
		tokens, err := listOpsRiskDisableCandidates(tx, row, tokenIDs)
		if err != nil {
			return err
		}
		result = newOpsRiskTokenResult(row, tokenIDs, tokens, false, "disabled_tokens")
		disabled, users, err := disableOpsRiskTokensWithTx(tx, row, tokens, operatorID, reason)
		if err != nil {
			return err
		}
		result["disabled_tokens"] = disabled
		changedUsers = append(changedUsers, users...)
		if disabled == 0 {
			return nil
		}
		return setOpsRiskAuditStatusWithTx(tx, row, OpsRiskStatusReviewing, time.Now().Unix())
	})
	if err != nil {
		return nil, err
	}
	invalidateOpsRiskUsers(changedUsers)
	return result, nil
}

func RestoreKeysForOpsRiskAudit(id int64, operatorID int, reason string, dryRun bool) (map[string]interface{}, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "管理员确认用户已重新满足群资格，恢复风控审计关联 API Key"
	}
	row, err := getOpsRiskAudit(model.DB, id, false)
	if err != nil {
		return nil, err
	}
	if err := validateOpsRiskDestructiveStatus(row.Status, true); err != nil {
		return nil, err
	}
	userID, accountControlled, err := userScopedAdminRiskControl(model.DB, row)
	if err != nil {
		return nil, err
	}
	if accountControlled {
		result, err := previewAdminRiskControlledTokens(row, userID)
		if err != nil {
			return nil, err
		}
		if dryRun {
			return result, nil
		}
		stats, err := restoreAdminRiskControlledTokens(row.SiteId, userID, operatorID, reason, row.RiskType)
		if err != nil {
			return nil, err
		}
		result["dry_run"] = false
		result["matched_tokens"] = stats.MatchedTokens
		result["restored_tokens"] = stats.RestoredTokens
		result["matched_user_controls"] = boolCount(stats.ActiveControl)
		result["released_user_controls"] = stats.ReleasedControls
		result["activated_records"] = stats.ActivatedRecords
		result["cancelled_stale_activations"] = stats.CancelledStaleActivations
		result["closed_audits"] = stats.ClosedAudits
		return result, nil
	}
	if dryRun {
		tokenIDs, err := requiredOpsRiskTokenIDs(row)
		if err != nil {
			return nil, err
		}
		candidates, err := listOpsRiskRestoreCandidates(model.DB, row, tokenIDs)
		if err != nil {
			return nil, err
		}
		return newOpsRiskTokenResult(row, tokenIDs, restoreCandidateTokens(candidates), true, "restored_tokens"), nil
	}

	var result map[string]interface{}
	changedUsers := make([]int, 0)
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		row, err := getOpsRiskAudit(tx, id, true)
		if err != nil {
			return err
		}
		if err := validateOpsRiskDestructiveStatus(row.Status, true); err != nil {
			return err
		}
		tokenIDs, err := requiredOpsRiskTokenIDs(row)
		if err != nil {
			return err
		}
		candidates, err := listOpsRiskRestoreCandidates(tx, row, tokenIDs)
		if err != nil {
			return err
		}
		result = newOpsRiskTokenResult(row, tokenIDs, restoreCandidateTokens(candidates), false, "restored_tokens")
		restored, users, err := restoreOpsRiskTokensWithTx(tx, row, candidates, operatorID, reason)
		if err != nil {
			return err
		}
		result["restored_tokens"] = restored
		changedUsers = append(changedUsers, users...)
		if restored == 0 {
			return nil
		}
		now := time.Now().Unix()
		releasedControls, err := releaseOpsRiskUserControlsWithTx(tx, row, users, now)
		if err != nil {
			return err
		}
		result["released_user_controls"] = releasedControls
		return setOpsRiskAuditStatusWithTx(tx, row, OpsRiskStatusClosed, now)
	})
	if err != nil {
		return nil, err
	}
	invalidateOpsRiskUsers(changedUsers)
	return result, nil
}

type AdminRiskKeyRestoreResult struct {
	ActiveControl             bool  `json:"active_control"`
	MatchedTokens             int64 `json:"matched_tokens"`
	RestoredTokens            int64 `json:"restored_tokens"`
	ActivatedRecords          int64 `json:"activated_records"`
	CancelledStaleActivations int64 `json:"cancelled_stale_activations"`
	ReleasedControls          int64 `json:"released_controls"`
	ClosedAudits              int64 `json:"closed_audits"`
}

type adminRiskRestoreState struct {
	ActiveControl        bool
	RestorableTokenCount int64
}

type adminRiskTokenRow struct {
	UserID  int `gorm:"column:user_id"`
	TokenID int `gorm:"column:token_id"`
}

func boolCount(value bool) int64 {
	if value {
		return 1
	}
	return 0
}

func userScopedAdminRiskControl(db *gorm.DB, row *model.OpsAccountRiskAudit) (int, bool, error) {
	if db == nil || row == nil {
		return 0, false, nil
	}
	userIDs := parseOpsRiskIntList(row.UserIds)
	if len(userIDs) != 1 || row.Subject != fmt.Sprintf("user:%d", userIDs[0]) {
		return 0, false, nil
	}
	var count int64
	err := db.Model(&model.RiskUserControl{}).
		Where("site_id = ? AND user_id = ? AND enabled = ? AND reason_code = ?", row.SiteId, userIDs[0], true, row.RiskType).
		Count(&count).Error
	return userIDs[0], count > 0, err
}

func loadAdminRiskRestoreTokenIDMap(db *gorm.DB, siteID string, userIDs []int) (map[int][]int, error) {
	result := make(map[int][]int, len(userIDs))
	userIDs = dedupePositiveInts(userIDs)
	if db == nil || len(userIDs) == 0 {
		return result, nil
	}
	tokenSets := make(map[int]map[int]struct{}, len(userIDs))
	var rows []adminRiskTokenRow
	if err := db.Table("risk_token_activations AS activation").
		Select("DISTINCT activation.user_id, activation.token_id").
		Joins("JOIN risk_user_controls AS control ON control.site_id = activation.site_id AND control.user_id = activation.user_id AND control.reason_code = activation.reason_code AND control.enabled = ?", true).
		Joins("JOIN tokens ON tokens.id = activation.token_id AND tokens.user_id = activation.user_id AND tokens.status = ? AND tokens.deleted_at IS NULL", common.TokenStatusDisabled).
		Where("activation.site_id = ? AND activation.user_id IN ? AND activation.status IN ?", siteID, userIDs, []string{model.RiskTokenActivationPending, model.RiskTokenActivationExpired}).
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		if tokenSets[row.UserID] == nil {
			tokenSets[row.UserID] = map[int]struct{}{}
		}
		tokenSets[row.UserID][row.TokenID] = struct{}{}
	}

	rows = nil
	if err := db.Table("ops_account_risk_actions AS action").
		Select("DISTINCT action.user_id, action.token_id").
		Joins("JOIN ops_account_risk_audit AS audit ON audit.id = action.audit_id AND audit.site_id = action.site_id AND audit.status IN ?", []string{OpsRiskStatusOpen, OpsRiskStatusReviewing}).
		Joins("JOIN risk_user_controls AS control ON control.site_id = action.site_id AND control.user_id = action.user_id AND control.reason_code = audit.risk_type AND control.enabled = ?", true).
		Joins("JOIN tokens ON tokens.id = action.token_id AND tokens.user_id = action.user_id AND tokens.status = ? AND tokens.deleted_at IS NULL", common.TokenStatusDisabled).
		Joins("LEFT JOIN ops_account_risk_actions AS restored ON restored.source_action_id = action.id AND restored.action_type = ?", model.OpsAccountRiskActionRestore).
		Where("action.site_id = ? AND action.user_id IN ? AND action.action_type = ? AND restored.id IS NULL", siteID, userIDs, model.OpsAccountRiskActionDisable).
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		if tokenSets[row.UserID] == nil {
			tokenSets[row.UserID] = map[int]struct{}{}
		}
		tokenSets[row.UserID][row.TokenID] = struct{}{}
	}
	for _, userID := range userIDs {
		ids := make([]int, 0, len(tokenSets[userID]))
		for tokenID := range tokenSets[userID] {
			ids = append(ids, tokenID)
		}
		sort.Ints(ids)
		result[userID] = ids
	}
	return result, nil
}

func previewAdminRiskControlledTokens(row *model.OpsAccountRiskAudit, userID int) (map[string]interface{}, error) {
	tokenIDsByUser, err := loadAdminRiskRestoreTokenIDMap(model.DB, row.SiteId, []int{userID})
	if err != nil {
		return nil, err
	}
	tokenIDs := tokenIDsByUser[userID]
	tokens := make([]model.Token, 0, len(tokenIDs))
	if len(tokenIDs) > 0 {
		if err := opsRiskTokenQuery(model.DB, tokenIDs, common.TokenStatusDisabled).
			Where("tokens.user_id = ?", userID).Order("tokens.id asc").Find(&tokens).Error; err != nil {
			return nil, err
		}
	}
	result := newOpsRiskTokenResult(row, tokenIDs, tokens, true, "restored_tokens")
	result["matched_user_controls"] = int64(1)
	result["released_user_controls"] = int64(0)
	return result, nil
}

func loadAdminRiskRestoreStateMap(siteID string, userIDs []int) (map[int]adminRiskRestoreState, error) {
	result := make(map[int]adminRiskRestoreState, len(userIDs))
	userIDs = dedupePositiveInts(userIDs)
	if len(userIDs) == 0 {
		return result, nil
	}
	siteID = strings.TrimSpace(siteID)
	if siteID == "" {
		siteID = AgentSiteID()
	}

	var controls []model.RiskUserControl
	if err := model.DB.Select("user_id").
		Where("site_id = ? AND user_id IN ? AND enabled = ?", siteID, userIDs, true).
		Find(&controls).Error; err != nil {
		return nil, err
	}
	for _, control := range controls {
		result[control.UserId] = adminRiskRestoreState{ActiveControl: true}
	}
	if len(controls) == 0 {
		return result, nil
	}

	tokenIDsByUser, err := loadAdminRiskRestoreTokenIDMap(model.DB, siteID, userIDs)
	if err != nil {
		return nil, err
	}
	for userID, state := range result {
		state.RestorableTokenCount = int64(len(tokenIDsByUser[userID]))
		result[userID] = state
	}
	return result, nil
}

func RestoreAdminRiskControlledTokens(siteID string, userID int, operatorID int, reason string) (*AdminRiskKeyRestoreResult, error) {
	return restoreAdminRiskControlledTokens(siteID, userID, operatorID, reason, "")
}

func restoreAdminRiskControlledTokens(siteID string, userID int, operatorID int, reason string, expectedReasonCode string) (*AdminRiskKeyRestoreResult, error) {
	result := &AdminRiskKeyRestoreResult{}
	if userID <= 0 {
		return result, errors.New("invalid user id")
	}
	siteID = strings.TrimSpace(siteID)
	if siteID == "" {
		siteID = AgentSiteID()
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "管理员从用户管理恢复 Key 使用权限"
	}

	err := model.DB.Transaction(func(tx *gorm.DB) error {
		var user model.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Select("id", "role").Where("id = ?", userID).First(&user).Error; err != nil {
			return err
		}
		if user.Role >= common.RoleAdminUser {
			return nil
		}

		var control model.RiskUserControl
		controlQuery := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("site_id = ? AND user_id = ? AND enabled = ?", siteID, userID, true)
		if expectedReasonCode = strings.TrimSpace(expectedReasonCode); expectedReasonCode != "" {
			controlQuery = controlQuery.Where("reason_code = ?", expectedReasonCode)
		}
		err := controlQuery.First(&control).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		result.ActiveControl = true

		var activations []model.RiskTokenActivation
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("site_id = ? AND user_id = ? AND reason_code = ? AND status IN ?", siteID, userID, control.ReasonCode, []string{model.RiskTokenActivationPending, model.RiskTokenActivationExpired}).
			Order("id desc").Find(&activations).Error; err != nil {
			return err
		}
		activationTokenIDs := make([]int, 0, len(activations))
		for _, activation := range activations {
			activationTokenIDs = append(activationTokenIDs, activation.TokenId)
		}
		activationTokenIDs = dedupePositiveInts(activationTokenIDs)

		riskType := firstAgentNonEmpty(control.ReasonCode, "risk_control_triggered")
		subject := fmt.Sprintf("user:%d", userID)
		var audit model.OpsAccountRiskAudit
		err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("site_id = ? AND risk_type = ? AND subject = ?", siteID, riskType, subject).
			First(&audit).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			audit = model.OpsAccountRiskAudit{
				SiteId: siteID, RiskType: riskType, Severity: "high", Subject: subject,
				UserIds: encodeOpsRiskIntList([]int{userID}), TokenIds: encodeOpsRiskIntList(activationTokenIDs),
				Status:   OpsRiskStatusReviewing,
				Evidence: mustAgentJSON(map[string]any{"source": "admin_user_key_restore", "user_id": userID}),
			}
			if err := tx.Create(&audit).Error; err != nil {
				return err
			}
		} else if err != nil {
			return err
		} else {
			mergedTokenIDs := mergeOpsRiskIntLists(parseOpsRiskIntList(audit.TokenIds), activationTokenIDs)
			if err := tx.Model(&model.OpsAccountRiskAudit{}).Where("id = ?", audit.Id).
				Update("token_ids", encodeOpsRiskIntList(mergedTokenIDs)).Error; err != nil {
				return err
			}
			audit.TokenIds = encodeOpsRiskIntList(mergedTokenIDs)
		}

		restoredTokenIDs := make(map[int]struct{})
		if tokenIDs := parseOpsRiskIntList(audit.TokenIds); len(tokenIDs) > 0 {
			candidates, err := listOpsRiskRestoreCandidates(tx, &audit, tokenIDs)
			if err != nil {
				return err
			}
			result.MatchedTokens += int64(len(candidates))
			restored, _, err := restoreOpsRiskTokensWithTx(tx, &audit, candidates, operatorID, reason)
			if err != nil {
				return err
			}
			result.RestoredTokens += restored
			for _, candidate := range candidates {
				restoredTokenIDs[candidate.Token.Id] = struct{}{}
			}
		}

		var activationTokens []model.Token
		if len(activationTokenIDs) > 0 {
			if err := tx.Where("user_id = ? AND id IN ? AND status = ?", userID, activationTokenIDs, common.TokenStatusDisabled).
				Order("id asc").Find(&activationTokens).Error; err != nil {
				return err
			}
		}
		for _, token := range activationTokens {
			if _, alreadyRestored := restoredTokenIDs[token.Id]; alreadyRestored {
				continue
			}
			result.MatchedTokens++
			updated := tx.Model(&model.Token{}).
				Where("id = ? AND user_id = ? AND status = ?", token.Id, userID, common.TokenStatusDisabled).
				Update("status", common.TokenStatusEnabled)
			if updated.Error != nil {
				return updated.Error
			}
			if updated.RowsAffected == 0 {
				continue
			}
			tokenID := token.Id
			uid := userID
			action := model.OpsAccountRiskAction{
				SiteId: siteID, AuditId: audit.Id, ActionKey: fmt.Sprintf("admin-activation-restore:%d:%d", control.Id, token.Id),
				ActionType: model.OpsAccountRiskActionRestore, TokenId: &tokenID, UserId: &uid,
				TokenName: token.Name, TokenGroup: token.Group, PreviousTokenStatus: common.TokenStatusDisabled,
				NewTokenStatus: common.TokenStatusEnabled, PreviousAuditStatus: audit.Status, NewAuditStatus: OpsRiskStatusClosed,
				OperatorId: operatorID, Reason: reason,
			}
			if err := tx.Create(&action).Error; err != nil {
				return err
			}
			restoredTokenIDs[token.Id] = struct{}{}
			result.RestoredTokens++
		}

		now := time.Now().Unix()
		if len(restoredTokenIDs) > 0 {
			ids := make([]int, 0, len(restoredTokenIDs))
			for tokenID := range restoredTokenIDs {
				ids = append(ids, tokenID)
			}
			activated := tx.Model(&model.RiskTokenActivation{}).
				Where("site_id = ? AND user_id = ? AND reason_code = ? AND token_id IN ? AND status IN ?", siteID, userID, control.ReasonCode, ids, []string{model.RiskTokenActivationPending, model.RiskTokenActivationExpired}).
				Updates(map[string]interface{}{"status": model.RiskTokenActivationActivated, "activated_at": now, "updated_at": now})
			if activated.Error != nil {
				return activated.Error
			}
			result.ActivatedRecords = activated.RowsAffected
		}
		cancelled := tx.Model(&model.RiskTokenActivation{}).
			Where("site_id = ? AND user_id = ? AND reason_code = ? AND status IN ?", siteID, userID, control.ReasonCode, []string{model.RiskTokenActivationPending, model.RiskTokenActivationExpired}).
			Updates(map[string]interface{}{"status": model.RiskTokenActivationCancelled, "updated_at": now})
		if cancelled.Error != nil {
			return cancelled.Error
		}
		result.CancelledStaleActivations = cancelled.RowsAffected

		released := tx.Model(&model.RiskUserControl{}).
			Where("id = ? AND site_id = ? AND user_id = ? AND enabled = ?", control.Id, siteID, userID, true).
			Updates(map[string]interface{}{
				"enabled": false, "key_recreate_required": false, "activation_required": false,
				"existing_keys_disabled_at": 0, "updated_at": now,
			})
		if released.Error != nil {
			return released.Error
		}
		result.ReleasedControls = released.RowsAffected

		uid := userID
		statusAction := model.OpsAccountRiskAction{
			SiteId: siteID, AuditId: audit.Id, ActionKey: fmt.Sprintf("admin-control-release:%d", control.Id),
			ActionType: model.OpsAccountRiskActionStatus, UserId: &uid,
			PreviousAuditStatus: audit.Status, NewAuditStatus: OpsRiskStatusClosed,
			OperatorId: operatorID, Reason: reason,
		}
		if err := tx.Create(&statusAction).Error; err != nil {
			return err
		}
		if audit.Status != OpsRiskStatusClosed {
			if err := setOpsRiskAuditStatusWithTx(tx, &audit, OpsRiskStatusClosed, now); err != nil {
				return err
			}
			result.ClosedAudits = 1
		}
		return nil
	})
	if err != nil {
		return result, err
	}
	if result.ActiveControl {
		invalidateOpsRiskUsers([]int{userID})
		_ = model.InvalidateUserCache(userID)
	}
	return result, nil
}

func getOpsRiskAudit(db *gorm.DB, id int64, forUpdate bool) (*model.OpsAccountRiskAudit, error) {
	query := db.Where("id = ? AND site_id = ?", id, AgentSiteID())
	if forUpdate {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	var row model.OpsAccountRiskAudit
	if err := query.First(&row).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

func validateOpsRiskDestructiveStatus(status string, _ bool) error {
	switch status {
	case OpsRiskStatusIgnored, OpsRiskStatusClosed:
		return fmt.Errorf("risk audit status %s does not allow destructive actions", status)
	case OpsRiskStatusOpen, OpsRiskStatusReviewing:
	default:
		return fmt.Errorf("invalid current risk audit status: %s", status)
	}
	return nil
}

func requiredOpsRiskTokenIDs(row *model.OpsAccountRiskAudit) ([]int, error) {
	tokenIDs := parseOpsRiskIntList(row.TokenIds)
	if len(tokenIDs) == 0 {
		return nil, errors.New("risk audit has no token_ids")
	}
	return tokenIDs, nil
}

func opsRiskTokenQuery(db *gorm.DB, tokenIDs []int, status int) *gorm.DB {
	return db.Model(&model.Token{}).
		Where("tokens.id IN ?", tokenIDs).
		Where("tokens.status = ?", status).
		Where("tokens.user_id NOT IN (?)", db.Model(&model.User{}).Select("id").Where("role >= ?", common.RoleAdminUser))
}

func listOpsRiskDisableCandidates(db *gorm.DB, row *model.OpsAccountRiskAudit, tokenIDs []int) ([]model.Token, error) {
	usedTokenIDs := db.Model(&model.OpsAccountRiskAction{}).
		Select("token_id").
		Where("site_id = ? AND audit_id = ? AND action_type = ? AND token_id IS NOT NULL", row.SiteId, row.Id, model.OpsAccountRiskActionDisable)
	var tokens []model.Token
	err := opsRiskTokenQuery(db, tokenIDs, common.TokenStatusEnabled).
		Where("tokens.id NOT IN (?)", usedTokenIDs).
		Order("tokens.id asc").
		Find(&tokens).Error
	return tokens, err
}

type opsRiskRestoreCandidate struct {
	DisableAction model.OpsAccountRiskAction
	Token         model.Token
}

func listOpsRiskRestoreCandidates(db *gorm.DB, row *model.OpsAccountRiskAudit, tokenIDs []int) ([]opsRiskRestoreCandidate, error) {
	var disableActions []model.OpsAccountRiskAction
	if err := db.Where(
		"site_id = ? AND audit_id = ? AND action_type = ? AND previous_token_status = ? AND new_token_status = ? AND token_id IN ?",
		row.SiteId, row.Id, model.OpsAccountRiskActionDisable, common.TokenStatusEnabled, common.TokenStatusDisabled, tokenIDs,
	).Order("id asc").Find(&disableActions).Error; err != nil {
		return nil, err
	}
	if len(disableActions) == 0 {
		return []opsRiskRestoreCandidate{}, nil
	}
	actionIDs := make([]int64, 0, len(disableActions))
	for _, action := range disableActions {
		actionIDs = append(actionIDs, action.Id)
	}
	var restoredSourceIDs []int64
	if err := db.Model(&model.OpsAccountRiskAction{}).
		Where("action_type = ? AND source_action_id IN ?", model.OpsAccountRiskActionRestore, actionIDs).
		Pluck("source_action_id", &restoredSourceIDs).Error; err != nil {
		return nil, err
	}
	restored := make(map[int64]bool, len(restoredSourceIDs))
	for _, sourceID := range restoredSourceIDs {
		restored[sourceID] = true
	}
	eligibleTokenIDs := make([]int, 0, len(disableActions))
	for _, action := range disableActions {
		if !restored[action.Id] && action.TokenId != nil {
			eligibleTokenIDs = append(eligibleTokenIDs, *action.TokenId)
		}
	}
	if len(eligibleTokenIDs) == 0 {
		return []opsRiskRestoreCandidate{}, nil
	}
	var tokens []model.Token
	if err := opsRiskTokenQuery(db, eligibleTokenIDs, common.TokenStatusDisabled).Find(&tokens).Error; err != nil {
		return nil, err
	}
	tokensByID := make(map[int]model.Token, len(tokens))
	for _, token := range tokens {
		tokensByID[token.Id] = token
	}
	candidates := make([]opsRiskRestoreCandidate, 0, len(tokens))
	for _, action := range disableActions {
		if restored[action.Id] || action.TokenId == nil {
			continue
		}
		if token, ok := tokensByID[*action.TokenId]; ok {
			candidates = append(candidates, opsRiskRestoreCandidate{DisableAction: action, Token: token})
		}
	}
	return candidates, nil
}

func restoreCandidateTokens(candidates []opsRiskRestoreCandidate) []model.Token {
	tokens := make([]model.Token, 0, len(candidates))
	for _, candidate := range candidates {
		tokens = append(tokens, candidate.Token)
	}
	return tokens
}

func newOpsRiskTokenResult(row *model.OpsAccountRiskAudit, tokenIDs []int, tokens []model.Token, dryRun bool, countKey string) map[string]interface{} {
	details := make([]OpsRiskTokenDetail, 0, len(tokens))
	for _, token := range tokens {
		details = append(details, OpsRiskTokenDetail{Id: token.Id, UserId: token.UserId, Name: token.Name, Status: token.Status, Group: token.Group})
	}
	return map[string]interface{}{
		"audit_id": row.Id, "user_ids": parseOpsRiskIntList(row.UserIds), "token_ids": tokenIDs,
		"dry_run": dryRun, "tokens": details, "matched_tokens": int64(len(tokens)), countKey: int64(0),
	}
}

func disableOpsRiskTokensWithTx(tx *gorm.DB, row *model.OpsAccountRiskAudit, tokens []model.Token, operatorID int, reason string) (int64, []int, error) {
	var disabled int64
	changedUsers := make([]int, 0, len(tokens))
	for _, token := range tokens {
		res := tx.Model(&model.Token{}).
			Where("id = ? AND status = ?", token.Id, common.TokenStatusEnabled).
			Where("user_id NOT IN (?)", tx.Model(&model.User{}).Select("id").Where("role >= ?", common.RoleAdminUser)).
			Update("status", common.TokenStatusDisabled)
		if res.Error != nil {
			return 0, nil, res.Error
		}
		if res.RowsAffected == 0 {
			continue
		}
		tokenID, userID := token.Id, token.UserId
		action := model.OpsAccountRiskAction{
			SiteId: row.SiteId, AuditId: row.Id, ActionKey: fmt.Sprintf("disable:%d", token.Id), ActionType: model.OpsAccountRiskActionDisable,
			TokenId: &tokenID, UserId: &userID, TokenName: token.Name, TokenGroup: token.Group,
			PreviousTokenStatus: common.TokenStatusEnabled, NewTokenStatus: common.TokenStatusDisabled,
			PreviousAuditStatus: row.Status, NewAuditStatus: OpsRiskStatusReviewing,
			OperatorId: operatorID, Reason: reason,
		}
		if err := tx.Create(&action).Error; err != nil {
			return 0, nil, err
		}
		disabled++
		changedUsers = append(changedUsers, token.UserId)
	}
	return disabled, changedUsers, nil
}

func restoreOpsRiskTokensWithTx(tx *gorm.DB, row *model.OpsAccountRiskAudit, candidates []opsRiskRestoreCandidate, operatorID int, reason string) (int64, []int, error) {
	var restored int64
	changedUsers := make([]int, 0, len(candidates))
	for _, candidate := range candidates {
		token := candidate.Token
		res := tx.Model(&model.Token{}).
			Where("id = ? AND status = ?", token.Id, common.TokenStatusDisabled).
			Where("user_id NOT IN (?)", tx.Model(&model.User{}).Select("id").Where("role >= ?", common.RoleAdminUser)).
			Update("status", common.TokenStatusEnabled)
		if res.Error != nil {
			return 0, nil, res.Error
		}
		if res.RowsAffected == 0 {
			continue
		}
		tokenID, userID := token.Id, token.UserId
		sourceActionID := candidate.DisableAction.Id
		action := model.OpsAccountRiskAction{
			SiteId: row.SiteId, AuditId: row.Id, ActionKey: fmt.Sprintf("restore:%d", sourceActionID), ActionType: model.OpsAccountRiskActionRestore,
			TokenId: &tokenID, UserId: &userID, SourceActionId: &sourceActionID, TokenName: token.Name, TokenGroup: token.Group,
			PreviousTokenStatus: common.TokenStatusDisabled, NewTokenStatus: common.TokenStatusEnabled,
			PreviousAuditStatus: row.Status, NewAuditStatus: OpsRiskStatusClosed,
			OperatorId: operatorID, Reason: reason,
		}
		if err := tx.Create(&action).Error; err != nil {
			return 0, nil, err
		}
		restored++
		changedUsers = append(changedUsers, token.UserId)
	}
	return restored, changedUsers, nil
}

func releaseOpsRiskUserControlsWithTx(tx *gorm.DB, row *model.OpsAccountRiskAudit, userIDs []int, updatedAt int64) (int64, error) {
	userIDs = dedupePositiveInts(userIDs)
	if tx == nil || row == nil || len(userIDs) == 0 || strings.TrimSpace(row.RiskType) == "" {
		return 0, nil
	}
	result := tx.Model(&model.RiskUserControl{}).
		Where("site_id = ? AND user_id IN ? AND enabled = ? AND reason_code = ?", row.SiteId, userIDs, true, row.RiskType).
		Updates(map[string]interface{}{
			"enabled":                   false,
			"key_recreate_required":     false,
			"activation_required":       false,
			"existing_keys_disabled_at": 0,
			"updated_at":                updatedAt,
		})
	return result.RowsAffected, result.Error
}

func setOpsRiskAuditStatusWithTx(tx *gorm.DB, row *model.OpsAccountRiskAudit, status string, updatedAt int64) error {
	if row.Status == status {
		return nil
	}
	res := tx.Model(&model.OpsAccountRiskAudit{}).
		Where("id = ? AND site_id = ? AND status = ?", row.Id, row.SiteId, row.Status).
		Updates(map[string]interface{}{"status": status, "updated_at": updatedAt})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		return errors.New("risk audit status changed concurrently")
	}
	row.Status = status
	row.UpdatedAt = updatedAt
	return nil
}

func invalidateOpsRiskUsers(userIDs []int) {
	for _, userID := range dedupePositiveInts(userIDs) {
		_ = model.InvalidateUserTokensCache(userID)
	}
}

func UpsertOpsAccountRiskAudit(row *model.OpsAccountRiskAudit) error {
	return upsertOpsAccountRiskAuditWithTx(model.DB, row)
}

func upsertOpsAccountRiskAuditWithTx(tx *gorm.DB, row *model.OpsAccountRiskAudit) error {
	if tx == nil {
		tx = model.DB
	}
	if row == nil {
		return errors.New("nil risk audit")
	}
	row.SiteId = strings.TrimSpace(row.SiteId)
	if row.SiteId == "" {
		row.SiteId = AgentSiteID()
	}
	row.RiskType = strings.TrimSpace(row.RiskType)
	row.Subject = strings.TrimSpace(row.Subject)
	if row.RiskType == "" || row.Subject == "" {
		return errors.New("risk_type and subject are required")
	}
	row.Severity = strings.ToLower(strings.TrimSpace(row.Severity))
	if row.Severity == "" {
		row.Severity = "medium"
	}
	row.Status = strings.ToLower(strings.TrimSpace(row.Status))
	if row.Status == "" {
		row.Status = OpsRiskStatusOpen
	}
	if _, err := normalizeOpsRiskStatus(row.Status); err != nil {
		return err
	}
	row.UserIds = encodeOpsRiskIntList(parseOpsRiskIntList(row.UserIds))
	row.TokenIds = encodeOpsRiskIntList(parseOpsRiskIntList(row.TokenIds))
	if strings.TrimSpace(row.Evidence) == "" {
		row.Evidence = "{}"
	}

	var existing model.OpsAccountRiskAudit
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("site_id = ? AND risk_type = ? AND subject = ?", row.SiteId, row.RiskType, row.Subject).
		First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		created := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "site_id"}, {Name: "risk_type"}, {Name: "subject"}},
			DoNothing: true,
		}).Create(row)
		if created.Error != nil {
			return created.Error
		}
		if created.RowsAffected == 1 {
			return nil
		}
		err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("site_id = ? AND risk_type = ? AND subject = ?", row.SiteId, row.RiskType, row.Subject).
			First(&existing).Error
	}
	if err != nil {
		return err
	}
	if _, err := normalizeOpsRiskStatus(existing.Status); err != nil {
		return err
	}
	userIDs := mergeOpsRiskIntLists(parseOpsRiskIntList(existing.UserIds), parseOpsRiskIntList(row.UserIds))
	tokenIDs := mergeOpsRiskIntLists(parseOpsRiskIntList(existing.TokenIds), parseOpsRiskIntList(row.TokenIds))
	now := time.Now().Unix()
	updates := map[string]interface{}{
		"severity": row.Severity, "user_ids": encodeOpsRiskIntList(userIDs), "token_ids": encodeOpsRiskIntList(tokenIDs),
		"ip": row.Ip, "evidence": row.Evidence, "updated_at": now,
	}
	if err := tx.Model(&model.OpsAccountRiskAudit{}).Where("id = ? AND site_id = ?", existing.Id, existing.SiteId).Updates(updates).Error; err != nil {
		return err
	}
	existing.Severity = row.Severity
	existing.UserIds = updates["user_ids"].(string)
	existing.TokenIds = updates["token_ids"].(string)
	existing.Ip = row.Ip
	existing.Evidence = row.Evidence
	existing.UpdatedAt = now
	*row = existing
	return nil
}

func mergeOpsRiskIntLists(lists ...[]int) []int {
	seen := map[int]bool{}
	values := make([]int, 0)
	for _, list := range lists {
		for _, value := range list {
			if value > 0 && !seen[value] {
				seen[value] = true
				values = append(values, value)
			}
		}
	}
	sort.Ints(values)
	return values
}

func encodeOpsRiskIntList(values []int) string {
	data, _ := json.Marshal(dedupePositiveInts(values))
	return string(data)
}

func parseOpsRiskIntList(raw string) []int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var arr []int
	if err := json.Unmarshal([]byte(raw), &arr); err == nil {
		return dedupePositiveInts(arr)
	}
	var floats []float64
	if err := json.Unmarshal([]byte(raw), &floats); err == nil {
		for _, v := range floats {
			if v > 0 {
				arr = append(arr, int(v))
			}
		}
		return dedupePositiveInts(arr)
	}
	parts := strings.FieldsFunc(raw, func(r rune) bool { return r == ',' || r == ' ' || r == '\n' || r == '\t' || r == ';' })
	for _, part := range parts {
		if n, err := strconv.Atoi(strings.Trim(part, "[]\"'")); err == nil && n > 0 {
			arr = append(arr, n)
		}
	}
	return dedupePositiveInts(arr)
}

func dedupePositiveInts(values []int) []int {
	seen := map[int]bool{}
	out := make([]int, 0, len(values))
	for _, v := range values {
		if v > 0 && !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}
