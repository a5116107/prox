package service

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func executeQuizQuestionDraw(action *model.AgentAction, payload map[string]any) (map[string]any, error) {
	return quizQuestionDraw(action, payload)
}

func executeQuizRoundLoad(action *model.AgentAction, payload map[string]any) (map[string]any, error) {
	return quizRoundLoad(action, payload)
}

func executeQuizAnswerSubmit(action *model.AgentAction, payload map[string]any) (map[string]any, error) {
	return quizAnswerSubmit(action, payload)
}

func quizQuestionDraw(action *model.AgentAction, payload map[string]any) (map[string]any, error) {
	now := time.Now()
	siteID := AgentSiteID()
	platform := agentQuizPlatform(payload)
	groupID := agentQuizGroupID(payload)
	scopeMode := agentQuizScopeMode(payload)
	userID := int(agentPayloadFloat(payload, "user_id", "new_api_user_id", "resolved_new_api_user_id"))
	externalUserID := firstAgentNonEmpty(agentPayloadString(payload, "target_external_id", "external_user_id", "user_external_id"), action.TargetId)
	scopeKey := quizScopeKey(platform, groupID, scopeMode, userID, externalUserID)
	businessDate := model.AgentBusinessDateAt(now)
	drawKey := firstAgentNonEmpty(agentPayloadString(payload, "draw_key", "idempotency_key"), action.IdempotencyKey)
	if drawKey == "" {
		drawKey = quizStableKey(siteID, scopeKey, strconv.FormatInt(now.UnixNano(), 10))
	}
	ttlSeconds := quizBoundedInt(payload, 600, 60, 86400, "question_ttl_seconds", "ttl_seconds")
	maxAttempts := quizBoundedInt(payload, 2, 1, 20, "max_attempts_per_question", "max_attempts")
	rewardQuota := quizBoundedInt(payload, 100000, 0, 1000000000, "reward_quota", "quota")
	groupLimit := quizBoundedInt(payload, 0, 0, 100000, "quiz_limit_per_group", "question_count")
	userLimit := quizBoundedInt(payload, 10, 0, 100000, "max_per_user_day")
	rules := map[string]any{
		"max_attempts":     maxAttempts,
		"reward_quota":     rewardQuota,
		"budget_pool":      firstAgentNonEmpty(agentPayloadString(payload, "budget_pool", "pool"), "activity"),
		"close_on_correct": agentBoolPayload(payload, "close_on_correct") || scopeMode != "per_group",
		"max_winners":      quizBoundedInt(payload, 0, 0, 100000, "max_winners_per_question", "max_winners"),
	}

	var result map[string]any
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		if existing, err := quizFindActiveDrawTx(tx, siteID, scopeKey, now.Unix()); err == nil {
			if err := quizEnsureEntryTx(tx, siteID, existing.RoundId, userID, externalUserID, now.Unix()); err != nil {
				return err
			}
			result, err = quizDrawResponseTx(tx, existing, userID, externalUserID)
			return err
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		if userLimit > 0 {
			count, err := quizTodayUserRoundsTx(tx, siteID, platform, groupID, userID, externalUserID, businessDate)
			if err != nil {
				return err
			}
			if count >= int64(userLimit) {
				return fmt.Errorf("quiz user daily limit reached: %d", userLimit)
			}
		}
		if groupLimit > 0 {
			var count int64
			if err := tx.Model(&model.QuizQuestionDraw{}).Where("site_id = ? AND platform = ? AND group_id = ? AND business_date = ?", siteID, platform, groupID, businessDate).Count(&count).Error; err != nil {
				return err
			}
			if count >= int64(groupLimit) {
				return fmt.Errorf("quiz group daily limit reached: %d", groupLimit)
			}
		}

		bank, err := quizResolveBankTx(tx, siteID, platform, groupID, payload)
		if err != nil {
			return err
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", bank.Id).First(&bank).Error; err != nil {
			return err
		}
		// A concurrent draw can commit while this transaction waits for the bank
		// lock. Recheck after acquiring it so a scope has only one open round.
		if existing, err := quizFindActiveDrawTx(tx, siteID, scopeKey, now.Unix()); err == nil {
			if err := quizEnsureEntryTx(tx, siteID, existing.RoundId, userID, externalUserID, now.Unix()); err != nil {
				return err
			}
			result, err = quizDrawResponseTx(tx, existing, userID, externalUserID)
			return err
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		question, err := quizSelectQuestionTx(tx, bank.Id, scopeKey, businessDate, drawKey)
		if err != nil {
			return err
		}
		options, err := quizDecodeOptions(question.OptionsJson)
		if err != nil {
			return err
		}
		optionOrder := quizDeterministicOrder(len(options), drawKey+":"+strconv.Itoa(question.Id))
		orderJSON, _ := json.Marshal(optionOrder)
		rulesJSON, _ := json.Marshal(rules)
		roundKey := "quiz:" + quizStableKey(siteID, scopeKey, drawKey)[:32]
		round := model.GameRound{
			SiteId: siteID, Platform: platform, GroupId: groupID, GameCode: "quiz", RoundKey: roundKey,
			Status: "open", RandomSeed: drawKey, RuleJson: string(rulesJSON), ResultJson: "{}",
			OpenedAt: now.Unix(), CreatedAt: now.Unix(), UpdatedAt: now.Unix(),
		}
		if err := tx.Create(&round).Error; err != nil {
			return err
		}
		draw := model.QuizQuestionDraw{
			SiteId: siteID, BankId: bank.Id, QuestionId: question.Id, RoundId: round.Id, DrawKey: drawKey,
			Platform: platform, GroupId: groupID, ScopeMode: scopeMode, ScopeKey: scopeKey,
			BusinessDate: businessDate, OptionOrderJson: string(orderJSON), RulesJson: string(rulesJSON),
			Status: "open", ExpiresAt: now.Add(time.Duration(ttlSeconds) * time.Second).Unix(),
			CreatedAt: now.Unix(), UpdatedAt: now.Unix(),
		}
		if err := tx.Create(&draw).Error; err != nil {
			return err
		}
		if err := quizEnsureEntryTx(tx, siteID, round.Id, userID, externalUserID, now.Unix()); err != nil {
			return err
		}
		result, err = quizDrawResponseTx(tx, &draw, userID, externalUserID)
		return err
	})
	return result, err
}

func quizRoundLoad(action *model.AgentAction, payload map[string]any) (map[string]any, error) {
	now := time.Now().Unix()
	siteID := AgentSiteID()
	platform := agentQuizPlatform(payload)
	groupID := agentQuizGroupID(payload)
	scopeMode := agentQuizScopeMode(payload)
	userID := int(agentPayloadFloat(payload, "user_id", "new_api_user_id", "resolved_new_api_user_id"))
	externalUserID := firstAgentNonEmpty(agentPayloadString(payload, "target_external_id", "external_user_id", "user_external_id"), action.TargetId)
	scopeKey := quizScopeKey(platform, groupID, scopeMode, userID, externalUserID)
	var result map[string]any
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		draw, err := quizFindActiveDrawTx(tx, siteID, scopeKey, now)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			result = map[string]any{"ok": true, "active": false, "scope_mode": scopeMode}
			return nil
		}
		if err != nil {
			return err
		}
		result, err = quizDrawResponseTx(tx, draw, userID, externalUserID)
		return err
	})
	return result, err
}

func quizAnswerSubmit(action *model.AgentAction, payload map[string]any) (map[string]any, error) {
	now := time.Now().Unix()
	siteID := AgentSiteID()
	userID := int(agentPayloadFloat(payload, "user_id", "new_api_user_id", "resolved_new_api_user_id"))
	externalUserID := firstAgentNonEmpty(agentPayloadString(payload, "target_external_id", "external_user_id", "user_external_id"), action.TargetId)
	drawID := int(agentPayloadFloat(payload, "draw_id", "question_draw_id"))
	roundKey := strings.TrimSpace(agentPayloadString(payload, "round_key"))
	var result map[string]any
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		var draw model.QuizQuestionDraw
		query := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("site_id = ?", siteID)
		if drawID > 0 {
			query = query.Where("id = ?", drawID)
		} else if roundKey != "" {
			query = query.Joins("JOIN game_rounds gr ON gr.id = quiz_question_draws.round_id").Where("gr.round_key = ?", roundKey)
		} else {
			return errors.New("quiz draw_id or round_key is required")
		}
		if err := query.First(&draw).Error; err != nil {
			return err
		}
		if draw.Status != "open" || draw.ExpiresAt <= now {
			if entry, err := quizFindEntryForUpdateTx(tx, siteID, draw.RoundId, userID, externalUserID); err == nil {
				switch entry.Status {
				case "answered":
					result = map[string]any{"ok": true, "correct": true, "already_answered": true, "draw_id": draw.Id, "round_key": quizRoundKeyTx(tx, draw.RoundId)}
					return nil
				case "locked":
					result = map[string]any{"ok": true, "correct": false, "locked": true, "closed": true, "draw_id": draw.Id, "round_key": quizRoundKeyTx(tx, draw.RoundId)}
					return nil
				}
			} else if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			if draw.Status == "open" && draw.ExpiresAt <= now {
				_ = quizCloseDrawTx(tx, &draw, "expired", now, map[string]any{"status": "expired"})
			}
			return errors.New("quiz round is closed or expired")
		}

		var question model.QuizQuestion
		if err := tx.Where("id = ? AND status = ?", draw.QuestionId, "published").First(&question).Error; err != nil {
			return err
		}
		options, err := quizDecodeOptions(question.OptionsJson)
		if err != nil {
			return err
		}
		var order []int
		if err := json.Unmarshal([]byte(draw.OptionOrderJson), &order); err != nil || len(order) != len(options) {
			return errors.New("quiz option order is invalid")
		}
		shownIndex, err := quizAnswerShownIndex(payload, options, order)
		if err != nil {
			return err
		}
		correctShownIndex := -1
		for index, original := range order {
			if original == question.CorrectIndex {
				correctShownIndex = index
				break
			}
		}
		if correctShownIndex < 0 {
			return errors.New("quiz correct option is invalid")
		}
		isCorrect := shownIndex == correctShownIndex
		if isCorrect && userID <= 0 {
			result = map[string]any{"ok": true, "correct": true, "requires_binding": true, "draw_id": draw.Id, "round_key": roundKey}
			return nil
		}

		entry, payloadMap, err := quizLoadEntryForUpdateTx(tx, siteID, draw.RoundId, userID, externalUserID, now)
		if err != nil {
			return err
		}
		if entry.Status == "answered" {
			result = map[string]any{"ok": true, "correct": true, "already_answered": true, "draw_id": draw.Id, "round_key": quizRoundKeyTx(tx, draw.RoundId)}
			return nil
		}
		if entry.Status == "locked" {
			result = map[string]any{"ok": true, "correct": false, "locked": true, "draw_id": draw.Id, "round_key": quizRoundKeyTx(tx, draw.RoundId), "correct_option_index": correctShownIndex}
			return nil
		}

		rules := map[string]any{}
		_ = json.Unmarshal([]byte(draw.RulesJson), &rules)
		maxAttempts := quizMapInt(rules, 2, "max_attempts")
		attempts := quizMapInt(payloadMap, 0, "attempts") + 1
		remaining := maxAttempts - attempts
		payloadMap["attempts"] = attempts
		payloadMap["last_answer_index"] = shownIndex
		payloadMap["updated_at"] = now
		entryStatus := "active"
		rewardQuota := 0
		closed := false

		if isCorrect {
			if ok, reason := CanReceiveCommunityBenefit(userID, draw.Platform, draw.GroupId, externalUserID, "game_reward"); !ok {
				return fmt.Errorf("quiz reward blocked: %s", reason)
			}
			entryStatus = "answered"
			payloadMap["answered"] = true
			payloadMap["answered_at"] = now
			rewardQuota = quizMapInt(rules, 0, "reward_quota")
			if rewardQuota > 0 {
				pool := firstAgentNonEmpty(quizMapString(rules, "budget_pool"), "activity")
				meta, _ := json.Marshal(map[string]any{"game_code": "quiz", "draw_id": draw.Id, "question_id": draw.QuestionId, "platform": draw.Platform, "group_id": draw.GroupId})
				idempotencyKey := fmt.Sprintf("quiz-reward:%d:%d", draw.Id, userID)
				if err := model.ApplyQuotaMutationTx(tx, siteID, userID, rewardQuota, pool, "quiz_reward", idempotencyKey, "quiz_correct", action.Id, strconv.Itoa(draw.RoundId), 0, string(meta)); err != nil {
					return err
				}
			}
			closeOnCorrect := quizMapBool(rules, "close_on_correct") || draw.ScopeMode != "per_group"
			maxWinners := quizMapInt(rules, 0, "max_winners")
			if maxWinners > 0 {
				var winners int64
				if err := tx.Model(&model.GameEntry{}).Where("round_id = ? AND status = ?", draw.RoundId, "answered").Count(&winners).Error; err != nil {
					return err
				}
				closeOnCorrect = closeOnCorrect || winners+1 >= int64(maxWinners)
			}
			if closeOnCorrect {
				closed = true
			}
		} else if remaining <= 0 {
			entryStatus = "locked"
			payloadMap["locked"] = true
			payloadMap["locked_at"] = now
			remaining = 0
			closed = draw.ScopeMode != "per_group"
		}

		entryJSON, _ := json.Marshal(payloadMap)
		if err := tx.Model(&model.GameEntry{}).Where("id = ?", entry.Id).Updates(map[string]any{"status": entryStatus, "entry_payload_json": string(entryJSON), "updated_at": now}).Error; err != nil {
			return err
		}
		if closed {
			status := "answered"
			if !isCorrect {
				status = "locked"
			}
			if err := quizCloseDrawTx(tx, &draw, status, now, map[string]any{"status": status, "winner_user_id": userID}); err != nil {
				return err
			}
		}

		result = map[string]any{
			"ok": true, "correct": isCorrect, "draw_id": draw.Id, "round_key": quizRoundKeyTx(tx, draw.RoundId),
			"attempts": attempts, "remaining_attempts": remaining, "locked": entryStatus == "locked",
			"closed": closed, "reward_quota": rewardQuota,
		}
		if isCorrect || entryStatus == "locked" {
			result["correct_option_index"] = correctShownIndex
			result["correct_answer"] = options[question.CorrectIndex]
			result["explanation"] = question.Explanation
		}
		return nil
	})
	return result, err
}

func quizResolveBankTx(tx *gorm.DB, siteID, platform, groupID string, payload map[string]any) (model.QuizBank, error) {
	var bank model.QuizBank
	bankID := int(agentPayloadFloat(payload, "bank_id"))
	bankCode := strings.TrimSpace(agentPayloadString(payload, "bank_code"))
	if bankID > 0 {
		return bank, tx.Where("id = ? AND site_id = ? AND status = ?", bankID, siteID, "published").First(&bank).Error
	}
	if bankCode != "" {
		return bank, tx.Where("site_id = ? AND code = ? AND status = ?", siteID, bankCode, "published").First(&bank).Error
	}
	var binding model.QuizBankBinding
	err := tx.Raw(`
		SELECT * FROM quiz_bank_bindings
		WHERE site_id = ? AND enabled = ?
		  AND (platform = ? OR platform = '*')
		  AND (group_id = ? OR group_id = '*' OR group_id = '')
		ORDER BY CASE WHEN platform = ? THEN 0 ELSE 1 END,
		         CASE WHEN group_id = ? THEN 0 WHEN group_id = '*' THEN 1 ELSE 2 END,
		         priority DESC, id ASC
		LIMIT 1`, siteID, true, platform, groupID, platform, groupID).Scan(&binding).Error
	if err == nil && binding.Id == 0 {
		err = gorm.ErrRecordNotFound
	}
	if err == nil {
		err = tx.Where("id = ? AND site_id = ? AND status = ?", binding.BankId, siteID, "published").First(&bank).Error
		return bank, err
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return bank, err
	}
	err = tx.Where("site_id = ? AND status = ?", siteID, "published").Order("id asc").First(&bank).Error
	return bank, err
}

func quizSelectQuestionTx(tx *gorm.DB, bankID int, scopeKey, businessDate, seed string) (model.QuizQuestion, error) {
	var questions []model.QuizQuestion
	if err := tx.Where("bank_id = ? AND status = ? AND weight > 0", bankID, "published").Order("id asc").Find(&questions).Error; err != nil {
		return model.QuizQuestion{}, err
	}
	if len(questions) == 0 {
		return model.QuizQuestion{}, errors.New("quiz bank has no published questions")
	}
	var used []int
	if err := tx.Model(&model.QuizQuestionDraw{}).Where("bank_id = ? AND scope_key = ? AND business_date = ?", bankID, scopeKey, businessDate).Pluck("question_id", &used).Error; err != nil {
		return model.QuizQuestion{}, err
	}
	usedSet := map[int]bool{}
	for _, id := range used {
		usedSet[id] = true
	}
	candidates := make([]model.QuizQuestion, 0, len(questions))
	for _, question := range questions {
		if !usedSet[question.Id] {
			candidates = append(candidates, question)
		}
	}
	if len(candidates) == 0 {
		candidates = questions
	}
	index := int(quizSeedUint64(seed+":"+scopeKey+":"+strconv.Itoa(len(used))) % uint64(len(candidates)))
	return candidates[index], nil
}

func quizFindActiveDrawTx(tx *gorm.DB, siteID, scopeKey string, now int64) (*model.QuizQuestionDraw, error) {
	var draw model.QuizQuestionDraw
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("site_id = ? AND scope_key = ? AND status = ? AND expires_at > ?", siteID, scopeKey, "open", now).Order("id desc").First(&draw).Error
	return &draw, err
}

func quizEnsureEntryTx(tx *gorm.DB, siteID string, roundID, userID int, externalUserID string, now int64) error {
	_, _, err := quizLoadEntryForUpdateTx(tx, siteID, roundID, userID, externalUserID, now)
	return err
}

func quizFindEntryForUpdateTx(tx *gorm.DB, siteID string, roundID, userID int, externalUserID string) (*model.GameEntry, error) {
	var entry model.GameEntry
	query := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("site_id = ? AND round_id = ?", siteID, roundID)
	switch {
	case userID > 0 && externalUserID != "":
		query = query.Where("user_id = ? OR external_user_id = ?", userID, externalUserID)
	case userID > 0:
		query = query.Where("user_id = ?", userID)
	default:
		query = query.Where("external_user_id = ?", externalUserID)
	}
	err := query.Order("id desc").First(&entry).Error
	return &entry, err
}

func quizLoadEntryForUpdateTx(tx *gorm.DB, siteID string, roundID, userID int, externalUserID string, now int64) (*model.GameEntry, map[string]any, error) {
	entry, err := quizFindEntryForUpdateTx(tx, siteID, roundID, userID, externalUserID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		entry = &model.GameEntry{SiteId: siteID, RoundId: roundID, UserId: userID, ExternalUserId: externalUserID, StakeQuota: 0, EntryPayloadJson: "{\"attempts\":0}", Status: "active", CreatedAt: now, UpdatedAt: now}
		if err := tx.Create(entry).Error; err != nil {
			return nil, nil, err
		}
	} else if err != nil {
		return nil, nil, err
	}
	payload := map[string]any{}
	_ = json.Unmarshal([]byte(entry.EntryPayloadJson), &payload)
	return entry, payload, nil
}

func quizDrawResponseTx(tx *gorm.DB, draw *model.QuizQuestionDraw, userID int, externalUserID string) (map[string]any, error) {
	var question model.QuizQuestion
	if err := tx.Where("id = ?", draw.QuestionId).First(&question).Error; err != nil {
		return nil, err
	}
	options, err := quizDecodeOptions(question.OptionsJson)
	if err != nil {
		return nil, err
	}
	var order []int
	if err := json.Unmarshal([]byte(draw.OptionOrderJson), &order); err != nil || len(order) != len(options) {
		return nil, errors.New("quiz option order is invalid")
	}
	shown := make([]string, len(order))
	for index, original := range order {
		if original < 0 || original >= len(options) {
			return nil, errors.New("quiz option order is out of range")
		}
		shown[index] = options[original]
	}
	entry, payload, err := quizLoadEntryForUpdateTx(tx, draw.SiteId, draw.RoundId, userID, externalUserID, time.Now().Unix())
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"ok": true, "active": true, "draw_id": draw.Id, "round_key": quizRoundKeyTx(tx, draw.RoundId),
		"scope_mode": draw.ScopeMode, "expires_at": draw.ExpiresAt,
		"question": map[string]any{"id": question.Id, "external_key": question.ExternalKey, "prompt": question.Prompt, "options": shown, "difficulty": question.Difficulty, "language": question.Language},
		"entry":    map[string]any{"id": entry.Id, "status": entry.Status, "attempts": quizMapInt(payload, 0, "attempts")},
	}, nil
}

func quizCloseDrawTx(tx *gorm.DB, draw *model.QuizQuestionDraw, status string, now int64, result map[string]any) error {
	resultJSON, _ := json.Marshal(result)
	if err := tx.Model(&model.QuizQuestionDraw{}).Where("id = ? AND status = ?", draw.Id, "open").Updates(map[string]any{"status": status, "answered_at": now, "updated_at": now}).Error; err != nil {
		return err
	}
	return tx.Model(&model.GameRound{}).Where("id = ?", draw.RoundId).Updates(map[string]any{"status": "closed", "closed_at": now, "result_json": string(resultJSON), "updated_at": now}).Error
}

func quizTodayUserRoundsTx(tx *gorm.DB, siteID, platform, groupID string, userID int, externalUserID, businessDate string) (int64, error) {
	query := tx.Model(&model.QuizQuestionDraw{}).Distinct("quiz_question_draws.id").Joins("JOIN game_entries ge ON ge.round_id = quiz_question_draws.round_id").Where("quiz_question_draws.site_id = ? AND quiz_question_draws.platform = ? AND quiz_question_draws.group_id = ? AND quiz_question_draws.business_date = ?", siteID, platform, groupID, businessDate)
	if userID > 0 && externalUserID != "" {
		query = query.Where("ge.user_id = ? OR ge.external_user_id = ?", userID, externalUserID)
	} else if userID > 0 {
		query = query.Where("ge.user_id = ?", userID)
	} else {
		query = query.Where("ge.external_user_id = ?", externalUserID)
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func quizScopeKey(platform, groupID, scopeMode string, userID int, externalUserID string) string {
	identity := externalUserID
	if userID > 0 {
		identity = strconv.Itoa(userID)
	}
	if identity == "" {
		identity = "anonymous"
	}
	if scopeMode == "per_group" {
		return strings.Join([]string{platform, groupID, "group"}, ":")
	}
	return strings.Join([]string{platform, groupID, "user", identity}, ":")
}

func quizDecodeOptions(raw string) ([]string, error) {
	var options []string
	if err := json.Unmarshal([]byte(raw), &options); err != nil {
		return nil, err
	}
	if len(options) < 2 || len(options) > 10 {
		return nil, errors.New("quiz question must have 2 to 10 options")
	}
	for index := range options {
		options[index] = strings.TrimSpace(options[index])
		if options[index] == "" {
			return nil, errors.New("quiz option is empty")
		}
	}
	return options, nil
}

func quizAnswerShownIndex(payload map[string]any, options []string, order []int) (int, error) {
	for _, key := range []string{"answer_index", "option_index"} {
		if value, ok := payload[key]; ok {
			index, err := strconv.Atoi(strings.TrimSpace(fmt.Sprint(value)))
			if err == nil && index >= 0 && index < len(order) {
				return index, nil
			}
		}
	}
	answer := strings.TrimSpace(agentPayloadString(payload, "answer_text", "answer"))
	for shownIndex, original := range order {
		if strings.EqualFold(answer, options[original]) {
			return shownIndex, nil
		}
	}
	return -1, errors.New("quiz answer is invalid")
}

func quizDeterministicOrder(size int, seed string) []int {
	order := make([]int, size)
	for index := range order {
		order[index] = index
	}
	for index := size - 1; index > 0; index-- {
		other := int(quizSeedUint64(seed+":"+strconv.Itoa(index)) % uint64(index+1))
		order[index], order[other] = order[other], order[index]
	}
	return order
}

func quizSeedUint64(value string) uint64 {
	sum := sha256.Sum256([]byte(value))
	return binary.BigEndian.Uint64(sum[:8])
}

func quizStableKey(values ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(values, "\x00")))
	return hex.EncodeToString(sum[:])
}

func quizBoundedInt(payload map[string]any, fallback, minimum, maximum int, keys ...string) int {
	value := fallback
	for _, key := range keys {
		if raw, ok := payload[key]; ok {
			if parsed, err := strconv.Atoi(strings.TrimSpace(fmt.Sprint(raw))); err == nil {
				value = parsed
				break
			}
		}
	}
	if value < minimum {
		value = minimum
	}
	if value > maximum {
		value = maximum
	}
	return value
}

func quizMapInt(values map[string]any, fallback int, keys ...string) int {
	return quizBoundedInt(values, fallback, -1000000000, 1000000000, keys...)
}

func quizMapString(values map[string]any, key string) string {
	return strings.TrimSpace(fmt.Sprint(values[key]))
}

func quizMapBool(values map[string]any, key string) bool {
	value := strings.ToLower(strings.TrimSpace(fmt.Sprint(values[key])))
	return value == "true" || value == "1" || value == "yes" || value == "on"
}

func quizRoundKeyTx(tx *gorm.DB, roundID int) string {
	var round model.GameRound
	if err := tx.Select("round_key").Where("id = ?", roundID).First(&round).Error; err != nil {
		return ""
	}
	return round.RoundKey
}

func QuizQuestionContentHash(prompt string, options []string, correctIndex int) string {
	normalized := make([]string, len(options))
	correctAnswer := ""
	if correctIndex >= 0 && correctIndex < len(options) {
		correctAnswer = strings.ToLower(strings.TrimSpace(options[correctIndex]))
	}
	for index, option := range options {
		normalized[index] = strings.ToLower(strings.TrimSpace(option))
	}
	sort.Strings(normalized)
	return quizStableKey(strings.ToLower(strings.TrimSpace(prompt)), strings.Join(normalized, "\x00"), correctAnswer)
}
