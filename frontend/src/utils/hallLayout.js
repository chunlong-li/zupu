// ============================================================================
// 祠堂牌位布局算法（纯数据层，与渲染方式无关）
// 规则（2026-09-13 用户定）：
//   - 第一版：按世代排——第一排第一世、第二排第二世……
//   - 同世代内按年龄排（出生越早越靠前）
//   - 预留：后续切换为「按房支排」（mode 字段已预留，输出结构不变）
// ============================================================================

// 归一化出生日期为可比较的排序键
// 支持：'YYYY-MM-DD'、'YYYY/M/D'、'YYYY年M月D日'、纯年份 'YYYY'
// 返回 'YYYYMMDD' 形式字符串；无法解析返回 null
export function normalizeBirthDate(str) {
  if (!str || typeof str !== 'string') return null
  const s = str.trim()
  if (!s) return null

  // 1) ISO / 斜杠格式
  let m = s.match(/^(\d{4})[-/](\d{1,2})[-/](\d{1,2})$/)
  if (m) return `${m[1]}${m[2].padStart(2, '0')}${m[3].padStart(2, '0')}`

  // 2) 中文年月日
  m = s.match(/^(\d{4})年(\d{1,2})月(\d{1,2})日?$/)
  if (m) return `${m[1]}${m[2].padStart(2, '0')}${m[3].padStart(2, '0')}`

  // 3) 纯年份
  m = s.match(/^(\d{4})$/)
  if (m) return `${m[1]}0000`

  return null
}

// 提取成员排序键：出生越早越靠前；无出生信息的排后面（按 id 稳定排序）
function birthSortKey(member) {
  // 优先公历，其次农历，再次干支/年号
  const candidates = [
    member.birth_gregorian,
    member.birth_lunar,
    member.birth_ganzhi,
    member.birth_era,
  ]
  for (const c of candidates) {
    const key = normalizeBirthDate(c)
    if (key) return key
  }
  // 无任何出生信息：排最后，用 id 保证稳定
  return '99999999' + String(member.id).padStart(10, '0')
}

// 构建祠堂牌位布局
// 是否已故：任一卒年字段非空（含"已逝"等标记，存 death_era）
function isDeceased(m) {
  const notEmpty = (v) => !!v && !['不详', '不詳'].includes(String(v).trim())
  return (
    notEmpty(m.death_gregorian) ||
    notEmpty(m.death_lunar) ||
    notEmpty(m.death_ganzhi) ||
    notEmpty(m.death_era)
  )
}

// 排内重排：最大的（最年长）居中，其余按年龄交替往两边对称展开（小的在两边、大的在中间）
function centerLargest(members) {
  const right = [] // 最大 + 第2、4、6...大（放中间及右侧）
  const left = [] // 第3、5、7...大（放左侧）
  members.forEach((m, i) => {
    if (i === 0 || i % 2 === 1) right.push(m)
    else left.push(m)
  })
  return [...left.reverse(), ...right]
}

// @param {Array} members 成员数组（来自 /api/members 或 /api/hall/layout，已脱敏）
// @param {Object} [opts]
// @param {string} [opts.mode='generation'] 排布模式：'generation'（按世代）| 'branch'（按房支，预留）
// @returns {{mode: string, rows: Array<{generation:number|null, branch:string|null, members:Array}>}}
export function buildHallLayout(members, opts = {}) {
  const mode = opts.mode || 'generation'
  // 祠堂只供奉已故成员（排除在世）
  const list = (Array.isArray(members) ? members : []).filter(isDeceased)

  if (mode === 'branch') {
    // 预留：按房支分组（后续实现），结构对齐 generation 模式
    return buildBranchLayout(list)
  }

  // 默认：按世代分组
  const groups = new Map()
  for (const m of list) {
    const gen = m.generation ?? -1
    if (!groups.has(gen)) groups.set(gen, [])
    groups.get(gen).push(m)
  }

  const rows = []
  // 世代升序（第一世在第一排）
  const generations = [...groups.keys()].sort((a, b) => a - b)
  for (const gen of generations) {
    const members = groups.get(gen)
    // 同世代内按年龄排（出生早在前）
    members.sort((a, b) => {
      const ka = birthSortKey(a)
      const kb = birthSortKey(b)
      if (ka !== kb) return ka < kb ? -1 : 1
      return (a.id ?? 0) - (b.id ?? 0)
    })
    rows.push({ generation: gen, branch: null, members: centerLargest(members) })
  }

  return { mode, rows }
}

// 预留：按房支排布（结构对齐，逻辑后续实现）
function buildBranchLayout(list) {
  const groups = new Map()
  for (const m of list) {
    const branch = m.branch || '未分房'
    if (!groups.has(branch)) groups.set(branch, [])
    groups.get(branch).push(m)
  }
  const rows = []
  for (const [branch, members] of groups) {
    // 房支内部仍按世代 + 年龄排（复用 generation 逻辑）
    const sub = buildHallLayout(members, { mode: 'generation' })
    for (const r of sub.rows) {
      rows.push({ generation: r.generation, branch, members: r.members })
    }
  }
  return { mode: 'branch', rows }
}
