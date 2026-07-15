package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type OpsQuizBankInput struct {
	Code            string `json:"code"`
	Name            string `json:"name"`
	Description     string `json:"description"`
	DefaultLanguage string `json:"default_language"`
	Status          string `json:"status"`
}

type OpsQuizQuestionInput struct {
	ExternalKey  string   `json:"external_key"`
	CategoryCode string   `json:"category_code"`
	CategoryName string   `json:"category_name"`
	Prompt       string   `json:"prompt"`
	Question     string   `json:"question"`
	Options      []string `json:"options"`
	CorrectIndex *int     `json:"correct_index"`
	AnswerIndex  *int     `json:"answer_index"`
	Answer       string   `json:"answer"`
	Explanation  string   `json:"explanation"`
	Difficulty   string   `json:"difficulty"`
	Language     string   `json:"language"`
	Status       string   `json:"status"`
	Weight       int      `json:"weight"`
	Source       string   `json:"source"`
}

type OpsQuizImportRequest struct {
	DryRun    bool                   `json:"dry_run"`
	Publish   bool                   `json:"publish"`
	Questions []OpsQuizQuestionInput `json:"questions"`
}

type OpsQuizBindingInput struct {
	Id        int            `json:"id"`
	BankId    int            `json:"bank_id"`
	Platform  string         `json:"platform"`
	GroupId   string         `json:"group_id"`
	Enabled   *bool          `json:"enabled"`
	Priority  int            `json:"priority"`
	Rules     map[string]any `json:"rules"`
	RulesJson string         `json:"rules_json"`
}

func ListOpsQuizBanks(siteID string) ([]map[string]any, error) {
	siteID = normalizeOpsSiteID(siteID)
	var banks []model.QuizBank
	if err := model.DB.Where("site_id = ?", siteID).Order("id asc").Find(&banks).Error; err != nil {
		return nil, err
	}
	items := make([]map[string]any, 0, len(banks))
	for _, bank := range banks {
		var total, published int64
		_ = model.DB.Model(&model.QuizQuestion{}).Where("bank_id = ?", bank.Id).Count(&total).Error
		_ = model.DB.Model(&model.QuizQuestion{}).Where("bank_id = ? AND status = ?", bank.Id, "published").Count(&published).Error
		items = append(items, map[string]any{"bank": bank, "question_count": total, "published_question_count": published})
	}
	return items, nil
}

func CreateOpsQuizBank(siteID string, actorID int, input OpsQuizBankInput) (*model.QuizBank, error) {
	now := time.Now().Unix()
	bank := model.QuizBank{SiteId: normalizeOpsSiteID(siteID), Code: quizSlug(input.Code), Name: strings.TrimSpace(input.Name), Description: strings.TrimSpace(input.Description), DefaultLanguage: quizDefault(input.DefaultLanguage, "zh-CN"), Status: quizBankStatus(input.Status), CreatedBy: actorID, CreatedAt: now, UpdatedAt: now}
	if bank.Code == "" || bank.Name == "" {
		return nil, errors.New("quiz bank code and name are required")
	}
	if err := model.DB.Create(&bank).Error; err != nil {
		return nil, err
	}
	return &bank, nil
}

func UpdateOpsQuizBank(siteID string, bankID int, input OpsQuizBankInput) (*model.QuizBank, error) {
	var bank model.QuizBank
	if err := model.DB.Where("id = ? AND site_id = ?", bankID, normalizeOpsSiteID(siteID)).First(&bank).Error; err != nil {
		return nil, err
	}
	updates := map[string]any{"updated_at": time.Now().Unix()}
	if code := quizSlug(input.Code); code != "" {
		updates["code"] = code
	}
	if name := strings.TrimSpace(input.Name); name != "" {
		updates["name"] = name
	}
	updates["description"] = strings.TrimSpace(input.Description)
	if input.DefaultLanguage != "" {
		updates["default_language"] = strings.TrimSpace(input.DefaultLanguage)
	}
	if input.Status != "" {
		updates["status"] = quizBankStatus(input.Status)
	}
	if err := model.DB.Model(&bank).Updates(updates).Error; err != nil {
		return nil, err
	}
	return &bank, model.DB.Where("id = ?", bank.Id).First(&bank).Error
}

func PublishOpsQuizBank(siteID string, bankID int) (*model.QuizBank, error) {
	var count int64
	if err := model.DB.Model(&model.QuizQuestion{}).Where("bank_id = ? AND status = ?", bankID, "published").Count(&count).Error; err != nil {
		return nil, err
	}
	if count == 0 {
		return nil, errors.New("quiz bank has no published questions")
	}
	return UpdateOpsQuizBank(siteID, bankID, OpsQuizBankInput{Status: "published"})
}

func ListOpsQuizQuestions(siteID string, bankID int, status, search string, offset, limit int) (map[string]any, error) {
	if err := quizRequireBank(siteID, bankID); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	query := model.DB.Model(&model.QuizQuestion{}).Where("bank_id = ?", bankID)
	if status = strings.TrimSpace(status); status != "" {
		query = query.Where("status = ?", status)
	}
	if search = strings.TrimSpace(search); search != "" {
		like := "%" + search + "%"
		query = query.Where("prompt LIKE ? OR external_key LIKE ?", like, like)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}
	var rows []model.QuizQuestion
	if err := query.Order("id desc").Offset(offset).Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	categoryIDs := make([]int, 0, len(rows))
	for _, row := range rows {
		if row.CategoryId > 0 {
			categoryIDs = append(categoryIDs, row.CategoryId)
		}
	}
	categories := make([]model.QuizCategory, 0)
	if len(categoryIDs) > 0 {
		if err := model.DB.Where("id IN ? AND bank_id = ?", categoryIDs, bankID).Find(&categories).Error; err != nil {
			return nil, err
		}
	}
	categoryByID := make(map[int]model.QuizCategory, len(categories))
	for _, category := range categories {
		categoryByID[category.Id] = category
	}
	items := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		options, _ := quizDecodeOptions(row.OptionsJson)
		items = append(items, map[string]any{"question": row, "options": options, "category": categoryByID[row.CategoryId]})
	}
	return map[string]any{"items": items, "total": total, "offset": offset, "limit": limit}, nil
}

func SaveOpsQuizQuestion(siteID string, actorID, bankID, questionID int, input OpsQuizQuestionInput) (*model.QuizQuestion, error) {
	if err := quizRequireBank(siteID, bankID); err != nil {
		return nil, err
	}
	normalized, categoryCode, categoryName, err := normalizeOpsQuizQuestion(input, actorID)
	if err != nil {
		return nil, err
	}
	now := time.Now().Unix()
	var saved model.QuizQuestion
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		categoryID, err := ensureQuizCategoryTx(tx, bankID, categoryCode, categoryName, now)
		if err != nil {
			return err
		}
		normalized.BankId = bankID
		normalized.CategoryId = categoryID
		normalized.UpdatedAt = now
		if questionID <= 0 {
			normalized.CreatedAt = now
			if err := tx.Create(&normalized).Error; err != nil {
				return err
			}
			saved = normalized
			return nil
		}
		var existing model.QuizQuestion
		if err := tx.Where("id = ? AND bank_id = ?", questionID, bankID).First(&existing).Error; err != nil {
			return err
		}
		updates := map[string]any{"category_id": categoryID, "external_key": normalized.ExternalKey, "prompt": normalized.Prompt, "options_json": normalized.OptionsJson, "correct_index": normalized.CorrectIndex, "explanation": normalized.Explanation, "difficulty": normalized.Difficulty, "language": normalized.Language, "status": normalized.Status, "weight": normalized.Weight, "source": normalized.Source, "content_hash": normalized.ContentHash, "updated_at": now}
		if err := tx.Model(&existing).Updates(updates).Error; err != nil {
			return err
		}
		return tx.Where("id = ?", existing.Id).First(&saved).Error
	})
	return &saved, err
}

func ImportOpsQuizQuestions(siteID string, actorID, bankID int, request OpsQuizImportRequest) (map[string]any, error) {
	if err := quizRequireBank(siteID, bankID); err != nil {
		return nil, err
	}
	if len(request.Questions) == 0 || len(request.Questions) > 5000 {
		return nil, errors.New("quiz import requires 1 to 5000 questions")
	}
	type preparedQuestion struct {
		Question     model.QuizQuestion
		CategoryCode string
		CategoryName string
	}
	prepared := make([]preparedQuestion, 0, len(request.Questions))
	seen := map[string]bool{}
	for index, input := range request.Questions {
		question, categoryCode, categoryName, err := normalizeOpsQuizQuestion(input, actorID)
		if err != nil {
			return nil, fmt.Errorf("question %d: %w", index+1, err)
		}
		if request.Publish {
			question.Status = "published"
		}
		if seen[question.ContentHash] {
			continue
		}
		seen[question.ContentHash] = true
		prepared = append(prepared, preparedQuestion{Question: question, CategoryCode: categoryCode, CategoryName: categoryName})
	}
	result := map[string]any{"received": len(request.Questions), "valid": len(prepared), "created": 0, "updated": 0, "skipped_duplicates": len(request.Questions) - len(prepared), "dry_run": request.DryRun}
	if request.DryRun {
		return result, nil
	}
	now := time.Now().Unix()
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		for _, item := range prepared {
			question := item.Question
			question.BankId = bankID
			question.CreatedAt = now
			question.UpdatedAt = now
			categoryID, err := ensureQuizCategoryTx(tx, bankID, item.CategoryCode, item.CategoryName, now)
			if err != nil {
				return err
			}
			question.CategoryId = categoryID
			var existing model.QuizQuestion
			err = tx.Where("bank_id = ? AND (external_key = ? OR content_hash = ?)", bankID, question.ExternalKey, question.ContentHash).First(&existing).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				if err := tx.Create(&question).Error; err != nil {
					return err
				}
				result["created"] = result["created"].(int) + 1
				continue
			}
			if err != nil {
				return err
			}
			updates := map[string]any{"category_id": categoryID, "prompt": question.Prompt, "options_json": question.OptionsJson, "correct_index": question.CorrectIndex, "explanation": question.Explanation, "difficulty": question.Difficulty, "language": question.Language, "status": question.Status, "weight": question.Weight, "source": question.Source, "content_hash": question.ContentHash, "updated_at": now}
			if err := tx.Model(&existing).Updates(updates).Error; err != nil {
				return err
			}
			result["updated"] = result["updated"].(int) + 1
		}
		return nil
	})
	return result, err
}

func SetOpsQuizQuestionStatus(siteID string, bankID, questionID int, status string) error {
	if err := quizRequireBank(siteID, bankID); err != nil {
		return err
	}
	status = quizQuestionStatus(status)
	return model.DB.Model(&model.QuizQuestion{}).Where("id = ? AND bank_id = ?", questionID, bankID).Updates(map[string]any{"status": status, "updated_at": time.Now().Unix()}).Error
}

func ListOpsQuizBindings(siteID string) ([]map[string]any, error) {
	siteID = normalizeOpsSiteID(siteID)
	var bindings []model.QuizBankBinding
	if err := model.DB.Where("site_id = ?", siteID).Order("priority desc, id asc").Find(&bindings).Error; err != nil {
		return nil, err
	}
	items := make([]map[string]any, 0, len(bindings))
	for _, binding := range bindings {
		var bank model.QuizBank
		_ = model.DB.Where("id = ?", binding.BankId).First(&bank).Error
		rules := map[string]any{}
		_ = json.Unmarshal([]byte(binding.RulesJson), &rules)
		items = append(items, map[string]any{"binding": binding, "bank": bank, "rules": rules})
	}
	return items, nil
}

func SaveOpsQuizBinding(siteID string, input OpsQuizBindingInput) (*model.QuizBankBinding, error) {
	siteID = normalizeOpsSiteID(siteID)
	if err := quizRequireBank(siteID, input.BankId); err != nil {
		return nil, err
	}
	platform := strings.ToLower(strings.TrimSpace(input.Platform))
	if platform == "" {
		platform = "*"
	}
	groupID := strings.TrimSpace(input.GroupId)
	if groupID == "" {
		groupID = "*"
	}
	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
	}
	rulesJSON := strings.TrimSpace(input.RulesJson)
	if input.Rules != nil {
		raw, err := json.Marshal(input.Rules)
		if err != nil {
			return nil, err
		}
		rulesJSON = string(raw)
	}
	if rulesJSON == "" {
		rulesJSON = "{}"
	}
	now := time.Now().Unix()
	if input.Id > 0 {
		var existing model.QuizBankBinding
		if err := model.DB.Where("id = ? AND site_id = ?", input.Id, siteID).First(&existing).Error; err != nil {
			return nil, err
		}
		updates := map[string]any{
			"bank_id": input.BankId, "platform": platform, "group_id": groupID,
			"enabled": enabled, "priority": input.Priority, "rules_json": rulesJSON, "updated_at": now,
		}
		if err := model.DB.Model(&existing).Updates(updates).Error; err != nil {
			return nil, err
		}
		return &existing, model.DB.Where("id = ? AND site_id = ?", existing.Id, siteID).First(&existing).Error
	}
	row := model.QuizBankBinding{SiteId: siteID, BankId: input.BankId, Platform: platform, GroupId: groupID, Enabled: enabled, Priority: input.Priority, RulesJson: rulesJSON, CreatedAt: now, UpdatedAt: now}
	err := model.DB.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "site_id"}, {Name: "platform"}, {Name: "group_id"}}, DoUpdates: clause.Assignments(map[string]any{"bank_id": input.BankId, "enabled": enabled, "priority": input.Priority, "rules_json": rulesJSON, "updated_at": now})}).Create(&row).Error
	if err != nil {
		return nil, err
	}
	return &row, model.DB.Where("site_id = ? AND platform = ? AND group_id = ?", siteID, platform, groupID).First(&row).Error
}

func DeleteOpsQuizBinding(siteID string, bindingID int) error {
	return model.DB.Where("id = ? AND site_id = ?", bindingID, normalizeOpsSiteID(siteID)).Delete(&model.QuizBankBinding{}).Error
}

func GetOpsQuizStats(siteID string) (map[string]any, error) {
	siteID = normalizeOpsSiteID(siteID)
	result := map[string]any{"site_id": siteID}
	for key, table := range map[string]any{"banks": &model.QuizBank{}, "questions": &model.QuizQuestion{}, "bindings": &model.QuizBankBinding{}, "draws": &model.QuizQuestionDraw{}} {
		var count int64
		query := model.DB.Model(table)
		if key != "questions" {
			query = query.Where("site_id = ?", siteID)
		} else {
			query = query.Joins("JOIN quiz_banks qb ON qb.id = quiz_questions.bank_id").Where("qb.site_id = ?", siteID)
		}
		if err := query.Count(&count).Error; err != nil {
			return nil, err
		}
		result[key] = count
	}
	var open int64
	_ = model.DB.Model(&model.QuizQuestionDraw{}).Where("site_id = ? AND status = ? AND expires_at > ?", siteID, "open", time.Now().Unix()).Count(&open).Error
	result["open_draws"] = open
	return result, nil
}

func normalizeOpsQuizQuestion(input OpsQuizQuestionInput, actorID int) (model.QuizQuestion, string, string, error) {
	prompt := strings.TrimSpace(quizDefault(input.Prompt, input.Question))
	if prompt == "" {
		return model.QuizQuestion{}, "", "", errors.New("prompt is required")
	}
	options := make([]string, 0, len(input.Options))
	seen := map[string]bool{}
	for _, raw := range input.Options {
		option := strings.TrimSpace(raw)
		key := strings.ToLower(option)
		if option == "" || seen[key] {
			continue
		}
		seen[key] = true
		options = append(options, option)
	}
	if len(options) < 2 || len(options) > 10 {
		return model.QuizQuestion{}, "", "", errors.New("question must have 2 to 10 distinct options")
	}
	correctIndex := -1
	if input.CorrectIndex != nil {
		correctIndex = *input.CorrectIndex
	} else if input.AnswerIndex != nil {
		correctIndex = *input.AnswerIndex
	} else if answer := strings.TrimSpace(input.Answer); answer != "" {
		for index, option := range options {
			if strings.EqualFold(option, answer) {
				correctIndex = index
				break
			}
		}
	}
	if correctIndex < 0 || correctIndex >= len(options) {
		return model.QuizQuestion{}, "", "", errors.New("correct answer is not present in options")
	}
	optionsJSON, _ := json.Marshal(options)
	contentHash := QuizQuestionContentHash(prompt, options, correctIndex)
	externalKey := strings.TrimSpace(input.ExternalKey)
	if externalKey == "" {
		externalKey = "q-" + contentHash[:20]
	}
	weight := input.Weight
	if weight <= 0 {
		weight = 100
	}
	question := model.QuizQuestion{ExternalKey: externalKey, Prompt: prompt, OptionsJson: string(optionsJSON), CorrectIndex: correctIndex, Explanation: strings.TrimSpace(input.Explanation), Difficulty: quizDefault(strings.TrimSpace(input.Difficulty), "normal"), Language: quizDefault(strings.TrimSpace(input.Language), "zh-CN"), Status: quizQuestionStatus(input.Status), Weight: weight, Source: quizDefault(strings.TrimSpace(input.Source), "manual"), ContentHash: contentHash, CreatedBy: actorID}
	return question, quizSlug(input.CategoryCode), quizDefault(strings.TrimSpace(input.CategoryName), strings.TrimSpace(input.CategoryCode)), nil
}

func ensureQuizCategoryTx(tx *gorm.DB, bankID int, code, name string, now int64) (int, error) {
	if code == "" {
		return 0, nil
	}
	row := model.QuizCategory{BankId: bankID, Code: code, Name: quizDefault(name, code), Status: "active", CreatedAt: now, UpdatedAt: now}
	if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "bank_id"}, {Name: "code"}}, DoUpdates: clause.Assignments(map[string]any{"name": row.Name, "status": "active", "updated_at": now})}).Create(&row).Error; err != nil {
		return 0, err
	}
	if err := tx.Where("bank_id = ? AND code = ?", bankID, code).First(&row).Error; err != nil {
		return 0, err
	}
	return row.Id, nil
}

func quizRequireBank(siteID string, bankID int) error {
	if bankID <= 0 {
		return errors.New("invalid quiz bank id")
	}
	var count int64
	if err := model.DB.Model(&model.QuizBank{}).Where("id = ? AND site_id = ?", bankID, normalizeOpsSiteID(siteID)).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func normalizeOpsSiteID(siteID string) string {
	if siteID = strings.TrimSpace(siteID); siteID != "" {
		return siteID
	}
	return AgentSiteID()
}

func quizSlug(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var builder strings.Builder
	for _, r := range value {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' || r == '_' {
			builder.WriteRune(r)
		} else if r == ' ' || r == '.' || r == '/' {
			builder.WriteByte('-')
		}
	}
	return strings.Trim(builder.String(), "-_")
}

func quizDefault(value, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return value
	}
	return fallback
}

func quizBankStatus(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "published", "disabled":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "draft"
	}
}

func quizQuestionStatus(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "published", "disabled":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "draft"
	}
}
