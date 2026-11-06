package services

import (
	"context"
	"errors"
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
					value, err := gate.Requirement(context.Background())
					if err == nil && !value.Enabled {
						err = errors.New("invalidated in-flight requirement returned stale state")
					}
					finished <- err
				} else {
					value, err := gate.AcceptedVersion(context.Background(), 42)
					if err == nil && value != 0 {
						err = errors.New("invalidated in-flight consent returned stale version")
					}
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

func TestPDConsentCache_BoundedRetry(t *testing.T) {
	for _, kind := range []string{"accepted", "requirement"} {
		t.Run(kind, func(t *testing.T) {
			var gate *PDConsentGateService
			reads := 0
			gate = NewPDConsentGateService(cacheRaceConsents{read: func() (int, error) {
				reads++
				gate.Invalidate(42)
				return 7, nil
			}}, cacheRaceSettings{read: func() (*models.PDConsentSettings, error) {
				reads++
				gate.InvalidateAll()
				return &models.PDConsentSettings{Required: false}, nil
			}}, time.Hour)
			var err error
			if kind == "accepted" {
				_, err = gate.AcceptedVersion(context.Background(), 42)
			} else {
				_, err = gate.Requirement(context.Background())
			}
			require.ErrorIs(t, err, errConsentStateChanged)
			require.Equal(t, consentReadAttempts, reads)
			require.Empty(t, gate.acceptedReads)
			_, cached := gate.accepted.Load(42)
			require.False(t, cached)
			require.True(t, gate.requirementAt.IsZero())
		})
	}
}

func TestPDConsentCache_OtherSubjectDoesNotInvalidateRead(t *testing.T) {
	var gate *PDConsentGateService
	reads := 0
	gate = NewPDConsentGateService(cacheRaceConsents{read: func() (int, error) {
		reads++
		gate.Invalidate(99)
		return 7, nil
	}}, nil, time.Hour)
	value, err := gate.AcceptedVersion(context.Background(), 42)
	require.NoError(t, err)
	require.Equal(t, 7, value)
	_, err = gate.AcceptedVersion(context.Background(), 42)
	require.NoError(t, err)
	require.Equal(t, 1, reads, "unrelated changes must not force extra database reads")
	require.Empty(t, gate.acceptedReads)
}

func TestPDConsentCache_CanceledReadDoesNotPopulateCache(t *testing.T) {
	for _, kind := range []string{"accepted", "requirement"} {
		t.Run(kind, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			gate := NewPDConsentGateService(cacheRaceConsents{read: func() (int, error) {
				cancel()
				return 7, nil
			}}, cacheRaceSettings{read: func() (*models.PDConsentSettings, error) {
				cancel()
				return &models.PDConsentSettings{Required: true, Text: "<p>Consent</p>"}, nil
			}}, time.Hour)
			var err error
			if kind == "accepted" {
				_, err = gate.AcceptedVersion(ctx, 42)
			} else {
				_, err = gate.Requirement(ctx)
			}
			require.ErrorIs(t, err, context.Canceled)
			require.Empty(t, gate.acceptedReads)
			_, cached := gate.accepted.Load(42)
			require.False(t, cached)
			require.True(t, gate.requirementAt.IsZero())
		})
	}
}
