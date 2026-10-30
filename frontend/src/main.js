import { createApp } from 'vue'
import ElementPlus from 'element-plus'
import 'element-plus/dist/index.css'
import App from './App.vue'
import router from './router'
import api from './api'
import './style.css'

const app = createApp(App)
app.use(ElementPlus)
app.use(router)

// 本机（127.0.0.1）免登录：无 token 时先向后端确认身份 —— 后端对 loopback 请求会返回「族员」访客身份，
// 拿到即按正常登录后的方式写入本地（角色=族员，看不到也无权用管理/AI 功能）；非本机请求会 401，保持未登录走正常登录流程。
async function bootLocalGuest() {
  if (localStorage.getItem('token')) return                 // 已有 token（正常登录）→ 不动
  if (localStorage.getItem('local_guest') === '1') return    // 已是本机访客 → 不重复请求
  try {
    const { data } = await api.get('/auth/check')
    if (data && data.user) {
      localStorage.setItem('user', JSON.stringify(data.user))
      if (data.role_aliases) localStorage.setItem('role_aliases', JSON.stringify(data.role_aliases))
      if (data.local_guest) localStorage.setItem('local_guest', '1')
    }
  } catch (e) { /* 非本机：忽略，路由守卫会引导到登录页 */ }
}

bootLocalGuest().finally(() => app.mount('#app'))
