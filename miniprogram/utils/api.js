const { MODE, BASE_URL, CLOUD_ENV, CLOUD_SERVICE, API_PREFIX } = require('./config')

const TOKEN_KEY = 'zupu_token'

function getToken() { return wx.getStorageSync(TOKEN_KEY) || '' }
function setToken(t) { wx.setStorageSync(TOKEN_KEY, t || '') }
function clearToken() { wx.removeStorageSync(TOKEN_KEY) }

// 统一处理响应：401 → 清 token 回登录；2xx → 返回 data；其它 → toast + reject
function handleResponse(res, resolve, reject) {
  if (res.statusCode === 401) {
    clearToken()
    wx.reLaunch({ url: '/pages/login/index' })
    reject(new Error('请先登录'))
    return
  }
  if (res.statusCode >= 200 && res.statusCode < 300) {
    resolve(res.data)
    return
  }
  const msg = (res.data && res.data.error) || ('请求失败(' + res.statusCode + ')')
  wx.showToast({ title: msg, icon: 'none' })
  reject(new Error(msg))
}

function failHandler(reject) {
  return (err) => {
    const msg = err.errMsg || '网络错误'
    wx.showToast({ title: msg, icon: 'none' })
    reject(new Error(msg))
  }
}

function headers() {
  const h = { 'Content-Type': 'application/json' }
  if (getToken()) h.Authorization = 'Bearer ' + getToken()
  return h
}

// ===== 云托管模式（微信云托管 / CloudBase Run）=====
// 小程序用 wx.cloud.callContainer 直连容器：不需要备案域名、不用配 request 合法域名，
// 只要求小程序后台开通云开发，并在 config.js 填 CLOUD_ENV 与云托管服务名 CLOUD_SERVICE。
let cloudReady = false
function ensureCloud() {
  if (cloudReady) return
  if (!wx.cloud) throw new Error('基础库过低，请升级微信或在后台开通云开发')
  wx.cloud.init({ env: CLOUD_ENV, traceUser: true })
  cloudReady = true
}

function cloudRequest(method, path, data) {
  return new Promise((resolve, reject) => {
    try { ensureCloud() } catch (e) {
      wx.showToast({ title: e.message, icon: 'none' })
      reject(e)
      return
    }
    wx.cloud.callContainer({
      config: { env: CLOUD_ENV },
      path: API_PREFIX + path,
      method,
      data,
      timeout: 20000,
      header: Object.assign({ 'X-WX-SERVICE': CLOUD_SERVICE }, headers()),
      success: (res) => handleResponse(res, resolve, reject),
      fail: failHandler(reject)
    })
  })
}

// ===== 自建服务器模式（http/https）=====
function httpRequest(method, path, data) {
  return new Promise((resolve, reject) => {
    wx.request({
      url: BASE_URL + API_PREFIX + path,
      method,
      data,
      timeout: 20000,
      header: headers(),
      success: (res) => handleResponse(res, resolve, reject),
      fail: failHandler(reject)
    })
  })
}

function request(method, path, data) {
  return MODE === 'cloud' ? cloudRequest(method, path, data) : httpRequest(method, path, data)
}

const get = (p, d) => request('GET', p, d)
const post = (p, d) => request('POST', p, d)

function login(username, password) {
  return post('/auth/login', { username, password }).then((d) => {
    setToken(d.token)
    if (d.user) wx.setStorageSync('zupu_user', d.user)
    return d
  })
}

module.exports = { request, get, post, login, getToken, setToken, clearToken }
