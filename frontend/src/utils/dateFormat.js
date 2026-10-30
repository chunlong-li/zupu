/**
 * 生卒日期格式化工具（繁体中文）
 *
 * 规则：
 * 1. 生卒完全为空/标记为"不详" → "生卒未詳"
 * 2. 仅生不详 → "生年未詳" + 完整卒日；仅卒不详（已逝标记）→ 完整生日 + "卒年未詳"
 * 3. 仅知年份、不知月日 → "生於××年"，不补默认值
 * 4. 仅知大致朝代/时期 → "生於××年間""卒於××季"
 * 5. 统一用繁体汉字
 */

function isEmpty(val) {
  return !val || !val.trim() || val.trim() === '不详' || val.trim() === '不詳'
}

// 从农历字符串提取月日部分（去掉年份），如"一九九三年七月廿八" → "七月廿八"
function lunarMonthDay(lunarStr) {
  if (!lunarStr) return ''
  const idx = lunarStr.indexOf('年')
  if (idx < 0) return ''
  return lunarStr.slice(idx + 1)
}

// 去世标记：表示「已知去世但日期不详」
const DEATH_MARKS = ['已逝', '已故', '逝世', '已卒', '殁', '病故', '亡故']

// 判断某个字段值是否为去世标记
export function isDeathMark(val) {
  if (!val) return false
  return DEATH_MARKS.includes(val.trim())
}

// 规范化卒年字段：death_gregorian 填了去世标记 → 转存到 death_era 并清空日期字段
// （留空 = 在世，清空所有卒日期字段）
export function normalizeDeathFields(p) {
  const dg = (p.death_gregorian || '').trim()
  if (!dg) {
    p.death_gregorian = ''
    p.death_lunar = ''
    p.death_ganzhi = ''
    p.death_era = ''
  } else if (isDeathMark(dg)) {
    p.death_era = dg
    p.death_gregorian = ''
    p.death_lunar = ''
    p.death_ganzhi = ''
  }
}

function isBirthEmpty(p) {
  return isEmpty(p.birth_gregorian) && isEmpty(p.birth_lunar) && isEmpty(p.birth_era) && isEmpty(p.birth_ganzhi)
}

function isDeathEmpty(p) {
  return isEmpty(p.death_gregorian) && isEmpty(p.death_lunar) && isEmpty(p.death_era) && isEmpty(p.death_ganzhi)
}

function formatBirthDate(p) {
  if (!isEmpty(p.birth_era)) {
    const md = lunarMonthDay(p.birth_lunar)
    return md ? `生於${p.birth_era.trim()}年${md}` : `生於${p.birth_era.trim()}`
  }
  if (!isEmpty(p.birth_lunar)) {
    return `生於${p.birth_lunar.trim()}`
  }
  if (!isEmpty(p.birth_gregorian)) {
    const d = p.birth_gregorian.trim()
    if (/^\d{4}$/.test(d)) {
      return `生於${d}年`
    }
    return `生於${d}`
  }
  return ''
}

function formatDeathDate(p) {
  // 去世标记优先（death_era 或 death_gregorian 里的「已逝」等）→ 卒年未詳
  if (isDeathMark(p.death_era) || isDeathMark(p.death_gregorian)) return '卒年未詳'
  if (!isEmpty(p.death_era)) {
    const md = lunarMonthDay(p.death_lunar)
    return md ? `卒於${p.death_era.trim()}年${md}` : `卒於${p.death_era.trim()}`
  }
  if (!isEmpty(p.death_lunar)) {
    return `卒於${p.death_lunar.trim()}`
  }
  if (!isEmpty(p.death_gregorian)) {
    const d = p.death_gregorian.trim()
    if (/^\d{4}$/.test(d)) {
      return `卒於${d}年`
    }
    return `卒於${d}`
  }
  return ''
}

/**
 * 格式化生卒日期（组合显示）
 * @param {Object} p - 人物对象，含 birth_gregorian/birth_lunar/birth_era/death_gregorian/death_lunar/death_era
 * @returns {string} 格式化后的生卒字符串
 */
export function formatLifeDates(p) {
  const birth = formatBirthDate(p)
  const death = formatDeathDate(p)
  const birthUnknown = birth === '' // 生未详
  const deathUnknown = isDeathMark(p.death_era) || isDeathMark(p.death_gregorian) // 卒未详（已逝标记）

  // 生未详 + 卒未详 → 生卒未詳
  if (birthUnknown && deathUnknown) return '生卒未詳'
  // 生未详 → 生年未詳（+ 卒年，若有）
  if (birthUnknown) return death ? `生年未詳　${death}` : '生年未詳'
  // 卒未详（已逝）→ 生於××　卒年未詳
  if (deathUnknown) return `${birth}　卒年未詳`
  // 卒空（在世）→ 生於××
  if (!death) return birth
  // 生卒都有 → 生於×× — 卒於××
  return `${birth} — ${death}`
}

/**
 * 格式化出生日期（单独显示）
 */
export function formatBirth(p) {
  return formatBirthDate(p) || '生年未詳'
}

/**
 * 格式化卒日期（单独显示）
 */
export function formatDeath(p) {
  return formatDeathDate(p) || ''
}