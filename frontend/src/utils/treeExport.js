import { getWatermarkLevel, drawCanvasWatermark, watermarkSvgFragment, watermarkCssFragment } from './watermark'
import { subsetTtf, bytesToBase64, FontError } from './fontSubset'
import { treeFont, treeFontIsCustom, isNonTtfFont, fontExt } from './treeFonts'
import { computePlan, pageMetrics } from './printPaging'
import { buildPrintDoc } from './printHtml'
// 树图导出工具：印刷级高清 PNG（300 DPI）+ 矢量 SVG（无限清晰，印刷厂可任意放大）
// 树图本身是纯矢量 SVG，旧逻辑按屏幕 96 DPI 1:1 光栅化，打大图会糊。
// 高清 PNG：用 viewBox 让矢量按目标像素光栅化；SVG：导出自包含文件（头像内联为 data URI）。

// 300 DPI 相对 96 DPI 屏幕的倍率
const PRINT_SCALE = 300 / 96
// 画布单边最大像素（浏览器 canvas 上限通常 8192/16384，留余量并控制内存）
const MAX_CANVAS_PX = 14000

function fetchAsDataURL(url) {
  return fetch(url, { credentials: 'include' })
    .then(r => r.blob())
    .then(b => new Promise((resolve, reject) => {
      const fr = new FileReader()
      fr.onload = () => resolve(fr.result)
      fr.onerror = reject
      fr.readAsDataURL(b)
    }))
}

// 把克隆 SVG 里所有 <image> 的外链头像（/uploads/xxx）内联为 data URI，
// 使导出的 SVG/位图脱离当前视图也能完整显示印刷。已是 data URI 的跳过。
async function inlineImages(clone) {
  const imgs = Array.from(clone.querySelectorAll('image'))
  await Promise.all(imgs.map(async im => {
    const href = im.getAttribute('href') || im.getAttribute('xlink:href') || ''
    if (!href || href.startsWith('data:')) return
    try {
      const data = await fetchAsDataURL(href) // 相对路径同源，fetch 按当前 origin 解析
      im.setAttribute('href', data)
      im.removeAttribute('xlink:href')
    } catch (e) {
      // 个别头像拉取失败不阻塞导出（该头像留空，不影响线条文字）
    }
  }))
}

function serializeSvg(clone) {
  clone.setAttribute('xmlns', 'http://www.w3.org/2000/svg')
  clone.setAttribute('xmlns:xlink', 'http://www.w3.org/1999/xlink')
  return new XMLSerializer().serializeToString(clone)
}

// 准备克隆 SVG：重置视图平移/缩放为恒等，用 viewBox 裁剪到内容区域。
// crop = { x, y, w, h }（内容坐标）；不传则取整幅 contentW × contentH。
// 把「画面上实际生效的 font-family」内联到克隆的 text/tspan 上。
// 视图里的字体是用 CSS 变量 --tree-font 施加的，而克隆出来的 SVG 一旦离开本页面
// （PNG 走 <img> 光栅化、SVG/PDF 是独立文件），变量不会解析，所以必须内联成具体字体族。
function inlineFontFamily(srcSvg, clone) {
  const src = srcSvg.querySelectorAll('text, tspan')
  const dst = clone.querySelectorAll('text, tspan')
  if (src.length !== dst.length) return   // 结构不一致就不动，保持原样
  for (let i = 0; i < dst.length; i++) {
    let fam = ''
    try { fam = getComputedStyle(src[i]).fontFamily || '' } catch (e) { fam = '' }
    if (fam) dst[i].setAttribute('font-family', fam)
  }
}

// 树图字体档：clone 前把字体内联，并在选了「上传字体」时把该字体子集内嵌进去，
// 这样 PNG/PDF/SVG 三种导出在别的机器上都与画面一致。
async function applyTreeFont(clone, opts) {
  if (!opts || !opts.treeFont) return null
  if (!treeFontIsCustom.value) return null      // 系统字体：本地渲染即可，无法内嵌（由「内嵌字体」档提示）
  try {
    return await embedSubsetFont(clone)
  } catch (e) {
    // 上传的是 woff2/otf 之类：不能内嵌，但 PNG/PDF/普通 SVG 仍要能导出（文字退回系统字体）
    if (e && (e.code === 'FONT_UNSUPPORTED' || e.code === 'FONT_NONE')) return null
    throw e
  }
}

function prepClone(srcSvg, contentW, contentH, crop, opts) {
  const c = crop || { x: 0, y: 0, w: contentW, h: contentH }
  const clone = srcSvg.cloneNode(true)
  const g = clone.querySelector('g')
  if (g) g.setAttribute('transform', 'translate(0,0) scale(1)')
  clone.setAttribute('viewBox', `${c.x} ${c.y} ${c.w} ${c.h}`)
  if (opts && opts.treeFont && treeFont.value && treeFont.value.stack) inlineFontFamily(srcSvg, clone)
  return { clone, crop: c }
}

// 导出高清 PNG（默认 300 DPI；scale 可传 600/96 等更高倍率）。bg 为画布底色。
// crop 可选 {x,y,w,h}（内纪/外纪纵向裁剪用）。
// scale 可选：默认 PRINT_SCALE（300/96），向后兼容；无论传多大都会按 MAX_CANVAS_PX 上限自动降倍率。
export async function exportHighResPng(srcSvg, contentW, contentH, filename, bg, crop, scaleOpt, opts) {
  const { clone, crop: c } = prepClone(srcSvg, contentW, contentH, crop, opts)
  // 目标像素：裁剪尺寸 × 打印倍率；超大树按 MAX_CANVAS_PX 等比收缩
  let scale = Number(scaleOpt) > 0 ? Number(scaleOpt) : PRINT_SCALE
  const maxSide = Math.max(c.w, c.h) * scale
  if (maxSide > MAX_CANVAS_PX) scale = MAX_CANVAS_PX / Math.max(c.w, c.h)
  const outW = Math.round(c.w * scale)
  const outH = Math.round(c.h * scale)
  clone.setAttribute('width', outW)
  clone.setAttribute('height', outH)

  await applyTreeFont(clone, opts)
  await inlineImages(clone)
  const xml = serializeSvg(clone)
  const url = URL.createObjectURL(new Blob([xml], { type: 'image/svg+xml;charset=utf-8' }))
  const img = new Image()
  await new Promise((resolve, reject) => { img.onload = resolve; img.onerror = reject; img.src = url })

  const canvas = document.createElement('canvas')
  canvas.width = outW; canvas.height = outH
  const ctx = canvas.getContext('2d')
  ctx.fillStyle = bg || '#f7f3ec'
  ctx.fillRect(0, 0, outW, outH)
  ctx.drawImage(img, 0, 0, outW, outH)
  drawCanvasWatermark(ctx, outW, outH, getWatermarkLevel())
  URL.revokeObjectURL(url)

  const a = document.createElement('a')
  a.href = canvas.toDataURL('image/png')
  a.download = filename
  a.click()
  return { width: outW, height: outH }
}

// 导出矢量 PDF：走浏览器打印管线（对 SVG 是矢量输出，中文/竖排用系统字体，印刷质量最好；
// jsPDF/svg2pdf 嵌中文字体麻烦且竖排支持差，故用打印为 PDF）。
// 按裁剪区物理尺寸（96dpi → mm）设定 @page 纸张，margin:0，用户在打印对话框选「另存为 PDF」。
export async function exportPrintPDF(srcSvg, contentW, contentH, filename, bg, crop, opts) {
  const { clone, crop: c } = prepClone(srcSvg, contentW, contentH, crop, opts)
  clone.setAttribute('width', c.w)
  clone.setAttribute('height', c.h)
  // 同步开窗口，避免 await 后被弹窗拦截
  const win = window.open('', '_blank')
  if (!win) throw new Error('浏览器拦截了弹出窗口，请允许弹出后重试')
  await applyTreeFont(clone, opts)
  await inlineImages(clone)
  let svgXml = serializeSvg(clone)
  const wmFrag2 = watermarkSvgFragment(getWatermarkLevel())
  if (wmFrag2) {
    svgXml = svgXml.replace('</svg>', wmFrag2 + '</svg>')
  }
  const wmCss = watermarkCssFragment(getWatermarkLevel())
  // CSS 像素(96dpi) → 物理毫米
  const wmm = (c.w / 96) * 25.4
  const hmm = (c.h / 96) * 25.4
  const html = `<!DOCTYPE html><html><head><meta charset="utf-8"><title>${filename}</title>
<style>
  @page { size: ${wmm.toFixed(1)}mm ${hmm.toFixed(1)}mm; margin: 0; }
  html,body { margin:0; padding:0; background:${bg || '#f7f3ec'}; }
  body { width:${wmm.toFixed(1)}mm; height:${hmm.toFixed(1)}mm; }
  svg { display:block; width:100%; height:100%; }
  ${wmCss}
</style></head>
<body>${svgXml}
<script>
  window.onload = function(){ setTimeout(function(){ try{ window.focus(); window.print(); }catch(e){} }, 300); };
<\/script></body></html>`
  win.document.open()
  win.document.write(html)
  win.document.close()
}

// 导出矢量 SVG（线条/文字无限清晰，适合印刷放大）。头像内联，文件自包含。
// opts: { withPortrait?: boolean, withFont?: boolean }
//   withPortrait —— 调用方需先把头像切到「原图」状态并等图片就绪；这里只声明意图并做一次校验，
//                   随后仍由 inlineImages() 把 <image> 内联为 data URI。
//   withFont     —— 取服务器上「已上传字体」，按图上实际用到的字符做**子集化**后以 @font-face
//                   内嵌（整包中文字体几 MB，子集后几百 KB）。系统字体（楷体/宋体）浏览器读不到
//                   字体文件，无法内嵌 → 抛 FontError('FONT_NONE') 由调用方明确提示。
// 返回 { size, withPortrait, font }，font = { family, chars, missing, fontBytes, label } | null
export async function exportVectorSvg(srcSvg, contentW, contentH, filename, crop, opts) {
  const o = opts || {}
  const { clone, crop: c } = prepClone(srcSvg, contentW, contentH, crop, o)
  clone.setAttribute('width', c.w)
  clone.setAttribute('height', c.h)
  if (o.withPortrait && clone.querySelectorAll('image').length === 0) {
    throw new FontError('PORTRAIT_NONE', '当前视图没有可内嵌的头像（头像未加载），请先让头像显示出来再导出')
  }
  let font = null
  if (o.withFont) font = await embedSubsetFont(clone, { requireCustom: true })   // 显式「内嵌字体」档：系统字体时抛 FONT_NONE
  else font = await applyTreeFont(clone, o)                    // 选了上传字体的其它 SVG 档：静默内嵌，保证跨机器一致
  await inlineImages(clone)
  let svgContent = serializeSvg(clone)
  const wmFrag = watermarkSvgFragment(getWatermarkLevel())
  if (wmFrag) {
    // 在 </svg> 前插入水印
    svgContent = svgContent.replace('</svg>', wmFrag + '</svg>')
  }
  const xml = '<?xml version="1.0" encoding="UTF-8"?>\n' + svgContent
  const blob = new Blob([xml], { type: 'image/svg+xml;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  a.click()
  setTimeout(() => URL.revokeObjectURL(url), 1000)
  return { size: blob.size, withPortrait: !!o.withPortrait, font }
}

// 应用用 axios 实例带 Bearer token（不是 cookie），这里 raw fetch 必须自己带上，否则 401 会被误判成「没有上传字体」
function authHeaders() {
  try {
    const t = localStorage.getItem('token')
    return t ? { Authorization: 'Bearer ' + t } : {}
  } catch (e) { return {} }
}

// 「内嵌字体」不可用时的统一提示（系统字体浏览器读不到字体文件，不能静默出一个「以为带了其实没带」的文件）
export const SVG_FONT_NO_UPLOAD_MSG =
  '当前使用系统字体（楷体/宋体），浏览器读不到系统字体文件，无法内嵌；如需字形随文件走，请用 PDF 档（打印时自动内嵌），或先在「牒记预览」页上传字体后再导出'

// 取服务器上已上传的字体：优先选导出图里正在用的族，否则取第一个
async function pickUploadedFont(clone, opts) {
  const o = opts || {}
  // 显式「内嵌字体」档：当前选的是系统字体（内置档）时必须明确提示，不能拿别的上传字体顶上
  const cur0 = treeFont.value
  if (o.requireCustom && !(cur0 && cur0.custom)) throw new FontError('FONT_NONE', SVG_FONT_NO_UPLOAD_MSG)
  let list = []
  try {
    const r = await fetch('/api/fonts', { credentials: 'include', headers: authHeaders() })
    if (r.ok) list = await r.json()
  } catch (e) { /* 网络异常按「没有上传字体」处理 */ }
  if (!Array.isArray(list) || !list.length) throw new FontError('FONT_NONE', SVG_FONT_NO_UPLOAD_MSG)
  // ① 当前树图选中的上传字体优先
  const cur = treeFont.value
  if (cur && cur.custom && cur.family) {
    const hit = list.find(f => f.family === cur.family)
    if (hit) return hit
    throw new FontError('FONT_UNSUPPORTED', `当前选中的上传字体「${cur.label || cur.family}」在服务器上已不存在，请重新上传或改选其它字体`)
  }
  const used = new Set()
  clone.querySelectorAll('text, tspan').forEach(t => {
    String(t.getAttribute('font-family') || '').split(',').forEach(s => {
      const v = s.trim().replace(/^["']|["']$/g, '')
      if (v) used.add(v)
    })
  })
  return list.find(f => used.has(f.family)) || list[0]
}

// 收集图上所有文字（含即将插入的水印文案）→ 子集化 → 注入 @font-face 并统一定向到内嵌族
async function embedSubsetFont(clone, opts) {
  const info = await pickUploadedFont(clone, opts)
  // 非 TTF（woff2/otf/ttc）特殊提示：不是笼统的「无法内嵌」，而是告诉用户补一份 TTF
  if (isNonTtfFont(info)) {
    const ext = fontExt(info.name || info.url)
    throw new FontError('FONT_UNSUPPORTED', `该字体是 ${ext}，暂不支持子集内嵌；如需导出内嵌字体的 SVG，请额外上传一份 TTF 版本`)
  }
  const resp = await fetch(encodeURI(info.url), { credentials: 'include', headers: authHeaders() })
  if (!resp.ok) throw new FontError('FONT_UNSUPPORTED', `字体「${info.label || info.name}」读取失败，无法内嵌`)
  const buf = await resp.arrayBuffer()
  const textChars = new Set()
  const subsetChars = new Set()
  const addText = (t, onlyText) => { for (const ch of String(t || '')) { subsetChars.add(ch); if (onlyText) textChars.add(ch) } }
  clone.querySelectorAll('text, tspan').forEach(t => addText(t.textContent, true))
  const wmFrag = watermarkSvgFragment(getWatermarkLevel())
  if (wmFrag) {
    try {
      const d = new DOMParser().parseFromString('<svg xmlns="http://www.w3.org/2000/svg">' + wmFrag + '</svg>', 'image/svg+xml')
      d.querySelectorAll('text, tspan').forEach(t => addText(t.textContent, true))
    } catch (e) { /* 水印解析失败不影响正文子集 */ }
  }
  // 数字/字母/标点兜底：这些字如果字体里有就该嵌进去，否则会退化成系统字体（不算「缺字」）
  addText('0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz .,:;·—、。()（）', false)
  const sub = subsetTtf(buf, subsetChars)
  const missing = sub.missing.filter(ch => textChars.has(ch))   // 只报正文真正用到、而字体里没有的字
  const family = 'ZupuSubsetFont'
  const style = clone.ownerDocument.createElementNS('http://www.w3.org/2000/svg', 'style')
  style.textContent = `@font-face{font-family:'${family}';src:url(data:font/ttf;base64,${bytesToBase64(sub.data)}) format('truetype');font-weight:normal;font-style:normal;}` +
    `text,tspan{font-family:'${family}',serif;}`
  clone.insertBefore(style, clone.firstChild)
  return { family, label: info.label || info.name, chars: sub.keptChars, missing: missing.length, missingChars: missing.slice(0, 12), fontBytes: sub.data.length, glyphs: sub.numGlyphs }
}

// ============================================================================
// 世系图「分页打印 / 卷轴」——矢量切页（A3/A4、横竖、索引页、重叠）
//
// 与上面几种导出的关系：PNG/SVG/PDF 三种既有导出完全保留、行为不变；
// 这里新增的是「打印稿」：屏幕预览照旧整张显示，只有打印文档按纸型切成多页。
// 分页算法在 utils/printPaging.js（纯计算），打印文档在 utils/printHtml.js（纯字符串），
// 本文件负责把它们接到真实 DOM 上：收集节点包围盒 → 克隆矢量内容 → 逐页 viewBox 裁剪。
// ============================================================================

// 从内容 <g> 的 transform 里读出 pan/zoom（各视图都是 translate(x,y) scale(z)）
function readPanZoom(svgEl) {
  let g = null
  for (const ch of svgEl.children) if (ch.tagName.toLowerCase() === 'g') { g = ch; break }
  const t = (g && g.getAttribute('transform')) || ''
  const m = /translate\(\s*(-?[\d.eE+]+)[,\s]+(-?[\d.eE+]+)\s*\)\s*scale\(\s*(-?[\d.eE+]+)\s*\)/.exec(t)
  if (!m) return { g, pan: { x: 0, y: 0, zoom: 1 } }
  return { g, pan: { x: parseFloat(m[1]), y: parseFloat(m[2]), zoom: parseFloat(m[3]) || 1 } }
}

// 收集所有「实体」的包围盒（内容坐标系）：头像框 / 圆圈 / 头像图片 / 姓名与字辈文字。
// 连线（path/line）**不算**障碍物 —— 允许从连接线中间切开，靠重叠带保证跨缝的线两张都在。
// 这样切页线只会落在节点之间的空隙，任何节点/名字都不会被切两半。
export function collectTreeBoxes(svgEl) {
  if (!svgEl) return []
  const sr = svgEl.getBoundingClientRect()
  const { pan } = readPanZoom(svgEl)
  const z = pan.zoom || 1
  const out = []
  const els = svgEl.querySelectorAll('rect, circle, image, text, ellipse')
  for (const el of els) {
    if (el.closest('defs, clipPath, mask, marker, pattern, symbol')) continue
    const tag = el.tagName.toLowerCase()
    if (tag === 'text' || tag === 'tspan') {
      if (!String(el.textContent || '').trim()) continue
    }
    let r
    try { r = el.getBoundingClientRect() } catch (e) { continue }
    if (!r || r.width <= 0 || r.height <= 0) continue
    out.push({
      x: (r.left - sr.left - pan.x) / z,
      y: (r.top - sr.top - pan.y) / z,
      w: r.width / z,
      h: r.height / z,
      tag
    })
  }
  return out
}

// 把源 SVG 里「实际生效的文字样式」内联成属性。
// 视图里的字号/字重/字体来自 SFC scoped CSS（.node-name 等），克隆件离开本页面后这些规则不再存在，
// 不内联就会出现「打印出来的名字比屏幕大一圈、互相压字」。
const TEXT_STYLE_PROPS = [
  'font-size', 'font-family', 'font-weight', 'font-style', 'fill', 'fill-opacity', 'stroke', 'stroke-width',
  'text-anchor', 'dominant-baseline', 'alignment-baseline', 'letter-spacing', 'writing-mode', 'opacity',
  'font-variant-numeric', 'direction'
]
function inlineTextStyles(srcSvg, clone) {
  const src = srcSvg.querySelectorAll('text, tspan')
  const dst = clone.querySelectorAll('text, tspan')
  if (src.length !== dst.length) return 0
  let n = 0
  for (let i = 0; i < dst.length; i++) {
    let cs
    try { cs = getComputedStyle(src[i]) } catch (e) { continue }
    if (!cs) continue
    for (const p of TEXT_STYLE_PROPS) {
      const v = cs.getPropertyValue(p)
      if (v) dst[i].setAttribute(p, v)
    }
    n++
  }
  return n
}

// 克隆矢量内容（不设 viewBox：逐页各自设），返回 { inner, docCss }
// inner   = <defs> + 内容 <g>（transform 已重置为恒等，坐标即内容坐标系）
// docCss  = 从克隆里抽出的 <style>（字体子集 @font-face 等），整份打印文档只放一次
async function prepPagedContent(srcSvg, opts) {
  const clone = srcSvg.cloneNode(true)
  const { g } = readPanZoom(clone)
  if (g) g.setAttribute('transform', 'translate(0,0) scale(1)')
  clone.removeAttribute('width')
  clone.removeAttribute('height')
  clone.removeAttribute('viewBox')
  clone.removeAttribute('style')
  const styled = inlineTextStyles(srcSvg, clone)
  if (opts && opts.treeFont) await applyTreeFont(clone, opts)
  let docCss = ''
  clone.querySelectorAll('style').forEach(st => {
    docCss += '\n' + (st.textContent || '')
    st.parentNode.removeChild(st)
  })
  await inlineImages(clone)
  // 水印：一层覆盖每页 viewBox 的半透明图案，跟随内容坐标
  const wmFrag = watermarkSvgFragment(getWatermarkLevel())
  const wm = wmFrag || ''
  const inner = Array.from(clone.childNodes).map(n => {
    if (n.nodeType === 1) return new XMLSerializer().serializeToString(n)
    return ''
  }).join('')
  return { inner: inner + wm, docCss, styled }
}

// 只算方案（对话框里「预计张数」实时显示用，不做克隆，很快）
export function planPagedPrint({ contentW, contentH, boxes, rows, bounds, opts }) {
  return computePlan({ contentW, contentH, boxes: boxes || [], rows: rows || [], opts, bounds })
}

// 生成打印稿 HTML（矢量、多页）
export async function buildPagedPrintHtml(srcSvg, plan, meta) {
  const { inner, docCss } = await prepPagedContent(srcSvg, { treeFont: true })
  return buildPrintDoc({ plan, meta: meta || {}, contentInner: inner, docCss })
}

// 打开打印视图（新窗口）。与既有 exportPrintPDF 一致：先同步开窗，避免 await 后被拦截。
// 分页 HTML 里已按选定纸型写好 @page（A3/A4 横竖 / 卷轴超长页），用户在打印对话框选「另存为 PDF」即得矢量 PDF。
export async function openPagedPrint(srcSvg, plan, meta) {
  const win = window.open('', '_blank')
  if (!win) throw new Error('浏览器拦截了弹出窗口，请允许弹出后重试')
  const html = await buildPagedPrintHtml(srcSvg, plan, meta)
  win.document.open()
  win.document.write(html)
  win.document.close()
  return html
}

// Debug/自测用：不弹窗，直接返回 HTML（页面里也可用来做「另存 HTML」）
export async function pagedPrintHtmlForTest(srcSvg, plan, meta) {
  return buildPagedPrintHtml(srcSvg, plan, meta)
}

export { pageMetrics }
