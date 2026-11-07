const api = require('../../utils/api')

Page({
  data: { m: null, spouses: [], children: [], fatherName: '', motherName: '', loading: true },
  onLoad(query) {
    this.id = query.id
    this.load()
  },
  load() {
    this.setData({ loading: true })
    Promise.all([
      api.get('/members/' + this.id + '/detail'),
      api.get('/members/' + this.id + '/children').catch(() => [])
    ]).then(([d, ch]) => {
      const m = d.member || d
      const spouses = d.spouses || []
      this.setData({
        m,
        spouses,
        children: (ch && ch.data) || ch || [],
        deathText: this.deathText(m)
      })
      // 父母名字：接口给的是 id，用成员列表兜底匹配
      if (m.father_id || m.mother_id) {
        api.get('/members').then((list) => {
          const find = (id) => (list || []).find((x) => x.id === id)
          this.setData({
            fatherName: m.father_id ? (find(m.father_id) || {}).name || '' : '',
            motherName: m.mother_id ? (find(m.mother_id) || {}).name || '' : ''
          })
        }).catch(() => {})
      }
    }).catch(() => {}).then(() => this.setData({ loading: false }))
  },
  deathText(m) {
    const era = String(m.death_era || '').trim()
    if (/已逝|已故|逝世|已卒|殁|病故|亡故|去世/.test(era)) return era
    if (m.death_gregorian) return m.death_gregorian
    return '在世'
  },
  goMember(e) {
    wx.navigateTo({ url: '/pages/detail/index?id=' + e.currentTarget.dataset.id })
  }
})
