//go:build !windows

package proc

import "errors"

var errUnsupported = errors.New("proc: 仅 Windows 支持该操作")

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
