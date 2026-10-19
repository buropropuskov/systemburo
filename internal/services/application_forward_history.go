package services

import (
	"context"
	"encoding/json"
	"strings"

	"gorm.io/gorm"
)

type forwardRecipientDisplay struct {
	forwardRecipientDetail
	DisplayName string `json:"display_name"`
}

// decodeForwardRecipientDetails accepts only the complete versioned schema.
// Pointer fields distinguish an explicit false from a missing historical fact.
// One malformed recipient makes this event unavailable instead of inventing a
// purpose or silently presenting only part of its audience.
func decodeForwardRecipientDetails(meta map[string]json.RawMessage) []forwardRecipientDetail {
	var version int
	if json.Unmarshal(meta["forward_schema_version"], &version) != nil || version != 2 {
		return nil
	}
	var rows []struct {
		UserID           *int    `json:"user_id"`
		Purpose          *string `json:"purpose"`
		RequiredApproval *bool   `json:"required_approval"`
		AccessGranted    *bool   `json:"access_granted"`
	}
	if json.Unmarshal(meta["recipient_details"], &rows) != nil {
		return nil
	}
	result := make([]forwardRecipientDetail, 0, len(rows))
	seen := make(map[int]bool)
	for _, row := range rows {
		if row.UserID == nil || *row.UserID <= 0 || row.Purpose == nil || row.RequiredApproval == nil || row.AccessGranted == nil || seen[*row.UserID] {
			return nil
		}
		if *row.Purpose != "approval" && *row.Purpose != "view" {
			return nil
		}
		if *row.Purpose == "view" && *row.RequiredApproval {
			return nil
		}
		if *row.Purpose == "approval" && !*row.AccessGranted {
			return nil
		}
		seen[*row.UserID] = true
		result = append(result, forwardRecipientDetail{UserID: *row.UserID, Purpose: *row.Purpose, RequiredApproval: *row.RequiredApproval, AccessGranted: *row.AccessGranted})
	}
	return result
}

// sanitizeForwardRecipientMetadata only changes response copies. Legacy names
// without stable IDs cannot safely be matched to a consent or display-name mask.
// They remain in storage, but are omitted from both history read surfaces.
func sanitizeForwardRecipientMetadata(ctx context.Context, db *gorm.DB, originals []json.RawMessage) ([]json.RawMessage, error) {
	metadata := make([]map[string]json.RawMessage, len(originals))
	details := make([][]forwardRecipientDetail, len(originals))
	ids := make([]int, 0)
	seen := make(map[int]bool)
	for i, original := range originals {
		if json.Unmarshal(original, &metadata[i]) != nil || metadata[i] == nil {
			metadata[i] = make(map[string]json.RawMessage)
		}
		details[i] = decodeForwardRecipientDetails(metadata[i])
		for _, detail := range details[i] {
			if detail.UserID > 0 && !seen[detail.UserID] {
				seen[detail.UserID] = true
				ids = append(ids, detail.UserID)
			}
		}
	}
	names := make(map[int]string)
	if len(ids) > 0 {
		type recipientRow struct {
			ID         int
			Username   string
			LastName   *string
			FirstName  *string
			MiddleName *string
		}
		var rows []recipientRow
		if err := db.WithContext(ctx).Table("users").Select("id, username, last_name, first_name, middle_name").Where("id IN ?", ids).Scan(&rows).Error; err != nil {
			return nil, err
		}
		masks, err := loadForwardNameMasks(ctx, db, ids)
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			name := strings.TrimSpace(formatFullName(row.LastName, row.FirstName, row.MiddleName))
			if name == "" {
				name = "@" + row.Username
			}
			names[row.ID] = maskName(masks, &row.ID, name)
		}
	}
	result := make([]json.RawMessage, len(metadata))
	for i, meta := range metadata {
		resolved := make([]forwardRecipientDisplay, 0, len(details[i]))
		displayNames := make([]string, 0, len(details[i]))
		for _, detail := range details[i] {
			if detail.UserID <= 0 {
				continue
			}
			name, found := names[detail.UserID]
			if !found {
				name = "Сведения о получателе недоступны"
			}
			resolved = append(resolved, forwardRecipientDisplay{forwardRecipientDetail: detail, DisplayName: name})
			displayNames = append(displayNames, name)
		}
		meta["recipients"], _ = json.Marshal(displayNames)
		meta["recipient_details"], _ = json.Marshal(resolved)
		meta["recipient_details_available"], _ = json.Marshal(len(resolved) > 0)
		result[i], _ = json.Marshal(meta)
	}
	return result, nil
}

// loadForwardNameMasks shares the viewer-aware display policy and keeps errors
// visible. Only stable recipient IDs may be resolved; legacy names are untouched.
func loadForwardNameMasks(ctx context.Context, db *gorm.DB, ids []int) (map[int]string, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	masks, err := loadNameMasks(ctx, db)
	if err != nil {
		return nil, err
	}
	result := make(map[int]string)
	for _, id := range ids {
		if name, ok := masks[id]; ok {
			result[id] = name
		}
	}
	return result, nil
}
