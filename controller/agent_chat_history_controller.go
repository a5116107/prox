package controller

import (
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

// GetAgentChatHistory 取某会话最近对话历史（供 adapter 拼进 LLM 上下文）。
func GetAgentChatHistory(c *gin.Context) {
	if !service.AgentChatOpsSecretOK(c.DefaultQuery("source", "qq"), c.GetHeader("Authorization"), c.Query("secret"), c.GetHeader("X-Telegram-Bot-Api-Secret-Token")) {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "invalid chatops secret"})
		return
	}
	var req struct {
		Source         string `json:"source"`
		RoomID         string `json:"room_id"`
		UserExternalID string `json:"user_external_id"`
		Limit          int    `json:"limit"`
	}
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "无效的参数")
		return
	}
	if req.Source == "" {
		req.Source = c.DefaultQuery("source", "qq")
	}
	items := service.GetAgentChatHistory(req.Source, req.RoomID, req.UserExternalID, req.Limit)
	common.ApiSuccess(c, gin.H{"history": items})
}

// AppendAgentChatHistory 追加一轮对话（用户消息 + agent 回复）到会话历史。
func AppendAgentChatHistory(c *gin.Context) {
	if !service.AgentChatOpsSecretOK(c.DefaultQuery("source", "qq"), c.GetHeader("Authorization"), c.Query("secret"), c.GetHeader("X-Telegram-Bot-Api-Secret-Token")) {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "invalid chatops secret"})
		return
	}
	var req struct {
		Source         string `json:"source"`
		RoomID         string `json:"room_id"`
		UserExternalID string `json:"user_external_id"`
		UserText       string `json:"user_text"`
		AssistantText  string `json:"assistant_text"`
	}
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "无效的参数")
		return
	}
	if req.Source == "" {
		req.Source = c.DefaultQuery("source", "qq")
	}
	service.AppendAgentChatHistory(req.Source, req.RoomID, req.UserExternalID, req.UserText, req.AssistantText)
	common.ApiSuccess(c, gin.H{"ok": true})
}
