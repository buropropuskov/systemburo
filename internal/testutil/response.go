package testutil

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

// envelope mirrors handlers.Response for test parsing.
type envelope struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data"`
	Error   string          `json:"error"`
}

// ParseResponse extracts and unmarshals the .data field from a success envelope.
func ParseResponse[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var env envelope
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &env), "failed to parse envelope: %s", rec.Body.String())
	require.True(t, env.Success, "expected success=true, got error: %s", env.Error)

	var data T
	require.NoError(t, json.Unmarshal(env.Data, &data), "failed to parse data: %s", string(env.Data))
	return data
}

// ParseMessage extracts a string message from a success envelope's .data field.
func ParseMessage(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var env envelope
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &env))
	require.True(t, env.Success, "expected success=true, got error: %s", env.Error)

	var msg string
	require.NoError(t, json.Unmarshal(env.Data, &msg))
	return msg
}

// ParseMap extracts a map from a success envelope's .data field.
func ParseMap(t *testing.T, rec *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	return ParseResponse[map[string]interface{}](t, rec)
}

// ParseSlice extracts a slice of maps from a success envelope's .data field.
func ParseSlice(t *testing.T, rec *httptest.ResponseRecorder) []map[string]interface{} {
	t.Helper()
	return ParseResponse[[]map[string]interface{}](t, rec)
}
