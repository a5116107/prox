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
	"github.com/QuantumNous/new-api/tools/probes/internal/probeutil"
	"gorm.io/gorm"
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
	siteID := mustEnv("PROBE_SITE_ID")
	signupQuota := mustEnvInt("PROBE_SIGNUP_QUOTA")
	inviteeQuota := mustEnvInt("PROBE_INVITEE_QUOTA")
	inviterQuota := mustEnvInt("PROBE_INVITER_QUOTA")
	if signupQuota <= 0 || inviteeQuota <= 0 {
		panic("probe signup and invitee quotas must be positive")
	}
	if inviterQuota < int(common.QuotaPerUnit) {
		panic(fmt.Sprintf("PROBE_INVITER_QUOTA must be at least %d", int(common.QuotaPerUnit)))
	}
	common.IsMasterNode = true
	if err := model.InitDB(); err != nil {
		panic(err)
	}
	model.InitOptionMap()
	if err := model.InitLogDB(); err != nil {
		panic(err)
	}
	common.RedisEnabled = false

	today := model.AgentBusinessDateAt(time.Now())

	agentCfg := operation_setting.GetAgentSetting()
	agentCfg.SiteID = siteID
	if agentCfg.GrowthBudgetQuota < signupQuota+inviteeQuota+inviterQuota+10 {
		agentCfg.GrowthBudgetQuota = signupQuota + inviteeQuota + inviterQuota + 10
	}
	paymentCfg := operation_setting.GetPaymentSetting()
	oldComplianceConfirmed := paymentCfg.ComplianceConfirmed
	oldComplianceTermsVersion := paymentCfg.ComplianceTermsVersion
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
		paymentCfg.ComplianceConfirmed = oldComplianceConfirmed
		paymentCfg.ComplianceTermsVersion = oldComplianceTermsVersion
	}()

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	inviter := &model.User{Username: "gpi" + suffix[len(suffix)-6:], Password: "Password123!", DisplayName: "probe_inviter", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: "default", AffCode: "gi" + suffix[len(suffix)-4:]}
	if err := model.DB.Create(inviter).Error; err != nil {
		panic(err)
	}
	var invitee model.User
	var signupIdem, inviteeIdem, reserveIdem, transferIdem, reserveRepairIdem string
	cleanupDone := false
	cleanup := func() error {
		userIDs := []int{inviter.Id}
		keys := []string{signupIdem, inviteeIdem, reserveIdem, transferIdem, reserveRepairIdem}
		if invitee.Id > 0 {
			userIDs = append(userIDs, invitee.Id)
			derivedTransferIdem := fmt.Sprintf("invite-aff-transfer:%d:%d:%d", inviter.Id, inviterQuota, inviterQuota)
			keys = append(keys,
				fmt.Sprintf("signup:%d", invitee.Id),
				fmt.Sprintf("invitee:%d:%d", invitee.Id, inviter.Id),
				fmt.Sprintf("inviter-reserve:%d:%d", inviter.Id, invitee.Id),
				derivedTransferIdem,
				derivedTransferIdem+":reserve-repair",
			)
		}
		return model.DB.Transaction(func(tx *gorm.DB) error {
			if err := probeutil.CleanupLedgerArtifactsTx(tx, keys); err != nil {
				return err
			}
			if err := tx.Where("user_id IN ?", userIDs).Delete(&model.Log{}).Error; err != nil {
				return err
			}
			return tx.Unscoped().Where("id IN ?", userIDs).Delete(&model.User{}).Error
		})
	}
	defer func() {
		if !cleanupDone {
			if err := cleanup(); err != nil {
				fmt.Fprintf(os.Stderr, "probe cleanup failed: %v\n", err)
			}
		}
	}()

	beforeUsed, beforeFrozen := 0, 0
	var beforePool model.AgentBudgetPool
	if err := model.DB.Where("site_id = ? AND pool_type = ? AND budget_date = ?", siteID, "growth", today).First(&beforePool).Error; err == nil {
		beforeUsed = beforePool.UsedQuota
		beforeFrozen = beforePool.FrozenQuota
	}

	invitee = model.User{Username: "gpu" + suffix[len(suffix)-6:], Password: "Password123!", DisplayName: "probe_user", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: "default"}
	if err := invitee.Insert(inviter.Id); err != nil {
		panic(err)
	}
	signupIdem = fmt.Sprintf("signup:%d", invitee.Id)
	inviteeIdem = fmt.Sprintf("invitee:%d:%d", invitee.Id, inviter.Id)
	reserveIdem = fmt.Sprintf("inviter-reserve:%d:%d", inviter.Id, invitee.Id)

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
	transferIdem = fmt.Sprintf("invite-aff-transfer:%d:%d:%d", inviter.Id, freshInviter.AffQuota, inviterQuota)
	reserveRepairIdem = transferIdem + ":reserve-repair"
	if err := freshInviter.TransferAffQuotaToQuota(inviterQuota); err != nil {
		panic(err)
	}

	var afterPool model.AgentBudgetPool
	if err := model.DB.Where("site_id = ? AND pool_type = ? AND budget_date = ?", siteID, "growth", today).First(&afterPool).Error; err != nil {
		panic(err)
	}

	fmt.Printf("transfer_ok inviter=%d quota=%d used_delta=%d frozen_delta=%d\n",
		inviter.Id, inviterQuota, afterPool.UsedQuota-beforeUsed, afterPool.FrozenQuota-beforeFrozen)

	if err := cleanup(); err != nil {
		panic(err)
	}
	cleanupDone = true

	var finalPool model.AgentBudgetPool
	if err := model.DB.Where("site_id = ? AND pool_type = ? AND budget_date = ?", siteID, "growth", today).First(&finalPool).Error; err == nil {
		fmt.Printf("cleanup_ok pool_used=%d pool_frozen=%d\n", finalPool.UsedQuota, finalPool.FrozenQuota)
	} else {
		fmt.Printf("cleanup_ok pool_missing=true\n")
	}
}
