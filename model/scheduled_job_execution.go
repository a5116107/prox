package model

import (
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	ScheduledJobPending   = "pending"
	ScheduledJobRunning   = "running"
	ScheduledJobCompleted = "completed"
)

// ScheduledJobExecution provides durable ownership for one logical scheduler
// run. The fencing token prevents an expired worker from completing a run
// after another node has taken over.
type ScheduledJobExecution struct {
	Id           int    `json:"id" gorm:"primaryKey"`
	SiteId       string `json:"site_id" gorm:"type:varchar(64);not null;uniqueIndex:ux_scheduled_job_execution,priority:1"`
	JobName      string `json:"job_name" gorm:"type:varchar(96);not null;uniqueIndex:ux_scheduled_job_execution,priority:2"`
	RunKey       string `json:"run_key" gorm:"type:varchar(128);not null;uniqueIndex:ux_scheduled_job_execution,priority:3"`
	Status       string `json:"status" gorm:"type:varchar(16);not null;default:'pending';index"`
	OwnerId      string `json:"owner_id" gorm:"type:varchar(192);not null;default:'';index"`
	FencingToken int64  `json:"fencing_token" gorm:"not null;default:1"`
	LeaseUntil   int64  `json:"lease_until" gorm:"not null;default:0;index"`
	Attempts     int    `json:"attempts" gorm:"not null;default:1"`
	LastError    string `json:"last_error" gorm:"type:text"`
	CompletedAt  int64  `json:"completed_at" gorm:"not null;default:0;index"`
	UpdatedAt    int64  `json:"updated_at" gorm:"autoUpdateTime"`
	CreatedAt    int64  `json:"created_at" gorm:"autoCreateTime"`
}

func (ScheduledJobExecution) TableName() string { return "scheduled_job_executions" }

func ClaimScheduledJobExecution(siteID, jobName, runKey, ownerID string, leaseTTL time.Duration) (*ScheduledJobExecution, bool, error) {
	siteID = strings.TrimSpace(siteID)
	jobName = strings.TrimSpace(jobName)
	runKey = strings.TrimSpace(runKey)
	ownerID = strings.TrimSpace(ownerID)
	if siteID == "" || jobName == "" || runKey == "" || ownerID == "" {
		return nil, false, errors.New("invalid scheduled job execution key")
	}
	if leaseTTL < 30*time.Second {
		leaseTTL = 30 * time.Second
	}
	now := time.Now()
	row := ScheduledJobExecution{
		SiteId: siteID, JobName: jobName, RunKey: runKey,
		Status: ScheduledJobRunning, OwnerId: ownerID, FencingToken: 1,
		LeaseUntil: now.Add(leaseTTL).Unix(), Attempts: 1,
	}
	created := DB.Clauses(clause.OnConflict{DoNothing: true}).Create(&row)
	if created.Error != nil {
		return nil, false, created.Error
	}
	if created.RowsAffected > 0 {
		return &row, true, nil
	}

	var existing ScheduledJobExecution
	if err := DB.Where("site_id = ? AND job_name = ? AND run_key = ?", siteID, jobName, runKey).First(&existing).Error; err != nil {
		return nil, false, err
	}
	if existing.Status == ScheduledJobCompleted {
		return &existing, false, nil
	}

	takeover := DB.Model(&ScheduledJobExecution{}).
		Where("id = ? AND fencing_token = ? AND status <> ? AND lease_until <= ?", existing.Id, existing.FencingToken, ScheduledJobCompleted, now.Unix()).
		Updates(map[string]any{
			"status": ScheduledJobRunning, "owner_id": ownerID,
			"fencing_token": gorm.Expr("fencing_token + 1"),
			"lease_until":   now.Add(leaseTTL).Unix(), "attempts": gorm.Expr("attempts + 1"),
			"last_error": "", "updated_at": now.Unix(),
		})
	if takeover.Error != nil {
		return nil, false, takeover.Error
	}
	if takeover.RowsAffected == 0 {
		return &existing, false, nil
	}
	if err := DB.First(&existing, existing.Id).Error; err != nil {
		return nil, false, err
	}
	return &existing, true, nil
}

func CompleteScheduledJobExecution(id int, ownerID string, fencingToken int64) (bool, error) {
	if id <= 0 || strings.TrimSpace(ownerID) == "" || fencingToken <= 0 {
		return false, errors.New("invalid scheduled job completion")
	}
	now := time.Now().Unix()
	result := DB.Model(&ScheduledJobExecution{}).
		Where("id = ? AND owner_id = ? AND fencing_token = ? AND status = ?", id, strings.TrimSpace(ownerID), fencingToken, ScheduledJobRunning).
		Updates(map[string]any{
			"status": ScheduledJobCompleted, "lease_until": 0,
			"completed_at": now, "last_error": "", "updated_at": now,
		})
	return result.RowsAffected > 0, result.Error
}

func FailScheduledJobExecution(id int, ownerID string, fencingToken int64, lastError string) (bool, error) {
	if id <= 0 || strings.TrimSpace(ownerID) == "" || fencingToken <= 0 {
		return false, errors.New("invalid scheduled job failure")
	}
	now := time.Now().Unix()
	result := DB.Model(&ScheduledJobExecution{}).
		Where("id = ? AND owner_id = ? AND fencing_token = ? AND status = ?", id, strings.TrimSpace(ownerID), fencingToken, ScheduledJobRunning).
		Updates(map[string]any{
			"status": ScheduledJobPending, "lease_until": 0,
			"last_error": strings.TrimSpace(lastError), "updated_at": now,
		})
	return result.RowsAffected > 0, result.Error
}
