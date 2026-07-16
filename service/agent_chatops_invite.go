package service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// AgentChatOpsInviteRequest is the authoritative invite event API used by QQ/TG/community adapters.
type AgentChatOpsInviteRequest struct {
	Source             string         `json:"source"`
	RoomID             string         `json:"room_id"`
	MessageID          string         `json:"message_id"`
	UserExternalID     string         `json:"user_external_id"`
	Username           string         `json:"username"`
	NewAPIUserID       int            `json:"new_api_user_id"`
	Action             string         `json:"action"`
	CampaignCode       string         `json:"campaign_code"`
	InviteCode         string         `json:"invite_code"`
	InviteURL          string         `json:"invite_url"`
	InviterUserID      int            `json:"inviter_user_id"`
	InviterExternalID  string         `json:"inviter_external_id"`
	OperatorExternalID string         `json:"operator_external_id"`
	InviteeUserID      int            `json:"invitee_user_id"`
	InviteeExternalID  string         `json:"invitee_external_id"`
	InviteeUsername    string         `json:"invitee_username"`
	InviterRewardQuota int            `json:"inviter_reward_quota"`
	InviteeRewardQuota int            `json:"invitee_reward_quota"`
	BudgetPool         string         `json:"budget_pool"`
	Metadata           map[string]any `json:"metadata"`
}

type AgentChatOpsInviteClaim struct {
	Stage           string `json:"stage"`
	RewardUserID    int    `json:"reward_user_id"`
	Quota           int    `json:"quota"`
	Status          string `json:"status"`
	ClaimID         int    `json:"claim_id,omitempty"`
	OpsFundLedgerID int    `json:"ops_fund_ledger_id,omitempty"`
	Error           string `json:"error,omitempty"`
}

type AgentChatOpsInviteResult struct {
	Success            bool                      `json:"success"`
	Action             string                    `json:"action"`
	Status             string                    `json:"status"`
	Reply              string                    `json:"reply,omitempty"`
	CampaignID         int                       `json:"campaign_id,omitempty"`
	CampaignCode       string                    `json:"campaign_code,omitempty"`
	InviteLinkID       int                       `json:"invite_link_id,omitempty"`
	InviteEdgeID       int                       `json:"invite_edge_id,omitempty"`
	InviteCode         string                    `json:"invite_code,omitempty"`
	InviteURL          string                    `json:"invite_url,omitempty"`
	InviterUserID      int                       `json:"inviter_user_id,omitempty"`
	InviteeUserID      int                       `json:"invitee_user_id,omitempty"`
	InviterRewardQuota int                       `json:"inviter_reward_quota,omitempty"`
	InviteeRewardQuota int                       `json:"invitee_reward_quota,omitempty"`
	Awarded            bool                      `json:"awarded"`
	Claims             []AgentChatOpsInviteClaim `json:"claims,omitempty"`
	Stats              map[string]int            `json:"stats,omitempty"`
}

func HandleAgentChatOpsInvite(req AgentChatOpsInviteRequest) (AgentChatOpsInviteResult, error) {
	req.normalize()
	gamePolicy, err := resolveEffectiveGroupGamePolicyTx(model.DB, AgentSiteID(), req.Source, req.RoomID, "invite")
	if err != nil {
		return AgentChatOpsInviteResult{}, err
	}
	if cfg, ok := chatOpsGroupConfigByScope(req.Source, req.RoomID); ok && !cfg.InviteEnabled && req.Action != "stats" {
		return AgentChatOpsInviteResult{Success: false, Action: req.Action, Status: "disabled", Reply: "本群邀请奖励暂未开启，请关注管理员公告。"}, nil
	}
	if gamePolicy.Found && !gamePolicy.Enabled && req.Action != "stats" {
		return AgentChatOpsInviteResult{Success: false, Action: req.Action, Status: "disabled", Reply: "本群邀请奖励暂未开启，请关注管理员公告。"}, nil
	}
	switch req.Action {
	case "link":
		return handleAgentChatOpsInviteLink(req)
	case "join":
		return handleAgentChatOpsInviteJoin(req)
	case "verify_claim", "claim", "verify":
		return handleAgentChatOpsInviteVerifyClaim(req)
	case "stats":
		return handleAgentChatOpsInviteStats(req)
	default:
		return AgentChatOpsInviteResult{}, fmt.Errorf("unsupported invite action: %s", req.Action)
	}
}

func (req *AgentChatOpsInviteRequest) normalize() {
	req.Source = strings.ToLower(strings.TrimSpace(req.Source))
	if req.Source == "telegram" {
		req.Source = "tg"
	}
	if req.Source == "" {
		req.Source = "qq"
	}
	req.Action = strings.ToLower(strings.TrimSpace(req.Action))
	req.RoomID = strings.TrimSpace(req.RoomID)
	req.UserExternalID = strings.TrimSpace(req.UserExternalID)
	req.Username = strings.TrimSpace(req.Username)
	req.CampaignCode = strings.TrimSpace(req.CampaignCode)
	req.InviteCode = strings.TrimSpace(req.InviteCode)
	req.InviteURL = strings.TrimSpace(req.InviteURL)
	req.InviterExternalID = strings.TrimSpace(req.InviterExternalID)
	req.OperatorExternalID = strings.TrimSpace(req.OperatorExternalID)
	req.InviteeExternalID = strings.TrimSpace(req.InviteeExternalID)
	req.InviteeUsername = strings.TrimSpace(req.InviteeUsername)
	req.BudgetPool = strings.TrimSpace(req.BudgetPool)
	if req.BudgetPool == "" {
		req.BudgetPool = "community"
	}
	if req.Metadata == nil {
		req.Metadata = map[string]any{}
	}
}

func (req AgentChatOpsInviteRequest) chatReqFor(externalID string, username string, newAPIUserID int) AgentChatOpsRequest {
	return AgentChatOpsRequest{
		Source:         req.Source,
		RoomID:         req.RoomID,
		MessageID:      req.MessageID,
		UserExternalID: strings.TrimSpace(externalID),
		Username:       strings.TrimSpace(username),
		NewAPIUserID:   newAPIUserID,
		Raw:            map[string]any{"invite_action": req.Action, "campaign_code": req.CampaignCode},
	}
}

func handleAgentChatOpsInviteLink(req AgentChatOpsInviteRequest) (AgentChatOpsInviteResult, error) {
	identity := ResolveAgentChatOpsIdentity(req.chatReqFor(req.UserExternalID, req.Username, req.NewAPIUserID))
	inviterID := req.InviterUserID
	if inviterID <= 0 && identity.UserBound {
		inviterID = identity.NewAPIUserID
	}
	if inviterID <= 0 {
		return AgentChatOpsInviteResult{}, errors.New("invite link requires bound inviter")
	}
	var out AgentChatOpsInviteResult
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		rewardPolicy, err := inviteRewardConfigTx(tx, req)
		if err != nil {
			return err
		}
		campaign, err := ensureAgentInviteCampaignTx(tx, req)
		if err != nil {
			return err
		}
		code := req.InviteCode
		if code == "" {
			code = deterministicInviteCode(AgentSiteID(), campaign.CampaignCode, inviterID)
		}
		inviteURL := req.InviteURL
		if inviteURL == "" {
			inviteURL = fmt.Sprintf("/#/register?aff=%d&ic=%s", inviterID, code)
		}
		now := time.Now().Unix()
		link := model.InviteLink{SiteId: AgentSiteID(), CampaignId: campaign.Id, InviterUserId: inviterID, Provider: req.Source, ExternalUserId: req.UserExternalID, InviteCode: code, InviteUrl: inviteURL, Status: "active", CreatedAt: now, UpdatedAt: now}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&link).Error; err != nil {
			return err
		}
		if err := tx.Where("site_id = ? AND campaign_id = ? AND inviter_user_id = ?", AgentSiteID(), campaign.Id, inviterID).First(&link).Error; err != nil {
			return err
		}
		_ = tx.Create(&model.InviteEvent{SiteId: AgentSiteID(), CampaignId: campaign.Id, InviteLinkId: link.Id, EventType: "link", Provider: req.Source, ExternalUserId: req.UserExternalID, UserId: inviterID, GroupId: req.RoomID, MetadataJson: inviteMetadataJSON(req), CreatedAt: now}).Error
		out = AgentChatOpsInviteResult{Success: true, Action: "link", Status: "ok", CampaignID: campaign.Id, CampaignCode: campaign.CampaignCode, InviteLinkID: link.Id, InviteCode: link.InviteCode, InviteURL: link.InviteUrl, InviterUserID: inviterID, InviterRewardQuota: rewardPolicy.InviterQuota, InviteeRewardQuota: rewardPolicy.InviteeQuota}
		return nil
	})
	if err == nil && out.Success {
		recordChatOpsGroupMetric(req.Source, req.RoomID, "invite_links", 0)
	}
	return out, err
}

func handleAgentChatOpsInviteJoin(req AgentChatOpsInviteRequest) (AgentChatOpsInviteResult, error) {
	inviteeExternal := firstNonEmptyAgent(req.InviteeExternalID, req.UserExternalID)
	inviteeUsername := firstNonEmptyAgent(req.InviteeUsername, req.Username)
	if inviteeExternal == "" && req.InviteeUserID <= 0 {
		return AgentChatOpsInviteResult{}, errors.New("invite join requires invitee external id or user id")
	}
	var out AgentChatOpsInviteResult
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		campaign, link, inviterID, err := resolveInviteCampaignLinkAndInviterTx(tx, req)
		if err != nil {
			return err
		}
		if inviterID <= 0 {
			_ = tx.Create(&model.InviteEvent{SiteId: AgentSiteID(), CampaignId: campaign.Id, EventType: "join_unattributed", Provider: req.Source, ExternalUserId: inviteeExternal, UserId: req.InviteeUserID, GroupId: req.RoomID, MetadataJson: inviteMetadataJSON(req), CreatedAt: time.Now().Unix()}).Error
			out = AgentChatOpsInviteResult{Success: true, Action: "join", Status: "no_inviter", CampaignID: campaign.Id, CampaignCode: campaign.CampaignCode}
			return nil
		}
		inviteeID := req.InviteeUserID
		if inviteeID <= 0 {
			ident := ResolveAgentChatOpsIdentity(req.chatReqFor(inviteeExternal, inviteeUsername, 0))
			if ident.UserBound {
				inviteeID = ident.NewAPIUserID
			}
		}
		edge, err := upsertInviteEdgeTx(tx, campaign, link, req, inviterID, inviteeID, inviteeExternal, "join", "pending")
		if err != nil {
			return err
		}
		_ = tx.Create(&model.InviteEvent{SiteId: AgentSiteID(), CampaignId: campaign.Id, InviteLinkId: inviteLinkID(link), InviteEdgeId: edge.Id, EventType: "join", Provider: req.Source, ExternalUserId: inviteeExternal, UserId: inviteeID, GroupId: req.RoomID, MetadataJson: inviteMetadataJSON(req), CreatedAt: time.Now().Unix()}).Error
		out = AgentChatOpsInviteResult{Success: true, Action: "join", Status: "ok", CampaignID: campaign.Id, CampaignCode: campaign.CampaignCode, InviteLinkID: inviteLinkID(link), InviteEdgeID: edge.Id, InviterUserID: inviterID, InviteeUserID: inviteeID}
		return nil
	})
	if err == nil && out.Success && out.Status == "ok" {
		recordChatOpsGroupMetric(req.Source, req.RoomID, "joins", 0)
	}
	return out, err
}

func handleAgentChatOpsInviteVerifyClaim(req AgentChatOpsInviteRequest) (AgentChatOpsInviteResult, error) {
	inviteeExternal := firstNonEmptyAgent(req.InviteeExternalID, req.UserExternalID)
	inviteeUsername := firstNonEmptyAgent(req.InviteeUsername, req.Username)
	ident := ResolveAgentChatOpsIdentity(req.chatReqFor(inviteeExternal, inviteeUsername, req.InviteeUserID))
	inviteeID := req.InviteeUserID
	if inviteeID <= 0 && ident.UserBound {
		inviteeID = ident.NewAPIUserID
	}
	if inviteeID <= 0 {
		return AgentChatOpsInviteResult{}, errors.New("invite verify claim requires bound invitee")
	}
	var out AgentChatOpsInviteResult
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		campaign, edge, err := findOrCreateClaimableInviteEdgeTx(tx, req, inviteeID, inviteeExternal)
		if err != nil {
			return err
		}
		if edge == nil || edge.Id <= 0 {
			out = AgentChatOpsInviteResult{Success: true, Action: "verify_claim", Status: "no_edge", InviteeUserID: inviteeID}
			return nil
		}
		if edge.InviterUserId <= 0 || edge.InviterUserId == inviteeID {
			out = AgentChatOpsInviteResult{Success: true, Action: "verify_claim", Status: "invalid_inviter", InviteEdgeID: edge.Id, InviterUserID: edge.InviterUserId, InviteeUserID: inviteeID}
			return nil
		}
		rewardPolicy, err := inviteRewardConfigTx(tx, req)
		if err != nil {
			return err
		}
		inviterReward, inviteeReward, pool := rewardPolicy.InviterQuota, rewardPolicy.InviteeQuota, rewardPolicy.BudgetPool
		now := time.Now().Unix()
		updates := map[string]interface{}{"stage": "verified", "status": "verified", "updated_at": now}
		if edge.InviteeUserId <= 0 {
			updates["invitee_user_id"] = inviteeID
			edge.InviteeUserId = inviteeID
		}
		if err := tx.Model(&model.InviteEdge{}).Where("id = ?", edge.Id).Updates(updates).Error; err != nil {
			return err
		}
		claims := make([]AgentChatOpsInviteClaim, 0, 2)
		if inviterReward > 0 {
			c, err := payInviteRewardClaimTx(tx, req, campaign, edge, "inviter", edge.InviterUserId, inviterReward, pool, rewardPolicy.MaxPerUserDay)
			if err != nil {
				return err
			}
			claims = append(claims, c)
		}
		if inviteeReward > 0 {
			c, err := payInviteRewardClaimTx(tx, req, campaign, edge, "invitee", inviteeID, inviteeReward, pool, rewardPolicy.MaxPerUserDay)
			if err != nil {
				return err
			}
			claims = append(claims, c)
		}
		_ = tx.Create(&model.InviteEvent{SiteId: AgentSiteID(), CampaignId: campaign.Id, InviteLinkId: edge.InviteLinkId, InviteEdgeId: edge.Id, EventType: "verify_claim", Provider: req.Source, ExternalUserId: inviteeExternal, UserId: inviteeID, GroupId: req.RoomID, MetadataJson: inviteMetadataJSON(req), CreatedAt: now}).Error
		awarded := false
		for _, c := range claims {
			if c.Status == "paid" {
				awarded = true
			}
		}
		status := "already_claimed"
		if awarded {
			status = "paid"
		}
		out = AgentChatOpsInviteResult{Success: true, Action: "verify_claim", Status: status, CampaignID: campaign.Id, CampaignCode: campaign.CampaignCode, InviteEdgeID: edge.Id, InviteLinkID: edge.InviteLinkId, InviterUserID: edge.InviterUserId, InviteeUserID: inviteeID, InviterRewardQuota: inviterReward, InviteeRewardQuota: inviteeReward, Awarded: awarded, Claims: claims}
		return nil
	})
	if err == nil && out.Success && out.Awarded {
		rewardCost := 0
		for _, c := range out.Claims {
			if c.Status == "paid" && c.Quota > 0 && c.RewardUserID > 0 {
				rewardCost += c.Quota
				model.RecordCommunityBotRewardLog(c.RewardUserID, fmt.Sprintf("invite %s reward %d", c.Stage, c.Quota), c.Quota, 0, req.RoomID, inviteRewardSourceType(c.Stage))
			}
		}
		recordChatOpsGroupMetric(req.Source, req.RoomID, "", rewardCost)
	}
	return out, err
}

func handleAgentChatOpsInviteStats(req AgentChatOpsInviteRequest) (AgentChatOpsInviteResult, error) {
	identity := ResolveAgentChatOpsIdentity(req.chatReqFor(req.UserExternalID, req.Username, req.NewAPIUserID))
	inviterID := req.InviterUserID
	if inviterID <= 0 && identity.UserBound {
		inviterID = identity.NewAPIUserID
	}
	if inviterID <= 0 {
		return AgentChatOpsInviteResult{}, errors.New("invite stats requires bound inviter")
	}
	var links, joins, verified, paid int64
	var earned int
	site := AgentSiteID()
	model.DB.Model(&model.InviteLink{}).Where("site_id = ? AND inviter_user_id = ?", site, inviterID).Count(&links)
	model.DB.Model(&model.InviteEdge{}).Where("site_id = ? AND inviter_user_id = ?", site, inviterID).Count(&joins)
	model.DB.Model(&model.InviteEdge{}).Where("site_id = ? AND inviter_user_id = ? AND status IN ?", site, inviterID, []string{"verified", "paid"}).Count(&verified)
	model.DB.Model(&model.InviteRewardClaim{}).Where("site_id = ? AND reward_user_id = ? AND reward_stage = ? AND status = ?", site, inviterID, "inviter", "paid").Count(&paid)
	_ = model.DB.Model(&model.InviteRewardClaim{}).Select("COALESCE(SUM(quota),0)").Where("site_id = ? AND reward_user_id = ? AND reward_stage = ? AND status = ?", site, inviterID, "inviter", "paid").Scan(&earned).Error
	return AgentChatOpsInviteResult{Success: true, Action: "stats", Status: "ok", InviterUserID: inviterID, Stats: map[string]int{"links": int(links), "joins": int(joins), "verified": int(verified), "paid": int(paid), "earned_quota": earned}}, nil
}

func ensureAgentInviteCampaignTx(tx *gorm.DB, req AgentChatOpsInviteRequest) (*model.InviteCampaign, error) {
	code := req.CampaignCode
	if code == "" {
		code = defaultInviteCampaignCode(req)
	}
	now := time.Now().Unix()
	platform := chatOpsPlatformKey(req.Source, req.RoomID)
	campaign := model.InviteCampaign{SiteId: AgentSiteID(), CampaignCode: code, Name: code, SourcePlatform: platform, SourceGroupId: req.RoomID, TargetPlatform: platform, TargetGroupId: req.RoomID, Status: "active", RuleJson: inviteMetadataJSON(req), CreatedAt: now, UpdatedAt: now}
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&campaign).Error; err != nil {
		return nil, err
	}
	if err := tx.Where("campaign_code = ?", code).First(&campaign).Error; err != nil {
		return nil, err
	}
	return &campaign, nil
}

func resolveInviteCampaignLinkAndInviterTx(tx *gorm.DB, req AgentChatOpsInviteRequest) (*model.InviteCampaign, *model.InviteLink, int, error) {
	campaign, err := ensureAgentInviteCampaignTx(tx, req)
	if err != nil {
		return nil, nil, 0, err
	}
	var link model.InviteLink
	linkFound := false
	if req.InviteCode != "" {
		if err := tx.Where("site_id = ? AND invite_code = ?", AgentSiteID(), req.InviteCode).First(&link).Error; err == nil {
			linkFound = true
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, 0, err
		}
	}
	if !linkFound && req.InviteURL != "" {
		if err := tx.Where("site_id = ? AND invite_url = ?", AgentSiteID(), req.InviteURL).First(&link).Error; err == nil {
			linkFound = true
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, 0, err
		}
	}
	inviterID := req.InviterUserID
	if linkFound {
		inviterID = link.InviterUserId
		if link.CampaignId > 0 {
			_ = tx.Where("id = ?", link.CampaignId).First(campaign).Error
		}
		return campaign, &link, inviterID, nil
	}
	if inviterID <= 0 {
		ext := firstNonEmptyAgent(req.InviterExternalID, req.OperatorExternalID)
		if ext != "" {
			ident := ResolveAgentChatOpsIdentity(req.chatReqFor(ext, "", 0))
			if ident.UserBound {
				inviterID = ident.NewAPIUserID
			}
		}
	}
	return campaign, nil, inviterID, nil
}

func upsertInviteEdgeTx(tx *gorm.DB, campaign *model.InviteCampaign, link *model.InviteLink, req AgentChatOpsInviteRequest, inviterID int, inviteeID int, inviteeExternal string, stage string, status string) (*model.InviteEdge, error) {
	now := time.Now().Unix()
	var edge model.InviteEdge
	err := gorm.ErrRecordNotFound
	if inviteeExternal != "" {
		err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("site_id = ? AND invitee_provider = ? AND invitee_external_id = ?", AgentSiteID(), req.Source, inviteeExternal).First(&edge).Error
	}
	if errors.Is(err, gorm.ErrRecordNotFound) && inviteeID > 0 {
		err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("site_id = ? AND invitee_user_id = ?", AgentSiteID(), inviteeID).First(&edge).Error
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		edge = model.InviteEdge{SiteId: AgentSiteID(), CampaignId: campaign.Id, InviteLinkId: inviteLinkID(link), InviterUserId: inviterID, InviteeUserId: inviteeID, InviteeProvider: req.Source, InviteeExternalId: inviteeExternal, Stage: stage, Status: status, CreatedAt: now, UpdatedAt: now}
		if err := tx.Create(&edge).Error; err != nil {
			return nil, err
		}
		return &edge, nil
	}
	if err != nil {
		return nil, err
	}
	updates := map[string]interface{}{"updated_at": now}
	if edge.CampaignId <= 0 {
		updates["campaign_id"] = campaign.Id
		edge.CampaignId = campaign.Id
	}
	if edge.InviteLinkId <= 0 && link != nil {
		updates["invite_link_id"] = link.Id
		edge.InviteLinkId = link.Id
	}
	if edge.InviterUserId <= 0 {
		updates["inviter_user_id"] = inviterID
		edge.InviterUserId = inviterID
	}
	if edge.InviteeUserId <= 0 && inviteeID > 0 {
		updates["invitee_user_id"] = inviteeID
		edge.InviteeUserId = inviteeID
	}
	if edge.Stage == "" || edge.Stage == "join" || stage == "verified" {
		updates["stage"] = stage
		edge.Stage = stage
	}
	if edge.Status == "" || edge.Status == "pending" || status == "verified" {
		updates["status"] = status
		edge.Status = status
	}
	if len(updates) > 1 {
		if err := tx.Model(&model.InviteEdge{}).Where("id = ?", edge.Id).Updates(updates).Error; err != nil {
			return nil, err
		}
	}
	return &edge, nil
}

func findOrCreateClaimableInviteEdgeTx(tx *gorm.DB, req AgentChatOpsInviteRequest, inviteeID int, inviteeExternal string) (*model.InviteCampaign, *model.InviteEdge, error) {
	campaign, link, inviterID, err := resolveInviteCampaignLinkAndInviterTx(tx, req)
	if err != nil {
		return nil, nil, err
	}
	var edge model.InviteEdge
	found := false
	if inviteeExternal != "" {
		err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("site_id = ? AND invitee_provider = ? AND invitee_external_id = ?", AgentSiteID(), req.Source, inviteeExternal).First(&edge).Error
		if err == nil {
			found = true
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, err
		}
	}
	if !found && inviteeID > 0 {
		err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("site_id = ? AND invitee_user_id = ?", AgentSiteID(), inviteeID).First(&edge).Error
		if err == nil {
			found = true
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, err
		}
	}
	if found {
		return campaign, &edge, nil
	}
	if inviterID <= 0 {
		var u struct{ InviterId int }
		if err := tx.Model(&model.User{}).Select("inviter_id").Where("id = ?", inviteeID).First(&u).Error; err == nil && u.InviterId > 0 {
			inviterID = u.InviterId
			_ = tx.Create(&model.InviteEvent{SiteId: AgentSiteID(), CampaignId: campaign.Id, EventType: "fallback_inviter_id", Provider: req.Source, ExternalUserId: inviteeExternal, UserId: inviteeID, GroupId: req.RoomID, MetadataJson: inviteMetadataJSONWith(req, map[string]any{"legacy_inviter_id": inviterID}), CreatedAt: time.Now().Unix()}).Error
		}
	}
	if inviterID <= 0 {
		return campaign, nil, nil
	}
	edgePtr, errp := upsertInviteEdgeTx(tx, campaign, link, req, inviterID, inviteeID, inviteeExternal, "verified", "verified")
	return campaign, edgePtr, errp
}

func payInviteRewardClaimTx(tx *gorm.DB, req AgentChatOpsInviteRequest, campaign *model.InviteCampaign, edge *model.InviteEdge, stage string, rewardUserID int, quota int, pool string, maxPerUserDay int) (AgentChatOpsInviteClaim, error) {
	out := AgentChatOpsInviteClaim{Stage: stage, RewardUserID: rewardUserID, Quota: quota, Status: "skipped"}
	if rewardUserID <= 0 || quota <= 0 {
		return out, nil
	}
	now := time.Now().Unix()
	var claim model.InviteRewardClaim
	claimFound := false
	if err := tx.Where("site_id = ? AND invite_edge_id = ? AND reward_stage = ?", AgentSiteID(), edge.Id, stage).First(&claim).Error; err == nil {
		claimFound = true
		out.ClaimID = claim.Id
		out.OpsFundLedgerID = claim.OpsFundLedgerId
		if strings.TrimSpace(claim.Status) == "paid" {
			out.Status = "already_paid"
			return out, nil
		}
		if strings.TrimSpace(claim.Status) == "blocked_daily_limit" {
			out.Status = claim.Status
			out.Error = claim.Error
			return out, nil
		}
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return out, err
	}
	if maxPerUserDay > 0 && !claimFound {
		var rewardUser model.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id").Where("id = ?", rewardUserID).First(&rewardUser).Error; err != nil {
			return out, err
		}
		// Another node may have created this claim while this transaction was
		// waiting for the per-user serialization lock. Reload before counting so
		// an already-paid edge is replayed instead of reported as rate-limited.
		var concurrentClaim model.InviteRewardClaim
		if err := tx.Where("site_id = ? AND invite_edge_id = ? AND reward_stage = ?", AgentSiteID(), edge.Id, stage).First(&concurrentClaim).Error; err == nil {
			claim = concurrentClaim
			claimFound = true
			out.ClaimID = claim.Id
			out.OpsFundLedgerID = claim.OpsFundLedgerId
			switch strings.TrimSpace(claim.Status) {
			case "paid":
				out.Status = "already_paid"
				return out, nil
			case "blocked_daily_limit":
				out.Status = claim.Status
				out.Error = claim.Error
				return out, nil
			}
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return out, err
		}
	}
	if maxPerUserDay > 0 && !claimFound {
		dayStart := model.AgentBusinessDayStartAt(time.Now()).Unix()
		var paidToday int64
		if err := tx.Model(&model.InviteRewardClaim{}).
			Where("site_id = ? AND reward_user_id = ? AND status IN ? AND created_at >= ?", AgentSiteID(), rewardUserID, []string{"pending", "paid"}, dayStart).
			Count(&paidToday).Error; err != nil {
			return out, err
		}
		if paidToday >= int64(maxPerUserDay) {
			claim = model.InviteRewardClaim{SiteId: AgentSiteID(), CampaignId: campaign.Id, InviteEdgeId: edge.Id, RewardStage: stage, RewardUserId: rewardUserID, Quota: quota, Status: "blocked_daily_limit", Error: fmt.Sprintf("invite daily reward limit reached: %d", maxPerUserDay), CreatedAt: now, UpdatedAt: now}
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&claim).Error; err != nil {
				return out, err
			}
			out.ClaimID = claim.Id
			out.Status = claim.Status
			out.Error = claim.Error
			return out, nil
		}
	}
	benefitExternalID := firstNonEmptyAgent(req.InviteeExternalID, req.InviterExternalID, req.UserExternalID)
	if strings.TrimSpace(stage) == "inviter" {
		benefitExternalID = firstNonEmptyAgent(req.InviterExternalID, req.UserExternalID, req.InviteeExternalID)
	}
	benefitType := inviteRewardSourceType(stage)
	if ok, reason := CanReceiveCommunityBenefit(rewardUserID, req.Source, req.RoomID, benefitExternalID, benefitType); !ok {
		out.Status = "blocked_membership"
		out.Error = firstNonEmptyAgent(reason, "membership benefit denied")
		if !claimFound {
			claim = model.InviteRewardClaim{SiteId: AgentSiteID(), CampaignId: campaign.Id, InviteEdgeId: edge.Id, RewardStage: stage, RewardUserId: rewardUserID, Quota: quota, Status: out.Status, Error: out.Error, CreatedAt: now, UpdatedAt: now}
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&claim).Error; err != nil {
				return out, err
			}
			if claim.Id == 0 {
				if err := tx.Where("site_id = ? AND invite_edge_id = ? AND reward_stage = ?", AgentSiteID(), edge.Id, stage).First(&claim).Error; err != nil {
					return out, err
				}
			}
			out.ClaimID = claim.Id
			return out, nil
		}
		if err := tx.Model(&model.InviteRewardClaim{}).Where("id = ?", claim.Id).Updates(map[string]interface{}{"status": out.Status, "error": out.Error, "updated_at": now}).Error; err != nil {
			return out, err
		}
		out.ClaimID = claim.Id
		return out, nil
	}
	if !claimFound {
		claim = model.InviteRewardClaim{SiteId: AgentSiteID(), CampaignId: campaign.Id, InviteEdgeId: edge.Id, RewardStage: stage, RewardUserId: rewardUserID, Quota: quota, Status: "pending", CreatedAt: now, UpdatedAt: now}
		res := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&claim)
		if res.Error != nil {
			return out, res.Error
		}
		if res.RowsAffected == 0 {
			if err := tx.Where("site_id = ? AND invite_edge_id = ? AND reward_stage = ?", AgentSiteID(), edge.Id, stage).First(&claim).Error; err != nil {
				return out, err
			}
			if strings.TrimSpace(claim.Status) == "paid" {
				out.ClaimID = claim.Id
				out.OpsFundLedgerID = claim.OpsFundLedgerId
				out.Status = "already_paid"
				return out, nil
			}
		}
	} else if strings.TrimSpace(claim.Status) != "pending" {
		if err := tx.Model(&model.InviteRewardClaim{}).Where("id = ?", claim.Id).Updates(map[string]interface{}{"status": "pending", "error": "", "updated_at": now}).Error; err != nil {
			return out, err
		}
		claim.Status = "pending"
		claim.Error = ""
	}
	out.ClaimID = claim.Id
	idem := fmt.Sprintf("invite:%s:%d:%s:%d", AgentSiteID(), edge.Id, stage, rewardUserID)
	remark := fmt.Sprintf("invite %s reward edge=%d campaign=%s", stage, edge.Id, campaign.CampaignCode)
	meta := inviteMetadataJSONWith(req, map[string]any{"edge_id": edge.Id, "campaign_id": campaign.Id, "reward_stage": stage, "reward_user_id": rewardUserID})
	if err := model.GrantQuotaFromBudgetPoolWithSourceTx(tx, rewardUserID, pool, quota, idem, remark, inviteRewardSourceType(stage), meta); err != nil {
		status := "failed"
		if errors.Is(err, model.ErrOpsFundQuotaInsufficient) {
			status = "degraded_fund"
		} else if errors.Is(err, model.ErrBudgetPoolQuotaInsufficient) {
			status = "degraded_budget"
		}
		out.Status = status
		out.Error = err.Error()
		if updateErr := tx.Model(&model.InviteRewardClaim{}).Where("id = ?", claim.Id).Updates(map[string]interface{}{"status": status, "error": err.Error(), "updated_at": now}).Error; updateErr != nil {
			return out, updateErr
		}
		return out, nil
	}
	var ledger model.OpsFundLedger
	ledgerID := 0
	if err := tx.Where("site_id = ? AND idempotency_key = ?", AgentSiteID(), idem).First(&ledger).Error; err == nil {
		ledgerID = ledger.Id
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return out, err
	}
	if err := tx.Model(&model.InviteRewardClaim{}).Where("id = ?", claim.Id).Updates(map[string]interface{}{"status": "paid", "ops_fund_ledger_id": ledgerID, "error": "", "updated_at": now}).Error; err != nil {
		return out, err
	}
	out.ClaimID = claim.Id
	out.OpsFundLedgerID = ledgerID
	out.Status = "paid"
	return out, nil
}

func inviteRewardSourceType(stage string) string {
	switch strings.TrimSpace(stage) {
	case "inviter":
		return "invite_inviter_reward"
	case "invitee":
		return "invite_invitee_reward"
	default:
		return "invite_reward"
	}
}

type inviteRewardPolicy struct {
	InviterQuota  int
	InviteeQuota  int
	BudgetPool    string
	MaxPerUserDay int
}

func inviteRewardConfigTx(tx *gorm.DB, req AgentChatOpsInviteRequest) (inviteRewardPolicy, error) {
	policy := inviteRewardPolicy{InviterQuota: 1500000, InviteeQuota: 750000, BudgetPool: "community", MaxPerUserDay: 10}
	gamePolicy, err := resolveEffectiveGroupGamePolicyTx(tx, AgentSiteID(), req.Source, req.RoomID, "invite")
	if err != nil {
		return policy, err
	}
	rules := gamePolicy.Rules
	if rules == nil {
		rules = map[string]any{}
	}
	if cfg, ok := chatOpsGroupConfigByScopeFromDB(tx, req.Source, req.RoomID); ok {
		chatOpsRules := chatOpsNestedMap(chatOpsJSONMap(cfg.RuleJson)["invite"])
		rules = mergeGamePolicyMaps(rules, chatOpsRules)
		if cfg.InviteRewardQuota > 0 {
			policy.InviterQuota = cfg.InviteRewardQuota
		}
		if cfg.InviteeRewardQuota > 0 {
			policy.InviteeQuota = cfg.InviteeRewardQuota
		}
	}
	if value, ok := gamePolicyInt(rules, "inviter_reward_quota"); ok {
		policy.InviterQuota = value
	} else if value, ok := gamePolicyInt(rules, "reward_quota"); ok {
		policy.InviterQuota = value
	}
	if value, ok := gamePolicyInt(rules, "invitee_reward_quota"); ok {
		policy.InviteeQuota = value
	}
	if value, ok := gamePolicyInt(rules, "max_per_user_day"); ok {
		policy.MaxPerUserDay = value
	}
	if policy.InviterQuota < 0 || policy.InviteeQuota < 0 || policy.MaxPerUserDay < 0 {
		return policy, errors.New("invite reward policy contains a negative value")
	}
	policy.BudgetPool, err = effectivePolicyBudgetPool(gamePolicy, rules, "community")
	return policy, err
}

func defaultInviteCampaignCode(req AgentChatOpsInviteRequest) string {
	room := safeInviteCodePart(req.RoomID)
	if room == "" {
		room = "private"
	}
	return strings.Join([]string{"chatops", safeInviteCodePart(AgentSiteID()), safeInviteCodePart(req.Source), room}, "-")
}

func deterministicInviteCode(site string, campaign string, inviterID int) string {
	seed := fmt.Sprintf("%s:%s:%d", site, campaign, inviterID)
	sum := sha256.Sum256([]byte(seed))
	return fmt.Sprintf("%s-%d-%s", safeInviteCodePart(site), inviterID, hex.EncodeToString(sum[:])[:10])
}

func safeInviteCodePart(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	if v == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range v {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else if r == '-' || r == '_' {
			b.WriteRune('-')
		}
	}
	out := strings.Trim(b.String(), "-")
	if len(out) > 48 {
		out = out[:48]
	}
	return out
}

func inviteLinkID(link *model.InviteLink) int {
	if link == nil {
		return 0
	}
	return link.Id
}

func inviteMetadataJSON(req AgentChatOpsInviteRequest) string {
	return inviteMetadataJSONWith(req, nil)
}

func inviteMetadataJSONWith(req AgentChatOpsInviteRequest, extra map[string]any) string {
	m := map[string]any{"source": req.Source, "room_id": req.RoomID, "message_id": req.MessageID, "user_external_id": req.UserExternalID, "username": req.Username, "invite_code": req.InviteCode, "invite_url": req.InviteURL, "metadata": req.Metadata}
	for k, v := range extra {
		m[k] = v
	}
	raw, _ := json.Marshal(m)
	return string(raw)
}
