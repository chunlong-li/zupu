#!/usr/bin/env node
// 小程序页面「真数据」冒烟：把 wx.request 换成 node 的 fetch 打到本机后端，
// 依次跑 成员列表 / 详情 / 世系图 的载入逻辑，验证不会报错且数据非空。
// 用法：先起后端（FAMILY_TREE_PORT=18099），再 node tools/smoke.js [baseUrl] [token]
const path = require('path')
const ROOT = path.resolve(__dirname, '..')
const BASE = process.argv[2] || 'http://127.0.0.1:18099'
const TOKEN = process.argv[3] || ''

// 让 config 走 http 模式并指向本机
const cfgPath = require.resolve(path.join(ROOT, 'utils/config.js'))
require.cache[cfgPath] = { id: cfgPath, filename: cfgPath, loaded: true, exports: { MODE: 'http', BASE_URL: BASE, CLOUD_ENV: '', CLOUD_SERVICE: '', API_PREFIX: '/api' } }

const storage = { zupu_token: TOKEN }
global.wx = {
  getStorageSync: (k) => storage[k] || '',
  setStorageSync: (k, v) => { storage[k] = v },
  removeStorageSync: (k) => { delete storage[k] },
  showToast: (o) => console.log('   toast:', o.title),
  reLaunch: (o) => console.log('   reLaunch →', o.url),
  switchTab: () => {},
  navigateTo: (o) => console.log('   navigateTo →', o.url),
  request: (o) => {
    const url = o.url
    const init = { method: o.method || 'GET', headers: o.header || {} }
    if (o.data && o.method !== 'GET') init.body = JSON.stringify(o.data)
    fetch(url, init)
      .then(async (r) => { const t = await r.text(); let d = t; try { d = JSON.parse(t) } catch (e) {} o.success && o.success({ statusCode: r.status, data: d }) })
      .catch((e) => o.fail && o.fail({ errMsg: String(e) }))
  },
  cloud: {}
}
global.getApp = () => ({ globalData: {} })
global.App = () => {}
let captured = null
global.Page = (o) => { captured = o }

function loadPage(rel) {
  const p = path.join(ROOT, rel)
  delete require.cache[require.resolve(p)]
  captured = null
  require(p)
  const page = Object.assign({}, captured)
  page.data = JSON.parse(JSON.stringify(captured.data || {}))
  page.setData = function (patch) { Object.assign(this.data, patch) }
  return page
}
const sleep = (ms) => new Promise((r) => setTimeout(r, ms))

;(async () => {
  let fail = 0
  const check = (name, cond, extra) => {
    console.log((cond ? '  ✅ ' : '  ❌ ') + name + (extra ? '  ' + extra : ''))
    if (!cond) fail++
  }

  // 1) 成员列表
  const members = loadPage('pages/members/index.js')
  members.onShow()
  await sleep(1200)
  check('成员列表载入', members.data.all.length > 0, `共 ${members.data.all.length} 人`)
  if (!members.data.all.length) { console.log('\n列表为空，后续跳过'); process.exit(1) }
  check('房支下拉', members.data.branches.length > 0, JSON.stringify(members.data.branches.slice(0, 2)))
  check('世代下拉', members.data.gens.length > 1, members.data.gens.slice(0, 3).join('/'))
  check('分页首屏 20 条', members.data.list.length === Math.min(20, members.data.all.length), `${members.data.list.length} 条`)
  members.setData({ kw: members.data.all[0].displayName })
  members.applyFilter()
  check('搜索过滤命中', members.data.total >= 1, `命中 ${members.data.total}`)
  members.setData({ kw: '' , genIdx: 2 })
  members.applyFilter()
  check('世代筛选生效', members.data.total > 0 && members.data.total < members.data.all.length, `第2世 ${members.data.total} 人`)

  // 2) 详情
  const detail = loadPage('pages/detail/index.js')
  detail.onLoad({ id: String(members.data.all[0].id) })
  await sleep(1200)
  check('详情载入', !!detail.data.m, detail.data.m && detail.data.m.name)
  check('生卒文案', !!detail.data.deathText, String(detail.data.deathText))
  check('子女/配偶字段存在', Array.isArray(detail.data.children) && Array.isArray(detail.data.spouses))

  // 3) 世系图
  const tree = loadPage('pages/tree/index.js')
  tree.setData({ zupu_token: TOKEN })
  tree.onShow()
  await sleep(1500)
  check('世系图节点', tree.data.nodes.length > 0, `${tree.data.nodes.length} 个节点 / ${tree.data.edges.length} 条连线`)
  check('连线数量合理', tree.data.edges.length > 0)
  const n0 = tree.data.zoom || tree.data.scale
  tree.onTouchStart && tree.onTouchStart({ touches: [{ clientX: 10, clientY: 10 }] })
  tree.onTouchMove && tree.onTouchMove({ touches: [{ clientX: 80, clientY: 60 }] })
  check('拖动改变位移', tree.data.tx !== 30 || tree.data.ty !== 30, `tx=${tree.data.tx} ty=${tree.data.ty}`)
  tree.reset && tree.reset()
  check('复位', tree.data.scale === 1 && tree.data.tx === 30)

  console.log(fail ? `\n${fail} 项失败` : '\n全部通过')
  process.exit(fail ? 1 : 0)
})()
