package models

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm/schema"
)

func TestNotificationIntentArchiveIdentityAndBusinessKey(t *testing.T) {
	s, err := schema.Parse(&NotificationIntent{}, &sync.Map{}, schema.NamingStrategy{})
	require.NoError(t, err)
	require.Len(t, s.PrimaryFields, 1)
	require.Equal(t, "id", s.PrimaryFields[0].DBName)
	require.True(t, s.PrimaryFields[0].AutoIncrement)
	key := s.FieldsByDBName["event_key"]
	require.NotNil(t, key)
	require.True(t, key.NotNull)
	require.False(t, key.PrimaryKey)
	uniqueKey := false
	for _, index := range s.ParseIndexes() {
		if index.Class == "UNIQUE" && len(index.Fields) == 1 && index.Fields[0].Field.DBName == "event_key" {
			uniqueKey = true
		}
	}
	require.True(t, uniqueKey, "event_key must remain the unique business deduplication key")
}
