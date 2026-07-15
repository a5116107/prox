package model

import (
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestAgentBusinessDateUsesAsiaShanghaiBoundary(t *testing.T) {
	beforeMidnightUTC := time.Date(2026, 7, 12, 15, 59, 59, 0, time.UTC)
	afterMidnightUTC := time.Date(2026, 7, 12, 16, 0, 0, 0, time.UTC)

	require.Equal(t, "2026-07-12", AgentBusinessDateAt(beforeMidnightUTC))
	require.Equal(t, "2026-07-13", AgentBusinessDateAt(afterMidnightUTC))
}

func TestEnsureAgentBudgetPoolDoesNotReduceRestoredCapacity(t *testing.T) {
	setupRewardLedgerTestDB(t)
	budgetDate := AgentBusinessDateAt(time.Now())
	restored := AgentBudgetPool{
		SiteId: "test-site", PoolType: "activity", BudgetDate: budgetDate,
		TotalQuota: 1_900, UsedQuota: 900, Status: "active",
	}
	require.NoError(t, DB.Create(&restored).Error)

	pool, err := EnsureAgentBudgetPool("test-site", "activity", budgetDate, 1_000)
	require.NoError(t, err)
	require.Equal(t, 1_900, pool.TotalQuota)
	require.Equal(t, 1_000, pool.TotalQuota-pool.UsedQuota-pool.FrozenQuota)
}

func TestSetBudgetPoolAvailableCapacityTxIsAuditableAndIdempotent(t *testing.T) {
	setupRewardLedgerTestDB(t)
	budgetDate := AgentBusinessDateAt(time.Now())
	require.NoError(t, DB.Create(&AgentBudgetPool{
		SiteId: "test-site", PoolType: "activity", BudgetDate: budgetDate,
		TotalQuota: 100, UsedQuota: 20, FrozenQuota: 10, Status: "active",
	}).Error)

	apply := func(poolType string, target int, key string) (*AgentBudgetPool, error) {
		var pool *AgentBudgetPool
		err := DB.Transaction(func(tx *gorm.DB) error {
			var err error
			pool, err = SetBudgetPoolAvailableCapacityTx(
				tx, "test-site", poolType, budgetDate, target, key, "operator limit",
			)
			return err
		})
		return pool, err
	}

	pool, err := apply("activity", 40, "settings-idempotency-1")
	require.NoError(t, err)
	require.Equal(t, 40, pool.TotalQuota-pool.UsedQuota-pool.FrozenQuota)

	pool, err = apply("activity", 40, "settings-idempotency-1")
	require.NoError(t, err)
	require.Equal(t, 40, pool.TotalQuota-pool.UsedQuota-pool.FrozenQuota)

	var adjustments []AgentBudgetTransaction
	require.NoError(t, DB.Where("idempotency_key = ?", "settings-idempotency-1").Find(&adjustments).Error)
	require.Len(t, adjustments, 1)
	require.Equal(t, "capacity_set", adjustments[0].TransactionType)
	require.Equal(t, OpsBudgetRestoreSourceSettings, adjustments[0].SourceType)
	require.Equal(t, -30, adjustments[0].Quota)
	require.Equal(t, 40, adjustments[0].BalanceAfter)

	_, err = apply("activity", 50, "settings-idempotency-1")
	require.ErrorIs(t, err, ErrBudgetAdjustmentIdempotencyConflict)
	_, err = apply("growth", 40, "settings-idempotency-1")
	require.ErrorIs(t, err, ErrBudgetAdjustmentIdempotencyConflict)

	zeroPool, err := apply("growth", 0, "settings-zero-1")
	require.NoError(t, err)
	require.NotNil(t, zeroPool)
	require.Zero(t, zeroPool.TotalQuota-zeroPool.UsedQuota-zeroPool.FrozenQuota)
	_, err = apply("growth", 0, "settings-zero-1")
	require.NoError(t, err)

	var zeroAdjustments []AgentBudgetTransaction
	require.NoError(t, DB.Where("idempotency_key = ?", "settings-zero-1").Find(&zeroAdjustments).Error)
	require.Len(t, zeroAdjustments, 1)
	require.Zero(t, zeroAdjustments[0].Quota)
	require.Zero(t, zeroAdjustments[0].BalanceAfter)
}

func TestSetBudgetPoolAvailableCapacityTxConcurrentReplayMutatesOnce(t *testing.T) {
	setupRewardLedgerTestDB(t)
	budgetDate := AgentBusinessDateAt(time.Now())
	require.NoError(t, DB.Create(&AgentBudgetPool{
		SiteId: "test-site", PoolType: "activity", BudgetDate: budgetDate,
		TotalQuota: 100, UsedQuota: 20, Status: "active",
	}).Error)

	const callers = 12
	errs := make(chan error, callers)
	var wg sync.WaitGroup
	for range callers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- DB.Transaction(func(tx *gorm.DB) error {
				_, err := SetBudgetPoolAvailableCapacityTx(
					tx, "test-site", "activity", budgetDate, 25,
					"settings-concurrent-1", "concurrent operator retry",
				)
				return err
			})
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}

	var pool AgentBudgetPool
	require.NoError(t, DB.Where(
		"site_id = ? AND pool_type = ? AND budget_date = ?",
		"test-site", "activity", budgetDate,
	).First(&pool).Error)
	require.Equal(t, 25, pool.TotalQuota-pool.UsedQuota-pool.FrozenQuota)

	var count int64
	require.NoError(t, DB.Model(&AgentBudgetTransaction{}).
		Where("idempotency_key = ?", "settings-concurrent-1").Count(&count).Error)
	require.Equal(t, int64(1), count)

	var adjustment AgentBudgetTransaction
	require.NoError(t, DB.Where("idempotency_key = ?", "settings-concurrent-1").First(&adjustment).Error)
	require.Equal(t, -55, adjustment.Quota)
	require.Equal(t, 25, adjustment.BalanceAfter)
}

func TestEnsureOpsDailyBudgetCapacityHonorsIndependentResetModes(t *testing.T) {
	t.Run("budget and fund", func(t *testing.T) {
		setupRewardLedgerTestDB(t)
		cfg := operation_setting.GetAgentSetting()
		cfg.SiteID = "test-site"
		cfg.BudgetEnabled = true
		cfg.DailyBudgetResetEnabled = true
		cfg.DailyFundResetEnabled = true
		cfg.DailyBudgetQuota = 900
		cfg.GrowthBudgetQuota = 100
		cfg.ActivityBudgetQuota = 200
		cfg.GameBudgetQuota = 300
		cfg.OpsCompBudgetQuota = 400
		cfg.CommunityBudgetQuota = 500
		cfg.OpsFundDailyTargetQuota = 2_000
		require.NoError(t, DB.Model(&OpsFundAccount{}).
			Where("site_id = ? AND fund_type = ?", "test-site", "operations").
			Update("balance_quota", 10).Error)

		for range 2 {
			result, err := ensureDailyBudgetCapacityForTest("test-site", "2026-07-13")
			require.NoError(t, err)
			require.Len(t, result.RestoredPoolTypes, 6)
			require.Equal(t, 2_000, result.FundMinimumQuota)
		}

		var poolCount int64
		require.NoError(t, DB.Model(&AgentBudgetPool{}).
			Where("site_id = ? AND budget_date = ?", "test-site", "2026-07-13").
			Count(&poolCount).Error)
		require.Equal(t, int64(6), poolCount)

		var fund OpsFundAccount
		require.NoError(t, DB.Where(
			"site_id = ? AND fund_type = ?", "test-site", "operations",
		).First(&fund).Error)
		require.Equal(t, 2_000, fund.BalanceQuota)

		var fundResetCount int64
		require.NoError(t, DB.Model(&OpsFundLedger{}).
			Where("site_id = ? AND source_type = ?", "test-site", OpsBudgetRestoreSourceDailyFund).
			Count(&fundResetCount).Error)
		require.Equal(t, int64(1), fundResetCount)
	})

	t.Run("fund only", func(t *testing.T) {
		setupRewardLedgerTestDB(t)
		cfg := operation_setting.GetAgentSetting()
		cfg.SiteID = "test-site"
		cfg.BudgetEnabled = true
		cfg.DailyBudgetResetEnabled = false
		cfg.DailyFundResetEnabled = true
		cfg.GrowthBudgetQuota = 1_000
		cfg.OpsFundDailyTargetQuota = 2_000
		require.NoError(t, DB.Model(&OpsFundAccount{}).
			Where("site_id = ? AND fund_type = ?", "test-site", "operations").
			Update("balance_quota", 10).Error)

		result, err := ensureDailyBudgetCapacityForTest("test-site", "2026-07-13")
		require.NoError(t, err)
		require.Empty(t, result.RestoredPoolTypes)
		require.Equal(t, 2_000, result.FundMinimumQuota)

		var poolCount int64
		require.NoError(t, DB.Model(&AgentBudgetPool{}).
			Where("site_id = ? AND budget_date = ?", "test-site", "2026-07-13").
			Count(&poolCount).Error)
		require.Zero(t, poolCount)

		var fund OpsFundAccount
		require.NoError(t, DB.Where(
			"site_id = ? AND fund_type = ?", "test-site", "operations",
		).First(&fund).Error)
		require.Equal(t, 2_000, fund.BalanceQuota)
	})

	t.Run("budget only", func(t *testing.T) {
		setupRewardLedgerTestDB(t)
		cfg := operation_setting.GetAgentSetting()
		cfg.SiteID = "test-site"
		cfg.BudgetEnabled = true
		cfg.DailyBudgetResetEnabled = true
		cfg.DailyFundResetEnabled = false
		cfg.DailyBudgetQuota = 0
		cfg.GrowthBudgetQuota = 1_000
		cfg.ActivityBudgetQuota = 0
		cfg.GameBudgetQuota = 0
		cfg.OpsCompBudgetQuota = 0
		cfg.CommunityBudgetQuota = 0
		cfg.OpsFundDailyTargetQuota = 2_000
		require.NoError(t, DB.Model(&OpsFundAccount{}).
			Where("site_id = ? AND fund_type = ?", "test-site", "operations").
			Update("balance_quota", 10).Error)

		result, err := ensureDailyBudgetCapacityForTest("test-site", "2026-07-13")
		require.NoError(t, err)
		require.Equal(t, []string{"growth"}, result.RestoredPoolTypes)
		require.Zero(t, result.FundMinimumQuota)

		var pool AgentBudgetPool
		require.NoError(t, DB.Where(
			"site_id = ? AND pool_type = ? AND budget_date = ?",
			"test-site", "growth", "2026-07-13",
		).First(&pool).Error)
		require.Equal(t, 1_000, pool.TotalQuota)

		var fund OpsFundAccount
		require.NoError(t, DB.Where(
			"site_id = ? AND fund_type = ?", "test-site", "operations",
		).First(&fund).Error)
		require.Equal(t, 10, fund.BalanceQuota)
	})
}

func ensureDailyBudgetCapacityForTest(siteID string, budgetDate string) (*AgentDailyBudgetCapacityResult, error) {
	var result *AgentDailyBudgetCapacityResult
	err := DB.Transaction(func(tx *gorm.DB) error {
		var err error
		result, err = EnsureOpsDailyBudgetCapacityForDateTx(
			tx,
			siteID,
			budgetDate,
			AgentDailyBudgetResetRequestID(siteID, budgetDate),
			"automatic daily budget reset test",
		)
		return err
	})
	return result, err
}
