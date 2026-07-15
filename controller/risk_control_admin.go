package controller

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

type opsRiskStatusRequest struct {
	Status string `json:"status"`
	Reason string `json:"reason"`
}

type opsRiskActionRequest struct {
	Reason string `json:"reason"`
	DryRun *bool  `json:"dry_run"`
}

func decodeStrictOpsRiskJSON(c *gin.Context, dst any) error {
	if c.Request == nil || c.Request.Body == nil {
		return fmt.Errorf("请求体不能为空")
	}
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return fmt.Errorf("无效的 JSON 请求体: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return fmt.Errorf("JSON 请求体只能包含一个对象")
		}
		return fmt.Errorf("无效的 JSON 请求体: %w", err)
	}
	return nil
}

func ListOpsAccountRiskAudits(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("p", c.DefaultQuery("page", "1")))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", c.DefaultQuery("size", "20")))
	result, err := service.ListOpsAccountRiskAudits(service.OpsAccountRiskQuery{
		RiskType: c.Query("risk_type"),
		Severity: c.Query("severity"),
		Status:   c.DefaultQuery("status", service.OpsRiskStatusActive),
		Keyword:  c.Query("keyword"),
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": result})
}

func UpdateOpsAccountRiskAuditStatus(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "无效的风控审计 ID")
		return
	}
	var req opsRiskStatusRequest
	if err := decodeStrictOpsRiskJSON(c, &req); err != nil {
		common.ApiError(c, err)
		return
	}
	if strings.TrimSpace(req.Status) == "" {
		common.ApiErrorMsg(c, "status 为必填字段")
		return
	}
	row, err := service.UpdateOpsAccountRiskAuditStatus(id, req.Status, c.GetInt("id"), req.Reason)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": row})
}

func DisableOpsAccountRiskAuditKeys(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "无效的风控审计 ID")
		return
	}
	var req opsRiskActionRequest
	if err := decodeStrictOpsRiskJSON(c, &req); err != nil {
		common.ApiError(c, err)
		return
	}
	if req.DryRun == nil {
		common.ApiErrorMsg(c, "dry_run 为必填字段")
		return
	}
	result, err := service.DisableKeysForOpsRiskAudit(id, c.GetInt("id"), req.Reason, *req.DryRun)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": result})
}

func RestoreOpsAccountRiskAuditKeys(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "无效的风控审计 ID")
		return
	}
	var req opsRiskActionRequest
	if err := decodeStrictOpsRiskJSON(c, &req); err != nil {
		common.ApiError(c, err)
		return
	}
	if req.DryRun == nil {
		common.ApiErrorMsg(c, "dry_run 为必填字段")
		return
	}
	result, err := service.RestoreKeysForOpsRiskAudit(id, c.GetInt("id"), req.Reason, *req.DryRun)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": result})
}
