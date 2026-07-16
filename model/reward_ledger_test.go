package model

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupRewardLedgerTestDB(t *testing.T) {
	t.Helper()
	oldDB, oldLogDB := DB, LOG_DB
	oldSQLite := common.UsingSQLite
	oldRedis := common.RedisEnabled
	oldBatch := common.BatchUpdateEnabled
	oldAgent := *operation_setting.GetAgentSetting()
	oldCheckin := *operation_setting.GetCheckinSetting()
	oldPayment := *operation_setting.GetPaymentSetting()
	oldNewUserQuota := common.QuotaForNewUser
	oldInviteeQuota := common.QuotaForInvitee
	oldInviterQuota := common.QuotaForInviter

	dsn := fmt.Sprintf("file:reward_%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	DB = db
	LOG_DB = db
	common.UsingSQLite = true
	common.RedisEnabled = false
	common.BatchUpdateEnabled = false
	initCol()
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(
		&User{},
		&AgentBudgetPool{},
		&AgentBudgetTransaction{},
		&UserQuotaTransaction{},
		&OpsFundAccount{},
		&OpsFundLedger{},
		&CommunityBotReward{},
		&Checkin{},
		&Log{},
	))
	require.NoError(t, db.Create(&OpsFundAccount{
		SiteId: "test-site", FundType: "operations", BalanceQuota: 10_000_000, Status: "active",
	}).Error)
	t.Cleanup(func() {
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
		DB, LOG_DB = oldDB, oldLogDB
		common.UsingSQLite = oldSQLite
		common.RedisEnabled = oldRedis
		common.BatchUpdateEnabled = oldBatch
		common.QuotaForNewUser = oldNewUserQuota
		common.QuotaForInvitee = oldInviteeQuota
		common.QuotaForInviter = oldInviterQuota
		*operation_setting.GetAgentSetting() = oldAgent
		*operation_setting.GetCheckinSetting() = oldCheckin
		*operation_setting.GetPaymentSetting() = oldPayment
	})
}

func TestGrantCommunityBotRewardIfNeededUsesCommunityBudgetPool(t *testing.T) {
	setupRewardLedgerTestDB(t)
	operation_setting.GetAgentSetting().SiteID = "test-site"
	operation_setting.GetAgentSetting().CommunityBudgetQuota = 1000000

	user := User{
		Username:    "ledger_user",
		Password:    "Password123!",
		DisplayName: "ledger",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		Quota:       0,
		Group:       "default",
		AffCode:     "aff-ledger-1",
	}
	require.NoError(t, DB.Create(&user).Error)

	granted, err := GrantCommunityBotRewardIfNeeded(user.Id, 1, "provider-user-1", "room-1", "daily_message", "2026-06-21", 500000, 6)
	require.NoError(t, err)
	require.True(t, granted)

	var reward CommunityBotReward
	require.NoError(t, DB.First(&reward).Error)
	require.Equal(t, 500000, reward.Quota)

	var pool AgentBudgetPool
	require.NoError(t, DB.Where("site_id = ? AND pool_type = ? AND budget_date = ?", "test-site", "community", AgentBusinessDateAt(time.Now())).First(&pool).Error)
	require.Equal(t, 500000, pool.UsedQuota)

	var txn AgentBudgetTransaction
	require.NoError(t, DB.Where("idempotency_key = ?", "community-reward:"+itoa(user.Id)+":room-1:daily_message:2026-06-21").First(&txn).Error)
	require.Equal(t, 500000, txn.Quota)

	var fresh User
	require.NoError(t, DB.First(&fresh, user.Id).Error)
	require.Equal(t, 500000, fresh.Quota)
}

func TestBudgetGrantReplaySucceedsAfterCapacityIsConsumedAndRejectsConflict(t *testing.T) {
	setupRewardLedgerTestDB(t)
	operation_setting.GetAgentSetting().SiteID = "test-site"
	operation_setting.GetAgentSetting().ActivityBudgetQuota = 10
	user := User{
		Username: "budget-replay", Password: "Password123!", DisplayName: "budget replay",
		Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: "default", AffCode: "aff-budget-replay",
	}
	require.NoError(t, DB.Create(&user).Error)

	grant := func(quota int) error {
		return DB.Transaction(func(tx *gorm.DB) error {
			return GrantQuotaFromBudgetPoolWithSourceTx(tx, user.Id, "activity", quota, "budget-replay-key", "replay", "test_reward", "")
		})
	}
	require.NoError(t, grant(10))
	require.NoError(t, grant(10))
	require.ErrorIs(t, grant(9), ErrBudgetAdjustmentIdempotencyConflict)

	var fresh User
	require.NoError(t, DB.First(&fresh, user.Id).Error)
	require.Equal(t, 10, fresh.Quota)
	var walletRows int64
	require.NoError(t, DB.Model(&UserQuotaTransaction{}).Where("idempotency_key = ?", "user:budget-replay-key").Count(&walletRows).Error)
	require.Equal(t, int64(1), walletRows)
}

func TestUserCheckinByChannelUsesActivityBudgetPool(t *testing.T) {
	setupRewardLedgerTestDB(t)
	operation_setting.GetAgentSetting().SiteID = "test-site"
	operation_setting.GetAgentSetting().ActivityBudgetQuota = 1000000

	checkinCfg := operation_setting.GetCheckinSetting()
	checkinCfg.Enabled = true
	checkinCfg.MinQuota = 500000
	checkinCfg.MaxQuota = 500000
	checkinCfg.Channels = map[string]operation_setting.CheckinChannelSetting{
		CheckinChannelCommunity: {DailyBudget: 1000000},
	}

	user := User{
		Username:    "checkin_user",
		Password:    "Password123!",
		DisplayName: "checkin",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		Quota:       0,
		Group:       "default",
		AffCode:     "aff-ledger-2",
	}
	require.NoError(t, DB.Create(&user).Error)

	checkin, err := UserCheckinByChannel(user.Id, CheckinChannelCommunity)
	require.NoError(t, err)
	require.Equal(t, 500000, checkin.QuotaAwarded)

	var pool AgentBudgetPool
	require.NoError(t, DB.Where("site_id = ? AND pool_type = ? AND budget_date = ?", "test-site", "activity", AgentBusinessDateAt(time.Now())).First(&pool).Error)
	require.Equal(t, 500000, pool.UsedQuota)

	var txn AgentBudgetTransaction
	require.NoError(t, DB.Where("idempotency_key = ?", "checkin:community:"+itoa(user.Id)+":"+checkin.CheckinDate).First(&txn).Error)
	require.Equal(t, 500000, txn.Quota)

	var fresh User
	require.NoError(t, DB.First(&fresh, user.Id).Error)
	require.Equal(t, 500000, fresh.Quota)
}

func TestUserCheckinByChannelUsesBackendSelectedBudgetPool(t *testing.T) {
	setupRewardLedgerTestDB(t)
	agent := operation_setting.GetAgentSetting()
	agent.SiteID = "test-site"
	agent.CommunityBudgetQuota = 1_000_000

	checkinCfg := operation_setting.GetCheckinSetting()
	checkinCfg.Enabled = true
	checkinCfg.MinQuota = 250_000
	checkinCfg.MaxQuota = 250_000

	user := User{
		Username: "checkin_custom_pool", Password: "Password123!", DisplayName: "checkin custom pool",
		Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: "default", AffCode: "aff-checkin-custom-pool",
	}
	require.NoError(t, DB.Create(&user).Error)

	checkin, err := UserCheckinByChannelWithQuotaFromPool(user.Id, CheckinChannelQQ, 250_000, "community")
	require.NoError(t, err)
	var pool AgentBudgetPool
	require.NoError(t, DB.Where("site_id = ? AND pool_type = ? AND budget_date = ?", "test-site", "community", checkin.CheckinDate).First(&pool).Error)
	require.Equal(t, 250_000, pool.UsedQuota)
	var activityPool AgentBudgetPool
	require.NoError(t, DB.Where("site_id = ? AND pool_type = ?", "test-site", "activity").First(&activityPool).Error)
	require.Equal(t, 0, activityPool.UsedQuota)
}

func TestUserCheckinAutoFundsConfiguredDailyBudgetOnce(t *testing.T) {
	setupRewardLedgerTestDB(t)
	agentCfg := operation_setting.GetAgentSetting()
	agentCfg.SiteID = "test-site"
	agentCfg.DailyBudgetQuota = 0
	agentCfg.GrowthBudgetQuota = 0
	agentCfg.ActivityBudgetQuota = 1_000_000
	agentCfg.GameBudgetQuota = 0
	agentCfg.OpsCompBudgetQuota = 0
	agentCfg.CommunityBudgetQuota = 0

	checkinCfg := operation_setting.GetCheckinSetting()
	checkinCfg.Enabled = true
	checkinCfg.MinQuota = 500_000
	checkinCfg.MaxQuota = 500_000
	checkinCfg.Channels = map[string]operation_setting.CheckinChannelSetting{
		CheckinChannelQQ: {DailyBudget: 1_000_000},
	}

	require.NoError(t, DB.Model(&OpsFundAccount{}).
		Where("site_id = ? AND fund_type = ?", "test-site", "operations").
		Update("balance_quota", 811).Error)
	user := User{
		Username: "daily_budget_user", Password: "Password123!", DisplayName: "daily budget",
		Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: "default", AffCode: "aff-daily-budget",
	}
	require.NoError(t, DB.Create(&user).Error)

	checkin, err := UserCheckinByChannel(user.Id, CheckinChannelQQ)
	require.NoError(t, err)
	require.Equal(t, 500_000, checkin.QuotaAwarded)

	var fund OpsFundAccount
	require.NoError(t, DB.Where("site_id = ? AND fund_type = ?", "test-site", "operations").First(&fund).Error)
	require.Equal(t, 500_000, fund.BalanceQuota)

	var resetLedger OpsFundLedger
	require.NoError(t, DB.Where("source_type = ?", OpsBudgetRestoreSourceDailyFund).First(&resetLedger).Error)
	require.Equal(t, 999_189, resetLedger.DeltaQuota)

	_, budgetDate, _ := currentAgentBudgetPoolMeta("activity")
	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		_, err := ensureCurrentAgentDailyBudgetFundingTx(tx, "test-site", budgetDate)
		return err
	}))
	require.NoError(t, DB.Where("site_id = ? AND fund_type = ?", "test-site", "operations").First(&fund).Error)
	require.Equal(t, 500_000, fund.BalanceQuota)

	var resetCount int64
	require.NoError(t, DB.Model(&OpsFundLedger{}).Where("source_type = ?", OpsBudgetRestoreSourceDailyFund).Count(&resetCount).Error)
	require.Equal(t, int64(1), resetCount)
}

func TestGrantAfterDailyResetPreservesRestoredPoolCapacity(t *testing.T) {
	setupRewardLedgerTestDB(t)
	agentCfg := operation_setting.GetAgentSetting()
	agentCfg.SiteID = "test-site"
	agentCfg.DailyBudgetQuota = 0
	agentCfg.GrowthBudgetQuota = 0
	agentCfg.ActivityBudgetQuota = 1_000_000
	agentCfg.GameBudgetQuota = 0
	agentCfg.OpsCompBudgetQuota = 0
	agentCfg.CommunityBudgetQuota = 0

	user := User{
		Username: "post_reset_user", Password: "Password123!", DisplayName: "post reset",
		Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: "default", AffCode: "aff-post-reset",
	}
	require.NoError(t, DB.Create(&user).Error)

	budgetDate := AgentBusinessDateAt(time.Now())
	require.NoError(t, DB.Create(&AgentBudgetPool{
		SiteId: "test-site", PoolType: "activity", BudgetDate: budgetDate,
		TotalQuota: 1_000_000, UsedQuota: 900_000, Status: "active",
	}).Error)
	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		_, err := ensureCurrentAgentDailyBudgetFundingTx(tx, "test-site", budgetDate)
		return err
	}))

	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		return GrantQuotaFromBudgetPoolWithSourceTx(
			tx, user.Id, "activity", 100_000, "post-reset-grant", "grant after daily reset", "quiz_reward", "",
		)
	}))

	var pool AgentBudgetPool
	require.NoError(t, DB.Where(
		"site_id = ? AND pool_type = ? AND budget_date = ?", "test-site", "activity", budgetDate,
	).First(&pool).Error)
	require.Equal(t, 1_900_000, pool.TotalQuota)
	require.Equal(t, 1_000_000, pool.UsedQuota)
	require.Equal(t, 900_000, pool.TotalQuota-pool.UsedQuota-pool.FrozenQuota)
}

func TestInsertUsesGrowthBudgetPoolForSignupAndInvite(t *testing.T) {
	setupRewardLedgerTestDB(t)
	operation_setting.GetAgentSetting().SiteID = "test-site"
	operation_setting.GetAgentSetting().GrowthBudgetQuota = 3000000

	paymentCfg := operation_setting.GetPaymentSetting()
	paymentCfg.ComplianceConfirmed = true
	paymentCfg.ComplianceTermsVersion = operation_setting.CurrentComplianceTermsVersion

	common.QuotaForNewUser = 500000
	common.QuotaForInvitee = 300000
	common.QuotaForInviter = 700000
	defer func() {
		common.QuotaForNewUser = 0
		common.QuotaForInvitee = 0
		common.QuotaForInviter = 0
		paymentCfg.ComplianceConfirmed = false
		paymentCfg.ComplianceTermsVersion = ""
	}()

	inviter := User{
		Username:    "inviter_user",
		Password:    "Password123!",
		DisplayName: "inviter",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		Quota:       0,
		Group:       "default",
		AffCode:     "AFF1",
	}
	require.NoError(t, DB.Create(&inviter).Error)

	newUser := User{
		Username:    "new_user",
		Password:    "Password123!",
		DisplayName: "new user",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		Group:       "default",
	}
	require.NoError(t, newUser.Insert(inviter.Id))

	var pool AgentBudgetPool
	require.NoError(t, DB.Where("site_id = ? AND pool_type = ? AND budget_date = ?", "test-site", "growth", AgentBusinessDateAt(time.Now())).First(&pool).Error)
	require.Equal(t, 800000, pool.UsedQuota)
	require.Equal(t, 700000, pool.FrozenQuota)

	var freshUser User
	require.NoError(t, DB.Where("id = ?", newUser.Id).First(&freshUser).Error)
	require.Equal(t, 800000, freshUser.Quota)
	require.Equal(t, inviter.Id, freshUser.InviterId)

	var freshInviter User
	require.NoError(t, DB.Where("id = ?", inviter.Id).First(&freshInviter).Error)
	require.Equal(t, 1, freshInviter.AffCount)
	require.Equal(t, 700000, freshInviter.AffQuota)
	require.Equal(t, 700000, freshInviter.AffHistoryQuota)

	var grantCount int64
	require.NoError(t, DB.Model(&AgentBudgetTransaction{}).Where("pool_type = ? AND transaction_type = ?", "growth", "grant").Count(&grantCount).Error)
	require.Equal(t, int64(2), grantCount)
	var reserveCount int64
	require.NoError(t, DB.Model(&AgentBudgetTransaction{}).Where("pool_type = ? AND transaction_type = ?", "growth", "reserve").Count(&reserveCount).Error)
	require.Equal(t, int64(1), reserveCount)
}

func TestTransferAffQuotaConsumesReservedGrowthBudget(t *testing.T) {
	setupRewardLedgerTestDB(t)
	operation_setting.GetAgentSetting().SiteID = "test-site"
	operation_setting.GetAgentSetting().GrowthBudgetQuota = 2000000

	user := User{
		Username:        "aff_user",
		Password:        "Password123!",
		DisplayName:     "aff",
		Role:            common.RoleCommonUser,
		Status:          common.UserStatusEnabled,
		Quota:           0,
		Group:           "default",
		AffCode:         "AFF2",
		AffQuota:        600000,
		AffHistoryQuota: 600000,
	}
	require.NoError(t, DB.Create(&user).Error)

	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		return ReserveQuotaFromBudgetPoolTx(tx, user.Id, "growth", 600000, "seed-reserve:aff-user", "seed inviter reserve")
	}))

	require.NoError(t, user.TransferAffQuotaToQuota(600000))

	var pool AgentBudgetPool
	require.NoError(t, DB.Where("site_id = ? AND pool_type = ? AND budget_date = ?", "test-site", "growth", AgentBusinessDateAt(time.Now())).First(&pool).Error)
	require.Equal(t, 600000, pool.UsedQuota)
	require.Equal(t, 0, pool.FrozenQuota)

	var fresh User
	require.NoError(t, DB.Where("id = ?", user.Id).First(&fresh).Error)
	require.Equal(t, 0, fresh.AffQuota)
	require.Equal(t, 600000, fresh.Quota)

	var reserveCount int64
	require.NoError(t, DB.Model(&AgentBudgetTransaction{}).Where("transaction_type = ?", "reserve").Count(&reserveCount).Error)
	require.Equal(t, int64(1), reserveCount)
	var consumeCount int64
	require.NoError(t, DB.Model(&AgentBudgetTransaction{}).Where("transaction_type = ?", "consume_frozen").Count(&consumeCount).Error)
	require.Equal(t, int64(1), consumeCount)
}

func itoa(v int) string {
	return fmt.Sprintf("%d", v)
}
