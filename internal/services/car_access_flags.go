package services

import (
	"context"
	"systemburo/internal/models"

	"gorm.io/gorm"
)

// CarAccessFlags describes one admission record, never the whole attachment.
type CarAccessFlags struct {
	IndividualRoofAccess  bool `json:"individual_roof_access"`
	IndividualFreeParking bool `json:"individual_free_parking"`
	RoofAccess            bool `json:"roof_access"`
	FreeParking           bool `json:"free_parking"`
}

const carAccessSelectSQL = `c.individual_roof_access, c.individual_free_parking,
	(COALESCE(c.individual_roof_access, false) OR COALESCE(a.roof_access, false)) AS roof_access,
	(COALESCE(c.individual_free_parking, false) OR COALESCE(a.free_parking, false)) AS free_parking`

func carAccessFlags(car models.Car) CarAccessFlags {
	return CarAccessFlags{
		IndividualRoofAccess:  car.IndividualRoofAccess,
		IndividualFreeParking: car.IndividualFreeParking,
		RoofAccess:            car.IndividualRoofAccess || car.Attachment.RoofAccess,
		FreeParking:           car.IndividualFreeParking || car.Attachment.FreeParking,
	}
}

// hydrateRegistryCarAccess uses only the admission IDs already selected by the
// scoped registry query. It never resolves another car by its number or mark.
func hydrateRegistryCarAccess(ctx context.Context, db *gorm.DB, cars []UniqueCarWithRelations) error {
	ids := make([]int, 0, len(cars))
	for _, car := range cars {
		if car.ActiveCarID != nil {
			ids = append(ids, *car.ActiveCarID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	var rows []struct {
		ID int
		CarAccessFlags
	}
	if err := db.WithContext(ctx).Table("cars c").Select("c.id, "+carAccessSelectSQL).
		Joins("JOIN attachments a ON a.id = c.attachment_id").Where("c.id IN ?", ids).Scan(&rows).Error; err != nil {
		return err
	}
	byID := make(map[int]CarAccessFlags, len(rows))
	for _, row := range rows {
		byID[row.ID] = row.CarAccessFlags
	}
	for i := range cars {
		if cars[i].ActiveCarID != nil {
			cars[i].CarAccessFlags = byID[*cars[i].ActiveCarID]
		}
	}
	return nil
}
