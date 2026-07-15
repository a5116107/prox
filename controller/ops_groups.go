package controller

import (
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

func ListOpsGroups(c *gin.Context) {
	siteID := strings.TrimSpace(c.Query("site_id"))
	items, err := service.ListOpsGroups(siteID, c.Query("platform"), c.Query("role"), c.Query("status"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"site_id": service.AgentSiteID(),
		"count":   len(items),
		"items":   items,
	})
}

func GetOpsGroup(c *gin.Context) {
	view, err := service.GetOpsGroup(c.Query("site_id"), parseOpsGroupID(c))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, view)
}

func CreateOpsGroup(c *gin.Context) {
	var req service.OpsGroupSaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	view, err := service.CreateOpsGroup(c.Query("site_id"), req)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, view)
}

func CreateOpsGroupsBulk(c *gin.Context) {
	var req service.OpsGroupBulkSaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	result, err := service.CreateOpsGroupsBulk(c.Query("site_id"), req)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}

func UpdateOpsGroup(c *gin.Context) {
	var req service.OpsGroupSaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	view, err := service.UpdateOpsGroup(c.Query("site_id"), parseOpsGroupID(c), req)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, view)
}

func CloneOpsGroup(c *gin.Context) {
	var req service.OpsGroupSaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	view, err := service.CloneOpsGroup(c.Query("site_id"), parseOpsGroupID(c), req)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, view)
}

func GetOpsGroupEffective(c *gin.Context) {
	data, err := service.GetOpsGroupEffective(c.Query("site_id"), parseOpsGroupID(c))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, data)
}

func GetOpsGroupImpactPreview(c *gin.Context) {
	data, err := service.GetOpsGroupImpactPreview(c.Query("site_id"), parseOpsGroupID(c))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, data)
}

func SaveOpsGroupChatOps(c *gin.Context) {
	var req service.OpsGroupChatOpsUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	view, err := service.SaveOpsGroupChatOps(c.Query("site_id"), parseOpsGroupID(c), req)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, view)
}

func SaveOpsGroupGames(c *gin.Context) {
	var req service.OpsGroupGamesUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	view, err := service.SaveOpsGroupGames(c.Query("site_id"), parseOpsGroupID(c), req)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, view)
}

func parseOpsGroupID(c *gin.Context) int {
	id, _ := strconv.Atoi(c.Param("id"))
	return id
}
