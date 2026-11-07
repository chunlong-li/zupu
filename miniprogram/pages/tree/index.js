// 世系图（简版）：按世代分层排布，父子之间画直线；单指拖动、双指缩放
// 数据来源：/api/members/with-spouses（含配偶名），本地排版
const api = require('../../utils/api')

const COL_W = 130    // 同一世代相邻两人的水平间距
const ROW_H = 170    // 世代之间的垂直间距
const NODE_W = 104
const NODE_H = 64
const MARGIN = 40

Page({
  data: {
    nodes: [],
    edges: [],
    scale: 1,
    tx: 30,
    ty: 30,
    loading: true,
    stageW: 0,
    stageH: 0
  },
  onShow() {
    if (!api.getToken()) { wx.reLaunch({ url: '/pages/login/index' }); return }
    if (!this.data.nodes.length) this.load()
  },
  load() {
    this.setData({ loading: true })
    api.get('/members/with-spouses').then((list) => {
      const arr = (list || []).slice()
      const byGen = {}
      arr.forEach((m) => {
        const g = Number(m.generation) || 0
        ;(byGen[g] = byGen[g] || []).push(m)
      })
      const gens = Object.keys(byGen).map(Number).sort((a, b) => a - b)
      const pos = {}      // id -> {x,y}
      const nodes = []
      gens.forEach((g, gi) => {
        // 同一世代按「生父 + 排行」排序，让兄弟挨着
        byGen[g].sort((a, b) => (a.father_id - b.father_id) || String(a.rank || '').localeCompare(String(b.rank || '')) || (a.id - b.id))
        byGen[g].forEach((m, i) => {
          const x = MARGIN + i * COL_W
          const y = MARGIN + gi * ROW_H
          pos[m.id] = { x, y }
          nodes.push({
            id: m.id,
            name: m.name || ((m.surname || '') + (m.given_name || '')),
            genText: g === 0 ? '外纪' : '第' + g + '世',
            x, y
          })
        })
      })
      // 连线：父节点底部中点 → 子节点顶部中点
      const edges = []
      arr.forEach((m) => {
        const p = pos[m.father_id]
        const c = pos[m.id]
        if (!p || !c) return
        const x1 = p.x + NODE_W / 2
        const y1 = p.y + NODE_H
        const x2 = c.x + NODE_W / 2
        const y2 = c.y
        const dx = x2 - x1
        const dy = y2 - y1
        const len = Math.sqrt(dx * dx + dy * dy)
        const deg = Math.atan2(dy, dx) * 180 / Math.PI
        edges.push({ id: m.id, left: x1, top: y1, len, deg })
      })
      const stageW = MARGIN * 2 + (Math.max.apply(null, gens.map((g) => byGen[g].length)) || 1) * COL_W
      const stageH = MARGIN * 2 + gens.length * ROW_H
      this.setData({ nodes, edges, loading: false, stageW, stageH })
    }).catch(() => this.setData({ loading: false }))
  },
  onTouchStart(e) {
    if (e.touches.length === 1) {
      this._start = { x: e.touches[0].clientX, y: e.touches[0].clientY, tx: this.data.tx, ty: this.data.ty }
    } else if (e.touches.length === 2) {
      const dx = e.touches[0].clientX - e.touches[1].clientX
      const dy = e.touches[0].clientY - e.touches[1].clientY
      this._dist = Math.sqrt(dx * dx + dy * dy) || 1
      this._scale0 = this.data.scale
    }
  },
  onTouchMove(e) {
    if (e.touches.length === 1 && this._start) {
      this.setData({
        tx: this._start.tx + (e.touches[0].clientX - this._start.x),
        ty: this._start.ty + (e.touches[0].clientY - this._start.y)
      })
    } else if (e.touches.length === 2 && this._dist) {
      const dx = e.touches[0].clientX - e.touches[1].clientX
      const dy = e.touches[0].clientY - e.touches[1].clientY
      const d = Math.sqrt(dx * dx + dy * dy) || 1
      const scale = Math.min(3, Math.max(0.3, (this._scale0 || 1) * (d / this._dist)))
      this.setData({ scale })
    }
  },
  onTouchEnd() { this._start = null; this._dist = 0 },
  reset() { this.setData({ scale: 1, tx: 30, ty: 30 }) },
  goDetail(e) {
    wx.navigateTo({ url: '/pages/detail/index?id=' + e.currentTarget.dataset.id })
  }
})
