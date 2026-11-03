package handler

import "testing"

// 排行解析：阿拉伯数字、行N、中文数字、长子/次子 都要认。
// 曾经纯数字（1/2/3）返回 0 → 被当成"没排行"，谱书里排序退回出生日期/id，表现为"写数字无效"。
func TestRankNumFor(t *testing.T) {
	cases := map[string]int{
		"1":   1,
		"3":   3,
		"12":  12,
		"行3":  3,
		"行十二": 12,
		"十二":  12,
		"十五":  15,
		"二十":  20,
		"长":   1,
		"長":   1,
		"长子":  1,
		"次":   2,
		"次子":  2,
		"三":   3,
		"三子":  3,
		"幼":   99,
		"末":   99,
		"":    0,
		"无":   0,
	}
	for in, want := range cases {
		if got := rankNumFor(in); got != want {
			t.Errorf("rankNumFor(%q) = %d, want %d", in, got, want)
		}
	}
}

// 兄弟排序：按排行序数升序；没排行的排最后（其后仍按出生、id 兜底）
func TestSortChildrenByRank(t *testing.T) {
	n1 := &chuiZhuNode{id: 11, rank: "3"}
	n2 := &chuiZhuNode{id: 12, rank: "1"}
	n3 := &chuiZhuNode{id: 13, rank: "2"}
	n4 := &chuiZhuNode{id: 14, rank: ""}
	nodes := []*chuiZhuNode{n1, n2, n3, n4}
	sortChildren(nodes)

	want := []int64{12, 13, 11, 14}
	for i, w := range want {
		if nodes[i].id != w {
			got := make([]int64, 0, len(nodes))
			for _, n := range nodes {
				got = append(got, n.id)
			}
			t.Fatalf("排序结果 %v, want %v", got, want)
		}
	}
}

// 中文/传统写法与阿拉伯数字混用，也应得到同一顺序
func TestSortChildrenMixedWriting(t *testing.T) {
	a := &chuiZhuNode{id: 1, rank: "长子"}
	b := &chuiZhuNode{id: 2, rank: "2"}
	c := &chuiZhuNode{id: 3, rank: "三"}
	nodes := []*chuiZhuNode{c, b, a}
	sortChildren(nodes)
	if nodes[0].id != 1 || nodes[1].id != 2 || nodes[2].id != 3 {
		t.Fatalf("混用写法排序错误: ids = %d,%d,%d", nodes[0].id, nodes[1].id, nodes[2].id)
	}
}
