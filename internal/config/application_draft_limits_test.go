package config

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestApplicationDraft_ConfigLimits(t *testing.T) {
	config := validConfig()
	require.NoError(t, config.Validate())
	config.ApplicationFileDraftMaxCount = 0
	require.ErrorContains(t, config.Validate(), "APPLICATION_FILE_DRAFT_MAX_COUNT")
	config.ApplicationFileDraftMaxCount = 50
	config.ApplicationFileDraftMaxTotal = config.UploadMaxFileSize - 1
	require.ErrorContains(t, config.Validate(), "APPLICATION_FILE_DRAFT_MAX_TOTAL_SIZE")
	config.ApplicationFileDraftMaxTotal = config.UploadMaxFileSize
	require.NoError(t, config.Validate())
}
