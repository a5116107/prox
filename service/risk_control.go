package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/bytedance/gopkg/util/gopool"
	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func normalizeRiskActivationSource(source string) string {
	source = strings.ToLower(strings.TrimSpace(source))
	switch source {
	case "telegram":
		return "tg"
	case "":
		return "qq"
	default:
		return source
	}
}

func activeRiskControlSetting() *operation_setting.RiskControlSetting {
	cfg := operation_setting.GetRiskControlSetting()
	if cfg == nil {
		return nil
	}
	return cfg
}

func riskActivationTTLMinutes() int {
	cfg := activeRiskControlSetting()
	if cfg == nil || cfg.ActivationCodeTTLMinutes <= 0 {
		return 10
	}
	if cfg.ActivationCodeTTLMinutes > 120 {
		return 120
	}
	return cfg.ActivationCodeTTLMinutes
}

func riskClampInt(value int, fallback int, min int, max int) int {
	if value <= 0 {
		value = fallback
	}
	if min > 0 && value < min {
		value = min
	}
	if max > 0 && value > max {
		value = max
	}
	return value
}

func riskTrackingNeeded(cfg *operation_setting.RiskControlSetting) bool {
	if cfg == nil {
		return false
	}
	return cfg.RequestIPTrackingEnabled ||
		cfg.SameIPMultiAccountUsageEnabled ||
		cfg.DynamicIPChurnEnabled ||
		cfg.InactiveTokenDisableEnabled
}

func riskSameIPWindowMinutes(cfg *operation_setting.RiskControlSetting) int {
	if cfg == nil {
		return 60
	}
	return riskClampInt(cfg.SameIPMultiAccountUsageWindowMinutes, 60, 1, 24*60)
}

func riskSameIPUserLimit(cfg *operation_setting.RiskControlSetting) int {
	if cfg == nil {
		return 1
	}
	return riskClampInt(cfg.SameIPMultiAccountUsageUserLimit, 1, 1, 50)
}

func riskDynamicIPWindowMinutes(cfg *operation_setting.RiskControlSetting) int {
	if cfg == nil {
		return 30
	}
	return riskClampInt(cfg.DynamicIPChurnWindowMinutes, 30, 1, 24*60)
}

func riskDynamicIPDistinctLimit(cfg *operation_setting.RiskControlSetting) int {
	if cfg == nil {
		return 6
	}
	return riskClampInt(cfg.DynamicIPChurnDistinctIPLimit, 6, 2, 100)
}

func riskBurstRegisterWindowMinutes(cfg *operation_setting.RiskControlSetting) int {
	if cfg == nil {
		return 10
	}
	return riskClampInt(cfg.BurstRegisterWindowMinutes, 10, 1, 24*60)
}

func riskBurstRegisterLimit(cfg *operation_setting.RiskControlSetting) int {
	if cfg == nil {
		return 3
	}
	return riskClampInt(cfg.BurstRegisterLimit, 3, 2, 100)
}

func riskInactiveTokenDisableDays(cfg *operation_setting.RiskControlSetting) int {
	if cfg == nil {
		return 7
	}
	return riskClampInt(cfg.InactiveTokenDisableDays, 7, 1, 365)
}

func riskTokenIPRedisKey(siteId string, tokenId int) string {
	return fmt.Sprintf("risk:token:%s:%d:ips", strings.TrimSpace(siteId), tokenId)
}

func riskIPUserRedisKey(siteId string, clientIP string) string {
	return fmt.Sprintf("risk:ip:%s:%s:users", strings.TrimSpace(siteId), strings.TrimSpace(clientIP))
}

func PrepareRiskTokenActivationForUser(userId int) (*model.RiskUserControl, error) {
	cfg := activeRiskControlSetting()
	if cfg == nil || !cfg.Enabled || !cfg.HighRiskKeyRecreateRequired || !cfg.HighRiskActivationRequired || userId <= 0 {
		return nil, nil
	}
	control, err := model.GetRiskUserControl(AgentSiteID(), userId)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	if !control.Enabled || !control.KeyRecreateRequired || !control.ActivationRequired {
		return nil, nil
	}
	control.ActivationSource = normalizeRiskActivationSource(firstAgentNonEmpty(control.ActivationSource, cfg.HighRiskActivationSource))
	return control, nil
}

func GetRiskTokenActivationForUserToken(userId int, tokenId int) (*model.RiskTokenActivation, error) {
	if userId <= 0 || tokenId <= 0 {
		return nil, gorm.ErrRecordNotFound
	}
	row, err := model.GetLatestRiskTokenActivation(AgentSiteID(), userId, tokenId)
	if err != nil {
		return nil, err
	}
	if row.Status == model.RiskTokenActivationPending && row.ExpiresAt > 0 && row.ExpiresAt <= common.GetTimestamp() {
		row.Status = model.RiskTokenActivationExpired
	}
	return row, nil
}

func CreateRiskTokenActivationForToken(tx *gorm.DB, userId int, token *model.Token, control *model.RiskUserControl) (string, int64, error) {
	if tx == nil || token == nil || token.Id <= 0 || userId <= 0 || control == nil {
		return "", 0, errors.New("invalid risk token activation request")
	}
	code, codeHash, expiresAt, err := PrepareAgentChatOpsBindCodeWithTTL(userId, riskActivationTTLMinutes())
	if err != nil {
		return "", 0, err
	}
	if err := model.CreateAgentChatBindCodeWithTx(tx, AgentSiteID(), userId, codeHash, expiresAt); err != nil {
		return "", 0, err
	}
	reason := strings.TrimSpace(control.Reason)
	if reason == "" {
		reason = "高风险账号需完成 QQ 绑定激活后，新的 API Key 才能启用。"
	}
	if err := model.CreateRiskTokenActivationWithTx(tx, &model.RiskTokenActivation{
		SiteId:           AgentSiteID(),
		UserId:           userId,
		TokenId:          token.Id,
		ActivationSource: normalizeRiskActivationSource(control.ActivationSource),
		CodeHash:         codeHash,
		Status:           model.RiskTokenActivationPending,
		ReasonCode:       firstAgentNonEmpty(control.ReasonCode, "high_risk_activation_required"),
		Reason:           reason,
		ExpiresAt:        expiresAt,
	}); err != nil {
		return "", 0, err
	}
	return code, expiresAt, nil
}

func ReissueRiskTokenActivationCode(userId int, token *model.Token) (string, int64, error) {
	if token == nil || token.Id <= 0 || userId <= 0 {
		return "", 0, errors.New("invalid token")
	}
	control, err := PrepareRiskTokenActivationForUser(userId)
	if err != nil {
		return "", 0, err
	}
	if control == nil {
		return "", 0, errors.New("当前 Key 无需重新生成激活码")
	}
	code, codeHash, expiresAt, err := PrepareAgentChatOpsBindCodeWithTTL(userId, riskActivationTTLMinutes())
	if err != nil {
		return "", 0, err
	}
	reason := strings.TrimSpace(control.Reason)
	if reason == "" {
		reason = "高风险账号需完成 QQ 绑定激活后，新的 API Key 才能启用。"
	}
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		if err := model.CreateAgentChatBindCodeWithTx(tx, AgentSiteID(), userId, codeHash, expiresAt); err != nil {
			return err
		}
		if err := tx.Model(&model.Token{}).Where("id = ? AND user_id = ?", token.Id, userId).Update("status", common.TokenStatusDisabled).Error; err != nil {
			return err
		}
		return model.CreateRiskTokenActivationWithTx(tx, &model.RiskTokenActivation{
			SiteId:           AgentSiteID(),
			UserId:           userId,
			TokenId:          token.Id,
			ActivationSource: normalizeRiskActivationSource(control.ActivationSource),
			CodeHash:         codeHash,
			Status:           model.RiskTokenActivationPending,
			ReasonCode:       firstAgentNonEmpty(control.ReasonCode, "high_risk_activation_required"),
			Reason:           reason,
			ExpiresAt:        expiresAt,
		})
	})
	if err != nil {
		return "", 0, err
	}
	token.Status = common.TokenStatusDisabled
	if common.RedisEnabled {
		_ = model.InvalidateUserTokensCache(userId)
	}
	return code, expiresAt, nil
}

func EnsureRiskTokenCanBeEnabled(userId int, token *model.Token) error {
	cfg := activeRiskControlSetting()
	if cfg == nil || !cfg.Enabled || token == nil || token.Id <= 0 || userId <= 0 {
		return nil
	}
	row, err := GetRiskTokenActivationForUserToken(userId, token.Id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	if row == nil || row.Status == model.RiskTokenActivationActivated {
		return nil
	}
	source := normalizeRiskActivationSource(firstAgentNonEmpty(row.ActivationSource, cfg.HighRiskActivationSource))
	return fmt.Errorf("该 Key 需先完成 %s 绑定激活", strings.ToUpper(source))
}

func ActivateRiskTokensOnBind(req AgentChatOpsBindConfirmRequest, userId int, code string) (int, error) {
	if userId <= 0 || strings.TrimSpace(code) == "" {
		return 0, nil
	}
	cfg := activeRiskControlSetting()
	if cfg == nil || !cfg.Enabled {
		return 0, nil
	}
	source := normalizeRiskActivationSource(req.Source)
	now := common.GetTimestamp()
	rows, err := model.GetPendingRiskTokenActivationsByCode(
		AgentSiteID(),
		userId,
		source,
		hashAgentBindCode(AgentSiteID(), code),
		now,
	)
	if err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, nil
	}
	ids := make([]int, 0, len(rows))
	tokenIDs := make([]int, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.Id)
		tokenIDs = append(tokenIDs, row.TokenId)
	}
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.Token{}).
			Where("user_id = ? AND id IN ?", userId, tokenIDs).
			Update("status", common.TokenStatusEnabled).Error; err != nil {
			return err
		}
		return tx.Model(&model.RiskTokenActivation{}).Where("id IN ?", ids).Updates(map[string]interface{}{
			"status":           model.RiskTokenActivationActivated,
			"activated_at":     now,
			"external_user_id": strings.TrimSpace(req.UserExternalID),
			"updated_at":       now,
		}).Error
	})
	if err != nil {
		return 0, err
	}
	if common.RedisEnabled {
		_ = model.InvalidateUserTokensCache(userId)
	}
	model.RecordLogEvent(userId, model.LogTypeSystem, fmt.Sprintf("已完成 %s 绑定激活，恢复 %d 个高风险 API Key", strings.ToUpper(source), len(tokenIDs)), model.LogEventOptions{
		Category:   "risk",
		Source:     source,
		Action:     "risk_token_activate",
		Status:     "success",
		SiteId:     AgentSiteID(),
		RiskLevel:  "high",
		RewardType: "",
		Other: map[string]interface{}{
			"risk_activation": map[string]interface{}{
				"activated_count":  len(tokenIDs),
				"token_ids":        tokenIDs,
				"external_user_id": strings.TrimSpace(req.UserExternalID),
			},
		},
	})
	return len(tokenIDs), nil
}

func EnforceOAuthRegisterIPLimit(providerSlug string, clientIP string) error {
	cfg := activeRiskControlSetting()
	if cfg == nil || !cfg.Enabled {
		return nil
	}
	if !cfg.SameIPSameDayOAuthRegisterEnabled && !cfg.BurstRegisterEnabled {
		return nil
	}
	clientIP = strings.TrimSpace(clientIP)
	if clientIP == "" {
		return nil
	}
	if cfg.SameIPSameDayOAuthRegisterEnabled {
		limit := cfg.SameIPSameDayOAuthRegisterLimit
		if limit <= 0 {
			limit = 1
		}
		count, err := model.CountOAuthRegisterAttempts(AgentSiteID(), clientIP, model.AgentBusinessDateAt(time.Now()))
		if err != nil {
			return err
		}
		if count >= int64(limit) {
			msg := strings.TrimSpace(cfg.SameIPRegisterBlockMessage)
			if msg == "" {
				msg = "检测到同一 IP 当天已注册过账号，请更换 IP 后再试。"
			}
			return errors.New(msg)
		}
	}
	if cfg.BurstRegisterEnabled {
		since := time.Now().Add(-time.Duration(riskBurstRegisterWindowMinutes(cfg)) * time.Minute).Unix()
		count, err := model.CountOAuthRegisterAttemptsSince(AgentSiteID(), clientIP, since)
		if err != nil {
			return err
		}
		if count >= int64(riskBurstRegisterLimit(cfg)) {
			msg := strings.TrimSpace(cfg.BurstRegisterBlockMessage)
			if msg == "" {
				msg = "检测到该 IP 在短时间内注册过多账号，请稍后更换 IP 后再试。"
			}
			return errors.New(msg)
		}
	}
	_ = providerSlug
	return nil
}

func RecordOAuthRegisterAttempt(providerSlug string, userId int, clientIP string) error {
	clientIP = strings.TrimSpace(clientIP)
	if clientIP == "" || userId <= 0 {
		return nil
	}
	return model.CreateOAuthRegisterAttempt(&model.OAuthRegisterAttempt{
		SiteId:       AgentSiteID(),
		ProviderSlug: strings.TrimSpace(providerSlug),
		UserId:       userId,
		ClientIP:     clientIP,
		RegisterDate: model.AgentBusinessDateAt(time.Now()),
	})
}

func GetRiskControlSettingTTLSeconds() int {
	return riskActivationTTLMinutes() * 60
}

func opsRiskAuditEnforcementActive(status string) (bool, error) {
	switch status {
	case OpsRiskStatusOpen, OpsRiskStatusReviewing:
		return true, nil
	case OpsRiskStatusIgnored, OpsRiskStatusClosed:
		return false, nil
	default:
		return false, fmt.Errorf("invalid current risk audit status: %s", status)
	}
}

func ApplyRiskControlToUser(userId int, reasonCode string, reason string) (*model.RiskUserControl, int64, error) {
	cfg := activeRiskControlSetting()
	if cfg == nil || !cfg.Enabled || userId <= 0 {
		return nil, 0, nil
	}
	now := common.GetTimestamp()
	reasonCode = firstAgentNonEmpty(strings.TrimSpace(reasonCode), "risk_control_triggered")
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "账号触发风控，需重新创建 Key 并完成账号校验后再使用。"
	}
	control := &model.RiskUserControl{
		SiteId:              AgentSiteID(),
		UserId:              userId,
		RiskLevel:           "high",
		ReasonCode:          reasonCode,
		Reason:              reason,
		Enabled:             true,
		KeyRecreateRequired: cfg.HighRiskKeyRecreateRequired,
		ActivationRequired:  cfg.HighRiskActivationRequired,
		ActivationSource:    normalizeRiskActivationSource(cfg.HighRiskActivationSource),
	}
	var disabledCount int64
	excludedAdmin := false
	manualDecision := false
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		var user model.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id", "role").Where("id = ?", userId).First(&user).Error; err != nil {
			return err
		}
		if user.Role >= common.RoleAdminUser {
			excludedAdmin = true
			return nil
		}

		tokenIDs := make([]int, 0)
		if cfg.HighRiskKeyRecreateRequired {
			var enabledTokens []model.Token
			if err := tx.Where("user_id = ? AND status = ?", userId, common.TokenStatusEnabled).Order("id asc").Find(&enabledTokens).Error; err != nil {
				return err
			}
			for _, token := range enabledTokens {
				tokenIDs = append(tokenIDs, token.Id)
			}
		}

		audit := model.OpsAccountRiskAudit{
			SiteId: AgentSiteID(), RiskType: reasonCode, Severity: "high", Subject: fmt.Sprintf("user:%d", userId),
			UserIds: encodeOpsRiskIntList([]int{userId}), TokenIds: encodeOpsRiskIntList(tokenIDs),
			Evidence: mustAgentJSON(map[string]any{
				"source": "runtime_risk_control", "reason_code": reasonCode, "reason": reason, "observed_at": now,
			}),
			Status: OpsRiskStatusOpen,
		}
		if err := upsertOpsAccountRiskAuditWithTx(tx, &audit); err != nil {
			return err
		}
		enforcementActive, err := opsRiskAuditEnforcementActive(audit.Status)
		if err != nil {
			return err
		}
		if !enforcementActive {
			manualDecision = true
			return nil
		}
		if cfg.HighRiskKeyRecreateRequired {
			bounds := parseOpsRiskIntList(audit.TokenIds)
			if len(bounds) > 0 {
				candidates, err := listOpsRiskDisableCandidates(tx, &audit, bounds)
				if err != nil {
					return err
				}
				disabledCount, _, err = disableOpsRiskTokensWithTx(tx, &audit, candidates, 0, reason)
				if err != nil {
					return err
				}
				if disabledCount > 0 {
					control.ExistingKeysDisabledAt = now
					if err := setOpsRiskAuditStatusWithTx(tx, &audit, OpsRiskStatusReviewing, now); err != nil {
						return err
					}
				}
			}
		}
		return model.UpsertRiskUserControlWithTx(tx, control)
	})
	if err != nil {
		return nil, 0, err
	}
	if excludedAdmin || manualDecision {
		return nil, 0, nil
	}
	if common.RedisEnabled {
		_ = model.InvalidateUserTokensCache(userId)
		_ = model.InvalidateUserCache(userId)
	}
	return control, disabledCount, nil
}

func DescribeDisabledRiskToken(userId int, tokenId int) string {
	if userId <= 0 || tokenId <= 0 {
		return ""
	}
	if row, err := GetRiskTokenActivationForUserToken(userId, tokenId); err == nil && row != nil {
		if row.Status == model.RiskTokenActivationPending || row.Status == model.RiskTokenActivationExpired {
			return firstAgentNonEmpty(strings.TrimSpace(row.Reason), "该 Key 需先完成账号绑定激活后再使用。")
		}
	}
	if control, err := model.GetRiskUserControl(AgentSiteID(), userId); err == nil && control != nil && control.Enabled {
		return firstAgentNonEmpty(strings.TrimSpace(control.Reason), "账号已触发风控，请重新创建 Key 并完成账号校验后再使用。")
	}
	return ""
}

func TrackAndEnforceTokenRequestRisk(ctx context.Context, token *model.Token, clientIP string) error {
	cfg := activeRiskControlSetting()
	if cfg == nil || !cfg.Enabled || token == nil || token.Id <= 0 || token.UserId <= 0 {
		return nil
	}
	clientIP = strings.TrimSpace(clientIP)
	if clientIP == "" || !riskTrackingNeeded(cfg) {
		return nil
	}
	now := time.Now().Unix()
	tokenIPCount, sameIPUserCount, redisTracked, err := trackRiskUsageInRedis(ctx, cfg, token, clientIP, now)
	if err != nil {
		common.SysLog("[RiskControl] redis tracking failed: " + err.Error())
	}
	persistFingerprint := cfg.RequestIPTrackingEnabled || cfg.InactiveTokenDisableEnabled || cfg.DynamicIPChurnEnabled || cfg.SameIPMultiAccountUsageEnabled
	if persistFingerprint {
		// The current request must participate in the enforcement decision. With
		// Redis unavailable, doing this asynchronously creates an off-by-one
		// window where the threshold request is incorrectly allowed.
		if !redisTracked && (cfg.DynamicIPChurnEnabled || cfg.SameIPMultiAccountUsageEnabled) {
			if upsertErr := model.UpsertRiskRequestFingerprint(AgentSiteID(), token.UserId, token.Id, clientIP, now); upsertErr != nil {
				common.SysLog("[RiskControl] fingerprint upsert failed: " + upsertErr.Error())
			}
		} else {
			gopool.Go(func() {
				if upsertErr := model.UpsertRiskRequestFingerprint(AgentSiteID(), token.UserId, token.Id, clientIP, now); upsertErr != nil {
					common.SysLog("[RiskControl] fingerprint upsert failed: " + upsertErr.Error())
				}
			})
		}
	}
	if cfg.SameIPMultiAccountUsageEnabled {
		if sameIPUserCount == 0 && !redisTracked {
			dbCount, dbErr := model.CountDistinctRiskUsersByIPSince(AgentSiteID(), clientIP, now-int64(riskSameIPWindowMinutes(cfg))*60, token.UserId)
			if dbErr == nil {
				sameIPUserCount = dbCount
			}
		}
		if sameIPUserCount >= int64(riskSameIPUserLimit(cfg)) {
			reason := strings.TrimSpace(cfg.SameIPMultiAccountUsageBlockMessage)
			if reason == "" {
				reason = "检测到同一 IP 在短时间内切换多个账号访问，当前 Key 已触发风控，请更换独立网络后重试。"
			}
			_, _, _ = ApplyRiskControlToUser(token.UserId, "same_ip_multi_account_usage", reason)
			return errors.New(reason)
		}
	}
	if cfg.DynamicIPChurnEnabled {
		if tokenIPCount == 0 && !redisTracked {
			dbCount, dbErr := model.CountDistinctRiskIPsByTokenSince(AgentSiteID(), token.Id, now-int64(riskDynamicIPWindowMinutes(cfg))*60)
			if dbErr == nil {
				tokenIPCount = dbCount
			}
		}
		if tokenIPCount >= int64(riskDynamicIPDistinctLimit(cfg)) {
			reason := strings.TrimSpace(cfg.DynamicIPChurnBlockMessage)
			if reason == "" {
				reason = "检测到当前 Key 在短时间内频繁切换 IP，已触发风控，请完成账号校验后再试。"
			}
			_, _, _ = ApplyRiskControlToUser(token.UserId, "dynamic_ip_churn", reason)
			return errors.New(reason)
		}
	}
	return nil
}

func trackRiskUsageInRedis(ctx context.Context, cfg *operation_setting.RiskControlSetting, token *model.Token, clientIP string, now int64) (tokenIPCount int64, sameIPUserCount int64, tracked bool, err error) {
	if !common.RedisEnabled || common.RDB == nil || token == nil {
		return 0, 0, false, nil
	}
	siteId := AgentSiteID()
	pipe := common.RDB.TxPipeline()

	tokenKey := riskTokenIPRedisKey(siteId, token.Id)
	tokenWindowSeconds := int64(riskDynamicIPWindowMinutes(cfg) * 60)
	if cfg.DynamicIPChurnEnabled || cfg.RequestIPTrackingEnabled || cfg.InactiveTokenDisableEnabled {
		pipe.ZAdd(ctx, tokenKey, &redis.Z{Score: float64(now), Member: clientIP})
		pipe.ZRemRangeByScore(ctx, tokenKey, "-inf", fmt.Sprintf("%d", now-tokenWindowSeconds))
		tokenCard := pipe.ZCard(ctx, tokenKey)
		pipe.Expire(ctx, tokenKey, time.Duration(tokenWindowSeconds+3600)*time.Second)
		defer func() {
			if tokenCard != nil {
				tokenIPCount = tokenCard.Val()
			}
		}()
	}

	var userCard *redis.IntCmd
	if cfg.SameIPMultiAccountUsageEnabled {
		ipKey := riskIPUserRedisKey(siteId, clientIP)
		ipWindowSeconds := int64(riskSameIPWindowMinutes(cfg) * 60)
		pipe.ZAdd(ctx, ipKey, &redis.Z{Score: float64(now), Member: fmt.Sprintf("%d", token.UserId)})
		pipe.ZRemRangeByScore(ctx, ipKey, "-inf", fmt.Sprintf("%d", now-ipWindowSeconds))
		userCard = pipe.ZCard(ctx, ipKey)
		pipe.Expire(ctx, ipKey, time.Duration(ipWindowSeconds+3600)*time.Second)
		defer func() {
			if userCard != nil {
				sameIPUserCount = userCard.Val() - 1
				if sameIPUserCount < 0 {
					sameIPUserCount = 0
				}
			}
		}()
	}

	if _, err = pipe.Exec(ctx); err != nil {
		return 0, 0, false, err
	}
	return tokenIPCount, sameIPUserCount, true, nil
}

func RunRiskControlMaintenanceOnce(ctx context.Context) {
	cfg := activeRiskControlSetting()
	if cfg == nil || !cfg.Enabled {
		return
	}
	if cfg.InactiveTokenDisableEnabled {
		offset := 0
		limit := 200
		cutoff := time.Now().Add(-time.Duration(riskInactiveTokenDisableDays(cfg)) * 24 * time.Hour).Unix()
		reason := strings.TrimSpace(cfg.InactiveTokenDisableReason)
		if reason == "" {
			reason = "长时间未活跃的账号需重新创建 Key 并完成校验后再使用。"
		}
		for {
			users, err := model.ListRiskMaintenanceUsers(limit, offset)
			if err != nil {
				common.SysLog("[RiskControl] maintenance list users failed: " + err.Error())
				break
			}
			if len(users) == 0 {
				break
			}
			for _, user := range users {
				latestTokenSeen, err := model.GetLatestEnabledTokenAccessedAt(user.Id)
				if err != nil {
					continue
				}
				latestRiskSeen, err := model.GetLatestRiskRequestSeenAt(user.Id)
				if err != nil {
					continue
				}
				latestActivity := user.CreatedAt
				if user.LastLoginAt > latestActivity {
					latestActivity = user.LastLoginAt
				}
				if latestTokenSeen > latestActivity {
					latestActivity = latestTokenSeen
				}
				if latestRiskSeen > latestActivity {
					latestActivity = latestRiskSeen
				}
				if latestActivity > 0 && latestActivity < cutoff {
					_, _, _ = ApplyRiskControlToUser(user.Id, "inactive_token_disable", reason)
				}
			}
			if len(users) < limit {
				break
			}
			offset += limit
		}
	}
	_ = model.DeleteOldRiskRequestFingerprints(time.Now().Add(-45 * 24 * time.Hour).Unix())
}
