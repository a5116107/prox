package model

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrIdentityBindingConflict = errors.New("external identity is already bound to another user")

func normalizeIdentityProvider(provider string) string {
	provider = strings.ToLower(strings.TrimSpace(provider))
	switch provider {
	case "telegram":
		return "tg"
	case "discord", "dc", "hhhl":
		return "community"
	case "":
		return "qq"
	default:
		return provider
	}
}

func UpsertUserIdentityBindingRowWithTx(tx *gorm.DB, binding *UserIdentityBinding) error {
	if binding == nil {
		return errors.New("identity binding is nil")
	}
	if tx == nil {
		tx = DB
	}
	binding.SiteId = CanonicalSiteID(binding.SiteId)
	binding.Provider = normalizeIdentityProvider(binding.Provider)
	binding.ExternalUserId = strings.TrimSpace(binding.ExternalUserId)
	binding.Username = strings.TrimSpace(binding.Username)
	if binding.SiteId == "" || binding.Provider == "" || binding.ExternalUserId == "" || binding.UserId <= 0 {
		return errors.New("invalid identity binding")
	}
	now := time.Now().Unix()
	if binding.Status == "" {
		binding.Status = "active"
	}
	if binding.BoundAt == 0 {
		binding.BoundAt = now
	}
	binding.LastSeenAt = now
	if binding.CreatedAt == 0 {
		binding.CreatedAt = now
	}
	binding.UpdatedAt = now

	var existing UserIdentityBinding
	err := lockForUpdate(tx).Where(
		"site_id = ? AND provider = ? AND external_user_id = ?",
		binding.SiteId,
		binding.Provider,
		binding.ExternalUserId,
	).First(&existing).Error
	if err == nil {
		if existing.UserId != binding.UserId {
			return fmt.Errorf("%w: site=%s provider=%s external_user_id=%s existing_user_id=%d requested_user_id=%d",
				ErrIdentityBindingConflict,
				binding.SiteId,
				binding.Provider,
				binding.ExternalUserId,
				existing.UserId,
				binding.UserId,
			)
		}
		return tx.Model(&UserIdentityBinding{}).Where("id = ?", existing.Id).Updates(map[string]any{
			"username":     binding.Username,
			"status":       binding.Status,
			"bound_at":     binding.BoundAt,
			"last_seen_at": binding.LastSeenAt,
			"updated_at":   now,
		}).Error
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	result := tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "site_id"}, {Name: "provider"}, {Name: "external_user_id"}},
		DoNothing: true,
	}).Create(binding)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected > 0 {
		return nil
	}

	if err := tx.Where(
		"site_id = ? AND provider = ? AND external_user_id = ?",
		binding.SiteId,
		binding.Provider,
		binding.ExternalUserId,
	).First(&existing).Error; err != nil {
		return err
	}
	if existing.UserId != binding.UserId {
		return fmt.Errorf("%w: site=%s provider=%s external_user_id=%s existing_user_id=%d requested_user_id=%d",
			ErrIdentityBindingConflict,
			binding.SiteId,
			binding.Provider,
			binding.ExternalUserId,
			existing.UserId,
			binding.UserId,
		)
	}
	return nil
}

func UpsertUserIdentityBindingRow(binding *UserIdentityBinding) error {
	return UpsertUserIdentityBindingRowWithTx(DB, binding)
}

func UpsertUserIdentityBindingWithTx(tx *gorm.DB, siteID string, userID int, provider, externalUserID, username string) error {
	return UpsertUserIdentityBindingRowWithTx(tx, &UserIdentityBinding{
		SiteId:         CanonicalSiteID(siteID),
		UserId:         userID,
		Provider:       provider,
		ExternalUserId: externalUserID,
		Username:       username,
		Status:         "active",
	})
}

func UpsertUserIdentityBinding(siteID string, userID int, provider, externalUserID, username string) error {
	return UpsertUserIdentityBindingWithTx(DB, siteID, userID, provider, externalUserID, username)
}

func GetUserIdentityBindingByExternal(siteID, provider, externalUserID string) (*UserIdentityBinding, error) {
	var row UserIdentityBinding
	err := DB.Where(
		"site_id = ? AND provider = ? AND external_user_id = ? AND status = ?",
		CanonicalSiteID(siteID),
		normalizeIdentityProvider(provider),
		strings.TrimSpace(externalUserID),
		"active",
	).First(&row).Error
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func GetLatestUserIdentityBindingByUser(siteID, provider string, userID int) (*UserIdentityBinding, error) {
	var row UserIdentityBinding
	err := DB.Where(
		"site_id = ? AND provider = ? AND user_id = ? AND status = ?",
		CanonicalSiteID(siteID),
		normalizeIdentityProvider(provider),
		userID,
		"active",
	).Order("last_seen_at desc, updated_at desc, id desc").First(&row).Error
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func ListActiveUserIdentityBindingsByUsername(siteID, username string, providers ...string) ([]UserIdentityBinding, error) {
	siteID = CanonicalSiteID(siteID)
	username = strings.TrimSpace(username)
	if siteID == "" || username == "" {
		return []UserIdentityBinding{}, nil
	}
	query := DB.Where("site_id = ? AND LOWER(username) = ? AND status = ?", siteID, strings.ToLower(username), "active")
	if len(providers) > 0 {
		normalized := make([]string, 0, len(providers))
		seen := make(map[string]struct{}, len(providers))
		for _, provider := range providers {
			provider = normalizeIdentityProvider(provider)
			if provider == "" {
				continue
			}
			if _, ok := seen[provider]; ok {
				continue
			}
			seen[provider] = struct{}{}
			normalized = append(normalized, provider)
		}
		if len(normalized) > 0 {
			query = query.Where("provider IN ?", normalized)
		}
	}
	var rows []UserIdentityBinding
	err := query.Order("updated_at desc, id desc").Find(&rows).Error
	return rows, err
}

func ListUserIdentityBindings(siteID, provider string) ([]UserIdentityBinding, error) {
	var rows []UserIdentityBinding
	tx := DB.Order("updated_at desc, id desc")
	if strings.TrimSpace(siteID) != "" {
		tx = tx.Where("site_id = ?", CanonicalSiteID(siteID))
	}
	if strings.TrimSpace(provider) != "" {
		tx = tx.Where("provider = ?", normalizeIdentityProvider(provider))
	}
	err := tx.Find(&rows).Error
	return rows, err
}

func FindUserIdentityBinding(siteID, provider, externalUserID string) (*UserIdentityBinding, error) {
	return GetUserIdentityBindingByExternal(siteID, provider, externalUserID)
}

func IsIdentityBindingNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}
