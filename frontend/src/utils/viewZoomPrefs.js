// 预览缩放偏好（本地记忆）
// 用途：把「用户调过的缩放值」记在 localStorage，作为**自动适应宽度的上限**：
//       实际生效值 = min(按容器宽度自动适应的值, 记住的值)
// 这样：屏幕小 → 自动更小，取更小的那个（与原来一致）；屏幕大 → 不会被擅自放大超过记住的值。
// 注意：localStorage 不可用（隐私模式等）时静默降级为默认值，不影响功能。

function readNum(key, dft) {
  try {
    const v = Number(localStorage.getItem(key))
    return Number.isFinite(v) && v > 0 ? v : dft
  } catch (e) {
    return dft
  }
}

function writeNum(key, v) {
  try {
    localStorage.setItem(key, String(v))
  } catch (e) { /* ignore */ }
}

// 苏式垂珠体：记住的预览百分比（默认 50）
export const SUZHU_ZOOM_KEY = 'suzhuZoomPct'
export const SUZHU_ZOOM_DEFAULT = 50
export const readSuzhuZoomPct = () => readNum(SUZHU_ZOOM_KEY, SUZHU_ZOOM_DEFAULT)
export const saveSuzhuZoomPct = v => writeNum(SUZHU_ZOOM_KEY, v)

// 是否已经存在用户保存过的值（用于区分「从未调过」与「调过」）
export function hasUserZoomPref(key) {
  try {
    const v = Number(localStorage.getItem(key))
    return Number.isFinite(v) && v > 0
  } catch (e) {
    return false
  }
}

// 牒记式（竖版/横版共用）：记住的预览比率（默认 0.70）
export const DIEJI_SCALE_KEY = 'diejiPreviewScale'
export const DIEJI_SCALE_DEFAULT = 0.70
export const readDiejiPreviewScale = () => readNum(DIEJI_SCALE_KEY, DIEJI_SCALE_DEFAULT)
export const saveDiejiPreviewScale = v => writeNum(DIEJI_SCALE_KEY, v)
