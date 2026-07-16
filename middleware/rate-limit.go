package middleware

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
)

const timeFormat = "2006-01-02T15:04:05.000Z"

var inMemoryRateLimiter common.InMemoryRateLimiter

var redisSlidingWindowRateLimitScript = redis.NewScript(`
local key = KEYS[1]
local maximum = tonumber(ARGV[1])
local window = tonumber(ARGV[2])
local now_parts = redis.call('TIME')
local now = tonumber(now_parts[1]) + tonumber(now_parts[2]) / 1000000
local cutoff = now - window

while true do
    local oldest = redis.call('LINDEX', key, -1)
    if not oldest or tonumber(oldest) > cutoff then
        break
    end
    redis.call('RPOP', key)
end

if redis.call('LLEN', key) >= maximum then
    redis.call('EXPIRE', key, math.max(window, 1))
    return 0
end

redis.call('LPUSH', key, tostring(now))
redis.call('EXPIRE', key, math.max(window, 1))
return 1
`)

var (
	rateLimitRedisErrorLogMu sync.Mutex
	rateLimitRedisErrorAt    time.Time
)

var defNext = func(c *gin.Context) {
	c.Next()
}

func redisRateLimiter(ctx context.Context, key string, maxRequestNum int, duration int64) (bool, error) {
	if common.RDB == nil {
		return false, fmt.Errorf("redis rate limiter is enabled without a redis client")
	}
	result, err := redisSlidingWindowRateLimitScript.Run(
		ctx,
		common.RDB,
		[]string{key},
		maxRequestNum,
		duration,
	).Int()
	if err != nil {
		return false, err
	}
	return result == 1, nil
}

func logRateLimitRedisFallback(err error) {
	rateLimitRedisErrorLogMu.Lock()
	defer rateLimitRedisErrorLogMu.Unlock()
	if time.Since(rateLimitRedisErrorAt) < time.Minute {
		return
	}
	rateLimitRedisErrorAt = time.Now()
	common.SysError("redis rate limiter failed; using the per-node memory limiter: " + err.Error())
}

func allowRateLimitRequest(c *gin.Context, key string, maxRequestNum int, duration int64) bool {
	if maxRequestNum <= 0 {
		return false
	}
	if duration <= 0 {
		duration = 1
	}
	if common.RedisEnabled {
		allowed, err := redisRateLimiter(c.Request.Context(), key, maxRequestNum, duration)
		if err == nil {
			return allowed
		}
		logRateLimitRedisFallback(err)
	}
	inMemoryRateLimiter.Init(common.RateLimitKeyExpirationDuration)
	return inMemoryRateLimiter.Request("fallback:"+key, maxRequestNum, duration)
}

func rejectRateLimitedRequest(c *gin.Context, duration int64, scope string) {
	if duration <= 0 {
		duration = 1
	}
	c.Header("Retry-After", strconv.FormatInt(duration, 10))
	c.Header("X-RateLimit-Scope", scope)
	c.Status(http.StatusTooManyRequests)
	c.Abort()
}

type rateLimitIdentityFunc func(c *gin.Context) string

func rateLimitFactoryWithIdentity(maxRequestNum int, duration int64, mark string, identityFn rateLimitIdentityFunc) func(c *gin.Context) {
	return func(c *gin.Context) {
		identity := identityFn(c)
		key := "rateLimit:" + mark + ":" + identity
		if !allowRateLimitRequest(c, key, maxRequestNum, duration) {
			rejectRateLimitedRequest(c, duration, mark)
		}
	}
}

func rateLimitFactory(maxRequestNum int, duration int64, mark string) func(c *gin.Context) {
	return rateLimitFactoryWithIdentity(maxRequestNum, duration, mark, ipRateLimitIdentity)
}

func ipRateLimitIdentity(c *gin.Context) string {
	clientIP := strings.TrimSpace(c.ClientIP())
	if clientIP == "" {
		clientIP = "unknown"
	}
	return "ip:" + clientIP
}

func sessionRateLimitIdentity(c *gin.Context) (identity string) {
	defer func() {
		if recover() != nil {
			identity = ""
		}
	}()
	session := sessions.Default(c)
	switch id := session.Get("id").(type) {
	case int:
		if id > 0 {
			return fmt.Sprintf("user:%d", id)
		}
	case int64:
		if id > 0 {
			return fmt.Sprintf("user:%d", id)
		}
	case uint:
		if id > 0 {
			return fmt.Sprintf("user:%d", id)
		}
	}
	return ""
}

func credentialRateLimitIdentity(c *gin.Context) string {
	for _, header := range []string{
		"Authorization",
		"X-Api-Key",
		"X-Hermes-Key",
		"X-Telegram-Bot-Api-Secret-Token",
		"Access-Token",
	} {
		value := strings.TrimSpace(c.GetHeader(header))
		if value == "" {
			continue
		}
		digest := sha256.Sum256([]byte(header + ":" + value))
		return fmt.Sprintf("credential:%x", digest[:12])
	}
	return ""
}

func webRateLimitIdentity(c *gin.Context) string {
	if identity := sessionRateLimitIdentity(c); identity != "" {
		return identity
	}
	return ipRateLimitIdentity(c)
}

func apiRateLimitIdentity(c *gin.Context) string {
	if identity := sessionRateLimitIdentity(c); identity != "" {
		return identity
	}
	if identity := credentialRateLimitIdentity(c); identity != "" {
		return identity
	}
	return ipRateLimitIdentity(c)
}

func isStaticAssetRequest(c *gin.Context) bool {
	if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
		return false
	}
	requestPath := strings.ToLower(c.Request.URL.Path)
	for _, prefix := range []string{"/static/", "/assets/", "/fonts/"} {
		if strings.HasPrefix(requestPath, prefix) {
			return true
		}
	}
	switch requestPath {
	case "/favicon.ico", "/logo.png", "/manifest.json", "/site.webmanifest", "/robots.txt", "/service-worker.js", "/sw.js":
		return true
	}
	switch path.Ext(requestPath) {
	case ".avif", ".css", ".eot", ".gif", ".ico", ".jpeg", ".jpg", ".js", ".map", ".png", ".svg", ".ttf", ".webp", ".woff", ".woff2":
		return true
	default:
		return false
	}
}

func GlobalWebRateLimit() func(c *gin.Context) {
	if !common.GlobalWebRateLimitEnable {
		return defNext
	}
	limiter := rateLimitFactoryWithIdentity(common.GlobalWebRateLimitNum, common.GlobalWebRateLimitDuration, "GW", webRateLimitIdentity)
	return func(c *gin.Context) {
		if isStaticAssetRequest(c) {
			return
		}
		limiter(c)
	}
}

func GlobalAPIRateLimit() func(c *gin.Context) {
	if !common.GlobalApiRateLimitEnable {
		return defNext
	}
	identityLimiter := rateLimitFactoryWithIdentity(common.GlobalApiRateLimitNum, common.GlobalApiRateLimitDuration, "GA", apiRateLimitIdentity)
	ipUmbrellaLimit := common.GlobalApiRateLimitNum * 4
	if ipUmbrellaLimit < common.GlobalApiRateLimitNum {
		ipUmbrellaLimit = common.GlobalApiRateLimitNum
	}
	ipUmbrellaLimiter := rateLimitFactoryWithIdentity(ipUmbrellaLimit, common.GlobalApiRateLimitDuration, "GAI", ipRateLimitIdentity)
	return func(c *gin.Context) {
		identityLimiter(c)
		if c.IsAborted() {
			return
		}
		ipUmbrellaLimiter(c)
	}
}

func CriticalRateLimit() func(c *gin.Context) {
	if common.CriticalRateLimitEnable {
		return rateLimitFactory(common.CriticalRateLimitNum, common.CriticalRateLimitDuration, "CT")
	}
	return defNext
}

func DownloadRateLimit() func(c *gin.Context) {
	return rateLimitFactory(common.DownloadRateLimitNum, common.DownloadRateLimitDuration, "DW")
}

func UploadRateLimit() func(c *gin.Context) {
	return rateLimitFactory(common.UploadRateLimitNum, common.UploadRateLimitDuration, "UP")
}

// userRateLimitFactory creates a rate limiter keyed by authenticated user ID
// instead of client IP, making it resistant to proxy rotation attacks.
// Must be used AFTER authentication middleware (UserAuth).
func userRateLimitFactory(maxRequestNum int, duration int64, mark string) func(c *gin.Context) {
	return func(c *gin.Context) {
		userId := c.GetInt("id")
		if userId == 0 {
			c.Status(http.StatusUnauthorized)
			c.Abort()
			return
		}
		key := fmt.Sprintf("rateLimit:%s:user:%d", mark, userId)
		if !allowRateLimitRequest(c, key, maxRequestNum, duration) {
			rejectRateLimitedRequest(c, duration, mark)
		}
	}
}

// SearchRateLimit returns a per-user rate limiter for search endpoints.
// Configurable via SEARCH_RATE_LIMIT_ENABLE / SEARCH_RATE_LIMIT / SEARCH_RATE_LIMIT_DURATION.
func SearchRateLimit() func(c *gin.Context) {
	if !common.SearchRateLimitEnable {
		return defNext
	}
	return userRateLimitFactory(common.SearchRateLimitNum, common.SearchRateLimitDuration, "SR")
}

// AgentChatOpsRateLimit 用于 agent 内部 chatops 接口的高频限流。
// 这些接口有 X-Hermes-Key/secret 鉴权，属于受信任的内部高频调用（每条消息会触发
// resolve + history get + history append 等多次请求），不能用面向公网的严格限流，
// 否则会被 429 打断记忆/身份解析。默认 600 次/60 秒，可用 AGENT_CHATOPS_RATE_LIMIT 调整。
func AgentChatOpsRateLimit() func(c *gin.Context) {
	num := common.GetEnvOrDefault("AGENT_CHATOPS_RATE_LIMIT", 600)
	dur := int64(common.GetEnvOrDefault("AGENT_CHATOPS_RATE_LIMIT_DURATION", 60))
	return rateLimitFactory(num, dur, "ACO")
}
