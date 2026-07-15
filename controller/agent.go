package controller

import (
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

func GetAgentDashboard(c *gin.Context) {
	data, err := service.GetAgentDashboard()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, data)
}

func GetAgentSiteState(c *gin.Context) {
	data, err := service.GetAgentSiteState()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, data)
}

func GetAgentTools(c *gin.Context) {
	common.ApiSuccess(c, service.AgentToolCatalog())
}

func GetAgentEvents(c *gin.Context) {
	rows, err := model.ListAgentEvents(service.AgentSiteID(), parseAgentLimit(c.DefaultQuery("limit", "50")))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, rows)
}

func CreateAgentEvent(c *gin.Context) {
	var req struct {
		EventType string         `json:"event_type"`
		Source    string         `json:"source"`
		Severity  string         `json:"severity"`
		Status    string         `json:"status"`
		Title     string         `json:"title"`
		Payload   map[string]any `json:"payload"`
	}
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "无效的参数")
		return
	}
	if strings.TrimSpace(req.EventType) == "" {
		common.ApiErrorMsg(c, "event_type is required")
		return
	}
	if req.Source == "" {
		req.Source = "admin"
	}
	if req.Severity == "" {
		req.Severity = "info"
	}
	if req.Status == "" {
		req.Status = "open"
	}
	row := model.AgentEvent{SiteId: service.AgentSiteID(), EventType: req.EventType, Source: req.Source, Severity: req.Severity, Status: req.Status, ActorType: "admin", ActorUserId: c.GetInt("id"), Title: req.Title, PayloadJson: agentControllerJSON(req.Payload)}
	if err := model.DB.Create(&row).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "agent.event_create", map[string]interface{}{"event_type": req.EventType})
	common.ApiSuccess(c, row)
}

func GetAgentActions(c *gin.Context) {
	rows, err := model.ListAgentActions(service.AgentSiteID(), parseAgentLimit(c.DefaultQuery("limit", "50")))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, rows)
}

func CreateAgentAction(c *gin.Context) {
	var req service.AgentActionRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "无效的参数")
		return
	}
	row, err := service.AgentCreateAction(req, c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "agent.action_create", map[string]interface{}{"action_type": req.ActionType})
	common.ApiSuccess(c, row)
}

func GetAgentApprovals(c *gin.Context) {
	rows, err := model.ListAgentApprovals(service.AgentSiteID(), c.Query("status"), parseAgentLimit(c.DefaultQuery("limit", "50")))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, rows)
}

func DecideAgentApproval(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var req struct {
		Decision string `json:"decision"`
		Comment  string `json:"comment"`
	}
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "无效的参数")
		return
	}
	if err := service.AgentDecideApproval(id, c.GetInt("id"), req.Decision, req.Comment); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "agent.approval_decide", map[string]interface{}{"id": id, "decision": req.Decision})
	common.ApiSuccess(c, gin.H{"id": id, "decision": req.Decision})
}

func GetAgentBudgets(c *gin.Context) {
	date := c.DefaultQuery("date", "")
	if date == "" {
		_ = service.EnsureAgentRuntimeDefaults()
	}
	rows, err := model.ListAgentBudgetPools(service.AgentSiteID(), date)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, rows)
}

func GetAgentRisks(c *gin.Context) {
	rows, err := model.ListAgentRiskProfiles(service.AgentSiteID(), parseAgentLimit(c.DefaultQuery("limit", "50")))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, rows)
}

func EvaluateAgentRisk(c *gin.Context) {
	var req struct {
		UserId  int            `json:"user_id"`
		Payload map[string]any `json:"payload"`
	}
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "无效的参数")
		return
	}
	res, err := service.AgentRiskEvaluate(req.UserId, req.Payload)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "agent.risk_evaluate", map[string]interface{}{"user_id": req.UserId, "score": res.Score})
	common.ApiSuccess(c, res)
}

func parseAgentLimit(raw string) int {
	limit, _ := strconv.Atoi(raw)
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	return limit
}

func agentControllerJSON(v any) string {
	if v == nil {
		return "{}"
	}
	b, err := common.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}
