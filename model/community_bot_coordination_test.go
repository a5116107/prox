package model

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func setupCommunityBotCoordinationTest(t *testing.T) {
	t.Helper()
	require.NoError(t, DB.AutoMigrate(
		&CommunityBotRoomState{},
		&CommunityBotMessageClaim{},
	))
	require.NoError(t, DB.Exec("DELETE FROM community_bot_room_states").Error)
	require.NoError(t, DB.Exec("DELETE FROM community_bot_message_claims").Error)
}

func TestAdvanceCommunityBotRoomStateNeverRegresses(t *testing.T) {
	setupCommunityBotCoordinationTest(t)

	advanced, err := AdvanceCommunityBotRoomState("room-1", "msg-200")
	require.NoError(t, err)
	require.True(t, advanced)

	advanced, err = AdvanceCommunityBotRoomState("room-1", "msg-100")
	require.NoError(t, err)
	require.False(t, advanced)

	state, err := GetCommunityBotRoomState("room-1")
	require.NoError(t, err)
	require.Equal(t, "msg-200", state.LastMessageId)
}

func TestAdvanceCommunityBotRoomStateKeepsHighestConcurrentCursor(t *testing.T) {
	setupCommunityBotCoordinationTest(t)

	const workers = 16
	start := make(chan struct{})
	errors := make(chan error, workers)
	var wg sync.WaitGroup
	for i := 1; i <= workers; i++ {
		wg.Add(1)
		go func(cursor int) {
			defer wg.Done()
			<-start
			_, err := AdvanceCommunityBotRoomState("room-concurrent", fmt.Sprintf("msg-%03d", cursor))
			errors <- err
		}(i)
	}
	close(start)
	wg.Wait()
	close(errors)
	for err := range errors {
		require.NoError(t, err)
	}

	state, err := GetCommunityBotRoomState("room-concurrent")
	require.NoError(t, err)
	require.Equal(t, "msg-016", state.LastMessageId)
}

func TestClaimCommunityBotMessageAllowsOneConcurrentOwner(t *testing.T) {
	setupCommunityBotCoordinationTest(t)

	const workers = 16
	var acquired atomic.Int32
	var winnerMu sync.Mutex
	var winner *CommunityBotMessageClaim
	start := make(chan struct{})
	errors := make(chan error, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			<-start
			claim, ok, err := ClaimCommunityBotMessage(
				"prox",
				"room-1",
				"msg-1",
				"command",
				fmt.Sprintf("worker-%d", worker),
				2*time.Minute,
			)
			errors <- err
			if err != nil {
				return
			}
			if ok {
				acquired.Add(1)
				winnerMu.Lock()
				winner = claim
				winnerMu.Unlock()
			}
		}(i)
	}
	close(start)
	wg.Wait()
	close(errors)
	for err := range errors {
		require.NoError(t, err)
	}
	require.EqualValues(t, 1, acquired.Load())
	require.NotNil(t, winner)

	completed, err := CompleteCommunityBotMessageClaim(winner.Id, winner.OwnerId, winner.FencingToken)
	require.NoError(t, err)
	require.True(t, completed)

	_, ok, err := ClaimCommunityBotMessage("prox", "room-1", "msg-1", "command", "late-worker", 2*time.Minute)
	require.NoError(t, err)
	require.False(t, ok)
}

func TestCommunityBotMessageClaimAllowsOneConcurrentLeaseTakeover(t *testing.T) {
	setupCommunityBotCoordinationTest(t)

	first, ok, err := ClaimCommunityBotMessage("prox", "room-1", "msg-expired", "command", "worker-a", time.Minute)
	require.NoError(t, err)
	require.True(t, ok)
	require.NoError(t, DB.Model(&CommunityBotMessageClaim{}).
		Where("id = ?", first.Id).
		Update("lease_until", time.Now().Add(-time.Minute).Unix()).Error)

	const workers = 16
	var acquired atomic.Int32
	start := make(chan struct{})
	errors := make(chan error, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			<-start
			_, claimed, claimErr := ClaimCommunityBotMessage(
				"prox",
				"room-1",
				"msg-expired",
				"command",
				fmt.Sprintf("takeover-%d", worker),
				time.Minute,
			)
			errors <- claimErr
			if claimErr == nil && claimed {
				acquired.Add(1)
			}
		}(i)
	}
	close(start)
	wg.Wait()
	close(errors)
	for err := range errors {
		require.NoError(t, err)
	}
	require.EqualValues(t, 1, acquired.Load())
}

func TestCommunityBotMessageClaimUsesFencingOnLeaseTakeover(t *testing.T) {
	setupCommunityBotCoordinationTest(t)

	first, ok, err := ClaimCommunityBotMessage("prox", "room-1", "msg-2", "command", "worker-a", time.Minute)
	require.NoError(t, err)
	require.True(t, ok)
	require.NoError(t, DB.Model(&CommunityBotMessageClaim{}).
		Where("id = ?", first.Id).
		Update("lease_until", time.Now().Add(-time.Minute).Unix()).Error)

	second, ok, err := ClaimCommunityBotMessage("prox", "room-1", "msg-2", "command", "worker-b", time.Minute)
	require.NoError(t, err)
	require.True(t, ok)
	require.Greater(t, second.FencingToken, first.FencingToken)

	completed, err := CompleteCommunityBotMessageClaim(first.Id, first.OwnerId, first.FencingToken)
	require.NoError(t, err)
	require.False(t, completed)

	completed, err = CompleteCommunityBotMessageClaim(second.Id, second.OwnerId, second.FencingToken)
	require.NoError(t, err)
	require.True(t, completed)
}
