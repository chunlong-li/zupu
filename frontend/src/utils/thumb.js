// 缩略图 URL：后端已有 GET /api/thumbnail?path=/uploads/xxx&size=120（磁盘缓存，上传时已预热 120/400 两档）
// 列表/弹窗一律走它，别直接加载原图（原图动辄几百 KB~几 MB，13 张就是十几 MB）。
// 注意：书页/打印仍必须用原图路径（清晰），只有"缩略预览"走这里。
export const THUMB_MINI = 120    // 可选图片池（一屏几十张，取最小档）
export const THUMB_LIST = 240    // 列表卡片 / 已选缩略（＝上传时预生成的档位）
export const THUMB_LARGE = 400   // 大图预览 / 详情（＝上传时预生成的档位）

// 兼容旧名
export const THUMB_PICK = THUMB_LIST

export function thumbUrl(path, size = THUMB_LIST) {
  const p = String(path || '')
  if (!p) return ''
  // path 可能是 "/uploads/x.png" 或 "uploads/x.png" 或裸文件名
  const norm = p.startsWith('/') ? p : (p.startsWith('uploads/') ? '/' + p : '/uploads/' + p)
  return '/api/thumbnail?path=' + encodeURIComponent(norm) + '&size=' + size
}

// 谱书/弹窗里的图片加载失败（源文件被删、路径丢失）→ 给格子挂 img-missing 类，
// 由各处 CSS 显示占位（不报错、不白屏、不撑破版面）。
export function onGalleryImgError(e) {
  const img = e && e.target
  if (!img) return
  const box = img.closest('.g-cell') || img.closest('.gal-item')
  if (box) box.classList.add('img-missing')
  img.style.visibility = 'hidden'
}
