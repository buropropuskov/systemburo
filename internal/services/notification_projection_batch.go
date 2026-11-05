package services

import (
	"context"
	"sort"
	"strings"
)

// Bound every IN list independently of legacy feed length. Results, including
// missing/denied objects, remain request-local so repeated references cost no
// additional SQL and policy cannot outlive the request that resolved it fresh.
const notificationProjectionBatchSize = 100

type notificationProjectionEntry struct {
	index    int
	ref      notificationReference
	groupKey *string
}

type notificationApplicationProjection struct {
	Number  string
	Allowed bool
}
type notificationSupplementProjection struct{ ID, ApplicationID, Number int }
type notificationQuestionProjection struct{ ID, ApplicationID, AuthorUserID int }
type notificationAnswerProjection struct{ ID, QuestionID, AuthorUserID int }
type notificationFeedbackProjection struct{ ID, UserID int }

type notificationProjectionBatch struct {
	applications map[int]notificationApplicationProjection
	supplements  map[int]notificationSupplementProjection
	questions    map[int]notificationQuestionProjection
	answers      map[int]notificationAnswerProjection
	feedback     map[int]notificationFeedbackProjection
	entities     map[string]map[int]int
	existence    map[string]map[int]bool
}

func newNotificationProjectionBatch() *notificationProjectionBatch {
	return &notificationProjectionBatch{
		applications: map[int]notificationApplicationProjection{},
		supplements:  map[int]notificationSupplementProjection{},
		questions:    map[int]notificationQuestionProjection{},
		answers:      map[int]notificationAnswerProjection{},
		feedback:     map[int]notificationFeedbackProjection{},
		entities:     map[string]map[int]int{"cars": {}, "employees": {}},
		existence:    map[string]map[int]bool{"organizations": {}, "companies": {}, "news": {}, "documents": {}},
	}
}

func notificationProjectionIDs(ids map[int]struct{}) []int {
	result := make([]int, 0, len(ids))
	for id := range ids {
		if id > 0 {
			result = append(result, id)
		}
	}
	sort.Ints(result)
	return result
}

func notificationProjectionTypeAllowed(p *notificationProjectionPolicy, ref notificationReference) bool {
	meta, known := NotificationTypeMeta(ref.Type)
	if !known || meta.Permission != "" && !p.permissions.Has(meta.Permission) {
		return false
	}
	return (ref.Type != NotificationTypeNewsPublished && ref.Type != NotificationTypeDocumentPublished) || p.permissions.Has(KeyPageNews)
}

func (s *notificationService) preloadNotificationProjection(ctx context.Context, p *notificationProjectionPolicy, entries []notificationProjectionEntry) error {
	b := p.batch
	refs := make([]notificationReference, 0, len(entries))
	for _, entry := range entries {
		if notificationProjectionTypeAllowed(p, entry.ref) {
			refs = append(refs, entry.ref)
		}
	}
	// Resolve technical entity->application links before application permissions.
	for _, kind := range []string{"cars", "employees"} {
		ids := map[int]struct{}{}
		for _, ref := range refs {
			if (ref.Type == NotificationTypeTrashRestored || ref.Type == NotificationTypeApplicationDatesChanged && ref.Variant == "entity") && ref.EntityKind == kind {
				if _, known := b.entities[kind][ref.EntityID]; !known {
					ids[ref.EntityID] = struct{}{}
				}
			}
		}
		values := notificationProjectionIDs(ids)
		if len(values) == 0 {
			continue
		}
		for _, id := range values {
			b.entities[kind][id] = 0
		}
		var rows []struct{ ID, ApplicationID int }
		if err := s.db.WithContext(ctx).Table(kind+" e").Select("e.id,a.application_id").
			Joins("JOIN attachments a ON a.id = e.attachment_id").Where("e.id IN ?", values).Find(&rows).Error; err != nil {
			return nameMaskUnavailable(err)
		}
		for _, row := range rows {
			b.entities[kind][row.ID] = row.ApplicationID
		}
	}
	ids := map[int]struct{}{}
	for _, ref := range refs {
		if ref.ApplicationID > 0 {
			if _, known := b.applications[ref.ApplicationID]; !known {
				ids[ref.ApplicationID] = struct{}{}
			}
		}
		if ref.Type == NotificationTypeTrashRestored {
			id := b.entities[ref.EntityKind][ref.EntityID]
			if id > 0 {
				if _, known := b.applications[id]; !known {
					ids[id] = struct{}{}
				}
			}
		}
	}
	if err := s.preloadNotificationApplications(ctx, p, notificationProjectionIDs(ids)); err != nil {
		return err
	}
	accessible := refs[:0]
	for _, ref := range refs {
		if ref.ApplicationID > 0 && !b.applications[ref.ApplicationID].Allowed {
			continue
		}
		accessible = append(accessible, ref)
	}
	refs = accessible
	if err := s.preloadNotificationRelations(ctx, b, refs); err != nil {
		return err
	}
	if err := s.preloadNotificationObjects(ctx, b, refs); err != nil {
		return err
	}
	actorIDs := map[int]struct{}{}
	for _, ref := range refs {
		if ref.ActorID > 0 && b.referenceReadable(p, ref) {
			if _, known := p.names[ref.ActorID]; !known {
				actorIDs[ref.ActorID] = struct{}{}
			}
		}
	}
	return s.preloadNotificationActors(ctx, p, notificationProjectionIDs(actorIDs))
}

func (s *notificationService) preloadNotificationApplications(ctx context.Context, p *notificationProjectionPolicy, ids []int) error {
	// A row may identify both an explicit application and a restored entity's
	// current application. Keep each SQL statement bounded even in that case.
	for start := 0; start < len(ids); start += notificationProjectionBatchSize {
		end := min(start+notificationProjectionBatchSize, len(ids))
		values := ids[start:end]
		for _, id := range values {
			p.batch.applications[id] = notificationApplicationProjection{}
		}
		var rows []struct {
			ID                int
			ApplicationNumber *string
			Allowed           bool
		}
		// Same ownership/banned/super/approver/responsible/viewer semantics as the
		// former per-row reader; permissions come from the current effective viewer.
		if err := s.db.WithContext(ctx).Table("applications a").Select(`a.id,a.application_number,
		(a.sender_user_id = ? OR (? AND (? OR
		 EXISTS(SELECT 1 FROM application_approvers ap WHERE ap.user_id = ?) OR
		 EXISTS(SELECT 1 FROM application_responsible_users aru WHERE aru.application_id = a.id AND aru.user_id = ?) OR
		 EXISTS(SELECT 1 FROM application_viewers av WHERE av.application_id = a.id AND av.user_id = ?)))) AS allowed`,
			p.viewerID, !p.viewer.IsBanned, p.viewer.IsSuperAdmin, p.viewerID, p.viewerID, p.viewerID).
			Where("a.id IN ?", values).Find(&rows).Error; err != nil {
			return nameMaskUnavailable(err)
		}
		for _, row := range rows {
			p.batch.applications[row.ID] = notificationApplicationProjection{Number: applicationNumberOrFallback(optionalString(row.ApplicationNumber), row.ID), Allowed: row.Allowed}
		}
	}
	return nil
}

func (s *notificationService) preloadNotificationRelations(ctx context.Context, b *notificationProjectionBatch, refs []notificationReference) error {
	supplementIDs, questionIDs, answerIDs := map[int]struct{}{}, map[int]struct{}{}, map[int]struct{}{}
	for _, ref := range refs {
		if ref.SupplementID > 0 {
			if _, known := b.supplements[ref.SupplementID]; !known {
				supplementIDs[ref.SupplementID] = struct{}{}
			}
		}
		if ref.QuestionID > 0 {
			if _, known := b.questions[ref.QuestionID]; !known {
				questionIDs[ref.QuestionID] = struct{}{}
			}
		}
		if ref.AnswerID > 0 {
			if _, known := b.answers[ref.AnswerID]; !known {
				answerIDs[ref.AnswerID] = struct{}{}
			}
		}
	}
	if ids := notificationProjectionIDs(supplementIDs); len(ids) > 0 {
		for _, id := range ids {
			b.supplements[id] = notificationSupplementProjection{}
		}
		var rows []notificationSupplementProjection
		if err := s.db.WithContext(ctx).Table("application_supplements").Select("id,application_id,number").Where("id IN ?", ids).Find(&rows).Error; err != nil {
			return nameMaskUnavailable(err)
		}
		for _, row := range rows {
			b.supplements[row.ID] = row
		}
	}
	if ids := notificationProjectionIDs(questionIDs); len(ids) > 0 {
		for _, id := range ids {
			b.questions[id] = notificationQuestionProjection{}
		}
		var rows []notificationQuestionProjection
		if err := s.db.WithContext(ctx).Table("application_questions").Select("id,application_id,author_user_id").Where("id IN ?", ids).Find(&rows).Error; err != nil {
			return nameMaskUnavailable(err)
		}
		for _, row := range rows {
			b.questions[row.ID] = row
		}
	}
	if ids := notificationProjectionIDs(answerIDs); len(ids) > 0 {
		for _, id := range ids {
			b.answers[id] = notificationAnswerProjection{}
		}
		var rows []notificationAnswerProjection
		if err := s.db.WithContext(ctx).Table("application_answers").Select("id,question_id,author_user_id").Where("id IN ?", ids).Find(&rows).Error; err != nil {
			return nameMaskUnavailable(err)
		}
		for _, row := range rows {
			b.answers[row.ID] = row
		}
	}
	return nil
}

func (s *notificationService) preloadNotificationObjects(ctx context.Context, b *notificationProjectionBatch, refs []notificationReference) error {
	for _, table := range []string{"organizations", "companies", "news", "documents", "feedback"} {
		ids := map[int]struct{}{}
		for _, ref := range refs {
			id := 0
			switch table {
			case "organizations", "companies":
				if (ref.Type == NotificationTypeDirectoryPending || ref.Type == NotificationTypeDirectoryResolved) && ref.DirectoryKind == table {
					id = ref.DirectoryID
				}
			case "news":
				if ref.Type == NotificationTypeNewsPublished {
					id = ref.NewsID
				}
			case "documents":
				if ref.Type == NotificationTypeDocumentPublished {
					id = ref.DocumentID
				}
			case "feedback":
				if ref.Type == NotificationTypeFeedbackCreated || ref.Type == NotificationTypeFeedbackAnswered {
					id = ref.FeedbackID
				}
			}
			if id <= 0 {
				continue
			}
			if table == "feedback" {
				if _, known := b.feedback[id]; !known {
					ids[id] = struct{}{}
				}
			} else if _, known := b.existence[table][id]; !known {
				ids[id] = struct{}{}
			}
		}
		values := notificationProjectionIDs(ids)
		if len(values) == 0 {
			continue
		}
		if table == "feedback" {
			for _, id := range values {
				b.feedback[id] = notificationFeedbackProjection{}
			}
			var rows []notificationFeedbackProjection
			if err := s.db.WithContext(ctx).Table(table).Select("id,user_id").Where("id IN ?", values).Find(&rows).Error; err != nil {
				return nameMaskUnavailable(err)
			}
			for _, row := range rows {
				b.feedback[row.ID] = row
			}
			continue
		}
		for _, id := range values {
			b.existence[table][id] = false
		}
		var rows []struct{ ID int }
		query := s.db.WithContext(ctx).Table(table).Select("id").Where("id IN ?", values)
		if table == "news" {
			query = query.Where("is_active = true")
		}
		if table == "documents" {
			query = query.Where("is_visible = true")
		}
		if err := query.Find(&rows).Error; err != nil {
			return nameMaskUnavailable(err)
		}
		for _, row := range rows {
			b.existence[table][row.ID] = true
		}
	}
	return nil
}

func (b *notificationProjectionBatch) referenceReadable(p *notificationProjectionPolicy, ref notificationReference) bool {
	if !notificationProjectionTypeAllowed(p, ref) {
		return false
	}
	if strings.HasPrefix(ref.Type, "application_") && ref.ApplicationID == 0 {
		return false
	}
	if ref.ApplicationID > 0 && !b.applications[ref.ApplicationID].Allowed {
		return false
	}
	if ref.SupplementID > 0 {
		row := b.supplements[ref.SupplementID]
		if row.ID == 0 || row.ApplicationID != ref.ApplicationID {
			return false
		}
	}
	if ref.QuestionID > 0 {
		row := b.questions[ref.QuestionID]
		if row.ID == 0 || row.ApplicationID != ref.ApplicationID || ref.Type == NotificationTypeApplicationQuestion && row.AuthorUserID != ref.ActorID {
			return false
		}
	}
	if ref.AnswerID > 0 {
		row := b.answers[ref.AnswerID]
		if row.ID == 0 || row.QuestionID != ref.QuestionID || row.AuthorUserID != ref.ActorID {
			return false
		}
	}
	switch ref.Type {
	case NotificationTypeApplicationDatesChanged:
		return ref.Variant != "entity" || b.entities[ref.EntityKind][ref.EntityID] == ref.ApplicationID
	case NotificationTypeDirectoryPending, NotificationTypeDirectoryResolved:
		return b.existence[ref.DirectoryKind][ref.DirectoryID]
	case NotificationTypeNewsPublished:
		return b.existence["news"][ref.NewsID]
	case NotificationTypeDocumentPublished:
		return b.existence["documents"][ref.DocumentID]
	case NotificationTypeFeedbackCreated:
		return b.feedback[ref.FeedbackID].ID > 0
	case NotificationTypeFeedbackAnswered:
		return b.feedback[ref.FeedbackID].ID > 0 && b.feedback[ref.FeedbackID].UserID == p.viewerID
	case NotificationTypeTrashRestored:
		id := b.entities[ref.EntityKind][ref.EntityID]
		return id > 0 && b.applications[id].Allowed
	default:
		return true
	}
}

// Only a current processing grant can supersede a processing revocation. A
// transfer grant, account role/activity or disabled display setting cannot do so.
// Keep this scoped to notification projection; other consent helpers are separate.
const notificationActorProcessingGrantSQL = `EXISTS(SELECT 1 FROM pd_consents ca WHERE ca.user_id = users.id AND ca.consent_type = 'pd_processing' AND ca.granted = true AND ca.revoked_at IS NULL)`
const notificationActorRevokedSQL = `NOT (` + notificationActorProcessingGrantSQL + `)
 AND EXISTS(SELECT 1 FROM pd_consents cr WHERE cr.user_id = users.id AND cr.consent_type = 'pd_processing' AND cr.revoked_at IS NOT NULL)`

// Preserve the common mask helper's gated/exempt/never-granted semantics. Only
// processing revocation above is unconditional for every actor and viewer role.
const notificationActorConsentMaskedSQL = `? AND (` + gatedUsersWhere + `) AND NOT (` + notificationActorProcessingGrantSQL + `)`
const notificationActorNameAllowedSQL = `NOT (` + notificationActorRevokedSQL + `) AND NOT (` + notificationActorConsentMaskedSQL + `)`

func (s *notificationService) preloadNotificationActors(ctx context.Context, p *notificationProjectionPolicy, ids []int) error {
	if len(ids) == 0 {
		return nil
	}
	for _, id := range ids {
		p.names[id] = ""
	}
	var rows []struct {
		ID                     int
		Username               string
		Revoked, ConsentMasked bool
	}
	if err := s.db.WithContext(ctx).Table("users").Select("id,username,("+notificationActorRevokedSQL+") AS revoked,("+notificationActorConsentMaskedSQL+") AS consent_masked", p.consentMaskingActive).Where("id IN ?", ids).Find(&rows).Error; err != nil {
		return nameMaskUnavailable(err)
	}
	allowed := make([]int, 0, len(rows))
	usernames := make(map[int]string, len(rows))
	for _, row := range rows {
		if row.Revoked || row.ConsentMasked {
			p.names[row.ID] = "@" + row.Username
		} else {
			allowed = append(allowed, row.ID)
			usernames[row.ID] = row.Username
		}
	}
	if len(allowed) == 0 {
		return nil
	}
	if !p.displayMaskBypass {
		var displays []struct {
			UserID      int
			DisplayName string
		}
		// Do not load global approver masks: arbitrary display_name text is just
		// as personal as the real name. Recheck consent in the guarded source SQL.
		if err := s.db.WithContext(ctx).Table("application_approvers").
			Select("application_approvers.user_id,application_approvers.display_name").
			Joins("JOIN users ON users.id = application_approvers.user_id").
			Where("application_approvers.user_id IN ?", allowed).
			Where("application_approvers.display_name IS NOT NULL AND TRIM(application_approvers.display_name) <> ''").
			Where(notificationActorNameAllowedSQL, p.consentMaskingActive).Find(&displays).Error; err != nil {
			return nameMaskUnavailable(err)
		}
		for _, row := range displays {
			p.names[row.UserID] = row.DisplayName
		}
		unmasked := allowed[:0]
		for _, id := range allowed {
			if p.names[id] == "" {
				unmasked = append(unmasked, id)
			}
		}
		allowed = unmasked
		if len(allowed) == 0 {
			return nil
		}
	}
	var names []struct {
		ID                              int
		LastName, FirstName, MiddleName *string
	}
	// Only permitted IDs enter a name-select. The revocation predicate is checked
	// again in that SQL statement to close a revoke between the two bulk reads.
	if err := s.db.WithContext(ctx).Table("users").Select("id,last_name,first_name,middle_name").
		Where("id IN ?", allowed).Where(notificationActorNameAllowedSQL, p.consentMaskingActive).Find(&names).Error; err != nil {
		return nameMaskUnavailable(err)
	}
	for _, row := range names {
		name := formatFullName(row.LastName, row.FirstName, row.MiddleName)
		if strings.TrimSpace(name) == "" {
			name = "@" + usernames[row.ID]
		}
		p.names[row.ID] = name
	}
	return nil
}
