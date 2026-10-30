package backup

import (
	"archive/zip"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"familytree/internal/gedcom"

	// 调度器自己要开库做 WAL 检查点，必须确保 sqlite3 驱动已注册。
	// 不能依赖 main 恰好 import 了 internal/db —— 否则这里会静默降级成「打不开库、不检查点」。
	_ "github.com/mattn/go-sqlite3"
)

// manifestEntryName 包内清单文件名（与 handler/backup.go 保持一致）
const manifestEntryName = "backup-manifest.json"

// 备份类型标签（与 handler/backup.go 的 backupKindLabel 口径一致）
const (
	kindDaily   = "daily"
	kindMonthly = "monthly"
	kindSystem  = "system"
)

// dirStat 目录文件数与字节数
type dirStat struct {
	Files int   `json:"files"`
	Bytes int64 `json:"bytes"`
}

// backupManifest 包内 backup-manifest.json
// ⚠ 字段口径与 handler/backup.go 的 backupManifest 保持一致（两个包各有入口，避免循环依赖故各自实现）
type backupManifest struct {
	ManifestVersion     int                `json:"manifest_version"`
	AppVersion          string             `json:"app_version"`
	BackupType          string             `json:"backup_type"`
	BackupTypeLabel     string             `json:"backup_type_label"`
	GeneratedAt         string             `json:"generated_at"`
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
	Snapshots           map[string]string  `json:"snapshots,omitempty"`
	NotIncluded         []string           `json:"not_included,omitempty"`
	Notes               string             `json:"notes"`
}

// backupKindLabel 备份类型中文标签
func backupKindLabel(kind string) string {
	switch kind {
	case kindDaily:
		return "每日备份（数据库+缩略图；附件/字体走同日硬链接快照）"
	case kindMonthly:
		return "每月备份（完整数据：数据库+附件+字体+缩略图）"
	case kindSystem:
		return "系统备份（仅前端页面，不含任何族谱数据）"
	}
	return kind
}

// Scheduler 自动备份调度器
type Scheduler struct {
	DataDir  string
	Timezone string
	stopCh   chan struct{}
	loc      *time.Location
}

// Start 启动定时备份
// - 每日 02:00 后：数据库全量日备份 + GEDCOM 快照 + 多媒体硬链接快照
// - 每月 1 日 03:00 后：月备份（全量）
// - 启动时自动做一次系统备份（前端文件，以版本号命名）
func (s *Scheduler) Start() {
	loc, err := time.LoadLocation(s.Timezone)
	if err != nil || loc == nil {
		loc = time.Local
	}
	s.loc = loc

	s.stopCh = make(chan struct{})
	go s.loop()

	// 启动时做一次系统备份
	if err := s.systemBackup(); err != nil {
		log.Printf("[备份] 启动时系统备份失败: %v", err)
	}

	log.Printf("[备份] 自动备份调度已启动（每日02:00 保留30天 / 每月1日03:00 / GEDCOM快照保留30天 / 多媒体硬链接快照保留30天，时区 %s）", s.Timezone)
}

// Stop 停止调度
func (s *Scheduler) Stop() {
	close(s.stopCh)
}

func (s *Scheduler) loop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	s.checkAndRun()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.checkAndRun()
		}
	}
}

func (s *Scheduler) checkAndRun() {
	now := time.Now().In(s.loc)

	// 每日：凌晨2点后补做
	if now.Hour() >= 2 {
		// 先做大体积二进制的硬链接快照（附件/字体），再做数据库日备份
		// —— 这样日备份包内的 manifest 能准确指向当天已生成的快照
		if err := s.mediaSnapshot(); err != nil {
			log.Printf("[备份] 多媒体快照失败: %v", err)
		}
		if err := s.fontsSnapshot(); err != nil {
			log.Printf("[备份] 字体快照失败: %v", err)
		}
		if err := s.dailyBackup(); err != nil {
			log.Printf("[备份] 每日备份失败: %v", err)
		}
		if err := s.gedcomSnapshot(); err != nil {
			log.Printf("[备份] GEDCOM 快照失败: %v", err)
		}
	}

	// 每月备份：1号凌晨3点后补做
	if now.Day() == 1 && now.Hour() >= 3 {
		if err := s.monthlyBackup(); err != nil {
			log.Printf("[备份] 每月备份失败: %v", err)
		}
	}
}

// checkpointWAL 打包前把 WAL 合并回主库文件。
// 只打包 data/ 时尤其重要：实测主库文件落后真实数据 273 行（传记/账本/附件各 1 行），
// 只看主库文件会以为「备份漏数据」。检查点失败不影响还原（包内仍含 -wal/-shm）。
func (s *Scheduler) checkpointWAL(what string) {
	db, err := s.openDB()
	if err != nil {
		log.Printf("[备份] %s 跳过 WAL 检查点（打不开库）: %v", what, err)
		return
	}
	defer db.Close()
	if _, err := db.Exec(`PRAGMA wal_checkpoint(TRUNCATE)`); err != nil {
		log.Printf("[备份] %s WAL 检查点失败（包内仍含 -wal/-shm，可正常还原）: %v", what, err)
	}
}

// tableCounts 从 sqlite_master 动态取表统计行数（不写死表清单）
func (s *Scheduler) tableCounts() (map[string]int64, int64) {
	counts := map[string]int64{}
	db, err := s.openDB()
	if err != nil {
		return counts, 0
	}
	defer db.Close()
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

// appVersion 读取当前版本号
func (s *Scheduler) appVersion() string {
	db, err := s.openDB()
	if err != nil {
		return "unknown"
	}
	defer db.Close()
	var v string
	_ = db.QueryRow(`SELECT value FROM settings WHERE key = 'version'`).Scan(&v)
	if v == "" {
		v = "unknown"
	}
	return v
}

// buildManifest 生成包内清单
func (s *Scheduler) buildManifest(kind string, dirs []string, snapshots map[string]string, notes string) []byte {
	m := &backupManifest{
		ManifestVersion: 1,
		AppVersion:      s.appVersion(),
		BackupType:      kind,
		BackupTypeLabel: backupKindLabel(kind),
		GeneratedAt:     time.Now().In(s.loc).Format(time.RFC3339),
		Tables:          map[string]int64{},
		Directories:     map[string]dirStat{},
		FileCounts:      map[string]int{},
		Snapshots:       snapshots,
		Notes:           notes,
	}
	if kind != kindSystem {
		m.Tables, m.TotalRows = s.tableCounts()
		m.TableCount = len(m.Tables)
		m.WALCheckpoint = "TRUNCATE"
	}
	for _, d := range dirs {
		name := filepath.Base(filepath.Clean(d))
		n, b := dirStatsOf(d)
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
	if kind == kindSystem {
		m.NotIncluded = []string{"数据库（全部表）", "上传附件 upload/", "上传字体 fonts/", "缩略图 thumbnails/", "地图瓦片 map/"}
	}
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return nil
	}
	return b
}

// dirStatsOf 统计目录内文件数与字节数
func dirStatsOf(dir string) (int, int64) {
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

// systemBackup 系统备份：只备份前端文件，不备份数据
func (s *Scheduler) systemBackup() error {
	webDir := "/data/web"
	if _, err := os.Stat(webDir); os.IsNotExist(err) {
		webDir = "./web"
	}

	backupDir := filepath.Join(s.DataDir, "backup", "system")
	os.MkdirAll(backupDir, 0o755)

	timestamp := time.Now().In(s.loc).Format("20060102_150405")
	zipName := fmt.Sprintf("system_%s.zip", timestamp)
	zipPath := filepath.Join(backupDir, zipName)

	man := s.buildManifest(kindSystem, []string{webDir}, nil,
		"系统备份只含前端页面文件（web/），不含任何族谱数据；要备份数据请用「一键备份」。")
	if err := zipDirsExtra([]string{webDir}, map[string][]byte{manifestEntryName: man}, zipPath); err != nil {
		os.Remove(zipPath)
		return err
	}

	// 系统备份只保留最近 10 份（每次启动/更新重启都会生成，不清理会无限膨胀）
	s.cleanOld(backupDir, 10)

	log.Printf("[备份] 系统备份完成: %s", zipName)
	return nil
}

// dailyBackup 数据库全量日备份
// 打包内容：data/（SQLite 库，打包前已 checkpoint）+ thumbnails/ + backup-manifest.json。
// upload/（65MB+）与 fonts/（200MB+）是大体积二进制，走同日的硬链接快照（media / fonts），
// 不重复塞进 30 份日备份 zip —— 否则 30 天就是数 GB 的重复数据。
func (s *Scheduler) dailyBackup() error {
	dir := filepath.Join(s.DataDir, "backup", "daily")
	os.MkdirAll(dir, 0o755)
	timestamp := time.Now().In(s.loc).Format("20060102")
	zipName := fmt.Sprintf("familytree_daily_%s.zip", timestamp)
	zipPath := filepath.Join(dir, zipName)

	if _, err := os.Stat(zipPath); err == nil {
		return nil
	}
	if matches, _ := filepath.Glob(filepath.Join(dir, "familytree_daily_"+timestamp+"_*.zip")); len(matches) > 0 {
		return nil
	}

	s.checkpointWAL("每日备份")

	dirs := []string{
		filepath.Join(s.DataDir, "data"),
		filepath.Join(s.DataDir, "thumbnails"),
	}
	snapshots := map[string]string{
		"media": filepath.ToSlash(filepath.Join("backup", "media", timestamp)) + "（上传附件，硬链接去重）",
		"fonts": filepath.ToSlash(filepath.Join("backup", "fonts", timestamp)) + "（上传字体，硬链接去重）",
	}
	man := s.buildManifest(kindDaily, dirs, snapshots,
		"数据库日备份。上传附件与上传字体体积大且多为重复内容，按天做硬链接快照（backup/media、backup/fonts）去重保存，还原时可在「多媒体快照 / 字体快照」里选择对应日期。")
	if err := zipDirsExtra(dirs, map[string][]byte{manifestEntryName: man}, zipPath); err != nil {
		os.Remove(zipPath)
		return err
	}
	s.cleanOld(dir, 30)
	log.Printf("[备份] 每日备份完成: %s", zipName)
	return nil
}

// gedcomSnapshot 生成每日 GEDCOM 文字快照（保留 30 天）
// 同时生成两份：gedcom/ 普通版（不含账号，可对外分享）、gedcom_accounts/ 带账号密码版（可恢复登录密码）
func (s *Scheduler) gedcomSnapshot() error {
	dir := filepath.Join(s.DataDir, "backup", "gedcom")
	accDir := filepath.Join(s.DataDir, "backup", "gedcom_accounts")
	os.MkdirAll(dir, 0o755)
	os.MkdirAll(accDir, 0o755)
	timestamp := time.Now().In(s.loc).Format("20060102")
	fileName := fmt.Sprintf("familytree_gedcom_%s.ged", timestamp)
	filePath := filepath.Join(dir, fileName)
	accName := fmt.Sprintf("familytree_accounts_%s.ged", timestamp)
	accPath := filepath.Join(accDir, accName)

	if _, err := os.Stat(filePath); err == nil {
		if _, err2 := os.Stat(accPath); err2 == nil {
			return nil // 今天已生成
		}
	}

	db, err := s.openDB()
	if err != nil {
		return err
	}
	defer db.Close()

	text, err := gedcom.Generate(db)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filePath, []byte(text), 0o644); err != nil {
		return err
	}

	// 带账号密码版（含密码哈希，供恢复登录密码；文件仅理事长/超管可查看）
	if accText, accErr := gedcom.GenerateWithAccounts(db, true); accErr == nil {
		os.WriteFile(accPath, []byte(accText), 0o644)
	}

	// 清理：保留最近 30 天 GEDCOM 快照（文件名带日期，按字典序 = 时间序）
	s.cleanByPrefix(dir, "familytree_gedcom_", 30)
	s.cleanByPrefix(accDir, "familytree_accounts_", 30)
	log.Printf("[备份] GEDCOM 快照完成: %s + %s", fileName, accName)
	return nil
}

// fontsSnapshot 字体硬链接快照：每天一个目录，未变文件硬链接上一份快照（字体几乎不变 → 几乎不占额外磁盘）
// 字体是大体积二进制（本机实测 220MB），塞进 30 份日备份 zip 会变成数 GB 重复数据，故走快照去重。
// 目录结构：backup/fonts/YYYYMMDD/...（相对 fonts/ 的路径）
func (s *Scheduler) fontsSnapshot() error {
	fontsDir := filepath.Join(s.DataDir, "fonts")
	if _, err := os.Stat(fontsDir); os.IsNotExist(err) {
		return nil // 没有上传过字体，跳过
	}

	base := filepath.Join(s.DataDir, "backup", "fonts")
	os.MkdirAll(base, 0o755)
	timestamp := time.Now().In(s.loc).Format("20060102")
	dest := filepath.Join(base, timestamp)

	if _, err := os.Stat(dest); err == nil {
		return nil // 今天已生成
	}

	prev := s.latestSnapshot(base, timestamp)
	if err := snapshotDir(fontsDir, prev, dest); err != nil {
		return err
	}

	s.cleanSnapshotDirs(base, 30)
	log.Printf("[备份] 字体硬链接快照完成: %s", timestamp)
	return nil
}

// mediaSnapshot 多媒体硬链接快照：每天一个目录，未变文件硬链接上一份快照，新增/变更复制，已删除不出现
// 目录结构：backup/media/YYYYMMDD/...（相对 upload/ 的路径）
func (s *Scheduler) mediaSnapshot() error {
	uploadDir := filepath.Join(s.DataDir, "upload")
	if _, err := os.Stat(uploadDir); os.IsNotExist(err) {
		return nil // 无上传目录，跳过
	}

	base := filepath.Join(s.DataDir, "backup", "media")
	os.MkdirAll(base, 0o755)
	timestamp := time.Now().In(s.loc).Format("20060102")
	dest := filepath.Join(base, timestamp)

	if _, err := os.Stat(dest); err == nil {
		return nil // 今天已生成
	}

	// 找到最近一次快照作为硬链接来源
	prev := s.latestSnapshot(base, timestamp)
	if err := snapshotDir(uploadDir, prev, dest); err != nil {
		return err
	}

	// 清理：保留最近 30 天快照目录
	s.cleanSnapshotDirs(base, 30)
	log.Printf("[备份] 多媒体硬链接快照完成: %s", timestamp)
	return nil
}

// snapshotDir 生成 dest 快照目录（单遍遍历今天的 upload）：
// - 文件未变（与 prev 同内容）→ 硬链接 prev 中对应文件（共享 inode，不占额外磁盘）
// - 文件新增/变更 → 复制到 dest
// - 文件已从 upload 删除 → 不出现（快照真实反映当天状态）
func snapshotDir(uploadDir, prev, dest string) error {
	os.MkdirAll(dest, 0o755)
	err := filepath.Walk(uploadDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel(uploadDir, path)
		if relErr != nil {
			return relErr
		}
		target := filepath.Join(dest, rel)
		os.MkdirAll(filepath.Dir(target), 0o755)

		// 优先硬链接上一份快照中内容相同的文件
		if prev != "" {
			prevPath := filepath.Join(prev, rel)
			if sameFile(path, prevPath) {
				return os.Link(prevPath, target)
			}
		}
		return copyFile(path, target)
	})
	if err != nil {
		os.RemoveAll(dest) // 失败则清空半成品，下次重试
		return err
	}
	return nil
}

// openDB 打开主数据库（只读）
func (s *Scheduler) openDB() (*sql.DB, error) {
	dbPath := filepath.Join(s.DataDir, "data", "familytree.db")
	return sql.Open("sqlite3", dbPath+"?_busy_timeout=5000")
}

// latestSnapshot 返回 base 下除 exclude 外按名字序最大的快照目录
func (s *Scheduler) latestSnapshot(base, exclude string) string {
	entries, err := os.ReadDir(base)
	if err != nil {
		return ""
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() || e.Name() == exclude {
			continue
		}
		names = append(names, e.Name())
	}
	if len(names) == 0 {
		return ""
	}
	sort.Strings(names)
	return filepath.Join(base, names[len(names)-1])
}

// sameFile 判断两个路径是否指向同一 inode 或内容一致（大小 + 内容）
func sameFile(a, b string) bool {
	ai, err := os.Stat(a)
	if err != nil {
		return false
	}
	bi, err := os.Stat(b)
	if err != nil {
		return false
	}
	if os.SameFile(ai, bi) {
		return true
	}
	if ai.Size() != bi.Size() {
		return false
	}
	// 大小一致但 inode 不同 → 比较内容
	af, err := os.Open(a)
	if err != nil {
		return false
	}
	defer af.Close()
	bf, err := os.Open(b)
	if err != nil {
		return false
	}
	defer bf.Close()

	bufA := make([]byte, 64*1024)
	bufB := make([]byte, 64*1024)
	for {
		na, _ := af.Read(bufA)
		nb, _ := bf.Read(bufB)
		if na != nb || string(bufA[:na]) != string(bufB[:nb]) {
			return false
		}
		if na == 0 {
			return true
		}
	}
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

// cleanSnapshotDirs 保留最近 keep 个快照目录（按目录名字典序，名字即日期）
func (s *Scheduler) cleanSnapshotDirs(base string, keep int) {
	entries, err := os.ReadDir(base)
	if err != nil {
		return
	}
	var dirs []os.DirEntry
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, e)
		}
	}
	if len(dirs) <= keep {
		return
	}
	sort.Slice(dirs, func(i, j int) bool { return dirs[i].Name() < dirs[j].Name() })
	for i := 0; i < len(dirs)-keep; i++ {
		os.RemoveAll(filepath.Join(base, dirs[i].Name()))
		log.Printf("[备份] 清理旧多媒体快照: %s", dirs[i].Name())
	}
}

// cleanByPrefix 清理按前缀命名的文件，保留最近 keep 个（按名字序）
func (s *Scheduler) cleanByPrefix(dir, prefix string, keep int) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(e.Name(), prefix) {
			names = append(names, e.Name())
		}
	}
	if len(names) <= keep {
		return
	}
	sort.Strings(names)
	for i := 0; i < len(names)-keep; i++ {
		os.Remove(filepath.Join(dir, names[i]))
		log.Printf("[备份] 清理旧 GEDCOM 快照: %s", names[i])
	}
}

func (s *Scheduler) monthlyBackup() error {
	dir := filepath.Join(s.DataDir, "backup", "monthly")
	os.MkdirAll(dir, 0o755)
	timestamp := time.Now().In(s.loc).Format("200601")
	zipName := fmt.Sprintf("familytree_monthly_%s.zip", timestamp)
	zipPath := filepath.Join(dir, zipName)

	if _, err := os.Stat(zipPath); err == nil {
		return nil
	}
	if err := s.createFullZip(zipPath); err != nil {
		return err
	}
	log.Printf("[备份] 每月备份完成: %s", zipName)
	return nil
}

// createFullZip 打包 data + upload + fonts + thumbnails（月备份：每月一份完整可恢复点，含多媒体与字体）
func (s *Scheduler) createFullZip(zipPath string) error {
	s.checkpointWAL("每月备份")
	dirs := []string{
		filepath.Join(s.DataDir, "data"),
		filepath.Join(s.DataDir, "upload"),
		filepath.Join(s.DataDir, "fonts"),
		filepath.Join(s.DataDir, "thumbnails"),
	}
	man := s.buildManifest(kindMonthly, dirs, nil,
		"完整数据备份：含数据库全部表（传记/账本/成员/用户等）、上传附件、上传字体、缩略图。不包含程序文件与地图瓦片缓存。")
	return zipDirsExtra(dirs, map[string][]byte{manifestEntryName: man}, zipPath)
}

// zipDirsExtra 打包多个目录，并额外写入根级条目（如 backup-manifest.json）。
// 句柄用完即关；写失败删掉半成品包，避免留下能下载却打不开的假备份。
func zipDirsExtra(dirs []string, extra map[string][]byte, zipPath string) error {
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

	names := make([]string, 0, len(extra))
	for n := range extra {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
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
			continue // 目录不存在（例如没上传过字体）不算失败
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
			f.Close()
			return copyErr
		})
		if err != nil {
			return abort(err)
		}
	}
	if err := zw.Close(); err != nil {
		zf.Close()
		os.Remove(zipPath)
		return err
	}
	return zf.Close()
}

func (s *Scheduler) cleanOld(dir string, keep int) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	var zips []os.DirEntry
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".zip") {
			zips = append(zips, e)
		}
	}
	if len(zips) <= keep {
		return
	}
	sort.Slice(zips, func(i, j int) bool {
		ii, _ := zips[i].Info()
		ij, _ := zips[j].Info()
		return ii.ModTime().Before(ij.ModTime())
	})
	for i := 0; i < len(zips)-keep; i++ {
		os.Remove(filepath.Join(dir, zips[i].Name()))
		log.Printf("[备份] 清理旧备份: %s", zips[i].Name())
	}
}
