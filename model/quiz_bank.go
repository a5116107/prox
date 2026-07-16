package model

type QuizBank struct {
	Id              int    `json:"id"`
	SiteId          string `json:"site_id" gorm:"type:varchar(64);not null;uniqueIndex:ux_quiz_bank_code,priority:1;index"`
	Code            string `json:"code" gorm:"type:varchar(64);not null;uniqueIndex:ux_quiz_bank_code,priority:2"`
	Name            string `json:"name" gorm:"type:varchar(160);not null"`
	Description     string `json:"description" gorm:"type:text"`
	DefaultLanguage string `json:"default_language" gorm:"type:varchar(16);not null;default:zh-CN"`
	Status          string `json:"status" gorm:"type:varchar(24);not null;default:draft;index"`
	CreatedBy       int    `json:"created_by" gorm:"index"`
	CreatedAt       int64  `json:"created_at" gorm:"index"`
	UpdatedAt       int64  `json:"updated_at"`
}

func (QuizBank) TableName() string { return "quiz_banks" }

type QuizCategory struct {
	Id        int    `json:"id"`
	BankId    int    `json:"bank_id" gorm:"not null;uniqueIndex:ux_quiz_category_code,priority:1;index"`
	Code      string `json:"code" gorm:"type:varchar(64);not null;uniqueIndex:ux_quiz_category_code,priority:2"`
	Name      string `json:"name" gorm:"type:varchar(120);not null"`
	Status    string `json:"status" gorm:"type:varchar(24);not null;default:active;index"`
	CreatedAt int64  `json:"created_at" gorm:"index"`
	UpdatedAt int64  `json:"updated_at"`
}

func (QuizCategory) TableName() string { return "quiz_categories" }

type QuizQuestion struct {
	Id           int    `json:"id"`
	BankId       int    `json:"bank_id" gorm:"not null;uniqueIndex:ux_quiz_question_external,priority:1;uniqueIndex:ux_quiz_question_hash,priority:1;index"`
	CategoryId   int    `json:"category_id" gorm:"index"`
	ExternalKey  string `json:"external_key" gorm:"type:varchar(128);not null;uniqueIndex:ux_quiz_question_external,priority:2"`
	Prompt       string `json:"prompt" gorm:"type:text;not null"`
	OptionsJson  string `json:"options_json" gorm:"type:text;not null"`
	CorrectIndex int    `json:"correct_index" gorm:"not null"`
	Explanation  string `json:"explanation" gorm:"type:text"`
	Difficulty   string `json:"difficulty" gorm:"type:varchar(24);not null;default:normal;index"`
	Language     string `json:"language" gorm:"type:varchar(16);not null;default:zh-CN;index"`
	Status       string `json:"status" gorm:"type:varchar(24);not null;default:draft;index"`
	Weight       int    `json:"weight" gorm:"not null;default:100"`
	Source       string `json:"source" gorm:"type:varchar(64);not null;default:manual"`
	ContentHash  string `json:"content_hash" gorm:"type:char(64);not null;uniqueIndex:ux_quiz_question_hash,priority:2"`
	CreatedBy    int    `json:"created_by" gorm:"index"`
	CreatedAt    int64  `json:"created_at" gorm:"index"`
	UpdatedAt    int64  `json:"updated_at"`
}

func (QuizQuestion) TableName() string { return "quiz_questions" }

type QuizBankBinding struct {
	Id        int    `json:"id"`
	SiteId    string `json:"site_id" gorm:"type:varchar(64);not null;uniqueIndex:ux_quiz_bank_binding,priority:1;index"`
	BankId    int    `json:"bank_id" gorm:"not null;index"`
	Platform  string `json:"platform" gorm:"type:varchar(32);not null;uniqueIndex:ux_quiz_bank_binding,priority:2;index"`
	GroupId   string `json:"group_id" gorm:"type:varchar(128);not null;uniqueIndex:ux_quiz_bank_binding,priority:3;index"`
	Enabled   bool   `json:"enabled" gorm:"not null;index"`
	Priority  int    `json:"priority" gorm:"not null;default:0"`
	RulesJson string `json:"rules_json" gorm:"type:text"`
	CreatedAt int64  `json:"created_at" gorm:"index"`
	UpdatedAt int64  `json:"updated_at"`
}

func (QuizBankBinding) TableName() string { return "quiz_bank_bindings" }

type QuizQuestionDraw struct {
	Id              int    `json:"id"`
	SiteId          string `json:"site_id" gorm:"type:varchar(64);not null;index"`
	BankId          int    `json:"bank_id" gorm:"not null;index"`
	QuestionId      int    `json:"question_id" gorm:"not null;index"`
	RoundId         int    `json:"round_id" gorm:"not null;uniqueIndex"`
	DrawKey         string `json:"draw_key" gorm:"type:varchar(160);not null;uniqueIndex"`
	Platform        string `json:"platform" gorm:"type:varchar(32);not null;index"`
	GroupId         string `json:"group_id" gorm:"type:varchar(128);not null;index"`
	ScopeMode       string `json:"scope_mode" gorm:"type:varchar(24);not null;index"`
	ScopeKey        string `json:"scope_key" gorm:"type:varchar(320);not null;index"`
	BusinessDate    string `json:"business_date" gorm:"type:varchar(16);not null;index"`
	OptionOrderJson string `json:"option_order_json" gorm:"type:text;not null"`
	RulesJson       string `json:"rules_json" gorm:"type:text"`
	Status          string `json:"status" gorm:"type:varchar(24);not null;default:open;index"`
	ExpiresAt       int64  `json:"expires_at" gorm:"index"`
	AnsweredAt      int64  `json:"answered_at" gorm:"index"`
	CreatedAt       int64  `json:"created_at" gorm:"index"`
	UpdatedAt       int64  `json:"updated_at"`
}

func (QuizQuestionDraw) TableName() string { return "quiz_question_draws" }
