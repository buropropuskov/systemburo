package upload

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestContainedPath(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"", "../outside", "/outside", "a/../outside", "a//b", "a/./b", `a\b`, `C:/outside`, `C:outside`, ".. /outside", ".../outside"} {
		_, err := ContainedPath(root, name, false)
		require.Error(t, err, name)
	}
	p, err := ContainedPath(root, "tables/data.jsonl", false)
	require.NoError(t, err)
	require.Equal(t, filepath.Join(root, "tables", "data.jsonl"), p)
	_, err = ContainedPath(root, "tables/data.jsonl", true)
	require.Error(t, err)
	_, err = ContainedPath(root, "file.pdf", true)
	require.NoError(t, err)
}

func TestContainedPathRejectsSymlink(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "link")); err != nil {
		t.Fatalf("symlink creation required for containment test: %v", err)
	}
	_, err := ContainedPath(root, "link/file", false)
	require.Error(t, err)
	if err := os.Symlink(filepath.Join(outside, "missing"), filepath.Join(root, "dangling")); err != nil {
		t.Fatal(err)
	}
	_, err = ContainedPath(root, "dangling", true)
	require.Error(t, err)
	_, err = StoredPath(root, "link/nested", "file.pdf")
	require.Error(t, err)
	alias := filepath.Join(t.TempDir(), "configured")
	require.NoError(t, os.Symlink(root, alias))
	p, err := StoredPath(alias, "application_files", "file.pdf")
	require.NoError(t, err)
	require.Equal(t, filepath.Join(root, "application_files", "file.pdf"), p)
}
