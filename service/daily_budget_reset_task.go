package service

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/bytedance/gopkg/util/gopool"
)

const opsDailyBudgetResetTickInterval = time.Minute

var (
	opsDailyBudgetResetOnce    sync.Once
	opsDailyBudgetResetRunning atomic.Bool
)

func StartOpsDailyBudgetResetTask() {
	opsDailyBudgetResetOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}
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

	result, err := EnsureOpsDailyBudgetCapacity()
	if err != nil {
		logger.LogWarn(context.Background(), fmt.Sprintf("operations daily budget reset failed: %v", err))
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
