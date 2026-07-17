package probeutil

import (
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type budgetPoolDelta struct {
	used   int
	frozen int
}

// CleanupLedgerArtifactsTx reverses ledger effects created by a probe and
// removes the corresponding immutable journals. The caller owns the enclosing
// transaction and any probe-specific rows or users.
func CleanupLedgerArtifactsTx(tx *gorm.DB, idempotencyKeys []string) error {
	if tx == nil {
		return errors.New("cleanup transaction is nil")
	}
	keys := compactKeys(idempotencyKeys)
	if len(keys) == 0 {
		return nil
	}

	var budgetRows []model.AgentBudgetTransaction
	if err := tx.Where("idempotency_key IN ?", keys).Find(&budgetRows).Error; err != nil {
		return err
	}
	poolDeltas := map[int]budgetPoolDelta{}
	budgetIDs := make([]int, 0, len(budgetRows))
	for _, row := range budgetRows {
		if row.Quota <= 0 {
			return fmt.Errorf("invalid probe budget transaction quota: id=%d quota=%d", row.Id, row.Quota)
		}
		budgetIDs = append(budgetIDs, row.Id)
		delta := poolDeltas[row.PoolId]
		switch row.TransactionType {
		case "grant":
			delta.used += row.Quota
		case "reserve":
			delta.frozen += row.Quota
		case "consume_frozen":
			delta.used += row.Quota
			delta.frozen -= row.Quota
		default:
			return fmt.Errorf("unsupported probe budget transaction type %q", row.TransactionType)
		}
		poolDeltas[row.PoolId] = delta
	}
	for poolID, delta := range poolDeltas {
		if poolID <= 0 || delta.used < 0 {
			return fmt.Errorf("invalid probe pool delta: pool=%d used=%d frozen=%d", poolID, delta.used, delta.frozen)
		}
		var pool model.AgentBudgetPool
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&pool, poolID).Error; err != nil {
			return fmt.Errorf("lock probe budget pool %d: %w", poolID, err)
		}
		newUsed := pool.UsedQuota - delta.used
		newFrozen := pool.FrozenQuota - delta.frozen
		if newUsed < 0 || newFrozen < 0 {
			return fmt.Errorf(
				"probe budget pool %d would underflow: used=%d-%d frozen=%d-%d",
				poolID, pool.UsedQuota, delta.used, pool.FrozenQuota, delta.frozen,
			)
		}
		result := tx.Model(&model.AgentBudgetPool{}).Where("id = ?", poolID).Updates(map[string]any{
			"used_quota":   newUsed,
			"frozen_quota": newFrozen,
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return fmt.Errorf("probe budget pool %d was not updated", poolID)
		}
	}

	fundKeys := make([]string, 0, len(keys)*2)
	userKeys := make([]string, 0, len(keys))
	for _, key := range keys {
		fundKeys = append(fundKeys, key, key+"_fund")
		userKeys = append(userKeys, "user:"+key)
	}
	var fundRows []model.OpsFundLedger
	if err := tx.Where("idempotency_key IN ?", fundKeys).Find(&fundRows).Error; err != nil {
		return err
	}
	fundDeltas := map[int]int{}
	fundIDs := make([]int, 0, len(fundRows))
	for _, row := range fundRows {
		fundIDs = append(fundIDs, row.Id)
		fundDeltas[row.FundAccountId] += row.DeltaQuota
	}
	for accountID, delta := range fundDeltas {
		if accountID <= 0 {
			return fmt.Errorf("invalid probe fund account id %d", accountID)
		}
		var account model.OpsFundAccount
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&account, accountID).Error; err != nil {
			return fmt.Errorf("lock probe fund account %d: %w", accountID, err)
		}
		newBalance := account.BalanceQuota - delta
		if newBalance < 0 {
			return fmt.Errorf("probe fund account %d would underflow: balance=%d delta=%d", accountID, account.BalanceQuota, delta)
		}
		result := tx.Model(&model.OpsFundAccount{}).Where("id = ?", accountID).
			Update("balance_quota", newBalance)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return fmt.Errorf("probe fund account %d was not found", accountID)
		}
	}
	if len(fundIDs) > 0 {
		if err := tx.Where("id IN ?", fundIDs).Delete(&model.OpsFundLedger{}).Error; err != nil {
			return err
		}
	}
	if err := tx.Where("idempotency_key IN ?", userKeys).Delete(&model.UserQuotaTransaction{}).Error; err != nil {
		return err
	}
	if len(budgetIDs) > 0 {
		if err := tx.Where("id IN ?", budgetIDs).Delete(&model.AgentBudgetTransaction{}).Error; err != nil {
			return err
		}
	}
	return nil
}

func compactKeys(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
