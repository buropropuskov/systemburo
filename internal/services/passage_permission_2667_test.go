package services_test

import (
	"context"
	"github.com/stretchr/testify/require"
	"systemburo/internal/models"
	"systemburo/internal/services"
	"testing"
	"time"
)

func TestPassage2667DBReceiverDefaultHonorsPersonalDeny(t *testing.T) {
	w := setupPassage2667World(t, services.ElementCar)
	user := models.User{Username: "receiver2667", Password: "x", TypeID: 1, IsActive: true}
	require.NoError(t, w.db.Create(&user).Error)
	resolve := func() services.PermissionSet {
		set, err := services.NewPermissionResolver(w.db).Resolve(context.Background(), user.ID)
		require.NoError(t, err)
		return set
	}
	set := resolve()
	require.False(t, set.Has(services.KeyDetailPassageCorrect))
	require.NoError(t, w.db.Create(&models.ApplicationApprover{UserID: user.ID}).Error)
	set = resolve()
	require.True(t, set.Has(services.KeyDetailPassageCorrect))
	require.NoError(t, w.db.Create(&models.UserPermissionOverride{UserID: user.ID, PermissionKey: services.KeyDetailPassageCorrect, Value: "deny", GrantedAt: time.Now().UTC()}).Error)
	set = resolve()
	require.False(t, set.Has(services.KeyDetailPassageCorrect))
	require.NoError(t, w.db.Model(&user).Update("is_admin", true).Error)
	set = resolve()
	require.False(t, set.Has(services.KeyDetailPassageCorrect))
	require.NoError(t, w.db.Model(&user).Update("is_active", false).Error)
	set = resolve()
	require.False(t, set.Has(services.KeyDetailPassageCorrect))
}
