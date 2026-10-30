package handler

import (
	_ "embed"
	"strings"
)

//go:embed ydna.txt
var ydnaData string

//go:embed ydna_alias.txt
var ydnaAliasData string

// init 解析嵌入的 ISOGG Y-DNA 完整树（每行 child<TAB>parent）与位点别名（位点<TAB>单倍群名）。
// 数据源：ISOGG Y-DNA Haplogroup Tree 2019-2020，约 9.6 万节点，由 tools/import_haplogroup.py 生成。
func init() {
	for _, line := range strings.Split(ydnaData, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) == 2 {
			haploParentY[parts[0]] = parts[1]
		}
	}
	for _, line := range strings.Split(ydnaAliasData, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) == 2 {
			yAlias[parts[0]] = parts[1]
		}
	}
}
