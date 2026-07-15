package model

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type AgentChatBinding struct {
	Id             int    `json:"id" gorm:"primaryKey"`
	SiteId         string `json:"site_id" gorm:"type:varchar(64);not null;index;uniqueIndex:ux_agent_chat_binding"`
	Source         string `json:"source" gorm:"type:varchar(32);not null;index;uniqueIndex:ux_agent_chat_binding"`
	RoomId         string `json:"room_id" gorm:"type:varchar(128);not null;default:'';index;uniqueIndex:ux_agent_chat_binding"`
	ExternalUserId string `json:"external_user_id" gorm:"type:varchar(128);not null;index;uniqueIndex:ux_agent_chat_binding"`
	Username       string `json:"username" gorm:"type:varchar(128);not null;default:'';index"`
	NewAPIUserId   int    `json:"newapi_user_id" gorm:"index"`
	Role           string `json:"role" gorm:"type:varchar(32);not null;default:'member';index"`
	Enabled        bool   `json:"enabled" gorm:"not null;default:true;index"`
	Remark         string `json:"remark" gorm:"type:text"`
	UpdatedAt      int64  `json:"updated_at" gorm:"autoUpdateTime"`
	CreatedAt      int64  `json:"created_at" gorm:"autoCreateTime;index"`
}

func (AgentChatBinding) TableName() string { return "agent_chat_bindings" }

type AgentChatBindCode struct {
	Id        int    `json:"id" gorm:"primaryKey"`
	SiteId    string `json:"site_id" gorm:"type:varchar(64);not null;index;uniqueIndex:ux_agent_chat_bind_code"`
	UserId    int    `json:"user_id" gorm:"not null;index"`
	CodeHash  string `json:"-" gorm:"type:varchar(128);not null;index;uniqueIndex:ux_agent_chat_bind_code"`
	ExpiresAt int64  `json:"expires_at" gorm:"not null;index"`
	UsedAt    int64  `json:"used_at" gorm:"not null;default:0;index"`
	UpdatedAt int64  `json:"updated_at" gorm:"autoUpdateTime"`
	CreatedAt int64  `json:"created_at" gorm:"autoCreateTime;index"`
}

func (AgentChatBindCode) TableName() string { return "agent_chat_bind_codes" }

type AgentTask struct {
	Id               int    `json:"id" gorm:"primaryKey"`
	SiteId           string `json:"site_id" gorm:"type:varchar(64);not null;index"`
	TaskType         string `json:"task_type" gorm:"type:varchar(64);not null;default:'chatops';index"`
	AgentName        string `json:"agent_name" gorm:"type:varchar(64);not null;default:'director';index"`
	Source           string `json:"source" gorm:"type:varchar(32);not null;index"`
	RoomId           string `json:"room_id" gorm:"type:varchar(128);not null;default:'';index"`
	MessageId        string `json:"message_id" gorm:"type:varchar(128);not null;default:'';index"`
	IssuerExternalId string `json:"issuer_external_id" gorm:"type:varchar(128);not null;default:'';index"`
	IssuerUsername   string `json:"issuer_username" gorm:"type:varchar(128);not null;default:'';index"`
	IssuerRole       string `json:"issuer_role" gorm:"type:varchar(32);not null;default:'member';index"`
	Text             string `json:"text" gorm:"type:text"`
	Command          string `json:"command" gorm:"type:varchar(128);not null;default:'';index"`
	Status           string `json:"status" gorm:"type:varchar(32);not null;default:'received';index"`
	RiskLevel        string `json:"risk_level" gorm:"type:varchar(32);not null;default:'low';index"`
	RiskScore        int    `json:"risk_score" gorm:"not null;default:0"`
	ActionId         int    `json:"action_id" gorm:"index"`
	ApprovalId       int    `json:"approval_id" gorm:"index"`
	PayloadJson      string `json:"payload_json" gorm:"type:text"`
	ResultJson       string `json:"result_json" gorm:"type:text"`
	Error            string `json:"error" gorm:"type:text"`
	UpdatedAt        int64  `json:"updated_at" gorm:"autoUpdateTime"`
	CreatedAt        int64  `json:"created_at" gorm:"autoCreateTime;index"`
}

func (AgentTask) TableName() string { return "agent_tasks" }

type AgentTaskRun struct {
	Id         int    `json:"id" gorm:"primaryKey"`
	SiteId     string `json:"site_id" gorm:"type:varchar(64);not null;index"`
	TaskId     int    `json:"task_id" gorm:"not null;index"`
	ActionId   int    `json:"action_id" gorm:"index"`
	RunType    string `json:"run_type" gorm:"type:varchar(64);not null;default:'planner';index"`
	Status     string `json:"status" gorm:"type:varchar(32);not null;default:'running';index"`
	InputJson  string `json:"input_json" gorm:"type:text"`
	OutputJson string `json:"output_json" gorm:"type:text"`
	Error      string `json:"error" gorm:"type:text"`
	StartedAt  int64  `json:"started_at" gorm:"index"`
	FinishedAt int64  `json:"finished_at" gorm:"index"`
	UpdatedAt  int64  `json:"updated_at" gorm:"autoUpdateTime"`
	CreatedAt  int64  `json:"created_at" gorm:"autoCreateTime;index"`
}

func (AgentTaskRun) TableName() string { return "agent_task_runs" }

func ListAgentTasks(siteId string, status string, limit int) ([]AgentTask, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	tx := DB.Where("site_id = ?", strings.TrimSpace(siteId)).Order("id desc").Limit(limit)
	if strings.TrimSpace(status) != "" {
		tx = tx.Where("status = ?", strings.TrimSpace(status))
	}
	var rows []AgentTask
	err := tx.Find(&rows).Error
	return rows, err
}

func CreateAgentTask(task *AgentTask) error {
	if task == nil {
		return nil
	}
	return DB.Create(task).Error
}

func UpdateAgentTaskResult(id int, siteId string, updates map[string]interface{}) error {
	if updates == nil {
		updates = map[string]interface{}{}
	}
	updates["updated_at"] = time.Now().Unix()
	return DB.Model(&AgentTask{}).Where("id = ? AND site_id = ?", id, strings.TrimSpace(siteId)).Updates(updates).Error
}

func UpsertAgentChatBindingWithTx(tx *gorm.DB, binding *AgentChatBinding) error {
	if binding == nil {
		return nil
	}
	if tx == nil {
		tx = DB
	}
	binding.SiteId = CanonicalSiteID(binding.SiteId)
	binding.Source = normalizeIdentityProvider(binding.Source)
	binding.RoomId = strings.TrimSpace(binding.RoomId)
	binding.ExternalUserId = strings.TrimSpace(binding.ExternalUserId)
	if binding.SiteId == "" || binding.Source == "" || binding.RoomId == "" || binding.ExternalUserId == "" {
		return errors.New("invalid agent chat binding")
	}

	updateExisting := func(existing *AgentChatBinding) error {
		if existing == nil || existing.Id <= 0 {
			return errors.New("invalid existing agent chat binding")
		}
		if existing.NewAPIUserId > 0 {
			if binding.NewAPIUserId > 0 && existing.NewAPIUserId != binding.NewAPIUserId {
				return fmt.Errorf("%w: site=%s source=%s room_id=%s external_user_id=%s existing_user_id=%d requested_user_id=%d",
					ErrIdentityBindingConflict,
					binding.SiteId,
					binding.Source,
					binding.RoomId,
					binding.ExternalUserId,
					existing.NewAPIUserId,
					binding.NewAPIUserId,
				)
			}
			// An observation without a resolved user must not erase an existing
			// ownership link.
			binding.NewAPIUserId = existing.NewAPIUserId
		}
		return tx.Model(&AgentChatBinding{}).Where("id = ?", existing.Id).Updates(map[string]interface{}{
			"username":        binding.Username,
			"new_api_user_id": binding.NewAPIUserId,
			"role":            binding.Role,
			"enabled":         binding.Enabled,
			"remark":          binding.Remark,
			"updated_at":      time.Now().Unix(),
		}).Error
	}

	var existing AgentChatBinding
	err := lockForUpdate(tx).Where(
		"site_id = ? AND source = ? AND room_id = ? AND external_user_id = ?",
		binding.SiteId,
		binding.Source,
		binding.RoomId,
		binding.ExternalUserId,
	).First(&existing).Error
	if err == nil {
		return updateExisting(&existing)
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	result := tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "site_id"}, {Name: "source"}, {Name: "room_id"}, {Name: "external_user_id"}},
		DoNothing: true,
	}).Create(binding)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected > 0 {
		return nil
	}

	// Another transaction inserted the identity after our initial read. Re-read
	// it and apply the same ownership check instead of overwriting user_id in an
	// ON CONFLICT update.
	if err := lockForUpdate(tx).Where(
		"site_id = ? AND source = ? AND room_id = ? AND external_user_id = ?",
		binding.SiteId,
		binding.Source,
		binding.RoomId,
		binding.ExternalUserId,
	).First(&existing).Error; err != nil {
		return err
	}
	return updateExisting(&existing)
}

func UpsertAgentChatBinding(binding *AgentChatBinding) error {
	return UpsertAgentChatBindingWithTx(DB, binding)
}

func GetAgentChatBinding(siteId string, source string, roomId string, externalUserId string) (*AgentChatBinding, error) {
	var row AgentChatBinding
	siteId = strings.TrimSpace(siteId)
	source = normalizeIdentityProvider(source)
	roomId = strings.TrimSpace(roomId)
	externalUserId = strings.TrimSpace(externalUserId)
	err := DB.Where(
		"site_id = ? AND source = ? AND room_id = ? AND external_user_id = ? AND enabled = ?",
		siteId,
		source,
		roomId,
		externalUserId,
		true,
	).Order("updated_at desc, id desc").First(&row).Error
	return &row, err
}

func GetAnyAgentChatBinding(siteId string, source string, roomId string, externalUserId string) (*AgentChatBinding, error) {
	siteId = strings.TrimSpace(siteId)
	source = normalizeIdentityProvider(source)
	externalUserId = strings.TrimSpace(externalUserId)
	roomId = strings.TrimSpace(roomId)

	queryOne := func(tx *gorm.DB) (*AgentChatBinding, error) {
		var row AgentChatBinding
		if err := tx.Order("enabled desc, updated_at desc, id desc").Limit(1).Find(&row).Error; err != nil {
			return nil, err
		}
		if row.Id <= 0 {
			return nil, gorm.ErrRecordNotFound
		}
		return &row, nil
	}

	tx := DB.Where("site_id = ? AND source = ? AND room_id = ? AND external_user_id = ?", siteId, source, roomId, externalUserId)
	return queryOne(tx)
}

func ListAgentChatBindingsByExternal(siteId string, source string, externalUserId string) ([]AgentChatBinding, error) {
	var rows []AgentChatBinding
	siteId = strings.TrimSpace(siteId)
	source = normalizeIdentityProvider(source)
	externalUserId = strings.TrimSpace(externalUserId)
	tx := DB.Where("site_id = ? AND source = ? AND external_user_id = ?", siteId, source, externalUserId).Order("enabled desc, updated_at desc, id desc")
	err := tx.Find(&rows).Error
	return rows, err
}

func SetAgentChatBindingsEnabled(siteId string, source string, roomId string, externalUserId string, enabled bool, userID int) (int64, error) {
	siteId = strings.TrimSpace(siteId)
	source = normalizeIdentityProvider(source)
	roomId = strings.TrimSpace(roomId)
	externalUserId = strings.TrimSpace(externalUserId)
	if siteId == "" || source == "" || externalUserId == "" {
		return 0, errors.New("invalid agent chat binding selector")
	}
	updates := map[string]interface{}{
		"enabled":    enabled,
		"updated_at": time.Now().Unix(),
	}
	if userID > 0 {
		updates["new_api_user_id"] = userID
	}
	tx := DB.Model(&AgentChatBinding{}).Where("site_id = ? AND source = ? AND room_id = ? AND external_user_id = ?", siteId, source, roomId, externalUserId)
	result := tx.Updates(updates)
	return result.RowsAffected, result.Error
}

func UpsertAgentChatBindingFromMembership(siteId string, source string, roomId string, externalUserId string, userID int, username string, role string, remark string) (*AgentChatBinding, error) {
	siteId = strings.TrimSpace(siteId)
	source = normalizeIdentityProvider(source)
	roomId = strings.TrimSpace(roomId)
	externalUserId = strings.TrimSpace(externalUserId)
	username = strings.TrimSpace(username)
	role = strings.TrimSpace(role)
	remark = strings.TrimSpace(remark)
	if role == "" {
		role = "member"
	}
	if siteId == "" || source == "" || roomId == "" || externalUserId == "" {
		return nil, errors.New("invalid membership binding payload")
	}
	binding := &AgentChatBinding{
		SiteId:         siteId,
		Source:         source,
		RoomId:         roomId,
		ExternalUserId: externalUserId,
		Username:       username,
		NewAPIUserId:   userID,
		Role:           role,
		Enabled:        true,
		Remark:         remark,
	}
	if current, err := GetAnyAgentChatBinding(siteId, source, roomId, externalUserId); err == nil && current != nil {
		if binding.Username == "" {
			binding.Username = current.Username
		}
		if binding.NewAPIUserId <= 0 {
			binding.NewAPIUserId = current.NewAPIUserId
		}
		if binding.Role == "" || binding.Role == "member" {
			if strings.TrimSpace(current.Role) != "" {
				binding.Role = current.Role
			}
		}
		if binding.Remark == "" {
			binding.Remark = current.Remark
		}
	}
	if err := UpsertAgentChatBinding(binding); err != nil {
		return nil, err
	}
	return binding, nil
}

func CreateAgentChatBindCodeWithTx(tx *gorm.DB, siteId string, userId int, codeHash string, expiresAt int64) error {
	if tx == nil {
		tx = DB
	}
	now := time.Now().Unix()
	_ = tx.Model(&AgentChatBindCode{}).Where("site_id = ? AND user_id = ? AND used_at = 0", strings.TrimSpace(siteId), userId).Updates(map[string]interface{}{"used_at": now, "updated_at": now}).Error
	row := &AgentChatBindCode{SiteId: strings.TrimSpace(siteId), UserId: userId, CodeHash: strings.TrimSpace(codeHash), ExpiresAt: expiresAt, UsedAt: 0}
	return tx.Create(row).Error
}

func CreateAgentChatBindCode(siteId string, userId int, codeHash string, expiresAt int64) error {
	return CreateAgentChatBindCodeWithTx(DB, siteId, userId, codeHash, expiresAt)
}

func ConsumeAgentChatBindCodeWithTx(tx *gorm.DB, siteId string, codeHash string) (*AgentChatBindCode, error) {
	if tx == nil {
		tx = DB
	}
	now := time.Now().Unix()
	var row AgentChatBindCode
	if err := lockForUpdate(tx).Where("site_id = ? AND code_hash = ? AND used_at = 0 AND expires_at > ?", CanonicalSiteID(siteId), strings.TrimSpace(codeHash), now).First(&row).Error; err != nil {
		return nil, err
	}
	res := tx.Model(&AgentChatBindCode{}).Where("id = ? AND used_at = 0", row.Id).Updates(map[string]interface{}{"used_at": now, "updated_at": now})
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	row.UsedAt = now
	return &row, nil
}

func ConsumeAgentChatBindCode(siteId string, codeHash string) (*AgentChatBindCode, error) {
	return ConsumeAgentChatBindCodeWithTx(DB, siteId, codeHash)
}

func BackfillChatMembershipUserByExternal(source string, externalUserID string, userID int) (int64, int64, error) {
	source = normalizeIdentityProvider(source)
	externalUserID = strings.TrimSpace(externalUserID)
	if source == "" || externalUserID == "" || userID <= 0 {
		return 0, 0, errors.New("invalid membership user backfill selector")
	}

	now := time.Now().Unix()
	var stateRows int64
	var eventRows int64
	err := DB.Transaction(func(tx *gorm.DB) error {
		stateResult := tx.Model(&ChatMembershipState{}).
			Where("site_id = ? AND source = ? AND external_user_id = ? AND new_api_user_id = 0",
				chatMembershipSiteID(),
				source,
				externalUserID,
			).
			Updates(map[string]interface{}{
				"new_api_user_id": userID,
				"updated_at":      now,
			})
		if stateResult.Error != nil {
			return stateResult.Error
		}
		stateRows = stateResult.RowsAffected

		eventResult := tx.Model(&ChatMembershipEvent{}).
			Where("site_id = ? AND source = ? AND external_user_id = ? AND new_api_user_id = 0",
				chatMembershipSiteID(),
				source,
				externalUserID,
			).
			Update("new_api_user_id", userID)
		if eventResult.Error != nil {
			return eventResult.Error
		}
		eventRows = eventResult.RowsAffected
		return nil
	})
	return stateRows, eventRows, err
}

func ListChatMembershipStatesWithoutUser(limit int) ([]ChatMembershipState, error) {
	if limit <= 0 || limit > 2000 {
		limit = 500
	}
	var rows []ChatMembershipState
	err := DB.
		Where("site_id = ? AND new_api_user_id = 0", chatMembershipSiteID()).
		Order("updated_at desc, id desc").
		Limit(limit).
		Find(&rows).Error
	return rows, err
}

func BackfillChatMembershipUsersFromBindings(limit int) (int64, int64, error) {
	if limit <= 0 || limit > 2000 {
		limit = 500
	}

	var bindings []AgentChatBinding
	if err := DB.
		Where("site_id = ? AND new_api_user_id > 0", chatMembershipSiteID()).
		Order("updated_at desc, id desc").
		Limit(limit).
		Find(&bindings).Error; err != nil {
		return 0, 0, err
	}

	seen := make(map[string]struct{}, len(bindings))
	var totalStateRows int64
	var totalEventRows int64
	for _, binding := range bindings {
		source := normalizeIdentityProvider(binding.Source)
		externalUserID := strings.TrimSpace(binding.ExternalUserId)
		userID := binding.NewAPIUserId
		if source == "" || externalUserID == "" || userID <= 0 {
			continue
		}
		key := fmt.Sprintf("%s|%s|%d", source, externalUserID, userID)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}

		stateRows, eventRows, err := BackfillChatMembershipUserByExternal(source, externalUserID, userID)
		if err != nil {
			return totalStateRows, totalEventRows, err
		}
		totalStateRows += stateRows
		totalEventRows += eventRows
	}

	return totalStateRows, totalEventRows, nil
}
