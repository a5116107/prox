package controller

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

func GetOpsRewardFundOverview(c *gin.Context) {
	rewardFundOverview, err := service.GetOpsRewardFundOverview(c.Param("site_id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, rewardFundOverview)
}

func GetOpsBudgetSettings(c *gin.Context) {
	budgetSettings, err := service.GetOpsBudgetSettings(c.Param("site_id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, budgetSettings)
}

func UpdateOpsBudgetSettings(c *gin.Context) {
	var request service.OpsBudgetSettingsUpdateRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiError(c, err)
		return
	}
	budgetSettings, err := service.UpdateOpsBudgetSettings(c.Param("site_id"), request)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "ops.budget_settings_update", map[string]interface{}{
		"site_id": c.Param("site_id"), "apply_to_today": request.ApplyToToday,
		"request_id": request.RequestID,
	})
	common.ApiSuccess(c, budgetSettings)
}

func RestoreOpsBudgetCapacity(c *gin.Context) {
	var request service.OpsBudgetCapacityRestoreRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiError(c, err)
		return
	}
	restoreResult, err := service.RestoreOpsBudgetCapacity(c.Param("site_id"), request)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "ops.budget_capacity_restore", map[string]interface{}{
		"site_id": restoreResult.SiteID, "budget_date": restoreResult.BudgetDate,
		"pool_types": restoreResult.RestoredPoolTypes, "request_id": restoreResult.IdempotencyRequest,
	})
	common.ApiSuccess(c, restoreResult)
}

func GetOpsInviteJourneyOverview(c *gin.Context) {
	data, err := service.GetOpsInviteJourneyOverview(c.Param("site_id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, data)
}
