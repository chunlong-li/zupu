//go:build !windows

package handler

import "syscall"

// diskUsage 返回 path 所在卷的可用 / 总字节数（类 Unix 实现）
func diskUsage(path string) (free, total uint64) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return 0, 0
	}
	return st.Bavail * uint64(st.Bsize), st.Blocks * uint64(st.Bsize)
}
