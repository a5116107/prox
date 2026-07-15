package service

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/require"
)

const quizTestUserID = 910001

func setupQuizBankTest(t *testing.T) {
	t.Helper()
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false

	require.NoError(t, model.DB.AutoMigrate(
		&model.User{},
		&model.GameRound{},
		&model.GameEntry{},
		&model.QuizBank{},
		&model.QuizCategory{},
		&model.QuizQuestion{},
		&model.QuizBankBinding{},
		&model.QuizQuestionDraw{},
		&model.AgentBudgetPool{},
		&model.AgentBudgetTransaction{},
		&model.OpsFundAccount{},
		&model.OpsFundLedger{},
	))

	oldAgent := *operation_setting.GetAgentSetting()
	oldMembership := *operation_setting.GetMembershipRiskSetting()
	t.Cleanup(func() {
		*operation_setting.GetAgentSetting() = oldAgent
		*operation_setting.GetMembershipRiskSetting() = oldMembership
	})
	agentSetting := operation_setting.GetAgentSetting()
	agentSetting.SiteID = "quiz-test"
	agentSetting.ActivityBudgetQuota = 1_000_000
	agentSetting.DailyBudgetResetEnabled = true
	agentSetting.DailyFundResetEnabled = true
	membershipSetting := operation_setting.GetMembershipRiskSetting()
	membershipSetting.Enabled = false
	membershipSetting.DryRun = false

	cleanupQuizBankTestRows(t)
	require.NoError(t, model.DB.Create(&model.User{
		Id: quizTestUserID, Username: "quiz_test_user", Password: "not-used-in-test", Status: 1,
	}).Error)
}

func cleanupQuizBankTestRows(t *testing.T) {
	t.Helper()
	tables := []string{
		"ops_fund_ledgers",
		"agent_budget_transactions",
		"agent_budget_pools",
		"ops_fund_accounts",
		"quiz_question_draws",
		"game_entries",
		"game_rounds",
		"quiz_bank_bindings",
		"quiz_questions",
		"quiz_categories",
		"quiz_banks",
	}
	for _, table := range tables {
		require.NoError(t, model.DB.Exec("DELETE FROM "+table).Error)
	}
	require.NoError(t, model.DB.Unscoped().Where("id = ?", quizTestUserID).Delete(&model.User{}).Error)
	t.Cleanup(func() {
		for _, table := range tables {
			_ = model.DB.Exec("DELETE FROM " + table).Error
		}
		_ = model.DB.Unscoped().Where("id = ?", quizTestUserID).Delete(&model.User{}).Error
	})
}

func seedQuizBank(t *testing.T, code string, prompts ...string) model.QuizBank {
	t.Helper()
	now := time.Now().Unix()
	bank := model.QuizBank{SiteId: "quiz-test", Code: code, Name: code, Status: "published", DefaultLanguage: "zh-CN", CreatedAt: now, UpdatedAt: now}
	require.NoError(t, model.DB.Create(&bank).Error)
	for index, prompt := range prompts {
		options := []string{fmt.Sprintf("wrong-%d", index), fmt.Sprintf("correct-%d", index), fmt.Sprintf("other-%d", index)}
		encoded, err := json.Marshal(options)
		require.NoError(t, err)
		question := model.QuizQuestion{
			BankId: bank.Id, ExternalKey: fmt.Sprintf("%s-%d", code, index+1), Prompt: prompt,
			OptionsJson: string(encoded), CorrectIndex: 1, Explanation: "because", Difficulty: "normal",
			Language: "zh-CN", Status: "published", Weight: 100, Source: "test",
			ContentHash: QuizQuestionContentHash(prompt, options, 1), CreatedAt: now, UpdatedAt: now,
		}
		require.NoError(t, model.DB.Create(&question).Error)
	}
	return bank
}

func quizTestAction(key string) *model.AgentAction {
	return &model.AgentAction{Id: 77, TargetId: "qq-user-910001", IdempotencyKey: key}
}

func quizTestPayload(drawKey string) map[string]any {
	return map[string]any{
		"source": "qq", "room_id": "room-a", "scope_mode": "per_user",
		"user_id": quizTestUserID, "user_external_id": "qq-user-910001",
		"draw_key": drawKey, "question_ttl_seconds": 600, "max_attempts_per_question": 2,
		"max_per_user_day": 10, "reward_quota": 0, "budget_pool": "activity",
	}
}

func quizResultQuestion(t *testing.T, result map[string]any) map[string]any {
	t.Helper()
	question, ok := result["question"].(map[string]any)
	require.True(t, ok)
	return question
}

func quizResultInt(t *testing.T, result map[string]any, key string) int {
	t.Helper()
	value, ok := result[key].(int)
	if ok {
		return value
	}
	floatValue, ok := result[key].(float64)
	require.True(t, ok, "expected numeric %s, got %#v", key, result[key])
	return int(floatValue)
}

func shownOptionIndex(t *testing.T, result map[string]any, option string) int {
	t.Helper()
	options, ok := quizResultQuestion(t, result)["options"].([]string)
	require.True(t, ok)
	for index, current := range options {
		if current == option {
			return index
		}
	}
	t.Fatalf("option %q not found in %#v", option, options)
	return -1
}

func TestQuizDrawDoesNotLeakAnswerAndRestoresOpenRound(t *testing.T) {
	setupQuizBankTest(t)
	seedQuizBank(t, "general", "2 + 2 = ?")

	first, err := quizQuestionDraw(quizTestAction("draw-a"), quizTestPayload("draw-a"))
	require.NoError(t, err)
	require.True(t, first["active"].(bool))
	require.NotContains(t, first, "correct_answer")
	require.NotContains(t, first, "correct_option_index")
	question := quizResultQuestion(t, first)
	require.NotContains(t, question, "correct_index")
	require.NotContains(t, question, "answer")

	second, err := quizQuestionDraw(quizTestAction("draw-b"), quizTestPayload("draw-b"))
	require.NoError(t, err)
	require.Equal(t, quizResultInt(t, first, "draw_id"), quizResultInt(t, second, "draw_id"))
	require.Equal(t, first["round_key"], second["round_key"])
}

func TestQuizAnswerAttemptsAndQuestionRotation(t *testing.T) {
	setupQuizBankTest(t)
	seedQuizBank(t, "rotation", "question one", "question two")

	draw, err := quizQuestionDraw(quizTestAction("rotation-a"), quizTestPayload("rotation-a"))
	require.NoError(t, err)
	firstQuestionID := quizResultInt(t, quizResultQuestion(t, draw), "id")
	wrongIndex := shownOptionIndex(t, draw, "wrong-0")
	correctIndex := shownOptionIndex(t, draw, "correct-0")

	wrong, err := quizAnswerSubmit(quizTestAction("answer-wrong"), map[string]any{
		"draw_id": quizResultInt(t, draw, "draw_id"), "user_id": quizTestUserID,
		"user_external_id": "qq-user-910001", "answer_index": wrongIndex,
	})
	require.NoError(t, err)
	require.False(t, wrong["correct"].(bool))
	require.Equal(t, 1, quizResultInt(t, wrong, "remaining_attempts"))
	require.NotContains(t, wrong, "correct_answer")

	correct, err := quizAnswerSubmit(quizTestAction("answer-correct"), map[string]any{
		"draw_id": quizResultInt(t, draw, "draw_id"), "user_id": quizTestUserID,
		"user_external_id": "qq-user-910001", "answer_index": correctIndex,
	})
	require.NoError(t, err)
	require.True(t, correct["correct"].(bool))
	require.Equal(t, "correct-0", correct["correct_answer"])

	next, err := quizQuestionDraw(quizTestAction("rotation-b"), quizTestPayload("rotation-b"))
	require.NoError(t, err)
	require.NotEqual(t, firstQuestionID, quizResultInt(t, quizResultQuestion(t, next), "id"))
}

func TestQuizLocksAfterMaximumAttempts(t *testing.T) {
	setupQuizBankTest(t)
	seedQuizBank(t, "attempts", "only question")
	draw, err := quizQuestionDraw(quizTestAction("attempts-a"), quizTestPayload("attempts-a"))
	require.NoError(t, err)
	wrongIndex := shownOptionIndex(t, draw, "wrong-0")
	payload := map[string]any{
		"draw_id": quizResultInt(t, draw, "draw_id"), "user_id": quizTestUserID,
		"user_external_id": "qq-user-910001", "answer_index": wrongIndex,
	}
	_, err = quizAnswerSubmit(quizTestAction("attempt-one"), payload)
	require.NoError(t, err)
	locked, err := quizAnswerSubmit(quizTestAction("attempt-two"), payload)
	require.NoError(t, err)
	require.True(t, locked["locked"].(bool))
	require.True(t, locked["closed"].(bool))
	require.Equal(t, 0, quizResultInt(t, locked, "remaining_attempts"))
	require.Equal(t, "correct-0", locked["correct_answer"])
}

func TestQuizBindingPrecedenceAndContentHash(t *testing.T) {
	setupQuizBankTest(t)
	wildcard := seedQuizBank(t, "wildcard", "wildcard question")
	exact := seedQuizBank(t, "exact", "exact question")
	now := time.Now().Unix()
	require.NoError(t, model.DB.Create(&model.QuizBankBinding{
		SiteId: "quiz-test", BankId: wildcard.Id, Platform: "*", GroupId: "*", Enabled: true, Priority: 999, CreatedAt: now, UpdatedAt: now,
	}).Error)
	require.NoError(t, model.DB.Create(&model.QuizBankBinding{
		SiteId: "quiz-test", BankId: exact.Id, Platform: "qq", GroupId: "room-a", Enabled: true, Priority: 1, CreatedAt: now, UpdatedAt: now,
	}).Error)

	resolved, err := quizResolveBankTx(model.DB, "quiz-test", "qq", "room-a", map[string]any{})
	require.NoError(t, err)
	require.Equal(t, exact.Id, resolved.Id)

	first := QuizQuestionContentHash("Capital of France?", []string{"Berlin", "Paris", "Rome"}, 1)
	second := QuizQuestionContentHash(" capital of france? ", []string{"Rome", "Berlin", "Paris"}, 2)
	require.Equal(t, first, second, "semantically identical questions must deduplicate after option reordering")
	require.NotEqual(t, first, QuizQuestionContentHash("Capital of France?", []string{"Berlin", "Paris", "Rome"}, 0))
}

func TestOpsQuizBindingCanChangePlatformAndGroup(t *testing.T) {
	setupQuizBankTest(t)
	bank := seedQuizBank(t, "binding-edit", "editable binding")
	enabled := true
	created, err := SaveOpsQuizBinding("quiz-test", OpsQuizBindingInput{
		BankId: bank.Id, Platform: "qq", GroupId: "old-room", Enabled: &enabled, Priority: 10,
	})
	require.NoError(t, err)

	disabled := false
	updated, err := SaveOpsQuizBinding("quiz-test", OpsQuizBindingInput{
		Id: created.Id, BankId: bank.Id, Platform: "tg", GroupId: "new-room",
		Enabled: &disabled, Priority: 20, Rules: map[string]any{"max_attempts_per_question": 3},
	})
	require.NoError(t, err)
	require.Equal(t, created.Id, updated.Id)
	require.Equal(t, "tg", updated.Platform)
	require.Equal(t, "new-room", updated.GroupId)
	require.False(t, updated.Enabled)
	require.Equal(t, 20, updated.Priority)

	var count int64
	require.NoError(t, model.DB.Model(&model.QuizBankBinding{}).Where("site_id = ?", "quiz-test").Count(&count).Error)
	require.Equal(t, int64(1), count)
}

func TestOpsQuizQuestionListIncludesCategoryMetadata(t *testing.T) {
	setupQuizBankTest(t)
	bank := seedQuizBank(t, "category-list", "categorized question")
	index := 1
	_, err := SaveOpsQuizQuestion("quiz-test", quizTestUserID, bank.Id, 0, OpsQuizQuestionInput{
		ExternalKey: "category-question", CategoryCode: "science", CategoryName: "Science",
		Prompt: "Water formula?", Options: []string{"CO2", "H2O"}, CorrectIndex: &index,
		Status: "published", Weight: 100,
	})
	require.NoError(t, err)

	result, err := ListOpsQuizQuestions("quiz-test", bank.Id, "", "category-question", 0, 20)
	require.NoError(t, err)
	items := result["items"].([]map[string]any)
	require.Len(t, items, 1)
	category := items[0]["category"].(model.QuizCategory)
	require.Equal(t, "science", category.Code)
	require.Equal(t, "Science", category.Name)
}

func TestQuizRewardUsesGameMembershipGuard(t *testing.T) {
	setupQuizBankTest(t)
	membership := operation_setting.GetMembershipRiskSetting()
	membership.Enabled = true
	membership.DryRun = false
	membership.BlockGameRewardOnLeft = true
	membership.BlockCheckinOnLeft = false
	require.True(t, membershipBenefitGuardEnabled(membership, "game_reward"))
	require.False(t, membershipBenefitGuardEnabled(membership, "checkin"))
	require.Equal(t, "game_reward", normalizeMembershipBenefitType("game_reward"))
}

func TestQuizRewardMutationIsIdempotent(t *testing.T) {
	setupQuizBankTest(t)
	seedQuizBank(t, "reward", "reward question")
	payload := quizTestPayload("reward-a")
	payload["reward_quota"] = 1000
	draw, err := quizQuestionDraw(quizTestAction("reward-a"), payload)
	require.NoError(t, err)
	correctIndex := shownOptionIndex(t, draw, "correct-0")
	answerPayload := map[string]any{
		"draw_id": quizResultInt(t, draw, "draw_id"), "user_id": quizTestUserID,
		"user_external_id": "qq-user-910001", "answer_index": correctIndex,
	}

	first, err := quizAnswerSubmit(quizTestAction("reward-answer-a"), answerPayload)
	require.NoError(t, err)
	require.True(t, first["correct"].(bool))
	replayed, err := quizAnswerSubmit(quizTestAction("reward-answer-b"), answerPayload)
	require.NoError(t, err)
	require.True(t, replayed["already_answered"].(bool))

	var user model.User
	require.NoError(t, model.DB.First(&user, quizTestUserID).Error)
	require.Equal(t, 1000, user.Quota)
	var mutations int64
	require.NoError(t, model.DB.Model(&model.AgentBudgetTransaction{}).
		Where("idempotency_key = ?", fmt.Sprintf("quiz-reward:%d:%d", quizResultInt(t, draw, "draw_id"), quizTestUserID)).
		Count(&mutations).Error)
	require.Equal(t, int64(1), mutations)
}
