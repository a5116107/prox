package controller

import (
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

func GetOpsReleases(c *gin.Context) {
	siteID := strings.TrimSpace(c.Param("site_id"))
	if siteID == "" {
		siteID = strings.TrimSpace(c.Query("site_id"))
	}
	data, err := service.GetOpsReleaseOverview(siteID)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, data)
}

func GetOpsReleaseImpactPreview(c *gin.Context) {
	siteID := strings.TrimSpace(c.Param("site_id"))
	if siteID == "" {
		siteID = strings.TrimSpace(c.Query("site_id"))
	}
	data, err := service.GetOpsReleaseImpactPreview(siteID)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, data)
}

func GetOpsAudits(c *gin.Context) {
	siteID := strings.TrimSpace(c.Param("site_id"))
	if siteID == "" {
		siteID = strings.TrimSpace(c.Query("site_id"))
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	data, err := service.GetOpsAuditOverview(siteID, limit)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, data)
}

func PublishOpsRelease(c *gin.Context) {
	siteID := strings.TrimSpace(c.Param("site_id"))
	if siteID == "" {
		siteID = strings.TrimSpace(c.Query("site_id"))
	}
	var req service.OpsReleasePublishRequest
	if err := common.UnmarshalBodyReusable(c, &req); err != nil {
		common.ApiError(c, err)
		return
	}
	data, err := service.PublishOpsRelease(siteID, c.GetInt("id"), c.GetString("username"), req)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, data)
}

func RollbackOpsRelease(c *gin.Context) {
	siteID := strings.TrimSpace(c.Param("site_id"))
	if siteID == "" {
		siteID = strings.TrimSpace(c.Query("site_id"))
	}
	var req service.OpsReleaseRollbackRequest
	if err := common.UnmarshalBodyReusable(c, &req); err != nil {
		common.ApiError(c, err)
		return
	}
	data, err := service.RollbackOpsRelease(siteID, c.GetInt("id"), c.GetString("username"), req)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, data)
}
