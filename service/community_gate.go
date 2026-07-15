package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

type CommunityGateResult struct {
	Enabled           bool     `json:"enabled"`
	UserID            int      `json:"user_id"`
	Username          string   `json:"username"`
	ProviderSlug      string   `json:"provider_slug"`
	ProviderUserID    string   `json:"provider_user_id"`
	RoomID            string   `json:"room_id"`
	RequiredRoomIDs   []string `json:"required_room_ids"`
	MatchedRoomIDs    []string `json:"matched_room_ids"`
	MissingRoomIDs    []string `json:"missing_room_ids"`
	RoomMatchMode     string   `json:"room_match_mode"`
	Compliant         bool     `json:"compliant"`
	HasOAuthBinding   bool     `json:"has_oauth_binding"`
	HasRoomMembership bool     `json:"has_room_membership"`
	ReasonCode        string   `json:"reason_code"`
	Reason            string   `json:"reason"`
	DeniedMessage     string   `json:"denied_message"`
	CheckedAt         int64    `json:"checked_at"`
}

type CommunityGateScanUserResult struct {
	UserID         int    `json:"user_id"`
	Username       string `json:"username"`
	ProviderUserID string `json:"provider_user_id"`
	Compliant      bool   `json:"compliant"`
	ReasonCode     string `json:"reason_code"`
	Reason         string `json:"reason"`
	TokensEligible int    `json:"tokens_eligible"`
	TokensDisabled int    `json:"tokens_disabled"`
	RestoreCount   int    `json:"restore_count"`
}

type CommunityGateScanResult struct {
	DryRun         bool                          `json:"dry_run"`
	ScannedUsers   int                           `json:"scanned_users"`
	CompliantUsers int                           `json:"compliant_users"`
	BlockedUsers   int                           `json:"blocked_users"`
	ErrorUsers     int                           `json:"error_users"`
	TokensEligible int                           `json:"tokens_eligible"`
	TokensDisabled int                           `json:"tokens_disabled"`
	Users          []CommunityGateScanUserResult `json:"users"`
}

type communityGateCachedResult struct {
	result    *CommunityGateResult
	expiresAt time.Time
	signature string
}

type communityGateRoomMemberCache struct {
	roomID    string
	members   map[string]struct{}
	total     int
	scannedAt time.Time
	expiresAt time.Time
}

type communityGateRuntimeCache struct {
	sync.RWMutex
	results map[int]communityGateCachedResult
	rooms   map[string]*communityGateRoomMemberCache
}

var communityGateCache = communityGateRuntimeCache{
	results: map[int]communityGateCachedResult{},
	rooms:   map[string]*communityGateRoomMemberCache{},
}

type communityGateRoomMember struct {
	ID     string                 `json:"id"`
	UserID string                 `json:"userId"`
	User   *communityGateRoomUser `json:"user"`
}

type communityGateRoomUser struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Name     string `json:"name"`
	IsBot    bool   `json:"isBot"`
}

func communityGateEffectiveConfig() (*operation_setting.CommunityGateSetting, string, string, string, []string, string, string) {
	gate := operation_setting.GetCommunityGateSetting()
	bot := operation_setting.GetCommunityBotSetting()
	providerSlug := strings.TrimSpace(gate.ProviderSlug)
	if providerSlug == "" {
		providerSlug = strings.TrimSpace(bot.ProviderSlug)
	}
	if providerSlug == "" {
		providerSlug = "dc.hhhl.cc"
	}
	communityHost := strings.TrimRight(strings.TrimSpace(gate.CommunityHost), "/")
	if communityHost == "" {
		communityHost = strings.TrimRight(strings.TrimSpace(bot.CommunityHost), "/")
	}
	primaryRoomID, roomIDs, roomMatchMode := communityGateResolveRooms(gate, bot)
	botToken := strings.TrimSpace(bot.BotToken)
	return gate, providerSlug, communityHost, primaryRoomID, roomIDs, roomMatchMode, botToken
}

func communityGateNormalizeRoomIDs(values []string) []string {
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		roomID := strings.TrimSpace(value)
		if roomID == "" {
			continue
		}
		if _, ok := seen[roomID]; ok {
			continue
		}
		seen[roomID] = struct{}{}
		out = append(out, roomID)
	}
	return out
}

func communityGateContainsRoom(values []string, roomID string) bool {
	roomID = strings.TrimSpace(roomID)
	if roomID == "" {
		return false
	}
	for _, value := range values {
		if strings.TrimSpace(value) == roomID {
			return true
		}
	}
	return false
}

func communityGateResolveRooms(gate *operation_setting.CommunityGateSetting, bot *operation_setting.CommunityBotSetting) (string, []string, string) {
	primaryRoomID := strings.TrimSpace(gate.RoomID)
	roomIDs := communityGateNormalizeRoomIDs(gate.RoomIDs)
	if primaryRoomID != "" && !communityGateContainsRoom(roomIDs, primaryRoomID) {
		roomIDs = append([]string{primaryRoomID}, roomIDs...)
	}
	if len(roomIDs) == 0 {
		fallbackRoomID := strings.TrimSpace(bot.RoomID)
		if fallbackRoomID != "" {
			roomIDs = []string{fallbackRoomID}
			if primaryRoomID == "" {
				primaryRoomID = fallbackRoomID
			}
		}
	}
	if primaryRoomID == "" && len(roomIDs) > 0 {
		primaryRoomID = roomIDs[0]
	}
	roomMatchMode := strings.TrimSpace(strings.ToLower(gate.RoomMatchMode))
	if roomMatchMode != "all_of" {
		roomMatchMode = "any_of"
	}
	return primaryRoomID, roomIDs, roomMatchMode
}

func communityGateSignature(cfg *operation_setting.CommunityGateSetting, providerSlug, communityHost, primaryRoomID string, roomIDs []string, roomMatchMode string) string {
	return fmt.Sprintf(
		"%t|%s|%s|%s|%s|%s|%t|%t",
		cfg.Enabled,
		providerSlug,
		communityHost,
		primaryRoomID,
		strings.Join(roomIDs, ","),
		roomMatchMode,
		cfg.RequireOAuthBinding,
		cfg.RequireRoomMembership,
	)
}

func communityGateTTL(cfg *operation_setting.CommunityGateSetting) time.Duration {
	seconds := cfg.MemberCacheTTLSeconds
	if seconds <= 0 {
		seconds = 600
	}
	if seconds < 60 {
		seconds = 60
	}
	return time.Duration(seconds) * time.Second
}

func CommunityGateInvalidateUser(userID int) {
	communityGateCache.Lock()
	defer communityGateCache.Unlock()
	delete(communityGateCache.results, userID)
}

func CommunityGateInvalidateMembership(userID int, roomID string) {
	communityGateCache.Lock()
	defer communityGateCache.Unlock()
	if userID > 0 {
		delete(communityGateCache.results, userID)
	}
	roomID = strings.TrimSpace(roomID)
	if roomID == "" {
		return
	}
	for cacheKey, entry := range communityGateCache.rooms {
		if entry != nil && strings.TrimSpace(entry.roomID) == roomID {
			delete(communityGateCache.rooms, cacheKey)
		}
	}
}

func EvaluateCommunityGate(ctx context.Context, userID int, refresh bool) (*CommunityGateResult, error) {
	cfg, providerSlug, communityHost, primaryRoomID, roomIDs, roomMatchMode, botToken := communityGateEffectiveConfig()
	now := time.Now()
	sig := communityGateSignature(cfg, providerSlug, communityHost, primaryRoomID, roomIDs, roomMatchMode)
	if !refresh {
		communityGateCache.RLock()
		cached, ok := communityGateCache.results[userID]
		communityGateCache.RUnlock()
		if ok && cached.signature == sig && now.Before(cached.expiresAt) && cached.result != nil {
			copyResult := *cached.result
			return &copyResult, nil
		}
	}

	gateDecision := &CommunityGateResult{
		Enabled:         cfg.Enabled,
		UserID:          userID,
		ProviderSlug:    providerSlug,
		RoomID:          primaryRoomID,
		RequiredRoomIDs: append([]string(nil), roomIDs...),
		MatchedRoomIDs:  []string{},
		MissingRoomIDs:  []string{},
		RoomMatchMode:   roomMatchMode,
		DeniedMessage:   cfg.DeniedMessage,
		CheckedAt:       now.Unix(),
	}
	if strings.TrimSpace(gateDecision.DeniedMessage) == "" {
		gateDecision.DeniedMessage = "请先使用 dc.hhhl.cc 社区授权登录，并加入本站社区群聊后再使用 API Key。"
	}
	if !cfg.Enabled {
		gateDecision.Compliant = true
		gateDecision.ReasonCode = "disabled"
		gateDecision.Reason = "community gate disabled"
		return cacheCommunityGateResult(ctx, userID, sig, gateDecision, communityGateTTL(cfg)), nil
	}
	if userID <= 0 {
		gateDecision.ReasonCode = "invalid_user"
		gateDecision.Reason = "invalid user id"
		return gateDecision, nil
	}
	user, err := model.GetUserById(userID, false)
	if err != nil {
		return gateDecision, err
	}
	gateDecision.Username = user.Username
	if cfg.AllowAdminBypass && user.Role >= common.RoleAdminUser {
		gateDecision.Compliant = true
		gateDecision.ReasonCode = "admin_bypass"
		gateDecision.Reason = "administrator bypass"
		return cacheCommunityGateResult(ctx, userID, sig, gateDecision, communityGateTTL(cfg)), nil
	}
	if cfg.RequireOAuthBinding {
		_, providerUserID, err := model.GetCommunityGateBinding(providerSlug, userID)
		if err != nil || strings.TrimSpace(providerUserID) == "" {
			gateDecision.Compliant = false
			gateDecision.HasOAuthBinding = false
			gateDecision.ReasonCode = "missing_oauth_binding"
			gateDecision.Reason = "missing required dc.hhhl.cc OAuth binding"
			_ = auditCommunityGateResult(ctx, cfg, gateDecision)
			return cacheCommunityGateResult(ctx, userID, sig, gateDecision, communityGateTTL(cfg)), nil
		}
		gateDecision.ProviderUserID = strings.TrimSpace(providerUserID)
		gateDecision.HasOAuthBinding = true
	} else {
		gateDecision.HasOAuthBinding = true
	}
	if cfg.RequireRoomMembership {
		if len(roomIDs) == 0 {
			gateDecision.ReasonCode = "room_not_configured"
			gateDecision.Reason = "community gate room ids are not configured"
			_ = auditCommunityGateResult(ctx, cfg, gateDecision)
			return cacheCommunityGateResult(ctx, userID, sig, gateDecision, communityGateTTL(cfg)), nil
		}
		if botToken == "" {
			return gateDecision, errors.New("community bot token is not configured")
		}
		for _, requiredRoomID := range roomIDs {
			ok, err := communityGateHasRoomMember(ctx, communityHost, botToken, requiredRoomID, gateDecision.ProviderUserID, cfg.MemberScanLimit, communityGateTTL(cfg), refresh)
			if err != nil {
				return gateDecision, err
			}
			if ok {
				gateDecision.MatchedRoomIDs = append(gateDecision.MatchedRoomIDs, requiredRoomID)
			} else {
				gateDecision.MissingRoomIDs = append(gateDecision.MissingRoomIDs, requiredRoomID)
			}
		}
		if roomMatchMode == "all_of" {
			gateDecision.HasRoomMembership = len(gateDecision.RequiredRoomIDs) > 0 && len(gateDecision.MissingRoomIDs) == 0
		} else {
			gateDecision.HasRoomMembership = len(gateDecision.MatchedRoomIDs) > 0
		}
		if !gateDecision.HasRoomMembership {
			gateDecision.Compliant = false
			if roomMatchMode == "all_of" {
				gateDecision.ReasonCode = "missing_required_rooms"
			} else {
				gateDecision.ReasonCode = "not_in_any_required_room"
			}
			gateDecision.Reason = communityGateRoomRequirementReason(roomMatchMode, gateDecision.MatchedRoomIDs, gateDecision.MissingRoomIDs)
			_ = auditCommunityGateResult(ctx, cfg, gateDecision)
			return cacheCommunityGateResult(ctx, userID, sig, gateDecision, communityGateTTL(cfg)), nil
		}
	} else {
		gateDecision.HasRoomMembership = true
	}
	gateDecision.Compliant = true
	gateDecision.ReasonCode = "ok"
	if cfg.RequireRoomMembership {
		gateDecision.Reason = communityGateRoomRequirementReason(roomMatchMode, gateDecision.MatchedRoomIDs, gateDecision.MissingRoomIDs)
	} else {
		gateDecision.Reason = "community gate passed"
	}
	_ = auditCommunityGateResult(ctx, cfg, gateDecision)
	return cacheCommunityGateResult(ctx, userID, sig, gateDecision, communityGateTTL(cfg)), nil
}

func communityGateRoomRequirementReason(roomMatchMode string, matchedRoomIDs []string, missingRoomIDs []string) string {
	matched := strings.Join(matchedRoomIDs, ",")
	missing := strings.Join(missingRoomIDs, ",")
	if roomMatchMode == "all_of" {
		if missing == "" {
			return "required community room membership satisfied for all configured rooms"
		}
		return fmt.Sprintf("missing required community rooms: %s (matched: %s)", missing, matched)
	}
	if matched == "" {
		return fmt.Sprintf("not in any required community room (required: %s)", missing)
	}
	return fmt.Sprintf("matched required community rooms: %s", matched)
}

func cacheCommunityGateResult(ctx context.Context, userID int, sig string, gateDecision *CommunityGateResult, ttl time.Duration) *CommunityGateResult {
	if userID <= 0 || gateDecision == nil {
		return gateDecision
	}
	if !accessControlPersistenceEnabled(ctx) {
		return gateDecision
	}
	copyResult := *gateDecision
	communityGateCache.Lock()
	communityGateCache.results[userID] = communityGateCachedResult{result: &copyResult, signature: sig, expiresAt: time.Now().Add(ttl)}
	communityGateCache.Unlock()
	return gateDecision
}

func auditCommunityGateResult(ctx context.Context, cfg *operation_setting.CommunityGateSetting, gateDecision *CommunityGateResult) error {
	if !accessControlPersistenceEnabled(ctx) || cfg == nil || !cfg.AuditEnabled || gateDecision == nil || gateDecision.UserID <= 0 {
		return nil
	}
	return model.UpsertCommunityGateAudit(&model.CommunityGateAudit{
		UserId:            gateDecision.UserID,
		Username:          gateDecision.Username,
		ProviderSlug:      gateDecision.ProviderSlug,
		ProviderUserId:    gateDecision.ProviderUserID,
		RoomId:            gateDecision.RoomID,
		Compliant:         gateDecision.Compliant,
		HasOAuthBinding:   gateDecision.HasOAuthBinding,
		HasRoomMembership: gateDecision.HasRoomMembership,
		ReasonCode:        gateDecision.ReasonCode,
		Reason:            gateDecision.Reason,
		CheckedAt:         gateDecision.CheckedAt,
	})
}

func communityGateDeniedMessage(gateDecision *CommunityGateResult) string {
	message := ""
	if gateDecision != nil {
		message = strings.TrimSpace(gateDecision.DeniedMessage)
	}
	if message == "" {
		message = "请先使用 dc.hhhl.cc 社区授权登录，并加入本站社区群聊后再使用 API Key。"
	}
	return message
}

func CommunityGateCheckUserAction(ctx context.Context, userID int, refresh bool) (*CommunityGateResult, error) {
	cfg := operation_setting.GetCommunityGateSetting()
	if cfg == nil || !cfg.Enabled {
		return &CommunityGateResult{Enabled: false, UserID: userID, Compliant: true, ReasonCode: "disabled", Reason: "community gate disabled", CheckedAt: time.Now().Unix()}, nil
	}
	gateDecision, err := EvaluateCommunityGate(ctx, userID, refresh)
	if err != nil {
		return gateDecision, fmt.Errorf("社区准入检查暂时失败：%w", err)
	}
	if gateDecision == nil {
		return nil, errors.New("社区准入检查无结果")
	}
	if !gateDecision.Compliant {
		return gateDecision, errors.New(communityGateDeniedMessage(gateDecision))
	}
	return gateDecision, nil
}

func CommunityGateCheckToken(ctx context.Context, userID int) error {
	cfg := operation_setting.GetCommunityGateSetting()
	if cfg == nil || !cfg.Enabled || !cfg.BlockTokenWhenNotCompliant {
		return nil
	}
	_, err := CommunityGateCheckUserAction(ctx, userID, false)
	return err
}

func communityGateHasRoomMember(ctx context.Context, communityHost, botToken, roomID, providerUserID string, scanLimit int, ttl time.Duration, refresh bool) (bool, error) {
	providerUserID = strings.TrimSpace(providerUserID)
	if providerUserID == "" {
		return false, nil
	}
	entry, err := communityGateGetRoomMembers(ctx, communityHost, botToken, roomID, scanLimit, ttl, refresh)
	if err != nil {
		return false, err
	}
	if communityGateRoomMemberHit(entry, providerUserID) {
		return true, nil
	}
	if refresh {
		return false, nil
	}
	entry, err = communityGateGetRoomMembers(ctx, communityHost, botToken, roomID, scanLimit, ttl, true)
	if err != nil {
		return false, err
	}
	return communityGateRoomMemberHit(entry, providerUserID), nil
}

func communityGateRoomMemberHit(entry *communityGateRoomMemberCache, providerUserID string) bool {
	if entry == nil {
		return false
	}
	if _, ok := entry.members[providerUserID]; ok {
		return true
	}
	_, ok := entry.members[strings.ToLower(providerUserID)]
	return ok
}

func communityGateGetRoomMembers(ctx context.Context, communityHost, botToken, roomID string, scanLimit int, ttl time.Duration, refresh bool) (*communityGateRoomMemberCache, error) {
	cacheKey := strings.TrimRight(communityHost, "/") + "|" + roomID
	now := time.Now()
	communityGateCache.RLock()
	cached := communityGateCache.rooms[cacheKey]
	communityGateCache.RUnlock()
	if !refresh && cached != nil && now.Before(cached.expiresAt) {
		return cached, nil
	}
	entry, err := communityGateFetchRoomMembers(ctx, communityHost, botToken, roomID, scanLimit)
	if err != nil {
		if cached != nil && now.Sub(cached.scannedAt) < 24*time.Hour {
			common.SysLog("[CommunityGate] using stale room member cache after refresh failure: " + err.Error())
			return cached, nil
		}
		return nil, err
	}
	entry.expiresAt = now.Add(ttl)
	communityGateCache.Lock()
	communityGateCache.rooms[cacheKey] = entry
	communityGateCache.Unlock()
	return entry, nil
}

func communityGateFetchRoomMembers(ctx context.Context, communityHost, botToken, roomID string, scanLimit int) (*communityGateRoomMemberCache, error) {
	if scanLimit <= 0 {
		scanLimit = 3000
	}
	if scanLimit < 100 {
		scanLimit = 100
	}
	members := map[string]struct{}{}
	untilID := ""
	scanned := 0
	for scanned < scanLimit {
		limit := 100
		if remaining := scanLimit - scanned; remaining < limit {
			limit = remaining
		}
		payload := map[string]any{"roomId": roomID, "limit": limit}
		if untilID != "" {
			payload["untilId"] = untilID
		}
		var page []communityGateRoomMember
		if err := communityGet(ctx, communityHost, botToken, "chat/rooms/members", payload, &page); err != nil {
			if !isCommunityMethodNotAllowed(err) {
				return nil, err
			}
			if err := communityPost(ctx, communityHost, botToken, "chat/rooms/members", payload, &page); err != nil {
				return nil, err
			}
		}
		if len(page) == 0 {
			break
		}
		for _, member := range page {
			untilID = strings.TrimSpace(member.ID)
			if member.UserID != "" {
				members[member.UserID] = struct{}{}
				members[strings.ToLower(member.UserID)] = struct{}{}
			}
			if member.User != nil {
				if member.User.ID != "" {
					members[member.User.ID] = struct{}{}
					members[strings.ToLower(member.User.ID)] = struct{}{}
				}
				if member.User.Username != "" {
					members[member.User.Username] = struct{}{}
					members[strings.ToLower(member.User.Username)] = struct{}{}
				}
				if member.User.Name != "" {
					members[member.User.Name] = struct{}{}
					members[strings.ToLower(member.User.Name)] = struct{}{}
				}
			}
		}
		scanned += len(page)
		if len(page) < limit || untilID == "" {
			break
		}
	}
	return &communityGateRoomMemberCache{roomID: roomID, members: members, total: scanned, scannedAt: time.Now()}, nil
}

func CommunityGateScanAndFreeze(ctx context.Context, dryRun bool, limit int) (*CommunityGateScanResult, error) {
	cfg := operation_setting.GetCommunityGateSetting()
	result := &CommunityGateScanResult{DryRun: dryRun, Users: make([]CommunityGateScanUserResult, 0)}
	users, err := model.ListCommunityGateUsers(limit)
	if err != nil {
		return result, err
	}
	freezeAllowed := strings.TrimSpace(cfg.TokenDisableMode) != "off"
	for i, user := range users {
		gate, err := EvaluateCommunityGate(ctx, user.Id, i == 0)
		result.ScannedUsers++
		item := CommunityGateScanUserResult{UserID: user.Id, Username: user.Username}
		if gate != nil {
			item.ProviderUserID = gate.ProviderUserID
			item.Compliant = gate.Compliant
			item.ReasonCode = gate.ReasonCode
			item.Reason = gate.Reason
		}
		if err != nil {
			result.ErrorUsers++
			item.ReasonCode = "check_error"
			item.Reason = err.Error()
			result.Users = appendScanUser(result.Users, item)
			continue
		}
		if gate != nil && gate.Compliant {
			result.CompliantUsers++
			continue
		}
		result.BlockedUsers++
		if freezeAllowed {
			stats, err := model.FreezeUserTokensForCommunityGate(user.Id, item.Reason, dryRun)
			if err != nil {
				result.ErrorUsers++
				item.ReasonCode = "freeze_error"
				item.Reason = err.Error()
			} else if stats != nil {
				item.TokensEligible = stats.Eligible
				item.TokensDisabled = stats.Disabled
				result.TokensEligible += stats.Eligible
				result.TokensDisabled += stats.Disabled
			}
		}
		result.Users = appendScanUser(result.Users, item)
	}
	return result, nil
}

func appendScanUser(items []CommunityGateScanUserResult, item CommunityGateScanUserResult) []CommunityGateScanUserResult {
	if len(items) >= 300 {
		return items
	}
	return append(items, item)
}

func RestoreCommunityGateUserTokens(userID int) (*model.CommunityGateTokenFreezeStats, error) {
	CommunityGateInvalidateUser(userID)
	return model.RestoreCommunityGateUserTokens(userID)
}

func GetCommunityGateUserStatus(ctx context.Context, userID int, refresh bool) (map[string]any, error) {
	gateDecision, err := EvaluateCommunityGate(ctx, userID, refresh)
	if err != nil {
		return nil, err
	}
	activeFreezes, freezeErr := model.CountActiveCommunityGateFreezes(userID)
	if freezeErr != nil {
		return nil, freezeErr
	}
	joinURL := ""
	communityHost := ""
	roomID := ""
	providerSlug := ""
	if gateDecision != nil {
		roomID = strings.TrimSpace(gateDecision.RoomID)
		providerSlug = strings.TrimSpace(gateDecision.ProviderSlug)
	}
	_, providerSlugEffective, communityHostEffective, primaryRoomIDEffective, roomIDsEffective, roomMatchModeEffective, _ := communityGateEffectiveConfig()
	if providerSlug == "" {
		providerSlug = providerSlugEffective
	}
	communityHost = strings.TrimRight(strings.TrimSpace(communityHostEffective), "/")
	if roomID == "" {
		roomID = strings.TrimSpace(primaryRoomIDEffective)
	}
	requiredRoomIDs := append([]string(nil), roomIDsEffective...)
	roomMatchMode := roomMatchModeEffective
	matchedRoomIDs := []string{}
	missingRoomIDs := []string{}
	if gateDecision != nil {
		if len(gateDecision.RequiredRoomIDs) > 0 {
			requiredRoomIDs = append([]string(nil), gateDecision.RequiredRoomIDs...)
		}
		if strings.TrimSpace(gateDecision.RoomMatchMode) != "" {
			roomMatchMode = gateDecision.RoomMatchMode
		}
		matchedRoomIDs = append([]string(nil), gateDecision.MatchedRoomIDs...)
		missingRoomIDs = append([]string(nil), gateDecision.MissingRoomIDs...)
	}
	joinTargetRoomID := roomID
	if joinTargetRoomID == "" && len(requiredRoomIDs) > 0 {
		joinTargetRoomID = requiredRoomIDs[0]
	}
	if len(missingRoomIDs) > 0 {
		joinTargetRoomID = missingRoomIDs[0]
	}
	if communityHost != "" && joinTargetRoomID != "" {
		joinURL = communityHost + "/chat/room/" + joinTargetRoomID
	}
	joinTargets := make([]map[string]any, 0, len(requiredRoomIDs))
	for _, requiredRoomID := range requiredRoomIDs {
		joinTargets = append(joinTargets, map[string]any{
			"room_id":  requiredRoomID,
			"join_url": communityHost + "/chat/room/" + requiredRoomID,
			"joined":   communityGateContainsRoom(matchedRoomIDs, requiredRoomID),
		})
	}
	return map[string]any{
		"gate":                   gateDecision,
		"compliant":              gateDecision != nil && gateDecision.Compliant,
		"has_active_frozen_keys": activeFreezes > 0,
		"active_frozen_keys":     activeFreezes,
		"can_restore":            gateDecision != nil && gateDecision.Compliant && activeFreezes > 0,
		"join_url":               joinURL,
		"join_targets":           joinTargets,
		"bind_url":               "/profile?tab=bindings",
		"provider_slug":          providerSlug,
		"community_host":         communityHost,
		"room_id":                roomID,
		"room_ids":               requiredRoomIDs,
		"room_match_mode":        roomMatchMode,
		"matched_room_ids":       matchedRoomIDs,
		"missing_room_ids":       missingRoomIDs,
		"denied_message":         communityGateDeniedMessage(gateDecision),
	}, nil
}

func RestoreCommunityGateUserTokensIfCompliant(ctx context.Context, userID int) (*model.CommunityGateTokenFreezeStats, *CommunityGateResult, error) {
	gate, err := CommunityGateCheckUserAction(ctx, userID, true)
	if err != nil {
		return nil, gate, err
	}
	stats, err := RestoreCommunityGateUserTokens(userID)
	return stats, gate, err
}

func GetCommunityGateAdminStatus() (map[string]any, error) {
	cfg, providerSlug, communityHost, primaryRoomID, roomIDs, roomMatchMode, botToken := communityGateEffectiveConfig()
	audits, err := model.ListCommunityGateAudits(100)
	if err != nil {
		return nil, err
	}
	communityGateCache.RLock()
	roomCacheCount := len(communityGateCache.rooms)
	userCacheCount := len(communityGateCache.results)
	communityGateCache.RUnlock()
	return map[string]any{
		"enabled":                        cfg.Enabled,
		"provider_slug":                  providerSlug,
		"community_host":                 communityHost,
		"room_id":                        primaryRoomID,
		"room_ids":                       roomIDs,
		"room_match_mode":                roomMatchMode,
		"require_oauth_binding":          cfg.RequireOAuthBinding,
		"require_room_membership":        cfg.RequireRoomMembership,
		"only_allow_provider_register":   cfg.OnlyAllowProviderRegister,
		"disable_password_register":      cfg.DisablePasswordRegister,
		"disable_builtin_oauth_register": cfg.DisableBuiltinOAuthRegister,
		"block_token_when_not_compliant": cfg.BlockTokenWhenNotCompliant,
		"allow_admin_bypass":             cfg.AllowAdminBypass,
		"member_cache_ttl_seconds":       cfg.MemberCacheTTLSeconds,
		"member_scan_limit":              cfg.MemberScanLimit,
		"token_disable_mode":             cfg.TokenDisableMode,
		"denied_message":                 cfg.DeniedMessage,
		"audit_enabled":                  cfg.AuditEnabled,
		"bot_token_configured":           botToken != "",
		"runtime_cache":                  map[string]any{"rooms": roomCacheCount, "users": userCacheCount},
		"recent_audits":                  audits,
	}, nil
}
