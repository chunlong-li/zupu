#!/usr/bin/env node
// 小程序静态自检：不依赖微信开发者工具，尽量把「打开就报错」的情况提前抓出来。
// 1) 用假 wx/App/Page/getApp 加载每个页面 js，确认没有顶层异常；
// 2) 从 wxml 里抽出 bindtap/bindinput/... 的处理函数名，与页面方法比对；
// 3) 从 wxml 里抽出 {{xxx}} 的数据字段（首段标识符），与 data 的 key 比对（缺的只提示，不判死）。
const fs = require('fs')
const path = require('path')

const root = path.resolve(__dirname, '..')
const pagesDir = path.join(root, 'pages')
let problems = 0
const warn = (m) => { console.log('  ⚠️ ' + m); problems++ }
const ok = (m) => console.log('  ✅ ' + m)

// ---- 假运行环境 ----
const storage = {}
global.wx = {
  getStorageSync: (k) => storage[k] || '',
  setStorageSync: (k, v) => { storage[k] = v },
  removeStorageSync: (k) => { delete storage[k] },
  showToast: () => {},
  reLaunch: () => {},
  switchTab: () => {},
  navigateTo: () => {},
  request: (o) => { setTimeout(() => o.success && o.success({ statusCode: 200, data: [] }), 0) },
  cloud: { init: () => {}, callContainer: (o) => { setTimeout(() => o.success && o.success({ statusCode: 200, data: [] }), 0) } }
}
global.getApp = () => ({ globalData: {} })
global.App = () => {}
let pageObj = null
global.Page = (o) => { pageObj = o }

const files = []
fs.readdirSync(pagesDir).forEach((d) => {
  const p = path.join(pagesDir, d, 'index.js')
  if (fs.existsSync(p)) files.push(p)
})

files.forEach((jsPath) => {
  const name = path.relative(root, jsPath)
  pageObj = null
  delete require.cache[require.resolve(jsPath)]
  try {
    require(jsPath)
  } catch (e) {
    warn(`${name} 加载失败：${e.message}`)
    return
  }
  if (!pageObj) { warn(`${name} 没有调用 Page()`); return }
  const methods = new Set(Object.keys(pageObj))
  const dataKeys = new Set(Object.keys(pageObj.data || {}))

  const wxmlPath = jsPath.replace(/\.js$/, '.wxml')
  if (!fs.existsSync(wxmlPath)) { warn(`${name} 缺少同名 wxml`); return }
  const wxml = fs.readFileSync(wxmlPath, 'utf8')

  // 事件处理函数
  const handlers = [...wxml.matchAll(/bind[a-z]+="([A-Za-z_$][\w$]*)"/g)].map((m) => m[1])
  const missingHandlers = [...new Set(handlers)].filter((h) => !methods.has(h))
  if (missingHandlers.length) warn(`${name} wxml 引用了不存在的处理函数：${missingHandlers.join(', ')}`)

  // 数据字段（只取 {{ 里的首个标识符，跳过 wx:for 的 item/index）
  const exprs = [...wxml.matchAll(/\{\{([^}]+)\}\}/g)].map((m) => m[1])
  const idents = new Set()
  exprs.forEach((e) => {
    // 去掉字符串字面量和「属性访问」（item.name 只算 item），否则会把字段名误报成缺 data
    const clean = e.replace(/'[^']*'/g, '').replace(/"[^"]*"/g, '').replace(/\.[A-Za-z_$][\w$]*/g, '')
    ;(clean.match(/[A-Za-z_$][\w$]*/g) || []).forEach((id) => idents.add(id))
  })
  const builtin = new Set(['item', 'index', 'true', 'false', 'null', 'undefined', 'length', 'toFixed', 'Math'])
  const missingData = [...idents].filter((id) => !dataKeys.has(id) && !builtin.has(id) && !methods.has(id))
  if (missingData.length) warn(`${name} wxml 用到但 data 里没有：${missingData.join(', ')}`)

  if (!missingHandlers.length && !missingData.length) ok(`${name}（${methods.size} 个方法 / ${dataKeys.size} 个 data 字段）`)
})

// ---- app.json 的页面都要存在 ----
const appJson = JSON.parse(fs.readFileSync(path.join(root, 'app.json'), 'utf8'))
appJson.pages.forEach((p) => {
  const js = path.join(root, p + '.js')
  const wxml = path.join(root, p + '.wxml')
  if (!fs.existsSync(js) || !fs.existsSync(wxml)) warn(`app.json 声明的页面缺文件：${p}`)
})
appJson.tabBar.list.forEach((t) => {
  if (!appJson.pages.includes(t.pagePath)) warn(`tabBar 指向的页面没在 pages 里：${t.pagePath}`)
})

console.log(problems ? `\n共 ${problems} 个问题` : '\n全部通过')
process.exit(problems ? 1 : 0)
