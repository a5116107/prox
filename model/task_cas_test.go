package model

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestMain(m *testing.M) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		panic("failed to open test db: " + err.Error())
	}
	DB = db
	LOG_DB = db

	common.UsingSQLite = true
	common.RedisEnabled = false
	common.BatchUpdateEnabled = false
	common.LogConsumeEnabled = true
	initCol()

	sqlDB, err := db.DB()
	if err != nil {
		panic("failed to get sql.DB: " + err.Error())
	}
	sqlDB.SetMaxOpenConns(1)

	if err := db.AutoMigrate(
		&Task{},
		&TaskBillingOperation{},
		&Midjourney{},
		&User{},
		&Token{},
		&Log{},
		&Channel{},
		&Ability{},
		&TopUp{},
		&SubscriptionPlan{},
		&SubscriptionOrder{},
		&UserSubscription{},
		&PerfMetric{},
		&UserQuotaTransaction{},
		&SubscriptionQuotaTransaction{},
	); err != nil {
		panic("failed to migrate: " + err.Error())
	}

	os.Exit(m.Run())
}

func truncateTables(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		DB.Exec("DELETE FROM tasks")
		DB.Exec("DELETE FROM task_billing_operations")
		DB.Exec("DELETE FROM midjourneys")
		DB.Exec("DELETE FROM users")
		DB.Exec("DELETE FROM tokens")
		DB.Exec("DELETE FROM logs")
		DB.Exec("DELETE FROM channels")
		DB.Exec("DELETE FROM abilities")
		DB.Exec("DELETE FROM top_ups")
		DB.Exec("DELETE FROM subscription_orders")
		DB.Exec("DELETE FROM subscription_plans")
		DB.Exec("DELETE FROM user_subscriptions")
		DB.Exec("DELETE FROM perf_metrics")
		DB.Exec("DELETE FROM user_quota_transactions")
		DB.Exec("DELETE FROM subscription_quota_transactions")
	})
}

func insertTask(t *testing.T, task *Task) {
	t.Helper()
	task.CreatedAt = time.Now().Unix()
	task.UpdatedAt = time.Now().Unix()
	require.NoError(t, DB.Create(task).Error)
}

// ---------------------------------------------------------------------------
// Snapshot / Equal — pure logic tests (no DB)
// ---------------------------------------------------------------------------

func TestSnapshotEqual_Same(t *testing.T) {
	s := taskSnapshot{
		Status:     TaskStatusInProgress,
		Progress:   "50%",
		StartTime:  1000,
		FinishTime: 0,
		FailReason: "",
		ResultURL:  "",
		Data:       json.RawMessage(`{"key":"value"}`),
	}
	assert.True(t, s.Equal(s))
}

func TestSnapshotEqual_DifferentStatus(t *testing.T) {
	a := taskSnapshot{Status: TaskStatusInProgress, Data: json.RawMessage(`{}`)}
	b := taskSnapshot{Status: TaskStatusSuccess, Data: json.RawMessage(`{}`)}
	assert.False(t, a.Equal(b))
}

func TestSnapshotEqual_DifferentProgress(t *testing.T) {
	a := taskSnapshot{Status: TaskStatusInProgress, Progress: "30%", Data: json.RawMessage(`{}`)}
	b := taskSnapshot{Status: TaskStatusInProgress, Progress: "60%", Data: json.RawMessage(`{}`)}
	assert.False(t, a.Equal(b))
}

func TestSnapshotEqual_DifferentData(t *testing.T) {
	a := taskSnapshot{Status: TaskStatusInProgress, Data: json.RawMessage(`{"a":1}`)}
	b := taskSnapshot{Status: TaskStatusInProgress, Data: json.RawMessage(`{"a":2}`)}
	assert.False(t, a.Equal(b))
}

func TestSnapshotEqual_NilVsEmpty(t *testing.T) {
	a := taskSnapshot{Status: TaskStatusInProgress, Data: nil}
	b := taskSnapshot{Status: TaskStatusInProgress, Data: json.RawMessage{}}
	// bytes.Equal(nil, []byte{}) == true
	assert.True(t, a.Equal(b))
}

func TestSnapshot_Roundtrip(t *testing.T) {
	task := &Task{
		Status:     TaskStatusInProgress,
		Progress:   "42%",
		StartTime:  1234,
		FinishTime: 5678,
		FailReason: "timeout",
		PrivateData: TaskPrivateData{
			ResultURL: "https://example.com/result.mp4",
		},
		Data: json.RawMessage(`{"model":"test-model"}`),
	}
	snap := task.Snapshot()
	assert.Equal(t, task.Status, snap.Status)
	assert.Equal(t, task.Progress, snap.Progress)
	assert.Equal(t, task.StartTime, snap.StartTime)
	assert.Equal(t, task.FinishTime, snap.FinishTime)
	assert.Equal(t, task.FailReason, snap.FailReason)
	assert.Equal(t, task.PrivateData.ResultURL, snap.ResultURL)
	assert.JSONEq(t, string(task.Data), string(snap.Data))
}

// ---------------------------------------------------------------------------
// UpdateWithStatus CAS — DB integration tests
// ---------------------------------------------------------------------------

func TestUpdateWithStatus_Win(t *testing.T) {
	truncateTables(t)

	task := &Task{
		TaskID:   "task_cas_win",
		Status:   TaskStatusInProgress,
		Progress: "50%",
		Data:     json.RawMessage(`{}`),
	}
	insertTask(t, task)

	task.Status = TaskStatusSuccess
	task.Progress = "100%"
	won, err := task.UpdateWithStatus(TaskStatusInProgress)
	require.NoError(t, err)
	assert.True(t, won)

	var reloaded Task
	require.NoError(t, DB.First(&reloaded, task.ID).Error)
	assert.EqualValues(t, TaskStatusSuccess, reloaded.Status)
	assert.Equal(t, "100%", reloaded.Progress)
}

func TestUpdateWithStatus_Lose(t *testing.T) {
	truncateTables(t)

	task := &Task{
		TaskID: "task_cas_lose",
		Status: TaskStatusFailure,
		Data:   json.RawMessage(`{}`),
	}
	insertTask(t, task)

	task.Status = TaskStatusSuccess
	won, err := task.UpdateWithStatus(TaskStatusInProgress) // wrong fromStatus
	require.NoError(t, err)
	assert.False(t, won)

	var reloaded Task
	require.NoError(t, DB.First(&reloaded, task.ID).Error)
	assert.EqualValues(t, TaskStatusFailure, reloaded.Status) // unchanged
}

func TestUpdateWithStatus_ConcurrentWinner(t *testing.T) {
	truncateTables(t)

	task := &Task{
		TaskID: "task_cas_race",
		Status: TaskStatusInProgress,
		Quota:  1000,
		Data:   json.RawMessage(`{}`),
	}
	insertTask(t, task)

	const goroutines = 5
	wins := make([]bool, goroutines)
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			t := &Task{}
			*t = Task{
				ID:       task.ID,
				TaskID:   task.TaskID,
				Status:   TaskStatusSuccess,
				Progress: "100%",
				Quota:    task.Quota,
				Data:     json.RawMessage(`{}`),
			}
			t.CreatedAt = task.CreatedAt
			t.UpdatedAt = time.Now().Unix()
			won, err := t.UpdateWithStatus(TaskStatusInProgress)
			if err == nil {
				wins[idx] = won
			}
		}(i)
	}
	wg.Wait()

	winCount := 0
	for _, w := range wins {
		if w {
			winCount++
		}
	}
	assert.Equal(t, 1, winCount, "exactly one goroutine should win the CAS")
}

func makeTaskBillingOperation(key string, task *Task) *TaskBillingOperation {
	return &TaskBillingOperation{
		OperationKey: key, TaskKind: "task", TaskRecordId: task.ID, TaskId: task.TaskID,
		Operation: "refund", UserId: task.UserId, TokenId: task.PrivateData.TokenId,
		ChannelId: task.ChannelId, BillingSource: "wallet", PreConsumedQuota: task.Quota,
		ActualQuota: 0, FundingDelta: -task.Quota, UsageDelta: -task.Quota,
		LogType: LogTypeRefund, LogQuota: task.Quota,
	}
}

func TestTransitionTaskWithBillingOperationIsAtomic(t *testing.T) {
	truncateTables(t)
	task := &Task{
		TaskID: "task_atomic_outbox", UserId: 41, ChannelId: 7, Quota: 800,
		Status: TaskStatusInProgress, Progress: "50%", Data: json.RawMessage(`{}`),
	}
	insertTask(t, task)

	fromStatus := task.Status
	task.Status = TaskStatusFailure
	task.Progress = "100%"
	operation := makeTaskBillingOperation("task:task_atomic_outbox:refund", task)
	won, err := TransitionTaskWithBillingOperation(task, fromStatus, operation)
	require.NoError(t, err)
	require.True(t, won)

	var storedTask Task
	require.NoError(t, DB.First(&storedTask, task.ID).Error)
	assert.EqualValues(t, TaskStatusFailure, storedTask.Status)
	var storedOperation TaskBillingOperation
	require.NoError(t, DB.Where("operation_key = ?", operation.OperationKey).First(&storedOperation).Error)
	assert.Equal(t, -800, storedOperation.FundingDelta)
	assert.Equal(t, TaskBillingOperationPending, storedOperation.Status)
}

func TestTransitionTaskWithBillingOperationConflictRollsBackStatus(t *testing.T) {
	truncateTables(t)
	task := &Task{
		TaskID: "task_atomic_conflict", UserId: 42, Quota: 500,
		Status: TaskStatusInProgress, Progress: "50%", Data: json.RawMessage(`{}`),
	}
	insertTask(t, task)
	key := "task:task_atomic_conflict:refund"
	seed := makeTaskBillingOperation(key, task)
	seed.TaskRecordId = task.ID + 99
	require.NoError(t, DB.Create(seed).Error)

	fromStatus := task.Status
	task.Status = TaskStatusFailure
	task.Progress = "100%"
	won, err := TransitionTaskWithBillingOperation(task, fromStatus, makeTaskBillingOperation(key, task))
	require.Error(t, err)
	assert.False(t, won, "a rolled-back transaction must not report a committed CAS win")

	var stored Task
	require.NoError(t, DB.First(&stored, task.ID).Error)
	assert.EqualValues(t, TaskStatusInProgress, stored.Status, "transaction rollback must restore the task status")
}

func TestTransitionMidjourneyWithBillingOperationConflictRollsBackStatus(t *testing.T) {
	truncateTables(t)
	task := &Midjourney{
		MjId: "mj_atomic_conflict", UserId: 44, ChannelId: 9, Quota: 700,
		Status: "IN_PROGRESS", Progress: "50%",
	}
	require.NoError(t, DB.Create(task).Error)
	key := "midjourney:mj_atomic_conflict:refund"
	seed := &TaskBillingOperation{
		OperationKey: key, TaskKind: "midjourney", TaskRecordId: int64(task.Id + 99),
		TaskId: task.MjId, Operation: "refund", UserId: task.UserId, ChannelId: task.ChannelId,
		BillingSource: "wallet", PreConsumedQuota: task.Quota, FundingDelta: -task.Quota,
		UsageDelta: -task.Quota, LogType: LogTypeRefund, LogQuota: task.Quota,
	}
	require.NoError(t, DB.Create(seed).Error)

	task.Status = "FAILURE"
	task.Progress = "100%"
	candidate := *seed
	candidate.Id = 0
	candidate.TaskRecordId = int64(task.Id)
	won, err := TransitionMidjourneyWithBillingOperation(task, "IN_PROGRESS", &candidate)
	require.Error(t, err)
	assert.False(t, won)

	var stored Midjourney
	require.NoError(t, DB.First(&stored, task.Id).Error)
	assert.Equal(t, "IN_PROGRESS", stored.Status)
}

func TestEnsureTaskBillingOperationRejectsImmutableReplayChanges(t *testing.T) {
	truncateTables(t)
	task := &Task{
		TaskID: "task_replay_conflict", UserId: 43, ChannelId: 8, Quota: 600,
		Status: TaskStatusInProgress, Data: json.RawMessage(`{}`),
	}
	insertTask(t, task)

	tests := []struct {
		name   string
		mutate func(*TaskBillingOperation)
	}{
		{name: "group", mutate: func(operation *TaskBillingOperation) { operation.Group = "other" }},
		{name: "model", mutate: func(operation *TaskBillingOperation) { operation.ModelName = "other-model" }},
		{name: "reason", mutate: func(operation *TaskBillingOperation) { operation.Reason = "other reason" }},
		{name: "log metadata", mutate: func(operation *TaskBillingOperation) { operation.LogOther = `{"task_id":"other"}` }},
	}
	for index, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			seed := makeTaskBillingOperation(fmt.Sprintf("task:replay:%d", index), task)
			seed.Group = "default"
			seed.ModelName = "test-model"
			seed.Reason = "upstream failed"
			seed.LogOther = `{"task_id":"task_replay_conflict"}`
			require.NoError(t, DB.Create(seed).Error)

			candidate := *seed
			candidate.Id = 0
			test.mutate(&candidate)
			_, err := EnsureTaskBillingOperation(&candidate)
			require.ErrorContains(t, err, "idempotency conflict")
		})
	}
}

func TestTaskBillingOperationTokenAndUsageStepsAreTransactional(t *testing.T) {
	truncateTables(t)
	user := &User{Id: 43, Username: "task-op-user", Quota: 1000, UsedQuota: 700}
	token := &Token{Id: 43, UserId: user.Id, Key: "task-op-token", RemainQuota: 100, UsedQuota: 900}
	channel := &Channel{Id: 43, Name: "task-op-channel", Key: "key", UsedQuota: 700}
	require.NoError(t, DB.Create(user).Error)
	require.NoError(t, DB.Create(token).Error)
	require.NoError(t, DB.Create(channel).Error)

	operation, err := EnsureTaskBillingOperation(&TaskBillingOperation{
		OperationKey: "task:task-op-steps:refund", TaskKind: "task", TaskRecordId: 1,
		TaskId: "task-op-steps", Operation: "refund", UserId: user.Id, TokenId: token.Id,
		ChannelId: channel.Id, BillingSource: "wallet", PreConsumedQuota: 300,
		FundingDelta: -300, UsageDelta: -300, LogType: LogTypeRefund, LogQuota: 300,
	})
	require.NoError(t, err)
	claimed, won, err := ClaimTaskBillingOperationByKey(operation.OperationKey, 30)
	require.NoError(t, err)
	require.True(t, won)

	require.NoError(t, ApplyTaskBillingTokenStep(claimed.Id, claimed.LeaseToken))
	require.NoError(t, ApplyTaskBillingTokenStep(claimed.Id, claimed.LeaseToken))
	require.NoError(t, ApplyTaskBillingUsageStep(claimed.Id, claimed.LeaseToken))
	require.NoError(t, ApplyTaskBillingUsageStep(claimed.Id, claimed.LeaseToken))

	var storedToken Token
	require.NoError(t, DB.First(&storedToken, token.Id).Error)
	assert.Equal(t, 400, storedToken.RemainQuota)
	assert.Equal(t, 600, storedToken.UsedQuota)
	var storedUser User
	require.NoError(t, DB.First(&storedUser, user.Id).Error)
	assert.Equal(t, 400, storedUser.UsedQuota)
	var storedChannel Channel
	require.NoError(t, DB.First(&storedChannel, channel.Id).Error)
	assert.EqualValues(t, 400, storedChannel.UsedQuota)
}
