package controller

import (
	"net/http"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

func GetOpsCommunityGateStatus(c *gin.Context) {
	_ = c.Param("site_id")
	status, err := service.GetCommunityGateAdminStatus()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": status})
}

func GetCommunityGateStatus(c *gin.Context) {
	status, err := service.GetCommunityGateAdminStatus()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": status})
}

func GetMyCommunityGateStatus(c *gin.Context) {
	userID := c.GetInt("id")
	refresh := c.Query("refresh") == "true" || c.Query("refresh") == "1"
	ctx, cancel := contextWithTimeout(c, 60*time.Second)
	defer cancel()
	status, err := service.GetCommunityGateUserStatus(ctx, userID, refresh)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": status})
}

func RestoreMyCommunityGateTokens(c *gin.Context) {
	userID := c.GetInt("id")
	ctx, cancel := contextWithTimeout(c, 60*time.Second)
	defer cancel()
	stats, gate, err := service.RestoreCommunityGateUserTokensIfCompliant(ctx, userID)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": gin.H{"stats": stats, "gate": gate}})
}

func CheckCommunityGateUser(c *gin.Context) {
	userID, err := strconv.Atoi(c.Param("user_id"))
	if err != nil || userID <= 0 {
		common.ApiErrorMsg(c, "无效的用户 ID")
		return
	}
	ctx, cancel := contextWithTimeout(c, 60*time.Second)
	defer cancel()
	result, err := service.EvaluateCommunityGate(ctx, userID, true)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "community_gate.check", map[string]interface{}{
		"user_id":   userID,
		"compliant": result.Compliant,
		"reason":    result.ReasonCode,
	})
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": result})
}

func CheckOpsCommunityGateUser(c *gin.Context) {
	_ = c.Param("site_id")
	CheckCommunityGateUser(c)
}

func ScanCommunityGate(c *gin.Context) {
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
	result, err := service.CommunityGateScanAndFreeze(ctx, dryRun, req.Limit)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "community_gate.scan", map[string]interface{}{
		"dry_run":         dryRun,
		"scanned_users":   result.ScannedUsers,
		"blocked_users":   result.BlockedUsers,
		"tokens_disabled": result.TokensDisabled,
	})
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": result})
}

func ScanOpsCommunityGate(c *gin.Context) {
	_ = c.Param("site_id")
	ScanCommunityGate(c)
}

func RestoreCommunityGateTokens(c *gin.Context) {
	userID, err := strconv.Atoi(c.Param("user_id"))
	if err != nil || userID <= 0 {
		common.ApiErrorMsg(c, "无效的用户 ID")
		return
	}
	stats, err := service.RestoreCommunityGateUserTokens(userID)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "community_gate.restore", map[string]interface{}{
		"user_id":  userID,
		"restored": stats.Restored,
	})
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": stats})
}

func RestoreOpsCommunityGateTokens(c *gin.Context) {
	_ = c.Param("site_id")
	RestoreCommunityGateTokens(c)
}
