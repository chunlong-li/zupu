package handler

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// GenerationWord 字辈派语
type GenerationWord struct {
	ID              int64  `json:"id"`
	Branch          string `json:"branch"`
	Poem            string `json:"poem"`
	WordsArray      string `json:"words_array"`
	StartGeneration int    `json:"start_generation"`
}

// GenerationWordHandler 字辈处理器
type GenerationWordHandler struct {
	DB *sql.DB
}

func (h *GenerationWordHandler) List(c *gin.Context) {
	rows, err := h.DB.Query(`SELECT id, branch, poem, words_array, start_generation FROM generation_words ORDER BY id`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()
	list := []GenerationWord{}
	for rows.Next() {
		var g GenerationWord
		if err := rows.Scan(&g.ID, &g.Branch, &g.Poem, &g.WordsArray, &g.StartGeneration); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		list = append(list, g)
	}
	c.JSON(http.StatusOK, list)
}

func (h *GenerationWordHandler) Create(c *gin.Context) {
	var g GenerationWord
	if err := c.ShouldBindJSON(&g); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// 理事提交 → 进入审核队列
	if isElder(c) {
		submitDraft(h.DB, "generation_word", "create", 0, g, c)
		return
	}
	res, err := h.DB.Exec(`INSERT INTO generation_words (branch, poem, words_array, start_generation) VALUES (?,?,?,?)`,
		g.Branch, g.Poem, g.WordsArray, g.StartGeneration)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	g.ID, _ = res.LastInsertId()
	c.JSON(http.StatusOK, g)
}

func (h *GenerationWordHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var g GenerationWord
	if err := c.ShouldBindJSON(&g); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// 理事提交 → 进入审核队列
	if isElder(c) {
		submitDraft(h.DB, "generation_word", "update", id, g, c)
		return
	}
	_, err := h.DB.Exec(`UPDATE generation_words SET branch=?, poem=?, words_array=?, start_generation=? WHERE id=?`,
		g.Branch, g.Poem, g.WordsArray, g.StartGeneration, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	g.ID = id
	c.JSON(http.StatusOK, g)
}

func (h *GenerationWordHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	// 理事提交 → 进入审核队列
	if isElder(c) {
		submitDraft(h.DB, "generation_word", "delete", id, gin.H{"id": id}, c)
		return
	}
	_, err := h.DB.Exec(`DELETE FROM generation_words WHERE id=?`, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// applyDraft 将字辈派语草稿应用到正式库
func (h *GenerationWordHandler) applyDraft(d *EditDraft) error {
	var g GenerationWord
	if err := json.Unmarshal([]byte(d.Payload), &g); err != nil {
		return err
	}
	switch d.Action {
	case "create":
		_, err := h.DB.Exec(`INSERT INTO generation_words (branch, poem, words_array, start_generation) VALUES (?,?,?,?)`,
			g.Branch, g.Poem, g.WordsArray, g.StartGeneration)
		return err
	case "update":
		_, err := h.DB.Exec(`UPDATE generation_words SET branch=?, poem=?, words_array=?, start_generation=? WHERE id=?`,
			g.Branch, g.Poem, g.WordsArray, g.StartGeneration, d.EntityID)
		return err
	case "delete":
		_, err := h.DB.Exec(`DELETE FROM generation_words WHERE id=?`, d.EntityID)
		return err
	}
	return nil
}
