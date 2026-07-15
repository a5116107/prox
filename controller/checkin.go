package controller

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
)

type CheckinTarget struct {
	Kind        string `json:"kind,omitempty"`
	Mode        string `json:"mode"`
	URL         string `json:"url"`
	Label       string `json:"label"`
	Description string `json:"description"`
}

// GetCheckinStatus 获取用户签到状态和历史记录
func GetCheckinStatus(c *gin.Context) {
	setting := operation_setting.GetCheckinSetting()
	if !setting.Enabled {
		common.ApiErrorMsg(c, "签到功能未启用")
		return
	}
	userId := c.GetInt("id")

	if allowed, deniedReason := service.CanCurrentUserReceiveCheckinBenefitReason(userId); !allowed {
		model.RecordLogEvent(userId, model.LogTypeSystem, "web checkin denied: membership expired", model.LogEventOptions{
			Category:   "reward",
			Source:     "web",
			Action:     "checkin",
			Status:     "denied",
			SiteId:     detectCheckinSiteID(c),
			BudgetPool: "activity",
			Other: map[string]interface{}{
				"reason": deniedReason,
			},
		})
		common.ApiErrorMsg(c, webCheckinDeniedMessage(deniedReason))
		return
	}
	// 获取月份参数，默认为当前月份
	month := c.DefaultQuery("month", time.Now().Format("2006-01"))

	stats, err := model.GetUserCheckinStats(userId, month)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"enabled":   setting.Enabled,
			"min_quota": setting.MinQuota,
			"max_quota": setting.MaxQuota,
			"jump_mode": setting.EffectiveJumpMode(),
			"target":    buildCheckinTarget(setting, c),
			"targets":   buildCheckinTargets(setting, c),
			"stats":     stats,
		},
	})
}

// DoCheckin 执行用户签到
func DoCheckin(c *gin.Context) {
	setting := operation_setting.GetCheckinSetting()
	if !setting.Enabled {
		common.ApiErrorMsg(c, "签到功能未启用")
		return
	}

	if setting.EffectiveJumpMode() != "api" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "请通过社区或主群入口完成签到",
			"data": gin.H{
				"target":  buildCheckinTarget(setting, c),
				"targets": buildCheckinTargets(setting, c),
			},
		})
		return
	}

	userId := c.GetInt("id")

	if allowed, deniedReason := service.CanCurrentUserReceiveCheckinBenefitReason(userId); !allowed {
		model.RecordLogEvent(userId, model.LogTypeSystem, "web checkin denied: membership expired", model.LogEventOptions{
			Category:   "reward",
			Source:     "web",
			Action:     "checkin",
			Status:     "denied",
			SiteId:     detectCheckinSiteID(c),
			Group:      model.CheckinChannelWeb,
			BudgetPool: "activity",
			Other: map[string]interface{}{
				"checkin": map[string]interface{}{
					"channel": model.CheckinChannelWeb,
					"reason":  deniedReason,
				},
				"reason": deniedReason,
			},
		})
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": webCheckinDeniedMessage(deniedReason),
		})
		return
	}

	checkin, err := model.UserCheckin(userId)
	if err != nil {
		model.RecordLogEvent(userId, model.LogTypeSystem, "web checkin failed", model.LogEventOptions{
			Category:   "reward",
			Source:     "web",
			Action:     "checkin",
			Status:     "failed",
			SiteId:     detectCheckinSiteID(c),
			Group:      model.CheckinChannelWeb,
			BudgetPool: "activity",
			Other: map[string]interface{}{
				"checkin": map[string]interface{}{
					"channel": model.CheckinChannelWeb,
					"reason":  err.Error(),
				},
			},
		})
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	model.RecordLogEvent(userId, model.LogTypeSystem, fmt.Sprintf("用户签到，获得额度 %s", logger.LogQuota(checkin.QuotaAwarded)), model.LogEventOptions{
		Quota:      checkin.QuotaAwarded,
		Category:   "reward",
		Source:     "web",
		Action:     "checkin",
		Status:     "success",
		SiteId:     detectCheckinSiteID(c),
		Group:      model.CheckinChannelWeb,
		BudgetPool: "activity",
		RewardType: "checkin_web",
		Other: map[string]interface{}{
			"checkin": map[string]interface{}{
				"channel":       model.CheckinChannelWeb,
				"quota_awarded": checkin.QuotaAwarded,
				"checkin_date":  checkin.CheckinDate,
			},
		},
	})
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "签到成功",
		"data": gin.H{
			"quota_awarded": checkin.QuotaAwarded,
			"checkin_date":  checkin.CheckinDate},
	})
}

func webCheckinDeniedMessage(reason string) string {
	switch strings.TrimSpace(reason) {
	case "missing_oauth_binding", "membership_missing":
		return "请先在社区群重新验牌后，再领取社区签到奖励。"
	case "not_in_any_required_room", "missing_required_rooms":
		return "你当前未满足社区群资格，请确认仍在社区群内，并在社区群重新验牌后，再领取社区签到奖励。"
	case model.ChatMembershipStatusGrace, "grace_expired", model.ChatMembershipStatusLeftExpired, model.ChatMembershipStatusSuspectedLeft:
		return "你的群成员资格已失效，请重新加入对应社区群，并在社区群重新验牌后，再领取社区签到奖励。"
	case "room_not_configured", "membership_check_failed":
		return "社区签到资格校验暂时异常，请稍后重试或联系管理员。"
	default:
		return "请先在社区群重新验牌后，再领取社区签到奖励。"
	}
}

func buildCheckinTarget(setting *operation_setting.CheckinSetting, c *gin.Context) CheckinTarget {
	targets := buildCheckinTargets(setting, c)
	if len(targets) > 0 {
		return targets[0]
	}
	return CheckinTarget{
		Mode:        "direct",
		Label:       "签到入口未配置",
		Description: "请联系管理员配置社区或群聊签到入口",
	}
}

func buildCheckinTargets(setting *operation_setting.CheckinSetting, c *gin.Context) []CheckinTarget {
	siteID := detectCheckinSiteID(c)
	communityURL, primaryURL, primaryKind := resolveCheckinEntryURLs(setting, siteID)
	targets := make([]CheckinTarget, 0, 2)

	appendTarget := func(target CheckinTarget) {
		target.URL = strings.TrimSpace(target.URL)
		if target.URL == "" {
			return
		}
		for _, existing := range targets {
			if existing.URL == target.URL {
				return
			}
		}
		targets = append(targets, target)
	}

	appendTarget(CheckinTarget{
		Kind:        "community",
		Mode:        "direct",
		URL:         communityURL,
		Label:       "社区签到",
		Description: "前往社区群完成签到",
	})

	if primaryURL != "" {
		label := "群签到"
		description := "前往主群完成签到"
		switch primaryKind {
		case "qq":
			label = "QQ签到"
			description = "前往 QQ 群完成签到"
		case "tg":
			label = "TG签到"
			description = "前往 TG 群完成签到"
		}
		appendTarget(CheckinTarget{
			Kind:        primaryKind,
			Mode:        "direct",
			URL:         primaryURL,
			Label:       label,
			Description: description,
		})
	}

	return targets
}

func resolveCheckinEntryURLs(setting *operation_setting.CheckinSetting, siteID string) (communityURL string, primaryURL string, primaryKind string) {
	accessCfg := operation_setting.GetAccessControlSetting()
	if setting != nil {
		communityURL = strings.TrimSpace(setting.CommunityCheckinURL)
		primaryURL = strings.TrimSpace(setting.QQCheckinURL)
	}
	switch siteID {
	case "prox":
		primaryKind = "qq"
		communityURL = firstControllerNonEmpty(communityURL, strings.TrimSpace(accessCfg.CommunityJoinURL), agentChatOpsCommunityLink("community"))
		primaryURL = firstControllerNonEmpty(primaryURL, strings.TrimSpace(accessCfg.PrimaryJoinURL), agentChatOpsCommunityLink("qq"))
	default:
		primaryKind = "community"
		communityURL = firstControllerNonEmpty(communityURL, strings.TrimSpace(accessCfg.CommunityJoinURL), agentChatOpsCommunityLink("community"))
		if primaryURL == "" {
			primaryURL = communityURL
		}
	}
	return communityURL, primaryURL, primaryKind
}

func detectCheckinSiteID(c *gin.Context) string {
	if siteID := model.CanonicalSiteID(""); siteID != "default" {
		return siteID
	}
	if serverAddr := strings.TrimSpace(systemServerAddress()); serverAddr != "" {
		if siteID := model.CanonicalSiteID(serverAddr); siteID != "default" {
			return siteID
		}
	}
	if host := strings.TrimSpace(c.Request.Host); host != "" {
		return model.CanonicalSiteID("https://" + host)
	}
	return "default"
}

func systemServerAddress() string {
	return strings.TrimSpace(system_setting.ServerAddress)
}
