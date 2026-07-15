package controller

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

func GetAdminUserOpsGrid(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	filters := service.AdminUserOpsGridFilters{
		Keyword:          c.Query("keyword"),
		Group:            c.Query("group"),
		AccessLevel:      c.Query("access_level"),
		CommunityBound:   parseOptionalBool(c.Query("community_bound")),
		HasCommunityRoom: parseOptionalBool(c.Query("has_community_room")),
		QQBound:          parseOptionalBool(c.Query("qq_bound")),
		TGBound:          parseOptionalBool(c.Query("tg_bound")),
		PrimaryBound:     parseOptionalBool(c.Query("primary_bound")),
		HasFrozenKeys:    parseOptionalBool(c.Query("has_frozen_keys")),
		OverrideMode:     c.Query("override_mode"),
		Page:             pageInfo.GetPage(),
		PageSize:         pageInfo.GetPageSize(),
	}
	if roleStr := strings.TrimSpace(c.Query("role")); roleStr != "" {
		if parsed, err := strconv.Atoi(roleStr); err == nil {
			filters.Role = &parsed
		}
	}
	if statusStr := strings.TrimSpace(c.Query("status")); statusStr != "" {
		if parsed, err := strconv.Atoi(statusStr); err == nil {
			filters.Status = &parsed
		}
	}
	ctx, cancel := contextWithTimeout(c, 90*time.Second)
	defer cancel()
	items, total, err := service.ListAdminUserOpsGrid(ctx, c.Query("site_id"), filters)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetItems(items)
	pageInfo.SetTotal(int(total))
	common.ApiSuccess(c, pageInfo)
}

func GetAdminUserOpsProfile(c *gin.Context) {
	userID, ok := parseOpsUserID(c)
	if !ok {
		return
	}
	refresh := c.Query("refresh") == "1" || strings.EqualFold(c.Query("refresh"), "true")
	ctx, cancel := contextWithTimeout(c, 90*time.Second)
	defer cancel()
	profile, err := service.GetAdminUserOpsProfile(ctx, c.Query("site_id"), userID, refresh)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, profile)
}

func GetAdminUserBindings(c *gin.Context) {
	userID, ok := parseOpsUserID(c)
	if !ok {
		return
	}
	data, err := service.GetAdminUserBindings(c.Query("site_id"), userID)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, data)
}

func RecomputeAdminUserMembership(c *gin.Context) {
	userID, ok := parseOpsUserID(c)
	if !ok {
		return
	}
	ctx, cancel := contextWithTimeout(c, 90*time.Second)
	defer cancel()
	data, err := service.RecomputeAdminUserMembership(ctx, c.Query("site_id"), userID)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAuditFor(c, userID, "user.ops_recompute_membership", map[string]any{"site_id": c.Query("site_id")})
	common.ApiSuccess(c, data)
}

func UpdateAdminUserAccessOverride(c *gin.Context) {
	userID, ok := parseOpsUserID(c)
	if !ok {
		return
	}
	var req struct {
		Mode   string   `json:"mode"`
		Groups []string `json:"groups"`
		Reason string   `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	ctx, cancel := contextWithTimeout(c, 90*time.Second)
	defer cancel()
	data, err := service.UpdateAdminUserAccessOverride(ctx, c.Query("site_id"), userID, req.Mode, req.Groups, req.Reason)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAuditFor(c, userID, "user.ops_access_override", map[string]any{
		"mode":   req.Mode,
		"groups": req.Groups,
		"reason": req.Reason,
	})
	common.ApiSuccess(c, data)
}

func RestoreAdminUserKeys(c *gin.Context) {
	userID, ok := parseOpsUserID(c)
	if !ok {
		return
	}
	ctx, cancel := contextWithTimeout(c, 90*time.Second)
	defer cancel()
	data, err := service.RestoreAdminUserKeys(ctx, c.Query("site_id"), userID, c.GetInt("id"))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
			"data":    data,
		})
		return
	}
	recordManageAuditFor(c, userID, "user.ops_restore_keys", map[string]any{
		"site_id": c.Query("site_id"), "risk_control": data["risk_control"],
	})
	common.ApiSuccess(c, data)
}

func parseOpsUserID(c *gin.Context) (int, bool) {
	userID, err := strconv.Atoi(c.Param("id"))
	if err != nil || userID <= 0 {
		common.ApiErrorMsg(c, "无效的用户 ID")
		return 0, false
	}
	return userID, true
}

func parseOptionalBool(raw string) *bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "yes", "y", "on":
		value := true
		return &value
	case "0", "false", "no", "n", "off":
		value := false
		return &value
	default:
		return nil
	}
}
