package handler

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// EditDraft 编辑草稿（提交-审核队列中的一条待审修改）
type EditDraft struct {
	ID              int64         `json:"id"`
	EntityType      string        `json:"entity_type"` // member | spouse | event | article | cemetery | generation_word
	Action          string        `json:"action"`      // create | update | delete | rename
	EntityID        int64         `json:"entity_id"`
	Payload         string        `json:"payload"`
	Changes         []FieldChange `json:"changes"` // 前端「查看修改」弹窗内容
	Summary         string        `json:"summary"` // 删除类草稿的对象描述
	SubmittedBy     int           `json:"submitted_by"`
	SubmittedByName string        `json:"submitted_by_name"`
	Status          string        `json:"status"` // pending | approved | rejected
	ReviewerBy      int           `json:"reviewer_by"`
	ReviewerName    string        `json:"reviewer_name"`
	Reason          string        `json:"reason"`
	CreatedAt       string        `json:"created_at"`
	ReviewedAt      string        `json:"reviewed_at"`
}

// DraftHandler 审核队列处理器
type DraftHandler struct {
	DB     *sql.DB
	EncKey string
}

// isElderOnly 当前登录角色是否仅为理事（理事写操作进入审核队列）
func isElder(c *gin.Context) bool {
	role := c.GetString("role")
	return role == RoleElder || role == RoleBranchElder
}

// submitDraft 保存一条待审核草稿，并以 JSON 返回「已提交审核」提示
func submitDraft(db *sql.DB, entityType, action string, entityID int64, payload interface{}, c *gin.Context) {
	b, _ := json.Marshal(payload)
	// 提交时计算字段级变更（供审核时「查看修改」弹窗）；删除类草稿固化对象描述
	var changesJSON string
	if action == "delete" {
		c := []FieldChange{{Field: "删除对象", From: entitySummary(db, entityType, entityID), To: "（删除该记录）"}}
		cb, _ := json.Marshal(c)
		changesJSON = string(cb)
	} else {
		changes := computeChanges(db, entityType, action, entityID, string(b))
		cb, _ := json.Marshal(changes)
		changesJSON = string(cb)
	}
	uid, uname := currentUser(c)
	_, err := db.Exec(`INSERT INTO edit_drafts
		(entity_type, action, entity_id, payload, changes, submitted_by, submitted_by_name, status)
		VALUES (?,?,?,?,?,?,?, 'pending')`,
		entityType, action, entityID, string(b), changesJSON, uid, uname)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"draft":   true,
		"message": "已提交审核，待理事长/超级管理员审核通过后生效",
	})
}

// ListDrafts 列出草稿（理事长及以上）
// GET /api/drafts?status=pending
func (h *DraftHandler) ListDrafts(c *gin.Context) {
	status := c.Query("status")
	q := `SELECT id, entity_type, action, entity_id, COALESCE(payload,''), COALESCE(changes,''), submitted_by, submitted_by_name,
		status, reviewer_by, reviewer_name, COALESCE(reason,''), COALESCE(created_at,''), COALESCE(reviewed_at,'')
		FROM edit_drafts`
	conds := []string{}
	args := []interface{}{}
	switch status {
	case "pending":
		conds = append(conds, "status = 'pending'")
	case "done":
		conds = append(conds, "status IN ('approved','rejected')")
	}
	if bcond, bargs := BranchScope(c, h.DB, "branch_id"); bcond != "" {
		conds = append(conds, "entity_type = 'member' AND entity_id IN (SELECT id FROM members WHERE "+bcond+")")
		args = append(args, bargs...)
	}
	if len(conds) > 0 {
		q += " WHERE " + strings.Join(conds, " AND ")
	}
	q += ` ORDER BY id DESC`
	rows, err := h.DB.Query(q, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	list := []EditDraft{}
	for rows.Next() {
		var d EditDraft
		var changesJSON string
		if err := rows.Scan(&d.ID, &d.EntityType, &d.Action, &d.EntityID, &d.Payload, &changesJSON,
			&d.SubmittedBy, &d.SubmittedByName, &d.Status, &d.ReviewerBy, &d.ReviewerName,
			&d.Reason, &d.CreatedAt, &d.ReviewedAt); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		// 优先使用提交时固化的变更；为空时（旧数据）动态计算
		if changesJSON != "" {
			_ = json.Unmarshal([]byte(changesJSON), &d.Changes)
		} else {
			d.Changes = computeChanges(h.DB, d.EntityType, d.Action, d.EntityID, d.Payload)
		}
		if d.Action == "delete" {
			d.Summary = entitySummary(h.DB, d.EntityType, d.EntityID)
		}
		list = append(list, d)
	}
	c.JSON(http.StatusOK, list)
}

// ReviewDraft 审核草稿：approve 合并入库，reject 驳回
// POST /api/drafts/:id/review  body: {"action":"approve|reject","reason":"..."}
func (h *DraftHandler) ReviewDraft(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	var body struct {
		Action string `json:"action"`
		Reason string `json:"reason"`
	}
	_ = c.ShouldBindJSON(&body)
	if body.Action != "approve" && body.Action != "reject" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "action 必须为 approve 或 reject"})
		return
	}

	var d EditDraft
	err = h.DB.QueryRow(`SELECT id, entity_type, action, entity_id, COALESCE(payload,''),
		submitted_by, submitted_by_name, status FROM edit_drafts WHERE id = ?`, id).
		Scan(&d.ID, &d.EntityType, &d.Action, &d.EntityID, &d.Payload,
			&d.SubmittedBy, &d.SubmittedByName, &d.Status)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "草稿不存在"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if d.Status != "pending" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "该草稿已处理，请勿重复审核"})
		return
	}

	uid, uname := currentUser(c)

	if body.Action == "reject" {
		_, err := h.DB.Exec(`UPDATE edit_drafts SET status='rejected', reviewer_by=?, reviewer_name=?, reason=?, reviewed_at=CURRENT_TIMESTAMP WHERE id=?`,
			uid, uname, body.Reason, id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "status": "rejected"})
		return
	}

	// approve：应用到正式库
	if err := h.applyDraft(&d); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "合并失败：" + err.Error()})
		return
	}
	_, err = h.DB.Exec(`UPDATE edit_drafts SET status='approved', reviewer_by=?, reviewer_name=?, reason=?, reviewed_at=CURRENT_TIMESTAMP WHERE id=?`,
		uid, uname, body.Reason, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "status": "approved"})
}

// applyDraft 将草稿应用到正式库（member / spouse / event / article / cemetery / generation_word / settings）
func (h *DraftHandler) applyDraft(d *EditDraft) error {
	changedBy := d.SubmittedBy
	changedByName := d.SubmittedByName

	switch d.EntityType {
	case "member":
		mh := &MemberHandler{DB: h.DB, EncKey: h.EncKey}
		switch d.Action {
		case "create":
			var m Member
			if err := json.Unmarshal([]byte(d.Payload), &m); err != nil {
				return err
			}
			_, err := mh.applyCreate(&m, changedBy, changedByName)
			return err
		case "update":
			var m Member
			if err := json.Unmarshal([]byte(d.Payload), &m); err != nil {
				return err
			}
			return mh.applyUpdate(d.EntityID, &m, changedBy, changedByName)
		case "rename":
			var req struct {
				GivenName string `json:"given_name"`
			}
			if err := json.Unmarshal([]byte(d.Payload), &req); err != nil {
				return err
			}
			_, err := mh.applyRename(d.EntityID, req.GivenName, changedBy, changedByName)
			return err
		case "delete":
			return mh.applyDelete(d.EntityID, changedBy, changedByName)
		}
	case "spouse":
		sh := &SpouseHandler{DB: h.DB, EncKey: h.EncKey}
		switch d.Action {
		case "create":
			var s Spouse
			if err := json.Unmarshal([]byte(d.Payload), &s); err != nil {
				return err
			}
			_, err := sh.applyCreate(&s, changedBy, changedByName)
			return err
		case "update":
			var s Spouse
			if err := json.Unmarshal([]byte(d.Payload), &s); err != nil {
				return err
			}
			return sh.applyUpdate(d.EntityID, &s, changedBy, changedByName)
		case "delete":
			return sh.applyDelete(d.EntityID, changedBy, changedByName)
		}
	case "event":
		eh := &EventHandler{DB: h.DB}
		return eh.applyDraft(d)
	case "article":
		eh := &EventHandler{DB: h.DB}
		return eh.applyDraft(d)
	case "cemetery":
		eh := &EventHandler{DB: h.DB}
		return eh.applyDraft(d)
	case "biography":
		bh := &BiographyHandler{DB: h.DB}
		return bh.applyDraft(d)
	case "generation_word":
		gh := &GenerationWordHandler{DB: h.DB}
		return gh.applyDraft(d)
	case "member_bio":
		bh := &BioHandler{DB: h.DB}
		switch d.Action {
		case "create":
			var e BioEntry
			if err := json.Unmarshal([]byte(d.Payload), &e); err != nil {
				return err
			}
			_, err := bh.applyCreateMemberBio(e.MemberID, &e)
			return err
		case "update":
			var e BioEntry
			if err := json.Unmarshal([]byte(d.Payload), &e); err != nil {
				return err
			}
			return bh.applyUpdateMemberBio(d.EntityID, &e)
		case "delete":
			return bh.applyDeleteMemberBio(d.EntityID)
		}
	case "spouse_bio":
		bh := &BioHandler{DB: h.DB}
		switch d.Action {
		case "create":
			var e SpouseBioEntry
			if err := json.Unmarshal([]byte(d.Payload), &e); err != nil {
				return err
			}
			_, err := bh.applyCreateSpouseBio(e.SpouseID, &e)
			return err
		case "update":
			var e SpouseBioEntry
			if err := json.Unmarshal([]byte(d.Payload), &e); err != nil {
				return err
			}
			return bh.applyUpdateSpouseBio(d.EntityID, &e)
		case "delete":
			return bh.applyDeleteSpouseBio(d.EntityID)
		}
	}
	return nil
}
