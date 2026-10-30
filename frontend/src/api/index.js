import axios from 'axios'

const api = axios.create({
  baseURL: '/api',
  timeout: 10000
})

// 需要按「当前房支」过滤的只读接口前缀
// 注意：/dashboard/upcoming 后端已支持 ?branch_id=&include_children=（handler/upcoming.go 用 BranchScope）
const BRANCH_SCOPED = ['/members', '/heatmap', '/migrations', '/drafts', '/dashboard/upcoming']

// 请求拦截：确保所有请求（含 App 初始化、路由守卫前）都带上 localStorage token
api.interceptors.request.use(cfg => {
  try {
    const t = localStorage.getItem('token')
    if (t) cfg.headers = { ...(cfg.headers || {}), Authorization: 'Bearer ' + t }
  } catch (e) { /* ignore */ }
  return cfg
})

// 请求拦截：自动为房支范围内的 GET 接口附加 branch_id / include_children
api.interceptors.request.use(cfg => {
  try {
    if (cfg.method && cfg.method.toLowerCase() !== 'get') return cfg
    // 个别页面需要「全量成员」自行按 URL 房支过滤（如支谱：根＝URL 分支，含下级取顶部开关），
    // 显式传 skipBranchScope 跳过全局房支范围；不影响其它请求。
    if (cfg.skipBranchScope) return cfg
    const url = cfg.url || ''
    const scoped = BRANCH_SCOPED.some(p => url === p || url.startsWith(p + '?') || url.startsWith(p + '/'))
    if (!scoped) return cfg
    const s = JSON.parse(localStorage.getItem('branchCtx') || 'null')
    if (!s || !s.current) return cfg
    cfg.params = Object.assign({}, cfg.params, {
      branch_id: s.current,
      include_children: s.includeChildren === false ? 0 : 1
    })
  } catch (e) { /* ignore */ }
  return cfg
})

export default api
