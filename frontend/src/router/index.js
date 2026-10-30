import { createRouter, createWebHistory } from 'vue-router'
import api from '../api'

const routes = [
  // 无需登录
  { path: '/login', name: 'login', component: () => import('../views/LoginView.vue'), meta: { guest: true } },
  { path: '/change-password', name: 'changePassword', component: () => import('../views/ChangePasswordView.vue') },

  // 需要登录
  { path: '/', redirect: '/dashboard' },
  { path: '/tree', name: 'tree', component: () => import('../views/TreeView.vue'), meta: { auth: true } },
  { path: '/sutree', name: 'sutree', component: () => import('../views/SuTreeView.vue'), meta: { auth: true } },
  { path: '/susi', name: 'susi', component: () => import('../views/SuSiTreeView.vue'), meta: { auth: true } },
  { path: '/suzhu', name: 'suzhu', component: () => import('../views/SuZhuTiView.vue'), meta: { auth: true } },
  { path: '/gentree', name: 'genTree', component: () => import('../views/GenerationTreeView.vue'), meta: { auth: true } },
  { path: '/members', name: 'members', component: () => import('../views/MemberList.vue'), meta: { auth: true } },
  { path: '/branches', name: 'branches', component: () => import('../views/BranchView.vue'), meta: { auth: true } },
  { path: '/member/:id', name: 'memberDetail', component: () => import('../views/MemberDetail.vue'), meta: { auth: true } },
  { path: '/generation', name: 'generation', component: () => import('../views/GenerationView.vue'), meta: { auth: true } },
  // 谱头文化拆成三个子路由（菜单要能正确高亮子项）；旧 ?tab=xxx 链接照旧兼容
  {
    path: '/culture',
    redirect: (to) => {
      const t = String((to.query && to.query.tab) || '')
      const tab = ['events', 'articles', 'bios'].includes(t) ? t : 'events'
      const q = Object.assign({}, to.query || {})
      delete q.tab
      return { path: '/culture/' + tab, query: q }
    }
  },
  { path: '/culture/:tab(events|articles|bios)', name: 'culture', component: () => import('../views/CultureView.vue'), meta: { auth: true } },
  { path: '/knowledge', name: 'knowledge', component: () => import('../views/KnowledgeView.vue'), meta: { auth: true } },
  { path: '/hall', name: 'hall', component: () => import('../views/HallView.vue'), meta: { auth: true } },
  // 祠宇志 / 茔墓志：真实祠堂、真实祖茔的档案（与 /hall「电子祠堂」场景是两回事）
  { path: '/shrines', name: 'shrines', component: () => import('../views/ShrineView.vue'), meta: { auth: true } },
  { path: '/graves', name: 'graves', component: () => import('../views/GraveView.vue'), meta: { auth: true } },
  { path: '/accounts', name: 'accounts', component: () => import('../views/AccountsView.vue'), meta: { auth: true } },
  { path: '/accounts/:id', name: 'accountBook', component: () => import('../views/AccountBookView.vue'), meta: { auth: true } },
  { path: '/merit', name: 'merit', component: () => import('../views/MeritSteleView.vue'), meta: { auth: true } },
  { path: '/ai-assistant', name: 'aiAssistant', component: () => import('../views/AiAssistantView.vue'), meta: { auth: true, roles: ['理事长', '超级管理员'] } },
  { path: '/review', name: 'review', component: () => import('../views/ReviewCenter.vue'), meta: { auth: true, roles: ['理事长', '超级管理员'] } },
  { path: '/dashboard', name: 'dashboard', component: () => import('../views/Dashboard.vue') },
  { path: '/birthday', name: 'birthday', component: () => import('../views/BirthdayView.vue'), meta: { auth: true } },
  { path: '/deathday', name: 'deathday', component: () => import('../views/DeathdayView.vue'), meta: { auth: true } },
  { path: '/traditions', name: 'traditions', component: () => import('../views/TraditionalEventsView.vue'), meta: { auth: true } },
  { path: '/heatmap', name: 'heatmap', component: () => import('../views/HeatmapView.vue'), meta: { auth: true } },
  { path: '/migrations', name: 'migrations', component: () => import('../views/MigrationView.vue'), meta: { auth: true } },
  { path: '/contacts', name: 'contacts', component: () => import('../views/ContactListView.vue'), meta: { auth: true } },
  { path: '/backup', name: 'backup', component: () => import('../views/BackupView.vue'), meta: { auth: true, roles: ['理事长', '超级管理员'] } },
  { path: '/settings', name: 'settings', component: () => import('../views/SettingsView.vue'), meta: { auth: true, roles: ['理事长', '超级管理员'] } },
  { path: '/server-settings', name: 'serverSettings', component: () => import('../views/ServerSettingsView.vue'), meta: { auth: true, roles: ['理事长', '超级管理员'] } },
  { path: '/about', name: 'about', component: () => import('../views/AboutView.vue'), meta: { auth: true, roles: ['理事长', '超级管理员'] } },
  { path: '/usage-guide', name: 'usageGuide', component: () => import('../views/UsageGuideView.vue'), meta: { auth: true, roles: ['理事长', '超级管理员'] } },
  { path: '/data-check', name: 'dataCheck', component: () => import('../views/DataCheckView.vue'), meta: { auth: true, roles: ['理事长', '超级管理员'] } },
  { path: '/update', name: 'update', component: () => import('../views/UpdateView.vue'), meta: { auth: true, roles: ['理事长', '超级管理员'] } },
  { path: '/map-offline', name: 'mapOffline', component: () => import('../views/MapOfflineView.vue'), meta: { auth: true, roles: ['理事长', '超级管理员'] } },
  { path: '/spouses-list', name: 'spousesList', component: () => import('../views/SpouseList.vue'), meta: { auth: true } },
  { path: '/spouse/:id', name: 'spouseDetail', component: () => import('../views/SpouseDetail.vue'), meta: { auth: true } },
  { path: '/demo-dieji', name: 'demoDieji', component: () => import('../views/DiejiDemoView.vue'), meta: { auth: true } },
  { path: '/demo-dieji-land', name: 'demoDiejiLand', component: () => import('../views/DiejiLandView.vue'), meta: { auth: true } },
  { path: '/cover-design', name: 'coverDesign', component: () => import('../views/CoverDesignView.vue'), meta: { auth: true } },
  { path: '/media', name: 'media', component: () => import('../views/MediaView.vue'), meta: { auth: true } },
  { path: '/test-zupu', name: 'testZupu', component: () => import('../views/TestZupuView.vue'), meta: { auth: true } },
  // 兜底：未知路径（含已删页面留下的旧链接，如 /outable）→ 回首页，避免 <router-view> 一片空白
  { path: '/:pathMatch(.*)*', redirect: '/dashboard' }
]

const router = createRouter({
  history: createWebHistory(),
  routes
})

// 路由守卫：检查登录状态
router.beforeEach(async (to, from, next) => {
  const token = localStorage.getItem('token')

  // 初始化 axios header
  if (token) {
    api.defaults.headers.Authorization = 'Bearer ' + token
  }

  // 游客页面直接放行
  if (to.meta.guest) {
    // 仅真正的登录（有 token）才把登录页跳走；本机免登录访客没有 token，应能正常进登录页
    if (token) {
      return next('/dashboard')
    }
    return next()
  }

  // 修改密码页面：有 token 即可
  if (to.path === '/change-password') {
    if (!token) return next('/login')
    return next()
  }

  // 需要登录的页面
  if (to.meta.auth) {
    // 无 token 时也先问一次 /auth/check：本机（127.0.0.1）访问会返回「族员」身份，无需登录
    // 验证 token / 本机身份有效性
    try {
      const resp = await api.get('/auth/check')
      if (resp.data && resp.data.user) {
        localStorage.setItem('user', JSON.stringify(resp.data.user))
        if (resp.data.role_aliases) {
          localStorage.setItem('role_aliases', JSON.stringify(resp.data.role_aliases))
        }
      }
    } catch (e) {
      localStorage.removeItem('token')
      localStorage.removeItem('user')
      delete api.defaults.headers.Authorization
      next('/login')
      return
    }
    // 角色守卫：路由声明了 roles 时，仅允许对应角色访问
    if (to.meta.roles && to.meta.roles.length) {
      let user = null
      try { user = JSON.parse(localStorage.getItem('user') || 'null') } catch (e) { /* ignore */ }
      if (!user || !to.meta.roles.includes(user.role)) {
        return next('/dashboard')
      }
    }
    next()
    return
  }

  next()
})

export default router