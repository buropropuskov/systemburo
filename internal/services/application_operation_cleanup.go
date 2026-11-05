package services

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
)

const applicationOperationCleanupBatch = 500

// CleanupApplicationOperations removes one bounded batch outside both the
// command validity window and its retention margin. Locked rows are skipped:
// cleanup must not wait on or interfere with an active command/replay.
func CleanupApplicationOperations(ctx context.Context, db *gorm.DB, now time.Time) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	cutoff := now.UTC().Add(-ApplicationOperationRetention)
	result := db.WithContext(ctx).Exec(`WITH expired AS (
		SELECT owner_user_id, effective_user_id, scope, key
		FROM application_operations
		WHERE created_at < ?
		ORDER BY created_at, owner_user_id, effective_user_id, scope, key
		LIMIT ? FOR UPDATE SKIP LOCKED
	)
	DELETE FROM application_operations AS operation USING expired
	WHERE operation.owner_user_id = expired.owner_user_id
	  AND operation.effective_user_id = expired.effective_user_id
	  AND operation.scope = expired.scope AND operation.key = expired.key`, cutoff, applicationOperationCleanupBatch)
	if result.Error != nil {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		// The scheduler may log this error; never propagate SQL or stored values.
		return 0, errors.New("application operation cleanup unavailable")
	}
	return result.RowsAffected, nil
}
