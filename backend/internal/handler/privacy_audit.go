package handler

import (
	"database/sql"
	"strconv"
)

// LogPrivacyAccess 记录敏感信息查看（详情页 as_viewer=1 模式下）
// username: 访问者的登录用户名
// viewerMemberID: 访问者关联的族谱成员 ID（0=未关联族谱成员, -1=admin）
// targetID: 被查看的成员 id
// distance: 亲属距离
func LogPrivacyAccess(database *sql.DB, username string, viewerMemberID, targetID int64, distance int) {
	// 不记录本人查看自己
	if distance == 0 {
		return
	}
	// 不记录未关联族谱成员的 admin 查看
	if viewerMemberID <= 0 {
		return
	}

	level := "公开"
	switch {
	case distance == 1:
		level = "直系全量"
	case distance >= 2 && distance <= 3:
		level = "近亲脱敏"
	case distance > 3 || distance == -1:
		level = "远亲受限"
	}

	targetName := "成员#" + strconv.FormatInt(targetID, 10)
	var n string
	if err := database.QueryRow(`SELECT COALESCE(name,'') FROM members WHERE id = ?`, targetID).Scan(&n); err == nil && n != "" {
		targetName = n
	}

	detail := username + "查看了「" + targetName + "」→ " + level
	writeLog(database, 0, "查看隐私", "成员详情", detail, username, "")
}

// LogPrivacyAccessBulk 记录列表脱敏查询（批量查看时记录概要）
func LogPrivacyAccessBulk(database *sql.DB, memberCount int, username string) {
	if username == "admin" || username == "" {
		return
	}
	writeLog(database, 0, "查询成员",
		"成员列表",
		username+"查询了成员列表（"+strconv.Itoa(memberCount)+"人，已按权限脱敏）",
		username, "")
}
