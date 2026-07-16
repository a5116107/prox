package model

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"

	"github.com/bytedance/gopkg/util/gopool"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func applyExplicitLogTextFilter(tx *gorm.DB, column string, value string) (*gorm.DB, error) {
	if value == "" {
		return tx, nil
	}
	if strings.Contains(value, "%") {
		pattern, err := sanitizeLikePattern(value)
		if err != nil {
			return nil, err
		}
		return tx.Where(column+" LIKE ? ESCAPE '!'", pattern), nil
	}
	return tx.Where(column+" = ?", value), nil
}

type LogQueryFilter struct {
	LogType           int
	StartTimestamp    int64
	EndTimestamp      int64
	ModelName         string
	Username          string
	TokenName         string
	StartIdx          int
	Num               int
	Channel           int
	Group             string
	RequestId         string
	UpstreamRequestId string
	SiteId            string
	Category          string
	Source            string
	Action            string
	Status            string
	RoomId            string
	ExternalUserId    string
	BudgetPool        string
	RewardType        string
	RiskLevel         string
	Tags              string
}

type LogEventOptions struct {
	Username          string
	CreatedAt         int64
	Quota             int
	PromptTokens      int
	CompletionTokens  int
	UseTime           int
	IsStream          bool
	ChannelId         int
	TokenId           int
	TokenName         string
	ModelName         string
	Group             string
	Ip                string
	RequestId         string
	UpstreamRequestId string
	SiteId            string
	Category          string
	Source            string
	Action            string
	Status            string
	RoomId            string
	ExternalUserId    string
	BudgetPool        string
	RewardType        string
	RiskLevel         string
	Tags              []string
	Other             map[string]interface{}
}

type Log struct {
	Id                int     `json:"id" gorm:"index:idx_created_at_id,priority:2;index:idx_user_id_id,priority:2"`
	UserId            int     `json:"user_id" gorm:"index;index:idx_user_id_id,priority:1"`
	CreatedAt         int64   `json:"created_at" gorm:"bigint;index:idx_created_at_id,priority:1;index:idx_created_at_type"`
	Type              int     `json:"type" gorm:"index:idx_created_at_type"`
	Content           string  `json:"content"`
	Username          string  `json:"username" gorm:"index;index:index_username_model_name,priority:2;default:''"`
	TokenName         string  `json:"token_name" gorm:"index;default:''"`
	ModelName         string  `json:"model_name" gorm:"index;index:index_username_model_name,priority:1;default:''"`
	Quota             int     `json:"quota" gorm:"default:0"`
	PromptTokens      int     `json:"prompt_tokens" gorm:"default:0"`
	CompletionTokens  int     `json:"completion_tokens" gorm:"default:0"`
	UseTime           int     `json:"use_time" gorm:"default:0"`
	IsStream          bool    `json:"is_stream"`
	ChannelId         int     `json:"channel" gorm:"index"`
	ChannelName       string  `json:"channel_name" gorm:"->"`
	TokenId           int     `json:"token_id" gorm:"default:0;index"`
	Group             string  `json:"group" gorm:"index"`
	Ip                string  `json:"ip" gorm:"index;default:''"`
	RequestId         string  `json:"request_id,omitempty" gorm:"type:varchar(64);index:idx_logs_request_id;default:''"`
	UpstreamRequestId string  `json:"upstream_request_id,omitempty" gorm:"type:varchar(128);index:idx_logs_upstream_request_id;default:''"`
	SiteId            string  `json:"site_id,omitempty" gorm:"type:varchar(64);index;default:''"`
	Category          string  `json:"category,omitempty" gorm:"type:varchar(64);index;default:''"`
	Source            string  `json:"source,omitempty" gorm:"type:varchar(32);index;default:''"`
	Action            string  `json:"action,omitempty" gorm:"type:varchar(64);index;default:''"`
	Status            string  `json:"status,omitempty" gorm:"type:varchar(32);index;default:''"`
	RoomId            string  `json:"room_id,omitempty" gorm:"type:varchar(128);index;default:''"`
	ExternalUserId    string  `json:"external_user_id,omitempty" gorm:"type:varchar(128);index;default:''"`
	BudgetPool        string  `json:"budget_pool,omitempty" gorm:"type:varchar(64);index;default:''"`
	RewardType        string  `json:"reward_type,omitempty" gorm:"type:varchar(64);index;default:''"`
	RiskLevel         string  `json:"risk_level,omitempty" gorm:"type:varchar(32);index;default:''"`
	Tags              string  `json:"tags,omitempty" gorm:"type:varchar(512);default:''"`
	Other             string  `json:"other"`
	IdempotencyKey    *string `json:"idempotency_key,omitempty" gorm:"type:varchar(128);uniqueIndex"`
}

// don't use iota, avoid change log type value
const (
	LogTypeUnknown = 0
	LogTypeTopup   = 1
	LogTypeConsume = 2
	LogTypeManage  = 3
	LogTypeSystem  = 4
	LogTypeError   = 5
	LogTypeRefund  = 6
	LogTypeLogin   = 7
)

func formatUserLogs(logs []*Log, startIdx int) {
	for i := range logs {
		logs[i].ChannelName = ""
		var otherMap map[string]interface{}
		otherMap, _ = common.StrToMap(logs[i].Other)
		if otherMap != nil {
			// Remove admin-only debug fields.
			delete(otherMap, "admin_info")
			// Remove operation-audit details (operator/route info), admin-only.
			delete(otherMap, "audit_info")
			// delete(otherMap, "reject_reason")
			delete(otherMap, "stream_status")
		}
		logs[i].Other = common.MapToJsonStr(otherMap)
		logs[i].Id = startIdx + i + 1
	}
}

func GetLogByTokenId(tokenId int) (logs []*Log, err error) {
	err = LOG_DB.Model(&Log{}).Where("token_id = ?", tokenId).Order("id desc").Limit(common.MaxRecentItems).Find(&logs).Error
	formatUserLogs(logs, 0)
	return logs, err
}

func RecordLog(userId int, logType int, content string) {
	RecordLogEvent(userId, logType, content, LogEventOptions{})
}

// RecordLogWithAdminInfo 记录操作日志，并将管理员相关信息存入 Other.admin_info，
func RecordLogWithAdminInfo(userId int, logType int, content string, adminInfo map[string]interface{}) {
	other := map[string]interface{}{}
	if len(adminInfo) > 0 {
		other["admin_info"] = adminInfo
	}
	RecordLogEvent(userId, logType, content, LogEventOptions{
		Category: "audit",
		Source:   "admin",
		Other:    other,
	})
}

// buildOpField 构建语言无关的操作描述（写入 Other.op）。
// 前端依据 action(稳定操作标识) + params(结构化参数) 在渲染期用 i18n 本地化展示，
// 因此不在数据库中存储自然语言句子。
func buildOpField(action string, params map[string]interface{}) map[string]interface{} {
	op := map[string]interface{}{
		"action": action,
	}
	if len(params) > 0 {
		op["params"] = params
	}
	return op
}

func normalizeLogTags(tags []string) string {
	if len(tags) == 0 {
		return ""
	}
	seen := make(map[string]struct{}, len(tags))
	out := make([]string, 0, len(tags))
	for _, tag := range tags {
		tag = strings.ToLower(strings.TrimSpace(tag))
		if tag == "" {
			continue
		}
		if _, ok := seen[tag]; ok {
			continue
		}
		seen[tag] = struct{}{}
		out = append(out, tag)
	}
	return strings.Join(out, ",")
}

func cloneLogOther(other map[string]interface{}) map[string]interface{} {
	if len(other) == 0 {
		return map[string]interface{}{}
	}
	out := make(map[string]interface{}, len(other))
	for k, v := range other {
		out[k] = v
	}
	return out
}

func firstNonEmptyLogString(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func buildLogEntry(userId int, logType int, content string, opts LogEventOptions) *Log {
	createdAt := opts.CreatedAt
	if createdAt == 0 {
		createdAt = common.GetTimestamp()
	}
	username := strings.TrimSpace(opts.Username)
	if username == "" && userId > 0 {
		username, _ = GetUsernameById(userId, false)
	}
	siteID := strings.TrimSpace(opts.SiteId)
	if siteID == "" {
		siteID = currentAgentSiteID()
	}
	return &Log{
		UserId:            userId,
		Username:          username,
		CreatedAt:         createdAt,
		Type:              logType,
		Content:           content,
		TokenName:         strings.TrimSpace(opts.TokenName),
		ModelName:         strings.TrimSpace(opts.ModelName),
		Quota:             opts.Quota,
		PromptTokens:      opts.PromptTokens,
		CompletionTokens:  opts.CompletionTokens,
		UseTime:           opts.UseTime,
		IsStream:          opts.IsStream,
		ChannelId:         opts.ChannelId,
		TokenId:           opts.TokenId,
		Group:             strings.TrimSpace(opts.Group),
		Ip:                strings.TrimSpace(opts.Ip),
		RequestId:         strings.TrimSpace(opts.RequestId),
		UpstreamRequestId: strings.TrimSpace(opts.UpstreamRequestId),
		SiteId:            siteID,
		Category:          strings.TrimSpace(opts.Category),
		Source:            strings.TrimSpace(opts.Source),
		Action:            strings.TrimSpace(opts.Action),
		Status:            strings.TrimSpace(opts.Status),
		RoomId:            strings.TrimSpace(opts.RoomId),
		ExternalUserId:    strings.TrimSpace(opts.ExternalUserId),
		BudgetPool:        strings.TrimSpace(opts.BudgetPool),
		RewardType:        strings.TrimSpace(opts.RewardType),
		RiskLevel:         strings.TrimSpace(opts.RiskLevel),
		Tags:              normalizeLogTags(opts.Tags),
		Other:             common.MapToJsonStr(cloneLogOther(opts.Other)),
	}
}

func RecordLogEvent(userId int, logType int, content string, opts LogEventOptions) {
	if logType == LogTypeConsume && !common.LogConsumeEnabled {
		return
	}
	log := buildLogEntry(userId, logType, content, opts)
	if err := LOG_DB.Create(log).Error; err != nil {
		common.SysLog("failed to record log: " + err.Error())
	}
}

func resolveAuditStatus(auditInfo map[string]interface{}) string {
	if len(auditInfo) == 0 {
		return ""
	}
	if success, ok := auditInfo["success"].(bool); ok {
		if success {
			return "success"
		}
		return "failed"
	}
	if status, ok := auditInfo["status"]; ok {
		switch v := status.(type) {
		case int:
			if v >= 200 && v < 400 {
				return "success"
			}
			return "failed"
		case int32:
			if v >= 200 && v < 400 {
				return "success"
			}
			return "failed"
		case int64:
			if v >= 200 && v < 400 {
				return "success"
			}
			return "failed"
		case float64:
			if v >= 200 && v < 400 {
				return "success"
			}
			return "failed"
		}
	}
	return ""
}

func escapeLikeLiteral(input string) string {
	input = strings.ReplaceAll(input, "!", "!!")
	input = strings.ReplaceAll(input, "%", "!%")
	input = strings.ReplaceAll(input, "_", "!_")
	return input
}

func applyContainsLogFilter(tx *gorm.DB, column string, value string) (*gorm.DB, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return tx, nil
	}
	if strings.Contains(value, "%") {
		pattern, err := sanitizeLikePattern(value)
		if err != nil {
			return nil, err
		}
		return tx.Where(column+" LIKE ? ESCAPE '!'", pattern), nil
	}
	return tx.Where(column+" LIKE ? ESCAPE '!'", "%"+escapeLikeLiteral(value)+"%"), nil
}

func applyLogQueryFilters(tx *gorm.DB, filter LogQueryFilter, includeUsername bool) (*gorm.DB, error) {
	var err error
	if filter.LogType != LogTypeUnknown {
		tx = tx.Where("logs.type = ?", filter.LogType)
	}
	if tx, err = applyExplicitLogTextFilter(tx, "logs.model_name", filter.ModelName); err != nil {
		return nil, err
	}
	if includeUsername {
		if tx, err = applyExplicitLogTextFilter(tx, "logs.username", filter.Username); err != nil {
			return nil, err
		}
	}
	if filter.TokenName != "" {
		tx = tx.Where("logs.token_name = ?", filter.TokenName)
	}
	if filter.RequestId != "" {
		tx = tx.Where("logs.request_id = ?", filter.RequestId)
	}
	if filter.UpstreamRequestId != "" {
		tx = tx.Where("logs.upstream_request_id = ?", filter.UpstreamRequestId)
	}
	if filter.StartTimestamp != 0 {
		tx = tx.Where("logs.created_at >= ?", filter.StartTimestamp)
	}
	if filter.EndTimestamp != 0 {
		tx = tx.Where("logs.created_at <= ?", filter.EndTimestamp)
	}
	if filter.Channel != 0 {
		tx = tx.Where("logs.channel_id = ?", filter.Channel)
	}
	if filter.Group != "" {
		tx = tx.Where("logs."+logGroupCol+" = ?", filter.Group)
	}
	if tx, err = applyExplicitLogTextFilter(tx, "logs.site_id", filter.SiteId); err != nil {
		return nil, err
	}
	if tx, err = applyExplicitLogTextFilter(tx, "logs.category", filter.Category); err != nil {
		return nil, err
	}
	if tx, err = applyExplicitLogTextFilter(tx, "logs.source", filter.Source); err != nil {
		return nil, err
	}
	if tx, err = applyExplicitLogTextFilter(tx, "logs.action", filter.Action); err != nil {
		return nil, err
	}
	if tx, err = applyExplicitLogTextFilter(tx, "logs.status", filter.Status); err != nil {
		return nil, err
	}
	if tx, err = applyExplicitLogTextFilter(tx, "logs.room_id", filter.RoomId); err != nil {
		return nil, err
	}
	if tx, err = applyExplicitLogTextFilter(tx, "logs.external_user_id", filter.ExternalUserId); err != nil {
		return nil, err
	}
	if tx, err = applyExplicitLogTextFilter(tx, "logs.budget_pool", filter.BudgetPool); err != nil {
		return nil, err
	}
	if tx, err = applyExplicitLogTextFilter(tx, "logs.reward_type", filter.RewardType); err != nil {
		return nil, err
	}
	if tx, err = applyExplicitLogTextFilter(tx, "logs.risk_level", filter.RiskLevel); err != nil {
		return nil, err
	}
	if tx, err = applyContainsLogFilter(tx, "logs.tags", filter.Tags); err != nil {
		return nil, err
	}
	return tx, nil
}

// RecordLoginLog 记录用户登录成功的审计日志（type=LogTypeLogin）。
// username 由调用方传入（登录流程已持有用户对象），避免额外的数据库查询。
// content 为英文兜底文本（用于导出/经典前端）；action+params 供前端本地化渲染。
// extra 可携带 login_method、user_agent 等附加信息（普通用户可见）。
func RecordLoginLog(userId int, username string, content string, ip string, action string, params map[string]interface{}, extra map[string]interface{}) {
	other := map[string]interface{}{}
	for k, v := range extra {
		other[k] = v
	}
	other["op"] = buildOpField(action, params)
	loginSource := "web"
	if method, ok := other["login_method"].(string); ok && strings.TrimSpace(method) != "" {
		loginSource = strings.TrimSpace(method)
	}
	RecordLogEvent(userId, LogTypeLogin, content, LogEventOptions{
		Username: username,
		Ip:       ip,
		Category: "auth",
		Source:   loginSource,
		Action:   action,
		Status:   "success",
		Other:    other,
	})
}

// RecordOperationAuditLog 记录管理/高危操作审计日志（type=LogTypeManage）。
// logUserId 为日志归属者（面向用户的操作如额度调整归属目标用户，资源类操作如渠道/系统设置归属操作者），
// username 内部按 logUserId 查询。content 为英文兜底文本（导出/经典前端用）。
// action+params 写入 Other.op，供前端本地化渲染（普通用户可见，不含敏感信息）。
// adminInfo 存放操作者身份（写入 Other.admin_info，普通用户查询时剥离）；
// auditInfo 存放路由/方法/结果等中间件兜底信息（写入 Other.audit_info，普通用户查询时剥离）。
func RecordOperationAuditLog(logUserId int, content string, ip string, action string, params map[string]interface{}, adminInfo map[string]interface{}, auditInfo map[string]interface{}) {
	other := map[string]interface{}{
		"op": buildOpField(action, params),
	}
	if len(adminInfo) > 0 {
		other["admin_info"] = adminInfo
	}
	if len(auditInfo) > 0 {
		other["audit_info"] = auditInfo
	}
	RecordLogEvent(logUserId, LogTypeManage, content, LogEventOptions{
		Ip:       ip,
		Category: "audit",
		Source:   "admin",
		Action:   action,
		Status:   resolveAuditStatus(auditInfo),
		Other:    other,
	})
}

func RecordTopupLog(userId int, content string, callerIp string, paymentMethod string, callbackPaymentMethod string) {
	adminInfo := map[string]interface{}{
		"server_ip":               common.GetIp(),
		"node_name":               common.NodeName,
		"caller_ip":               callerIp,
		"payment_method":          paymentMethod,
		"callback_payment_method": callbackPaymentMethod,
		"version":                 common.Version,
	}
	other := map[string]interface{}{
		"admin_info": adminInfo,
	}
	RecordLogEvent(userId, LogTypeTopup, content, LogEventOptions{
		Ip:         callerIp,
		Category:   "billing",
		Source:     "web",
		Action:     "topup",
		Status:     "success",
		BudgetPool: "",
		Other:      other,
	})
}

func RecordErrorLog(c *gin.Context, userId int, channelId int, modelName string, tokenName string, content string, tokenId int, useTimeSeconds int,
	isStream bool, group string, other map[string]interface{}) {
	logger.LogInfo(c, fmt.Sprintf("record error log: userId=%d, channelId=%d, modelName=%s, tokenName=%s, content=%s", userId, channelId, modelName, tokenName, common.LocalLogPreview(content)))
	username := c.GetString("username")
	requestId := c.GetString(common.RequestIdKey)
	upstreamRequestId := c.GetString(common.UpstreamRequestIdKey)
	// 判断是否需要记录 IP
	needRecordIp := false
	if settingMap, err := GetUserSetting(userId, false); err == nil {
		if settingMap.RecordIpLog {
			needRecordIp = true
		}
	}
	RecordLogEvent(userId, LogTypeError, content, LogEventOptions{
		Username:  username,
		TokenName: tokenName,
		ModelName: modelName,
		ChannelId: channelId,
		TokenId:   tokenId,
		UseTime:   useTimeSeconds,
		IsStream:  isStream,
		Group:     group,
		Ip: func() string {
			if needRecordIp {
				return c.ClientIP()
			}
			return ""
		}(),
		RequestId:         requestId,
		UpstreamRequestId: upstreamRequestId,
		Category:          "api",
		Source:            "upstream",
		Action:            "request_error",
		Status:            "failed",
		Other:             other,
	})
}

type RecordConsumeLogParams struct {
	ChannelId        int                    `json:"channel_id"`
	PromptTokens     int                    `json:"prompt_tokens"`
	CompletionTokens int                    `json:"completion_tokens"`
	ModelName        string                 `json:"model_name"`
	TokenName        string                 `json:"token_name"`
	Quota            int                    `json:"quota"`
	Content          string                 `json:"content"`
	TokenId          int                    `json:"token_id"`
	UseTimeSeconds   int                    `json:"use_time_seconds"`
	IsStream         bool                   `json:"is_stream"`
	Group            string                 `json:"group"`
	Other            map[string]interface{} `json:"other"`
}

func RecordConsumeLog(c *gin.Context, userId int, params RecordConsumeLogParams) {
	if !common.LogConsumeEnabled {
		return
	}
	logger.LogInfo(c, fmt.Sprintf("record consume log: userId=%d, params=%s", userId, common.GetJsonString(params)))
	username := c.GetString("username")
	requestId := c.GetString(common.RequestIdKey)
	upstreamRequestId := c.GetString(common.UpstreamRequestIdKey)
	// 判断是否需要记录 IP
	needRecordIp := false
	if settingMap, err := GetUserSetting(userId, false); err == nil {
		if settingMap.RecordIpLog {
			needRecordIp = true
		}
	}
	RecordLogEvent(userId, LogTypeConsume, params.Content, LogEventOptions{
		Username:         username,
		Quota:            params.Quota,
		PromptTokens:     params.PromptTokens,
		CompletionTokens: params.CompletionTokens,
		UseTime:          params.UseTimeSeconds,
		IsStream:         params.IsStream,
		ChannelId:        params.ChannelId,
		TokenId:          params.TokenId,
		TokenName:        params.TokenName,
		ModelName:        params.ModelName,
		Group:            params.Group,
		Ip: func() string {
			if needRecordIp {
				return c.ClientIP()
			}
			return ""
		}(),
		RequestId:         requestId,
		UpstreamRequestId: upstreamRequestId,
		Category:          "api",
		Source:            "upstream",
		Action:            "consume",
		Status:            "success",
		Other:             params.Other,
	})
	if common.DataExportEnabled {
		gopool.Go(func() {
			LogQuotaData(userId, username, params.ModelName, params.Quota, common.GetTimestamp(), params.PromptTokens+params.CompletionTokens)
		})
	}
}

type RecordTaskBillingLogParams struct {
	UserId    int
	LogType   int
	Content   string
	ChannelId int
	ModelName string
	Quota     int
	TokenId   int
	Group     string
	Other     map[string]interface{}
}

func RecordTaskBillingLog(params RecordTaskBillingLogParams) {
	_, _ = recordTaskBillingLog(params, "")
}

// RecordTaskBillingLogIdempotent inserts at most one billing log for an outbox operation.
// A nullable unique key keeps all existing non-outbox log writers unchanged.
func RecordTaskBillingLogIdempotent(params RecordTaskBillingLogParams, idempotencyKey string) (bool, error) {
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if idempotencyKey == "" {
		return false, errors.New("task billing log idempotency key is empty")
	}
	if params.LogType == LogTypeConsume && !common.LogConsumeEnabled {
		return false, nil
	}
	return recordTaskBillingLog(params, idempotencyKey)
}

func recordTaskBillingLog(params RecordTaskBillingLogParams, idempotencyKey string) (bool, error) {
	if params.LogType == LogTypeConsume && !common.LogConsumeEnabled {
		return false, nil
	}
	username, _ := GetUsernameById(params.UserId, false)
	tokenName := ""
	if params.TokenId > 0 {
		if token, err := GetTokenById(params.TokenId); err == nil {
			tokenName = token.Name
		}
	}
	logEntry := buildLogEntry(params.UserId, params.LogType, params.Content, LogEventOptions{
		Username:  username,
		TokenName: tokenName,
		ModelName: params.ModelName,
		Quota:     params.Quota,
		ChannelId: params.ChannelId,
		TokenId:   params.TokenId,
		Group:     params.Group,
		Category:  "task",
		Source:    "system",
		Action:    "task_billing",
		Status:    "success",
		Other:     params.Other,
	})
	if idempotencyKey == "" {
		if err := LOG_DB.Create(logEntry).Error; err != nil {
			common.SysLog("failed to record task billing log: " + err.Error())
			return false, err
		}
		return true, nil
	}
	key := idempotencyKey
	logEntry.IdempotencyKey = &key
	result := LOG_DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "idempotency_key"}},
		DoNothing: true,
	}).Create(logEntry)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

func GetAllLogs(filter LogQueryFilter) (logs []*Log, total int64, err error) {
	tx := LOG_DB
	if tx, err = applyLogQueryFilters(tx, filter, true); err != nil {
		return nil, 0, err
	}
	err = tx.Model(&Log{}).Count(&total).Error
	if err != nil {
		return nil, 0, err
	}
	err = tx.Order("logs.created_at desc, logs.id desc").Limit(filter.Num).Offset(filter.StartIdx).Find(&logs).Error
	if err != nil {
		return nil, 0, err
	}

	channelIds := types.NewSet[int]()
	for _, log := range logs {
		if log.ChannelId != 0 {
			channelIds.Add(log.ChannelId)
		}
	}

	if channelIds.Len() > 0 {
		var channels []struct {
			Id   int    `gorm:"column:id"`
			Name string `gorm:"column:name"`
		}
		if common.MemoryCacheEnabled {
			// Cache get channel
			for _, channelId := range channelIds.Items() {
				if cacheChannel, err := CacheGetChannel(channelId); err == nil {
					channels = append(channels, struct {
						Id   int    `gorm:"column:id"`
						Name string `gorm:"column:name"`
					}{
						Id:   channelId,
						Name: cacheChannel.Name,
					})
				}
			}
		} else {
			// Bulk query channels from DB
			if err = DB.Table("channels").Select("id, name").Where("id IN ?", channelIds.Items()).Find(&channels).Error; err != nil {
				return logs, total, err
			}
		}
		channelMap := make(map[int]string, len(channels))
		for _, channel := range channels {
			channelMap[channel.Id] = channel.Name
		}
		for i := range logs {
			logs[i].ChannelName = channelMap[logs[i].ChannelId]
		}
	}

	return logs, total, err
}

const logSearchCountLimit = 10000

func GetUserLogs(userId int, filter LogQueryFilter) (logs []*Log, total int64, err error) {
	var tx *gorm.DB
	if filter.LogType == LogTypeUnknown {
		tx = LOG_DB.Where("logs.user_id = ?", userId)
	} else {
		tx = LOG_DB.Where("logs.user_id = ? and logs.type = ?", userId, filter.LogType)
	}
	if tx, err = applyLogQueryFilters(tx, filter, false); err != nil {
		return nil, 0, err
	}
	err = tx.Model(&Log{}).Limit(logSearchCountLimit).Count(&total).Error
	if err != nil {
		common.SysError("failed to count user logs: " + err.Error())
		return nil, 0, errors.New("查询日志失败")
	}
	err = tx.Order("logs.id desc").Limit(filter.Num).Offset(filter.StartIdx).Find(&logs).Error
	if err != nil {
		common.SysError("failed to search user logs: " + err.Error())
		return nil, 0, errors.New("查询日志失败")
	}

	formatUserLogs(logs, filter.StartIdx)
	return logs, total, err
}

type Stat struct {
	Quota int `json:"quota"`
	Rpm   int `json:"rpm"`
	Tpm   int `json:"tpm"`
}

func SumUsedQuota(filter LogQueryFilter) (stat Stat, err error) {
	tx := LOG_DB.Table("logs").Select("sum(quota) quota")

	// 为rpm和tpm创建单独的查询
	rpmTpmQuery := LOG_DB.Table("logs").Select("count(*) rpm, sum(prompt_tokens) + sum(completion_tokens) tpm")

	if tx, err = applyExplicitLogTextFilter(tx, "username", filter.Username); err != nil {
		return stat, err
	}
	if rpmTpmQuery, err = applyExplicitLogTextFilter(rpmTpmQuery, "username", filter.Username); err != nil {
		return stat, err
	}
	if filter.TokenName != "" {
		tx = tx.Where("token_name = ?", filter.TokenName)
		rpmTpmQuery = rpmTpmQuery.Where("token_name = ?", filter.TokenName)
	}
	if filter.StartTimestamp != 0 {
		tx = tx.Where("created_at >= ?", filter.StartTimestamp)
	}
	if filter.EndTimestamp != 0 {
		tx = tx.Where("created_at <= ?", filter.EndTimestamp)
	}
	if tx, err = applyExplicitLogTextFilter(tx, "model_name", filter.ModelName); err != nil {
		return stat, err
	}
	if rpmTpmQuery, err = applyExplicitLogTextFilter(rpmTpmQuery, "model_name", filter.ModelName); err != nil {
		return stat, err
	}
	if filter.Channel != 0 {
		tx = tx.Where("channel_id = ?", filter.Channel)
		rpmTpmQuery = rpmTpmQuery.Where("channel_id = ?", filter.Channel)
	}
	if filter.Group != "" {
		tx = tx.Where(logGroupCol+" = ?", filter.Group)
		rpmTpmQuery = rpmTpmQuery.Where(logGroupCol+" = ?", filter.Group)
	}
	if filter.SiteId != "" {
		tx = tx.Where("site_id = ?", filter.SiteId)
		rpmTpmQuery = rpmTpmQuery.Where("site_id = ?", filter.SiteId)
	}
	if filter.Category != "" {
		tx = tx.Where("category = ?", filter.Category)
		rpmTpmQuery = rpmTpmQuery.Where("category = ?", filter.Category)
	}
	if filter.Source != "" {
		tx = tx.Where("source = ?", filter.Source)
		rpmTpmQuery = rpmTpmQuery.Where("source = ?", filter.Source)
	}
	if filter.Action != "" {
		tx = tx.Where("action = ?", filter.Action)
		rpmTpmQuery = rpmTpmQuery.Where("action = ?", filter.Action)
	}
	if filter.Status != "" {
		tx = tx.Where("status = ?", filter.Status)
		rpmTpmQuery = rpmTpmQuery.Where("status = ?", filter.Status)
	}
	if filter.RoomId != "" {
		tx = tx.Where("room_id = ?", filter.RoomId)
		rpmTpmQuery = rpmTpmQuery.Where("room_id = ?", filter.RoomId)
	}
	if filter.BudgetPool != "" {
		tx = tx.Where("budget_pool = ?", filter.BudgetPool)
		rpmTpmQuery = rpmTpmQuery.Where("budget_pool = ?", filter.BudgetPool)
	}
	if filter.RewardType != "" {
		tx = tx.Where("reward_type = ?", filter.RewardType)
		rpmTpmQuery = rpmTpmQuery.Where("reward_type = ?", filter.RewardType)
	}
	if filter.RiskLevel != "" {
		tx = tx.Where("risk_level = ?", filter.RiskLevel)
		rpmTpmQuery = rpmTpmQuery.Where("risk_level = ?", filter.RiskLevel)
	}

	tx = tx.Where("type = ?", LogTypeConsume)
	rpmTpmQuery = rpmTpmQuery.Where("type = ?", LogTypeConsume)

	// 只统计最近60秒的rpm和tpm
	rpmTpmQuery = rpmTpmQuery.Where("created_at >= ?", time.Now().Add(-60*time.Second).Unix())

	// 执行查询
	if err := tx.Scan(&stat).Error; err != nil {
		common.SysError("failed to query log stat: " + err.Error())
		return stat, errors.New("查询统计数据失败")
	}
	if err := rpmTpmQuery.Scan(&stat).Error; err != nil {
		common.SysError("failed to query rpm/tpm stat: " + err.Error())
		return stat, errors.New("查询统计数据失败")
	}

	return stat, nil
}

func SumUsedToken(logType int, startTimestamp int64, endTimestamp int64, modelName string, username string, tokenName string) (token int) {
	tx := LOG_DB.Table("logs").Select("ifnull(sum(prompt_tokens),0) + ifnull(sum(completion_tokens),0)")
	if username != "" {
		tx = tx.Where("username = ?", username)
	}
	if tokenName != "" {
		tx = tx.Where("token_name = ?", tokenName)
	}
	if startTimestamp != 0 {
		tx = tx.Where("created_at >= ?", startTimestamp)
	}
	if endTimestamp != 0 {
		tx = tx.Where("created_at <= ?", endTimestamp)
	}
	if modelName != "" {
		tx = tx.Where("model_name = ?", modelName)
	}
	tx.Where("type = ?", LogTypeConsume).Scan(&token)
	return token
}

func DeleteOldLog(ctx context.Context, targetTimestamp int64, limit int) (int64, error) {
	var total int64 = 0

	for {
		if nil != ctx.Err() {
			return total, ctx.Err()
		}

		result := LOG_DB.Where("created_at < ?", targetTimestamp).Limit(limit).Delete(&Log{})
		if nil != result.Error {
			return total, result.Error
		}

		total += result.RowsAffected

		if result.RowsAffected < int64(limit) {
			break
		}
	}

	return total, nil
}
