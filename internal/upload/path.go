package upload

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ContainedPath accepts relative package paths; stored names must be basenames.
// Root is the trusted configured upload/package root. Resolve it so configured
// symlink roots work; symlinks below this boundary (including storage subdirs)
// are rejected, including dangling links.
func ContainedPath(root, name string, basename bool) (string, error) {
	if name == "" || strings.ContainsAny(name, "\\:\x00") || filepath.IsAbs(name) || strings.HasPrefix(name, "/") {
		return "", fmt.Errorf("invalid file path")
	}
	parts := strings.Split(name, "/")
	if basename && len(parts) != 1 {
		return "", fmt.Errorf("stored name must be a basename")
	}
	for _, part := range parts {
		if part == "" || part == "." || part == ".." || strings.TrimRight(part, " .") != part {
			return "", fmt.Errorf("invalid file path component")
		}
	}
	base, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(base); err == nil {
		base = resolved
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	path := base
	for _, part := range parts {
		path = filepath.Join(path, part)
		info, err := os.Lstat(path)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		if err == nil && info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("symlink file path is forbidden")
		}
	}
	return path, nil
}

// StoredPath verifies the basename and storage subdirectory beneath uploadRoot.
func StoredPath(uploadRoot, subdir, name string) (string, error) {
	if _, err := ContainedPath(uploadRoot, name, true); err != nil {
		return "", err
	}
	return ContainedPath(uploadRoot, subdir+"/"+name, false)
}
