package model

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	TaskBillingOperationPending    = "pending"
	TaskBillingOperationProcessing = "processing"
	TaskBillingOperationCompleted  = "completed"
)

var ErrTaskBillingOperationLeaseLost = errors.New("task billing operation lease lost")

// TaskBillingOperation is a durable outbox for asynchronous task settlement.
// Every external side effect is either inherently idempotent or paired with a
// step flag in the same database transaction as the affected row.
type TaskBillingOperation struct {
	Id                    int64  `json:"id"`
	OperationKey          string `json:"operation_key" gorm:"type:varchar(128);uniqueIndex"`
	TaskKind              string `json:"task_kind" gorm:"type:varchar(32);index"`
	TaskRecordId          int64  `json:"task_record_id" gorm:"index"`
	TaskId                string `json:"task_id" gorm:"type:varchar(128);index"`
	Operation             string `json:"operation" gorm:"type:varchar(32);index"`
	UserId                int    `json:"user_id" gorm:"index"`
	TokenId               int    `json:"token_id" gorm:"index"`
	ChannelId             int    `json:"channel_id" gorm:"index"`
	SubscriptionId        int    `json:"subscription_id" gorm:"index"`
	BillingSource         string `json:"billing_source" gorm:"type:varchar(32)"`
	Group                 string `json:"group" gorm:"type:varchar(64)"`
	ModelName             string `json:"model_name" gorm:"type:varchar(191)"`
	Reason                string `json:"reason" gorm:"type:text"`
	PreConsumedQuota      int    `json:"pre_consumed_quota"`
	ActualQuota           int    `json:"actual_quota"`
	FundingDelta          int    `json:"funding_delta"`
	UsageDelta            int    `json:"usage_delta"`
	LogType               int    `json:"log_type"`
	LogQuota              int    `json:"log_quota"`
	LogOther              string `json:"log_other" gorm:"type:text"`
	FundingApplied        bool   `json:"funding_applied"`
	TokenApplied          bool   `json:"token_applied"`
	TokenCacheInvalidated bool   `json:"token_cache_invalidated"`
	UsageApplied          bool   `json:"usage_applied"`
	LogApplied            bool   `json:"log_applied"`
	Status                string `json:"status" gorm:"type:varchar(32);index"`
	LeaseToken            string `json:"-" gorm:"type:varchar(64);index"`
	LeaseUntil            int64  `json:"lease_until" gorm:"index"`
	NextAttemptAt         int64  `json:"next_attempt_at" gorm:"index"`
	AttemptCount          int    `json:"attempt_count"`
	LastError             string `json:"last_error" gorm:"type:text"`
	CompletedAt           int64  `json:"completed_at" gorm:"index"`
	CreatedAt             int64  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt             int64  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (TaskBillingOperation) TableName() string { return "task_billing_operations" }

func normalizeTaskBillingOperation(operation *TaskBillingOperation) error {
	if operation == nil {
		return errors.New("task billing operation is nil")
	}
	operation.OperationKey = strings.TrimSpace(operation.OperationKey)
	if operation.OperationKey == "" || len(operation.OperationKey) > 128 {
		return errors.New("task billing operation key is invalid")
	}
	if strings.TrimSpace(operation.TaskKind) == "" || strings.TrimSpace(operation.Operation) == "" {
		return errors.New("task billing operation identity is incomplete")
	}
	if operation.UserId <= 0 || operation.FundingDelta == 0 {
		return errors.New("task billing operation mutation is invalid")
	}
	if operation.Status == "" {
		operation.Status = TaskBillingOperationPending
	}
	if operation.TokenId <= 0 {
		operation.TokenApplied = true
		operation.TokenCacheInvalidated = true
	}
	if operation.UsageDelta == 0 {
		operation.UsageApplied = true
	}
	return nil
}

func validateTaskBillingOperationReplay(existing *TaskBillingOperation, candidate *TaskBillingOperation) error {
	if existing.TaskKind != candidate.TaskKind || existing.TaskRecordId != candidate.TaskRecordId ||
		existing.TaskId != candidate.TaskId || existing.Operation != candidate.Operation ||
		existing.UserId != candidate.UserId || existing.TokenId != candidate.TokenId ||
		existing.ChannelId != candidate.ChannelId || existing.SubscriptionId != candidate.SubscriptionId ||
		existing.BillingSource != candidate.BillingSource || existing.Group != candidate.Group ||
		existing.ModelName != candidate.ModelName || existing.Reason != candidate.Reason ||
		existing.PreConsumedQuota != candidate.PreConsumedQuota ||
		existing.ActualQuota != candidate.ActualQuota || existing.FundingDelta != candidate.FundingDelta ||
		existing.UsageDelta != candidate.UsageDelta || existing.LogType != candidate.LogType ||
		existing.LogQuota != candidate.LogQuota || existing.LogOther != candidate.LogOther {
		return fmt.Errorf("task billing operation idempotency conflict: %s", candidate.OperationKey)
	}
	return nil
}

func createTaskBillingOperationTx(tx *gorm.DB, operation *TaskBillingOperation) error {
	if err := normalizeTaskBillingOperation(operation); err != nil {
		return err
	}
	return tx.Create(operation).Error
}

func EnsureTaskBillingOperation(operation *TaskBillingOperation) (*TaskBillingOperation, error) {
	if err := normalizeTaskBillingOperation(operation); err != nil {
		return nil, err
	}
	result := DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "operation_key"}},
		DoNothing: true,
	}).Create(operation)
	if result.Error != nil {
		return nil, result.Error
	}
	var stored TaskBillingOperation
	if err := DB.Where("operation_key = ?", operation.OperationKey).First(&stored).Error; err != nil {
		return nil, err
	}
	if err := validateTaskBillingOperationReplay(&stored, operation); err != nil {
		return nil, err
	}
	return &stored, nil
}

// TransitionTaskWithBillingOperation commits the terminal task state and its
// billing outbox row atomically. A lost CAS leaves neither change behind.
func TransitionTaskWithBillingOperation(task *Task, fromStatus TaskStatus, operation *TaskBillingOperation) (bool, error) {
	if task == nil || task.ID <= 0 {
		return false, errors.New("task is not persisted")
	}
	won := false
	err := DB.Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&Task{}).Where("id = ? AND status = ?", task.ID, fromStatus).Select("*").Updates(task)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return nil
		}
		won = true
		if operation == nil {
			return nil
		}
		operation.TaskKind = "task"
		operation.TaskRecordId = task.ID
		operation.TaskId = task.TaskID
		return createTaskBillingOperationTx(tx, operation)
	})
	if err != nil {
		return false, err
	}
	return won, err
}

func TransitionMidjourneyWithBillingOperation(task *Midjourney, fromStatus string, operation *TaskBillingOperation) (bool, error) {
	if task == nil || task.Id <= 0 {
		return false, errors.New("midjourney task is not persisted")
	}
	won := false
	err := DB.Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&Midjourney{}).Where("id = ? AND status = ?", task.Id, fromStatus).Select("*").Updates(task)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return nil
		}
		won = true
		if operation == nil {
			return nil
		}
		operation.TaskKind = "midjourney"
		operation.TaskRecordId = int64(task.Id)
		operation.TaskId = task.MjId
		return createTaskBillingOperationTx(tx, operation)
	})
	if err != nil {
		return false, err
	}
	return won, err
}

func taskBillingLeaseToken() string {
	token, err := common.GenerateRandomCharsKey(32)
	if err == nil && token != "" {
		return token
	}
	return fmt.Sprintf("lease-%d", time.Now().UnixNano())
}

func claimTaskBillingOperation(operationKey string, leaseSeconds int64) (*TaskBillingOperation, bool, error) {
	if leaseSeconds <= 0 {
		leaseSeconds = 30
	}
	now := time.Now().Unix()
	leaseToken := taskBillingLeaseToken()
	result := DB.Model(&TaskBillingOperation{}).
		Where("operation_key = ? AND completed_at = 0 AND lease_until <= ? AND next_attempt_at <= ?", operationKey, now, now).
		Updates(map[string]interface{}{
			"status":        TaskBillingOperationProcessing,
			"lease_token":   leaseToken,
			"lease_until":   now + leaseSeconds,
			"attempt_count": gorm.Expr("attempt_count + 1"),
			"last_error":    "",
		})
	if result.Error != nil {
		return nil, false, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, false, nil
	}
	var operation TaskBillingOperation
	if err := DB.Where("operation_key = ? AND lease_token = ?", operationKey, leaseToken).First(&operation).Error; err != nil {
		return nil, false, err
	}
	return &operation, true, nil
}

func ClaimTaskBillingOperationByKey(operationKey string, leaseSeconds int64) (*TaskBillingOperation, bool, error) {
	return claimTaskBillingOperation(strings.TrimSpace(operationKey), leaseSeconds)
}

func ClaimPendingTaskBillingOperations(limit int, leaseSeconds int64) ([]*TaskBillingOperation, error) {
	if limit <= 0 {
		limit = 50
	}
	now := time.Now().Unix()
	var candidates []TaskBillingOperation
	if err := DB.Select("operation_key").
		Where("completed_at = 0 AND lease_until <= ? AND next_attempt_at <= ?", now, now).
		Order("id").Limit(limit * 2).Find(&candidates).Error; err != nil {
		return nil, err
	}
	claimed := make([]*TaskBillingOperation, 0, limit)
	for _, candidate := range candidates {
		operation, won, err := claimTaskBillingOperation(candidate.OperationKey, leaseSeconds)
		if err != nil {
			return claimed, err
		}
		if won {
			claimed = append(claimed, operation)
			if len(claimed) >= limit {
				break
			}
		}
	}
	return claimed, nil
}

func MarkTaskBillingOperationStep(id int64, leaseToken string, column string) error {
	allowed := map[string]bool{
		"funding_applied": true, "token_cache_invalidated": true, "log_applied": true,
	}
	if !allowed[column] {
		return fmt.Errorf("unsupported task billing step: %s", column)
	}
	result := DB.Model(&TaskBillingOperation{}).
		Where("id = ? AND lease_token = ? AND completed_at = 0", id, leaseToken).
		Updates(map[string]interface{}{column: true, "lease_until": time.Now().Add(30 * time.Second).Unix()})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrTaskBillingOperationLeaseLost
	}
	return nil
}

func loadLeasedTaskBillingOperationTx(tx *gorm.DB, id int64, leaseToken string) (*TaskBillingOperation, error) {
	var operation TaskBillingOperation
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&operation, id).Error; err != nil {
		return nil, err
	}
	if operation.CompletedAt != 0 || operation.LeaseToken != leaseToken {
		return nil, ErrTaskBillingOperationLeaseLost
	}
	return &operation, nil
}

// ApplyTaskBillingTokenStep changes token quota and marks the step in one DB transaction.
func ApplyTaskBillingTokenStep(id int64, leaseToken string) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		operation, err := loadLeasedTaskBillingOperationTx(tx, id, leaseToken)
		if err != nil {
			return err
		}
		if operation.TokenApplied {
			return nil
		}
		if operation.TokenId > 0 && operation.FundingDelta != 0 {
			updates := map[string]interface{}{
				"accessed_time": common.GetTimestamp(),
			}
			if operation.FundingDelta > 0 {
				updates["remain_quota"] = gorm.Expr("remain_quota - ?", operation.FundingDelta)
				updates["used_quota"] = gorm.Expr("used_quota + ?", operation.FundingDelta)
			} else {
				refund := -operation.FundingDelta
				updates["remain_quota"] = gorm.Expr("remain_quota + ?", refund)
				updates["used_quota"] = gorm.Expr("used_quota - ?", refund)
			}
			if err := tx.Model(&Token{}).Where("id = ?", operation.TokenId).Updates(updates).Error; err != nil {
				return err
			}
		}
		return tx.Model(&TaskBillingOperation{}).Where("id = ? AND lease_token = ?", id, leaseToken).
			Updates(map[string]interface{}{"token_applied": true, "lease_until": time.Now().Add(30 * time.Second).Unix()}).Error
	})
}

func nonNegativeQuotaExpr(delta int) clause.Expr {
	return gorm.Expr("CASE WHEN used_quota + ? < 0 THEN 0 ELSE used_quota + ? END", delta, delta)
}

// ApplyTaskBillingUsageStep reconciles aggregate usage and marks the step atomically.
func ApplyTaskBillingUsageStep(id int64, leaseToken string) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		operation, err := loadLeasedTaskBillingOperationTx(tx, id, leaseToken)
		if err != nil {
			return err
		}
		if operation.UsageApplied {
			return nil
		}
		if operation.UsageDelta != 0 {
			if err := tx.Model(&User{}).Where("id = ?", operation.UserId).
				Update("used_quota", nonNegativeQuotaExpr(operation.UsageDelta)).Error; err != nil {
				return err
			}
			if operation.ChannelId > 0 {
				if err := tx.Model(&Channel{}).Where("id = ?", operation.ChannelId).
					Update("used_quota", nonNegativeQuotaExpr(operation.UsageDelta)).Error; err != nil {
					return err
				}
			}
		}
		return tx.Model(&TaskBillingOperation{}).Where("id = ? AND lease_token = ?", id, leaseToken).
			Updates(map[string]interface{}{"usage_applied": true, "lease_until": time.Now().Add(30 * time.Second).Unix()}).Error
	})
}

func CompleteTaskBillingOperation(id int64, leaseToken string) error {
	now := time.Now().Unix()
	result := DB.Model(&TaskBillingOperation{}).
		Where("id = ? AND lease_token = ? AND completed_at = 0", id, leaseToken).
		Where("funding_applied = ? AND token_applied = ? AND token_cache_invalidated = ? AND usage_applied = ? AND log_applied = ?", true, true, true, true, true).
		Updates(map[string]interface{}{
			"status": TaskBillingOperationCompleted, "completed_at": now,
			"lease_token": "", "lease_until": int64(0), "next_attempt_at": int64(0), "last_error": "",
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrTaskBillingOperationLeaseLost
	}
	return nil
}

func RetryTaskBillingOperation(id int64, leaseToken string, operationErr error) error {
	message := "task billing operation failed"
	if operationErr != nil {
		message = operationErr.Error()
	}
	var operation TaskBillingOperation
	if err := DB.Select("attempt_count").First(&operation, id).Error; err != nil {
		return err
	}
	delay := int64(5)
	for i := 1; i < operation.AttemptCount && delay < 300; i++ {
		delay *= 2
	}
	if delay > 300 {
		delay = 300
	}
	result := DB.Model(&TaskBillingOperation{}).Where("id = ? AND lease_token = ? AND completed_at = 0", id, leaseToken).
		Updates(map[string]interface{}{
			"status": TaskBillingOperationPending, "lease_token": "", "lease_until": int64(0),
			"next_attempt_at": time.Now().Unix() + delay, "last_error": message,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrTaskBillingOperationLeaseLost
	}
	return nil
}
