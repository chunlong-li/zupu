package handler

import (
	"testing"
)

func TestRankNumber(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"", 0},
		{"行一", 1},
		{"行二", 2},
		{"行十", 10},
		{"行十一", 11},
		{"行十五", 15},
		{"行二十", 20},
		{"行廿三", 23},
		{"行12", 12},
		{"12", 12},
		{"长子", 1},
		{"長子", 1},
		{"次子", 2},
		{"独子", 0},
		{"unknown", 0},
	}
	for _, c := range cases {
		if got := rankNumber(c.in); got != c.want {
			t.Errorf("rankNumber(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestBirthSortKey(t *testing.T) {
	cases := []struct {
		in   string
		want int64
	}{
		{"", 0},
		{"1700", 17000000},
		{"1732-12-11", 17321211},
		{"1732.04.09", 17320409},
		{"清康熙三十九年", 0},
		{"约1700年", 17000000},
	}
	for _, c := range cases {
		if got := birthSortKey(c.in); got != c.want {
			t.Errorf("birthSortKey(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

// 覆盖任务要求的两条用例 + 配偶随夫 + 未关联者最后。
func TestSortBiographiesWithKeys(t *testing.T) {
	list := []Biography{
		{ID: 1, Name: "李渊", SortYear: 1700},                     // 第1世 行一
		{ID: 2, Name: "李少渊", SortYear: 1732},                    // 第2世 行一
		{ID: 3, Name: "李言渊", SortYear: 0},                       // 第3世 行一
		{ID: 4, Name: "李言澤", SortYear: 0},                       // 第3世 行二
		{ID: 5, Name: "李言嵐", SortYear: 0},                       // 第3世 行三
		{ID: 6, Name: "李少岚", SortYear: 2099},                    // 第2世 行三，sort_year 很大
		{ID: 7, Name: "李少泽", SortYear: 0},                       // 第2世 行二
		{ID: 8, Name: "陈秀兰", SortYear: 0, PersonType: "spouse"}, // 李渊之配
		{ID: 9, Name: "外族友人", SortYear: 1600, PersonType: "other"},
	}
	keys := map[int64]bioSortKey{
		1: {linked: true, generation: 1, hasRank: true, rank: 1, birth: 17000000},
		2: {linked: true, generation: 2, hasRank: true, rank: 1, birth: 17321211},
		3: {linked: true, generation: 3, hasRank: true, rank: 1, birth: 17480314},
		4: {linked: true, generation: 3, hasRank: true, rank: 2, birth: 17530128},
		5: {linked: true, generation: 3, hasRank: true, rank: 3, birth: 17550427},
		6: {linked: true, generation: 2, hasRank: true, rank: 3, birth: 17341204},
		7: {linked: true, generation: 2, hasRank: true, rank: 2, birth: 17320409},
		// 配偶：跟随丈夫（李渊）第1世 行一
		8: {linked: true, isSpouse: true, generation: 1, hasRank: true, rank: 1},
		// 9：无键 → 未关联
	}
	sortBiographiesWithKeys(list, keys)

	want := []int64{1, 8, 2, 7, 6, 3, 4, 5, 9}
	for i, id := range want {
		if list[i].ID != id {
			got := make([]int64, len(list))
			for k := range list {
				got[k] = list[k].ID
			}
			t.Fatalf("order = %v, want %v", got, want)
		}
	}
}

// sort_year 只能在「同世代 + 同排行」内起作用。
func TestSortYearOnlyWithinSameGenerationAndRank(t *testing.T) {
	list := []Biography{
		{ID: 1, SortYear: 2099}, // 第1世 行一，sort_year 很大
		{ID: 2, SortYear: 0},    // 第2世 行一
	}
	keys := map[int64]bioSortKey{
		1: {linked: true, generation: 1, hasRank: true, rank: 1, birth: 17000000},
		2: {linked: true, generation: 2, hasRank: true, rank: 1, birth: 17300000},
	}
	sortBiographiesWithKeys(list, keys)
	if list[0].ID != 1 {
		t.Fatalf("低世代（sort_year 很大）应排在前，得到 %d,%d", list[0].ID, list[1].ID)
	}

	// 同世代同排行：sort_year 有值者覆盖生年比较
	list2 := []Biography{
		{ID: 3, SortYear: 1800},
		{ID: 4, SortYear: 0},
	}
	keys2 := map[int64]bioSortKey{
		3: {linked: true, generation: 4, hasRank: true, rank: 1, birth: 17500000},
		4: {linked: true, generation: 4, hasRank: true, rank: 1, birth: 17400000},
	}
	sortBiographiesWithKeys(list2, keys2)
	if list2[0].ID != 4 || list2[1].ID != 3 {
		t.Fatalf("同世同排内应按 sort_year/生年次序，得到 %d,%d", list2[0].ID, list2[1].ID)
	}
}

// 没有排行的同辈改用生年；没有生年再按 id。
func TestSortWithoutRankFallsBackToBirthThenID(t *testing.T) {
	list := []Biography{
		{ID: 1}, {ID: 2}, {ID: 3},
	}
	keys := map[int64]bioSortKey{
		1: {linked: true, generation: 2, birth: 17400000},
		2: {linked: true, generation: 2, birth: 17300000},
		3: {linked: true, generation: 2, birth: 17500000},
	}
	sortBiographiesWithKeys(list, keys)
	if list[0].ID != 2 || list[1].ID != 1 || list[2].ID != 3 {
		t.Fatalf("无排行应按生年排序，得到 %d,%d,%d", list[0].ID, list[1].ID, list[2].ID)
	}

	list2 := []Biography{{ID: 7}, {ID: 5}, {ID: 6}}
	keys2 := map[int64]bioSortKey{
		5: {linked: true, generation: 2},
		6: {linked: true, generation: 2},
		7: {linked: true, generation: 2},
	}
	sortBiographiesWithKeys(list2, keys2)
	if list2[0].ID != 5 || list2[1].ID != 6 || list2[2].ID != 7 {
		t.Fatalf("无排行无生年应按 id 兜底，得到 %d,%d,%d", list2[0].ID, list2[1].ID, list2[2].ID)
	}
}
