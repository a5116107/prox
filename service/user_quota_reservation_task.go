package service

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/bytedance/gopkg/util/gopool"
)

const (
	userQuotaReservationTickInterval = time.Minute
	userQuotaReservationBatchSize    = 200
)

var (
	userQuotaReservationTaskOnce sync.Once
	userQuotaReservationRunning  atomic.Bool
)

func StartUserQuotaReservationTask() {
	userQuotaReservationTaskOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}
		gopool.Go(func() {
			logger.LogInfo(context.Background(), fmt.Sprintf(
				"user quota reservation task started: tick=%s", userQuotaReservationTickInterval,
			))
			ticker := time.NewTicker(userQuotaReservationTickInterval)
			defer ticker.Stop()

			runUserQuotaReservationTaskOnce()
			for range ticker.C {
				runUserQuotaReservationTaskOnce()
			}
		})
	})
}

func runUserQuotaReservationTaskOnce() {
	if !userQuotaReservationRunning.CompareAndSwap(false, true) {
		return
	}
	defer userQuotaReservationRunning.Store(false)

	total := 0
	for {
		refunded, err := model.RefundExpiredUserQuotaReservations(time.Now(), userQuotaReservationBatchSize)
		if err != nil {
			logger.LogWarn(context.Background(), fmt.Sprintf("expired user quota reservation refund failed: %v", err))
			return
		}
		total += refunded
		if refunded < userQuotaReservationBatchSize {
			break
		}
	}
	if total > 0 {
		logger.LogInfo(context.Background(), fmt.Sprintf("refunded %d expired user quota reservations", total))
	}
}
