// 服务器接入方式，只改这一处。
//  'http'  ：直接请求自建服务器（需要 HTTPS + 已备案域名 + 不带端口，并在小程序后台配「request 合法域名」）
//  'cloud' ：走微信云托管（CloudBase Run）——小程序用 wx.cloud.callContainer 直连容器，
//            **不需要备案域名、不需要配 request 域名**，只要在小程序后台开通云开发并填环境 ID。
const MODE = 'cloud'

// http 模式用
const BASE_URL = 'https://nas.djbk.top:8083'

// cloud 模式用（微信云托管）
const CLOUD_ENV = 'your-env-id'     // 云开发环境 ID，如 zupu-8gxxxxxx
const CLOUD_SERVICE = 'zupu'        // 云托管服务名（部署时填的）

// 后端接口统一前缀（web 端 axios 的 baseURL 也是 /api）
const API_PREFIX = '/api'

module.exports = { MODE, BASE_URL, CLOUD_ENV, CLOUD_SERVICE, API_PREFIX }
