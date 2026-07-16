package service

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupGamePolicyTest(t *testing.T) {
	t.Helper()
	setupQuizBankTest(t)
	require.NoError(t, model.DB.AutoMigrate(
		&model.ChatGroup{}, &model.GroupGameConfig{}, &model.GroupChatOpsConfig{},
		&model.InviteCampaign{}, &model.InviteEdge{}, &model.InviteRewardClaim{},
	))
	require.NoError(t, model.DB.Where("site_id = ?", "quiz-test").Delete(&model.ChatGroup{}).Error)
	require.NoError(t, model.DB.Where("site_id = ?", "quiz-test").Delete(&model.GroupGameConfig{}).Error)
	require.NoError(t, model.DB.Where("site_id = ?", "quiz-test").Delete(&model.GroupChatOpsConfig{}).Error)
	require.NoError(t, model.DB.Where("site_id = ?", "quiz-test").Delete(&model.InviteRewardClaim{}).Error)
	require.NoError(t, model.DB.Where("site_id = ?", "quiz-test").Delete(&model.InviteEdge{}).Error)
	require.NoError(t, model.DB.Where("site_id = ?", "quiz-test").Delete(&model.InviteCampaign{}).Error)
	t.Cleanup(func() {
		_ = model.DB.Where("site_id = ?", "quiz-test").Delete(&model.ChatGroup{}).Error
		_ = model.DB.Where("site_id = ?", "quiz-test").Delete(&model.InviteRewardClaim{}).Error
		_ = model.DB.Where("site_id = ?", "quiz-test").Delete(&model.InviteEdge{}).Error
		_ = model.DB.Where("site_id = ?", "quiz-test").Delete(&model.InviteCampaign{}).Error
		_ = model.DB.Where("site_id = ?", "quiz-test").Delete(&model.GroupGameConfig{}).Error
		_ = model.DB.Where("site_id = ?", "quiz-test").Delete(&model.GroupChatOpsConfig{}).Error
	})
}

func TestEffectiveGroupGamePolicyMergesPlatformAndGroupLayers(t *testing.T) {
	setupGamePolicyTest(t)
	require.NoError(t, model.DB.Create(&model.GroupGameConfig{
		SiteId: "quiz-test", Platform: "qq", GroupId: "", GameCode: "quiz",
		Enabled: true, BudgetPool: "activity", RuleJson: `{"max_per_user_day":3,"nested":{"base":1}}`,
	}).Error)
	require.NoError(t, model.DB.Create(&model.GroupGameConfig{
		SiteId: "quiz-test", Platform: "qq_group", GroupId: "room-1", GameCode: "quiz",
		Enabled: false, BudgetPool: "community", RuleJson: `{"reward_quota":250,"nested":{"group":2}}`,
	}).Error)

	policy, err := resolveEffectiveGroupGamePolicyTx(model.DB, "quiz-test", "qq", "room-1", "quiz")
	require.NoError(t, err)
	require.True(t, policy.Found)
	require.False(t, policy.Enabled)
	require.Equal(t, "community", policy.BudgetPool)
	require.Equal(t, 3, int(policy.Rules["max_per_user_day"].(float64)))
	require.Equal(t, 250, int(policy.Rules["reward_quota"].(float64)))
	nested := policy.Rules["nested"].(map[string]any)
	require.Equal(t, 1, int(nested["base"].(float64)))
	require.Equal(t, 2, int(nested["group"].(float64)))
}

func TestEffectiveGroupGamePolicyRejectsMalformedRules(t *testing.T) {
	setupGamePolicyTest(t)
	require.NoError(t, model.DB.Create(&model.GroupGameConfig{
		SiteId: "quiz-test", Platform: "qq_group", GroupId: "room-1", GameCode: "quiz",
		Enabled: true, BudgetPool: "activity", RuleJson: `{broken`,
	}).Error)

	_, err := resolveEffectiveGroupGamePolicyTx(model.DB, "quiz-test", "qq", "room-1", "quiz")
	require.ErrorContains(t, err, "invalid quiz rules")
}

func TestSaveOpsGroupGamesValidatesWholeBatchBeforeWriting(t *testing.T) {
	setupGamePolicyTest(t)
	group := model.ChatGroup{
		SiteId: "quiz-test", Platform: "qq_group", GroupId: "room-1",
		GroupName: "Room 1", Role: "primary", Status: "active", Language: "zh-CN", Timezone: "Asia/Shanghai",
	}
	require.NoError(t, model.DB.Create(&group).Error)

	_, err := SaveOpsGroupGames("quiz-test", group.Id, OpsGroupGamesUpdateRequest{Games: []OpsGroupGameUpdateItem{
		{GameCode: "quiz", BudgetPool: "activity", Rule: map[string]any{"reward_quota": 100}},
		{GameCode: "invite", BudgetPool: "unknown"},
	}})
	require.ErrorContains(t, err, "invalid budget_pool")

	var count int64
	require.NoError(t, model.DB.Model(&model.GroupGameConfig{}).Where("site_id = ? AND group_id = ?", "quiz-test", "room-1").Count(&count).Error)
	require.Zero(t, count)
}

func TestInviteRewardPolicyUsesBackendValuesIncludingExplicitZero(t *testing.T) {
	setupGamePolicyTest(t)
	require.NoError(t, model.DB.Create(&model.GroupGameConfig{
		SiteId: "quiz-test", Platform: "qq_group", GroupId: "room-1", GameCode: "invite",
		Enabled: true, BudgetPool: "activity",
		RuleJson: `{"inviter_reward_quota":0,"invitee_reward_quota":250,"max_per_user_day":2}`,
	}).Error)

	policy, err := inviteRewardConfigTx(model.DB, AgentChatOpsInviteRequest{
		Source: "qq", RoomID: "room-1", InviterRewardQuota: 999999999,
		InviteeRewardQuota: 999999999, BudgetPool: "ops_comp",
	})
	require.NoError(t, err)
	require.Equal(t, 0, policy.InviterQuota)
	require.Equal(t, 250, policy.InviteeQuota)
	require.Equal(t, 2, policy.MaxPerUserDay)
	require.Equal(t, "activity", policy.BudgetPool)
}

func TestCheckinRuleUsesBackendBudgetPool(t *testing.T) {
	policy := effectiveGroupGamePolicy{
		Found: true, Enabled: true, BudgetPool: "community",
		Rules: map[string]any{"reward_min": float64(100), "reward_max": float64(200)},
	}
	rule, _, _, pool, err := chatOpsCheckinRule(nil, policy)
	require.NoError(t, err)
	require.Equal(t, 100, rule.RewardMin)
	require.Equal(t, 200, rule.RewardMax)
	require.Equal(t, "community", pool)
}

func TestChatOpsCheckinSettlesBackendBudgetPool(t *testing.T) {
	setupGamePolicyTest(t)
	now := time.Now().Unix()
	require.NoError(t, model.DB.Create(&model.GroupGameConfig{
		SiteId: "quiz-test", Platform: "qq_group", GroupId: "room-1", GameCode: "checkin",
		Enabled: true, BudgetPool: "community", RuleJson: `{"fixed_quota":250000}`,
		CreatedAt: now, UpdatedAt: now,
	}).Error)
	checkin := operation_setting.GetCheckinSetting()
	checkin.Enabled = true
	checkin.MinQuota = 1
	checkin.MaxQuota = 1
	checkin.Channels = map[string]operation_setting.CheckinChannelSetting{}
	operation_setting.GetAgentSetting().CommunityBudgetQuota = 1_000_000

	result := HandleAgentChatOpsCheckin(AgentChatOpsRequest{
		Source: "qq", RoomID: "room-1", MessageID: "checkin-backend-pool",
		UserExternalID: "qq-user-910001", Username: "quiz_test_user", NewAPIUserID: quizTestUserID,
	})
	require.True(t, result.Success, result.Reply)
	require.Equal(t, 250_000, result.QuotaAwarded)

	var transaction model.AgentBudgetTransaction
	require.NoError(t, model.DB.Where("idempotency_key LIKE ?", "checkin:qq:"+fmt.Sprint(quizTestUserID)+":%").First(&transaction).Error)
	require.Equal(t, "community", transaction.PoolType)
}

func TestInviteDailyRewardLimitIsSerializedAcrossConcurrentClaims(t *testing.T) {
	setupGamePolicyTest(t)
	if sqlDB, err := model.DB.DB(); err == nil {
		previousMax := sqlDB.Stats().MaxOpenConnections
		sqlDB.SetMaxOpenConns(1)
		t.Cleanup(func() { sqlDB.SetMaxOpenConns(previousMax) })
	}
	now := time.Now().Unix()
	campaign := model.InviteCampaign{
		SiteId: "quiz-test", CampaignCode: "concurrent-limit", Status: "active",
		CreatedAt: now, UpdatedAt: now,
	}
	require.NoError(t, model.DB.Create(&campaign).Error)
	edges := []model.InviteEdge{
		{SiteId: "quiz-test", CampaignId: campaign.Id, InviterUserId: quizTestUserID, InviteeProvider: "qq", InviteeExternalId: "invitee-a", Stage: "verified", Status: "verified", CreatedAt: now, UpdatedAt: now},
		{SiteId: "quiz-test", CampaignId: campaign.Id, InviterUserId: quizTestUserID, InviteeProvider: "qq", InviteeExternalId: "invitee-b", Stage: "verified", Status: "verified", CreatedAt: now, UpdatedAt: now},
	}
	for index := range edges {
		require.NoError(t, model.DB.Create(&edges[index]).Error)
	}
	req := AgentChatOpsInviteRequest{
		Source: "qq", RoomID: "room-1", UserExternalID: "qq-user-910001",
		InviterExternalID: "qq-user-910001",
	}

	start := make(chan struct{})
	results := make(chan AgentChatOpsInviteClaim, len(edges))
	errors := make(chan error, len(edges))
	var wait sync.WaitGroup
	for index := range edges {
		wait.Add(1)
		go func(edge model.InviteEdge) {
			defer wait.Done()
			<-start
			var result AgentChatOpsInviteClaim
			err := model.DB.Transaction(func(tx *gorm.DB) error {
				var claimErr error
				result, claimErr = payInviteRewardClaimTx(tx, req, &campaign, &edge, "inviter", quizTestUserID, 100, "community", 1)
				return claimErr
			})
			if err != nil {
				errors <- err
				return
			}
			results <- result
		}(edges[index])
	}
	close(start)
	wait.Wait()
	close(results)
	close(errors)
	for err := range errors {
		require.NoError(t, err)
	}
	statuses := map[string]int{}
	for result := range results {
		statuses[result.Status]++
	}
	require.Equal(t, 1, statuses["paid"])
	require.Equal(t, 1, statuses["blocked_daily_limit"])

	var user model.User
	require.NoError(t, model.DB.First(&user, quizTestUserID).Error)
	require.Equal(t, 100, user.Quota)
}
