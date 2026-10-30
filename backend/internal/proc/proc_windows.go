//go:build windows

// Package proc 解决 Windows 上「子进程随控制台窗口一起被杀」的问题：
// 在线更新自重启时，替换器与新服务必须脱离原控制台，否则用户一件「Press any key」
// 关掉旧窗口，新服务就跟着一起没了（表现为「更新完没有再启动」）。
package proc

import (
	"os"
	"os/exec"
	"strings"
	"syscall"
	"unsafe"
)

const (
	createNewConsole    = 0x00000010
	detachedProcess     = 0x00000008
	createNewProcessGrp = 0x00000200
)

var (
	kernel32                 = syscall.NewLazyDLL("kernel32.dll")
	procGetConsoleWindow     = kernel32.NewProc("GetConsoleWindow")
	procSetConsoleTitleW     = kernel32.NewProc("SetConsoleTitleW")
	procSetConsoleOutputCP   = kernel32.NewProc("SetConsoleOutputCP")
	procCreateToolhelp32Snap = kernel32.NewProc("CreateToolhelp32Snapshot")
	procProcess32First       = kernel32.NewProc("Process32FirstW")
	procProcess32Next        = kernel32.NewProc("Process32NextW")
)

const th32csSnapProcess = 0x00000002

// PROCESSENTRY32W
type processEntry32 struct {
	Size            uint32
	CntUsage        uint32
	ProcessID       uint32
	DefaultHeapID   uintptr
	ModuleID        uint32
	CntThreads      uint32
	ParentProcessID uint32
	PriClassBase    int32
	Flags           uint32
	ExeFile         [260]uint16
}

// StartDetached 启动完全脱离当前进程/控制台的子进程（DETACHED_PROCESS）：
// 不占控制台，也不会因为父进程所在的控制台窗口被关闭而终止；日志写入 logPath（可为空）。
func StartDetached(exe string, args []string, dir, logPath string) error {
	cmd := exec.Command(exe, args...)
	cmd.Dir = dir
	if logPath != "" {
		if lf, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644); err == nil {
			cmd.Stdout = lf
			cmd.Stderr = lf
		}
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: detachedProcess | createNewProcessGrp}
	return cmd.Start()
}

// StartInNewConsole 在独立的新控制台窗口里启动进程（重启服务用：用户能看到日志，
// 关掉那个新窗口才会停止服务）。返回后调用方应尽快退出，避免两个实例并存。
func StartInNewConsole(exe string, args []string, dir string) error {
	cmd := exec.Command(exe, args...)
	cmd.Dir = dir
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: createNewConsole}
	return cmd.Start()
}

// InitConsole 把当前控制台输出代码页设为 UTF-8 并设置窗口标题（无控制台时静默跳过）。
func InitConsole(title string) {
	h, _, _ := procGetConsoleWindow.Call()
	if h == 0 {
		return
	}
	procSetConsoleOutputCP.Call(uintptr(65001))
	if title != "" {
		if p, err := syscall.UTF16PtrFromString(title); err == nil {
			procSetConsoleTitleW.Call(uintptr(unsafe.Pointer(p)))
		}
	}
}

// LauncherPID 返回「启动本进程的 cmd.exe」的 PID（用户双击 start.bat 的场景），
// 不是 cmd.exe（如直接双击 exe、或由 Explorer 启动）则返回 0。重启后可据此关掉旧控制台窗口。
func LauncherPID() int {
	ppid, exe := parentProcess(os.Getpid())
	if ppid <= 0 {
		return 0
	}
	if strings.EqualFold(exe, "cmd.exe") {
		return ppid
	}
	return 0
}

// parentProcess 用进程快照查出指定进程的父进程 PID 与父进程可执行文件名。
func parentProcess(pid int) (int, string) {
	snap, _, _ := procCreateToolhelp32Snap.Call(uintptr(th32csSnapProcess), 0)
	if snap == 0 || snap == uintptr(syscall.InvalidHandle) {
		return 0, ""
	}
	defer syscall.CloseHandle(syscall.Handle(snap))
	var e processEntry32
	e.Size = uint32(unsafe.Sizeof(e))
	r, _, _ := procProcess32First.Call(snap, uintptr(unsafe.Pointer(&e)))
	for r != 0 {
		if int(e.ProcessID) == pid {
			return int(e.ParentProcessID), syscall.UTF16ToString(e.ExeFile[:])
		}
		r, _, _ = procProcess32Next.Call(snap, uintptr(unsafe.Pointer(&e)))
	}
	return 0, ""
}

// KillProcess 结束指定进程（用于关掉承载旧服务窗口的 cmd.exe：结束它，窗口随之消失）。
func KillProcess(pid int) error {
	if pid <= 0 {
		return nil
	}
	p, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return p.Kill()
}
