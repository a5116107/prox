package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gorilla/websocket"
)

// 社区实时 streaming：通过 Misskey 的 WebSocket streaming 订阅聊天室（chatRoom）频道，
// 新消息毫秒级推送，收到即调用 handleCommunityCommand 处理签到/验牌命令。
//
// 与轮询扫描（ScanCommunityMessagesAndReward）的关系：
//   - streaming 负责命令的实时响应（签到/验牌），延迟 ~亚秒级；
//   - 轮询保留作为发言奖励统计与 streaming 断线时的兜底；
//   - 两条链路通过数据库消息领取记录竞争执行权；按时间排序的轮询链路负责
//     连续推进房间游标，避免实时消息乱序时越过尚未完成的较早命令。
//
// 启动后维护一条长连接，断线自动重连（指数退避）。配置变化（房间/令牌/host）
// 会在下一次重连循环中自动生效。

var (
	communityStreamingOnce sync.Once
	communityStreamingStop = make(chan struct{})
)

// communityStreamingChannelMessage 是 Misskey streaming 推送的频道消息外层结构：
// {"type":"channel","body":{"id":"<connId>","type":"message","body":{...communityMessage...}}}
type communityStreamingEnvelope struct {
	Type string `json:"type"`
	Body struct {
		ID   string          `json:"id"`
		Type string          `json:"type"`
		Body json.RawMessage `json:"body"`
	} `json:"body"`
}

// StartCommunityStreaming 启动社区实时 streaming 监听任务（仅启动一次）。
func StartCommunityStreaming() {
	communityStreamingOnce.Do(func() {
		go communityStreamingLoop()
	})
}

// communityStreamingLoop 维护到社区 streaming 的长连接，断线自动重连。
func communityStreamingLoop() {
	backoff := time.Second
	const maxBackoff = 30 * time.Second
	for {
		select {
		case <-communityStreamingStop:
			return
		default:
		}

		cfg := operation_setting.GetCommunityBotSetting()
		if cfg == nil || !cfg.Enabled || !cfg.StreamingEnabled {
			// 未启用：休眠后重试（支持运行时开启）。
			if sleepOrStop(communityStreamingStop, 15*time.Second) {
				return
			}
			continue
		}
		host := strings.TrimRight(strings.TrimSpace(cfg.CommunityHost), "/")
		roomID := strings.TrimSpace(cfg.RoomID)
		token := strings.TrimSpace(cfg.BotToken)
		if host == "" || roomID == "" || token == "" {
			if sleepOrStop(communityStreamingStop, 15*time.Second) {
				return
			}
			continue
		}

		err := communityStreamingConnect(host, roomID, token)
		if err != nil {
			common.SysLog("[CommunityStreaming] connection ended: " + err.Error())
			// 断线退避重连。
			if sleepOrStop(communityStreamingStop, backoff) {
				return
			}
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			continue
		}
		// 正常返回（如收到 stop），重置退避。
		backoff = time.Second
	}
}

// communityStreamingConnect 建立一条 streaming 连接并阻塞处理消息，直到出错或收到停止信号。
func communityStreamingConnect(host string, roomID string, token string) error {
	// http(s) -> ws(s)
	wsURL := host
	if strings.HasPrefix(wsURL, "https://") {
		wsURL = "wss://" + strings.TrimPrefix(wsURL, "https://")
	} else if strings.HasPrefix(wsURL, "http://") {
		wsURL = "ws://" + strings.TrimPrefix(wsURL, "http://")
	}
	wsURL = strings.TrimRight(wsURL, "/") + "/streaming?i=" + url.QueryEscape(token)

	dialer := websocket.Dialer{
		HandshakeTimeout: 15 * time.Second,
	}
	header := http.Header{}
	header.Set("User-Agent", communityBotUserAgent)

	conn, _, err := dialer.Dial(wsURL, header)
	if err != nil {
		return err
	}
	defer conn.Close()

	// 订阅 chatRoom 频道。
	connectMsg := map[string]any{
		"type": "connect",
		"body": map[string]any{
			"channel": "chatRoom",
			"id":      "newapi-room-" + roomID,
			"params":  map[string]any{"roomId": roomID},
		},
	}
	if err := conn.WriteJSON(connectMsg); err != nil {
		return err
	}
	common.SysLog("[CommunityStreaming] connected and subscribed chatRoom room_id=" + roomID)

	// 心跳：定期发送 ping，保活连接。
	pingDone := make(chan struct{})
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				_ = conn.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(10*time.Second))
			case <-pingDone:
				return
			}
		}
	}()
	defer close(pingDone)

	// 在独立 goroutine 监听停止信号，触发关闭。
	go func() {
		select {
		case <-communityStreamingStop:
			_ = conn.Close()
		case <-pingDone:
		}
	}()

	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			return err
		}
		communityStreamingHandleRaw(data, roomID)
	}
}

// communityStreamingHandleRaw 解析一条 streaming 原始消息并分发命令处理。
func communityStreamingHandleRaw(data []byte, roomID string) {
	var env communityStreamingEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		return
	}
	if env.Type != "channel" || env.Body.Type != "message" {
		return
	}

	var msg communityMessage
	if err := json.Unmarshal(env.Body.Body, &msg); err != nil {
		return
	}
	if msg.ID == "" {
		return
	}

	user := msg.FromUser
	if user == nil {
		user = msg.User
	}
	if user == nil || user.ID == "" {
		return
	}

	cfg := operation_setting.GetCommunityBotSetting()
	if cfg == nil || !cfg.Enabled {
		return
	}
	// 忽略 bot 自身消息，避免命令回声触发。
	if cfg.AntiSpamIgnoreBot && user.IsBot {
		return
	}
	// 忽略 bot 自己的用户 ID（双保险）。
	if botID := strings.TrimSpace(cfg.BotUserID); botID != "" && user.ID == botID {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	handled, _, processErr := processCommunityCommandOnce(ctx, cfg, roomID, msg, user)
	if processErr != nil {
		common.SysLog("[CommunityStreaming] command claim failed message_id=" + msg.ID + " err=" + processErr.Error())
		return
	}
	if handled {
		common.SysLog("[CommunityStreaming] command handled message_id=" + msg.ID + " user_id=" + user.ID + " username=" + user.Username)
	}
}

// sleepOrStop 休眠指定时长；若期间收到 stop 信号则返回 true。
func sleepOrStop(stop <-chan struct{}, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-stop:
		return true
	case <-timer.C:
		return false
	}
}
