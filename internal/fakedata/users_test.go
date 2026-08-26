package fakedata_test

// Чистая проверка среза наливки пользователей (#1682, том 5): Plan честно показывает
// количество для каждого профиля, без обращения к базе. Проверка "пользователи реально
// созданы через сервисы, у организаций есть согласующие, вход под созданным пользователем
// работает" живёт в internal/handlers (правило проекта -- тесты с базой только там).

import (
	"testing"

	"systemburo/internal/fakedata"
	"systemburo/internal/models"

	"github.com/stretchr/testify/require"
)

// Пользователи и принимающие масштабируются вместе с профилем -- на нулевом объёме
// пользователей (гипотетическое переопределение -users=0) принимающих тоже не будет:
// расставлять роли некому.
func TestPlan_UsersScaleWithProfile(t *testing.T) {
	for _, name := range fakedata.ProfileNames() {
		t.Run(name, func(t *testing.T) {
			profile, err := fakedata.ProfileByName(name)
			require.NoError(t, err)

			counts := planCounts(fakedata.Plan(profile))

			require.Equal(t, profile.Users, counts[models.AuditEntityUser])
			require.Positive(t, counts[models.AuditEntityApprover],
				"на профиле с пользователями принимающих должно планироваться хотя бы несколько")
			require.LessOrEqual(t, counts[models.AuditEntityApprover], profile.Users,
				"принимающих не может быть больше, чем всего пользователей")
		})
	}
}

// Профиль "large" (500 пользователей) не должен раздувать принимающих пропорционально --
// это "несколько" людей на стенде, а не десятая часть всей базы.
func TestPlan_ApproverCountIsCapped(t *testing.T) {
	profile, err := fakedata.ProfileByName("large")
	require.NoError(t, err)

	counts := planCounts(fakedata.Plan(profile))

	require.LessOrEqual(t, counts[models.AuditEntityApprover], 20)
}
