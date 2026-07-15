package controller

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

// GameAdmin reverse-proxy controller.
//
// Proxies /api/game-admin/* requests to the Hermes adapter's /game-admin/*
// endpoints. The adapter is a Python sidecar running on the same host
// (default http://127.0.0.1:18181). Authentication is handled by the
// X-Hermes-Key shared-secret header, validated on both sides.
//
// Environment variables:
//   GAME_ADMIN_UPSTREAM  — adapter base URL (default "http://127.0.0.1:18181")
//   GAME_ADMIN_KEY       — shared secret sent as X-Hermes-Key

var (
	gameAdminUpstream string
	gameAdminKey      string
	gameAdminOnce     sync.Once
	gameAdminClient   *http.Client
)

func initGameAdmin() {
	gameAdminOnce.Do(func() {
		gameAdminUpstream = firstNonEmptyString(os.Getenv("COMMUNITY_GAME_ADAPTER_URL"), os.Getenv("GAME_ADMIN_UPSTREAM"), os.Getenv("HERMES_ADAPTER_URL"))
		if gameAdminUpstream == "" {
			gameAdminUpstream = "http://127.0.0.1:18181"
		}
		gameAdminUpstream = strings.TrimRight(gameAdminUpstream, "/")

		gameAdminKey = firstNonEmptyString(os.Getenv("GAME_ADMIN_KEY"), os.Getenv("HERMES_ADAPTER_KEY"), os.Getenv("ADAPTER_KEY"))

		gameAdminClient = &http.Client{
			Timeout: 30 * time.Second,
			// Don't follow redirects — pass them through
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}

		common.SysLog(fmt.Sprintf("GameAdmin proxy initialized: upstream=%s, key_set=%v",
			gameAdminUpstream, gameAdminKey != ""))
	})
}

// ProxyGameAdmin handles all /api/game-admin/* requests by proxying them
// to the adapter's /game-admin/* endpoint.
func ProxyGameAdmin(c *gin.Context) {
	initGameAdmin()

	// Extract the sub-path after /api/game-admin
	proxyPath := c.Param("proxyPath")
	if proxyPath == "" {
		proxyPath = "/"
	}

	// Build upstream URL: /api/game-admin/games → /game-admin/games
	targetURL := gameAdminUpstream + "/game-admin" + proxyPath
	if c.Request.URL.RawQuery != "" {
		targetURL += "?" + c.Request.URL.RawQuery
	}

	// Create upstream request
	var bodyReader io.Reader
	if c.Request.Body != nil && c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
		bodyReader = c.Request.Body
		defer c.Request.Body.Close()
	}

	req, err := http.NewRequestWithContext(c.Request.Context(), c.Request.Method, targetURL, bodyReader)
	if err != nil {
		common.SysError(fmt.Sprintf("GameAdmin proxy: failed to create request: %v", err))
		c.JSON(http.StatusBadGateway, gin.H{
			"success": false,
			"message": "failed to create upstream request",
		})
		return
	}

	// Forward content-type if present
	if ct := c.GetHeader("Content-Type"); ct != "" {
		req.Header.Set("Content-Type", ct)
	}
	if cl := c.GetHeader("Content-Length"); cl != "" {
		req.Header.Set("Content-Length", cl)
		if n, err := strconv.ParseInt(cl, 10, 64); err == nil {
			req.ContentLength = n
		}
	}
	if accept := c.GetHeader("Accept"); accept != "" {
		req.Header.Set("Accept", accept)
	}
	req.Header.Set("Host", req.URL.Host)
	req.Header.Set("X-Forwarded-Host", c.Request.Host)
	req.Header.Set("X-Forwarded-Proto", firstNonEmptyString(c.GetHeader("X-Forwarded-Proto"), "https"))

	// Attach shared-secret auth header
	if gameAdminKey != "" {
		req.Header.Set("X-Hermes-Key", gameAdminKey)
		req.Header.Set("X-GameAdmin-Key", gameAdminKey)
	}

	// Audit: log non-GET requests from admin users
	if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
		userId := c.GetInt("id")
		username := c.GetString("username")
		common.SysLog(fmt.Sprintf("GameAdmin audit: user=%d(%s) method=%s path=%s",
			userId, username, c.Request.Method, proxyPath))
	}

	// Execute request
	resp, err := gameAdminClient.Do(req)
	if err != nil {
		common.SysError(fmt.Sprintf("GameAdmin proxy: upstream error: %v", err))
		c.JSON(http.StatusBadGateway, gin.H{
			"success": false,
			"message": "game admin service unavailable",
		})
		return
	}
	defer resp.Body.Close()

	// Copy response headers
	for key, values := range resp.Header {
		for _, value := range values {
			c.Writer.Header().Add(key, value)
		}
	}

	// Write status and body
	c.Writer.WriteHeader(resp.StatusCode)
	io.Copy(c.Writer, resp.Body)
}

// GameAdminHealth is a lightweight check that the adapter is reachable.
func GameAdminHealth(c *gin.Context) {
	initGameAdmin()

	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet,
		gameAdminUpstream+"/game-admin/health", nil)
	if err != nil {
		common.ApiErrorMsg(c, "failed to check game admin health")
		return
	}
	if gameAdminKey != "" {
		req.Header.Set("X-Hermes-Key", gameAdminKey)
	}

	resp, err := gameAdminClient.Do(req)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data": gin.H{
				"status":   "unreachable",
				"upstream": gameAdminUpstream,
				"error":    err.Error(),
			},
		})
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	c.Data(resp.StatusCode, resp.Header.Get("Content-Type"), body)
}

// ProxyGameAdminDirect handles /game-admin/* requests (top-level, not under /api/).
// This serves the adapter's full HTML admin panel and its internal API calls.
func ProxyGameAdminDirect(c *gin.Context) {
	initGameAdmin()

	proxyPath := c.Param("proxyPath")
	if proxyPath == "" {
		proxyPath = "/"
	}

	targetURL := gameAdminUpstream + "/game-admin" + proxyPath
	if c.Request.URL.RawQuery != "" {
		targetURL += "?" + c.Request.URL.RawQuery
	}

	var bodyReader io.Reader
	if c.Request.Body != nil && c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
		bodyReader = c.Request.Body
		defer c.Request.Body.Close()
	}

	req, err := http.NewRequestWithContext(c.Request.Context(), c.Request.Method, targetURL, bodyReader)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"success": false, "message": "failed to create upstream request"})
		return
	}

	if ct := c.GetHeader("Content-Type"); ct != "" {
		req.Header.Set("Content-Type", ct)
	}
	if cl := c.GetHeader("Content-Length"); cl != "" {
		req.Header.Set("Content-Length", cl)
		if n, err := strconv.ParseInt(cl, 10, 64); err == nil {
			req.ContentLength = n
		}
	}
	if accept := c.GetHeader("Accept"); accept != "" {
		req.Header.Set("Accept", accept)
	}
	if gameAdminKey != "" {
		req.Header.Set("X-Hermes-Key", gameAdminKey)
		req.Header.Set("X-GameAdmin-Key", gameAdminKey)
	}
	req.Header.Set("Host", req.URL.Host)
	req.Header.Set("X-Forwarded-Host", c.Request.Host)
	req.Header.Set("X-Forwarded-Proto", firstNonEmptyString(c.GetHeader("X-Forwarded-Proto"), "https"))

	if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
		userId := c.GetInt("id")
		username := c.GetString("username")
		common.SysLog(fmt.Sprintf("GameAdmin audit: user=%d(%s) method=%s path=%s",
			userId, username, c.Request.Method, proxyPath))
	}

	resp, err := gameAdminClient.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"success": false, "message": "game admin service unavailable"})
		return
	}
	defer resp.Body.Close()

	for key, values := range resp.Header {
		for _, value := range values {
			c.Writer.Header().Add(key, value)
		}
	}

	c.Writer.WriteHeader(resp.StatusCode)
	io.Copy(c.Writer, resp.Body)
}
