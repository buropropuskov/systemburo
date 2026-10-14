package services

import (
	"encoding/json"
	"fmt"
	"testing"

	"systemburo/internal/models"

	"github.com/stretchr/testify/require"
)

func TestPersonalFieldsChanged_PartialRequest(t *testing.T) {
	var existing models.UniqueEmployee
	require.NoError(t, json.Unmarshal([]byte(`{"last_name":"fixture","first_name":"fixture","middle_name":"fixture","position":"fixture","passport_series_number":"fixture","patent_number":"fixture","other_permission":"fixture","citizenship_id":7}`), &existing))
	for _, body := range []string{`{}`, `{"company_id":2}`, `{"citizenship_id":null}`, `{"citizenship_id":7}`} {
		t.Run(body, func(t *testing.T) {
			var req NewUniqueEmployeeRequest
			require.NoError(t, json.Unmarshal([]byte(body), &req))
			require.False(t, personalFieldsChanged(&existing, req))
		})
	}
	for _, field := range []string{"last_name", "first_name", "middle_name", "position", "passport_series_number", "patent_number", "other_permission"} {
		for _, tc := range []struct {
			value   string
			changed bool
		}{
			{`null`, false},
			{`"fixture"`, false},
			{`"different"`, true},
			{`""`, true},
		} {
			t.Run(field+"/"+tc.value, func(t *testing.T) {
				var req NewUniqueEmployeeRequest
				require.NoError(t, json.Unmarshal([]byte(fmt.Sprintf(`{"%s":%s}`, field, tc.value)), &req))
				require.Equal(t, tc.changed, personalFieldsChanged(&existing, req))
			})
		}
	}
	for _, body := range []string{`{"citizenship_id":8}`, `{"citizenship_id":0}`} {
		var req NewUniqueEmployeeRequest
		require.NoError(t, json.Unmarshal([]byte(body), &req))
		require.True(t, personalFieldsChanged(&existing, req))
	}
	var empty models.UniqueEmployee
	var emptyRequest NewUniqueEmployeeRequest
	require.NoError(t, json.Unmarshal([]byte(`{"last_name":""}`), &emptyRequest))
	require.False(t, personalFieldsChanged(&empty, emptyRequest), "пустая строка и отсутствующее значение эквивалентны для этой проверки")
	newValue := "new"
	require.True(t, personalFieldsChanged(&empty, NewUniqueEmployeeRequest{LastName: &newValue}))
	citizenship := 7
	require.True(t, personalFieldsChanged(&empty, NewUniqueEmployeeRequest{CitizenshipID: &citizenship}))
}
