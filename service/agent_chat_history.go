package service

import (
	"github.com/QuantumNous/new-api/model"
)

// AgentChatHistoryItem 返回给 adapter 的历史条目。
type AgentChatHistoryItem struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// GetAgentChatHistory 取某会话最近历史，返回正序消息列表（供 adapter 拼进 prompt）。
func GetAgentChatHistory(source, roomID, userExternalID string, limit int) []AgentChatHistoryItem {
	convKey := model.BuildAgentConvKey(source, roomID, userExternalID)
	rows, err := model.GetRecentAgentChatMessages(convKey, limit)
	if err != nil || len(rows) == 0 {
		return []AgentChatHistoryItem{}
	}
	out := make([]AgentChatHistoryItem, 0, len(rows))
	for _, r := range rows {
		out = append(out, AgentChatHistoryItem{Role: r.Role, Content: r.Content})
	}
	return out
}

// AppendAgentChatHistory 追加一轮对话（用户消息 + agent 回复）到会话历史。
func AppendAgentChatHistory(source, roomID, userExternalID, userText, assistantText string) {
	convKey := model.BuildAgentConvKey(source, roomID, userExternalID)
	siteID := AgentSiteID()
	if userText != "" {
		_ = model.AppendAgentChatMessage(siteID, source, convKey, "user", userText)
	}
	if assistantText != "" {
		_ = model.AppendAgentChatMessage(siteID, source, convKey, "assistant", assistantText)
	}
}
