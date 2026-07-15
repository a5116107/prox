package controller

import (
	"net/http"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

func GetAccessControlStatus(c *gin.Context) {
	status, err := service.GetAccessControlAdminStatus()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": status})
}

func GetMyAccessControlStatus(c *gin.Context) {
	userID := c.GetInt("id")
	refresh := c.Query("refresh") == "true" || c.Query("refresh") == "1"
	ctx, cancel := contextWithTimeout(c, 60*time.Second)
	defer cancel()
	status, err := service.GetUserAccessControlStatus(ctx, userID, refresh)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": status})
}

func RefreshMyAccessControlStatus(c *gin.Context) {
	userID := c.GetInt("id")
	ctx, cancel := contextWithTimeout(c, 60*time.Second)
	defer cancel()
	stats, state, err := service.RestoreAccessControlUserTokensIfCompliant(ctx, userID)
	status, statusErr := service.GetUserAccessControlStatus(ctx, userID, true)
	if statusErr != nil {
		common.ApiError(c, statusErr)
		return
	}
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
			"data": gin.H{
				"state":  state,
				"status": status,
				"stats":  stats,
			},
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"state":  state,
			"status": status,
			"stats":  stats,
		},
	})
}

func CheckAccessControlUser(c *gin.Context) {
	userID, err := strconv.Atoi(c.Param("user_id"))
	if err != nil || userID <= 0 {
		common.ApiErrorMsg(c, "无效的用户 ID")
		return
	}
	ctx, cancel := contextWithTimeout(c, 60*time.Second)
	defer cancel()
	status, err := service.GetUserAccessControlStatus(ctx, userID, true)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "access_control.check", map[string]any{
		"user_id":      userID,
		"access_level": status["access_level"],
		"reason_code":  status["reason_code"],
	})
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": status})
}

func ScanAccessControl(c *gin.Context) {
	var req struct {
		DryRun *bool `json:"dry_run"`
		Limit  int   `json:"limit"`
	}
	_ = common.DecodeJson(c.Request.Body, &req)
	dryRun := true
	if req.DryRun != nil {
		dryRun = *req.DryRun
	}
	ctx, cancel := contextWithTimeout(c, 180*time.Second)
	defer cancel()
	result, err := service.ScanAccessControlAndFreeze(ctx, dryRun, req.Limit)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "access_control.scan", map[string]any{
		"dry_run":         dryRun,
		"scanned_users":   result.ScannedUsers,
		"blocked_users":   result.BlockedUsers,
		"tokens_disabled": result.TokensDisabled,
	})
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": result})
}

func RestoreAccessControlTokens(c *gin.Context) {
	userID, err := strconv.Atoi(c.Param("user_id"))
	if err != nil || userID <= 0 {
		common.ApiErrorMsg(c, "无效的用户 ID")
		return
	}
	ctx, cancel := contextWithTimeout(c, 60*time.Second)
	defer cancel()
	stats, state, err := service.RestoreAccessControlUserTokensIfCompliant(ctx, userID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
			"data": gin.H{
				"state": state,
				"stats": stats,
			},
		})
		return
	}
	recordManageAudit(c, "access_control.restore", map[string]any{
		"user_id":  userID,
		"restored": stats.Restored,
	})
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": gin.H{"state": state, "stats": stats}})
}

func OverrideAccessControl(c *gin.Context) {
	var req struct {
		UserID int      `json:"user_id"`
		Mode   string   `json:"mode"`
		Groups []string `json:"groups"`
		Reason string   `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	if req.UserID <= 0 {
		common.ApiErrorMsg(c, "无效的用户 ID")
		return
	}
	if req.Mode == "" || req.Mode == "clear" {
		if err := model.ClearUserSiteAccessOverride(service.AgentSiteID(), req.UserID); err != nil {
			common.ApiError(c, err)
			return
		}
	} else {
		if err := model.SetUserSiteAccessOverride(service.AgentSiteID(), req.UserID, req.Mode, req.Groups, req.Reason); err != nil {
			common.ApiError(c, err)
			return
		}
	}
	ctx, cancel := contextWithTimeout(c, 60*time.Second)
	defer cancel()
	status, err := service.GetUserAccessControlStatus(ctx, req.UserID, true)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "access_control.override", map[string]any{
		"user_id": req.UserID,
		"mode":    req.Mode,
		"groups":  req.Groups,
	})
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": status})
}
