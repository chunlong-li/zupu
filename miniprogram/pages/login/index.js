const api = require('../../utils/api')
const app = getApp()

Page({
  data: { username: '', password: '', loading: false, familySurname: '' },
  onLoad() {
    const app = getApp()
    this.setData({ familySurname: (app.globalData && app.globalData.familySurname) || '' })
  },
  onInput(e) { this.setData({ [e.currentTarget.dataset.k]: e.detail.value }) },
  doLogin() {
    const { username, password } = this.data
    if (!username || !password) { wx.showToast({ title: '请输入账号和密码', icon: 'none' }); return }
    this.setData({ loading: true })
    api.login(username, password)
      .then((d) => {
        app.globalData.user = d.user || null
        wx.switchTab({ url: '/pages/members/index' })
      })
      .catch(() => {})
      .then(() => this.setData({ loading: false }))
  }
})
