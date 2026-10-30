package handler

import (
	"database/sql"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// KBExportHandler 供传承服务器（家族知识库）只读同步族谱内容
// 鉴权：请求头 X-KB-Secret（或 ?secret=）需与 FAMILY_TREE_KB_SSO_SECRET 一致
type KBExportHandler struct {
	DB     *sql.DB
	Secret string
}

// Export 全量导出（供知识库「家族谱牒」分类同步）
// GET /api/kb/export
func (h *KBExportHandler) Export(c *gin.Context) {
	secret := c.GetHeader("X-KB-Secret")
	if secret == "" {
		secret = c.Query("secret")
	}
	if strings.TrimSpace(h.Secret) == "" || secret != h.Secret {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权限"})
		return
	}

	out := gin.H{
		"articles":         []Article{},
		"biographies":      []Biography{},
		"generation_words": []GenerationWord{},
		"traditions":       []Tradition{},
	}

	// 谱头文化：谱序 / 凡例 / 族规 / 祖训 / 家族大记事 …（按分类名再自然序）
	if rows, err := h.DB.Query(`SELECT id, category, title, author, year, content FROM articles ORDER BY category, id`); err == nil {
		list := []Article{}
		for rows.Next() {
			var a Article
			if err := rows.Scan(&a.ID, &a.Category, &a.Title, &a.Author, &a.Year, &a.Content); err == nil {
				list = append(list, a)
			}
		}
		rows.Close()
		out["articles"] = list
	}

	// 名人传记：全文导出（含未公开），由知识库按 public 字段决定密级（公开=族内可见，未公开=机密）
	if rows, err := h.DB.Query(`SELECT id, ` + biographyCols +
		`, COALESCE(created_by,''), COALESCE(created_at,''), COALESCE(updated_at,'')
		FROM biographies ORDER BY id`); err == nil {
		list := []Biography{}
		for rows.Next() {
			var b Biography
			if err := rows.Scan(&b.ID, &b.PersonType, &b.MemberID, &b.SpouseID, &b.Name, &b.Zi, &b.Hao, &b.Gender,
				&b.BirthGregorian, &b.BirthLunar, &b.BirthEra, &b.DeathGregorian, &b.DeathLunar, &b.DeathEra,
				&b.NativePlace, &b.CurrentPlace, &b.Title, &b.Occupation, &b.Honors, &b.Portrait, &b.Summary, &b.Content,
				&b.Reason, &b.SortYear, &b.SortOrder, &b.Public, &b.CreatedBy, &b.CreatedAt, &b.UpdatedAt); err != nil {
				break
			}
			list = append(list, b)
		}
		rows.Close()
		// 与 /api/biographies 保持同一顺序：世代 → 排行 → 生年 → id（配偶随夫、未关联者最后）
		sortBiographies(h.DB, list)
		out["biographies"] = list
	}

	// 字辈派语
	if rows, err := h.DB.Query(`SELECT id, branch, poem, words_array, start_generation FROM generation_words ORDER BY id`); err == nil {
		list := []GenerationWord{}
		for rows.Next() {
			var g GenerationWord
			if err := rows.Scan(&g.ID, &g.Branch, &g.Poem, &g.WordsArray, &g.StartGeneration); err == nil {
				list = append(list, g)
			}
		}
		rows.Close()
		out["generation_words"] = list
	}

	// 家族传统日程（传统节日/祭祖等）：农历月日 + 说明 + 今年公历
	if rows, err := h.DB.Query(`SELECT id, title, lunar_month, lunar_day, is_leap_month, description FROM traditions ORDER BY lunar_month, lunar_day, id`); err == nil {
		now := time.Now()
		list := []Tradition{}
		for rows.Next() {
			var t Tradition
			var leap int
			if err := rows.Scan(&t.ID, &t.Title, &t.LunarMonth, &t.LunarDay, &leap, &t.Description); err == nil {
				t.IsLeapMonth = leap != 0
				// 农历日为 0 = 不固定（如清明按节气、除夕按年末），没有确定的公历对应日，留空即可
				if t.LunarDay > 0 {
					t.ThisYearSolar = safeLunarToSolar(now.Year(), t.LunarMonth, t.IsLeapMonth, t.LunarDay)
				}
				list = append(list, t)
			}
		}
		rows.Close()
		out["traditions"] = list
	}

	c.JSON(http.StatusOK, out)
}
