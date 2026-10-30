// 行傳（齒錄）條首小字：「某公第幾子」（传统写法，见参考 实体族谱/行传-03）
//
// 目的：把数据里已有的父子关系，写成传统齿录条首的那一行小字：
//   謨阜長子 → 謨阜公長子      謨阜三子 → 謨阜公第三子
//   謨阜之子 → 謨阜公之子      始祖／房祖／配 … → 原样显示
//
// 口径（与实物谱一致）：
//   · 排行「長/次」不加「第」（長子、次子），三子起才写「第三子」（第三子、第四子…）；
//   · 父名后加「公」作尊称（父名已带「公」则不重复）；
//   · 取不到行第（无父、父名缺、关系字不是「X子」）→ 返回原字串；空字串 → 不显示。
//
// 版面（2026-10 属主口径）：條首小字一律排成**固定两段**，不再按长度被动折行：
//   第 1 段＝父名＋公（如「謨阜公」）；第 2 段＝行第（如「長子」「第三子」「第十五子」）。
//   竖排版式里两段＝自右而左两列（右＝父名，左＝行第）；横排版式里＝上下两行。
//   取不到父名或行第时：只显示能得到的那一段（如「配」「始祖」只剩一段，单段显示）；
//   两段都取不到 → 空串，调用方不显示（格子留空，框线照画）。

const CN_RANK = ['', '一', '二', '三', '四', '五', '六', '七', '八', '九', '十',
  '十一', '十二', '十三', '十四', '十五', '十六', '十七', '十八', '十九', '二十']
// 排行用字：長/次/三/四…（与后端 relLabel/cnRank 同口径）
const RANK_CHAR = ['', '長', '次', '三', '四', '五', '六', '七', '八', '九', '十',
  '十一', '十二', '十三', '十四', '十五', '十六', '十七', '十八', '十九', '二十']

/** n（1 起）→ 排行用字：1 長、2 次、3 三 … */
export function rankChar(n) {
  const i = Number(n) || 0
  return RANK_CHAR[i] || String(i)
}

/** n（1 起）→ 中文数字：1 一、2 二、3 三 …（用于「第三子」的阿拉伯/中文统一） */
export function cnNumber(n) {
  const i = Number(n) || 0
  return CN_RANK[i] || String(i)
}

/** 父名 + 公（已带「公」则不加） */
function withGong(fatherName) {
  const s = String(fatherName || '').trim()
  if (!s) return ''
  return /公$/.test(s) ? s : s + '公'
}

/** 行第文字 → 标准写法：長子／次子 不加「第」，其余「第三子」「第十五子」 */
function rankText(rankNum, suffix) {
  const r = String(rankNum || '')
  if (!r) return ''
  const s = suffix === '女' ? '女' : '子'
  if (r === '長' || r === '次') return r + s
  return '第' + r.replace(/^第/, '') + s
}

/**
 * 由「父名 + 排行序号 + 性别」生成条首小字（固定两段）。
 * @param {string} fatherName 父名（行傳所见的父之名讳）
 * @param {number} idx        在同胞（同性别）中的排行，1 起
 * @param {number} gender     1 男（默认）/ 2 女
 * @returns {{f:string,r:string}} f＝「父名＋公」，r＝「長子／次子／第三子…」；取不到的那段为 ''
 */
export function relHeadParts(fatherName, idx, gender) {
  const father = withGong(fatherName)
  const i = Number(idx) || 0
  if (!father || i <= 0) return { f: '', r: '' }
  return { f: father, r: rankText(rankChar(i), Number(gender) === 2 ? '女' : '子') }
}

/**
 * 由「父名 + 排行序号 + 性别」生成条首小字（單段拼字串，兼容舊呼叫）。
 * @returns {string} 如「謨阜公第三子」「謨阜公長子」；取不到时返回 ''
 */
export function relHeadOf(fatherName, idx, gender) {
  const p = relHeadParts(fatherName, idx, gender)
  return p.f + p.r
}

// 已有关系字的各种写法（后端垂珠體 relLabel 输出「謨阜長子」「謨阜十五子」「謨阜之子」…）
const RE_RANKED = /^(.*?)(長|次|[一二三四五六七八九十]+)([子女])$/
const RE_SINGLE = /^(.*?)之([子女])$/

/**
 * 已有关系字（形如「謨阜長子」）→ 條首小字的两段（固定两段排版用）。
 *   「謨阜長子」   → { f:'謨阜公', r:'長子' }
 *   「謨阜十五子」 → { f:'謨阜公', r:'第十五子' }
 *   「謨阜之子」   → { f:'謨阜公', r:'之子' }
 *   无父/无行第（始祖、房祖、配、繼配…）→ { f:原字串, r:'' }（只显示能得到的那一段）
 *   空 → { f:'', r:'' }（调用方据此不显示）
 */
export function relHeadPartsOfText(rel) {
  const s = String(rel == null ? '' : rel).trim()
  if (!s) return { f: '', r: '' }
  // 已是「…公第三子」口径：父名段已带「公」，直接拆行第
  let m = s.match(/^(.*?)(第[一二三四五六七八九十]+[子女])$/)
  if (m && m[1]) return { f: m[1], r: m[2] }
  m = s.match(RE_SINGLE)
  if (m && m[1]) return { f: withGong(m[1]), r: '之' + m[2] }
  m = s.match(RE_RANKED)
  if (m && m[1]) return { f: withGong(m[1]), r: rankText(m[2], m[3]) }
  return { f: s, r: '' }
}

/**
 * 已有关系字 → 单段字串（舊介面，兼容既有呼叫）。
 * 无父/无行第（始祖、房祖、配…）→ 原样返回；空 → ''（调用方据此不显示）。
 */
export function relHeadOfText(rel) {
  const p = relHeadPartsOfText(rel)
  return p.f + p.r
}
