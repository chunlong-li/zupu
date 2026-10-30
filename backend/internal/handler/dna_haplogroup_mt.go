package handler

import (
	_ "embed"
	"strings"
)

//go:embed mt_dna.txt
var mtDNAData string

// init 解析嵌入的 PhyloTree mtDNA 完整树（每行 child<TAB>parent），直接写入 haploParent。
// 数据源：https://www.phylotree.org/ （PhyloTree mtDNA tree Build 17, 2016，约 1.28 万节点）。
// 由 tools/import_haplogroup.py 生成，更新时替换 mt_dna.txt 重跑即可。
func init() {
	for _, line := range strings.Split(mtDNAData, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) == 2 {
			haploParentMT[parts[0]] = parts[1]
		}
	}
}
