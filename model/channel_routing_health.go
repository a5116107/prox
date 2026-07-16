package model

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/go-redis/redis/v8"
)

type channelRoutingRuntimeState struct {
	failureCount  int
	lastFailure   time.Time
	cooldownUntil time.Time
	ewmaLatencyMS float64
	ewmaTTFTMS    float64
}

var (
	channelRoutingHealthMu sync.RWMutex
	channelRoutingHealth   = make(map[int]*channelRoutingRuntimeState)
)

func channelRoutingCooldownKey(channelID int) string {
	return fmt.Sprintf("channelRouting:%d:cooldown", channelID)
}

func channelRoutingFailuresKey(channelID int) string {
	return fmt.Sprintf("channelRouting:%d:failures", channelID)
}

func channelRoutingFailureWindow() time.Duration {
	seconds := common.ChannelRoutingFailureWindowSeconds
	if seconds <= 0 {
		seconds = 60
	}
	return time.Duration(seconds) * time.Second
}

func channelRoutingFailureThreshold() int {
	threshold := common.ChannelRoutingFailureThreshold
	if threshold <= 0 {
		threshold = 1
	}
	return threshold
}

func channelRoutingCooldown(statusCode int) time.Duration {
	seconds := common.ChannelRoutingCooldownSeconds
	if statusCode == http.StatusTooManyRequests {
		seconds = common.ChannelRoutingRateLimitCooldownSeconds
	}
	if seconds <= 0 {
		seconds = 1
	}
	return time.Duration(seconds) * time.Second
}

func shouldPenalizeChannelRouting(statusCode int) bool {
	return statusCode <= 0 ||
		statusCode == http.StatusUnauthorized ||
		statusCode == http.StatusForbidden ||
		statusCode == http.StatusRequestTimeout ||
		statusCode == http.StatusTooManyRequests ||
		statusCode >= http.StatusInternalServerError
}

func setLocalChannelRoutingCooldown(channelID int, until time.Time) {
	channelRoutingHealthMu.Lock()
	state := channelRoutingHealth[channelID]
	if state == nil {
		state = &channelRoutingRuntimeState{}
		channelRoutingHealth[channelID] = state
	}
	if until.After(state.cooldownUntil) {
		state.cooldownUntil = until
	}
	channelRoutingHealthMu.Unlock()
}

func publishChannelRoutingCooldown(channelID int, cooldown time.Duration) {
	if cooldown <= 0 {
		return
	}
	until := time.Now().Add(cooldown)
	setLocalChannelRoutingCooldown(channelID, until)
	if !common.RedisEnabled || common.RDB == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if err := common.RDB.Set(ctx, channelRoutingCooldownKey(channelID), until.UnixMilli(), cooldown).Err(); err != nil {
		common.SysError(fmt.Sprintf("publish channel routing cooldown failed: channel_id=%d error=%v", channelID, err))
	}
}

func recordDistributedChannelRoutingFailure(channelID int) bool {
	if !common.RedisEnabled || common.RDB == nil {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	var countCommand *redis.IntCmd
	_, err := common.RDB.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		countCommand = pipe.Incr(ctx, channelRoutingFailuresKey(channelID))
		pipe.Expire(ctx, channelRoutingFailuresKey(channelID), channelRoutingFailureWindow())
		return nil
	})
	if err != nil {
		common.SysError(fmt.Sprintf("record channel routing failure failed: channel_id=%d error=%v", channelID, err))
		return false
	}
	if countCommand == nil {
		return false
	}
	return countCommand.Val() >= int64(channelRoutingFailureThreshold())
}

func RecordChannelRoutingFailure(channelID int, statusCode int) {
	if !common.ChannelRoutingHealthEnabled || channelID <= 0 || !shouldPenalizeChannelRouting(statusCode) {
		return
	}

	now := time.Now()
	openLocalCircuit := false
	channelRoutingHealthMu.Lock()
	state := channelRoutingHealth[channelID]
	if state == nil {
		state = &channelRoutingRuntimeState{}
		channelRoutingHealth[channelID] = state
	}
	if state.lastFailure.IsZero() || now.Sub(state.lastFailure) > channelRoutingFailureWindow() {
		state.failureCount = 0
	}
	state.failureCount++
	state.lastFailure = now
	openLocalCircuit = statusCode == http.StatusTooManyRequests || state.failureCount >= channelRoutingFailureThreshold()
	channelRoutingHealthMu.Unlock()

	if statusCode == http.StatusTooManyRequests {
		publishChannelRoutingCooldown(channelID, channelRoutingCooldown(statusCode))
		return
	}
	if openLocalCircuit || recordDistributedChannelRoutingFailure(channelID) {
		publishChannelRoutingCooldown(channelID, channelRoutingCooldown(statusCode))
	}
}

func RecordChannelRoutingSuccess(channelID int, latency time.Duration, ttft time.Duration) {
	if !common.ChannelRoutingHealthEnabled || channelID <= 0 {
		return
	}
	channelRoutingHealthMu.Lock()
	state := channelRoutingHealth[channelID]
	if state == nil {
		state = &channelRoutingRuntimeState{}
		channelRoutingHealth[channelID] = state
	}
	state.failureCount = 0
	state.lastFailure = time.Time{}
	if latency > 0 {
		state.ewmaLatencyMS = updateChannelRoutingEWMA(state.ewmaLatencyMS, float64(latency.Milliseconds()))
	}
	if ttft > 0 {
		state.ewmaTTFTMS = updateChannelRoutingEWMA(state.ewmaTTFTMS, float64(ttft.Milliseconds()))
	}
	channelRoutingHealthMu.Unlock()
}

func updateChannelRoutingEWMA(current float64, sample float64) float64 {
	if current <= 0 {
		return sample
	}
	const alpha = 0.2
	return alpha*sample + (1-alpha)*current
}

func localChannelRoutingCooldown(channelID int, now time.Time) time.Time {
	channelRoutingHealthMu.RLock()
	defer channelRoutingHealthMu.RUnlock()
	state := channelRoutingHealth[channelID]
	if state == nil || !state.cooldownUntil.After(now) {
		return time.Time{}
	}
	return state.cooldownUntil
}

func distributedChannelRoutingCooldowns(channelIDs []int, now time.Time) map[int]time.Time {
	result := make(map[int]time.Time)
	if !common.RedisEnabled || common.RDB == nil || len(channelIDs) == 0 {
		return result
	}
	keys := make([]string, 0, len(channelIDs))
	for _, channelID := range channelIDs {
		keys = append(keys, channelRoutingCooldownKey(channelID))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	values, err := common.RDB.MGet(ctx, keys...).Result()
	if err != nil {
		return result
	}
	for index, value := range values {
		if value == nil {
			continue
		}
		untilMS, parseErr := strconv.ParseInt(fmt.Sprint(value), 10, 64)
		if parseErr != nil {
			continue
		}
		until := time.UnixMilli(untilMS)
		if until.After(now) {
			result[channelIDs[index]] = until
			setLocalChannelRoutingCooldown(channelIDs[index], until)
		}
	}
	return result
}

func IsChannelRoutingAvailable(channelID int) bool {
	if !common.ChannelRoutingHealthEnabled || channelID <= 0 {
		return true
	}
	now := time.Now()
	if localChannelRoutingCooldown(channelID, now).After(now) {
		return false
	}
	return !distributedChannelRoutingCooldowns([]int{channelID}, now)[channelID].After(now)
}

func filterChannelRoutingCandidates(channels []*Channel) []*Channel {
	if !common.ChannelRoutingHealthEnabled || len(channels) <= 1 {
		return channels
	}
	now := time.Now()
	channelIDs := make([]int, 0, len(channels))
	for _, channel := range channels {
		channelIDs = append(channelIDs, channel.Id)
	}
	distributedCooldowns := distributedChannelRoutingCooldowns(channelIDs, now)
	healthy := make([]*Channel, 0, len(channels))
	for _, channel := range channels {
		localCooldown := localChannelRoutingCooldown(channel.Id, now)
		distributedCooldown := distributedCooldowns[channel.Id]
		if localCooldown.After(now) || distributedCooldown.After(now) {
			continue
		}
		healthy = append(healthy, channel)
	}
	if len(healthy) == 0 {
		return channels
	}
	return healthy
}

func channelRoutingEffectiveWeight(channel *Channel, allConfiguredWeightsZero bool) int {
	weight := channel.GetWeight()
	if allConfiguredWeightsZero {
		weight = 100
	}
	if weight <= 0 {
		return 0
	}
	if !common.ChannelRoutingHealthEnabled {
		return weight
	}

	channelRoutingHealthMu.RLock()
	state := channelRoutingHealth[channel.Id]
	failureCount := 0
	latencyMS := float64(channel.ResponseTime)
	if state != nil {
		failureCount = state.failureCount
		if state.ewmaTTFTMS > 0 {
			latencyMS = state.ewmaTTFTMS
		} else if state.ewmaLatencyMS > 0 {
			latencyMS = state.ewmaLatencyMS
		}
	}
	channelRoutingHealthMu.RUnlock()

	baselineMS := common.ChannelRoutingLatencyBaselineMS
	if baselineMS <= 0 {
		baselineMS = 2000
	}
	if latencyMS > float64(baselineMS) {
		weight = int(float64(weight) * float64(baselineMS) / latencyMS)
	}
	if failureCount > 0 {
		weight /= failureCount + 1
	}
	if weight < 1 {
		return 1
	}
	return weight
}

func pruneChannelRoutingHealth(activeChannels map[int]*Channel) {
	channelRoutingHealthMu.Lock()
	for channelID := range channelRoutingHealth {
		if _, exists := activeChannels[channelID]; !exists {
			delete(channelRoutingHealth, channelID)
		}
	}
	channelRoutingHealthMu.Unlock()
}
