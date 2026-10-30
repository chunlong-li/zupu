package handler

import (
	"database/sql"
)

// ============================================================================
// 亲属距离计算（BFS 双向搜索）
// 图结构：父子边（father_id, mother_id）+ 配偶边（spouses 表）
// 距离 = 最短路径边数
//   - 自身 → 自身 = 0
//   - 自身 → 父/母/子女/配偶 = 1
//   - 自身 → 兄弟 = 2 (自身→父→兄弟)
//   - 自身 → 祖父 = 2
//   - 自身 → 堂兄弟 = 4
// ============================================================================

// RelationDistance 计算两个成员之间的亲属距离
// 返回 -1 表示不可达（无亲属关系或超出搜索范围）
func RelationDistance(db *sql.DB, fromID, toID int64) int {
	if fromID == toID {
		return 0
	}

	const maxDepth = 10 // 最大搜索深度（防止全库扫描）

	// 双向 BFS：用 map[int64]int 存访问过的节点深度
	visitedF := map[int64]int{fromID: 0}
	visitedT := map[int64]int{toID: 0}

	// 使用队列，元素: (id, depth)
	type node struct {
		id    int64
		depth int
	}
	frontF := []node{{fromID, 0}}
	frontT := []node{{toID, 0}}

	for len(frontF) > 0 || len(frontT) > 0 {
		// 优先扩展较小的 frontier（双向搜索优化）
		if len(frontF) > 0 && (len(frontF) <= len(frontT) || len(frontT) == 0) {
			cur := frontF[0]
			frontF = frontF[1:]
			if cur.depth >= maxDepth {
				continue
			}
			for _, neighbor := range getNeighbors(db, cur.id) {
				if _, seen := visitedF[neighbor]; seen {
					continue
				}
				visitedF[neighbor] = cur.depth + 1
				if d, ok := visitedT[neighbor]; ok {
					return cur.depth + 1 + d // 汇合
				}
				frontF = append(frontF, node{neighbor, cur.depth + 1})
			}
		} else if len(frontT) > 0 {
			cur := frontT[0]
			frontT = frontT[1:]
			if cur.depth >= maxDepth {
				continue
			}
			for _, neighbor := range getNeighbors(db, cur.id) {
				if _, seen := visitedT[neighbor]; seen {
					continue
				}
				visitedT[neighbor] = cur.depth + 1
				if d, ok := visitedF[neighbor]; ok {
					return cur.depth + 1 + d
				}
				frontT = append(frontT, node{neighbor, cur.depth + 1})
			}
		}
	}

	return -1 // 不可达
}

// getNeighbors 获取成员的所有相邻节点（父子 + 配偶）
func getNeighbors(db *sql.DB, id int64) []int64 {
	neighbors := make([]int64, 0, 6)
	seen := map[int64]bool{}

	add := func(nid int64) {
		if nid > 0 && !seen[nid] {
			seen[nid] = true
			neighbors = append(neighbors, nid)
		}
	}

	// 获取当前成员的 father_id, mother_id
	var fatherID, motherID sql.NullInt64
	db.QueryRow(`SELECT father_id, mother_id FROM members WHERE id = ?`, id).Scan(&fatherID, &motherID)
	if fatherID.Valid {
		add(fatherID.Int64)
	}
	if motherID.Valid {
		add(motherID.Int64)
	}

	// 获取子女
	childRows, err := db.Query(`SELECT id FROM members WHERE father_id = ? OR mother_id = ?`, id, id)
	if err == nil {
		for childRows.Next() {
			var cid int64
			childRows.Scan(&cid)
			add(cid)
		}
		childRows.Close()
	}

	// 获取配偶（spouses.member_id 指向本人 → 配偶本人如果是成员，则在 members 表中）
	// 同时处理反向：本人作为配偶出现在其他成员名下
	spRows, err := db.Query(`SELECT id, member_id FROM spouses WHERE id IN (
		SELECT id FROM spouses WHERE member_id = ?
	)`, id)
	if err == nil {
		for spRows.Next() {
			var sid, mid int64
			spRows.Scan(&sid, &mid)
			add(mid)
		}
		spRows.Close()
	}

	return neighbors
}

// FindMemberByName 根据姓名查找成员 ID（用于将登录用户匹配到族谱成员）
func FindMemberByName(db *sql.DB, name string) int64 {
	var id int64
	err := db.QueryRow(`SELECT id FROM members WHERE name = ? LIMIT 1`, name).Scan(&id)
	if err != nil {
		return 0
	}
	return id
}

// ============================================================================
// 五服判断（古代丧服制度，2026-09-13 用户定）
// 五服以内 = 两人同高祖，即存在共同祖先，且该祖先到两人的距离都 ≤ 4
// 覆盖：直系上4代(高祖)/下4代(玄孙) + 旁系到族兄弟(同高祖)
// 实现：各自往上收集 4 代祖先（含自己），两集合有交集即为五服以内
// ============================================================================

// collectAncestors 收集某成员往上 depth 代的祖先集合（含自己）
// 走 father_id + mother_id（父母都算，符合现代族谱）
func collectAncestors(db *sql.DB, id int64, depth int) map[int64]bool {
	result := map[int64]bool{id: true}
	current := map[int64]bool{id: true}
	for d := 0; d < depth; d++ {
		next := map[int64]bool{}
		for cid := range current {
			var fatherID, motherID sql.NullInt64
			db.QueryRow(`SELECT father_id, mother_id FROM members WHERE id = ?`, cid).Scan(&fatherID, &motherID)
			if fatherID.Valid && fatherID.Int64 > 0 {
				next[fatherID.Int64] = true
				result[fatherID.Int64] = true
			}
			if motherID.Valid && motherID.Int64 > 0 {
				next[motherID.Int64] = true
				result[motherID.Int64] = true
			}
		}
		current = next
		if len(current) == 0 {
			break
		}
	}
	return result
}

// WithinFiveFu 判断两人是否在五服以内（同高祖）
func WithinFiveFu(db *sql.DB, aID, bID int64) bool {
	if aID == bID {
		return true
	}
	ancestorsA := collectAncestors(db, aID, 4)
	ancestorsB := collectAncestors(db, bID, 4)
	for id := range ancestorsA {
		if ancestorsB[id] {
			return true
		}
	}
	return false
}
