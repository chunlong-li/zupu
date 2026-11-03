//go:build !windows

package proc

import (
	"errors"
	"os"
	"syscall"
)

var errUnsupported = errors.New("proc: 仅 Windows 支持该操作")

// ExecSelf 用新二进制**原地替换**当前进程（execve）：PID 不变、进程不退出。
// 在线更新后靠它自己回到新版本，不依赖 Docker restart 策略 / systemd / nohup 父进程。
func ExecSelf(exe string, args []string) error {
	if len(args) == 0 {
		args = []string{exe}
	}
	return syscall.Exec(exe, args, os.Environ())
}

// StartDetached 非 Windows 平台不支持（Linux/Docker 由 restart=unless-stopped 拉起）
func StartDetached(exe string, args []string, dir, logPath string) error { return errUnsupported }

// StartInNewConsole 非 Windows 平台不支持
func StartInNewConsole(exe string, args []string, dir string) error { return errUnsupported }

// InitConsole 非 Windows 平台无需处理
func InitConsole(title string) {}

// LauncherPID 非 Windows 平台无此概念
func LauncherPID() int { return 0 }

// KillProcess 非 Windows 平台不支持
func KillProcess(pid int) error { return errUnsupported }
