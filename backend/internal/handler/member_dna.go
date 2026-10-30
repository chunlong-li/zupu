package handler

import (
	"database/sql"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// DNAMatchItem 单条 DNA 匹配结果
type DNAMatchItem struct {
	MemberID   int64  `json:"member_id"`
	Name       string `json:"name"`
	Generation int    `json:"generation"`
	Branch     string `json:"branch"`
	Field      string `json:"field"`      // haploid | mito | diploid
	MatchType  string `json:"match_type"` // exact | prefix
	Code       string `json:"code"`       // 对方编码
	Relation   string `json:"relation"`   // 父系同源 | 母系同源 | 常染色体匹配
	Score      int    `json:"score"`      // 亲缘可能性评分（越大越可能）
}

// DNAMatch 比对某成员与全库成员的 DNA 编码（精确 + 前缀匹配）
// GET /members/:id/dna-match
func (h *MemberHandler) DNAMatch(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)

	var name, branch, hap, dip, mito string
	var gen int
	err := h.DB.QueryRow(`SELECT name, generation, branch, dna_haploid, dna_diploid, dna_mito FROM members WHERE id = ?`, id).
		Scan(&name, &gen, &branch, &hap, &dip, &mito)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "成员不存在"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	hap = DecryptValue(hap, h.EncKey)
	dip = DecryptValue(dip, h.EncKey)
	mito = DecryptValue(mito, h.EncKey)

	matches := []DNAMatchItem{}
	target := gin.H{"id": id, "name": name, "generation": gen, "branch": branch, "dna_haploid": hap, "dna_diploid": dip, "dna_mito": mito}
	if hap == "" && dip == "" && mito == "" {
		c.JSON(http.StatusOK, gin.H{"target": target, "matches": matches, "total": 0, "note": "该成员未录入 DNA 编码"})
		return
	}

	rows, err := h.DB.Query(`SELECT id, name, generation, branch, dna_haploid, dna_diploid, dna_mito FROM members WHERE id != ? AND (dna_haploid != '' OR dna_diploid != '' OR dna_mito != '')`, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	for rows.Next() {
		var mid int64
		var mname, mbranch, mhap, mdip, mmito string
		var mgen int
		if err := rows.Scan(&mid, &mname, &mgen, &mbranch, &mhap, &mdip, &mmito); err != nil {
			continue
		}
		mhap = DecryptValue(mhap, h.EncKey)
		mdip = DecryptValue(mdip, h.EncKey)
		mmito = DecryptValue(mmito, h.EncKey)

		appendDNAMatch(&matches, "haploid", "父系同源", hap, mhap, mid, mname, mgen, mbranch)
		appendDNAMatch(&matches, "mito", "母系同源", mito, mmito, mid, mname, mgen, mbranch)
		appendDNAMatch(&matches, "diploid", "常染色体匹配", dip, mdip, mid, mname, mgen, mbranch)
	}

	// 按成员聚合「亲缘可能性评分」降序排序（同父系完全一致最可能 → 排最前）
	memberScore := map[int64]int{}
	for _, m := range matches {
		memberScore[m.MemberID] += m.Score
	}
	sort.SliceStable(matches, func(i, j int) bool {
		si, sj := memberScore[matches[i].MemberID], memberScore[matches[j].MemberID]
		if si != sj {
			return si > sj
		}
		if matches[i].Generation != matches[j].Generation {
			return matches[i].Generation < matches[j].Generation
		}
		return matches[i].MemberID < matches[j].MemberID
	})

	c.JSON(http.StatusOK, gin.H{"target": target, "matches": matches, "total": len(matches)})
}

// dnaMatchScore 返回某维度某匹配类型的评分（exact > lineage > prefix）
func dnaMatchScore(field, matchType string) int {
	switch field {
	case "haploid":
		switch matchType {
		case "exact":
			return 6
		case "lineage":
			return 4
		case "prefix":
			return 3
		}
	case "mito":
		switch matchType {
		case "exact":
			return 4
		case "lineage":
			return 3
		case "prefix":
			return 2
		}
	case "diploid":
		switch matchType {
		case "exact":
			return 2
		case "prefix":
			return 1
		}
	}
	return 0
}

// appendDNAMatch 比对单个 DNA 维度（haploid/mito/diploid），命中则按精确/同支系/前缀追加结果
func appendDNAMatch(matches *[]DNAMatchItem, field, relation, qcode, mcode string, mid int64, mname string, mgen int, mbranch string) {
	if qcode == "" || mcode == "" {
		return
	}
	// 规范化后比较：使别名（如 M122）与规范名（O-M122）能精确命中
	q := canonicalHaplo(field, qcode)
	m := canonicalHaplo(field, mcode)
	if q == m {
		*matches = append(*matches, DNAMatchItem{mid, mname, mgen, mbranch, field, "exact", mcode, relation, dnaMatchScore(field, "exact")})
	} else if field != "diploid" && haploLineage(field, q, m) {
		*matches = append(*matches, DNAMatchItem{mid, mname, mgen, mbranch, field, "lineage", mcode, relation, dnaMatchScore(field, "lineage")})
	} else if dnaPrefixMatch(q, m) {
		*matches = append(*matches, DNAMatchItem{mid, mname, mgen, mbranch, field, "prefix", mcode, relation, dnaMatchScore(field, "prefix")})
	}
}

// dnaPrefixMatch 判断两个编码是否有前缀关系（较短者需 >= 2 字符，避免单字符过度匹配）
func dnaPrefixMatch(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	if len(a) > len(b) {
		if len(b) < 2 {
			return false
		}
		return strings.HasPrefix(a, b)
	}
	if len(a) < 2 {
		return false
	}
	return strings.HasPrefix(b, a)
}

// DNAMatchExternal 外部认亲比对：手动输入一段 DNA 编码（父系/母系/二倍体任意组合），与全库成员匹配
// POST /members/dna-match-external
// 场景：外地人 / 被拐卖者回来认亲，或输入他处检测报告编码来比对族内亲属（不依赖任何成员已录入字段）
func (h *MemberHandler) DNAMatchExternal(c *gin.Context) {
	var req struct {
		Haploid string `json:"haploid"` // 父系 Y-DNA 单倍群
		Mito    string `json:"mito"`    // 母系线粒体单倍群
		Diploid string `json:"diploid"` // 二倍体 STR 位点串
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	hap := strings.TrimSpace(req.Haploid)
	mito := strings.TrimSpace(req.Mito)
	dip := strings.TrimSpace(req.Diploid)

	if hap == "" && mito == "" && dip == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请至少输入一项 DNA 编码（父系 / 母系 / 二倍体）"})
		return
	}

	rows, err := h.DB.Query(`SELECT id, name, generation, branch, dna_haploid, dna_diploid, dna_mito FROM members WHERE (dna_haploid != '' OR dna_diploid != '' OR dna_mito != '')`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	matches := []DNAMatchItem{}
	for rows.Next() {
		var mid int64
		var mname, mbranch, mhap, mdip, mmito string
		var mgen int
		if err := rows.Scan(&mid, &mname, &mgen, &mbranch, &mhap, &mdip, &mmito); err != nil {
			continue
		}
		mhap = DecryptValue(mhap, h.EncKey)
		mdip = DecryptValue(mdip, h.EncKey)
		mmito = DecryptValue(mmito, h.EncKey)

		appendDNAMatch(&matches, "haploid", "父系同源", hap, mhap, mid, mname, mgen, mbranch)
		appendDNAMatch(&matches, "mito", "母系同源", mito, mmito, mid, mname, mgen, mbranch)
		appendDNAMatch(&matches, "diploid", "常染色体匹配", dip, mdip, mid, mname, mgen, mbranch)
	}

	// 按成员聚合「亲缘可能性评分」降序排序（同父系完全一致最可能 → 排最前）
	memberScore := map[int64]int{}
	for _, m := range matches {
		memberScore[m.MemberID] += m.Score
	}
	sort.SliceStable(matches, func(i, j int) bool {
		si, sj := memberScore[matches[i].MemberID], memberScore[matches[j].MemberID]
		if si != sj {
			return si > sj
		}
		if matches[i].Generation != matches[j].Generation {
			return matches[i].Generation < matches[j].Generation
		}
		return matches[i].MemberID < matches[j].MemberID
	})

	c.JSON(http.StatusOK, gin.H{
		"query":   gin.H{"haploid": hap, "mito": mito, "diploid": dip},
		"matches": matches,
		"total":   len(matches),
	})
}
