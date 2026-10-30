// 「图片展示」多条目数据模型 + 旧数据迁移 + 书页推导（四个视图共用）
//
// 新格式：{ entries: [ { id, title, images:[{id,path,description,memberName}], layout, showCaption }, ... ] }
// 旧格式：{ images:[...], layout, showCaption }  ← 自动迁移成「1 个条目」，属主已选好的图不丢
// 顺序即书内顺序；空条目（没选图）不产页、不进目錄。
import { ref } from 'vue'

export const LAYOUTS = [1, 2, 4, 6, 9]
let seq = 0
export function newEntryId() {
  seq += 1
  return 'e' + Date.now().toString(36) + seq.toString(36)
}
export function layoutLabel(n) {
  return n === 1 ? '单张' : n + ' 宫格'
}
// 条目标题：填了用填的；没填且只有一个条目 → 「图片展示」；多个 → 「图片展示 1/2/3…」
export function entryTitle(en, idx, total) {
  const t = String((en && en.title) || '').trim()
  if (t) return t
  return total > 1 ? '图片展示 ' + (idx + 1) : '图片展示'
}

function normEntry(e) {
  const imgs = Array.isArray(e && e.images) ? e.images.filter((x) => x && x.path) : []
  const layout = LAYOUTS.includes(e && e.layout) ? e.layout : 4
  return {
    id: (e && e.id) || newEntryId(),
    title: (e && e.title) || '',
    images: imgs,
    layout,
    showCaption: !(e && e.showCaption === false),
  }
}

// 读到的任意历史数据 → 条目数组
export function normalizeEntries(raw) {
  if (!raw || typeof raw !== 'object') return []
  if (Array.isArray(raw.entries)) return raw.entries.map(normEntry)
  if (Array.isArray(raw.images) && raw.images.length) return [normEntry(raw)]   // 旧格式迁移
  return []
}

// 由条目推导书页（扁平列表，顺序 = 书内顺序；空条目跳过）
export function buildEntryPages(entries) {
  const out = []
  const list = entries || []
  list.forEach((en, ei) => {
    if (!en.images || !en.images.length) return
    const per = LAYOUTS.includes(en.layout) ? en.layout : 4
    const total = Math.ceil(en.images.length / per)
    for (let i = 0; i < total; i++) {
      out.push({
        entryIndex: ei,
        entryNo: ei + 1,
        title: entryTitle(en, ei, list.length),
        images: en.images.slice(i * per, (i + 1) * per),
        per,
        partNo: i + 1,
        partTotal: total,
        showCaption: en.showCaption !== false,
        imageCount: en.images.length,
      })
    }
  })
  return out
}

// 每个（有图的）条目的页数与张数 —— 供目錄逐条出行
export function entrySummaries(entries) {
  const list = entries || []
  const out = []
  list.forEach((en, ei) => {
    if (!en.images || !en.images.length) return
    const per = LAYOUTS.includes(en.layout) ? en.layout : 4
    out.push({
      index: ei,
      title: entryTitle(en, ei, list.length),
      count: en.images.length,
      pages: Math.ceil(en.images.length / per),
      layout: per,
    })
  })
  return out
}

// localStorage 持久化（每个视图一个 key，互不覆盖）
export function createGalleryStore(key) {
  const entries = ref([])
  function load() {
    try {
      entries.value = normalizeEntries(JSON.parse(localStorage.getItem(key) || 'null'))
    } catch (e) { entries.value = [] }
    return entries.value
  }
  function save() {
    try {
      localStorage.setItem(key, JSON.stringify({ entries: entries.value }))
    } catch (e) { /* ignore */ }
  }
  return { entries, load, save }
}

// 深拷贝（弹窗内部编辑用，避免直接改父层对象）
export function cloneEntries(list) {
  try { return JSON.parse(JSON.stringify(list || [])) } catch (e) { return [] }
}

// ===== 家族共享（苏氏垂珠体 + 竖版/横版牒记式 用同一套条目；支谱仍按房支各存）=====
export const FAMILY_GALLERY_KEY = 'familyGallery'
// 旧键（各谱书各存一份的历史数据）：共享键为空时按"条目多 → 图多 → 键序"采纳其中一份
export const LEGACY_GALLERY_KEYS = ['diejiGallery', 'suzhuGallery']

function legacyImageCount(list) {
  return (list || []).reduce((n, e) => n + ((e.images && e.images.length) || 0), 0)
}
function readRaw(key) {
  try { return JSON.parse(localStorage.getItem(key) || 'null') } catch (e) { return null }
}

export function createFamilyGalleryStore() {
  const entries = ref([])
  function save() {
    try {
      localStorage.setItem(FAMILY_GALLERY_KEY, JSON.stringify({ entries: entries.value, savedAt: Date.now() }))
    } catch (e) { /* ignore */ }
  }
  function load() {
    const shared = normalizeEntries(readRaw(FAMILY_GALLERY_KEY))
    if (shared.length) { entries.value = shared; return entries.value }
    // 共享键还没有数据 → 从旧键里迁一份（其余旧键原样保留作备份，不删）
    const cands = LEGACY_GALLERY_KEYS
      .map((k) => ({ key: k, list: normalizeEntries(readRaw(k)) }))
      .filter((c) => c.list.length)
    if (!cands.length) { entries.value = []; return entries.value }
    cands.sort((a, b) =>
      (b.list.length - a.list.length) ||
      (legacyImageCount(b.list) - legacyImageCount(a.list)) ||
      (LEGACY_GALLERY_KEYS.indexOf(a.key) - LEGACY_GALLERY_KEYS.indexOf(b.key)))
    entries.value = cands[0].list
    save()   // 采纳后写入共享键（旧键保留）
    return entries.value
  }
  return { entries, load, save }
}
