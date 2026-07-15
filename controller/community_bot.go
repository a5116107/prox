package controller

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
)

func GetCommunityBotStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    service.GetCommunityBotStatus(),
	})
}

func GetCommunityBotStats(c *gin.Context) {
	roomID := strings.TrimSpace(operation_setting.GetCommunityBotSetting().RoomID)
	stats, err := model.GetCommunityBotAdminStats(roomID, c.Query("date"), 100)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": stats})
}

func StartCommunityBotOAuth(c *gin.Context) {
	serverAddress := strings.TrimRight(system_setting.ServerAddress, "/")
	if serverAddress == "" {
		serverAddress = strings.TrimRight(c.Request.Header.Get("Origin"), "/")
	}
	if serverAddress == "" {
		proto := c.GetHeader("X-Forwarded-Proto")
		if proto == "" {
			if c.Request.TLS != nil {
				proto = "https"
			} else {
				proto = "http"
			}
		}
		serverAddress = proto + "://" + strings.TrimRight(c.Request.Host, "/")
	}
	start, err := service.BuildCommunityBotOAuthStart(serverAddress)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "community_bot.oauth_start", map[string]interface{}{})
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    start,
	})
}

func CommunityBotOAuthCallback(c *gin.Context) {
	redirectURI := strings.TrimSpace(operation_setting.GetCommunityBotSetting().OAuthCallbackURL)
	if redirectURI == "" {
		serverAddress := strings.TrimRight(system_setting.ServerAddress, "/")
		if serverAddress == "" {
			serverAddress = strings.TrimRight(c.Request.Header.Get("Origin"), "/")
		}
		if serverAddress == "" {
			proto := c.GetHeader("X-Forwarded-Proto")
			if proto == "" {
				if c.Request.TLS != nil {
					proto = "https"
				} else {
					proto = "http"
				}
			}
			serverAddress = proto + "://" + strings.TrimRight(c.Request.Host, "/")
		}
		redirectURI = serverAddress + c.Request.URL.Path
	}
	ctx, cancel := contextWithTimeout(c, 30*time.Second)
	defer cancel()
	session := c.Query("session")
	if session == "" {
		session = c.Query("code")
	}
	if err := service.CompleteCommunityBotOAuth(ctx, session, c.Query("state"), redirectURI); err != nil {
		c.Data(http.StatusBadRequest, "text/html; charset=utf-8", []byte("Community bot authorization failed: "+err.Error()))
		return
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte("Community bot authorized successfully. You can close this page."))
}

func SaveCommunityBotToken(c *gin.Context) {
	var req struct {
		Token string `json:"token"`
	}
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "无效的参数")
		return
	}
	ctx, cancel := contextWithTimeout(c, 20*time.Second)
	defer cancel()
	user, err := service.SaveCommunityBotToken(ctx, strings.TrimSpace(req.Token))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "community_bot.token_save", map[string]interface{}{
		"bot_user_id":    user.ID,
		"bot_username":   user.Username,
		"community_host": operation_setting.GetCommunityBotSetting().CommunityHost,
	})
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "社区机器人 Token 已保存",
		"data": gin.H{
			"bot_user_id":  user.ID,
			"bot_username": user.Username,
		},
	})
}

func TestCommunityBotMessage(c *gin.Context) {
	cfg := operation_setting.GetCommunityBotSetting()
	if strings.TrimSpace(cfg.BotToken) == "" {
		common.ApiErrorMsg(c, "社区机器人尚未授权")
		return
	}
	text := strings.TrimSpace(c.PostForm("text"))
	if text == "" {
		var req struct {
			Text string `json:"text"`
		}
		_ = common.DecodeJson(c.Request.Body, &req)
		text = strings.TrimSpace(req.Text)
	}
	if text == "" {
		text = "社区管家测试消息：New API 已成功连接社区群聊。"
	}
	ctx, cancel := contextWithTimeout(c, 20*time.Second)
	defer cancel()
	if err := service.CommunitySendRoomMessage(ctx, text); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "community_bot.test_message", map[string]interface{}{})
	common.ApiSuccess(c, nil)
}

func ScanCommunityBotMessages(c *gin.Context) {
	ctx, cancel := contextWithTimeout(c, 60*time.Second)
	defer cancel()
	result, err := service.ScanCommunityMessagesAndReward(ctx)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "community_bot.scan", map[string]interface{}{
		"scanned_messages": result.ScannedMessages,
		"rewarded_users":   result.RewardedUsers,
	})
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    result,
	})
}

func contextWithTimeout(c *gin.Context, d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(c.Request.Context(), d)
}
