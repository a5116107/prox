package model

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/setting/operation_setting"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// OpsFundAccount 运营基金账户(每站一个,真实资金唯一真相,不占 users.quota)
type OpsFundAccount struct {
	Id           int    `json:"id" gorm:"primaryKey"`
	SiteId       string `json:"site_id" gorm:"type:varchar(64);not null;index;uniqueIndex:ux_ops_fund_account"`
	FundType     string `json:"fund_type" gorm:"type:varchar(32);not null;default:'operations';uniqueIndex:ux_ops_fund_account"`
	BalanceQuota int    `json:"balance_quota" gorm:"not null;default:0"`
	Status       string `json:"status" gorm:"type:varchar(32);not null;default:'active';index"`
	Remark       string `json:"remark" gorm:"type:text"`
	UpdatedAt    int64  `json:"updated_at" gorm:"autoUpdateTime"`
	CreatedAt    int64  `json:"created_at" gorm:"autoCreateTime"`
}

// OpsFundLedger 基金流水(正=收入/扣用户,负=支出/给用户);settlement_id+mutation_index 可重放单局
type OpsFundLedger struct {
	Id             int    `json:"id" gorm:"primaryKey"`
	SiteId         string `json:"site_id" gorm:"type:varchar(64);not null;index"`
	FundAccountId  int    `json:"fund_account_id" gorm:"not null;index"`
	DeltaQuota     int    `json:"delta_quota" gorm:"not null"`
	BalanceAfter   int    `json:"balance_after" gorm:"not null;default:0"`
	SourceType     string `json:"source_type" gorm:"type:varchar(32);not null;index"`
	SourcePoolType string `json:"source_pool_type" gorm:"type:varchar(32);index"`
	UserId         int    `json:"user_id" gorm:"index"`
	ActionId       int    `json:"action_id" gorm:"index"`
	SettlementId   string `json:"settlement_id" gorm:"type:varchar(64);index"`
	MutationIndex  int    `json:"mutation_index" gorm:"not null;default:0"`
	IdempotencyKey string `json:"idempotency_key" gorm:"type:varchar(128);not null;default:'';uniqueIndex:ux_ops_fund_ledger_idem"`
	Remark         string `json:"remark" gorm:"type:text"`
	MetadataJson   string `json:"metadata_json" gorm:"type:text"`
	CreatedAt      int64  `json:"created_at" gorm:"autoCreateTime;index"`
}

const (
	OpsBudgetRestoreSourceAdmin     = "admin_budget_restore"
	OpsBudgetRestoreSourceSettings  = "admin_budget_settings"
	OpsBudgetRestoreSourceDaily     = "daily_budget_reset"
	OpsBudgetRestoreSourceDailyFund = "daily_fund_reset"
)

var ErrBudgetAdjustmentIdempotencyConflict = errors.New("budget adjustment idempotency key was already used for a different operation")

var agentDailyBudgetPoolTypes = []string{"daily", "growth", "activity", "game", "ops_comp", "community"}

type AgentDailyBudgetCapacityResult struct {
	SiteID            string
	BudgetDate        string
	Pools             []*AgentBudgetPool
	Fund              *OpsFundAccount
	FundMinimumQuota  int
	RestoredPoolTypes []string
	RequestID         string
}

// ensureOpsFundAccountTx 确保基金账户存在并加锁返回
func ensureOpsFundAccountTx(tx *gorm.DB, siteID string, fundType string) (*OpsFundAccount, error) {
	if tx == nil {
		return nil, errors.New("db transaction is nil")
	}
	fundType = strings.TrimSpace(fundType)
	if fundType == "" {
		fundType = "operations"
	}
	acc := OpsFundAccount{SiteId: strings.TrimSpace(siteID), FundType: fundType, BalanceQuota: 0, Status: "active"}
	tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&acc)
	var fa OpsFundAccount
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("site_id = ? AND fund_type = ?", strings.TrimSpace(siteID), fundType).
		First(&fa).Error; err != nil {
		return nil, err
	}
	return &fa, nil
}

// RestoreBudgetPoolCapacityTx restores the selected pool's available quota
// without erasing usage or frozen commitments. The adjustment is auditable and
// idempotent, so operators can safely retry a timed-out request.
func RestoreBudgetPoolCapacityTx(tx *gorm.DB, siteID string, poolType string, budgetDate string, targetAvailable int, idempotencyKey string, remark string) (*AgentBudgetPool, error) {
	return RestoreBudgetPoolCapacityWithSourceTx(
		tx, siteID, poolType, budgetDate, targetAvailable, idempotencyKey, remark, OpsBudgetRestoreSourceAdmin,
	)
}

func RestoreBudgetPoolCapacityWithSourceTx(tx *gorm.DB, siteID string, poolType string, budgetDate string, targetAvailable int, idempotencyKey string, remark string, sourceType string) (*AgentBudgetPool, error) {
	return adjustBudgetPoolAvailableCapacityTx(
		tx, siteID, poolType, budgetDate, targetAvailable,
		idempotencyKey, remark, sourceType, false,
	)
}

// SetBudgetPoolAvailableCapacityTx applies an operator-approved limit to the
// current day. Unlike a restore, it can reduce remaining capacity to zero
// without changing already used or frozen quota.
func SetBudgetPoolAvailableCapacityTx(tx *gorm.DB, siteID string, poolType string, budgetDate string, targetAvailable int, idempotencyKey string, remark string) (*AgentBudgetPool, error) {
	return adjustBudgetPoolAvailableCapacityTx(
		tx, siteID, poolType, budgetDate, targetAvailable,
		idempotencyKey, remark, OpsBudgetRestoreSourceSettings, true,
	)
}

func replayBudgetPoolAdjustmentTx(tx *gorm.DB, replay *AgentBudgetTransaction, siteID string, poolType string, budgetDate string, sourceType string, transactionType string, targetAvailable int, exact bool) (*AgentBudgetPool, error) {
	pool, err := lockAgentBudgetPoolTx(tx, siteID, poolType, budgetDate)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrBudgetAdjustmentIdempotencyConflict
		}
		return nil, err
	}
	if replay == nil || replay.PoolId != pool.Id || replay.SiteId != siteID ||
		replay.PoolType != poolType || replay.SourceType != sourceType ||
		replay.TransactionType != transactionType || (exact && replay.BalanceAfter != targetAvailable) {
		return nil, ErrBudgetAdjustmentIdempotencyConflict
	}
	return pool, nil
}

func adjustBudgetPoolAvailableCapacityTx(tx *gorm.DB, siteID string, poolType string, budgetDate string, targetAvailable int, idempotencyKey string, remark string, sourceType string, exact bool) (*AgentBudgetPool, error) {
	if tx == nil {
		return nil, errors.New("db transaction is nil")
	}
	siteID = CanonicalSiteID(siteID)
	poolType = strings.TrimSpace(poolType)
	budgetDate = strings.TrimSpace(budgetDate)
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if err := validOpsPoolType(poolType); err != nil {
		return nil, err
	}
	if poolType == "" {
		return nil, errors.New("a concrete budget pool is required")
	}
	sourceType, err := validBudgetRestoreSource(sourceType)
	if err != nil {
		return nil, err
	}
	if budgetDate == "" || targetAvailable < 0 || idempotencyKey == "" || (!exact && targetAvailable == 0) {
		return nil, errors.New("budget date, non-negative target available quota, and idempotency key are required")
	}
	transactionType := "capacity_restore"
	if exact {
		transactionType = "capacity_set"
	}

	var replay AgentBudgetTransaction
	if err := tx.Where("idempotency_key = ?", idempotencyKey).First(&replay).Error; err == nil {
		return replayBudgetPoolAdjustmentTx(
			tx, &replay, siteID, poolType, budgetDate,
			sourceType, transactionType, targetAvailable, exact,
		)
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	seed := AgentBudgetPool{SiteId: siteID, PoolType: poolType, BudgetDate: budgetDate, TotalQuota: targetAvailable, Status: "active"}
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&seed).Error; err != nil {
		return nil, err
	}
	pool, err := lockAgentBudgetPoolTx(tx, siteID, poolType, budgetDate)
	if err != nil {
		return nil, err
	}
	available := pool.TotalQuota - pool.UsedQuota - pool.FrozenQuota
	increase := targetAvailable - available
	if !exact && increase < 0 {
		increase = 0
	}

	txn := AgentBudgetTransaction{
		SiteId: pool.SiteId, PoolId: pool.Id, PoolType: pool.PoolType,
		TransactionType: transactionType, SourceType: sourceType,
		Quota: increase, BalanceAfter: available + increase,
		IdempotencyKey: idempotencyKey, Remark: strings.TrimSpace(remark),
	}
	res := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&txn)
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		if err := tx.Where("idempotency_key = ?", idempotencyKey).First(&replay).Error; err != nil {
			return nil, err
		}
		return replayBudgetPoolAdjustmentTx(
			tx, &replay, siteID, poolType, budgetDate,
			sourceType, transactionType, targetAvailable, exact,
		)
	}
	if res.RowsAffected == 1 && increase != 0 {
		if err := tx.Model(&AgentBudgetPool{}).Where("id = ?", pool.Id).Updates(map[string]interface{}{
			"total_quota": gorm.Expr("total_quota + ?", increase),
			"status":      "active",
		}).Error; err != nil {
			return nil, err
		}
	}
	return lockAgentBudgetPoolTx(tx, siteID, poolType, budgetDate)
}

// EnsureOpsFundMinimumTx raises the shared operations fund to the requested
// minimum and records the exact increase. Existing money is never discarded.
func EnsureOpsFundMinimumTx(tx *gorm.DB, siteID string, minimumBalance int, idempotencyKey string, remark string) (*OpsFundAccount, error) {
	return EnsureOpsFundMinimumWithSourceTx(
		tx, siteID, minimumBalance, idempotencyKey, remark, OpsBudgetRestoreSourceAdmin,
	)
}

func EnsureOpsFundMinimumWithSourceTx(tx *gorm.DB, siteID string, minimumBalance int, idempotencyKey string, remark string, sourceType string) (*OpsFundAccount, error) {
	if tx == nil {
		return nil, errors.New("db transaction is nil")
	}
	siteID = CanonicalSiteID(siteID)
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	sourceType, err := validBudgetRestoreSource(sourceType)
	if err != nil {
		return nil, err
	}
	if minimumBalance <= 0 || idempotencyKey == "" {
		return nil, errors.New("minimum balance and idempotency key are required")
	}
	var replay OpsFundLedger
	if err := tx.Where("idempotency_key = ?", idempotencyKey).First(&replay).Error; err == nil {
		return ensureOpsFundAccountTx(tx, siteID, "operations")
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	fund, err := ensureOpsFundAccountTx(tx, siteID, "operations")
	if err != nil {
		return nil, err
	}
	delta := minimumBalance - fund.BalanceQuota
	if delta < 0 {
		delta = 0
	}
	newBalance := fund.BalanceQuota + delta
	ledger := OpsFundLedger{
		SiteId: siteID, FundAccountId: fund.Id, DeltaQuota: delta,
		BalanceAfter: newBalance, SourceType: sourceType,
		IdempotencyKey: idempotencyKey, Remark: strings.TrimSpace(remark),
	}
	res := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&ledger)
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 1 && delta > 0 {
		if err := tx.Model(&OpsFundAccount{}).Where("id = ?", fund.Id).Updates(map[string]interface{}{
			"balance_quota": newBalance,
			"status":        "active",
		}).Error; err != nil {
			return nil, err
		}
	}
	return ensureOpsFundAccountTx(tx, siteID, "operations")
}

func EnabledAgentDailyBudgetPoolTypes() []string {
	enabled := make([]string, 0, len(agentDailyBudgetPoolTypes))
	for _, poolType := range agentDailyBudgetPoolTypes {
		if currentAgentPoolQuota(poolType) > 0 {
			enabled = append(enabled, poolType)
		}
	}
	return enabled
}

func AgentDailyBudgetResetRequestID(siteID string, budgetDate string) string {
	return stableQuotaReplaySettlementID("daily-budget-reset:" + CanonicalSiteID(siteID) + ":" + strings.TrimSpace(budgetDate))
}

func configuredAgentBudgetTargets() map[string]int {
	cfg := operation_setting.GetAgentSetting()
	return map[string]int{
		"daily": cfg.DailyBudgetQuota, "growth": cfg.GrowthBudgetQuota,
		"activity": cfg.ActivityBudgetQuota, "game": cfg.GameBudgetQuota,
		"ops_comp": cfg.OpsCompBudgetQuota, "community": cfg.CommunityBudgetQuota,
	}
}

func configuredOpsFundDailyTarget(targets map[string]int) int {
	cfg := operation_setting.GetAgentSetting()
	if cfg.OpsFundDailyTargetQuota > 0 {
		return cfg.OpsFundDailyTargetQuota
	}
	total := 0
	for _, poolType := range agentDailyBudgetPoolTypes {
		if targets[poolType] > 0 {
			total += targets[poolType]
		}
	}
	return total
}

// EnsureOpsDailyBudgetCapacityForDateTx applies the configured pool and fund
// resets independently. Stable per-day keys make retries idempotent while a
// distinct operator request can still explicitly change today's capacity.
func EnsureOpsDailyBudgetCapacityForDateTx(tx *gorm.DB, siteID string, budgetDate string, requestID string, remark string) (*AgentDailyBudgetCapacityResult, error) {
	if tx == nil {
		return nil, errors.New("db transaction is nil")
	}
	siteID = CanonicalSiteID(siteID)
	budgetDate = strings.TrimSpace(budgetDate)
	requestID = strings.TrimSpace(requestID)
	if budgetDate == "" || requestID == "" {
		return nil, errors.New("budget date and request id are required")
	}

	result := &AgentDailyBudgetCapacityResult{
		SiteID: siteID, BudgetDate: budgetDate, RequestID: requestID,
	}
	cfg := operation_setting.GetAgentSetting()
	if cfg == nil || !cfg.BudgetEnabled || (!cfg.DailyBudgetResetEnabled && !cfg.DailyFundResetEnabled) {
		return result, nil
	}

	targets := configuredAgentBudgetTargets()
	if cfg.DailyBudgetResetEnabled {
		for _, poolType := range agentDailyBudgetPoolTypes {
			target := targets[poolType]
			if target <= 0 {
				continue
			}
			pool, err := RestoreBudgetPoolCapacityWithSourceTx(
				tx, siteID, poolType, budgetDate, target,
				requestID+":pool:"+poolType, remark, OpsBudgetRestoreSourceDaily,
			)
			if err != nil {
				return nil, err
			}
			result.Pools = append(result.Pools, pool)
			result.RestoredPoolTypes = append(result.RestoredPoolTypes, poolType)
		}
	}

	if cfg.DailyFundResetEnabled {
		result.FundMinimumQuota = configuredOpsFundDailyTarget(targets)
		if result.FundMinimumQuota > 0 {
			fund, err := EnsureOpsFundMinimumWithSourceTx(
				tx, siteID, result.FundMinimumQuota,
				requestID+":fund", remark, OpsBudgetRestoreSourceDailyFund,
			)
			if err != nil {
				return nil, err
			}
			result.Fund = fund
		}
	}
	return result, nil
}

func EnsureCurrentOpsDailyBudgetCapacity(siteID string) (*AgentDailyBudgetCapacityResult, error) {
	budgetDate := AgentBusinessDateAt(time.Now())
	var result *AgentDailyBudgetCapacityResult
	err := DB.Transaction(func(tx *gorm.DB) error {
		var err error
		result, err = EnsureOpsDailyBudgetCapacityForDateTx(
			tx,
			siteID,
			budgetDate,
			AgentDailyBudgetResetRequestID(siteID, budgetDate),
			"automatic daily reward budget reset",
		)
		return err
	})
	return result, err
}

func EnsureAgentDailyBudgetCapacityTx(tx *gorm.DB, siteID string, budgetDate string, poolTypes []string, sourceType string, requestID string, remark string) (*AgentDailyBudgetCapacityResult, error) {
	if tx == nil {
		return nil, errors.New("db transaction is nil")
	}
	siteID = CanonicalSiteID(siteID)
	budgetDate = strings.TrimSpace(budgetDate)
	requestID = strings.TrimSpace(requestID)
	if budgetDate == "" || requestID == "" {
		return nil, errors.New("budget date and request id are required")
	}
	sourceType, err := validBudgetRestoreSource(sourceType)
	if err != nil {
		return nil, err
	}

	selected := make([]string, 0, len(poolTypes))
	seen := make(map[string]struct{}, len(poolTypes))
	for _, raw := range poolTypes {
		poolType := strings.TrimSpace(raw)
		if err := validOpsPoolType(poolType); err != nil {
			return nil, err
		}
		if poolType == "" {
			return nil, errors.New("budget pool type is required")
		}
		if _, duplicate := seen[poolType]; duplicate {
			continue
		}
		if currentAgentPoolQuota(poolType) <= 0 {
			return nil, fmt.Errorf("budget pool is disabled or has no configured quota: %s", poolType)
		}
		seen[poolType] = struct{}{}
		selected = append(selected, poolType)
	}
	if len(selected) == 0 {
		return nil, errors.New("at least one enabled budget pool is required")
	}

	result := &AgentDailyBudgetCapacityResult{
		SiteID: siteID, BudgetDate: budgetDate, RestoredPoolTypes: selected, RequestID: requestID,
	}
	for _, poolType := range selected {
		target := currentAgentPoolQuota(poolType)
		pool, err := RestoreBudgetPoolCapacityWithSourceTx(
			tx, siteID, poolType, budgetDate, target,
			requestID+":pool:"+poolType, remark, sourceType,
		)
		if err != nil {
			return nil, err
		}
		result.Pools = append(result.Pools, pool)
		result.FundMinimumQuota += target
	}
	fund, err := EnsureOpsFundMinimumWithSourceTx(
		tx, siteID, result.FundMinimumQuota, requestID+":fund", remark, sourceType,
	)
	if err != nil {
		return nil, err
	}
	result.Fund = fund
	return result, nil
}

func validBudgetRestoreSource(sourceType string) (string, error) {
	sourceType = strings.TrimSpace(sourceType)
	switch sourceType {
	case OpsBudgetRestoreSourceAdmin, OpsBudgetRestoreSourceSettings, OpsBudgetRestoreSourceDaily, OpsBudgetRestoreSourceDailyFund:
		return sourceType, nil
	default:
		return "", errors.New("unsupported budget restore source: " + sourceType)
	}
}

// validOpsPoolType 已知池类型(未知报错,不 silent fallback — V2 硬规则1)
func validOpsPoolType(poolType string) error {
	switch strings.TrimSpace(poolType) {
	case "", "daily":
		return nil
	case "growth", "activity", "game", "ops_comp", "community":
		return nil
	}
	return errors.New("unknown pool type: " + poolType)
}

func quotaReplayFieldsFromMetadata(idempotencyKey string, metadataJson string, defaultMutationIndex int) (string, int, int) {
	settlementID := ""
	mutationIndex := defaultMutationIndex
	actionID := 0
	meta := map[string]any{}
	if strings.TrimSpace(metadataJson) != "" {
		_ = json.Unmarshal([]byte(metadataJson), &meta)
	}
	settlementID = firstQuotaReplayString(meta, "settlement_id", "settlement", "round_key")
	if settlementID == "" {
		settlementID = stableQuotaReplaySettlementID(idempotencyKey)
	}
	if len(settlementID) > 64 {
		settlementID = settlementID[:64]
	}
	if idx, ok := firstQuotaReplayInt(meta, "mutation_index", "settlement_mutation_index"); ok {
		mutationIndex = idx
	}
	if mutationIndex < 0 {
		mutationIndex = 0
	}
	if aid, ok := firstQuotaReplayInt(meta, "agent_action_id", "action_id"); ok {
		actionID = aid
	}
	return settlementID, mutationIndex, actionID
}

func stableQuotaReplaySettlementID(idempotencyKey string) string {
	key := strings.TrimSpace(idempotencyKey)
	if key == "" {
		return ""
	}
	if len(key) <= 64 {
		return key
	}
	sum := sha256.Sum256([]byte(key))
	prefix := key
	if len(prefix) > 47 {
		prefix = prefix[:47]
	}
	return prefix + "-" + hex.EncodeToString(sum[:])[:16]
}

func firstQuotaReplayString(meta map[string]any, keys ...string) string {
	for _, key := range keys {
		v, ok := meta[key]
		if !ok || v == nil {
			continue
		}
		if s := strings.TrimSpace(strings.Trim(strings.TrimSpace(toQuotaReplayString(v)), `"`)); s != "" {
			return s
		}
	}
	return ""
}

func firstQuotaReplayInt(meta map[string]any, keys ...string) (int, bool) {
	for _, key := range keys {
		v, ok := meta[key]
		if !ok || v == nil {
			continue
		}
		switch t := v.(type) {
		case int:
			return t, true
		case int64:
			return int(t), true
		case float64:
			return int(t), true
		case string:
			if n, err := strconv.Atoi(strings.TrimSpace(t)); err == nil {
				return n, true
			}
		}
	}
	return 0, false
}

func toQuotaReplayString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case []byte:
		return string(t)
	default:
		return fmt.Sprint(t)
	}
}

// ApplyQuotaMutationTx 统一额度结算:
//
//	delta>0 给用户: 池可用+基金余额校验; user+delta; pool.used+delta; fund-delta; ledger -delta
//	delta<0 扣用户: 用户余额校验; user-|delta|; fund+|delta|; ledger +|delta|; pool.used 不变(日控财务解耦)
func ApplyQuotaMutationTx(tx *gorm.DB, siteID string, userID int, delta int, poolType string, sourceType string, idempotencyKey string, remark string, actionID int, settlementID string, mutationIndex int, metadataJson string) error {
	if tx == nil {
		return errors.New("db transaction is nil")
	}
	if userID <= 0 {
		return errors.New("user id is empty")
	}
	if delta == 0 {
		return errors.New("delta must be non-zero")
	}
	if err := validOpsPoolType(poolType); err != nil {
		return err
	}
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if idempotencyKey == "" {
		return errors.New("idempotency key is empty")
	}
	poolType = strings.TrimSpace(poolType)
	sourceType = strings.TrimSpace(sourceType)
	if sourceType == "" {
		if delta < 0 {
			sourceType = "game_stake"
		} else {
			sourceType = "reward_grant"
		}
	}
	settlementID = strings.TrimSpace(settlementID)
	if settlementID == "" {
		settlementID = stableQuotaReplaySettlementID(idempotencyKey)
	}
	if len(settlementID) > 64 {
		settlementID = settlementID[:64]
	}
	if mutationIndex < 0 {
		mutationIndex = 0
	}

	_, budgetDate, totalQuota := currentAgentBudgetPoolMeta(poolType)
	if delta > 0 {
		if _, err := ensureCurrentAgentDailyBudgetFundingTx(tx, siteID, budgetDate); err != nil {
			return err
		}
	}
	if _, err := ensureAgentBudgetPoolTx(tx, siteID, poolType, budgetDate, totalQuota); err != nil {
		return err
	}
	pool, err := lockAgentBudgetPoolTx(tx, siteID, poolType, budgetDate)
	if err != nil {
		return err
	}
	fund, err := ensureOpsFundAccountTx(tx, siteID, "operations")
	if err != nil {
		return err
	}
	available := pool.TotalQuota - pool.UsedQuota - pool.FrozenQuota

	// 基金流水方向与用户余额 mutation 对冲：
	//   delta > 0: 给用户发放，基金支出，ledger 为负
	//   delta < 0: 扣用户下注/付款，基金收入，ledger 为正
	ledgerDelta := -delta
	transactionType := "grant"
	budgetBalanceAfter := available
	if delta > 0 {
		budgetBalanceAfter = available - delta
	} else {
		transactionType = "charge"
	}
	budgetTxn := AgentBudgetTransaction{
		SiteId:          pool.SiteId,
		PoolId:          pool.Id,
		PoolType:        pool.PoolType,
		UserId:          userID,
		ActionId:        actionID,
		SourceType:      sourceType,
		TransactionType: transactionType,
		Quota:           delta,
		BalanceAfter:    budgetBalanceAfter,
		SettlementId:    settlementID,
		MutationIndex:   mutationIndex,
		IdempotencyKey:  idempotencyKey,
		Remark:          strings.TrimSpace(remark),
		MetadataJson:    strings.TrimSpace(metadataJson),
	}
	budgetClaimed, err := claimAgentBudgetTransactionTx(tx, &budgetTxn)
	if err != nil {
		return err
	}
	if !budgetClaimed {
		return nil
	}
	ledger := OpsFundLedger{
		SiteId:         siteID,
		FundAccountId:  fund.Id,
		DeltaQuota:     ledgerDelta,
		BalanceAfter:   fund.BalanceQuota,
		SourceType:     sourceType,
		SourcePoolType: poolType,
		UserId:         userID,
		ActionId:       actionID,
		SettlementId:   settlementID,
		MutationIndex:  mutationIndex,
		IdempotencyKey: idempotencyKey,
		Remark:         remark,
		MetadataJson:   metadataJson,
	}
	res := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&ledger)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return nil
	}

	if delta > 0 {
		if available < delta {
			return errors.New("budget pool quota insufficient (daily cap)")
		}
		if fund.BalanceQuota < delta {
			return errors.New("ops fund insufficient")
		}
		if err := ApplyUserQuotaMutationTx(tx, UserQuotaMutation{
			UserID: userID, DeltaQuota: delta, RequestID: idempotencyKey,
			SourceType: sourceType, TransactionType: "ops_grant",
			IdempotencyKey: "user:" + idempotencyKey, Remark: strings.TrimSpace(remark),
		}); err != nil {
			return err
		}
		if err := tx.Model(&AgentBudgetPool{}).Where("id = ?", pool.Id).Update("used_quota", gorm.Expr("used_quota + ?", delta)).Error; err != nil {
			return err
		}
		newBal := fund.BalanceQuota - delta
		if err := tx.Model(&OpsFundAccount{}).Where("id = ?", fund.Id).Update("balance_quota", newBal).Error; err != nil {
			return err
		}
		if err := tx.Model(&OpsFundLedger{}).Where("id = ?", ledger.Id).Update("balance_after", newBal).Error; err != nil {
			return err
		}
	} else {
		abs := -delta
		if err := ApplyUserQuotaMutationTx(tx, UserQuotaMutation{
			UserID: userID, DeltaQuota: -abs, RequestID: idempotencyKey,
			SourceType: sourceType, TransactionType: "ops_charge",
			IdempotencyKey: "user:" + idempotencyKey, Remark: strings.TrimSpace(remark),
		}); err != nil {
			return err
		}
		// pool.used_quota 不变(负向扣款不占日控)
		newBal := fund.BalanceQuota + abs
		if err := tx.Model(&OpsFundAccount{}).Where("id = ?", fund.Id).Update("balance_quota", newBal).Error; err != nil {
			return err
		}
		if err := tx.Model(&OpsFundLedger{}).Where("id = ?", ledger.Id).Update("balance_after", newBal).Error; err != nil {
			return err
		}
	}
	return nil
}

// ApplyQuotaSettlementBatchTx 批量结算(一个事务多条 mutation,任一失败整体回滚)— Phase 2
func ApplyQuotaSettlementBatchTx(tx *gorm.DB, siteID string, settlementID string, mutations []QuotaMutation) error {
	if tx == nil {
		return errors.New("db transaction is nil")
	}
	for i, m := range mutations {
		idem := strings.TrimSpace(m.IdempotencyKey)
		if idem == "" {
			idem = strings.Join([]string{
				strings.TrimSpace(settlementID),
				strconv.Itoa(i),
				strings.TrimSpace(m.SourceType),
				strings.TrimSpace(m.PoolType),
				strconv.Itoa(m.UserID),
			}, "-")
		}
		if err := ApplyQuotaMutationTx(tx, siteID, m.UserID, m.Delta, m.PoolType, m.SourceType, idem, m.Remark, m.ActionID, settlementID, i, m.MetadataJson); err != nil {
			return err
		}
	}
	return nil
}

type QuotaMutation struct {
	UserID         int
	Delta          int
	PoolType       string
	SourceType     string
	IdempotencyKey string
	Remark         string
	ActionID       int
	MetadataJson   string
}
