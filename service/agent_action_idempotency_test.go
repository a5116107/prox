package service

import (
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupAgentActionIdempotencyTestDB(t *testing.T) {
	t.Helper()
	oldDB := model.DB
	oldAgent := *operation_setting.GetAgentSetting()
	dsn := fmt.Sprintf("file:agent_action_%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.AgentAction{}))
	model.DB = db
	operation_setting.GetAgentSetting().SiteID = "prox-test"
	t.Cleanup(func() {
		model.DB = oldDB
		*operation_setting.GetAgentSetting() = oldAgent
	})
}

func TestAgentCreateActionReplaysCompletedIdempotentAction(t *testing.T) {
	setupAgentActionIdempotencyTestDB(t)
	payload := map[string]any{"game_action": true, "settlement_id": "settlement-1"}
	storedPayload := map[string]any{"game_action": true, "settlement_id": "settlement-1", "user_id": 17}
	existing := model.AgentAction{
		SiteId: "prox-test", ActionType: "reward.settlement.batch", AgentName: "director",
		TargetType: "budget", TargetId: "room-1", UserId: 17, QuotaAmount: 500,
		BudgetPool: "game", Status: "completed", IdempotencyKey: "settlement-1",
		Reason: "treasure refund", PayloadJson: mustAgentJSON(storedPayload), ResultJson: `{"success":true}`,
	}
	require.NoError(t, model.DB.Create(&existing).Error)

	replayed, err := AgentCreateAction(AgentActionRequest{
		ActionType: "reward.settlement.batch", AgentName: "director",
		TargetType: "budget", TargetId: "room-1", UserId: 17, QuotaAmount: 500,
		BudgetPool: "game", Reason: "treasure refund", Payload: payload,
		IdempotencyKey: "settlement-1",
	}, 0)

	require.NoError(t, err)
	require.Equal(t, existing.Id, replayed.Id)
	require.Equal(t, "completed", replayed.Status)
	var count int64
	require.NoError(t, model.DB.Model(&model.AgentAction{}).Count(&count).Error)
	require.Equal(t, int64(1), count)
}

func TestAgentCreateActionRejectsMismatchedIdempotencyReplay(t *testing.T) {
	setupAgentActionIdempotencyTestDB(t)
	existing := model.AgentAction{
		SiteId: "prox-test", ActionType: "reward.settlement.batch", AgentName: "director",
		TargetType: "budget", TargetId: "room-1", UserId: 17, QuotaAmount: 500,
		BudgetPool: "game", Status: "completed", IdempotencyKey: "settlement-2",
		Reason: "treasure refund", PayloadJson: mustAgentJSON(map[string]any{"game_action": true, "user_id": 17}),
	}
	require.NoError(t, model.DB.Create(&existing).Error)

	_, err := AgentCreateAction(AgentActionRequest{
		ActionType: "reward.settlement.batch", AgentName: "director",
		TargetType: "budget", TargetId: "room-1", UserId: 17, QuotaAmount: 999,
		BudgetPool: "game", Reason: "different mutation",
		Payload: map[string]any{"game_action": true}, IdempotencyKey: "settlement-2",
	}, 0)

	require.ErrorContains(t, err, "idempotency key conflict")
}
