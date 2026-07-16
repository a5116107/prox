package model

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func resetChannelRoutingHealthForTest() {
	channelRoutingHealthMu.Lock()
	channelRoutingHealth = make(map[int]*channelRoutingRuntimeState)
	channelRoutingHealthMu.Unlock()
}

func configureChannelRoutingTest(tb testing.TB) {
	tb.Helper()

	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	originalRedisEnabled := common.RedisEnabled
	originalHealthEnabled := common.ChannelRoutingHealthEnabled
	originalFailureThreshold := common.ChannelRoutingFailureThreshold
	originalFailureWindow := common.ChannelRoutingFailureWindowSeconds
	originalCooldown := common.ChannelRoutingCooldownSeconds
	originalRateLimitCooldown := common.ChannelRoutingRateLimitCooldownSeconds
	originalLatencyBaseline := common.ChannelRoutingLatencyBaselineMS
	originalGroupChannels := group2model2channels
	originalChannels := channelsIDM

	common.MemoryCacheEnabled = true
	common.RedisEnabled = false
	common.ChannelRoutingHealthEnabled = true
	common.ChannelRoutingFailureThreshold = 2
	common.ChannelRoutingFailureWindowSeconds = 60
	common.ChannelRoutingCooldownSeconds = 30
	common.ChannelRoutingRateLimitCooldownSeconds = 30
	common.ChannelRoutingLatencyBaselineMS = 2000

	priority := int64(10)
	weight := uint(100)
	channelsIDM = map[int]*Channel{
		1: {Id: 1, Name: "channel-1", Status: common.ChannelStatusEnabled, Priority: &priority, Weight: &weight},
		2: {Id: 2, Name: "channel-2", Status: common.ChannelStatusEnabled, Priority: &priority, Weight: &weight},
	}
	group2model2channels = map[string]map[string][]int{
		"default": {"gpt-test": {1, 2}},
	}
	resetChannelRoutingHealthForTest()

	tb.Cleanup(func() {
		common.MemoryCacheEnabled = originalMemoryCacheEnabled
		common.RedisEnabled = originalRedisEnabled
		common.ChannelRoutingHealthEnabled = originalHealthEnabled
		common.ChannelRoutingFailureThreshold = originalFailureThreshold
		common.ChannelRoutingFailureWindowSeconds = originalFailureWindow
		common.ChannelRoutingCooldownSeconds = originalCooldown
		common.ChannelRoutingRateLimitCooldownSeconds = originalRateLimitCooldown
		common.ChannelRoutingLatencyBaselineMS = originalLatencyBaseline
		group2model2channels = originalGroupChannels
		channelsIDM = originalChannels
		resetChannelRoutingHealthForTest()
	})
}

func BenchmarkGetRandomSatisfiedChannelWithRoutingHealth(b *testing.B) {
	configureChannelRoutingTest(b)
	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		selected, err := GetRandomSatisfiedChannelWithExclusions("default", "gpt-test", 0, nil)
		if err != nil {
			b.Fatal(err)
		}
		if selected == nil {
			b.Fatal("expected a channel")
		}
	}
}

func TestGetRandomSatisfiedChannelExcludesAttemptedPeer(t *testing.T) {
	configureChannelRoutingTest(t)

	first, err := GetRandomSatisfiedChannelWithExclusions("default", "gpt-test", 0, nil)
	require.NoError(t, err)
	require.NotNil(t, first)

	second, err := GetRandomSatisfiedChannelWithExclusions("default", "gpt-test", 1, map[int]struct{}{first.Id: {}})
	require.NoError(t, err)
	require.NotNil(t, second)
	require.NotEqual(t, first.Id, second.Id)
}

func TestChannelRoutingCooldownSkipsUnhealthyChannelWhenPeerExists(t *testing.T) {
	configureChannelRoutingTest(t)

	RecordChannelRoutingFailure(1, http.StatusTooManyRequests)
	require.False(t, IsChannelRoutingAvailable(1))

	for range 20 {
		selected, err := GetRandomSatisfiedChannelWithExclusions("default", "gpt-test", 0, nil)
		require.NoError(t, err)
		require.NotNil(t, selected)
		require.Equal(t, 2, selected.Id)
	}
}

func TestChannelRoutingCircuitOpensAfterFailureThreshold(t *testing.T) {
	configureChannelRoutingTest(t)

	RecordChannelRoutingFailure(1, http.StatusBadGateway)
	require.True(t, IsChannelRoutingAvailable(1))
	RecordChannelRoutingFailure(1, http.StatusServiceUnavailable)
	require.False(t, IsChannelRoutingAvailable(1))

	RecordChannelRoutingSuccess(1, 100*time.Millisecond, 80*time.Millisecond)
	require.False(t, IsChannelRoutingAvailable(1), "an in-flight success must not cancel an active cooldown")
}

func TestChannelRoutingDoesNotPenalizeRequestValidationErrors(t *testing.T) {
	configureChannelRoutingTest(t)

	for range 5 {
		RecordChannelRoutingFailure(1, http.StatusBadRequest)
	}
	require.True(t, IsChannelRoutingAvailable(1))
}

func TestChannelRoutingLatencyLowersEffectiveWeight(t *testing.T) {
	configureChannelRoutingTest(t)

	RecordChannelRoutingSuccess(1, 8*time.Second, 6*time.Second)
	RecordChannelRoutingSuccess(2, 300*time.Millisecond, 200*time.Millisecond)

	require.Less(
		t,
		channelRoutingEffectiveWeight(channelsIDM[1], false),
		channelRoutingEffectiveWeight(channelsIDM[2], false),
	)
}

func TestChannelRoutingFailsOpenWhenEveryPeerIsCoolingDown(t *testing.T) {
	configureChannelRoutingTest(t)

	RecordChannelRoutingFailure(1, http.StatusTooManyRequests)
	RecordChannelRoutingFailure(2, http.StatusTooManyRequests)

	selected, err := GetRandomSatisfiedChannelWithExclusions("default", "gpt-test", 0, nil)
	require.NoError(t, err)
	require.NotNil(t, selected)
}

func TestChannelRoutingDatabasePathUsesExclusionsAndCooldowns(t *testing.T) {
	configureChannelRoutingTest(t)
	originalDB := DB
	originalSQLite := common.UsingSQLite
	common.MemoryCacheEnabled = false
	common.UsingSQLite = true

	dsn := fmt.Sprintf("file:channel-routing-%d?mode=memory&cache=shared", time.Now().UnixNano())
	database, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	DB = database
	t.Cleanup(func() {
		DB = originalDB
		common.UsingSQLite = originalSQLite
	})
	require.NoError(t, DB.AutoMigrate(&Channel{}, &Ability{}))

	priority := int64(10)
	weight := uint(100)
	channels := []Channel{
		{Id: 1, Name: "database-channel-1", Status: common.ChannelStatusEnabled, Priority: &priority, Weight: &weight},
		{Id: 2, Name: "database-channel-2", Status: common.ChannelStatusEnabled, Priority: &priority, Weight: &weight},
	}
	require.NoError(t, DB.Create(&channels).Error)
	require.NoError(t, DB.Create(&[]Ability{
		{Group: "default", Model: "gpt-test", ChannelId: 1, Enabled: true, Priority: &priority, Weight: weight},
		{Group: "default", Model: "gpt-test", ChannelId: 2, Enabled: true, Priority: &priority, Weight: weight},
	}).Error)

	selected, err := GetChannelWithExclusions("default", "gpt-test", 1, map[int]struct{}{1: {}})
	require.NoError(t, err)
	require.NotNil(t, selected)
	require.Equal(t, 2, selected.Id)

	RecordChannelRoutingFailure(1, http.StatusTooManyRequests)
	for range 10 {
		selected, err = GetChannelWithExclusions("default", "gpt-test", 0, nil)
		require.NoError(t, err)
		require.NotNil(t, selected)
		require.Equal(t, 2, selected.Id)
	}
}
