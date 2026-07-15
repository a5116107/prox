package controller

import (
	"net/http"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

func GetOpsAccessPolicy(c *gin.Context) {
	view, err := service.GetOpsAccessPolicy(c.Param("site_id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": view})
}

func UpdateOpsAccessPolicy(c *gin.Context) {
	var req service.OpsAccessPolicySaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	view, err := service.SaveOpsAccessPolicy(c.Param("site_id"), req)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "ops.access_policy.update", map[string]any{
		"site_id":  c.Param("site_id"),
		"warnings": view.Warnings,
	})
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": view})
}

func DryRunOpsAccessPolicy(c *gin.Context) {
	var req service.OpsAccessPolicyDryRunRequest
	_ = common.DecodeJson(c.Request.Body, &req)
	ctx, cancel := contextWithTimeout(c, 60*time.Second)
	defer cancel()
	out, err := service.DryRunOpsAccessPolicy(ctx, c.Param("site_id"), req)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": out})
}

func ExplainOpsAccessPolicy(c *gin.Context) {
	var req service.OpsAccessPolicyExplainRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	if req.UserID <= 0 {
		common.ApiErrorMsg(c, "无效的用户 ID")
		return
	}
	ctx, cancel := contextWithTimeout(c, 60*time.Second)
	defer cancel()
	out, err := service.ExplainOpsAccessPolicyUser(ctx, c.Param("site_id"), req)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": out})
}

func ExplainOpsAccessPolicyUser(c *gin.Context) {
	userID, err := strconv.Atoi(c.Param("user_id"))
	if err != nil || userID <= 0 {
		common.ApiErrorMsg(c, "无效的用户 ID")
		return
	}
	refresh := c.Query("refresh") == "true" || c.Query("refresh") == "1"
	req := service.OpsAccessPolicyExplainRequest{UserID: userID, RequestedGroup: c.Query("group"), Refresh: &refresh}
	ctx, cancel := contextWithTimeout(c, 60*time.Second)
	defer cancel()
	out, err := service.ExplainOpsAccessPolicyUser(ctx, c.Param("site_id"), req)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": out})
}
