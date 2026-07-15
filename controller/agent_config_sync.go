package controller

import (
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

func ExportAgentGameConfig(c *gin.Context) {
	if !service.AgentChatOpsSecretOK(c.DefaultQuery("source", "admin"), c.GetHeader("Authorization"), c.Query("secret"), c.GetHeader("X-Telegram-Bot-Api-Secret-Token")) {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "invalid chatops secret"})
		return
	}
	siteID := c.DefaultQuery("site_id", service.AgentSiteID())
	snapshot, err := service.ExportAgentGameConfigSnapshot(siteID)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"site_id":             siteID,
		"snapshot":            snapshot,
		"legacy_import_guard": service.GetAgentConfigImportGuard(siteID, ""),
	})
}

func ImportAgentGameConfig(c *gin.Context) {
	if !service.AgentChatOpsSecretOK(c.DefaultQuery("source", "admin"), c.GetHeader("Authorization"), c.Query("secret"), c.GetHeader("X-Telegram-Bot-Api-Secret-Token")) {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "invalid chatops secret"})
		return
	}
	var req struct {
		SiteID      string         `json:"site_id"`
		Reason      string         `json:"reason"`
		ActorUserID int            `json:"actor_user_id"`
		Snapshot    map[string]any `json:"snapshot"`
	}
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "无效的参数")
		return
	}
	result, err := service.ImportAgentGameConfigSnapshot(req.Snapshot, req.SiteID, req.ActorUserID, req.Reason)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if ok, exists := result["ok"].(bool); exists && !ok {
		message := "legacy game config import blocked"
		if raw := strings.TrimSpace(common.Interface2String(result["message"])); raw != "" {
			message = raw
		}
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": message,
			"data":    result,
		})
		return
	}
	common.ApiSuccess(c, result)
}

func ExportAgentChatBindings(c *gin.Context) {
	if !service.AgentChatOpsSecretOK(c.DefaultQuery("source", "admin"), c.GetHeader("Authorization"), c.Query("secret"), c.GetHeader("X-Telegram-Bot-Api-Secret-Token")) {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "invalid chatops secret"})
		return
	}
	provider := c.DefaultQuery("provider", "tg")
	rows, err := service.ExportAgentChatBindings(provider)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"provider": provider,
		"count":    len(rows),
		"bindings": rows,
	})
}
