package service

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/require"
)

func TestRestoreOpsBudgetCapacityRestoresSelectedPoolsAndFund(t *testing.T) {
	require.NoError(t, model.DB.AutoMigrate(
		&model.AgentBudgetPool{},
		&model.AgentBudgetTransaction{},
		&model.OpsFundAccount{},
		&model.OpsFundLedger{},
	))
	cleanupOpsRewardFundTestTables(t)

	oldAgent := *operation_setting.GetAgentSetting()
	t.Cleanup(func() { *operation_setting.GetAgentSetting() = oldAgent })
	setting := operation_setting.GetAgentSetting()
	setting.SiteID = "restore-site"
	setting.DailyBudgetQuota = 500_000
	setting.ActivityBudgetQuota = 1_000_000
	setting.GrowthBudgetQuota = 2_000_000

	today := model.AgentBusinessDateAt(time.Now())
	require.NoError(t, model.DB.Create(&model.AgentBudgetPool{
		SiteId: "restore-site", PoolType: "activity", BudgetDate: today,
		TotalQuota: 1_000_000, UsedQuota: 900_000, FrozenQuota: 0, Status: "active",
	}).Error)
	require.NoError(t, model.DB.Create(&model.OpsFundAccount{
		SiteId: "restore-site", FundType: "operations", BalanceQuota: 10_000, Status: "active",
	}).Error)

	request := OpsBudgetCapacityRestoreRequest{
		PoolTypes: []string{"daily", "activity", "growth"},
		RequestID: "restore-request-1",
		Reason:    "restore rewards after operator review",
	}
	result, err := RestoreOpsBudgetCapacity("restore-site", request)
	require.NoError(t, err)
	require.Equal(t, 3_500_000, result.FundMinimumQuota)
	require.Len(t, result.Pools, 3)
	require.Equal(t, 3_500_000, result.Fund.BalanceQuota)

	replayed, err := RestoreOpsBudgetCapacity("restore-site", request)
	require.NoError(t, err)
	require.Equal(t, 3_500_000, replayed.Fund.BalanceQuota)

	var activity model.AgentBudgetPool
	require.NoError(t, model.DB.Where("site_id = ? AND pool_type = ? AND budget_date = ?", "restore-site", "activity", today).First(&activity).Error)
	require.Equal(t, 1_900_000, activity.TotalQuota)
	require.Equal(t, 1_000_000, activity.TotalQuota-activity.UsedQuota-activity.FrozenQuota)

	var budgetAdjustments, fundAdjustments int64
	require.NoError(t, model.DB.Model(&model.AgentBudgetTransaction{}).Where("source_type = ?", "admin_budget_restore").Count(&budgetAdjustments).Error)
	require.NoError(t, model.DB.Model(&model.OpsFundLedger{}).Where("source_type = ?", "admin_budget_restore").Count(&fundAdjustments).Error)
	require.Equal(t, int64(3), budgetAdjustments)
	require.Equal(t, int64(1), fundAdjustments)
}

func TestEnsureOpsDailyBudgetCapacityIsIdempotent(t *testing.T) {
	require.NoError(t, model.DB.AutoMigrate(
		&model.AgentBudgetPool{},
		&model.AgentBudgetTransaction{},
		&model.OpsFundAccount{},
		&model.OpsFundLedger{},
	))
	cleanupOpsRewardFundTestTables(t)

	oldAgent := *operation_setting.GetAgentSetting()
	t.Cleanup(func() { *operation_setting.GetAgentSetting() = oldAgent })
	setting := operation_setting.GetAgentSetting()
	setting.SiteID = "daily-reset-site"
	setting.DailyBudgetQuota = 100_000
	setting.GrowthBudgetQuota = 200_000
	setting.ActivityBudgetQuota = 1_000_000
	setting.GameBudgetQuota = 300_000
	setting.OpsCompBudgetQuota = 400_000
	setting.CommunityBudgetQuota = 500_000
	setting.DailyBudgetResetEnabled = true
	setting.DailyFundResetEnabled = true
	setting.OpsFundDailyTargetQuota = 0

	today := model.AgentBusinessDateAt(time.Now())
	require.NoError(t, model.DB.Create(&model.AgentBudgetPool{
		SiteId: "daily-reset-site", PoolType: "activity", BudgetDate: today,
		TotalQuota: 1_000_000, UsedQuota: 900_000, Status: "active",
	}).Error)
	require.NoError(t, model.DB.Create(&model.OpsFundAccount{
		SiteId: "daily-reset-site", FundType: "operations", BalanceQuota: 811, Status: "active",
	}).Error)

	first, err := EnsureOpsDailyBudgetCapacity()
	require.NoError(t, err)
	require.Equal(t, 2_500_000, first.FundMinimumQuota)
	require.Equal(t, 2_500_000, first.Fund.BalanceQuota)
	require.ElementsMatch(t, []string{"daily", "growth", "activity", "game", "ops_comp", "community"}, first.RestoredPoolTypes)

	replayed, err := EnsureOpsDailyBudgetCapacity()
	require.NoError(t, err)
	require.Equal(t, 2_500_000, replayed.Fund.BalanceQuota)

	var activity model.AgentBudgetPool
	require.NoError(t, model.DB.Where("site_id = ? AND pool_type = ? AND budget_date = ?", "daily-reset-site", "activity", today).First(&activity).Error)
	require.Equal(t, 1_000_000, activity.TotalQuota-activity.UsedQuota-activity.FrozenQuota)

	var fundResetCount int64
	require.NoError(t, model.DB.Model(&model.OpsFundLedger{}).Where("source_type = ?", model.OpsBudgetRestoreSourceDailyFund).Count(&fundResetCount).Error)
	require.Equal(t, int64(1), fundResetCount)
}

func TestEnsureAgentRuntimeDefaultsPreservesDailyRestoredCapacity(t *testing.T) {
	require.NoError(t, model.DB.AutoMigrate(
		&model.AgentBudgetPool{},
		&model.AgentBudgetTransaction{},
		&model.OpsFundAccount{},
		&model.OpsFundLedger{},
		&model.AgentMemory{},
		&model.AgentEvent{},
	))
	cleanupOpsRewardFundTestTables(t)
	require.NoError(t, model.DB.Exec("DELETE FROM agent_memories").Error)
	require.NoError(t, model.DB.Exec("DELETE FROM agent_events").Error)
	t.Cleanup(func() {
		_ = model.DB.Exec("DELETE FROM agent_memories").Error
		_ = model.DB.Exec("DELETE FROM agent_events").Error
	})

	oldAgent := *operation_setting.GetAgentSetting()
	t.Cleanup(func() { *operation_setting.GetAgentSetting() = oldAgent })
	setting := operation_setting.GetAgentSetting()
	setting.SiteID = "runtime-default-site"
	setting.BudgetEnabled = true
	setting.DailyBudgetResetEnabled = true
	setting.DailyFundResetEnabled = false
	setting.DailyBudgetQuota = 0
	setting.GrowthBudgetQuota = 0
	setting.ActivityBudgetQuota = 1_000
	setting.GameBudgetQuota = 0
	setting.OpsCompBudgetQuota = 0
	setting.CommunityBudgetQuota = 0

	today := model.AgentBusinessDateAt(time.Now())
	require.NoError(t, model.DB.Create(&model.AgentBudgetPool{
		SiteId: "runtime-default-site", PoolType: "activity", BudgetDate: today,
		TotalQuota: 1_000, UsedQuota: 900, Status: "active",
	}).Error)

	require.NoError(t, EnsureAgentRuntimeDefaults())
	require.NoError(t, EnsureAgentRuntimeDefaults())

	var pool model.AgentBudgetPool
	require.NoError(t, model.DB.Where(
		"site_id = ? AND pool_type = ? AND budget_date = ?",
		"runtime-default-site", "activity", today,
	).First(&pool).Error)
	require.Equal(t, 1_900, pool.TotalQuota)
	require.Equal(t, 1_000, pool.TotalQuota-pool.UsedQuota-pool.FrozenQuota)

	var restoreCount int64
	require.NoError(t, model.DB.Model(&model.AgentBudgetTransaction{}).
		Where("source_type = ?", model.OpsBudgetRestoreSourceDaily).
		Count(&restoreCount).Error)
	require.Equal(t, int64(1), restoreCount)
}

func TestOpsRewardFundOverviewAndInviteJourney(t *testing.T) {
	require.NoError(t, model.DB.AutoMigrate(
		&model.User{},
		&model.ChatGroup{},
		&model.GroupChatOpsConfig{},
		&model.GroupGameConfig{},
		&model.GroupMetricsDaily{},
		&model.AgentBudgetPool{},
		&model.AgentBudgetTransaction{},
		&model.OpsFundAccount{},
		&model.OpsFundLedger{},
		&model.GameRound{},
		&model.GameEntry{},
		&model.GameSettlement{},
		&model.GameCommission{},
		&model.InviteCampaign{},
		&model.InviteLink{},
		&model.InviteEdge{},
		&model.InviteEvent{},
		&model.InviteRewardClaim{},
		&model.InviteRiskFlag{},
	))
	cleanupOpsRewardFundTestTables(t)
	oldAgent := *operation_setting.GetAgentSetting()
	t.Cleanup(func() { *operation_setting.GetAgentSetting() = oldAgent })

	siteID := "ops-fund-test"
	operation_setting.GetAgentSetting().SiteID = siteID
	operation_setting.GetAgentSetting().DailyBudgetResetEnabled = false
	operation_setting.GetAgentSetting().DailyFundResetEnabled = false
	now := time.Now().Unix()
	today := model.AgentBusinessDateAt(time.Now())

	require.NoError(t, model.DB.Create(&model.ChatGroup{SiteId: siteID, Platform: "qq_group", GroupId: "main-1", GroupName: "主场群", Role: "primary_mainfield", Status: "active", CreatedAt: now, UpdatedAt: now}).Error)
	require.NoError(t, model.DB.Create(&model.GroupChatOpsConfig{
		SiteId: siteID, Platform: "qq_group", GroupId: "main-1", CheckinEnabled: true, VerifyEnabled: true, InviteEnabled: true,
		CheckinQuota: 100, InviteRewardQuota: 300, InviteeRewardQuota: 200, DailyGroupRewardLimit: 2000, CreatedAt: now, UpdatedAt: now,
	}).Error)
	require.NoError(t, model.DB.Create(&model.GroupGameConfig{SiteId: siteID, Platform: "qq_group", GroupId: "main-1", GameCode: "dice", Enabled: true, BudgetPool: "game", CreatedAt: now, UpdatedAt: now}).Error)
	require.NoError(t, model.DB.Create(&model.GroupMetricsDaily{SiteId: siteID, Platform: "qq_group", GroupId: "main-1", MetricDate: today, InviteLinks: 1, Joins: 1, Verifies: 1, CommissionQuota: 150, RewardCostQuota: 100}).Error)

	require.NoError(t, model.DB.Create(&model.AgentBudgetPool{SiteId: siteID, PoolType: "community", BudgetDate: today, TotalQuota: 500, UsedQuota: 490, FrozenQuota: 0, Status: "active"}).Error)
	require.NoError(t, model.DB.Create(&model.AgentBudgetPool{SiteId: siteID, PoolType: "game", BudgetDate: today, TotalQuota: 5000, UsedQuota: 1000, FrozenQuota: 0, Status: "active"}).Error)
	require.NoError(t, model.DB.Create(&model.OpsFundAccount{SiteId: siteID, FundType: "operations", BalanceQuota: 1000, Status: "active"}).Error)
	var acc model.OpsFundAccount
	require.NoError(t, model.DB.Where("site_id = ?", siteID).First(&acc).Error)
	require.NoError(t, model.DB.Create(&model.OpsFundLedger{SiteId: siteID, FundAccountId: acc.Id, DeltaQuota: 5000, BalanceAfter: 5000, SourceType: "fund_topup", IdempotencyKey: "topup-1", CreatedAt: now}).Error)
	inviteLedger := model.OpsFundLedger{SiteId: siteID, FundAccountId: acc.Id, DeltaQuota: -100, BalanceAfter: 4900, SourceType: "invite_inviter_reward", SourcePoolType: "community", UserId: 10, IdempotencyKey: "invite-1", CreatedAt: now}
	require.NoError(t, model.DB.Create(&inviteLedger).Error)
	gameLedger := model.OpsFundLedger{SiteId: siteID, FundAccountId: acc.Id, DeltaQuota: 300, BalanceAfter: 5200, SourceType: "game_stake", SourcePoolType: "game", UserId: 11, IdempotencyKey: "game-in", CreatedAt: now}
	require.NoError(t, model.DB.Create(&gameLedger).Error)
	require.NoError(t, model.DB.Create(&model.OpsFundLedger{SiteId: siteID, FundAccountId: acc.Id, DeltaQuota: -200, BalanceAfter: 5000, SourceType: "game_payout", SourcePoolType: "game", UserId: 12, IdempotencyKey: "game-out", CreatedAt: now}).Error)
	round := model.GameRound{SiteId: siteID, Platform: "qq_group", GroupId: "main-1", GameCode: "dice", RoundKey: "round-1", Status: "closed", CreatedAt: now, UpdatedAt: now}
	require.NoError(t, model.DB.Create(&round).Error)
	settlement := model.GameSettlement{SiteId: siteID, RoundId: round.Id, UserId: 11, SettlementKey: "settlement-1", StakeQuota: 300, PayoutQuota: 150, CommissionQuota: 150, OpsFundLedgerId: gameLedger.Id, Status: "closed", CreatedAt: now, UpdatedAt: now}
	require.NoError(t, model.DB.Create(&settlement).Error)
	require.NoError(t, model.DB.Create(&model.GameCommission{SiteId: siteID, RoundId: round.Id, SettlementId: settlement.Id, GameCode: "dice", GroupId: "main-1", CommissionQuota: 150, OpsFundLedgerId: gameLedger.Id, CreatedAt: now}).Error)

	campaign := model.InviteCampaign{SiteId: siteID, CampaignCode: "camp-main", Name: "主场邀请", SourcePlatform: "community", SourceGroupId: "room-1", TargetPlatform: "qq_group", TargetGroupId: "main-1", Status: "active", CreatedAt: now, UpdatedAt: now}
	require.NoError(t, model.DB.Create(&campaign).Error)
	link := model.InviteLink{SiteId: siteID, CampaignId: campaign.Id, InviterUserId: 10, Provider: "community", ExternalUserId: "u10", InviteCode: "code-10", Status: "active", CreatedAt: now, UpdatedAt: now}
	require.NoError(t, model.DB.Create(&link).Error)
	edge := model.InviteEdge{SiteId: siteID, CampaignId: campaign.Id, InviteLinkId: link.Id, InviterUserId: 10, InviteeUserId: 20, InviteeProvider: "community", InviteeExternalId: "u20", Stage: "verified", Status: "verified", CreatedAt: now, UpdatedAt: now}
	require.NoError(t, model.DB.Create(&edge).Error)
	require.NoError(t, model.DB.Create(&model.InviteEvent{SiteId: siteID, CampaignId: campaign.Id, InviteLinkId: link.Id, InviteEdgeId: edge.Id, EventType: "verify_claim", Provider: "community", ExternalUserId: "u20", UserId: 20, GroupId: "room-1", CreatedAt: now}).Error)
	require.NoError(t, model.DB.Create(&model.InviteRewardClaim{SiteId: siteID, CampaignId: campaign.Id, InviteEdgeId: edge.Id, RewardStage: "inviter", RewardUserId: 10, Quota: 100, Status: "paid", OpsFundLedgerId: inviteLedger.Id, CreatedAt: now, UpdatedAt: now}).Error)
	require.NoError(t, model.DB.Create(&model.InviteRewardClaim{SiteId: siteID, CampaignId: campaign.Id, InviteEdgeId: edge.Id, RewardStage: "invitee", RewardUserId: 20, Quota: 50, Status: "blocked_membership", Error: "left", CreatedAt: now, UpdatedAt: now}).Error)

	fund, err := GetOpsRewardFundOverview(siteID)
	require.NoError(t, err)
	require.Equal(t, siteID, fund.SiteID)
	require.Equal(t, 1000, fund.Fund["balance_quota"])
	require.Equal(t, "degraded", fund.Degradation["state"])
	require.Equal(t, 2, len(fund.BudgetPoolsToday))
	require.Equal(t, int64(150), fund.CommissionAudit["commission_quota"])
	require.Equal(t, int64(100), fund.InviteRewardAudit["paid_claim_quota"])
	require.Equal(t, int64(0), fund.InviteRewardAudit["paid_without_ledger_count"])
	require.Equal(t, 1000, fund.EffectiveAvailableQuota)
	require.Equal(t, "Asia/Shanghai", fund.BudgetSettings.BudgetTimezone)
	require.False(t, fund.BudgetSettings.DailyBudgetResetEnabled)
	require.False(t, fund.BudgetSettings.DailyFundResetEnabled)
	for _, pool := range fund.BudgetPoolsToday {
		require.Contains(t, pool, "effective_available_quota")
		require.Contains(t, pool, "effective_available_usd")
	}

	journey, err := GetOpsInviteJourneyOverview(siteID)
	require.NoError(t, err)
	require.Equal(t, siteID, journey.SiteID)
	require.Equal(t, int64(1), journey.Funnel["links"])
	require.Equal(t, int64(1), journey.Funnel["paid_claims"])
	require.Equal(t, int64(1), journey.Funnel["blocked_claims"])
	require.NotEmpty(t, journey.StateMachine)
	require.NotEmpty(t, journey.ClaimStatuses)
	require.NotEmpty(t, journey.Problems)
}

func TestOpsRewardFundOverviewBackfillsPaidClaimLedgerLinks(t *testing.T) {
	require.NoError(t, model.DB.AutoMigrate(
		&model.OpsFundAccount{},
		&model.OpsFundLedger{},
		&model.InviteCampaign{},
		&model.InviteEdge{},
		&model.InviteRewardClaim{},
	))
	cleanupOpsRewardFundTestTables(t)
	oldAgent := *operation_setting.GetAgentSetting()
	t.Cleanup(func() { *operation_setting.GetAgentSetting() = oldAgent })

	siteID := "ops-fund-backfill"
	operation_setting.GetAgentSetting().SiteID = siteID
	operation_setting.GetAgentSetting().DailyBudgetResetEnabled = false
	operation_setting.GetAgentSetting().DailyFundResetEnabled = false
	now := time.Now().Unix()

	require.NoError(t, model.DB.Create(&model.OpsFundAccount{SiteId: siteID, FundType: "operations", BalanceQuota: 9000, Status: "active", CreatedAt: now, UpdatedAt: now}).Error)
	var acc model.OpsFundAccount
	require.NoError(t, model.DB.Where("site_id = ? AND fund_type = ?", siteID, "operations").First(&acc).Error)

	campaign := model.InviteCampaign{SiteId: siteID, CampaignCode: "camp-backfill", Name: "backfill", SourcePlatform: "community", SourceGroupId: "room-1", TargetPlatform: "qq_group", TargetGroupId: "main-1", Status: "active", CreatedAt: now, UpdatedAt: now}
	require.NoError(t, model.DB.Create(&campaign).Error)
	edge := model.InviteEdge{SiteId: siteID, CampaignId: campaign.Id, InviterUserId: 77, InviteeUserId: 88, InviteeProvider: "community", InviteeExternalId: "u88", Stage: "verified", Status: "verified", CreatedAt: now, UpdatedAt: now}
	require.NoError(t, model.DB.Create(&edge).Error)

	ledgerIDEM := "invite:ops-fund-backfill:1:inviter:77"
	ledger := model.OpsFundLedger{SiteId: siteID, FundAccountId: acc.Id, DeltaQuota: -2500000, BalanceAfter: 6500, SourceType: "invite_inviter_reward", SourcePoolType: "community", UserId: 77, IdempotencyKey: ledgerIDEM, CreatedAt: now}
	require.NoError(t, model.DB.Create(&ledger).Error)

	claim := model.InviteRewardClaim{SiteId: siteID, CampaignId: campaign.Id, InviteEdgeId: edge.Id, RewardStage: "inviter", RewardUserId: 77, Quota: 2500000, Status: "paid", OpsFundLedgerId: 0, CreatedAt: now, UpdatedAt: now}
	require.NoError(t, model.DB.Create(&claim).Error)

	fund, err := GetOpsRewardFundOverview(siteID)
	require.NoError(t, err)
	require.Equal(t, int64(0), fund.InviteRewardAudit["paid_without_ledger_count"])

	var repaired model.InviteRewardClaim
	require.NoError(t, model.DB.First(&repaired, claim.Id).Error)
	require.Equal(t, ledger.Id, repaired.OpsFundLedgerId)
}

func cleanupOpsRewardFundTestTables(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		for _, table := range []string{
			"invite_risk_flags", "invite_reward_claims", "invite_events", "invite_edges", "invite_links", "invite_campaigns",
			"game_commissions", "game_settlements", "game_entries", "game_rounds",
			"ops_fund_ledgers", "ops_fund_accounts", "agent_budget_transactions", "agent_budget_pools",
			"group_metrics_daily", "group_game_configs", "group_chatops_configs", "chat_groups",
		} {
			model.DB.Exec("DELETE FROM " + table)
		}
	})
	for _, table := range []string{
		"invite_risk_flags", "invite_reward_claims", "invite_events", "invite_edges", "invite_links", "invite_campaigns",
		"game_commissions", "game_settlements", "game_entries", "game_rounds",
		"ops_fund_ledgers", "ops_fund_accounts", "agent_budget_transactions", "agent_budget_pools",
		"group_metrics_daily", "group_game_configs", "group_chatops_configs", "chat_groups",
	} {
		model.DB.Exec("DELETE FROM " + table)
	}
}
