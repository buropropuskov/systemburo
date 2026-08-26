//go:build linux

package diskspace

import (
	"fmt"
	"syscall"
)

// statfs читает место на разделе через syscall.Statfs (объём) и syscall.Stat
// (номер устройства для дедупликации - st_dev, а не statfs.Fsid: поле Fsid у
// Statfs_t на linux непереносимо между архитектурами, Stat_t.Dev - стандартный
// способ узнать "тот же ли это раздел", которым пользуется сам `stat`).
func statfs(path string) (Usage, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return Usage{}, fmt.Errorf("statfs %s: %w", path, err)
	}
	var info syscall.Stat_t
	if err := syscall.Stat(path, &info); err != nil {
		return Usage{}, fmt.Errorf("stat %s: %w", path, err)
	}

	bsize := int64(st.Bsize)
	return Usage{
		Device:     info.Dev,
		TotalBytes: int64(st.Blocks) * bsize,
		FreeBytes:  int64(st.Bavail) * bsize,
	}, nil
}
