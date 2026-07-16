package model

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupUserQuotaLedgerTestDB(t *testing.T) {
	t.Helper()
	oldDB := DB
	oldSQLite := common.UsingSQLite
	oldPostgreSQL := common.UsingPostgreSQL
	oldRedis := common.RedisEnabled
	oldBatch := common.BatchUpdateEnabled

	dsn := fmt.Sprintf("file:user_quota_%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	DB = db
	common.UsingSQLite = true
	common.UsingPostgreSQL = false
	common.RedisEnabled = false
	common.BatchUpdateEnabled = false
	initCol()
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&User{}, &UserQuotaReservation{}, &UserQuotaTransaction{}))

	t.Cleanup(func() {
		_ = sqlDB.Close()
		DB = oldDB
		common.UsingSQLite = oldSQLite
		common.UsingPostgreSQL = oldPostgreSQL
		common.RedisEnabled = oldRedis
		common.BatchUpdateEnabled = oldBatch
		initCol()
	})
}

func createQuotaLedgerUser(t *testing.T, username string, quota int) User {
	t.Helper()
	user := User{
		Username: username, Password: "Password123!", DisplayName: username,
		Role: common.RoleCommonUser, Status: common.UserStatusEnabled,
		Quota: quota, Group: "default", AffCode: "aff-" + username,
	}
	require.NoError(t, DB.Create(&user).Error)
	return user
}

func readQuotaLedgerUser(t *testing.T, userID int) User {
	t.Helper()
	var user User
	require.NoError(t, DB.First(&user, userID).Error)
	return user
}

func TestDecreaseUserQuotaIsConditionalEvenWhenBatchingIsEnabled(t *testing.T) {
	setupUserQuotaLedgerTestDB(t)
	common.BatchUpdateEnabled = true
	user := createQuotaLedgerUser(t, "conditional-debit", 50)

	require.NoError(t, ApplyUserQuotaMutation(UserQuotaMutation{
		UserID: user.Id, DeltaQuota: -30, RequestID: "conditional-debit-1",
		SourceType: "test", TransactionType: "test_debit", IdempotencyKey: "conditional-debit-1",
	}))
	err := ApplyUserQuotaMutation(UserQuotaMutation{
		UserID: user.Id, DeltaQuota: -30, RequestID: "conditional-debit-2",
		SourceType: "test", TransactionType: "test_debit", IdempotencyKey: "conditional-debit-2",
	})
	require.ErrorIs(t, err, ErrUserQuotaInsufficient)
	require.Equal(t, 20, readQuotaLedgerUser(t, user.Id).Quota)
}

func TestConcurrentConditionalDebitNeverCreatesNegativeBalance(t *testing.T) {
	setupUserQuotaLedgerTestDB(t)
	user := createQuotaLedgerUser(t, "concurrent-debit", 100)

	var successes atomic.Int64
	var unexpected atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			requestID := fmt.Sprintf("concurrent-debit-%d", index)
			err := ApplyUserQuotaMutation(UserQuotaMutation{
				UserID: user.Id, DeltaQuota: -10, RequestID: requestID,
				SourceType: "test", TransactionType: "test_debit", IdempotencyKey: requestID,
			})
			switch {
			case err == nil:
				successes.Add(1)
			case errors.Is(err, ErrUserQuotaInsufficient):
			default:
				unexpected.Add(1)
			}
		}(i)
	}
	wg.Wait()

	require.Equal(t, int64(10), successes.Load())
	require.Zero(t, unexpected.Load())
	require.Equal(t, 0, readQuotaLedgerUser(t, user.Id).Quota)
}

func TestUserQuotaReservationLifecycleIsIdempotentAndReconciled(t *testing.T) {
	setupUserQuotaLedgerTestDB(t)
	user := createQuotaLedgerUser(t, "reservation-lifecycle", 100)

	require.NoError(t, ReserveUserQuota("request-lifecycle", user.Id, 40, "api_billing"))
	require.NoError(t, ReserveUserQuota("request-lifecycle", user.Id, 40, "api_billing"))
	require.Equal(t, 60, readQuotaLedgerUser(t, user.Id).Quota)

	require.NoError(t, ResizeUserQuotaReservation("request-lifecycle", user.Id, 70))
	require.Equal(t, 30, readQuotaLedgerUser(t, user.Id).Quota)
	require.NoError(t, ResizeUserQuotaReservation("request-lifecycle", user.Id, 50))
	require.Equal(t, 50, readQuotaLedgerUser(t, user.Id).Quota)

	require.NoError(t, SettleUserQuotaReservation("request-lifecycle", user.Id, 30, "api_billing"))
	require.NoError(t, SettleUserQuotaReservation("request-lifecycle", user.Id, 30, "api_billing"))
	require.Equal(t, 70, readQuotaLedgerUser(t, user.Id).Quota)

	var reservation UserQuotaReservation
	require.NoError(t, DB.Where("request_id = ?", "request-lifecycle").First(&reservation).Error)
	require.Equal(t, UserQuotaReservationSettled, reservation.Status)
	require.Equal(t, 30, reservation.SettledQuota)

	var ledgerCount int64
	require.NoError(t, DB.Model(&UserQuotaTransaction{}).
		Where("reservation_id = ?", reservation.Id).Count(&ledgerCount).Error)
	require.Equal(t, int64(4), ledgerCount)
	report, err := ReconcileUserQuotaAccounting(time.Now())
	require.NoError(t, err)
	require.Zero(t, report.NegativeUsers)
	require.Zero(t, report.OrphanReservations)
	require.Zero(t, report.ReservationLedgerMismatches)
}

func TestUserQuotaReservationRefundAndExpiryAreIdempotent(t *testing.T) {
	setupUserQuotaLedgerTestDB(t)
	user := createQuotaLedgerUser(t, "reservation-refund", 100)

	require.NoError(t, ReserveUserQuota("request-refund", user.Id, 25, "api_billing"))
	require.NoError(t, RefundUserQuotaReservation("request-refund", user.Id, "request failed"))
	require.NoError(t, RefundUserQuotaReservation("request-refund", user.Id, "retry"))
	require.Equal(t, 100, readQuotaLedgerUser(t, user.Id).Quota)

	require.NoError(t, ReserveUserQuota("request-expired", user.Id, 10, "api_billing"))
	require.NoError(t, DB.Model(&UserQuotaReservation{}).Where("request_id = ?", "request-expired").
		Update("expires_at", time.Now().Add(-time.Minute).Unix()).Error)
	refunded, err := RefundExpiredUserQuotaReservations(time.Now(), 10)
	require.NoError(t, err)
	require.Equal(t, 1, refunded)
	require.Equal(t, 100, readQuotaLedgerUser(t, user.Id).Quota)

	report, err := ReconcileUserQuotaAccounting(time.Now())
	require.NoError(t, err)
	require.Zero(t, report.ActiveReservations)
	require.Zero(t, report.ReservationLedgerMismatches)
}

func TestUserQuotaReservationRejectsTerminalStateReplay(t *testing.T) {
	setupUserQuotaLedgerTestDB(t)
	user := createQuotaLedgerUser(t, "reservation-terminal-replay", 100)

	require.NoError(t, ReserveUserQuota("request-settled-replay", user.Id, 20, "api_billing"))
	require.NoError(t, SettleUserQuotaReservation("request-settled-replay", user.Id, 20, "api_billing"))
	require.ErrorIs(t,
		ReserveUserQuota("request-settled-replay", user.Id, 20, "api_billing"),
		ErrUserQuotaReservationState,
	)

	require.NoError(t, ReserveUserQuota("request-refunded-replay", user.Id, 15, "api_billing"))
	require.NoError(t, RefundUserQuotaReservation("request-refunded-replay", user.Id, "request failed"))
	require.ErrorIs(t,
		ReserveUserQuota("request-refunded-replay", user.Id, 15, "api_billing"),
		ErrUserQuotaReservationState,
	)
	require.Equal(t, 80, readQuotaLedgerUser(t, user.Id).Quota)
}

func TestTrustedSettlementCreatesOneDurableDebit(t *testing.T) {
	setupUserQuotaLedgerTestDB(t)
	user := createQuotaLedgerUser(t, "trusted-settlement", 100)

	require.NoError(t, SettleUserQuotaReservation("request-trusted", user.Id, 35, "api_billing"))
	require.NoError(t, SettleUserQuotaReservation("request-trusted", user.Id, 35, "api_billing"))
	require.Equal(t, 65, readQuotaLedgerUser(t, user.Id).Quota)

	var count int64
	require.NoError(t, DB.Model(&UserQuotaTransaction{}).
		Where("request_id = ?", "request-trusted").Count(&count).Error)
	require.Equal(t, int64(1), count)
}

func TestZeroQuotaSettlementCreatesDurableTerminalState(t *testing.T) {
	setupUserQuotaLedgerTestDB(t)
	user := createQuotaLedgerUser(t, "zero-settlement", 100)

	require.NoError(t, SettleUserQuotaReservation("request-zero", user.Id, 0, "api_billing"))
	require.NoError(t, SettleUserQuotaReservation("request-zero", user.Id, 0, "api_billing"))
	require.Equal(t, 100, readQuotaLedgerUser(t, user.Id).Quota)

	var reservation UserQuotaReservation
	require.NoError(t, DB.Where("request_id = ?", "request-zero").First(&reservation).Error)
	require.Equal(t, UserQuotaReservationSettled, reservation.Status)
	require.Zero(t, reservation.SettledQuota)

	var transactionCount int64
	require.NoError(t, DB.Model(&UserQuotaTransaction{}).
		Where("request_id = ?", "request-zero").Count(&transactionCount).Error)
	require.Equal(t, int64(1), transactionCount)
	require.ErrorIs(t,
		SettleUserQuotaReservation("request-zero", user.Id, 1, "api_billing"),
		ErrUserQuotaReservationConflict,
	)
}

func TestUserQuotaSourceTypeLengthIsValidated(t *testing.T) {
	setupUserQuotaLedgerTestDB(t)
	user := createQuotaLedgerUser(t, "source-validation", 100)

	err := ReserveUserQuota("request-long-source", user.Id, 10, strings.Repeat("x", 65))
	require.EqualError(t, err, "wallet source type cannot exceed 64 characters")
	require.Equal(t, 100, readQuotaLedgerUser(t, user.Id).Quota)
}

func TestInsufficientReservationLeavesNoPartialRows(t *testing.T) {
	setupUserQuotaLedgerTestDB(t)
	user := createQuotaLedgerUser(t, "insufficient-reservation", 10)

	err := ReserveUserQuota("request-insufficient", user.Id, 11, "api_billing")
	require.ErrorIs(t, err, ErrUserQuotaInsufficient)
	require.Equal(t, 10, readQuotaLedgerUser(t, user.Id).Quota)

	var reservationCount, ledgerCount int64
	require.NoError(t, DB.Model(&UserQuotaReservation{}).Count(&reservationCount).Error)
	require.NoError(t, DB.Model(&UserQuotaTransaction{}).Count(&ledgerCount).Error)
	require.Zero(t, reservationCount)
	require.Zero(t, ledgerCount)
}

func TestApplyUserQuotaMutationIsIdempotentAndAuditable(t *testing.T) {
	setupUserQuotaLedgerTestDB(t)
	user := createQuotaLedgerUser(t, "direct-mutation", 100)
	mutation := UserQuotaMutation{
		UserID: user.Id, DeltaQuota: -30, RequestID: "task-direct-mutation",
		SourceType: "task_billing", TransactionType: "task_settle",
		IdempotencyKey: "task:direct-mutation:settle", Remark: "task adjustment",
	}

	require.NoError(t, ApplyUserQuotaMutation(mutation))
	require.NoError(t, ApplyUserQuotaMutation(mutation))
	require.Equal(t, 70, readQuotaLedgerUser(t, user.Id).Quota)

	var rows []UserQuotaTransaction
	require.NoError(t, DB.Where("idempotency_key = ?", mutation.IdempotencyKey).Find(&rows).Error)
	require.Len(t, rows, 1)
	require.Equal(t, 100, rows[0].BalanceBefore)
	require.Equal(t, 70, rows[0].BalanceAfter)

	conflict := mutation
	conflict.DeltaQuota = -31
	require.ErrorIs(t, ApplyUserQuotaMutation(conflict), ErrUserQuotaMutationConflict)
	require.Equal(t, 70, readQuotaLedgerUser(t, user.Id).Quota)
}

func TestApplyUserQuotaMutationRollsBackInsufficientDebit(t *testing.T) {
	setupUserQuotaLedgerTestDB(t)
	user := createQuotaLedgerUser(t, "direct-mutation-insufficient", 10)
	mutation := UserQuotaMutation{
		UserID: user.Id, DeltaQuota: -11, RequestID: "insufficient-direct-mutation",
		SourceType: "task_billing", TransactionType: "task_settle",
		IdempotencyKey: "task:direct-mutation:insufficient",
	}

	require.ErrorIs(t, ApplyUserQuotaMutation(mutation), ErrUserQuotaInsufficient)
	require.Equal(t, 10, readQuotaLedgerUser(t, user.Id).Quota)
	var count int64
	require.NoError(t, DB.Model(&UserQuotaTransaction{}).Where("idempotency_key = ?", mutation.IdempotencyKey).Count(&count).Error)
	require.Zero(t, count)
}

func TestSetUserQuotaBalanceIsIdempotentByTarget(t *testing.T) {
	setupUserQuotaLedgerTestDB(t)
	user := createQuotaLedgerUser(t, "quota-override", 100)

	require.NoError(t, SetUserQuotaBalance(user.Id, 35, "admin-override:1", "admin_override", "override"))
	require.NoError(t, SetUserQuotaBalance(user.Id, 35, "admin-override:1", "admin_override", "retry"))
	require.Equal(t, 35, readQuotaLedgerUser(t, user.Id).Quota)
	require.ErrorIs(t,
		SetUserQuotaBalance(user.Id, 36, "admin-override:1", "admin_override", "conflict"),
		ErrUserQuotaMutationConflict,
	)
}

func TestRedeemWritesUserQuotaJournalOnce(t *testing.T) {
	setupUserQuotaLedgerTestDB(t)
	require.NoError(t, DB.AutoMigrate(&Redemption{}))
	user := createQuotaLedgerUser(t, "redemption-journal", 100)
	redemption := Redemption{Key: "redemption-journal-key", Status: common.RedemptionCodeStatusEnabled, Quota: 25}
	require.NoError(t, DB.Create(&redemption).Error)

	quota, err := Redeem(redemption.Key, user.Id)
	require.NoError(t, err)
	require.Equal(t, 25, quota)
	_, err = Redeem(redemption.Key, user.Id)
	require.ErrorIs(t, err, ErrRedeemFailed)
	require.Equal(t, 125, readQuotaLedgerUser(t, user.Id).Quota)

	var journal UserQuotaTransaction
	require.NoError(t, DB.Where("idempotency_key = ?", fmt.Sprintf("redemption:%d", redemption.Id)).First(&journal).Error)
	require.Equal(t, 25, journal.DeltaQuota)
	require.Equal(t, 100, journal.BalanceBefore)
	require.Equal(t, 125, journal.BalanceAfter)
}

func TestRepairNegativeUserQuotasWritesAuditableCorrections(t *testing.T) {
	setupUserQuotaLedgerTestDB(t)
	first := createQuotaLedgerUser(t, "repair-negative-1", -10)
	second := createQuotaLedgerUser(t, "repair-negative-2", -20)
	positive := createQuotaLedgerUser(t, "repair-positive", 5)

	repaired, err := RepairNegativeUserQuotas("repair-test-v1", 100)
	require.NoError(t, err)
	require.Equal(t, 2, repaired)
	require.Zero(t, readQuotaLedgerUser(t, first.Id).Quota)
	require.Zero(t, readQuotaLedgerUser(t, second.Id).Quota)
	require.Equal(t, 5, readQuotaLedgerUser(t, positive.Id).Quota)

	var rows []UserQuotaTransaction
	require.NoError(t, DB.Where("source_type = ?", "negative_balance_repair").Order("user_id ASC").Find(&rows).Error)
	require.Len(t, rows, 2)
	require.Equal(t, 10, rows[0].DeltaQuota)
	require.Equal(t, 20, rows[1].DeltaQuota)

	repaired, err = RepairNegativeUserQuotas("repair-test-v1", 100)
	require.NoError(t, err)
	require.Zero(t, repaired)
}

func TestRepairAllNegativeUserQuotasProcessesEveryBatch(t *testing.T) {
	setupUserQuotaLedgerTestDB(t)
	first := createQuotaLedgerUser(t, "repair-batch-1", -10)
	second := createQuotaLedgerUser(t, "repair-batch-2", -20)
	third := createQuotaLedgerUser(t, "repair-batch-3", -30)

	repaired, err := repairAllNegativeUserQuotas("repair-batches-v1", 1)
	require.NoError(t, err)
	require.Equal(t, 3, repaired)
	require.Zero(t, readQuotaLedgerUser(t, first.Id).Quota)
	require.Zero(t, readQuotaLedgerUser(t, second.Id).Quota)
	require.Zero(t, readQuotaLedgerUser(t, third.Id).Quota)
}
