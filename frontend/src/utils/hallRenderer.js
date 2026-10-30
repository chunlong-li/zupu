// ============================================================================
// 祠堂渲染层 —— 抽象接口 + 2.5D 实现
// 设计原则（2026-09-13 定）：把「渲染」抽象成可替换的一层，
// 上层（HallView：数据、点击、水印、详情联动）只依赖接口，不关心底层是 2.5D 还是 3D。
// 后续升真 3D：新增 HallRenderer3D 实现，HallView 一行不改。
// ============================================================================
import { getWatermarkLevel } from './watermark.js'

// 阿拉伯数字 → 中文数字（0~99，用于世代标签「第一世」等）
function numberToChinese(n) {
  if (n == null || !Number.isInteger(n) || n < 0) return String(n ?? '')
  const digits = ['零', '一', '二', '三', '四', '五', '六', '七', '八', '九']
  if (n < 10) return digits[n]
  if (n < 20) return '十' + (n % 10 === 0 ? '' : digits[n % 10])
  if (n < 100) {
    const tens = Math.floor(n / 10)
    const ones = n % 10
    return digits[tens] + '十' + (ones === 0 ? '' : digits[ones])
  }
  return String(n)
}

// ---------------------------------------------------------------------------
// HallRenderer 接口契约（JSDoc 描述，JS 无运行时约束）
// 任何渲染器必须实现以下方法：
//   mount(container)               挂载到 DOM 容器
//   render(layout)                 渲染牌位布局（layout 来自 buildHallLayout）
//   onPick(cb: (memberId) => void) 注册点击牌位回调
//   highlight(memberId | null)     高亮/取消高亮某个牌位
//   setWatermark(show)             开关水印
//   capture()                      截图（返回 { width, height, blob }）
//   dispose()                      销毁释放（解绑事件、清理 DOM/GL 资源）
// ---------------------------------------------------------------------------

// ============================================================================
// 2.5D 实现：DOM 牌位 + CSS 3D 透视（俯视角牌位墙）
// 优点：点击/hover 用原生 DOM 事件，命中检测零成本；水印用 overlay 层
// ============================================================================
export class HallRenderer2D5 {
  constructor() {
    this.container = null
    this.el = null // 牌位墙
    this.watermarkEl = null
    this._pickCb = null
    this._highlightId = null
  }

  mount(container) {
    this.container = container

    // 牌位墙容器（宽度撑开到内容，超出时横向滚动；每排居中；第一世靠顶部）
    this.el = document.createElement('div')
    this.el.className = 'hall-wall'
    Object.assign(this.el.style, {
      position: 'relative',
      width: 'max-content',
      minWidth: '100%',
      minHeight: '100%',
      display: 'flex',
      flexDirection: 'column',
      alignItems: 'center',
      justifyContent: 'flex-start', // 第一世靠顶部，不垂直居中
      rowGap: '22px', // 各级台阶之间的间距（留出立面厚度空间）
      margin: '0 auto', // 内容窄时整体水平居中
      paddingBottom: '20px',
    })

    // 水印层（绝对定位覆盖，独立于牌位墙）
    this.watermarkEl = document.createElement('div')
    this.watermarkEl.className = 'hall-watermark'
    Object.assign(this.watermarkEl.style, {
      position: 'absolute',
      inset: '0',
      pointerEvents: 'none',
      display: 'none',
      zIndex: '10',
    })

    this.container.appendChild(this.el)
    this.container.appendChild(this.watermarkEl)

    // 事件委托：点牌位 → 回调 memberId
    this.el.addEventListener('click', (e) => {
      const card = e.target.closest('[data-member-id]')
      if (card && this._pickCb) {
        this._pickCb(Number(card.dataset.memberId))
      }
    })
  }

  render(layout) {
    if (!this.el) return
    this.el.innerHTML = ''
    for (const row of layout.rows) {
      const rowEl = this._renderRow(row)
      this.el.appendChild(rowEl)
    }
    // 初始横向居中：内容超出容器时滚动到中间
    this._centerHorizontally()
  }

  _centerHorizontally() {
    if (!this.container || !this.el) return
    const overflow = this.el.scrollWidth - this.container.clientWidth
    if (overflow > 0) {
      this.container.scrollLeft = Math.floor(overflow / 2)
    }
  }

  _renderRow(row) {
    // 一级台阶（2.5D：透视俯视的立体台阶，顶面放牌位 + 前面立面）
    const step = document.createElement('div')
    step.className = 'hall-step'
    Object.assign(step.style, {
      position: 'relative',
      width: 'max-content',
      margin: '0 auto',
      flexShrink: 0,
      transformStyle: 'preserve-3d',
      transform: 'perspective(1100px) rotateX(35deg)',
      transformOrigin: '50% 100%',
    })

    // 台面（顶面，浅木色，牌位立在其上）
    const surface = document.createElement('div')
    surface.className = 'hall-step-surface'
    Object.assign(surface.style, {
      display: 'flex',
      alignItems: 'flex-end', // 牌位底部对齐台面
      justifyContent: 'center',
      gap: '12px',
      padding: '12px 20px 10px',
      background: 'linear-gradient(180deg, #b0854f 0%, #8a6538 60%, #6d4c26 100%)',
      borderRadius: '5px',
      boxShadow: 'inset 0 1px 0 rgba(255,235,200,.22), 0 6px 16px rgba(0,0,0,.45)',
      flexWrap: 'nowrap',
      flexShrink: 0,
      width: 'max-content',
    })

    // 世代标签（竖排，台面左端）
    if (row.generation != null && row.generation >= 0) {
      const genLabel = document.createElement('div')
      genLabel.className = 'hall-gen-label'
      genLabel.textContent = `第${numberToChinese(row.generation)}世`
      Object.assign(genLabel.style, {
        alignSelf: 'center',
        fontSize: '12px',
        color: '#2b1d0f',
        writingMode: 'vertical-rl', // 竖排（中文数字不会倒置）
        letterSpacing: '2px',
        marginRight: '6px',
        fontWeight: '600',
        textShadow: '0 1px 0 rgba(255,235,200,.25)',
      })
      surface.appendChild(genLabel)
    }

    // 牌位
    for (const m of row.members) {
      surface.appendChild(this._renderCard(m))
    }

    step.appendChild(surface)

    // 立面（台阶前面：绕台面前缘向下翻转 90°，底边朝向观察者）
    const face = document.createElement('div')
    face.className = 'hall-step-face'
    Object.assign(face.style, {
      position: 'absolute',
      left: '0',
      right: '0',
      top: '100%',
      height: '22px',
      background: 'linear-gradient(180deg, #3a2a18 0%, #241708 100%)',
      transformOrigin: 'top center',
      transform: 'rotateX(90deg)',
      borderRadius: '0 0 4px 4px',
    })
    step.appendChild(face)

    return step
  }

  _renderCard(m) {
    const card = document.createElement('div')
    card.className = 'hall-card'
    card.dataset.memberId = m.id
    Object.assign(card.style, {
      width: '60px',
      height: '150px', // 窄高竖长，更像真实牌位
      flexShrink: 0, // 关键：人多时也不压缩牌位
      padding: '14px 4px',
      border: '1px solid #c8b28b',
      borderRadius: '4px',
      background: 'linear-gradient(#4a3520, #2f2113)',
      color: '#f3e5c8',
      textAlign: 'center',
      cursor: 'pointer',
      boxShadow: '0 3px 8px rgba(0,0,0,.35)',
      transition: 'transform .15s, box-shadow .15s',
      display: 'flex',
      flexDirection: 'column',
      alignItems: 'center',
      justifyContent: 'center',
      gap: '8px',
      overflow: 'hidden',
      boxSizing: 'border-box',
    })

    // 姓名（竖排）
    const name = document.createElement('div')
    name.textContent = m.name || ''
    Object.assign(name.style, {
      fontSize: '16px',
      fontWeight: '600',
      writingMode: 'vertical-rl',
      letterSpacing: '6px',
      maxHeight: '100%',
      overflow: 'hidden',
      lineHeight: '1.2',
    })
    // 字辈（有则显示，竖排小字）
    const zi = document.createElement('div')
    zi.textContent = m.generation_word || ''
    Object.assign(zi.style, {
      fontSize: '11px',
      opacity: '.8',
      writingMode: 'vertical-rl',
      letterSpacing: '3px',
    })

    card.appendChild(name)
    if (m.generation_word) card.appendChild(zi)

    // hover 效果
    card.addEventListener('mouseenter', () => {
      if (this._highlightId === m.id) return
      card.style.transform = 'translateY(-4px)'
      card.style.boxShadow = '0 6px 14px rgba(0,0,0,.5)'
    })
    card.addEventListener('mouseleave', () => {
      card.style.transform = this._highlightId === m.id ? 'translateY(-4px)' : ''
      card.style.boxShadow = this._highlightId === m.id ? '0 6px 14px rgba(0,0,0,.5)' : '0 3px 8px rgba(0,0,0,.35)'
    })

    return card
  }

  onPick(cb) {
    this._pickCb = cb
  }

  highlight(memberId) {
    this._highlightId = memberId
    if (!this.el) return
    const cards = this.el.querySelectorAll('.hall-card')
    cards.forEach((card) => {
      const isTarget = Number(card.dataset.memberId) === memberId
      card.style.outline = isTarget ? '2px solid #d4af37' : 'none'
    })
  }

  // 水印：复用全局水印等级（未激活显示，激活后关闭）
  setWatermark(show) {
    if (!this.watermarkEl) return
    const level = getWatermarkLevel()
    if (!show || level <= 0) {
      this.watermarkEl.style.display = 'none'
      this.watermarkEl.innerHTML = ''
      return
    }
    // 斜铺水印文字（与 treeExport 的 drawCanvasWatermark 同款文案）
    const text = '试用版·来源系统'
    const frag = document.createDocumentFragment()
    const n = 24
    for (let i = 0; i < n; i++) {
      const span = document.createElement('span')
      span.textContent = text
      Object.assign(span.style, {
        position: 'absolute',
        left: `${(i % 6) * 18}%`,
        top: `${(i * 17) % 90}%`,
        transform: 'rotate(-28deg)',
        fontSize: '22px',
        color: '#333',
        opacity: level >= 3 ? '1' : level >= 2 ? '.75' : '.45',
        whiteSpace: 'nowrap',
      })
      frag.appendChild(span)
    }
    this.watermarkEl.style.display = 'block'
    this.watermarkEl.appendChild(frag)
  }

  // 截图：TODO 后续用 html2canvas 或重绘到 canvas（复用 watermark.js 的 canvas 水印）
  async capture() {
    throw new Error('capture() 未实现：后续接 html2canvas 或 canvas 重绘')
  }

  dispose() {
    if (this.container) {
      if (this.el) this.el.remove()
      if (this.watermarkEl) this.watermarkEl.remove()
    }
    this.container = null
    this.el = null
    this.watermarkEl = null
    this._pickCb = null
    this._highlightId = null
  }
}

// ============================================================================
// 预留：真 3D 实现（Three.js，后续接入）
// 预加载入口（页面 load 后 requestIdleCallback 后台加载，不阻塞主流程）：
//   window.addEventListener('load', () => {
//     requestIdleCallback(() => import('./HallRenderer3D'), { timeout: 3000 })
//   })
// ============================================================================
// export class HallRenderer3D { ... 实现同 HallRenderer 契约 ... }
