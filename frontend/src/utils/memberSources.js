// 资料来源（member_sources）共用工具：
//   · 谱书三版式（垂珠体 / 牒记式 / 支谱）的行传页要用「据：…」小字脚注 → 一次取全量、按成员分组；
//   · 成员页（MemberDetail）用按成员的接口，见 /members/:id/sources。
import api from '../api'

// 常用类型（前端下拉默认项；kind 本身是自由文本，可自定义）
export const SOURCE_KINDS = ['族谱旧谱', '口述', '墓碑', '家书', '方志', '其他']

// 谱书脚注口径：多条来源用「；」分隔，前置「据：」；没有来源 → 返回空串（调用方据此不渲染、不留空行）
export const SOURCE_SEP = '；'

// 取全量资料来源（失败不阻塞谱书渲染：返回空数组）
export async function fetchAllSources() {
  try {
    const { data } = await api.get('/member-sources')
    return Array.isArray(data) ? data : []
  } catch (e) {
    return []
  }
}

// 按 member_id 分组：{ [memberId]: [{ text, kind }] }（空文本的行直接跳过）
export function groupSourcesByMember(list) {
  const map = {}
  ;(list || []).forEach((s) => {
    const id = Number(s && s.member_id)
    const text = String((s && s.text) == null ? '' : s.text).trim()
    if (!id || !text) return
    if (!map[id]) map[id] = []
    map[id].push({ text, kind: String((s && s.kind) || '').trim() })
  })
  return map
}

// 某成员的脚注文字：'据：光緒壬辰劉氏族譜·卷三·第12頁；口述：…'；无来源返回 ''
export function sourceFoot(map, memberId, prefix = '据：') {
  const arr = map && map[Number(memberId)]
  if (!arr || !arr.length) return ''
  return prefix + arr.map((s) => s.text).join(SOURCE_SEP)
}
