package controller

import (
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

func GetAllLogs(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	logType, _ := strconv.Atoi(c.Query("type"))
	channel, _ := strconv.Atoi(c.Query("channel"))
	filter := model.LogQueryFilter{
		LogType:           logType,
		StartTimestamp:    parseLogInt64Query(c, "start_timestamp"),
		EndTimestamp:      parseLogInt64Query(c, "end_timestamp"),
		ModelName:         c.Query("model_name"),
		Username:          c.Query("username"),
		TokenName:         c.Query("token_name"),
		StartIdx:          pageInfo.GetStartIdx(),
		Num:               pageInfo.GetPageSize(),
		Channel:           channel,
		Group:             c.Query("group"),
		RequestId:         c.Query("request_id"),
		UpstreamRequestId: c.Query("upstream_request_id"),
		SiteId:            c.Query("site_id"),
		Category:          c.Query("category"),
		Source:            c.Query("source"),
		Action:            c.Query("action"),
		Status:            c.Query("status"),
		RoomId:            c.Query("room_id"),
		ExternalUserId:    c.Query("external_user_id"),
		BudgetPool:        c.Query("budget_pool"),
		RewardType:        c.Query("reward_type"),
		RiskLevel:         c.Query("risk_level"),
		Tags:              c.Query("tags"),
	}
	logs, total, err := model.GetAllLogs(filter)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(logs)
	common.ApiSuccess(c, pageInfo)
	return
}

func GetUserLogs(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	userId := c.GetInt("id")
	logType, _ := strconv.Atoi(c.Query("type"))
	filter := model.LogQueryFilter{
		LogType:           logType,
		StartTimestamp:    parseLogInt64Query(c, "start_timestamp"),
		EndTimestamp:      parseLogInt64Query(c, "end_timestamp"),
		ModelName:         c.Query("model_name"),
		TokenName:         c.Query("token_name"),
		StartIdx:          pageInfo.GetStartIdx(),
		Num:               pageInfo.GetPageSize(),
		Group:             c.Query("group"),
		RequestId:         c.Query("request_id"),
		UpstreamRequestId: c.Query("upstream_request_id"),
		SiteId:            c.Query("site_id"),
		Category:          c.Query("category"),
		Source:            c.Query("source"),
		Action:            c.Query("action"),
		Status:            c.Query("status"),
		RoomId:            c.Query("room_id"),
		ExternalUserId:    c.Query("external_user_id"),
		BudgetPool:        c.Query("budget_pool"),
		RewardType:        c.Query("reward_type"),
		RiskLevel:         c.Query("risk_level"),
		Tags:              c.Query("tags"),
	}
	logs, total, err := model.GetUserLogs(userId, filter)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(logs)
	common.ApiSuccess(c, pageInfo)
	return
}

// Deprecated: SearchAllLogs 已废弃，前端未使用该接口。
func SearchAllLogs(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": false,
		"message": "该接口已废弃",
	})
}

// Deprecated: SearchUserLogs 已废弃，前端未使用该接口。
func SearchUserLogs(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": false,
		"message": "该接口已废弃",
	})
}

func GetLogByKey(c *gin.Context) {
	tokenId := c.GetInt("token_id")
	if tokenId == 0 {
		c.JSON(200, gin.H{
			"success": false,
			"message": "无效的令牌",
		})
		return
	}
	logs, err := model.GetLogByTokenId(tokenId)
	if err != nil {
		c.JSON(200, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(200, gin.H{
		"success": true,
		"message": "",
		"data":    logs,
	})
}

func GetLogsStat(c *gin.Context) {
	logType, _ := strconv.Atoi(c.Query("type"))
	channel, _ := strconv.Atoi(c.Query("channel"))
	filter := model.LogQueryFilter{
		LogType:        logType,
		StartTimestamp: parseLogInt64Query(c, "start_timestamp"),
		EndTimestamp:   parseLogInt64Query(c, "end_timestamp"),
		TokenName:      c.Query("token_name"),
		Username:       c.Query("username"),
		ModelName:      c.Query("model_name"),
		Channel:        channel,
		Group:          c.Query("group"),
		SiteId:         c.Query("site_id"),
		Category:       c.Query("category"),
		Source:         c.Query("source"),
		Action:         c.Query("action"),
		Status:         c.Query("status"),
		RoomId:         c.Query("room_id"),
		BudgetPool:     c.Query("budget_pool"),
		RewardType:     c.Query("reward_type"),
		RiskLevel:      c.Query("risk_level"),
	}
	stat, err := model.SumUsedQuota(filter)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	//tokenNum := model.SumUsedToken(logType, startTimestamp, endTimestamp, modelName, username, "")
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"quota": stat.Quota,
			"rpm":   stat.Rpm,
			"tpm":   stat.Tpm,
		},
	})
	return
}

func GetLogsSelfStat(c *gin.Context) {
	username := c.GetString("username")
	logType, _ := strconv.Atoi(c.Query("type"))
	channel, _ := strconv.Atoi(c.Query("channel"))
	filter := model.LogQueryFilter{
		LogType:        logType,
		StartTimestamp: parseLogInt64Query(c, "start_timestamp"),
		EndTimestamp:   parseLogInt64Query(c, "end_timestamp"),
		TokenName:      c.Query("token_name"),
		ModelName:      c.Query("model_name"),
		Username:       username,
		Channel:        channel,
		Group:          c.Query("group"),
		SiteId:         c.Query("site_id"),
		Category:       c.Query("category"),
		Source:         c.Query("source"),
		Action:         c.Query("action"),
		Status:         c.Query("status"),
		RoomId:         c.Query("room_id"),
		BudgetPool:     c.Query("budget_pool"),
		RewardType:     c.Query("reward_type"),
		RiskLevel:      c.Query("risk_level"),
	}
	quotaNum, err := model.SumUsedQuota(filter)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	//tokenNum := model.SumUsedToken(logType, startTimestamp, endTimestamp, modelName, username, tokenName)
	c.JSON(200, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"quota": quotaNum.Quota,
			"rpm":   quotaNum.Rpm,
			"tpm":   quotaNum.Tpm,
			//"token": tokenNum,
		},
	})
	return
}

func DeleteHistoryLogs(c *gin.Context) {
	targetTimestamp, _ := strconv.ParseInt(c.Query("target_timestamp"), 10, 64)
	if targetTimestamp == 0 {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "target timestamp is required",
		})
		return
	}
	count, err := model.DeleteOldLog(c.Request.Context(), targetTimestamp, 100)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    count,
	})
	return
}

func parseLogInt64Query(c *gin.Context, key string) int64 {
	value, _ := strconv.ParseInt(c.Query(key), 10, 64)
	return value
}
