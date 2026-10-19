package services

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestS6History_UserDetailsAreResponseOnly(t *testing.T) {
	raw := json.RawMessage(`{"last_name":{"old":"PrivateOldS6","new":"PrivateNewS6"},"email":{"old":"fixture@example.invalid","new":"other@example.invalid"},"position":{"old":"A","new":"B"},"impersonated_by":7}`)
	original := string(raw)
	safe := maskUserAuditDetails(map[int]string{1: "OperatorS6"}, 1, raw)
	require.NotContains(t, string(safe), "Private")
	require.NotContains(t, string(safe), "example.invalid")
	require.Contains(t, string(safe), "position")
	require.Contains(t, string(safe), "impersonated_by")
	require.Equal(t, original, string(raw))
	require.Equal(t, raw, maskUserAuditDetails(nil, 1, raw))
	require.Equal(t, raw, maskUserAuditDetails(map[int]string{2: "Other"}, 1, raw))
	require.Nil(t, maskUserAuditDetails(map[int]string{1: "OperatorS6"}, 1, json.RawMessage(`"legacy"`)))
}
