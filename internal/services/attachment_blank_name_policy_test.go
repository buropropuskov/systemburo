package services

import (
	"strings"
	"testing"

	"systemburo/internal/models"

	"github.com/stretchr/testify/require"
)

func TestArchiveOriginal_BlankNameProjection(t *testing.T) {
	last, first, middle, contact := "FixtureSurname", "FixtureName", "FixtureMiddle", "fixture@example.invalid"
	sender := &models.User{ID: 41, LastName: &last, FirstName: &first, MiddleName: &middle, Email: &contact, Phone: &contact}
	bctx := &BlankContext{Sender: sender, Attachment: &models.Attachment{}, Approvers: []Approver{
		{UserID: 42, LastName: last, FirstName: first, Required: true},
		{UserID: 43, LastName: "Visible", Required: true},
	}}
	applyBlankNameMasks(bctx, map[int]string{41: "@fixture", 42: "Оператор бюро"})
	require.Equal(t, "@fixture", derefStr(bctx.Sender.LastName))
	require.Nil(t, bctx.Sender.FirstName)
	require.Nil(t, bctx.Sender.Email)
	require.Nil(t, bctx.Sender.Phone)
	require.Equal(t, "Оператор бюро", bctx.Approvers[0].LastName)
	require.Empty(t, bctx.Approvers[0].FirstName)
	require.Equal(t, "Visible", bctx.Approvers[1].LastName)
	require.False(t, strings.Contains(formatBlankFilename(bctx), last))
	require.Equal(t, last, derefStr(sender.LastName), "projection must not mutate stored source")
	require.Equal(t, contact, derefStr(sender.Email))
}
