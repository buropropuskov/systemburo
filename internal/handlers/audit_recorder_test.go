package handlers_test

import (
	"context"
	"encoding/json"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// TestAuditRecorder_Record_WritesRow проверяет, что Record пишет строку со всеми
// полями и сериализует details в jsonb. DB-тест в пакете handlers (урок #706).
func TestAuditRecorder_Record_WritesRow(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	rec := services.NewAuditRecorder(db)
	entityID := 42
	actorID := 7
	details := map[string]any{"name": map[string]any{"old": "А", "new": "Б"}}

	err := rec.Record(context.Background(), db, "citizenship", &entityID, "updated", &actorID, details)
	require.NoError(t, err)

	var row models.AuditLog
	require.NoError(t, db.Where("entity_type = ?", "citizenship").First(&row).Error)
	assert.Equal(t, "citizenship", row.EntityType)
	require.NotNil(t, row.EntityID)
	assert.Equal(t, 42, *row.EntityID)
	assert.Equal(t, "updated", row.Action)
	require.NotNil(t, row.ActorUserID)
	assert.Equal(t, 7, *row.ActorUserID)
	assert.False(t, row.CreatedAt.IsZero())

	var got map[string]any
	require.NoError(t, json.Unmarshal(row.Details, &got))
	assert.Equal(t, details["name"], got["name"])
}

// TestAuditRecorder_Record_NilDetailsAndActor: nil details/actor/entity пишутся без ошибки.
func TestAuditRecorder_Record_NilDetailsAndActor(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	rec := services.NewAuditRecorder(db)
	err := rec.Record(context.Background(), db, "user_type", nil, "deleted", nil, nil)
	require.NoError(t, err)

	var row models.AuditLog
	require.NoError(t, db.Where("entity_type = ?", "user_type").First(&row).Error)
	assert.Nil(t, row.EntityID)
	assert.Nil(t, row.ActorUserID)
	assert.Len(t, row.Details, 0)
	assert.Equal(t, "deleted", row.Action)
}

// TestAuditRecorder_Record_NilExecFallsBackToDB: exec=nil использует db рекордера.
func TestAuditRecorder_Record_NilExecFallsBackToDB(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	rec := services.NewAuditRecorder(db)
	id := 1
	require.NoError(t, rec.Record(context.Background(), nil, "company", &id, "created", nil, nil))

	var count int64
	require.NoError(t, db.Model(&models.AuditLog{}).Where("entity_type = ?", "company").Count(&count).Error)
	assert.Equal(t, int64(1), count)
}

// TestAuditRecorder_Record_ParticipatesInTransaction: запись через tx откатывается
// вместе с транзакцией - доказывает атомарность аудита с основным действием.
func TestAuditRecorder_Record_ParticipatesInTransaction(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	rec := services.NewAuditRecorder(db)
	id := 99

	// rollback: запись внутри tx, которая откатывается -> строки нет
	_ = db.Transaction(func(tx *gorm.DB) error {
		require.NoError(t, rec.Record(context.Background(), tx, "mark", &id, "created", nil, nil))
		return assert.AnError // форсируем rollback
	})
	var afterRollback int64
	require.NoError(t, db.Model(&models.AuditLog{}).Where("entity_type = ?", "mark").Count(&afterRollback).Error)
	assert.Equal(t, int64(0), afterRollback, "запись должна откатиться вместе с tx")

	// commit: та же запись в успешной tx -> строка есть
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		return rec.Record(context.Background(), tx, "mark", &id, "created", nil, nil)
	}))
	var afterCommit int64
	require.NoError(t, db.Model(&models.AuditLog{}).Where("entity_type = ?", "mark").Count(&afterCommit).Error)
	assert.Equal(t, int64(1), afterCommit)
}

// TestAuditRecorder_Log_SwallowsAndWrites: Log не возвращает ошибку и пишет строку.
func TestAuditRecorder_Log_SwallowsAndWrites(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	rec := services.NewAuditRecorder(db)
	id := 5
	rec.Log(context.Background(), db, "organization", &id, "renamed", &id, map[string]any{"name": "Новое"})

	var count int64
	require.NoError(t, db.Model(&models.AuditLog{}).Where("entity_type = ?", "organization").Count(&count).Error)
	assert.Equal(t, int64(1), count)
}
