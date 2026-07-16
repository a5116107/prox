package service

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type failingFunding struct {
	err error
}

func (f *failingFunding) Source() string         { return BillingSourceSubscription }
func (f *failingFunding) PreConsume(_ int) error { return f.err }
func (f *failingFunding) Settle(_ int) error     { return nil }
func (f *failingFunding) Refund() error          { return nil }

func setupWalletFundingTestDB(t *testing.T) {
	t.Helper()
	oldDB := model.DB
	oldSQLite := common.UsingSQLite
	oldPostgreSQL := common.UsingPostgreSQL
	oldRedis := common.RedisEnabled
	oldBatch := common.BatchUpdateEnabled

	dsn := fmt.Sprintf("file:wallet_funding_%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	common.UsingSQLite = true
	common.UsingPostgreSQL = false
	common.RedisEnabled = false
	common.BatchUpdateEnabled = false
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(
		&model.User{}, &model.UserQuotaReservation{}, &model.UserQuotaTransaction{},
		&model.SubscriptionPlan{}, &model.UserSubscription{},
		&model.SubscriptionPreConsumeRecord{}, &model.SubscriptionQuotaTransaction{},
	))

	t.Cleanup(func() {
		_ = sqlDB.Close()
		model.DB = oldDB
		common.UsingSQLite = oldSQLite
		common.UsingPostgreSQL = oldPostgreSQL
		common.RedisEnabled = oldRedis
		common.BatchUpdateEnabled = oldBatch
	})
}

func createWalletFundingUser(t *testing.T, quota int) model.User {
	t.Helper()
	user := model.User{
		Username: "wallet-funding-" + strings.ReplaceAll(t.Name(), "/", "-"),
		Password: "Password123!", DisplayName: "wallet funding",
		Role: common.RoleCommonUser, Status: common.UserStatusEnabled,
		Quota: quota, Group: "default", AffCode: "wallet-funding-aff",
	}
	require.NoError(t, model.DB.Create(&user).Error)
	return user
}

func walletFundingBalance(t *testing.T, userID int) int {
	t.Helper()
	var quota int
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", userID).Select("quota").Scan(&quota).Error)
	return quota
}

func TestBillingSessionSettlesZeroDeltaWalletReservation(t *testing.T) {
	setupWalletFundingTestDB(t)
	user := createWalletFundingUser(t, 100)
	funding := &WalletFunding{
		requestId: "wallet-zero-delta", userId: user.Id, sourceType: "api_billing",
	}
	require.NoError(t, funding.PreConsume(40))

	session := &BillingSession{
		relayInfo: &relaycommon.RelayInfo{UserId: user.Id, IsPlayground: true},
		funding:   funding, preConsumedQuota: 40,
	}
	require.NoError(t, session.Settle(40))
	require.Equal(t, 60, walletFundingBalance(t, user.Id))

	var reservation model.UserQuotaReservation
	require.NoError(t, model.DB.Where("request_id = ?", "wallet-zero-delta").First(&reservation).Error)
	require.Equal(t, model.UserQuotaReservationSettled, reservation.Status)
	require.Equal(t, 40, reservation.SettledQuota)
}

func TestWalletFundingRefundIsIdempotent(t *testing.T) {
	setupWalletFundingTestDB(t)
	user := createWalletFundingUser(t, 100)
	funding := &WalletFunding{
		requestId: "wallet-refund", userId: user.Id, sourceType: "api_billing",
	}
	require.NoError(t, funding.PreConsume(35))
	require.Equal(t, 65, walletFundingBalance(t, user.Id))
	require.NoError(t, funding.Refund())
	require.NoError(t, funding.Refund())
	require.Equal(t, 100, walletFundingBalance(t, user.Id))
}

func TestWalletFundingFailedOverageKeepsRecoverableReservation(t *testing.T) {
	setupWalletFundingTestDB(t)
	user := createWalletFundingUser(t, 100)
	funding := &WalletFunding{
		requestId: "wallet-overage", userId: user.Id, sourceType: "api_billing",
	}
	require.NoError(t, funding.PreConsume(40))

	err := funding.Settle(70)
	require.Error(t, err)
	require.True(t, errors.Is(err, model.ErrUserQuotaInsufficient))
	require.Equal(t, 60, walletFundingBalance(t, user.Id))
	require.NoError(t, funding.Refund())
	require.Equal(t, 100, walletFundingBalance(t, user.Id))
}

func TestBillingSessionClassifiesSubscriptionFundingErrors(t *testing.T) {
	testCases := []struct {
		name         string
		fundingError error
		expectedCode types.ErrorCode
	}{
		{name: "missing subscription", fundingError: fmt.Errorf("wrapped: %w", model.ErrNoActiveSubscription), expectedCode: types.ErrorCodeInsufficientUserQuota},
		{name: "insufficient quota", fundingError: fmt.Errorf("wrapped: %w", model.ErrSubscriptionQuotaInsufficient), expectedCode: types.ErrorCodeInsufficientUserQuota},
		{name: "storage failure", fundingError: errors.New("storage failure"), expectedCode: types.ErrorCodeUpdateDataError},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			session := &BillingSession{
				relayInfo: &relaycommon.RelayInfo{UserId: 1, ForcePreConsume: true, IsPlayground: true},
				funding:   &failingFunding{err: testCase.fundingError},
			}
			apiErr := session.preConsume(nil, 0)
			require.NotNil(t, apiErr)
			require.Equal(t, testCase.expectedCode, apiErr.GetErrorCode())
			require.True(t, errors.Is(apiErr, testCase.fundingError))
		})
	}
}

func TestBillingSessionPersistsSubscriptionReserveAndSettlement(t *testing.T) {
	setupWalletFundingTestDB(t)
	user := createWalletFundingUser(t, 0)
	now := model.GetDBTimestamp()
	plan := model.SubscriptionPlan{Title: "billing-session-subscription", TotalAmount: 100, QuotaResetPeriod: model.SubscriptionResetNever}
	require.NoError(t, model.DB.Create(&plan).Error)
	subscription := model.UserSubscription{
		UserId: user.Id, PlanId: plan.Id, AmountTotal: 100,
		StartTime: now - 60, EndTime: now + 3600, Status: "active",
	}
	require.NoError(t, model.DB.Create(&subscription).Error)

	funding := &SubscriptionFunding{
		requestId: "billing-subscription-lifecycle", userId: user.Id,
		modelName: "gpt-test", amount: 20,
	}
	require.NoError(t, funding.PreConsume(20))
	session := &BillingSession{
		relayInfo: &relaycommon.RelayInfo{UserId: user.Id, IsPlayground: true},
		funding:   funding, preConsumedQuota: 20,
	}
	require.NoError(t, session.Reserve(50))
	require.NoError(t, session.Reserve(50))
	require.NoError(t, session.Settle(35))
	require.NoError(t, session.Settle(35))

	var storedSubscription model.UserSubscription
	require.NoError(t, model.DB.First(&storedSubscription, subscription.Id).Error)
	require.Equal(t, int64(35), storedSubscription.AmountUsed)
	var record model.SubscriptionPreConsumeRecord
	require.NoError(t, model.DB.Where("request_id = ?", "billing-subscription-lifecycle").First(&record).Error)
	require.Equal(t, int64(35), record.PreConsumed)
	require.Equal(t, "settled", record.Status)
	require.Equal(t, int64(35), session.relayInfo.SubscriptionPreConsumed)
}
