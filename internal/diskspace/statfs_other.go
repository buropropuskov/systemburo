//go:build !linux

package diskspace

import "errors"

// ErrUnsupported - платформа без реализации Statfs. Прод разворачивается только в
// Docker на Linux; сборка на других GOOS остаётся зелёной ради локальной
// разработки и CI на не-Linux раннерах, но реальных чисел здесь не даёт.
var ErrUnsupported = errors.New("diskspace: statfs is not supported on this platform")

func statfs(path string) (Usage, error) {
	return Usage{}, ErrUnsupported
}
