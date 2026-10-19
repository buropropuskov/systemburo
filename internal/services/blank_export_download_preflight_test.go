package services

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"systemburo/internal/download"

	"github.com/stretchr/testify/require"
)

func TestArchiveOriginal_PreflightRejectsIncompleteSelection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fixture.xlsx")
	require.NoError(t, os.WriteFile(path, []byte("fixture"), 0600))
	for _, second := range []download.ZipEntry{
		{Path: path + ".missing", Name: "missing.xlsx"},
		{Path: path, Name: "closed.xlsx", Open: func() (io.ReadCloser, error) { return nil, errors.New("fixture open failure") }},
		{Path: filepath.Dir(path), Name: "directory"},
	} {
		entries, err := prepareArchiveZip([]download.ZipEntry{{Path: path, Name: "valid.xlsx"}, second})
		require.Error(t, err)
		require.Nil(t, entries)
	}
	entries, err := prepareArchiveZip([]download.ZipEntry{{Path: path, Name: "valid.xlsx"}})
	require.NoError(t, err)
	require.Len(t, entries, 1)
}
