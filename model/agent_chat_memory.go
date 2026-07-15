package model

import (
	"strings"
	"time"
)

// AgentChatMessage 群聊/私聊会话历史，用于让 agent 具备多轮对话记忆。
// 按会话键(conv_key)隔离：群里每个用户、每个私聊各自独立的上下文。
type AgentChatMessage struct {
	Id        int    `json:"id" gorm:"primaryKey;autoIncrement"`
	SiteId    string `json:"site_id" gorm:"type:varchar(64);not null;index:idx_agent_chat_conv"`
	Source    string `json:"source" gorm:"type:varchar(16);not null"` // qq / tg / community
	ConvKey   string `json:"conv_key" gorm:"type:varchar(128);not null;index:idx_agent_chat_conv"`
	Role      string `json:"role" gorm:"type:varchar(16);not null"` // user / assistant
	Content   string `json:"content" gorm:"type:text"`
	CreatedAt int64  `json:"created_at" gorm:"autoCreateTime;index:idx_agent_chat_conv"`
}

func (AgentChatMessage) TableName() string {
	return "agent_chat_messages"
}

// BuildAgentConvKey 生成会话键：群里按 群+用户 隔离，私聊按 用户 隔离。
func BuildAgentConvKey(source, roomID, userExternalID string) string {
	source = strings.TrimSpace(source)
	roomID = strings.TrimSpace(roomID)
	userExternalID = strings.TrimSpace(userExternalID)
	if roomID != "" {
		return source + ":group:" + roomID + ":" + userExternalID
	}
	return source + ":private:" + userExternalID
}

// AppendAgentChatMessage 追加一条会话消息。
func AppendAgentChatMessage(siteID, source, convKey, role, content string) error {
	content = strings.TrimSpace(content)
	if content == "" || strings.TrimSpace(convKey) == "" {
		return nil
	}
	// 限制单条长度，避免超大消息撑爆上下文。
	runes := []rune(content)
	if len(runes) > 2000 {
		content = string(runes[:2000])
	}
	msg := &AgentChatMessage{
		SiteId:    siteID,
		Source:    source,
		ConvKey:   convKey,
		Role:      role,
		Content:   content,
		CreatedAt: time.Now().Unix(),
	}
	return DB.Create(msg).Error
}

// GetRecentAgentChatMessages 取某会话最近 limit 条历史（按时间正序返回，便于直接拼进 prompt）。
func GetRecentAgentChatMessages(convKey string, limit int) ([]AgentChatMessage, error) {
	if limit <= 0 || limit > 50 {
		limit = 12
	}
	var rows []AgentChatMessage
	// 先取最近 limit 条(倒序)，再反转成正序。
	if err := DB.Where("conv_key = ?", strings.TrimSpace(convKey)).
		Order("id DESC").Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	for i, j := 0, len(rows)-1; i < j; i, j = i+1, j-1 {
		rows[i], rows[j] = rows[j], rows[i]
	}
	return rows, nil
}

// PurgeOldAgentChatMessages 清理超过 keepDays 天的历史，控制表体积。
func PurgeOldAgentChatMessages(keepDays int) error {
	if keepDays <= 0 {
		keepDays = 14
	}
	cutoff := time.Now().Add(-time.Duration(keepDays) * 24 * time.Hour).Unix()
	return DB.Where("created_at < ?", cutoff).Delete(&AgentChatMessage{}).Error
}
