package services

import "context"

// Apply the same identity policy to workbook cells and the download filename.
// The context is private to this generation: stored records are never rewritten.
func (s *attachmentBlankService) applyBlankNamePolicy(ctx context.Context, bctx *BlankContext, original bool) error {
	var masks map[int]string
	var err error
	if original {
		masks, err = loadConsentMasks(ctx, s.db)
	} else {
		masks, err = loadNameMasks(ctx, s.db)
	}
	if err != nil {
		return err
	}
	// Withdrawal survives disabling the prompt and has no administrator exception.
	ids := make([]int, 0, len(bctx.Approvers)+1)
	if bctx.Sender != nil {
		ids = append(ids, bctx.Sender.ID)
	}
	for _, approver := range bctx.Approvers {
		ids = append(ids, approver.UserID)
	}
	if len(ids) > 0 {
		var withdrawn []struct {
			ID       int
			Username string
		}
		err = s.db.WithContext(ctx).Table("users").Select("users.id, users.username").
			Where("users.id IN ?", ids).
			Where(`EXISTS (SELECT 1 FROM pd_consents r WHERE r.user_id = users.id
			AND r.consent_type = ? AND r.revoked_at IS NOT NULL)
			AND NOT EXISTS (SELECT 1 FROM pd_consents c WHERE c.user_id = users.id
			AND c.consent_type = ? AND c.granted = true AND c.revoked_at IS NULL)`,
				ConsentTypePDProcessing, ConsentTypePDProcessing).Scan(&withdrawn).Error
		if err != nil {
			return nameMaskUnavailable(err)
		}
		if masks == nil {
			masks = make(map[int]string)
		}
		for _, subject := range withdrawn {
			masks[subject.ID] = "@" + subject.Username
		}
	}
	applyBlankNameMasks(bctx, masks)
	return nil
}

func applyBlankNameMasks(bctx *BlankContext, masks map[int]string) {
	if bctx.Sender != nil && isMasked(masks, bctx.Sender.ID) {
		sender := *bctx.Sender
		label := masks[sender.ID]
		sender.LastName, sender.FirstName, sender.MiddleName = &label, nil, nil
		sender.Email, sender.Phone = nil, nil
		bctx.Sender = &sender
	}
	for i := range bctx.Approvers {
		a := &bctx.Approvers[i]
		if label, ok := masks[a.UserID]; ok {
			a.LastName, a.FirstName, a.MiddleName = label, "", ""
		}
	}
}
