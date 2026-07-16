package model

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/setting/operation_setting"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrBudgetPoolQuotaInsufficient = errors.New("budget pool quota insufficient")
var ErrOpsFundQuotaInsufficient = errors.New("ops fund insufficient")

func currentAgentSiteID() string {
	cfg := operation_setting.GetAgentSetting()
	if cfg == nil {
		return CanonicalSiteID("")
	}
	return CanonicalSiteID(cfg.SiteID)
}

func currentAgentPoolQuota(poolType string) int {
	cfg := operation_setting.GetAgentSetting()
	if cfg == nil {
		return 0
	}
	switch strings.TrimSpace(poolType) {
	case "growth":
		return cfg.GrowthBudgetQuota
	case "activity":
		return cfg.ActivityBudgetQuota
	case "game":
		return cfg.GameBudgetQuota
	case "ops_comp":
		return cfg.OpsCompBudgetQuota
	case "community":
		return cfg.CommunityBudgetQuota
	default:
		return cfg.DailyBudgetQuota
	}
}

func currentAgentBudgetPoolMeta(poolType string) (siteID string, budgetDate string, totalQuota int) {
	poolType = strings.TrimSpace(poolType)
	if poolType == "" {
		poolType = "daily"
	}
	siteID = currentAgentSiteID()
	budgetDate = AgentBusinessDateAt(time.Now())
	totalQuota = currentAgentPoolQuota(poolType)
	return siteID, budgetDate, totalQuota
}

func ensureCurrentAgentDailyBudgetFundingTx(tx *gorm.DB, siteID string, budgetDate string) (*AgentDailyBudgetCapacityResult, error) {
	return EnsureOpsDailyBudgetCapacityForDateTx(
		tx,
		siteID,
		budgetDate,
		AgentDailyBudgetResetRequestID(siteID, budgetDate),
		"automatic daily reward budget reset",
	)
}

func lockAgentBudgetPoolTx(tx *gorm.DB, siteID string, poolType string, budgetDate string) (*AgentBudgetPool, error) {
	var pool AgentBudgetPool
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("site_id = ? AND pool_type = ? AND budget_date = ?", siteID, strings.TrimSpace(poolType), budgetDate).
		First(&pool).Error; err != nil {
		return nil, err
	}
	return &pool, nil
}

func ensureAgentBudgetPoolTx(tx *gorm.DB, siteID string, poolType string, budgetDate string, totalQuota int) (*AgentBudgetPool, error) {
	poolType = strings.TrimSpace(poolType)
	if poolType == "" {
		poolType = "daily"
	}
	pool := AgentBudgetPool{
		SiteId:     strings.TrimSpace(siteID),
		PoolType:   poolType,
		BudgetDate: strings.TrimSpace(budgetDate),
		TotalQuota: totalQuota,
		Status:     "active",
	}
	// Existing daily capacity is changed only by the auditable restore path.
	// Re-applying the configured seed here would erase a same-day reset as soon
	// as the next reward is granted.
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&pool).Error; err != nil {
		return nil, err
	}
	if err := tx.Where("site_id = ? AND pool_type = ? AND budget_date = ?", pool.SiteId, pool.PoolType, pool.BudgetDate).First(&pool).Error; err != nil {
		return nil, err
	}
	return &pool, nil
}

func claimAgentBudgetTransactionTx(tx *gorm.DB, transaction *AgentBudgetTransaction) (bool, error) {
	if tx == nil || transaction == nil {
		return false, errors.New("budget transaction context is nil")
	}
	claim := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(transaction)
	if claim.Error != nil {
		return false, claim.Error
	}
	if claim.RowsAffected == 1 {
		return true, nil
	}
	var existing AgentBudgetTransaction
	if err := tx.Where("idempotency_key = ?", transaction.IdempotencyKey).First(&existing).Error; err != nil {
		return false, err
	}
	if existing.SiteId != transaction.SiteId ||
		existing.PoolId != transaction.PoolId ||
		existing.PoolType != transaction.PoolType ||
		existing.UserId != transaction.UserId ||
		existing.ActionId != transaction.ActionId ||
		existing.SourceType != transaction.SourceType ||
		existing.TransactionType != transaction.TransactionType ||
		existing.Quota != transaction.Quota ||
		existing.SettlementId != transaction.SettlementId ||
		existing.MutationIndex != transaction.MutationIndex {
		return false, fmt.Errorf("%w: idempotency_key=%s", ErrBudgetAdjustmentIdempotencyConflict, transaction.IdempotencyKey)
	}
	return false, nil
}

func GrantQuotaFromBudgetPoolWithSourceTx(tx *gorm.DB, userId int, poolType string, quota int, idempotencyKey string, remark string, sourceType string, metadataJson string) error {
	if tx == nil {
		return errors.New("db transaction is nil")
	}
	if userId <= 0 {
		return errors.New("user id is empty")
	}
	if quota <= 0 {
		return errors.New("quota must be positive")
	}
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if idempotencyKey == "" {
		return errors.New("idempotency key is empty")
	}

	siteID, budgetDate, totalQuota := currentAgentBudgetPoolMeta(poolType)
	if _, err := ensureCurrentAgentDailyBudgetFundingTx(tx, siteID, budgetDate); err != nil {
		return err
	}
	if _, err := ensureAgentBudgetPoolTx(tx, siteID, poolType, budgetDate, totalQuota); err != nil {
		return err
	}

	pool, err := lockAgentBudgetPoolTx(tx, siteID, poolType, budgetDate)
	if err != nil {
		return err
	}

	available := pool.TotalQuota - pool.UsedQuota - pool.FrozenQuota
	fund, err := ensureOpsFundAccountTx(tx, siteID, "operations")
	if err != nil {
		return err
	}
	sourceType = strings.TrimSpace(sourceType)
	if sourceType == "" {
		sourceType = "reward_grant"
	}
	settlementID, mutationIndex, actionID := quotaReplayFieldsFromMetadata(idempotencyKey, metadataJson, 0)

	txn := AgentBudgetTransaction{
		SiteId:          pool.SiteId,
		PoolId:          pool.Id,
		PoolType:        pool.PoolType,
		UserId:          userId,
		ActionId:        actionID,
		SourceType:      sourceType,
		TransactionType: "grant",
		Quota:           quota,
		BalanceAfter:    available - quota,
		SettlementId:    settlementID,
		MutationIndex:   mutationIndex,
		IdempotencyKey:  idempotencyKey,
		Remark:          strings.TrimSpace(remark),
		MetadataJson:    strings.TrimSpace(metadataJson),
	}
	claimed, err := claimAgentBudgetTransactionTx(tx, &txn)
	if err != nil {
		return err
	}
	if !claimed {
		return nil
	}
	if available < quota {
		return fmt.Errorf("%w: available=%d required=%d pool=%s site=%s date=%s",
			ErrBudgetPoolQuotaInsufficient, available, quota, pool.PoolType, pool.SiteId, pool.BudgetDate)
	}
	if fund.BalanceQuota < quota {
		return fmt.Errorf("%w: balance=%d required=%d site=%s pool=%s", ErrOpsFundQuotaInsufficient, fund.BalanceQuota, quota, siteID, strings.TrimSpace(poolType))
	}
	if err := tx.Model(&AgentBudgetPool{}).Where("id = ?", pool.Id).
		Update("used_quota", gorm.Expr("used_quota + ?", quota)).Error; err != nil {
		return err
	}
	if err := ApplyUserQuotaMutationTx(tx, UserQuotaMutation{
		UserID: userId, DeltaQuota: quota, RequestID: idempotencyKey,
		SourceType: sourceType, TransactionType: "budget_grant",
		IdempotencyKey: "user:" + idempotencyKey, Remark: strings.TrimSpace(remark),
	}); err != nil {
		return err
	}
	newBal := fund.BalanceQuota - quota
	if err := tx.Model(&OpsFundAccount{}).Where("id = ?", fund.Id).Update("balance_quota", newBal).Error; err != nil {
		return err
	}
	ledger := OpsFundLedger{
		SiteId:         siteID,
		FundAccountId:  fund.Id,
		DeltaQuota:     -quota,
		BalanceAfter:   newBal,
		SourceType:     sourceType,
		SourcePoolType: strings.TrimSpace(poolType),
		UserId:         userId,
		ActionId:       actionID,
		SettlementId:   settlementID,
		MutationIndex:  mutationIndex,
		IdempotencyKey: idempotencyKey,
		Remark:         strings.TrimSpace(remark),
		MetadataJson:   strings.TrimSpace(metadataJson),
	}
	if res := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&ledger); res.Error != nil {
		return res.Error
	}
	return nil
}

func GrantQuotaFromBudgetPoolTx(tx *gorm.DB, userId int, poolType string, quota int, idempotencyKey string, remark string) error {
	return GrantQuotaFromBudgetPoolWithSourceTx(tx, userId, poolType, quota, idempotencyKey, remark, "reward_grant", "")
}

func ReserveQuotaFromBudgetPoolTx(tx *gorm.DB, userId int, poolType string, quota int, idempotencyKey string, remark string) error {
	return ReserveQuotaFromBudgetPoolWithSourceTx(tx, userId, poolType, quota, idempotencyKey, remark, "invite_inviter_reserve")
}

func ReserveQuotaFromBudgetPoolWithSourceTx(tx *gorm.DB, userId int, poolType string, quota int, idempotencyKey string, remark string, sourceType string) error {
	if tx == nil {
		return errors.New("db transaction is nil")
	}
	if userId <= 0 {
		return errors.New("user id is empty")
	}
	if quota <= 0 {
		return errors.New("quota must be positive")
	}
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if idempotencyKey == "" {
		return errors.New("idempotency key is empty")
	}
	sourceType = strings.TrimSpace(sourceType)
	if sourceType == "" {
		sourceType = "reserve"
	}

	siteID, budgetDate, totalQuota := currentAgentBudgetPoolMeta(poolType)
	if _, err := ensureCurrentAgentDailyBudgetFundingTx(tx, siteID, budgetDate); err != nil {
		return err
	}
	if _, err := ensureAgentBudgetPoolTx(tx, siteID, poolType, budgetDate, totalQuota); err != nil {
		return err
	}

	pool, err := lockAgentBudgetPoolTx(tx, siteID, poolType, budgetDate)
	if err != nil {
		return err
	}
	available := pool.TotalQuota - pool.UsedQuota - pool.FrozenQuota

	settlementID, mutationIndex, actionID := quotaReplayFieldsFromMetadata(idempotencyKey, "", 0)
	txn := AgentBudgetTransaction{
		SiteId:          pool.SiteId,
		PoolId:          pool.Id,
		PoolType:        pool.PoolType,
		UserId:          userId,
		ActionId:        actionID,
		SourceType:      sourceType,
		TransactionType: "reserve",
		Quota:           quota,
		BalanceAfter:    available - quota,
		SettlementId:    settlementID,
		MutationIndex:   mutationIndex,
		IdempotencyKey:  idempotencyKey,
		Remark:          strings.TrimSpace(remark),
	}
	claimed, err := claimAgentBudgetTransactionTx(tx, &txn)
	if err != nil {
		return err
	}
	if !claimed {
		return nil
	}
	if available < quota {
		return fmt.Errorf("%w: available=%d required=%d pool=%s site=%s date=%s",
			ErrBudgetPoolQuotaInsufficient, available, quota, pool.PoolType, pool.SiteId, pool.BudgetDate)
	}
	if err := tx.Model(&AgentBudgetPool{}).Where("id = ?", pool.Id).
		Update("frozen_quota", gorm.Expr("frozen_quota + ?", quota)).Error; err != nil {
		return err
	}
	// reserve 即基金支出(邀请奖励一旦承诺即平台负债;consume_frozen 不再二次动基金)
	fund, ferr := ensureOpsFundAccountTx(tx, siteID, "operations")
	if ferr != nil {
		return ferr
	}
	if fund.BalanceQuota < quota {
		return errors.New("ops fund insufficient for reserve")
	}
	fundBal := fund.BalanceQuota - quota
	if err := tx.Model(&OpsFundAccount{}).Where("id = ?", fund.Id).Update("balance_quota", fundBal).Error; err != nil {
		return err
	}
	fl := OpsFundLedger{SiteId: siteID, FundAccountId: fund.Id, DeltaQuota: -quota, BalanceAfter: fundBal, SourceType: sourceType, SourcePoolType: poolType, UserId: userId, ActionId: actionID, SettlementId: settlementID, MutationIndex: mutationIndex, IdempotencyKey: idempotencyKey + "_fund", Remark: "reserve: " + strings.TrimSpace(remark)}
	if res := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&fl); res.Error != nil {
		return res.Error
	}
	return nil
}

// EnsureUserReservedQuotaTx repairs historical invitation balances that were
// created before per-user budget reservations were recorded. It only reserves
// the missing amount in today's pool and is safe to call before every payout.
func EnsureUserReservedQuotaTx(tx *gorm.DB, userId int, poolType string, quota int, idempotencyKey string, remark string) error {
	if tx == nil {
		return errors.New("db transaction is nil")
	}
	if userId <= 0 || quota <= 0 {
		return errors.New("user id and quota must be positive")
	}
	siteID, budgetDate, totalQuota := currentAgentBudgetPoolMeta(poolType)
	if _, err := ensureAgentBudgetPoolTx(tx, siteID, poolType, budgetDate, totalQuota); err != nil {
		return err
	}
	pool, err := lockAgentBudgetPoolTx(tx, siteID, poolType, budgetDate)
	if err != nil {
		return err
	}

	var reserved int64
	err = tx.Model(&AgentBudgetTransaction{}).
		Where("pool_id = ? AND user_id = ? AND transaction_type IN ?", pool.Id, userId, []string{"reserve", "consume_frozen"}).
		Select("COALESCE(SUM(CASE WHEN transaction_type = 'reserve' THEN quota ELSE -quota END), 0)").
		Scan(&reserved).Error
	if err != nil {
		return err
	}
	missing := quota - int(reserved)
	if missing <= 0 {
		return nil
	}
	return ReserveQuotaFromBudgetPoolWithSourceTx(
		tx,
		userId,
		poolType,
		missing,
		strings.TrimSpace(idempotencyKey),
		strings.TrimSpace(remark),
		"legacy_invite_reserve_repair",
	)
}

func ConsumeReservedQuotaFromBudgetPoolTx(tx *gorm.DB, userId int, poolType string, quota int, idempotencyKey string, remark string) error {
	if tx == nil {
		return errors.New("db transaction is nil")
	}
	if userId <= 0 {
		return errors.New("user id is empty")
	}
	if quota <= 0 {
		return errors.New("quota must be positive")
	}
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if idempotencyKey == "" {
		return errors.New("idempotency key is empty")
	}

	siteID, budgetDate, totalQuota := currentAgentBudgetPoolMeta(poolType)
	if _, err := ensureAgentBudgetPoolTx(tx, siteID, poolType, budgetDate, totalQuota); err != nil {
		return err
	}

	pool, err := lockAgentBudgetPoolTx(tx, siteID, poolType, budgetDate)
	if err != nil {
		return err
	}
	available := pool.TotalQuota - pool.UsedQuota - pool.FrozenQuota

	settlementID, mutationIndex, actionID := quotaReplayFieldsFromMetadata(idempotencyKey, "", 0)
	txn := AgentBudgetTransaction{
		SiteId:          pool.SiteId,
		PoolId:          pool.Id,
		PoolType:        pool.PoolType,
		UserId:          userId,
		ActionId:        actionID,
		SourceType:      "consume_frozen",
		TransactionType: "consume_frozen",
		Quota:           quota,
		BalanceAfter:    available,
		SettlementId:    settlementID,
		MutationIndex:   mutationIndex,
		IdempotencyKey:  idempotencyKey,
		Remark:          strings.TrimSpace(remark),
	}
	claimed, err := claimAgentBudgetTransactionTx(tx, &txn)
	if err != nil {
		return err
	}
	if !claimed {
		return nil
	}
	if pool.FrozenQuota < quota {
		return fmt.Errorf("budget pool reserved quota insufficient: frozen=%d required=%d pool=%s site=%s date=%s",
			pool.FrozenQuota, quota, pool.PoolType, pool.SiteId, pool.BudgetDate)
	}
	poolUpdate := tx.Model(&AgentBudgetPool{}).
		Where("id = ? AND frozen_quota >= ?", pool.Id, quota).
		Updates(map[string]interface{}{
			"used_quota":   gorm.Expr("used_quota + ?", quota),
			"frozen_quota": gorm.Expr("frozen_quota - ?", quota),
		})
	if poolUpdate.Error != nil {
		return poolUpdate.Error
	}
	if poolUpdate.RowsAffected == 0 {
		return fmt.Errorf("budget pool reserved quota insufficient: frozen=%d required=%d pool=%s site=%s date=%s",
			pool.FrozenQuota, quota, pool.PoolType, pool.SiteId, pool.BudgetDate)
	}
	return ApplyUserQuotaMutationTx(tx, UserQuotaMutation{
		UserID: userId, DeltaQuota: quota, RequestID: idempotencyKey,
		SourceType: "consume_frozen", TransactionType: "budget_consume",
		IdempotencyKey: "user:" + idempotencyKey, Remark: strings.TrimSpace(remark),
	})
}
