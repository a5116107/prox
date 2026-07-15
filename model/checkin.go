package model

import (
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"gorm.io/gorm"
)

// 签到渠道：不同渠道的签到相互独立（各自每日一次、各自预算池），但都遵循后台配置。
const (
	CheckinChannelWeb       = "web"
	CheckinChannelCommunity = "community"
	CheckinChannelQQ        = "qq"
	CheckinChannelTG        = "tg"
)

// normalizeCheckinChannel 归一化渠道值，空值回退为 web（兼容旧调用）。
func normalizeCheckinChannel(channel string) string {
	switch channel {
	case CheckinChannelWeb, CheckinChannelCommunity, CheckinChannelQQ, CheckinChannelTG:
		return channel
	default:
		return CheckinChannelWeb
	}
}

// Checkin 签到记录
type Checkin struct {
	Id           int    `json:"id" gorm:"primaryKey;autoIncrement"`
	UserId       int    `json:"user_id" gorm:"not null;uniqueIndex:idx_user_checkin_date_channel"`
	CheckinDate  string `json:"checkin_date" gorm:"type:varchar(10);not null;uniqueIndex:idx_user_checkin_date_channel"` // 格式: YYYY-MM-DD
	Channel      string `json:"channel" gorm:"type:varchar(16);not null;default:'web';uniqueIndex:idx_user_checkin_date_channel"`
	QuotaAwarded int    `json:"quota_awarded" gorm:"not null"`
	CreatedAt    int64  `json:"created_at" gorm:"bigint"`
}

// CheckinRecord 用于API返回的签到记录（不包含敏感字段）
type CheckinRecord struct {
	CheckinDate  string `json:"checkin_date"`
	Channel      string `json:"channel"`
	QuotaAwarded int    `json:"quota_awarded"`
}

type CheckinRule struct {
	FixedQuota int `json:"fixed_quota"`
	RewardMin  int `json:"reward_min"`
	RewardMax  int `json:"reward_max"`
	BonusDays  int `json:"bonus_days"`
	BonusExtra int `json:"bonus_extra"`
}

type CheckinAwardPreview struct {
	Channel      string `json:"channel"`
	QuotaAwarded int    `json:"quota_awarded"`
	BaseQuota    int    `json:"base_quota"`
	BonusQuota   int    `json:"bonus_quota"`
	StreakAfter  int    `json:"streak_after"`
}

func (Checkin) TableName() string {
	return "checkins"
}

// migrateCheckinLegacyUniqueIndex removes the obsolete unique index on
// (user_id, checkin_date). Multi-channel checkin requires uniqueness to be
// scoped by (user_id, checkin_date, channel); keeping the legacy index blocks
// same-day signins across web/community/qq/tg.
func migrateCheckinLegacyUniqueIndex() error {
	if DB == nil || !DB.Migrator().HasTable(&Checkin{}) {
		return nil
	}
	if DB.Migrator().HasIndex(&Checkin{}, "idx_user_checkin_date") {
		if err := DB.Migrator().DropIndex(&Checkin{}, "idx_user_checkin_date"); err != nil {
			return fmt.Errorf("drop legacy checkin index idx_user_checkin_date: %w", err)
		}
	}
	return nil
}

// GetUserCheckinRecords 获取用户在指定日期范围内的签到记录（所有渠道）
func GetUserCheckinRecords(userId int, startDate, endDate string) ([]Checkin, error) {
	var records []Checkin
	err := DB.Where("user_id = ? AND checkin_date >= ? AND checkin_date <= ?",
		userId, startDate, endDate).
		Order("checkin_date DESC").
		Find(&records).Error
	return records, err
}

// HasCheckedInTodayChannel 检查用户今天在指定渠道是否已签到
func HasCheckedInTodayChannel(userId int, channel string) (bool, error) {
	channel = normalizeCheckinChannel(channel)
	today := AgentBusinessDateAt(time.Now())
	var count int64
	err := DB.Model(&Checkin{}).
		Where("user_id = ? AND checkin_date = ? AND channel = ?", userId, today, channel).
		Count(&count).Error
	return count > 0, err
}

// GetTodayCheckedChannels returns channels the user has already checked in today
func GetTodayCheckedChannels(userId int) ([]string, error) {
	today := AgentBusinessDateAt(time.Now())
	var channels []string
	if err := DB.Model(&Checkin{}).Where("user_id = ? AND checkin_date = ?", userId, today).
		Distinct().Pluck("channel", &channels).Error; err != nil {
		return nil, err
	}
	return channels, nil
}

// GetTodayCheckinSummary returns per-channel checkin records for today
func GetTodayCheckinSummary(userId int) ([]map[string]interface{}, error) {
	today := AgentBusinessDateAt(time.Now())
	var records []Checkin
	if err := DB.Where("user_id = ? AND checkin_date = ?", userId, today).Find(&records).Error; err != nil {
		return nil, err
	}
	result := make([]map[string]interface{}, len(records))
	for i, r := range records {
		result[i] = map[string]interface{}{
			"channel":       r.Channel,
			"quota_awarded": r.QuotaAwarded,
			"checkin_date":  r.CheckinDate,
		}
	}
	return result, nil
}

// HasCheckedInToday 兼容旧调用：检查 web 渠道今天是否已签到
func HasCheckedInToday(userId int) (bool, error) {
	return HasCheckedInTodayAnyChannel(userId)
}

// HasCheckedInTodayAnyChannel 检查用户今天任一渠道是否已签到
func HasCheckedInTodayAnyChannel(userId int) (bool, error) {
	today := AgentBusinessDateAt(time.Now())
	var count int64
	err := DB.Model(&Checkin{}).
		Where("user_id = ? AND checkin_date = ?", userId, today).
		Count(&count).Error
	return count > 0, err
}

// CountConsecutiveCheckinDays 基于权威 checkins 表计算指定渠道的连续签到天数。
func CountConsecutiveCheckinDays(userId int, channel string) int {
	if userId <= 0 {
		return 0
	}
	channel = normalizeCheckinChannel(channel)
	var records []Checkin
	if err := DB.Where("user_id = ? AND channel = ?", userId, channel).
		Order("checkin_date DESC").
		Limit(366).
		Find(&records).Error; err != nil {
		return 0
	}
	expected := AgentBusinessDateAt(time.Now())
	streak := 0
	for _, record := range records {
		if record.CheckinDate != expected {
			if streak == 0 {
				continue
			}
			break
		}
		streak++
		day, err := time.Parse("2006-01-02", expected)
		if err != nil {
			break
		}
		expected = day.AddDate(0, 0, -1).Format("2006-01-02")
	}
	return streak
}

// effectiveCheckinQuotaRange 返回指定渠道的签到额度范围（遵循后台配置，渠道可覆盖）
func effectiveCheckinQuotaRange(setting *operation_setting.CheckinSetting, channel string) (int, int) {
	min, max := setting.ChannelQuotaRange(channel)
	return min, max
}

func effectiveCheckinQuotaRangeForRule(setting *operation_setting.CheckinSetting, channel string, rule *CheckinRule) (int, int) {
	minQuota, maxQuota := effectiveCheckinQuotaRange(setting, channel)
	if rule == nil {
		return minQuota, maxQuota
	}
	if rule.RewardMin > 0 {
		minQuota = rule.RewardMin
	}
	if rule.RewardMax > 0 {
		maxQuota = rule.RewardMax
	}
	if maxQuota <= 0 {
		maxQuota = minQuota
	}
	if minQuota <= 0 {
		minQuota = maxQuota
	}
	if maxQuota < minQuota {
		maxQuota = minQuota
	}
	return minQuota, maxQuota
}

func countConsecutiveCheckinDaysBeforeDate(userId int, channel string, anchor time.Time) int {
	if userId <= 0 {
		return 0
	}
	channel = normalizeCheckinChannel(channel)
	var records []Checkin
	if err := DB.Where("user_id = ? AND channel = ?", userId, channel).
		Order("checkin_date DESC").
		Limit(366).
		Find(&records).Error; err != nil {
		return 0
	}
	expected := anchor.AddDate(0, 0, -1).Format("2006-01-02")
	streak := 0
	for _, record := range records {
		if record.CheckinDate != expected {
			if streak == 0 {
				continue
			}
			break
		}
		streak++
		day, err := time.Parse("2006-01-02", expected)
		if err != nil {
			break
		}
		expected = day.AddDate(0, 0, -1).Format("2006-01-02")
	}
	return streak
}

func PreviewCheckinAwardByChannel(userId int, channel string, rule *CheckinRule) (*CheckinAwardPreview, error) {
	channel = normalizeCheckinChannel(channel)
	setting := operation_setting.GetCheckinSetting()
	if !setting.Enabled {
		return nil, errors.New("签到功能未启用")
	}
	if !setting.ChannelEnabled(channel) {
		return nil, errors.New("当前渠道签到未开启")
	}
	hasChecked, err := HasCheckedInTodayChannel(userId, channel)
	if err != nil {
		return nil, err
	}
	if hasChecked {
		return nil, errors.New("今日已签到")
	}

	baseQuota := 0
	if rule != nil && rule.FixedQuota > 0 {
		baseQuota = rule.FixedQuota
	} else {
		minQuota, maxQuota := effectiveCheckinQuotaRangeForRule(setting, channel, rule)
		baseQuota = minQuota
		if maxQuota > minQuota {
			baseQuota = minQuota + rand.Intn(maxQuota-minQuota+1)
		}
		baseQuota = int(float64(baseQuota) * CheckinTier(CountTotalCheckinDays(userId)+1).Multiplier)
	}

	streakAfter := countConsecutiveCheckinDaysBeforeDate(userId, channel, time.Now()) + 1
	bonusQuota := 0
	if rule != nil && rule.BonusDays > 0 && rule.BonusExtra > 0 && streakAfter >= rule.BonusDays && streakAfter%rule.BonusDays == 0 {
		bonusQuota = rule.BonusExtra
	}

	return &CheckinAwardPreview{
		Channel:      channel,
		QuotaAwarded: baseQuota + bonusQuota,
		BaseQuota:    baseQuota,
		BonusQuota:   bonusQuota,
		StreakAfter:  streakAfter,
	}, nil
}

// UserCheckinByChannel 执行指定渠道的用户签到（渠道隔离：每渠道每日一次、独立额度）
func UserCheckinByChannel(userId int, channel string) (*Checkin, error) {
	return UserCheckinByChannelWithQuota(userId, channel, 0)
}

// UserCheckinByChannelWithQuota supports a group-level fixed quota override.
// quotaOverride<=0 keeps the site/channel random quota rule.
func UserCheckinByChannelWithQuota(userId int, channel string, quotaOverride int) (*Checkin, error) {
	channel = normalizeCheckinChannel(channel)
	setting := operation_setting.GetCheckinSetting()
	if !setting.Enabled {
		return nil, errors.New("签到功能未启用")
	}
	if !setting.ChannelEnabled(channel) {
		return nil, errors.New("当前渠道签到未开启")
	}

	// 检查今天该渠道是否已签到
	hasChecked, err := HasCheckedInTodayChannel(userId, channel)
	if err != nil {
		return nil, err
	}
	if hasChecked {
		return nil, errors.New("今日已签到")
	}

	// 计算奖励额度：群组固定额度优先；否则按渠道配置随机并叠加签到等级倍率。
	quotaAwarded := quotaOverride
	if quotaAwarded <= 0 {
		minQuota, maxQuota := effectiveCheckinQuotaRange(setting, channel)
		quotaAwarded = minQuota
		if maxQuota > minQuota {
			quotaAwarded = minQuota + rand.Intn(maxQuota-minQuota+1)
		}
		// 用户等级加倍率（累计签到天数派生：青铜×1.0 / 白银×1.1 / 黄金×1.25 / 钻石×1.5）
		quotaAwarded = int(float64(quotaAwarded) * CheckinTier(CountTotalCheckinDays(userId)+1).Multiplier)
	}

	// 渠道独立预算池：若配置了当日发放上限，校验「当日该渠道已发放 + 本次」不超预算。
	if budget := setting.ChannelDailyBudget(channel); budget > 0 {
		todayDate := AgentBusinessDateAt(time.Now())
		var awarded int64
		DB.Model(&Checkin{}).
			Where("checkin_date = ? AND channel = ?", todayDate, channel).
			Select("COALESCE(SUM(quota_awarded), 0)").Scan(&awarded)
		if int(awarded)+quotaAwarded > budget {
			return nil, errors.New("今日该渠道签到奖励池已发完，请明天再来")
		}
	}

	today := AgentBusinessDateAt(time.Now())
	checkin := &Checkin{
		UserId:       userId,
		CheckinDate:  today,
		Channel:      channel,
		QuotaAwarded: quotaAwarded,
		CreatedAt:    time.Now().Unix(),
	}

	if common.UsingSQLite {
		return userCheckinWithoutTransaction(checkin, userId, quotaAwarded, channel)
	}
	return userCheckinWithTransaction(checkin, userId, quotaAwarded, channel)
}

// CountTotalCheckinDays 用户所有渠道累计签到天数（去重日期）。
func CountTotalCheckinDays(userId int) int {
	var count int64
	DB.Model(&Checkin{}).Where("user_id = ?", userId).
		Distinct("checkin_date").Count(&count)
	return int(count)
}

// CheckinTierInfo 等级信息：名称/emoji/倍率/累计天数/下一级。
type CheckinTierInfo struct {
	Name           string  `json:"name"`
	Emoji          string  `json:"emoji"`
	Multiplier     float64 `json:"multiplier"`
	TotalDays      int     `json:"total_days"`
	NextName       string  `json:"next_name"`
	NextEmoji      string  `json:"next_emoji"`
	NextMultiplier float64 `json:"next_multiplier"`
	DaysToNext     int     `json:"days_to_next"`
}

// CheckinTier 根据累计签到天数派生等级及奖励倍率。
// 青铜×1.0(0-6) / 白银×1.1(7-29) / 黄金×1.25(30-89) / 钻石×1.5(90+)。
func CheckinTier(totalDays int) CheckinTierInfo {
	type t struct {
		name, emoji string
		min         int
		mult        float64
	}
	tiers := []t{
		{"青铜", "🥉", 0, 1.0},
		{"白银", "🥈", 7, 1.1},
		{"黄金", "🥇", 30, 1.25},
		{"钻石", "💎", 90, 1.5},
	}
	idx := 0
	for i, tr := range tiers {
		if totalDays >= tr.min {
			idx = i
		}
	}
	cur := tiers[idx]
	info := CheckinTierInfo{
		Name: cur.name, Emoji: cur.emoji, Multiplier: cur.mult, TotalDays: totalDays,
	}
	if idx < len(tiers)-1 {
		nx := tiers[idx+1]
		info.NextName = nx.name
		info.NextEmoji = nx.emoji
		info.NextMultiplier = nx.mult
		info.DaysToNext = nx.min - totalDays
		if info.DaysToNext < 0 {
			info.DaysToNext = 0
		}
	}
	return info
}

// UserCheckin 兼容旧调用：默认 web 渠道签到
func UserCheckin(userId int) (*Checkin, error) {
	return UserCheckinByChannel(userId, CheckinChannelWeb)
}

// userCheckinWithTransaction 使用事务执行签到（适用于 MySQL 和 PostgreSQL）
func userCheckinWithTransaction(checkin *Checkin, userId int, quotaAwarded int, channel string) (*Checkin, error) {
	err := DB.Transaction(func(tx *gorm.DB) error {
		// 步骤1: 创建签到记录
		// 数据库唯一约束 (user_id, checkin_date, channel) 防止并发重复签到
		if err := tx.Create(checkin).Error; err != nil {
			return errors.New("签到失败，请稍后重试")
		}

		// 步骤2: 在事务中扣减统一活动奖池并增加用户额度
		if err := grantCheckinQuotaFromBudgetTx(tx, userId, quotaAwarded, channel, checkin.CheckinDate); err != nil {
			if errors.Is(err, ErrBudgetPoolQuotaInsufficient) {
				return errors.New("今日签到奖励池已发完，请明天再来")
			}
			if errors.Is(err, ErrOpsFundQuotaInsufficient) {
				return errors.New("当前活动基金不足，请联系管理员补充签到基金")
			}
			return errors.New("签到失败：更新额度出错")
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// 事务成功后，异步更新缓存。
	scheduleUserQuotaCacheIncrement(userId, int64(quotaAwarded))

	return checkin, nil
}

// userCheckinWithoutTransaction 不使用事务执行签到（适用于 SQLite）
func userCheckinWithoutTransaction(checkin *Checkin, userId int, quotaAwarded int, channel string) (*Checkin, error) {
	return userCheckinWithTransaction(checkin, userId, quotaAwarded, channel)
}

func grantCheckinQuotaFromBudgetTx(tx *gorm.DB, userId int, quotaAwarded int, channel string, checkinDate string) error {
	idem := fmt.Sprintf("checkin:%s:%d:%s", normalizeCheckinChannel(channel), userId, checkinDate)
	remark := fmt.Sprintf("checkin reward channel=%s user_id=%d date=%s", normalizeCheckinChannel(channel), userId, checkinDate)
	metadata := fmt.Sprintf(`{"settlement_id":%q,"channel":%q,"checkin_date":%q,"user_id":%d,"source_type":"checkin_reward"}`, idem, normalizeCheckinChannel(channel), checkinDate, userId)
	return GrantQuotaFromBudgetPoolWithSourceTx(tx, userId, "activity", quotaAwarded, idem, remark, "checkin_reward", metadata)
}

// GetUserCheckinStatsByChannel 获取用户某渠道的签到统计信息
func GetUserCheckinStatsByChannel(userId int, month string, channel string) (map[string]interface{}, error) {
	channel = normalizeCheckinChannel(channel)
	startDate := month + "-01"
	endDate := month + "-31"

	var records []Checkin
	if err := DB.Where("user_id = ? AND channel = ? AND checkin_date >= ? AND checkin_date <= ?",
		userId, channel, startDate, endDate).
		Order("checkin_date DESC").Find(&records).Error; err != nil {
		return nil, err
	}

	checkinRecords := make([]CheckinRecord, len(records))
	for i, r := range records {
		checkinRecords[i] = CheckinRecord{
			CheckinDate:  r.CheckinDate,
			Channel:      r.Channel,
			QuotaAwarded: r.QuotaAwarded,
		}
	}

	hasCheckedToday, _ := HasCheckedInTodayChannel(userId, channel)

	var totalCheckins int64
	var totalQuota int64
	DB.Model(&Checkin{}).Where("user_id = ? AND channel = ?", userId, channel).Count(&totalCheckins)
	DB.Model(&Checkin{}).Where("user_id = ? AND channel = ?", userId, channel).Select("COALESCE(SUM(quota_awarded), 0)").Scan(&totalQuota)

	return map[string]interface{}{
		"channel":          channel,
		"total_quota":      totalQuota,
		"total_checkins":   totalCheckins,
		"checkin_count":    len(records),
		"checked_in_today": hasCheckedToday,
		"records":          checkinRecords,
	}, nil
}

// GetUserCheckinStats 兼容旧调用：返回 web 渠道统计 + 全渠道累计
func GetUserCheckinStats(userId int, month string) (map[string]interface{}, error) {
	startDate := month + "-01"
	endDate := month + "-31"

	records, err := GetUserCheckinRecords(userId, startDate, endDate)
	if err != nil {
		return nil, err
	}

	checkinRecords := make([]CheckinRecord, len(records))
	for i, r := range records {
		checkinRecords[i] = CheckinRecord{
			CheckinDate:  r.CheckinDate,
			Channel:      r.Channel,
			QuotaAwarded: r.QuotaAwarded,
		}
	}

	hasCheckedToday, _ := HasCheckedInTodayAnyChannel(userId)
	checkedChannels, _ := GetTodayCheckedChannels(userId)
	todaySummary, _ := GetTodayCheckinSummary(userId)

	var totalCheckins int64
	var totalQuota int64
	DB.Model(&Checkin{}).Where("user_id = ?", userId).Count(&totalCheckins)
	DB.Model(&Checkin{}).Where("user_id = ?", userId).Select("COALESCE(SUM(quota_awarded), 0)").Scan(&totalQuota)

	return map[string]interface{}{
		"total_quota":         totalQuota,
		"total_checkins":      totalCheckins,
		"checkin_count":       len(records),
		"checked_in_today":    hasCheckedToday,
		"checked_in_channels": checkedChannels,
		"today_summary":       todaySummary,
		"records":             checkinRecords,
	}, nil
}
