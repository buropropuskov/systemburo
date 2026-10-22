package handlers_test

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAuthSwaggerTransportContract(t *testing.T) {
	raw, err := os.ReadFile("../../docs/swagger.json")
	require.NoError(t, err)
	var document struct {
		Paths map[string]map[string]struct {
			Description string `json:"description"`
			Parameters  []struct {
				In string `json:"in"`
			} `json:"parameters"`
			Responses map[string]struct {
				Schema struct {
					AllOf []struct {
						Ref        string `json:"$ref"`
						Properties map[string]struct {
							Ref string `json:"$ref"`
						} `json:"properties"`
					} `json:"allOf"`
				} `json:"schema"`
				Headers map[string]any `json:"headers"`
			} `json:"responses"`
		} `json:"paths"`
	}
	require.NoError(t, json.Unmarshal(raw, &document))
	for path, model := range map[string]string{"/login": "models.LoginResponse", "/refresh-token": "models.TokenPairResponse"} {
		t.Run(path, func(t *testing.T) {
			operation, ok := document.Paths[path]["post"]
			require.True(t, ok)
			require.Contains(t, operation.Description, "HttpOnly cookie refresh_token")
			require.Contains(t, operation.Description, "data.token")
			require.Contains(t, operation.Description, "data.refreshToken")
			require.Contains(t, operation.Description, "пустой строкой")
			response, ok := operation.Responses["200"]
			require.True(t, ok)
			var envelope, data bool
			for _, part := range response.Schema.AllOf {
				envelope = envelope || strings.HasSuffix(part.Ref, "/handlers.Response")
				data = data || part.Properties["data"].Ref == "#/definitions/"+model
			}
			require.True(t, envelope, "success envelope must be documented")
			require.True(t, data, "data must use the actual response model")
			require.Contains(t, response.Headers, "Set-Cookie")
			if path == "/refresh-token" {
				for _, parameter := range operation.Parameters {
					require.NotEqual(t, "body", parameter.In, "refresh does not accept a JSON token")
				}
			}
		})
	}
}
