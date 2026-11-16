package services

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"systemburo/internal/crypto"
	"systemburo/internal/models"

	"gorm.io/gorm"
)

var (
	ErrInvalidAccountOwnerConfirmation = errors.New("invalid account owner confirmation")
	ErrAccountOwnerStorageUnavailable  = errors.New("account owner confirmation storage is unavailable")
	ErrAccountOwnerConfirmationChanged = errors.New("account owner confirmation has changed")
	ErrAccountOwnerConfirmationDenied  = errors.New("account owner confirmation is denied")
)

type AccountOwnerEvidence struct {
	FullName  string `json:"full_name"`
	SourceRef string `json:"source_ref"`
}

type AccountOwnerConfirmationInput struct {
	Evidence         AccountOwnerEvidence
	EffectiveFrom    time.Time
	ExpectedRevision int
}

type AccountOwnerStore struct {
	db  *gorm.DB
	key []byte
}

func NewAccountOwnerStore(db *gorm.DB, key []byte) *AccountOwnerStore {
	return &AccountOwnerStore{db: db, key: key}
}

// Confirm appends verified evidence. Callers must authorize the operation before
// calling the store. ExpectedRevision prevents a stale correction from replacing
// someone else's confirmation.
func (s *AccountOwnerStore) Confirm(ctx context.Context, userID, actorID int, input AccountOwnerConfirmationInput) (int, error) {
	now := time.Now().UTC()
	input.Evidence.FullName = strings.TrimSpace(input.Evidence.FullName)
	input.Evidence.SourceRef = strings.TrimSpace(input.Evidence.SourceRef)
	if userID <= 0 || actorID <= 0 || input.ExpectedRevision < 0 ||
		input.EffectiveFrom.IsZero() || input.EffectiveFrom.After(now) ||
		!validAccountOwnerText(input.Evidence.FullName) || !validAccountOwnerText(input.Evidence.SourceRef) {
		return 0, ErrInvalidAccountOwnerConfirmation
	}
	if len(s.key) != 32 {
		return 0, ErrAccountOwnerStorageUnavailable
	}
	body, err := json.Marshal(input.Evidence)
	if err != nil {
		return 0, err
	}
	ciphertext, err := crypto.Encrypt(string(body), s.key)
	if err != nil {
		return 0, err
	}

	record := models.AccountOwnerConfirmation{
		UserID: userID, ConfirmedByID: actorID, EffectiveFrom: input.EffectiveFrom.UTC(),
		Evidence: ciphertext, CreatedAt: now,
	}
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		actor, user, err := lockAdminTarget(ctx, tx, actorID, userID)
		if err != nil {
			return err
		}
		if actor.ID == user.ID || (user.IsAdmin || user.IsSuperAdmin) && !actor.IsSuperAdmin {
			return ErrAccountOwnerConfirmationDenied
		}
		previous, err := latestAccountOwnerConfirmation(tx, userID)
		if err != nil {
			return err
		}
		if previous.ID != input.ExpectedRevision {
			return ErrAccountOwnerConfirmationChanged
		}
		return tx.Create(&record).Error
	})
	if err != nil {
		return 0, err
	}
	return record.ID, nil
}

type AccountOwnerApprovalResult struct {
	UserID     int                   `json:"user_id"`
	OnBehalfOf *int                  `json:"on_behalf_of,omitempty"`
	Approved   time.Time             `json:"approved_at"`
	Evidence   *AccountOwnerEvidence `json:"evidence,omitempty"`
}

// VerifyApproval must be called only after authorizing this specific request.
// The event determines both the identity and the date. Commit the audit before
// returning the decrypted evidence.
func (s *AccountOwnerStore) VerifyApproval(ctx context.Context, applicationID, eventID, actorID int, reason string) (*AccountOwnerApprovalResult, error) {
	reason = strings.TrimSpace(reason)
	if applicationID <= 0 || eventID <= 0 || actorID <= 0 || !validAccountOwnerText(reason) {
		return nil, ErrInvalidAccountOwnerConfirmation
	}
	if len(s.key) != 32 {
		return nil, ErrAccountOwnerStorageUnavailable
	}
	encryptedReason, err := crypto.Encrypt(reason, s.key)
	if err != nil {
		return nil, err
	}
	var result AccountOwnerApprovalResult
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var event models.AuditLog
		if err := tx.Select("actor_user_id", "created_at", "details").
			Take(&event, "id = ? AND entity_type = ? AND entity_id = ? AND action = ? AND actor_user_id > 0",
				eventID, models.AuditEntityApplication, applicationID, "approve").Error; err != nil {
			return err
		}
		result.UserID, result.Approved = *event.ActorUserID, event.CreatedAt
		var details struct {
			ImpersonatedBy *int `json:"impersonated_by"`
		}
		if len(event.Details) > 0 {
			if err := json.Unmarshal(event.Details, &details); err != nil {
				return err
			}
		}
		if details.ImpersonatedBy != nil {
			if *details.ImpersonatedBy <= 0 {
				return ErrAccountOwnerStorageUnavailable
			}
			result.OnBehalfOf = event.ActorUserID
			result.UserID = *details.ImpersonatedBy
		}
		record, err := latestAccountOwnerConfirmation(tx, result.UserID)
		if err != nil {
			return err
		}
		audit := models.AccountOwnerVerification{ApprovalEventID: eventID, ActorUserID: actorID, Reason: encryptedReason}
		if record.ID != 0 && !event.CreatedAt.Before(record.EffectiveFrom) {
			result.Evidence, err = s.decryptEvidence(record.Evidence)
			if err != nil {
				return err
			}
			audit.ConfirmationID = &record.ID
		}
		if err := tx.Create(&audit).Error; err != nil {
			return err
		}
		return tx.Create(&models.PDAuditLog{
			UserID: &actorID, Action: "verify", Resource: "account_owner",
			ResourceID: &eventID, StatusCode: 200,
		}).Error
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (s *AccountOwnerStore) decryptEvidence(ciphertext string) (*AccountOwnerEvidence, error) {
	plaintext, err := crypto.Decrypt(ciphertext, s.key)
	if err != nil {
		return nil, err
	}
	var evidence AccountOwnerEvidence
	if err := json.Unmarshal([]byte(plaintext), &evidence); err != nil {
		return nil, err
	}
	return &evidence, nil
}

func validAccountOwnerText(value string) bool {
	return value != "" && utf8.ValidString(value) && utf8.RuneCountInString(value) <= 300
}

func latestAccountOwnerConfirmation(db *gorm.DB, userID int) (models.AccountOwnerConfirmation, error) {
	var record models.AccountOwnerConfirmation
	err := db.Where("user_id = ?", userID).Order("id DESC").Take(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = nil
	}
	return record, err
}
