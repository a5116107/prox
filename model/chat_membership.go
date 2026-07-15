package model

import (
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	ChatMembershipStatusActive          = "active"
	ChatMembershipStatusUnboundObserved = "unbound_observed"
	ChatMembershipStatusSuspectedLeft   = "suspected_left"
	ChatMembershipStatusGrace           = "grace"
	ChatMembershipStatusLeftExpired     = "left_expired"
	ChatMembershipStatusRejoinPending   = "rejoin_pending"
	ChatMembershipStatusRestored        = "restored"
	ChatMembershipStatusManualBypass    = "manual_bypass"

	ChatMembershipEventJoin        = "join"
	ChatMembershipEventLeave       = "leave"
	ChatMembershipEventKick        = "kick"
	ChatMembershipEventScanAbsent  = "scan_absent"
	ChatMembershipEventScanPresent = "scan_present"
)

type ChatMembershipState struct {
	Id             int    `json:"id" gorm:"primaryKey"`
	SiteId         string `json:"site_id" gorm:"type:varchar(64);not null;index;uniqueIndex:ux_chat_membership_state,priority:1"`
	Source         string `json:"source" gorm:"type:varchar(16);not null;index;uniqueIndex:ux_chat_membership_state,priority:2"`
	RoomId         string `json:"room_id" gorm:"type:varchar(128);not null;index;uniqueIndex:ux_chat_membership_state,priority:3"`
	ExternalUserId string `json:"external_user_id" gorm:"type:varchar(256);not null;index;uniqueIndex:ux_chat_membership_state,priority:4"`
	NewAPIUserId   int    `json:"new_api_user_id" gorm:"column:new_api_user_id;index"`
	Status         string `json:"status" gorm:"type:varchar(32);not null;index"`
	JoinedAt       int64  `json:"joined_at" gorm:"not null;default:0"`
	LastSeenAt     int64  `json:"last_seen_at" gorm:"not null;default:0"`
	LeftAt         int64  `json:"left_at" gorm:"not null;default:0"`
	GraceUntil     int64  `json:"grace_until" gorm:"not null;default:0;index"`
	RestoredAt     int64  `json:"restored_at" gorm:"not null;default:0"`
	LeaveCount30d  int    `json:"leave_count_30d" gorm:"column:leave_count_30d;not null;default:0"`
	RiskScore      int    `json:"risk_score" gorm:"not null;default:0"`
	LastEventId    string `json:"last_event_id" gorm:"type:varchar(256);not null;default:''"`
	BypassReason   string `json:"bypass_reason" gorm:"type:varchar(255);not null;default:''"`
	BypassUntil    int64  `json:"bypass_until" gorm:"not null;default:0"`
	MetadataJson   string `json:"metadata_json" gorm:"type:text"`
	UpdatedAt      int64  `json:"updated_at" gorm:"autoUpdateTime"`
	CreatedAt      int64  `json:"created_at" gorm:"autoCreateTime"`
}

func (ChatMembershipState) TableName() string {
	return "chat_membership_states"
}

type ChatMembershipEvent struct {
	Id             int    `json:"id" gorm:"primaryKey"`
	EventId        string `json:"event_id" gorm:"type:varchar(256);not null;uniqueIndex"`
	SiteId         string `json:"site_id" gorm:"type:varchar(64);not null;index"`
	Source         string `json:"source" gorm:"type:varchar(16);not null;index"`
	RoomId         string `json:"room_id" gorm:"type:varchar(128);not null;index"`
	ExternalUserId string `json:"external_user_id" gorm:"type:varchar(256);not null;index"`
	NewAPIUserId   int    `json:"new_api_user_id" gorm:"column:new_api_user_id;index"`
	EventType      string `json:"event_type" gorm:"type:varchar(32);not null;index"`
	EventAt        int64  `json:"event_at" gorm:"not null;default:0;index"`
	RawPayloadJson string `json:"raw_payload_json" gorm:"type:text"`
	HandledResult  string `json:"handled_result" gorm:"type:varchar(64);not null;default:''"`
	CreatedAt      int64  `json:"created_at" gorm:"autoCreateTime"`
}

func (ChatMembershipEvent) TableName() string {
	return "chat_membership_events"
}

func chatMembershipSiteID() string {
	return currentAgentSiteID()
}

type ChatMembershipEventInput struct {
	EventId        string
	Source         string
	RoomId         string
	ExternalUserId string
	NewAPIUserId   int
	EventType      string
	EventAt        int64
	RawPayloadJson string
	MetadataJson   string
	GraceUntil     int64
}

func UpsertChatMembershipEvent(input ChatMembershipEventInput) (*ChatMembershipEvent, bool, error) {
	input.Source = strings.TrimSpace(strings.ToLower(input.Source))
	input.RoomId = strings.TrimSpace(input.RoomId)
	input.ExternalUserId = strings.TrimSpace(input.ExternalUserId)
	input.EventType = strings.TrimSpace(strings.ToLower(input.EventType))
	input.EventId = strings.TrimSpace(input.EventId)
	if input.Source == "" || input.RoomId == "" || input.ExternalUserId == "" || input.EventType == "" || input.EventId == "" {
		return nil, false, errors.New("invalid membership event")
	}
	if input.EventAt <= 0 {
		input.EventAt = time.Now().Unix()
	}
	event := ChatMembershipEvent{}
	err := DB.Transaction(func(tx *gorm.DB) error {
		effectiveUserID := input.NewAPIUserId
		if effectiveUserID <= 0 {
			var existing ChatMembershipState
			err := tx.Select("new_api_user_id").
				Where("site_id = ? AND source = ? AND room_id = ? AND external_user_id = ?",
					chatMembershipSiteID(),
					input.Source,
					input.RoomId,
					input.ExternalUserId,
				).
				Take(&existing).Error
			if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			if existing.NewAPIUserId > 0 {
				effectiveUserID = existing.NewAPIUserId
			}
		}
		event = ChatMembershipEvent{
			EventId:        input.EventId,
			SiteId:         chatMembershipSiteID(),
			Source:         input.Source,
			RoomId:         input.RoomId,
			ExternalUserId: input.ExternalUserId,
			NewAPIUserId:   effectiveUserID,
			EventType:      input.EventType,
			EventAt:        input.EventAt,
			RawPayloadJson: input.RawPayloadJson,
			HandledResult:  "accepted",
		}
		result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&event)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			event.HandledResult = "duplicate"
			return nil
		}
		status := ChatMembershipStatusActive
		updates := map[string]interface{}{
			"last_seen_at":  input.EventAt,
			"last_event_id": input.EventId,
			"metadata_json": input.MetadataJson,
			"updated_at":    time.Now().Unix(),
		}
		if effectiveUserID > 0 {
			updates["new_api_user_id"] = effectiveUserID
		}
		switch input.EventType {
		case ChatMembershipEventJoin, ChatMembershipEventScanPresent:
			if effectiveUserID > 0 {
				status = ChatMembershipStatusActive
				updates["restored_at"] = input.EventAt
			} else {
				status = ChatMembershipStatusUnboundObserved
				updates["restored_at"] = int64(0)
			}
			updates["status"] = status
			updates["joined_at"] = input.EventAt
			updates["left_at"] = int64(0)
			updates["grace_until"] = int64(0)
		case ChatMembershipEventLeave, ChatMembershipEventKick, ChatMembershipEventScanAbsent:
			status = ChatMembershipStatusGrace
			updates["status"] = status
			updates["left_at"] = input.EventAt
			updates["grace_until"] = input.GraceUntil
			updates["leave_count_30d"] = gorm.Expr("chat_membership_states.leave_count_30d + ?", 1)
			updates["risk_score"] = gorm.Expr("chat_membership_states.risk_score + ?", 10)
		default:
			return errors.New("unsupported membership event type")
		}
		state := ChatMembershipState{
			SiteId:         chatMembershipSiteID(),
			Source:         input.Source,
			RoomId:         input.RoomId,
			ExternalUserId: input.ExternalUserId,
			NewAPIUserId:   effectiveUserID,
			Status:         status,
			JoinedAt:       input.EventAt,
			LastSeenAt:     input.EventAt,
			LastEventId:    input.EventId,
			MetadataJson:   input.MetadataJson,
		}
		if status == ChatMembershipStatusGrace {
			state.JoinedAt = 0
			state.LeftAt = input.EventAt
			state.GraceUntil = input.GraceUntil
			state.LeaveCount30d = 1
			state.RiskScore = 10
		} else if status == ChatMembershipStatusActive {
			state.RestoredAt = input.EventAt
		}
		return tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "site_id"}, {Name: "source"}, {Name: "room_id"}, {Name: "external_user_id"}},
			DoUpdates: clause.Assignments(updates),
		}).Create(&state).Error
	})
	if err != nil {
		return nil, false, err
	}
	return &event, event.HandledResult != "duplicate", nil
}

func GetChatMembershipState(source string, roomID string, externalUserID string) (*ChatMembershipState, error) {
	var state ChatMembershipState
	err := DB.Where("site_id = ? AND source = ? AND room_id = ? AND external_user_id = ?",
		chatMembershipSiteID(),
		strings.TrimSpace(strings.ToLower(source)),
		strings.TrimSpace(roomID),
		strings.TrimSpace(externalUserID),
	).First(&state).Error
	if err != nil {
		return nil, err
	}
	return &state, nil
}

func ListChatMembershipStates(status string, source string, roomID string, limit int) ([]ChatMembershipState, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	query := DB.Where("site_id = ?", chatMembershipSiteID()).Order("updated_at desc, id desc").Limit(limit)
	if strings.TrimSpace(status) != "" {
		query = query.Where("status = ?", strings.TrimSpace(status))
	}
	if strings.TrimSpace(source) != "" {
		query = query.Where("source = ?", strings.TrimSpace(strings.ToLower(source)))
	}
	if strings.TrimSpace(roomID) != "" {
		query = query.Where("room_id = ?", strings.TrimSpace(roomID))
	}
	var states []ChatMembershipState
	return states, query.Find(&states).Error
}

func ListChatMembershipStatesByUser(userID int, limit int) ([]ChatMembershipState, error) {
	if userID <= 0 {
		return []ChatMembershipState{}, nil
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	var states []ChatMembershipState
	err := DB.Where("site_id = ? AND new_api_user_id = ?", chatMembershipSiteID(), userID).
		Order("updated_at desc, id desc").
		Limit(limit).
		Find(&states).Error
	return states, err
}

func ListChatMembershipStatesByExternal(source string, externalUserID string, limit int) ([]ChatMembershipState, error) {
	source = strings.TrimSpace(strings.ToLower(source))
	externalUserID = strings.TrimSpace(externalUserID)
	if source == "" || externalUserID == "" {
		return []ChatMembershipState{}, nil
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	var states []ChatMembershipState
	err := DB.Where("site_id = ? AND source = ? AND external_user_id = ?", chatMembershipSiteID(), source, externalUserID).
		Order("updated_at desc, id desc").
		Limit(limit).
		Find(&states).Error
	return states, err
}

func UpdateChatMembershipStatesByExternal(source string, roomID string, externalUserID string, updates map[string]interface{}) (int64, error) {
	source = strings.TrimSpace(strings.ToLower(source))
	roomID = strings.TrimSpace(roomID)
	externalUserID = strings.TrimSpace(externalUserID)
	if source == "" || externalUserID == "" {
		return 0, errors.New("invalid membership state selector")
	}
	if updates == nil {
		updates = map[string]interface{}{}
	}
	updates["updated_at"] = time.Now().Unix()
	tx := DB.Model(&ChatMembershipState{}).
		Where("site_id = ? AND source = ? AND room_id = ? AND external_user_id = ?", chatMembershipSiteID(), source, roomID, externalUserID)
	result := tx.Updates(updates)
	return result.RowsAffected, result.Error
}

func GetChatMembershipStateByID(id int) (*ChatMembershipState, error) {
	if id <= 0 {
		return nil, errors.New("invalid membership state id")
	}
	var state ChatMembershipState
	err := DB.Where("site_id = ? AND id = ?", chatMembershipSiteID(), id).First(&state).Error
	if err != nil {
		return nil, err
	}
	return &state, nil
}

func UpdateChatMembershipStateStatus(id int, status string, updates map[string]interface{}) (*ChatMembershipState, error) {
	status = strings.TrimSpace(status)
	if id <= 0 || status == "" {
		return nil, errors.New("invalid membership state update")
	}
	if updates == nil {
		updates = map[string]interface{}{}
	}
	updates["status"] = status
	updates["updated_at"] = time.Now().Unix()
	if err := DB.Model(&ChatMembershipState{}).
		Where("site_id = ? AND id = ?", chatMembershipSiteID(), id).
		Updates(updates).Error; err != nil {
		return nil, err
	}
	return GetChatMembershipStateByID(id)
}

func CountChatMembershipStatesByStatus() (map[string]int64, error) {
	type row struct {
		Status string
		Count  int64
	}
	var rows []row
	err := DB.Model(&ChatMembershipState{}).
		Select("status, count(*) as count").
		Where("site_id = ?", chatMembershipSiteID()).
		Group("status").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := map[string]int64{}
	for _, r := range rows {
		out[r.Status] = r.Count
	}
	return out, nil
}

func ListChatMembershipStatesByStatusAndUserLink(status string, resolved bool, limit int) ([]ChatMembershipState, error) {
	if limit <= 0 || limit > 2000 {
		limit = 500
	}
	query := DB.Where("site_id = ?", chatMembershipSiteID()).
		Order("updated_at desc, id desc").
		Limit(limit)
	if strings.TrimSpace(status) != "" {
		query = query.Where("status = ?", strings.TrimSpace(status))
	}
	if resolved {
		query = query.Where("new_api_user_id > 0")
	} else {
		query = query.Where("new_api_user_id = 0")
	}
	var states []ChatMembershipState
	return states, query.Find(&states).Error
}

func CountChatMembershipStatesByStatusAndUserLink(status string, resolved bool) (int64, error) {
	query := DB.Model(&ChatMembershipState{}).
		Where("site_id = ?", chatMembershipSiteID())
	if strings.TrimSpace(status) != "" {
		query = query.Where("status = ?", strings.TrimSpace(status))
	}
	if resolved {
		query = query.Where("new_api_user_id > 0")
	} else {
		query = query.Where("new_api_user_id = 0")
	}
	var count int64
	return count, query.Count(&count).Error
}

func CountChatMembershipEventsWithoutUser() (int64, error) {
	query := DB.Model(&ChatMembershipEvent{}).
		Where("site_id = ? AND new_api_user_id = 0", chatMembershipSiteID())
	var count int64
	return count, query.Count(&count).Error
}

func GetLatestChatMembershipEventByExternal(source string, roomID string, externalUserID string) (*ChatMembershipEvent, error) {
	source = strings.TrimSpace(strings.ToLower(source))
	roomID = strings.TrimSpace(roomID)
	externalUserID = strings.TrimSpace(externalUserID)
	if source == "" || roomID == "" || externalUserID == "" {
		return nil, errors.New("invalid membership event selector")
	}
	var row ChatMembershipEvent
	err := DB.
		Where("site_id = ? AND source = ? AND room_id = ? AND external_user_id = ?",
			chatMembershipSiteID(),
			source,
			roomID,
			externalUserID,
		).
		Order("event_at desc, created_at desc, id desc").
		First(&row).Error
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func ExpireChatMembershipGrace(now int64, dryRun bool, limit int) (int64, error) {
	if now <= 0 {
		now = time.Now().Unix()
	}
	if limit <= 0 || limit > 1000 {
		limit = 500
	}
	query := DB.Model(&ChatMembershipState{}).
		Where("site_id = ? AND status = ? AND grace_until > 0 AND grace_until <= ?",
			chatMembershipSiteID(), ChatMembershipStatusGrace, now).
		Limit(limit)
	if dryRun {
		var count int64
		err := query.Count(&count).Error
		return count, err
	}
	result := query.Updates(map[string]interface{}{
		"status":     ChatMembershipStatusLeftExpired,
		"updated_at": time.Now().Unix(),
	})
	return result.RowsAffected, result.Error
}
