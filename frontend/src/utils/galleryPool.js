// 「图片展示」可选图片池：通用多媒体库 + 成员附件
//
// 旧写法：先串行 for 一遍所有成员（Dieji/Branch 是 130+ 位成员的 130+ 次 /attachments 请求，每人一次），
// 全部跑完才 galleryPool.value = out —— 期间弹窗一直显示「没有可用图片（请先上传）」，
// 属主看到的就是"图片展示不能用"；130 次串行请求在真实网络下要几十秒，也就是"加载有点慢"。
//
// 现写法：① 先请求多媒体库（1 个请求，毫秒级）→ 立刻把属主上传的素材铺进池子（先能选、先用）；
//        ② 再**并发 8** 拉成员附件补齐（130 人 ÷ 8 ≈ 17 轮，耗时约 1/N）。
import api from '../api'
import { normalizeEntries } from './galleryStore'
import { branchCtx } from './branchContext'

const ATT_CONCURRENCY = 8

let branchNamesCache = null
// 房支 id → 名称（用于把素材归属显示成「长房」而不是 id）
export async function fetchBranchNames() {
  if (branchNamesCache) return branchNamesCache
  const names = {}
  try {
    const { data } = await api.get('/branches')
    ;(data || []).forEach((b) => { names[String(b.id)] = b.name || ('房支 ' + b.id) })
  } catch (e) { /* 拿不到就退回 id */ }
  branchNamesCache = names
  return names
}

function normPath(p) {
  const s = String(p || '')
  if (!s) return ''
  return s.startsWith('/') ? s : (s.startsWith('uploads/') ? '/' + s : '/uploads/' + s)
}

// 通用多媒体库（只取图片；视频不进谱书/打印）
// 归属（owner）来自后端的 branch_id：0＝总谱，>0＝某房支 → 弹窗据此分组（总谱默认只看总谱素材）
export async function fetchMediaImages() {
  try {
    const names = await fetchBranchNames()
    // 按授权房支过滤（后端强制）：带上顶部「含下级」开关，让下级房支素材也算在范围内
    const inc = branchCtx.includeChildren ? '1' : '0'
    const { data } = await api.get('/media?include_children=' + inc)
    return (data || [])
      .filter((x) => x.type !== 'video')
      .map((x) => {
        const bid = Number(x.branch_id || 0)
        const owner = bid > 0 ? ('branch:' + bid) : 'family'
        return {
          id: 'm' + x.id,
          path: normPath(x.path),
          description: x.description || '',
          memberName: x.title || x.category || '家族多媒体',
          source: 'media',
          sourceKey: owner,
          sourceLabel: bid > 0 ? (names[String(bid)] || ('房支 ' + bid)) : '总谱',
          owner, branchId: bid,
        }
      })
      .filter((x) => x.path && x.path !== '/uploads/')
  } catch (e) {
    return []
  }
}

// 成员附件：并发 ATT_CONCURRENCY 路
export async function fetchMemberImages(memberList) {
  const list = (memberList || []).filter((m) => m && m.id != null)
  const out = []
  let cursor = 0
  async function worker() {
    while (cursor < list.length) {
      const m = list[cursor++]
      try {
        const { data } = await api.get('/members/' + m.id + '/attachments')
        ;(data || []).forEach((a) => {
          const p = normPath(a.path)
          if (!p) return
          out.push({
            id: a.id,
            path: p,
            description: a.description || '',
            memberName: m.name || m.given_name || '',
            source: 'member',
            sourceKey: 'member',
            sourceLabel: '成员附件' + (m.branch ? '·' + m.branch : ''),
          })
        })
      } catch (e) { /* 单个成员失败不影响整体 */ }
    }
  }
  const n = Math.min(ATT_CONCURRENCY, list.length)
  await Promise.all(Array.from({ length: n }, worker))
  return out
}

// 房支来源：各支谱「图片展示」里已选的图片（localStorage: branchGallery:<id>）。
// 说明：支谱条目本身也存在本机 localStorage，所以这里读到的是**本机配置过的房支**；
//      同一路径在多个房支出现时按路径去重（id 也按路径，选一次即处处选中），并记 alsoIn 供提示。
export async function fetchBranchImages() {
  const names = await fetchBranchNames()

  const perBranch = []   // [{ bid, label, map: Map<path, item> }]
  let keys = []
  try {
    keys = Object.keys(localStorage).filter((k) => k.startsWith('branchGallery:'))
  } catch (e) { keys = [] }
  keys.sort()
  for (const key of keys) {
    const bid = key.slice('branchGallery:'.length)
    const label = names[bid] || ('房支 ' + bid)
    let raw = null
    try { raw = JSON.parse(localStorage.getItem(key) || 'null') } catch (e) { continue }
    const entries = normalizeEntries(raw)
    const map = new Map()
    entries.forEach((en) => (en.images || []).forEach((im) => {
      const path = normPath(im.path)
      if (!path || map.has(path)) return
      map.set(path, { path, description: im.description || '', memberName: im.memberName || '' })
    }))
    if (map.size) perBranch.push({ bid, label, map })
  }
  const out = []
  for (const b of perBranch) {
    const others = perBranch.filter((x) => x.bid !== b.bid && x.map.has)
    for (const [path, im] of b.map) {
      const alsoIn = perBranch.filter((x) => x.bid !== b.bid && x.map.has(path)).map((x) => x.label)
      out.push({
        id: 'br-' + path, path,
        description: im.description,
        memberName: im.memberName || b.label,
        source: 'branch', sourceKey: 'bsheet:' + b.bid, sourceLabel: b.label + '·支谱图片',
        branchId: b.bid, alsoIn,
      })
    }
  }
  return out
}

// 当前登录用户被授权的房支 id 列表（镜像后端 AuthorizedMediaBranchIDs）：
//   null ＝ 不限制（超级管理员）；[] ＝ 未分配房支（只看总谱素材）；否则＝本房支(+「含下级」时的下级)
export function permittedBranchIds() {
  let u = null
  try { u = JSON.parse(localStorage.getItem('user') || 'null') } catch (e) { u = null }
  if (!u) return []
  if (u.role === '超级管理员') return null
  const name = String(u.branch || '').trim()
  if (!name) return []
  const list = branchCtx.branches || []
  const own = list.find((b) => b.name === name)
  if (!own) return []
  const ids = [own.id]
  const inc = branchCtx.includeChildren !== false
  if (inc && own.path) {
    list.forEach((b) => {
      if (b.id === own.id || !b.path) return
      if (b.path === own.path || b.path.indexOf(own.path + '.') === 0) ids.push(b.id)
    })
  }
  return ids
}

// 某个来源键是否在该用户权限内
//   family / bsheet:<id> / branch:<id> 按授权房支判断；member 始终可用（成员接口本身已按顶部上下文取数）
export function sourcePermitted(key) {
  if (!key || key === 'member') return true
  if (key === 'family') return true           // 总谱素材对所有人可见（后端只返回总谱素材本身）
  const ids = permittedBranchIds()
  if (ids === null) return true
  const m = String(key).match(/^(?:branch|bsheet):(\d+)$/)
  if (!m) return false
  return ids.indexOf(Number(m[1])) >= 0
}
