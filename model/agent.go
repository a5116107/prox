package model

import (
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type AgentConfig struct {
	Id        int    `json:"id" gorm:"primaryKey"`
	SiteId    string `json:"site_id" gorm:"type:varchar(64);not null;index;uniqueIndex:ux_agent_config"`
	Key       string `json:"key" gorm:"type:varchar(128);not null;index;uniqueIndex:ux_agent_config"`
	Value     string `json:"value" gorm:"type:text"`
	ValueType string `json:"value_type" gorm:"type:varchar(32);not null;default:'string'"`
	Remark    string `json:"remark" gorm:"type:text"`
	UpdatedAt int64  `json:"updated_at" gorm:"autoUpdateTime"`
	CreatedAt int64  `json:"created_at" gorm:"autoCreateTime"`
}

func (AgentConfig) TableName() string { return "agent_configs" }

type AgentEvent struct {
	Id              int    `json:"id" gorm:"primaryKey"`
	SiteId          string `json:"site_id" gorm:"type:varchar(64);not null;index"`
	EventType       string `json:"event_type" gorm:"type:varchar(64);not null;index"`
	Source          string `json:"source" gorm:"type:varchar(64);not null;index"`
	Severity        string `json:"severity" gorm:"type:varchar(32);not null;default:'info';index"`
	Status          string `json:"status" gorm:"type:varchar(32);not null;default:'open';index"`
	ActorType       string `json:"actor_type" gorm:"type:varchar(32);not null;default:''"`
	ActorUserId     int    `json:"actor_user_id" gorm:"index"`
	ActorExternalId string `json:"actor_external_id" gorm:"type:varchar(128);index"`
	Title           string `json:"title" gorm:"type:varchar(255);not null;default:''"`
	PayloadJson     string `json:"payload_json" gorm:"type:text"`
	ResultJson      string `json:"result_json" gorm:"type:text"`
	UpdatedAt       int64  `json:"updated_at" gorm:"autoUpdateTime"`
	CreatedAt       int64  `json:"created_at" gorm:"autoCreateTime;index"`
}

func (AgentEvent) TableName() string { return "agent_events" }

type AgentAction struct {
	Id               int    `json:"id" gorm:"primaryKey"`
	SiteId           string `json:"site_id" gorm:"type:varchar(64);not null;index"`
	ActionType       string `json:"action_type" gorm:"type:varchar(64);not null;index"`
	AgentName        string `json:"agent_name" gorm:"type:varchar(64);not null;default:'director'"`
	TargetType       string `json:"target_type" gorm:"type:varchar(64);not null;default:'';index"`
	TargetId         string `json:"target_id" gorm:"type:varchar(128);not null;default:'';index"`
	UserId           int    `json:"user_id" gorm:"index"`
	RiskLevel        string `json:"risk_level" gorm:"type:varchar(32);not null;default:'low';index"`
	RiskScore        int    `json:"risk_score" gorm:"not null;default:0"`
	QuotaAmount      int    `json:"quota_amount" gorm:"not null;default:0;index"`
	BudgetPool       string `json:"budget_pool" gorm:"type:varchar(32);not null;default:'';index"`
	ApprovalRequired bool   `json:"approval_required" gorm:"not null;index"`
	ApprovalId       int    `json:"approval_id" gorm:"index"`
	Status           string `json:"status" gorm:"type:varchar(32);not null;default:'pending';index"`
	IdempotencyKey   string `json:"idempotency_key" gorm:"type:varchar(128);not null;default:'';index;uniqueIndex:ux_agent_action_idempotency"`
	Reason           string `json:"reason" gorm:"type:text"`
	PayloadJson      string `json:"payload_json" gorm:"type:text"`
	ResultJson       string `json:"result_json" gorm:"type:text"`
	ExecutedAt       int64  `json:"executed_at" gorm:"index"`
	UpdatedAt        int64  `json:"updated_at" gorm:"autoUpdateTime"`
	CreatedAt        int64  `json:"created_at" gorm:"autoCreateTime;index"`
}

func (AgentAction) TableName() string { return "agent_actions" }

type AgentActionApproval struct {
	Id          int    `json:"id" gorm:"primaryKey"`
	SiteId      string `json:"site_id" gorm:"type:varchar(64);not null;index"`
	ActionId    int    `json:"action_id" gorm:"not null;index"`
	Status      string `json:"status" gorm:"type:varchar(32);not null;default:'pending';index"`
	ReviewerId  int    `json:"reviewer_id" gorm:"index"`
	Decision    string `json:"decision" gorm:"type:varchar(32);not null;default:''"`
	Comment     string `json:"comment" gorm:"type:text"`
	RequestedBy string `json:"requested_by" gorm:"type:varchar(64);not null;default:'agent'"`
	DecidedAt   int64  `json:"decided_at" gorm:"index"`
	ExpiresAt   int64  `json:"expires_at" gorm:"index"`
	UpdatedAt   int64  `json:"updated_at" gorm:"autoUpdateTime"`
	CreatedAt   int64  `json:"created_at" gorm:"autoCreateTime;index"`
}

func (AgentActionApproval) TableName() string { return "agent_action_approvals" }

type AgentBudgetPool struct {
	Id          int    `json:"id" gorm:"primaryKey"`
	SiteId      string `json:"site_id" gorm:"type:varchar(64);not null;index;uniqueIndex:ux_agent_budget_pool"`
	PoolType    string `json:"pool_type" gorm:"type:varchar(32);not null;index;uniqueIndex:ux_agent_budget_pool"`
	BudgetDate  string `json:"budget_date" gorm:"type:varchar(16);not null;index;uniqueIndex:ux_agent_budget_pool"`
	TotalQuota  int    `json:"total_quota" gorm:"not null;default:0"`
	UsedQuota   int    `json:"used_quota" gorm:"not null;default:0"`
	FrozenQuota int    `json:"frozen_quota" gorm:"not null;default:0"`
	Status      string `json:"status" gorm:"type:varchar(32);not null;default:'active';index"`
	Remark      string `json:"remark" gorm:"type:text"`
	UpdatedAt   int64  `json:"updated_at" gorm:"autoUpdateTime"`
	CreatedAt   int64  `json:"created_at" gorm:"autoCreateTime"`
}

func (AgentBudgetPool) TableName() string { return "agent_budget_pools" }

var agentBusinessLocation = time.FixedZone("Asia/Shanghai", 8*60*60)

// AgentBusinessDateAt returns the calendar date shared by rewards, games,
// check-ins, risk counters, and operations reporting.
func AgentBusinessDateAt(now time.Time) string {
	return now.In(agentBusinessLocation).Format("2006-01-02")
}

// AgentBusinessDayStartAt returns midnight for the shared business calendar.
// A fixed offset avoids depending on timezone data inside minimal containers.
func AgentBusinessDayStartAt(now time.Time) time.Time {
	local := now.In(agentBusinessLocation)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, agentBusinessLocation)
}

type AgentBudgetTransaction struct {
	Id              int    `json:"id" gorm:"primaryKey"`
	SiteId          string `json:"site_id" gorm:"type:varchar(64);not null;index"`
	PoolId          int    `json:"pool_id" gorm:"not null;index"`
	PoolType        string `json:"pool_type" gorm:"type:varchar(32);not null;index"`
	UserId          int    `json:"user_id" gorm:"index"`
	ActionId        int    `json:"action_id" gorm:"index"`
	SourceType      string `json:"source_type" gorm:"type:varchar(64);not null;default:'';index"`
	TransactionType string `json:"transaction_type" gorm:"type:varchar(32);not null;index"`
	Quota           int    `json:"quota" gorm:"not null"`
	BalanceAfter    int    `json:"balance_after" gorm:"not null;default:0"`
	SettlementId    string `json:"settlement_id" gorm:"type:varchar(64);not null;default:'';index"`
	MutationIndex   int    `json:"mutation_index" gorm:"not null;default:0;index"`
	IdempotencyKey  string `json:"idempotency_key" gorm:"type:varchar(128);not null;default:'';index;uniqueIndex:ux_agent_budget_txn_idempotency"`
	Remark          string `json:"remark" gorm:"type:text"`
	MetadataJson    string `json:"metadata_json" gorm:"type:text"`
	CreatedAt       int64  `json:"created_at" gorm:"autoCreateTime;index"`
}

func (AgentBudgetTransaction) TableName() string { return "agent_budget_transactions" }

type AgentRiskProfile struct {
	Id              int    `json:"id" gorm:"primaryKey"`
	SiteId          string `json:"site_id" gorm:"type:varchar(64);not null;index;uniqueIndex:ux_agent_risk_profile"`
	UserId          int    `json:"user_id" gorm:"not null;index;uniqueIndex:ux_agent_risk_profile"`
	CommunityUserId string `json:"community_user_id" gorm:"type:varchar(128);not null;default:'';index"`
	RiskScore       int    `json:"risk_score" gorm:"not null;default:0;index"`
	RiskLevel       string `json:"risk_level" gorm:"type:varchar(32);not null;default:'low';index"`
	Reason          string `json:"reason" gorm:"type:text"`
	FlagsJson       string `json:"flags_json" gorm:"type:text"`
	LastEventAt     int64  `json:"last_event_at" gorm:"index"`
	UpdatedAt       int64  `json:"updated_at" gorm:"autoUpdateTime"`
	CreatedAt       int64  `json:"created_at" gorm:"autoCreateTime"`
}

func (AgentRiskProfile) TableName() string { return "agent_risk_profiles" }

type AgentRiskEvent struct {
	Id          int    `json:"id" gorm:"primaryKey"`
	SiteId      string `json:"site_id" gorm:"type:varchar(64);not null;index"`
	UserId      int    `json:"user_id" gorm:"index"`
	RiskType    string `json:"risk_type" gorm:"type:varchar(64);not null;index"`
	RiskScore   int    `json:"risk_score" gorm:"not null;default:0"`
	RiskLevel   string `json:"risk_level" gorm:"type:varchar(32);not null;default:'low';index"`
	Source      string `json:"source" gorm:"type:varchar(64);not null;default:'';index"`
	PayloadJson string `json:"payload_json" gorm:"type:text"`
	CreatedAt   int64  `json:"created_at" gorm:"autoCreateTime;index"`
}

func (AgentRiskEvent) TableName() string { return "agent_risk_events" }

type AgentMemory struct {
	Id          int    `json:"id" gorm:"primaryKey"`
	SiteId      string `json:"site_id" gorm:"type:varchar(64);not null;index;uniqueIndex:ux_agent_memory_site"`
	AgentName   string `json:"agent_name" gorm:"type:varchar(64);not null;index;uniqueIndex:ux_agent_memory_site"`
	Scope       string `json:"scope" gorm:"type:varchar(64);not null;index;uniqueIndex:ux_agent_memory_site"`
	MemoryKey   string `json:"memory_key" gorm:"type:varchar(128);not null;index;uniqueIndex:ux_agent_memory_site"`
	MemoryValue string `json:"memory_value" gorm:"type:text"`
	ExpiresAt   int64  `json:"expires_at" gorm:"index"`
	UpdatedAt   int64  `json:"updated_at" gorm:"autoUpdateTime"`
	CreatedAt   int64  `json:"created_at" gorm:"autoCreateTime"`
}

func (AgentMemory) TableName() string { return "agent_memories" }

type AgentMemoryCandidate struct {
	Id             int     `json:"id" gorm:"primaryKey"`
	SiteId         string  `json:"site_id" gorm:"type:varchar(64);not null;index"`
	AgentName      string  `json:"agent_name" gorm:"type:varchar(64);not null;default:'director';index"`
	Source         string  `json:"source" gorm:"type:varchar(32);not null;default:'';index"`
	RoomId         string  `json:"room_id" gorm:"type:varchar(128);not null;default:'';index"`
	UserExternalId string  `json:"user_external_id" gorm:"type:varchar(128);not null;default:'';index"`
	Username       string  `json:"username" gorm:"type:varchar(128);not null;default:''"`
	Category       string  `json:"category" gorm:"type:varchar(32);not null;default:'candidate';index"`
	Scope          string  `json:"scope" gorm:"type:varchar(64);not null;default:'group';index"`
	MemoryKey      string  `json:"memory_key" gorm:"type:varchar(128);not null;default:'';index"`
	Text           string  `json:"text" gorm:"type:text"`
	Reason         string  `json:"reason" gorm:"type:varchar(255);not null;default:''"`
	Confidence     float64 `json:"confidence" gorm:"not null;default:0"`
	Status         string  `json:"status" gorm:"type:varchar(32);not null;default:'open';index"`
	PayloadJson    string  `json:"payload_json" gorm:"type:text"`
	ExpiresAt      int64   `json:"expires_at" gorm:"index"`
	UpdatedAt      int64   `json:"updated_at" gorm:"autoUpdateTime"`
	CreatedAt      int64   `json:"created_at" gorm:"autoCreateTime;index"`
}

func (AgentMemoryCandidate) TableName() string { return "agent_memory_candidates" }

type AgentMemoryEvent struct {
	Id             int    `json:"id" gorm:"primaryKey"`
	SiteId         string `json:"site_id" gorm:"type:varchar(64);not null;index"`
	AgentName      string `json:"agent_name" gorm:"type:varchar(64);not null;default:'director';index"`
	EventType      string `json:"event_type" gorm:"type:varchar(64);not null;index"`
	Category       string `json:"category" gorm:"type:varchar(32);not null;default:'';index"`
	Source         string `json:"source" gorm:"type:varchar(32);not null;default:'';index"`
	RoomId         string `json:"room_id" gorm:"type:varchar(128);not null;default:'';index"`
	UserExternalId string `json:"user_external_id" gorm:"type:varchar(128);not null;default:'';index"`
	MemoryId       int    `json:"memory_id" gorm:"index"`
	CandidateId    int    `json:"candidate_id" gorm:"index"`
	Reason         string `json:"reason" gorm:"type:varchar(255);not null;default:''"`
	PayloadJson    string `json:"payload_json" gorm:"type:text"`
	CreatedAt      int64  `json:"created_at" gorm:"autoCreateTime;index"`
}

func (AgentMemoryEvent) TableName() string { return "agent_memory_events" }

type AgentMemoryPolicy struct {
	Id                  int    `json:"id" gorm:"primaryKey"`
	SiteId              string `json:"site_id" gorm:"type:varchar(64);not null;index;uniqueIndex:ux_agent_memory_policy"`
	Source              string `json:"source" gorm:"type:varchar(32);not null;default:'';index;uniqueIndex:ux_agent_memory_policy"`
	RoomId              string `json:"room_id" gorm:"type:varchar(128);not null;default:'';index;uniqueIndex:ux_agent_memory_policy"`
	AutoCaptureEnabled  bool   `json:"auto_capture_enabled" gorm:"not null;default:true"`
	NoiseTTLSeconds     int    `json:"noise_ttl_seconds" gorm:"not null;default:86400"`
	CandidateTTLSeconds int    `json:"candidate_ttl_seconds" gorm:"not null;default:604800"`
	CoreTTLSeconds      int    `json:"core_ttl_seconds" gorm:"not null;default:0"`
	RiskTTLSeconds      int    `json:"risk_ttl_seconds" gorm:"not null;default:7776000"`
	NoiseSampleRate     int    `json:"noise_sample_rate" gorm:"not null;default:10"`
	UpdatedAt           int64  `json:"updated_at" gorm:"autoUpdateTime"`
	CreatedAt           int64  `json:"created_at" gorm:"autoCreateTime"`
}

func (AgentMemoryPolicy) TableName() string { return "agent_memory_policies" }

type AgentActivityPlan struct {
	Id          int    `json:"id" gorm:"primaryKey"`
	SiteId      string `json:"site_id" gorm:"type:varchar(64);not null;index"`
	PlanType    string `json:"plan_type" gorm:"type:varchar(64);not null;index"`
	Title       string `json:"title" gorm:"type:varchar(255);not null;default:''"`
	Status      string `json:"status" gorm:"type:varchar(32);not null;default:'draft';index"`
	BudgetQuota int    `json:"budget_quota" gorm:"not null;default:0"`
	StartAt     int64  `json:"start_at" gorm:"index"`
	EndAt       int64  `json:"end_at" gorm:"index"`
	RulesJson   string `json:"rules_json" gorm:"type:text"`
	ResultJson  string `json:"result_json" gorm:"type:text"`
	CreatedBy   int    `json:"created_by" gorm:"index"`
	UpdatedAt   int64  `json:"updated_at" gorm:"autoUpdateTime"`
	CreatedAt   int64  `json:"created_at" gorm:"autoCreateTime;index"`
}

func (AgentActivityPlan) TableName() string { return "agent_activity_plans" }

func EnsureAgentBudgetPool(siteId string, poolType string, budgetDate string, totalQuota int) (*AgentBudgetPool, error) {
	siteId = strings.TrimSpace(siteId)
	poolType = strings.TrimSpace(poolType)
	budgetDate = strings.TrimSpace(budgetDate)
	if siteId == "" || poolType == "" || budgetDate == "" {
		return nil, errors.New("invalid agent budget pool key")
	}
	pool := AgentBudgetPool{SiteId: siteId, PoolType: poolType, BudgetDate: budgetDate, TotalQuota: totalQuota, Status: "active"}
	err := DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "site_id"}, {Name: "pool_type"}, {Name: "budget_date"}},
		DoNothing: true,
	}).Create(&pool).Error
	if err != nil {
		return nil, err
	}
	err = DB.Where("site_id = ? AND pool_type = ? AND budget_date = ?", siteId, poolType, budgetDate).First(&pool).Error
	return &pool, err
}

func ListAgentBudgetPools(siteId string, budgetDate string) ([]AgentBudgetPool, error) {
	var pools []AgentBudgetPool
	tx := DB.Where("site_id = ?", strings.TrimSpace(siteId)).Order("budget_date desc, pool_type asc")
	if strings.TrimSpace(budgetDate) != "" {
		tx = tx.Where("budget_date = ?", strings.TrimSpace(budgetDate))
	}
	err := tx.Find(&pools).Error
	return pools, err
}

func ListAgentActions(siteId string, limit int) ([]AgentAction, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var rows []AgentAction
	err := DB.Where("site_id = ?", strings.TrimSpace(siteId)).Order("id desc").Limit(limit).Find(&rows).Error
	return rows, err
}

func ListAgentEvents(siteId string, limit int) ([]AgentEvent, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var rows []AgentEvent
	err := DB.Where("site_id = ?", strings.TrimSpace(siteId)).Order("id desc").Limit(limit).Find(&rows).Error
	return rows, err
}

func ListAgentApprovals(siteId string, status string, limit int) ([]AgentActionApproval, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	tx := DB.Where("site_id = ?", strings.TrimSpace(siteId)).Order("id desc").Limit(limit)
	if strings.TrimSpace(status) != "" {
		tx = tx.Where("status = ?", strings.TrimSpace(status))
	}
	var rows []AgentActionApproval
	err := tx.Find(&rows).Error
	return rows, err
}

func ListAgentRiskProfiles(siteId string, limit int) ([]AgentRiskProfile, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var rows []AgentRiskProfile
	err := DB.Where("site_id = ?", strings.TrimSpace(siteId)).Order("risk_score desc, updated_at desc").Limit(limit).Find(&rows).Error
	return rows, err
}

func UpsertAgentRiskProfile(profile AgentRiskProfile) error {
	return DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "site_id"}, {Name: "user_id"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"community_user_id": profile.CommunityUserId,
			"risk_score":        profile.RiskScore,
			"risk_level":        profile.RiskLevel,
			"reason":            profile.Reason,
			"flags_json":        profile.FlagsJson,
			"last_event_at":     profile.LastEventAt,
			"updated_at":        time.Now().Unix(),
		}),
	}).Create(&profile).Error
}

func CreateAgentActionWithApproval(action *AgentAction, approval *AgentActionApproval) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(action).Error; err != nil {
			return err
		}
		if approval != nil {
			approval.ActionId = action.Id
			if err := tx.Create(approval).Error; err != nil {
				return err
			}
			action.ApprovalId = approval.Id
			return tx.Model(action).Updates(map[string]interface{}{"approval_id": approval.Id, "updated_at": time.Now().Unix()}).Error
		}
		return nil
	})
}
