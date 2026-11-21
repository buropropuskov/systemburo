package services

import (
	"context"
	"fmt"
)

type applicationChangeEntity struct {
	ID, AttachmentID int
	Label            string
}

// Load each entity kind and directory once per operation, including a bulk edit.
// The caller has already checked current sender ownership of this application.
func (s *notificationService) loadChangeProjection(ctx context.Context, appID int, changes []applicationFieldChange) (map[string]map[int]applicationChangeEntity, map[string]map[int]string, error) {
	entities := map[string]map[int]applicationChangeEntity{"cars": {}, "employees": {}}
	for _, kind := range []string{"cars", "employees"} {
		ids := map[int]struct{}{}
		for _, change := range changes {
			if change.Kind == kind {
				ids[change.EntityID] = struct{}{}
			}
		}
		values := notificationProjectionIDs(ids)
		if len(values) == 0 {
			continue
		}
		label := "COALESCE(e.car_number,'Автомобиль')"
		if kind == "employees" {
			label = "COALESCE(NULLIF(TRIM(CONCAT_WS(' ',e.last_name,e.first_name,e.middle_name)),''),'Сотрудник')"
		}
		var rows []applicationChangeEntity
		if err := s.db.WithContext(ctx).Table(kind+" e").Select("e.id,e.attachment_id,"+label+" AS label").Joins("JOIN attachments a ON a.id=e.attachment_id").Where("e.id IN ? AND a.application_id = ? AND e.is_purged = false", values, appID).Find(&rows).Error; err != nil {
			return nil, nil, err
		}
		for _, row := range rows {
			entities[kind][row.ID] = row
		}
	}
	catalogs := map[string]map[int]string{"unload_places": {}, "system_tables": {}}
	for _, directory := range []struct{ table, column, field string }{{"unload_places", "name", "places"}, {"system_tables", "display_name", "tables"}} {
		ids := map[int]struct{}{}
		for _, change := range changes {
			if change.Field == directory.field {
				for _, id := range append(append([]int{}, change.Before...), change.After...) {
					ids[id] = struct{}{}
				}
			}
		}
		values := notificationProjectionIDs(ids)
		if len(values) == 0 {
			continue
		}
		var rows []struct {
			ID   int
			Name string
		}
		if err := s.db.WithContext(ctx).Table(directory.table).Select(fmt.Sprintf("id,COALESCE(%s,'') AS name", directory.column)).Where("id IN ?", values).Find(&rows).Error; err != nil {
			return nil, nil, err
		}
		for _, row := range rows {
			catalogs[directory.table][row.ID] = row.Name
		}
	}
	return entities, catalogs, nil
}
