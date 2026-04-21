package crypto_test

import (
	"encoding/hex"
	"testing"

	"systemburo/internal/crypto"
	"systemburo/internal/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testKey32 returns a deterministic 32-byte AES key for tests.
func testKey32() []byte {
	key, _ := hex.DecodeString("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	return key
}

func strPtr(s string) *string { return &s }

// setKey sets globalKey and schedules cleanup to restore nil.
// NOT parallel-safe: tests sharing globalKey must run sequentially.
func setKey(t *testing.T, key []byte) {
	t.Helper()
	crypto.SetGlobalKey(key)
	t.Cleanup(func() { crypto.SetGlobalKey(nil) })
}

// ---------- Employee ----------

func TestEmployee_BeforeSave_EncryptsPassport(t *testing.T) {
	setKey(t, testKey32())

	passport := "1234 567890"
	patent := "AB-123456"
	e := &models.Employee{
		PassportSeriesNumber: strPtr(passport),
		PatentNumber:         strPtr(patent),
	}

	err := e.BeforeSave(nil)
	require.NoError(t, err)

	assert.NotEqual(t, passport, *e.PassportSeriesNumber, "passport should be ciphertext after BeforeSave")
	assert.NotEqual(t, patent, *e.PatentNumber, "patent should be ciphertext after BeforeSave")
	assert.NotNil(t, e.PassportSeriesNumberHMAC, "HMAC should be set")
	assert.NotNil(t, e.PatentNumberHMAC, "HMAC should be set")
	assert.Len(t, *e.PassportSeriesNumberHMAC, 64, "HMAC is hex-encoded SHA256 = 64 chars")
}

func TestEmployee_AfterFind_DecryptsPassport(t *testing.T) {
	setKey(t, testKey32())

	passport := "9999 111111"
	e := &models.Employee{PassportSeriesNumber: strPtr(passport)}

	require.NoError(t, e.BeforeSave(nil))
	ciphertext := *e.PassportSeriesNumber
	require.NotEqual(t, passport, ciphertext)

	require.NoError(t, e.AfterFind(nil))
	assert.Equal(t, passport, *e.PassportSeriesNumber, "AfterFind should decrypt back to plaintext")
}

func TestEmployee_RoundTrip(t *testing.T) {
	setKey(t, testKey32())

	tests := []struct {
		name     string
		passport string
		patent   string
	}{
		{"standard passport", "1234 567890", "AB-123456"},
		{"empty strings", "", ""},
		{"unicode", "паспорт 0000", "патент-ЮЮ"},
		{"long value", "1234567890123456789012345678901234567890", "PATENT-VERY-LONG-NUMBER-999999999"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := &models.Employee{
				PassportSeriesNumber: strPtr(tc.passport),
				PatentNumber:         strPtr(tc.patent),
			}

			require.NoError(t, e.BeforeSave(nil))
			require.NoError(t, e.AfterFind(nil))

			assert.Equal(t, tc.passport, *e.PassportSeriesNumber)
			assert.Equal(t, tc.patent, *e.PatentNumber)
		})
	}
}

// ---------- UniqueEmployee ----------

func TestUniqueEmployee_BeforeSave_EncryptsFields(t *testing.T) {
	setKey(t, testKey32())

	passport := "5555 666666"
	e := &models.UniqueEmployee{PassportSeriesNumber: strPtr(passport)}

	require.NoError(t, e.BeforeSave(nil))

	assert.NotEqual(t, passport, *e.PassportSeriesNumber)
	assert.NotNil(t, e.PassportSeriesNumberHMAC)
}

func TestUniqueEmployee_RoundTrip(t *testing.T) {
	setKey(t, testKey32())

	passport := "7777 888888"
	patent := "PAT-999"
	e := &models.UniqueEmployee{
		PassportSeriesNumber: strPtr(passport),
		PatentNumber:         strPtr(patent),
	}

	require.NoError(t, e.BeforeSave(nil))
	require.NoError(t, e.AfterFind(nil))

	assert.Equal(t, passport, *e.PassportSeriesNumber)
	assert.Equal(t, patent, *e.PatentNumber)
}

func TestUniqueEmployee_HMAC_Deterministic(t *testing.T) {
	setKey(t, testKey32())

	passport := "1111 222222"

	e1 := &models.UniqueEmployee{PassportSeriesNumber: strPtr(passport)}
	e2 := &models.UniqueEmployee{PassportSeriesNumber: strPtr(passport)}

	require.NoError(t, e1.BeforeSave(nil))
	require.NoError(t, e2.BeforeSave(nil))

	assert.Equal(t, *e1.PassportSeriesNumberHMAC, *e2.PassportSeriesNumberHMAC,
		"same plaintext must produce identical HMAC")
}

func TestUniqueEmployee_HMAC_Search(t *testing.T) {
	setKey(t, testKey32())

	passport := "3333 444444"
	key := crypto.GetGlobalKey()

	employees := make([]*models.UniqueEmployee, 3)
	passports := []string{"1111 000000", passport, "9999 000000"}
	for i, p := range passports {
		employees[i] = &models.UniqueEmployee{PassportSeriesNumber: strPtr(p)}
		require.NoError(t, employees[i].BeforeSave(nil))
	}

	searchHMAC := crypto.ComputeHMAC(passport, key)

	found := false
	foundIdx := -1
	for i, e := range employees {
		if e.PassportSeriesNumberHMAC != nil && *e.PassportSeriesNumberHMAC == searchHMAC {
			found = true
			foundIdx = i
		}
	}

	assert.True(t, found, "should find employee by HMAC")
	assert.Equal(t, 1, foundIdx, "should find the correct employee")
}

// ---------- ApplicationEmployee ----------

func TestApplicationEmployee_RoundTrip(t *testing.T) {
	setKey(t, testKey32())

	passport := "4444 555555"
	patent := "XYZ-777"
	e := &models.ApplicationEmployee{
		PassportSeriesNumber: strPtr(passport),
		PatentNumber:         strPtr(patent),
	}

	require.NoError(t, e.BeforeSave(nil))

	assert.NotEqual(t, passport, *e.PassportSeriesNumber, "should be encrypted")
	assert.NotNil(t, e.PassportSeriesNumberHMAC)
	assert.NotNil(t, e.PatentNumberHMAC)

	require.NoError(t, e.AfterFind(nil))

	assert.Equal(t, passport, *e.PassportSeriesNumber)
	assert.Equal(t, patent, *e.PatentNumber)
}

// ---------- Passthrough (nil key) ----------

func TestEncryption_NilKey_Passthrough(t *testing.T) {
	crypto.SetGlobalKey(nil)
	t.Cleanup(func() { crypto.SetGlobalKey(nil) })

	passport := "0000 999999"
	patent := "PASS-THROUGH"

	tests := []struct {
		name string
		hook func() (*string, *string, *string, *string, error)
	}{
		{
			"Employee",
			func() (*string, *string, *string, *string, error) {
				e := &models.Employee{
					PassportSeriesNumber: strPtr(passport),
					PatentNumber:         strPtr(patent),
				}
				err := e.BeforeSave(nil)
				return e.PassportSeriesNumber, e.PatentNumber,
					e.PassportSeriesNumberHMAC, e.PatentNumberHMAC, err
			},
		},
		{
			"UniqueEmployee",
			func() (*string, *string, *string, *string, error) {
				e := &models.UniqueEmployee{
					PassportSeriesNumber: strPtr(passport),
					PatentNumber:         strPtr(patent),
				}
				err := e.BeforeSave(nil)
				return e.PassportSeriesNumber, e.PatentNumber,
					e.PassportSeriesNumberHMAC, e.PatentNumberHMAC, err
			},
		},
		{
			"ApplicationEmployee",
			func() (*string, *string, *string, *string, error) {
				e := &models.ApplicationEmployee{
					PassportSeriesNumber: strPtr(passport),
					PatentNumber:         strPtr(patent),
				}
				err := e.BeforeSave(nil)
				return e.PassportSeriesNumber, e.PatentNumber,
					e.PassportSeriesNumberHMAC, e.PatentNumberHMAC, err
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			passportOut, patentOut, hmacPassport, hmacPatent, err := tc.hook()
			require.NoError(t, err)

			assert.Equal(t, passport, *passportOut, "nil key = passthrough, data stored as plaintext")
			assert.Equal(t, patent, *patentOut)
			assert.Equal(t, passport, *hmacPassport, "nil key = HMAC returns value unchanged")
			assert.Equal(t, patent, *hmacPatent)
		})
	}
}

// ---------- Nonce uniqueness ----------

func TestEncryption_DifferentCiphertexts(t *testing.T) {
	setKey(t, testKey32())

	passport := "6666 777777"

	e1 := &models.Employee{PassportSeriesNumber: strPtr(passport)}
	e2 := &models.Employee{PassportSeriesNumber: strPtr(passport)}

	require.NoError(t, e1.BeforeSave(nil))
	require.NoError(t, e2.BeforeSave(nil))

	assert.NotEqual(t, *e1.PassportSeriesNumber, *e2.PassportSeriesNumber,
		"same plaintext should produce different ciphertexts due to random nonce")

	// But HMAC must be the same
	assert.Equal(t, *e1.PassportSeriesNumberHMAC, *e2.PassportSeriesNumberHMAC,
		"HMAC must be deterministic regardless of nonce")
}

// ---------- Nil fields ----------

func TestEmployee_NilFields_NoError(t *testing.T) {
	setKey(t, testKey32())

	e := &models.Employee{
		PassportSeriesNumber: nil,
		PatentNumber:         nil,
	}

	require.NoError(t, e.BeforeSave(nil))
	assert.Nil(t, e.PassportSeriesNumber)
	assert.Nil(t, e.PatentNumber)
	assert.Nil(t, e.PassportSeriesNumberHMAC)
	assert.Nil(t, e.PatentNumberHMAC)

	require.NoError(t, e.AfterFind(nil))
	assert.Nil(t, e.PassportSeriesNumber)
	assert.Nil(t, e.PatentNumber)
}

// ---------- Cross-model HMAC consistency ----------

func TestHMAC_ConsistentAcrossModels(t *testing.T) {
	setKey(t, testKey32())

	passport := "5555 123456"

	emp := &models.Employee{PassportSeriesNumber: strPtr(passport)}
	uniq := &models.UniqueEmployee{PassportSeriesNumber: strPtr(passport)}
	app := &models.ApplicationEmployee{PassportSeriesNumber: strPtr(passport)}

	require.NoError(t, emp.BeforeSave(nil))
	require.NoError(t, uniq.BeforeSave(nil))
	require.NoError(t, app.BeforeSave(nil))

	assert.Equal(t, *emp.PassportSeriesNumberHMAC, *uniq.PassportSeriesNumberHMAC,
		"HMAC must match between Employee and UniqueEmployee")
	assert.Equal(t, *uniq.PassportSeriesNumberHMAC, *app.PassportSeriesNumberHMAC,
		"HMAC must match between UniqueEmployee and ApplicationEmployee")
}
