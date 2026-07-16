package model

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestUserQuotaReservationConcurrentPostgres(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("QUIZ_POSTGRES_TEST_DSN"))
	if dsn == "" {
		t.Skip("QUIZ_POSTGRES_TEST_DSN is not set")
	}

	adminDB, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	schema := "user_quota_" + strconv.FormatInt(time.Now().UnixNano(), 36)
	require.NoError(t, adminDB.Exec(`CREATE SCHEMA "`+schema+`"`).Error)
	t.Cleanup(func() {
		_ = adminDB.Exec(`DROP SCHEMA IF EXISTS "` + schema + `" CASCADE`).Error
	})

	db, err := gorm.Open(postgres.Open(userQuotaPostgresDSNWithSchema(t, dsn, schema)), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&User{}, &UserQuotaReservation{}, &UserQuotaTransaction{}, &TopUp{},
		&SubscriptionPlan{}, &UserSubscription{},
		&SubscriptionPreConsumeRecord{}, &SubscriptionQuotaTransaction{},
	))

	oldDB := DB
	oldSQLite, oldMySQL, oldPostgreSQL := common.UsingSQLite, common.UsingMySQL, common.UsingPostgreSQL
	oldRedis, oldBatch := common.RedisEnabled, common.BatchUpdateEnabled
	t.Cleanup(func() {
		DB = oldDB
		common.UsingSQLite, common.UsingMySQL, common.UsingPostgreSQL = oldSQLite, oldMySQL, oldPostgreSQL
		common.RedisEnabled, common.BatchUpdateEnabled = oldRedis, oldBatch
		initCol()
	})
	DB = db
	common.UsingSQLite, common.UsingMySQL, common.UsingPostgreSQL = false, false, true
	common.RedisEnabled, common.BatchUpdateEnabled = false, false
	initCol()

	t.Run("distinct requests cannot overdraw one user", func(t *testing.T) {
		user := createQuotaLedgerUser(t, "pg-distinct-reservations", 100)
		const workers = 20
		start := make(chan struct{})
		errorsCh := make(chan error, workers)
		var wg sync.WaitGroup
		for worker := 0; worker < workers; worker++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()
				<-start
				errorsCh <- ReserveUserQuota("pg-distinct-"+strconv.Itoa(index), user.Id, 10, "postgres_test")
			}(worker)
		}
		close(start)
		wg.Wait()
		close(errorsCh)

		succeeded, insufficient := 0, 0
		for reserveErr := range errorsCh {
			switch {
			case reserveErr == nil:
				succeeded++
			case errors.Is(reserveErr, ErrUserQuotaInsufficient):
				insufficient++
			default:
				require.NoError(t, reserveErr)
			}
		}
		require.Equal(t, 10, succeeded)
		require.Equal(t, 10, insufficient)
		require.Equal(t, 0, readQuotaLedgerUser(t, user.Id).Quota)
	})

	t.Run("same reservation request is idempotent across workers", func(t *testing.T) {
		user := createQuotaLedgerUser(t, "pg-same-reservation", 100)
		const workers = 20
		start := make(chan struct{})
		errorsCh := make(chan error, workers)
		var wg sync.WaitGroup
		for worker := 0; worker < workers; worker++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				errorsCh <- ReserveUserQuota("pg-same-request", user.Id, 30, "postgres_test")
			}()
		}
		close(start)
		wg.Wait()
		close(errorsCh)
		for reserveErr := range errorsCh {
			require.NoError(t, reserveErr)
		}
		require.Equal(t, 70, readQuotaLedgerUser(t, user.Id).Quota)
		var reservations, transactions int64
		require.NoError(t, DB.Model(&UserQuotaReservation{}).Where("request_id = ?", "pg-same-request").Count(&reservations).Error)
		require.NoError(t, DB.Model(&UserQuotaTransaction{}).Where("request_id = ?", "pg-same-request").Count(&transactions).Error)
		require.Equal(t, int64(1), reservations)
		require.Equal(t, int64(1), transactions)
	})

	t.Run("trusted settlement is idempotent across workers", func(t *testing.T) {
		user := createQuotaLedgerUser(t, "pg-trusted-settlement", 100)
		const workers = 20
		start := make(chan struct{})
		errorsCh := make(chan error, workers)
		var wg sync.WaitGroup
		for worker := 0; worker < workers; worker++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				errorsCh <- SettleUserQuotaReservation("pg-trusted-request", user.Id, 30, "postgres_test")
			}()
		}
		close(start)
		wg.Wait()
		close(errorsCh)
		for settleErr := range errorsCh {
			require.NoError(t, settleErr)
		}
		require.Equal(t, 70, readQuotaLedgerUser(t, user.Id).Quota)
		var transactions int64
		require.NoError(t, DB.Model(&UserQuotaTransaction{}).Where("request_id = ?", "pg-trusted-request").Count(&transactions).Error)
		require.Equal(t, int64(1), transactions)
	})

	t.Run("same resize target is idempotent across workers", func(t *testing.T) {
		user := createQuotaLedgerUser(t, "pg-same-resize", 100)
		require.NoError(t, ReserveUserQuota("pg-resize-request", user.Id, 20, "postgres_test"))
		const workers = 20
		start := make(chan struct{})
		errorsCh := make(chan error, workers)
		var wg sync.WaitGroup
		for worker := 0; worker < workers; worker++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				errorsCh <- ResizeUserQuotaReservation("pg-resize-request", user.Id, 60)
			}()
		}
		close(start)
		wg.Wait()
		close(errorsCh)
		for resizeErr := range errorsCh {
			require.NoError(t, resizeErr)
		}
		require.Equal(t, 40, readQuotaLedgerUser(t, user.Id).Quota)

		var resizeTransactions int64
		require.NoError(t, DB.Model(&UserQuotaTransaction{}).
			Where("request_id = ? AND transaction_type = ?", "pg-resize-request", "resize").
			Count(&resizeTransactions).Error)
		require.Equal(t, int64(1), resizeTransactions)
	})

	t.Run("same refund is idempotent across workers", func(t *testing.T) {
		user := createQuotaLedgerUser(t, "pg-same-refund", 100)
		require.NoError(t, ReserveUserQuota("pg-refund-request", user.Id, 30, "postgres_test"))
		const workers = 20
		start := make(chan struct{})
		errorsCh := make(chan error, workers)
		var wg sync.WaitGroup
		for worker := 0; worker < workers; worker++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				errorsCh <- RefundUserQuotaReservation("pg-refund-request", user.Id, "postgres retry")
			}()
		}
		close(start)
		wg.Wait()
		close(errorsCh)
		for refundErr := range errorsCh {
			require.NoError(t, refundErr)
		}
		require.Equal(t, 100, readQuotaLedgerUser(t, user.Id).Quota)

		var refundTransactions int64
		require.NoError(t, DB.Model(&UserQuotaTransaction{}).
			Where("request_id = ? AND transaction_type = ?", "pg-refund-request", "refund").
			Count(&refundTransactions).Error)
		require.Equal(t, int64(1), refundTransactions)
	})

	t.Run("subscription lifecycle is idempotent across workers", func(t *testing.T) {
		const userID = 7001
		now := GetDBTimestamp()
		plan := SubscriptionPlan{Title: "pg-subscription-lifecycle", TotalAmount: 100, QuotaResetPeriod: SubscriptionResetNever}
		require.NoError(t, DB.Create(&plan).Error)
		subscription := UserSubscription{
			UserId: userID, PlanId: plan.Id, AmountTotal: 100,
			StartTime: now - 60, EndTime: now + 3600, Status: "active",
		}
		require.NoError(t, DB.Create(&subscription).Error)

		runConcurrent := func(operation func() error) {
			const workers = 20
			start := make(chan struct{})
			errorsCh := make(chan error, workers)
			var wg sync.WaitGroup
			for worker := 0; worker < workers; worker++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					<-start
					errorsCh <- operation()
				}()
			}
			close(start)
			wg.Wait()
			close(errorsCh)
			for operationErr := range errorsCh {
				require.NoError(t, operationErr)
			}
		}

		runConcurrent(func() error {
			_, err := PreConsumeUserSubscription("pg-subscription-request", userID, "gpt-test", 0, 30)
			return err
		})
		runConcurrent(func() error {
			return ResizeSubscriptionPreConsume("pg-subscription-request", userID, subscription.Id, 60)
		})
		runConcurrent(func() error {
			return SettleSubscriptionPreConsume("pg-subscription-request", userID, subscription.Id, 40)
		})

		var stored UserSubscription
		require.NoError(t, DB.First(&stored, subscription.Id).Error)
		require.Equal(t, int64(40), stored.AmountUsed)
		var transactionCount int64
		require.NoError(t, DB.Model(&SubscriptionQuotaTransaction{}).
			Where("request_id = ?", "pg-subscription-request").Count(&transactionCount).Error)
		require.Equal(t, int64(3), transactionCount)
	})

	t.Run("epay callback credits once across workers", func(t *testing.T) {
		user := createQuotaLedgerUser(t, "pg-epay-callback", 100)
		topUp := TopUp{
			UserId: user.Id, Amount: 2, Money: 2, TradeNo: "pg-epay-callback",
			PaymentProvider: PaymentProviderEpay, Status: common.TopUpStatusPending,
		}
		require.NoError(t, DB.Create(&topUp).Error)
		const workers = 20
		start := make(chan struct{})
		errorsCh := make(chan error, workers)
		creditedCh := make(chan bool, workers)
		var wg sync.WaitGroup
		for worker := 0; worker < workers; worker++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				_, _, credited, callbackErr := RechargeEpay(topUp.TradeNo, "alipay")
				creditedCh <- credited
				errorsCh <- callbackErr
			}()
		}
		close(start)
		wg.Wait()
		close(errorsCh)
		close(creditedCh)
		for callbackErr := range errorsCh {
			require.NoError(t, callbackErr)
		}
		creditedCount := 0
		for credited := range creditedCh {
			if credited {
				creditedCount++
			}
		}
		require.Equal(t, 1, creditedCount)
		expectedQuota := 100 + int(decimal.NewFromInt(topUp.Amount).Mul(decimal.NewFromFloat(common.QuotaPerUnit)).IntPart())
		require.Equal(t, expectedQuota, readQuotaLedgerUser(t, user.Id).Quota)
		var creditTransactions int64
		require.NoError(t, DB.Model(&UserQuotaTransaction{}).
			Where("request_id = ?", fmt.Sprintf("topup:%d", topUp.Id)).Count(&creditTransactions).Error)
		require.Equal(t, int64(1), creditTransactions)
	})

	report, err := ReconcileUserQuotaAccounting(time.Now())
	require.NoError(t, err)
	require.Zero(t, report.NegativeUsers)
	require.Zero(t, report.OrphanReservations)
	require.Zero(t, report.ReservationLedgerMismatches)
}

func userQuotaPostgresDSNWithSchema(t *testing.T, dsn string, schema string) string {
	t.Helper()
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		parsed, err := url.Parse(dsn)
		require.NoError(t, err)
		query := parsed.Query()
		query.Set("search_path", schema)
		parsed.RawQuery = query.Encode()
		return parsed.String()
	}
	return strings.TrimSpace(dsn) + " search_path=" + schema
}
