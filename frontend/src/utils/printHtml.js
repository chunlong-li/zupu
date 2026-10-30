// 打印文档生成：把分页方案渲染成「一张纸一个 section」的 HTML/矢量打印稿。
//
// 每一页的结构：
//   <section class="pg">
//     <div class="hd">谱名 · 视图 / 卷册（房支）/ 本张世代范围</div>
//     <div class="bd"><svg viewBox="裁剪区">同一份矢量内容 + 切口续接标记</svg></div>
//     <div class="ft">← 接第 x 张 / 第 x/y 张 · 行 r/R · 列 c/C · 格 A1 / 接第 y 张 →</div>
//     + 四角裁剪标记 + 装订侧留边（CSS，单位 mm）
//   </section>
//
// @page 只在打印文档里声明（size: A3/A4 横竖），不写进应用页面 → 屏幕预览零影响。

import { PAPER_LABEL, ORIENT_LABEL, MODE_LABEL, MM_PER_PX, SCROLL_HINT } from './printPaging'

function esc(s) {
  return String(s == null ? '' : s).replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]))
}
const mm = v => (Math.round(v * 1000) / 1000) + 'mm'

// 单页矢量：viewBox = 裁剪区，物理宽高 = 裁剪尺寸 × 比例（保证各页同比例、不缩放）
function pageSvg(page, plan, contentInner, extra) {
  const s = plan.mmPerPx
  const c = page.crop
  return `<svg class="pgsvg" xmlns="http://www.w3.org/2000/svg" viewBox="${c.x} ${c.y} ${c.w} ${c.h}"`
    + ` width="${mm(c.w * s)}" height="${mm(c.h * s)}" preserveAspectRatio="xMinYMin meet">`
    + contentInner + (extra || '') + `</svg>`
}

// 切口续接标记（矢量）：虚线 + 箭头文字，画在空隙里（切页线本身落在节点空隙）
function seamMarks(page, plan, bgInk) {
  const s = plan.mmPerPx
  const c = page.crop
  const ink = bgInk || '#8c5a3c'
  const fs = (2.6 / s).toFixed(2)
  const sw = (0.22 / s).toFixed(3)
  const dash = `${(1.5 / s).toFixed(2)} ${(1.5 / s).toFixed(2)}`
  let out = `<g class="seams" font-family="Songti SC, SimSun, serif" font-size="${fs}" fill="${ink}">`
  const seam = page.seam || {}
  const topY = c.y + 3.4 / s
  if (seam.left) {
    out += `<line x1="${seam.left.at}" y1="${c.y}" x2="${seam.left.at}" y2="${c.y + c.h}" stroke="${ink}" stroke-width="${sw}" stroke-dasharray="${dash}" opacity="0.85"/>`
    out += `<text x="${(seam.left.at + 0.9 / s).toFixed(2)}" y="${topY.toFixed(2)}" text-anchor="start">← 接第 ${seam.left.pageNo} 张（${esc(seam.left.grid)}）</text>`
  }
  if (seam.right) {
    out += `<line x1="${seam.right.at}" y1="${c.y}" x2="${seam.right.at}" y2="${c.y + c.h}" stroke="${ink}" stroke-width="${sw}" stroke-dasharray="${dash}" opacity="0.85"/>`
    out += `<text x="${(seam.right.at - 0.9 / s).toFixed(2)}" y="${topY.toFixed(2)}" text-anchor="end">接第 ${seam.right.pageNo} 张（${esc(seam.right.grid)}）→</text>`
  }
  const leftX = c.x + 1.2 / s
  if (seam.top) {
    out += `<line x1="${c.x}" y1="${seam.top.at}" x2="${c.x + c.w}" y2="${seam.top.at}" stroke="${ink}" stroke-width="${sw}" stroke-dasharray="${dash}" opacity="0.6"/>`
    out += `<text x="${leftX.toFixed(2)}" y="${(seam.top.at + 2.4 / s).toFixed(2)}" text-anchor="start">↑ 接第 ${seam.top.pageNo} 张（${esc(seam.top.grid)}）</text>`
  }
  if (seam.bottom) {
    out += `<line x1="${c.x}" y1="${seam.bottom.at}" x2="${c.x + c.w}" y2="${seam.bottom.at}" stroke="${ink}" stroke-width="${sw}" stroke-dasharray="${dash}" opacity="0.6"/>`
    out += `<text x="${leftX.toFixed(2)}" y="${(seam.bottom.at - 1.4 / s).toFixed(2)}" text-anchor="start">↓ 接第 ${seam.bottom.pageNo} 张（${esc(seam.bottom.grid)}）</text>`
  }
  return out + '</g>'
}

// 版面 CSS（全部毫米单位，只作用于打印文档）
function baseCss(plan, meta) {
  const m = plan.metrics
  const bg = meta.bg || '#ffffff'
  const ink = meta.ink || '#3a2a18'
  return `
  html,body{margin:0;padding:0;background:#fff;color:${ink};}
  body{font-family:"Songti SC","SimSun","Noto Serif CJK SC","Source Han Serif SC",serif;-webkit-print-color-adjust:exact;print-color-adjust:exact;}
  @page{size:${mm(m.paperW)} ${mm(m.paperH)};margin:0;}
  .pg{position:relative;width:${mm(m.paperW)};height:${mm(m.paperH)};overflow:hidden;background:${bg};page-break-after:always;break-after:page;}
  .pg:last-child{page-break-after:auto;break-after:auto;}
  .hd{position:absolute;left:${mm(m.left)};top:${mm(m.padY)};width:${mm(m.areaW)};height:${mm(m.headerMm)};box-sizing:border-box;
      display:flex;align-items:flex-end;justify-content:space-between;gap:3mm;border-bottom:.3mm solid #8c5a3c;padding-bottom:1.1mm;font-size:3.1mm;line-height:1.1;}
  .hd .t1{font-weight:700;letter-spacing:.2mm;}
  .hd .t2{color:#8c5a3c;}
  .hd .t3{color:#6b4a2f;white-space:nowrap;}
  .bd{position:absolute;left:${mm(m.left)};top:${mm(m.bodyTop)};width:${mm(m.areaW)};height:${mm(m.areaH)};overflow:hidden;}
  .bd svg{display:block;}
  .ft{position:absolute;left:${mm(m.left)};top:${mm(m.bodyTop + m.areaH + 1)};width:${mm(m.areaW)};height:${mm(Math.max(4, m.footerMm - 1))};
      box-sizing:border-box;display:flex;align-items:center;justify-content:space-between;gap:2mm;font-size:2.7mm;color:#5a4632;border-top:.22mm solid #cbb89a;padding-top:1.2mm;}
  .ft .mid{font-weight:600;color:#3a2a18;white-space:nowrap;}
  .ft .l,.ft .r{white-space:nowrap;color:#8c5a3c;}
  .crop{position:absolute;width:2.6mm;height:2.6mm;border:0 solid #555;opacity:.9;}
  .crop.tl{left:${mm(m.left - 2.9)};top:${mm(m.bodyTop - 2.9)};border-left-width:.22mm;border-top-width:.22mm;}
  .crop.tr{left:${mm(m.left + m.areaW)};top:${mm(m.bodyTop - 2.9)};border-right-width:.22mm;border-top-width:.22mm;}
  .crop.bl{left:${mm(m.left - 2.9)};top:${mm(m.bodyTop + m.areaH)};border-left-width:.22mm;border-bottom-width:.22mm;}
  .crop.br{left:${mm(m.left + m.areaW)};top:${mm(m.bodyTop + m.areaH)};border-right-width:.22mm;border-bottom-width:.22mm;}
  .bind{position:absolute;top:${mm(m.bodyTop)};height:${mm(m.areaH)};border-left:.2mm dashed #b9a184;}
  .bind.left{left:${mm(m.left - 3.6)};}
  .bind.right{left:${mm(m.left + m.areaW + 3.6)};}
  .bind i{position:absolute;top:0;left:.6mm;font-style:normal;font-size:2.4mm;color:#b9a184;writing-mode:vertical-rl;letter-spacing:.5mm;}
  @media print{.screen-only{display:none !important;}.pg{box-shadow:none;margin:0;}.pg + .pg{break-before:page;}}
  @media screen{body{background:#e7e2d8;padding:8mm 0 14mm;}.pg{margin:0 auto 10mm;box-shadow:0 2mm 5mm rgba(0,0,0,.28);}
    .screen-only{position:fixed;left:0;right:0;top:0;z-index:99;background:#2b2118;color:#f6efe2;padding:2.2mm 4mm;display:flex;gap:4mm;align-items:center;font:3.2mm/1.3 sans-serif;}
    .screen-only button{font:3.4mm/1 sans-serif;padding:1.6mm 4mm;border-radius:1mm;border:0;background:#c9a227;color:#241a10;cursor:pointer;font-weight:700;}
    .screen-only b{color:#e8c65c;}}
  .note{font-size:2.5mm;color:#8c5a3c;}
  `
}

// 索引页：整图缩略 + 网格坐标 + 每格页码/世代
function indexPageHtml(plan, meta, contentInner) {
  const m = plan.metrics
  const b = plan.bounds
  const n = plan.mapPages.length
  const thumbH = Math.max(40, Math.min(m.areaH * (n <= 8 ? 0.6 : n <= 24 ? 0.48 : 0.38), m.areaH - 34))
  const tableH = m.areaH - thumbH - 6
  const ts = Math.min(m.areaW / b.w, thumbH / b.h)
  const tw = b.w * ts
  const th = b.h * ts
  // 表格分栏：每栏可放行数
  const rowH = 4.6
  const perCol = Math.max(6, Math.floor((tableH - 8) / rowH))
  const nCols = Math.max(1, Math.min(5, Math.ceil(n / perCol)))
  const groups = []
  for (let i = 0; i < nCols; i++) {
    const chunk = plan.mapPages.slice(i * perCol, (i + 1) * perCol)
    if (chunk.length) groups.push(chunk)
  }
  const cell = (page) => {
    const l = (page.tile.x - b.x) / b.w * 100
    const t = (page.tile.y - b.y) / b.h * 100
    const w = page.tile.w / b.w * 100
    const h = page.tile.h / b.h * 100
    return `<div class="cell" style="left:${l.toFixed(3)}%;top:${t.toFixed(3)}%;width:${w.toFixed(3)}%;height:${h.toFixed(3)}%">`
      + `<span class="gc">${esc(page.grid)}</span><span class="gp">P${page.pageNo}</span></div>`
  }
  const table = (chunk, first) => `<table>
    <thead><tr><th>格</th><th>页码</th><th>世代范围</th>${first ? '<th>房支</th>' : ''}</tr></thead>
    <tbody>${chunk.map(p => `<tr><td class="gc2">${esc(p.grid)}</td><td>第 ${p.pageNo} 张</td>
      <td>${esc(p.genFrom || '—')}${p.genTo && p.genTo !== p.genFrom ? ' ~ ' + esc(p.genTo) : ''}</td>
      ${first ? `<td>${esc(meta.volume || '—')}</td>` : ''}</tr>`).join('')}</tbody></table>`

  const volLine = `${meta.familyName || ''}${meta.familyName ? ' · ' : ''}${meta.viewTitle || '世系图'}`
  return `<section class="pg idx" data-kind="index">
    <div class="hd"><span class="t1">${esc(volLine)} — 索 引</span><span class="t2">${meta.volume ? '房支：' + esc(meta.volume) : ''}</span><span class="t3">共 ${plan.totalPages} 张 · 图页 ${plan.mapPageCount} 张</span></div>
    <div class="bd">
      <div class="ixwrap">
        <div class="thumb" style="width:${mm(tw)};height:${mm(th)}">
          <svg class="tsvg" xmlns="http://www.w3.org/2000/svg" viewBox="${b.x} ${b.y} ${b.w} ${b.h}" width="100%" height="100%" preserveAspectRatio="xMidYMid meet">${contentInner}</svg>
          <div class="grid">${plan.mapPages.map(cell).join('')}</div>
        </div>
        <div class="ixtab" style="height:${mm(tableH)}">
          <div class="ixtitle">${esc(PAPER_LABEL[plan.opts.paper])} ${esc(ORIENT_LABEL[plan.opts.orientation])} · ${plan.cols} 列 × ${plan.rows} 行 · 重缝 ${plan.overlapMmEff}mm · 内容比例 ${plan.scalePct.toFixed(1)}%（1px = ${plan.mmPerPx.toFixed(3)}mm）${plan.scaleMode !== 'fixed' ? ' · 自动切满' : ''}</div>
          <div class="ixfill">内容 ${plan.contentWmm} × ${plan.contentHmm}mm ÷ 纸面可用 ${plan.metrics.areaW} × ${plan.metrics.areaH}mm → 每张覆盖 <b>${(plan.coverage * 100).toFixed(1)}%</b>；最右列空白 <b>${plan.blankRightMm}mm</b>（${((plan.blankRightMm / plan.metrics.areaW) * 100).toFixed(1)}%），最下行空白 <b>${plan.blankBottomMm}mm</b>（${((plan.blankBottomMm / plan.metrics.areaH) * 100).toFixed(1)}%）。</div>
          <div class="cols n${groups.length}">${groups.map((g, i) => table(g, i === 0)).join('')}</div>
          <div class="ixfoot">格 = 列字母 + 行号（A1 为左上角）；阅读顺序按行自上而下、行内自左而右；相邻页重叠 ${plan.overlapMmEff}mm，切口上的连接线两张都能看到。</div>
        </div>
      </div>
    </div>
    <div class="ft"><span class="l">索引</span><span class="mid">第 1/${plan.totalPages} 张 · 索引页（放在最前面）</span><span class="r">${esc(meta.volume || '')}</span></div>
    <div class="crop tl"></div><div class="crop tr"></div><div class="crop bl"></div><div class="crop br"></div>
    ${m.bindLeft ? '<div class="bind left"><i>装订侧</i></div>' : ''}${m.bindRight ? '<div class="bind right"><i>装订侧</i></div>' : ''}
  </section>`
}

function mapPageHtml(page, plan, meta, contentInner) {
  const genRange = page.genFrom
    ? (page.genTo && page.genTo !== page.genFrom ? `${page.genFrom} ~ ${page.genTo}` : page.genFrom)
    : '世代范围未标注'
  const seam = page.seam || {}
  const leftTxt = seam.left ? `← 接第 ${seam.left.pageNo} 张（${seam.left.grid}）` : ''
  const rightTxt = seam.right ? `接第 ${seam.right.pageNo} 张（${seam.right.grid}）→` : ''
  const topTxt = seam.top ? `↑ 接第 ${seam.top.pageNo} 张（${seam.top.grid}）` : ''
  const botTxt = seam.bottom ? `↓ 接第 ${seam.bottom.pageNo} 张（${seam.bottom.grid}）` : ''
  const vert = [topTxt, botTxt].filter(Boolean).join(' · ')
  return `<section class="pg map" data-kind="map" data-page="${page.pageNo}" data-grid="${esc(page.grid)}" data-row="${page.row}" data-col="${page.col}" data-gen-from="${esc(page.genFrom)}" data-gen-to="${esc(page.genTo)}">
    <div class="hd">
      <span class="t1">${esc(meta.familyName || '')}${meta.familyName ? ' · ' : ''}${esc(meta.viewTitle || '世系图')}</span>
      <span class="t2">${meta.volume ? '房支：' + esc(meta.volume) : ''}</span>
      <span class="t3">${esc(genRange)}</span>
    </div>
    <div class="bd">${pageSvg(page, plan, contentInner, seamMarks(page, plan, meta.ink))}</div>
    <div class="ft">
      <span class="l">${esc(leftTxt)}${leftTxt && vert ? ' · ' : ''}${esc(vert)}</span>
      <span class="mid">第 ${page.pageNo}/${plan.totalPages} 张 · 行 ${page.row}/${page.rowCount} · 列 ${page.col}/${page.colCount} · 格 ${esc(page.grid)}${page.fill ? ' · 覆盖 ' + (page.fill.area * 100).toFixed(0) + '%' + (page.anchored ? '（末页回借）' : '') : ''}</span>
      <span class="r">${esc(rightTxt)}</span>
    </div>
    <div class="crop tl"></div><div class="crop tr"></div><div class="crop bl"></div><div class="crop br"></div>
    ${plan.metrics.bindLeft ? '<div class="bind left"><i>装订侧</i></div>' : ''}${plan.metrics.bindRight ? '<div class="bind right"><i>装订侧</i></div>' : ''}
  </section>`
}

function singlePageHtml(page, plan, meta, contentInner) {
  const genRange = page.genFrom ? `${page.genFrom}${page.genTo && page.genTo !== page.genFrom ? ' ~ ' + page.genTo : ''}` : ''
  return `<section class="pg one" data-kind="single">
    <div class="hd"><span class="t1">${esc(meta.familyName || '')}${meta.familyName ? ' · ' : ''}${esc(meta.viewTitle || '世系图')}</span><span class="t2">${meta.volume ? '房支：' + esc(meta.volume) : ''}</span><span class="t3">${esc(genRange)}</span></div>
    <div class="bd">${pageSvg(page, plan, contentInner, '')}</div>
    <div class="ft"><span class="l">整张一页（按版面缩放）</span><span class="mid">共 1 张 · 内容比例 ${plan.scalePct.toFixed(1)}%</span><span class="r">${esc(meta.volume || '')}</span></div>
    <div class="crop tl"></div><div class="crop tr"></div><div class="crop bl"></div><div class="crop br"></div>
  </section>`
}

function scrollPageHtml(page, plan, meta, contentInner) {
  const genRange = page.genFrom ? `${page.genFrom}${page.genTo && page.genTo !== page.genFrom ? ' ~ ' + page.genTo : ''}` : ''
  const axisTxt = plan.scroll && plan.scroll.axis === 'h' ? '横向卷轴' : '纵向卷轴'
  return `<section class="pg scroll" data-kind="scroll" data-scroll-w="${page.pageWmm}" data-scroll-h="${page.pageHmm}">
    <div class="hd"><span class="t1">${esc(meta.familyName || '')}${meta.familyName ? ' · ' : ''}${esc(meta.viewTitle || '世系图')}</span><span class="t2">${meta.volume ? '房支：' + esc(meta.volume) : ''}</span><span class="t3">${axisTxt} ${page.pageWmm} × ${page.pageHmm} mm</span></div>
    <div class="bd">${pageSvg(page, plan, contentInner, '')}</div>
    <div class="ft"><span class="l">卷轴（超长单页）· ${esc(genRange)}</span><span class="mid">${page.pageWmm} × ${page.pageHmm} mm · 内容比例 ${plan.scalePct.toFixed(1)}%</span><span class="r">绘图仪 / 打印店出图</span></div>
    <div class="crop tl"></div><div class="crop tr"></div><div class="crop bl"></div><div class="crop br"></div>
  </section>`
}

// 索引页专用 CSS（缩略图 + 网格 + 表格）
function indexCss(plan) {
  return `
  .ixwrap{display:flex;flex-direction:column;height:100%;gap:3mm;}
  .thumb{position:relative;border:.3mm solid #b9a184;background:#fff;overflow:hidden;flex:0 0 auto;margin:0 auto;}
  .thumb .tsvg{position:absolute;inset:0;width:100%;height:100%;}
  .thumb .grid{position:absolute;inset:0;}
  .thumb .cell{position:absolute;box-sizing:border-box;border:.2mm dashed #c0392b;background:rgba(192,57,43,.03);}
  .thumb .cell .gc{position:absolute;left:0;top:0;background:#c0392b;color:#fff;font-size:2.2mm;line-height:1;padding:.35mm .8mm;font-weight:700;}
  .thumb .cell .gp{position:absolute;right:.4mm;bottom:.4mm;font-size:2.1mm;color:#8c2b1e;background:#fff8;padding:0 .5mm;}
  .ixtab{flex:1 1 auto;display:flex;flex-direction:column;gap:1.6mm;min-height:0;}
  .ixtitle{font-size:2.7mm;color:#5a4632;border-bottom:.22mm solid #cbb89a;padding-bottom:1mm;}
  .ixfill{font-size:2.5mm;color:#6b5a44;margin-top:.8mm;}
  .ixfill b{color:#8c2b1e;}
  .ixtab .cols{display:flex;gap:4mm;flex:1 1 auto;min-height:0;}
  .ixtab .cols table{border-collapse:collapse;font-size:2.6mm;line-height:1.18;flex:1 1 0;align-self:flex-start;}
  .ixtab th,.ixtab td{border:.18mm solid #cbb89a;padding:.55mm .9mm;text-align:left;white-space:nowrap;}
  .ixtab th{background:#f3ebdc;color:#5a4632;font-weight:600;}
  .ixtab td.gc2{font-weight:700;color:#8c2b1e;}
  .ixfoot{font-size:2.35mm;color:#8c5a3c;}
  `
}

// 生成完整打印 HTML
// contentInner: 已去掉外层 <svg> 的矢量内容（defs + 内容 <g>，g 的 transform 已重置为恒等）
// docCss: 克隆时抽出的 <style>（字体子集 @font-face），整份文档只放一次
export function buildPrintDoc({ plan, meta = {}, contentInner, docCss = '' }) {
  const o = plan.opts
  const m = plan.metrics
  let pagesHtml = ''
  if (plan.mode === 'tile') {
    if (plan.indexPage) pagesHtml += indexPageHtml(plan, meta, contentInner)
    pagesHtml += plan.mapPages.map(p => mapPageHtml(p, plan, meta, contentInner)).join('\n')
  } else if (plan.mode === 'single') {
    pagesHtml = singlePageHtml(plan.mapPages[0], plan, meta, contentInner)
  } else {
    pagesHtml = scrollPageHtml(plan.mapPages[0], plan, meta, contentInner)
  }
  const label = `${PAPER_LABEL[o.paper] || ''} ${ORIENT_LABEL[o.orientation] || ''} ${MODE_LABEL[plan.mode] || ''}`.trim()
  const sizeLine = plan.mode === 'scroll'
    ? `卷轴 ${plan.scroll.width} × ${plan.scroll.length} mm`
    : `${m.paperW} × ${m.paperH} mm（${label}）`
  const hint = plan.mode === 'scroll' ? SCROLL_HINT : '在打印对话框里选「另存为 PDF」可得到矢量 PDF；缩放请选「默认/100%」，不要选「适合页面」。'
  const css = baseCss(plan, meta) + (plan.mode === 'tile' ? indexCss(plan) : '')
  return `<!DOCTYPE html><html lang="zh-CN"><head><meta charset="utf-8"><title>${esc(meta.filename || '世系图-打印')}</title>
<style>${docCss || ''}</style>
<style>${css}</style>
</head><body>
<div class="screen-only">
  <b>${esc(meta.title || '世系图打印稿')}</b>
  <span>${esc(sizeLine)} · 共 ${plan.totalPages} 张 · ${hint}</span>
  <button onclick="window.print()">打印 / 另存为 PDF</button>
</div>
${pagesHtml}
</body></html>`
}
