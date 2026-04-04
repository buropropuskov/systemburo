package upload

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateFileType_JPEG(t *testing.T) {
	data := append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, make([]byte, 508)...)
	detected, err := ValidateFileType(bytes.NewReader(data), []string{"image/jpeg", "image/png"})
	require.NoError(t, err)
	assert.Equal(t, "image/jpeg", detected)
}

func TestValidateFileType_PNG(t *testing.T) {
	data := append([]byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}, make([]byte, 504)...)
	detected, err := ValidateFileType(bytes.NewReader(data), []string{"image/jpeg", "image/png"})
	require.NoError(t, err)
	assert.Equal(t, "image/png", detected)
}

func TestValidateFileType_Disallowed(t *testing.T) {
	data := append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, make([]byte, 508)...)
	_, err := ValidateFileType(bytes.NewReader(data), []string{"image/png"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "image/jpeg")
}

func TestValidateFileType_PDF(t *testing.T) {
	data := append([]byte("%PDF-1.4"), make([]byte, 504)...)
	detected, err := ValidateFileType(bytes.NewReader(data), []string{"application/pdf"})
	require.NoError(t, err)
	assert.Equal(t, "application/pdf", detected)
}

func TestValidateFileSize_OK(t *testing.T) {
	assert.NoError(t, ValidateFileSize(1024, 10*1024*1024))
}

func TestValidateFileSize_Exceeds(t *testing.T) {
	err := ValidateFileSize(20*1024*1024, 10*1024*1024)
	assert.Error(t, err)
}
