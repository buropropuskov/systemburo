package validator

import (
	"encoding/json"
	"net/http"
	"testing"

	"systemburo/internal/apperr"

	"github.com/stretchr/testify/require"
)

func TestDatabaseText2903(t *testing.T) {
	type fields struct {
		Value *string
		Rows  []map[string]any
	}
	bad := "prefix\x00suffix"
	for name, value := range map[string]any{
		"string": bad, "pointer": &bad,
		"nested":            fields{Rows: []map[string]any{{"text": &bad}}},
		"map_key":           map[string]any{bad: "ordinary"},
		"json_escape":       json.RawMessage(`{"text":"\u0000"}`),
		"json_array":        json.RawMessage(`[{"text":"\u0000"}]`),
		"malformed_json":    json.RawMessage(`{`),
		"no_name_heuristic": struct{ Password string }{bad},
	} {
		t.Run(name, func(t *testing.T) {
			var app *apperr.Error
			require.ErrorAs(t, DatabaseText(value), &app)
			require.Equal(t, http.StatusBadRequest, app.Code)
			require.NotContains(t, app.Message, bad)
		})
	}
	for name, value := range map[string]any{
		"unicode": "Кириллица 😀\n\r\t", "literal_escape": `\u0000 %00 %2500`,
		"raw_literal": json.RawMessage(`{"text":"\\u0000"}`),
		"binary":      []byte{0, 1, 255}, "binary_array": [3]byte{0, 1, 255},
		"nil": nil, "nil_pointer": (*string)(nil), "nested_empty": fields{},
	} {
		t.Run(name, func(t *testing.T) { require.NoError(t, DatabaseText(value)) })
	}
}
