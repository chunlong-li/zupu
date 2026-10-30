package handler

import (
	"net/http"
	"strconv"
	"strings"
	"unicode"

	"github.com/gin-gonic/gin"
)

// DNASTRDetail 单个位点的比对明细
type DNASTRDetail struct {
	Locus  string `json:"locus"`
	Parent []int  `json:"parent"`
	Child  []int  `json:"child"`
	Match  bool   `json:"match"`
}

// extractDigits 从字符串中提取连续数字（忽略非数字字符，如 "l12" -> 12）
func extractDigits(s string) int {
	var b strings.Builder
	for _, r := range s {
		if unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	n, _ := strconv.Atoi(b.String())
	return n
}

// parseSTRText 解析 "位点名/数1/数2-位点名/数1/数2-..." 格式（- 或换行或中文逗号分隔位点，/ 分隔字段）
func parseSTRText(text string) map[string][]int {
	result := map[string][]int{}
	text = strings.ReplaceAll(text, "\n", "-")
	text = strings.ReplaceAll(text, "\r", "-")
	text = strings.ReplaceAll(text, "，", "-")
	for _, part := range strings.Split(text, "-") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		fields := strings.Split(part, "/")
		if len(fields) < 3 {
			continue
		}
		locus := strings.TrimSpace(fields[0])
		a1 := extractDigits(fields[1])
		a2 := extractDigits(fields[2])
		if locus == "" || a1 <= 0 || a2 <= 0 || a1 > 99 || a2 > 99 {
			continue
		}
		result[locus] = []int{a1, a2}
	}
	return result
}

// CompareDNASTR 亲子比对 POST /members/dna-str-compare
// 从两成员的「二倍体」字段（dna_diploid）解密并解析 STR 位点串，孟德尔排除法比对共有位点
func (h *MemberHandler) CompareDNASTR(c *gin.Context) {
	var req struct {
		ChildID  int64 `json:"child_id"`
		ParentID int64 `json:"parent_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.ChildID == 0 || req.ParentID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请提供 child_id 和 parent_id"})
		return
	}

	load := func(id int64) (string, map[string][]int) {
		var name, diploid string
		h.DB.QueryRow(`SELECT name, COALESCE(dna_diploid,'') FROM members WHERE id = ?`, id).Scan(&name, &diploid)
		diploid = DecryptValue(diploid, h.EncKey)
		return name, parseSTRText(diploid)
	}

	childName, childLoci := load(req.ChildID)
	parentName, parentLoci := load(req.ParentID)

	matched, mismatched := 0, 0
	var mismatchLoci []string
	detail := []DNASTRDetail{}
	for locus, ch := range childLoci {
		p, ok := parentLoci[locus]
		if !ok {
			continue
		}
		m := ch[0] == p[0] || ch[0] == p[1] || ch[1] == p[0] || ch[1] == p[1]
		if m {
			matched++
		} else {
			mismatched++
			mismatchLoci = append(mismatchLoci, locus)
		}
		detail = append(detail, DNASTRDetail{locus, p, ch, m})
	}

	conclusion := "不排除亲子关系（符合孟德尔遗传）"
	if matched == 0 && mismatched == 0 {
		conclusion = "数据不足：双方未在「二倍体」字段录入 STR 位点"
	} else if mismatched >= 2 {
		conclusion = "排除亲子关系"
	} else if mismatched == 1 {
		conclusion = "存疑：1 个位点不符（可能为突变），建议加测位点"
	}

	c.JSON(http.StatusOK, gin.H{
		"child":           gin.H{"id": req.ChildID, "name": childName},
		"parent":          gin.H{"id": req.ParentID, "name": parentName},
		"loci_compared":   matched + mismatched,
		"loci_matched":    matched,
		"loci_mismatched": mismatched,
		"mismatch_loci":   mismatchLoci,
		"conclusion":      conclusion,
		"detail":          detail,
	})
}
