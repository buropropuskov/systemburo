package testutil

import (
	"testing"

	"systemburo/internal/models"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// TestData holds IDs of seeded reference data for use in tests.
type TestData struct {
	OrgID     int
	CompanyID int
}

// SeedTestData creates an organization and company for test user registration.
func SeedTestData(t *testing.T, db *gorm.DB) TestData {
	t.Helper()

	org := models.Organization{Name: "Test Organization"}
	err := db.Create(&org).Error
	require.NoError(t, err, "failed to seed organization")

	comp := models.Company{Name: "Test Company"}
	err = db.Create(&comp).Error
	require.NoError(t, err, "failed to seed company")

	return TestData{
		OrgID:     org.ID,
		CompanyID: comp.ID,
	}
}
