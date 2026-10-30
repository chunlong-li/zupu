package handler

import (
	"database/sql"
	"strconv"

	"github.com/gin-gonic/gin"
)

// BranchScope 解析请求中的 ?branch_id=&include_children= 房支过滤。
// 返回 SQL 条件片段（如 "m.branch_id IN (...)"）与参数；无过滤时返回空串。
// col 为成员表房支字段（如 "branch_id" 或 "m.branch_id"）。
func BranchScope(c *gin.Context, db *sql.DB, col string) (string, []interface{}) {
	branchID, _ := strconv.ParseInt(c.Query("branch_id"), 10, 64)
	if branchID <= 0 {
		return "", nil
	}
	inc := c.Query("include_children") == "1" || c.Query("include_children") == "true"
	if inc {
		var p string
		if err := db.QueryRow(`SELECT COALESCE(path,'') FROM branches WHERE id=?`, branchID).Scan(&p); err == nil && p != "" {
			return col + ` IN (SELECT id FROM branches WHERE path = ? OR path LIKE ?)`, []interface{}{p, p + ".%"}
		}
	}
	return col + ` = ?`, []interface{}{branchID}
}

// BranchMemberIDs 返回某房支（含下级）的成员 id 列表；无过滤时返回 nil。
// 用于配偶/热力等以成员为外键的关联查询。
func BranchMemberIDs(c *gin.Context, db *sql.DB) []interface{} {
	branchID, _ := strconv.ParseInt(c.Query("branch_id"), 10, 64)
	if branchID <= 0 {
		return nil
	}
	inc := c.Query("include_children") == "1" || c.Query("include_children") == "true"
	q := `SELECT id FROM members WHERE branch_id = ?`
	args := []interface{}{branchID}
	if inc {
		var p string
		if err := db.QueryRow(`SELECT COALESCE(path,'') FROM branches WHERE id=?`, branchID).Scan(&p); err == nil && p != "" {
			q = `SELECT id FROM members WHERE branch_id IN (SELECT id FROM branches WHERE path = ? OR path LIKE ?)`
			args = []interface{}{p, p + ".%"}
		}
	}
	rows, err := db.Query(q, args...)
	if err != nil {
		return nil
	}
	defer rows.Close()
	out := []interface{}{}
	for rows.Next() {
		var id int64
		if rows.Scan(&id) == nil {
			out = append(out, id)
		}
	}
	return out
}

// AndCond 把条件片段拼为 " AND xxx"（空则返回空串）
func AndCond(cond string) string {
	if cond == "" {
		return ""
	}
	return " AND " + cond
}

// sqlInPlaceholders 生成 (?,?,?) 占位符
func sqlInPlaceholders(n int) string {
	if n <= 0 {
		return "(NULL)"
	}
	s := "("
	for i := 0; i < n; i++ {
		if i > 0 {
			s += ","
		}
		s += "?"
	}
	return s + ")"
}
