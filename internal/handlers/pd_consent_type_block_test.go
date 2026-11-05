package handlers_test

import (
	"net/http"
	"testing"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Consent kinds are independent: only processing consent controls whether an
// administrator may edit this person's card. Dates are fixture facts, never
// compared with the machine's current day.
func TestConsentBlock_ConsentTypesRemainIndependent(t *testing.T) {
	type consentState struct {
		kind    string
		revoked bool
	}
	cases := []struct {
		name            string
		consents        []consentState
		otherUserActive bool
		wantStatus      int
	}{
		{"processing_revoked_transfer_active", []consentState{{"pd_processing", true}, {"pd_transfer", false}}, false, http.StatusConflict},
		{"transfer_only_active", []consentState{{"pd_transfer", false}}, false, http.StatusOK},
		{"transfer_only_revoked", []consentState{{"pd_transfer", true}}, false, http.StatusOK},
		{"processing_active_transfer_revoked", []consentState{{"pd_processing", false}, {"pd_transfer", true}}, false, http.StatusOK},
		{"processing_regranted_transfer_active", []consentState{{"pd_processing", true}, {"pd_processing", false}, {"pd_transfer", false}}, false, http.StatusOK},
		{"both_revoked", []consentState{{"pd_processing", true}, {"pd_transfer", true}}, false, http.StatusConflict},
		{"other_person_processing_is_independent", []consentState{{"pd_processing", true}}, true, http.StatusConflict},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e, db, cleanup := testutil.SetupTestApp(t)
			defer cleanup()
			testutil.CleanDB(t, db)
			td := testutil.SeedTestData(t, db)
			admin := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
			const username = "consent_type_subject"
			const password = "password123456789012345678901234"
			testutil.RegisterAndLogin(t, e, username, password, 1, td.OrgID, td.CompanyID)
			var person models.User
			require.NoError(t, db.Where("username = ?", username).First(&person).Error)
			require.NoError(t, db.Model(&models.User{}).Where("id = ?", person.ID).
				Update("position", "Исходная должность").Error)

			anchor := time.Date(2026, time.January, 1, 12, 0, 0, 0, time.UTC)
			for i, state := range tc.consents {
				consent := models.PDConsent{
					UserID: person.ID, ConsentType: state.kind, Granted: !state.revoked,
					GrantedAt: anchor.Add(time.Duration(i) * time.Hour), DocumentVersion: 1,
				}
				if state.revoked {
					revoked := consent.GrantedAt.Add(30 * time.Minute)
					consent.RevokedAt = &revoked
				}
				require.NoError(t, db.Create(&consent).Error)
			}
			if tc.otherUserActive {
				testutil.RegisterAndLogin(t, e, "consent_type_other", password, 1, td.OrgID, td.CompanyID)
				var other models.User
				require.NoError(t, db.Where("username = ?", "consent_type_other").First(&other).Error)
				require.NoError(t, db.Create(&models.PDConsent{
					UserID: other.ID, ConsentType: "pd_processing", Granted: true,
					GrantedAt: anchor, DocumentVersion: 1,
				}).Error)
			}

			rec := testutil.PUT(t, e, "/users/"+username+"/info",
				`{"position":"Новая должность"}`, testutil.AuthHeader(admin))
			assert.Equal(t, tc.wantStatus, rec.Code, "administrative edit response: %s", rec.Body.String())
			var position string
			require.NoError(t, db.Model(&models.User{}).Where("id = ?", person.ID).
				Select("position").Row().Scan(&position))
			wantPosition := "Новая должность"
			if tc.wantStatus == http.StatusConflict {
				wantPosition = "Исходная должность"
			}
			assert.Equal(t, wantPosition, position, "a refused edit must preserve the stored card")
		})
	}
}
