package probeutil

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestCleanupLedgerArtifactsTxReversesProbeEffects(t *testing.T) {
	db := openCleanupTestDB(t, "reverses-effects")

	pool := model.AgentBudgetPool{SiteId: "prox", PoolType: "growth", BudgetDate: "2026-07-18", TotalQuota: 100, UsedQuota: 13, FrozenQuota: 3}
	require.NoError(t, db.Create(&pool).Error)
	fund := model.OpsFundAccount{SiteId: "prox", FundType: "operations", BalanceQuota: 97, Status: "active"}
	require.NoError(t, db.Create(&fund).Error)

	keys := []string{"grant-key", "reserve-key", "consume-key"}
	require.NoError(t, db.Create(&[]model.AgentBudgetTransaction{
		{SiteId: "prox", PoolId: pool.Id, PoolType: "growth", TransactionType: "grant", Quota: 2, IdempotencyKey: keys[0]},
		{SiteId: "prox", PoolId: pool.Id, PoolType: "growth", TransactionType: "reserve", Quota: 1, IdempotencyKey: keys[1]},
		{SiteId: "prox", PoolId: pool.Id, PoolType: "growth", TransactionType: "consume_frozen", Quota: 1, IdempotencyKey: keys[2]},
	}).Error)
	require.NoError(t, db.Create(&[]model.OpsFundLedger{
		{SiteId: "prox", FundAccountId: fund.Id, DeltaQuota: -2, IdempotencyKey: keys[0]},
		{SiteId: "prox", FundAccountId: fund.Id, DeltaQuota: -1, IdempotencyKey: keys[1] + "_fund"},
	}).Error)
	require.NoError(t, db.Create(&[]model.UserQuotaTransaction{
		{UserId: 11, RequestId: keys[0], SourceType: "probe", TransactionType: "budget_grant", DeltaQuota: 2, IdempotencyKey: "user:" + keys[0]},
		{UserId: 11, RequestId: keys[2], SourceType: "probe", TransactionType: "budget_consume", DeltaQuota: 1, IdempotencyKey: "user:" + keys[2]},
	}).Error)

	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		return CleanupLedgerArtifactsTx(tx, keys)
	}))
	require.NoError(t, db.First(&pool, pool.Id).Error)
	require.Equal(t, 10, pool.UsedQuota)
	require.Equal(t, 3, pool.FrozenQuota)
	require.NoError(t, db.First(&fund, fund.Id).Error)
	require.Equal(t, 100, fund.BalanceQuota)

	var budgetCount, fundCount, userCount int64
	require.NoError(t, db.Model(&model.AgentBudgetTransaction{}).Count(&budgetCount).Error)
	require.NoError(t, db.Model(&model.OpsFundLedger{}).Count(&fundCount).Error)
	require.NoError(t, db.Model(&model.UserQuotaTransaction{}).Count(&userCount).Error)
	require.Zero(t, budgetCount)
	require.Zero(t, fundCount)
	require.Zero(t, userCount)
}

func TestCleanupLedgerArtifactsTxRestoresConsumedFrozenQuota(t *testing.T) {
	db := openCleanupTestDB(t, "restores-consumed-frozen")
	pool := model.AgentBudgetPool{SiteId: "prox", PoolType: "growth", BudgetDate: "2026-07-18", TotalQuota: 100, UsedQuota: 5, FrozenQuota: 2}
	require.NoError(t, db.Create(&pool).Error)
	require.NoError(t, db.Create(&model.AgentBudgetTransaction{
		SiteId: "prox", PoolId: pool.Id, PoolType: "growth", TransactionType: "consume_frozen", Quota: 3, IdempotencyKey: "consume-only",
	}).Error)

	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		return CleanupLedgerArtifactsTx(tx, []string{"consume-only"})
	}))
	require.NoError(t, db.First(&pool, pool.Id).Error)
	require.Equal(t, 2, pool.UsedQuota)
	require.Equal(t, 5, pool.FrozenQuota)
}

func TestCleanupLedgerArtifactsTxRollsBackOnPoolUnderflow(t *testing.T) {
	db := openCleanupTestDB(t, "rejects-underflow")
	pool := model.AgentBudgetPool{SiteId: "prox", PoolType: "growth", BudgetDate: "2026-07-18", TotalQuota: 100, UsedQuota: 1}
	require.NoError(t, db.Create(&pool).Error)
	txn := model.AgentBudgetTransaction{SiteId: "prox", PoolId: pool.Id, PoolType: "growth", TransactionType: "grant", Quota: 2, IdempotencyKey: "underflow"}
	require.NoError(t, db.Create(&txn).Error)

	err := db.Transaction(func(tx *gorm.DB) error {
		return CleanupLedgerArtifactsTx(tx, []string{"underflow"})
	})
	require.ErrorContains(t, err, "would underflow")
	require.NoError(t, db.First(&pool, pool.Id).Error)
	require.Equal(t, 1, pool.UsedQuota)
	require.NoError(t, db.First(&txn, txn.Id).Error)
}

func TestCleanupLedgerArtifactsTxRollsBackOnUnknownTransactionType(t *testing.T) {
	db := openCleanupTestDB(t, "rejects-unknown-type")
	pool := model.AgentBudgetPool{SiteId: "prox", PoolType: "growth", BudgetDate: "2026-07-18", TotalQuota: 100}
	require.NoError(t, db.Create(&pool).Error)
	txn := model.AgentBudgetTransaction{SiteId: "prox", PoolId: pool.Id, PoolType: "growth", TransactionType: "mystery", Quota: 1, IdempotencyKey: "unknown"}
	require.NoError(t, db.Create(&txn).Error)

	err := db.Transaction(func(tx *gorm.DB) error {
		return CleanupLedgerArtifactsTx(tx, []string{"unknown"})
	})
	require.ErrorContains(t, err, "unsupported probe budget transaction type")
	require.NoError(t, db.First(&txn, txn.Id).Error)
}

func openCleanupTestDB(t *testing.T, name string) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:probe-cleanup-"+name+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.AgentBudgetPool{},
		&model.AgentBudgetTransaction{},
		&model.OpsFundAccount{},
		&model.OpsFundLedger{},
		&model.UserQuotaTransaction{},
	))
	return db
}
