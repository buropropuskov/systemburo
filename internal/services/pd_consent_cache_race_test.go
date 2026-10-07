package services

import (
	"context"
	"sync"
	"testing"
	"time"

	"systemburo/internal/models"

	"github.com/stretchr/testify/require"
)

type cacheRaceSettings struct {
	SettingsService
	read func() (*models.PDConsentSettings, error)
}

func (s cacheRaceSettings) GetPDConsentSettings(context.Context) (*models.PDConsentSettings, error) {
	return s.read()
}

type cacheRaceConsents struct {
	ConsentService
	read func() (int, error)
}

func (s cacheRaceConsents) ActiveVersion(context.Context, int, string) (int, error) {
	return s.read()
}

func TestPDConsentCache_InvalidationDuringRead(t *testing.T) {
	for _, kind := range []string{"requirement", "accepted", "accepted_all"} {
		t.Run(kind, func(t *testing.T) {
			loaded, resume, finished := make(chan struct{}), make(chan struct{}), make(chan error, 1)
			var pauseOnce, releaseOnce sync.Once
			release := func() { releaseOnce.Do(func() { close(resume) }) }
			t.Cleanup(release)
			pause := func() { pauseOnce.Do(func() { close(loaded); <-resume }) }
			settings := models.PDConsentSettings{Required: false, Version: 1, Text: "<p>Consent</p>"}
			accepted := 1
			gate := NewPDConsentGateService(cacheRaceConsents{read: func() (int, error) {
				value := accepted
				pause()
				return value, nil
			}}, cacheRaceSettings{read: func() (*models.PDConsentSettings, error) {
				value := settings
				pause()
				return &value, nil
			}}, time.Hour)
			go func() {
				if kind == "requirement" {
					_, err := gate.Requirement(context.Background())
					finished <- err
				} else {
					_, err := gate.AcceptedVersion(context.Background(), 42)
					finished <- err
				}
			}()
			select {
			case <-loaded:
			case <-time.After(5 * time.Second):
				t.Fatal("чтение не достигло контрольной точки")
			}
			settings.Required = true
			accepted = 0
			if kind == "accepted" {
				gate.Invalidate(42)
			} else {
				gate.InvalidateAll()
			}
			release()
			select {
			case err := <-finished:
				require.NoError(t, err)
			case <-time.After(5 * time.Second):
				t.Fatal("чтение не завершилось")
			}
			if kind == "requirement" {
				value, err := gate.Requirement(context.Background())
				require.NoError(t, err)
				require.True(t, value.Enabled, "старое чтение восстановило отключённое требование")
			} else {
				value, err := gate.AcceptedVersion(context.Background(), 42)
				require.NoError(t, err)
				require.Zero(t, value, "старое чтение восстановило отозванное согласие")
			}
		})
	}
}
