package service

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

const (
	communityBotUserAgent       = "NewAPI-CommunityBot/1.0"
	communityOAuthStateNS       = "new-api-community-bot-oauth"
	communityBotCommandClaimTTL = 2 * time.Minute
	communityBotClaimRetention  = 90 * 24 * time.Hour
)

var (
	communityBotTaskOnce sync.Once
	communityBotTaskStop = make(chan struct{})
	communityBotWorkerID = fmt.Sprintf(
		"%s:%d:%s",
		firstNonEmpty(os.Getenv("HOSTNAME"), "new-api"),
		os.Getpid(),
		common.GetRandomString(8),
	)
)

type CommunityBotStatus struct {
	Enabled       bool   `json:"enabled"`
	Configured    bool   `json:"configured"`
	Authorized    bool   `json:"authorized"`
	CommunityHost string `json:"community_host"`
	RoomID        string `json:"room_id"`
	BotUserID     string `json:"bot_user_id"`
	BotUsername   string `json:"bot_username"`
	LastMessageID string `json:"last_message_id"`
	LastScannedAt int64  `json:"last_scanned_at"`
}

type CommunityOAuthStart struct {
	AuthorizeURL string `json:"authorize_url"`
	CallbackURL  string `json:"callback_url"`
}

type communityOAuthTokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	Scope        string `json:"scope"`
	Error        string `json:"error"`
	ErrorDesc    string `json:"error_description"`
}

type communityUser struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Name     string `json:"name"`
	Host     string `json:"host"`
	IsBot    bool   `json:"isBot"`
}

type communityInviteResult struct {
	Status  string
	Created bool
	Notify  bool
}

type communityRoom struct {
	ID       string        `json:"id"`
	Name     string        `json:"name"`
	IsJoined bool          `json:"isJoined"`
	OwnerID  string        `json:"ownerId"`
	Owner    communityUser `json:"owner"`
}

type communityMessage struct {
	ID         string         `json:"id"`
	Text       string         `json:"text"`
	CreatedAt  string         `json:"createdAt"`
	FromUserID string         `json:"fromUserId"`
	FromUser   *communityUser `json:"fromUser"`
	User       *communityUser `json:"user"`
	ToRoomID   string         `json:"toRoomId"`
	RoomID     string         `json:"roomId"`
	IsDeleted  bool           `json:"isDeleted"`
}

type communityAdapterPlanResponse struct {
	OK     bool   `json:"ok"`
	Error  string `json:"error"`
	Result struct {
		Reply       string `json:"reply"`
		Risk        string `json:"risk"`
		Notes       string `json:"notes"`
		GameHandled bool   `json:"game_handled"`
	} `json:"result"`
}

type CommunityBotScanResult struct {
	ScannedMessages int `json:"scanned_messages"`
	RewardedUsers   int `json:"rewarded_users"`
	SkippedUsers    int `json:"skipped_users"`
	CommandHits     int `json:"command_hits"`
}

func GetCommunityBotStatus() CommunityBotStatus {
	cfg := operation_setting.GetCommunityBotSetting()
	state, _ := model.GetCommunityBotRoomState(strings.TrimSpace(cfg.RoomID))
	return CommunityBotStatus{
		Enabled:       cfg.Enabled,
		Configured:    strings.TrimSpace(cfg.CommunityHost) != "" && strings.TrimSpace(cfg.RoomID) != "",
		Authorized:    strings.TrimSpace(cfg.BotToken) != "",
		CommunityHost: strings.TrimRight(strings.TrimSpace(cfg.CommunityHost), "/"),
		RoomID:        strings.TrimSpace(cfg.RoomID),
		BotUserID:     strings.TrimSpace(cfg.BotUserID),
		BotUsername:   strings.TrimSpace(cfg.BotUsername),
		LastMessageID: state.LastMessageId,
		LastScannedAt: state.LastScannedAt,
	}
}

func BuildCommunityBotOAuthStart(serverAddress string) (*CommunityOAuthStart, error) {
	cfg := operation_setting.GetCommunityBotSetting()
	host := strings.TrimRight(strings.TrimSpace(cfg.CommunityHost), "/")
	if host == "" {
		return nil, errors.New("community host is empty")
	}
	callbackURL := strings.TrimSpace(cfg.OAuthCallbackURL)
	if callbackURL == "" {
		serverAddress = strings.TrimRight(strings.TrimSpace(serverAddress), "/")
		if serverAddress != "" {
			callbackURL = serverAddress + "/oauth/callback"
		}
	}
	if callbackURL == "" {
		return nil, errors.New("server address is empty")
	}
	state := randomURLSafe(24) + "." + common.HmacSha256(communityOAuthStateNS+":"+callbackURL, common.SessionSecret)
	if err := model.UpdateOptionsBulk(map[string]string{
		"community_bot_setting.oauth_verifier_secret": "miauth",
		"community_bot_setting.oauth_state_secret":    state,
	}); err != nil {
		return nil, err
	}

	callbackWithState, err := url.Parse(callbackURL)
	if err != nil {
		return nil, err
	}
	cbQuery := callbackWithState.Query()
	cbQuery.Set("state", state)
	callbackWithState.RawQuery = cbQuery.Encode()

	sessionID := randomURLSafe(24)
	u, err := url.Parse(host + "/miauth/" + url.PathEscape(sessionID))
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("name", "NewAPI社区管家")
	q.Set("callback", callbackWithState.String())
	q.Set("permission", "read:account,read:chat,write:chat")
	u.RawQuery = q.Encode()
	return &CommunityOAuthStart{AuthorizeURL: u.String(), CallbackURL: callbackURL}, nil
}

func CompleteCommunityBotOAuth(ctx context.Context, code string, state string, redirectURI string) error {
	cfg := operation_setting.GetCommunityBotSetting()
	if code == "" || state == "" {
		return errors.New("missing MiAuth session or state")
	}
	if state != strings.TrimSpace(cfg.OAuthStateSecret) {
		return errors.New("invalid community bot oauth state")
	}
	host := strings.TrimRight(strings.TrimSpace(cfg.CommunityHost), "/")
	if host == "" || strings.TrimSpace(cfg.OAuthVerifierSecret) == "" {
		return errors.New("community bot MiAuth is not initialized")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, host+"/api/miauth/"+url.PathEscape(code)+"/check", strings.NewReader("{}"))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", communityBotUserAgent)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("community miauth check status=%d body=%s", resp.StatusCode, truncateCommunityLog(string(body), 500))
	}
	var tokenResp struct {
		OK    bool          `json:"ok"`
		Token string        `json:"token"`
		User  communityUser `json:"user"`
		Error string        `json:"error"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return err
	}
	if !tokenResp.OK || tokenResp.Token == "" {
		return fmt.Errorf("community miauth check failed: %s", tokenResp.Error)
	}
	me := &tokenResp.User
	if me.ID == "" {
		var err error
		me, err = CommunityAPIGetMe(ctx, host, tokenResp.Token)
		if err != nil {
			return err
		}
	}
	return model.UpdateOptionsBulk(map[string]string{
		"community_bot_setting.bot_token":             tokenResp.Token,
		"community_bot_setting.bot_user_id":           me.ID,
		"community_bot_setting.bot_username":          me.Username,
		"community_bot_setting.oauth_verifier_secret": "",
		"community_bot_setting.oauth_state_secret":    "",
	})
}

func SaveCommunityBotToken(ctx context.Context, token string) (*communityUser, error) {
	cfg := operation_setting.GetCommunityBotSetting()
	host := strings.TrimRight(strings.TrimSpace(cfg.CommunityHost), "/")
	if host == "" {
		return nil, errors.New("community host is empty")
	}
	if strings.TrimSpace(token) == "" {
		return nil, errors.New("community bot token is empty")
	}
	me, err := CommunityAPIGetMe(ctx, host, token)
	if err != nil {
		return nil, err
	}
	if err := model.UpdateOptionsBulk(map[string]string{
		"community_bot_setting.bot_token":             strings.TrimSpace(token),
		"community_bot_setting.bot_user_id":           me.ID,
		"community_bot_setting.bot_username":          me.Username,
		"community_bot_setting.oauth_verifier_secret": "",
		"community_bot_setting.oauth_state_secret":    "",
	}); err != nil {
		return nil, err
	}
	return me, nil
}

func CommunityAPIGetMe(ctx context.Context, host string, token string) (*communityUser, error) {
	var user communityUser
	if err := communityPost(ctx, host, token, "i", map[string]any{}, &user); err != nil {
		return nil, err
	}
	if user.ID == "" {
		return nil, errors.New("community /api/i returned empty user")
	}
	return &user, nil
}

func CommunitySendRoomMessage(ctx context.Context, text string) error {
	cfg := operation_setting.GetCommunityBotSetting()
	return communitySendRoomMessageWithConfig(ctx, cfg, strings.TrimSpace(cfg.RoomID), text)
}

func communitySendRoomMessageWithConfig(ctx context.Context, cfg *operation_setting.CommunityBotSetting, roomID string, text string) error {
	_, err := communitySendRoomMessageReturnID(ctx, cfg, roomID, text)
	return err
}

// communitySendRoomMessageReturnID 发送普通社区通知并返回新消息 ID。
// 普通通知尊重 NotificationEnabled；签到/验牌/游戏命令回复使用
// communitySendCommandRoomMessageReturnID，避免后台关闭通知后群命令“已处理但无回复”。
func communitySendRoomMessageReturnID(ctx context.Context, cfg *operation_setting.CommunityBotSetting, roomID string, text string) (string, error) {
	return communitySendRoomMessageReturnIDWithMode(ctx, cfg, roomID, text, true)
}

// communitySendCommandRoomMessageReturnID 发送命令回复并返回新消息 ID。
// 命令回复属于交互闭环，不受 NotificationEnabled 控制；否则用户发送“签到/验牌/菜单”会没有任何反馈。
func communitySendCommandRoomMessageReturnID(ctx context.Context, cfg *operation_setting.CommunityBotSetting, roomID string, text string) (string, error) {
	return communitySendRoomMessageReturnIDWithMode(ctx, cfg, roomID, text, false)
}

func communitySendRoomMessageReturnIDWithMode(ctx context.Context, cfg *operation_setting.CommunityBotSetting, roomID string, text string, requireNotificationEnabled bool) (string, error) {
	if cfg == nil {
		return "", errors.New("community bot setting is nil")
	}
	if requireNotificationEnabled && !cfg.NotificationEnabled {
		common.SysLog(fmt.Sprintf("[CommunityBot] notification skipped: notification disabled room_id=%s", roomID))
		return "", nil
	}
	if strings.TrimSpace(cfg.BotToken) == "" || strings.TrimSpace(roomID) == "" || strings.TrimSpace(text) == "" {
		return "", fmt.Errorf("community message send skipped: bot_token=%t room_id=%t text=%t", strings.TrimSpace(cfg.BotToken) != "", strings.TrimSpace(roomID) != "", strings.TrimSpace(text) != "")
	}
	var out struct {
		ID string `json:"id"`
	}
	if err := communityPost(ctx, cfg.CommunityHost, cfg.BotToken, "chat/messages/create-to-room", map[string]any{
		"toRoomId": roomID,
		"text":     text,
	}, &out); err != nil {
		return "", err
	}
	if strings.TrimSpace(out.ID) == "" {
		return "", errors.New("community message sent but response id is empty")
	}
	return out.ID, nil
}

// communityDeleteMessage 撤回一条社区房间消息（bot 自己的或用户的）。
func communityDeleteMessage(cfg *operation_setting.CommunityBotSetting, messageID string) {
	if cfg == nil || strings.TrimSpace(messageID) == "" || strings.TrimSpace(cfg.BotToken) == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := communityPost(ctx, cfg.CommunityHost, cfg.BotToken, "chat/messages/delete", map[string]any{
		"messageId": strings.TrimSpace(messageID),
	}, nil); err != nil {
		common.SysLog(fmt.Sprintf("[CommunityBot] burn delete failed message_id=%s err=%s", messageID, err.Error()))
	}
}

// communityScheduleBurn 阅后即焚：延迟撤回一组社区房间消息（bot 回复 + 用户命令）。
// 延迟由 cfg.CommandBurnAfterSeconds 控制，<=0 表示不撤回。
func communityScheduleBurn(cfg *operation_setting.CommunityBotSetting, messageIDs []string) {
	delay := cfg.CommandBurnAfterSeconds
	if delay <= 0 {
		return
	}
	ids := make([]string, 0, len(messageIDs))
	for _, id := range messageIDs {
		if strings.TrimSpace(id) != "" {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return
	}
	go func() {
		time.Sleep(time.Duration(delay) * time.Second)
		for _, id := range ids {
			communityDeleteMessage(cfg, id)
		}
	}()
}

func InviteCommunityUserIfEnabled(ctx context.Context, providerId int, providerSlug string, providerUserID string, username string) {
	cfg := operation_setting.GetCommunityBotSetting()
	if cfg == nil || !cfg.Enabled || !cfg.AutoInviteEnabled || !cfg.InviteOnOAuthLogin {
		return
	}
	if strings.TrimSpace(cfg.RoomID) == "" || strings.TrimSpace(cfg.BotToken) == "" {
		model.RecordLogEvent(0, model.LogTypeSystem, "community auto invite skipped: room or token empty", model.LogEventOptions{
			Category:       "community",
			Source:         "community",
			Action:         "invite_auto",
			Status:         "skipped",
			SiteId:         AgentSiteID(),
			RoomId:         strings.TrimSpace(cfg.RoomID),
			ExternalUserId: strings.TrimSpace(providerUserID),
			Other: map[string]interface{}{
				"community_invite": map[string]interface{}{
					"provider_slug": providerSlug,
					"username":      username,
					"reason":        "room_or_token_empty",
				},
			},
		})
		return
	}
	if slug := strings.TrimSpace(cfg.ProviderSlug); slug != "" && providerSlug != slug {
		model.RecordLogEvent(0, model.LogTypeSystem, "community auto invite skipped: provider mismatch", model.LogEventOptions{
			Category:       "community",
			Source:         "community",
			Action:         "invite_auto",
			Status:         "skipped",
			SiteId:         AgentSiteID(),
			RoomId:         strings.TrimSpace(cfg.RoomID),
			ExternalUserId: strings.TrimSpace(providerUserID),
			Other: map[string]interface{}{
				"community_invite": map[string]interface{}{
					"provider_slug": providerSlug,
					"username":      username,
					"reason":        "provider_mismatch",
				},
			},
		})
		return
	}
	resolvedUserID, resolvedUsername, err := ResolveCommunityUserID(ctx, cfg, providerUserID, username)
	if err != nil {
		common.SysLog(fmt.Sprintf("[CommunityBot] invite resolve failed provider_user_id=%s username=%s room_id=%s err=%s", providerUserID, username, cfg.RoomID, err.Error()))
		_, _, _ = model.UpsertCommunityBotInvite(providerId, firstNonEmpty(providerUserID, username), username, cfg.RoomID, "resolve_failed", err.Error())
		model.RecordLogEvent(0, model.LogTypeSystem, "community auto invite resolve failed", model.LogEventOptions{
			Category:       "community",
			Source:         "community",
			Action:         "invite_auto",
			Status:         "failed",
			SiteId:         AgentSiteID(),
			RoomId:         cfg.RoomID,
			ExternalUserId: strings.TrimSpace(providerUserID),
			Other: map[string]interface{}{
				"community_invite": map[string]interface{}{
					"provider_slug": providerSlug,
					"username":      username,
					"reason":        err.Error(),
				},
			},
		})
		return
	}
	result, err := communityInviteUser(ctx, cfg, resolvedUserID)
	if err != nil {
		common.SysLog(fmt.Sprintf("[CommunityBot] invite failed provider_user_id=%s resolved_user_id=%s room_id=%s err=%s", providerUserID, resolvedUserID, cfg.RoomID, err.Error()))
		_, _, _ = model.UpsertCommunityBotInvite(providerId, resolvedUserID, resolvedUsername, cfg.RoomID, "failed", err.Error())
		model.RecordLogEvent(0, model.LogTypeSystem, "community auto invite failed", model.LogEventOptions{
			Category:       "community",
			Source:         "community",
			Action:         "invite_auto",
			Status:         "failed",
			SiteId:         AgentSiteID(),
			RoomId:         cfg.RoomID,
			ExternalUserId: strings.TrimSpace(resolvedUserID),
			Other: map[string]interface{}{
				"community_invite": map[string]interface{}{
					"provider_slug":     providerSlug,
					"provider_user_id":  providerUserID,
					"resolved_user_id":  resolvedUserID,
					"resolved_username": resolvedUsername,
					"reason":            err.Error(),
				},
			},
		})
		return
	}
	invite, created, err := model.UpsertCommunityBotInvite(providerId, resolvedUserID, resolvedUsername, cfg.RoomID, result.Status, "")
	if err != nil {
		common.SysLog(fmt.Sprintf("[CommunityBot] invite state save failed provider_user_id=%s room_id=%s err=%s", resolvedUserID, cfg.RoomID, err.Error()))
	}
	logStatus := "success"
	if !result.Created {
		logStatus = "skipped"
	}
	model.RecordLogEvent(0, model.LogTypeSystem, "community auto invite ensured", model.LogEventOptions{
		Category:       "community",
		Source:         "community",
		Action:         "invite_auto",
		Status:         logStatus,
		SiteId:         AgentSiteID(),
		RoomId:         cfg.RoomID,
		ExternalUserId: strings.TrimSpace(resolvedUserID),
		RewardType:     "invite_auto",
		Tags:           []string{"community", result.Status},
		Other: map[string]interface{}{
			"community_invite": map[string]interface{}{
				"provider_slug":     providerSlug,
				"provider_user_id":  providerUserID,
				"resolved_user_id":  resolvedUserID,
				"resolved_username": resolvedUsername,
				"invite_status":     result.Status,
				"created":           result.Created,
				"notify":            result.Notify,
				"invite_id": func() int {
					if invite != nil {
						return invite.Id
					}
					return 0
				}(),
			},
		},
	})
	common.SysLog(fmt.Sprintf("[CommunityBot] invite ensured provider_user_id=%s resolved_user_id=%s username=%s room_id=%s status=%s created=%t", providerUserID, resolvedUserID, resolvedUsername, cfg.RoomID, result.Status, created))
	shouldNotify := cfg.NotificationEnabled && cfg.NotifyOnInvite && result.Notify && (created || (invite != nil && invite.NotifySentAt == 0))
	if shouldNotify {
		msg := renderCommunityTemplate(cfg.InviteNotificationTemplate, map[string]string{
			"username": resolvedUsername,
			"user_id":  resolvedUserID,
			"room_id":  cfg.RoomID,
		})
		if err := communitySendRoomMessageWithConfig(ctx, cfg, cfg.RoomID, msg); err == nil {
			_ = model.MarkCommunityBotInviteNotified(resolvedUserID, cfg.RoomID)
		}
	}
}

func ResolveCommunityUserID(ctx context.Context, cfg *operation_setting.CommunityBotSetting, providerUserID string, username string) (string, string, error) {
	providerUserID = strings.TrimSpace(providerUserID)
	username = strings.TrimSpace(username)
	candidates := make([]string, 0, 2)
	if providerUserID != "" {
		candidates = append(candidates, providerUserID)
	}
	if username != "" && username != providerUserID {
		candidates = append(candidates, username)
	}
	if len(candidates) == 0 {
		return "", "", errors.New("community user id and username are empty")
	}
	for _, candidate := range candidates {
		if isLikelyCommunityUserID(candidate) {
			return candidate, firstNonEmpty(username, candidate), nil
		}
	}
	for _, candidate := range candidates {
		user, err := communityLookupUser(ctx, cfg, candidate)
		if err == nil && user != nil && strings.TrimSpace(user.ID) != "" {
			return strings.TrimSpace(user.ID), firstNonEmpty(user.Username, username, candidate), nil
		}
	}
	return "", "", fmt.Errorf("failed to resolve community user id from provider_user_id=%s username=%s", providerUserID, username)
}

func communityLookupUser(ctx context.Context, cfg *operation_setting.CommunityBotSetting, username string) (*communityUser, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return nil, errors.New("community username is empty")
	}
	payloads := []map[string]any{
		{"username": username},
		{"username": username, "host": nil},
	}
	var lastErr error
	for _, payload := range payloads {
		var user communityUser
		if err := communityPost(ctx, cfg.CommunityHost, cfg.BotToken, "users/show", payload, &user); err != nil {
			lastErr = err
			continue
		}
		if strings.TrimSpace(user.ID) != "" {
			return &user, nil
		}
	}
	if lastErr == nil {
		lastErr = errors.New("community users/show returned empty user")
	}
	return nil, lastErr
}

func isLikelyCommunityUserID(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) < 8 || len(value) > 32 {
		return false
	}
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			continue
		}
		return false
	}
	return strings.HasPrefix(value, "an") || strings.HasPrefix(value, "am")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func communityInviteUser(ctx context.Context, cfg *operation_setting.CommunityBotSetting, userID string) (*communityInviteResult, error) {
	var out map[string]any
	err := communityPost(ctx, cfg.CommunityHost, cfg.BotToken, "chat/rooms/invitations/create", map[string]any{
		"roomId": cfg.RoomID,
		"userId": userID,
	}, &out)
	if err == nil {
		return &communityInviteResult{Status: "created", Created: true, Notify: true}, nil
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "already member") || strings.Contains(msg, "already joined") || strings.Contains(msg, "is already a member") {
		return &communityInviteResult{Status: "already_member", Created: false, Notify: false}, nil
	}
	if strings.Contains(msg, "already invited") || strings.Contains(msg, "invitation already") || strings.Contains(msg, "duplicate") || strings.Contains(msg, "already exists") || strings.Contains(msg, "unique") {
		return &communityInviteResult{Status: "already_invited", Created: false, Notify: false}, nil
	}
	return nil, err
}

func GrantCommunityJoinRewardIfNeeded(ctx context.Context, providerId int, providerSlug string, providerUserID string, username string, user *model.User) {
	cfg := operation_setting.GetCommunityBotSetting()
	if cfg == nil || !cfg.Enabled || !cfg.JoinRewardEnabled || user == nil || user.Id <= 0 {
		return
	}
	if slug := strings.TrimSpace(cfg.ProviderSlug); slug != "" && providerSlug != slug {
		return
	}
	roomID := strings.TrimSpace(cfg.RoomID)
	if roomID == "" {
		return
	}
	if ok, reason := CanReceiveCommunityBenefit(user.Id, "community", roomID, providerUserID, "join_reward"); !ok {
		common.SysLog(fmt.Sprintf("[CommunityBot] join reward blocked by membership user_id=%d provider_user_id=%s reason=%s", user.Id, providerUserID, reason))
		return
	}
	joined, err := CommunityUserJoinedRoom(ctx, cfg, roomID)
	if err != nil {
		common.SysLog(fmt.Sprintf("[CommunityBot] join reward room check failed user_id=%d provider_user_id=%s err=%s", user.Id, providerUserID, err.Error()))
		return
	}
	if !joined {
		return
	}
	quota := randomQuota(cfg.JoinRewardMinQuota, cfg.JoinRewardMaxQuota)
	granted, err := model.GrantCommunityBotRewardIfNeeded(user.Id, providerId, providerUserID, roomID, "join", "once", quota, 0)
	if err != nil {
		common.SysLog(fmt.Sprintf("[CommunityBot] join reward grant failed user_id=%d err=%s", user.Id, err.Error()))
		return
	}
	if granted {
		content := fmt.Sprintf("加入社区群聊奖励 %s", logger.LogQuota(quota))
		model.RecordCommunityBotRewardLog(user.Id, content, quota, 0, roomID, "join")
		if cfg.NotificationEnabled && cfg.NotifyOnJoinReward {
			_ = communitySendRoomMessageWithConfig(ctx, cfg, roomID, renderCommunityTemplate(cfg.JoinRewardTemplate, map[string]string{
				"username": username,
				"user_id":  providerUserID,
				"amount":   logger.LogQuota(quota),
				"room_id":  roomID,
			}))
		}
	}
}

func CommunityUserJoinedRoom(ctx context.Context, cfg *operation_setting.CommunityBotSetting, roomID string) (bool, error) {
	// chat/rooms/show reports whether the bot account itself joined the room, not whether
	// the OAuth user joined it. Keep join rewards fail-closed until the community exposes
	// a target-user membership check endpoint.
	return false, errors.New("target user room membership check is not available")
}

// communityScanInterval 计算两次扫描之间的间隔。
//
// 优先使用秒级配置 MessageScanIntervalSeconds（用于命令近实时响应，如 30 秒），
// 未配置时回退到分钟级 MessageScanIntervalMinutes（默认 5 分钟），保持向后兼容。
// 为避免对社区 API 造成压力，最小间隔限制为 10 秒。
func communityScanInterval(cfg *operation_setting.CommunityBotSetting) time.Duration {
	if cfg.MessageScanIntervalSeconds > 0 {
		seconds := cfg.MessageScanIntervalSeconds
		if seconds < 2 {
			seconds = 2
		}
		return time.Duration(seconds) * time.Second
	}
	minutes := cfg.MessageScanIntervalMinutes
	if minutes <= 0 {
		minutes = 5
	}
	return time.Duration(minutes) * time.Minute
}

func StartCommunityBotTask() {
	if !common.IsMasterNode {
		common.SysLog("[CommunityBot] background task skipped on slave node")
		return
	}
	communityBotTaskOnce.Do(func() {
		// Realtime command response uses Misskey streaming; polling remains as fallback and for daily-message stats.
		StartCommunityStreaming()
		go func() {
			for {
				cfg := operation_setting.GetCommunityBotSetting()
				interval := communityScanInterval(cfg)
				select {
				case <-time.After(interval):
					ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
					_, err := ScanCommunityMessagesAndReward(ctx)
					cancel()
					if err != nil {
						common.SysLog("[CommunityBot] scheduled scan failed: " + err.Error())
					}
				case <-communityBotTaskStop:
					return
				}
			}
		}()
		go func() {
			cleanup := func() {
				cutoff := time.Now().Add(-communityBotClaimRetention).Unix()
				if err := model.DeleteCompletedCommunityBotMessageClaimsBefore(cutoff); err != nil {
					common.SysLog("[CommunityBot] message claim cleanup failed: " + err.Error())
				}
			}
			cleanup()
			ticker := time.NewTicker(24 * time.Hour)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					cleanup()
				case <-communityBotTaskStop:
					return
				}
			}
		}()
	})
}

func processCommunityCommandOnce(ctx context.Context, cfg *operation_setting.CommunityBotSetting, roomID string, msg communityMessage, user *communityUser) (handled bool, resolved bool, err error) {
	if msg.ID == "" || !communityLooksLikeGameCommand(normalizeCommunityCommandText(cfg, msg.Text)) {
		return false, false, nil
	}
	claim, acquired, err := model.ClaimCommunityBotMessage(
		AgentSiteID(),
		roomID,
		msg.ID,
		"command",
		communityBotWorkerID,
		communityBotCommandClaimTTL,
	)
	if err != nil {
		return false, false, err
	}
	if !acquired {
		return false, claim != nil && claim.Status == model.CommunityBotClaimCompleted, nil
	}

	handled = handleCommunityCommand(ctx, cfg, roomID, msg, user)
	var completionErr error
	for attempt := 0; attempt < 3; attempt++ {
		completed, completeErr := model.CompleteCommunityBotMessageClaim(claim.Id, claim.OwnerId, claim.FencingToken)
		if completeErr == nil && completed {
			return handled, true, nil
		}
		if completeErr != nil {
			completionErr = completeErr
		} else {
			completionErr = errors.New("community bot message claim lost its fencing token")
		}
		time.Sleep(time.Duration(attempt+1) * 50 * time.Millisecond)
	}
	return handled, false, completionErr
}

func ScanCommunityMessagesAndReward(ctx context.Context) (*CommunityBotScanResult, error) {
	cfg := operation_setting.GetCommunityBotSetting()
	result := &CommunityBotScanResult{}
	if cfg == nil || !cfg.Enabled {
		return result, nil
	}
	roomID := strings.TrimSpace(cfg.RoomID)
	if roomID == "" || strings.TrimSpace(cfg.BotToken) == "" {
		return result, errors.New("community bot room or token is empty")
	}
	messages, err := fetchCommunityRoomTimeline(ctx, cfg, roomID)
	if err != nil {
		return result, err
	}
	result.ScannedMessages = len(messages)
	if len(messages) == 0 {
		return result, nil
	}
	sort.Slice(messages, func(i, j int) bool { return messages[i].CreatedAt < messages[j].CreatedAt })

	// 命令去重游标：只处理上次扫描之后产生的新消息。
	// Misskey 的消息 ID（aid 格式）字典序即时间序，可直接字符串比较。
	// 这样保证同一条「签到/验牌」命令只被处理一次——删除 bot 回复也不会重发。
	commandCursor := ""
	if state, err := model.GetCommunityBotRoomState(roomID); err == nil && state != nil {
		commandCursor = strings.TrimSpace(state.LastMessageId)
	}

	lastID := ""
	cursorBlocked := false
	stats := map[string]*dailyCommunityUserStat{}
	for _, msg := range messages {
		previousLastID := lastID
		if !cursorBlocked && msg.ID != "" {
			lastID = msg.ID
		}
		user := msg.FromUser
		if user == nil {
			user = msg.User
		}
		if user == nil || user.ID == "" {
			continue
		}
		if cfg.AntiSpamIgnoreBot && communityIsSelfBotMessage(cfg, user) {
			continue
		}
		// 仅对游标之后的新消息执行命令，避免重复回复历史命令。
		candidateText := normalizeCommunityCommandText(cfg, msg.Text)
		isCommandCandidate := communityLooksLikeGameCommand(candidateText)
		commandResolved := true
		if msg.ID != "" && (commandCursor == "" || msg.ID > commandCursor) {
			if isCommandCandidate {
				common.SysLog(fmt.Sprintf("[CommunityBot] command candidate source=poll room_id=%s message_id=%s cursor=%s user_id=%s username=%s text=%q", roomID, msg.ID, commandCursor, user.ID, user.Username, truncateCommunityLog(candidateText, 120)))
			}
			if isCommandCandidate && !(cfg.AntiSpamIgnoreBot && user.IsBot) {
				handled, resolved, processErr := processCommunityCommandOnce(ctx, cfg, roomID, msg, user)
				if processErr != nil {
					commandResolved = false
					common.SysLog(fmt.Sprintf("[CommunityBot] command claim failed source=poll room_id=%s message_id=%s err=%s", roomID, msg.ID, processErr.Error()))
				} else if !resolved {
					commandResolved = false
				}
				if handled {
					result.CommandHits++
				}
			}
		}
		if !cursorBlocked && !commandResolved {
			lastID = previousLastID
			cursorBlocked = true
		}
		if cfg.AntiSpamIgnoreBot && user.IsBot {
			continue
		}
		text := normalizeCommunityText(msg.Text)
		if len([]rune(text)) < maxInt(cfg.AntiSpamMinChars, 0) {
			continue
		}
		createdAt, _ := time.Parse(time.RFC3339Nano, msg.CreatedAt)
		if createdAt.IsZero() {
			createdAt = time.Now()
		}
		date := communityBotStatDate(createdAt)
		key := user.ID + "|" + date
		st := stats[key]
		if st == nil {
			st = &dailyCommunityUserStat{UserID: user.ID, Username: user.Username, Date: date, Texts: map[string]struct{}{}, LastMessageID: msg.ID}
			stats[key] = st
		}
		if _, exists := st.Texts[text]; !exists {
			st.Count++
			st.Texts[text] = struct{}{}
		}
		if msg.ID != "" {
			st.LastMessageID = msg.ID
		}
	}
	if lastID != "" {
		if _, err := model.AdvanceCommunityBotRoomState(roomID, lastID); err != nil {
			return result, fmt.Errorf("advance community bot room cursor: %w", err)
		}
	}
	return result, nil
}

func communityIsSelfBotMessage(cfg *operation_setting.CommunityBotSetting, user *communityUser) bool {
	if cfg == nil || user == nil {
		return false
	}
	if strings.TrimSpace(cfg.BotUserID) != "" && strings.TrimSpace(user.ID) == strings.TrimSpace(cfg.BotUserID) {
		return true
	}
	if strings.TrimSpace(cfg.BotUsername) != "" && strings.EqualFold(strings.TrimSpace(user.Username), strings.TrimSpace(cfg.BotUsername)) {
		return true
	}
	return false
}

func communityLooksLikeGameCommand(text string) bool {
	text = strings.TrimSpace(normalizeCommunityText(text))
	if text == "" {
		return false
	}
	text = strings.TrimPrefix(text, "/")
	commands := []string{
		"游戏", "菜单", "玩法", "帮助", "帮助游戏", "help", "menu", "game", "games",
		"验牌", "我要验牌", "验证", "verify",
		"签到", "打卡", "给我擦皮鞋", "checkin", "signin",
		"邀请", "我的邀请", "邀请链接", "invite", "myinvite",
		"运势", "今日运势", "抽签", "fortune",
		"答题", "题目", "答", "answer", "quiz",
		"彩票", "买彩票", "双色球", "选号", "机选", "开奖", "lottery", "pick",
		"夺宝", "夺宝奇兵", "加入夺宝", "treasure",
		"排行榜", "排名", "leaderboard", "rank", "龙虎榜",
		"福袋", "开福袋", "拆福袋", "领福袋", "luckybag",
		"我的", "资产", "余额", "积分", "profile", "me",
		"骰子", "摇骰子", "掷骰", "dice",
		"红包", "红包雨", "发红包", "抢红包", "redpacket",
		"猜拳", "石头剪刀布", "剪刀石头布", "rps", "接受", "应战",
		"比大小", "比数字", "对决", "compare",
		"成语接龙", "接龙", "成接", "idiom",
		"坐庄", "开庄", "猜", "banker", "guess",
		"悬赏", "发布悬赏", "接单", "完成悬赏", "取消悬赏", "bounty",
		"竞猜", "预测", "押", "竞猜结算", "predict",
		"转盘", "幸运转盘", "抽奖", "轮盘", "wheel",
	}
	for _, cmd := range commands {
		if text == cmd || strings.HasPrefix(text, cmd+" ") || strings.HasPrefix(text, cmd+"：") || strings.HasPrefix(text, cmd+":") || strings.HasPrefix(text, cmd+"，") || strings.HasPrefix(text, cmd+",") {
			return true
		}
	}
	return false
}

func communityGameAdapterBaseURL() string {
	base := firstNonEmpty(
		os.Getenv("COMMUNITY_GAME_ADAPTER_URL"),
		os.Getenv("HERMES_ADAPTER_URL"),
		os.Getenv("GAME_ADMIN_UPSTREAM"),
		os.Getenv("HERMES_ADAPTER_UPSTREAM"),
	)
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	if base == "" {
		// Production Compose maps this stable name to the host gateway.
		base = "http://host.docker.internal:18181"
	}
	return base
}

func communityGameAdapterKey() string {
	return strings.TrimSpace(firstNonEmpty(
		os.Getenv("COMMUNITY_GAME_ADAPTER_KEY"),
		os.Getenv("HERMES_ADAPTER_KEY"),
		os.Getenv("GAME_ADMIN_KEY"),
	))
}

func communityForwardGameCommandToAdapter(ctx context.Context, cfg *operation_setting.CommunityBotSetting, roomID string, msg communityMessage, user *communityUser, text string) (string, bool, error) {
	if !communityLooksLikeGameCommand(text) {
		return "", false, nil
	}
	if user == nil || strings.TrimSpace(user.ID) == "" {
		return "", true, errors.New("community game user is empty")
	}
	baseURL := communityGameAdapterBaseURL()
	key := communityGameAdapterKey()
	if key == "" {
		return "", true, errors.New("community game adapter key is empty")
	}
	display := firstNonEmpty(user.Username, user.ID)
	payload := map[string]any{
		"source":           "community",
		"platform":         "community",
		"group_id":         roomID,
		"room_id":          roomID,
		"chat_id":          roomID,
		"user_id":          user.ID,
		"user_external_id": user.ID,
		"username":         display,
		"text":             text,
		"message":          text,
		"message_id":       msg.ID,
		"community_bound":  false,
		"user_bound":       false,
		"new_api_user_id":  0,
		"quota_balance":    0,
	}
	logUserID := 0
	if boundUser, ok := communityResolveBoundUser(cfg, user); ok && boundUser != nil {
		logUserID = boundUser.Id
		payload["community_bound"] = true
		payload["user_bound"] = true
		payload["new_api_user_id"] = boundUser.Id
		payload["quota_balance"] = boundUser.Quota
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/v1/tasks/plan", bytes.NewReader(body))
	if err != nil {
		return "", true, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("User-Agent", communityBotUserAgent)
	client := http.Client{Timeout: 12 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		model.RecordLogEvent(logUserID, model.LogTypeSystem, "community game adapter request failed", model.LogEventOptions{
			Category:       "community",
			Source:         "community",
			Action:         "game",
			Status:         "failed",
			SiteId:         AgentSiteID(),
			RoomId:         roomID,
			ExternalUserId: user.ID,
			Other: map[string]interface{}{
				"community_game": map[string]interface{}{"command": text, "reason": err.Error()},
			},
		})
		return "", true, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		err := fmt.Errorf("adapter status=%d body=%s", resp.StatusCode, truncateCommunityLog(string(respBody), 300))
		model.RecordLogEvent(logUserID, model.LogTypeSystem, "community game adapter status failed", model.LogEventOptions{
			Category:       "community",
			Source:         "community",
			Action:         "game",
			Status:         "failed",
			SiteId:         AgentSiteID(),
			RoomId:         roomID,
			ExternalUserId: user.ID,
			Other: map[string]interface{}{
				"community_game": map[string]interface{}{"command": text, "reason": err.Error()},
			},
		})
		return "", true, err
	}
	var out communityAdapterPlanResponse
	if err := json.Unmarshal(respBody, &out); err != nil {
		return "", true, fmt.Errorf("adapter response decode failed: %w", err)
	}
	if !out.OK {
		if out.Error == "" {
			out.Error = "adapter returned ok=false"
		}
		return "", true, errors.New(out.Error)
	}
	reply := strings.TrimSpace(out.Result.Reply)
	if reply == "" {
		return "", true, errors.New("adapter returned empty reply")
	}
	model.RecordLogEvent(logUserID, model.LogTypeSystem, "community game command relayed", model.LogEventOptions{
		Category:       "community",
		Source:         "community",
		Action:         "game",
		Status:         "success",
		SiteId:         AgentSiteID(),
		RoomId:         roomID,
		ExternalUserId: user.ID,
		Other: map[string]interface{}{
			"community_game": map[string]interface{}{"command": text, "game_handled": out.Result.GameHandled, "notes": out.Result.Notes},
		},
	})
	return reply, true, nil
}

// handleCommunityCommand 处理社区房间内的命令消息（签到 / 验牌 / 游戏）。
//
// 返回 true 表示该消息是一条已识别的命令（用于统计 CommandHits）。
//
// 命令进入本函数前必须先取得持久化消息 claim。streaming 与 polling 共同
// 竞争同一唯一键，polling 只在较早命令都已解决后推进房间游标。
func handleCommunityCommand(ctx context.Context, cfg *operation_setting.CommunityBotSetting, roomID string, msg communityMessage, user *communityUser) bool {
	text := normalizeCommunityCommandText(cfg, msg.Text)
	switch text {
	case "验牌", "verify", "验证":
		reply := communityVerifyCommand(ctx, cfg, roomID, user)
		replyID, err := communitySendCommandRoomMessageReturnID(ctx, cfg, roomID, reply)
		if err != nil {
			common.SysLog(fmt.Sprintf("[CommunityBot] command reply failed room_id=%s message_id=%s err=%s", roomID, msg.ID, err.Error()))
		}
		// 阅后即焚：撤回 bot 回复 + 用户命令消息，保持房间清爽。
		communityScheduleBurn(cfg, []string{replyID, msg.ID})
		common.SysLog(fmt.Sprintf("[CommunityBot] command handle verify room_id=%s message_id=%s reply_id=%s user_id=%s username=%s", roomID, msg.ID, replyID, user.ID, user.Username))
		return true
	case "签到", "checkin":
		reply := communityCheckinCommand(ctx, cfg, roomID, user)
		replyID, err := communitySendCommandRoomMessageReturnID(ctx, cfg, roomID, reply)
		if err != nil {
			common.SysLog(fmt.Sprintf("[CommunityBot] command reply failed room_id=%s message_id=%s err=%s", roomID, msg.ID, err.Error()))
		}
		communityScheduleBurn(cfg, []string{replyID, msg.ID})
		common.SysLog(fmt.Sprintf("[CommunityBot] command handle checkin room_id=%s message_id=%s reply_id=%s user_id=%s username=%s", roomID, msg.ID, replyID, user.ID, user.Username))
		return true
	default:
		if reply, handled, err := communityForwardGameCommandToAdapter(ctx, cfg, roomID, msg, user, text); handled {
			if err != nil {
				display := firstNonEmpty(user.Username, user.ID)
				common.SysLog(fmt.Sprintf("[CommunityBot] command game adapter failed room_id=%s message_id=%s user_id=%s username=%s command=%s err=%s", roomID, msg.ID, user.ID, user.Username, text, err.Error()))
				reply = fmt.Sprintf("@%s 游戏服务暂时不可用，已记录日志，请稍后再试。", display)
			}
			replyID, sendErr := communitySendCommandRoomMessageReturnID(ctx, cfg, roomID, reply)
			if sendErr != nil {
				common.SysLog(fmt.Sprintf("[CommunityBot] command game reply failed room_id=%s message_id=%s err=%s", roomID, msg.ID, sendErr.Error()))
			}
			communityScheduleBurn(cfg, []string{replyID, msg.ID})
			common.SysLog(fmt.Sprintf("[CommunityBot] command handle game room_id=%s message_id=%s reply_id=%s user_id=%s username=%s command=%s", roomID, msg.ID, replyID, user.ID, user.Username, text))
			return true
		}
		return false
	}
}

// communityResolveBoundUser 将社区用户解析为已绑定的 new-api 用户。
// 未绑定时返回 (nil, false)。
func cacheCommunityIdentityBinding(userID int, providerUserID string, username string) {
	if userID <= 0 || strings.TrimSpace(providerUserID) == "" {
		return
	}
	if err := model.UpsertUserIdentityBinding(AgentSiteID(), userID, "community", providerUserID, strings.TrimSpace(username)); err != nil {
		common.SysLog(fmt.Sprintf("[CommunityBot] identity binding cache write failed user_id=%d provider_user_id=%s username=%s err=%s", userID, providerUserID, strings.TrimSpace(username), err.Error()))
	}
}

func communityResolveBoundUser(cfg *operation_setting.CommunityBotSetting, user *communityUser) (*model.User, bool) {
	matchedUserID, err := model.FindCommunityBoundUser(cfg.ProviderSlug, user.ID, user.Username)
	if err != nil || matchedUserID <= 0 {
		return nil, false
	}
	cacheCommunityIdentityBinding(matchedUserID, user.ID, user.Username)
	u, err := model.GetUserById(matchedUserID, false)
	if err != nil || u == nil || u.Id <= 0 {
		return nil, false
	}
	return u, true
}

// communityUserHasActiveToken 判断用户是否已开通可用的 API Key（存在已启用的 token）。
func communityUserHasActiveToken(userID int) (bool, error) {
	count, err := model.CountUserEnabledTokens(userID)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// communityBindGuideText 生成未绑定用户的引导文案。
func communityBindGuideText(username string) string {
	cfg := operation_setting.GetCommunityBotSetting()
	host := strings.TrimRight(strings.TrimSpace(cfg.CommunityHost), "/")
	if host == "" {
		host = "https://dc.hhhl.cc"
	}
	if tpl := strings.TrimSpace(cfg.BindGuideTemplate); tpl != "" {
		return renderCommunityTemplate(tpl, map[string]string{"username": username, "host": host})
	}
	return fmt.Sprintf("@%s 你还未绑定本站账号。请先在站点使用 %s 社区授权登录完成绑定（账号与社区身份打通）后，再回到群里发送「签到」即可领取额度。", username, host)
}

// communityCheckinCommand 执行真实签到：解析绑定用户 -> 调用 model.UserCheckin
// 真实发放额度并写入 checkins 台账（唯一约束保证每日一次），返回真实结果文案。
func communityCheckinCommand(ctx context.Context, cfg *operation_setting.CommunityBotSetting, roomID string, user *communityUser) string {
	display := firstNonEmpty(user.Username, user.ID)

	if !operation_setting.GetCheckinSetting().Enabled {
		model.RecordLogEvent(0, model.LogTypeSystem, "community checkin denied: disabled", model.LogEventOptions{
			Category:       "community",
			Source:         "community",
			Action:         "checkin",
			Status:         "denied",
			SiteId:         AgentSiteID(),
			RoomId:         roomID,
			ExternalUserId: user.ID,
			Group:          model.CheckinChannelCommunity,
		})
		return fmt.Sprintf("@%s 当前签到功能未开启，请稍后再试。", display)
	}

	boundUser, ok := communityResolveBoundUser(cfg, user)
	if !ok {
		model.RecordLogEvent(0, model.LogTypeSystem, "community checkin denied: not bound", model.LogEventOptions{
			Category:       "community",
			Source:         "community",
			Action:         "checkin",
			Status:         "denied",
			SiteId:         AgentSiteID(),
			RoomId:         roomID,
			ExternalUserId: user.ID,
			Group:          model.CheckinChannelCommunity,
		})
		return communityBindGuideText(display)
	}

	if allowed, deniedReason := CanCurrentUserReceiveCheckinBenefitReason(boundUser.Id); !allowed {
		model.RecordLogEvent(boundUser.Id, model.LogTypeSystem, "community checkin denied: membership expired", model.LogEventOptions{
			Category:   "community",
			Source:     "community",
			Action:     "checkin",
			Status:     "denied",
			SiteId:     AgentSiteID(),
			Group:      model.CheckinChannelCommunity,
			BudgetPool: "activity",
			RewardType: "checkin_community",
			RoomId:     roomID,
			Tags:       []string{"community", "checkin", "membership_risk"},
			Other: map[string]interface{}{
				"reason":           deniedReason,
				"provider":         cfg.ProviderSlug,
				"provider_user_id": user.ID,
			},
		})
		return communityCheckinDeniedMessage(deniedReason)
	}

	checkin, err := model.UserCheckinByChannel(boundUser.Id, model.CheckinChannelCommunity)
	if err != nil {
		model.RecordLogEvent(boundUser.Id, model.LogTypeSystem, "community checkin failed", model.LogEventOptions{
			Category:       "community",
			Source:         "community",
			Action:         "checkin",
			Status:         "failed",
			SiteId:         AgentSiteID(),
			RoomId:         roomID,
			ExternalUserId: user.ID,
			Group:          model.CheckinChannelCommunity,
			BudgetPool:     "activity",
			Other: map[string]interface{}{
				"checkin": map[string]interface{}{
					"reason": err.Error(),
				},
			},
		})
		// 已签到 / 未启用 等业务错误，直接把真实原因返回给用户。
		if tpl := strings.TrimSpace(cfg.CheckinFailedTemplate); tpl != "" {
			return renderCommunityTemplate(tpl, map[string]string{"username": display, "reason": err.Error()})
		}
		return fmt.Sprintf("@%s 签到失败：%s", display, err.Error())
	}

	// 落管理可查的台账。
	model.RecordCommunityBotRewardLog(boundUser.Id, fmt.Sprintf("社区签到奖励 %s", logger.LogQuota(checkin.QuotaAwarded)), checkin.QuotaAwarded, 0, roomID, "checkin")
	model.RecordLogEvent(boundUser.Id, model.LogTypeSystem, "community checkin success", model.LogEventOptions{
		Quota:          checkin.QuotaAwarded,
		Category:       "community",
		Source:         "community",
		Action:         "checkin",
		Status:         "success",
		SiteId:         AgentSiteID(),
		RoomId:         roomID,
		ExternalUserId: user.ID,
		Group:          model.CheckinChannelCommunity,
		BudgetPool:     "activity",
		RewardType:     "checkin_community",
		Tags:           []string{"community", "checkin"},
		Other: map[string]interface{}{
			"checkin": map[string]interface{}{
				"quota_awarded": checkin.QuotaAwarded,
				"checkin_date":  checkin.CheckinDate,
			},
		},
	})

	common.SysLog(fmt.Sprintf("[CommunityBot] checkin granted user_id=%d provider_user_id=%s quota=%d date=%s room_id=%s", boundUser.Id, user.ID, checkin.QuotaAwarded, checkin.CheckinDate, roomID))

	// 查询签到后的最新额度用于展示「当前额度」。失败则回退到 boundUser 已读取的值 + 本次奖励。
	currentQuota := boundUser.Quota + checkin.QuotaAwarded
	if fresh, ferr := model.GetUserById(boundUser.Id, false); ferr == nil && fresh != nil {
		currentQuota = fresh.Quota
	}

	values := map[string]string{
		"username": display,
		"amount":   communityFormatUSD(checkin.QuotaAwarded),
		"balance":  communityFormatUSD(currentQuota),
	}
	if tpl := strings.TrimSpace(cfg.CheckinSuccessTemplate); tpl != "" {
		return renderCommunityTemplate(tpl, values)
	}
	return fmt.Sprintf("🎉 签到成功！@%s\n获得 %s ！\n💵 当前额度：%s", display, communityFormatUSD(checkin.QuotaAwarded), communityFormatUSD(currentQuota))
}

func communityCheckinDeniedMessage(reason string) string {
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

// communityFormatUSD 将额度按美元（2 位小数）格式化，用于群内签到/验牌结果展示。
// 例如：500000 -> "1.00 美刀"。
func communityFormatUSD(quota int) string {
	usd := float64(quota) / common.QuotaPerUnit
	return fmt.Sprintf("%.2f 美刀", usd)
}

// communityVerifyCommand 执行真实验牌（身份验证，非游戏）。
//
// 按业务定义检查 3 项，三项全部通过才算验牌成功：
//
//	① 是否已在社区注册并绑定账号
//	② 是否已开通 API Key 权限（存在已启用的 token）
//	③ 账号余额/额度状态（剩余额度 > 0）
//
// 验牌通过是参与 P2P 对战和系统坐庄类游戏的前提条件。返回逐项真实结果。
func communityVerifyCommand(ctx context.Context, cfg *operation_setting.CommunityBotSetting, roomID string, user *communityUser) string {
	display := firstNonEmpty(user.Username, user.ID)

	// ① 注册并绑定账号
	boundUser, ok := communityResolveBoundUser(cfg, user)
	if !ok {
		common.SysLog(fmt.Sprintf("[CommunityBot] verify fail not_bound provider_user_id=%s username=%s room_id=%s", user.ID, user.Username, roomID))
		model.RecordLogEvent(0, model.LogTypeSystem, "community verify denied: not bound", model.LogEventOptions{
			Category:       "community",
			Source:         "community",
			Action:         "verify",
			Status:         "denied",
			SiteId:         AgentSiteID(),
			RoomId:         roomID,
			ExternalUserId: user.ID,
			Other: map[string]interface{}{
				"verify": map[string]interface{}{
					"passed":         false,
					"reason":         "not_bound",
					"reason_code":    "not_bound",
					"reason_message": "未绑定站点账号",
				},
			},
		})
		return fmt.Sprintf("🛡️ 验牌结果 @%s\n① 账号绑定：❌ 未绑定\n② API Key：—\n③ 额度状态：—\n\n%s", display, communityBindGuideText(display))
	}

	// ② 已开通 API Key 权限（存在已启用的 token）
	hasActiveKey, keyErr := communityUserHasActiveToken(boundUser.Id)
	if keyErr != nil {
		common.SysLog(fmt.Sprintf("[CommunityBot] verify token query failed user_id=%d err=%s", boundUser.Id, keyErr.Error()))
		model.RecordLogEvent(boundUser.Id, model.LogTypeSystem, "community verify failed: token query failed", model.LogEventOptions{
			Category:       "community",
			Source:         "community",
			Action:         "verify",
			Status:         "failed",
			SiteId:         AgentSiteID(),
			RoomId:         roomID,
			ExternalUserId: user.ID,
			Other: map[string]interface{}{
				"verify": map[string]interface{}{
					"passed":         false,
					"reason":         "token_query_failed",
					"reason_code":    "token_query_failed",
					"reason_message": "查询 API Key 状态失败",
				},
			},
		})
		return fmt.Sprintf("@%s 验牌暂时失败，请稍后再试。", display)
	}

	// ③ 账号余额/额度状态（剩余额度 > 0）
	hasQuota := boundUser.Quota > 0

	keyMark := "✅ 已开通"
	if !hasActiveKey {
		keyMark = "❌ 未开通"
	}
	quotaMark := fmt.Sprintf("✅ %s", communityFormatUSD(boundUser.Quota))
	if !hasQuota {
		quotaMark = "❌ 额度不足"
	}

	if hasActiveKey && hasQuota {
		common.SysLog(fmt.Sprintf("[CommunityBot] verify pass user_id=%d provider_user_id=%s room_id=%s", boundUser.Id, user.ID, roomID))
		model.RecordLogEvent(boundUser.Id, model.LogTypeSystem, "community verify success", model.LogEventOptions{
			Category:       "community",
			Source:         "community",
			Action:         "verify",
			Status:         "success",
			SiteId:         AgentSiteID(),
			RoomId:         roomID,
			ExternalUserId: user.ID,
			Other: map[string]interface{}{
				"verify": map[string]interface{}{
					"passed":         true,
					"reason":         "ok",
					"reason_code":    "ok",
					"reason_message": "验牌通过",
					"has_api_key":    true,
					"has_quota":      true,
					"current_quota":  boundUser.Quota,
				},
			},
		})
		if tpl := strings.TrimSpace(cfg.VerifyPassTemplate); tpl != "" {
			return renderCommunityTemplate(tpl, map[string]string{"username": display, "key": keyMark, "quota": quotaMark})
		}
		return fmt.Sprintf("🛡️ 验牌结果 @%s\n① 账号绑定：✅ 已绑定\n② API Key：%s\n③ 额度状态：%s\n\n验牌通过 ✅ 你已满足参与 P2P 对战与系统坐庄类游戏的条件。", display, keyMark, quotaMark)
	}

	reasonCode, reasonMessage := communityVerifyFailureReason(hasActiveKey, hasQuota)
	common.SysLog(fmt.Sprintf("[CommunityBot] verify fail user_id=%d has_key=%t has_quota=%t room_id=%s", boundUser.Id, hasActiveKey, hasQuota, roomID))
	model.RecordLogEvent(boundUser.Id, model.LogTypeSystem, "community verify failed", model.LogEventOptions{
		Category:       "community",
		Source:         "community",
		Action:         "verify",
		Status:         "failed",
		SiteId:         AgentSiteID(),
		RoomId:         roomID,
		ExternalUserId: user.ID,
		Other: map[string]interface{}{
			"verify": map[string]interface{}{
				"passed":         false,
				"reason":         reasonCode,
				"reason_code":    reasonCode,
				"reason_message": reasonMessage,
				"has_api_key":    hasActiveKey,
				"has_quota":      hasQuota,
				"current_quota":  boundUser.Quota,
			},
		},
	})
	tip := ""
	if !hasActiveKey {
		tip = "请先在站点开通 API Key。"
	} else if !hasQuota {
		tip = "请先为账号充值或获取额度（可发送「签到」领取）。"
	}
	if tpl := strings.TrimSpace(cfg.VerifyFailedTemplate); tpl != "" {
		return renderCommunityTemplate(tpl, map[string]string{"username": display, "key": keyMark, "quota": quotaMark, "tip": tip})
	}
	return fmt.Sprintf("🛡️ 验牌结果 @%s\n① 账号绑定：✅ 已绑定\n② API Key：%s\n③ 额度状态：%s\n\n验牌未通过：%s", display, keyMark, quotaMark, tip)
}

func communityVerifyFailureReason(hasActiveKey bool, hasQuota bool) (string, string) {
	switch {
	case !hasActiveKey && !hasQuota:
		return "missing_api_key_and_quota", "未开通 API Key 且额度不足"
	case !hasActiveKey:
		return "missing_api_key", "未开通 API Key"
	case !hasQuota:
		return "insufficient_quota", "额度不足"
	default:
		return "verify_failed", "验牌未通过"
	}
}

func communityBotStatDate(t time.Time) string {
	return model.AgentBusinessDateAt(t)
}

type dailyCommunityUserStat struct {
	UserID        string
	Username      string
	Date          string
	Count         int
	Texts         map[string]struct{}
	LastMessageID string
}

var communityDailyUnboundLogLast sync.Map // key room_id|provider_user_id|date -> unix seconds

func communityShouldLogDailyUnbound(roomID string, providerUserID string, date string) bool {
	key := strings.TrimSpace(roomID) + "|" + strings.TrimSpace(providerUserID) + "|" + strings.TrimSpace(date)
	now := time.Now().Unix()
	if value, ok := communityDailyUnboundLogLast.Load(key); ok {
		if last, ok := value.(int64); ok && now-last < 3600 {
			return false
		}
	}
	communityDailyUnboundLogLast.Store(key, now)
	return true
}

func fetchCommunityRoomTimeline(ctx context.Context, cfg *operation_setting.CommunityBotSetting, roomID string) ([]communityMessage, error) {
	limit := cfg.MessageScanLimit
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	payload := map[string]any{"roomId": roomID, "limit": limit}
	var messages []communityMessage
	if err := communityGet(ctx, cfg.CommunityHost, cfg.BotToken, "chat/messages/room-timeline", payload, &messages); err != nil {
		if !isCommunityMethodNotAllowed(err) {
			return nil, err
		}
		if err := communityPost(ctx, cfg.CommunityHost, cfg.BotToken, "chat/messages/room-timeline", payload, &messages); err != nil {
			return nil, err
		}
	}
	lookback := cfg.MessageLookbackMinutes
	if lookback <= 0 {
		lookback = 1440
	}
	cutoff := time.Now().Add(-time.Duration(lookback) * time.Minute)
	filtered := make([]communityMessage, 0, len(messages))
	for _, msg := range messages {
		if msg.CreatedAt == "" {
			filtered = append(filtered, msg)
			continue
		}
		createdAt, err := time.Parse(time.RFC3339Nano, msg.CreatedAt)
		if err != nil || createdAt.After(cutoff) {
			filtered = append(filtered, msg)
		}
	}
	return filtered, nil
}

func communityProviderID(slug string) int {
	provider, err := model.GetCustomOAuthProviderBySlug(strings.TrimSpace(slug))
	if err != nil || provider == nil {
		return 0
	}
	return provider.Id
}

type communityAPIError struct {
	Endpoint   string
	StatusCode int
	Body       string
}

func (e *communityAPIError) Error() string {
	return fmt.Sprintf("community api %s status=%d body=%s", e.Endpoint, e.StatusCode, e.Body)
}

func isCommunityMethodNotAllowed(err error) bool {
	var apiErr *communityAPIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusMethodNotAllowed
}

func communityGet(ctx context.Context, host string, token string, endpoint string, payload any, out any) error {
	return communityRequest(ctx, http.MethodGet, host, token, endpoint, payload, out)
}

func communityPost(ctx context.Context, host string, token string, endpoint string, payload any, out any) error {
	return communityRequest(ctx, http.MethodPost, host, token, endpoint, payload, out)
}

func communityRequest(ctx context.Context, method string, host string, token string, endpoint string, payload any, out any) error {
	host = strings.TrimRight(strings.TrimSpace(host), "/")
	if host == "" {
		return errors.New("community host is empty")
	}
	if token == "" {
		return errors.New("community token is empty")
	}
	bodyMap := map[string]any{}
	if payload != nil {
		b, _ := json.Marshal(payload)
		_ = json.Unmarshal(b, &bodyMap)
	}
	endpointURL := host + "/api/" + strings.TrimPrefix(endpoint, "/")
	var bodyReader io.Reader
	if strings.EqualFold(method, http.MethodGet) {
		values := url.Values{}
		for key, value := range bodyMap {
			addCommunityQueryValue(values, key, value)
		}
		if encoded := values.Encode(); encoded != "" {
			endpointURL += "?" + encoded
		}
	} else {
		bodyMap["i"] = token
		body, _ := json.Marshal(bodyMap)
		bodyReader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpointURL, bodyReader)
	if err != nil {
		return err
	}
	if !strings.EqualFold(method, http.MethodGet) {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", communityBotUserAgent)
	client := http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &communityAPIError{Endpoint: endpoint, StatusCode: resp.StatusCode, Body: truncateCommunityLog(string(respBody), 500)}
	}
	if out != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, out); err != nil {
			return fmt.Errorf("community api %s decode failed: %w body=%s", endpoint, err, truncateCommunityLog(string(respBody), 300))
		}
	}
	return nil
}

func addCommunityQueryValue(values url.Values, key string, value any) {
	switch v := value.(type) {
	case nil:
		return
	case string:
		values.Add(key, v)
	case []string:
		for _, item := range v {
			values.Add(key, item)
		}
	case []any:
		for _, item := range v {
			values.Add(key, fmt.Sprint(item))
		}
	default:
		values.Add(key, fmt.Sprint(v))
	}
}

func randomURLSafe(n int) string {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return common.GetRandomString(n)
	}
	return base64.RawURLEncoding.EncodeToString(buf)
}

func randomQuota(minQuota int, maxQuota int) int {
	if minQuota <= 0 && maxQuota <= 0 {
		return 0
	}
	if minQuota <= 0 {
		minQuota = maxQuota
	}
	if maxQuota <= 0 {
		maxQuota = minQuota
	}
	if maxQuota < minQuota {
		minQuota, maxQuota = maxQuota, minQuota
	}
	if maxQuota == minQuota {
		return minQuota
	}
	delta := int64(maxQuota - minQuota + 1)
	n, err := rand.Int(rand.Reader, big.NewInt(delta))
	if err != nil {
		return minQuota
	}
	return minQuota + int(n.Int64())
}

var whitespaceRe = regexp.MustCompile(`\s+`)
var communityMentionPrefixRe = regexp.MustCompile(`^@[\pL\pN_\.\\-]+[\s:：，,。]*`)

func normalizeCommunityText(text string) string {
	text = strings.TrimSpace(text)
	text = whitespaceRe.ReplaceAllString(text, " ")
	return strings.ToLower(text)
}

func normalizeCommunityCommandText(cfg *operation_setting.CommunityBotSetting, text string) string {
	text = normalizeCommunityText(text)
	if text == "" {
		return ""
	}
	// 社区群里常见的触发方式是 "@机器人 签到 / @机器人 验牌"。
	// 这里只剥离单个前缀提及，不影响正文统计口径。
	if strings.HasPrefix(text, "@") {
		text = communityMentionPrefixRe.ReplaceAllString(text, "")
		text = strings.TrimSpace(text)
	}
	botName := strings.ToLower(strings.TrimSpace(cfg.BotUsername))
	if botName != "" {
		prefixes := []string{
			"@" + botName + " ",
			"@" + botName + ":",
			"@" + botName + "：",
			"@" + botName + ",",
			"@" + botName + "，",
			"@" + botName + "。",
		}
		for _, prefix := range prefixes {
			if strings.HasPrefix(text, prefix) {
				text = strings.TrimSpace(strings.TrimPrefix(text, prefix))
				break
			}
		}
	}
	text = strings.TrimPrefix(text, "@")
	return strings.TrimSpace(text)
}

func renderCommunityTemplate(tpl string, values map[string]string) string {
	if strings.TrimSpace(tpl) == "" {
		return ""
	}
	out := tpl
	for k, v := range values {
		out = strings.ReplaceAll(out, "{"+k+"}", v)
	}
	return out
}

func truncateCommunityLog(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

func maxInt(a int, b int) int {
	if a > b {
		return a
	}
	return b
}
