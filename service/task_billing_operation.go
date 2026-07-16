package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/bytedance/gopkg/util/gopool"
)

const (
	taskBillingOperationRefund = "refund"
	taskBillingOperationSettle = "settle"
	taskBillingLeaseSeconds    = int64(30)
)

func taskBillingOperationKey(task *model.Task, operation string, actualQuota int) string {
	stage := operation
	if operation == taskBillingOperationSettle {
		stage = fmt.Sprintf("recalculate_%d", actualQuota)
	}
	return taskBillingMutationID(task, stage)
}

func newTaskBillingOperation(task *model.Task, operation string, actualQuota int, reason string) *model.TaskBillingOperation {
	preConsumedQuota := task.Quota
	delta := actualQuota - preConsumedQuota
	logType := model.LogTypeConsume
	logQuota := delta
	if operation == taskBillingOperationRefund {
		actualQuota = 0
		delta = -preConsumedQuota
		logType = model.LogTypeRefund
		logQuota = preConsumedQuota
	} else if delta < 0 {
		logType = model.LogTypeRefund
		logQuota = -delta
	}
	if delta == 0 {
		return nil
	}
	billingSource := strings.TrimSpace(task.PrivateData.BillingSource)
	if billingSource == "" {
		billingSource = BillingSourceWallet
	}
	other := taskBillingOther(task)
	other["task_id"] = task.TaskID
	other["reason"] = reason
	other["pre_consumed_quota"] = preConsumedQuota
	other["actual_quota"] = actualQuota
	return &model.TaskBillingOperation{
		OperationKey:     taskBillingOperationKey(task, operation, actualQuota),
		TaskKind:         "task",
		TaskRecordId:     task.ID,
		TaskId:           task.TaskID,
		Operation:        operation,
		UserId:           task.UserId,
		TokenId:          task.PrivateData.TokenId,
		ChannelId:        task.ChannelId,
		SubscriptionId:   task.PrivateData.SubscriptionId,
		BillingSource:    billingSource,
		Group:            task.Group,
		ModelName:        taskModelName(task),
		Reason:           reason,
		PreConsumedQuota: preConsumedQuota,
		ActualQuota:      actualQuota,
		FundingDelta:     delta,
		UsageDelta:       delta,
		LogType:          logType,
		LogQuota:         logQuota,
		LogOther:         common.MapToJsonStr(other),
	}
}

func midjourneyBillingOperationKey(task *model.Midjourney) string {
	taskID := strings.TrimSpace(task.MjId)
	if taskID == "" {
		taskID = fmt.Sprintf("db-%d", task.Id)
	}
	key := "midjourney:" + taskID + ":refund"
	if len(key) > 128 {
		key = "midjourney:" + fmt.Sprintf("%x", common.Sha256Raw([]byte(key)))
	}
	return key
}

func newMidjourneyRefundOperation(task *model.Midjourney, reason string) *model.TaskBillingOperation {
	if task.Quota == 0 {
		return nil
	}
	billingSource := strings.TrimSpace(task.BillingSource)
	if billingSource == "" {
		billingSource = BillingSourceWallet
	}
	other := map[string]interface{}{
		"task_id": task.MjId, "reason": reason,
		"pre_consumed_quota": task.Quota, "actual_quota": 0,
	}
	return &model.TaskBillingOperation{
		OperationKey: midjourneyBillingOperationKey(task), TaskKind: "midjourney",
		TaskRecordId: int64(task.Id), TaskId: task.MjId, Operation: taskBillingOperationRefund,
		UserId: task.UserId, TokenId: task.TokenId, ChannelId: task.ChannelId,
		SubscriptionId: task.SubscriptionId, BillingSource: billingSource, Group: task.Group,
		ModelName: CovertMjpActionToModelName(task.Action), Reason: reason,
		PreConsumedQuota: task.Quota, ActualQuota: 0, FundingDelta: -task.Quota,
		UsageDelta: -task.Quota, LogType: model.LogTypeRefund, LogQuota: task.Quota,
		LogOther: common.MapToJsonStr(other),
	}
}

func applyTaskBillingFunding(operation *model.TaskBillingOperation) error {
	if operation.BillingSource == BillingSourceSubscription && operation.SubscriptionId > 0 {
		_, err := model.PostConsumeUserSubscriptionDeltaIdempotentWithResult(
			operation.SubscriptionId,
			int64(operation.FundingDelta),
			operation.OperationKey,
			"task_"+operation.Operation,
			operation.OperationKey,
		)
		return err
	}
	_, err := model.ApplyUserQuotaMutationWithResult(model.UserQuotaMutation{
		UserID: operation.UserId, DeltaQuota: -operation.FundingDelta,
		RequestID: operation.OperationKey, SourceType: "task_billing",
		TransactionType: "task_" + operation.Operation,
		IdempotencyKey:  operation.OperationKey,
		Remark:          "asynchronous task billing adjustment",
	})
	return err
}

func processClaimedTaskBillingOperation(ctx context.Context, operation *model.TaskBillingOperation) error {
	if !operation.FundingApplied {
		if err := applyTaskBillingFunding(operation); err != nil {
			return fmt.Errorf("apply funding: %w", err)
		}
		if err := model.MarkTaskBillingOperationStep(operation.Id, operation.LeaseToken, "funding_applied"); err != nil {
			return err
		}
		operation.FundingApplied = true
	}
	if !operation.TokenApplied {
		if err := model.ApplyTaskBillingTokenStep(operation.Id, operation.LeaseToken); err != nil {
			return fmt.Errorf("apply token quota: %w", err)
		}
		operation.TokenApplied = true
	}
	if !operation.TokenCacheInvalidated {
		if err := model.InvalidateTokenCacheById(operation.TokenId); err != nil {
			return fmt.Errorf("invalidate token cache: %w", err)
		}
		if err := model.MarkTaskBillingOperationStep(operation.Id, operation.LeaseToken, "token_cache_invalidated"); err != nil {
			return err
		}
		operation.TokenCacheInvalidated = true
	}
	if !operation.UsageApplied {
		if err := model.ApplyTaskBillingUsageStep(operation.Id, operation.LeaseToken); err != nil {
			return fmt.Errorf("apply usage totals: %w", err)
		}
		operation.UsageApplied = true
	}
	if !operation.LogApplied {
		other, err := common.StrToMap(operation.LogOther)
		if err != nil {
			return fmt.Errorf("decode billing log metadata: %w", err)
		}
		_, err = model.RecordTaskBillingLogIdempotent(model.RecordTaskBillingLogParams{
			UserId: operation.UserId, LogType: operation.LogType, Content: operation.Reason,
			ChannelId: operation.ChannelId, ModelName: operation.ModelName, Quota: operation.LogQuota,
			TokenId: operation.TokenId, Group: operation.Group, Other: other,
		}, operation.OperationKey)
		if err != nil {
			return fmt.Errorf("record billing log: %w", err)
		}
		if err := model.MarkTaskBillingOperationStep(operation.Id, operation.LeaseToken, "log_applied"); err != nil {
			return err
		}
		operation.LogApplied = true
	}
	return model.CompleteTaskBillingOperation(operation.Id, operation.LeaseToken)
}

func processClaimedTaskBillingOperationWithRetry(ctx context.Context, operation *model.TaskBillingOperation) error {
	err := processClaimedTaskBillingOperation(ctx, operation)
	if err == nil {
		return nil
	}
	if retryErr := model.RetryTaskBillingOperation(operation.Id, operation.LeaseToken, err); retryErr != nil &&
		!errors.Is(retryErr, model.ErrTaskBillingOperationLeaseLost) {
		return errors.Join(err, retryErr)
	}
	return err
}

func ProcessTaskBillingOperationByKey(ctx context.Context, operationKey string) error {
	operation, claimed, err := model.ClaimTaskBillingOperationByKey(operationKey, taskBillingLeaseSeconds)
	if err != nil || !claimed {
		return err
	}
	return processClaimedTaskBillingOperationWithRetry(ctx, operation)
}

func ProcessPendingTaskBillingOperations(ctx context.Context, limit int) (int, error) {
	operations, err := model.ClaimPendingTaskBillingOperations(limit, taskBillingLeaseSeconds)
	if err != nil {
		return 0, err
	}
	completed := 0
	var operationErrors []error
	for _, operation := range operations {
		if err := processClaimedTaskBillingOperationWithRetry(ctx, operation); err != nil {
			operationErrors = append(operationErrors, fmt.Errorf("%s: %w", operation.OperationKey, err))
			continue
		}
		completed++
	}
	return completed, errors.Join(operationErrors...)
}

func StartTaskBillingOperationTask() {
	gopool.Go(func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			if completed, err := ProcessPendingTaskBillingOperations(context.Background(), 100); err != nil {
				logger.LogWarn(context.Background(), fmt.Sprintf("task billing outbox retry failed: %v", err))
			} else if completed > 0 {
				logger.LogInfo(context.Background(), fmt.Sprintf("completed %d task billing outbox operations", completed))
			}
			<-ticker.C
		}
	})
}

// FinalizeTaskTransition atomically persists a terminal task and the required
// billing operation, then attempts the outbox immediately. Background workers
// resume any unfinished steps after process or node failure.
func FinalizeTaskTransition(ctx context.Context, task *model.Task, fromStatus model.TaskStatus, actualQuota int, reason string) (bool, error) {
	var operation *model.TaskBillingOperation
	switch task.Status {
	case model.TaskStatusFailure:
		operation = newTaskBillingOperation(task, taskBillingOperationRefund, 0, reason)
	case model.TaskStatusSuccess:
		if actualQuota > 0 && actualQuota != task.Quota {
			operation = newTaskBillingOperation(task, taskBillingOperationSettle, actualQuota, reason)
			task.Quota = actualQuota
		}
	}
	won, err := model.TransitionTaskWithBillingOperation(task, fromStatus, operation)
	if err != nil || !won || operation == nil {
		return won, err
	}
	return won, ProcessTaskBillingOperationByKey(ctx, operation.OperationKey)
}

func FinalizeMidjourneyTransition(ctx context.Context, task *model.Midjourney, fromStatus string, refund bool, reason string) (bool, error) {
	var operation *model.TaskBillingOperation
	if refund {
		operation = newMidjourneyRefundOperation(task, reason)
	}
	won, err := model.TransitionMidjourneyWithBillingOperation(task, fromStatus, operation)
	if err != nil || !won || operation == nil {
		return won, err
	}
	return won, ProcessTaskBillingOperationByKey(ctx, operation.OperationKey)
}
