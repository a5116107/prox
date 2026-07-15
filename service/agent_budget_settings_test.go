package service

import (
	"testing"

	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/require"
)

func TestAgentSettingPublicMapIncludesDailyBudgetControls(t *testing.T) {
	oldAgent := *operation_setting.GetAgentSetting()
	t.Cleanup(func() { *operation_setting.GetAgentSetting() = oldAgent })

	cfg := operation_setting.GetAgentSetting()
	cfg.CommunityBudgetQuota = 123
	cfg.DailyBudgetResetEnabled = false
	cfg.DailyFundResetEnabled = true
	cfg.OpsFundDailyTargetQuota = 456

	settings := agentSettingPublicMap()
	require.Equal(t, 123, settings["community_budget_quota"])
	require.Equal(t, false, settings["daily_budget_reset_enabled"])
	require.Equal(t, true, settings["daily_fund_reset_enabled"])
	require.Equal(t, 456, settings["ops_fund_daily_target_quota"])
}
