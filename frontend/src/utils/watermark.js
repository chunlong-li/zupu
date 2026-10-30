// 水印状态（全局共享，treeExport.js 读取）
let _level = 0

export function setWatermarkLevel(level) {
  _level = level
}

export function getWatermarkLevel() {
  return _level
}

// 在 2D canvas 上绘制斜向水印文字
export function drawCanvasWatermark(ctx, w, h, level) {
  if (!level || level <= 0) return
  const text = '试用版·来源系统'
  const configs = {
    1: { opacity: 0.45, gap: 400, font: 24 },
    2: { opacity: 0.75, gap: 240, font: 24 },
    3: { opacity: 1.0,  gap: 150, font: 24 },
  }
  const cfg = configs[level] || configs[1]
  ctx.save()
  ctx.globalAlpha = cfg.opacity
  ctx.fillStyle = '#333'
  ctx.font = `${cfg.font}px sans-serif`
  ctx.textAlign = 'center'
  ctx.textBaseline = 'middle'
  for (let y = -h; y < h * 2; y += cfg.gap) {
    for (let x = -w; x < w * 2; x += cfg.gap) {
      ctx.save()
      ctx.translate(x, y)
      ctx.rotate(-28 * Math.PI / 180)
      ctx.fillText(text, 0, 0)
      ctx.restore()
    }
  }
  ctx.restore()
}

// 生成水印 SVG 片段（用于 SVG/PDF 导出）
export function watermarkSvgFragment(level) {
  if (!level || level <= 0) return ''
  const configs = {
    1: { opacity: '0.45', gap: 400, font: 24 },
    2: { opacity: '0.75', gap: 240, font: 24 },
    3: { opacity: '1.0',  gap: 150, font: 24 },
  }
  const cfg = configs[level] || configs[1]
  const text = '试用版·来源系统'
  const item = `<text x="0" y="0" font-size="${cfg.font}" fill="#333" opacity="${cfg.opacity}" text-anchor="middle" dominant-baseline="central" transform="rotate(-28)">${text}</text>`
  
  let items = ''
  // 生成一个足够大的 pattern（约 4×3 个 item）
  const pw = cfg.gap * 6, ph = cfg.gap * 4
  for (let y = 0; y < ph; y += cfg.gap) {
    for (let x = 0; x < pw; x += cfg.gap) {
      items += `<g transform="translate(${x},${y})">${item}</g>\n`
    }
  }
  return `<defs><pattern id="watermarkPattern" x="0" y="0" width="${pw}" height="${ph}" patternUnits="userSpaceOnUse"><rect width="${pw}" height="${ph}" fill="none"/>\n${items}</pattern></defs><rect x="0" y="0" width="100%" height="100%" fill="url(#watermarkPattern)"/>`
}

// UTF-8 安全的 base64 编码（btoa 不支持中文）
function toBase64(str) {
  const bytes = new TextEncoder().encode(str)
  let binary = ''
  for (let i = 0; i < bytes.length; i++) {
    binary += String.fromCharCode(bytes[i])
  }
  return btoa(binary)
}

// 生成打印用 CSS 水印样式片段
export function watermarkCssFragment(level) {
  if (!level || level <= 0) return ''
  const configs = {
    1: { opacity: 0.45, size: '400px 270px' },
    2: { opacity: 0.75, size: '240px 160px' },
    3: { opacity: 1.0,  size: '150px 105px' },
  }
  const cfg = configs[level] || configs[1]
  const svgData = toBase64(`<svg xmlns="http://www.w3.org/2000/svg" width="240" height="160"><text x="120" y="80" font-size="24" fill="#333" text-anchor="middle" transform="rotate(-28 120 80)">试用版·来源系统</text></svg>`)
  return `.watermark-overlay{position:fixed;inset:0;pointer-events:none;z-index:99999;opacity:${cfg.opacity};background-repeat:repeat;background-size:${cfg.size};background-image:url(data:image/svg+xml;base64,${svgData})}`
}
