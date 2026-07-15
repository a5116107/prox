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

const riskControlMaintenanceTickInterval = 30 * time.Minute

var (
	riskControlMaintenanceOnce    sync.Once
	riskControlMaintenanceRunning atomic.Bool
)

func StartRiskControlMaintenanceTask() {
	riskControlMaintenanceOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}
		gopool.Go(func() {
			logger.LogInfo(context.Background(), fmt.Sprintf("risk control maintenance task started: tick=%s", riskControlMaintenanceTickInterval))
			ticker := time.NewTicker(riskControlMaintenanceTickInterval)
			defer ticker.Stop()

			runRiskControlMaintenance()
			for range ticker.C {
				runRiskControlMaintenance()
			}
		})
	})
}

func runRiskControlMaintenance() {
	if !riskControlMaintenanceRunning.CompareAndSwap(false, true) {
		return
	}
	defer riskControlMaintenanceRunning.Store(false)
	RunRiskControlMaintenanceOnce(context.Background())
}
