package main

import (
	"familytree/internal/discovery"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"familytree/internal/backup"
	"familytree/internal/config"
	"familytree/internal/db"
	"familytree/internal/handler"
	"familytree/internal/proc"
	"familytree/internal/router"
)

func main() {
	// Windows 辅助模式（均为「让服务脱离旧控制台独立运行」，详见 internal/proc）：
	//   familytree.exe --self-replace <主exe路径>      在线更新后替换正在运行的 exe
	//   familytree.exe --relaunch <主exe路径> <pid>    等旧进程退出后在新窗口重启服务
	if runtime.GOOS == "windows" {
		if len(os.Args) > 2 && os.Args[1] == "--self-replace" {
			runSelfReplace(os.Args[2], argPID(os.Args, 3))
			return
		}
		if len(os.Args) > 3 && os.Args[1] == "--relaunch" {
			runRelaunch(os.Args[2], os.Args[3], argPID(os.Args, 4))
			return
		}
		// 控制台输出用 UTF-8、窗口标题带上说明（无控制台时自动跳过）
		proc.InitConsole("族谱系统 - 关闭此窗口即停止服务")
	}

	// 本地应急重置（不挂任何 HTTP 路由，只能在容器/宿主机内执行）：
	//   docker exec -it familytree /app/familytree --reset-admin-password [用户名]
	// 用途：管理员密码彻底忘记、无法登录网页时，从宿主机重置一次。
	// 安全性：执行它需要宿主机/容器控制权（等价 root），而拥有该权限的人本来就能直接改库，
	//         因此不增加攻击面；输出只打印到 stdout，不写日志、不落盘。
	if len(os.Args) > 1 && os.Args[1] == "--reset-admin-password" {
		runResetPassword(os.Args)
		return
	}
	//   docker exec -it familytree /app/familytree --add-admin <用户名>
	// 新建一个超级管理员（或把已有账号提升为超级管理员），随机 12 位密码。
	// 场景：全库只有一个管理员、密码又忘了，先用 --reset-admin-password 救回来；
	//       或者平时就多留一个管理账号，避免单点。
	if len(os.Args) > 1 && os.Args[1] == "--add-admin" {
		runAddAdmin(os.Args)
		return
	}

	cfg := config.Load()

	database, err := db.Init(cfg)
	if err != nil {
		log.Fatalf("初始化数据库失败: %v", err)
	}
	defer database.Close()

	// 确保用户表包含 token 和 need_change_pw 列
	handler.EnsureUserTable(database)

	// 启动时清理更新残留：回滚备份只保留最近 3 份、删除残留更新包
	handler.PruneUpdateArtifacts(cfg.AppDir, 3)

	// 加载登录防爆破配置（从 settings 表）
	handler.LoadLoginConfig(database)

	// 加载隐私与权限矩阵（从 settings 表）
	handler.LoadPrivacyConfig(database)

	// 启动自动备份调度器
	scheduler := backup.Scheduler{DataDir: cfg.DataDir, Timezone: cfg.Timezone}
	scheduler.Start()

	r := router.Setup(cfg, database, &scheduler)

	addr := ":" + cfg.Port
	var ver string
	database.QueryRow(`SELECT value FROM settings WHERE key = 'version'`).Scan(&ver)
	// 局域网广播发现（手机 App 自动找服务用；FAMILY_TREE_DISCOVERY=0 可关）
	if discovery.Enabled() {
		p, _ := strconv.Atoi(cfg.Port)
		discovery.Start(p, ver)          // 可选：UDP 广播（容器需映射 UDP 端口）
		discovery.StartMDNS(p, ver)      // mDNS/Bonjour（非容器安装可直接被发现；容器需 host 网络）
	}
	log.Printf("族谱系统 v%s 已启动，监听 %s", ver, addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("服务启动失败: %v", err)
	}
}

var _ = os.Getenv

// runSelfReplace Windows 自更新替换器：
// 等待主进程退出 → 复制自己到主 exe 位置 → 关掉旧窗口 → 启动新版本主程序 → 退出自己
// launcherPID：启动主进程的 cmd.exe（start.bat）PID，>0 时顺手关掉那个旧窗口。
func runSelfReplace(targetExe string, launcherPID int) {
	self, err := os.Executable()
	if err != nil {
		log.Fatalf("[更新] 获取自身路径失败: %v", err)
	}

	// 重试复制：主进程退出和文件句柄释放可能有延迟，最多等约 6 秒
	// 先复制成 .new 再改名，避免中途被杀时把主程序写坏
	var lastErr error
	tmpExe := targetExe + ".new"
	for i := 0; i < 20; i++ {
		time.Sleep(300 * time.Millisecond)
		if err := copyFile(self, tmpExe); err != nil {
			lastErr = err
			continue
		}
		if err := os.Rename(tmpExe, targetExe); err != nil {
			lastErr = err
			os.Remove(tmpExe)
			continue
		}
		lastErr = nil
		break
	}
	if lastErr != nil {
		log.Fatalf("[更新] 替换主程序失败: %v", lastErr)
	}
	log.Printf("[更新] 已替换主程序: %s", targetExe)

	if err := startAppInNewConsole(targetExe); err != nil {
		log.Fatalf("[更新] 启动新版本失败: %v", err)
	}
	closeOldWindow(launcherPID)
	log.Printf("[更新] 已在新窗口启动新版本，本窗口可以关闭")

	os.Exit(0)
}

// runRelaunch 等旧进程退出后，在新控制台窗口里重启服务（数据还原后重新加载数据库用）。
// 必须在新窗口启动：旧窗口（start.bat 里的 pause）一被关掉，附着在同一控制台上的进程会被系统杀掉。
func runRelaunch(targetExe, pidStr string, launcherPID int) {
	if pid, err := strconv.Atoi(pidStr); err == nil {
		if p, err := os.FindProcess(pid); err == nil {
			done := make(chan struct{})
			go func() { _, _ = p.Wait(); close(done) }()
			select {
			case <-done:
			case <-time.After(30 * time.Second):
			}
		}
	}
	time.Sleep(800 * time.Millisecond) // 给端口/文件句柄留一点释放时间
	if err := startAppInNewConsole(targetExe); err != nil {
		log.Printf("[重启] 启动失败: %v（请手动运行 start.bat）", err)
		os.Exit(1)
	}
	closeOldWindow(launcherPID)
	os.Exit(0)
}

// argPID 取可选的启动器 PID 参数（命令行传入，非法则返回 0）
func argPID(args []string, idx int) int {
	if len(args) > idx {
		if v, err := strconv.Atoi(args[idx]); err == nil {
			return v
		}
	}
	return 0
}

// closeOldWindow 关掉启动旧服务的控制台窗口（结束承载它的 cmd.exe；窗口随之消失）
func closeOldWindow(launcherPID int) {
	if launcherPID > 0 {
		if err := proc.KillProcess(launcherPID); err != nil {
			log.Printf("[重启] 关闭旧窗口失败: %v", err)
		}
	}
}

// startAppInNewConsole 优先用 start.bat 拉起（它会设置更新源环境变量、UTF-8 代码页并打印启动信息），
// 找不到 start.bat 时直接启动 exe。新进程拥有独立控制台窗口，不再受旧窗口开关影响。
func startAppInNewConsole(targetExe string) error {
	exeDir := filepath.Dir(targetExe)
	bat := filepath.Join(exeDir, "start.bat")
	if fi, err := os.Stat(bat); err == nil && !fi.IsDir() {
		return proc.StartInNewConsole("cmd.exe", []string{"/c", bat}, exeDir)
	}
	return proc.StartInNewConsole(targetExe, nil, exeDir)
}

// copyFile 复制文件（用于自更新替换）
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	os.MkdirAll(filepath.Dir(dst), 0o755)
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

// runResetPassword 本地应急重置：随机 12 位密码，强制下次登录修改。
func runResetPassword(args []string) {
	cfg := config.Load()
	database, err := db.Init(cfg)
	if err != nil {
		fmt.Println("初始化数据库失败:", err)
		os.Exit(1)
	}
	defer database.Close()
	handler.EnsureUserTable(database)

	username := "admin"
	if len(args) > 2 && strings.TrimSpace(args[2]) != "" {
		username = strings.TrimSpace(args[2])
	}
	var id int64
	if err := database.QueryRow(`SELECT id FROM users WHERE username = ?`, username).Scan(&id); err != nil {
		fmt.Printf("未找到用户 %s：%v\n", username, err)
		os.Exit(1)
	}
	pw := handler.RandomNumeric(12)
	if _, err := database.Exec(`UPDATE users SET password = ?, need_change_pw = 1 WHERE id = ?`, handler.HashPassword(pw), id); err != nil {
		fmt.Println("重置失败:", err)
		os.Exit(1)
	}
	_, _ = database.Exec(`INSERT INTO operation_logs (username, action, detail, created_at) VALUES (?, ?, ?, datetime('now'))`,
		"local-cli", "reset-password", "通过本地命令重置 "+username+" 的密码")
	fmt.Printf("已重置用户 %s 的密码：%s\n（请登录后立即修改；此密码只显示这一次）\n", username, pw)
}

// runAddAdmin 本地新建/提升超级管理员（仅容器内可执行，不挂 HTTP 路由）。
func runAddAdmin(args []string) {
	if len(args) < 3 || strings.TrimSpace(args[2]) == "" {
		fmt.Println("用法: familytree --add-admin <用户名>")
		os.Exit(1)
	}
	username := strings.TrimSpace(args[2])
	cfg := config.Load()
	database, err := db.Init(cfg)
	if err != nil {
		fmt.Println("初始化数据库失败:", err)
		os.Exit(1)
	}
	defer database.Close()
	handler.EnsureUserTable(database)

	pw := handler.RandomNumeric(12)
	var id int64
	err = database.QueryRow(`SELECT id FROM users WHERE username = ?`, username).Scan(&id)
	if err == nil {
		// 已存在：提升为超级管理员并重置密码
		if _, e := database.Exec(`UPDATE users SET role = ?, password = ?, need_change_pw = 1 WHERE id = ?`,
			handler.RoleSuperAdmin, handler.HashPassword(pw), id); e != nil {
			fmt.Println("更新失败:", e)
			os.Exit(1)
		}
		fmt.Printf("已把用户 %s 提升为超级管理员，新密码：%s\n", username, pw)
	} else {
		if _, e := database.Exec(`INSERT INTO users (username, password, role, branch, need_change_pw) VALUES (?, ?, ?, '', 1)`,
			username, handler.HashPassword(pw), handler.RoleSuperAdmin); e != nil {
			fmt.Println("创建失败:", e)
			os.Exit(1)
		}
		fmt.Printf("已创建超级管理员 %s，初始密码：%s\n", username, pw)
	}
	_, _ = database.Exec(`INSERT INTO operation_logs (username, action, detail, created_at) VALUES (?, ?, ?, datetime('now'))`,
		"local-cli", "add-admin", "通过本地命令新增/提升管理员 "+username)
	fmt.Println("（请登录后立即修改；此密码只显示这一次）")
}
