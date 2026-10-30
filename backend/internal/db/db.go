package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"familytree/internal/config"
	"familytree/internal/lunar"

	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/crypto/bcrypt"
)

// Init 初始化数据库连接并自动建表
func Init(cfg *config.Config) (*sql.DB, error) {
	// 确保目录存在
	dataDir := filepath.Join(cfg.DataDir, "data")
	uploadDir := filepath.Join(cfg.DataDir, "upload")
	backupDir := filepath.Join(cfg.DataDir, "backup")
	for _, d := range []string{dataDir, uploadDir, backupDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return nil, fmt.Errorf("创建目录 %s 失败: %w", d, err)
		}
	}

	dbPath := filepath.Join(dataDir, "familytree.db")
	database, err := sql.Open("sqlite3", dbPath+"?_busy_timeout=5000&_journal_mode=WAL")
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %w", err)
	}

	if err := migrate(database); err != nil {
		return nil, err
	}

	if err := seedAdmin(database, cfg); err != nil {
		return nil, err
	}

	return database, nil
}

// hashPassword 密码哈希（bcrypt，cost=12；与 handler 包登录逻辑一致）
func hashPassword(pw string) string {
	b, err := bcrypt.GenerateFromPassword([]byte(pw), 12)
	if err != nil {
		return ""
	}
	return string(b)
}

// seedAdmin 首次启动创建管理员账号
func seedAdmin(database *sql.DB, cfg *config.Config) error {
	var count int
	err := database.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	// 密码需哈希后入库（登录时与 hashPassword(输入) 比对）
	_, err = database.Exec(`INSERT INTO users (username, password, role) VALUES (?,?,?)`,
		cfg.AdminUser, hashPassword(cfg.AdminPass), "超级管理员")
	return err
}

// migrate 自动建表
func migrate(database *sql.DB) error {
	schema := `
CREATE TABLE IF NOT EXISTS members (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    generation INT NOT NULL DEFAULT 1,
    branch VARCHAR(64) DEFAULT '',
    branch_id INT DEFAULT 0,
    surname VARCHAR(16) DEFAULT '',
    given_name VARCHAR(64) DEFAULT '',
    name VARCHAR(64) NOT NULL,
    zi VARCHAR(64) DEFAULT '',
    hao VARCHAR(64) DEFAULT '',
    generation_word VARCHAR(16) DEFAULT '',
    rank VARCHAR(16) DEFAULT '',
    gender TINYINT NOT NULL DEFAULT 1,
    father_id INT DEFAULT 0,
    mother_id INT DEFAULT 0,
    mother_spouse_id INT DEFAULT 0,
    clan_type VARCHAR(16) DEFAULT '本生',
    record_type VARCHAR(8) DEFAULT '内纪',
    foster_father_id INT DEFAULT 0,
    birth_gregorian VARCHAR(16) DEFAULT '',
    birth_lunar VARCHAR(32) DEFAULT '',
    birth_ganzhi VARCHAR(16) DEFAULT '',
    birth_era VARCHAR(32) DEFAULT '',
    death_gregorian VARCHAR(16) DEFAULT '',
    death_lunar VARCHAR(32) DEFAULT '',
    death_ganzhi VARCHAR(16) DEFAULT '',
    death_era VARCHAR(32) DEFAULT '',
    lifespan INT DEFAULT 0,
    native_place VARCHAR(128) DEFAULT '',
    current_place VARCHAR(128) DEFAULT '',
    title VARCHAR(128) DEFAULT '',
    occupation VARCHAR(64) DEFAULT '',
    education VARCHAR(64) DEFAULT '',
    biography TEXT DEFAULT '',
    burial_place VARCHAR(128) DEFAULT '',
    portrait VARCHAR(256) DEFAULT '',
    married_out VARCHAR(256) DEFAULT '',
    former_name VARCHAR(256) DEFAULT '',
    phone VARCHAR(20) DEFAULT '',
    dna_haploid VARCHAR(128) DEFAULT '',
    dna_diploid VARCHAR(128) DEFAULT '',
    dna_mito VARCHAR(128) DEFAULT '',
    no_heir TINYINT DEFAULT 0,
    lost_contact TINYINT DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS spouses (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    member_id INT NOT NULL,
    surname VARCHAR(16) DEFAULT '',
    given_name VARCHAR(64) DEFAULT '',
    name VARCHAR(64) NOT NULL,
    type VARCHAR(16) DEFAULT '正配',
    family VARCHAR(128) DEFAULT '',
    birth_gregorian VARCHAR(16) DEFAULT '',
    birth_lunar VARCHAR(32) DEFAULT '',
    birth_ganzhi VARCHAR(16) DEFAULT '',
    birth_era VARCHAR(32) DEFAULT '',
    death_gregorian VARCHAR(16) DEFAULT '',
    death_lunar VARCHAR(32) DEFAULT '',
    death_ganzhi VARCHAR(16) DEFAULT '',
    death_era VARCHAR(32) DEFAULT '',
    occupation VARCHAR(64) DEFAULT '',
    education VARCHAR(64) DEFAULT '',
    biography TEXT DEFAULT '',
    remark VARCHAR(256) DEFAULT '',
    phone VARCHAR(20) DEFAULT '',
    dna_haploid VARCHAR(128) DEFAULT '',
    dna_diploid VARCHAR(128) DEFAULT '',
    dna_mito VARCHAR(128) DEFAULT ''
);

CREATE TABLE IF NOT EXISTS generation_words (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    branch VARCHAR(64) DEFAULT '',
    poem TEXT DEFAULT '',
    words_array TEXT DEFAULT '[]',
    start_generation INT DEFAULT 1
);

CREATE TABLE IF NOT EXISTS branches (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name VARCHAR(64) NOT NULL UNIQUE,
    sort_order INT DEFAULT 0,
    remark VARCHAR(256) DEFAULT '',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    year VARCHAR(16) DEFAULT '',
    title VARCHAR(128) NOT NULL,
    content TEXT DEFAULT '',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS articles (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    category VARCHAR(32) DEFAULT '谱序',
    title VARCHAR(128) NOT NULL,
    author VARCHAR(64) DEFAULT '',
    year VARCHAR(16) DEFAULT '',
    content TEXT DEFAULT '',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- 祠墓总表：祠堂与祖茔共用（旧版只有 name/location/description/photo 四个字段，
-- 仍是「谱头文化 → 祠墓」tab 的数据源，四字段语义不改）。
-- 祠宇志 / 茔墓志 在此表上扩展：kind 区分两类条目，各自的专属字段直接同表（宽表，与 members 同风格），
-- 只有真正 1:N 的多条内容（始建重修 / 楹联匾额 / 族产公田）才拆子表，避免建平行主表。
CREATE TABLE IF NOT EXISTS cemeteries (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name VARCHAR(128) NOT NULL,
    location VARCHAR(256) DEFAULT '',
    description TEXT DEFAULT '',
    photo VARCHAR(256) DEFAULT '',
    kind VARCHAR(8) DEFAULT '',
    shrine_type VARCHAR(16) DEFAULT '祠堂',
    lng REAL DEFAULT 0,
    lat REAL DEFAULT 0,
    architecture TEXT DEFAULT '',
    spirit_layout TEXT DEFAULT '',
    ritual TEXT DEFAULT '',
    photos TEXT DEFAULT '',
    plan_images TEXT DEFAULT '',
    sort_order INT DEFAULT 0,
    member_id INT DEFAULT NULL,
    owner_name VARCHAR(64) DEFAULT '',
    generation INT DEFAULT 0,
    birth VARCHAR(32) DEFAULT '',
    death VARCHAR(32) DEFAULT '',
    epitaph TEXT DEFAULT '',
    mountain_orientation VARCHAR(64) DEFAULT '',
    updated_at VARCHAR(32) DEFAULT '',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (member_id) REFERENCES members(id) ON DELETE SET NULL
);
-- ⚠️ 注意：cemeteries 的 kind 索引不能写在这里 —— 老库的 cemeteries 表还没有 kind 列，
-- 而本次 schema 是先执行、再跑 ALTER 补列；索引必须等 ALTER 之后建（见 migrate 末尾）。

-- 祠宇志子表①：始建与重修历史（年份 + 事件 + 说明），一座祠宇可多条
CREATE TABLE IF NOT EXISTS shrine_history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    cemetery_id INT NOT NULL,
    year VARCHAR(32) DEFAULT '',
    event VARCHAR(64) DEFAULT '',
    note TEXT DEFAULT '',
    sort_order INT DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_shrine_history_cemetery ON shrine_history (cemetery_id, sort_order, id);

-- 祠宇志子表②：楹联匾额（类型 + 位置 + 内容），可多条
-- kind：匾额 / 对联 / 其他（"其他"＝找不到对应分类时的兜底）；空 = 未分类（旧数据不猜）
-- position 仍是自由文本（大门额 / 大门联 / 神龛联 / 厅柱联 / 正厅横匾 …）
CREATE TABLE IF NOT EXISTS shrine_couplets (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    cemetery_id INT NOT NULL,
    kind VARCHAR(16) DEFAULT '',
    position VARCHAR(32) DEFAULT '',
    content TEXT DEFAULT '',
    sort_order INT DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_shrine_couplets_cemetery ON shrine_couplets (cemetery_id, sort_order, id);

-- 祠宇志子表③：族产公田（名称 + 面积 + 说明），可多条
CREATE TABLE IF NOT EXISTS shrine_properties (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    cemetery_id INT NOT NULL,
    name VARCHAR(128) DEFAULT '',
    area VARCHAR(64) DEFAULT '',
    note TEXT DEFAULT '',
    sort_order INT DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_shrine_properties_cemetery ON shrine_properties (cemetery_id, sort_order, id);

CREATE TABLE IF NOT EXISTS biographies (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    person_type VARCHAR(16) DEFAULT 'member',
    member_id INT DEFAULT 0,
    spouse_id INT DEFAULT 0,
    name VARCHAR(64) DEFAULT '',
    zi VARCHAR(32) DEFAULT '',
    hao VARCHAR(32) DEFAULT '',
    gender VARCHAR(8) DEFAULT '',
    birth_gregorian VARCHAR(32) DEFAULT '',
    birth_lunar VARCHAR(64) DEFAULT '',
    birth_era VARCHAR(64) DEFAULT '',
    death_gregorian VARCHAR(32) DEFAULT '',
    death_lunar VARCHAR(64) DEFAULT '',
    death_era VARCHAR(64) DEFAULT '',
    native_place VARCHAR(128) DEFAULT '',
    current_place VARCHAR(128) DEFAULT '',
    title VARCHAR(64) DEFAULT '',
    occupation VARCHAR(64) DEFAULT '',
    honors VARCHAR(256) DEFAULT '',
    portrait VARCHAR(512) DEFAULT '',
    summary TEXT DEFAULT '',
    content TEXT DEFAULT '',
    reason VARCHAR(256) DEFAULT '',
    sort_year INT DEFAULT 0,
    sort_order INT DEFAULT 0,
    public INT DEFAULT 1,
    created_by VARCHAR(64) DEFAULT '',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_bio_member ON biographies (member_id);

CREATE INDEX IF NOT EXISTS idx_bio_spouse ON biographies (spouse_id);

CREATE TABLE IF NOT EXISTS users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username VARCHAR(64) NOT NULL UNIQUE,
    password VARCHAR(128) NOT NULL,
    role VARCHAR(32) DEFAULT '族员',
    branch VARCHAR(64) DEFAULT '',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS operation_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INT DEFAULT 0,
    action VARCHAR(64) DEFAULT '',
    target VARCHAR(64) DEFAULT '',
    detail TEXT DEFAULT '',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS attachments (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    member_id INT DEFAULT 0,
    category VARCHAR(32) DEFAULT '照片',
    path VARCHAR(256) DEFAULT '',
    description VARCHAR(256) DEFAULT '',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS media_assets (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    path VARCHAR(256) DEFAULT '',
    type VARCHAR(16) DEFAULT 'image',
    category VARCHAR(32) DEFAULT '其他',
    title VARCHAR(128) DEFAULT '',
    description VARCHAR(512) DEFAULT '',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS settings (
    key VARCHAR(64) PRIMARY KEY,
    value TEXT DEFAULT ''
);

CREATE TABLE IF NOT EXISTS member_bio_entries (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    member_id INT NOT NULL,
    title VARCHAR(64) DEFAULT '',
    content TEXT DEFAULT '',
    sort_order INT DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS spouse_bio_entries (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    spouse_id INT NOT NULL,
    title VARCHAR(64) DEFAULT '',
    content TEXT DEFAULT '',
    sort_order INT DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- 资料来源：一个成员可有多条（族谱旧谱／口述／墓碑／家书／方志／其他，kind 可自定义），
-- 用于谱书行传页的「据：…」小字脚注。用独立子表而不是 members 上加一列 source_note：
-- 单列存不下多条、也分不出类型；整库备份（zip 内的 familytree.db）天然带上此表。
CREATE TABLE IF NOT EXISTS member_sources (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    member_id INT NOT NULL,
    "text" VARCHAR(500) DEFAULT '',
    kind VARCHAR(32) DEFAULT '其他',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_member_sources_member ON member_sources (member_id, id);

CREATE TABLE IF NOT EXISTS entity_versions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    entity_type VARCHAR(16) NOT NULL DEFAULT 'member',
    entity_id INT NOT NULL,
    version_no INT NOT NULL DEFAULT 1,
    snapshot TEXT DEFAULT '',
    changed_by INT DEFAULT 0,
    changed_by_name VARCHAR(64) DEFAULT '',
    note VARCHAR(256) DEFAULT '',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_entity_versions ON entity_versions (entity_type, entity_id, version_no);

CREATE TABLE IF NOT EXISTS edit_drafts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    entity_type VARCHAR(16) NOT NULL DEFAULT 'member',
    action VARCHAR(16) NOT NULL DEFAULT 'create',
    entity_id INT DEFAULT 0,
    payload TEXT DEFAULT '',
    changes TEXT DEFAULT '',
    submitted_by INT DEFAULT 0,
    submitted_by_name VARCHAR(64) DEFAULT '',
    status VARCHAR(16) NOT NULL DEFAULT 'pending',
    reviewer_by INT DEFAULT 0,
    reviewer_name VARCHAR(64) DEFAULT '',
    reason VARCHAR(512) DEFAULT '',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    reviewed_at DATETIME
);
CREATE INDEX IF NOT EXISTS idx_edit_drafts_status ON edit_drafts (status, id);

CREATE TABLE IF NOT EXISTS incense_offers (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INT DEFAULT 0,
    username VARCHAR(64) DEFAULT '',
    count INT DEFAULT 3,
    offered_at INTEGER DEFAULT 0,
    expires_at INTEGER DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_incense_offers_expires ON incense_offers (expires_at);

CREATE TABLE IF NOT EXISTS traditions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    title VARCHAR(128) NOT NULL,
    lunar_month INT DEFAULT 1,
    lunar_day INT DEFAULT 1,
    is_leap_month INT DEFAULT 0,
    description TEXT DEFAULT '',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS migrations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    from_place VARCHAR(128) DEFAULT '',
    to_place VARCHAR(128) DEFAULT '',
    era VARCHAR(64) DEFAULT '',
    year INT DEFAULT 0,
    branch VARCHAR(64) DEFAULT '',
    member_ids TEXT DEFAULT '',
    member_count INT DEFAULT 0,
    description TEXT DEFAULT '',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS account_books (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    title VARCHAR(128) NOT NULL DEFAULT '',
    year INT DEFAULT 0,
    project VARCHAR(128) DEFAULT '',
    sort_order INT DEFAULT 0,
    is_pinned INT DEFAULT 0,
    book_type VARCHAR(16) DEFAULT 'project',
    description TEXT DEFAULT '',
    status VARCHAR(16) DEFAULT 'open',
    is_public INT DEFAULT 1,
    initial_balance REAL DEFAULT 0,
    merit_title VARCHAR(160) DEFAULT '',
    merit_threshold REAL DEFAULT 0,
    merit_numeral VARCHAR(16) DEFAULT 'arabic',
    merit_show_amount INT DEFAULT 1,
    merit_footer VARCHAR(255) DEFAULT '',
    created_by INT DEFAULT 0,
    created_by_name VARCHAR(64) DEFAULT '',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS account_entries (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    book_id INT NOT NULL,
    kind VARCHAR(8) NOT NULL DEFAULT 'in',
    amount REAL DEFAULT 0,
    category VARCHAR(64) DEFAULT '',
    entry_date VARCHAR(32) DEFAULT '',
    summary VARCHAR(512) DEFAULT '',
    handler VARCHAR(64) DEFAULT '',
    voucher VARCHAR(1024) DEFAULT '',
    member_id INT DEFAULT 0,
    donor_public INT DEFAULT 1,
    status VARCHAR(16) DEFAULT 'pending',
    submitted_by INT DEFAULT 0,
    submitted_by_name VARCHAR(64) DEFAULT '',
    reviewer_by INT DEFAULT 0,
    reviewer_name VARCHAR(64) DEFAULT '',
    reason VARCHAR(512) DEFAULT '',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    reviewed_at DATETIME
);
CREATE INDEX IF NOT EXISTS idx_account_entries_book ON account_entries (book_id, status);
CREATE INDEX IF NOT EXISTS idx_account_entries_pending ON account_entries (status, id);

-- 功德碑：可存在多个，各自独立配置（碑额/门槛/每列人数/数字写法/样式/金额/落款）
CREATE TABLE IF NOT EXISTS merit_steles (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name VARCHAR(128) DEFAULT '',
    book_id INT DEFAULT 0,
    title VARCHAR(160) DEFAULT '',
    threshold REAL DEFAULT 0,
    per_column INT DEFAULT 12,
    numeral VARCHAR(16) DEFAULT 'arabic',
    style VARCHAR(16) DEFAULT 'stone',
    border VARCHAR(16) DEFAULT 'none',
    show_amount INT DEFAULT 1,
    footer VARCHAR(255) DEFAULT '',
    created_by INT DEFAULT 0,
    created_by_name VARCHAR(64) DEFAULT '',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS geocode_cache (
    place TEXT PRIMARY KEY,
    lng REAL DEFAULT 0,
    lat REAL DEFAULT 0,
    provider VARCHAR(16) DEFAULT '',
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

INSERT OR IGNORE INTO settings (key, value) VALUES ('family_name', '');
INSERT OR IGNORE INTO settings (key, value) VALUES ('family_surname', '');
INSERT OR IGNORE INTO settings (key, value) VALUES ('tanghao', '');
INSERT OR IGNORE INTO settings (key, value) VALUES ('junwang', '');
INSERT OR IGNORE INTO settings (key, value) VALUES ('zuji', '');
INSERT OR IGNORE INTO settings (key, value) VALUES ('shizu_name', '');
INSERT OR IGNORE INTO settings (key, value) VALUES ('shizu_bio', '');
INSERT OR IGNORE INTO settings (key, value) VALUES ('shiqianzu_name', '');
INSERT OR IGNORE INTO settings (key, value) VALUES ('shiqianzu_place', '');
INSERT OR IGNORE INTO settings (key, value) VALUES ('ai_assistant_enabled', '0');
INSERT OR IGNORE INTO settings (key, value) VALUES ('ai_provider', 'local');
INSERT OR IGNORE INTO settings (key, value) VALUES ('ai_base_url', '');
INSERT OR IGNORE INTO settings (key, value) VALUES ('ai_model', '');
INSERT OR IGNORE INTO settings (key, value) VALUES ('ai_api_key', '');
INSERT OR IGNORE INTO settings (key, value) VALUES ('ai_max_tokens', '2048');
INSERT OR IGNORE INTO settings (key, value) VALUES ('ai_timeout_sec', '180');
INSERT OR IGNORE INTO settings (key, value) VALUES ('ai_path', '');
INSERT OR IGNORE INTO settings (key, value) VALUES ('biography_criteria', '');
INSERT OR REPLACE INTO settings (key, value) VALUES ('version', '1.3.9');
INSERT OR IGNORE INTO settings (key, value) VALUES ('deployed_at', CAST(strftime('%s','now') AS TEXT));
INSERT OR IGNORE INTO settings (key, value) VALUES ('account_enabled', '0');
INSERT OR IGNORE INTO settings (key, value) VALUES ('account_submitters', '[]');
INSERT OR IGNORE INTO settings (key, value) VALUES ('account_fund_base', '0');
-- db_instance_id 由应用层首次启动时用 crypto/rand 生成真 UUID，此处不预置
`
	_, err := database.Exec(schema)
	if err != nil {
		return err
	}

	// 安全迁移：给旧表添加新列（忽略已存在的列）
	alterStatements := []string{
		`ALTER TABLE members ADD COLUMN surname VARCHAR(16) DEFAULT ''`,
		`ALTER TABLE members ADD COLUMN given_name VARCHAR(64) DEFAULT ''`,
		`ALTER TABLE members ADD COLUMN birth_era VARCHAR(32) DEFAULT ''`,
		`ALTER TABLE members ADD COLUMN death_era VARCHAR(32) DEFAULT ''`,
		`ALTER TABLE spouses ADD COLUMN surname VARCHAR(16) DEFAULT ''`,
		`ALTER TABLE spouses ADD COLUMN given_name VARCHAR(64) DEFAULT ''`,
		`ALTER TABLE spouses ADD COLUMN birth_era VARCHAR(32) DEFAULT ''`,
		`ALTER TABLE spouses ADD COLUMN death_era VARCHAR(32) DEFAULT ''`,
		`ALTER TABLE spouses ADD COLUMN remark VARCHAR(256) DEFAULT ''`,
		`ALTER TABLE members ADD COLUMN mother_spouse_id INT DEFAULT 0`,
		`ALTER TABLE spouses ADD COLUMN portrait VARCHAR(512) DEFAULT ''`,
		`ALTER TABLE attachments ADD COLUMN spouse_id INT DEFAULT 0`,
		`ALTER TABLE members ADD COLUMN birth_calendar VARCHAR(8) DEFAULT 'solar'`,
		`ALTER TABLE spouses ADD COLUMN birth_calendar VARCHAR(8) DEFAULT 'solar'`,
		`ALTER TABLE members ADD COLUMN former_name VARCHAR(256) DEFAULT ''`,
		`ALTER TABLE members ADD COLUMN branch_id INT DEFAULT 0`,
		`ALTER TABLE members ADD COLUMN occupation VARCHAR(64) DEFAULT ''`,
		`ALTER TABLE members ADD COLUMN education VARCHAR(64) DEFAULT ''`,
		`ALTER TABLE spouses ADD COLUMN occupation VARCHAR(64) DEFAULT ''`,
		`ALTER TABLE spouses ADD COLUMN education VARCHAR(64) DEFAULT ''`,
		`ALTER TABLE spouses ADD COLUMN biography TEXT DEFAULT ''`,
		`ALTER TABLE spouses ADD COLUMN phone VARCHAR(20) DEFAULT ''`,
		`ALTER TABLE spouses ADD COLUMN dna_haploid VARCHAR(128) DEFAULT ''`,
		`ALTER TABLE spouses ADD COLUMN dna_diploid VARCHAR(128) DEFAULT ''`,
		`ALTER TABLE spouses ADD COLUMN dna_mito VARCHAR(128) DEFAULT ''`,
		`ALTER TABLE edit_drafts ADD COLUMN changes TEXT DEFAULT ''`,
		`ALTER TABLE members ADD COLUMN record_type VARCHAR(8) DEFAULT '内纪'`,
		`ALTER TABLE members ADD COLUMN current_place VARCHAR(128) DEFAULT ''`,
		`ALTER TABLE members ADD COLUMN phone VARCHAR(20) DEFAULT ''`,
		`ALTER TABLE members ADD COLUMN dna_code VARCHAR(128) DEFAULT ''`,
		`ALTER TABLE members ADD COLUMN dna_type VARCHAR(8) DEFAULT ''`,
		`ALTER TABLE members ADD COLUMN dna_haploid VARCHAR(128) DEFAULT ''`,
		`ALTER TABLE members ADD COLUMN dna_diploid VARCHAR(128) DEFAULT ''`,
		`ALTER TABLE members ADD COLUMN dna_mito VARCHAR(128) DEFAULT ''`,
		`ALTER TABLE operation_logs ADD COLUMN ip VARCHAR(64) DEFAULT ''`,
		`ALTER TABLE account_books ADD COLUMN is_pinned INT DEFAULT 0`,
		`ALTER TABLE account_books ADD COLUMN book_type VARCHAR(16) DEFAULT 'project'`,
		`ALTER TABLE account_books ADD COLUMN merit_title VARCHAR(160) DEFAULT ''`,
		`ALTER TABLE account_books ADD COLUMN merit_threshold REAL DEFAULT 0`,
		`ALTER TABLE account_books ADD COLUMN merit_numeral VARCHAR(16) DEFAULT 'arabic'`,
		`ALTER TABLE account_books ADD COLUMN merit_show_amount INT DEFAULT 1`,
		`ALTER TABLE account_books ADD COLUMN merit_footer VARCHAR(255) DEFAULT ''`,
		`ALTER TABLE members ADD COLUMN no_heir TINYINT DEFAULT 0`,
		// 多媒体素材归属：0＝总谱；>0＝某房支 id（旧记录无此列 → 迁移后默认 0＝总谱，仍可见）
		`ALTER TABLE media_assets ADD COLUMN branch_id INT DEFAULT 0`,
		`ALTER TABLE members ADD COLUMN lost_contact TINYINT DEFAULT 0`,
		`ALTER TABLE merit_steles ADD COLUMN border VARCHAR(16) DEFAULT 'none'`,
		// 祠宇志 / 茔墓志：在既有 cemeteries（「祠堂与祖茔」共用表）上扩展，
		// 不建平行主表；旧行 kind 留空（未分类，不猜），CultureView「祠墓」tab 仍照旧列出。
		// 注意 SQLite 限制：ADD COLUMN 的默认值必须是常量 → updated_at 用 VARCHAR 由 handler 写入。
		`ALTER TABLE cemeteries ADD COLUMN kind VARCHAR(8) DEFAULT ''`,
		`ALTER TABLE cemeteries ADD COLUMN shrine_type VARCHAR(16) DEFAULT '祠堂'`,
		`ALTER TABLE cemeteries ADD COLUMN lng REAL DEFAULT 0`,
		`ALTER TABLE cemeteries ADD COLUMN lat REAL DEFAULT 0`,
		`ALTER TABLE cemeteries ADD COLUMN architecture TEXT DEFAULT ''`,
		`ALTER TABLE cemeteries ADD COLUMN spirit_layout TEXT DEFAULT ''`,
		`ALTER TABLE cemeteries ADD COLUMN ritual TEXT DEFAULT ''`,
		`ALTER TABLE cemeteries ADD COLUMN photos TEXT DEFAULT ''`,
		`ALTER TABLE cemeteries ADD COLUMN plan_images TEXT DEFAULT ''`,
		`ALTER TABLE cemeteries ADD COLUMN sort_order INT DEFAULT 0`,
		`ALTER TABLE cemeteries ADD COLUMN member_id INT DEFAULT NULL`,
		`ALTER TABLE cemeteries ADD COLUMN owner_name VARCHAR(64) DEFAULT ''`,
		`ALTER TABLE cemeteries ADD COLUMN generation INT DEFAULT 0`,
		`ALTER TABLE cemeteries ADD COLUMN birth VARCHAR(32) DEFAULT ''`,
		`ALTER TABLE cemeteries ADD COLUMN death VARCHAR(32) DEFAULT ''`,
		`ALTER TABLE cemeteries ADD COLUMN epitaph TEXT DEFAULT ''`,
		`ALTER TABLE cemeteries ADD COLUMN mountain_orientation VARCHAR(64) DEFAULT ''`,
		`ALTER TABLE cemeteries ADD COLUMN updated_at VARCHAR(32) DEFAULT ''`,
		// 楹联匾额加「类型」：匾额 / 对联 / 其他（老数据留空＝未分类，不猜）
		`ALTER TABLE shrine_couplets ADD COLUMN kind VARCHAR(16) DEFAULT ''`,
	}
	for _, stmt := range alterStatements {
		database.Exec(stmt) // 忽略错误（列已存在等情况）
	}

	// 祠宇志/茔墓志图片迁移（幂等）：旧「祠墓」只有单张 photo，把它回填成 photos JSON 数组，
	// 老数据在祠宇志/茔墓志里也能看到那张图；photo 列本身保留不动（CultureView 仍按单图渲染）。
	database.Exec(`UPDATE cemeteries SET photos = '["' || photo || '"]'
		WHERE COALESCE(photos,'') = '' AND COALESCE(photo,'') <> ''`)

	// kind 索引：必须在上面 ALTER 补完 kind 列之后建（老库执行 schema 时还没有这一列）
	database.Exec(`CREATE INDEX IF NOT EXISTS idx_cemeteries_kind ON cemeteries (kind, sort_order)`)

	// 功德碑「多碑化」迁移：首次创建 merit_steles 时，把旧的「账本内嵌功德碑配置」迁出为独立记录
	var steleCount int
	if database.QueryRow(`SELECT COUNT(*) FROM merit_steles`).Scan(&steleCount) == nil && steleCount == 0 {
		if rows, err := database.Query(`SELECT id, COALESCE(title,''), COALESCE(merit_title,''), COALESCE(merit_threshold,0),
			COALESCE(merit_numeral,'arabic'), COALESCE(merit_show_amount,1), COALESCE(merit_footer,'')
			FROM account_books WHERE book_type = 'donation' ORDER BY id ASC`); err == nil {
			type mrec struct {
				id    int64
				title string
				mt    string
				mr    float64
				mn    string
				ms    int
				mf    string
			}
			var recs []mrec
			for rows.Next() {
				var r mrec
				if rows.Scan(&r.id, &r.title, &r.mt, &r.mr, &r.mn, &r.ms, &r.mf) == nil {
					recs = append(recs, r)
				}
			}
			rows.Close()
			for _, r := range recs {
				name := r.title
				if name == "" {
					name = "功德碑"
				}
				database.Exec(`INSERT INTO merit_steles (name, book_id, title, threshold, per_column, numeral, style, show_amount, footer)
					VALUES (?,?,?,?,?,?,?,?,?)`, name, r.id, r.mt, r.mr, 12, r.mn, "stone", r.ms, r.mf)
			}
		}
	}

	// 功德碑「框/字拆分」迁移：旧耦合 style（含纹样）拆为 style(配色)+border(纹样)
	database.Exec(`UPDATE merit_steles SET border='huiwen', style='stone' WHERE style='huiwen'`)
	database.Exec(`UPDATE merit_steles SET border='wan', style='stone' WHERE style='wan'`)
	database.Exec(`UPDATE merit_steles SET border='yun', style='stone' WHERE style='cloud'`)
	database.Exec(`UPDATE merit_steles SET border='none', style='gold' WHERE style='dragon'`)

	// 功德碑「边框款系细分」迁移：旧纹样名 → 新款系键（回纹细分/万字细分/祥云）
	database.Exec(`UPDATE merit_steles SET border='hui2' WHERE border='huiwen'`)
	database.Exec(`UPDATE merit_steles SET border='wan1' WHERE border='wan'`)

	// 功德碑「边框精简」迁移：仅保留 回纹·单线(hui1)/回纹·带框线(hui2)，其余款系归并
	database.Exec(`UPDATE merit_steles SET border='hui2' WHERE border IN ('hui3','yun')`)
	database.Exec(`UPDATE merit_steles SET border='hui1' WHERE border IN ('wan1','wan2','wan3','wan4','wan5','wan6','wan7','wan8')`)
	database.Exec(`UPDATE merit_steles SET border='none' WHERE border NOT IN ('none','hui1','hui2')`)

	// 房支层级化迁移：branches 增加 上级/路径/层级/名讳，并把「名称全局唯一」改为「同一上级下唯一」
	brCols := map[string]bool{}
	if rs, err := database.Query(`PRAGMA table_info(branches)`); err == nil {
		for rs.Next() {
			var cid, notnull, pk int
			var name, ctype string
			var dflt sql.NullString
			if rs.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk) == nil {
				brCols[name] = true
			}
		}
		rs.Close()
	}
	if !brCols["parent_id"] {
		// 旧表 name 是全局唯一约束，SQLite 无法直接去掉 → 重建表
		database.Exec(`CREATE TABLE branches_new (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name VARCHAR(64) NOT NULL,
			subtitle VARCHAR(64) DEFAULT '',
			parent_id INTEGER DEFAULT 0,
			path VARCHAR(255) DEFAULT '',
			level INTEGER DEFAULT 1,
			sort_order INT DEFAULT 0,
			remark VARCHAR(256) DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP)`)
		database.Exec(`INSERT INTO branches_new (id, name, parent_id, path, level, sort_order, remark, created_at)
			SELECT id, name, 0, CAST(id AS TEXT), 1, sort_order, remark, created_at FROM branches`)
		database.Exec(`DROP TABLE branches`)
		database.Exec(`ALTER TABLE branches_new RENAME TO branches`)
	}
	// 保底列（兼容中间版本/异常情况）
	database.Exec(`ALTER TABLE branches ADD COLUMN subtitle VARCHAR(64) DEFAULT ''`)
	database.Exec(`ALTER TABLE branches ADD COLUMN parent_id INTEGER DEFAULT 0`)
	database.Exec(`ALTER TABLE branches ADD COLUMN path VARCHAR(255) DEFAULT ''`)
	database.Exec(`ALTER TABLE branches ADD COLUMN level INTEGER DEFAULT 1`)
	database.Exec(`UPDATE branches SET path = CAST(id AS TEXT) WHERE path IS NULL OR path = ''`)
	database.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS ux_branches_parent_name ON branches(parent_id, name)`)

	// 房支数据迁移：从旧 members.branch 文本去重生成 branches，并回填 branch_id
	database.Exec(`INSERT OR IGNORE INTO branches (name, sort_order)
		SELECT DISTINCT TRIM(branch), 0 FROM members WHERE branch IS NOT NULL AND TRIM(branch) <> ''`)
	database.Exec(`UPDATE members SET branch_id = COALESCE((SELECT b.id FROM branches b WHERE b.name = TRIM(members.branch)), 0)`)
	// 归一化：把已归属房支的成员 branch 文本对齐为房支规范名（去掉旧数据的首尾空格）
	database.Exec(`UPDATE members SET branch = (SELECT b.name FROM branches b WHERE b.id = members.branch_id) WHERE branch_id > 0`)

	// 生平条目迁移：把旧 members.biography 文本一次性转为首条「生平」条目（仅当该成员尚无条目时）
	database.Exec(`INSERT INTO member_bio_entries (member_id, title, content, sort_order)
		SELECT id, '生平', biography, 0 FROM members
		WHERE COALESCE(biography,'') <> ''
		  AND NOT EXISTS (SELECT 1 FROM member_bio_entries e WHERE e.member_id = members.id)`)

	// 配偶生平条目迁移：把旧 spouses.biography 文本一次性转为首条「生平」条目（仅当该配偶尚无条目时）
	database.Exec(`INSERT INTO spouse_bio_entries (spouse_id, title, content, sort_order)
		SELECT id, '生平', biography, 0 FROM spouses
		WHERE COALESCE(biography,'') <> ''
		  AND NOT EXISTS (SELECT 1 FROM spouse_bio_entries e WHERE e.spouse_id = spouses.id)`)

	// 历史数据补齐农历：只有公历（或只有年份）的先辈，按农历补生/殁的农历、干支、年号
	backfillLunarInfo(database)

	return nil
}

// lunarFromGregorian 公历字符串（YYYY-MM-DD 或 YYYY）→ 农历/干支/年号；无法解析返回空
func lunarFromGregorian(g string) (string, string, string) {
	g = strings.TrimSpace(g)
	if g == "" {
		return "", "", ""
	}
	if t, err := time.Parse("2006-01-02", g); err == nil {
		info := lunar.FromGregorian(t.Year(), int(t.Month()), t.Day())
		return info.Lunar, info.Ganzhi, info.Era + info.EraYear
	}
	if y, err := strconv.Atoi(g); err == nil {
		info := lunar.FromGregorianYear(y)
		return "", "", info.Era + info.EraYear
	}
	return "", "", ""
}

// backfillLunarInfo 历史数据补齐：生/殁只填了公历、农历/干支/年号全空时，按农历推算回写。
// 幂等：已有农历数据的一律不动；只补全空。
func backfillLunarInfo(database *sql.DB) {
	for _, tbl := range []string{"members", "spouses"} {
		rows, err := database.Query(`SELECT id, COALESCE(birth_gregorian,''), COALESCE(death_gregorian,''),
			COALESCE(birth_lunar,''), COALESCE(birth_ganzhi,''), COALESCE(birth_era,''),
			COALESCE(death_lunar,''), COALESCE(death_ganzhi,''), COALESCE(death_era,'') FROM ` + tbl)
		if err != nil {
			continue
		}
		type rec struct {
			id                       int64
			bl, bgz, be, dl, dgz, de string
			changed                  bool
		}
		var ups []rec
		for rows.Next() {
			var id int64
			var bg, dg string
			var r rec
			if err := rows.Scan(&id, &bg, &dg, &r.bl, &r.bgz, &r.be, &r.dl, &r.dgz, &r.de); err != nil {
				continue
			}
			r.id = id
			if r.bl == "" && r.bgz == "" && r.be == "" {
				if l, gz, e := lunarFromGregorian(bg); l != "" || e != "" {
					r.bl, r.bgz, r.be, r.changed = l, gz, e, true
				}
			}
			if r.dl == "" && r.dgz == "" && r.de == "" {
				if l, gz, e := lunarFromGregorian(dg); l != "" || e != "" {
					r.dl, r.dgz, r.de, r.changed = l, gz, e, true
				}
			}
			if r.changed {
				ups = append(ups, r)
			}
		}
		rows.Close()
		for _, r := range ups {
			database.Exec(`UPDATE `+tbl+` SET birth_lunar=?, birth_ganzhi=?, birth_era=?, death_lunar=?, death_ganzhi=?, death_era=? WHERE id=?`,
				r.bl, r.bgz, r.be, r.dl, r.dgz, r.de, r.id)
		}
	}
}
