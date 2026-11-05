package services

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"systemburo/internal/apperr"
	"systemburo/internal/models"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestPasswordPHCRejectsUnsupportedWorkBeforeAdmission(t *testing.T) {
	valid := unknownLoginPHC
	cases := map[string]string{
		"missing leading delimiter": strings.TrimPrefix(valid, "$"),
		"wrong algorithm i":         strings.Replace(valid, "argon2id", "argon2i", 1),
		"wrong algorithm d":         strings.Replace(valid, "argon2id", "argon2d", 1),
		"old version":               strings.Replace(valid, "v=19", "v=16", 1),
		"version suffix":            strings.Replace(valid, "v=19", "v=19x", 1),
		"empty memory":              strings.Replace(valid, "m=19456", "m=", 1),
		"zero memory":               strings.Replace(valid, "m=19456", "m=0", 1),
		"different memory":          strings.Replace(valid, "19456", "19455", 1),
		"huge memory":               strings.Replace(valid, "19456", "4294967295", 1),
		"overflow memory":           strings.Replace(valid, "19456", "4294967296", 1),
		"signed memory":             strings.Replace(valid, "19456", "+19456", 1),
		"negative memory":           strings.Replace(valid, "19456", "-19456", 1),
		"leading zero":              strings.Replace(valid, "19456", "019456", 1),
		"zero passes":               strings.Replace(valid, "t=2", "t=0", 1),
		"different passes":          strings.Replace(valid, "t=2", "t=3", 1),
		"zero lanes":                strings.Replace(valid, "p=1", "p=0", 1),
		"different lanes":           strings.Replace(valid, "p=1", "p=2", 1),
		"narrowing overflow":        strings.Replace(valid, "p=1", "p=256", 1),
		"parameter suffix":          strings.Replace(valid, "p=1", "p=1x", 1),
		"parameter delimiter":       strings.Replace(valid, "t=2,p=1", "t=2;p=1", 1),
		"duplicate parameter":       strings.Replace(valid, "t=2", "m=2", 1),
		"parameter order":           strings.Replace(valid, "t=2,p=1", "p=1,t=2", 1),
		"salt too short":            strings.Replace(valid, "AAAAAAAAAAAAAAAAAAAAAA$", "AA$", 1),
		"salt padding":              strings.Replace(valid, "AAAAAAAAAAAAAAAAAAAAAA$", "AAAAAAAAAAAAAAAAAAAAAA==$", 1),
		"noncanonical salt bits":    strings.Replace(valid, "AAAAAAAAAAAAAAAAAAAAAA$", "AAAAAAAAAAAAAAAAAAAAAB$", 1),
		"invalid salt alphabet":     strings.Replace(valid, "AAAAAAAAAAAAAAAAAAAAAA$", "!AAAAAAAAAAAAAAAAAAAAA$", 1),
		"key too short":             valid[:len(valid)-1],
		"key too long":              valid + "A",
		"empty key":                 valid[:strings.LastIndex(valid, "$")+1],
		"noncanonical key bits":     valid[:len(valid)-1] + "B",
		"newline key":               valid[:len(valid)-1] + "\n",
		"extra part":                valid + "$",
		"large input":               strings.Repeat("$", 4096),
	}
	// Occupy the only slot: malformed PHC must return without waiting for it.
	// A derivation reached by mistake either blocks or errors as busy; neither
	// qualifies as the required format error.
	SetArgon2Concurrency(1)
	defer SetArgon2Concurrency(0)
	sem := passwordLimit.Load().slots
	sem <- struct{}{}
	defer func() { <-sem }()
	for name, phc := range cases {
		t.Run(name, func(t *testing.T) {
			matched, err := verifyPasswordContext(context.Background(), phc, "unused")
			require.ErrorContains(t, err, "invalid PHC format")
			require.False(t, matched)
			require.NotContains(t, err.Error(), phc)
			require.Len(t, sem, 1)
		})
	}
}

func TestPasswordPHCCurrentAndRustCompatibleProfile(t *testing.T) {
	// Rust argon2 0.5.3 Argon2::default/SaltString::generate and the Go
	// migration generator use the same algorithm, version, cost and lengths.
	salt, key, params, err := parsePHC(unknownLoginPHC)
	require.NoError(t, err)
	require.Len(t, unknownLoginPHC, passwordPHCMaxLength)
	require.Len(t, salt, 16)
	require.Len(t, key, 32)
	require.Equal(t, argon2Params{memory: 19456, time: 2, threads: 1}, params)

	hash, err := hashPasswordContext(context.Background(), "source-compatible-password")
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(hash, passwordPHCPrefix))
	matched, err := verifyPasswordContext(context.Background(), hash, "source-compatible-password")
	require.NoError(t, err)
	require.True(t, matched)
	matched, err = verifyPasswordContext(context.Background(), hash, "another-password")
	require.NoError(t, err)
	require.False(t, matched)
}

func TestPasswordPHCDummyReusesAdmittedSlot(t *testing.T) {
	SetArgon2Concurrency(1)
	defer SetArgon2Concurrency(0)
	ctx, release, err := admitPasswordOperation(context.Background())
	require.NoError(t, err)
	defer release()
	sem := passwordLimit.Load().slots
	require.Len(t, sem, 1)
	// Reacquiring this one-slot gate would return busy instead of completing.
	matched, err := verifyPasswordContext(ctx, unknownLoginPHC, "synthetic-attempt")
	require.NoError(t, err)
	require.False(t, matched)
	require.Len(t, sem, 1, "verification must not release the enclosing admission")
}

func TestPasswordPHCUnknownLoginKeepsGenericFailureAndLockout(t *testing.T) {
	SetArgon2Concurrency(1)
	defer SetArgon2Concurrency(0)
	var lookups int
	db := loginBudgetFaultDB(t, func(tx *gorm.DB) {
		lookups++
		require.Len(t, passwordLimit.Load().slots, 1, "login admits before lookup")
		tx.AddError(gorm.ErrRecordNotFound)
	})
	guard := newLoginGuard(5, loginFailureWindow, accountLockDuration)
	guard.now = func() time.Time { return time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC) }
	service := &authService{loginGuard: guard, db: db}
	for attempt := 1; attempt <= 5; attempt++ {
		response, err := service.Login(context.Background(), models.LoginRequest{
			Username: "absent-synthetic-user", Password: "synthetic-attempt",
		}, &RequestMeta{IPAddress: "192.0.2.39"})
		require.Nil(t, response)
		var domain *apperr.Error
		require.ErrorAs(t, err, &domain)
		if attempt < 5 {
			require.Equal(t, http.StatusUnauthorized, domain.Code)
			require.Equal(t, "Неверный логин или пароль", domain.Message)
		} else {
			require.Equal(t, http.StatusTooManyRequests, domain.Code)
		}
		require.Empty(t, passwordLimit.Load().slots)
	}
	require.Equal(t, 5, lookups)
}

func TestPasswordPHCUnknownLookupCancellationDoesNotCountFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	db := loginBudgetFaultDB(t, func(tx *gorm.DB) {
		tx.AddError(gorm.ErrRecordNotFound)
		cancel()
	})
	guard := newLoginGuard(5, loginFailureWindow, accountLockDuration)
	service := &authService{loginGuard: guard, db: db}
	_, err := service.Login(ctx, models.LoginRequest{Username: "cancelled-absent-user", Password: "unused"},
		&RequestMeta{IPAddress: "192.0.2.40"})
	require.True(t, errors.Is(err, context.Canceled))
	require.Empty(t, guard.entries)
}
