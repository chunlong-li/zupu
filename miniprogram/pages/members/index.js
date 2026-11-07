const api = require('../../utils/api')

const PAGE_SIZE = 20

Page({
  data: {
    kw: '',
    list: [],
    all: [],
    page: 1,
    hasMore: true,
    loading: false,
    total: 0,
    branches: [{ id: 0, name: '全部房支' }],
    branchIdx: 0,
    gens: ['全部世代'],
    genIdx: 0
  },
  onShow() {
    if (!api.getToken()) { wx.reLaunch({ url: '/pages/login/index' }); return }
    if (!this.data.all.length) this.loadAll()
  },
  onPullDownRefresh() {
    this.loadAll().then(() => wx.stopPullDownRefresh())
  },
  onReachBottom() {
    if (this.data.hasMore) this.slicePage(this.data.page + 1)
  },
  loadAll() {
    this.setData({ loading: true })
    // 房支下拉：0 = 全部
    api.get('/branches').then((bs) => {
      const branches = [{ id: 0, name: '全部房支' }].concat((bs || []).map((b) => ({ id: b.id, name: b.name })))
      this.setData({ branches })
    }).catch(() => {})
    return api.get('/members')
      .then((list) => {
        const all = (list || []).map((m) => ({
          ...m,
          displayName: m.name || ((m.surname || '') + (m.given_name || '')),
          genText: m.record_type === '外纪' ? '外纪' + (m.generation + 1) + '世' : '第' + m.generation + '世',
          deathText: this.deathText(m)
        }))
        // 世代下拉：按数据里的世代动态生成
        const maxGen = all.reduce((mx, m) => Math.max(mx, Number(m.generation) || 0), 0)
        const gens = ['全部世代']
        for (let g = 0; g <= maxGen; g++) gens.push(g === 0 ? '外纪' : '第' + g + '世')
        this.setData({ all, total: all.length, gens })
        this.applyFilter()
      })
      .catch(() => {})
      .then(() => this.setData({ loading: false }))
  },
  deathText(m) {
    const era = String(m.death_era || '').trim()
    if (/已逝|已故|逝世|已卒|殁|病故|亡故|去世/.test(era)) return era
    if (m.death_gregorian) return m.death_gregorian
    return '在世'
  },
  onKw(e) { this.setData({ kw: e.detail.value }); this.applyFilter() },
  onBranch(e) { this.setData({ branchIdx: Number(e.detail.value) }); this.applyFilter() },
  onGen(e) { this.setData({ genIdx: Number(e.detail.value) }); this.applyFilter() },
  applyFilter() {
    const q = (this.data.kw || '').trim()
    const all = this.data.all
    const bid = (this.data.branches[this.data.branchIdx] || {}).id || 0
    const gi = this.data.genIdx
    let filtered = all
    if (bid) filtered = filtered.filter((m) => m.branch_id === bid)
    if (gi > 0) filtered = filtered.filter((m) => (Number(m.generation) || 0) === gi - 1)
    if (q) filtered = filtered.filter((m) => String(m.displayName || '').indexOf(q) >= 0 || String(m.zi || '').indexOf(q) >= 0 || String(m.hao || '').indexOf(q) >= 0)
    this._filtered = filtered
    this.slicePage(1)
  },
  slicePage(page) {
    const filtered = this._filtered || this.data.all
    const end = page * PAGE_SIZE
    this.setData({ list: filtered.slice(0, end), page, hasMore: end < filtered.length, total: filtered.length })
  },
  goDetail(e) {
    wx.navigateTo({ url: '/pages/detail/index?id=' + e.currentTarget.dataset.id })
  }
})
