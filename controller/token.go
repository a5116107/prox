package controller

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func accessControlViolationPayload(ctx context.Context, userID int, state *model.UserSiteAccessState, requestedGroup string, action string, err error) gin.H {
	effectiveState := state
	if violation, ok := service.IsAccessControlViolation(err); ok && effectiveState == nil {
		effectiveState = violation.State
	}
	payload := gin.H{"access_control": gin.H{}}
	accessPayload := payload["access_control"].(gin.H)
	if userID > 0 {
		if status, statusErr := service.GetUserAccessControlStatus(ctx, userID, false); statusErr == nil && status != nil {
			for key, value := range status {
				accessPayload[key] = value
			}
		}
	}
	if effectiveState != nil {
		accessPayload["state"] = effectiveState
		accessPayload["access_level"] = effectiveState.AccessLevel
		accessPayload["reason_code"] = effectiveState.ReasonCode
		accessPayload["reason_message"] = effectiveState.ReasonMessage
		accessPayload["effective_groups"] = effectiveState.EffectiveGroupList()
		accessPayload["next_steps"] = service.BuildAccessControlNextSteps(effectiveState)
		accessPayload["primary_bound"] = effectiveState.PrimaryBound
		accessPayload["community_bound"] = effectiveState.CommunityBound
		accessPayload["primary_platform"] = effectiveState.PrimaryPlatform
		accessPayload["matched_primary_group_id"] = effectiveState.MatchedPrimaryGroupId
	}
	accessPayload["requested_group"] = strings.TrimSpace(requestedGroup)
	accessPayload["action"] = strings.TrimSpace(action)
	if violation, ok := service.IsAccessControlViolation(err); ok {
		accessLevel := ""
		primaryBound := false
		communityBound := false
		primaryPlatform := ""
		matchedPrimaryGroupID := ""
		if effectiveState != nil {
			accessLevel = effectiveState.AccessLevel
			primaryBound = effectiveState.PrimaryBound
			communityBound = effectiveState.CommunityBound
			primaryPlatform = effectiveState.PrimaryPlatform
			matchedPrimaryGroupID = effectiveState.MatchedPrimaryGroupId
		}
		accessPayload["state"] = effectiveState
		accessPayload["access_level"] = accessLevel
		accessPayload["reason_code"] = violation.Code
		accessPayload["reason_message"] = violation.Message
		accessPayload["effective_groups"] = violation.AllowedGroups
		accessPayload["next_steps"] = service.BuildAccessControlNextSteps(effectiveState)
		accessPayload["requested_group"] = violation.RequestedGroup
		accessPayload["action"] = violation.Action
		accessPayload["primary_bound"] = primaryBound
		accessPayload["community_bound"] = communityBound
		accessPayload["primary_platform"] = primaryPlatform
		accessPayload["matched_primary_group_id"] = matchedPrimaryGroupID
		if _, exists := accessPayload["denied_message"]; !exists {
			accessPayload["denied_message"] = violation.Message
		}
	}
	return payload
}

func recordAccessControlTokenDecision(c *gin.Context, state *model.UserSiteAccessState, action string, requestedGroup string, err error) {
	status := "success"
	reasonCode := ""
	reasonMessage := ""
	effectiveState := state
	allowedGroups := []string{}
	if violation, ok := service.IsAccessControlViolation(err); ok {
		status = "denied"
		reasonCode = violation.Code
		reasonMessage = violation.Message
		allowedGroups = violation.AllowedGroups
		if effectiveState == nil {
			effectiveState = violation.State
		}
	}
	if effectiveState != nil && status == "success" {
		reasonCode = effectiveState.ReasonCode
		reasonMessage = effectiveState.ReasonMessage
		allowedGroups = effectiveState.EffectiveGroupList()
	}
	model.RecordLogEvent(c.GetInt("id"), model.LogTypeSystem, service.BuildAccessControlLogMessage(action, requestedGroup, effectiveState, allowedGroups, status == "denied"), model.LogEventOptions{
		SiteId:   service.AgentSiteID(),
		Category: "access_control",
		Source:   "web",
		Action:   fmt.Sprintf("token_%s", strings.ToLower(strings.TrimSpace(action))),
		Status:   status,
		Other: map[string]any{
			"access_control": map[string]any{
				"requested_group": strings.TrimSpace(requestedGroup),
				"reason_code":     reasonCode,
				"reason_message":  reasonMessage,
				"access_level": func() string {
					if effectiveState != nil {
						return effectiveState.AccessLevel
					}
					return ""
				}(),
				"effective_groups": allowedGroups,
				"next_steps": func() []string {
					if effectiveState != nil {
						return service.BuildAccessControlNextSteps(effectiveState)
					}
					return []string{}
				}(),
			},
		},
	})
}

func buildMaskedTokenResponse(token *model.Token) *model.Token {
	if token == nil {
		return nil
	}
	_ = model.AttachCommunityGateFreezeMetadata([]*model.Token{token})
	_ = model.AttachAccessControlFreezeMetadata([]*model.Token{token})
	_ = model.AttachRiskTokenActivationMetadata([]*model.Token{token})
	maskedToken := *token
	maskedToken.Key = token.GetMaskedKey()
	return &maskedToken
}

func buildMaskedTokenResponses(tokens []*model.Token) []*model.Token {
	_ = model.AttachCommunityGateFreezeMetadata(tokens)
	_ = model.AttachAccessControlFreezeMetadata(tokens)
	_ = model.AttachRiskTokenActivationMetadata(tokens)
	maskedTokens := make([]*model.Token, 0, len(tokens))
	for _, token := range tokens {
		if token == nil {
			continue
		}
		maskedToken := *token
		maskedToken.Key = token.GetMaskedKey()
		maskedTokens = append(maskedTokens, &maskedToken)
	}
	return maskedTokens
}

func ensureAccessControlForTokenMutation(c *gin.Context, requestedGroup string, action string) (string, bool) {
	ctx, cancel := contextWithTimeout(c, 60*time.Second)
	defer cancel()
	groupName, state, err := service.EnsureUserCanCreateOrEnableToken(ctx, c.GetInt("id"), requestedGroup, action)
	if err != nil {
		recordAccessControlTokenDecision(c, state, action, requestedGroup, err)
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
			"data":    accessControlViolationPayload(ctx, c.GetInt("id"), state, requestedGroup, action, err),
		})
		return "", false
	}
	recordAccessControlTokenDecision(c, state, action, requestedGroup, nil)
	return groupName, true
}

func GetAllTokens(c *gin.Context) {
	userId := c.GetInt("id")
	pageInfo := common.GetPageQuery(c)
	tokens, err := model.GetAllUserTokens(userId, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	total, _ := model.CountUserTokens(userId)
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(buildMaskedTokenResponses(tokens))
	common.ApiSuccess(c, pageInfo)
}

func SearchTokens(c *gin.Context) {
	userId := c.GetInt("id")
	keyword := c.Query("keyword")
	token := c.Query("token")

	pageInfo := common.GetPageQuery(c)

	tokens, total, err := model.SearchUserTokens(userId, keyword, token, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(buildMaskedTokenResponses(tokens))
	common.ApiSuccess(c, pageInfo)
}

func GetToken(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	userId := c.GetInt("id")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	token, err := model.GetTokenByIds(id, userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, buildMaskedTokenResponse(token))
}

func GetTokenKey(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	userId := c.GetInt("id")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	token, err := model.GetTokenByIds(id, userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"key": token.GetFullKey(),
	})
}

func GetTokenStatus(c *gin.Context) {
	tokenId := c.GetInt("token_id")
	userId := c.GetInt("id")
	token, err := model.GetTokenByIds(tokenId, userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	expiredAt := token.ExpiredTime
	if expiredAt == -1 {
		expiredAt = 0
	}
	c.JSON(http.StatusOK, gin.H{
		"object":          "credit_summary",
		"total_granted":   token.RemainQuota,
		"total_used":      0, // not supported currently
		"total_available": token.RemainQuota,
		"expires_at":      expiredAt * 1000,
	})
}

func GetTokenUsage(c *gin.Context) {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "No Authorization header",
		})
		return
	}

	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "Invalid Bearer token",
		})
		return
	}
	tokenKey := parts[1]

	token, err := model.GetTokenByKey(strings.TrimPrefix(tokenKey, "sk-"), false)
	if err != nil {
		common.SysError("failed to get token by key: " + err.Error())
		common.ApiErrorI18n(c, i18n.MsgTokenGetInfoFailed)
		return
	}

	expiredAt := token.ExpiredTime
	if expiredAt == -1 {
		expiredAt = 0
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    true,
		"message": "ok",
		"data": gin.H{
			"object":               "token_usage",
			"name":                 token.Name,
			"total_granted":        token.RemainQuota + token.UsedQuota,
			"total_used":           token.UsedQuota,
			"total_available":      token.RemainQuota,
			"unlimited_quota":      token.UnlimitedQuota,
			"model_limits":         token.GetModelLimitsMap(),
			"model_limits_enabled": token.ModelLimitsEnabled,
			"expires_at":           expiredAt,
		},
	})
}

func AddToken(c *gin.Context) {
	token := model.Token{}
	err := c.ShouldBindJSON(&token)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if len(token.Name) > 50 {
		common.ApiErrorI18n(c, i18n.MsgTokenNameTooLong)
		return
	}
	resolvedGroup, ok := ensureAccessControlForTokenMutation(c, token.Group, "create")
	if !ok {
		return
	}
	// 非无限额度时，检查额度值是否超出有效范围
	if !token.UnlimitedQuota {
		if token.RemainQuota < 0 {
			common.ApiErrorI18n(c, i18n.MsgTokenQuotaNegative)
			return
		}
		maxQuotaValue := int((1000000000 * common.QuotaPerUnit))
		if token.RemainQuota > maxQuotaValue {
			common.ApiErrorI18n(c, i18n.MsgTokenQuotaExceedMax, map[string]any{"Max": maxQuotaValue})
			return
		}
	}
	// 检查用户令牌数量是否已达上限
	maxTokens := operation_setting.GetMaxUserTokens()
	count, err := model.CountUserTokens(c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if int(count) >= maxTokens {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": fmt.Sprintf("已达到最大令牌数量限制 (%d)", maxTokens),
		})
		return
	}
	key, err := common.GenerateKey()
	if err != nil {
		common.ApiErrorI18n(c, i18n.MsgTokenGenerateFailed)
		common.SysLog("failed to generate token key: " + err.Error())
		return
	}
	cleanToken := model.Token{
		UserId:             c.GetInt("id"),
		Name:               token.Name,
		Key:                key,
		CreatedTime:        common.GetTimestamp(),
		AccessedTime:       common.GetTimestamp(),
		ExpiredTime:        token.ExpiredTime,
		RemainQuota:        token.RemainQuota,
		UnlimitedQuota:     token.UnlimitedQuota,
		ModelLimitsEnabled: token.ModelLimitsEnabled,
		ModelLimits:        token.ModelLimits,
		AllowIps:           token.AllowIps,
		Group:              resolvedGroup,
		CrossGroupRetry:    token.CrossGroupRetry,
		Status:             common.TokenStatusEnabled,
	}
	riskControl, err := service.PrepareRiskTokenActivationForUser(c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if riskControl != nil {
		cleanToken.Status = common.TokenStatusDisabled
	}
	var bindCode string
	var expiresAt int64
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&cleanToken).Error; err != nil {
			return err
		}
		if riskControl == nil {
			return nil
		}
		var activationErr error
		bindCode, expiresAt, activationErr = service.CreateRiskTokenActivationForToken(tx, c.GetInt("id"), &cleanToken, riskControl)
		return activationErr
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if riskControl != nil {
		cleanToken.RiskActivationRequired = true
		cleanToken.RiskActivationSource = riskControl.ActivationSource
		cleanToken.RiskActivationStatus = model.RiskTokenActivationPending
		cleanToken.RiskActivationExpiresAt = expiresAt
		cleanToken.RiskActivationReason = firstNonEmptyString(riskControl.Reason, "高风险账号需完成 QQ 绑定激活后，新的 API Key 才能启用。")
		cleanToken.RiskActivationBindCode = bindCode
		model.RecordLog(cleanToken.UserId, model.LogTypeSystem, "高风险用户新 API Key 已创建，请完成 QQ 绑定激活后再使用。")
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    buildMaskedTokenResponse(&cleanToken),
	})
}

func DeleteToken(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	userId := c.GetInt("id")
	err := model.DeleteTokenById(id, userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

func UpdateToken(c *gin.Context) {
	userId := c.GetInt("id")
	statusOnly := c.Query("status_only")
	token := model.Token{}
	err := c.ShouldBindJSON(&token)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if len(token.Name) > 50 {
		common.ApiErrorI18n(c, i18n.MsgTokenNameTooLong)
		return
	}
	if statusOnly == "" {
		if !token.UnlimitedQuota {
			if token.RemainQuota < 0 {
				common.ApiErrorI18n(c, i18n.MsgTokenQuotaNegative)
				return
			}
			maxQuotaValue := int((1000000000 * common.QuotaPerUnit))
			if token.RemainQuota > maxQuotaValue {
				common.ApiErrorI18n(c, i18n.MsgTokenQuotaExceedMax, map[string]any{"Max": maxQuotaValue})
				return
			}
		}
	}
	cleanToken, err := model.GetTokenByIds(token.Id, userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	groupMutation := planTokenGroupMutation(statusOnly, cleanToken.Group, token.Group, token.Status)
	if groupMutation.NeedsValidation {
		resolvedGroup, ok := ensureAccessControlForTokenMutation(c, groupMutation.RequestedGroup, groupMutation.Action)
		if !ok {
			return
		}
		groupMutation.RequestedGroup = resolvedGroup
		if strings.EqualFold(groupMutation.Action, "enable") {
			if cleanToken.Status == common.TokenStatusExpired && cleanToken.ExpiredTime <= common.GetTimestamp() && cleanToken.ExpiredTime != -1 {
				common.ApiErrorI18n(c, i18n.MsgTokenExpiredCannotEnable)
				return
			}
			if cleanToken.Status == common.TokenStatusExhausted && cleanToken.RemainQuota <= 0 && !cleanToken.UnlimitedQuota {
				common.ApiErrorI18n(c, i18n.MsgTokenExhaustedCannotEable)
				return
			}
			if err := service.EnsureRiskTokenCanBeEnabled(userId, cleanToken); err != nil {
				c.JSON(http.StatusOK, gin.H{
					"success": false,
					"message": err.Error(),
					"data":    buildMaskedTokenResponse(cleanToken),
				})
				return
			}
			// Persist the validated group even for status_only requests. Legacy
			// empty-group keys must never become enabled while still relying on
			// an implicit fallback at request time.
			cleanToken.Group = resolvedGroup
			_, _ = service.RestoreCommunityGateUserTokens(userId)
			if ctx, cancel := contextWithTimeout(c, 60*time.Second); cancel != nil {
				_, _, _ = service.RestoreAccessControlUserTokensIfCompliant(ctx, userId)
				cancel()
			}
		}
	}
	if statusOnly != "" {
		cleanToken.Status = token.Status
	} else {
		// If you add more fields, please also update token.Update()
		cleanToken.Name = token.Name
		if groupMutation.ApplyRequestedGroup {
			cleanToken.Group = groupMutation.RequestedGroup
		}
		cleanToken.ExpiredTime = token.ExpiredTime
		cleanToken.RemainQuota = token.RemainQuota
		cleanToken.UnlimitedQuota = token.UnlimitedQuota
		cleanToken.ModelLimitsEnabled = token.ModelLimitsEnabled
		cleanToken.ModelLimits = token.ModelLimits
		cleanToken.AllowIps = token.AllowIps
		cleanToken.CrossGroupRetry = token.CrossGroupRetry
	}
	err = cleanToken.Update()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    buildMaskedTokenResponse(cleanToken),
	})
}

type tokenGroupMutationPlan struct {
	RequestedGroup      string
	Action              string
	NeedsValidation     bool
	ApplyRequestedGroup bool
}

func planTokenGroupMutation(statusOnly string, currentGroup string, requestedGroup string, requestedStatus int) tokenGroupMutationPlan {
	currentGroup = strings.TrimSpace(currentGroup)
	requestedGroup = strings.TrimSpace(requestedGroup)
	plan := tokenGroupMutationPlan{
		RequestedGroup: requestedGroup,
	}
	if statusOnly != "" {
		if requestedStatus == common.TokenStatusEnabled {
			if requestedGroup == "" && currentGroup != "" {
				plan.RequestedGroup = currentGroup
			}
			plan.Action = "enable"
			// Enabling is always an entitlement transition. Legacy empty-group keys
			// must reach the shared validator and be rejected rather than bypass it.
			plan.NeedsValidation = true
		}
		return plan
	}
	if requestedGroup == "" || requestedGroup == currentGroup {
		return plan
	}
	plan.Action = "update"
	plan.NeedsValidation = true
	plan.ApplyRequestedGroup = true
	return plan
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

type TokenBatch struct {
	Ids []int `json:"ids"`
}

func DeleteTokenBatch(c *gin.Context) {
	tokenBatch := TokenBatch{}
	if err := c.ShouldBindJSON(&tokenBatch); err != nil || len(tokenBatch.Ids) == 0 {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	userId := c.GetInt("id")
	count, err := model.BatchDeleteTokens(tokenBatch.Ids, userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    count,
	})
}

func GetTokenKeysBatch(c *gin.Context) {
	tokenBatch := TokenBatch{}
	if err := c.ShouldBindJSON(&tokenBatch); err != nil || len(tokenBatch.Ids) == 0 {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	if len(tokenBatch.Ids) > 100 {
		common.ApiErrorI18n(c, i18n.MsgBatchTooMany, map[string]any{"Max": 100})
		return
	}
	userId := c.GetInt("id")
	tokens, err := model.GetTokenKeysByIds(tokenBatch.Ids, userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	keysMap := make(map[int]string)
	for _, t := range tokens {
		keysMap[t.Id] = t.GetFullKey()
	}
	common.ApiSuccess(c, gin.H{"keys": keysMap})
}
