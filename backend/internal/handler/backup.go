package handler

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"familytree/internal/config"
	"familytree/internal/proc"

	"github.com/gin-gonic/gin"
)

// BackupHandler 备份/还原处理器
type BackupHandler struct {
	DB  *sql.DB
	Cfg *config.Config
}

// ============================ 备份内容口径 ============================
// 所有备份入口（手动/每日/每月）统一按这里定义的目录集合打包，避免「各入口各打各的」
// 造成某个类型悄悄漏掉一类数据（历史问题：字体 220MB 完全不在任何 zip 里）。

const (
	KindManual  = "manual"
	KindDaily   = "daily"
	KindMonthly = "monthly"
	KindSystem  = "system"
)

// manifestEntryName 包内清单文件名（放 zip 根目录，解压第一眼就能看到）
const manifestEntryName = "backup-manifest.json"

// excludedDir 明确不打包的目录及原因（写进 manifest，用户可自行核对）
type excludedDir struct {
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

var fullBackupExcludes = []excludedDir{
	{"map", "地图瓦片缓存，可由离线地图包或在线地图重新下载"},
	{"runtime", "运行时程序文件，随安装包提供"},
	{"web", "前端页面产物，随安装包提供"},
	{"backup", "备份目录自身（打包会导致体积指数膨胀）"},
	{"updates", "在线更新残留文件"},
}

// dirStat 单个目录的文件数与字节数（manifest 里让用户一眼看出字体在不在）
type dirStat struct {
	Files int   `json:"files"`
	Bytes int64 `json:"bytes"`
}

// backupManifest 包内 backup-manifest.json 的结构
type backupManifest struct {
	ManifestVersion     int                `json:"manifest_version"`
	AppVersion          string             `json:"app_version"`
	BackupType          string             `json:"backup_type"`
	BackupTypeLabel     string             `json:"backup_type_label"`
	GeneratedAt         string             `json:"generated_at"`
	CreatedBy           string             `json:"created_by,omitempty"`
	ContainsDatabase    bool               `json:"contains_database"`
	ContainsAttachments bool               `json:"contains_attachments"`
	ContainsFonts       bool               `json:"contains_fonts"`
	ContainsWeb         bool               `json:"contains_web"`
	WALCheckpoint       string             `json:"wal_checkpoint,omitempty"`
	TableCount          int                `json:"table_count"`
	TotalRows           int64              `json:"total_rows"`
	Tables              map[string]int64   `json:"tables"`
	Directories         map[string]dirStat `json:"directories"`
	FileCounts          map[string]int     `json:"file_counts"`
	// —— 字体备份策略（手动备份可由用户选择） ——
	FontPolicy      string            `json:"font_policy,omitempty"`       // all | none | selected
	FontPolicyLabel string            `json:"font_policy_label,omitempty"` // 中文说明
	FontPolicyNote  string            `json:"font_policy_note,omitempty"`  // 例如「未备份字体（用户选择跳过）」
	FontsAvailable  int               `json:"fonts_available,omitempty"`   // 备份时字体目录里一共有几个
	Fonts           []string          `json:"fonts,omitempty"`             // 实际打进包的字体文件名清单
	FontBytes       int64             `json:"font_bytes,omitempty"`        // 这些字体的字节数
	ExcludedDirs    []excludedDir     `json:"excluded_dirs,omitempty"`
	Snapshots       map[string]string `json:"snapshots,omitempty"`
	NotIncluded     []string          `json:"not_included,omitempty"`
	Notes           string            `json:"notes"`
}

// 字体备份策略取值
const (
	FontPolicyAll      = "all"
	FontPolicyNone     = "none"
	FontPolicySelected = "selected"
)

// fontPolicyLabel 字体策略的中文说明（前端与 manifest 共用口径）
func fontPolicyLabel(policy string, picked, available int) string {
	switch policy {
	case FontPolicyNone:
		return "未备份字体（用户选择跳过）"
	case FontPolicySelected:
		return fmt.Sprintf("已选 %d 个字体（当前共 %d 个）", picked, available)
	default:
		return fmt.Sprintf("全部备份（%d 个）", picked)
	}
}

// zipFileEntry 需要单独打进包的单个文件（包内条目名 → 磁盘源路径）
type zipFileEntry struct {
	Name string
	Path string
}

// listFontFiles 列出 fonts 目录里真实可用的字体文件（与 GET /api/fonts 同口径：
// 扩展名 + 字体魔数校验），返回 文件名 → 字节数。
func listFontFiles(fontsDir string) map[string]int64 {
	out := map[string]int64{}
	entries, err := os.ReadDir(fontsDir)
	if err != nil {
		return out
	}
	for _, e := range entries {
		if e.IsDir() || !fontExtRe.MatchString(strings.ToLower(e.Name())) {
			continue
		}
		fi, err := e.Info()
		if err != nil {
			continue
		}
		if !fontMagicOK(filepath.Join(fontsDir, e.Name())) {
			continue
		}
		out[e.Name()] = fi.Size()
	}
	return out
}

// resolveFontBackup 解析字体备份策略，返回要打进包的文件与清单。
// 安全：名字只允许来自当前 fonts 目录里的真实文件（先 filepath.Base 归一，再查白名单），
// 天然免疫 ../ 之类的目录穿越；不存在的名字会被忽略并回报，不会让整次备份失败。
func resolveFontBackup(fontsDir, policy string, names []string) (files []zipFileEntry, included []string, bytes int64, note string, ignored []string) {
	avail := listFontFiles(fontsDir)
	availNames := make([]string, 0, len(avail))
	for n := range avail {
		availNames = append(availNames, n)
	}
	sort.Strings(availNames)

	appendFont := func(n string) {
		files = append(files, zipFileEntry{Name: "fonts/" + n, Path: filepath.Join(fontsDir, n)})
		included = append(included, n)
		bytes += avail[n]
	}

	switch policy {
	case FontPolicyNone:
		return nil, nil, 0, "未备份字体（用户选择跳过）", nil
	case FontPolicySelected:
		seen := map[string]bool{}
		for _, raw := range names {
			n := filepath.Base(filepath.Clean(strings.TrimSpace(raw))) // 只取基名，杜绝路径穿越
			if n == "" || n == "." || n == string(filepath.Separator) || seen[n] {
				continue
			}
			if _, ok := avail[n]; !ok {
				ignored = append(ignored, n)
				continue
			}
			seen[n] = true
			appendFont(n)
		}
		sort.Strings(included)
		sort.Strings(ignored)
		if len(included) == 0 {
			return nil, nil, 0, fmt.Sprintf("按用户选择备份字体，但选择为空 → 未备份字体（当前共 %d 个）", len(availNames)), ignored
		}
		note = fmt.Sprintf("按用户选择只备份 %d 个字体（当前共 %d 个），其余字体不在包内", len(included), len(availNames))
		return files, included, bytes, note, ignored
	default: // "" 或 all：保持老行为 = 全部备份
		for _, n := range availNames {
			appendFont(n)
		}
		note = fmt.Sprintf("全部备份（%d 个）", len(included))
		return files, included, bytes, note, nil
	}
}

// fontRestoreNote 还原后的字体情况说明。
// 关键语义：还原只覆盖「包里有的」，包里没有的字体一律保留 ——
// 「不备份字体」的包还原时绝不能把现有字体清空。
// 且完全同名（含扩展名）的字体一律跳过、不改名（见 unzipDirDedupFonts）。
// 必须在解压【之后】调用（此时字体目录已是最终状态）。
func fontRestoreNote(dataDir string, res *fontRestoreResult, man *backupManifest) string {
	final, _ := dirStats(filepath.Join(dataDir, "fonts"))
	if res == nil || res.PackageFonts == 0 {
		return fmt.Sprintf("本包未含字体，已保留当前字体（%d 个）", final)
	}
	base := res.note(final)
	if man != nil && man.FontPolicy == FontPolicySelected {
		base = fmt.Sprintf("本包为选择性备份（只含 %d 个字体）——%s", res.PackageFonts, base)
	}
	return base
}

// backupKindLabel 备份类型的中文标签（与前端文案保持一致）
func backupKindLabel(kind string) string {
	switch kind {
	case KindManual:
		return "手动备份（完整数据：数据库+附件+字体+缩略图）"
	case KindDaily:
		return "每日备份（数据库+缩略图；附件/字体走同日硬链接快照）"
	case KindMonthly:
		return "每月备份（完整数据：数据库+附件+字体+缩略图）"
	case KindSystem:
		return "系统备份（仅前端页面，不含任何族谱数据）"
	}
	return kind
}

// fullBackupDirs 「完整数据备份」打包的目录：
//
//	data/       SQLite 库（含 -wal/-shm；打包前已做 WAL 检查点）
//	upload/     上传的照片/视频/音频
//	thumbnails/ 缩略图缓存（很小，带上还原后观感一致）
//
// 字体（fonts/）不在这里 —— 它由用户选择策略（全部/部分/不备份），
// 以单文件形式打进包（见 resolveFontBackup），这样「只选几个字体」才好实现。
func fullBackupDirs(dataDir string) []string {
	return []string{
		filepath.Join(dataDir, "data"),
		filepath.Join(dataDir, "upload"),
		filepath.Join(dataDir, "thumbnails"),
	}
}

// checkpointWAL 打包备份前把 WAL 内容合并回主库文件。
// 不做的后果（实测）：主库文件落后真实数据 273 行（含传记/账本/附件各 1 行），
// 谁只拿走包里的 data/familytree.db 就会「看起来漏数据」。
func (h *BackupHandler) checkpointWAL(what string) {
	if h.DB == nil {
		return
	}
	if _, err := h.DB.Exec(`PRAGMA wal_checkpoint(TRUNCATE)`); err != nil {
		fmt.Printf("[备份] %s WAL 检查点失败（备份包内仍含 -wal/-shm，可正常还原）: %v\n", what, err)
	}
}

// appVersion 读取当前版本号
func appVersion(db *sql.DB) string {
	var v string
	if db != nil {
		_ = db.QueryRow(`SELECT value FROM settings WHERE key = 'version'`).Scan(&v)
	}
	if v == "" {
		v = "unknown"
	}
	return v
}

// sqliteTableCounts 从 sqlite_master 动态取表并统计行数。
// 不写死表清单 —— 以后新增表自动覆盖，不会再出现「加了表忘了加进备份/清空清单」。
func sqliteTableCounts(db *sql.DB) (map[string]int64, int64) {
	counts := map[string]int64{}
	if db == nil {
		return counts, 0
	}
	rows, err := db.Query(`SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name`)
	if err != nil {
		return counts, 0
	}
	var names []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err == nil && n != "" {
			names = append(names, n)
		}
	}
	rows.Close()

	var total int64
	for _, n := range names {
		var c int64
		if err := db.QueryRow(`SELECT COUNT(*) FROM "` + strings.ReplaceAll(n, `"`, `""`) + `"`).Scan(&c); err == nil {
			counts[n] = c
			total += c
		}
	}
	return counts, total
}

// dirStats 统计目录内文件数与字节数
func dirStats(dir string) (int, int64) {
	var files int
	var bytes int64
	_ = filepath.Walk(dir, func(_ string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		files++
		bytes += info.Size()
		return nil
	})
	return files, bytes
}

// manifestInput buildManifest 的入参（字体策略与目录分开传，便于「只备份部分字体」）
type manifestInput struct {
	Kind        string
	Dirs        []string
	CreatedBy   string
	Notes       string
	NotIncluded []string
	FontPolicy  string   // all | none | selected
	Fonts       []string // 实际打进包的字体文件名
	FontBytes   int64
	FontNote    string
	FontsAvail  int
}

// buildManifest 生成包内清单
func buildManifest(db *sql.DB, in manifestInput) *backupManifest {
	m := &backupManifest{
		ManifestVersion: 1,
		AppVersion:      appVersion(db),
		BackupType:      in.Kind,
		BackupTypeLabel: backupKindLabel(in.Kind),
		GeneratedAt:     time.Now().Format(time.RFC3339),
		CreatedBy:       in.CreatedBy,
		Tables:          map[string]int64{},
		Directories:     map[string]dirStat{},
		FileCounts:      map[string]int{},
		ExcludedDirs:    fullBackupExcludes,
		NotIncluded:     in.NotIncluded,
		Notes:           in.Notes,
	}
	if in.Kind != KindSystem {
		m.Tables, m.TotalRows = sqliteTableCounts(db)
		m.TableCount = len(m.Tables)
		m.WALCheckpoint = "TRUNCATE"
	}
	for _, d := range in.Dirs {
		name := filepath.Base(filepath.Clean(d))
		n, b := dirStats(d)
		m.Directories[name] = dirStat{Files: n, Bytes: b}
		switch name {
		case "data":
			m.ContainsDatabase = true
		case "upload":
			m.ContainsAttachments = true
			m.FileCounts["attachments"] = n
		case "fonts":
			m.ContainsFonts = true
			m.FileCounts["fonts"] = n
		case "thumbnails":
			m.FileCounts["thumbnails"] = n
		case "web":
			m.ContainsWeb = true
			m.FileCounts["web"] = n
		}
	}
	// 字体：显式记录策略、实际清单与字节数（fonts/ 不在 Dirs 里，单独记）
	if in.Kind != KindSystem {
		m.FontPolicy = in.FontPolicy
		if m.FontPolicy == "" {
			m.FontPolicy = FontPolicyAll
		}
		m.FontsAvailable = in.FontsAvail
		m.Fonts = in.Fonts
		m.FontBytes = in.FontBytes
		m.FontPolicyNote = in.FontNote
		m.FontPolicyLabel = fontPolicyLabel(m.FontPolicy, len(in.Fonts), in.FontsAvail)
		if len(in.Fonts) > 0 {
			m.ContainsFonts = true
			m.Directories["fonts"] = dirStat{Files: len(in.Fonts), Bytes: in.FontBytes}
			m.FileCounts["fonts"] = len(in.Fonts)
		}
		if m.FontPolicy == FontPolicyNone {
			m.NotIncluded = append(m.NotIncluded, "上传字体 fonts/（用户选择跳过）")
		}
	}
	return m
}

// marshalManifest 序列化清单（失败时返回 nil，不阻断备份）
func marshalManifest(m *backupManifest) []byte {
	if m == nil {
		return nil
	}
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return nil
	}
	return b
}

// readManifestFromZip 读取包内 backup-manifest.json（老备份包没有 → 返回 nil）
func readManifestFromZip(zipPath string) *backupManifest {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil
	}
	defer zr.Close()
	for _, f := range zr.File {
		if filepath.ToSlash(f.Name) != manifestEntryName {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil
		}
		b, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return nil
		}
		var m backupManifest
		if json.Unmarshal(b, &m) == nil {
			return &m
		}
		return nil
	}
	return nil
}

// verifyRestoredDB 还原后按 manifest 逐表核对行数（还原自检）。
// 返回不一致的描述列表；空列表 = 逐表一致。
func verifyRestoredDB(dataDir string, m *backupManifest) []string {
	if m == nil || !m.ContainsDatabase {
		return nil
	}
	dbPath := filepath.Join(dataDir, "data", "familytree.db")
	db, err := sql.Open("sqlite3", dbPath+"?_busy_timeout=5000")
	if err != nil {
		return []string{"无法打开还原后的数据库：" + err.Error()}
	}
	defer db.Close()
	got, total := sqliteTableCounts(db)
	var problems []string
	for t, want := range m.Tables {
		g, ok := got[t]
		if !ok {
			problems = append(problems, fmt.Sprintf("缺表 %s（应为 %d 行）", t, want))
			continue
		}
		if g != want {
			problems = append(problems, fmt.Sprintf("%s 行数 %d≠%d", t, g, want))
		}
	}
	if m.TableCount > 0 && len(got) < m.TableCount {
		problems = append(problems, fmt.Sprintf("表数 %d < 备份记录 %d", len(got), m.TableCount))
	}
	if m.TotalRows > 0 && total != m.TotalRows {
		problems = append(problems, fmt.Sprintf("总行数 %d≠%d", total, m.TotalRows))
	}
	sort.Strings(problems)
	return problems
}

// fileOpRetry 文件操作重试：Windows 上文件常被短暂占用（SQLite 内存映射、杀软扫描、句柄延迟释放等），
// 失败后每 250ms 重试一次，最多 20 次（约 5 秒）。
func fileOpRetry(fn func() error) error {
	var err error
	for i := 0; i < 20; i++ {
		if err = fn(); err == nil {
			return nil
		}
		time.Sleep(250 * time.Millisecond)
	}
	return err
}

// releaseDBForRestore 覆盖数据库文件前必须关闭 SQLite 连接。
// WAL 模式下 -shm 文件处于内存映射状态，Windows 不关闭连接直接覆盖会报：
// "The requested operation cannot be performed on a file with a user-mapped section open"（Linux 允许多开，故无此问题）。
// 关闭前先做一次 wal_checkpoint(TRUNCATE)，把 WAL 内容合并回主库，再清掉残留的 -wal/-shm。
func (h *BackupHandler) releaseDBForRestore() {
	if h.DB != nil {
		if _, err := h.DB.Exec(`PRAGMA wal_checkpoint(TRUNCATE)`); err != nil {
			fmt.Printf("[还原] WAL 检查点失败: %v\n", err)
		}
		_ = fileOpRetry(func() error { return h.DB.Close() })
	}
	dataDir := filepath.Join(h.Cfg.DataDir, "data")
	for _, n := range []string{"familytree.db-wal", "familytree.db-shm"} {
		p := filepath.Join(dataDir, n)
		if _, err := os.Stat(p); err == nil {
			_ = fileOpRetry(func() error { return os.Remove(p) })
		}
	}
}

// restoreRestartMsg 数据还原后需重启才能重新打开还原后的数据库
func restoreRestartMsg() string {
	if runtime.GOOS == "windows" {
		return "还原完成，程序将自动重启（新窗口）"
	}
	return "还原完成，程序将自动重启以加载还原后的数据"
}

// restartAfterRestore 数据还原后重启服务（重新打开还原后的数据库）。
//   - Linux/Docker：退出进程，由 restart=unless-stopped / 守护脚本拉起；
//   - Windows：先起一个**脱离控制台**的重启器（--relaunch），等本进程退出后在新控制台窗口里启动服务。
//     （不能直接在本控制台里起：旧窗口一旦关闭，附着的服务会被系统杀掉 →「还原后没有再启动」）
func restartAfterRestore() {
	if runtime.GOOS == "windows" {
		if exe, err := os.Executable(); err == nil {
			dir := filepath.Dir(exe)
			args := []string{"--relaunch", exe, strconv.Itoa(os.Getpid())}
			if lp := proc.LauncherPID(); lp > 0 {
				args = append(args, strconv.Itoa(lp)) // 顺手关掉承载旧服务的控制台窗口
			}
			if err := proc.StartDetached(exe, args, dir, filepath.Join(dir, "update-restart.log")); err != nil {
				fmt.Printf("[还原] 启动重启器失败: %v（请手动重新运行 start.bat）\n", err)
			}
		}
	}
	go func() {
		time.Sleep(1500 * time.Millisecond)
		os.Exit(0)
	}()
}

// Backup 一键备份：打包数据库 + 上传附件 + 缩略图 + 按用户选择的字体（存入 manual 目录）
// POST /api/backup
//
// 可选 body（不传 = 老行为 = 全部备份字体，保证向后兼容）：
//
//	{"fonts":"all"|"none"|"selected", "font_names":["xxx.ttf", ...]}
func (h *BackupHandler) Backup(c *gin.Context) {
	var req struct {
		Fonts     string   `json:"fonts"`
		FontNames []string `json:"font_names"`
	}
	// 空 body 会返回 io.EOF：视为「全部备份」，与旧版前端/脚本调用行为一致
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误：" + err.Error()})
		return
	}

	dataDir := h.Cfg.DataDir
	backupDir := filepath.Join(dataDir, "backup", "manual")
	os.MkdirAll(backupDir, 0o755)

	timestamp := time.Now().Format("20060102_150405")
	zipName := fmt.Sprintf("familytree_manual_%s.zip", timestamp)
	zipPath := filepath.Join(backupDir, zipName)

	// 打包前先把 WAL 落回主库，保证主库文件本身就是最新的
	h.checkpointWAL("手动备份")

	// 字体策略：默认全部；selected 只打勾选的（名字必须来自当前字体目录）
	fontsDir := filepath.Join(dataDir, "fonts")
	fontFiles, fontNames, fontBytes, fontNote, ignored := resolveFontBackup(fontsDir, req.Fonts, req.FontNames)
	availCount := len(listFontFiles(fontsDir))

	dirs := fullBackupDirs(dataDir)
	_, uname := currentUser(c)
	policy := req.Fonts
	if policy == "" {
		policy = FontPolicyAll
	}
	m := buildManifest(h.DB, manifestInput{
		Kind:       KindManual,
		Dirs:       dirs,
		CreatedBy:  uname,
		FontPolicy: policy,
		Fonts:      fontNames,
		FontBytes:  fontBytes,
		FontNote:   fontNote,
		FontsAvail: availCount,
		Notes: "完整数据备份：含数据库全部表（传记/账本/成员/用户等）、上传附件、缩略图，以及按所选策略打包的字体。" +
			"不包含程序文件与地图瓦片缓存。",
	})
	if err := zipDirFull(dirs, map[string][]byte{manifestEntryName: marshalManifest(m)}, fontFiles, zipPath); err != nil {
		os.Remove(zipPath)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"ok":           true,
		"file":         zipName,
		"path":         zipPath,
		"manifest":     m,
		"font_policy":  policy,
		"font_count":   len(fontNames),
		"font_bytes":   fontBytes,
		"font_summary": "字体：" + fontPolicyLabel(policy, len(fontNames), availCount),
		"font_ignored": ignored,
	})
}

// SystemBackup 系统备份：只备份前端文件（web 目录），不备份数据，以版本号命名
// POST /api/backup/system
func (h *BackupHandler) SystemBackup(c *gin.Context) {
	webDir := "/data/web"
	if _, err := os.Stat(webDir); os.IsNotExist(err) {
		webDir = "./web"
	}

	backupDir := filepath.Join(h.Cfg.DataDir, "backup", "system")
	os.MkdirAll(backupDir, 0o755)

	// 读取当前版本号
	version := appVersion(h.DB)

	timestamp := time.Now().Format("20060102_150405")
	zipName := fmt.Sprintf("system_v%s_%s.zip", version, timestamp)
	zipPath := filepath.Join(backupDir, zipName)

	// 只备份 web 目录，不备份 data / upload / fonts（纯净的程序页面）
	dirs := []string{webDir}
	m := buildManifest(h.DB, manifestInput{
		Kind:        KindSystem,
		Dirs:        dirs,
		Notes:       "系统备份只含前端页面文件（web/），不含任何族谱数据；要备份数据请用「一键备份」。",
		NotIncluded: []string{"数据库（全部表）", "上传附件 upload/", "上传字体 fonts/", "缩略图 thumbnails/", "地图瓦片 map/"},
	})
	if err := zipDirExtra(dirs, map[string][]byte{manifestEntryName: marshalManifest(m)}, zipPath); err != nil {
		os.Remove(zipPath)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 系统备份只保留最近 10 份，避免无限膨胀
	h.cleanOldBackups(backupDir, 10)

	c.JSON(http.StatusOK, gin.H{
		"ok":       true,
		"file":     zipName,
		"version":  version,
		"manifest": m,
	})
}

// writeGedcomManifest 在 .ged 旁边写一份 .manifest.json 说明。
// GEDCOM 是纯文本标准格式，塞不进 JSON，所以用同名 sidecar 让用户一眼看出「包里有什么、没有什么」。
func (h *BackupHandler) writeGedcomManifest(gedPath, kind string, withAccounts bool, text string) {
	contains := []string{"族人个人记录（INDI）", "家庭/婚姻关系（FAM）", "姓名/字号/生卒/籍贯/职业/学历/头衔", "成员简介（members.bio → NOTE）"}
	notIncluded := []string{
		"传记正文（biographies / member_bio_entries / spouse_bio_entries）",
		"家族账本与账目（account_books / account_entries）",
		"上传的照片/视频/附件（attachments、upload/）",
		"文章、祠堂、事件、牌位、传统等其它表",
		"上传字体（fonts/）",
	}
	if withAccounts {
		contains = append(contains, "成员登录账号与密码哈希（_ACCT/_PWHASH，可恢复原密码）")
	} else {
		notIncluded = append(notIncluded, "成员登录账号与密码哈希（此版本不含，可对外分享）")
	}
	tables, total := sqliteTableCounts(h.DB)
	m := map[string]interface{}{
		"manifest_version": 1,
		"app_version":      appVersion(h.DB),
		"backup_type":      kind,
		"backup_type_label": func() string {
			if withAccounts {
				return "GEDCOM 备份（带账号密码）"
			}
			return "GEDCOM 备份（不含账号，可对外分享）"
		}(),
		"generated_at":     time.Now().Format(time.RFC3339),
		"format":           "GEDCOM 5.5.1",
		"file":             filepath.Base(gedPath),
		"bytes":            int64(len(text)),
		"individuals":      strings.Count(text, "\n0 @I"),
		"families":         strings.Count(text, "\n0 @F"),
		"contains":         contains,
		"not_included":     notIncluded,
		"live_tables":      tables,
		"live_table_count": len(tables),
		"live_total_rows":  total,
		"notes":            "GEDCOM 只承载家谱关系，不含传记正文/账本/照片/字体。需要完整备份（可原样还原）请使用「一键备份」（手动备份）或每日/每月备份。",
	}
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(gedPath+".manifest.json", b, 0o644)
}

// BackupGedcom 备份为 GEDCOM：生成 .ged 文件存入 gedcom 分类目录（不含账号，可对外分享）
// POST /api/backup/gedcom
func (h *BackupHandler) BackupGedcom(c *gin.Context) {
	backupDir := filepath.Join(h.Cfg.DataDir, "backup", "gedcom")
	os.MkdirAll(backupDir, 0o755)

	text, err := GenerateGedcom(h.DB)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	timestamp := time.Now().Format("20060102_150405")
	fileName := fmt.Sprintf("familytree_%s.ged", timestamp)
	filePath := filepath.Join(backupDir, fileName)
	if err := os.WriteFile(filePath, []byte(text), 0o644); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	h.writeGedcomManifest(filePath, "gedcom", false, text)

	c.JSON(http.StatusOK, gin.H{
		"ok":       true,
		"file":     fileName,
		"path":     filePath,
		"manifest": filepath.Base(filePath) + ".manifest.json",
		"warning":  "GEDCOM 只含家谱关系，不含传记正文/账本/照片/字体；完整备份请用「一键备份」",
	})
}

// BackupGedcomAccounts 备份为 GEDCOM（带账号密码）：文件含成员账号与密码哈希，重新导入可恢复原密码
// 仅理事长/超级管理员可调用；存入单独的 gedcom_accounts 分类目录
// POST /api/backup/gedcom-accounts
func (h *BackupHandler) BackupGedcomAccounts(c *gin.Context) {
	role := c.GetString("role")
	if role != RoleChief && role != RoleSuperAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "仅理事长/超级管理员可备份带账号密码的 GEDCOM"})
		return
	}
	backupDir := filepath.Join(h.Cfg.DataDir, "backup", "gedcom_accounts")
	os.MkdirAll(backupDir, 0o755)

	text, err := GenerateGedcomWithAccounts(h.DB)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	timestamp := time.Now().Format("20060102_150405")
	fileName := fmt.Sprintf("familytree_accounts_%s.ged", timestamp)
	filePath := filepath.Join(backupDir, fileName)
	if err := os.WriteFile(filePath, []byte(text), 0o644); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	h.writeGedcomManifest(filePath, "gedcom_accounts", true, text)

	c.JSON(http.StatusOK, gin.H{
		"ok":       true,
		"file":     fileName,
		"path":     filePath,
		"manifest": filepath.Base(filePath) + ".manifest.json",
		"warning":  "GEDCOM 只含家谱关系与账号，不含传记正文/账本/照片/字体；完整备份请用「一键备份」",
	})
}

// List 列出备份包（支持 daily/monthly/manual/system/gedcom/gedcom_accounts/media 子目录）
// GET /api/backup/list?type=daily
func (h *BackupHandler) List(c *gin.Context) {
	backupDir := filepath.Join(h.Cfg.DataDir, "backup")
	typ := c.DefaultQuery("type", "daily")
	// 带账号密码的 GEDCOM（含密码哈希）仅理事长/超级管理员可查看
	if typ == "gedcom_accounts" {
		if role := c.GetString("role"); role != RoleChief && role != RoleSuperAdmin {
			c.JSON(http.StatusForbidden, gin.H{"error": "无权限"})
			return
		}
	}
	dir := filepath.Join(backupDir, typ)
	type fileInfo struct {
		Name string `json:"name"`
		Size int64  `json:"size"`
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		c.JSON(http.StatusOK, []fileInfo{})
		return
	}
	list := []fileInfo{}
	for _, e := range entries {
		// media / fonts 快照是目录（硬链接去重）
		if typ == "media" || typ == "fonts" {
			if !e.IsDir() {
				continue
			}
			list = append(list, fileInfo{Name: e.Name(), Size: dirSize(filepath.Join(dir, e.Name()))})
			continue
		}
		if e.IsDir() {
			continue
		}
		ext := filepath.Ext(e.Name())
		if ext != ".zip" && ext != ".ged" {
			continue
		}
		info, _ := e.Info()
		list = append(list, fileInfo{Name: e.Name(), Size: info.Size()})
	}
	// 备份文件名含时间戳（零填充），按名字倒序 = 最新的在最前面
	sort.Slice(list, func(i, j int) bool { return list[i].Name > list[j].Name })
	c.JSON(http.StatusOK, list)
}

// dirSize 递归计算目录总大小
func dirSize(path string) int64 {
	var total int64
	filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			total += info.Size()
		}
		return nil
	})
	return total
}

// Download 下载备份包
// GET /api/backup/download?file=xxx.zip&type=daily
// type=media 时：把指定快照目录临时打包成 zip 下载
func (h *BackupHandler) Download(c *gin.Context) {
	name := c.Query("file")
	typ := c.DefaultQuery("type", "daily")
	backupDir := filepath.Join(h.Cfg.DataDir, "backup", typ)
	path := filepath.Join(backupDir, name)

	if typ == "media" || typ == "fonts" {
		if st, err := os.Stat(path); err != nil || !st.IsDir() {
			c.JSON(http.StatusNotFound, gin.H{"error": "快照不存在"})
			return
		}
		// 打包到临时文件
		tmpZip := filepath.Join(os.TempDir(), name+".zip")
		if err := zipDir([]string{path}, tmpZip); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer os.Remove(tmpZip)
		c.FileAttachment(tmpZip, name+".zip")
		return
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "文件不存在"})
		return
	}
	c.FileAttachment(path, name)
}

// Restore 从备份包还原
// POST /api/restore  body: {"file": "xxx.zip", "type": "daily"}
func (h *BackupHandler) Restore(c *gin.Context) {
	var req struct {
		File string `json:"file"`
		Type string `json:"type"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Type == "" {
		req.Type = "daily"
	}

	// 如果是系统备份，还原到 web 目录
	if req.Type == "system" {
		backupDir := filepath.Join(h.Cfg.DataDir, "backup", "system")
		zipPath := filepath.Join(backupDir, req.File)
		if _, err := os.Stat(zipPath); os.IsNotExist(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "备份文件不存在"})
			return
		}
		// 系统备份还原到 web 根目录
		if err := unzipDir(zipPath, "/"); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "message": "系统文件已还原，请刷新页面"})
		return
	}

	// 多媒体快照还原：把快照目录内容覆盖到 upload 目录
	if req.Type == "media" || req.Type == "fonts" {
		sub := "media"
		dst := filepath.Join(h.Cfg.DataDir, "upload")
		label := "多媒体"
		if req.Type == "fonts" {
			sub = "fonts"
			dst = filepath.Join(h.Cfg.DataDir, "fonts")
			label = "字体"
		}
		snapDir := filepath.Join(h.Cfg.DataDir, "backup", sub, req.File)
		if st, err := os.Stat(snapDir); err != nil || !st.IsDir() {
			c.JSON(http.StatusNotFound, gin.H{"error": "快照不存在"})
			return
		}
		if err := os.RemoveAll(dst); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		os.MkdirAll(dst, 0o755)
		if err := copyTree(snapDir, dst); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		files, _ := dirStats(dst)
		c.JSON(http.StatusOK, gin.H{"ok": true, "files": files, "message": label + "已从快照还原（" + strconv.Itoa(files) + " 个文件）"})
		return
	}

	backupDir := filepath.Join(h.Cfg.DataDir, "backup", req.Type)
	zipPath := filepath.Join(backupDir, req.File)
	if _, err := os.Stat(zipPath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "备份文件不存在"})
		return
	}
	// 覆盖前先读包内清单（还原后自检用；老备份包没有这个文件 → nil）
	man := readManifestFromZip(zipPath)
	// 覆盖前先释放 SQLite 句柄（Windows 上 WAL 的 -shm 处于内存映射状态，不释放就无法覆盖）
	h.releaseDBForRestore()
	var fontRes *fontRestoreResult
	if err := fileOpRetry(func() error {
		var e error
		fontRes, e = unzipDirDedupFonts(zipPath, h.Cfg.DataDir, true)
		return e
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "还原失败：" + err.Error() + "（数据库连接已关闭，程序即将退出，请重新启动）"})
		restartAfterRestore()
		return
	}
	problems := verifyRestoredDB(h.Cfg.DataDir, man)
	// 字体说明：包里没有字体时明确告知「已保留当前字体」；同名同格式的跳过明细也在这里
	fontNote := fontRestoreNote(h.Cfg.DataDir, fontRes, man)
	c.JSON(http.StatusOK, gin.H{"ok": true, "needRestart": true, "autoRestart": true,
		"verify":       verifyResult(problems, man) + "；" + fontNote,
		"font_note":    fontNote,
		"font_restore": fontRes,
		"message":      restoreRestartMsg() + "。" + fontNote})
	restartAfterRestore()
}

// verifyResult 把还原自检结果整理成一句话（无清单/非数据备份时说明未做自检）
func verifyResult(problems []string, man *backupManifest) string {
	if man == nil || !man.ContainsDatabase {
		return "该备份包内没有 backup-manifest.json（多为旧版备份），已跳过逐表自检"
	}
	if len(problems) == 0 {
		return "已按包内 backup-manifest.json 逐表核对：" + strconv.Itoa(man.TableCount) + " 张表 " + strconv.FormatInt(man.TotalRows, 10) + " 行全部一致"
	}
	return "⚠ 还原自检发现问题：" + strings.Join(problems, "；")
}

// UploadRestore 上传备份文件并还原（把本机下载的 .zip 备份传回系统还原）
// POST /api/restore/upload  multipart/form-data：file=备份zip，type=data|system|auto（默认 auto）
// 还原前自动把当前状态备份到 manual/pre_restore_*.zip（安全网）；数据还原后自动重启进程以重新加载数据库
func (h *BackupHandler) UploadRestore(c *gin.Context) {
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请选择备份文件"})
		return
	}
	defer file.Close()

	if header != nil && header.Size > 2<<30 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "备份文件过大（上限 2GB）"})
		return
	}

	kind := c.PostForm("type")
	if kind == "" {
		kind = "auto"
	}

	manualDir := filepath.Join(h.Cfg.DataDir, "backup", "manual")
	os.MkdirAll(manualDir, 0o755)
	ts := time.Now().Format("20060102_150405")
	origName := "backup.zip"
	if header != nil && strings.TrimSpace(header.Filename) != "" {
		origName = filepath.Base(filepath.ToSlash(header.Filename))
	}
	zipPath := filepath.Join(manualDir, fmt.Sprintf("uploaded_%s_%s", ts, origName))

	dst, err := os.Create(zipPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存上传文件失败：" + err.Error()})
		return
	}
	_, copyErr := io.Copy(dst, file)
	dst.Close()
	if copyErr != nil {
		os.Remove(zipPath)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存上传文件失败：" + copyErr.Error()})
		return
	}

	// 校验：必须是 zip、条目路径安全（防目录穿越）、且内容与声明的类型相符
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		os.Remove(zipPath)
		c.JSON(http.StatusBadRequest, gin.H{"error": "不是有效的备份文件（需要 .zip）"})
		return
	}
	hasDB, hasWeb, unsafe := false, false, false
	for _, f := range zr.File {
		n := filepath.ToSlash(f.Name)
		if strings.HasPrefix(n, "/") || strings.Contains(n, "../") || strings.HasPrefix(n, "..") {
			unsafe = true
			break
		}
		if strings.HasSuffix(n, "data/familytree.db") {
			hasDB = true
		}
		if n == "web/index.html" || strings.HasSuffix(n, "/web/index.html") {
			hasWeb = true
		}
	}
	zr.Close()
	if unsafe {
		os.Remove(zipPath)
		c.JSON(http.StatusBadRequest, gin.H{"error": "备份文件路径异常，已拒绝还原"})
		return
	}

	if kind == "auto" {
		if hasDB {
			kind = "data"
		} else if hasWeb {
			kind = "system"
		}
	}
	if kind == "data" && !hasDB {
		os.Remove(zipPath)
		c.JSON(http.StatusBadRequest, gin.H{"error": "该文件里没有数据库（data/familytree.db），不是数据备份包"})
		return
	}
	if kind == "system" && !hasWeb {
		os.Remove(zipPath)
		c.JSON(http.StatusBadRequest, gin.H{"error": "该文件里没有前端文件（web/index.html），不是系统备份包"})
		return
	}
	if kind != "data" && kind != "system" {
		os.Remove(zipPath)
		c.JSON(http.StatusBadRequest, gin.H{"error": "无法识别该备份文件的类型"})
		return
	}

	// 系统（前端）还原：解到实际 web 目录的父目录
	if kind == "system" {
		webDir := "/data/web"
		if _, err := os.Stat(webDir); os.IsNotExist(err) {
			webDir = "./web"
		}
		if err := unzipDir(zipPath, filepath.Dir(webDir)); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "还原失败：" + err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "type": "system", "needRestart": false,
			"message": "系统文件已还原，请刷新页面"})
		return
	}

	// 数据还原：先把当前状态打包留底（打包前先 checkpoint，确保数据都在主库里），再解压覆盖
	preName := fmt.Sprintf("pre_restore_%s.zip", ts)
	dataDir := filepath.Join(h.Cfg.DataDir, "data")
	uploadDir := filepath.Join(h.Cfg.DataDir, "upload")
	if h.DB != nil {
		h.DB.Exec(`PRAGMA wal_checkpoint(TRUNCATE)`)
	}
	preDirs := []string{dataDir, uploadDir, filepath.Join(h.Cfg.DataDir, "fonts"), filepath.Join(h.Cfg.DataDir, "thumbnails")}
	preFontsDir := filepath.Join(h.Cfg.DataDir, "fonts")
	_, preFontNames, preFontBytes, preFontNote, _ := resolveFontBackup(preFontsDir, FontPolicyAll, nil)
	preManifest := buildManifest(h.DB, manifestInput{
		Kind:       KindManual,
		Dirs:       preDirs,
		FontPolicy: FontPolicyAll,
		Fonts:      preFontNames,
		FontBytes:  preFontBytes,
		FontNote:   preFontNote,
		FontsAvail: len(preFontNames),
		Notes:      "还原前自动留底（完整数据备份，含全部字体）",
	})
	if err := zipDirExtra(preDirs, map[string][]byte{manifestEntryName: marshalManifest(preManifest)}, filepath.Join(manualDir, preName)); err != nil {
		preName = ""
	}
	// 覆盖前读包内清单（还原后自检用）
	man := readManifestFromZip(zipPath)
	// 覆盖前释放 SQLite 句柄（Windows 上 WAL 的 -shm 处于内存映射状态，不释放就无法覆盖）
	h.releaseDBForRestore()
	var fontRes *fontRestoreResult
	if err := fileOpRetry(func() error {
		var e error
		fontRes, e = unzipDirDedupFonts(zipPath, h.Cfg.DataDir, true)
		return e
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "还原失败：" + err.Error() + "（数据库连接已关闭，程序即将退出，请重新启动）"})
		restartAfterRestore()
		return
	}

	problems := verifyRestoredDB(h.Cfg.DataDir, man)
	// 字体说明：包里没有字体时明确告知「已保留当前字体」；同名同格式的跳过明细也在这里
	fontNote := fontRestoreNote(h.Cfg.DataDir, fontRes, man)
	msg := "数据已还原"
	if preName != "" {
		msg += "；还原前的状态已备份为 " + preName
	}
	msg += "；" + verifyResult(problems, man)
	msg += "；" + fontNote
	msg += "；" + restoreRestartMsg()
	c.JSON(http.StatusOK, gin.H{"ok": true, "type": "data", "needRestart": true, "autoRestart": true,
		"verify":       verifyResult(problems, man),
		"font_note":    fontNote,
		"font_restore": fontRes, "message": msg})
	restartAfterRestore()
}

// DeleteBackup 删除备份文件
// POST /api/backup/delete  body: {"file": "xxx.zip", "type": "daily"}
func (h *BackupHandler) DeleteBackup(c *gin.Context) {
	var req struct {
		File string `json:"file"`
		Type string `json:"type"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Type == "" {
		req.Type = "daily"
	}
	backupDir := filepath.Join(h.Cfg.DataDir, "backup", req.Type)
	path := filepath.Join(backupDir, req.File)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "文件不存在"})
		return
	}
	if req.Type == "media" || req.Type == "fonts" {
		if err := os.RemoveAll(path); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	} else {
		if err := os.Remove(path); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		// GEDCOM 的说明文件是 .ged.manifest.json，一起删掉
		if ext := filepath.Ext(req.File); ext == ".ged" {
			os.Remove(path + ".manifest.json")
		}
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// Reset 恢复初始设置：清空所有业务数据（保留备份文件、账号、系统设置/授权）
// POST /api/backup/reset
func (h *BackupHandler) Reset(c *gin.Context) {
	// 表清单从 sqlite_master 动态取（以后新增表自动覆盖），只保留三类必须留存的表：
	//   users      账号（清掉就没人能登录了）
	//   settings   家族设置 / 授权码 / 加密密钥（清掉授权与密钥全丢）
	//   migrations 迁移记录（清掉会重跑迁移）
	// 旧实现写死 8 张表，实测 24 张表里有 16 张没清（传记、账本、房支、草稿、牌位…残留 232 行）。
	keep := map[string]bool{"users": true, "settings": true, "migrations": true}
	tables := []string{}
	if h.DB != nil {
		if rows, err := h.DB.Query(`SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name`); err == nil {
			for rows.Next() {
				var n string
				if rows.Scan(&n) == nil && n != "" && !keep[n] {
					tables = append(tables, n)
				}
			}
			rows.Close()
		}
	}
	// 先做 WAL 检查点，避免 DELETE 只落在 -wal 里
	h.checkpointWAL("恢复初始设置")
	for _, t := range tables {
		h.DB.Exec(`DELETE FROM "` + strings.ReplaceAll(t, `"`, `""`) + `"`)
	}
	// 重置自增ID（SQLite 用 sqlite_sequence）
	h.DB.Exec(`DELETE FROM sqlite_sequence`)
	// 清除上传文件与缩略图缓存（缩略图是附件的派生物，附件没了留着只会显示错图）；字体保留
	for _, d := range []string{"upload", "thumbnails"} {
		p := filepath.Join(h.Cfg.DataDir, d)
		os.RemoveAll(p)
		os.MkdirAll(p, 0o755)
	}
	c.JSON(http.StatusOK, gin.H{
		"ok":            true,
		"cleared":       len(tables),
		"kept":          []string{"users", "settings", "migrations"},
		"cleared_names": tables,
	})
}

// zipDir 将多个目录打包进 zip
func zipDir(dirs []string, zipPath string) error {
	return zipDirExtra(dirs, nil, zipPath)
}

// zipDirExtra 将多个目录打包进 zip；extra 为额外写入 zip 根目录的条目（文件名 → 内容），
// 用于塞 backup-manifest.json（先写清单，解压第一眼就能看到）。
func zipDirExtra(dirs []string, extra map[string][]byte, zipPath string) error {
	return zipDirFull(dirs, extra, nil, zipPath)
}

// zipDirFull 将多个目录 + 若干单独指定的文件打包进 zip。
// files 用于「只备份用户勾选的字体」这类按文件挑选的场景（entry 名已含 fonts/ 前缀）。
// 注意：文件句柄用完立即关闭（原来用 defer 累积，附件多时会「too many open files」导致备份失败）；
// 写失败会删掉半成品包，避免留下一个能下载但打不开的假备份。
func zipDirFull(dirs []string, extra map[string][]byte, files []zipFileEntry, zipPath string) error {
	zf, err := os.Create(zipPath)
	if err != nil {
		return err
	}

	zw := zip.NewWriter(zf)
	abort := func(e error) error {
		zw.Close()
		zf.Close()
		os.Remove(zipPath)
		return e
	}

	// 先写包内清单（按名字排序，保证输出可复现）
	extraNames := make([]string, 0, len(extra))
	for n := range extra {
		extraNames = append(extraNames, n)
	}
	sort.Strings(extraNames)
	for _, n := range extraNames {
		fw, err := zw.Create(n)
		if err != nil {
			return abort(err)
		}
		if _, err := fw.Write(extra[n]); err != nil {
			return abort(err)
		}
	}

	for _, dir := range dirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			continue // 目录不存在（例如还没上传过字体）不算失败
		}
		base := filepath.Dir(dir)
		err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}
			rel, relErr := filepath.Rel(base, path)
			if relErr != nil {
				return relErr
			}
			fw, err := zw.Create(filepath.ToSlash(rel))
			if err != nil {
				return err
			}
			f, err := os.Open(path)
			if err != nil {
				return err
			}
			_, copyErr := io.Copy(fw, f)
			f.Close() // 立即关闭，避免句柄耗尽
			return copyErr
		})
		if err != nil {
			return abort(err)
		}
	}

	// 再写单独指定的文件（例如用户勾选的字体）
	for _, fe := range files {
		if fe.Name == "" || fe.Path == "" {
			continue
		}
		if st, statErr := os.Stat(fe.Path); statErr != nil || st.IsDir() {
			continue // 备份期间文件被删掉也没关系，跳过即可
		}
		fw, err := zw.Create(filepath.ToSlash(fe.Name))
		if err != nil {
			return abort(err)
		}
		f, err := os.Open(fe.Path)
		if err != nil {
			return abort(err)
		}
		_, copyErr := io.Copy(fw, f)
		f.Close()
		if copyErr != nil {
			return abort(copyErr)
		}
	}
	if err := zw.Close(); err != nil {
		zf.Close()
		os.Remove(zipPath)
		return err
	}
	return zf.Close()
}

// fontRestoreResult 包内字体的还原结果（用于还原结果里的明确报告）
type fontRestoreResult struct {
	Restored     []string `json:"restored"`                  // 实际写入的字体
	SkippedSame  []string `json:"skipped_same"`              // 同名同格式且内容相同 → 跳过
	SkippedDiff  []string `json:"skipped_same_content_diff"` // 同名同格式但内容不同 → 也跳过（不覆盖）
	PackageFonts int      `json:"package_fonts"`             // 包内字体条目总数
}

// note 生成「字体：还原 N / 跳过 M（同名同格式）」这一句
func (r *fontRestoreResult) note(currentTotal int) string {
	if r == nil || r.PackageFonts == 0 {
		return fmt.Sprintf("本包未含字体，已保留当前字体（%d 个）", currentTotal)
	}
	s := fmt.Sprintf("字体：还原 %d 个", len(r.Restored))
	skipped := len(r.SkippedSame) + len(r.SkippedDiff)
	if skipped > 0 {
		s += fmt.Sprintf("；跳过 %d 个同名同格式（未覆盖、未改名，不会产生重复字体）", skipped)
		if len(r.SkippedDiff) > 0 {
			s += fmt.Sprintf("，其中 %d 个内容与包内不同：%s（如需包内版本请先删除该字体再还原）",
				len(r.SkippedDiff), strings.Join(r.SkippedDiff, "、"))
		}
	}
	s += fmt.Sprintf("；当前字体目录共 %d 个", currentTotal)
	return s
}

// contentEqual 比较磁盘文件与 zip 条目内容是否完全一致（分块哈希，恒定内存）
func contentEqual(path string, r io.Reader) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	h1 := sha256.New()
	h2 := sha256.New()
	if _, err := io.Copy(h1, f); err != nil {
		return false
	}
	if _, err := io.Copy(h2, r); err != nil {
		return false
	}
	return bytes.Equal(h1.Sum(nil), h2.Sum(nil))
}

// unzipDir 解压 zip 到目标目录
func unzipDir(zipPath, dest string) error {
	_, err := unzipDirDedupFonts(zipPath, dest, false)
	return err
}

// unzipDirDedupFonts 解压 zip 到目标目录。
// dedupFonts=true 时对 fonts/ 下的条目启用「同名即跳过」去重规则（属主要求）：
//   - 目标目录已存在【完全同名（含扩展名）】的文件 → 跳过，既不覆盖也不改名
//     （绝不用 uniqueFilePath 生成 X-2.ttf 那种副本，那正是不想要的重复）；
//   - 同名但扩展名不同（X.ttf vs X.woff2）→ 视为不同字体，照常还原；
//   - 服务端没有的 → 照常还原；
//   - 跳过时区分「内容相同」与「内容不同」，后者在还原结果里明确报告，由用户决定是否删除后重导。
//
// 其它目录（data/upload/thumbnails）行为不变（照旧覆盖）。
func unzipDirDedupFonts(zipPath, dest string, dedupFonts bool) (*fontRestoreResult, error) {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, err
	}
	defer zr.Close()

	res := &fontRestoreResult{}
	for _, f := range zr.File {
		name := filepath.ToSlash(f.Name)
		// 包内清单不是数据文件，不落盘
		if name == manifestEntryName {
			continue
		}
		dst := filepath.Join(dest, f.Name)
		if f.FileInfo().IsDir() {
			os.MkdirAll(dst, 0o755)
			continue
		}
		if dedupFonts && strings.HasPrefix(name, "fonts/") {
			res.PackageFonts++
			// 只在「完全同名」时跳过；同名不同格式（扩展名不同）走下面的正常写入分支
			if st, statErr := os.Stat(dst); statErr == nil && !st.IsDir() {
				base := filepath.Base(name)
				same := false
				if rc, openErr := f.Open(); openErr == nil {
					same = contentEqual(dst, rc)
					rc.Close()
				}
				if same {
					res.SkippedSame = append(res.SkippedSame, base)
				} else {
					res.SkippedDiff = append(res.SkippedDiff, base)
				}
				continue
			}
			os.MkdirAll(filepath.Dir(dst), 0o755)
			if err := writeZipEntry(f, dst); err != nil {
				return res, err
			}
			res.Restored = append(res.Restored, filepath.Base(name))
			continue
		}
		os.MkdirAll(filepath.Dir(dst), 0o755)
		if err := writeZipEntry(f, dst); err != nil {
			return res, err
		}
	}
	sort.Strings(res.Restored)
	sort.Strings(res.SkippedSame)
	sort.Strings(res.SkippedDiff)
	return res, nil
}

// writeZipEntry 把一个 zip 条目写到磁盘
func writeZipEntry(f *zip.File, dst string) error {
	df, err := os.Create(dst)
	if err != nil {
		return err
	}
	src, err := f.Open()
	if err != nil {
		df.Close()
		return err
	}
	_, copyErr := io.Copy(df, src)
	src.Close()
	if closeErr := df.Close(); copyErr == nil {
		copyErr = closeErr
	}
	return copyErr
}

// copyTree 递归复制目录内容（用于多媒体快照还原）
func copyTree(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		os.MkdirAll(filepath.Dir(target), 0o755)
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := os.Create(target)
		if err != nil {
			return err
		}
		defer out.Close()
		_, err = io.Copy(out, in)
		return err
	})
}

// cleanOldBackups 清理指定目录下多余的 .zip 备份，按修改时间只保留最近 keep 份
func (h *BackupHandler) cleanOldBackups(dir string, keep int) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	type zipFile struct {
		name    string
		modTime time.Time
	}
	var zips []zipFile
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".zip" {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		zips = append(zips, zipFile{name: e.Name(), modTime: info.ModTime()})
	}
	if len(zips) <= keep {
		return
	}
	sort.Slice(zips, func(i, j int) bool { return zips[i].modTime.After(zips[j].modTime) })
	for i := keep; i < len(zips); i++ {
		os.Remove(filepath.Join(dir, zips[i].name))
	}
}
