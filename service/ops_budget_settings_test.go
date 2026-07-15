package service

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/require"
)

func prepareOpsBudgetSettingsTest(t *testing.T, siteID string) {
	t.Helper()
	require.NoError(t, model.DB.AutoMigrate(
		&model.Option{},
		&model.AgentBudgetPool{},
		&model.AgentBudgetTransaction{},
		&model.OpsFundAccount{},
		&model.OpsFundLedger{},
	))
	cleanupOpsRewardFundTestTables(t)
	require.NoError(t, model.DB.Where("key LIKE ?", "agent_setting.%").Delete(&model.Option{}).Error)

	oldAgent := *operation_setting.GetAgentSetting()
	oldOptionMap := common.OptionMap
	common.OptionMap = make(map[string]string)
	t.Cleanup(func() {
		*operation_setting.GetAgentSetting() = oldAgent
		common.OptionMap = oldOptionMap
		_ = model.DB.Where("key LIKE ?", "agent_setting.%").Delete(&model.Option{}).Error
	})

	cfg := operation_setting.GetAgentSetting()
	cfg.SiteID = siteID
	cfg.BudgetEnabled = true
}

func TestUpdateOpsBudgetSettingsRequiresExplicitApplyForToday(t *testing.T) {
	prepareOpsBudgetSettingsTest(t, "settings-site")

	today := model.AgentBusinessDateAt(time.Now())
	require.NoError(t, model.DB.Create(&model.AgentBudgetPool{
		SiteId: "settings-site", PoolType: "activity", BudgetDate: today,
		TotalQuota: 100, UsedQuota: 20, Status: "active",
	}).Error)

	request := OpsBudgetSettingsUpdateRequest{
		DailyBudgetQuota: 500, ActivityBudgetQuota: 500,
		DailyBudgetResetEnabled: true, DailyFundResetEnabled: true,
		OpsFundDailyTargetQuota: 1_000,
	}
	settings, err := UpdateOpsBudgetSettings("settings-site", request)
	require.NoError(t, err)
	require.Equal(t, 500, settings.ActivityBudgetQuota)
	require.Equal(t, 1_000, settings.EffectiveFundTargetQuota)
	require.Equal(t, "Asia/Shanghai", settings.BudgetTimezone)

	var unchanged model.AgentBudgetPool
	require.NoError(t, model.DB.Where(
		"site_id = ? AND pool_type = ? AND budget_date = ?",
		"settings-site", "activity", today,
	).First(&unchanged).Error)
	require.Equal(t, 100, unchanged.TotalQuota)

	var fundCount int64
	require.NoError(t, model.DB.Model(&model.OpsFundAccount{}).
		Where("site_id = ?", "settings-site").Count(&fundCount).Error)
	require.Zero(t, fundCount)

	request.ApplyToToday = true
	request.RequestID = "settings-apply-today-1"
	request.Reason = "operator approved today's new limits"
	_, err = UpdateOpsBudgetSettings("settings-site", request)
	require.NoError(t, err)

	var applied model.AgentBudgetPool
	require.NoError(t, model.DB.Where(
		"site_id = ? AND pool_type = ? AND budget_date = ?",
		"settings-site", "activity", today,
	).First(&applied).Error)
	require.Equal(t, 500, applied.TotalQuota-applied.UsedQuota-applied.FrozenQuota)

	var fund model.OpsFundAccount
	require.NoError(t, model.DB.Where(
		"site_id = ? AND fund_type = ?", "settings-site", "operations",
	).First(&fund).Error)
	require.Equal(t, 1_000, fund.BalanceQuota)
}

func TestUpdateOpsBudgetSettingsSupportsIndependentApplyModes(t *testing.T) {
	t.Run("fund only", func(t *testing.T) {
		prepareOpsBudgetSettingsTest(t, "fund-only-settings-site")

		settings, err := UpdateOpsBudgetSettings("fund-only-settings-site", OpsBudgetSettingsUpdateRequest{
			DailyFundResetEnabled: true, OpsFundDailyTargetQuota: 1_500,
			ApplyToToday: true, RequestID: "settings-fund-only-1",
			Reason: "operator approved today's fund target",
		})
		require.NoError(t, err)
		require.Equal(t, 1_500, settings.EffectiveFundTargetQuota)

		var poolCount int64
		require.NoError(t, model.DB.Model(&model.AgentBudgetPool{}).
			Where("site_id = ?", "fund-only-settings-site").Count(&poolCount).Error)
		require.Zero(t, poolCount)

		var fund model.OpsFundAccount
		require.NoError(t, model.DB.Where(
			"site_id = ? AND fund_type = ?", "fund-only-settings-site", "operations",
		).First(&fund).Error)
		require.Equal(t, 1_500, fund.BalanceQuota)
	})

	t.Run("budget only", func(t *testing.T) {
		prepareOpsBudgetSettingsTest(t, "budget-only-settings-site")
		require.NoError(t, model.DB.Create(&model.OpsFundAccount{
			SiteId: "budget-only-settings-site", FundType: "operations",
			BalanceQuota: 100, Status: "active",
		}).Error)

		_, err := UpdateOpsBudgetSettings("budget-only-settings-site", OpsBudgetSettingsUpdateRequest{
			DailyBudgetQuota: 200, ActivityBudgetQuota: 500,
			DailyBudgetResetEnabled: true, OpsFundDailyTargetQuota: 1_500,
			ApplyToToday: true, RequestID: "settings-budget-only-1",
			Reason: "operator approved today's budget targets",
		})
		require.NoError(t, err)

		today := model.AgentBusinessDateAt(time.Now())
		for poolType, expectedAvailable := range map[string]int{"daily": 200, "activity": 500} {
			var pool model.AgentBudgetPool
			require.NoError(t, model.DB.Where(
				"site_id = ? AND pool_type = ? AND budget_date = ?",
				"budget-only-settings-site", poolType, today,
			).First(&pool).Error)
			require.Equal(t, expectedAvailable, pool.TotalQuota-pool.UsedQuota-pool.FrozenQuota)
		}

		var fund model.OpsFundAccount
		require.NoError(t, model.DB.Where(
			"site_id = ? AND fund_type = ?", "budget-only-settings-site", "operations",
		).First(&fund).Error)
		require.Equal(t, 100, fund.BalanceQuota)
	})

	t.Run("budget settings can reduce or disable today's remaining capacity", func(t *testing.T) {
		prepareOpsBudgetSettingsTest(t, "budget-reduce-settings-site")
		today := model.AgentBusinessDateAt(time.Now())
		require.NoError(t, model.DB.Create(&model.AgentBudgetPool{
			SiteId: "budget-reduce-settings-site", PoolType: "activity", BudgetDate: today,
			TotalQuota: 100, UsedQuota: 20, Status: "active",
		}).Error)
		require.NoError(t, model.DB.Create(&model.AgentBudgetPool{
			SiteId: "budget-reduce-settings-site", PoolType: "growth", BudgetDate: today,
			TotalQuota: 100, UsedQuota: 10, Status: "active",
		}).Error)

		request := OpsBudgetSettingsUpdateRequest{
			ActivityBudgetQuota:     40,
			DailyBudgetResetEnabled: true,
			ApplyToToday:            true, RequestID: "settings-budget-reduce-1",
			Reason: "operator lowered today's remaining limits",
		}
		_, err := UpdateOpsBudgetSettings("budget-reduce-settings-site", request)
		require.NoError(t, err)

		for poolType, expectedAvailable := range map[string]int{"activity": 40, "growth": 0} {
			var pool model.AgentBudgetPool
			require.NoError(t, model.DB.Where(
				"site_id = ? AND pool_type = ? AND budget_date = ?",
				"budget-reduce-settings-site", poolType, today,
			).First(&pool).Error)
			require.Equal(t, expectedAvailable, pool.TotalQuota-pool.UsedQuota-pool.FrozenQuota)
		}

		var adjustments []model.AgentBudgetTransaction
		require.NoError(t, model.DB.Where(
			"site_id = ? AND source_type = ?",
			"budget-reduce-settings-site", model.OpsBudgetRestoreSourceSettings,
		).Order("pool_type asc").Find(&adjustments).Error)
		require.Len(t, adjustments, 6)
		quotaByPool := make(map[string]int, len(adjustments))
		for _, adjustment := range adjustments {
			quotaByPool[adjustment.PoolType] = adjustment.Quota
			require.Equal(t, "capacity_set", adjustment.TransactionType)
		}
		require.Equal(t, map[string]int{
			"daily": 0, "growth": -90, "activity": -40,
			"game": 0, "ops_comp": 0, "community": 0,
		}, quotaByPool)

		_, err = UpdateOpsBudgetSettings("budget-reduce-settings-site", request)
		require.NoError(t, err)
		require.NoError(t, model.DB.Where(
			"site_id = ? AND source_type = ?",
			"budget-reduce-settings-site", model.OpsBudgetRestoreSourceSettings,
		).Find(&adjustments).Error)
		require.Len(t, adjustments, 6)

		conflict := request
		conflict.ActivityBudgetQuota = 60
		_, err = UpdateOpsBudgetSettings("budget-reduce-settings-site", conflict)
		require.ErrorIs(t, err, model.ErrBudgetAdjustmentIdempotencyConflict)
		require.Equal(t, 40, operation_setting.GetAgentSetting().ActivityBudgetQuota)
		var applied model.AgentBudgetPool
		require.NoError(t, model.DB.Where(
			"site_id = ? AND pool_type = ? AND budget_date = ?",
			"budget-reduce-settings-site", "activity", today,
		).First(&applied).Error)
		require.Equal(t, 40, applied.TotalQuota-applied.UsedQuota-applied.FrozenQuota)
	})
}

func TestOpsBudgetSettingsRejectInvalidOrCrossSiteWrites(t *testing.T) {
	oldAgent := *operation_setting.GetAgentSetting()
	t.Cleanup(func() { *operation_setting.GetAgentSetting() = oldAgent })
	operation_setting.GetAgentSetting().SiteID = "prox"

	_, err := UpdateOpsBudgetSettings("prox", OpsBudgetSettingsUpdateRequest{
		ActivityBudgetQuota: -1,
	})
	require.ErrorContains(t, err, "cannot be negative")

	_, err = UpdateOpsBudgetSettings("prox", OpsBudgetSettingsUpdateRequest{
		ApplyToToday: true, RequestID: "disabled-reset-1", Reason: "must not apply",
	})
	require.ErrorContains(t, err, "at least one daily reset must be enabled")

	_, err = UpdateOpsBudgetSettings("other-site", OpsBudgetSettingsUpdateRequest{})
	require.ErrorContains(t, err, "does not match this deployment")

	_, err = RestoreOpsBudgetCapacity("other-site", OpsBudgetCapacityRestoreRequest{
		PoolTypes: []string{"activity"}, RequestID: "cross-site-restore-1", Reason: "must stay local",
	})
	require.ErrorContains(t, err, "does not match this deployment")
}

func TestUpdateOpsBudgetSettingsRollsBackOptionsWithTodayMutation(t *testing.T) {
	prepareOpsBudgetSettingsTest(t, "settings-rollback-site")

	initial := OpsBudgetSettingsUpdateRequest{
		DailyBudgetQuota: 500, ActivityBudgetQuota: 500,
		DailyBudgetResetEnabled: true, DailyFundResetEnabled: true,
		OpsFundDailyTargetQuota: 1_000,
	}
	_, err := UpdateOpsBudgetSettings("settings-rollback-site", initial)
	require.NoError(t, err)
	require.Equal(t, 500, operation_setting.GetAgentSetting().ActivityBudgetQuota)

	today := model.AgentBusinessDateAt(time.Now())
	require.NoError(t, model.DB.Create(&model.AgentBudgetPool{
		SiteId: "settings-rollback-site", PoolType: "activity", BudgetDate: today,
		TotalQuota: 100, UsedQuota: 20, Status: "active",
	}).Error)
	require.NoError(t, model.DB.Migrator().DropTable(&model.OpsFundLedger{}))
	t.Cleanup(func() { _ = model.DB.AutoMigrate(&model.OpsFundLedger{}) })

	failed := initial
	failed.ActivityBudgetQuota = 800
	failed.OpsFundDailyTargetQuota = 1_200
	failed.ApplyToToday = true
	failed.RequestID = "settings-rollback-1"
	failed.Reason = "force transaction rollback"
	_, err = UpdateOpsBudgetSettings("settings-rollback-site", failed)
	require.Error(t, err)

	var stored model.Option
	require.NoError(t, model.DB.Where(
		"key = ?", "agent_setting.activity_budget_quota",
	).First(&stored).Error)
	require.Equal(t, "500", stored.Value)
	require.Equal(t, 500, operation_setting.GetAgentSetting().ActivityBudgetQuota)

	var pool model.AgentBudgetPool
	require.NoError(t, model.DB.Where(
		"site_id = ? AND pool_type = ? AND budget_date = ?",
		"settings-rollback-site", "activity", today,
	).First(&pool).Error)
	require.Equal(t, 100, pool.TotalQuota)
}
