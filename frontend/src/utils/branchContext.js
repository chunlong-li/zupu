// 全局房支上下文：顶部下拉选择的房支，作用于各页面的数据范围
//
// 默认值规则（2026-10-18 修订）：
//   · 登录后默认 = 该账号被分配的房支（users.branch 存的是房支名，按名字匹配 branches 表）；
//   · 账号未分配房支（含超管）→ 仍默认「宗谱（全部）」，即 current = 0；
//   · 用户手动改过（包括手动切回宗谱）→ 记在**该账号名下**，下次登录仍用它的选择；
//   · 记录按账号分开存（branchCtx:u<用户id>）→ 换账号登录不会残留上一个账号的房支。
import { reactive, computed, watch } from 'vue'
import api from '../api'

// 旧版全局记录键；同时作为「当前生效值」镜像，供 api/index.js 的请求拦截器读取
const LEGACY_KEY = 'branchCtx'
// 每账号一份记录
const userKey = uid => 'branchCtx:u' + uid
// 每账号记录的结构版本（旧版全局记录没有这个字段）
const SCHEMA = 2

export const branchCtx = reactive({
  branches: [],        // 扁平房支列表（含 parent_id/path/level/subtitle/count/count_all）
  current: 0,          // 当前房支 id，0 = 宗谱（全部）
  includeChildren: true, // 是否含下级房支
  initialized: false,  // 是否已按登录账号确定过默认房支
  loaded: false,
  uid: 0,              // 当前上下文对应的登录账号 id（0 = 未登录 / 本机免登录访客）
  picked: false        // 当前值是否来自用户手动选择（只有 true 才「记住」不动）
})

// 内部赋值期间挂起「用户手动改动」的判定
let internalWrite = false

function readJSON(key) {
  try { return JSON.parse(localStorage.getItem(key) || 'null') } catch (e) { return null }
}
function writeJSON(key, v) {
  try { localStorage.setItem(key, JSON.stringify(v)) } catch (e) { /* ignore */ }
}

// 本地登录账号（未登录 / 本机免登录访客时返回 null）
function loginUser() {
  const u = readJSON('user')
  return u && Number(u.id) > 0 ? u : null
}

// 当前登录账号 id（0 = 未登录 / 本机免登录访客）
export function contextUid() {
  const u = loginUser()
  return u ? Number(u.id) : 0
}

// 房支名 → 房支 id（未分配 / 名字对不上 → 0 = 宗谱）
function branchIdOfName(name) {
  const n = name ? String(name).trim() : ''
  if (!n) return 0
  const hit = branchCtx.branches.find(b => b.name === n)
  return hit ? hit.id : 0
}

// 接口拦截器默认值同一份数据源：当前生效值镜像（含账号 id，便于区分旧版记录）
function persist() {
  writeJSON(LEGACY_KEY, {
    current: branchCtx.current,
    includeChildren: branchCtx.includeChildren,
    uid: branchCtx.uid
  })
  if (!branchCtx.uid) return
  writeJSON(userKey(branchCtx.uid), {
    v: SCHEMA,
    current: branchCtx.current,
    includeChildren: branchCtx.includeChildren,
    picked: branchCtx.picked
  })
}

function assign(v, picked) {
  internalWrite = true
  branchCtx.current = Number(v && v.current) || 0
  branchCtx.includeChildren = !v || v.includeChildren !== false
  branchCtx.picked = !!picked
  internalWrite = false
}

function clearValues() {
  internalWrite = true
  branchCtx.current = 0
  branchCtx.includeChildren = true
  branchCtx.picked = false
  branchCtx.initialized = false
  internalWrite = false
}

// 载入某账号的房支上下文：
//   有它自己的记录（且用户手动选过）→ 用记录；
//   否则标记「待定」，等房支列表到位后按账号所属房支定默认。
function restoreFor(uid) {
  branchCtx.uid = uid
  const saved = uid ? readJSON(userKey(uid)) : null
  if (saved && saved.v === SCHEMA) {
    assign(saved, saved.picked)
    branchCtx.initialized = true
    persist()
    return true
  }
  // 旧版全局记录（没有 v / uid 字段）：只在非宗谱时视为用户当时的选择。
  // 之所以忽略 current=0：旧版本把「默认宗谱」也持久化了，无法与用户主动选宗谱区分，
  // 若沿用就会让「带房支的账号」永远停在宗谱（即本次要修的 bug）。
  const legacy = readJSON(LEGACY_KEY)
  if (legacy && legacy.v === undefined && legacy.uid === undefined && (Number(legacy.current) || 0) > 0) {
    assign(legacy, true)
    branchCtx.initialized = true
    persist()
    return true
  }
  clearValues()
  return false
}

// 初始化：按当前登录账号恢复（未登录时把上下文清空，避免残留上一个账号的房支）
restoreFor(contextUid())

// 任何来源的改动（顶部下拉、成员列表筛选等直接改 branchCtx 的地方）都算「用户选择」并保存
watch([() => branchCtx.current, () => branchCtx.includeChildren], () => {
  if (internalWrite) return
  branchCtx.picked = true
  branchCtx.initialized = true
  persist()
}, { flush: 'sync' })

// 退出登录时调用：清掉内存与镜像，但不删各账号自己的记录（下次登录仍尊重上次选择）
export function resetBranchContext() {
  branchCtx.branches = []
  branchCtx.loaded = false
  branchCtx.uid = 0
  clearValues()
  try { localStorage.removeItem(LEGACY_KEY) } catch (e) { /* ignore */ }
}

export function branchLabel(b) {
  if (!b) return ''
  return b.name + (b.subtitle ? ' · ' + b.subtitle : '')
}

export function findBranch(id) {
  return branchCtx.branches.find(b => b.id === id) || null
}

// 拉取房支列表；首次（该账号没选过）按账号所属房支定默认；选了就尊重记录
// 并发调用（App 挂载 / 路由切换 / 页面挂载）共用同一次请求
let branchesInflight = null
export function loadBranches() {
  if (!branchesInflight) {
    branchesInflight = reloadBranches().finally(() => { branchesInflight = null })
  }
  return branchesInflight
}

async function reloadBranches() {
  if (!localStorage.getItem('token')) return
  try {
    const { data } = await api.get('/branches')
    branchCtx.branches = data || []
    branchCtx.loaded = true
  } catch (e) {
    return
  }

  // 换账号（登录 / 切换账号）→ 丢弃上一个账号的上下文，按本账号的记录重建
  const uid = contextUid()
  if (uid !== branchCtx.uid) restoreFor(uid)

  // 没有「用户选择」记录，或记录里的房支已被删除 → 按账号所属房支定默认
  const needDefault = !branchCtx.initialized || !branchCtx.picked ||
    (branchCtx.current !== 0 && !findBranch(branchCtx.current))
  if (needDefault) {
    const account = await effectiveUser()
    if (account && Number(account.id) > 0) branchCtx.uid = Number(account.id)  // 本地 user 记录缺失时对齐账号 id
    const def = branchIdOfName(account && account.branch)
    internalWrite = true
    branchCtx.current = def
    branchCtx.initialized = true
    branchCtx.picked = false
    internalWrite = false
    persist()
  }
}

// 「账号所属房支」的数据源：
//   登录响应里已带 branch（登录/换账号后都是最新的）→ 直接用；
//   本地 user 记录里没有房支（可能是登录后管理员才分配的，或者是超管）→ 用 token 向
//   后端要一次最新身份（/auth/check），并顺手更新本地记录。每次页面加载最多问一次。
let userRefreshed = false
async function effectiveUser() {
  const u = loginUser()
  if (u && String(u.branch || '').trim()) return u
  if (userRefreshed || !localStorage.getItem('token')) return u
  userRefreshed = true
  try {
    const { data } = await api.get('/auth/check')
    const nu = data && data.user
    if (nu && Number(nu.id) > 0) {
      try { localStorage.setItem('user', JSON.stringify(nu)) } catch (e) { /* ignore */ }
      return nu
    }
  } catch (e) { /* ignore */ }
  return u
}

export function setCurrent(id) {
  branchCtx.current = Number(id) || 0
}

export function setIncludeChildren(v) {
  branchCtx.includeChildren = !!v
}

// 当前房支范围的中文说明（用于页面提示 / 截图核对）
export function branchScopeLabel() {
  if (!branchCtx.current) return '宗谱（全族）'
  const b = findBranch(branchCtx.current)
  const name = b ? branchLabel(b) : ('房支 #' + branchCtx.current)
  return name + (branchCtx.includeChildren ? '（含下级）' : '')
}

// 供 el-tree-select 使用的树（根为「宗谱（全部）」，value=0）
export function buildTreeSelect() {
  const nodes = branchCtx.branches.map(b => ({
    value: b.id,
    label: branchLabel(b),
    parent_id: b.parent_id || 0,
    sort_order: b.sort_order || 0,
    children: []
  }))
  const byId = {}
  nodes.forEach(n => { byId[n.value] = n })
  const roots = []
  nodes.forEach(n => {
    if (n.parent_id && byId[n.parent_id]) byId[n.parent_id].children.push(n)
    else roots.push(n)
  })
  const sortRec = list => {
    list.sort((a, b) => (a.sort_order - b.sort_order) || (a.value - b.value))
    list.forEach(x => sortRec(x.children))
  }
  return [{ value: 0, label: '宗谱（全部）', children: roots }]
}

// 扁平选项（带缩进），用于普通 el-select
export function buildFlatOptions() {
  const out = []
  const walk = (list, depth) => {
    list.forEach(n => {
      if (n.value !== 0) out.push({ id: n.value, label: '　'.repeat(depth) + n.label, depth })
      walk(n.children || [], depth + (n.value === 0 ? 0 : 1))
    })
  }
  walk(buildTreeSelect(), 0)
  return out
}

export const branchTree = computed(() => buildTreeSelect())

// 构造接口参数（房支筛选）：{ branch_id, include_children }
export function branchParams() {
  const p = {}
  if (branchCtx.current > 0) {
    p.branch_id = branchCtx.current
    p.include_children = branchCtx.includeChildren ? 1 : 0
  }
  return p
}
