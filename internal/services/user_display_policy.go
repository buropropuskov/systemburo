package services

import (
	"context"
	"strings"

	"gorm.io/gorm"
)

const (
	hiddenUserLabel  = "Имя скрыто"
	unnamedUserLabel = "Пользователь"
)

// userDisplayPolicy keeps consent restrictions separate from an optional receiver
// alias. An alias must not be mistaken for a withdrawn or missing consent.
type userDisplayPolicy struct {
	consent map[int]string
	names   map[int]string
}

func loadUserDisplayPolicy(ctx context.Context, db *gorm.DB) (userDisplayPolicy, error) {
	consent, err := loadConsentMasks(ctx, db)
	if err != nil {
		return userDisplayPolicy{}, err
	}
	names, err := mergeDisplayNameMasks(ctx, db, consent)
	if err != nil {
		return userDisplayPolicy{}, err
	}
	return userDisplayPolicy{consent: consent, names: names}, nil
}

func (p userDisplayPolicy) restricted(userID int) bool {
	_, hidden := p.consent[userID]
	return hidden
}

// label never receives authentication identifiers. Only names approved by the
// policy or the generic fallback can reach a public display string.
func (p userDisplayPolicy) label(userID int, last, first, middle *string) string {
	if p.restricted(userID) {
		return hiddenUserLabel
	}
	if name := strings.TrimSpace(p.names[userID]); name != "" {
		return name
	}
	name := strings.TrimSpace(formatFullName(last, first, middle))
	if name == "" {
		return unnamedUserLabel
	}
	return name
}
