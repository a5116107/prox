package service

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/model"
)

type AgentChatOpsLeaderboardEntry struct {
	Rank        int    `json:"rank"`
	UserID      int    `json:"user_id"`
	Username    string `json:"username"`
	Days        int    `json:"days"`
	Streak      int    `json:"streak"`
	TotalQuota  int    `json:"total_quota"`
	LastCheckin string `json:"last_checkin"`
}

type AgentChatOpsLeaderboardResult struct {
	Success bool                           `json:"success"`
	Channel string                         `json:"channel"`
	Entries []AgentChatOpsLeaderboardEntry `json:"entries"`
	Reply   string                         `json:"reply"`
	Error   string                         `json:"error,omitempty"`
}

type chatOpsLeaderboardRow struct {
	UserID      int    `gorm:"column:user_id"`
	Days        int    `gorm:"column:days"`
	TotalQuota  int    `gorm:"column:total_quota"`
	LastCheckin string `gorm:"column:last_checkin"`
}

func chatOpsLeaderboardChannelLabel(channel string) string {
	switch channel {
	case model.CheckinChannelQQ:
		return "QQ"
	case model.CheckinChannelTG:
		return "TG"
	case model.CheckinChannelCommunity:
		return "社区"
	case model.CheckinChannelWeb:
		return "站内"
	default:
		return strings.ToUpper(channel)
	}
}

func chatOpsLeaderboardDisplayName(users map[int]model.User, userID int) string {
	if u, ok := users[userID]; ok {
		if name := strings.TrimSpace(u.DisplayName); name != "" {
			return name
		}
		if name := strings.TrimSpace(u.Username); name != "" {
			return name
		}
	}
	return fmt.Sprintf("用户%d", userID)
}

// HandleAgentChatOpsLeaderboard returns the channel-level checkin growth ranking.
// It is backed only by the authoritative checkins table, so it stays consistent
// with QQ/TG/community checkin rewards and avoids adapter-local state.
func HandleAgentChatOpsLeaderboard(req AgentChatOpsRequest) AgentChatOpsLeaderboardResult {
	source := strings.ToLower(strings.TrimSpace(req.Source))
	if source == "" {
		source = "qq"
	}
	channel := chatOpsChannel(source)
	label := chatOpsLeaderboardChannelLabel(channel)
	if model.DB == nil {
		return AgentChatOpsLeaderboardResult{Success: false, Channel: channel, Reply: "成长榜暂不可用：数据库未初始化", Error: "db_not_ready"}
	}

	var rows []chatOpsLeaderboardRow
	err := model.DB.Table((&model.Checkin{}).TableName()).
		Select("user_id, COUNT(1) AS days, COALESCE(SUM(quota_awarded), 0) AS total_quota, MAX(checkin_date) AS last_checkin").
		Where("channel = ?", channel).
		Group("user_id").
		Order("days DESC, total_quota DESC, last_checkin DESC, user_id ASC").
		Limit(10).
		Scan(&rows).Error
	if err != nil {
		return AgentChatOpsLeaderboardResult{Success: false, Channel: channel, Reply: fmt.Sprintf("%s 签到成长榜读取失败，请稍后再试", label), Error: err.Error()}
	}
	if len(rows) == 0 {
		return AgentChatOpsLeaderboardResult{Success: true, Channel: channel, Entries: []AgentChatOpsLeaderboardEntry{}, Reply: fmt.Sprintf("📊 %s 签到成长榜\n暂无签到数据，先发送「签到」即可上榜。", label)}
	}

	ids := make([]int, 0, len(rows))
	for _, row := range rows {
		if row.UserID > 0 {
			ids = append(ids, row.UserID)
		}
	}
	users := map[int]model.User{}
	if len(ids) > 0 {
		var us []model.User
		if err := model.DB.Where("id IN ?", ids).Find(&us).Error; err == nil {
			for _, u := range us {
				users[u.Id] = u
			}
		}
	}

	entries := make([]AgentChatOpsLeaderboardEntry, 0, len(rows))
	lines := []string{fmt.Sprintf("📊 %s 签到成长榜（按累计签到天数）", label)}
	for i, row := range rows {
		entry := AgentChatOpsLeaderboardEntry{
			Rank:        i + 1,
			UserID:      row.UserID,
			Username:    chatOpsLeaderboardDisplayName(users, row.UserID),
			Days:        row.Days,
			Streak:      model.CountConsecutiveCheckinDays(row.UserID, channel),
			TotalQuota:  row.TotalQuota,
			LastCheckin: row.LastCheckin,
		}
		entries = append(entries, entry)
		lines = append(lines, fmt.Sprintf("%d. %s｜累计 %d 天｜连续 %d 天｜累计奖励 %s｜最近 %s", entry.Rank, entry.Username, entry.Days, entry.Streak, chatOpsFormatUSD(entry.TotalQuota), entry.LastCheckin))
	}
	return AgentChatOpsLeaderboardResult{Success: true, Channel: channel, Entries: entries, Reply: strings.Join(lines, "\n")}
}
