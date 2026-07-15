package service

import (
	"fmt"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func TestOpsRiskStatusesForQuery(t *testing.T) {
	tests := []struct {
		name   string
		status string
		want   []string
	}{
		{name: "all statuses", status: "", want: nil},
		{name: "active audits", status: OpsRiskStatusActive, want: []string{OpsRiskStatusOpen, OpsRiskStatusReviewing}},
		{name: "single status", status: OpsRiskStatusReviewing, want: []string{OpsRiskStatusReviewing}},
		{name: "trimmed status", status: " closed ", want: []string{OpsRiskStatusClosed}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, opsRiskStatusesForQuery(tt.status))
		})
	}
}

func TestValidateOpsRiskDestructiveStatus(t *testing.T) {
	tests := []struct {
		name      string
		status    string
		restoring bool
		wantError bool
	}{
		{name: "open allows disable", status: OpsRiskStatusOpen},
		{name: "open allows restore", status: OpsRiskStatusOpen, restoring: true},
		{name: "reviewing allows disable", status: OpsRiskStatusReviewing},
		{name: "reviewing allows restore", status: OpsRiskStatusReviewing, restoring: true},
		{name: "ignored blocks disable", status: OpsRiskStatusIgnored, wantError: true},
		{name: "ignored blocks restore", status: OpsRiskStatusIgnored, restoring: true, wantError: true},
		{name: "closed blocks disable", status: OpsRiskStatusClosed, wantError: true},
		{name: "closed blocks restore", status: OpsRiskStatusClosed, restoring: true, wantError: true},
		{name: "query-only active blocks actions", status: OpsRiskStatusActive, wantError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateOpsRiskDestructiveStatus(tt.status, tt.restoring)
			if tt.wantError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestRestoreKeysForOpsRiskAuditRestoresActivationBackedKeyAndControl(t *testing.T) {
	require.NoError(t, model.DB.AutoMigrate(
		&model.User{},
		&model.Token{},
		&model.RiskUserControl{},
		&model.RiskTokenActivation{},
		&model.OpsAccountRiskAudit{},
		&model.OpsAccountRiskAction{},
	))

	suffix := time.Now().UnixNano()
	user := model.User{
		Username: fmt.Sprintf("audit_restore_%d", suffix), Password: "test-password",
		Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: "default",
		AffCode: fmt.Sprintf("ar_%d", suffix),
	}
	require.NoError(t, model.DB.Create(&user).Error)
	siteID := AgentSiteID()
	reasonCode := fmt.Sprintf("audit_activation_%d", suffix)
	restorable := model.Token{
		UserId: user.Id, Key: fmt.Sprintf("sk-audit-restorable-%d", suffix), Status: common.TokenStatusDisabled,
		Name: "risk-owned", Group: "default", ExpiredTime: -1,
	}
	manualDisabled := model.Token{
		UserId: user.Id, Key: fmt.Sprintf("sk-audit-manual-%d", suffix), Status: common.TokenStatusDisabled,
		Name: "manual-disabled", Group: "default", ExpiredTime: -1,
	}
	require.NoError(t, model.DB.Create(&restorable).Error)
	require.NoError(t, model.DB.Create(&manualDisabled).Error)
	control := model.RiskUserControl{
		SiteId: siteID, UserId: user.Id, RiskLevel: "high", ReasonCode: reasonCode,
		Reason: "test", Enabled: true, KeyRecreateRequired: true, ActivationRequired: true,
		ActivationSource: "qq", ExistingKeysDisabledAt: time.Now().Unix(),
	}
	require.NoError(t, model.DB.Create(&control).Error)
	activation := model.RiskTokenActivation{
		SiteId: siteID, UserId: user.Id, TokenId: restorable.Id, ActivationSource: "qq",
		Status: model.RiskTokenActivationPending, ReasonCode: reasonCode, ExpiresAt: time.Now().Add(time.Minute).Unix(),
	}
	require.NoError(t, model.DB.Create(&activation).Error)
	audit := model.OpsAccountRiskAudit{
		SiteId: siteID, RiskType: reasonCode, Severity: "high", Subject: fmt.Sprintf("user:%d", user.Id),
		UserIds: encodeOpsRiskIntList([]int{user.Id}), TokenIds: "[]", Evidence: `{"source":"activation"}`, Status: OpsRiskStatusOpen,
	}
	require.NoError(t, model.DB.Create(&audit).Error)

	t.Cleanup(func() {
		_ = model.DB.Where("audit_id = ?", audit.Id).Delete(&model.OpsAccountRiskAction{}).Error
		_ = model.DB.Where("site_id = ? AND user_id = ?", siteID, user.Id).Delete(&model.RiskTokenActivation{}).Error
		_ = model.DB.Where("site_id = ? AND user_id = ?", siteID, user.Id).Delete(&model.RiskUserControl{}).Error
		_ = model.DB.Where("id = ?", audit.Id).Delete(&model.OpsAccountRiskAudit{}).Error
		_ = model.DB.Unscoped().Where("user_id = ?", user.Id).Delete(&model.Token{}).Error
		_ = model.DB.Unscoped().Where("id = ?", user.Id).Delete(&model.User{}).Error
	})

	preview, err := RestoreKeysForOpsRiskAudit(audit.Id, 77, "review complete", true)
	require.NoError(t, err)
	require.EqualValues(t, 1, preview["matched_tokens"])
	require.EqualValues(t, 1, preview["matched_user_controls"])
	require.Equal(t, true, preview["dry_run"])

	result, err := RestoreKeysForOpsRiskAudit(audit.Id, 77, "review complete", false)
	require.NoError(t, err)
	require.EqualValues(t, 1, result["restored_tokens"])
	require.EqualValues(t, 1, result["released_user_controls"])

	require.NoError(t, model.DB.First(&restorable, restorable.Id).Error)
	require.Equal(t, common.TokenStatusEnabled, restorable.Status)
	require.NoError(t, model.DB.First(&manualDisabled, manualDisabled.Id).Error)
	require.Equal(t, common.TokenStatusDisabled, manualDisabled.Status)
	require.NoError(t, model.DB.First(&activation, activation.Id).Error)
	require.Equal(t, model.RiskTokenActivationActivated, activation.Status)
	require.NoError(t, model.DB.First(&control, control.Id).Error)
	require.False(t, control.Enabled)
	require.NoError(t, model.DB.First(&audit, audit.Id).Error)
	require.Equal(t, OpsRiskStatusClosed, audit.Status)
}

func TestRestoreKeysForOpsRiskAuditReleasesPermissionOnlyControl(t *testing.T) {
	require.NoError(t, model.DB.AutoMigrate(
		&model.User{},
		&model.Token{},
		&model.RiskUserControl{},
		&model.RiskTokenActivation{},
		&model.OpsAccountRiskAudit{},
		&model.OpsAccountRiskAction{},
	))

	suffix := time.Now().UnixNano()
	user := model.User{
		Username: fmt.Sprintf("permission_restore_%d", suffix), Password: "test-password",
		Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: "default",
		AffCode: fmt.Sprintf("pr_%d", suffix),
	}
	require.NoError(t, model.DB.Create(&user).Error)
	siteID := AgentSiteID()
	reasonCode := fmt.Sprintf("permission_only_%d", suffix)
	control := model.RiskUserControl{
		SiteId: siteID, UserId: user.Id, RiskLevel: "high", ReasonCode: reasonCode,
		Reason: "test", Enabled: true, KeyRecreateRequired: true, ActivationRequired: true,
		ActivationSource: "qq", ExistingKeysDisabledAt: time.Now().Unix(),
	}
	require.NoError(t, model.DB.Create(&control).Error)
	audit := model.OpsAccountRiskAudit{
		SiteId: siteID, RiskType: reasonCode, Severity: "high", Subject: fmt.Sprintf("user:%d", user.Id),
		UserIds: encodeOpsRiskIntList([]int{user.Id}), TokenIds: "[]", Evidence: `{}`, Status: OpsRiskStatusReviewing,
	}
	require.NoError(t, model.DB.Create(&audit).Error)

	t.Cleanup(func() {
		_ = model.DB.Where("audit_id = ?", audit.Id).Delete(&model.OpsAccountRiskAction{}).Error
		_ = model.DB.Where("site_id = ? AND user_id = ?", siteID, user.Id).Delete(&model.RiskUserControl{}).Error
		_ = model.DB.Where("id = ?", audit.Id).Delete(&model.OpsAccountRiskAudit{}).Error
		_ = model.DB.Unscoped().Where("id = ?", user.Id).Delete(&model.User{}).Error
	})

	preview, err := RestoreKeysForOpsRiskAudit(audit.Id, 77, "permission reviewed", true)
	require.NoError(t, err)
	require.Zero(t, preview["matched_tokens"])
	require.EqualValues(t, 1, preview["matched_user_controls"])

	result, err := RestoreKeysForOpsRiskAudit(audit.Id, 77, "permission reviewed", false)
	require.NoError(t, err)
	require.Zero(t, result["restored_tokens"])
	require.EqualValues(t, 1, result["released_user_controls"])
	require.NoError(t, model.DB.First(&control, control.Id).Error)
	require.False(t, control.Enabled)
	require.NoError(t, model.DB.First(&audit, audit.Id).Error)
	require.Equal(t, OpsRiskStatusClosed, audit.Status)
}

func TestActiveRiskStatusIsQueryOnly(t *testing.T) {
	_, err := normalizeOpsRiskStatus(OpsRiskStatusActive)
	require.Error(t, err)
}

func TestOpsRiskAuditEnforcementActive(t *testing.T) {
	tests := []struct {
		status     string
		wantActive bool
		wantError  bool
	}{
		{status: OpsRiskStatusOpen, wantActive: true},
		{status: OpsRiskStatusReviewing, wantActive: true},
		{status: OpsRiskStatusIgnored},
		{status: OpsRiskStatusClosed},
		{status: OpsRiskStatusActive, wantError: true},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			active, err := opsRiskAuditEnforcementActive(tt.status)
			if tt.wantError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.wantActive, active)
		})
	}
}

func TestReleaseOpsRiskUserControlsMatchesAuditOwnership(t *testing.T) {
	require.NoError(t, model.DB.AutoMigrate(&model.RiskUserControl{}))
	require.NoError(t, model.DB.Exec("DELETE FROM risk_user_controls").Error)
	t.Cleanup(func() {
		_ = model.DB.Exec("DELETE FROM risk_user_controls").Error
	})

	now := time.Now().Unix()
	controls := []model.RiskUserControl{
		{SiteId: "prox", UserId: 1001, RiskLevel: "high", ReasonCode: "inactive_token_disable", Enabled: true, KeyRecreateRequired: true, ActivationRequired: true, ExistingKeysDisabledAt: now},
		{SiteId: "prox", UserId: 1002, RiskLevel: "high", ReasonCode: "dynamic_ip_churn", Enabled: true, KeyRecreateRequired: true, ActivationRequired: true, ExistingKeysDisabledAt: now},
		{SiteId: "prox", UserId: 1003, RiskLevel: "high", ReasonCode: "inactive_token_disable", Enabled: true, KeyRecreateRequired: true, ActivationRequired: true, ExistingKeysDisabledAt: now},
	}
	require.NoError(t, model.DB.Create(&controls).Error)

	audit := &model.OpsAccountRiskAudit{SiteId: "prox", RiskType: "inactive_token_disable"}
	released, err := releaseOpsRiskUserControlsWithTx(model.DB, audit, []int{1001, 1002}, now+1)
	require.NoError(t, err)
	require.EqualValues(t, 1, released)

	var matching model.RiskUserControl
	require.NoError(t, model.DB.Where("site_id = ? AND user_id = ?", "prox", 1001).First(&matching).Error)
	require.False(t, matching.Enabled)
	require.False(t, matching.KeyRecreateRequired)
	require.False(t, matching.ActivationRequired)
	require.Zero(t, matching.ExistingKeysDisabledAt)

	for _, userID := range []int{1002, 1003} {
		var untouched model.RiskUserControl
		require.NoError(t, model.DB.Where("site_id = ? AND user_id = ?", "prox", userID).First(&untouched).Error)
		require.True(t, untouched.Enabled)
	}
}

func TestRestoreAdminRiskControlledTokensRestoresOwnedKeysAndReleasesControl(t *testing.T) {
	require.NoError(t, model.DB.AutoMigrate(
		&model.User{},
		&model.Token{},
		&model.RiskUserControl{},
		&model.RiskTokenActivation{},
		&model.OpsAccountRiskAudit{},
		&model.OpsAccountRiskAction{},
	))

	suffix := time.Now().UnixNano()
	user := model.User{
		Username: fmt.Sprintf("risk_restore_%d", suffix), Password: "test-password",
		Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: "default",
		AffCode: fmt.Sprintf("rr_%d", suffix),
	}
	require.NoError(t, model.DB.Create(&user).Error)
	siteID := fmt.Sprintf("risk-restore-%d", suffix)
	reasonCode := "inactive_token_disable"

	restorable := model.Token{
		UserId: user.Id, Key: fmt.Sprintf("sk-restorable-%d", suffix), Status: common.TokenStatusDisabled,
		Name: "risk-owned", Group: "default", ExpiredTime: -1,
	}
	manualDisabled := model.Token{
		UserId: user.Id, Key: fmt.Sprintf("sk-manual-%d", suffix), Status: common.TokenStatusDisabled,
		Name: "manual-disabled", Group: "default", ExpiredTime: -1,
	}
	staleDeleted := model.Token{
		UserId: user.Id, Key: fmt.Sprintf("sk-deleted-%d", suffix), Status: common.TokenStatusDisabled,
		Name: "deleted", Group: "default", ExpiredTime: -1,
	}
	require.NoError(t, model.DB.Create(&restorable).Error)
	require.NoError(t, model.DB.Create(&manualDisabled).Error)
	require.NoError(t, model.DB.Create(&staleDeleted).Error)
	require.NoError(t, model.DB.Delete(&staleDeleted).Error)

	control := model.RiskUserControl{
		SiteId: siteID, UserId: user.Id, RiskLevel: "high", ReasonCode: reasonCode,
		Reason: "test", Enabled: true, KeyRecreateRequired: true, ActivationRequired: true,
		ActivationSource: "qq", ExistingKeysDisabledAt: time.Now().Unix(),
	}
	require.NoError(t, model.DB.Create(&control).Error)
	activations := []model.RiskTokenActivation{
		{SiteId: siteID, UserId: user.Id, TokenId: restorable.Id, ActivationSource: "qq", Status: model.RiskTokenActivationPending, ReasonCode: reasonCode, ExpiresAt: time.Now().Add(time.Minute).Unix()},
		{SiteId: siteID, UserId: user.Id, TokenId: staleDeleted.Id, ActivationSource: "qq", Status: model.RiskTokenActivationPending, ReasonCode: reasonCode, ExpiresAt: time.Now().Add(time.Minute).Unix()},
	}
	require.NoError(t, model.DB.Create(&activations).Error)
	audit := model.OpsAccountRiskAudit{
		SiteId: siteID, RiskType: reasonCode, Severity: "high", Subject: fmt.Sprintf("user:%d", user.Id),
		UserIds: encodeOpsRiskIntList([]int{user.Id}), TokenIds: "[]", Evidence: `{"source":"original"}`, Status: OpsRiskStatusReviewing,
	}
	require.NoError(t, model.DB.Create(&audit).Error)

	t.Cleanup(func() {
		_ = model.DB.Where("audit_id = ?", audit.Id).Delete(&model.OpsAccountRiskAction{}).Error
		_ = model.DB.Where("site_id = ? AND user_id = ?", siteID, user.Id).Delete(&model.RiskTokenActivation{}).Error
		_ = model.DB.Where("site_id = ? AND user_id = ?", siteID, user.Id).Delete(&model.RiskUserControl{}).Error
		_ = model.DB.Where("id = ?", audit.Id).Delete(&model.OpsAccountRiskAudit{}).Error
		_ = model.DB.Unscoped().Where("user_id = ?", user.Id).Delete(&model.Token{}).Error
		_ = model.DB.Unscoped().Where("id = ?", user.Id).Delete(&model.User{}).Error
	})

	states, err := loadAdminRiskRestoreStateMap(siteID, []int{user.Id})
	require.NoError(t, err)
	require.True(t, states[user.Id].ActiveControl)
	require.EqualValues(t, 1, states[user.Id].RestorableTokenCount)

	result, err := RestoreAdminRiskControlledTokens(siteID, user.Id, 77, "review complete")
	require.NoError(t, err)
	require.True(t, result.ActiveControl)
	require.EqualValues(t, 1, result.MatchedTokens)
	require.EqualValues(t, 1, result.RestoredTokens)
	require.EqualValues(t, 1, result.ActivatedRecords)
	require.EqualValues(t, 1, result.CancelledStaleActivations)
	require.EqualValues(t, 1, result.ReleasedControls)
	require.EqualValues(t, 1, result.ClosedAudits)

	var restoredToken model.Token
	require.NoError(t, model.DB.First(&restoredToken, restorable.Id).Error)
	require.Equal(t, common.TokenStatusEnabled, restoredToken.Status)
	var untouchedToken model.Token
	require.NoError(t, model.DB.First(&untouchedToken, manualDisabled.Id).Error)
	require.Equal(t, common.TokenStatusDisabled, untouchedToken.Status)

	require.NoError(t, model.DB.First(&control, control.Id).Error)
	require.False(t, control.Enabled)
	require.False(t, control.KeyRecreateRequired)
	require.False(t, control.ActivationRequired)
	require.Zero(t, control.ExistingKeysDisabledAt)
	require.NoError(t, model.DB.First(&audit, audit.Id).Error)
	require.Equal(t, OpsRiskStatusClosed, audit.Status)
	require.JSONEq(t, `{"source":"original"}`, audit.Evidence)

	var restoredActivation model.RiskTokenActivation
	require.NoError(t, model.DB.First(&restoredActivation, activations[0].Id).Error)
	require.Equal(t, model.RiskTokenActivationActivated, restoredActivation.Status)
	var staleActivation model.RiskTokenActivation
	require.NoError(t, model.DB.First(&staleActivation, activations[1].Id).Error)
	require.Equal(t, model.RiskTokenActivationCancelled, staleActivation.Status)

	var actionCount int64
	require.NoError(t, model.DB.Model(&model.OpsAccountRiskAction{}).Where("audit_id = ? AND operator_id = ?", audit.Id, 77).Count(&actionCount).Error)
	require.EqualValues(t, 2, actionCount)

	second, err := RestoreAdminRiskControlledTokens(siteID, user.Id, 77, "repeat")
	require.NoError(t, err)
	require.False(t, second.ActiveControl)
	require.Zero(t, second.RestoredTokens)
	require.NoError(t, model.DB.Model(&model.OpsAccountRiskAction{}).Where("audit_id = ? AND operator_id = ?", audit.Id, 77).Count(&actionCount).Error)
	require.EqualValues(t, 2, actionCount)
}

func TestMergeAdminUserOpsRestoreStateKeepsPermissionOnlyControlRestorable(t *testing.T) {
	profiles := []map[string]any{{"user_id": 42}}
	mergeAdminUserOpsGridRestoreState(
		profiles,
		map[int]int64{},
		map[int]int64{},
		map[int]adminRiskRestoreState{42: {ActiveControl: true}},
	)
	require.Zero(t, anyInt(profiles[0]["active_frozen_key_count"]))
	require.True(t, toBool(profiles[0]["has_active_risk_control"]))
	require.True(t, toBool(profiles[0]["can_restore"]))
}
