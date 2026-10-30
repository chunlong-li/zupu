// 世系图「分页打印 / 卷轴导出」——纸型、矢量切页、索引页、打印 HTML 生成
//
// 设计原则（与需求一一对应）：
//  1) **屏幕预览完全不动**：本模块只读 DOM（克隆 SVG 后按 viewBox 逐页裁剪），不修改原 SVG 的
//     尺寸 / pan / zoom / 样式；所有纸型与分页效果只出现在**打印文档**里。
//  2) **矢量切页**：每页一个 <svg viewBox="x y w h">，内容是同一份矢量图，绝不做截图切片。
//  3) **切页线必须落在节点空隙**：收集节点 / 文字的包围盒 → 投影成一维占用区间 → 只在空隙里下刀，
//     因此不会把任何一个头像框、姓名、配偶框切成两半。
//  4) **A3/A4 同一套算法**：只换纸张尺寸与缩放，A4 自然张数更多。
//  5) 相邻页重叠（默认 12mm）→ 跨缝的连接线在两张纸上都能看到。
//  6) 索引页（第一页）：整图缩略 + 网格坐标（A1..D4）+ 每格页码与世代范围。
//  7) 卷轴：一张超长单页（矢量），宽度按纸宽、长度按内容或预设。

// ===== 单位 =====
export const MM_PER_PX = 25.4 / 96 // 96 DPI 下 1 CSS px 的物理毫米数（屏幕 1:1）
const EPS = 1e-6

// ===== 纸型（mm，未旋转）=====
export const PAPER = {
  A4: { w: 210, h: 297 },
  A3: { w: 297, h: 420 }
}
export const PAPER_LABEL = { A4: 'A4', A3: 'A3' }
export const ORIENT_LABEL = { landscape: '横版', portrait: '竖版' }
export const MODE_LABEL = { single: '整张一页', tile: '分页（瓦片）', scroll: '卷轴（超长单页）' }

// 卷轴预设：宽 × 长（mm）；auto = 宽固定 297mm，长度按内容自动
export const SCROLL_PRESETS = [
  { value: '297x1500', width: 297, length: 1500, label: '297 × 1500 mm（至少 1.5 米）' },
  { value: '297x2500', width: 297, length: 2500, label: '297 × 2500 mm（至少 2.5 米）' },
  { value: '297x4000', width: 297, length: 4000, label: '297 × 4000 mm（至少 4 米）' },
  { value: '420x2500', width: 420, length: 2500, label: '420 × 2500 mm（A3 宽，至少 2.5 米）' },
  { value: 'auto', width: 297, length: 0, label: '自动加长（宽 297mm，长度按内容算）' }
]
export const SCROLL_HINT = '卷轴是超长单页，家用打印机印不了，需要绘图仪 / 打印店出图。'

export const DEFAULT_OPTS = {
  paper: 'A4',
  orientation: 'landscape',
  mode: 'tile',
  overlapMm: 12,
  includeIndex: true,
  // 'auto-save' = 自动：切满纸面（不超过 100% 时的张数）| 'auto-big' = 自动：切满且字更大（≥100%）
  // 'fixed'      = 用下面的 scalePct 钉死比例（50/75/100/125/150 等）
  scaleMode: 'auto-save',
  scalePct: 100,        // 100% = 96 DPI 屏幕 1:1（1 CSS px = 0.2646mm）
  scrollPreset: 'auto',
  bindingMm: 8,
  bindingSide: 'left'  // 装订侧：left / right（右侧留边更宽）
}

const clamp = (v, a, b) => Math.min(b, Math.max(a, v))
const r3 = v => Math.round(v * 1000) / 1000

// ===== 纸张 / 版面（毫米）=====
export function pageMetrics(opts) {
  const o = { ...DEFAULT_OPTS, ...(opts || {}) }
  const base = PAPER[o.paper] || PAPER.A4
  const landscape = o.orientation !== 'portrait'
  const paperW = landscape ? Math.max(base.w, base.h) : Math.min(base.w, base.h)
  const paperH = landscape ? Math.min(base.w, base.h) : Math.max(base.w, base.h)
  const padY = 6
  const headerMm = 9
  const footerMm = 8
  const sideBase = 10
  const binding = Math.max(0, Number(o.bindingMm) || 0)
  const bindLeft = o.bindingSide === 'right' ? 0 : binding
  const bindRight = o.bindingSide === 'right' ? binding : 0
  const left = sideBase + bindLeft
  const right = sideBase + bindRight
  return {
    paperW, paperH, landscape,
    padY, headerMm, footerMm,
    left, right, bindLeft, bindRight,
    bodyTop: padY + headerMm,
    areaW: r3(paperW - left - right),
    areaH: r3(paperH - padY * 2 - headerMm - footerMm)
  }
}

export function pageSizeMm(opts) {
  const m = pageMetrics(opts)
  return { w: m.paperW, h: m.paperH, label: `${PAPER_LABEL[opts.paper] || 'A4'} ${ORIENT_LABEL[opts.orientation] || '横版'}` }
}

// ===== 一维投影 / 空隙 =====
// 把包围盒投到某一轴，合并成占用区间
export function mergeIntervals(list) {
  const a = list.filter(v => v[1] - v[0] > EPS).sort((p, q) => p[0] - q[0])
  const out = []
  for (const iv of a) {
    const last = out[out.length - 1]
    if (last && iv[0] <= last[1] + EPS) last[1] = Math.max(last[1], iv[1])
    else out.push([iv[0], iv[1]])
  }
  return out
}

// 在 [start,end] 内、占用区间之间求空隙（两端也算）
export function freeGaps(intervals, start, end) {
  const gaps = []
  let cursor = start
  for (const [a, b] of intervals) {
    if (b <= start + EPS) continue
    if (a >= end - EPS) break
    if (a > cursor + EPS) gaps.push({ a: cursor, b: Math.min(a, end) })
    cursor = Math.max(cursor, b)
    if (cursor >= end - EPS) break
  }
  if (cursor < end - EPS) gaps.push({ a: cursor, b: end })
  return gaps
}

// 包围盒 → 轴投影（pad：给每条边留安全距，避免切到描边/抗锯齿）
function projectBoxes(boxes, axis, pad, start, end) {
  const list = boxes.map(b => {
    const s = axis === 'x' ? b.x : b.y
    const e = axis === 'x' ? b.x + b.w : b.y + b.h
    if (e < start - pad || s > end + pad) return null
    return [s - pad, e + pad]
  }).filter(Boolean)
  return mergeIntervals(list)
}

// 一维切页：**每页都按整张纸切**（T），页缘尽量落在节点空隙里；
// 页数 N 由内容长度定：N = ceil((total - T) / (T - 重叠)) + 1（用最少的整页数覆盖内容）；
// 各页起点从 0 均匀铺到 (total - T)，最后一页正好以「内容末尾」为基准（= 满页，会与前一页重叠）。
// 起点/终点落不到空隙时，就近吸附到空隙边缘（只移动起点，页宽仍是 T），保证不切节点。
// 返回 { pages:[{s,e}], overflow }
function tileAxis(start, end, gaps, T, ov) {
  const total = end - start
  if (total <= T + EPS) return { pages: [{ s: start, e: end }], overflow: false }
  const stepIdeal = Math.max(1, T - ov)
  const N = Math.max(2, Math.ceil((total - T) / stepIdeal) + 1)
  const inGap = (x) => gaps.some(g => x >= g.a - EPS && x <= g.b + EPS)
  const snapIn = (ideal, lo, hi, prefer) => {
    if (inGap(ideal)) return ideal
    let best = null
    for (const g of gaps) {
      for (const x of [g.a, g.b]) {
        if (x < lo - EPS || x > hi + EPS) continue
        if (best == null) { best = x; continue }
        const d = Math.abs(x - ideal), bd = Math.abs(best - ideal)
        if (d < bd - EPS || (Math.abs(d - bd) < EPS && prefer === 'min' && x < best)) best = x
      }
    }
    return best == null ? ideal : best
  }
  const span = total - T
  const step = (T - ov)
  const starts = []
  for (let k = 0; k < N; k++) starts.push(start + (span * k) / (N - 1))
  starts[0] = start
  starts[N - 1] = end - T
  // 每个中间起点在「同时满足前后两页重叠 ≥ ov」的窗口内，就近吸附到节点空隙；
  // 窗口由理想等距位置夹出来，所以吸附后每相邻两页仍然至少重叠 ov（12mm）。
  for (let k = 1; k < N - 1; k++) {
    const lo = Math.max(starts[k - 1] + 1, starts[k + 1] - step)
    const hi = Math.min(starts[k - 1] + step, starts[k + 1] - 1)
    if (hi < lo) continue
    let best = null
    for (const g of gaps) {
      for (const x of [g.a, g.b]) {
        if (x < lo - EPS || x > hi + EPS) continue
        if (best == null || Math.abs(x - starts[k]) < Math.abs(best - starts[k])) best = x
      }
    }
    if (best != null) starts[k] = best
  }
  return { pages: starts.map(s => ({ s, e: Math.min(end, s + T) })), overflow: false }
}

// ===== 世代行 =====
// rows: [{ y0, y1, label }]（由各视图按自己的布局给出；缺省时页面只显示行列角标）
export function rowsFromNodes(items, { y = n => n.y, h = () => 100, label = n => n.sub || ('第' + n.generation + '世') } = {}) {
  const map = new Map()
  ;(items || []).forEach(n => {
    const key = label(n)
    const y0 = y(n)
    const y1 = y0 + h(n)
    const cur = map.get(key)
    if (!cur) map.set(key, { label: key, y0, y1 })
    else { cur.y0 = Math.min(cur.y0, y0); cur.y1 = Math.max(cur.y1, y1) }
  })
  return [...map.values()].sort((a, b) => a.y0 - b.y0)
}

function rowsIn(rows, y0, y1) {
  if (!rows || !rows.length) return []
  const hit = rows.filter(r => r.y1 > y0 + EPS && r.y0 < y1 - EPS)
  return hit
}

// ===== 网格坐标：列 A..Z,AA.. ；行 1..N =====
export function colName(i) {
  let n = i + 1
  let s = ''
  while (n > 0) {
    const m = (n - 1) % 26
    s = String.fromCharCode(65 + m) + s
    n = Math.floor((n - 1) / 26)
  }
  return s
}

// ===== 主算法：出分页方案 =====
// bounds: { x, y, w, h }（内容坐标系；内纪/外纪裁剪也走这里）
// boxes : [{x,y,w,h}] 节点/文字包围盒（内容坐标系）
export function computePlan({ contentW, contentH, boxes = [], rows = [], opts = {}, bounds = null }) {
  const o = { ...DEFAULT_OPTS, ...(opts || {}) }
  const m = pageMetrics(o)
  const b = bounds && bounds.w > 0 && bounds.h > 0
    ? { x: bounds.x, y: bounds.y, w: bounds.w, h: bounds.h }
    : { x: 0, y: 0, w: contentW, h: contentH }
  const warnings = []
  const pad = 2 // 内容像素：给包围盒加的安全距（约 0.5mm@100%）

  if (o.mode === 'scroll') {
    const preset = SCROLL_PRESETS.find(p => p.value === o.scrollPreset) || SCROLL_PRESETS[1]
    const horizontal = o.scrollAxis === 'h'
    const padY = 8, headerMm = 10, footerMm = 10, side = 10
    const crossMm = preset.width // 短边（297/420mm）
    // 纵向卷轴：宽 = 短边，长度沿 y；横向卷轴：高 = 短边，长度沿 x
    const crossArea = crossMm - padY * 2 - headerMm - footerMm
    let scale, bodyW, bodyH, needed, pageW, pageH
    if (horizontal) {
      scale = crossArea / Math.max(1, b.h)   // 高度方向铺满
      bodyW = b.w * scale
      bodyH = crossArea
      needed = Math.ceil(side * 2 + bodyW)
      pageW = preset.length > 0 ? Math.max(preset.length, needed) : needed
      pageH = crossMm
    } else {
      scale = (crossMm - side * 2) / Math.max(1, b.w) // 宽度方向铺满
      bodyW = crossMm - side * 2
      bodyH = b.h * scale
      needed = Math.ceil(padY * 2 + headerMm + footerMm + bodyH)
      pageW = crossMm
      pageH = preset.length > 0 ? Math.max(preset.length, needed) : needed
    }
    const metrics = {
      paperW: pageW, paperH: pageH, landscape: pageW >= pageH,
      padY, headerMm, footerMm, left: side, right: side, bindLeft: 0, bindRight: 0,
      bodyTop: padY + headerMm,
      areaW: pageW - side * 2,
      areaH: pageH - padY * 2 - headerMm - footerMm
    }
    const page = {
      idx: 0, pageNo: 1, kind: 'scroll',
      crop: { x: b.x, y: b.y, w: b.w, h: b.h },
      genLabels: rows.map(r => r.label), genFrom: rows[0] ? rows[0].label : '', genTo: rows.length ? rows[rows.length - 1].label : '',
      bodyWmm: r3(bodyW), bodyHmm: r3(bodyH),
      pageWmm: pageW, pageHmm: pageH,
      overflow: needed > (preset.length || 0) ? Math.max(0, needed - preset.length) : 0
    }
    if (preset.length > 0 && needed > preset.length) {
      warnings.push(`内容需要 ${needed}mm，超过预设 ${preset.length}mm；已按 ${pageH}mm 自动加长（选「自动加长」可避免提示）`)
    }
    return {
      mode: 'scroll', opts: o, metrics,
      scale, scalePct: r3(scale / MM_PER_PX * 100), mmPerPx: scale,
      bounds: b, cols: 1, rows: 1, xCuts: [b.x, b.x + b.w], yCuts: [b.y, b.y + b.h],
      mapPages: [page], pages: [page], indexPage: null,
      totalPages: 1, mapPageCount: 1, warnings,
      scroll: { width: pageW, length: pageH, bodyW, bodyH, needed, preset: preset.value, axis: horizontal ? 'h' : 'v' }
    }
  }

  if (o.mode === 'single') {
    const scale = Math.min(m.areaW / Math.max(1, b.w), m.areaH / Math.max(1, b.h))
    const page = {
      idx: 0, pageNo: 1, kind: 'single',
      crop: { x: b.x, y: b.y, w: b.w, h: b.h },
      genLabels: rows.map(r => r.label), genFrom: rows[0] ? rows[0].label : '', genTo: rows.length ? rows[rows.length - 1].label : '',
      bodyWmm: r3(b.w * scale), bodyHmm: r3(b.h * scale),
      grid: '', row: 1, col: 1, rowCount: 1, colCount: 1
    }
    return {
      mode: 'single', opts: o, metrics: m,
      scale, scalePct: r3(scale / MM_PER_PX * 100), mmPerPx: scale,
      bounds: b, cols: 1, rows: 1, xCuts: [b.x, b.x + b.w], yCuts: [b.y, b.y + b.h],
      mapPages: [page], pages: [page], indexPage: null,
      totalPages: 1, mapPageCount: 1, warnings
    }
  }

  // ---- 分页（瓦片）----
  // 「切满纸面」：先算内容/纸面能凑出的 (列 N, 行 M, 比例 s)，再用真实空隙切页逐档评估，
  // 选「每页覆盖率高、张数不失控」的那一档；用户也可以钉死 50/75/100/125/150 中的某一档。
  const ovMm = clamp(Number(o.overlapMm) || 0, 0, 40)
  const ovEffMm = Math.min(ovMm, m.areaW * 0.08, m.areaH * 0.08)
  const baseScale = MM_PER_PX * clamp(Number(o.scalePct) || 100, 10, 400) / 100
  const xIx = projectBoxes(boxes, 'x', pad, b.x, b.x + b.w)
  const yIx = projectBoxes(boxes, 'y', pad, b.y, b.y + b.h)
  const xGaps = freeGaps(xIx, b.x, b.x + b.w)
  const yGaps = freeGaps(yIx, b.y, b.y + b.h)

  // 给定比例 → 一套真实裁剪（每页整页、页缘落在空隙、末页回借）+ 每页覆盖率/独有内容
  function geometryAt(scale) {
    const tileWpx = m.areaW / scale
    const tileHpx = m.areaH / scale
    const ovPx = ovEffMm / scale
    const px = tileAxis(b.x, b.x + b.w, xGaps, tileWpx, ovPx)
    const py = tileAxis(b.y, b.y + b.h, yGaps, tileHpx, ovPx)
    const cols = px.pages.length
    const rows = py.pages.length
    const pageAreaMm2 = m.areaW * m.areaH
    const cells = []
    let overflow = !!(px.overflow || py.overflow)
    let maxCropW = 0, maxCropH = 0
    for (let rr = 0; rr < rows; rr++) {
      for (let cc = 0; cc < cols; cc++) {
        const X = px.pages[cc]
        const Y = py.pages[rr]
        const crop = { x: X.s, y: Y.s, w: Math.max(0, X.e - X.s), h: Math.max(0, Y.e - Y.s) }
        maxCropW = Math.max(maxCropW, crop.w)
        maxCropH = Math.max(maxCropH, crop.h)
        const leftE = cc > 0 ? px.pages[cc - 1].e : null
        const aboveE = rr > 0 ? py.pages[rr - 1].e : null
        // 阅读顺序下「本页新出现的内容」= 右邻与下邻都没覆盖到的区域
        const ux = Math.max(0, X.e - Math.max(leftE == null ? X.s : leftE, X.s))
        const uy = Math.max(0, Y.e - Math.max(aboveE == null ? Y.s : aboveE, Y.s))
        const fillW = (crop.w * scale) / m.areaW
        const fillH = (crop.h * scale) / m.areaH
        cells.push({
          rr, cc,
          tile: { x: Math.max(leftE == null ? X.s : leftE, X.s), y: Math.max(aboveE == null ? Y.s : aboveE, Y.s),
                  w: ux, h: uy },
          crop,
          fillW,
          fillH,
          fillArea: Math.min(1, fillW * fillH),            // 含重叠：这张纸被内容占了多少
          uniqueArea: Math.min(1, (ux * uy * scale * scale) / pageAreaMm2), // 独有内容：首次出现的内容占比
          overlapXmm: leftE == null ? null : Math.max(0, leftE - X.s) * scale,
          overlapYmm: aboveE == null ? null : Math.max(0, aboveE - Y.s) * scale,
          anchored: !!(X.anchored || Y.anchored)
        })
      }
    }
    const contentWmm = b.w * scale
    const contentHmm = b.h * scale
    const coverage = (contentWmm * contentHmm) / Math.max(1, cells.length * pageAreaMm2)
    const lastColWmm = Math.max(...cells.filter(c => c.cc === cols - 1).map(c => c.crop.w)) * scale
    const lastRowHmm = Math.max(...cells.filter(c => c.rr === rows - 1).map(c => c.crop.h)) * scale
    return {
      scale, tileWpx, tileHpx, ovPx,
      cols, rows, cells, pages: cells.length,
      xPages: px.pages, yPages: py.pages,
      maxCropW, maxCropH,
      contentWmm, contentHmm, coverage,
      lastColWmm, lastRowHmm,
      blankRightMm: Math.max(0, m.areaW - lastColWmm),
      blankBottomMm: Math.max(0, m.areaH - lastRowHmm),
      overflow
    }
  }

  // 固定比例：每页都按整页切；只有「节点比一页还宽」时才逐档缩小
  function geometryFixed(scale0) {
    let sc = scale0
    let g = geometryAt(sc)
    for (let i = 0; i < 8 && g.overflow; i++) {
      const need = Math.max(g.maxCropW / g.tileWpx, g.maxCropH / g.tileHpx, 1.02)
      sc = sc / need
      g = geometryAt(sc)
    }
    // 内容只比一页宽一点点（1.0~1.18 页）：两列会几乎重复印两遍 → 把比例缩到刚好一列
    const contentWmm0 = b.w * g.scale
    if (g.cols === 2 && contentWmm0 > m.areaW && contentWmm0 < m.areaW * 1.18) {
      const s2 = m.areaW / b.w
      const g2 = geometryAt(s2)
      if (!g2.overflow && g2.cols === 1 && g2.pages <= g.pages) {
        g2.snappedFrom = r3(scale0 / MM_PER_PX * 100)
        g = g2
      }
    }
    return g
  }

  // 自动：在 40%~200% 内细扫 + 加入「N 列 × M 行刚好填满」的解析解，
  // 取「每页覆盖率最高」的一档；auto-big 只允许不低于 100%（字更大），auto-save 不超过 100% 时的张数。
  function geometryAuto(target) {
    const baseG = geometryAt(baseScale)
    const pagesBase = Math.max(1, baseG.pages)
    const pageLimit = target === 'big' ? Math.max(4, Math.round(pagesBase * 2)) : pagesBase
    // 自动模式不把字缩到看不清：低于 65%（约 6pt）的方案不参与「切满」评选；50% 等更小比例需用户显式选
    const pctMin = target === 'big' ? 100 : 65
    const seen = new Set()
    const cands = []
    const add = (pct) => {
      if (!(pct >= pctMin - 1e-9 && pct <= 200)) return
      const key = pct.toFixed(2)
      if (seen.has(key)) return
      seen.add(key)
      const g = geometryAt(baseScale * pct / 100)
      if (g.overflow) return
      if (g.pages > pageLimit) return
      cands.push({ pct, g })
    }
    for (let pct = 40; pct <= 200; pct += 0.5) add(pct)
    for (let N = 1; N <= 8; N++) {
      for (let M = 1; M <= 12; M++) {
        const sw = (N * m.areaW - (N - 1) * ovEffMm) / Math.max(1, b.w)
        const sh = (M * m.areaH - (M - 1) * ovEffMm) / Math.max(1, b.h)
        add(Math.min(sw, sh) / MM_PER_PX * 100)
      }
    }
    if (!cands.length) return { g: geometryFixed(baseScale), auto: null }
    // 评分：覆盖率为主，同时惩罚「最右列 / 最下行」里最难看的那条空白带
    const scoreOf = (g) => g.coverage - 0.35 * Math.max(g.blankRightMm / m.areaW, g.blankBottomMm / m.areaH)
    cands.forEach(c => { c.score = scoreOf(c.g) })
    cands.sort((a, c) => (c.score - a.score) || (c.pct - a.pct))
    const best = cands[0]
    const alt = cands.find(c => c.g.pages !== best.g.pages && c.g.coverage >= best.g.coverage - 0.03) || null
    const shortlist = cands.slice(0, 6).map(c => ({
      pct: r3(c.pct), pages: c.g.pages, cols: c.g.cols, rows: c.g.rows,
      coverage: r3(c.g.coverage), blankRightMm: r3(c.g.blankRightMm), blankBottomMm: r3(c.g.blankBottomMm),
      score: r3(c.score)
    }))
    return {
      g: best.g,
      auto: {
        target,
        pct: best.pct,
        coverage: best.g.coverage,
        pages: best.g.pages,
        pagesBase,
        baseCoverage: baseG.coverage,
        score: best.score,
        alt: alt ? { pct: alt.pct, pages: alt.g.pages, coverage: alt.g.coverage } : null,
        shortlist
      }
    }
  }

  let chosen = null
  let autoInfo = null
  const scaleMode = o.scaleMode || 'fixed'
  if (scaleMode === 'auto-big' || scaleMode === 'auto-save') {
    const r = geometryAuto(scaleMode === 'auto-big' ? 'big' : 'save')
    chosen = r.g
    autoInfo = r.auto
  } else {
    chosen = geometryFixed(baseScale)
  }
  const scale = chosen.scale
  const xPages = chosen.xPages
  const yPages = chosen.yPages
  const cols = chosen.cols
  const rowCount = chosen.rows
  const ovPx = chosen.ovPx
  if (chosen.overflow) warnings.push('有节点比一页还宽，已自动缩小内容比例')

  const mapPages = []
  const indexPage = o.includeIndex
    ? { idx: 0, pageNo: 1, kind: 'index', grid: '', row: 0, col: 0, rowCount, colCount: cols, crop: { ...b }, genLabels: rows.map(r => r.label), genFrom: rows[0] ? rows[0].label : '', genTo: rows.length ? rows[rows.length - 1].label : '' }
    : null
  const pageBase = indexPage ? 1 : 0
  for (const cell of chosen.cells) {
    const { rr, cc } = cell
    const hitRows = rowsIn(rows, cell.crop.y, cell.crop.y + cell.crop.h)
    mapPages.push({
      idx: (indexPage ? 1 : 0) + rr * cols + cc,
      pageNo: pageBase + rr * cols + cc + 1,
      kind: 'map',
      row: rr + 1, col: cc + 1, rowCount, colCount: cols,
      grid: colName(cc) + (rr + 1),
      tile: cell.tile,
      crop: cell.crop,
      bodyWmm: r3(cell.crop.w * scale),
      bodyHmm: r3(cell.crop.h * scale),
      fill: {
        area: r3(cell.fillArea),
        w: r3(cell.fillW),
        h: r3(cell.fillH),
        unique: r3(cell.uniqueArea)
      },
      overlapXmm: cell.overlapXmm == null ? null : r3(cell.overlapXmm),
      overlapYmm: cell.overlapYmm == null ? null : r3(cell.overlapYmm),
      anchored: !!cell.anchored,
      genLabels: hitRows.map(r => r.label),
      genFrom: hitRows.length ? hitRows[0].label : '',
      genTo: hitRows.length ? hitRows[hitRows.length - 1].label : '',
      seam: {
        left: cc > 0 ? { pageNo: pageBase + rr * cols + (cc - 1) + 1, grid: colName(cc - 1) + (rr + 1), at: cell.crop.x } : null,
        right: cc < cols - 1 ? { pageNo: pageBase + rr * cols + cc + 2, grid: colName(cc + 1) + (rr + 1), at: cell.crop.x + cell.crop.w } : null,
        top: rr > 0 ? { pageNo: pageBase + (rr - 1) * cols + cc + 1, grid: colName(cc) + rr, at: cell.crop.y } : null,
        bottom: rr < rowCount - 1 ? { pageNo: pageBase + (rr + 1) * cols + cc + 1, grid: colName(cc) + (rr + 2), at: cell.crop.y + cell.crop.h } : null
      }
    })
  }
  const pages = indexPage ? [indexPage, ...mapPages] : mapPages
  // 最后一页是「往回借」出来的整页：算出它与相邻页的实际重叠，过大就提示
  const lastMap = mapPages[mapPages.length - 1]
  const lastOvX = lastMap && lastMap.overlapXmm != null ? lastMap.overlapXmm : 0
  const lastOvY = lastMap && lastMap.overlapYmm != null ? lastMap.overlapYmm : 0
  if (lastMap && (lastOvX > m.areaW * 0.5 || lastOvY > m.areaH * 0.5)) {
    warnings.push(`最后一页为切满整页，与相邻页重叠 ${Math.max(lastOvX, lastOvY).toFixed(0)}mm（超过半页）；重复较多但每页都满`)
  }
  if (scale < MM_PER_PX * 30 / 100) warnings.push('内容比例已缩到 30% 以下，字号可能偏小，建议改用更大的纸（A3）或分页')
  if (cols * rowCount > 60) warnings.push(`共 ${cols * rowCount} 张图页，数量较多，打印前请注意张数`)

  return {
    mode: 'tile', opts: o, metrics: m,
    scale, scalePct: r3(scale / MM_PER_PX * 100), mmPerPx: scale,
    scaleMode,
    snappedFrom: chosen.snappedFrom || null,
    auto: autoInfo,
    coverage: r3(chosen.coverage),
    contentWmm: r3(chosen.contentWmm), contentHmm: r3(chosen.contentHmm),
    blankRightMm: r3(chosen.blankRightMm), blankBottomMm: r3(chosen.blankBottomMm),
    lastColWmm: r3(chosen.lastColWmm), lastRowHmm: r3(chosen.lastRowHmm),
    bounds: b, cols, rows: rowCount, xPages, yPages,
    overlapMm: ovMm, overlapMmEff: r3(ovEffMm), overlapPx: r3(ovPx),
    lastOverlapXmm: r3(lastOvX), lastOverlapYmm: r3(lastOvY),
    mapPages, indexPage, pages,
    totalPages: pages.length, mapPageCount: mapPages.length,
    warnings
  }
}

export function planTitle(plan) {
  if (!plan) return ''
  const o = plan.opts
  if (plan.mode === 'scroll') return `卷轴单页 ${plan.scroll.width} × ${plan.scroll.length}mm`
  return `预计 ${PAPER_LABEL[o.paper]} ${ORIENT_LABEL[o.orientation]} ${plan.totalPages} 张`
}
