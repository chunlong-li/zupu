package handler

import (
	"database/sql"
	"sort"
	"strconv"
	"strings"
)

// ============================================================================
// 亲属称谓扩展规则（配合 relation.go 的 Relations 使用）
//
// 思路：一次载入人员索引（members + spouses），在内存里按「有界层数」收集血缘/
// 姻缘关系（直系上 4 代、下 4 代、旁系 2 层），最后统一去重与排序。
// 好处：不做逐人查询（无 N+1）、不做递归（不爆栈）、输出可复现。
//
// 关系优先级（数字越小越近；同一人重复出现时保留更近的一条）：
//   0 直系（父母/祖辈/子孙）与配偶
//   1 旁系长辈与平辈（叔伯/姑姑/舅舅/姨、兄弟姐妹、堂表兄弟姐妹、堂表伯叔姑）
//   2 远亲（侄辈、侄孙辈）
//   3 姻亲（伯母婶婶/姑父/舅母/姨父/嫂子弟媳/姐夫妹夫/儿媳女婿/岳父母/公婆）
// ============================================================================

const (
	kinPriDirect = 0 // 直系
	kinPriPeer   = 1 // 旁系长辈/平辈
	kinPriFar    = 2 // 远亲
	kinPriInLaw  = 3 // 姻亲
	kinPriSpouse = 0 // 配偶（与直系同级，避免交叉婚配时「配偶」条目被表亲挤掉）
)

// kinNode 亲属计算用的人员节点
// members 与 spouses 两表编号各自独立：族内成员填 id，配偶条目填 spouseID
type kinNode struct {
	id             int64 // members.id；来自 spouses 表时为 0
	spouseID       int64 // spouses.id；族内成员时为 0
	surname        string
	givenName      string
	name           string
	gender         int // 1=男 2=女
	fatherID       int64
	motherID       int64
	motherSpouseID int64
	generation     int
	birthYear      int
	recordType     string // 内纪/外纪
	spType         string // 配偶类型：正配/继配/侧室
	spouses        []*kinNode
}

// key 去重键：族内成员按 member_id，配偶按 spouse_id
func (n *kinNode) key() string {
	if n == nil {
		return ""
	}
	if n.id > 0 {
		return "m:" + strconv.FormatInt(n.id, 10)
	}
	return "s:" + strconv.FormatInt(n.spouseID, 10)
}

// kinIndex 亲属索引（一次查询载入，供整个请求复用）
type kinIndex struct {
	members    map[int64]*kinNode
	spouses    map[int64]*kinNode
	children   map[int64][]*kinNode // 键为 members.id：father_id 或 mother_id 命中
	spouseKids map[int64][]*kinNode // 键为 spouses.id：mother_spouse_id 命中
}

// buildKinIndex 载入全部成员与配偶（单次查询，避免 N+1）
func buildKinIndex(db *sql.DB) *kinIndex {
	idx := &kinIndex{
		members:    map[int64]*kinNode{},
		spouses:    map[int64]*kinNode{},
		children:   map[int64][]*kinNode{},
		spouseKids: map[int64][]*kinNode{},
	}

	// 成员：含性别、父母、世代、内外纪、出生公历年
	rows, err := db.Query(`SELECT id, surname, given_name, name, gender, father_id, mother_id, mother_spouse_id,
		generation, record_type, birth_gregorian FROM members`)
	if err == nil {
		for rows.Next() {
			n := &kinNode{}
			var birth string
			if err := rows.Scan(&n.id, &n.surname, &n.givenName, &n.name, &n.gender, &n.fatherID, &n.motherID,
				&n.motherSpouseID, &n.generation, &n.recordType, &birth); err != nil {
				continue
			}
			n.birthYear = kinBirthYear(birth)
			if n.recordType == "" {
				n.recordType = "内纪"
			}
			idx.members[n.id] = n
		}
		rows.Close()
	}

	// 配偶：spouses 表无性别列，按配偶（member）的相反性别推断；世代/内外纪随配偶
	srows, err := db.Query(`SELECT id, member_id, surname, given_name, name, type, birth_gregorian FROM spouses`)
	if err == nil {
		for srows.Next() {
			s := &kinNode{}
			var memberID int64
			var birth string
			if err := srows.Scan(&s.spouseID, &memberID, &s.surname, &s.givenName, &s.name, &s.spType, &birth); err != nil {
				continue
			}
			s.birthYear = kinBirthYear(birth)
			idx.spouses[s.spouseID] = s
			if p := idx.members[memberID]; p != nil {
				s.gender = kinOppositeGender(p.gender)
				s.generation = p.generation
				s.recordType = p.recordType
				p.spouses = append(p.spouses, s)
			}
		}
		srows.Close()
	}

	// 父母 → 子女 反向索引（father_id 或 mother_id 命中；母亲为配偶条目时走 mother_spouse_id）
	ids := make([]int64, 0, len(idx.members))
	for id := range idx.members {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] }) // 固定遍历顺序，保证结果可复现
	for _, id := range ids {
		c := idx.members[id]
		if c.fatherID > 0 {
			idx.children[c.fatherID] = append(idx.children[c.fatherID], c)
		}
		if c.motherID > 0 {
			idx.children[c.motherID] = append(idx.children[c.motherID], c)
		}
		if c.motherSpouseID > 0 {
			idx.spouseKids[c.motherSpouseID] = append(idx.spouseKids[c.motherSpouseID], c)
		}
	}
	return idx
}

// kinBirthYear 取出生公历前 4 位数字作为出生年，取不到返回 0
func kinBirthYear(s string) int {
	s = strings.TrimSpace(s)
	if len(s) < 4 {
		return 0
	}
	y, err := strconv.Atoi(s[:4])
	if err != nil || y < 1 || y > 9999 {
		return 0
	}
	return y
}

// kinOppositeGender 由配偶性别推断本人性别（spouses 表没有性别列）
func kinOppositeGender(g int) int {
	if g == 1 {
		return 2
	}
	if g == 2 {
		return 1
	}
	return 2
}

// fatherOf / motherOf 取父亲、母亲（母亲优先取 mother_spouse_id 登记的配偶）
func (idx *kinIndex) fatherOf(n *kinNode) *kinNode {
	if n == nil || n.fatherID <= 0 {
		return nil
	}
	return idx.members[n.fatherID]
}

func (idx *kinIndex) motherOf(n *kinNode) *kinNode {
	if n == nil {
		return nil
	}
	if n.motherSpouseID > 0 {
		if m := idx.spouses[n.motherSpouseID]; m != nil {
			return m
		}
	}
	if n.motherID > 0 {
		return idx.members[n.motherID]
	}
	return nil
}

// parentsOf 父母（父在前，母在后）
func (idx *kinIndex) parentsOf(n *kinNode) []*kinNode {
	out := make([]*kinNode, 0, 2)
	if f := idx.fatherOf(n); f != nil {
		out = append(out, f)
	}
	if m := idx.motherOf(n); m != nil {
		out = append(out, m)
	}
	return out
}

// childrenOf 某人的子女：族内成员查 father_id/mother_id，配偶条目查 mother_spouse_id
func (idx *kinIndex) childrenOf(n *kinNode) []*kinNode {
	if n == nil {
		return nil
	}
	if n.id > 0 {
		return idx.children[n.id]
	}
	return idx.spouseKids[n.spouseID]
}

// siblingsOf 兄弟姐妹：同父或同母（家长可能是族内成员，也可能是配偶条目）
func (idx *kinIndex) siblingsOf(n *kinNode) []*kinNode {
	out := make([]*kinNode, 0, 4)
	seen := map[string]bool{}
	for _, p := range idx.parentsOf(n) {
		for _, c := range idx.childrenOf(p) {
			if c.key() == n.key() || seen[c.key()] {
				continue
			}
			seen[c.key()] = true
			out = append(out, c)
		}
	}
	return out
}

// spouseMembers 本人配偶中被登记为族内成员的情况（用于岳父/岳母、公公/婆婆）：
// 男性取其子女的母亲，女性取其子女的父亲
func (idx *kinIndex) spouseMembers(self *kinNode) []*kinNode {
	out := make([]*kinNode, 0, 2)
	seen := map[int64]bool{}
	for _, c := range idx.childrenOf(self) {
		pid := c.motherID
		if self.gender == 2 {
			pid = c.fatherID
		}
		if pid <= 0 || pid == self.id || seen[pid] {
			continue
		}
		seen[pid] = true
		if n := idx.members[pid]; n != nil {
			out = append(out, n)
		}
	}
	return out
}

// kinCrossRecord 是否跨内外纪：外纪世代另起，辈分不可比 → 旁系不计算
func kinCrossRecord(a, b *kinNode) bool {
	if a == nil || b == nil {
		return false
	}
	return a.recordType != b.recordType
}

// kinAgeOrder a 相对 b 的长幼：-1 年长、1 年幼、0 无法判断
func kinAgeOrder(a, b *kinNode) int {
	if a == nil || b == nil || a.birthYear == 0 || b.birthYear == 0 {
		return 0
	}
	if a.birthYear < b.birthYear {
		return -1
	}
	if a.birthYear > b.birthYear {
		return 1
	}
	return 0
}

// kinPeerTerms 同辈称谓：年长/年幼/无法判断
type kinPeerTerms struct {
	older   string
	younger string
	unknown string
}

// 兄弟姐妹 / 堂 / 表 的同辈称谓表（按性别取词）
var (
	kinSiblingTerms = map[int]kinPeerTerms{
		1: {"兄", "弟", "兄弟"},
		2: {"姐", "妹", "姐妹"},
	}
	kinTangTerms = map[int]kinPeerTerms{
		1: {"堂兄", "堂弟", "堂兄弟"},
		2: {"堂姐", "堂妹", "堂姐妹"},
	}
	kinBiaoTerms = map[int]kinPeerTerms{
		1: {"表哥", "表弟", "表兄弟"},
		2: {"表姐", "表妹", "表姐妹"},
	}
)

// kinPeerTitle 同辈称谓：按长幼取词，判断不了用中性叫法
func kinPeerTitle(m map[int]kinPeerTerms, gender, age int) string {
	t, ok := m[gender]
	if !ok {
		t = m[1]
	}
	switch age {
	case -1:
		return t.older
	case 1:
		return t.younger
	}
	return t.unknown
}

// kinElderTitle 长辈称谓：与参照人（通常是父亲）比长幼
func kinElderTitle(rel, ref *kinNode, older, younger, unknown string) string {
	switch kinAgeOrder(rel, ref) {
	case -1:
		return older
	case 1:
		return younger
	}
	return unknown
}

// kinUpDepth 向上世代差校准：返回 self 到 node 的代数（1..4），无法校准时用 fallback
func kinUpDepth(self, node *kinNode, fallback int) int {
	if self.generation > 0 && node != nil && node.generation > 0 {
		if d := self.generation - node.generation; d >= 1 && d <= 4 {
			return d
		}
	}
	return fallback
}

// 直系祖先称谓表（按校准后的代数取词）
var (
	kinPaternalAncestorTerms = []struct{ group, male, female string }{
		{"祖父母", "祖父", "祖母"},
		{"曾祖父母", "曾祖父", "曾祖母"},
		{"高祖父", "高祖父", "高祖母"},
	}
	kinMaternalAncestorTerms = []struct{ group, male, female string }{
		{"外祖父母", "外祖父", "外祖母"},
		{"外曾祖父母", "外曾祖父", "外曾祖母"},
	}
)

// addPaternalAncestor 父系祖先（depth：2=祖辈 3=曾祖辈 4=高祖辈）
func addPaternalAncestor(self *kinNode, kc *kinCollector, n *kinNode, depth int, male bool) {
	if n == nil || kinCrossRecord(self, n) {
		return
	}
	d := kinUpDepth(self, n, depth)
	if d < 2 || d > 4 {
		d = depth
	}
	t := kinPaternalAncestorTerms[d-2]
	if male {
		kc.add(n, t.group, t.male, kinPriDirect)
	} else {
		kc.add(n, t.group, t.female, kinPriDirect)
	}
}

// addMaternalAncestor 母系祖先（depth：2=外祖辈 3=外曾祖辈）
func addMaternalAncestor(self *kinNode, kc *kinCollector, n *kinNode, depth int, male bool) {
	if n == nil || kinCrossRecord(self, n) {
		return
	}
	d := kinUpDepth(self, n, depth)
	if d < 2 || d > 3 {
		d = depth
	}
	t := kinMaternalAncestorTerms[d-2]
	if male {
		kc.add(n, t.group, t.male, kinPriDirect)
	} else {
		kc.add(n, t.group, t.female, kinPriDirect)
	}
}

// kinDescendantTitle 直系后代称谓：优先用 generation 差值校准，退回遍历层级
func kinDescendantTitle(self, node *kinNode, depth int) (string, string) {
	d := depth
	if self.generation > 0 && node.generation > 0 {
		if gd := node.generation - self.generation; gd >= 1 && gd <= 4 {
			d = gd
		}
	}
	switch d {
	case 1:
		if node.gender == 2 {
			return "子女", "女儿"
		}
		return "子女", "儿子"
	case 2:
		if node.gender == 2 {
			return "孙子女", "孙女"
		}
		return "孙子女", "孙子"
	case 3:
		if node.gender == 2 {
			return "曾孙", "曾孙女"
		}
		return "曾孙", "曾孙"
	default:
		if node.gender == 2 {
			return "玄孙", "玄孙女"
		}
		return "玄孙", "玄孙"
	}
}

// kinEntry 一条亲属条目（去重前）
type kinEntry struct {
	group    string
	relation string
	node     *kinNode
	priority int
}

// kinCollector 亲属条目收集器
type kinCollector struct {
	entries []kinEntry
}

func (kc *kinCollector) add(n *kinNode, group, relation string, priority int) {
	if n == nil || group == "" {
		return
	}
	kc.entries = append(kc.entries, kinEntry{group: group, relation: relation, node: n, priority: priority})
}

// addBiaoCousin 表兄弟姐妹（姑表/舅表/姨表）并登记男性表兄弟（供表侄使用）
func (kc *kinCollector) addBiaoCousin(self, c *kinNode, group string, biaoBrothers *[]*kinNode) {
	if c == nil || c.key() == self.key() || kinCrossRecord(self, c) {
		return
	}
	kc.add(c, group, kinPeerTitle(kinBiaoTerms, c.gender, kinAgeOrder(c, self)), kinPriPeer)
	if c.gender == 1 {
		*biaoBrothers = append(*biaoBrothers, c)
	}
}

// addFatherCousin 父亲的表兄弟姐妹 → 表伯/表叔/表姑
func (kc *kinCollector) addFatherCousin(self, c, father *kinNode) {
	if c == nil || father == nil || c.key() == self.key() || c.key() == father.key() || kinCrossRecord(self, c) {
		return
	}
	if c.gender == 2 {
		kc.add(c, "表姑", "表姑", kinPriPeer)
		return
	}
	kc.add(c, "表伯叔", kinElderTitle(c, father, "表伯", "表叔", "表叔伯"), kinPriPeer)
}

// finish 去重（同一人保留最亲近一条）→ 排序 → 组装响应
func (kc *kinCollector) finish() map[string][]RelationInfo {
	best := map[string]kinEntry{}
	for _, e := range kc.entries {
		k := e.node.key()
		if old, ok := best[k]; !ok || kinEntryLess(e, old) {
			best[k] = e
		}
	}
	list := make([]kinEntry, 0, len(best))
	for _, e := range best {
		list = append(list, e)
	}
	sort.Slice(list, func(i, j int) bool { return kinEntryLess(list[i], list[j]) })

	result := map[string][]RelationInfo{}
	for _, e := range list {
		result[e.group] = append(result[e.group], RelationInfo{
			Relation:  e.relation,
			MemberID:  e.node.id,
			SpouseID:  e.node.spouseID,
			Surname:   e.node.surname,
			GivenName: e.node.givenName,
			Name:      e.node.name,
			Gender:    e.node.gender,
		})
	}
	return result
}

// kinEntryLess 排序/择优：优先级 → 组名 → 辈分 → 出生年 → 称谓 → 姓名 → 编号
// 排序键必须完全确定，否则不同请求的顺序会变（SQLite 返回无序）
func kinEntryLess(a, b kinEntry) bool {
	if a.priority != b.priority {
		return a.priority < b.priority
	}
	if a.group != b.group {
		return a.group < b.group
	}
	if a.node.generation != b.node.generation {
		if a.node.generation == 0 {
			return false
		}
		if b.node.generation == 0 {
			return true
		}
		return a.node.generation < b.node.generation
	}
	if a.node.birthYear != b.node.birthYear {
		if a.node.birthYear == 0 {
			return false
		}
		if b.node.birthYear == 0 {
			return true
		}
		return a.node.birthYear < b.node.birthYear
	}
	if a.relation != b.relation {
		return a.relation < b.relation
	}
	if a.node.name != b.node.name {
		return a.node.name < b.node.name
	}
	return a.node.key() < b.node.key()
}

// collectKin 按血缘/姻缘规则收集全部亲属条目（层数有界：上 4 下 4 旁系 2 层）
func collectKin(idx *kinIndex, self *kinNode) *kinCollector {
	kc := &kinCollector{}
	father := idx.fatherOf(self)
	mother := idx.motherOf(self)

	// ===== 直系上：父母 =====
	kc.add(father, "父母", "父亲", kinPriDirect)
	kc.add(mother, "父母", "母亲", kinPriDirect)

	// ===== 直系上：祖父母 → 曾祖父母 → 高祖父母（父系；用世代差校准称谓） =====
	up := father
	for depth := 2; depth <= 4 && up != nil; depth++ {
		f, m := idx.fatherOf(up), idx.motherOf(up)
		// 跨内外纪：直系父母已是另算，再往上跨纪世代不可比 → 跳过
		addPaternalAncestor(self, kc, f, depth, true)
		addPaternalAncestor(self, kc, m, depth, false)
		up = f
	}

	// ===== 直系上：外祖父母 → 外曾祖父母（母系；同样按世代差校准） =====
	if mother != nil {
		mf, mm := idx.fatherOf(mother), idx.motherOf(mother)
		addMaternalAncestor(self, kc, mf, 2, true)
		addMaternalAncestor(self, kc, mm, 2, false)
		if mf != nil {
			addMaternalAncestor(self, kc, idx.fatherOf(mf), 3, true)
			addMaternalAncestor(self, kc, idx.motherOf(mf), 3, false)
		}
	}

	// ===== 直系下：子女 → 孙子女 → 曾孙 → 玄孙（含儿媳/女婿） =====
	visited := map[string]bool{self.key(): true}
	cur := []*kinNode{self}
	for depth := 1; depth <= 4; depth++ {
		next := make([]*kinNode, 0, 4)
		for _, p := range cur {
			for _, ch := range idx.childrenOf(p) {
				if visited[ch.key()] {
					continue
				}
				visited[ch.key()] = true
				next = append(next, ch)
				// 子女（depth=1）不受内外纪限制；再往下跨纪则跳过
				if depth > 1 && kinCrossRecord(self, ch) {
					continue
				}
				group, rel := kinDescendantTitle(self, ch, depth)
				kc.add(ch, group, rel, kinPriDirect)
				// 子女之配偶：儿子之妻=儿媳，女儿之夫=女婿
				if depth == 1 {
					for _, sp := range ch.spouses {
						if ch.gender == 1 {
							kc.add(sp, "儿媳女婿", "儿媳", kinPriInLaw)
						} else if ch.gender == 2 {
							kc.add(sp, "儿媳女婿", "女婿", kinPriInLaw)
						}
					}
				}
			}
		}
		cur = next
	}

	// ===== 平辈：兄弟姐妹（同父或同母） =====
	var brothers, sisters []*kinNode
	for _, s := range idx.siblingsOf(self) {
		if kinCrossRecord(self, s) {
			continue
		}
		age := kinAgeOrder(s, self)
		if s.gender == 2 {
			kc.add(s, "姐妹", kinPeerTitle(kinSiblingTerms, 2, age), kinPriPeer)
			sisters = append(sisters, s)
		} else {
			kc.add(s, "兄弟", kinPeerTitle(kinSiblingTerms, 1, age), kinPriPeer)
			brothers = append(brothers, s)
		}
	}

	// ===== 父系旁系：叔伯 / 姑姑 =====
	var uncles, aunts []*kinNode // 叔伯（父之兄弟）、姑姑（父之姐妹）
	if father != nil {
		for _, u := range idx.siblingsOf(father) {
			if kinCrossRecord(self, u) {
				continue
			}
			if u.gender == 2 {
				kc.add(u, "姑姑", "姑姑", kinPriPeer)
				aunts = append(aunts, u)
			} else {
				kc.add(u, "叔伯", kinElderTitle(u, father, "伯父", "叔叔", "叔伯"), kinPriPeer)
				uncles = append(uncles, u)
			}
		}
	}

	// ===== 堂兄弟姐妹（叔伯的子女）+ 登记男性堂兄弟（供堂侄用） =====
	var tangBrothers []*kinNode
	for _, u := range uncles {
		for _, c := range idx.childrenOf(u) {
			if c.key() == self.key() || kinCrossRecord(self, c) {
				continue
			}
			kc.add(c, "堂兄弟姐妹", kinPeerTitle(kinTangTerms, c.gender, kinAgeOrder(c, self)), kinPriPeer)
			if c.gender == 1 {
				tangBrothers = append(tangBrothers, c)
			}
		}
	}

	// ===== 姑表兄弟姐妹（姑姑的子女） =====
	var biaoBrothers []*kinNode // 表兄弟（男，含姑表/舅表/姨表），供表侄用
	for _, a := range aunts {
		for _, c := range idx.childrenOf(a) {
			kc.addBiaoCousin(self, c, "姑表兄弟姐妹", &biaoBrothers)
		}
	}

	// ===== 母系旁系：舅舅 / 姨 及其子女（舅表、姨表） =====
	var maternalUncles, maternalAunts []*kinNode
	if mother != nil {
		for _, u := range idx.siblingsOf(mother) {
			if kinCrossRecord(self, u) {
				continue
			}
			if u.gender == 2 {
				kc.add(u, "姨", "姨", kinPriPeer)
				maternalAunts = append(maternalAunts, u)
				for _, c := range idx.childrenOf(u) {
					kc.addBiaoCousin(self, c, "姨表兄弟姐妹", &biaoBrothers)
				}
			} else {
				kc.add(u, "舅舅", "舅舅", kinPriPeer)
				maternalUncles = append(maternalUncles, u)
				for _, c := range idx.childrenOf(u) {
					kc.addBiaoCousin(self, c, "舅表兄弟姐妹", &biaoBrothers)
				}
			}
		}
	}

	// ===== 堂伯/堂叔/堂姑（祖父的兄弟之子女）=====
	// ===== 表伯/表叔/表姑（父亲的表兄弟姐妹：祖父姐妹之子女 + 祖母兄弟姐妹之子女）=====
	if father != nil {
		if gf := idx.fatherOf(father); gf != nil {
			for _, gu := range idx.siblingsOf(gf) {
				if kinCrossRecord(self, gu) {
					continue
				}
				if gu.gender == 1 {
					for _, c := range idx.childrenOf(gu) {
						if c.key() == father.key() || kinCrossRecord(self, c) {
							continue
						}
						if c.gender == 2 {
							kc.add(c, "堂姑", "堂姑", kinPriPeer)
						} else {
							kc.add(c, "堂伯叔", kinElderTitle(c, father, "堂伯", "堂叔", "堂叔伯"), kinPriPeer)
						}
					}
				} else {
					for _, c := range idx.childrenOf(gu) {
						kc.addFatherCousin(self, c, father)
					}
				}
			}
		}
		if gm := idx.motherOf(father); gm != nil {
			for _, ms := range idx.siblingsOf(gm) {
				if kinCrossRecord(self, ms) {
					continue
				}
				for _, c := range idx.childrenOf(ms) {
					kc.addFatherCousin(self, c, father)
				}
			}
		}
	}

	// ===== 侄辈：侄子/侄女、外甥/外甥女、侄孙/侄孙女 =====
	for _, b := range brothers {
		for _, c := range idx.childrenOf(b) {
			if kinCrossRecord(self, c) {
				continue
			}
			if c.gender == 2 {
				kc.add(c, "侄女", "侄女", kinPriFar)
				continue
			}
			kc.add(c, "侄子", "侄子", kinPriFar)
			// 侄孙/侄孙女（侄子之子女）
			for _, cc := range idx.childrenOf(c) {
				if kinCrossRecord(self, cc) {
					continue
				}
				if cc.gender == 2 {
					kc.add(cc, "侄孙", "侄孙女", kinPriFar)
				} else {
					kc.add(cc, "侄孙", "侄孙", kinPriFar)
				}
			}
		}
	}
	for _, s := range sisters {
		for _, c := range idx.childrenOf(s) {
			if kinCrossRecord(self, c) {
				continue
			}
			if c.gender == 2 {
				kc.add(c, "外甥女", "外甥女", kinPriFar)
			} else {
				kc.add(c, "外甥", "外甥", kinPriFar)
			}
		}
	}
	// 堂侄/堂侄女（堂兄弟之子女）
	for _, c := range tangBrothers {
		for _, cc := range idx.childrenOf(c) {
			if kinCrossRecord(self, cc) {
				continue
			}
			if cc.gender == 2 {
				kc.add(cc, "堂侄", "堂侄女", kinPriFar)
			} else {
				kc.add(cc, "堂侄", "堂侄", kinPriFar)
			}
		}
	}
	// 表侄/表侄女（表兄弟之子女）
	for _, c := range biaoBrothers {
		for _, cc := range idx.childrenOf(c) {
			if kinCrossRecord(self, cc) {
				continue
			}
			if cc.gender == 2 {
				kc.add(cc, "表侄", "表侄女", kinPriFar)
			} else {
				kc.add(cc, "表侄", "表侄", kinPriFar)
			}
		}
	}

	// ===== 姻亲：伯母/婶婶、姑父、舅母、姨父 =====
	for _, u := range uncles {
		for _, sp := range u.spouses {
			kc.add(sp, "伯母婶婶", kinElderTitle(u, father, "伯母", "婶婶", "婶母"), kinPriInLaw)
		}
	}
	for _, a := range aunts {
		for _, sp := range a.spouses {
			if sp.gender == 1 {
				kc.add(sp, "姑父", "姑父", kinPriInLaw)
			}
		}
	}
	for _, u := range maternalUncles {
		for _, sp := range u.spouses {
			if sp.gender == 2 {
				kc.add(sp, "舅母", "舅母", kinPriInLaw)
			}
		}
	}
	for _, a := range maternalAunts {
		for _, sp := range a.spouses {
			if sp.gender == 1 {
				kc.add(sp, "姨父", "姨父", kinPriInLaw)
			}
		}
	}

	// ===== 姻亲：嫂子/弟媳、姐夫/妹夫 =====
	for _, b := range brothers {
		title := kinElderTitle(b, self, "嫂子", "弟媳", "嫂子弟媳")
		for _, sp := range b.spouses {
			if sp.gender == 2 {
				kc.add(sp, "嫂子弟媳", title, kinPriInLaw)
			}
		}
	}
	for _, s := range sisters {
		title := kinElderTitle(s, self, "姐夫", "妹夫", "姐夫妹夫")
		for _, sp := range s.spouses {
			if sp.gender == 1 {
				kc.add(sp, "姐夫妹夫", title, kinPriInLaw)
			}
		}
	}

	// ===== 姻亲上 1 代：岳父/岳母（妻之父母）、公公/婆婆（夫之父母） =====
	for _, sp := range idx.spouseMembers(self) {
		f, m := idx.fatherOf(sp), idx.motherOf(sp)
		if self.gender == 2 {
			kc.add(f, "公婆", "公公", kinPriInLaw)
			kc.add(m, "公婆", "婆婆", kinPriInLaw)
		} else {
			kc.add(f, "岳父母", "岳父", kinPriInLaw)
			kc.add(m, "岳父母", "岳母", kinPriInLaw)
		}
	}

	// ===== 配偶（保持原有「配偶」组与正配/继配/侧室条目） =====
	for _, sp := range self.spouses {
		rel := sp.spType
		if rel == "" {
			rel = "配偶"
		}
		kc.add(sp, "配偶", rel, kinPriSpouse)
	}

	return kc
}
