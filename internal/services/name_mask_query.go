package services

import (
	"context"
	"encoding/json"
	"strings"

	"gorm.io/gorm"
)

// nameMaskQuery uses one policy snapshot for display, filtering and pagination.
// Names are a bound JSON parameter, never interpolated SQL literals. The actor
// expression and fallback must be static SQL from the calling service.
type nameMaskQuery struct {
	masks   map[int]string
	encoded string
}

func loadNameMaskQuery(ctx context.Context, db *gorm.DB) (nameMaskQuery, error) {
	masks, err := loadNameMasks(ctx, db)
	if err != nil {
		return nameMaskQuery{}, err
	}
	if masks == nil {
		masks = map[int]string{}
	}
	encoded, err := json.Marshal(masks)
	if err != nil {
		return nameMaskQuery{}, nameMaskUnavailable(err)
	}
	return nameMaskQuery{masks: masks, encoded: string(encoded)}, nil
}

func (m nameMaskQuery) name(actorSQL, fallbackSQL string) string {
	return "COALESCE((SELECT names ->> (" + actorSQL + ")::text FROM actor_display_masks), " + fallbackSQL + ")"
}

func (m nameMaskQuery) rawActor(db *gorm.DB, actorSQL, originalNameSQL, query string, args ...any) *gorm.DB {
	query = strings.ReplaceAll(query, originalNameSQL, m.name(actorSQL, originalNameSQL))
	for _, part := range []string{"u.last_name", "u.first_name", "u.middle_name"} {
		query = strings.ReplaceAll(query, part, "(CASE WHEN (SELECT names ->> ("+actorSQL+")::text FROM actor_display_masks) IS NOT NULL THEN NULL ELSE "+part+" END)")
	}
	return db.Raw("WITH actor_display_masks AS (SELECT ?::jsonb AS names) "+query,
		append([]any{m.encoded}, args...)...)
}
