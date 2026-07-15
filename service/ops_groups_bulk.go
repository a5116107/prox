package service

import (
	"errors"
	"strings"
	"time"
)

type OpsGroupBulkSaveRequest struct {
	SiteID          string                `json:"site_id"`
	Items           []OpsGroupSaveRequest `json:"items"`
	ContinueOnError *bool                 `json:"continue_on_error"`
}

type OpsGroupBulkSaveFailure struct {
	Index     int    `json:"index"`
	GroupID   string `json:"group_id"`
	GroupName string `json:"group_name"`
	Reason    string `json:"reason"`
}

type OpsGroupBulkSaveResult struct {
	SiteID       string                    `json:"site_id"`
	Total        int                       `json:"total"`
	CreatedCount int                       `json:"created_count"`
	FailedCount  int                       `json:"failed_count"`
	Created      []*OpsGroupView           `json:"created"`
	Failed       []OpsGroupBulkSaveFailure `json:"failed"`
	GeneratedAt  int64                     `json:"generated_at"`
}

func CreateOpsGroupsBulk(siteID string, req OpsGroupBulkSaveRequest) (*OpsGroupBulkSaveResult, error) {
	resolvedSiteID := resolveOpsGroupRequestSiteID(siteID, req.SiteID)
	if len(req.Items) == 0 {
		return nil, errors.New("items is required")
	}

	continueOnError := req.ContinueOnError == nil || *req.ContinueOnError
	result := &OpsGroupBulkSaveResult{
		SiteID:      resolvedSiteID,
		Total:       len(req.Items),
		Created:     make([]*OpsGroupView, 0, len(req.Items)),
		Failed:      make([]OpsGroupBulkSaveFailure, 0),
		GeneratedAt: time.Now().Unix(),
	}

	for idx, item := range req.Items {
		if strings.TrimSpace(item.SiteID) == "" {
			item.SiteID = resolvedSiteID
		}
		view, err := CreateOpsGroup(resolvedSiteID, item)
		if err != nil {
			result.Failed = append(result.Failed, OpsGroupBulkSaveFailure{
				Index:     idx,
				GroupID:   strings.TrimSpace(derefOpsString(item.GroupID)),
				GroupName: strings.TrimSpace(derefOpsString(item.GroupName)),
				Reason:    err.Error(),
			})
			if !continueOnError {
				break
			}
			continue
		}
		result.Created = append(result.Created, view)
	}

	result.CreatedCount = len(result.Created)
	result.FailedCount = len(result.Failed)
	return result, nil
}

func derefOpsString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
