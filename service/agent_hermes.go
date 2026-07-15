package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/setting/operation_setting"
)

type AgentHermesPlan struct {
	Reply            string              `json:"reply"`
	Risk             string              `json:"risk"`
	RequiresApproval bool                `json:"requires_approval"`
	Actions          []AgentHermesAction `json:"actions"`
	Notes            string              `json:"notes"`
}

type AgentHermesAction struct {
	Type    string         `json:"type"`
	Payload map[string]any `json:"payload"`
	Reason  string         `json:"reason"`
}

type agentHermesPlanResponse struct {
	OK      bool            `json:"ok"`
	SiteID  string          `json:"site_id"`
	Planner string          `json:"planner"`
	Result  AgentHermesPlan `json:"result"`
	Error   string          `json:"error"`
}

func AgentHermesEnabled(cfg *operation_setting.AgentSetting) bool {
	if cfg == nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(cfg.PlannerProvider), "hermes") && strings.TrimSpace(cfg.HermesBaseURL) != ""
}

func AgentPlanWithHermes(ctx context.Context, cfg *operation_setting.AgentSetting, req AgentChatOpsRequest, command string, taskID int, isAdmin bool) (*AgentHermesPlan, error) {
	if !AgentHermesEnabled(cfg) {
		return nil, errors.New("hermes planner is not configured")
	}
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.HermesBaseURL), "/")
	payload := map[string]any{
		"site_id":            AgentSiteID(),
		"site_name":          cfg.SiteName,
		"public_base_url":    cfg.PublicBaseURL,
		"api_base_url":       cfg.APIBaseURL,
		"source":             req.Source,
		"room_id":            req.RoomID,
		"message_id":         req.MessageID,
		"issuer_external_id": req.UserExternalID,
		"issuer_username":    req.Username,
		"issuer_role":        chatOpsRoleName(req.UserRole, isAdmin),
		"is_admin":           isAdmin,
		"new_api_user_id":    req.NewAPIUserID,
		"user_bound":         req.UserBound,
		"text":               req.Text,
		"command":            command,
		"task_id":            taskID,
		"community_room_id":  cfg.CommunityRoomID,
	}
	if state, err := GetAgentSiteState(); err == nil {
		payload["site_state_summary"] = agentStateSummary(state)
	}
	_ = AgentEnsureDefaultMemories()
	memoryContext := AgentMemoryContextForChatOps(req)
	payload["memory_context"] = memoryContext
	payload["memory_context_summary"] = memoryContext["summary"]
	body, _ := json.Marshal(payload)
	callCtx, cancel := context.WithTimeout(ctx, 55*time.Second)
	defer cancel()
	httpReq, err := http.NewRequestWithContext(callCtx, http.MethodPost, baseURL+"/v1/chatops/plan", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	if key := strings.TrimSpace(cfg.HermesAPIKey); key != "" {
		httpReq.Header.Set("Authorization", "Bearer "+key)
	}
	resp, err := (&http.Client{Timeout: 60 * time.Second}).Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("hermes planner status=%d body=%s", resp.StatusCode, truncateAgentText(string(respBody), 500))
	}
	var out agentHermesPlanResponse
	if err := json.Unmarshal(respBody, &out); err != nil {
		return nil, err
	}
	if !out.OK {
		return nil, errors.New(firstAgentNonEmpty(out.Error, "hermes planner failed"))
	}
	if strings.TrimSpace(out.Result.Risk) == "" {
		out.Result.Risk = "medium"
	}
	return &out.Result, nil
}
