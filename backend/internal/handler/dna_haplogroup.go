package handler

import (
	"strings"
	"unicode"
)

// haploParentY Y-DNA（父系）单倍群树：child -> parent（ISOGG 字母+数字命名）。
// 由 dna_haplogroup_y.go 从 ydna.txt 加载。
var haploParentY = map[string]string{}

// haploParentMT mtDNA（母系）单倍群树：child -> parent（PhyloTree 字母命名）。
// 由 dna_haplogroup_mt.go 从 mt_dna.txt 加载。
var haploParentMT = map[string]string{}

// yAlias Y-DNA 位点别名 -> ISOGG 名（如 "O-M175" -> "O"、"M175" -> "O"）。
// 由 dna_haplogroup_y.go 从 ydna_alias.txt 加载。
var yAlias = map[string]string{}

// canonicalHaplo 规范化单倍群编码。
// field: haploid（父系/Y-DNA）| mito（母系/mtDNA）；diploid 不参与单倍群比对。
func canonicalHaplo(field, s string) string {
	s = strings.TrimSpace(s)
	if field == "haploid" {
		up := strings.ToUpper(s)
		// 位点别名（O-M175 -> O、M175 -> O）
		if c, ok := yAlias[up]; ok {
			return c
		}
		if strings.Contains(up, "-") {
			return up // 未收录的位点命名，保留大写
		}
		return titleCase(up) // ISOGG 字母+数字命名：O2a1b
	}
	// mtDNA：首字母大写，其余小写（子支大小写敏感，如 D4a/M7a/B4a）
	return titleCase(s)
}

func titleCase(s string) string {
	var b strings.Builder
	for i, r := range s {
		if i == 0 {
			b.WriteRune(unicode.ToUpper(r))
		} else {
			b.WriteRune(unicode.ToLower(r))
		}
	}
	return b.String()
}

// haploParentFor 返回某维度对应的谱系树。
func haploParentFor(field string) map[string]string {
	if field == "haploid" {
		return haploParentY
	}
	return haploParentMT
}

// isHaploAncestor 判断 anc 是否为 node 的祖先（沿给定树向上遍历）。
func isHaploAncestor(tree map[string]string, anc, node string) bool {
	seen := map[string]bool{}
	for cur := node; cur != ""; cur = tree[cur] {
		if seen[cur] {
			break // 防环
		}
		seen[cur] = true
		if cur == anc {
			return true
		}
	}
	return false
}

// haploLineage 判断 a、b 两编码在对应谱系树上是否有「祖先-后代」关系（任一方向，不含相等）。
// field 决定用 Y-DNA 还是 mtDNA 树。
func haploLineage(field, a, b string) bool {
	if field != "haploid" && field != "mito" {
		return false
	}
	a = canonicalHaplo(field, a)
	b = canonicalHaplo(field, b)
	if a == "" || b == "" || a == b {
		return false
	}
	tree := haploParentFor(field)
	return isHaploAncestor(tree, a, b) || isHaploAncestor(tree, b, a)
}
