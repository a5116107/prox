package model

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func RefundExpiredUserQuotaReservations(now time.Time, limit int) (int, error) {
	if limit <= 0 {
		limit = 100
	}
	var rows []UserQuotaReservation
	if err := DB.Select("request_id", "user_id").
		Where("status = ? AND expires_at > 0 AND expires_at <= ?", UserQuotaReservationActive, now.Unix()).
		Order("expires_at ASC, id ASC").Limit(limit).Find(&rows).Error; err != nil {
		return 0, err
	}
	refunded := 0
	for _, row := range rows {
		err := RefundUserQuotaReservation(row.RequestId, row.UserId, "expired wallet reservation refunded")
		if err == nil {
			refunded++
			continue
		}
		if errors.Is(err, ErrUserQuotaReservationState) {
			continue
		}
		return refunded, err
	}
	return refunded, nil
}

// RepairNegativeUserQuotas is a bounded, idempotent migration. It forgives the
// historical overdrawn amount, records the correction, and never touches a
// non-negative balance.
func RepairNegativeUserQuotas(repairID string, limit int) (int, error) {
	repairID = strings.TrimSpace(repairID)
	if repairID == "" || len(repairID) > 128 {
		return 0, errors.New("repair id must contain 1 to 128 characters")
	}
	if limit <= 0 {
		limit = 1000
	}
	var repairedUserIDs []int
	err := DB.Transaction(func(tx *gorm.DB) error {
		var users []User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Select("id", "quota").Where("quota < 0").Order("id ASC").Limit(limit).Find(&users).Error; err != nil {
			return err
		}
		for _, user := range users {
			update := tx.Model(&User{}).Where("id = ? AND quota = ? AND quota < 0", user.Id, user.Quota).
				Update("quota", 0)
			if update.Error != nil {
				return update.Error
			}
			if update.RowsAffected == 0 {
				continue
			}
			if err := createUserQuotaTransactionTx(tx, &UserQuotaTransaction{
				UserId: user.Id, RequestId: repairID, SourceType: "negative_balance_repair",
				TransactionType: "repair", DeltaQuota: -user.Quota,
				BalanceBefore: user.Quota, BalanceAfter: 0,
				IdempotencyKey: fmt.Sprintf("%s:%d", repairID, user.Id),
				Remark:         "historical negative wallet balance repaired after conditional debit rollout",
			}); err != nil {
				return err
			}
			repairedUserIDs = append(repairedUserIDs, user.Id)
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	for _, userID := range repairedUserIDs {
		invalidateUserQuotaCache(userID, "negative balance repair")
	}
	return len(repairedUserIDs), nil
}

func repairAllNegativeUserQuotas(repairID string, batchSize int) (int, error) {
	if batchSize <= 0 {
		batchSize = 1000
	}
	total := 0
	for {
		repaired, err := RepairNegativeUserQuotas(repairID, batchSize)
		if err != nil {
			return total, err
		}
		total += repaired
		if repaired < batchSize {
			return total, nil
		}
	}
}

func ReconcileUserQuotaAccounting(now time.Time) (*UserQuotaReconciliationReport, error) {
	report := &UserQuotaReconciliationReport{}
	if err := DB.Model(&User{}).Where("quota < 0").Count(&report.NegativeUsers).Error; err != nil {
		return nil, err
	}
	if err := DB.Model(&UserQuotaReservation{}).Where("status = ?", UserQuotaReservationActive).
		Count(&report.ActiveReservations).Error; err != nil {
		return nil, err
	}
	if err := DB.Model(&UserQuotaReservation{}).
		Where("status = ? AND expires_at > 0 AND expires_at <= ?", UserQuotaReservationActive, now.Unix()).
		Count(&report.ExpiredActiveReservations).Error; err != nil {
		return nil, err
	}
	if err := DB.Table("user_quota_reservations AS r").
		Joins("LEFT JOIN users AS u ON u.id = r.user_id").Where("u.id IS NULL").
		Count(&report.OrphanReservations).Error; err != nil {
		return nil, err
	}

	type mismatchCount struct {
		Count int64
	}
	var mismatches mismatchCount
	if err := DB.Raw(`
SELECT COUNT(*) AS count
FROM user_quota_reservations AS r
LEFT JOIN (
    SELECT reservation_id, COALESCE(SUM(delta_quota), 0) AS delta_quota
    FROM user_quota_transactions
    WHERE reservation_id > 0
    GROUP BY reservation_id
) AS t ON t.reservation_id = r.id
WHERE r.status NOT IN (?, ?, ?)
   OR COALESCE(t.delta_quota, 0) <> CASE r.status
       WHEN ? THEN -r.reserved_quota
       WHEN ? THEN -r.settled_quota
       WHEN ? THEN 0
       ELSE 0
   END`,
		UserQuotaReservationActive, UserQuotaReservationSettled, UserQuotaReservationRefunded,
		UserQuotaReservationActive, UserQuotaReservationSettled, UserQuotaReservationRefunded,
	).Scan(&mismatches).Error; err != nil {
		return nil, err
	}
	report.ReservationLedgerMismatches = mismatches.Count
	return report, nil
}
