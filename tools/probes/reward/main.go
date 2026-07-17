package main

import (
	"fmt"
	"os"
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

func main() {
	common.IsMasterNode = true
	if err := model.InitDB(); err != nil {
		panic(err)
	}
	model.InitOptionMap()
	if err := model.InitLogDB(); err != nil {
		panic(err)
	}
	common.RedisEnabled = false

	siteID := mustEnv("PROBE_SITE_ID")
	roomID := mustEnv("PROBE_ROOM_ID")
	rewardQuota := 1
	checkinQuotaExpectedMax := 1
	today := model.AgentBusinessDateAt(time.Now())
	probeTag := fmt.Sprintf("probe-%d", time.Now().UnixNano())
	username1 := fmt.Sprintf("prw%d", time.Now().Unix()%1000000)
	username2 := fmt.Sprintf("pck%d", (time.Now().Unix()+1)%1000000)

	agentCfg := operation_setting.GetAgentSetting()
	agentCfg.SiteID = siteID
	if agentCfg.ActivityBudgetQuota < 10 {
		agentCfg.ActivityBudgetQuota = 10
	}

	checkinCfg := operation_setting.GetCheckinSetting()
	checkinCfg.Enabled = true
	enabled := true
	if checkinCfg.Channels == nil {
		checkinCfg.Channels = map[string]operation_setting.CheckinChannelSetting{}
	}
	checkinCfg.MinQuota = 1
	checkinCfg.MaxQuota = 1
	checkinCfg.Channels[model.CheckinChannelCommunity] = operation_setting.CheckinChannelSetting{
		Enabled:     &enabled,
		MinQuota:    1,
		MaxQuota:    1,
		DailyBudget: 0,
	}

	u1 := &model.User{
		Username:    username1,
		Password:    "Password123!",
		DisplayName: username1,
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		Group:       "default",
		AffCode:     "rw" + fmt.Sprintf("%04d", time.Now().Unix()%10000),
	}
	u2 := &model.User{
		Username:    username2,
		Password:    "Password123!",
		DisplayName: username2,
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		Group:       "default",
		AffCode:     "ck" + fmt.Sprintf("%04d", (time.Now().Unix()+1)%10000),
	}
	if err := model.DB.Create(u1).Error; err != nil {
		panic(err)
	}
	rewardIdem := fmt.Sprintf("community-reward:%d:%s:%s:%s", u1.Id, roomID, "community_reward", probeTag)
	var checkinIdem string
	cleanupDone := false
	cleanup := func() error {
		userIDs := []int{u1.Id}
		if u2.Id > 0 {
			userIDs = append(userIDs, u2.Id)
		}
		return model.DB.Transaction(func(tx *gorm.DB) error {
			if rewardIdem != "" {
				if err := tx.Where("user_id = ? AND room_id = ? AND reward_type = ? AND reward_key = ?", u1.Id, roomID, "community_reward", probeTag).
					Delete(&model.CommunityBotReward{}).Error; err != nil {
					return err
				}
			}
			if checkinIdem != "" {
				if err := tx.Where("user_id = ? AND channel = ? AND checkin_date = ?", u2.Id, model.CheckinChannelCommunity, today).
					Delete(&model.Checkin{}).Error; err != nil {
					return err
				}
			}
			if err := probeutil.CleanupLedgerArtifactsTx(tx, []string{rewardIdem, checkinIdem}); err != nil {
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
	if err := model.DB.Create(u2).Error; err != nil {
		panic(err)
	}
	checkinIdem = fmt.Sprintf("checkin:%s:%d:%s", model.CheckinChannelCommunity, u2.Id, today)

	beforeUsed := 0
	var beforePool model.AgentBudgetPool
	if err := model.DB.Where("site_id = ? AND pool_type = ? AND budget_date = ?", siteID, "activity", today).First(&beforePool).Error; err == nil {
		beforeUsed = beforePool.UsedQuota
	}

	granted, err := model.GrantCommunityBotRewardIfNeeded(u1.Id, 1, "probe-provider-user", roomID, "community_reward", probeTag, rewardQuota, 6)
	if err != nil {
		panic(err)
	}
	if !granted {
		panic("reward not granted")
	}

	var rewardTxn model.AgentBudgetTransaction
	if err := model.DB.Where("idempotency_key = ?", rewardIdem).First(&rewardTxn).Error; err != nil {
		panic(err)
	}

	checkin, err := model.UserCheckinByChannel(u2.Id, model.CheckinChannelCommunity)
	if err != nil {
		panic(err)
	}
	if checkin.QuotaAwarded < 1 || checkin.QuotaAwarded > checkinQuotaExpectedMax {
		panic(fmt.Sprintf("unexpected checkin quota=%d", checkin.QuotaAwarded))
	}

	if checkin.CheckinDate != today {
		panic(fmt.Sprintf("unexpected checkin date=%s", checkin.CheckinDate))
	}
	var checkinTxn model.AgentBudgetTransaction
	if err := model.DB.Where("idempotency_key = ?", checkinIdem).First(&checkinTxn).Error; err != nil {
		panic(err)
	}

	var afterPool model.AgentBudgetPool
	if err := model.DB.Where("site_id = ? AND pool_type = ? AND budget_date = ?", siteID, "activity", today).First(&afterPool).Error; err != nil {
		panic(err)
	}

	fmt.Printf("reward_ok user=%d quota=%d idem=%s\n", u1.Id, rewardQuota, rewardIdem)
	fmt.Printf("checkin_ok user=%d quota=%d idem=%s\n", u2.Id, checkin.QuotaAwarded, checkinIdem)
	fmt.Printf("pool_delta before=%d after=%d\n", beforeUsed, afterPool.UsedQuota)

	if err := cleanup(); err != nil {
		panic(err)
	}
	cleanupDone = true

	var finalPool model.AgentBudgetPool
	if err := model.DB.Where("site_id = ? AND pool_type = ? AND budget_date = ?", siteID, "activity", today).First(&finalPool).Error; err != nil {
		panic(err)
	}
	fmt.Printf("cleanup_ok pool_used=%d\n", finalPool.UsedQuota)
}
