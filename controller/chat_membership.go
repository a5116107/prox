package controller

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

func PostChatMembershipEvent(c *gin.Context) {
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, 1<<20))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if !service.VerifyMembershipEventSignature(body, c.GetHeader("X-NewAPI-Signature")) {
		common.ApiErrorMsg(c, "invalid membership event signature")
		return
	}
	var req service.ChatMembershipEventRequest
	if err := json.Unmarshal(body, &req); err != nil {
		common.ApiError(c, err)
		return
	}
	result, err := service.HandleChatMembershipEvent(req)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": result})
}

func GetChatMembershipOverview(c *gin.Context) {
	overview, err := service.GetChatMembershipOverview()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": overview})
}

func GetChatMembershipUnresolved(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	overview, err := service.GetChatMembershipUnresolved(limit)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": overview})
}

func ListChatMembershipStates(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	rows, err := service.ListChatMembershipStates(c.Query("status"), c.Query("source"), c.Query("room_id"), limit)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": rows})
}

func DryRunChatMembership(c *gin.Context) {
	result, err := service.DryRunExpireChatMembershipGrace()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": result})
}

func BackfillChatMembershipUsers(c *gin.Context) {
	var req service.ChatMembershipAdminBatchRequest
	_ = c.ShouldBindJSON(&req)
	if req.Limit <= 0 {
		req.Limit, _ = strconv.Atoi(c.DefaultQuery("limit", "500"))
	}
	result, err := service.AdminBackfillHistoricalMembership(req.Limit)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": result})
}

func DemoteUnresolvedChatMembershipStates(c *gin.Context) {
	var req service.ChatMembershipAdminBatchRequest
	_ = c.ShouldBindJSON(&req)
	if req.Limit <= 0 {
		req.Limit, _ = strconv.Atoi(c.DefaultQuery("limit", "500"))
	}
	result, err := service.AdminDemoteUnresolvedActiveMembership(req.Limit)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": result})
}

func RestoreChatMembershipState(c *gin.Context) {
	id, ok := parseChatMembershipStateID(c)
	if !ok {
		return
	}
	var req service.ChatMembershipAdminActionRequest
	_ = c.ShouldBindJSON(&req)
	result, err := service.RestoreChatMembershipState(id, req.Reason)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": result})
}

func BypassChatMembershipState(c *gin.Context) {
	id, ok := parseChatMembershipStateID(c)
	if !ok {
		return
	}
	var req service.ChatMembershipAdminActionRequest
	_ = c.ShouldBindJSON(&req)
	result, err := service.BypassChatMembershipState(id, req.Reason, req.UntilHours)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": result})
}

func ClearChatMembershipBypass(c *gin.Context) {
	id, ok := parseChatMembershipStateID(c)
	if !ok {
		return
	}
	var req service.ChatMembershipAdminActionRequest
	_ = c.ShouldBindJSON(&req)
	result, err := service.ClearChatMembershipBypass(id, req.Reason)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": result})
}

func parseChatMembershipStateID(c *gin.Context) (int, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "invalid membership state id")
		return 0, false
	}
	return id, true
}
