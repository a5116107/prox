package controller

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/console_setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
)

func GetAgentTasks(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	rows, err := model.ListAgentTasks(service.AgentSiteID(), c.Query("status"), limit)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, rows)
}

func CreateAgentTask(c *gin.Context) {
	var req struct {
		TaskType  string         `json:"task_type"`
		AgentName string         `json:"agent_name"`
		Text      string         `json:"text"`
		Payload   map[string]any `json:"payload"`
	}
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "无效的参数")
		return
	}
	row := &model.AgentTask{SiteId: service.AgentSiteID(), TaskType: firstControllerNonEmpty(req.TaskType, "admin"), AgentName: firstControllerNonEmpty(req.AgentName, "director"), Source: "admin", IssuerRole: "admin", Text: req.Text, Command: req.Text, Status: "open", PayloadJson: agentControllerJSON(req.Payload)}
	if err := model.CreateAgentTask(row); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "agent.task_create", map[string]interface{}{"task_id": row.Id})
	common.ApiSuccess(c, row)
}

func ReceiveAgentChatOpsQQ(c *gin.Context) {
	if !service.AgentChatOpsSecretOK("qq", c.GetHeader("Authorization"), c.Query("access_token"), "") {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "invalid chatops secret"})
		return
	}
	raw, err := service.DecodeAgentJSONRequest(c.Request)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	result, err := service.HandleAgentChatOps(c.Request.Context(), service.NormalizeAgentChatOpsQQ(raw))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}

func ReceiveAgentChatOpsTelegram(c *gin.Context) {
	if !service.AgentChatOpsSecretOK("tg", c.GetHeader("Authorization"), c.Query("secret"), c.GetHeader("X-Telegram-Bot-Api-Secret-Token")) {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "invalid chatops secret"})
		return
	}
	raw, err := service.DecodeAgentJSONRequest(c.Request)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	result, err := service.HandleAgentChatOps(c.Request.Context(), service.NormalizeAgentChatOpsTelegram(raw))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}

func ReceiveAgentChatOpsGeneric(c *gin.Context) {
	if !service.AgentChatOpsSecretOK(c.DefaultQuery("source", "community"), c.GetHeader("Authorization"), c.Query("secret"), "") {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "invalid chatops secret"})
		return
	}
	var req service.AgentChatOpsRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "无效的参数")
		return
	}
	if req.Source == "" {
		req.Source = c.DefaultQuery("source", "community")
	}
	result, err := service.HandleAgentChatOps(c.Request.Context(), req)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}

func CaptureAgentChatOpsMemory(c *gin.Context) {
	source := c.DefaultQuery("source", "community")
	if !service.AgentChatOpsSecretOK(source, c.GetHeader("Authorization"), c.Query("secret"), c.GetHeader("X-Telegram-Bot-Api-Secret-Token")) {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "invalid chatops secret"})
		return
	}
	var req service.AgentChatOpsRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "无效的参数")
		return
	}
	if req.Source == "" {
		req.Source = source
	}
	service.AgentCaptureChatMemory(req, 0)
	common.ApiSuccess(c, gin.H{"captured": true, "site_id": service.AgentSiteID()})
}

func firstControllerNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func ResolveAgentChatOpsBinding(c *gin.Context) {
	if !service.AgentChatOpsSecretOK(c.DefaultQuery("source", "qq"), c.GetHeader("Authorization"), c.Query("secret"), c.GetHeader("X-Telegram-Bot-Api-Secret-Token")) {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "invalid chatops secret"})
		return
	}
	var req service.AgentChatOpsRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "无效的参数")
		return
	}
	if req.Source == "" {
		req.Source = c.DefaultQuery("source", "qq")
	}
	identity := service.ResolveAgentChatOpsIdentity(req)
	common.ApiSuccess(c, identity)
}

func ExecuteAgentChatOpsGameAction(c *gin.Context) {
	if !service.AgentChatOpsSecretOK(c.DefaultQuery("source", "qq"), c.GetHeader("Authorization"), c.Query("secret"), c.GetHeader("X-Telegram-Bot-Api-Secret-Token")) {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "invalid chatops secret"})
		return
	}
	var req struct {
		Source         string         `json:"source"`
		RoomID         string         `json:"room_id"`
		MessageID      string         `json:"message_id"`
		UserExternalID string         `json:"user_external_id"`
		Username       string         `json:"username"`
		UserRole       string         `json:"user_role"`
		IsAdmin        bool           `json:"is_admin"`
		Action         map[string]any `json:"action"`
		Raw            map[string]any `json:"raw"`
	}
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "无效的参数")
		return
	}
	chatReq := service.AgentChatOpsRequest{Source: firstControllerNonEmpty(req.Source, c.DefaultQuery("source", "qq")), RoomID: req.RoomID, MessageID: req.MessageID, UserExternalID: req.UserExternalID, Username: req.Username, UserRole: req.UserRole, IsAdmin: req.IsAdmin, Raw: req.Raw}
	action, err := service.ExecuteAgentChatOpsGameAction(c.Request.Context(), chatReq, req.Action)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, action)
}

func CreateAgentChatOpsBindCode(c *gin.Context) {
	userID := c.GetInt("id")
	code, expiresAt, err := service.CreateAgentChatOpsBindCode(userID)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	channel := getAgentChatOpsBindChannel()
	model.RecordLogEvent(userID, model.LogTypeSystem, "chatops bind code created", model.LogEventOptions{
		Category: "chatops",
		Source:   "web",
		Action:   "bind_code_create",
		Status:   "success",
		SiteId:   service.AgentSiteID(),
		Other: map[string]interface{}{
			"chatops_bind": map[string]interface{}{
				"expires_at": expiresAt,
				"platform":   channel.Platform,
				"target":     channel.Target,
			},
		},
	})
	common.ApiSuccess(c, gin.H{
		"site_id":            service.AgentSiteID(),
		"code":               code,
		"expires_at":         expiresAt,
		"expires_in_seconds": 600,
		"usage":              fmt.Sprintf("回到%s发送：绑定 %s；也支持：验牌 %s", channel.Target, code, code),
		"chatops_platform":   channel.Platform,
		"chatops_source":     channel.Source,
		"chatops_target":     channel.Target,
		"join_url":           channel.JoinURL,
		"join_label":         channel.JoinLabel,
	})
}

func RegenerateTokenRiskActivationCode(c *gin.Context) {
	userID := c.GetInt("id")
	tokenID, err := strconv.Atoi(c.Param("id"))
	if err != nil || tokenID <= 0 {
		common.ApiErrorMsg(c, "无效的 Key ID")
		return
	}
	token, err := model.GetTokenByIds(tokenID, userID)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	code, expiresAt, err := service.ReissueRiskTokenActivationCode(userID, token)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"token_id":           token.Id,
		"code":               code,
		"expires_at":         expiresAt,
		"expires_in_seconds": service.GetRiskControlSettingTTLSeconds(),
		"usage":              fmt.Sprintf("回到%s发送：绑定 %s，完成后 Key 会自动恢复可用。", getAgentChatOpsBindChannel().Target, code),
	})
}

func GetAgentChatOpsBindPage(c *gin.Context) {
	channel := getAgentChatOpsBindChannel()
	lang, text := agentChatOpsBindPageLocale(c, channel)
	pageText, _ := json.Marshal(text)
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, fmt.Sprintf(`<!doctype html><html lang="%s"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>%s</title><style>body{font-family:system-ui,-apple-system,Segoe UI,sans-serif;background:radial-gradient(circle at top,#1d2b64,#0b1020 42vw,#050814);color:#eef2ff;margin:0;display:grid;place-items:center;min-height:100vh}.card{width:min(620px,92vw);background:#111936;border:1px solid #293456;border-radius:24px;padding:30px;box-shadow:0 24px 80px #0009}.top{display:flex;justify-content:space-between;gap:12px;align-items:flex-start}.lang a{color:#72f1b8;text-decoration:none;margin-left:12px;font-size:13px}.badge{display:inline-flex;align-items:center;border:1px solid #38507f;background:#172244;border-radius:999px;padding:7px 12px;color:#b8c7ff;font-size:13px;font-weight:700}.btn,.join{display:inline-flex;align-items:center;justify-content:center;gap:8px;background:#6d5dfc;color:white;border:0;border-radius:14px;padding:13px 18px;font-weight:800;cursor:pointer;text-decoration:none}.join{background:#12b981;margin:12px 0 6px}.btn:hover,.join:hover{transform:translateY(-1px);filter:brightness(1.05)}.code{font-size:34px;letter-spacing:6px;font-weight:900;background:#050814;border:1px solid #2d3a66;border-radius:16px;padding:18px;text-align:center;margin:18px 0}.muted{color:#a9b4d0;line-height:1.7}.small{font-size:13px}.ok{color:#72f1b8}.err{color:#ff8a8a}.steps{background:#0b1229;border:1px solid #26345c;border-radius:16px;padding:14px 16px;margin:18px 0}.steps b{color:#e5edff}</style></head><body><div class="card"><div class="top"><div><h1>%s</h1><span class="badge">%s</span></div><div class="lang"><a href="?lang=zh-CN">中文</a><a href="?lang=en">English</a></div></div><p class="muted">%s</p><div id="joinBox"></div><div class="steps"><div class="muted small">%s</div></div><button class="btn" onclick="gen()">%s</button><div id="out" class="muted"></div></div><script>const T=%s;function esc(s){return String(s||'').replace(/[&<>"']/g,function(c){return {'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]})}function resolveJoinURL(u){const raw=String(u||'').trim();if(!raw)return {ok:false,message:T.joinUnavailable};const x=new URL(raw,location.href);if(x.protocol==='https:'||x.protocol==='http:')return {ok:true,href:x.href};return {ok:false,message:T.joinUnavailable}}function renderJoin(){const box=document.getElementById('joinBox');try{const r=resolveJoinURL(T.joinURL);if(r.ok){box.innerHTML='<a class="join" target="_blank" rel="noopener noreferrer" href="'+esc(r.href)+'">'+esc(T.joinLabel)+'</a><div class="muted small">'+esc(T.joinHint)+'</div>'}else{box.innerHTML='<div class="muted small">'+esc(r.message)+'</div>'}}catch(e){box.innerHTML='<div class="err small">'+esc(T.joinUnavailable)+'</div><div class="muted small">'+esc(e&&e.message?e.message:String(e))+'</div>'}}async function gen(){const out=document.getElementById('out');out.textContent=T.loading;try{const r=await fetch('/api/agent/chatops/bind-code',{method:'POST',credentials:'include'});const d=await r.json();if(!d.success)throw new Error(d.message||T.failed);const x=d.data;out.innerHTML='<div class="code">'+esc(x.code)+'</div><p class="ok">'+esc(T.sendBind.replace('{code}',x.code))+'</p><p class="muted">'+esc(T.sendVerify.replace('{code}',x.code))+'</p>'}catch(e){out.innerHTML='<p class="err">'+esc(e.message)+'</p><p class="muted">'+esc(T.loginHint)+'</p>'}}renderJoin()</script></body></html>`,
		lang,
		text["title"],
		text["title"],
		text["channelBadge"],
		text["description"],
		text["steps"],
		text["button"],
		string(pageText),
	))
}

type agentChatOpsBindChannel struct {
	Platform     string
	Source       string
	Target       string
	JoinURL      string
	JoinLabel    string
	Identifier   string
	IsConfigured bool
}

func getAgentChatOpsBindChannel() agentChatOpsBindChannel {
	setting := operation_setting.GetAgentSetting()
	accessCfg := operation_setting.GetAccessControlSetting()
	if setting.QQBotEnabled {
		groupID := strings.TrimSpace(setting.QQGroupID)
		joinURL := firstControllerNonEmpty(strings.TrimSpace(accessCfg.PrimaryJoinURL), agentChatOpsCommunityLink("qq"))
		return agentChatOpsBindChannel{Platform: "QQ", Source: "qq", Target: "QQ 群", JoinURL: joinURL, JoinLabel: "加入 QQ 群", Identifier: groupID, IsConfigured: groupID != ""}
	}
	if setting.TGBotEnabled {
		chatID := strings.TrimSpace(setting.TGChatID)
		joinURL := firstControllerNonEmpty(strings.TrimSpace(accessCfg.PrimaryJoinURL), agentChatOpsCommunityLink("tg"), agentChatOpsCommunityLink("telegram"))
		return agentChatOpsBindChannel{Platform: "Telegram", Source: "tg", Target: "Telegram 群", JoinURL: joinURL, JoinLabel: "加入 Telegram 群", Identifier: chatID, IsConfigured: chatID != ""}
	}
	joinURL := firstControllerNonEmpty(strings.TrimSpace(accessCfg.CommunityJoinURL), agentChatOpsCommunityLink("community"), strings.TrimSpace(setting.CommunityHost))
	return agentChatOpsBindChannel{Platform: "Community", Source: "community", Target: "社区群", JoinURL: joinURL, JoinLabel: "打开社区入口", Identifier: strings.TrimSpace(setting.CommunityRoomID), IsConfigured: joinURL != ""}
}

func agentChatOpsCommunityLink(platform string) string {
	platform = strings.ToLower(strings.TrimSpace(platform))
	for _, item := range console_setting.GetAnnouncements() {
		communityType := strings.ToLower(strings.TrimSpace(agentChatOpsMapString(item, "communityType")))
		if communityType == platform || (platform == "tg" && communityType == "telegram") {
			if link := agentChatOpsHTTPURL(agentChatOpsMapString(item, "communityLink")); link != "" {
				return link
			}
		}
		var keys []string
		switch platform {
		case "qq":
			keys = []string{"qqGroupLink", "communityLink"}
		case "tg", "telegram":
			keys = []string{"tgGroupLink", "telegramGroupLink", "communityLink"}
		default:
			keys = []string{"communityLink", "qqGroupLink", "tgGroupLink"}
		}
		for _, key := range keys {
			if link := agentChatOpsHTTPURL(agentChatOpsMapString(item, key)); link != "" {
				return link
			}
		}
	}
	return ""
}

func agentChatOpsLocalizedTarget(langCode string, channel agentChatOpsBindChannel) string {
	source := strings.ToLower(strings.TrimSpace(channel.Source))
	switch langCode {
	case "en":
		switch source {
		case "qq":
			return "QQ group"
		case "tg":
			return "Telegram group"
		default:
			return "community chat"
		}
	case "ja":
		switch source {
		case "qq":
			return "QQ グループ"
		case "tg":
			return "Telegram グループ"
		default:
			return "コミュニティチャット"
		}
	case "fr":
		switch source {
		case "qq":
			return "groupe QQ"
		case "tg":
			return "groupe Telegram"
		default:
			return "chat communautaire"
		}
	case "ru":
		switch source {
		case "qq":
			return "группа QQ"
		case "tg":
			return "группа Telegram"
		default:
			return "чат сообщества"
		}
	case "vi":
		switch source {
		case "qq":
			return "nhóm QQ"
		case "tg":
			return "nhóm Telegram"
		default:
			return "chat cộng đồng"
		}
	default:
		switch source {
		case "qq":
			return "QQ 群"
		case "tg":
			return "Telegram 群"
		default:
			return "社区群"
		}
	}
}

func agentChatOpsLocalizedJoinLabel(langCode string, channel agentChatOpsBindChannel) string {
	source := strings.ToLower(strings.TrimSpace(channel.Source))
	switch langCode {
	case "en":
		switch source {
		case "qq":
			return "Join QQ group"
		case "tg":
			return "Join Telegram group"
		default:
			return "Open community chat"
		}
	case "ja":
		switch source {
		case "qq":
			return "QQ グループに参加"
		case "tg":
			return "Telegram グループに参加"
		default:
			return "コミュニティチャットを開く"
		}
	case "fr":
		switch source {
		case "qq":
			return "Rejoindre le groupe QQ"
		case "tg":
			return "Rejoindre le groupe Telegram"
		default:
			return "Ouvrir le chat communautaire"
		}
	case "ru":
		switch source {
		case "qq":
			return "Войти в группу QQ"
		case "tg":
			return "Войти в группу Telegram"
		default:
			return "Открыть чат сообщества"
		}
	case "vi":
		switch source {
		case "qq":
			return "Vào nhóm QQ"
		case "tg":
			return "Vào nhóm Telegram"
		default:
			return "Mở chat cộng đồng"
		}
	default:
		switch source {
		case "qq":
			return "加入 QQ 群"
		case "tg":
			return "加入 Telegram 群"
		default:
			return "打开社区入口"
		}
	}
}

func agentChatOpsMapString(item map[string]interface{}, key string) string {
	if item == nil {
		return ""
	}
	if value, ok := item[key].(string); ok {
		return strings.TrimSpace(value)
	}
	return ""
}

func agentChatOpsHTTPURL(value string) string {
	value = strings.TrimSpace(value)
	lower := strings.ToLower(value)
	if strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "http://") {
		return value
	}
	return ""
}

func agentChatOpsBindPageLocale(c *gin.Context, channel agentChatOpsBindChannel) (string, map[string]string) {
	rawLang := strings.ToLower(strings.TrimSpace(c.Query("lang")))
	if rawLang == "" {
		rawLang = strings.ToLower(c.GetHeader("Accept-Language"))
	}
	langCode := "zh-CN"
	switch {
	case strings.HasPrefix(rawLang, "zh"):
		langCode = "zh-CN"
	case strings.HasPrefix(rawLang, "ja"):
		langCode = "ja"
	case strings.HasPrefix(rawLang, "fr"):
		langCode = "fr"
	case strings.HasPrefix(rawLang, "ru"):
		langCode = "ru"
	case strings.HasPrefix(rawLang, "vi"):
		langCode = "vi"
	case strings.HasPrefix(rawLang, "en"):
		langCode = "en"
	}

	texts := map[string]map[string]string{
		"zh-CN": {
			"title":           "群聊身份绑定",
			"description":     "登录站点后点击生成绑定码，然后回到 {target} 发送 <b>绑定 绑定码</b> 或 <b>验牌 绑定码</b>。绑定码 10 分钟有效，只能使用一次。",
			"channelBadge":    "当前支持：{target}",
			"steps":           "流程：1）加入/打开群聊；2）生成绑定码；3）在群内发送 <b>绑定 绑定码</b>；需要验牌时发送 <b>验牌 绑定码</b>。",
			"button":          "生成绑定码",
			"loading":         "生成中...",
			"failed":          "生成失败",
			"sendBind":        "请在{target}发送：绑定 {code}",
			"sendVerify":      "也支持：验牌 {code}",
			"loginHint":       "请确认已经登录当前站点。",
			"joinHint":        "打开群入口后，在群内发送绑定命令完成验证。",
			"joinUnavailable": "当前后台未配置群跳转链接，请先从公告或管理员提供的入口进群。",
		},
		"en": {
			"title":           "Community chat identity binding",
			"description":     "Sign in to the site, generate a bind code, then return to the {target} and send <b>bind CODE</b> or <b>verify CODE</b>. The code is valid for 10 minutes and can be used only once.",
			"channelBadge":    "Supported now: {target}",
			"steps":           "Flow: 1) join/open the chat; 2) generate a bind code; 3) send <b>bind CODE</b> in the chat; use <b>verify CODE</b> when verification is needed.",
			"button":          "Generate bind code",
			"loading":         "Generating...",
			"failed":          "Failed to generate",
			"sendBind":        "Send in {target}: bind {code}",
			"sendVerify":      "Also supported: verify {code}",
			"loginHint":       "Please make sure you are signed in to this site.",
			"joinHint":        "After opening the chat entry, send the bind command in the chat to finish verification.",
			"joinUnavailable": "No chat invite link is configured yet. Use the announcement or admin-provided entry.",
		},
		"ja": {
			"title":           "コミュニティチャット本人確認",
			"description":     "サイトにログインしてバインドコードを生成し、{target} に戻って <b>bind CODE</b> または <b>verify CODE</b> を送信してください。コードは10分間有効で、一度だけ使用できます。",
			"channelBadge":    "現在対応：{target}",
			"steps":           "手順：1）チャットを開く/参加する；2）コードを生成；3）チャットで <b>bind CODE</b> を送信。確認時は <b>verify CODE</b> も使えます。",
			"button":          "バインドコードを生成",
			"loading":         "生成中...",
			"failed":          "生成に失敗しました",
			"sendBind":        "{target} で送信: bind {code}",
			"sendVerify":      "こちらも利用できます: verify {code}",
			"loginHint":       "現在のサイトにログインしていることを確認してください。",
			"joinHint":        "チャット入口を開き、チャット内でバインドコマンドを送信して確認を完了します。",
			"joinUnavailable": "チャット招待リンクはまだ設定されていません。公告または管理者提供の入口を使用してください。",
		},
		"fr": {
			"title":           "Liaison d’identité du chat communautaire",
			"description":     "Connectez-vous au site, générez un code de liaison, puis retournez dans {target} et envoyez <b>bind CODE</b> ou <b>verify CODE</b>. Le code est valable 10 minutes et utilisable une seule fois.",
			"channelBadge":    "Pris en charge : {target}",
			"steps":           "Flux : 1) ouvrir/rejoindre le chat ; 2) générer un code ; 3) envoyer <b>bind CODE</b> dans le chat ; utilisez <b>verify CODE</b> si nécessaire.",
			"button":          "Générer le code de liaison",
			"loading":         "Génération...",
			"failed":          "Échec de la génération",
			"sendBind":        "Envoyez dans {target} : bind {code}",
			"sendVerify":      "Également pris en charge : verify {code}",
			"loginHint":       "Veuillez vérifier que vous êtes connecté à ce site.",
			"joinHint":        "Après avoir ouvert l’entrée du chat, envoyez la commande de liaison dans le chat pour terminer la vérification.",
			"joinUnavailable": "Aucun lien d’invitation n’est configuré. Utilisez l’annonce ou l’entrée fournie par l’administrateur.",
		},
		"ru": {
			"title":           "Привязка личности в чате сообщества",
			"description":     "Войдите на сайт, создайте код привязки, затем вернитесь в {target} и отправьте <b>bind CODE</b> или <b>verify CODE</b>. Код действует 10 минут и может быть использован только один раз.",
			"channelBadge":    "Сейчас поддерживается: {target}",
			"steps":           "Шаги: 1) откройте/вступите в чат; 2) создайте код; 3) отправьте <b>bind CODE</b> в чат; для проверки используйте <b>verify CODE</b>.",
			"button":          "Создать код привязки",
			"loading":         "Создание...",
			"failed":          "Не удалось создать код",
			"sendBind":        "Отправьте в {target}: bind {code}",
			"sendVerify":      "Также поддерживается: verify {code}",
			"loginHint":       "Убедитесь, что вы вошли на этот сайт.",
			"joinHint":        "Откройте вход в чат и отправьте команду привязки в чате, чтобы завершить проверку.",
			"joinUnavailable": "Ссылка-приглашение не настроена. Используйте объявление или вход, предоставленный администратором.",
		},
		"vi": {
			"title":           "Liên kết danh tính chat cộng đồng",
			"description":     "Đăng nhập vào trang, tạo mã liên kết, sau đó quay lại {target} và gửi <b>bind CODE</b> hoặc <b>verify CODE</b>. Mã có hiệu lực trong 10 phút và chỉ dùng một lần.",
			"channelBadge":    "Đang hỗ trợ: {target}",
			"steps":           "Luồng: 1) mở/tham gia chat; 2) tạo mã; 3) gửi <b>bind CODE</b> trong chat; dùng <b>verify CODE</b> khi cần xác minh.",
			"button":          "Tạo mã liên kết",
			"loading":         "Đang tạo...",
			"failed":          "Tạo mã thất bại",
			"sendBind":        "Gửi trong {target}: bind {code}",
			"sendVerify":      "Cũng hỗ trợ: verify {code}",
			"loginHint":       "Vui lòng xác nhận bạn đã đăng nhập vào trang này.",
			"joinHint":        "Sau khi mở lối vào chat, gửi lệnh liên kết trong chat để hoàn tất xác minh.",
			"joinUnavailable": "Chưa cấu hình liên kết mời chat. Hãy dùng thông báo hoặc lối vào do quản trị viên cung cấp.",
		},
	}
	text := texts[langCode]
	targetLabel := agentChatOpsLocalizedTarget(langCode, channel)
	for key, value := range text {
		text[key] = strings.ReplaceAll(value, "{target}", targetLabel)
	}
	text["joinURL"] = channel.JoinURL
	text["joinLabel"] = agentChatOpsLocalizedJoinLabel(langCode, channel)
	text["platform"] = channel.Platform
	text["source"] = channel.Source
	text["target"] = targetLabel
	return langCode, text
}

func ConfirmAgentChatOpsBindCode(c *gin.Context) {
	if !service.AgentChatOpsSecretOK(c.DefaultQuery("source", "qq"), c.GetHeader("Authorization"), c.Query("secret"), c.GetHeader("X-Telegram-Bot-Api-Secret-Token")) {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "invalid chatops secret"})
		return
	}
	var req service.AgentChatOpsBindConfirmRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "无效的参数")
		return
	}
	if req.Source == "" {
		req.Source = c.DefaultQuery("source", "qq")
	}
	req.Scope = strings.ToLower(strings.TrimSpace(req.Scope))
	req.RoomID = strings.TrimSpace(req.RoomID)
	if req.Scope == "" && req.RoomID != "" {
		req.Scope = "room"
	}
	identity, err := service.ConfirmAgentChatOpsBindCode(req)
	if err != nil {
		model.RecordLogEvent(0, model.LogTypeSystem, "chatops bind confirm failed", model.LogEventOptions{
			Category:       "chatops",
			Source:         req.Source,
			Action:         "bind_confirm",
			Status:         "failed",
			SiteId:         service.AgentSiteID(),
			RoomId:         req.RoomID,
			ExternalUserId: req.UserExternalID,
			Other: map[string]interface{}{
				"chatops_bind": map[string]interface{}{
					"username":          req.Username,
					"scope":             req.Scope,
					"reason":            err.Error(),
					"reason_code":       identity.Reason,
					"room_required":     req.RoomID == "",
					"room_id_effective": req.RoomID,
				},
			},
		})
		common.ApiErrorMsg(c, err.Error())
		return
	}
	model.RecordLogEvent(identity.NewAPIUserID, model.LogTypeSystem, "chatops bind confirmed", model.LogEventOptions{
		Category:       "chatops",
		Source:         req.Source,
		Action:         "bind_confirm",
		Status:         "success",
		SiteId:         service.AgentSiteID(),
		RoomId:         req.RoomID,
		ExternalUserId: req.UserExternalID,
		Other: map[string]interface{}{
			"chatops_bind": map[string]interface{}{
				"username":          req.Username,
				"scope":             req.Scope,
				"activated_tokens":  identity.ActivatedTokens,
				"room_id_effective": req.RoomID,
			},
		},
	})
	common.ApiSuccess(c, identity)
}

// HandleAgentChatOpsInvite records invite link/join/verify-claim events and performs authoritative reward settlement.
func HandleAgentChatOpsInvite(c *gin.Context) {
	if !service.AgentChatOpsSecretOK(c.DefaultQuery("source", "qq"), c.GetHeader("Authorization"), c.Query("secret"), c.GetHeader("X-Telegram-Bot-Api-Secret-Token")) {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "invalid chatops secret"})
		return
	}
	var req service.AgentChatOpsInviteRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "无效的参数")
		return
	}
	if req.Source == "" {
		req.Source = c.DefaultQuery("source", "qq")
	}
	result, err := service.HandleAgentChatOpsInvite(req)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}

// CheckinAgentChatOps 处理来自 QQ/TG/社区 的签到命令，按渠道真实发额度并返回结果。
func CheckinAgentChatOps(c *gin.Context) {
	if !service.AgentChatOpsSecretOK(c.DefaultQuery("source", "qq"), c.GetHeader("Authorization"), c.Query("secret"), c.GetHeader("X-Telegram-Bot-Api-Secret-Token")) {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "invalid chatops secret"})
		return
	}
	var req service.AgentChatOpsRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "无效的参数")
		return
	}
	if req.Source == "" {
		req.Source = c.DefaultQuery("source", "qq")
	}
	result := service.HandleAgentChatOpsCheckin(req)
	common.ApiSuccess(c, result)
}

// VerifyAgentChatOps 处理验牌命令（3项检查：绑定 / API Key / 额度）。
func VerifyAgentChatOps(c *gin.Context) {
	if !service.AgentChatOpsSecretOK(c.DefaultQuery("source", "qq"), c.GetHeader("Authorization"), c.Query("secret"), c.GetHeader("X-Telegram-Bot-Api-Secret-Token")) {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "invalid chatops secret"})
		return
	}
	var req service.AgentChatOpsRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "无效的参数")
		return
	}
	if req.Source == "" {
		req.Source = c.DefaultQuery("source", "qq")
	}
	result := service.HandleAgentChatOpsVerify(req)
	common.ApiSuccess(c, result)
}

// LeaderboardAgentChatOps 成长榜（累计签到天数排名）。
func LeaderboardAgentChatOps(c *gin.Context) {
	if !service.AgentChatOpsSecretOK(c.DefaultQuery("source", "qq"), c.GetHeader("Authorization"), c.Query("secret"), c.GetHeader("X-Telegram-Bot-Api-Secret-Token")) {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "invalid chatops secret"})
		return
	}
	var req service.AgentChatOpsRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "无效的参数")
		return
	}
	if req.Source == "" {
		req.Source = c.DefaultQuery("source", "qq")
	}
	result := service.HandleAgentChatOpsLeaderboard(req)
	common.ApiSuccess(c, result)
}
