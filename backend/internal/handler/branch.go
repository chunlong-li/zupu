package handler

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// Branch 房支结构体（支持多级：宗谱 → 长房 → 长长房…）
type Branch struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`      // 短名/序数，如「长房」
	Subtitle  string `json:"subtitle"`  // 名讳/区分名，如「廷玉公」
	ParentID  int64  `json:"parent_id"` // 上级房支 id，0=直属宗谱
	Path      string `json:"path"`      // 层级路径（id 用点连接），如 3.7.12
	Level     int    `json:"level"`     // 层级，1=一级房支
	SortOrder int    `json:"sort_order"`
	Remark    string `json:"remark"`
	Count     int    `json:"count"`     // 本房支直属成员数
	CountAll  int    `json:"count_all"` // 含下级房支的成员数
}

// BranchHandler 房支处理器
type BranchHandler struct {
	DB *sql.DB
}

// List 返回全部房支（含成员数统计）
func (h *BranchHandler) List(c *gin.Context) {
	rows, err := h.DB.Query(`
		SELECT b.id, b.name, COALESCE(b.subtitle,''), COALESCE(b.parent_id,0), COALESCE(b.path,''), COALESCE(b.level,1), b.sort_order, COALESCE(b.remark,''),
			(SELECT COUNT(*) FROM members m WHERE m.branch_id = b.id) AS cnt,
			(SELECT COUNT(*) FROM members m WHERE m.branch_id IN (
				SELECT x.id FROM branches x WHERE x.path = b.path OR x.path LIKE b.path || '.%')) AS cnt_all
		FROM branches b
		ORDER BY b.parent_id, b.sort_order, b.id`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	list := []Branch{}
	for rows.Next() {
		var b Branch
		if err := rows.Scan(&b.ID, &b.Name, &b.Subtitle, &b.ParentID, &b.Path, &b.Level, &b.SortOrder, &b.Remark, &b.Count, &b.CountAll); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		list = append(list, b)
	}
	c.JSON(http.StatusOK, list)
}

// Create 新增房支
func (h *BranchHandler) Create(c *gin.Context) {
	var b Branch
	if err := c.ShouldBindJSON(&b); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	b.Name = strings.TrimSpace(b.Name)
	if b.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "房支名不能为空"})
		return
	}
	b.Subtitle = strings.TrimSpace(b.Subtitle)

	parentPath := ""
	level := 1
	if b.ParentID > 0 {
		var pp string
		var pl int
		if err := h.DB.QueryRow(`SELECT COALESCE(path,''), COALESCE(level,1) FROM branches WHERE id=?`, b.ParentID).Scan(&pp, &pl); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "上级房支不存在"})
			return
		}
		parentPath = pp
		level = pl + 1
	}

	res, err := h.DB.Exec(`INSERT INTO branches (name, subtitle, parent_id, sort_order, remark) VALUES (?,?,?,?,?)`,
		b.Name, b.Subtitle, b.ParentID, b.SortOrder, b.Remark)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			c.JSON(http.StatusConflict, gin.H{"error": "同一上级下已有同名房支"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	b.ID, _ = res.LastInsertId()
	b.Level = level
	if parentPath == "" {
		b.Path = strconv.FormatInt(b.ID, 10)
	} else {
		b.Path = parentPath + "." + strconv.FormatInt(b.ID, 10)
	}
	h.DB.Exec(`UPDATE branches SET path=?, level=? WHERE id=?`, b.Path, b.Level, b.ID)
	c.JSON(http.StatusOK, b)
}

// Update 更新房支
func (h *BranchHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var b Branch
	if err := c.ShouldBindJSON(&b); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	b.Name = strings.TrimSpace(b.Name)
	if b.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "房支名不能为空"})
		return
	}

	b.Name = strings.TrimSpace(b.Name)
	if b.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "房支名不能为空"})
		return
	}
	b.Subtitle = strings.TrimSpace(b.Subtitle)

	// 读取旧信息
	var oldName, oldPath string
	var oldLevel int
	var oldParent int64
	err := h.DB.QueryRow(`SELECT name, COALESCE(path,''), COALESCE(level,1), COALESCE(parent_id,0) FROM branches WHERE id=?`, id).
		Scan(&oldName, &oldPath, &oldLevel, &oldParent)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "房支不存在"})
		return
	}

	// 目标上级（未提供则保持原上级）
	newParent := b.ParentID
	if b.ParentID == 0 {
		newParent = oldParent
	}
	if newParent == id {
		c.JSON(http.StatusBadRequest, gin.H{"error": "上级不能是自己"})
		return
	}
	parentPath := ""
	level := 1
	if newParent > 0 {
		var pp string
		var pl int
		if err := h.DB.QueryRow(`SELECT COALESCE(path,''), COALESCE(level,1) FROM branches WHERE id=?`, newParent).Scan(&pp, &pl); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "上级房支不存在"})
			return
		}
		parentPath = pp
		level = pl + 1
	}
	newPath := strconv.FormatInt(id, 10)
	if parentPath != "" {
		newPath = parentPath + "." + strconv.FormatInt(id, 10)
	}
	// 禁止移动到自己或自己的后代之下
	if newParent > 0 && (newParent == id || strings.HasPrefix(parentPath, oldPath+".") || parentPath == oldPath) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "不能把房支移动到它自己的下级"})
		return
	}

	_, err = h.DB.Exec(`UPDATE branches SET name=?, subtitle=?, parent_id=?, sort_order=?, remark=?, path=?, level=? WHERE id=?`,
		b.Name, b.Subtitle, newParent, b.SortOrder, b.Remark, newPath, level, id)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			c.JSON(http.StatusConflict, gin.H{"error": "同一上级下已有同名房支"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 移动时重写所有后代的 path/level
	if newPath != oldPath {
		delta := level - oldLevel
		h.DB.Exec(`UPDATE branches SET path = ? || SUBSTR(path, ?), level = level + ? WHERE path LIKE ?`,
			newPath, len(oldPath)+1, delta, oldPath+".%")
	}

	// 改名时同步更新成员的 branch 文本（branch_id 不变）
	if oldName != b.Name {
		h.DB.Exec(`UPDATE members SET branch=? WHERE branch_id=?`, b.Name, id)
	}
	b.ID = id
	b.ParentID = newParent
	b.Path = newPath
	b.Level = level
	c.JSON(http.StatusOK, b)
}

// Delete 删除房支（成员仍保留，branch_id 置 0）
func (h *BranchHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var nchild int
	h.DB.QueryRow(`SELECT COUNT(*) FROM branches WHERE COALESCE(parent_id,0)=?`, id).Scan(&nchild)
	if nchild > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "该房支下还有下级房支，请先删除或移走下级"})
		return
	}
	_, err := h.DB.Exec(`DELETE FROM branches WHERE id = ?`, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// 该房支下的成员 branch_id 置 0，branch 文本清空
	h.DB.Exec(`UPDATE members SET branch_id=0, branch='' WHERE branch_id=?`, id)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// SetMemberBranch 批量设置/清除成员的房支归属
// POST /api/branches/set-members { branch_id, member_ids: [...], replace: bool }
//   - replace=true：以 member_ids 为该房支的完整成员集合（先清空该房支下未勾选成员）
//   - replace=false（默认）：仅把 member_ids 里的成员设为该房支，不动其他成员
func (h *BranchHandler) SetMemberBranch(c *gin.Context) {
	var req struct {
		BranchID  int64   `json:"branch_id"`
		MemberIDs []int64 `json:"member_ids"`
		Replace   bool    `json:"replace"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	branchName := ""
	if req.BranchID > 0 {
		err := h.DB.QueryRow(`SELECT name FROM branches WHERE id = ?`, req.BranchID).Scan(&branchName)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "房支不存在"})
			return
		}
	}

	// 先记录各成员旧房支（在替换清空之前），供级联判断
	oldBids := map[int64]int64{}
	for _, mid := range req.MemberIDs {
		var ob int64
		if err := h.DB.QueryRow(`SELECT COALESCE(branch_id,0) FROM members WHERE id=?`, mid).Scan(&ob); err == nil {
			oldBids[mid] = ob
		}
	}

	// 替换语义：先把该房支下所有成员清空（归入未分房），再按 member_ids 重新归属
	if req.Replace && req.BranchID > 0 {
		h.DB.Exec(`UPDATE members SET branch_id=0, branch='' WHERE branch_id=?`, req.BranchID)
	}

	cascaded := 0
	for _, mid := range req.MemberIDs {
		if req.BranchID > 0 {
			h.DB.Exec(`UPDATE members SET branch_id=?, branch=? WHERE id=?`, req.BranchID, branchName, mid)
		} else {
			h.DB.Exec(`UPDATE members SET branch_id=0, branch='' WHERE id=?`, mid)
		}
		if oldBid, ok := oldBids[mid]; ok && oldBid != req.BranchID {
			cascaded += cascadeBranchToDescendants(h.DB, mid, oldBid, req.BranchID, branchName)
		}
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "cascaded": cascaded})
}

// Backfill 一键回填：把未分房（branch_id=0）的成员，按「最近有房支的祖先」自动归位。
// 多源 BFS：以所有已有房支的成员为起点向下传播，遇到已分房的后代则停（分房保护）。
func (h *BranchHandler) Backfill(c *gin.Context) {
	cnt, err := backfillBranchByAncestor(h.DB)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "filled": cnt})
}

func backfillBranchByAncestor(db *sql.DB) (int, error) {
	type node struct {
		id       int64
		fatherID int64
		branchID int64
	}
	rows, err := db.Query(`SELECT id, father_id, COALESCE(branch_id,0) FROM members`)
	if err != nil {
		return 0, err
	}
	nodes := map[int64]node{}
	for rows.Next() {
		var n node
		if rows.Scan(&n.id, &n.fatherID, &n.branchID) == nil {
			nodes[n.id] = n
		}
	}
	rows.Close()

	children := map[int64][]int64{}
	for _, n := range nodes {
		if n.fatherID > 0 {
			children[n.fatherID] = append(children[n.fatherID], n.id)
		}
	}

	nameCache := map[int64]string{}
	getName := func(bid int64) string {
		if s, ok := nameCache[bid]; ok {
			return s
		}
		var s string
		db.QueryRow(`SELECT name FROM branches WHERE id=?`, bid).Scan(&s)
		nameCache[bid] = s
		return s
	}

	type qitem struct {
		id  int64
		bid int64
	}
	queue := []qitem{}
	for id, n := range nodes {
		if n.branchID > 0 {
			queue = append(queue, qitem{id, n.branchID})
		}
	}

	cnt := 0
	for len(queue) > 0 {
		it := queue[0]
		queue = queue[1:]
		for _, cid := range children[it.id] {
			c, ok := nodes[cid]
			if !ok || c.branchID != 0 {
				continue
			}
			name := getName(it.bid)
			if _, err := db.Exec(`UPDATE members SET branch_id=?, branch=? WHERE id=?`, it.bid, name, cid); err != nil {
				continue
			}
			c.branchID = it.bid
			nodes[cid] = c
			cnt++
			queue = append(queue, qitem{cid, it.bid})
		}
	}
	return cnt, nil
}
