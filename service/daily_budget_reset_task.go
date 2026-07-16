package service

import (
	"context"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/bytedance/gopkg/util/gopool"
)

const opsDailyBudgetResetTickInterval = time.Minute

const (
	opsDailyBudgetResetJobName  = "ops_daily_budget_reset"
	opsDailyBudgetResetLeaseTTL = 45 * time.Second
)

var (
	opsDailyBudgetResetOnce    sync.Once
	opsDailyBudgetResetRunning atomic.Bool
)

func StartOpsDailyBudgetResetTask() {
	opsDailyBudgetResetOnce.Do(func() {
		gopool.Go(func() {
			logger.LogInfo(context.Background(), fmt.Sprintf("operations daily budget reset task started: tick=%s", opsDailyBudgetResetTickInterval))
			runOpsDailyBudgetResetOnce()

			ticker := time.NewTicker(opsDailyBudgetResetTickInterval)
			defer ticker.Stop()
			for range ticker.C {
				runOpsDailyBudgetResetOnce()
			}
		})
	})
}

func runOpsDailyBudgetResetOnce() {
	if !opsDailyBudgetResetRunning.CompareAndSwap(false, true) {
		return
	}
	defer opsDailyBudgetResetRunning.Store(false)

	siteID := AgentSiteID()
	runKey := model.AgentBusinessDateAt(time.Now())
	ownerID := opsDailyBudgetResetOwnerID()
	execution, claimed, err := model.ClaimScheduledJobExecution(
		siteID, opsDailyBudgetResetJobName, runKey, ownerID, opsDailyBudgetResetLeaseTTL,
	)
	if err != nil {
		logger.LogWarn(context.Background(), fmt.Sprintf("operations daily budget reset lease failed: %v", err))
		return
	}
	if !claimed {
		return
	}

	result, err := EnsureOpsDailyBudgetCapacity()
	if err != nil {
		_, _ = model.FailScheduledJobExecution(execution.Id, ownerID, execution.FencingToken, err.Error())
		logger.LogWarn(context.Background(), fmt.Sprintf("operations daily budget reset failed: %v", err))
		return
	}
	completed, completeErr := model.CompleteScheduledJobExecution(execution.Id, ownerID, execution.FencingToken)
	if completeErr != nil || !completed {
		logger.LogWarn(context.Background(), fmt.Sprintf("operations daily budget reset completion lost: completed=%t err=%v", completed, completeErr))
		return
	}
	if common.DebugEnabled {
		logger.LogDebug(
			context.Background(),
			"operations daily budget ready: site=%s date=%s pools=%d fund_minimum=%d",
			result.SiteID,
			result.BudgetDate,
			len(result.RestoredPoolTypes),
			result.FundMinimumQuota,
		)
	}
}

func opsDailyBudgetResetOwnerID() string {
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "new-api"
	}
	return fmt.Sprintf("%s:%d", hostname, os.Getpid())
}
