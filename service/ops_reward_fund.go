package service

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"gorm.io/gorm"
)

type OpsBudgetCapacityRestoreRequest struct {
	PoolTypes []string `json:"pool_types"`
	RequestID string   `json:"request_id"`
	Reason    string   `json:"reason"`
}

type OpsBudgetCapacityRestoreResult struct {
	SiteID             string                   `json:"site_id"`
	BudgetDate         string                   `json:"budget_date"`
	Pools              []*model.AgentBudgetPool `json:"pools"`
	Fund               *model.OpsFundAccount    `json:"fund"`
	FundMinimumQuota   int                      `json:"fund_minimum_quota"`
	RestoredPoolTypes  []string                 `json:"restored_pool_types"`
	IdempotencyRequest string                   `json:"request_id"`
}

type OpsBudgetSettings struct {
	DailyBudgetQuota         int    `json:"daily_budget_quota"`
	GrowthBudgetQuota        int    `json:"growth_budget_quota"`
	ActivityBudgetQuota      int    `json:"activity_budget_quota"`
	GameBudgetQuota          int    `json:"game_budget_quota"`
	OpsCompBudgetQuota       int    `json:"ops_comp_budget_quota"`
	CommunityBudgetQuota     int    `json:"community_budget_quota"`
	DailyBudgetResetEnabled  bool   `json:"daily_budget_reset_enabled"`
	DailyFundResetEnabled    bool   `json:"daily_fund_reset_enabled"`
	OpsFundDailyTargetQuota  int    `json:"ops_fund_daily_target_quota"`
	EffectiveFundTargetQuota int    `json:"effective_fund_target_quota"`
	BudgetTimezone           string `json:"budget_timezone"`
}

type OpsBudgetSettingsUpdateRequest struct {
	DailyBudgetQuota        int    `json:"daily_budget_quota"`
	GrowthBudgetQuota       int    `json:"growth_budget_quota"`
	ActivityBudgetQuota     int    `json:"activity_budget_quota"`
	GameBudgetQuota         int    `json:"game_budget_quota"`
	OpsCompBudgetQuota      int    `json:"ops_comp_budget_quota"`
	CommunityBudgetQuota    int    `json:"community_budget_quota"`
	DailyBudgetResetEnabled bool   `json:"daily_budget_reset_enabled"`
	DailyFundResetEnabled   bool   `json:"daily_fund_reset_enabled"`
	OpsFundDailyTargetQuota int    `json:"ops_fund_daily_target_quota"`
	ApplyToToday            bool   `json:"apply_to_today"`
	RequestID               string `json:"request_id"`
	Reason                  string `json:"reason"`
}

type OpsRewardFundOverview struct {
	SiteID                  string            `json:"site_id"`
	Fund                    map[string]any    `json:"fund"`
	BudgetPoolsToday        []map[string]any  `json:"budget_pools_today"`
	RewardPolicy            map[string]any    `json:"reward_policy"`
	SourceBreakdown         []map[string]any  `json:"source_breakdown"`
	SourceBreakdownToday    []map[string]any  `json:"source_breakdown_today"`
	CommissionAudit         map[string]any    `json:"commission_audit"`
	InviteRewardAudit       map[string]any    `json:"invite_reward_audit"`
	Degradation             map[string]any    `json:"degradation"`
	RecentLedgers           []map[string]any  `json:"recent_ledgers"`
	BudgetSettings          OpsBudgetSettings `json:"budget_settings"`
	EffectiveAvailableQuota int               `json:"effective_available_quota"`
	SourceTables            map[string]string `json:"source_tables"`
	GeneratedAt             int64             `json:"generated_at"`
}

type OpsInviteJourneyOverview struct {
	SiteID        string            `json:"site_id"`
	Funnel        map[string]any    `json:"funnel"`
	StateMachine  []map[string]any  `json:"state_machine"`
	Campaigns     []map[string]any  `json:"campaigns"`
	ClaimStatuses []map[string]any  `json:"claim_statuses"`
	EdgeStatuses  []map[string]any  `json:"edge_statuses"`
	EventTypes    []map[string]any  `json:"event_types"`
	RiskFlags     []map[string]any  `json:"risk_flags"`
	RewardSource  map[string]any    `json:"reward_source"`
	Problems      []map[string]any  `json:"problems"`
	RecentEvents  []map[string]any  `json:"recent_events"`
	RecentClaims  []map[string]any  `json:"recent_claims"`
	SourceTables  map[string]string `json:"source_tables"`
	GeneratedAt   int64             `json:"generated_at"`
}

func RestoreOpsBudgetCapacity(siteID string, request OpsBudgetCapacityRestoreRequest) (*OpsBudgetCapacityRestoreResult, error) {
	resolvedSiteID, err := resolveOpsBudgetSiteID(siteID)
	if err != nil {
		return nil, err
	}
	requestID, reason, err := validateOpsBudgetChangeContext(request.RequestID, request.Reason)
	if err != nil {
		return nil, err
	}

	budgetDate := model.AgentBusinessDateAt(time.Now())
	return ensureOpsBudgetCapacity(
		resolvedSiteID,
		budgetDate,
		request.PoolTypes,
		model.OpsBudgetRestoreSourceAdmin,
		requestID,
		reason,
	)
}

func EnsureOpsDailyBudgetCapacity() (*OpsBudgetCapacityRestoreResult, error) {
	capacity, err := model.EnsureCurrentOpsDailyBudgetCapacity(AgentSiteID())
	if err != nil {
		return nil, err
	}
	return opsBudgetCapacityResult(capacity), nil
}

func ensureOpsBudgetCapacity(siteID string, budgetDate string, poolTypes []string, sourceType string, requestID string, reason string) (*OpsBudgetCapacityRestoreResult, error) {
	var capacity *model.AgentDailyBudgetCapacityResult
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		var err error
		capacity, err = model.EnsureAgentDailyBudgetCapacityTx(
			tx, siteID, budgetDate, poolTypes, sourceType, requestID, reason,
		)
		return err
	})
	if err != nil {
		return nil, err
	}
	return opsBudgetCapacityResult(capacity), nil
}

func opsBudgetCapacityResult(capacity *model.AgentDailyBudgetCapacityResult) *OpsBudgetCapacityRestoreResult {
	if capacity == nil {
		return &OpsBudgetCapacityRestoreResult{}
	}
	return &OpsBudgetCapacityRestoreResult{
		SiteID:             capacity.SiteID,
		BudgetDate:         capacity.BudgetDate,
		Pools:              capacity.Pools,
		Fund:               capacity.Fund,
		FundMinimumQuota:   capacity.FundMinimumQuota,
		RestoredPoolTypes:  capacity.RestoredPoolTypes,
		IdempotencyRequest: capacity.RequestID,
	}
}

func resolveOpsBudgetSiteID(siteID string) (string, error) {
	resolvedSiteID := resolveOpsSiteID(siteID)
	if resolvedSiteID != AgentSiteID() {
		return "", errors.New("budget site does not match this deployment")
	}
	return resolvedSiteID, nil
}

func validateOpsBudgetChangeContext(requestID string, reason string) (string, string, error) {
	requestID = strings.TrimSpace(requestID)
	reason = strings.TrimSpace(reason)
	if requestID == "" {
		return "", "", errors.New("request_id is required")
	}
	if len(requestID) > 96 {
		return "", "", errors.New("request_id is too long")
	}
	if reason == "" {
		return "", "", errors.New("reason is required")
	}
	if len(reason) > 500 {
		return "", "", errors.New("reason is too long")
	}
	return requestID, reason, nil
}

func opsBudgetTargetsFromRequest(request OpsBudgetSettingsUpdateRequest) map[string]int {
	return map[string]int{
		"daily": request.DailyBudgetQuota, "growth": request.GrowthBudgetQuota,
		"activity": request.ActivityBudgetQuota, "game": request.GameBudgetQuota,
		"ops_comp": request.OpsCompBudgetQuota, "community": request.CommunityBudgetQuota,
	}
}

func effectiveOpsFundTarget(explicitTarget int, targets map[string]int) int {
	if explicitTarget > 0 {
		return explicitTarget
	}
	total := 0
	for _, poolType := range []string{"daily", "growth", "activity", "game", "ops_comp", "community"} {
		if targets[poolType] > 0 {
			total += targets[poolType]
		}
	}
	return total
}

func GetOpsBudgetSettings(siteID string) (OpsBudgetSettings, error) {
	if _, err := resolveOpsBudgetSiteID(siteID); err != nil {
		return OpsBudgetSettings{}, err
	}
	cfg := operation_setting.GetAgentSetting()
	targets := map[string]int{
		"daily": cfg.DailyBudgetQuota, "growth": cfg.GrowthBudgetQuota,
		"activity": cfg.ActivityBudgetQuota, "game": cfg.GameBudgetQuota,
		"ops_comp": cfg.OpsCompBudgetQuota, "community": cfg.CommunityBudgetQuota,
	}
	return OpsBudgetSettings{
		DailyBudgetQuota: cfg.DailyBudgetQuota, GrowthBudgetQuota: cfg.GrowthBudgetQuota,
		ActivityBudgetQuota: cfg.ActivityBudgetQuota, GameBudgetQuota: cfg.GameBudgetQuota,
		OpsCompBudgetQuota: cfg.OpsCompBudgetQuota, CommunityBudgetQuota: cfg.CommunityBudgetQuota,
		DailyBudgetResetEnabled: cfg.DailyBudgetResetEnabled,
		DailyFundResetEnabled:   cfg.DailyFundResetEnabled,
		OpsFundDailyTargetQuota: cfg.OpsFundDailyTargetQuota,
		EffectiveFundTargetQuota: effectiveOpsFundTarget(
			cfg.OpsFundDailyTargetQuota, targets,
		),
		BudgetTimezone: "Asia/Shanghai",
	}, nil
}

func applyOpsBudgetSettingsToTodayTx(tx *gorm.DB, siteID string, request OpsBudgetSettingsUpdateRequest, targets map[string]int, fundTarget int) error {
	if tx == nil {
		return errors.New("db transaction is nil")
	}
	requestID, reason, err := validateOpsBudgetChangeContext(request.RequestID, request.Reason)
	if err != nil {
		return err
	}
	budgetDate := model.AgentBusinessDateAt(time.Now())
	if request.DailyBudgetResetEnabled {
		for _, poolType := range []string{"daily", "growth", "activity", "game", "ops_comp", "community"} {
			target := targets[poolType]
			if _, err := model.SetBudgetPoolAvailableCapacityTx(
				tx, siteID, poolType, budgetDate, target,
				requestID+":pool:"+poolType, reason,
			); err != nil {
				return err
			}
		}
	}
	if request.DailyFundResetEnabled {
		if _, err := model.EnsureOpsFundMinimumTx(
			tx, siteID, fundTarget, requestID+":fund", reason,
		); err != nil {
			return err
		}
	}
	return nil
}

func UpdateOpsBudgetSettings(siteID string, request OpsBudgetSettingsUpdateRequest) (*OpsBudgetSettings, error) {
	resolvedSiteID, err := resolveOpsBudgetSiteID(siteID)
	if err != nil {
		return nil, err
	}
	quotas := []int{
		request.DailyBudgetQuota, request.GrowthBudgetQuota, request.ActivityBudgetQuota,
		request.GameBudgetQuota, request.OpsCompBudgetQuota, request.CommunityBudgetQuota,
		request.OpsFundDailyTargetQuota,
	}
	for _, quota := range quotas {
		if quota < 0 {
			return nil, errors.New("budget quota cannot be negative")
		}
	}

	targets := opsBudgetTargetsFromRequest(request)
	configuredPoolTotal := effectiveOpsFundTarget(0, targets)
	if request.DailyBudgetResetEnabled && configuredPoolTotal <= 0 {
		return nil, errors.New("at least one daily budget must be positive when daily reset is enabled")
	}
	fundTarget := effectiveOpsFundTarget(request.OpsFundDailyTargetQuota, targets)
	if request.DailyFundResetEnabled && fundTarget <= 0 {
		return nil, errors.New("operating fund daily target must be positive when daily fund reset is enabled")
	}
	if request.ApplyToToday {
		if !request.DailyBudgetResetEnabled && !request.DailyFundResetEnabled {
			return nil, errors.New("at least one daily reset must be enabled when applying settings to today")
		}
		if _, _, err := validateOpsBudgetChangeContext(request.RequestID, request.Reason); err != nil {
			return nil, err
		}
	}

	values := map[string]string{
		"agent_setting.daily_budget_quota":          strconv.Itoa(request.DailyBudgetQuota),
		"agent_setting.growth_budget_quota":         strconv.Itoa(request.GrowthBudgetQuota),
		"agent_setting.activity_budget_quota":       strconv.Itoa(request.ActivityBudgetQuota),
		"agent_setting.game_budget_quota":           strconv.Itoa(request.GameBudgetQuota),
		"agent_setting.ops_comp_budget_quota":       strconv.Itoa(request.OpsCompBudgetQuota),
		"agent_setting.community_budget_quota":      strconv.Itoa(request.CommunityBudgetQuota),
		"agent_setting.daily_budget_reset_enabled":  strconv.FormatBool(request.DailyBudgetResetEnabled),
		"agent_setting.daily_fund_reset_enabled":    strconv.FormatBool(request.DailyFundResetEnabled),
		"agent_setting.ops_fund_daily_target_quota": strconv.Itoa(request.OpsFundDailyTargetQuota),
	}
	if err := model.UpdateOptionsBulkAtomically(values, func(tx *gorm.DB) error {
		if !request.ApplyToToday {
			return nil
		}
		return applyOpsBudgetSettingsToTodayTx(tx, resolvedSiteID, request, targets, fundTarget)
	}); err != nil {
		return nil, err
	}
	settings, err := GetOpsBudgetSettings(resolvedSiteID)
	return &settings, err
}

func GetOpsRewardFundOverview(siteID string) (*OpsRewardFundOverview, error) {
	resolvedSiteID, err := resolveOpsBudgetSiteID(siteID)
	if err != nil {
		return nil, err
	}
	if _, err := model.EnsureCurrentOpsDailyBudgetCapacity(resolvedSiteID); err != nil {
		return nil, err
	}
	_, _ = opsBackfillInviteClaimLedgerLinks(resolvedSiteID, 500)
	now := time.Now()
	dayStart := model.AgentBusinessDayStartAt(now)

	fund, err := opsFundAccountSnapshot(resolvedSiteID)
	if err != nil {
		return nil, err
	}
	budgetPools, err := opsBudgetPoolsToday(resolvedSiteID, model.AgentBusinessDateAt(now))
	if err != nil {
		return nil, err
	}
	rewardPolicy, err := opsRewardPolicySnapshot(resolvedSiteID)
	if err != nil {
		return nil, err
	}
	commissionAudit := opsCommissionFundAudit(resolvedSiteID)
	inviteAudit := opsInviteRewardFundAudit(resolvedSiteID)
	degradation := opsFundDegradationState(fund, budgetPools, rewardPolicy)
	fundBalance := opsIntAny(fund["balance_quota"])
	if fundBalance < 0 {
		fundBalance = 0
	}
	totalRemaining := 0
	for _, pool := range budgetPools {
		remaining := opsIntAny(pool["remaining_quota"])
		if remaining < 0 {
			remaining = 0
		}
		totalRemaining += remaining
		effective := remaining
		if fundBalance < effective {
			effective = fundBalance
		}
		pool["effective_available_quota"] = effective
		pool["effective_available_usd"] = agentQuotaUSDText64(int64(effective))
	}
	effectiveAvailable := totalRemaining
	if fundBalance < effectiveAvailable {
		effectiveAvailable = fundBalance
	}
	budgetSettings, err := GetOpsBudgetSettings(resolvedSiteID)
	if err != nil {
		return nil, err
	}
	recent, err := opsRecentFundLedgers(resolvedSiteID, 20)
	if err != nil {
		return nil, err
	}

	return &OpsRewardFundOverview{
		SiteID:                  resolvedSiteID,
		Fund:                    fund,
		BudgetPoolsToday:        budgetPools,
		RewardPolicy:            rewardPolicy,
		SourceBreakdown:         agentLedgerSourceBreakdown(resolvedSiteID, 0),
		SourceBreakdownToday:    agentLedgerSourceBreakdown(resolvedSiteID, dayStart.Unix()),
		CommissionAudit:         commissionAudit,
		InviteRewardAudit:       inviteAudit,
		Degradation:             degradation,
		RecentLedgers:           recent,
		BudgetSettings:          budgetSettings,
		EffectiveAvailableQuota: effectiveAvailable,
		SourceTables: map[string]string{
			"fund":                 "ops_fund_accounts",
			"fund_ledger":          "ops_fund_ledgers",
			"budget_pool":          "agent_budget_pools",
			"budget_transaction":   "agent_budget_transactions",
			"group_capability":     "group_chatops_configs",
			"group_game_config":    "group_game_configs",
			"group_metrics":        "group_metrics_daily",
			"game_commission":      "game_commissions",
			"invite_reward_claims": "invite_reward_claims",
		},
		GeneratedAt: time.Now().Unix(),
	}, nil
}

func GetOpsInviteJourneyOverview(siteID string) (*OpsInviteJourneyOverview, error) {
	resolvedSiteID := resolveOpsSiteID(siteID)
	_, _ = opsBackfillInviteClaimLedgerLinks(resolvedSiteID, 500)
	funnel := opsInviteFunnel(resolvedSiteID)
	claimStatuses := opsInviteClaimStatusRows(resolvedSiteID)
	edgeStatuses := opsInviteEdgeStatusRows(resolvedSiteID)
	eventTypes := opsInviteEventTypeRows(resolvedSiteID)
	rewardSource := opsInviteRewardFundAudit(resolvedSiteID)
	problems := opsInviteJourneyProblems(resolvedSiteID)
	recentEvents, err := opsRecentInviteEvents(resolvedSiteID, 20)
	if err != nil {
		return nil, err
	}
	recentClaims, err := opsRecentInviteClaims(resolvedSiteID, 20)
	if err != nil {
		return nil, err
	}
	return &OpsInviteJourneyOverview{
		SiteID:        resolvedSiteID,
		Funnel:        funnel,
		StateMachine:  opsInviteStateMachine(funnel),
		Campaigns:     opsInviteCampaignRows(resolvedSiteID),
		ClaimStatuses: claimStatuses,
		EdgeStatuses:  edgeStatuses,
		EventTypes:    eventTypes,
		RiskFlags:     opsInviteRiskFlagRows(resolvedSiteID),
		RewardSource:  rewardSource,
		Problems:      problems,
		RecentEvents:  recentEvents,
		RecentClaims:  recentClaims,
		SourceTables: map[string]string{
			"campaigns": "invite_campaigns",
			"links":     "invite_links",
			"edges":     "invite_edges",
			"events":    "invite_events",
			"claims":    "invite_reward_claims",
			"risk":      "invite_risk_flags",
			"fund":      "ops_fund_ledgers",
		},
		GeneratedAt: time.Now().Unix(),
	}, nil
}

func opsInviteClaimLedgerIdempotencyKey(siteID string, claim model.InviteRewardClaim) string {
	return fmt.Sprintf("invite:%s:%d:%s:%d", siteID, claim.InviteEdgeId, strings.TrimSpace(claim.RewardStage), claim.RewardUserId)
}

func opsBackfillInviteClaimLedgerLinks(siteID string, limit int) (int64, error) {
	if strings.TrimSpace(siteID) == "" {
		return 0, nil
	}
	if limit <= 0 || limit > 5000 {
		limit = 500
	}
	var claims []model.InviteRewardClaim
	if err := model.DB.
		Where("site_id = ? AND status = ? AND ops_fund_ledger_id = 0", siteID, "paid").
		Order("id asc").
		Limit(limit).
		Find(&claims).Error; err != nil {
		return 0, err
	}
	if len(claims) == 0 {
		return 0, nil
	}
	var updated int64
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		for _, claim := range claims {
			idem := opsInviteClaimLedgerIdempotencyKey(siteID, claim)
			if strings.TrimSpace(idem) == "" {
				continue
			}
			var ledger model.OpsFundLedger
			if err := tx.
				Where("site_id = ? AND idempotency_key = ?", siteID, idem).
				First(&ledger).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					continue
				}
				return err
			}
			if ledger.Id <= 0 {
				continue
			}
			res := tx.Model(&model.InviteRewardClaim{}).
				Where("id = ? AND ops_fund_ledger_id = 0", claim.Id).
				Updates(map[string]any{
					"ops_fund_ledger_id": ledger.Id,
					"updated_at":         time.Now().Unix(),
				})
			if res.Error != nil {
				return res.Error
			}
			updated += res.RowsAffected
		}
		return nil
	})
	return updated, err
}

func opsFundAccountSnapshot(siteID string) (map[string]any, error) {
	var acc model.OpsFundAccount
	err := model.DB.Where("site_id = ? AND fund_type = ?", siteID, "operations").First(&acc).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return map[string]any{
				"exists":        false,
				"fund_type":     "operations",
				"status":        "missing",
				"balance_quota": 0,
				"balance_usd":   agentQuotaUSDText64(0),
			}, nil
		}
		return nil, err
	}
	return map[string]any{
		"exists":        true,
		"id":            acc.Id,
		"fund_type":     acc.FundType,
		"status":        firstOpsNonEmpty(acc.Status, "active"),
		"balance_quota": acc.BalanceQuota,
		"balance_usd":   agentQuotaUSDText64(int64(acc.BalanceQuota)),
		"updated_at":    acc.UpdatedAt,
		"created_at":    acc.CreatedAt,
	}, nil
}

func opsBudgetPoolsToday(siteID string, budgetDate string) ([]map[string]any, error) {
	var rows []model.AgentBudgetPool
	if err := model.DB.Where("site_id = ? AND budget_date = ?", siteID, budgetDate).Order("pool_type asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		remaining := row.TotalQuota - row.UsedQuota - row.FrozenQuota
		state := "ok"
		if strings.TrimSpace(row.Status) != "" && row.Status != "active" {
			state = "paused"
		} else if row.TotalQuota > 0 && remaining <= 0 {
			state = "exhausted"
		} else if row.TotalQuota > 0 && remaining*10 <= row.TotalQuota {
			state = "low"
		}
		out = append(out, map[string]any{
			"id":              row.Id,
			"pool_type":       row.PoolType,
			"budget_date":     row.BudgetDate,
			"total_quota":     row.TotalQuota,
			"used_quota":      row.UsedQuota,
			"frozen_quota":    row.FrozenQuota,
			"remaining_quota": remaining,
			"remaining_usd":   agentQuotaUSDText64(int64(remaining)),
			"status":          firstOpsNonEmpty(row.Status, "active"),
			"degrade_state":   state,
		})
	}
	return out, nil
}

func opsRewardPolicySnapshot(siteID string) (map[string]any, error) {
	matrix, err := GetOpsGroupCapabilityMatrix(siteID, "", "", "")
	if err != nil {
		return nil, err
	}
	totals := opsMapIntAny(matrix.Summary["reward_totals"])
	invitePairQuota := totals["invite_reward_quota"] + totals["invitee_reward_quota"]
	plannedDailyLimit := totals["daily_group_reward_limit"]
	return map[string]any{
		"configured_groups":          matrix.Summary["configured_chatops_groups"],
		"checkin_enabled_groups":     matrix.Summary["checkin_enabled_groups"],
		"invite_enabled_groups":      matrix.Summary["invite_enabled_groups"],
		"budget_pools":               matrix.Summary["budget_pools"],
		"reward_totals":              totals,
		"invite_pair_reward_quota":   invitePairQuota,
		"planned_daily_reward_quota": plannedDailyLimit,
		"soft_floor_quota":           plannedDailyLimit,
		"hard_floor_quota":           invitePairQuota,
	}, nil
}

func opsFundDegradationState(fund map[string]any, pools []map[string]any, rewardPolicy map[string]any) map[string]any {
	balance := int64(opsIntAny(fund["balance_quota"]))
	exists := opsBoolAny(fund["exists"])
	status := opsStringAny(fund["status"])
	plannedDaily := int64(opsIntAny(rewardPolicy["planned_daily_reward_quota"]))
	invitePair := int64(opsIntAny(rewardPolicy["invite_pair_reward_quota"]))
	state := "ok"
	reason := "运营基金和今日预算足够覆盖当前配置。"
	action := "保持当前奖励策略。"
	if !exists {
		state = "hard_stop"
		reason = "运营基金账户还没有创建，所有依赖基金的奖励都缺少真实资金来源。"
		action = "先由管理员完成基金初始化或注资，再开放邀请/签到奖励。"
	} else if status != "" && status != "active" {
		state = "paused"
		reason = "运营基金账户不是 active 状态。"
		action = "检查基金账户状态，恢复 active 后再发放奖励。"
	} else if plannedDaily > 0 && balance <= 0 {
		state = "hard_stop"
		reason = "当前基金余额为 0，但群组仍配置了奖励上限。"
		action = "暂停高额邀请奖励或立即补充运营基金。"
	} else if plannedDaily > 0 && balance < plannedDaily {
		state = "degraded"
		reason = "基金余额低于今日群奖励上限，继续原策略会透支。"
		action = "自动降档邀请奖励、限制社区兜底 Key，或把 claim 标记为待补发。"
	} else if invitePair > 0 && balance < invitePair {
		state = "degraded"
		reason = "基金余额不足以覆盖一组邀请人+被邀请人奖励。"
		action = "降低邀请奖励或只开放社区 Key 引导。"
	}
	for _, pool := range pools {
		if opsStringAny(pool["degrade_state"]) == "exhausted" {
			state = "budget_exhausted"
			reason = fmt.Sprintf("%s 今日预算池已耗尽。", opsStringAny(pool["pool_type"]))
			action = "停止该池继续发奖，或调整该池今日上限。"
			break
		}
	}
	return map[string]any{
		"state":                      state,
		"reason":                     reason,
		"operator_action":            action,
		"balance_quota":              balance,
		"planned_daily_reward_quota": plannedDaily,
		"invite_pair_reward_quota":   invitePair,
	}
}

func opsRecentFundLedgers(siteID string, limit int) ([]map[string]any, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	var rows []model.OpsFundLedger
	if err := model.DB.Where("site_id = ?", siteID).Order("id desc").Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		out = append(out, map[string]any{
			"id":               row.Id,
			"delta_quota":      row.DeltaQuota,
			"delta_usd":        agentQuotaUSDText64(int64(row.DeltaQuota)),
			"balance_after":    row.BalanceAfter,
			"source_type":      row.SourceType,
			"source_pool_type": row.SourcePoolType,
			"user_id":          row.UserId,
			"settlement_id":    row.SettlementId,
			"mutation_index":   row.MutationIndex,
			"idempotency_key":  row.IdempotencyKey,
			"remark":           row.Remark,
			"created_at":       row.CreatedAt,
		})
	}
	return out, nil
}

func opsCommissionFundAudit(siteID string) map[string]any {
	gameAudit := agentGameAudit(siteID)
	type ledgerAgg struct {
		IncomeQuota  int64
		ExpenseQuota int64
		NetQuota     int64
	}
	var agg ledgerAgg
	_ = model.DB.Model(&model.OpsFundLedger{}).Select("COALESCE(SUM(CASE WHEN delta_quota > 0 THEN delta_quota ELSE 0 END), 0) AS income_quota, COALESCE(SUM(CASE WHEN delta_quota < 0 THEN -delta_quota ELSE 0 END), 0) AS expense_quota, COALESCE(SUM(delta_quota), 0) AS net_quota").
		Where("site_id = ? AND source_type LIKE ?", siteID, "game%").Scan(&agg).Error
	gameAudit["fund_income_quota"] = agg.IncomeQuota
	gameAudit["fund_expense_quota"] = agg.ExpenseQuota
	gameAudit["fund_net_quota"] = agg.NetQuota
	gameAudit["fund_net_usd"] = agentQuotaUSDText64(agg.NetQuota)
	gameAudit["explanation"] = "游戏抽佣不是单独充值流水；它体现在游戏结算的收入/支出净额与 game_commissions 审计表中。"
	return gameAudit
}

func opsInviteRewardFundAudit(siteID string) map[string]any {
	inviteAudit := agentInviteAudit(siteID)
	type ledgerAgg struct {
		ExpenseQuota int64
		Count        int64
	}
	var agg ledgerAgg
	_ = model.DB.Model(&model.OpsFundLedger{}).Select("COUNT(*) AS count, COALESCE(SUM(CASE WHEN delta_quota < 0 THEN -delta_quota ELSE 0 END), 0) AS expense_quota").
		Where("site_id = ? AND source_type IN ?", siteID, []string{"invite_reward", "invite_inviter_reward", "invite_invitee_reward"}).Scan(&agg).Error
	var paidQuota int64
	_ = model.DB.Model(&model.InviteRewardClaim{}).Select("COALESCE(SUM(quota), 0)").Where("site_id = ? AND status = ?", siteID, "paid").Scan(&paidQuota).Error
	var paidWithoutLedger int64
	_ = model.DB.Model(&model.InviteRewardClaim{}).Where("site_id = ? AND status = ? AND ops_fund_ledger_id = 0", siteID, "paid").Count(&paidWithoutLedger).Error
	inviteAudit["fund_ledger_count"] = agg.Count
	inviteAudit["fund_expense_quota"] = agg.ExpenseQuota
	inviteAudit["fund_expense_usd"] = agentQuotaUSDText64(agg.ExpenseQuota)
	inviteAudit["paid_claim_quota"] = paidQuota
	inviteAudit["paid_claim_usd"] = agentQuotaUSDText64(paidQuota)
	inviteAudit["paid_without_ledger_count"] = paidWithoutLedger
	return inviteAudit
}

func opsInviteFunnel(siteID string) map[string]any {
	var campaigns, links, edges, verifiedEdges, events, claims, paidClaims, blockedClaims int64
	var paidQuota int64
	_ = model.DB.Model(&model.InviteCampaign{}).Where("site_id = ?", siteID).Count(&campaigns).Error
	_ = model.DB.Model(&model.InviteLink{}).Where("site_id = ?", siteID).Count(&links).Error
	_ = model.DB.Model(&model.InviteEdge{}).Where("site_id = ?", siteID).Count(&edges).Error
	_ = model.DB.Model(&model.InviteEdge{}).Where("site_id = ? AND status IN ?", siteID, []string{"verified", "paid"}).Count(&verifiedEdges).Error
	_ = model.DB.Model(&model.InviteEvent{}).Where("site_id = ?", siteID).Count(&events).Error
	_ = model.DB.Model(&model.InviteRewardClaim{}).Where("site_id = ?", siteID).Count(&claims).Error
	_ = model.DB.Model(&model.InviteRewardClaim{}).Where("site_id = ? AND status = ?", siteID, "paid").Count(&paidClaims).Error
	_ = model.DB.Model(&model.InviteRewardClaim{}).Where("site_id = ? AND status IN ?", siteID, []string{"blocked_membership", "failed", "degraded_fund", "degraded_budget"}).Count(&blockedClaims).Error
	_ = model.DB.Model(&model.InviteRewardClaim{}).Select("COALESCE(SUM(quota), 0)").Where("site_id = ? AND status = ?", siteID, "paid").Scan(&paidQuota).Error
	return map[string]any{
		"campaigns":        campaigns,
		"links":            links,
		"joins":            edges,
		"verified":         verifiedEdges,
		"events":           events,
		"claims":           claims,
		"paid_claims":      paidClaims,
		"blocked_claims":   blockedClaims,
		"paid_quota":       paidQuota,
		"paid_quota_usd":   agentQuotaUSDText64(paidQuota),
		"conversion_notes": "link -> join(edge) -> verified(edge) -> claim -> paid/blocked",
	}
}

func opsInviteStateMachine(funnel map[string]any) []map[string]any {
	return []map[string]any{
		{"step": "link_created", "label": "生成邀请链接", "count": funnel["links"], "next": "invitee_joined"},
		{"step": "invitee_joined", "label": "被邀请人进群/注册", "count": funnel["joins"], "next": "membership_verified"},
		{"step": "membership_verified", "label": "完成绑定与群资格验证", "count": funnel["verified"], "next": "reward_claimed"},
		{"step": "reward_claimed", "label": "生成奖励 claim", "count": funnel["claims"], "next": "paid_or_blocked"},
		{"step": "paid", "label": "基金扣减并发放额度", "count": funnel["paid_claims"], "next": "done"},
		{"step": "blocked", "label": "成员资格/基金/预算拦截", "count": funnel["blocked_claims"], "next": "operator_review"},
	}
}

func opsInviteCampaignRows(siteID string) []map[string]any {
	var rows []model.InviteCampaign
	_ = model.DB.Where("site_id = ?", siteID).Order("id desc").Limit(50).Find(&rows).Error
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		out = append(out, map[string]any{
			"id":              row.Id,
			"campaign_code":   row.CampaignCode,
			"name":            row.Name,
			"source_platform": row.SourcePlatform,
			"source_group_id": row.SourceGroupId,
			"target_platform": row.TargetPlatform,
			"target_group_id": row.TargetGroupId,
			"status":          row.Status,
			"created_at":      row.CreatedAt,
			"updated_at":      row.UpdatedAt,
		})
	}
	return out
}

func opsInviteClaimStatusRows(siteID string) []map[string]any {
	type row struct {
		Status      string
		RewardStage string
		Count       int64
		Quota       int64
	}
	var rows []row
	_ = model.DB.Model(&model.InviteRewardClaim{}).Select("status, reward_stage, COUNT(*) AS count, COALESCE(SUM(quota), 0) AS quota").Where("site_id = ?", siteID).Group("status, reward_stage").Order("status asc, reward_stage asc").Scan(&rows).Error
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		out = append(out, map[string]any{"status": row.Status, "reward_stage": row.RewardStage, "count": row.Count, "quota": row.Quota, "quota_usd": agentQuotaUSDText64(row.Quota)})
	}
	return out
}

func opsInviteEdgeStatusRows(siteID string) []map[string]any {
	type row struct {
		Status string
		Stage  string
		Count  int64
	}
	var rows []row
	_ = model.DB.Model(&model.InviteEdge{}).Select("status, stage, COUNT(*) AS count").Where("site_id = ?", siteID).Group("status, stage").Order("status asc, stage asc").Scan(&rows).Error
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		out = append(out, map[string]any{"status": row.Status, "stage": row.Stage, "count": row.Count})
	}
	return out
}

func opsInviteEventTypeRows(siteID string) []map[string]any {
	type row struct {
		EventType string
		Count     int64
	}
	var rows []row
	_ = model.DB.Model(&model.InviteEvent{}).Select("event_type, COUNT(*) AS count").Where("site_id = ?", siteID).Group("event_type").Order("count desc").Scan(&rows).Error
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		out = append(out, map[string]any{"event_type": row.EventType, "count": row.Count})
	}
	return out
}

func opsInviteRiskFlagRows(siteID string) []map[string]any {
	type row struct {
		Status   string
		Severity string
		Count    int64
	}
	var rows []row
	_ = model.DB.Model(&model.InviteRiskFlag{}).Select("status, severity, COUNT(*) AS count").Where("site_id = ?", siteID).Group("status, severity").Order("count desc").Scan(&rows).Error
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		out = append(out, map[string]any{"status": row.Status, "severity": row.Severity, "count": row.Count})
	}
	return out
}

func opsInviteJourneyProblems(siteID string) []map[string]any {
	out := make([]map[string]any, 0)
	var selfInviteEdges int64
	_ = model.DB.Model(&model.InviteEdge{}).Where("site_id = ? AND inviter_user_id > 0 AND invitee_user_id > 0 AND inviter_user_id = invitee_user_id", siteID).Count(&selfInviteEdges).Error
	if selfInviteEdges > 0 {
		out = append(out, map[string]any{"code": "self_invite_edges", "severity": "warning", "count": selfInviteEdges, "message": "存在邀请人与被邀请人相同的边，需要风控复核。"})
	}
	var paidWithoutLedger int64
	_ = model.DB.Model(&model.InviteRewardClaim{}).Where("site_id = ? AND status = ? AND ops_fund_ledger_id = 0", siteID, "paid").Count(&paidWithoutLedger).Error
	if paidWithoutLedger > 0 {
		out = append(out, map[string]any{"code": "paid_claim_missing_fund_ledger", "severity": "warning", "count": paidWithoutLedger, "message": "存在已发放 claim 没有关联基金流水，影响审计追踪。"})
	}
	var failedClaims int64
	_ = model.DB.Model(&model.InviteRewardClaim{}).Where("site_id = ? AND status IN ?", siteID, []string{"failed", "degraded_fund", "degraded_budget", "blocked_membership"}).Count(&failedClaims).Error
	if failedClaims > 0 {
		out = append(out, map[string]any{"code": "claims_need_operator_review", "severity": "info", "count": failedClaims, "message": "有 claim 未发放，需要按成员资格、预算或基金原因处理。"})
	}
	return out
}

func opsRecentInviteEvents(siteID string, limit int) ([]map[string]any, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	var rows []model.InviteEvent
	if err := model.DB.Where("site_id = ?", siteID).Order("id desc").Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		out = append(out, map[string]any{
			"id":               row.Id,
			"campaign_id":      row.CampaignId,
			"invite_link_id":   row.InviteLinkId,
			"invite_edge_id":   row.InviteEdgeId,
			"event_type":       row.EventType,
			"provider":         row.Provider,
			"external_user_id": row.ExternalUserId,
			"user_id":          row.UserId,
			"group_id":         row.GroupId,
			"created_at":       row.CreatedAt,
		})
	}
	return out, nil
}

func opsRecentInviteClaims(siteID string, limit int) ([]map[string]any, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	var rows []model.InviteRewardClaim
	if err := model.DB.Where("site_id = ?", siteID).Order("id desc").Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		out = append(out, map[string]any{
			"id":                 row.Id,
			"campaign_id":        row.CampaignId,
			"invite_edge_id":     row.InviteEdgeId,
			"reward_stage":       row.RewardStage,
			"reward_user_id":     row.RewardUserId,
			"quota":              row.Quota,
			"quota_usd":          agentQuotaUSDText64(int64(row.Quota)),
			"status":             row.Status,
			"ops_fund_ledger_id": row.OpsFundLedgerId,
			"error":              row.Error,
			"created_at":         row.CreatedAt,
			"updated_at":         row.UpdatedAt,
		})
	}
	return out, nil
}

func opsMapIntAny(v any) map[string]int {
	out := map[string]int{}
	if m, ok := v.(map[string]int); ok {
		for key, value := range m {
			out[key] = value
		}
		return out
	}
	if m, ok := v.(map[string]any); ok {
		for key, value := range m {
			out[key] = opsIntAny(value)
		}
	}
	return out
}
