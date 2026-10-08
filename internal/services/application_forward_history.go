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

// loadForwardNameMasks mirrors the existing display policy, but keeps query
// errors visible to this privacy-sensitive read instead of treating them as an
// empty mask. Shared consent/permission helpers are deliberately unchanged.
func loadForwardNameMasks(ctx context.Context, db *gorm.DB, ids []int) (map[int]string, error) {
	type settingRow struct {
		Key   string
		Value string
	}
	var settings []settingRow
	if err := db.WithContext(ctx).Table("system_settings").Select("key, value").Where("key IN ?", []string{pdConsentRequiredKey, pdConsentTextKey}).Scan(&settings).Error; err != nil {
		return nil, err
	}
	var required bool
	var text string
	for _, setting := range settings {
		if setting.Key == pdConsentRequiredKey {
			required = setting.Value == "true"
		}
		if setting.Key == pdConsentTextKey {
			text = setting.Value
		}
	}
	masks := make(map[int]string)
	if required && hasVisibleText(text) {
		type consentRow struct {
			ID       int
			Username string
		}
		var rows []consentRow
		if err := db.WithContext(ctx).Table("users").Select("users.id, users.username").Where("users.id IN ?", ids).Where(gatedUsersWhere).Where(`NOT EXISTS (
			SELECT 1 FROM pd_consents c WHERE c.user_id = users.id AND c.consent_type = ?
			AND c.granted = true AND c.revoked_at IS NULL
		)`, ConsentTypePDProcessing).Scan(&rows).Error; err != nil {
			return nil, err
		}
		for _, row := range rows {
			masks[row.ID] = "@" + row.Username
		}
	}
	type approverRow struct {
		UserID      int
		DisplayName string
	}
	var approvers []approverRow
	if err := db.WithContext(ctx).Table("application_approvers").Select("user_id, display_name").Where("user_id IN ?", ids).Where("display_name IS NOT NULL AND TRIM(display_name) <> ''").Scan(&approvers).Error; err != nil {
		return nil, err
	}
	for _, row := range approvers {
		masks[row.UserID] = row.DisplayName
	}
	return masks, nil
}
