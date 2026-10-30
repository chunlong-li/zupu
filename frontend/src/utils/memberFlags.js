// 族谱成员标记（无嗣 / 迁居外地失联）—— 世系图统一标注文案
// 老谱惯例：已有的照写，缺啥补啥 + 失考

import { formatBirth, formatDeath } from './dateFormat'

// 原始成员对象（含 birth_/death_/burial_ 字段）→ 已知项文本
// 返回 { birth:'生…', death:'殁…', burial:'葬地' }；未知项为 ''（供下方拼「失考」）
export function memberKnown(m) {
  const p = m || {}
  let birth = formatBirth(p)                                  // '生於…' | '生年未詳'
  birth = (!birth || birth === '生年未詳') ? '' : birth.replace(/^生於/, '生')
  let death = formatDeath(p)                                  // '卒於…' | '卒年未詳' | ''
  death = (!death || death === '卒年未詳') ? '' : death.replace(/^卒於/, '殁')
  const burial = String(p.burial_place || '').trim()
  return { birth, death, burial }
}

// 已知项文本 → 老谱标注：已知的先列，再补缺失项 + 「失考」，无子女再缀「子嗣失考」
// known: { birth, death, burial }（值 = 已格式化文本，缺失传 ''/undefined；burial 传纯地名）
// showKnown=false 时只写缺失项（世系图/宝塔式等图形版面用，不显示生卒）
export function lostContactText(known, hasChildren = false, showKnown = true) {
  const k = known || {}
  const parts = []
  const miss = []
  if (k.birth) { if (showKnown) parts.push(k.birth) } else miss.push('生')
  if (k.death) { if (showKnown) parts.push(k.death) } else miss.push('殁')
  if (k.burial) { if (showKnown) parts.push('葬' + k.burial) } else miss.push('葬')
  if (miss.length) parts.push(miss.join('') + '失考')
  if (!hasChildren) parts.push('子嗣失考')
  if (!parts.length) parts.push('失考')
  return parts.join('，')
}

export const NO_HEIR = '无嗣' // 简体视图
export const NO_HEIR_TRAD = '無嗣' // 繁体视图（垂珠体/牒记式/苏式）
