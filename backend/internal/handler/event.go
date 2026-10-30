package handler

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Event 家族大事记
type Event struct {
	ID      int64  `json:"id"`
	Year    string `json:"year"`
	Title   string `json:"title"`
	Content string `json:"content"`
}

// Article 谱序/凡例/祖训等文章
type Article struct {
	ID       int64  `json:"id"`
	Category string `json:"category"`
	Title    string `json:"title"`
	Author   string `json:"author"`
	Year     string `json:"year"`
	Content  string `json:"content"`
}

// Cemetery 祠墓
type Cemetery struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Location    string `json:"location"`
	Description string `json:"description"`
	Photo       string `json:"photo"`
}

// EventHandler 大事记处理器
type EventHandler struct {
	DB *sql.DB
}

// --- Events ---

func (h *EventHandler) ListEvents(c *gin.Context) {
	rows, err := h.DB.Query(`SELECT id, year, title, content FROM events ORDER BY year, id`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()
	list := []Event{}
	for rows.Next() {
		var e Event
		rows.Scan(&e.ID, &e.Year, &e.Title, &e.Content)
		list = append(list, e)
	}
	c.JSON(http.StatusOK, list)
}

func (h *EventHandler) CreateEvent(c *gin.Context) {
	var e Event
	if err := c.ShouldBindJSON(&e); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// 理事提交 → 进入审核队列
	if isElder(c) {
		submitDraft(h.DB, "event", "create", 0, e, c)
		return
	}
	res, err := h.DB.Exec(`INSERT INTO events (year, title, content) VALUES (?,?,?)`, e.Year, e.Title, e.Content)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	e.ID, _ = res.LastInsertId()
	c.JSON(http.StatusOK, e)
}

func (h *EventHandler) UpdateEvent(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var e Event
	if err := c.ShouldBindJSON(&e); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// 理事提交 → 进入审核队列
	if isElder(c) {
		submitDraft(h.DB, "event", "update", id, e, c)
		return
	}
	h.DB.Exec(`UPDATE events SET year=?, title=?, content=? WHERE id=?`, e.Year, e.Title, e.Content, id)
	e.ID = id
	c.JSON(http.StatusOK, e)
}

func (h *EventHandler) DeleteEvent(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	// 理事提交 → 进入审核队列
	if isElder(c) {
		submitDraft(h.DB, "event", "delete", id, gin.H{"id": id}, c)
		return
	}
	h.DB.Exec(`DELETE FROM events WHERE id=?`, id)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// --- Articles ---

func (h *EventHandler) ListArticles(c *gin.Context) {
	cat := c.DefaultQuery("category", "")
	var rows *sql.Rows
	var err error
	if cat != "" {
		rows, err = h.DB.Query(`SELECT id, category, title, author, year, content FROM articles WHERE category=? ORDER BY id`, cat)
	} else {
		rows, err = h.DB.Query(`SELECT id, category, title, author, year, content FROM articles ORDER BY category, id`)
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()
	list := []Article{}
	for rows.Next() {
		var a Article
		rows.Scan(&a.ID, &a.Category, &a.Title, &a.Author, &a.Year, &a.Content)
		list = append(list, a)
	}
	c.JSON(http.StatusOK, list)
}

func (h *EventHandler) CreateArticle(c *gin.Context) {
	var a Article
	if err := c.ShouldBindJSON(&a); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// 理事提交 → 进入审核队列
	if isElder(c) {
		submitDraft(h.DB, "article", "create", 0, a, c)
		return
	}
	res, err := h.DB.Exec(`INSERT INTO articles (category, title, author, year, content) VALUES (?,?,?,?,?)`,
		a.Category, a.Title, a.Author, a.Year, a.Content)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	a.ID, _ = res.LastInsertId()
	c.JSON(http.StatusOK, a)
}

func (h *EventHandler) UpdateArticle(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var a Article
	if err := c.ShouldBindJSON(&a); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// 理事提交 → 进入审核队列
	if isElder(c) {
		submitDraft(h.DB, "article", "update", id, a, c)
		return
	}
	h.DB.Exec(`UPDATE articles SET category=?, title=?, author=?, year=?, content=? WHERE id=?`,
		a.Category, a.Title, a.Author, a.Year, a.Content, id)
	a.ID = id
	c.JSON(http.StatusOK, a)
}

func (h *EventHandler) DeleteArticle(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	// 理事提交 → 进入审核队列
	if isElder(c) {
		submitDraft(h.DB, "article", "delete", id, gin.H{"id": id}, c)
		return
	}
	h.DB.Exec(`DELETE FROM articles WHERE id=?`, id)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// --- Cemeteries ---

// ListCemeteries 祠墓列表。
// 兼容旧调用（不传 kind 时行为与以前完全一致）；可选：
//
//	?kind=shrine|grave  只取祠宇志 / 茔墓志
//	?kind=legacy        只取旧「祠墓」记录（kind 为空，尚未归入新模块）
func (h *EventHandler) ListCemeteries(c *gin.Context) {
	q := `SELECT id, name, location, description, photo FROM cemeteries`
	args := []interface{}{}
	switch kind := c.Query("kind"); {
	case kind == "legacy":
		q += ` WHERE COALESCE(kind,'') = ''`
	case kind != "":
		q += ` WHERE kind = ?`
		args = append(args, kind)
	}
	q += ` ORDER BY id`
	rows, err := h.DB.Query(q, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()
	list := []Cemetery{}
	for rows.Next() {
		var cm Cemetery
		rows.Scan(&cm.ID, &cm.Name, &cm.Location, &cm.Description, &cm.Photo)
		list = append(list, cm)
	}
	c.JSON(http.StatusOK, list)
}

func (h *EventHandler) CreateCemetery(c *gin.Context) {
	var cm Cemetery
	if err := c.ShouldBindJSON(&cm); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// 理事提交 → 进入审核队列
	if isElder(c) {
		submitDraft(h.DB, "cemetery", "create", 0, cm, c)
		return
	}
	res, err := h.DB.Exec(`INSERT INTO cemeteries (name, location, description, photo) VALUES (?,?,?,?)`,
		cm.Name, cm.Location, cm.Description, cm.Photo)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	cm.ID, _ = res.LastInsertId()
	c.JSON(http.StatusOK, cm)
}

func (h *EventHandler) UpdateCemetery(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var cm Cemetery
	if err := c.ShouldBindJSON(&cm); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// 理事提交 → 进入审核队列
	if isElder(c) {
		submitDraft(h.DB, "cemetery", "update", id, cm, c)
		return
	}
	h.DB.Exec(`UPDATE cemeteries SET name=?, location=?, description=?, photo=? WHERE id=?`,
		cm.Name, cm.Location, cm.Description, cm.Photo, id)
	cm.ID = id
	c.JSON(http.StatusOK, cm)
}

func (h *EventHandler) DeleteCemetery(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	// 理事提交 → 进入审核队列
	if isElder(c) {
		submitDraft(h.DB, "cemetery", "delete", id, gin.H{"id": id}, c)
		return
	}
	h.DB.Exec(`DELETE FROM cemeteries WHERE id=?`, id)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// applyDraft 将草稿应用到正式库（大事记/文章/祠墓共用）
func (h *EventHandler) applyDraft(d *EditDraft) error {
	switch d.EntityType {
	case "event":
		switch d.Action {
		case "create":
			var e Event
			if err := json.Unmarshal([]byte(d.Payload), &e); err != nil {
				return err
			}
			_, err := h.DB.Exec(`INSERT INTO events (year, title, content) VALUES (?,?,?)`, e.Year, e.Title, e.Content)
			return err
		case "update":
			var e Event
			if err := json.Unmarshal([]byte(d.Payload), &e); err != nil {
				return err
			}
			_, err := h.DB.Exec(`UPDATE events SET year=?, title=?, content=? WHERE id=?`, e.Year, e.Title, e.Content, d.EntityID)
			return err
		case "delete":
			_, err := h.DB.Exec(`DELETE FROM events WHERE id=?`, d.EntityID)
			return err
		}
	case "article":
		switch d.Action {
		case "create":
			var a Article
			if err := json.Unmarshal([]byte(d.Payload), &a); err != nil {
				return err
			}
			_, err := h.DB.Exec(`INSERT INTO articles (category, title, author, year, content) VALUES (?,?,?,?,?)`,
				a.Category, a.Title, a.Author, a.Year, a.Content)
			return err
		case "update":
			var a Article
			if err := json.Unmarshal([]byte(d.Payload), &a); err != nil {
				return err
			}
			_, err := h.DB.Exec(`UPDATE articles SET category=?, title=?, author=?, year=?, content=? WHERE id=?`,
				a.Category, a.Title, a.Author, a.Year, a.Content, d.EntityID)
			return err
		case "delete":
			_, err := h.DB.Exec(`DELETE FROM articles WHERE id=?`, d.EntityID)
			return err
		}
	case "cemetery":
		switch d.Action {
		case "create":
			var cm Cemetery
			if err := json.Unmarshal([]byte(d.Payload), &cm); err != nil {
				return err
			}
			_, err := h.DB.Exec(`INSERT INTO cemeteries (name, location, description, photo) VALUES (?,?,?,?)`,
				cm.Name, cm.Location, cm.Description, cm.Photo)
			return err
		case "update":
			var cm Cemetery
			if err := json.Unmarshal([]byte(d.Payload), &cm); err != nil {
				return err
			}
			_, err := h.DB.Exec(`UPDATE cemeteries SET name=?, location=?, description=?, photo=? WHERE id=?`,
				cm.Name, cm.Location, cm.Description, cm.Photo, d.EntityID)
			return err
		case "delete":
			_, err := h.DB.Exec(`DELETE FROM cemeteries WHERE id=?`, d.EntityID)
			return err
		}
	}
	return nil
}
