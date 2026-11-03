package services_test

import (
	"fmt"
	"net/http"
	"testing"

	"systemburo/internal/apperr"
	"systemburo/internal/services"

	"github.com/stretchr/testify/require"
)

func compositionBudgetContent(kind string, n int) services.AttachmentContentData {
	data := services.AttachmentContentData{}
	switch kind {
	case "employees":
		rows := make([]services.EmployeeInput, n)
		data.Employees = &rows
	case "vehicles":
		rows := make([]services.VehicleInput, n)
		data.Vehicles = &rows
	case "items":
		rows := make([]services.ItemInput, n)
		data.Items = &rows
	}
	return data
}

func TestCompositionBudget_PerTypeAndAcrossLists(t *testing.T) {
	require.Equal(t, 2000, services.MaxApplicationRowsPerType)
	for _, kind := range []string{"employees", "vehicles", "items"} {
		for _, parts := range [][]int{{1999}, {2000}, {2001}, {1000, 1000}, {1000, 1001}} {
			t.Run(fmt.Sprintf("%s_%v", kind, parts), func(t *testing.T) {
				submit := services.CompleteApplicationRequest{}
				supplement := services.CreateSupplementRequest{}
				total := 0
				for _, n := range parts {
					total += n
					content := compositionBudgetContent(kind, n)
					submit.Attachments = append(submit.Attachments, services.AttachmentData{Data: content})
					addition := services.SupplementAddition{}
					switch kind {
					case "employees":
						addition.Employees = *content.Employees
					case "vehicles":
						addition.Vehicles = *content.Vehicles
					case "items":
						addition.Items = *content.Items
					}
					supplement.Additions = append(supplement.Additions, addition)
				}
				for _, err := range []error{services.ValidateSubmitComposition(submit), services.ValidateSupplementComposition(supplement)} {
					if total <= 2000 {
						require.NoError(t, err)
						continue
					}
					var validation *apperr.Error
					require.ErrorAs(t, err, &validation)
					require.Equal(t, http.StatusBadRequest, validation.Code)
					require.Contains(t, validation.Message, "2001")
					require.Contains(t, validation.Message, "2000")
				}
			})
		}
	}
	// The limit is per type, never a combined cap of 2000 across unlike entities.
	for _, extra := range []int{0, 1} {
		submit := services.CompleteApplicationRequest{Attachments: []services.AttachmentData{
			{Data: compositionBudgetContent("employees", 2000)}, {Data: compositionBudgetContent("vehicles", 2000)},
			{Data: compositionBudgetContent("items", 2000+extra)},
		}}
		supplement := services.CreateSupplementRequest{Additions: []services.SupplementAddition{
			{Employees: make([]services.EmployeeInput, 2000)}, {Vehicles: make([]services.VehicleInput, 2000)},
			{Items: make([]services.ItemInput, 2000+extra)},
		}}
		for _, err := range []error{services.ValidateSubmitComposition(submit), services.ValidateSupplementComposition(supplement)} {
			if extra == 0 {
				require.NoError(t, err, "6000 mixed rows with 2000 of each type are within budget")
			} else {
				require.Error(t, err)
			}
		}
	}
	require.NoError(t, services.ValidateSubmitComposition(services.CompleteApplicationRequest{}))
	require.NoError(t, services.ValidateSupplementComposition(services.CreateSupplementRequest{}))
}
