package handler

import (
	"database/sql"
	"strings"

	"github.com/gin-gonic/gin"
)

// 房支写权限隔离：理事（RoleElder）只允许操作自己所属房支的成员/配偶。
// 理事长、超级管理员不受限制；理事未设置房支（users.branch 为空）时视为不限制（向后兼容）。

// elderBranchID 当前登录理事的房支 id。
// 非理事、或理事未设置房支、或房支名无法匹配时返回 0（0 表示不限制）。
func elderBranchID(c *gin.Context, db *sql.DB) int64 {
	if !isElder(c) {
		return 0
	}
	branchName := strings.TrimSpace(c.GetString("branch"))
	if branchName == "" {
		return 0
	}
	var id int64
	if err := db.QueryRow(`SELECT id FROM branches WHERE name = ?`, branchName).Scan(&id); err != nil {
		return 0
	}
	return id
}

// checkMemberBranchAccess 理事能否操作指定成员（按成员当前 branch_id 判断）。
func checkMemberBranchAccess(c *gin.Context, db *sql.DB, memberID int64) bool {
	bid := elderBranchID(c, db)
	if bid == 0 {
		return true
	}
	var mbID int64
	if err := db.QueryRow(`SELECT COALESCE(branch_id,0) FROM members WHERE id = ?`, memberID).Scan(&mbID); err != nil {
		return false
	}
	return mbID == bid
}

// checkCreateBranchAccess 理事新增成员时校验目标房支：仅允许写入自己房支或「未分配」(0)。
func checkCreateBranchAccess(c *gin.Context, db *sql.DB, branchID int64) bool {
	bid := elderBranchID(c, db)
	if bid == 0 {
		return true
	}
	return branchID == 0 || branchID == bid
}

// spouseMemberID 配偶所属成员 id（查不到返回 0）。
func spouseMemberID(db *sql.DB, spouseID int64) int64 {
	var mid int64
	if err := db.QueryRow(`SELECT member_id FROM spouses WHERE id = ?`, spouseID).Scan(&mid); err != nil {
		return 0
	}
	return mid
}

// bioEntryMemberID 成员生平条目所属成员 id（查不到返回 0）。
func bioEntryMemberID(db *sql.DB, entryID int64) int64 {
	var mid int64
	if err := db.QueryRow(`SELECT member_id FROM member_bio_entries WHERE id = ?`, entryID).Scan(&mid); err != nil {
		return 0
	}
	return mid
}

// spouseBioMemberID 配偶生平条目所属成员 id（经配偶 → 成员，查不到返回 0）。
func spouseBioMemberID(db *sql.DB, entryID int64) int64 {
	var mid int64
	if err := db.QueryRow(`SELECT s.member_id FROM spouse_bio_entries e JOIN spouses s ON s.id = e.spouse_id WHERE e.id = ?`, entryID).Scan(&mid); err != nil {
		return 0
	}
	return mid
}
