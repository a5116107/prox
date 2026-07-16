package model

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupSubscriptionPreConsumeTestDB(t *testing.T) {
	t.Helper()
	oldDB := DB
	oldSQLite := common.UsingSQLite
	oldPostgreSQL := common.UsingPostgreSQL
	oldRedis := common.RedisEnabled

	dsn := fmt.Sprintf("file:subscription_preconsume_%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	DB = db
	common.UsingSQLite = true
	common.UsingPostgreSQL = false
	common.RedisEnabled = false
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&SubscriptionPlan{}, &UserSubscription{}, &SubscriptionPreConsumeRecord{}, &SubscriptionQuotaTransaction{}))

	t.Cleanup(func() {
		_ = sqlDB.Close()
		DB = oldDB
		common.UsingSQLite = oldSQLite
		common.UsingPostgreSQL = oldPostgreSQL
		common.RedisEnabled = oldRedis
	})
}

func TestPreConsumeUserSubscriptionClassifiesMissingSubscription(t *testing.T) {
	setupSubscriptionPreConsumeTestDB(t)

	_, err := PreConsumeUserSubscription("missing-subscription", 1, "gpt-test", 0, 1)
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrNoActiveSubscription))
	require.False(t, errors.Is(err, ErrSubscriptionQuotaInsufficient))
}

func TestPreConsumeUserSubscriptionClassifiesInsufficientQuota(t *testing.T) {
	setupSubscriptionPreConsumeTestDB(t)
	now := GetDBTimestamp()
	plan := SubscriptionPlan{Title: "quota-test", TotalAmount: 10, QuotaResetPeriod: SubscriptionResetNever}
	require.NoError(t, DB.Create(&plan).Error)
	subscription := UserSubscription{
		UserId: 1, PlanId: plan.Id, AmountTotal: 10, AmountUsed: 9,
		StartTime: now - 60, EndTime: now + 3600, Status: "active",
	}
	require.NoError(t, DB.Create(&subscription).Error)

	_, err := PreConsumeUserSubscription("insufficient-subscription", 1, "gpt-test", 0, 2)
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrSubscriptionQuotaInsufficient))
	require.False(t, errors.Is(err, ErrNoActiveSubscription))
}

func createSubscriptionPreConsumeFixture(t *testing.T, userID int, total int64, used int64) UserSubscription {
	t.Helper()
	now := GetDBTimestamp()
	plan := SubscriptionPlan{Title: "lifecycle-test", TotalAmount: total, QuotaResetPeriod: SubscriptionResetNever}
	require.NoError(t, DB.Create(&plan).Error)
	subscription := UserSubscription{
		UserId: userID, PlanId: plan.Id, AmountTotal: total, AmountUsed: used,
		StartTime: now - 60, EndTime: now + 3600, Status: "active",
	}
	require.NoError(t, DB.Create(&subscription).Error)
	return subscription
}

func readSubscriptionPreConsumeFixture(t *testing.T, subscriptionID int, requestID string) (UserSubscription, SubscriptionPreConsumeRecord) {
	t.Helper()
	var subscription UserSubscription
	require.NoError(t, DB.First(&subscription, subscriptionID).Error)
	var record SubscriptionPreConsumeRecord
	require.NoError(t, DB.Where("request_id = ?", requestID).First(&record).Error)
	return subscription, record
}

func TestSubscriptionPreConsumeLifecycleIsDurableAndIdempotent(t *testing.T) {
	setupSubscriptionPreConsumeTestDB(t)
	subscription := createSubscriptionPreConsumeFixture(t, 11, 100, 0)

	result, err := PreConsumeUserSubscription("subscription-lifecycle", 11, "gpt-test", 0, 20)
	require.NoError(t, err)
	require.Equal(t, subscription.Id, result.UserSubscriptionId)
	require.Equal(t, int64(20), result.AmountUsedAfter)

	_, err = PreConsumeUserSubscription("subscription-lifecycle", 11, "gpt-test", 0, 20)
	require.NoError(t, err)
	_, err = PreConsumeUserSubscription("subscription-lifecycle", 11, "gpt-test", 0, 21)
	require.ErrorIs(t, err, ErrSubscriptionPreConsumeConflict)

	require.NoError(t, ResizeSubscriptionPreConsume("subscription-lifecycle", 11, subscription.Id, 50))
	require.NoError(t, ResizeSubscriptionPreConsume("subscription-lifecycle", 11, subscription.Id, 50))
	require.NoError(t, ResizeSubscriptionPreConsume("subscription-lifecycle", 11, subscription.Id, 30))
	require.NoError(t, SettleSubscriptionPreConsume("subscription-lifecycle", 11, subscription.Id, 40))
	require.NoError(t, SettleSubscriptionPreConsume("subscription-lifecycle", 11, subscription.Id, 40))
	require.ErrorIs(t, SettleSubscriptionPreConsume("subscription-lifecycle", 11, subscription.Id, 41), ErrSubscriptionPreConsumeConflict)
	require.ErrorIs(t, RefundSubscriptionPreConsume("subscription-lifecycle"), ErrSubscriptionPreConsumeState)

	storedSubscription, record := readSubscriptionPreConsumeFixture(t, subscription.Id, "subscription-lifecycle")
	require.Equal(t, int64(40), storedSubscription.AmountUsed)
	require.Equal(t, int64(40), record.PreConsumed)
	require.Equal(t, "settled", record.Status)

	var transactions []SubscriptionQuotaTransaction
	require.NoError(t, DB.Where("request_id = ?", "subscription-lifecycle").Order("id ASC").Find(&transactions).Error)
	require.Len(t, transactions, 4)
	var appliedTotal int64
	for _, transaction := range transactions {
		appliedTotal += transaction.DeltaQuota
	}
	require.Equal(t, int64(40), appliedTotal)
}

func TestSubscriptionPreConsumeRefundIncludesAdditionalReservation(t *testing.T) {
	setupSubscriptionPreConsumeTestDB(t)
	subscription := createSubscriptionPreConsumeFixture(t, 12, 100, 0)

	_, err := PreConsumeUserSubscription("subscription-refund", 12, "gpt-test", 0, 10)
	require.NoError(t, err)
	require.NoError(t, ResizeSubscriptionPreConsume("subscription-refund", 12, subscription.Id, 35))
	require.NoError(t, RefundSubscriptionPreConsume("subscription-refund"))
	require.NoError(t, RefundSubscriptionPreConsume("subscription-refund"))

	storedSubscription, record := readSubscriptionPreConsumeFixture(t, subscription.Id, "subscription-refund")
	require.Zero(t, storedSubscription.AmountUsed)
	require.Equal(t, int64(35), record.PreConsumed)
	require.Equal(t, "refunded", record.Status)

	var appliedTotal int64
	require.NoError(t, DB.Model(&SubscriptionQuotaTransaction{}).
		Where("request_id = ?", "subscription-refund").Select("COALESCE(SUM(delta_quota), 0)").Scan(&appliedTotal).Error)
	require.Zero(t, appliedTotal)
}

func TestSubscriptionRefundJournalRecordsAppliedDeltaAfterQuotaReset(t *testing.T) {
	setupSubscriptionPreConsumeTestDB(t)
	subscription := createSubscriptionPreConsumeFixture(t, 13, 100, 0)

	_, err := PreConsumeUserSubscription("subscription-reset-refund", 13, "gpt-test", 0, 30)
	require.NoError(t, err)
	require.NoError(t, DB.Model(&UserSubscription{}).Where("id = ?", subscription.Id).Update("amount_used", 0).Error)
	require.NoError(t, RefundSubscriptionPreConsume("subscription-reset-refund"))

	var refund SubscriptionQuotaTransaction
	require.NoError(t, DB.Where("idempotency_key = ?", "subscription:subscription-reset-refund:refund").First(&refund).Error)
	require.Equal(t, int64(-30), refund.RequestedDeltaQuota)
	require.Zero(t, refund.DeltaQuota)
	require.Zero(t, refund.AmountUsedBefore)
	require.Zero(t, refund.AmountUsedAfter)
}
