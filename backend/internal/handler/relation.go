package handler

import (
	"database/sql"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// RelationInfo 关系信息
type RelationInfo struct {
	Relation  string `json:"relation"`
	MemberID  int64  `json:"member_id"`
	SpouseID  int64  `json:"spouse_id"`
	Surname   string `json:"surname"`
	GivenName string `json:"given_name"`
	Name      string `json:"name"`
	Gender    int    `json:"gender"`
}

// Relations 某成员的所有亲属关系
// GET /api/members/:id/relations
//
// 返回 {"组名": [RelationInfo...]}，结构与字段名保持不变（前端直接按组渲染）。
// 具体称谓规则（长幼、堂表亲、姻亲、内外纪过滤）见 relation_extra.go：
// 一次载入人员索引后在内存里按有界层数收集，最后统一去重 + 组内排序。
func (h *MemberHandler) Relations(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)

	idx := buildKinIndex(h.DB)
	self := idx.members[id]
	if self == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "成员不存在"})
		return
	}

	c.JSON(http.StatusOK, collectKin(idx, self).finish())
}

// addRelation 添加一位亲属（单点查询；批量场景请用 buildKinIndex/collectKin）
func addRelation(db *sql.DB, result map[string][]RelationInfo, key string, memberID int64, relation string) {
	var surname, givenname, name string
	var gender int
	err := db.QueryRow(`SELECT surname, given_name, name, gender FROM members WHERE id = ?`, memberID).Scan(&surname, &givenname, &name, &gender)
	if err != nil {
		return
	}
	result[key] = append(result[key], RelationInfo{Relation: relation, MemberID: memberID, Surname: surname, GivenName: givenname, Name: name, Gender: gender})
}

// addChildren 添加某人的所有子女到指定关系组
// 生父或生母命中都算（谱内女性的子女同样要能查到）
func addChildren(db *sql.DB, result map[string][]RelationInfo, key string, parentID int64) {
	rows, err := db.Query(`SELECT id, surname, given_name, name, gender FROM members WHERE father_id = ? OR mother_id = ?`, parentID, parentID)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var rid int64
		var rsurname, rgivenname, rname string
		var rgender int
		if err := rows.Scan(&rid, &rsurname, &rgivenname, &rname, &rgender); err != nil {
			continue
		}
		result[key] = append(result[key], RelationInfo{Relation: key, MemberID: rid, Surname: rsurname, GivenName: rgivenname, Name: rname, Gender: rgender})
	}
}
