package services

import (
	"strings"
	"testing"

	"systemburo/internal/models"
)

func TestNotificationEntityPeriodDescription(t *testing.T) {
	for _, test := range []struct {
		mode  models.PeriodMode
		label string
	}{
		{models.PeriodInherit, "срок вложения"},
		{models.PeriodIndividual, "индивидуальный срок"},
	} {
		result := EntityPeriodCommandResult{PeriodMode: test.mode}
		got := notificationEntityPeriodDescription(result)
		if got != test.label+": без ограничения срока" {
			t.Fatalf("unexpected notification description: %q", got)
		}
		if !strings.HasPrefix(describeEntityPeriod(result), string(test.mode)+": ") {
			t.Fatal("technical audit description changed")
		}
	}
}
