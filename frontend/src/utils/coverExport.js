// 封面/封底导出：HTML/CSS → 自包含 SVG(foreignObject) → 300DPI PNG / 矢量 SVG
//
// 封面是用 HTML/CSS 画出来的（渐变、竖排文字、::before/::after 装订线针眼等），不是 SVG；
// 于是把 .bk 整棵克隆，套进 <svg><foreignObject> 并内联整页 CSS，
// 由浏览器引擎真实渲染（竖排、渐变、伪元素都保留），再光栅化成高清 PNG。
//
// ⚠️ 关键：交给 <img> 的 SVG 必须用 **data: URL**。
//    实测 Chrome 对 blob: URL 的 foreignObject 图片会污染 canvas
//    （toBlob/toDataURL 抛 "Tainted canvases may not be exported"），data: URL 则正常。
//
// 不做水印：封面是用户自家印书的稿子（世系图导出才带「试用版」水印）。

const A4_H_300 = 3508   // A4 高 297mm @ 300DPI（封面按 A4 竖版 → 300DPI 输出，满版无白边）
const MAX_SIDE = 14000  // canvas 单边上限，留余量

// 汇总页面所有可读样式表（同源；跨域的跳过）
function collectCss() {
  let css = ''
  for (const sheet of Array.from(document.styleSheets)) {
    let rules
    try { rules = sheet.cssRules } catch (e) { continue }
    if (!rules) continue
    for (const r of Array.from(rules)) css += r.cssText + '\n'
  }
  return css
}

// UTF-8 安全 base64（btoa 不支持中文）
function utf8ToBase64(str) {
  const bytes = new TextEncoder().encode(str)
  let bin = ''
  for (let i = 0; i < bytes.length; i++) bin += String.fromCharCode(bytes[i])
  return btoa(bin)
}

// 用克隆元素 + 内联 CSS 构建自包含 SVG 字符串
function buildCoverSvg(el, w, h, css) {
  const clone = el.cloneNode(true)
  clone.style.transform = 'none'
  clone.style.margin = '0'
  return `<svg xmlns="http://www.w3.org/2000/svg" width="${w}" height="${h}" viewBox="0 0 ${w} ${h}">`
    + `<foreignObject x="0" y="0" width="${w}" height="${h}">`
    + `<div xmlns="http://www.w3.org/1999/xhtml" style="width:${w}px;height:${h}px;margin:0;padding:0;">`
    + `<style>${css.replace(/&/g, '&amp;').replace(/</g, '&lt;')}</style>`
    + clone.outerHTML
    + `</div></foreignObject></svg>`
}

function downloadBlob(blob, filename) {
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  document.body.appendChild(a)
  a.click()
  a.remove()
  setTimeout(() => URL.revokeObjectURL(url), 1500)
}

function loadImage(src) {
  return new Promise((resolve, reject) => {
    const img = new Image()
    img.onload = () => resolve(img)
    img.onerror = () => reject(new Error('SVG 光栅化失败'))
    img.src = src
  })
}

function sizeOf(el) {
  const w = el.offsetWidth || 820
  const h = el.offsetHeight || Math.round(w * 1.414)
  return { w, h }
}

// 导出高清 PNG（默认按 A4 竖版 300 DPI，满版无白边）
export async function exportCoverPng(el, filename) {
  const { w, h } = sizeOf(el)
  const svg = buildCoverSvg(el, w, h, collectCss())
  // 必须 data: URL，blob: 会污染 canvas
  const img = await loadImage('data:image/svg+xml;base64,' + utf8ToBase64(svg))
  let scale = A4_H_300 / h
  if (Math.max(w, h) * scale > MAX_SIDE) scale = MAX_SIDE / Math.max(w, h)
  const outW = Math.round(w * scale)
  const outH = Math.round(h * scale)
  const canvas = document.createElement('canvas')
  canvas.width = outW
  canvas.height = outH
  const ctx = canvas.getContext('2d')
  ctx.fillStyle = '#ffffff'
  ctx.fillRect(0, 0, outW, outH)
  ctx.drawImage(img, 0, 0, outW, outH)
  const blob = await new Promise(res => canvas.toBlob(res, 'image/png'))
  downloadBlob(blob, filename)
  return { width: outW, height: outH }
}

// 导出矢量 SVG（自包含；竖排文字/渐变保留，可无限放大）
export function exportCoverSvg(el, filename) {
  const { w, h } = sizeOf(el)
  const svg = '<?xml version="1.0" encoding="UTF-8"?>\n' + buildCoverSvg(el, w, h, collectCss())
  downloadBlob(new Blob([svg], { type: 'image/svg+xml;charset=utf-8' }), filename)
  return { width: w, height: h }
}

// 顺序导出多个文件时，两次下载之间留点间隔（避免浏览器把第二个当弹窗拦截）
export function sleep(ms) {
  return new Promise(r => setTimeout(r, ms))
}
