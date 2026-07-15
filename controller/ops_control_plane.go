package controller

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

func GetOpsControlPlaneSite(c *gin.Context) {
	siteID := strings.TrimSpace(c.Param("site_id"))
	if siteID == "" {
		siteID = "default"
	}
	snapshot, err := service.BuildOpsControlPlaneSite(siteID)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, snapshot)
}
