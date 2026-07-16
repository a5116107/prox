package controller

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/console_setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/setting/system_setting"

	"github.com/gin-gonic/gin"
)

var completionRatioMetaOptionKeys = []string{
	"ModelPrice",
	"ModelRatio",
	"CompletionRatio",
	"CacheRatio",
	"CreateCacheRatio",
	"ImageRatio",
	"AudioRatio",
	"AudioCompletionRatio",
}

var bulkUnsupportedOptionKeys = map[string]struct{}{
	"ImageRatio":           {},
	"AudioRatio":           {},
	"AudioCompletionRatio": {},
	"CreateCacheRatio":     {},
}

func isPaymentComplianceOptionKey(key string) bool {
	return strings.HasPrefix(key, "payment_setting.compliance_")
}

func isPositiveOptionValue(value string) bool {
	intValue, err := strconv.Atoi(strings.TrimSpace(value))
	if err == nil {
		return intValue > 0
	}
	floatValue, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	return err == nil && floatValue > 0
}

func collectModelNamesFromOptionValue(raw string, modelNames map[string]struct{}) {
	if strings.TrimSpace(raw) == "" {
		return
	}

	var parsed map[string]any
	if err := common.UnmarshalJsonStr(raw, &parsed); err != nil {
		return
	}

	for modelName := range parsed {
		modelNames[modelName] = struct{}{}
	}
}

func buildCompletionRatioMetaValue(optionValues map[string]string) string {
	modelNames := make(map[string]struct{})
	for _, key := range completionRatioMetaOptionKeys {
		collectModelNamesFromOptionValue(optionValues[key], modelNames)
	}

	meta := make(map[string]ratio_setting.CompletionRatioInfo, len(modelNames))
	for modelName := range modelNames {
		meta[modelName] = ratio_setting.GetCompletionRatioInfo(modelName)
	}

	jsonBytes, err := common.Marshal(meta)
	if err != nil {
		return "{}"
	}
	return string(jsonBytes)
}

func GetOptions(c *gin.Context) {
	var options []*model.Option
	optionValues := make(map[string]string)
	common.OptionMapRWMutex.Lock()
	for k, v := range common.OptionMap {
		value := common.Interface2String(v)
		isSensitiveKey := strings.HasSuffix(k, "Token") ||
			strings.HasSuffix(k, "Secret") ||
			strings.HasSuffix(k, "Key") ||
			strings.HasSuffix(k, "secret") ||
			strings.HasSuffix(k, "api_key")
		if isSensitiveKey {
			continue
		}
		options = append(options, &model.Option{
			Key:   k,
			Value: value,
		})
		for _, optionKey := range completionRatioMetaOptionKeys {
			if optionKey == k {
				optionValues[k] = value
				break
			}
		}
	}
	common.OptionMapRWMutex.Unlock()
	options = append(options, &model.Option{
		Key:   "CompletionRatioMeta",
		Value: buildCompletionRatioMetaValue(optionValues),
	})
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    options,
	})
}

type OptionUpdateRequest struct {
	Key   string `json:"key"`
	Value any    `json:"value"`
}

type OptionBulkUpdateRequest struct {
	Updates []OptionUpdateRequest `json:"updates"`
}

func normalizeOptionUpdateValue(value any) string {
	switch typed := value.(type) {
	case bool:
		return common.Interface2String(typed)
	case float64:
		return common.Interface2String(typed)
	case int:
		return common.Interface2String(typed)
	default:
		return fmt.Sprintf("%v", typed)
	}
}

func validateOptionUpdate(c *gin.Context, key string, value string) bool {
	if strings.HasPrefix(key, "agent_setting.image_") {
		if err := operation_setting.ValidateAgentImageOption(key, value); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return false
		}
	}
	switch key {
	case "QuotaForInviter", "QuotaForInvitee":
		if isPositiveOptionValue(value) && !operation_setting.IsPaymentComplianceConfirmed() {
			common.ApiErrorI18n(c, i18n.MsgPaymentComplianceRequired)
			return false
		}
	default:
		if isPaymentComplianceOptionKey(key) {
			common.ApiErrorMsg(c, "合规确认字段不允许通过通用设置接口修改")
			return false
		}
	}

	switch key {
	case "GitHubOAuthEnabled":
		if value == "true" && common.GitHubClientId == "" {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无法启用 GitHub OAuth，请先填入 GitHub Client Id 以及 GitHub Client Secret！",
			})
			return false
		}
	case "discord.enabled":
		if value == "true" && system_setting.GetDiscordSettings().ClientId == "" {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无法启用 Discord OAuth，请先填入 Discord Client Id 以及 Discord Client Secret！",
			})
			return false
		}
	case "oidc.enabled":
		if value == "true" && system_setting.GetOIDCSettings().ClientId == "" {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无法启用 OIDC 登录，请先填入 OIDC Client Id 以及 OIDC Client Secret！",
			})
			return false
		}
	case "LinuxDOOAuthEnabled":
		if value == "true" && common.LinuxDOClientId == "" {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无法启用 LinuxDO OAuth，请先填入 LinuxDO Client Id 以及 LinuxDO Client Secret！",
			})
			return false
		}
	case "EmailDomainRestrictionEnabled":
		if value == "true" && len(common.EmailDomainWhitelist) == 0 {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无法启用邮箱域名限制，请先填入限制的邮箱域名！",
			})
			return false
		}
	case "WeChatAuthEnabled":
		if value == "true" && common.WeChatServerAddress == "" {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无法启用微信登录，请先填入微信登录相关配置信息！",
			})
			return false
		}
	case "TurnstileCheckEnabled":
		if value == "true" && common.TurnstileSiteKey == "" {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无法启用 Turnstile 校验，请先填入 Turnstile 校验相关配置信息！",
			})

			return false
		}
	case "TelegramOAuthEnabled":
		if value == "true" && common.TelegramBotToken == "" {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无法启用 Telegram OAuth，请先填入 Telegram Bot Token！",
			})
			return false
		}
	case "theme.frontend":
		if value != "default" && value != "classic" {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无效的主题值，可选值：default（新版前端）、classic（经典前端）",
			})
			return false
		}
	case "GroupRatio":
		err := ratio_setting.CheckGroupRatio(value)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return false
		}
	case "ImageRatio":
		err := ratio_setting.UpdateImageRatioByJSONString(value)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "图片倍率设置失败: " + err.Error(),
			})
			return false
		}
	case "AudioRatio":
		err := ratio_setting.UpdateAudioRatioByJSONString(value)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "音频倍率设置失败: " + err.Error(),
			})
			return false
		}
	case "AudioCompletionRatio":
		err := ratio_setting.UpdateAudioCompletionRatioByJSONString(value)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "音频补全倍率设置失败: " + err.Error(),
			})
			return false
		}
	case "CreateCacheRatio":
		err := ratio_setting.UpdateCreateCacheRatioByJSONString(value)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "缓存创建倍率设置失败: " + err.Error(),
			})
			return false
		}
	case "ModelRequestRateLimitGroup":
		err := setting.CheckModelRequestRateLimitGroup(value)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return false
		}
	case "AutomaticDisableStatusCodes":
		_, err := operation_setting.ParseHTTPStatusCodeRanges(value)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return false
		}
	case "AutomaticRetryStatusCodes":
		_, err := operation_setting.ParseHTTPStatusCodeRanges(value)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return false
		}
	case "console_setting.api_info":
		err := console_setting.ValidateConsoleSettings(value, "ApiInfo")
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return false
		}
	case "console_setting.announcements":
		err := console_setting.ValidateConsoleSettings(value, "Announcements")
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return false
		}
	case "console_setting.faq":
		err := console_setting.ValidateConsoleSettings(value, "FAQ")
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return false
		}
	case "console_setting.uptime_kuma_groups":
		err := console_setting.ValidateConsoleSettings(value, "UptimeKumaGroups")
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return false
		}
	}

	return true
}

func UpdateOption(c *gin.Context) {
	var option OptionUpdateRequest
	err := common.DecodeJson(c.Request.Body, &option)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的参数",
		})
		return
	}
	normalizedValue := normalizeOptionUpdateValue(option.Value)
	if !validateOptionUpdate(c, option.Key, normalizedValue) {
		return
	}
	err = model.UpdateOption(option.Key, normalizedValue)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	// 出于安全考虑只记录被修改的配置项名称，不记录配置值（可能含密钥等敏感信息）。
	recordManageAudit(c, "option.update", map[string]interface{}{
		"key": option.Key,
	})
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

func UpdateOptionsBulk(c *gin.Context) {
	var request OptionBulkUpdateRequest
	err := common.DecodeJson(c.Request.Body, &request)
	if err != nil || len(request.Updates) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的参数",
		})
		return
	}

	values := make(map[string]string, len(request.Updates))
	keys := make([]string, 0, len(request.Updates))
	for _, option := range request.Updates {
		if strings.TrimSpace(option.Key) == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "无效的参数",
			})
			return
		}
		if _, blocked := bulkUnsupportedOptionKeys[option.Key]; blocked {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "该配置暂不支持批量保存，请使用单项保存接口",
			})
			return
		}
		normalizedValue := normalizeOptionUpdateValue(option.Value)
		if !validateOptionUpdate(c, option.Key, normalizedValue) {
			return
		}
		values[option.Key] = normalizedValue
		keys = append(keys, option.Key)
	}

	err = model.UpdateOptionsBulk(values)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	recordManageAudit(c, "option.update_bulk", map[string]interface{}{
		"count": len(keys),
		"keys":  keys,
	})
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}
