package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestScheduledJobExecutionClaimsOnceAndCompletesWithFence(t *testing.T) {
	setupRewardLedgerTestDB(t)
	require.NoError(t, DB.AutoMigrate(&ScheduledJobExecution{}))
	require.NoError(t, DB.Exec("DELETE FROM scheduled_job_executions").Error)

	first, claimed, err := ClaimScheduledJobExecution("test-site", "daily-reset", "2026-07-16", "node-a", time.Minute)
	require.NoError(t, err)
	require.True(t, claimed)

	second, claimed, err := ClaimScheduledJobExecution("test-site", "daily-reset", "2026-07-16", "node-b", time.Minute)
	require.NoError(t, err)
	require.False(t, claimed)
	require.Equal(t, first.Id, second.Id)

	completed, err := CompleteScheduledJobExecution(first.Id, "node-a", first.FencingToken)
	require.NoError(t, err)
	require.True(t, completed)

	_, claimed, err = ClaimScheduledJobExecution("test-site", "daily-reset", "2026-07-16", "node-b", time.Minute)
	require.NoError(t, err)
	require.False(t, claimed)
}

func TestScheduledJobExecutionExpiredLeaseUsesFencing(t *testing.T) {
	setupRewardLedgerTestDB(t)
	require.NoError(t, DB.AutoMigrate(&ScheduledJobExecution{}))
	require.NoError(t, DB.Exec("DELETE FROM scheduled_job_executions").Error)

	first, claimed, err := ClaimScheduledJobExecution("test-site", "daily-reset", "2026-07-17", "node-a", time.Minute)
	require.NoError(t, err)
	require.True(t, claimed)
	require.NoError(t, DB.Model(&ScheduledJobExecution{}).Where("id = ?", first.Id).Update("lease_until", time.Now().Add(-time.Minute).Unix()).Error)

	takeover, claimed, err := ClaimScheduledJobExecution("test-site", "daily-reset", "2026-07-17", "node-b", time.Minute)
	require.NoError(t, err)
	require.True(t, claimed)
	require.Greater(t, takeover.FencingToken, first.FencingToken)

	completed, err := CompleteScheduledJobExecution(first.Id, "node-a", first.FencingToken)
	require.NoError(t, err)
	require.False(t, completed)
	completed, err = CompleteScheduledJobExecution(takeover.Id, "node-b", takeover.FencingToken)
	require.NoError(t, err)
	require.True(t, completed)
}

func TestScheduledJobExecutionFailureIsRetryable(t *testing.T) {
	setupRewardLedgerTestDB(t)
	require.NoError(t, DB.AutoMigrate(&ScheduledJobExecution{}))
	require.NoError(t, DB.Exec("DELETE FROM scheduled_job_executions").Error)

	first, claimed, err := ClaimScheduledJobExecution("test-site", "daily-reset", "2026-07-18", "node-a", time.Minute)
	require.NoError(t, err)
	require.True(t, claimed)
	failed, err := FailScheduledJobExecution(first.Id, "node-a", first.FencingToken, "temporary database error")
	require.NoError(t, err)
	require.True(t, failed)

	retry, claimed, err := ClaimScheduledJobExecution("test-site", "daily-reset", "2026-07-18", "node-b", time.Minute)
	require.NoError(t, err)
	require.True(t, claimed)
	require.Greater(t, retry.FencingToken, first.FencingToken)
	require.Equal(t, 2, retry.Attempts)
}
