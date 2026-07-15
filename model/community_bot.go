package model

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type CommunityBotInvite struct {
	Id               int    `json:"id" gorm:"primaryKey"`
	ProviderId       int    `json:"provider_id" gorm:"not null;index"`
	ProviderUserId   string `json:"provider_user_id" gorm:"type:varchar(256);not null;index;uniqueIndex:ux_community_bot_invite"`
	ProviderUserName string `json:"provider_user_name" gorm:"type:varchar(256);index"`
	RoomId           string `json:"room_id" gorm:"type:varchar(64);not null;index;uniqueIndex:ux_community_bot_invite"`
	Status           string `json:"status" gorm:"type:varchar(32);not null;default:'created';index"`
	LastError        string `json:"last_error" gorm:"type:text"`
	LastInvitedAt    int64  `json:"last_invited_at"`
	NotifySentAt     int64  `json:"notify_sent_at"`
	UpdatedAt        int64  `json:"updated_at" gorm:"autoUpdateTime"`
	CreatedAt        int64  `json:"created_at" gorm:"autoCreateTime"`
}

func (CommunityBotInvite) TableName() string {
	return "community_bot_invites"
}

type CommunityBotReward struct {
	Id             int    `json:"id" gorm:"primaryKey"`
	UserId         int    `json:"user_id" gorm:"not null;index;uniqueIndex:ux_community_bot_reward_once"`
	ProviderId     int    `json:"provider_id" gorm:"not null;index"`
	ProviderUserId string `json:"provider_user_id" gorm:"type:varchar(256);not null;index"`
	RoomId         string `json:"room_id" gorm:"type:varchar(64);not null;index;uniqueIndex:ux_community_bot_reward_once"`
	RewardType     string `json:"reward_type" gorm:"type:varchar(32);not null;index;uniqueIndex:ux_community_bot_reward_once"`
	RewardKey      string `json:"reward_key" gorm:"type:varchar(64);not null;index;uniqueIndex:ux_community_bot_reward_once"`
	Quota          int    `json:"quota" gorm:"not null"`
	MessageCount   int    `json:"message_count" gorm:"not null;default:0"`
	CreatedAt      int64  `json:"created_at" gorm:"autoCreateTime"`
}

func (CommunityBotReward) TableName() string {
	return "community_bot_rewards"
}

type CommunityBotMessageStat struct {
	Id             int    `json:"id" gorm:"primaryKey"`
	UserId         int    `json:"user_id" gorm:"not null;index;uniqueIndex:ux_community_bot_msg_stat"`
	ProviderId     int    `json:"provider_id" gorm:"not null;index"`
	ProviderUserId string `json:"provider_user_id" gorm:"type:varchar(256);not null;index"`
	RoomId         string `json:"room_id" gorm:"type:varchar(64);not null;index;uniqueIndex:ux_community_bot_msg_stat"`
	StatDate       string `json:"stat_date" gorm:"type:varchar(16);not null;index;uniqueIndex:ux_community_bot_msg_stat"`
	MessageCount   int    `json:"message_count" gorm:"not null;default:0"`
	DistinctTexts  int    `json:"distinct_texts" gorm:"not null;default:0"`
	LastMessageId  string `json:"last_message_id" gorm:"type:varchar(128);not null;default:''"`
	UpdatedAt      int64  `json:"updated_at" gorm:"autoUpdateTime"`
	CreatedAt      int64  `json:"created_at" gorm:"autoCreateTime"`
}

func (CommunityBotMessageStat) TableName() string {
	return "community_bot_message_stats"
}

type CommunityBotRoomState struct {
	Id            int    `json:"id" gorm:"primaryKey"`
	RoomId        string `json:"room_id" gorm:"type:varchar(64);not null;uniqueIndex"`
	LastMessageId string `json:"last_message_id" gorm:"type:varchar(128);not null;default:''"`
	LastScannedAt int64  `json:"last_scanned_at" gorm:"not null;default:0"`
	UpdatedAt     int64  `json:"updated_at" gorm:"autoUpdateTime"`
	CreatedAt     int64  `json:"created_at" gorm:"autoCreateTime"`
}

func (CommunityBotRoomState) TableName() string {
	return "community_bot_room_states"
}

const (
	CommunityBotClaimProcessing = "processing"
	CommunityBotClaimCompleted  = "completed"
)

// CommunityBotMessageClaim is the durable, fenced ownership record for one
// externally visible bot command. The composite key prevents streaming,
// polling, and separate application nodes from executing the same command at
// the same time.
type CommunityBotMessageClaim struct {
	Id           int    `json:"id" gorm:"primaryKey"`
	SiteId       string `json:"site_id" gorm:"type:varchar(64);not null;uniqueIndex:ux_community_bot_message_claim,priority:1"`
	RoomId       string `json:"room_id" gorm:"type:varchar(128);not null;uniqueIndex:ux_community_bot_message_claim,priority:2"`
	MessageId    string `json:"message_id" gorm:"type:varchar(128);not null;uniqueIndex:ux_community_bot_message_claim,priority:3"`
	ActionType   string `json:"action_type" gorm:"type:varchar(32);not null;uniqueIndex:ux_community_bot_message_claim,priority:4"`
	Status       string `json:"status" gorm:"type:varchar(16);not null;default:'processing';index"`
	OwnerId      string `json:"owner_id" gorm:"type:varchar(192);not null;default:'';index"`
	FencingToken int64  `json:"fencing_token" gorm:"not null;default:1"`
	LeaseUntil   int64  `json:"lease_until" gorm:"not null;default:0;index"`
	Attempts     int    `json:"attempts" gorm:"not null;default:1"`
	LastError    string `json:"last_error" gorm:"type:text"`
	CompletedAt  int64  `json:"completed_at" gorm:"not null;default:0;index"`
	UpdatedAt    int64  `json:"updated_at" gorm:"autoUpdateTime"`
	CreatedAt    int64  `json:"created_at" gorm:"autoCreateTime"`
}

func (CommunityBotMessageClaim) TableName() string {
	return "community_bot_message_claims"
}

func UpsertCommunityBotInvite(providerId int, providerUserId string, providerUserName string, roomId string, status string, lastError string) (*CommunityBotInvite, bool, error) {
	providerUserId = strings.TrimSpace(providerUserId)
	roomId = strings.TrimSpace(roomId)
	status = strings.TrimSpace(status)
	if providerUserId == "" || roomId == "" || status == "" {
		return nil, false, errors.New("invalid community invite key")
	}
	now := time.Now().Unix()
	created := false
	invite := CommunityBotInvite{
		ProviderId:       providerId,
		ProviderUserId:   providerUserId,
		ProviderUserName: strings.TrimSpace(providerUserName),
		RoomId:           roomId,
		Status:           status,
		LastError:        strings.TrimSpace(lastError),
		LastInvitedAt:    now,
	}
	err := DB.Transaction(func(tx *gorm.DB) error {
		res := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&invite)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected > 0 {
			created = true
			return nil
		}
		updates := map[string]interface{}{
			"provider_id":        providerId,
			"provider_user_name": strings.TrimSpace(providerUserName),
			"status":             status,
			"last_error":         strings.TrimSpace(lastError),
			"last_invited_at":    now,
			"updated_at":         now,
		}
		return tx.Model(&CommunityBotInvite{}).Where("provider_user_id = ? AND room_id = ?", providerUserId, roomId).Updates(updates).Error
	})
	if err != nil {
		return nil, false, err
	}
	var saved CommunityBotInvite
	if err := DB.Where("provider_user_id = ? AND room_id = ?", providerUserId, roomId).First(&saved).Error; err != nil {
		return nil, created, err
	}
	return &saved, created, nil
}

func MarkCommunityBotInviteNotified(providerUserId string, roomId string) error {
	return DB.Model(&CommunityBotInvite{}).Where("provider_user_id = ? AND room_id = ?", strings.TrimSpace(providerUserId), strings.TrimSpace(roomId)).Updates(map[string]interface{}{
		"notify_sent_at": time.Now().Unix(),
		"updated_at":     time.Now().Unix(),
	}).Error
}

func GrantCommunityBotRewardIfNeeded(userId int, providerId int, providerUserId string, roomId string, rewardType string, rewardKey string, quota int, messageCount int) (bool, error) {
	if userId <= 0 {
		return false, errors.New("user id is empty")
	}
	if strings.TrimSpace(roomId) == "" {
		return false, errors.New("room id is empty")
	}
	if strings.TrimSpace(rewardType) == "" || strings.TrimSpace(rewardKey) == "" {
		return false, errors.New("reward type/key is empty")
	}
	if quota <= 0 {
		return false, errors.New("quota must be positive")
	}

	granted := false
	err := DB.Transaction(func(tx *gorm.DB) error {
		reward := CommunityBotReward{
			UserId:         userId,
			ProviderId:     providerId,
			ProviderUserId: strings.TrimSpace(providerUserId),
			RoomId:         strings.TrimSpace(roomId),
			RewardType:     strings.TrimSpace(rewardType),
			RewardKey:      strings.TrimSpace(rewardKey),
			Quota:          quota,
			MessageCount:   messageCount,
		}
		result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&reward)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return nil
		}
		idem := fmt.Sprintf("community-reward:%d:%s:%s:%s", userId, strings.TrimSpace(roomId), strings.TrimSpace(rewardType), strings.TrimSpace(rewardKey))
		remark := fmt.Sprintf("community reward type=%s room=%s reward_key=%s provider_user=%s", strings.TrimSpace(rewardType), strings.TrimSpace(roomId), strings.TrimSpace(rewardKey), strings.TrimSpace(providerUserId))
		metadataJson := fmt.Sprintf(`{"room_id":%q,"reward_type":%q,"reward_key":%q,"provider_user_id":%q}`, strings.TrimSpace(roomId), strings.TrimSpace(rewardType), strings.TrimSpace(rewardKey), strings.TrimSpace(providerUserId))
		if err := GrantQuotaFromBudgetPoolWithSourceTx(tx, userId, "community", quota, idem, remark, "community_reward", metadataJson); err != nil {
			return err
		}
		granted = true
		return nil
	})
	if err != nil {
		return false, err
	}
	if granted {
		_ = invalidateUserCache(userId)
	}
	return granted, nil
}

func UpsertCommunityBotMessageStat(userId int, providerId int, providerUserId string, roomId string, statDate string, messageCount int, distinctTexts int, lastMessageId string) error {
	if userId <= 0 || roomId == "" || statDate == "" {
		return errors.New("invalid message stat key")
	}
	stat := CommunityBotMessageStat{
		UserId:         userId,
		ProviderId:     providerId,
		ProviderUserId: providerUserId,
		RoomId:         roomId,
		StatDate:       statDate,
		MessageCount:   messageCount,
		DistinctTexts:  distinctTexts,
		LastMessageId:  lastMessageId,
	}
	return DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "user_id"}, {Name: "room_id"}, {Name: "stat_date"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"provider_id":      providerId,
			"provider_user_id": providerUserId,
			"message_count":    messageCount,
			"distinct_texts":   distinctTexts,
			"last_message_id":  lastMessageId,
			"updated_at":       time.Now().Unix(),
		}),
	}).Create(&stat).Error
}

func GetCommunityBotRoomState(roomId string) (*CommunityBotRoomState, error) {
	var state CommunityBotRoomState
	err := DB.Where("room_id = ?", roomId).First(&state).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return &CommunityBotRoomState{RoomId: roomId}, nil
	}
	return &state, err
}

func AdvanceCommunityBotRoomState(roomId string, lastMessageId string) (bool, error) {
	roomId = strings.TrimSpace(roomId)
	lastMessageId = strings.TrimSpace(lastMessageId)
	if roomId == "" {
		return false, errors.New("room id is empty")
	}
	if lastMessageId == "" {
		return false, errors.New("last message id is empty")
	}
	now := time.Now().Unix()
	advance := func() (*gorm.DB, error) {
		result := DB.Model(&CommunityBotRoomState{}).
			Where("room_id = ? AND (last_message_id = '' OR last_message_id < ?)", roomId, lastMessageId).
			Updates(map[string]interface{}{
				"last_message_id": lastMessageId,
				"last_scanned_at": now,
				"updated_at":      now,
			})
		return result, result.Error
	}
	if result, err := advance(); err != nil {
		return false, err
	} else if result.RowsAffected > 0 {
		return true, nil
	}

	state := CommunityBotRoomState{RoomId: roomId, LastMessageId: lastMessageId, LastScannedAt: now}
	created := DB.Clauses(clause.OnConflict{DoNothing: true}).Create(&state)
	if created.Error != nil {
		return false, created.Error
	}
	if created.RowsAffected > 0 {
		return true, nil
	}

	// Another node may have inserted a lower cursor between the first UPDATE
	// and INSERT. A final conditional UPDATE closes that race without allowing
	// an older scanner to move the cursor backwards.
	result, err := advance()
	if err != nil {
		return false, err
	}
	return result.RowsAffected > 0, nil
}

func UpdateCommunityBotRoomState(roomId string, lastMessageId string) error {
	_, err := AdvanceCommunityBotRoomState(roomId, lastMessageId)
	return err
}

func ClaimCommunityBotMessage(siteId string, roomId string, messageId string, actionType string, ownerId string, leaseTTL time.Duration) (*CommunityBotMessageClaim, bool, error) {
	siteId = strings.TrimSpace(siteId)
	roomId = strings.TrimSpace(roomId)
	messageId = strings.TrimSpace(messageId)
	actionType = strings.TrimSpace(actionType)
	ownerId = strings.TrimSpace(ownerId)
	if siteId == "" || roomId == "" || messageId == "" || actionType == "" || ownerId == "" {
		return nil, false, errors.New("invalid community bot message claim")
	}
	if leaseTTL < 30*time.Second {
		leaseTTL = 30 * time.Second
	}
	now := time.Now().Unix()
	claim := CommunityBotMessageClaim{
		SiteId:       siteId,
		RoomId:       roomId,
		MessageId:    messageId,
		ActionType:   actionType,
		Status:       CommunityBotClaimProcessing,
		OwnerId:      ownerId,
		FencingToken: 1,
		LeaseUntil:   time.Now().Add(leaseTTL).Unix(),
		Attempts:     1,
	}
	created := DB.Clauses(clause.OnConflict{DoNothing: true}).Create(&claim)
	if created.Error != nil {
		return nil, false, created.Error
	}
	if created.RowsAffected > 0 {
		return &claim, true, nil
	}

	var existing CommunityBotMessageClaim
	if err := DB.Where(
		"site_id = ? AND room_id = ? AND message_id = ? AND action_type = ?",
		siteId, roomId, messageId, actionType,
	).First(&existing).Error; err != nil {
		return nil, false, err
	}
	if existing.Status == CommunityBotClaimCompleted {
		return &existing, false, nil
	}

	takeover := DB.Model(&CommunityBotMessageClaim{}).
		Where("id = ? AND fencing_token = ? AND status = ? AND lease_until <= ?",
			existing.Id, existing.FencingToken, CommunityBotClaimProcessing, now).
		Updates(map[string]interface{}{
			"status":        CommunityBotClaimProcessing,
			"owner_id":      ownerId,
			"fencing_token": gorm.Expr("fencing_token + 1"),
			"lease_until":   time.Now().Add(leaseTTL).Unix(),
			"attempts":      gorm.Expr("attempts + 1"),
			"last_error":    "",
			"updated_at":    now,
		})
	if takeover.Error != nil {
		return nil, false, takeover.Error
	}
	if takeover.RowsAffected == 0 {
		return &existing, false, nil
	}
	if err := DB.First(&existing, existing.Id).Error; err != nil {
		return nil, false, err
	}
	return &existing, true, nil
}

func CompleteCommunityBotMessageClaim(claimId int, ownerId string, fencingToken int64) (bool, error) {
	if claimId <= 0 || strings.TrimSpace(ownerId) == "" || fencingToken <= 0 {
		return false, errors.New("invalid community bot claim completion")
	}
	now := time.Now().Unix()
	result := DB.Model(&CommunityBotMessageClaim{}).
		Where("id = ? AND owner_id = ? AND fencing_token = ? AND status = ?", claimId, strings.TrimSpace(ownerId), fencingToken, CommunityBotClaimProcessing).
		Updates(map[string]interface{}{
			"status":       CommunityBotClaimCompleted,
			"lease_until":  0,
			"completed_at": now,
			"last_error":   "",
			"updated_at":   now,
		})
	return result.RowsAffected > 0, result.Error
}

func DeleteCompletedCommunityBotMessageClaimsBefore(cutoff int64) error {
	if cutoff <= 0 {
		return nil
	}
	return DB.Where("status = ? AND completed_at > 0 AND completed_at < ?", CommunityBotClaimCompleted, cutoff).
		Delete(&CommunityBotMessageClaim{}).Error
}

func GetCommunityBotRewardCount(userId int, roomId string, rewardType string, rewardKey string) (int64, error) {
	var count int64
	tx := DB.Model(&CommunityBotReward{}).Where("user_id = ? AND room_id = ? AND reward_type = ?", userId, roomId, rewardType)
	if rewardKey != "" {
		tx = tx.Where("reward_key = ?", rewardKey)
	}
	err := tx.Count(&count).Error
	return count, err
}

// FindCommunityBoundUser 按社区 provider 的稳定用户 ID 查找已绑定的 new-api 用户。
//
// 运行期主链：
// 1. user_identity_bindings(site_id, provider=community, external_user_id=stableID)
// 2. user_oauth_bindings(provider_id, provider_user_id=stableID)
//
// 当异常历史数据导致同一个 stableID 匹配到多个账号时，仍选“最佳账号”：
// 优先有启用中的 API Key，其次额度更高，其次 id 更小。
func FindCommunityBoundUser(providerSlug, providerUserID, providerUserName string) (int, error) {
	provider, err := GetCustomOAuthProviderBySlug(providerSlug)
	if err != nil {
		return 0, err
	}

	stableID := strings.TrimSpace(providerUserID)
	if stableID == "" {
		return 0, gorm.ErrRecordNotFound
	}
	if identity, identityErr := GetUserIdentityBindingByExternal(CommunityIdentitySiteID(), "community", stableID); identityErr == nil && identity != nil && identity.UserId > 0 {
		return identity.UserId, nil
	} else if identityErr != nil && !errors.Is(identityErr, gorm.ErrRecordNotFound) {
		return 0, identityErr
	}

	loadBindings := func(providerUserID string) ([]UserOAuthBinding, error) {
		providerUserID = strings.TrimSpace(providerUserID)
		if providerUserID == "" {
			return nil, nil
		}
		var rows []UserOAuthBinding
		if err := DB.Where("provider_id = ? AND provider_user_id = ?", provider.Id, providerUserID).Find(&rows).Error; err != nil {
			return nil, err
		}
		return rows, nil
	}
	uniqueUserIDs := func(rows []UserOAuthBinding) []int {
		candidateUserIDs := make([]int, 0, len(rows))
		seenUser := map[int]struct{}{}
		for _, row := range rows {
			if row.UserId <= 0 {
				continue
			}
			if _, ok := seenUser[row.UserId]; ok {
				continue
			}
			seenUser[row.UserId] = struct{}{}
			candidateUserIDs = append(candidateUserIDs, row.UserId)
		}
		return candidateUserIDs
	}

	bindings, err := loadBindings(stableID)
	if err != nil {
		return 0, err
	}
	if len(bindings) == 0 {
		legacyName := strings.TrimSpace(providerUserName)
		if legacyName != "" && legacyName != stableID {
			legacyBindings, legacyErr := loadBindings(legacyName)
			if legacyErr != nil {
				return 0, legacyErr
			}
			if len(legacyBindings) > 0 {
				legacyUserIDs := uniqueUserIDs(legacyBindings)
				if len(legacyUserIDs) == 1 {
					if err := UpdateUserOAuthBinding(legacyUserIDs[0], provider.Id, stableID); err != nil {
						return 0, err
					}
					_ = UpsertUserIdentityBinding(CommunityIdentitySiteID(), legacyUserIDs[0], "community", stableID, legacyName)
					return legacyUserIDs[0], nil
				}
				bindings = legacyBindings
			}
		}
		if len(bindings) == 0 {
			return 0, gorm.ErrRecordNotFound
		}
	}

	candidateUserIDs := uniqueUserIDs(bindings)
	if len(candidateUserIDs) == 0 {
		return 0, gorm.ErrRecordNotFound
	}
	if len(candidateUserIDs) == 1 {
		return candidateUserIDs[0], nil
	}

	bestID := 0
	bestActiveTokens := int64(-1)
	bestQuota := int(-1 << 62)
	for _, uid := range candidateUserIDs {
		u, err := GetUserById(uid, false)
		if err != nil || u == nil || u.Id <= 0 {
			continue
		}
		activeTokens, _ := CountUserEnabledTokens(uid)
		better := false
		switch {
		case activeTokens != bestActiveTokens:
			better = activeTokens > bestActiveTokens
		case u.Quota != bestQuota:
			better = u.Quota > bestQuota
		default:
			better = bestID == 0 || uid < bestID
		}
		if better {
			bestID = uid
			bestActiveTokens = activeTokens
			bestQuota = u.Quota
		}
	}
	if bestID == 0 {
		return candidateUserIDs[0], nil
	}
	return bestID, nil
}

func GetUserByCommunityProvider(providerId int, providerUserId string) (*User, error) {
	user, err := GetUserByOAuthBinding(providerId, providerUserId)
	if err == nil && user != nil && user.Id > 0 {
		return user, nil
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	return nil, fmt.Errorf("community provider user not bound: provider_id=%d provider_user_id=%s", providerId, providerUserId)
}

func RecordCommunityBotRewardLog(userId int, content string, quota int, messageCount int, roomId string, rewardType string) {
	other := map[string]interface{}{
		"community_bot": map[string]interface{}{
			"room_id":       roomId,
			"reward_type":   rewardType,
			"quota":         quota,
			"message_count": messageCount,
		},
	}
	RecordLogEvent(userId, LogTypeSystem, content, LogEventOptions{
		Quota:      quota,
		Category:   "community",
		Source:     "community",
		Action:     "community_reward",
		Status:     "success",
		RoomId:     roomId,
		BudgetPool: "activity",
		RewardType: rewardType,
		Tags:       []string{"community", rewardType},
		Other:      other,
	})
}

func GetCommunityBotAdminStats(roomId string, date string, limit int) (map[string]interface{}, error) {
	roomId = strings.TrimSpace(roomId)
	date = strings.TrimSpace(date)
	if date == "" {
		date = AgentBusinessDateAt(time.Now())
	}
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	var stats []CommunityBotMessageStat
	statQuery := DB.Model(&CommunityBotMessageStat{}).Where("stat_date = ?", date)
	if roomId != "" {
		statQuery = statQuery.Where("room_id = ?", roomId)
	}
	if err := statQuery.Order("message_count DESC, updated_at DESC").Limit(limit).Find(&stats).Error; err != nil {
		return nil, err
	}
	var totalMessageCount int64
	if roomId != "" {
		DB.Model(&CommunityBotMessageStat{}).Where("room_id = ? AND stat_date = ?", roomId, date).Select("COALESCE(SUM(message_count),0)").Scan(&totalMessageCount)
	} else {
		DB.Model(&CommunityBotMessageStat{}).Where("stat_date = ?", date).Select("COALESCE(SUM(message_count),0)").Scan(&totalMessageCount)
	}
	state, _ := GetCommunityBotRoomState(roomId)
	return map[string]interface{}{
		"date": date,
		"totals": map[string]interface{}{
			"users":         len(stats),
			"message_count": totalMessageCount,
		},
		"stats":      stats,
		"rewards":    []CommunityBotReward{},
		"room_state": state,
	}, nil
}
