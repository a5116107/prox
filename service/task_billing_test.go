package service

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestMain(m *testing.M) {
	postgresDSN := os.Getenv("TEST_POSTGRES_DSN")
	var db *gorm.DB
	var err error
	if postgresDSN != "" {
		db, err = gorm.Open(postgres.Open(postgresDSN), &gorm.Config{})
	} else {
		db, err = gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	}
	if err != nil {
		panic("failed to open test db: " + err.Error())
	}
	sqlDB, err := db.DB()
	if err != nil {
		panic("failed to get sql.DB: " + err.Error())
	}
	if postgresDSN != "" {
		sqlDB.SetMaxOpenConns(32)
	} else {
		sqlDB.SetMaxOpenConns(1)
	}

	model.DB = db
	model.LOG_DB = db

	common.UsingSQLite = postgresDSN == ""
	common.RedisEnabled = false
	common.BatchUpdateEnabled = false
	common.LogConsumeEnabled = true

	if err := db.AutoMigrate(
		&model.Task{},
		&model.TaskBillingOperation{},
		&model.Midjourney{},
		&model.User{},
		&model.Token{},
		&model.Log{},
		&model.Channel{},
		&model.TopUp{},
		&model.UserSubscription{},
		&model.UserQuotaTransaction{},
		&model.SubscriptionQuotaTransaction{},
	); err != nil {
		panic("failed to migrate: " + err.Error())
	}

	os.Exit(m.Run())
}

// ---------------------------------------------------------------------------
// Seed helpers
// ---------------------------------------------------------------------------

func truncate(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		model.DB.Exec("DELETE FROM tasks")
		model.DB.Exec("DELETE FROM task_billing_operations")
		model.DB.Exec("DELETE FROM midjourneys")
		model.DB.Exec("DELETE FROM users")
		model.DB.Exec("DELETE FROM tokens")
		model.DB.Exec("DELETE FROM logs")
		model.DB.Exec("DELETE FROM channels")
		model.DB.Exec("DELETE FROM top_ups")
		model.DB.Exec("DELETE FROM user_subscriptions")
		model.DB.Exec("DELETE FROM user_quota_transactions")
		model.DB.Exec("DELETE FROM subscription_quota_transactions")
	})
}

func seedUser(t *testing.T, id int, quota int) {
	t.Helper()
	user := &model.User{Id: id, Username: "test_user", Quota: quota, Status: common.UserStatusEnabled}
	require.NoError(t, model.DB.Create(user).Error)
}

func seedToken(t *testing.T, id int, userId int, key string, remainQuota int) {
	t.Helper()
	token := &model.Token{
		Id:          id,
		UserId:      userId,
		Key:         key,
		Name:        "test_token",
		Status:      common.TokenStatusEnabled,
		RemainQuota: remainQuota,
		UsedQuota:   0,
	}
	require.NoError(t, model.DB.Create(token).Error)
}

func seedSubscription(t *testing.T, id int, userId int, amountTotal int64, amountUsed int64) {
	t.Helper()
	sub := &model.UserSubscription{
		Id:          id,
		UserId:      userId,
		AmountTotal: amountTotal,
		AmountUsed:  amountUsed,
		Status:      "active",
		StartTime:   time.Now().Unix(),
		EndTime:     time.Now().Add(30 * 24 * time.Hour).Unix(),
	}
	require.NoError(t, model.DB.Create(sub).Error)
}

func seedChannel(t *testing.T, id int) {
	t.Helper()
	ch := &model.Channel{Id: id, Name: "test_channel", Key: "sk-test", Status: common.ChannelStatusEnabled}
	require.NoError(t, model.DB.Create(ch).Error)
}

func makeTask(userId, channelId, quota, tokenId int, billingSource string, subscriptionId int) *model.Task {
	return &model.Task{
		TaskID:    "task_" + time.Now().Format("150405.000"),
		UserId:    userId,
		ChannelId: channelId,
		Quota:     quota,
		Status:    model.TaskStatus(model.TaskStatusInProgress),
		Group:     "default",
		Data:      json.RawMessage(`{}`),
		CreatedAt: time.Now().Unix(),
		UpdatedAt: time.Now().Unix(),
		Properties: model.Properties{
			OriginModelName: "test-model",
		},
		PrivateData: model.TaskPrivateData{
			BillingSource:  billingSource,
			SubscriptionId: subscriptionId,
			TokenId:        tokenId,
			BillingContext: &model.TaskBillingContext{
				ModelPrice:      0.02,
				GroupRatio:      1.0,
				OriginModelName: "test-model",
			},
		},
	}
}

// ---------------------------------------------------------------------------
// Read-back helpers
// ---------------------------------------------------------------------------

func getUserQuota(t *testing.T, id int) int {
	t.Helper()
	var user model.User
	require.NoError(t, model.DB.Select("quota").Where("id = ?", id).First(&user).Error)
	return user.Quota
}

func getTokenRemainQuota(t *testing.T, id int) int {
	t.Helper()
	var token model.Token
	require.NoError(t, model.DB.Select("remain_quota").Where("id = ?", id).First(&token).Error)
	return token.RemainQuota
}

func getTokenUsedQuota(t *testing.T, id int) int {
	t.Helper()
	var token model.Token
	require.NoError(t, model.DB.Select("used_quota").Where("id = ?", id).First(&token).Error)
	return token.UsedQuota
}

func getSubscriptionUsed(t *testing.T, id int) int64 {
	t.Helper()
	var sub model.UserSubscription
	require.NoError(t, model.DB.Select("amount_used").Where("id = ?", id).First(&sub).Error)
	return sub.AmountUsed
}

func getLastLog(t *testing.T) *model.Log {
	t.Helper()
	var log model.Log
	err := model.LOG_DB.Order("id desc").First(&log).Error
	if err != nil {
		return nil
	}
	return &log
}

func countLogs(t *testing.T) int64 {
	t.Helper()
	var count int64
	model.LOG_DB.Model(&model.Log{}).Count(&count)
	return count
}

func getUserUsedQuota(t *testing.T, id int) int {
	t.Helper()
	var user model.User
	require.NoError(t, model.DB.Select("used_quota").First(&user, id).Error)
	return user.UsedQuota
}

func getChannelUsedQuota(t *testing.T, id int) int64 {
	t.Helper()
	var channel model.Channel
	require.NoError(t, model.DB.Select("used_quota").First(&channel, id).Error)
	return channel.UsedQuota
}

// ===========================================================================
// CAS + Billing integration tests
// Simulates the flow in updateVideoSingleTask (service/task_polling.go)
// ===========================================================================

// simulatePollBilling reproduces the CAS + billing logic from updateVideoSingleTask.
// It takes a persisted task (already in DB), applies the new status, and performs
// the conditional update + billing exactly as the polling loop does.
func simulatePollBilling(ctx context.Context, task *model.Task, newStatus model.TaskStatus, actualQuota int) {
	snap := task.Snapshot()

	task.Status = newStatus
	switch string(newStatus) {
	case model.TaskStatusSuccess:
		task.Progress = "100%"
		task.FinishTime = 9999
	case model.TaskStatusFailure:
		task.Progress = "100%"
		task.FinishTime = 9999
		task.FailReason = "upstream error"
	default:
		task.Progress = "50%"
	}

	isDone := task.Status == model.TaskStatus(model.TaskStatusSuccess) || task.Status == model.TaskStatus(model.TaskStatusFailure)
	if isDone && snap.Status != task.Status {
		_, _ = FinalizeTaskTransition(ctx, task, snap.Status, actualQuota, task.FailReason)
	} else if !snap.Equal(task.Snapshot()) {
		_, _ = task.UpdateWithStatus(snap.Status)
	}
}

func TestCASGuardedRefund_Win(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID, tokenID, channelID = 20, 20, 20
	const initQuota, preConsumed = 10000, 4000
	const tokenRemain = 6000

	seedUser(t, userID, initQuota)
	seedToken(t, tokenID, userID, "sk-cas-refund-win", tokenRemain)
	seedChannel(t, channelID)

	task := makeTask(userID, channelID, preConsumed, tokenID, BillingSourceWallet, 0)
	task.Status = model.TaskStatus(model.TaskStatusInProgress)
	require.NoError(t, model.DB.Create(task).Error)

	simulatePollBilling(ctx, task, model.TaskStatus(model.TaskStatusFailure), 0)

	// CAS wins: task in DB should now be FAILURE
	var reloaded model.Task
	require.NoError(t, model.DB.First(&reloaded, task.ID).Error)
	assert.EqualValues(t, model.TaskStatusFailure, reloaded.Status)

	// Refund should have happened
	assert.Equal(t, initQuota+preConsumed, getUserQuota(t, userID))
	assert.Equal(t, tokenRemain+preConsumed, getTokenRemainQuota(t, tokenID))

	log := getLastLog(t)
	require.NotNil(t, log)
	assert.Equal(t, model.LogTypeRefund, log.Type)
}

func TestCASGuardedRefund_Lose(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID, tokenID, channelID = 21, 21, 21
	const initQuota, preConsumed = 10000, 4000
	const tokenRemain = 6000

	seedUser(t, userID, initQuota)
	seedToken(t, tokenID, userID, "sk-cas-refund-lose", tokenRemain)
	seedChannel(t, channelID)

	// Create task with IN_PROGRESS in DB
	task := makeTask(userID, channelID, preConsumed, tokenID, BillingSourceWallet, 0)
	task.Status = model.TaskStatus(model.TaskStatusInProgress)
	require.NoError(t, model.DB.Create(task).Error)

	// Simulate another process already transitioning to FAILURE
	model.DB.Model(&model.Task{}).Where("id = ?", task.ID).Update("status", model.TaskStatusFailure)

	// Our process still has the old in-memory state (IN_PROGRESS) and tries to transition
	// task.Status is still IN_PROGRESS in the snapshot
	simulatePollBilling(ctx, task, model.TaskStatus(model.TaskStatusFailure), 0)

	// CAS lost: user quota should NOT change (no double refund)
	assert.Equal(t, initQuota, getUserQuota(t, userID))
	assert.Equal(t, tokenRemain, getTokenRemainQuota(t, tokenID))

	// No billing log should be created
	assert.Equal(t, int64(0), countLogs(t))
}

func TestCASGuardedSettle_Win(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID, tokenID, channelID = 22, 22, 22
	const initQuota, preConsumed = 10000, 5000
	const actualQuota = 3000 // over-charged, should get partial refund
	const tokenRemain = 8000

	seedUser(t, userID, initQuota)
	seedToken(t, tokenID, userID, "sk-cas-settle-win", tokenRemain)
	seedChannel(t, channelID)

	task := makeTask(userID, channelID, preConsumed, tokenID, BillingSourceWallet, 0)
	task.Status = model.TaskStatus(model.TaskStatusInProgress)
	require.NoError(t, model.DB.Create(task).Error)

	simulatePollBilling(ctx, task, model.TaskStatus(model.TaskStatusSuccess), actualQuota)

	// CAS wins: task should be SUCCESS
	var reloaded model.Task
	require.NoError(t, model.DB.First(&reloaded, task.ID).Error)
	assert.EqualValues(t, model.TaskStatusSuccess, reloaded.Status)

	// Settlement should refund the over-charge (5000 - 3000 = 2000 back to user)
	assert.Equal(t, initQuota+(preConsumed-actualQuota), getUserQuota(t, userID))
	assert.Equal(t, tokenRemain+(preConsumed-actualQuota), getTokenRemainQuota(t, tokenID))

	// task.Quota should be updated to actualQuota
	assert.Equal(t, actualQuota, task.Quota)
}

func TestNonTerminalUpdate_NoBilling(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID, channelID = 23, 23
	const initQuota, preConsumed = 10000, 3000

	seedUser(t, userID, initQuota)
	seedChannel(t, channelID)

	task := makeTask(userID, channelID, preConsumed, 0, BillingSourceWallet, 0)
	task.Status = model.TaskStatus(model.TaskStatusInProgress)
	task.Progress = "20%"
	require.NoError(t, model.DB.Create(task).Error)

	// Simulate a non-terminal poll update (still IN_PROGRESS, progress changed)
	simulatePollBilling(ctx, task, model.TaskStatus(model.TaskStatusInProgress), 0)

	// User quota should NOT change
	assert.Equal(t, initQuota, getUserQuota(t, userID))

	// No billing log
	assert.Equal(t, int64(0), countLogs(t))

	// Task progress should be updated in DB
	var reloaded model.Task
	require.NoError(t, model.DB.First(&reloaded, task.ID).Error)
	assert.Equal(t, "50%", reloaded.Progress)
}

// ===========================================================================
// Mock adaptor for terminal billing calculation tests
// ===========================================================================

type mockAdaptor struct {
	adjustReturn int
}

func (m *mockAdaptor) Init(_ *relaycommon.RelayInfo) {}
func (m *mockAdaptor) FetchTask(string, string, map[string]any, string) (*http.Response, error) {
	return nil, nil
}
func (m *mockAdaptor) ParseTaskResult([]byte) (*relaycommon.TaskInfo, error) { return nil, nil }
func (m *mockAdaptor) AdjustBillingOnComplete(_ *model.Task, _ *relaycommon.TaskInfo) int {
	return m.adjustReturn
}

// ===========================================================================
// PerCallBilling tests for the side-effect-free terminal calculation
// ===========================================================================

func TestCalculateTaskBilling_PerCallSkipsAdaptorAdjust(t *testing.T) {
	truncate(t)

	const userID, tokenID, channelID = 30, 30, 30
	const initQuota, preConsumed = 10000, 5000
	const tokenRemain = 8000

	seedUser(t, userID, initQuota)
	seedToken(t, tokenID, userID, "sk-percall-adaptor", tokenRemain)
	seedChannel(t, channelID)

	task := makeTask(userID, channelID, preConsumed, tokenID, BillingSourceWallet, 0)
	task.PrivateData.BillingContext.PerCallBilling = true

	adaptor := &mockAdaptor{adjustReturn: 2000}
	taskResult := &relaycommon.TaskInfo{Status: model.TaskStatusSuccess}

	actual, reason, ok := CalculateTaskBillingOnComplete(adaptor, task, taskResult)

	assert.False(t, ok)
	assert.Zero(t, actual)
	assert.Empty(t, reason)
	assert.Equal(t, initQuota, getUserQuota(t, userID))
	assert.Equal(t, tokenRemain, getTokenRemainQuota(t, tokenID))
	assert.Equal(t, preConsumed, task.Quota)
	assert.Equal(t, int64(0), countLogs(t))
}

func TestCalculateTaskBilling_PerCallSkipsTotalTokens(t *testing.T) {
	truncate(t)

	const userID, tokenID, channelID = 31, 31, 31
	const initQuota, preConsumed = 10000, 4000
	const tokenRemain = 7000

	seedUser(t, userID, initQuota)
	seedToken(t, tokenID, userID, "sk-percall-tokens", tokenRemain)
	seedChannel(t, channelID)

	task := makeTask(userID, channelID, preConsumed, tokenID, BillingSourceWallet, 0)
	task.PrivateData.BillingContext.PerCallBilling = true

	adaptor := &mockAdaptor{adjustReturn: 0}
	taskResult := &relaycommon.TaskInfo{Status: model.TaskStatusSuccess, TotalTokens: 9999}

	actual, reason, ok := CalculateTaskBillingOnComplete(adaptor, task, taskResult)

	assert.False(t, ok)
	assert.Zero(t, actual)
	assert.Empty(t, reason)
	assert.Equal(t, initQuota, getUserQuota(t, userID))
	assert.Equal(t, tokenRemain, getTokenRemainQuota(t, tokenID))
	assert.Equal(t, preConsumed, task.Quota)
	assert.Equal(t, int64(0), countLogs(t))
}

func TestCalculateTaskBilling_NonPerCallUsesAdaptor(t *testing.T) {
	truncate(t)

	const userID, tokenID, channelID = 32, 32, 32
	const initQuota, preConsumed = 10000, 5000
	const adaptorQuota = 3000
	const tokenRemain = 8000

	seedUser(t, userID, initQuota)
	seedToken(t, tokenID, userID, "sk-nonpercall-adj", tokenRemain)
	seedChannel(t, channelID)

	task := makeTask(userID, channelID, preConsumed, tokenID, BillingSourceWallet, 0)
	// PerCallBilling defaults to false

	adaptor := &mockAdaptor{adjustReturn: adaptorQuota}
	taskResult := &relaycommon.TaskInfo{Status: model.TaskStatusSuccess}

	actual, reason, ok := CalculateTaskBillingOnComplete(adaptor, task, taskResult)

	require.True(t, ok)
	assert.Equal(t, adaptorQuota, actual)
	assert.Equal(t, "adaptor计费调整", reason)
	assert.Equal(t, initQuota, getUserQuota(t, userID))
	assert.Equal(t, tokenRemain, getTokenRemainQuota(t, tokenID))
	assert.Equal(t, preConsumed, task.Quota)
}

func TestTaskBillingOperationRecoversAfterFundingCommit(t *testing.T) {
	truncate(t)
	ctx := context.Background()
	const userID, tokenID, channelID = 60, 60, 60
	const initialQuota, preConsumed = 10000, 2500
	seedUser(t, userID, initialQuota)
	seedToken(t, tokenID, userID, "sk-outbox-funding", 4000)
	seedChannel(t, channelID)
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", userID).Update("used_quota", preConsumed).Error)
	require.NoError(t, model.DB.Model(&model.Channel{}).Where("id = ?", channelID).Update("used_quota", preConsumed).Error)

	task := makeTask(userID, channelID, preConsumed, tokenID, BillingSourceWallet, 0)
	operation, err := model.EnsureTaskBillingOperation(newTaskBillingOperation(task, taskBillingOperationRefund, 0, "upstream failed"))
	require.NoError(t, err)

	// Simulate a process exit after the durable wallet mutation but before the
	// outbox funding_applied flag is persisted.
	require.NoError(t, applyTaskBillingFunding(operation))
	require.NoError(t, ProcessTaskBillingOperationByKey(ctx, operation.OperationKey))

	assert.Equal(t, initialQuota+preConsumed, getUserQuota(t, userID))
	assert.Equal(t, 4000+preConsumed, getTokenRemainQuota(t, tokenID))
	assert.Equal(t, 0, getUserUsedQuota(t, userID))
	assert.EqualValues(t, 0, getChannelUsedQuota(t, channelID))
	assert.Equal(t, int64(1), countLogs(t))
	var fundingRows int64
	require.NoError(t, model.DB.Model(&model.UserQuotaTransaction{}).
		Where("idempotency_key = ?", operation.OperationKey).Count(&fundingRows).Error)
	assert.Equal(t, int64(1), fundingRows)
	var stored model.TaskBillingOperation
	require.NoError(t, model.DB.Where("operation_key = ?", operation.OperationKey).First(&stored).Error)
	assert.Equal(t, model.TaskBillingOperationCompleted, stored.Status)
}

func TestTaskBillingLogIdempotencySupportsSeparateLogDatabase(t *testing.T) {
	truncate(t)
	if os.Getenv("TEST_POSTGRES_DSN") == "" {
		separateLogDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
		require.NoError(t, err)
		sqlDB, err := separateLogDB.DB()
		require.NoError(t, err)
		sqlDB.SetMaxOpenConns(1)
		require.NoError(t, separateLogDB.AutoMigrate(&model.Log{}))
		previousLogDB := model.LOG_DB
		model.LOG_DB = separateLogDB
		t.Cleanup(func() {
			model.LOG_DB = previousLogDB
			_ = sqlDB.Close()
		})
	}

	const userID = 63
	seedUser(t, userID, 1000)
	params := model.RecordTaskBillingLogParams{
		UserId: userID, LogType: model.LogTypeRefund, Content: "task refund",
		ModelName: "test-model", Quota: 100, Group: "default",
	}
	model.RecordTaskBillingLog(params)
	model.RecordTaskBillingLog(params)
	inserted, err := model.RecordTaskBillingLogIdempotent(params, "task:separate-log:refund")
	require.NoError(t, err)
	assert.True(t, inserted)
	inserted, err = model.RecordTaskBillingLogIdempotent(params, "task:separate-log:refund")
	require.NoError(t, err)
	assert.False(t, inserted)

	var total, ordinary, idempotent int64
	require.NoError(t, model.LOG_DB.Model(&model.Log{}).Count(&total).Error)
	require.NoError(t, model.LOG_DB.Model(&model.Log{}).Where("idempotency_key IS NULL").Count(&ordinary).Error)
	require.NoError(t, model.LOG_DB.Model(&model.Log{}).Where("idempotency_key = ?", "task:separate-log:refund").Count(&idempotent).Error)
	assert.Equal(t, int64(3), total)
	assert.Equal(t, int64(2), ordinary)
	assert.Equal(t, int64(1), idempotent)
}

func TestTaskBillingOperationRecoversAfterLogCommit(t *testing.T) {
	truncate(t)
	ctx := context.Background()
	const userID, tokenID, channelID = 61, 61, 61
	const initialQuota, preConsumed = 10000, 1200
	seedUser(t, userID, initialQuota)
	seedToken(t, tokenID, userID, "sk-outbox-log", 5000)
	seedChannel(t, channelID)
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", userID).Update("used_quota", preConsumed).Error)
	require.NoError(t, model.DB.Model(&model.Channel{}).Where("id = ?", channelID).Update("used_quota", preConsumed).Error)

	task := makeTask(userID, channelID, preConsumed, tokenID, BillingSourceWallet, 0)
	operation, err := model.EnsureTaskBillingOperation(newTaskBillingOperation(task, taskBillingOperationRefund, 0, "log crash"))
	require.NoError(t, err)
	claimed, won, err := model.ClaimTaskBillingOperationByKey(operation.OperationKey, taskBillingLeaseSeconds)
	require.NoError(t, err)
	require.True(t, won)
	require.NoError(t, applyTaskBillingFunding(claimed))
	require.NoError(t, model.MarkTaskBillingOperationStep(claimed.Id, claimed.LeaseToken, "funding_applied"))
	require.NoError(t, model.ApplyTaskBillingTokenStep(claimed.Id, claimed.LeaseToken))
	require.NoError(t, model.InvalidateTokenCacheById(claimed.TokenId))
	require.NoError(t, model.MarkTaskBillingOperationStep(claimed.Id, claimed.LeaseToken, "token_cache_invalidated"))
	require.NoError(t, model.ApplyTaskBillingUsageStep(claimed.Id, claimed.LeaseToken))
	other, err := common.StrToMap(claimed.LogOther)
	require.NoError(t, err)
	_, err = model.RecordTaskBillingLogIdempotent(model.RecordTaskBillingLogParams{
		UserId: claimed.UserId, LogType: claimed.LogType, Content: claimed.Reason,
		ChannelId: claimed.ChannelId, ModelName: claimed.ModelName, Quota: claimed.LogQuota,
		TokenId: claimed.TokenId, Group: claimed.Group, Other: other,
	}, claimed.OperationKey)
	require.NoError(t, err)

	// The log exists, but the process exits before log_applied. Release the
	// lease and replay immediately; the unique log key absorbs the duplicate.
	require.NoError(t, model.RetryTaskBillingOperation(claimed.Id, claimed.LeaseToken, assert.AnError))
	require.NoError(t, model.DB.Model(&model.TaskBillingOperation{}).Where("id = ?", claimed.Id).Update("next_attempt_at", 0).Error)
	require.NoError(t, ProcessTaskBillingOperationByKey(ctx, claimed.OperationKey))
	assert.Equal(t, int64(1), countLogs(t))
	assert.Equal(t, initialQuota+preConsumed, getUserQuota(t, userID))
	assert.Equal(t, 5000+preConsumed, getTokenRemainQuota(t, tokenID))
}

func TestTaskBillingOperationConcurrentWorkersApplyOnce(t *testing.T) {
	truncate(t)
	ctx := context.Background()
	const userID, tokenID, channelID = 62, 62, 62
	const initialQuota, preConsumed = 10000, 900
	seedUser(t, userID, initialQuota)
	seedToken(t, tokenID, userID, "sk-outbox-race", 5000)
	seedChannel(t, channelID)

	task := makeTask(userID, channelID, preConsumed, tokenID, BillingSourceWallet, 0)
	operation, err := model.EnsureTaskBillingOperation(newTaskBillingOperation(task, taskBillingOperationRefund, 0, "worker race"))
	require.NoError(t, err)

	const workers = 20
	var wg sync.WaitGroup
	errorsCh := make(chan error, workers)
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			errorsCh <- ProcessTaskBillingOperationByKey(ctx, operation.OperationKey)
		}()
	}
	wg.Wait()
	close(errorsCh)
	for workerErr := range errorsCh {
		require.NoError(t, workerErr)
	}

	assert.Equal(t, initialQuota+preConsumed, getUserQuota(t, userID))
	assert.Equal(t, 5000+preConsumed, getTokenRemainQuota(t, tokenID))
	assert.Equal(t, int64(1), countLogs(t))
	var fundingRows int64
	require.NoError(t, model.DB.Model(&model.UserQuotaTransaction{}).
		Where("idempotency_key = ?", operation.OperationKey).Count(&fundingRows).Error)
	assert.Equal(t, int64(1), fundingRows)
}

func TestCalculateTaskQuotaByTokensUsesFrozenRatios(t *testing.T) {
	task := makeTask(70, 70, 1000, 0, BillingSourceWallet, 0)
	task.PrivateData.BillingContext.ModelRatio = 2
	task.PrivateData.BillingContext.GroupRatio = 3
	task.PrivateData.BillingContext.OtherRatios = map[string]float64{"duration": 0.5}

	actual, _, ok := CalculateTaskQuotaByTokens(task, 100)
	require.True(t, ok)
	assert.Equal(t, 300, actual)
}

func TestFinalizeTaskTransitionSettlesSuccessDeltaExactlyOnce(t *testing.T) {
	truncate(t)
	ctx := context.Background()
	const userID, tokenID, channelID = 70, 70, 70
	const initialQuota, tokenRemain = 10000, 4000
	const preConsumed, actualQuota = 2500, 1500
	seedUser(t, userID, initialQuota)
	seedToken(t, tokenID, userID, "sk-success-settlement", tokenRemain)
	seedChannel(t, channelID)
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", userID).Update("used_quota", preConsumed).Error)
	require.NoError(t, model.DB.Model(&model.Token{}).Where("id = ?", tokenID).Update("used_quota", preConsumed).Error)
	require.NoError(t, model.DB.Model(&model.Channel{}).Where("id = ?", channelID).Update("used_quota", preConsumed).Error)

	task := makeTask(userID, channelID, preConsumed, tokenID, BillingSourceWallet, 0)
	task.TaskID = "task-success-settlement"
	require.NoError(t, model.DB.Create(task).Error)
	stale := *task
	task.Status = model.TaskStatusSuccess
	task.Progress = "100%"
	won, err := FinalizeTaskTransition(ctx, task, model.TaskStatusInProgress, actualQuota, "actual usage")
	require.NoError(t, err)
	require.True(t, won)

	stale.Status = model.TaskStatusSuccess
	stale.Progress = "100%"
	won, err = FinalizeTaskTransition(ctx, &stale, model.TaskStatusInProgress, actualQuota, "actual usage")
	require.NoError(t, err)
	assert.False(t, won)

	var storedTask model.Task
	require.NoError(t, model.DB.First(&storedTask, task.ID).Error)
	assert.EqualValues(t, model.TaskStatusSuccess, storedTask.Status)
	assert.Equal(t, actualQuota, storedTask.Quota)
	assert.Equal(t, initialQuota+(preConsumed-actualQuota), getUserQuota(t, userID))
	assert.Equal(t, tokenRemain+(preConsumed-actualQuota), getTokenRemainQuota(t, tokenID))
	assert.Equal(t, actualQuota, getTokenUsedQuota(t, tokenID))
	assert.Equal(t, actualQuota, getUserUsedQuota(t, userID))
	assert.EqualValues(t, actualQuota, getChannelUsedQuota(t, channelID))
	assert.Equal(t, int64(1), countLogs(t))

	var operation model.TaskBillingOperation
	require.NoError(t, model.DB.Where("task_record_id = ? AND operation = ?", task.ID, taskBillingOperationSettle).First(&operation).Error)
	assert.Equal(t, model.TaskBillingOperationCompleted, operation.Status)
	assert.Equal(t, preConsumed, operation.PreConsumedQuota)
	assert.Equal(t, actualQuota, operation.ActualQuota)
	assert.Equal(t, actualQuota-preConsumed, operation.FundingDelta)
	var fundingRows int64
	require.NoError(t, model.DB.Model(&model.UserQuotaTransaction{}).
		Where("idempotency_key = ?", operation.OperationKey).Count(&fundingRows).Error)
	assert.Equal(t, int64(1), fundingRows)
}

func TestFinalizeTaskTransitionChargesPositiveWalletDelta(t *testing.T) {
	truncate(t)
	ctx := context.Background()
	const userID, tokenID, channelID = 72, 72, 72
	const initialQuota, tokenRemain = 10000, 5000
	const preConsumed, actualQuota = 2000, 3000
	seedUser(t, userID, initialQuota)
	seedToken(t, tokenID, userID, "sk-positive-settlement", tokenRemain)
	seedChannel(t, channelID)
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", userID).Update("used_quota", preConsumed).Error)
	require.NoError(t, model.DB.Model(&model.Token{}).Where("id = ?", tokenID).Update("used_quota", preConsumed).Error)
	require.NoError(t, model.DB.Model(&model.Channel{}).Where("id = ?", channelID).Update("used_quota", preConsumed).Error)

	task := makeTask(userID, channelID, preConsumed, tokenID, BillingSourceWallet, 0)
	task.TaskID = "task-positive-settlement"
	require.NoError(t, model.DB.Create(task).Error)
	task.Status = model.TaskStatusSuccess
	task.Progress = "100%"
	won, err := FinalizeTaskTransition(ctx, task, model.TaskStatusInProgress, actualQuota, "actual usage")
	require.NoError(t, err)
	require.True(t, won)

	assert.Equal(t, initialQuota-(actualQuota-preConsumed), getUserQuota(t, userID))
	assert.Equal(t, tokenRemain-(actualQuota-preConsumed), getTokenRemainQuota(t, tokenID))
	assert.Equal(t, actualQuota, getTokenUsedQuota(t, tokenID))
	assert.Equal(t, actualQuota, getUserUsedQuota(t, userID))
	assert.EqualValues(t, actualQuota, getChannelUsedQuota(t, channelID))
	logEntry := getLastLog(t)
	require.NotNil(t, logEntry)
	assert.Equal(t, model.LogTypeConsume, logEntry.Type)
	assert.Equal(t, actualQuota-preConsumed, logEntry.Quota)
}

func TestFinalizeTaskTransitionRefundsSubscriptionWithoutToken(t *testing.T) {
	truncate(t)
	ctx := context.Background()
	const userID, channelID, subscriptionID = 73, 73, 73
	const preConsumed = 2000
	const subscriptionTotal, subscriptionUsed int64 = 100000, 50000
	seedUser(t, userID, 123)
	seedChannel(t, channelID)
	seedSubscription(t, subscriptionID, userID, subscriptionTotal, subscriptionUsed)
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", userID).Update("used_quota", preConsumed).Error)
	require.NoError(t, model.DB.Model(&model.Channel{}).Where("id = ?", channelID).Update("used_quota", preConsumed).Error)

	task := makeTask(userID, channelID, preConsumed, 0, BillingSourceSubscription, subscriptionID)
	task.TaskID = "task-subscription-refund"
	require.NoError(t, model.DB.Create(task).Error)
	task.Status = model.TaskStatusFailure
	task.Progress = "100%"
	task.FailReason = "upstream failed"
	won, err := FinalizeTaskTransition(ctx, task, model.TaskStatusInProgress, 0, task.FailReason)
	require.NoError(t, err)
	require.True(t, won)

	assert.Equal(t, subscriptionUsed-int64(preConsumed), getSubscriptionUsed(t, subscriptionID))
	assert.Equal(t, 123, getUserQuota(t, userID))
	assert.Zero(t, getUserUsedQuota(t, userID))
	assert.Zero(t, getChannelUsedQuota(t, channelID))
	assert.Equal(t, int64(1), countLogs(t))
	var operation model.TaskBillingOperation
	require.NoError(t, model.DB.Where("task_record_id = ?", task.ID).First(&operation).Error)
	assert.Equal(t, model.TaskBillingOperationCompleted, operation.Status)
	assert.True(t, operation.TokenApplied)
	assert.True(t, operation.TokenCacheInvalidated)
}

func TestFinalizeMidjourneyTransitionRefundsExactlyOnce(t *testing.T) {
	truncate(t)
	ctx := context.Background()
	const userID, tokenID, channelID = 71, 71, 71
	const initialQuota, preConsumed = 10000, 1800
	seedUser(t, userID, initialQuota)
	seedToken(t, tokenID, userID, "sk-midjourney-outbox", 5000)
	seedChannel(t, channelID)
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", userID).Update("used_quota", preConsumed).Error)
	require.NoError(t, model.DB.Model(&model.Channel{}).Where("id = ?", channelID).Update("used_quota", preConsumed).Error)

	task := &model.Midjourney{
		UserId: userID, MjId: "mj-outbox-refund", Action: "IMAGINE", Status: "IN_PROGRESS",
		Progress: "50%", ChannelId: channelID, Quota: preConsumed, TokenId: tokenID,
		Group: "default", BillingSource: BillingSourceWallet,
	}
	require.NoError(t, model.DB.Create(task).Error)
	stale := *task
	task.Status = "FAILURE"
	task.Progress = "100%"
	task.FailReason = "upstream failed"
	won, err := FinalizeMidjourneyTransition(ctx, task, "IN_PROGRESS", true, task.FailReason)
	require.NoError(t, err)
	require.True(t, won)

	stale.Status = "FAILURE"
	stale.Progress = "100%"
	stale.FailReason = "duplicate callback"
	won, err = FinalizeMidjourneyTransition(ctx, &stale, "IN_PROGRESS", true, stale.FailReason)
	require.NoError(t, err)
	assert.False(t, won)

	assert.Equal(t, initialQuota+preConsumed, getUserQuota(t, userID))
	assert.Equal(t, 5000+preConsumed, getTokenRemainQuota(t, tokenID))
	assert.Equal(t, int64(1), countLogs(t))
	var operations int64
	require.NoError(t, model.DB.Model(&model.TaskBillingOperation{}).
		Where("operation_key = ?", "midjourney:mj-outbox-refund:refund").Count(&operations).Error)
	assert.Equal(t, int64(1), operations)
}
