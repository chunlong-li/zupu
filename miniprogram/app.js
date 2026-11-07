const { getToken } = require('./utils/api')

App({
  globalData: {
    familySurname: '',
    user: null
  },
  onLaunch() {
    // 有 token 才去拉家族信息，避免未登录时反复弹错
    if (!getToken()) return
    const api = require('./utils/api')
    api.get('/settings/family-surname').then((d) => {
      this.globalData.familySurname = d.family_surname || ''
    }).catch(() => {})
  }
})
