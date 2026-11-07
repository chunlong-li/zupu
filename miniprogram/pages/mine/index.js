const api = require('../../utils/api')
const { MODE, BASE_URL, CLOUD_ENV, CLOUD_SERVICE } = require('../../utils/config')

Page({
  data: {
    user: null,
    modeText: '',
    serverText: ''
  },
  onShow() {
    const u = wx.getStorageSync('zupu_user') || null
    this.setData({
      user: u,
      modeText: MODE === 'cloud' ? '微信云托管' : '自建服务器',
      serverText: MODE === 'cloud' ? (CLOUD_ENV + ' / ' + CLOUD_SERVICE) : BASE_URL
    })
  },
  logout() {
    wx.showModal({
      title: '退出登录',
      content: '退出后需要重新输入账号密码',
      success: (r) => {
        if (!r.confirm) return
        api.clearToken()
        wx.removeStorageSync('zupu_user')
        wx.reLaunch({ url: '/pages/login/index' })
      }
    })
  }
})
