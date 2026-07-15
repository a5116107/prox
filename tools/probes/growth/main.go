package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	_ "github.com/QuantumNous/new-api/setting/performance_setting"
	_ "github.com/QuantumNous/new-api/setting/ratio_setting"
	_ "github.com/QuantumNous/new-api/setting/system_setting"
)

func mustEnv(k string) string {
	v := strings.TrimSpace(os.Getenv(k))
	if v == "" {
		panic("missing env: " + k)
	}
	return v
}

func mustEnvInt(k string) int {
	v := mustEnv(k)
	out, err := strconv.Atoi(v)
	if err != nil {
		panic(err)
	}
	return out
}

func main() {
	common.IsMasterNode = true
	if err := model.InitDB(); err != nil {
		panic(err)
	}
	model.InitOptionMap()
	common.RedisEnabled = false

	siteID := mustEnv("PROBE_SITE_ID")
	today := model.AgentBusinessDateAt(time.Now())
	signupQuota := mustEnvInt("PROBE_SIGNUP_QUOTA")
	inviteeQuota := mustEnvInt("PROBE_INVITEE_QUOTA")
	inviterQuota := mustEnvInt("PROBE_INVITER_QUOTA")

	agentCfg := operation_setting.GetAgentSetting()
	agentCfg.SiteID = siteID
	if agentCfg.GrowthBudgetQuota < signupQuota+inviteeQuota+inviterQuota+10 {
		agentCfg.GrowthBudgetQuota = signupQuota + inviteeQuota + inviterQuota + 10
	}
	paymentCfg := operation_setting.GetPaymentSetting()
	paymentCfg.ComplianceConfirmed = true
	paymentCfg.ComplianceTermsVersion = operation_setting.CurrentComplianceTermsVersion

	oldSignup := common.QuotaForNewUser
	oldInvitee := common.QuotaForInvitee
	oldInviter := common.QuotaForInviter
	common.QuotaForNewUser = signupQuota
	common.QuotaForInvitee = inviteeQuota
	common.QuotaForInviter = inviterQuota
	defer func() {
		common.QuotaForNewUser = oldSignup
		common.QuotaForInvitee = oldInvitee
		common.QuotaForInviter = oldInviter
		paymentCfg.ComplianceConfirmed = false
		paymentCfg.ComplianceTermsVersion = ""
	}()

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	inviter := &model.User{Username: "gpi" + suffix[len(suffix)-6:], Password: "Password123!", DisplayName: "probe_inviter", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: "default", AffCode: "gi" + suffix[len(suffix)-4:]}
	if err := model.DB.Create(inviter).Error; err != nil {
		panic(err)
	}

	beforeUsed, beforeFrozen := 0, 0
	var beforePool model.AgentBudgetPool
	if err := model.DB.Where("site_id = ? AND pool_type = ? AND budget_date = ?", siteID, "growth", today).First(&beforePool).Error; err == nil {
		beforeUsed = beforePool.UsedQuota
		beforeFrozen = beforePool.FrozenQuota
	}

	invitee := &model.User{Username: "gpu" + suffix[len(suffix)-6:], Password: "Password123!", DisplayName: "probe_user", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: "default"}
	if err := invitee.Insert(inviter.Id); err != nil {
		panic(err)
	}

	var midPool model.AgentBudgetPool
	if err := model.DB.Where("site_id = ? AND pool_type = ? AND budget_date = ?", siteID, "growth", today).First(&midPool).Error; err != nil {
		panic(err)
	}

	fmt.Printf("signup_ok user=%d inviter=%d quota=%d aff=%d used_delta=%d frozen_delta=%d\n",
		invitee.Id, inviter.Id, invitee.Quota, inviterQuota, midPool.UsedQuota-beforeUsed, midPool.FrozenQuota-beforeFrozen)

	freshInviter, err := model.GetUserById(inviter.Id, true)
	if err != nil {
		panic(err)
	}
	if err := freshInviter.TransferAffQuotaToQuota(inviterQuota); err != nil {
		panic(err)
	}

	var afterPool model.AgentBudgetPool
	if err := model.DB.Where("site_id = ? AND pool_type = ? AND budget_date = ?", siteID, "growth", today).First(&afterPool).Error; err != nil {
		panic(err)
	}

	fmt.Printf("transfer_ok inviter=%d quota=%d used_delta=%d frozen_delta=%d\n",
		inviter.Id, inviterQuota, afterPool.UsedQuota-beforeUsed, afterPool.FrozenQuota-beforeFrozen)

	rewardId1 := fmt.Sprintf("signup-reward:%d", invitee.Id)
	rewardId2 := fmt.Sprintf("invitee-reward:%d:%d", inviter.Id, invitee.Id)
	rewardId3 := fmt.Sprintf("inviter-reserve:%d:%d", inviter.Id, invitee.Id)

	if err := model.DB.Exec("DELETE FROM agent_budget_transactions WHERE idempotency_key IN (?, ?, ?) OR idempotency_key LIKE ?", rewardId1, rewardId2, rewardId3, fmt.Sprintf("aff-transfer:%d:%d:%%", inviter.Id, inviterQuota)).Error; err != nil {
		panic(err)
	}
	if err := model.DB.Exec("UPDATE agent_budget_pools SET used_quota = GREATEST(used_quota - ?, 0), frozen_quota = GREATEST(frozen_quota - ?, 0) WHERE site_id = ? AND pool_type = ? AND budget_date = ?", signupQuota+inviteeQuota+inviterQuota, inviterQuota, siteID, "growth", today).Error; err != nil {
		panic(err)
	}
	if err := model.DB.Exec("UPDATE users SET quota = GREATEST(quota - ?, 0) WHERE id = ?", signupQuota+inviteeQuota, invitee.Id).Error; err != nil {
		panic(err)
	}
	if err := model.DB.Exec("UPDATE users SET quota = GREATEST(quota - ?, 0), aff_quota = GREATEST(aff_quota - ?, 0), aff_history = GREATEST(aff_history - ?, 0), aff_count = GREATEST(aff_count - 1, 0) WHERE id = ?", inviterQuota, inviterQuota, inviterQuota, inviter.Id).Error; err != nil {
		panic(err)
	}
	if err := model.DB.Unscoped().Delete(&model.User{}, invitee.Id).Error; err != nil {
		panic(err)
	}
	if err := model.DB.Unscoped().Delete(&model.User{}, inviter.Id).Error; err != nil {
		panic(err)
	}

	var finalPool model.AgentBudgetPool
	if err := model.DB.Where("site_id = ? AND pool_type = ? AND budget_date = ?", siteID, "growth", today).First(&finalPool).Error; err == nil {
		fmt.Printf("cleanup_ok pool_used=%d pool_frozen=%d\n", finalPool.UsedQuota, finalPool.FrozenQuota)
	} else {
		fmt.Printf("cleanup_ok pool_missing=true\n")
	}
}
