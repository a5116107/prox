package controller

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

func GetOpsGroupCapabilityMatrix(c *gin.Context) {
	data, err := service.GetOpsGroupCapabilityMatrix(c.Param("site_id"), c.Query("platform"), c.Query("role"), c.Query("status"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, data)
}
