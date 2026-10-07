package entityarchive

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPackageFilePathContainment(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"../outside", "/outside", `C:\outside`, "tables/../../outside"} {
		_, err := openContainedPackageFile(root, name, nil)
		require.Error(t, err, name)
		res := VerifyResult{}
		check := verifyTableFile(TableFile{Table: "organizations", File: name}, root, nil, &res)
		require.False(t, check.OK)
		require.NotEmpty(t, res.Problems)
		res = VerifyResult{}
		check = verifyDataFile(DataFile{Table: applicationFilesTable, File: name}, root, nil, &res)
		require.False(t, check.OK)
		require.NotEmpty(t, res.Problems)
	}
	out := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(out, "outside"), []byte("secret"), 0600))
	require.NoError(t, os.Symlink(out, filepath.Join(root, "tables")))
	_, err := openContainedPackageFile(root, "tables/outside", nil)
	require.Error(t, err)
	_, err = loadPackageTables(root, []TableFile{{File: "tables/outside"}}, nil)
	require.Error(t, err)
}

func TestPackageStoredNamesValidated(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, "tables"), 0700))
	data := []byte("{\"id\":1,\"stored_name\":\"../outside.pdf\"}\n")
	sum := sha256.Sum256(data)
	require.NoError(t, os.WriteFile(filepath.Join(root, "tables", "files.jsonl"), data, 0600))
	res := VerifyResult{}
	check := verifyTableFile(TableFile{Table: applicationFilesTable, File: "tables/files.jsonl", Bytes: int64(len(data)), Rows: 1, SHA256: hex.EncodeToString(sum[:])}, root, nil, &res)
	require.False(t, check.OK)
	require.NotEmpty(t, res.Problems)
	for _, name := range []string{"../outside.pdf", `nested\outside.pdf`, "nested/outside.pdf", "/outside.pdf"} {
		err := writeDataFiles(root, t.TempDir(), []DataFile{{Table: applicationFilesTable, RowID: 1, File: "files/1"}}, map[int]string{1: name}, nil, false)
		require.Error(t, err, name)
	}
}

func TestPackageImportRejectsStorageSubdirSymlink(t *testing.T) {
	root, uploads, outside := t.TempDir(), t.TempDir(), t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, "files"), 0700))
	require.NoError(t, os.WriteFile(filepath.Join(root, "files", "1"), []byte("synthetic"), 0600))
	require.NoError(t, os.Symlink(outside, filepath.Join(uploads, applicationFilesDir)))
	err := writeDataFiles(root, uploads, []DataFile{{Table: applicationFilesTable, RowID: 1, File: "files/1"}}, map[int]string{1: "file.pdf"}, nil, false)
	require.Error(t, err)
	entries, err := os.ReadDir(outside)
	require.NoError(t, err)
	require.Empty(t, entries)
}

func TestPackagePurgeRejectsEscapingNames(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, applicationFilesDir), 0700))
	data := []byte("synthetic retained content")
	name := filepath.Join(outside, "retained.pdf")
	require.NoError(t, os.WriteFile(name, data, 0600))
	err := removeApplicationFiles(root, []appFileRow{{ID: 1, StoredName: name}})
	require.Error(t, err)
	require.NoError(t, os.Symlink(outside, filepath.Join(root, applicationFilesDir, "linked")))
	err = removeApplicationFiles(root, []appFileRow{{ID: 2, StoredName: "linked/retained.pdf"}})
	require.Error(t, err)
	got, err := os.ReadFile(name)
	require.NoError(t, err)
	require.Equal(t, data, got)
}
